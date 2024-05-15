package types

//go:generate go run github.com/ferranbt/fastssz/sszgen --path log.go -include ../../common/length.go,../../common/address.go,../../common/hash.go --objs Log
//go:generate go run github.com/ferranbt/fastssz/sszgen --path receipt.go -include bloom.go,log.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs Receipt
