package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/ethereum/go-ethereum/crypto"
)

type MPTOperation uint32

var ErrListTooBig = errors.New("list too big")

const (
	ReadMPTOperation MPTOperation = iota
	SetMPTOperation
	DeleteMPTOperation
)

type SimpleProof []Node

type Proof struct {
	operation MPTOperation
	key       []byte
	// path from root to the node with max matching to key prefix
	// if key is presented in the MPT this'll be simply path to corresponding node
	PathToNode SimpleProof
}

// BuildProof constructs a proof for the given key in the MPT.
// If no common path is found, the root node is included to prove that the key does not exist in the trie.
func BuildProof(tree *Reader, key []byte, op MPTOperation) (Proof, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	p := Proof{operation: op, key: key}

	pathToNode, err := BuildSimpleProof(tree, key)
	if err != nil {
		return p, err
	}
	p.PathToNode = pathToNode

	return p, nil
}

/*
verifies the merkle proof for read operation
@value: an actual value returned by read. should be nil for read that finished with ErrKeyNotFound
@rootHash: hash of MPT after the operation
*/
func (p *Proof) VerifyRead(key []byte, value []byte, rootHash common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.operation != ReadMPTOperation || !bytes.Equal(p.key, key) {
		return false, nil
	}

	mpt, err := unwrapSparseMpt(p)
	if err != nil {
		return false, err
	}

	// first check that the tree root is the same as given
	if !rootHash.Uint256().Eq(mpt.RootHash().Uint256()) {
		return false, nil
	}

	val, err := mpt.Get(p.key)

	if len(value) != 0 {
		// read op was successful and returned some value
		// now we need to check that the proof contains the same value
		return bytes.Equal(val, value), nil
	}

	// otherwise read op finished with ErrKeyNotFound
	// check that we also don't have this key in our tree
	return errors.Is(err, db.ErrKeyNotFound), nil
}

/*
verifies the merkle proof for delete operation
@deleted: whether key was actually removed
@rootHash: root hash of the original MPT
@newRootHash: root hash of MPT after the operation
*/
func (p *Proof) VerifyDelete(key []byte, deleted bool, rootHash, newRootHash common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.operation != DeleteMPTOperation || !bytes.Equal(p.key, key) {
		return false, nil
	}

	mpt, err := unwrapSparseMpt(p)
	if err != nil {
		return false, err
	}

	if !rootHash.Uint256().Eq(mpt.RootHash().Uint256()) {
		return false, nil
	}

	err = mpt.Delete(key)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return false, err
	}

	// now check that the tree root after deletion is the same as given in proof
	if !newRootHash.Uint256().Eq(mpt.RootHash().Uint256()) {
		return false, nil
	}

	return deleted == (err == nil), nil
}

/*
verifies the merkle proof for set operation
@rootHash: root hash of the original MPT
@newRootHash: root hash of MPT after the operation
*/
func (p *Proof) VerifySet(key []byte, value []byte, rootHash, newRootHash common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.operation != SetMPTOperation || !bytes.Equal(p.key, key) {
		return false, nil
	}

	mpt, err := unwrapSparseMpt(p)
	if err != nil {
		return false, err
	}

	if !rootHash.Uint256().Eq(mpt.RootHash().Uint256()) {
		return false, nil
	}

	// first modify the original tree
	if err := mpt.Set(p.key, value); err != nil {
		return false, err
	}

	// now check that the tree root after modification is the same as given in proof
	return newRootHash.Uint256().Eq(mpt.RootHash().Uint256()), nil
}

// unwrapSparseMpt creates sparse MPT trie from proof
func unwrapSparseMpt(p *Proof) (*MerklePatriciaTrie, error) {
	mpt := NewInMemMPT()
	if err := PopulateMptWithProof(mpt, p); err != nil {
		return nil, err
	}
	return mpt, nil
}

// PopulateMptWithProofNodes populates it with nodes contained in the proof, setting the root of the `mpt` instance
// to the first node from the `PathToNode` slice.
func PopulateMptWithProof(mpt *MerklePatriciaTrie, p *Proof) error {
	return populateMptWithProofNodes(mpt, p.PathToNode, true)
}

// populateMptWithProofNodes populates it with nodes contained in the proof, if `setRoot` is true,
// also sets the root of the `mpt` instance to the first node from the slice.
func populateMptWithProofNodes(mpt *MerklePatriciaTrie, proofNodes SimpleProof, setRoot bool) error {
	for i, node := range proofNodes {
		if nodeRef, err := mpt.storeNode(node); err != nil {
			return err
		} else if i == 0 && setRoot {
			mpt.root = nodeRef
		}
	}
	return nil
}

