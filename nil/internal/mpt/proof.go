package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

type Operation uint32

const (
	ReadOperation Operation = iota
	SetOperation
	DeleteOperation
)

type Proof struct {
	Operation Operation
	Key       []byte
	Nodes     [][]byte // RLP-encoded trie nodes forming the proof
}

func BuildProof(r *Reader, key []byte, op Operation) (Proof, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}

	if r.RootHash() == EmptyRootHash {
		emptyNode := []byte{0x80}
		emptyHash := common.Keccak256Hash(emptyNode)
		check.PanicIfNotf(emptyHash == EmptyRootHash, "empty node hash mismatch")
		return Proof{
			Operation: op,
			Key:       key,
			Nodes:     [][]byte{emptyNode},
		}, nil
	}

	collector := &proofNodeCollector{}
	if err := r.trie.Prove(key, collector); err != nil {
		return Proof{}, err
	}

	nodeList := make([][]byte, 0, len(collector.nodes))
	for _, node := range collector.nodes {
		nodeList = append(nodeList, node.value)
	}

	return Proof{
		Operation: op,
		Key:       key,
		Nodes:     nodeList,
	}, nil
}

func (p *Proof) ToBytesSlice() [][]byte {
	return p.Nodes
}

func (p *Proof) VerifyRead(key, value []byte, root common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.Operation != ReadOperation || !bytes.Equal(p.Key, key) {
		return false, nil
	}

	// Empty trie special case
	if root == EmptyRootHash {
		if len(p.Nodes) != 1 {
			return false, errors.New("invalid empty proof: unexpected number of nodes")
		}
		if !bytes.Equal(p.Nodes[0], []byte{0x80}) {
			return false, errors.New("invalid empty proof: expected RLP(nil)")
		}
		return len(value) == 0, nil
	}

	val, err := trie.VerifyProof(ethcommon.Hash(root), key, proofNodes(p.Nodes))
	if err != nil {
		return false, err
	}
	return bytes.Equal(val, value), nil
}

func VerifyProof(rootHash common.Hash, key []byte, nodes [][]byte) ([]byte, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if rootHash == EmptyRootHash {
		if len(nodes) != 1 || !bytes.Equal(nodes[0], []byte{0x80}) {
			return nil, errors.New("invalid proof for empty trie")
		}
		return nil, nil
	}
	return trie.VerifyProof(ethcommon.Hash(rootHash), key, proofNodes(nodes))
}

func (p *Proof) VerifyDelete(key []byte, deleted bool, root, newRoot common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.Operation != DeleteOperation || !bytes.Equal(p.Key, key) {
		return false, nil
	}

	mpt := NewInMemMPT()
	if err := PopulateTrieWithProof(mpt, p, root); err != nil {
		return false, err
	}

	err := mpt.Delete(key)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return false, err
	}
	return mpt.RootHash() == newRoot && deleted == (err == nil), nil
}

func (p *Proof) VerifySet(key, value []byte, root, newRoot common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.Operation != SetOperation || !bytes.Equal(p.Key, key) {
		return false, nil
	}

	mpt := NewInMemMPT()
	if err := PopulateTrieWithProof(mpt, p, root); err != nil {
		return false, err
	}

	if err := mpt.Set(key, value); err != nil {
		return false, err
	}
	return mpt.RootHash() == newRoot, nil
}

func PopulateTrieWithProof(mpt *MerklePatriciaTrie, p *Proof, root common.Hash) error {
	if err := PopulateMptWithProof(mpt, p); err != nil {
		return err
	}
	return mpt.SetRootHash(root)
}

// PopulateMptWithProof sets proof nodes to MPT without modifying root
func PopulateMptWithProof(mpt *MerklePatriciaTrie, p *Proof) error {
	for _, node := range p.Nodes {
		hash := common.Keccak256Hash(node)
		if err := mpt.setter.Set(hash[:], node); err != nil {
			return err
		}
	}
	return nil
}

func (p *Proof) Encode() ([]byte, error) {
	if len(p.Key) > maxRawKeyLen || len(p.Nodes) >= 256 {
		return nil, ErrListTooBig
	}
	buf := make([]byte, 0)
	buf = binary.BigEndian.AppendUint32(buf, uint32(p.Operation))
	buf = append(buf, byte(len(p.Key)))
	buf = append(buf, p.Key...)
	buf = append(buf, byte(len(p.Nodes)))
	for _, node := range p.Nodes {
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(node)))
		buf = append(buf, node...)
	}
	return buf, nil
}

func DecodeProof(data []byte) (Proof, error) {
	var p Proof
	if len(data) < 6 {
		return p, errors.New("too short")
	}
	p.Operation = Operation(binary.BigEndian.Uint32(data[0:4]))
	keyLen := int(data[4])
	p.Key = make([]byte, keyLen)
	copy(p.Key, data[5:5+keyLen])
	off := 5 + keyLen
	n := int(data[off])
	off++
	p.Nodes = make([][]byte, 0, n)
	for range n {
		if len(data[off:]) < 4 {
			return p, errors.New("truncated node")
		}
		ln := binary.BigEndian.Uint32(data[off : off+4])
		off += 4
		if len(data[off:]) < int(ln) {
			return p, errors.New("truncated blob")
		}
		p.Nodes = append(p.Nodes, data[off:off+int(ln)])
		off += int(ln)
	}
	return p, nil
}

type kvPair struct {
	key   common.Hash
	value []byte
}

// proofNodeCollector implements trie.ProofWriter
type proofNodeCollector struct {
	nodes []kvPair
}

func (p *proofNodeCollector) Put(key, value []byte) error {
	p.nodes = append(p.nodes, kvPair{
		key:   common.BytesToHash(key),
		value: value,
	})
	return nil
}

func (p *proofNodeCollector) Delete(key []byte) error {
	// noop: not used for proof construction
	return nil
}

// proofNodes implements ethdb.KeyValueReader using proof node slices
type proofNodeReader struct {
	nodes [][]byte
}

func (p *proofNodeReader) Get(key []byte) ([]byte, error) {
	hash := ethcommon.BytesToHash(key)
	for _, node := range p.nodes {
		if crypto.Keccak256Hash(node) == hash {
			return node, nil
		}
	}
	return nil, errMissingNode
}

func (p *proofNodeReader) Has(key []byte) (bool, error) {
	_, err := p.Get(key)
	if err != nil && !errors.Is(err, errMissingNode) {
		return false, err
	}
	return err == nil, nil
}

func proofNodes(nodes [][]byte) ethdb.KeyValueReader {
	return &proofNodeReader{nodes: nodes}
}
