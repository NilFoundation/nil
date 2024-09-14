package mpt

import (
	"errors"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type deleteAction int

const (
	daUnknown deleteAction = iota
	daDeleted
	daUpdated
	daUselessBranch
)

// keys above this len are hashed before putting into tree
const maxRawKeyLen = 32

type Reader struct {
	getter Getter
	root   Reference
}

type MerklePatriciaTrie struct {
	*Reader

	setter Setter
}

func (m *Reader) SetRootHash(root common.Hash) {
	m.root = root.Bytes()
}

func (m *Reader) RootHash() common.Hash {
	if !m.root.IsValid() {
		return common.EmptyHash
	}
	return common.BytesToHash(m.root)
}

func (m *Reader) Get(key []byte) (ret []byte, err error) {
	if m.root == nil {
		return nil, db.ErrKeyNotFound
	}
	if len(key) > maxRawKeyLen {
		key = poseidon.Sum(key)
	}
	path := newPath(key, false)

	node, err := m.get(m.root, *path)
	if err != nil {
		return nil, err
	}

	return node.Data(), nil
}

func (m *MerklePatriciaTrie) Set(key []byte, value []byte) error {
	return m.SetBatch([][]byte{key}, [][]byte{value})
}

// keys order is important: if any key is presented several times only last modification will be applied
func (m *MerklePatriciaTrie) SetBatch(keys [][]byte, values [][]byte) error {
	if len(keys) != len(values) || len(keys) == 0 {
		return ErrInvalidArgSize
	}

	paths := make([]*Path, 0)
	vals := make([][]byte, 0)

	keyToIndex := make(map[string]int)
	for i := range keys {
		k := keys[i]
		if len(k) > maxRawKeyLen {
			k = poseidon.Sum(k)
		}
		if idx, ok := keyToIndex[string(k)]; ok {
			vals[idx] = values[i]
		} else {
			keyToIndex[string(k)] = len(paths)
			paths = append(paths, newPath(k, false))
			vals = append(vals, values[i])
		}
	}

	root, err := m.setBatch(m.root, paths, vals)
	if err != nil {
		return err
	}
	m.root = root

	// We always save short root node in the storage, because `RootHash()` widens root to 32 bytes.
	// So next time we want to read the root node, it will be 32-bytes width and thus will be fetched from the storage.
	if len(m.root) < 32 {
		if err := m.setter.Set(m.RootHash().Bytes(), m.root); err != nil {
			return err
		}
	}

	return nil
}

func (m *MerklePatriciaTrie) Delete(key []byte) error {
	if !m.root.IsValid() {
		return nil
	}
	if len(key) > maxRawKeyLen {
		key = poseidon.Sum(key)
	}
	path := newPath(key, false)

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
	case daUnknown:
		fallthrough
	default:
		return ErrInvalidAction
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////

// Tree name must be unique across all trees in the DB

func NewReader(getter Getter) *Reader {
	return &Reader{getter: getter}
}

func NewDbReader(tx db.RoTx, shardId types.ShardId, name db.ShardedTableName) *Reader {
	return NewReader(NewDbGetter(tx, shardId, name))
}

func NewMPT(setter Setter, reader *Reader) *MerklePatriciaTrie {
	return &MerklePatriciaTrie{reader, setter}
}

func NewDbMPT(db db.RwTx, shardId types.ShardId, name db.ShardedTableName) *MerklePatriciaTrie {
	return NewMPT(NewDbSetter(db, shardId, name), NewDbReader(db, shardId, name))
}

func GetEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](root *Reader, entityKey []byte) (*S, error) {
	entityBytes, err := root.Get(entityKey)
	if err != nil {
		return nil, err
	}

	var entity S
	return &entity, T(&entity).UnmarshalSSZ(entityBytes)
}

////////////////////////////////////////////////////////////////////////////////

type deletionInfo struct {
	path Path
	ref  Reference
}

var noInfo = deletionInfo{Path{}, nil}

func (m *MerklePatriciaTrie) delete(nodeRef Reference, path *Path) (deleteAction, deletionInfo, error) {
	node, err := m.getNode(nodeRef)
	if err != nil {
		return daUnknown, noInfo, err
	}

	switch node := node.(type) {
	case *LeafNode:
		// If it's a leaf node, then it's either node we need or incorrect key provided.
		if path.Equal(node.Path()) {
			return daDeleted, noInfo, nil
		}
		return daUnknown, noInfo, db.ErrKeyNotFound

	case *ExtensionNode:
		// Extension node can't be removed directly, it passes delete request to the next node.
		// After that, several options are possible:
		// 1. The next node was deleted. Then this node should be deleted too.
		// 2. The next node was updated. Then we should update stored reference.
		// 3. The next node was a useless branch. Then we have to update our node depending on the next node type.

		if !path.StartsWith(node.Path()) {
			return daUnknown, noInfo, db.ErrKeyNotFound
		}
		action, info, err := m.delete(node.NextRef, path.Consume(node.Path().Size()))
		if err != nil {
			return daUnknown, noInfo, err
		}

		switch action {
		case daDeleted:
			// The next node was deleted. This node should be deleted also.
			return action, noInfo, nil
		case daUpdated:
			// The next node was updated. Update this node too.
			newRef, err := m.storeNode(newExtensionNode(node.Path(), info.ref))
			if err != nil {
				return daUnknown, noInfo, err
			}
			return action, deletionInfo{Path{}, newRef}, nil

		case daUselessBranch:
			// Next node was useless branch.
			child, err := m.getNode(info.ref)
			if err != nil {
				return daUnknown, noInfo, err
			}

			var newNode Node
			switch child := child.(type) {
			case *LeafNode:
				// If the next node is the leaf, our node is unnecessary.
				// Concat our path with leaf path and return reference to the leaf.
				path = node.Path().Combine(child.Path())
				newNode = newLeafNode(path, child.Data())

			case *ExtensionNode:
				// If the next node is the extension, merge this and next node into one.
				path = node.Path().Combine(child.Path())
				newNode = newExtensionNode(path, child.NextRef)

			case *BranchNode:
				// If the next node is the branch, concatenate paths and update stored reference.
				path = node.Path().Combine(&info.path)
				newNode = newExtensionNode(path, info.ref)

			default:
				panic("Invalid node type")
			}

			newReference, err := m.storeNode(newNode)
			if err != nil {
				return daUnknown, noInfo, err
			}

			return daUpdated, deletionInfo{Path{}, newReference}, nil
		case daUnknown:
			fallthrough
		default:
			return daUnknown, noInfo, ErrInvalidAction
		}

	case *BranchNode:
		// For branch node, things are quite complicated.
		// If the rest of the key is empty and there is stored value, just clear value field.
		// Otherwise, call _delete for the appropriate branch.
		// At this step, we will have delete action and (possibly) index of the branch we're working with.
		//
		// Then, if the next node was updated or was useless branch, just update reference.
		// If `_DeleteAction` is `DELETED` then either the next node or value of this node was removed.
		// We have to check if there are at least 2 branches or 1 branch and value still persist in this node.
		// If there are no branches and no value left, delete this node completely.
		// If there is a value but no branches, create leaf node with value and empty path
		// and return `USELESS_BRANCH` action.
		// If there is an only branch and no value, merge nibble of this branch and path of the underlying node
		// and return `USELESS_BRANCH` action.
		// Otherwise, our branch isn't useless and was updated.

		var action deleteAction
		var info deletionInfo
		var idx int

		// Decide if we need to remove the value of this node or go deeper.
		switch {
		case path.Empty() && len(node.Value) == 0:
			return daUnknown, noInfo, db.ErrKeyNotFound
		case path.Empty() && len(node.Value) != 0:
			node.Value = []byte{}
			action = daDeleted
		default:
			// Store idx of the branch we're working with.
			idx = path.At(0)

			if len(node.Branches[idx]) == 0 {
				return daUnknown, noInfo, db.ErrKeyNotFound
			}

			action, info, err = m.delete(node.Branches[idx], path.Consume(1))
			if err != nil {
				return daUnknown, noInfo, err
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

			switch {
			case validBranches == 0 && len(node.Data()) == 0:
				return daDeleted, noInfo, nil
			case validBranches == 0 && len(node.Data()) != 0:
				path = newPath([]byte{}, false)
				reference, err := m.storeNode(newLeafNode(path, node.Data()))
				if err != nil {
					return daUnknown, noInfo, err
				}
				return daUselessBranch, deletionInfo{*path, reference}, nil
			case validBranches == 1 && len(node.Data()) == 0:
				return m.buildNewNodeFromLastBranch(&node.Branches)
			default:
				reference, err := m.storeNode(node)
				if err != nil {
					return daUnknown, noInfo, err
				}
				return daUpdated, deletionInfo{Path{}, reference}, nil
			}

		case daUpdated:
			// Just update reference.
			node.Branches[idx] = info.ref
			reference, err := m.storeNode(node)
			if err != nil {
				return daUnknown, noInfo, err
			}

			return daUpdated, deletionInfo{Path{}, reference}, nil

		case daUselessBranch:
			// Just update reference.
			node.Branches[idx] = info.ref
			reference, err := m.storeNode(node)
			if err != nil {
				return daUnknown, noInfo, err
			}

			return daUpdated, deletionInfo{Path{}, reference}, nil
		case daUnknown:
			fallthrough
		default:
			return daUpdated, noInfo, ErrInvalidAction
		}
	}

	panic("Unreachable")
}

func (m *MerklePatriciaTrie) buildNewNodeFromLastBranch(branches *[BranchesNum]Reference) (deleteAction, deletionInfo, error) {
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
	prefixNibble := newPath([]byte{byte(idx)}, true)
	child, err := m.getNode(branches[idx])
	if err != nil {
		return daUnknown, noInfo, err
	}

	var path Path
	var node Node
	// Build new node.
	// If the next node is leaf or extension, merge it.
	// If the next node is a branch, create an extension node with one nibble in path.
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
		return daUnknown, noInfo, err
	}

	return daUselessBranch, deletionInfo{path, reference}, nil
}

func (m *Reader) get(nodeRef Reference, path Path) (Node, error) {
	return m.descendWithCallback(nodeRef, path, nil)
}

func (m *Reader) descendWithCallback(nodeRef Reference, path Path, cb func(Node)) (Node, error) {
	node, err := m.getNode(nodeRef)
	if err != nil {
		return nil, err
	}

	if cb != nil {
		cb(node)
	}

	// If the path is empty, our travel is over. Main `get` method will check if this node has a value.
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
			return m.descendWithCallback(node.NextRef, *restPath, cb)
		}

	case *BranchNode:
		// If we've found a branch node, go to the appropriate branch.
		branch := node.Branches[path.At(0)]
		if len(branch) > 0 {
			return m.descendWithCallback(branch, *path.Consume(1), cb)
		}
	}

	return nil, db.ErrKeyNotFound
}

