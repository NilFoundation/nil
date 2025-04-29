SSZ_TARGETS = \
    nil/internal/types/signature_encoding.go \
    nil/internal/types/account_encoding.go \
    nil/internal/types/block_encoding.go \
    nil/internal/types/collator_encoding.go \
    nil/internal/types/log_encoding.go \
    nil/internal/types/transaction_encoding.go \
    nil/internal/types/receipt_encoding.go \
    nil/internal/types/version_info_encoding.go

SSZGEN := cd nil/internal/types && go run github.com/NilFoundation/fastssz/sszgen

.PHONY: ssz_types
ssz_types: $(SSZ_TARGETS) nil/internal/types/error_string.go

nil/internal/types/error_string.go: nil/internal/types/exec_errors.go
	cd nil/internal/types && stringer -type=ErrorCode -trimprefix=Error

nil/internal/types/signature_encoding.go: nil/internal/types/signature.go nil/common/length.go
	$(SSZGEN) --path signature.go -include ../../common/hexutil/bytes.go --objs BlsAggregateSignature

nil/internal/types/log_encoding.go: nil/internal/types/log.go nil/common/length.go nil/internal/types/address.go nil/common/hash.go nil/internal/types/block.go
	$(SSZGEN) --path log.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,../../common/hash.go,block.go,uint256.go --objs Log,DebugLog

nil/internal/types/receipt_encoding.go: nil/internal/types/receipt.go nil/common/length.go nil/internal/types/address.go nil/internal/types/block.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go nil/internal/types/error_string.go
	$(SSZGEN) --path receipt.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,gas.go,value.go,block.go,bloom.go,log.go,transaction.go,exec_errors.go,../../common/hash.go,uint256.go --objs Receipt

nil/internal/types/transaction_encoding.go: nil/internal/types/transaction.go nil/common/length.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go nil/internal/types/signature.go nil/internal/types/account.go
	$(SSZGEN) --path transaction.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,gas.go,value.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go,signature.go,account.go,bitflags.go --objs Transaction,ExternalTransaction,InternalTransactionPayload,TransactionDigest,TransactionFlags,AsyncContext,AsyncResponsePayload

nil/internal/types/block_encoding.go: nil/internal/types/block.go nil/common/length.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/internal/types/transaction.go nil/common/hash.go nil/internal/types/signature.go nil/internal/types/signature_encoding.go
	$(SSZGEN) --path block.go -include ../../common/hexutil/bytes.go,../../common/length.go,signature.go,address.go,code.go,shard.go,bloom.go,log.go,value.go,transaction.go,gas.go,../../common/hash.go --objs BlockData,Block,TxCount

nil/internal/types/collator_encoding.go: nil/internal/types/collator.go nil/internal/types/shard.go nil/internal/types/block.go nil/internal/types/transaction.go
	$(SSZGEN) --path collator.go -include shard.go,block.go,transaction.go --objs Neighbor,CollatorState

nil/internal/types/account_encoding.go: nil/internal/types/account.go nil/common/length.go nil/internal/types/transaction.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go
	$(SSZGEN) --path account.go -include ../../common/length.go,transaction.go,address.go,value.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go --objs SmartContract,TokenBalance

nil/internal/types/version_info_encoding.go: nil/internal/types/version_info.go nil/common/hash.go nil/common/length.go
	$(SSZGEN) --path version_info.go -include ../../common/hash.go,../../common/length.go --objs VersionInfo
