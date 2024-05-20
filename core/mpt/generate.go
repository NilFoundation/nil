package mpt

//go:generate go run github.com/ferranbt/fastssz/sszgen --path node.go -include path.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs LeafNode,BranchNode,ExtensionNode
