package mpt

import (
	"fmt"

	"github.com/NilFoundation/nil/core/ssz"
)

type SszNodeKind = uint64

const (
	SszLeafNode SszNodeKind = iota
	SszExtensionNode
	SszBranchNode
)

const BranchesNum = 16

type Reference ssz.Vector

func (r *Reference) IsValid() bool {
	return len(*r) != 0
}

type Node interface {
	Encode() ([]byte, error)
	Path() *Path
	SetData(data []byte) error
	Data() []byte
}

type NodeBase struct {
	path Path
}

type LeafNode struct {
	NodeBase
	data []byte
}

type ExtensionNode struct {
	NodeBase
	NextRef Reference
}

type BranchNode struct {
	Branches [BranchesNum]Reference
	value    []byte
}

func newLeafNode(path *Path, data []byte) *LeafNode {
	node := &LeafNode{NodeBase{*path}, data}
	node.path.IsLeaf = true
	return node
}

func newExtensionNode(path *Path, next Reference) *ExtensionNode {
	return &ExtensionNode{NodeBase{*path}, next}
}

func newBranchNode(refs *[BranchesNum]Reference, value []byte) *BranchNode {
	return &BranchNode{*refs, value}
}

func (n *NodeBase) Path() *Path {
	return &n.path
}

func (n *BranchNode) Path() *Path {
	return nil
}

func (n *NodeBase) Data() []byte {
	return nil
}

func (n *LeafNode) Data() []byte {
	return n.data
}

func (n *BranchNode) Data() []byte {
	return n.value
}

func (n *LeafNode) SetData(data []byte) error {
	n.data = make([]byte, len(data))
	copy(n.data, data)
	return nil
}

func (n *ExtensionNode) SetData(data []byte) error {
	panic("SetData is illegal for ExtensionNode")
}

func (n *BranchNode) SetData(data []byte) error {
	panic("SetData is illegal for BranchNode")
}

func (n *LeafNode) Encode() ([]byte, error) {
	buf := make([]byte, 0)
	return ssz.MarshalSSZ(buf, SszLeafNode, ssz.SizedObjectSSZ(&n.path), (*ssz.Vector)(&n.data))
}

func (n *ExtensionNode) Encode() ([]byte, error) {
	buf := make([]byte, 0)
	return ssz.MarshalSSZ(buf, SszExtensionNode, ssz.SizedObjectSSZ(&n.path), (*ssz.Vector)(&n.NextRef))
}

func (n *BranchNode) Encode() ([]byte, error) {
	buf := make([]byte, 0)
	return ssz.MarshalSSZ(buf, SszBranchNode,
		(*ssz.Vector)(&n.Branches[0]), (*ssz.Vector)(&n.Branches[1]),
		(*ssz.Vector)(&n.Branches[2]), (*ssz.Vector)(&n.Branches[3]),
		(*ssz.Vector)(&n.Branches[4]), (*ssz.Vector)(&n.Branches[5]),
		(*ssz.Vector)(&n.Branches[6]), (*ssz.Vector)(&n.Branches[7]),
		(*ssz.Vector)(&n.Branches[8]), (*ssz.Vector)(&n.Branches[9]),
		(*ssz.Vector)(&n.Branches[10]), (*ssz.Vector)(&n.Branches[11]),
		(*ssz.Vector)(&n.Branches[12]), (*ssz.Vector)(&n.Branches[13]),
		(*ssz.Vector)(&n.Branches[14]), (*ssz.Vector)(&n.Branches[15]),
		(*ssz.Vector)(&n.value))
}

func DecodeNode(data []byte) (Node, error) {
	var nodeKind SszNodeKind
	if err := ssz.UnmarshalSSZ(data, 0, &nodeKind); err != nil {
		panic("SSZ unmarshal failed")
	}

	switch nodeKind {
	case SszLeafNode:
		node := LeafNode{}
		if err := ssz.UnmarshalSSZ(data, 0, &nodeKind, &node.path, (*ssz.Vector)(&node.data)); err != nil {
			panic("SSZ unmarshal failed")
		}
		return &node, nil
	case SszExtensionNode:
		node := ExtensionNode{}
		if err := ssz.UnmarshalSSZ(data, 0, &nodeKind, &node.path, (*ssz.Vector)(&node.NextRef)); err != nil {
			panic("SSZ unmarshal failed")
		}
		return &node, nil
	case SszBranchNode:
		node := BranchNode{}
		if err := ssz.UnmarshalSSZ(data, 0, &nodeKind,
			(*ssz.Vector)(&node.Branches[0]), (*ssz.Vector)(&node.Branches[1]),
			(*ssz.Vector)(&node.Branches[2]), (*ssz.Vector)(&node.Branches[3]),
			(*ssz.Vector)(&node.Branches[4]), (*ssz.Vector)(&node.Branches[5]),
			(*ssz.Vector)(&node.Branches[6]), (*ssz.Vector)(&node.Branches[7]),
			(*ssz.Vector)(&node.Branches[8]), (*ssz.Vector)(&node.Branches[9]),
			(*ssz.Vector)(&node.Branches[10]), (*ssz.Vector)(&node.Branches[11]),
			(*ssz.Vector)(&node.Branches[12]), (*ssz.Vector)(&node.Branches[13]),
			(*ssz.Vector)(&node.Branches[14]), (*ssz.Vector)(&node.Branches[15]),
			(*ssz.Vector)(&node.value)); err != nil {
			panic("SSZ unmarshal failed")
		}
		return &node, nil
	default:
		return nil, fmt.Errorf("unknown node kind %d", nodeKind)
	}
}
