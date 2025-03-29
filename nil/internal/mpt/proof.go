package mpt

import (
	"bytes"
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type MPTOperation uint32

const (
	ReadMPTOperation MPTOperation = iota
	SetMPTOperation
	DeleteMPTOperation
)

type ProofPath []Node

type Proof struct {
	operation MPTOperation
	key       []byte
	// path from root to the node with max matching to key prefix
	// if key is presented in the MPT this'll be simply path to corresponding node
	PathToNode ProofPath
}

// BuildProof constructs a proof for the given key in the MPT.
// If no common path is found, the root node is included to prove that the key does not exist in the trie.
func BuildProof(tree *Reader, key []byte, op MPTOperation) (Proof, error) {
	if len(key) > maxRawKeyLen {
		key = poseidon.Sum(key)
	}
	p := Proof{operation: op, key: key}

	if path, err := getMaxMatchingRoute(tree, key); err != nil {
		return p, err
	} else {
		p.PathToNode = path
	}

	if len(p.PathToNode) == 0 && tree.root != nil {
		if rootNode, err := tree.getNode(tree.root); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return p, err
			}
		} else {
			p.PathToNode = []Node{rootNode}
		}
	}

	return p, nil
}

/*
verifies the merkle proof for read operation
@value: an actual value returned by read. should be nil for read that finished with ErrKeyNotFound
@rootHash: hash of MPT after the operation
*/
func (p *Proof) VerifyRead(key []byte, value []byte, rootHash common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = poseidon.Sum(key)
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
		key = poseidon.Sum(key)
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
		key = poseidon.Sum(key)
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

// PopulateMptWithProof sets the root of the `mpt` instance to the first node from the `p` proof
// and populates it with nodes contained in the proof.
func PopulateMptWithProof(mpt *MerklePatriciaTrie, p *Proof) error {
	for i, node := range p.PathToNode {
		if nodeRef, err := mpt.storeNode(node); err != nil {
			return err
		} else if i == 0 {
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
		return nil, ssz.ErrListTooBig
	}

	buf := make([]byte, 0)
	buf = ssz.MarshalUint32(buf, uint32(p.operation))

	buf = ssz.MarshalUint8(buf, uint8(len(p.key)))
	buf = append(buf, p.key...)

	buf = ssz.MarshalUint8(buf, uint8(len(p.PathToNode)))
	for i := range p.PathToNode {
		node, err := p.PathToNode[i].Encode()
		if err != nil {
			return nil, err
		}
		buf = ssz.MarshalUint32(buf, uint32(len(node)))
		buf = append(buf, node...)
	}

	return buf, nil
}

func DecodeProof(data []byte) (Proof, error) {
	// here we deserialize proof from the data piece by piece
	// and each time advance the offset on correct amount of bytes

	p := Proof{}
	p.operation = MPTOperation(ssz.UnmarshallUint32(data))
	data = data[4:]

	keyLen := ssz.UnmarshallUint8(data)
	p.key = data[1 : 1+keyLen]
	data = data[1+keyLen:]

	pathLen := ssz.UnmarshallUint8(data)
	data = data[1:]

	for range pathLen {
		nodeLen := ssz.UnmarshallUint32(data)

		node, err := DecodeNode(data[4 : 4+nodeLen])
		if err != nil {
			return p, err
		}

		p.PathToNode = append(p.PathToNode, node)
		data = data[4+nodeLen:]
	}

	return p, nil
}