func ValidateHolder(holder InMemHolder) error {
	for key, value := range holder {
		expectedKey := calcNodeKey(value)
		if !bytes.Equal([]byte(key), expectedKey) {
			return fmt.Errorf("key %x doesn't match the hash %x of value", key, expectedKey)
		}
	}
	return nil
}

func getMaxMatchingRoute(tree *Reader, key []byte) ([]Node, error) {
	if tree.root == nil {
		return []Node{}, nil
	}

	nodes := make([]Node, 0)
	_, err := tree.descendWithCallback(tree.root, *newPath(key, false), func(n Node) {
		nodes = append(nodes, n)
	})
	if errors.Is(err, db.ErrKeyNotFound) {
		err = nil
	}

	return nodes, err
}

func (p *Proof) Encode() ([]byte, error) {
	if len(p.key) > maxRawKeyLen || len(p.PathToNode) >= (1<<8) {
		return nil, ErrListTooBig
	}

	buf := make([]byte, 0, len(p.key)+5)
	buf = binary.BigEndian.AppendUint32(buf, uint32(p.operation))
	buf = append(buf, byte(len(p.key)))
	buf = append(buf, p.key...)

	encodedPath, err := p.PathToNode.Encode()
	if err != nil {
		return nil, err
	}
	buf = append(buf, encodedPath...)

	return buf, nil
}

func DecodeProof(data []byte) (Proof, error) {
	// here we deserialize proof from the data piece by piece
	// and each time advance the offset on correct amount of bytes

	p := Proof{}
	p.operation = MPTOperation(binary.BigEndian.Uint32(data[:4]))
	keyLen := data[4]
	data = data[5:]

	p.key = data[0:keyLen]
	data = data[keyLen:]

	sp, err := DecodeSimpleProof(data)
	if err != nil {
		return p, err
	}
	p.PathToNode = sp

	return p, nil
}

// BuildSimpleProof constructs a `SimpleProof` for the given key in the MPT.
// If no common path is found, the root node is included to prove that the key does not exist in the trie.
func BuildSimpleProof(tree *Reader, key []byte) (SimpleProof, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}

	path, err := getMaxMatchingRoute(tree, key)
	if err != nil {
		return nil, err
	}

	if len(path) == 0 && tree.root != nil {
		if rootNode, err := tree.getNode(tree.root); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
		} else {
			path = []Node{rootNode}
		}
	}

	return path, nil
}

func (sp *SimpleProof) Encode() ([]byte, error) {
	buf := make([]byte, 0, 1)
	buf = append(buf, byte(len(*sp)))
	for i := range *sp {
		node, err := (*sp)[i].Encode()
		if err != nil {
			return nil, err
		}

		buf = binary.BigEndian.AppendUint32(buf, uint32(len(node)))
		buf = append(buf, node...)
	}

	return buf, nil
}

func DecodeSimpleProof(data []byte) (SimpleProof, error) {
	// here we deserialize simple proof from the data piece by piece
	// and each time advance the offset on correct amount of bytes

	sp := make(SimpleProof, 0)
	proofLen := data[0]
	data = data[1:]
	for range proofLen {
		nodeLen := binary.BigEndian.Uint32(data[:4])
		node, err := DecodeNode(data[4 : 4+nodeLen])
		if err != nil {
			return nil, err
		}

		sp = append(sp, node)
		data = data[4+nodeLen:]
	}

	return sp, nil
}

func (sp *SimpleProof) ToBytesSlice() ([][]byte, error) {
	bytesSlice := make([][]byte, 0, len(*sp))
	for i := range *sp {
		encodedNode, err := (*sp)[i].Encode()
		if err != nil {
			return nil, err
		}
		bytesSlice = append(bytesSlice, encodedNode)
	}

	return bytesSlice, nil
}

func SimpleProofFromBytesSlice(data [][]byte) (SimpleProof, error) {
	sp := make(SimpleProof, 0, len(data))
	for _, encodedNode := range data {
		node, err := DecodeNode(encodedNode)
		if err != nil {
			return nil, err
		}
		sp = append(sp, node)
	}
	return sp, nil
}

func (sp *SimpleProof) Verify(rootHash common.Hash, key []byte) ([]byte, error) {
	trie := NewInMemMPT()
	// populate without setting the root
	if err := populateMptWithProofNodes(trie, *sp, false); err != nil {
		return nil, err
	}
	trie.root = rootHash[:]
	val, err := trie.Get(key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	return val, err
}
