package mpt

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/ethereum/go-ethereum/rlp"
)

type nodeKind = uint8

const (
	leafNodeKind nodeKind = iota
	extensionNodeKind
	branchNodeKind
)

const BranchesNum = 16

type Reference []byte

func (r *Reference) IsValid() bool {
	return len(*r) != 0
}

type Node interface {
	Encode() ([]byte, error)
	// partial path from parent node to the current
	Path() *Path
	SetData(data []byte) error
	Data() []byte
}

func calcNodeKey(data []byte) []byte {
	return common.KeccakHash(data).Bytes()
}

type NodeBase struct {
	NodePath Path
}

type LeafNode struct {
	NodeBase
	LeafData []byte
}

type ExtensionNode struct {
	NodeBase
	NextRef Reference
}

type BranchNode struct {
	Branches [BranchesNum]Reference
	Value    []byte
}

func newLeafNode(path *Path, data []byte) *LeafNode {
	node := &LeafNode{NodeBase{*path}, data}
	return node
}

func newExtensionNode(path *Path, next Reference) *ExtensionNode {
	return &ExtensionNode{NodeBase{*path}, next}
}

func newBranchNode(refs *[BranchesNum]Reference, value []byte) *BranchNode {
	return &BranchNode{*refs, value}
}

func (n *ExtensionNode) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, n)
}

func (n *BranchNode) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, n)
}

func (n *LeafNode) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, n)
}

func (n ExtensionNode) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&n)
}

func (n BranchNode) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&n)
}

func (n LeafNode) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&n)
}

func (n *NodeBase) Path() *Path {
	return &n.NodePath
}

func (n *BranchNode) Path() *Path {
	return nil
}

func (n *NodeBase) Data() []byte {
	return nil
}

func (n *LeafNode) Data() []byte {
	return n.LeafData
}

func (n *BranchNode) Data() []byte {
	return n.Value
}

func (n *LeafNode) SetData(data []byte) error {
	n.LeafData = make([]byte, len(data))
	copy(n.LeafData, data)
	return nil
}

func (n *ExtensionNode) SetData([]byte) error {
	panic("SetData is illegal for ExtensionNode")
}

func (n *BranchNode) SetData([]byte) error {
	panic("SetData is illegal for BranchNode")
}

func encode[
	S any,
	T interface {
		~*S
		serialization.NilMarshaler
	},
](n T, kind nodeKind) ([]byte, error) {
	data, err := n.MarshalNil()
	if err != nil {
		return nil, err
	}
	buf, err := rlp.EncodeToBytes(kind)
	check.PanicIfErr(err)
	return append(buf, data...), nil
}

func (n *LeafNode) Encode() ([]byte, error) {
	return encode(n, leafNodeKind)
}

func (n *ExtensionNode) Encode() ([]byte, error) {
	return encode(n, extensionNodeKind)
}

func (n *BranchNode) Encode() ([]byte, error) {
	return encode(n, branchNodeKind)
}

func DecodeNode(data []byte) (Node, error) {
	var nodeKind nodeKind
	check.PanicIfErr(rlp.DecodeBytes(data[:1], &nodeKind))
	data = data[1:]

	switch nodeKind {
	case leafNodeKind:
		node := &LeafNode{}
		if err := node.UnmarshalNil(data); err != nil {
			return nil, err
		}
		return node, nil
	case extensionNodeKind:
		node := &ExtensionNode{}
		if err := node.UnmarshalNil(data); err != nil {
			return nil, err
		}
		return node, nil
	case branchNodeKind:
		node := &BranchNode{}
		if err := node.UnmarshalNil(data); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return nil, fmt.Errorf("unknown node kind %d", nodeKind)
	}
}
