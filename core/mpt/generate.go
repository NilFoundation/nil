package mpt

//go:generate go run github.com/ferranbt/fastssz/sszgen --path node.go -include path.go --objs LeafNode,BranchNode,ExtensionNode
