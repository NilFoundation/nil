package types

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path log.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,../../common/hash.go,block.go,uint256.go --objs Log,DebugLog
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path receipt.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,gas.go,value.go,block.go,bloom.go,log.go,message.go,exec_errors.go,../../common/hash.go,uint256.go --objs Receipt
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path message.go -include ../../common/length.go,address.go,gas.go,value.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go,signature.go,account.go,bitflags.go --objs Message,ExternalMessage,InternalMessagePayload,messageDigest,MessageFlags,EvmState,AsyncContext,AsyncResponsePayload
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path block.go -include ../../common/length.go,address.go,code.go,shard.go,bloom.go,log.go,value.go,message.go,../../common/hash.go --objs Block
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path collator.go -include shard.go,block.go,message.go --objs Neighbor,CollatorState
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path account.go -include ../../common/length.go,message.go,address.go,value.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go --objs SmartContract,CurrencyBalance
//go:generate go run github.com/NilFoundation/fastssz/sszgen --path version_info.go -include ../../common/hash.go,../../common/length.go --objs VersionInfo
//go:generate stringer -type=ErrorCode -trimprefix=Error
