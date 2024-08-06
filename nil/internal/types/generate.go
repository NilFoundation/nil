package types

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path log.go -include ../../common/length.go,address.go,../../common/hash.go,block.go --objs Log
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path receipt.go -include ../../common/length.go,address.go,gas.go,value.go,block.go,bloom.go,log.go,message.go,../../common/hash.go --objs Receipt
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path message.go -include ../../common/length.go,address.go,gas.go,value.go,uint256.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go,signature.go,account.go,bitflags.go --objs Message,ExternalMessage,InternalMessagePayload,messageDigest,MessageFlags
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path block.go -include ../../common/length.go,address.go,uint256.go,code.go,shard.go,bloom.go,log.go,value.go,message.go,../../common/hash.go --objs Block
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path collator.go -include shard.go,block.go,message.go --objs Neighbor,CollatorState
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path account.go -include ../../common/length.go,message.go,address.go,value.go,uint256.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go --objs SmartContract,CurrencyBalance
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path version_info.go -include ../../common/hash.go,../../common/length.go --objs VersionInfo
//go:generate stringer -type=MessageStatus -trimprefix=MessageStatus
