package mpt

import (
	"fmt"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type DeleteAction int

const (
	daUnknown DeleteAction = iota
	daDeleted
	daUpdated
	daUselessBranch
)

const TableName = "mpt"

var EmptyHash = common.BytesToHash(poseidon.Sum([]byte{0}))

type MerklePatriciaTrie struct {
	db   db.DB
	root Reference
}

func NewMerklePatriciaTrie(db db.DB) *MerklePatriciaTrie {
	return &MerklePatriciaTrie{db, nil}
}

func NewMerklePatriciaTrieWithRoot(db db.DB, root common.Hash) *MerklePatriciaTrie {
	return &MerklePatriciaTrie{db, root.Bytes()}
}

func (m *MerklePatriciaTrie) RootHash() common.Hash {
	if !m.root.IsValid() {
		return EmptyHash
	}
	return common.BytesToHash(m.root)
}

func (m *MerklePatriciaTrie) Get(key []byte) (ret []byte, err error) {
	if m.root == nil {
		return nil, fmt.Errorf("key error")
	}
	if len(key) > 32 {
		key = poseidon.Sum(key)
	}
	path := newPath(key, 0)

	node, err := m.get(m.root, *path)
	if err != nil {
		return nil, err
	}

	return node.Data(), nil
}

func (m *MerklePatriciaTrie) Set(key []byte, value []byte) error {
	if len(key) > 32 {
		key = poseidon.Sum(key)
	}

	path := newPath(key, 0)
	root, err := m.set(m.root, *path, value)
	if err != nil {
		return err
	}
	m.root = root

	return nil
}

func (m *MerklePatriciaTrie) Delete(key []byte) error {
	if !m.root.IsValid() {
		return nil
	}
	if len(key) > 32 {
		key = poseidon.Sum(key)
	}
	path := newPath(key, 0)

	action, info, err := m.delete(m.root, path)
	if err != nil {
		return err
	}

	switch action {
	case daDeleted:
		// Trie is empty
		m.root = nil
	case daUpdated:
		m.root = info.ref
	case daUselessBranch:
		m.root = info.ref
	default:
		return fmt.Errorf("invalid action")
	}
	return nil
}

type Info struct {
	path Path
	ref  Reference
}

var NoInfo = Info{Path{}, nil}

func (m *MerklePatriciaTrie) delete(nodeRef Reference, path *Path) (DeleteAction, Info, error) {
	node, err := m.getNode(nodeRef)
	if err != nil {
		return daUnknown, NoInfo, err
	}

	switch node := node.(type) {
	case *LeafNode:
		// If it's leaf node, then it's either node we need or incorrect key provided.
		if path.Equal(node.Path()) {
			return daDeleted, NoInfo, nil
		}
		return daUnknown, NoInfo, fmt.Errorf("key error")

	case *ExtensionNode:
		// Extension node can't be removed directly, it passes delete request to the next node.
		// After that several options are possible:
		// 1. Next node was deleted. Then this node should be deleted too.
		// 2. Next node was updated. Then we should update stored reference.
		// 3. Next node was useless branch. Then we have to update our node depending on the next node type.

		if !path.StartsWith(node.Path()) {
			return daUnknown, NoInfo, fmt.Errorf("key error")
		}
		action, info, err := m.delete(node.NextRef, path.Consume(node.Path().Size()))
		if err != nil {
			return daUnknown, NoInfo, err
		}

		switch action {
		case daDeleted:
			// Next node was deleted. This node should be deleted also.
			return action, NoInfo, nil
		case daUpdated:
			// Next node was updated. Update this node too.
			newRef, err := m.storeNode(newExtensionNode(node.Path(), info.ref))
			if err != nil {
				return daUnknown, NoInfo, err
			}
			return action, Info{Path{}, newRef}, nil

		case daUselessBranch:
			// Next node was useless branch.
			child, err := m.getNode(info.ref)
			if err != nil {
				return daUnknown, NoInfo, err
			}

			var newNode Node = nil
			switch child := child.(type) {
			case *LeafNode:
				// If next node is the leaf, our node is unnecessary.
				// Concat our path with leaf path and return reference to the leaf.
				path = node.Path().Combine(child.Path())
				newNode = newLeafNode(path, child.Data())

			case *ExtensionNode:
				// If next node is the extension, merge this and next node into one.
				path = node.Path().Combine(child.Path())
				newNode = newExtensionNode(path, child.NextRef)

			case *BranchNode:
				// If next node is the branch, concatenate paths and update stored reference.
				path = node.Path().Combine(&info.path)
				newNode = newExtensionNode(path, info.ref)

			default:
				panic("Invalid node")
			}

			newReference, err := m.storeNode(newNode)
			if err != nil {
				return daUnknown, NoInfo, err
			}

			return daUpdated, Info{Path{}, newReference}, nil
		default:
			return daUnknown, NoInfo, fmt.Errorf("invalid action")
		}

	case *BranchNode:
		// For branch node things are quite complicated.
		// If rest of the key is empty and there is stored value, just clear value field.
		// Otherwise call _delete for the appropriate branch.
		// At this step we will have delete action and (possibly) index of the branch we're working with.
		//
		// Then, if next node was updated or was useless branch, just update reference.
		// If `_DeleteAction` is `DELETED` then either the next node or value of this node was removed.
		// We have to check if there is at least 2 branches or 1 branch and value still persist in this node.
		// If there are no branches and no value left, delete this node completely.
		// If there is a value but no branches, create leaf node with value and empty path
		// and return `USELESS_BRANCH` action.
		// If there is an only branch and no value, merge nibble of this branch and path of the underlying node
		// and return `USELESS_BRANCH` action.
		// Otherwise our branch isn't useless and was updated.

		var action DeleteAction
		var info Info
		var idx int

		// Decide if we need to remove value of this node or go deeper.
		if path.Empty() && len(node.value) == 0 {
			return daUnknown, NoInfo, fmt.Errorf("key error")
		} else if path.Empty() && len(node.value) != 0 {
			node.value = []byte{}
			action = daDeleted
		} else {
			// Store idx of the branch we're working with.
			idx = path.At(0)

			if len(node.Branches[idx]) == 0 {
				return daUnknown, NoInfo, fmt.Errorf("key error")
			}

			action, info, err = m.delete(node.Branches[idx], path.Consume(1))
			if err != nil {
				return daUnknown, NoInfo, err
			}
			node.Branches[idx] = []byte{}
		}

		switch action {
		case daDeleted:
			validBranches := 0
			for _, ref := range node.Branches {
				if ref.IsValid() {
					validBranches++
				}
			}

			if validBranches == 0 && len(node.Data()) == 0 {
				// Branch node is empty, just delete it.
				return daDeleted, NoInfo, nil
			} else if validBranches == 0 && len(node.Data()) != 0 {
				// No branches, just value.
				path = newPath([]byte{}, 0)
				reference, err := m.storeNode(newLeafNode(path, node.Data()))
				if err != nil {
					return daUnknown, NoInfo, err
				}
				return daUselessBranch, Info{*path, reference}, nil
			} else if validBranches == 1 && len(node.Data()) == 0 {
				// No value and one branch
				return m.buildNewNodeFromLastBranch(&node.Branches)
			} else {
				// Branch has value and 1+ branches or no value and 2+ branches.
				// It isn't useless, so action is `UPDATED`.
				reference, err := m.storeNode(node)
				if err != nil {
					return daUnknown, NoInfo, err
				}

				return daUpdated, Info{Path{}, reference}, nil
			}

		case daUpdated:
			// Just update reference.
			node.Branches[idx] = info.ref
			reference, err := m.storeNode(node)
			if err != nil {
				return daUnknown, NoInfo, err
			}

			return daUpdated, Info{Path{}, reference}, nil

		case daUselessBranch:
			// Just update reference.
			node.Branches[idx] = info.ref
			reference, err := m.storeNode(node)
			if err != nil {
				return daUnknown, NoInfo, err
			}

			return daUpdated, Info{Path{}, reference}, nil
		default:
			return daUpdated, NoInfo, fmt.Errorf("invalid action")
		}
	}
	panic("Unreachable")
}

func (m *MerklePatriciaTrie) buildNewNodeFromLastBranch(branches *[BranchesNum]Reference) (DeleteAction, Info, error) {
	// Combines nibble of the only branch left with underlying node and creates new node.

	// Find the index of the only stored branch.
	idx := 0
	for i, ref := range branches {
		if ref.IsValid() {
			idx = i
			break
		}
	}

	// Path in leaf will contain one nibble (at this step).
	prefixNibble := newPath([]byte{byte(idx)}, 1)
	child, err := m.getNode(branches[idx])
	if err != nil {
		return daUnknown, NoInfo, err
	}

	var path Path
	var node Node
	// Build new node.
	// If next node is leaf or extension, merge it.
	// If next node is branch, create an extension node with one nibble in path.
	switch child := child.(type) {
	case *LeafNode:
		path = *prefixNibble.Combine(child.Path())
		node = newLeafNode(&path, child.Data())
	case *ExtensionNode:
		path = *prefixNibble.Combine(child.Path())
		node = newExtensionNode(&path, child.NextRef)
	case *BranchNode:
		path = *prefixNibble
		node = newExtensionNode(&path, branches[idx])
	}
	reference, err := m.storeNode(node)
	if err != nil {
		return daUnknown, NoInfo, err
	}

	return daUselessBranch, Info{path, reference}, nil
}

func (m *MerklePatriciaTrie) get(nodeRef Reference, path Path) (Node, error) {
	node, err := m.getNode(nodeRef)
	if err != nil {
		return nil, err
	}

	// If path is empty, our travel is over. Main `get` method will check if this node has a value.
	if path.Size() == 0 {
		return node, nil
	}
	switch node := node.(type) {
	case *LeafNode:
		// If we've found a leaf, it's either the leaf we're looking for or wrong leaf.
		if node.Path().Equal(&path) {
			return node, nil
		}

	case *ExtensionNode:
		// If we've found an extension, we need to go deeper.
		if path.StartsWith(node.Path()) {
			restPath := path.Consume(node.Path().Size())
			return m.get(node.NextRef, *restPath)
		}

	case *BranchNode:
		// If we've found a branch node, go to the appropriate branch.
		branch := node.Branches[path.At(0)]
		if len(branch) > 0 {
			return m.get(branch, *path.Consume(1))
		}
	}

	return nil, fmt.Errorf("not found")
}

func (m *MerklePatriciaTrie) set(nodeRef Reference, path Path, value []byte) (Reference, error) {
	if !nodeRef.IsValid() {
		return m.storeNode(newLeafNode(&path, value))
	}

	node, err := m.getNode(nodeRef)
	if err != nil {
		return nil, err
	}

	switch node := node.(type) {
	case *LeafNode:
		// If we're updating the leaf there are 2 possible ways:
		// 1. path is equals to the rest of the key. Then we should just update value of this leaf.
		// 2. path differs. Then we should split this node into several nodes.

		if node.Path().Equal(&path) {
			// Path is the same. Just change the value.
			if err := node.SetData(value); err != nil {
				return nil, err
			}
			return m.storeNode(node)
		}

		// If we are here, we have to split the node.

		// Find the common part of the key and leaf's path.
		commonPrefix := path.CommonPrefix(node.Path())

		// Cut off the common part.
		path.Consume(commonPrefix.Size())
		node.Path().Consume(commonPrefix.Size())

		// Create branch node to split paths.
		branchReference, err := m.createBranchNode(&path, value, node.Path(), node.Data())
		if err != nil {
			return nil, err
		}

		// If common part isn't empty, we have to create an extension node before branch node.
		// Otherwise, we need just branch node.
		if commonPrefix.Size() != 0 {
			return m.storeNode(newExtensionNode(commonPrefix, branchReference))
		}
		return branchReference, nil

	case *ExtensionNode:
		// If we're updating an extenstion there are 2 possible ways:
		// 1. Key starts with the extension node's path. Then we just go ahead and all the work will be done there.
		// 2. Key doesn't start with extension node's path. Then we have to split extension node.

		if path.StartsWith(node.Path()) {
			// Just go ahead.
			newReference, err := m.set(node.NextRef, *path.Consume(node.Path().Size()), value)
			if err != nil {
				return nil, err
			}
			return m.storeNode(newExtensionNode(node.Path(), newReference))
		}

		// Split extension node.

		// Find the common part of the key and extension's path.
		commonPrefix := path.CommonPrefix(node.Path())

		// Cut off the common part.
		path.Consume(commonPrefix.Size())
		node.Path().Consume(commonPrefix.Size())

		// Create an empty branch node. It may have or have not the value depending on the length
		// of the rest of the key.
		branches := [BranchesNum]Reference{}
		branchValue := []byte{}
		if path.Size() == 0 {
			branchValue = value
		}

		// If needed, create leaf branch for the value we're inserting.
		m.createBranchLeaf(&path, value, &branches)
		// If needed, create an extension node for the rest of the extension's path.
		m.createBranchExtension(node.Path(), node.NextRef, &branches)

		branchReference, err := m.storeNode(newBranchNode(&branches, branchValue))
		if err != nil {
			return nil, err
		}

		// If common part isn't empty, we have to create an extension node before branch node.
		// Otherwise, we need just branch node.
		if commonPrefix.Size() != 0 {
			return m.storeNode(newExtensionNode(commonPrefix, branchReference))
		}
		return branchReference, nil

	case *BranchNode:
		// For branch node things are easy.
		// 1. If key is empty, just store value in this node.
		// 2. If key isn't empty, just call `_update` with appropiate branch reference.

		if path.Size() == 0 {
			return m.storeNode(newBranchNode(&node.Branches, value))
		}

		idx := path.At(0)
		newReference, err := m.set(node.Branches[idx], *path.Consume(1), value)
		if err != nil {
			return nil, err
		}

		node.Branches[idx] = newReference

		return m.storeNode(node)
	}

	panic("Unexpected Node kind")
}

// Creates a branch node with up to two leaves and maybe value. Returns a reference to created node.
func (m *MerklePatriciaTrie) createBranchNode(path_a *Path, value_a []byte, path_b *Path, value_b []byte) (Reference, error) {
	if path_a.Size() == 0 && path_b.Size() == 0 {
		return nil, fmt.Errorf("incorrect paths")
	}

	branches := [BranchesNum]Reference{}
	var branchValue []byte = nil
	if path_a.Size() == 0 {
		branchValue = value_a
	} else if path_b.Size() == 0 {
		branchValue = value_b
	}
	m.createBranchLeaf(path_a, value_a, &branches)
	m.createBranchLeaf(path_b, value_b, &branches)

	return m.storeNode(newBranchNode(&branches, branchValue))
}

// If path isn't empty, creates leaf node and stores reference in appropriate branch.
func (m *MerklePatriciaTrie) createBranchLeaf(path *Path, value []byte, branches *[BranchesNum]Reference) {
	if path.Size() > 0 {
		idx := path.At(0)
		leaf, err := m.storeNode(newLeafNode(path.Consume(1), value))
		if err != nil {
			panic(err)
		}
		branches[idx] = leaf
	}
}

// If needed, creates an extension node and stores reference in appropriate branch.
// Otherwise, just stores provided reference.
func (m *MerklePatriciaTrie) createBranchExtension(path *Path, nextRef Reference, branches *[BranchesNum]Reference) {
	if path.Size() == 0 {
		panic("Path for extension node should contain at least one nibble")
	}
	if path.Size() == 1 {
		branches[path.At(0)] = nextRef
	} else {
		idx := path.At(0)
		reference, err := m.storeNode(newExtensionNode(path.Consume(1), nextRef))
		if err != nil {
			panic("Store node failed")
		}
		branches[idx] = reference
	}
}

func (m *MerklePatriciaTrie) storeNode(node Node) (Reference, error) {
	data, err := node.Encode()
	if err != nil {
		return nil, err
	}

	if len(data) < 32 {
		return data, nil
	}

	key := poseidon.Sum(data)
	if len(key) != 32 {
		key = common.BytesToHash(poseidon.Sum(data)).Bytes()
	}
	if err := m.db.Set(TableName, key, data); err != nil {
		return nil, err
	}
	return key, nil
}

func (m *MerklePatriciaTrie) getNode(ref Reference) (Node, error) {
	if len(ref) < 32 {
		return DecodeNode(ref)
	}
	data, err := m.db.Get(TableName, ref)
	if err != nil {
		return nil, err
	}
	return DecodeNode(*data)
}
