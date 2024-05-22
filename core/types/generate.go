package types

//go:generate go run github.com/ferranbt/fastssz/sszgen --path log.go -include ../../common/length.go,../../common/address.go,../../common/hash.go --objs Log
//go:generate go run github.com/ferranbt/fastssz/sszgen --path receipt.go -include bloom.go,log.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs Receipt
//go:generate go run github.com/ferranbt/fastssz/sszgen --path message.go -include uint256.go,code.go,shard.go,bloom.go,log.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs Message
//go:generate go run github.com/ferranbt/fastssz/sszgen --path block.go -include uint256.go,code.go,shard.go,bloom.go,log.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs Block
//go:generate go run github.com/ferranbt/fastssz/sszgen --path account.go -include uint256.go,code.go,shard.go,bloom.go,log.go,../../common/length.go,../../common/address.go,../../common/hash.go --objs SmartContract
//go:generate go run github.com/ferranbt/fastssz/sszgen --path version_info.go -include ../../common/hash.go,../../common/length.go --objs VersionInfo