func (m *MerklePatriciaTrie) setBatch(nodeRef Reference, paths []*Path, values [][]byte) (Reference, error) {
	if len(paths) != len(values) || len(paths) == 0 {
		return nil, ErrInvalidArgSize
	}

	commonPrefix := CommonPrefix(paths)

	if !nodeRef.IsValid() {
		// We have only one value so store it in a LeafNode
		if len(paths) == 1 {
			return m.storeNode(newLeafNode(paths[0], values[0]))
		}

		for _, p := range paths {
			p.Consume(commonPrefix.Size())
		}

		// Create branch node to split paths
		branchReference, err := m.createBranchNodeFromBatch(paths, values, nil, nil)
		if err != nil {
			return nil, err
		}

		// If common part isn't empty, we have to create an extension node before branch node
		// Otherwise, we need just branch node
		if commonPrefix.Size() != 0 {
			return m.storeNode(newExtensionNode(commonPrefix, branchReference))
		}

		return branchReference, nil
	}

	node, err := m.getNode(nodeRef)
	if errors.Is(err, db.ErrKeyNotFound) {
		node = newLeafNode(paths[0], nil)
		err = nil
	}
	if err != nil {
		return nil, err
	}

	switch node := node.(type) {
	case *LeafNode:
		// If we're updating the leaf, there are 2 possible ways:
		// 1. We have only one key and the node path is equal to the rest of the key. Then we should just update the value of this leaf
		// 2. We have at least 2 distinct keys. In that keys we need to split the node into several branches

		// We have only one key to update and it's the same as stored in the node. So just update the node value
		if len(paths) == 1 && node.Path().Equal(paths[0]) {
			if err := node.SetData(values[0]); err != nil {
				return nil, err
			}
			return m.storeNode(node)
		}

		// Otherwise we need to split the node

		// First check whether cur node will be replaced by some key from the batch
		equalFound := false
		for _, p := range paths {
			if p.Equal(node.Path()) {
				equalFound = true
				break
			}
		}
		if !equalFound {
			paths = append(paths, node.Path())
			values = append(values, node.Data())
			commonPrefix = commonPrefix.CommonPrefix(node.Path())
		}

		// Cut off the common part
		for _, p := range paths {
			p.Consume(commonPrefix.Size())
		}

		// Create branch node to split paths
		branchReference, err := m.createBranchNodeFromBatch(paths, values, nil, nil)
		if err != nil {
			return nil, err
		}

		// If common part isn't empty, we have to create an extension node before branch node
		// Otherwise, we need just branch node
		if commonPrefix.Size() > 0 {
			return m.storeNode(newExtensionNode(commonPrefix, branchReference))
		}
		return branchReference, nil

	case *ExtensionNode:
		// If we're updating an extension, there are 2 possible ways:
		// 1. All the keys start with the extension node's path. Then we just go ahead and all the work will be done there
		// 2. At least one key doesn't start with extension node's path. Then we have to split extension node

		if commonPrefix.StartsWith(node.Path()) {
			// Cut off the common part
			for _, p := range paths {
				p.Consume(node.Path().Size())
			}

			// And go ahead
			nextReference, err := m.setBatch(node.NextRef, paths, values)
			if err != nil {
				return nil, err
			}

			return m.storeNode(newExtensionNode(node.Path(), nextReference))
		}

		// Split extension node.

		// Find the common part of the key and extension's path.
		commonPrefix = commonPrefix.CommonPrefix(node.Path())

		// Cut off the common part.
		node.Path().Consume(commonPrefix.Size())
		for _, p := range paths {
			p.Consume(commonPrefix.Size())
		}

		branchReference, err := m.createBranchNodeFromBatch(paths, values, node.Path(), node.NextRef)
		if err != nil {
			return nil, err
		}

		// If common part isn't empty, we have to create an extension node before branch node.
		// Otherwise, we need just branch node.
		if commonPrefix.Size() > 0 {
			return m.storeNode(newExtensionNode(commonPrefix, branchReference))
		}
		return branchReference, nil

	case *BranchNode:
		// For a branch node we just need to forward all keys into appropriate branches
		// Also if any path is empty we need to update value of this particular branch node
		var pathGroups [BranchesNum][]*Path
		var valueGroups [BranchesNum][][]byte

		for i := range paths {
			if paths[i].Size() == 0 {
				node.Value = values[i]
			} else {
				idx := paths[i].At(0)
				pathGroups[idx] = append(pathGroups[idx], paths[i].Consume(1))
				valueGroups[idx] = append(valueGroups[idx], values[i])
			}
		}

		for i := range BranchesNum {
			if len(pathGroups[i]) == 0 {
				continue
			}

			newRef, err := m.setBatch(node.Branches[i], pathGroups[i], valueGroups[i])
			if err != nil {
				return nil, err
			}
			node.Branches[i] = newRef
		}

		return m.storeNode(node)
	}

	panic("Unexpected node type")
}

