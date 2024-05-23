package db

//go:generate go run github.com/ferranbt/fastssz/sszgen --path tables.go -include ../../common/hash.go,../../common/length.go --objs BlockHashAndMessageIndex
