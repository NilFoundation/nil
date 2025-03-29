package mpt

import (
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type SszNodeKind = uint8

const (
	SszLeafNode SszNodeKind = iota
	SszExtensionNode
	SszBranchNode
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
	key := poseidon.Sum(data)
	if len(key) != 32 {
		key = common.BytesToHash(key).Bytes()
	}
	return key
}

type NodeBase struct {
	NodePath Path
}

type LeafNode struct {
	NodeBase
	LeafData []byte `ssz-max:"100000000"`
}

type ExtensionNode struct {
	NodeBase
	NextRef Reference `ssz-max:"32"`
}

type BranchNode struct {
	Branches [BranchesNum]Reference `ssz-max:"16,32"`
	Value    []byte                 `ssz-max:"100000000"`
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
		ssz.Marshaler
	},
](n T, kind SszNodeKind) ([]byte, error) {
	buf := make([]byte, 0)
	buf = ssz.MarshalUint8(buf, kind)
	return n.MarshalSSZTo(buf)
}

func (n *LeafNode) Encode() ([]byte, error) {
	return encode(n, SszLeafNode)
}

func (n *ExtensionNode) Encode() ([]byte, error) {
	return encode(n, SszExtensionNode)
}

func (n *BranchNode) Encode() ([]byte, error) {
	return encode(n, SszBranchNode)
}

func DecodeNode(data []byte) (Node, error) {
	nodeKind := ssz.UnmarshallUint8(data)
	data = data[1:]

	switch nodeKind {
	case SszLeafNode:
		node := &LeafNode{}
		if err := node.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		return node, nil
	case SszExtensionNode:
		node := &ExtensionNode{}
		if err := node.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		return node, nil
	case SszBranchNode:
		node := &BranchNode{}
		if err := node.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return nil, fmt.Errorf("unknown node kind %d", nodeKind)
	}
}
