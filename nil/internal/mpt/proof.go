// Package mpt wraps go-ethereum's Merkle Patricia Trie.
package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
)

type MPTOperation uint32

var ErrListTooBig = errors.New("list too big")

const (
	ReadMPTOperation MPTOperation = iota
	SetMPTOperation
	DeleteMPTOperation
)

type Proof struct {
	operation MPTOperation
	key       []byte
	Nodes     [][]byte // RLP-encoded trie nodes forming the proof
}

func BuildProof(r *Reader, key []byte, op MPTOperation) (Proof, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}

	// Special case: empty trie (EmptyRootHash)
	if r.RootHash() == EmptyRootHash {
		emptyNode := []byte{0x80}
		return Proof{
			operation: op,
			key:       key,
			Nodes:     [][]byte{emptyNode},
		}, nil
	}

	db := newNodeDatabase(r.getter)
	t, err := trie.New(trie.TrieID(ethcommon.Hash(r.RootHash())), db)
	if err != nil {
		return Proof{}, err
	}

	nodes := make(map[ethcommon.Hash][]byte)
	collector := &proofNodeCollector{nodes: nodes}

	if err = t.Prove(key, collector); err != nil {
		return Proof{}, err
	}

	nodeList := make([][]byte, 0, len(collector.nodes))
	for _, node := range collector.nodes {
		nodeList = append(nodeList, node)
	}

	return Proof{
		operation: op,
		key:       key,
		Nodes:     nodeList,
	}, nil
}

func (p *Proof) PathToNode() SimpleProof {
	proof := make(SimpleProof, 0, len(p.Nodes))
	for _, encoded := range p.Nodes {
		node, err := DecodeNode(encoded)
		if err == nil {
			proof = append(proof, node)
		}
	}
	return proof
}

func (p *Proof) ToBytesSlice() [][]byte {
	return p.Nodes
}

func (p *Proof) VerifyRead(key, value []byte, root common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.operation != ReadMPTOperation || !bytes.Equal(p.key, key) {
		return false, nil
	}

	val, err := trie.VerifyProof(ethcommon.Hash(root), key, proofNodes(p.Nodes))
	if err != nil && err.Error() != "missing node" {
		return false, err
	}

	if len(value) != 0 {
		return bytes.Equal(val, value), nil
	}
	return val == nil, nil
}

func (p *Proof) VerifyDelete(key []byte, deleted bool, root, newRoot common.Hash) (bool, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	if p.operation != DeleteMPTOperation || !bytes.Equal(p.key, key) {
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
	if p.operation != SetMPTOperation || !bytes.Equal(p.key, key) {
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
	mpt.root = root[:]
	return PopulateMptWithProof(mpt, p)
}

// PopulateMptWithProof sets proof nodes to MPT without modifying root
func PopulateMptWithProof(mpt *MerklePatriciaTrie, p *Proof) error {
	for _, node := range p.Nodes {
		hash := crypto.Keccak256Hash(node)
		if err := mpt.setter.Set(hash[:], node); err != nil {
			return err
		}
	}
	return nil
}

func (p *Proof) Encode() ([]byte, error) {
	if len(p.key) > maxRawKeyLen || len(p.Nodes) >= 256 {
		return nil, ErrListTooBig
	}
	buf := make([]byte, 0)
	buf = binary.BigEndian.AppendUint32(buf, uint32(p.operation))
	buf = append(buf, byte(len(p.key)))
	buf = append(buf, p.key...)
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
	p.operation = MPTOperation(binary.BigEndian.Uint32(data[0:4]))
	keyLen := int(data[4])
	p.key = make([]byte, keyLen)
	copy(p.key, data[5:5+keyLen])
	off := 5 + keyLen
	n := int(data[off])
	off++
	p.Nodes = make([][]byte, 0, n)
	for i := 0; i < n; i++ {
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

// proofNodeCollector implements trie.ProofWriter
type proofNodeCollector struct {
	nodes map[ethcommon.Hash][]byte
}

func (p *proofNodeCollector) Put(key, value []byte) error {
	p.nodes[ethcommon.BytesToHash(key)] = value
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
	return nil, errors.New("missing node")
}

func (p *proofNodeReader) Has(key []byte) (bool, error) {
	_, err := p.Get(key)
	if err != nil && err.Error() != "missing node" {
		return false, err
	}
	return err == nil, nil
}

func proofNodes(nodes [][]byte) ethdb.KeyValueReader {
	return &proofNodeReader{nodes: nodes}
}

// SimpleProof is a parsed list of MPT nodes
// useful for visual inspection or transformation
// (e.g. PathToNode)
type SimpleProof []Node

func (sp SimpleProof) ToBytesSlice() ([][]byte, error) {
	res := make([][]byte, 0, len(sp))
	for _, node := range sp {
		encoded, err := node.Encode()
		if err != nil {
			return nil, err
		}
		res = append(res, encoded)
	}
	return res, nil
}

func (sp SimpleProof) Verify(root common.Hash, key []byte) ([]byte, error) {
	encoded, err := sp.ToBytesSlice()
	if err != nil {
		return nil, err
	}
	return trie.VerifyProof(ethcommon.Hash(root), key, proofNodes(encoded))
}
