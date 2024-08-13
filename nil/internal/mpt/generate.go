package mpt

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path node.go -include path.go --objs LeafNode,BranchNode,ExtensionNode
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path path.go --objs Path
