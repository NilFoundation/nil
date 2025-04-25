SSZ_TARGETS = \
    nil/internal/types/signature_encoding.go \
    nil/internal/types/account_encoding.go \
    nil/internal/types/block_encoding.go \
    nil/internal/types/collator_encoding.go \
    nil/internal/types/log_encoding.go \
    nil/internal/types/transaction_encoding.go \
    nil/internal/types/receipt_encoding.go \
    nil/internal/types/version_info_encoding.go

RLP_TARGETS = \
    nil/internal/types/signature_rlp_encoding.go \
    nil/internal/types/log_rlp_encoding.go \
    nil/internal/types/debug_log_rlp_encoding.go \
    nil/internal/types/receipt_rlp_encoding.go \
    nil/internal/types/transaction_rlp_encoding.go \
    nil/internal/types/external_transaction_rlp_encoding.go \
    nil/internal/types/internal_transaction_payload_rlp_encoding.go \
    nil/internal/types/transaction_digest_rlp_encoding.go \
    nil/internal/types/transaction_flags_rlp_encoding.go \
    nil/internal/types/async_context_rlp_encoding.go \
    nil/internal/types/async_response_payload_rlp_encoding.go \
    nil/internal/types/block_data_rlp_encoding.go \
    nil/internal/types/block_rlp_encoding.go \
    nil/internal/types/neighbor_rlp_encoding.go \
    nil/internal/types/collator_state_rlp_encoding.go \
    nil/internal/types/account_rlp_encoding.go \
    nil/internal/types/token_balance_rlp_encoding.go \
    nil/internal/types/version_info_rlp_encoding.go

SSZGEN := cd nil/internal/types && go run github.com/NilFoundation/fastssz/sszgen

RLPGEN := cd nil/internal/types && go run ../../cmd/rlpgen

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

.PHONY: rlp_types
rlp_types: $(RLP_TARGETS) ssz_types

nil/internal/types/signature_rlp_encoding.go: nil/internal/types/signature.go $(SSZ_TARGETS)
	$(RLPGEN) -type BlsAggregateSignature -out signature_rlp_encoding.go -decoder

nil/internal/types/log_rlp_encoding.go: nil/internal/types/log.go $(SSZ_TARGETS)
	$(RLPGEN) -type Log -out log_rlp_encoding.go -decoder

nil/internal/types/debug_log_rlp_encoding.go: nil/internal/types/log.go $(SSZ_TARGETS)
	$(RLPGEN) -type DebugLog -out debug_log_rlp_encoding.go -decoder

nil/internal/types/receipt_rlp_encoding.go: nil/internal/types/receipt.go $(SSZ_TARGETS)
	$(RLPGEN) -type Receipt -out receipt_rlp_encoding.go -decoder

nil/internal/types/transaction_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type Transaction -out transaction_rlp_encoding.go -decoder

nil/internal/types/external_transaction_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type ExternalTransaction -out external_transaction_rlp_encoding.go -decoder

nil/internal/types/internal_transaction_payload_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type InternalTransactionPayload -out internal_transaction_payload_rlp_encoding.go -decoder

nil/internal/types/transaction_digest_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type TransactionDigest -out transaction_digest_rlp_encoding.go -decoder

nil/internal/types/transaction_flags_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type TransactionFlags -out transaction_flags_rlp_encoding.go -decoder

nil/internal/types/async_context_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type AsyncContext -out async_context_rlp_encoding.go -decoder

nil/internal/types/async_response_payload_rlp_encoding.go: nil/internal/types/transaction.go $(SSZ_TARGETS)
	$(RLPGEN) -type AsyncResponsePayload -out async_response_payload_rlp_encoding.go -decoder

nil/internal/types/block_data_rlp_encoding.go: nil/internal/types/block.go $(SSZ_TARGETS)
	$(RLPGEN) -type BlockData -out block_data_rlp_encoding.go -decoder

nil/internal/types/block_rlp_encoding.go: nil/internal/types/block.go $(SSZ_TARGETS)
	$(RLPGEN) -type Block -out block_rlp_encoding.go -decoder

nil/internal/types/neighbor_rlp_encoding.go: nil/internal/types/collator.go $(SSZ_TARGETS)
	$(RLPGEN) -type Neighbor -out neighbor_rlp_encoding.go -decoder

nil/internal/types/collator_state_rlp_encoding.go: nil/internal/types/collator.go $(SSZ_TARGETS)
	$(RLPGEN) -type CollatorState -out collator_state_rlp_encoding.go -decoder

nil/internal/types/account_rlp_encoding.go: nil/internal/types/account.go $(SSZ_TARGETS)
	$(RLPGEN) -type SmartContract -out account_rlp_encoding.go -decoder

nil/internal/types/token_balance_rlp_encoding.go: nil/internal/types/account.go $(SSZ_TARGETS)
	$(RLPGEN) -type TokenBalance -out token_balance_rlp_encoding.go -decoder

nil/internal/types/version_info_rlp_encoding.go: nil/internal/types/version_info.go $(SSZ_TARGETS)
	$(RLPGEN) -type VersionInfo -out version_info_rlp_encoding.go -decoder