// Creates a branch node from list of keys / values. Returns a reference to created node.
// If extension path is non-empty, first store extRef in appropriate branch
func (m *MerklePatriciaTrie) createBranchNodeFromBatch(paths []*Path, values [][]byte, extPath *Path, extRef Reference) (Reference, error) {
	if len(paths) == 0 || len(paths) != len(values) {
		return nil, ErrInvalidArgSize
	}
	branches := [BranchesNum]Reference{}
	var branchValue []byte

	var pathGroups [BranchesNum][]*Path
	var valueGroups [BranchesNum][][]byte

	for i := range paths {
		if paths[i].Size() == 0 {
			// actually we don't need to check for key uniqueness here
			// because it was checked earlier
			branchValue = values[i]
		} else {
			idx := paths[i].At(0)
			pathGroups[idx] = append(pathGroups[idx], paths[i].Consume(1))
			valueGroups[idx] = append(valueGroups[idx], values[i])
		}
	}

	if extPath != nil {
		m.createBranchExtension(extPath, extRef, &branches)
	}

	for i := range BranchesNum {
		if len(pathGroups[i]) == 0 {
			continue
		}

		newRef, err := m.setBatch(branches[i], pathGroups[i], valueGroups[i])
		if err != nil {
			return nil, err
		}
		branches[i] = newRef
	}

	return m.storeNode(newBranchNode(&branches, branchValue))
}

// If needed, creates an extension node and stores reference in appropriate branch.
// Otherwise, just stores provided reference.
func (m *MerklePatriciaTrie) createBranchExtension(path *Path, nextRef Reference, branches *[BranchesNum]Reference) {
	check.PanicIfNotf(path.Size() > 0, "Path for extension node should contain at least one nibble")

	if path.Size() == 1 {
		branches[path.At(0)] = nextRef
	} else {
		idx := path.At(0)
		reference, err := m.storeNode(newExtensionNode(path.Consume(1), nextRef))
		check.PanicIfErr(err)
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
		key = common.BytesToHash(key).Bytes()
	}
	if err := m.setter.Set(key, data); err != nil {
		return nil, err
	}
	return key, nil
}

func (m *Reader) getNode(ref Reference) (Node, error) {
	if len(ref) < 32 {
		return DecodeNode(ref)
	}
	data, err := m.getter.Get(ref)
	if err != nil {
		return nil, err
	}
	return DecodeNode(data)
}
