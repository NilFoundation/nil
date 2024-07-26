package mpt

import (
	"errors"

	"github.com/NilFoundation/nil/core/db"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type MPTProof []Node

// Creates Sparse MPT from slice of Nodes. It could be used to get values by keys of any Node
// included into the proof.
func (p MPTProof) CreateMerklePatriciaTrie() (*MerklePatriciaTrie, error) {
	if len(p) == 0 {
		return nil, errors.New("empty proof")
	}

	holder := make(map[string][]byte)
	mpt := NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
	rootKey, err := mpt.storeNode(p[0])
	if err != nil {
		return nil, err
	}
	mpt.root = rootKey

	for _, node := range p[1:] {
		// storeNode computes hash of the node for each insertion, thus, successful following Get operation
		//   will indicate the validiy of the proof
		_, err = mpt.storeNode(node)
		if err != nil {
			return nil, err
		}
	}
	return mpt, nil
}

// CreateProof constructs a merkle proof for key. The result contains all nodes
// on the path to the value at key. The value itself is also included in the last
// node and can be retrieved by verifying the proof.
//
// If the trie does not contain a value for key, the returned proof contains all
// nodes of the longest existing prefix of the key (at least the root node), ending
// with the node that proves the absence of the key.
func (m *Reader) CreateProof(key []byte) (MPTProof, error) {
	if m.root == nil {
		// TODO: use error from MPT pkg?
		return nil, db.ErrKeyNotFound
	}
	if len(key) > maxRawKeyLen {
		key = poseidon.Sum(key)
	}
	path := newPath(key, 0)

	proof, err := m.extendProof(m.root, *path, nil)
	if err != nil {
		return nil, err
	}

	return proof, nil
}

// Traverses the trie appending current node to proof slice. Almost the copy of get(...) method
func (m *Reader) extendProof(nodeRef Reference, path Path, proof MPTProof) (MPTProof, error) {
	node, err := m.getNode(nodeRef)
	if err != nil {
		return nil, err
	}

	proof = append(proof, node)

	// If the path is empty, our travel is over. Main `get` method will check if this node has a value.
	if path.Size() == 0 {
		return proof, nil
	}

	switch node := node.(type) {
	case *LeafNode:
		// If we've found a leaf, it's either the leaf we're looking for or wrong leaf.
		if node.Path().Equal(&path) {
			return proof, nil
		}

	case *ExtensionNode:
		// If we've found an extension, we need to go deeper.
		if path.StartsWith(node.Path()) {
			restPath := path.Consume(node.Path().Size())
			return m.extendProof(node.NextRef, *restPath, proof)
		}

	case *BranchNode:
		// If we've found a branch node, go to the appropriate branch.
		branch := node.Branches[path.At(0)]
		if len(branch) > 0 {
			return m.extendProof(branch, *path.Consume(1), proof)
		}

	default:
		panic("Invalid node")
	}

	// TODO: use error from MPT pkg?
	return nil, db.ErrKeyNotFound
}

// VerifyProof checks existence of the key in a merkle proof. Returns the value for
// the key stored in the proof.
func VerifyProof(proof MPTProof, key []byte) ([]byte, error) {
	mpt, err := proof.CreateMerklePatriciaTrie()
	if err != nil {
		return nil, err
	}

	return mpt.Get(key)
}
