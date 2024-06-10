package db

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path tables.go -include ../../common/hash.go,../../common/length.go,../types/message.go --objs BlockHashAndMessageIndex
