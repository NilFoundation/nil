ifndef RLP_TARGETS_INCLUDED
RLP_TARGETS_INCLUDED := true

RLP_TARGETS = \
    nil/internal/types/signature_rlp_encoding.go \
    nil/internal/types/log_rlp_encoding.go \
    nil/internal/types/debug_log_rlp_encoding.go \
    nil/internal/types/receipt_rlp_encoding.go \
    nil/internal/types/transaction_rlp_encoding.go \
    nil/internal/types/external_transaction_rlp_encoding.go \
    nil/internal/types/internal_transaction_payload_rlp_encoding.go \
    nil/internal/types/tx_count_rlp_encoding.go \
    nil/internal/types/transaction_digest_rlp_encoding.go \
    nil/internal/types/async_context_rlp_encoding.go \
    nil/internal/types/async_response_payload_rlp_encoding.go \
    nil/internal/types/block_data_rlp_encoding.go \
    nil/internal/types/block_rlp_encoding.go \
    nil/internal/types/neighbor_rlp_encoding.go \
    nil/internal/types/collator_state_rlp_encoding.go \
    nil/internal/types/account_rlp_encoding.go \
    nil/internal/types/token_balance_rlp_encoding.go \
    nil/internal/types/version_info_rlp_encoding.go

TYPES_RLPGEN := cd nil/internal/types && $(RLPGEN_BIN)

.PHONY: error_stringer
error_stringer: nil/internal/types/error_string.go

nil/internal/types/error_string.go: nil/internal/types/exec_errors.go
	cd nil/internal/types && stringer -type=ErrorCode -trimprefix=Error

$(RLP_TARGETS): | $(RLPGEN_BIN) error_stringer

.PHONY: rlp_types
rlp_types: $(RLP_TARGETS) error_stringer

nil/internal/types/signature_rlp_encoding.go: nil/internal/types/signature.go
	$(TYPES_RLPGEN) -type BlsAggregateSignature -out signature_rlp_encoding.go -decoder

nil/internal/types/log_rlp_encoding.go: nil/internal/types/log.go
	$(TYPES_RLPGEN) -type Log -out log_rlp_encoding.go -decoder

nil/internal/types/debug_log_rlp_encoding.go: nil/internal/types/log.go
	$(TYPES_RLPGEN) -type DebugLog -out debug_log_rlp_encoding.go -decoder

nil/internal/types/receipt_rlp_encoding.go: nil/internal/types/receipt.go
	$(TYPES_RLPGEN) -type Receipt -out receipt_rlp_encoding.go -decoder

nil/internal/types/transaction_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type Transaction -out transaction_rlp_encoding.go -decoder

nil/internal/types/external_transaction_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type ExternalTransaction -out external_transaction_rlp_encoding.go -decoder

nil/internal/types/internal_transaction_payload_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type InternalTransactionPayload -out internal_transaction_payload_rlp_encoding.go -decoder

nil/internal/types/transaction_digest_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type TransactionDigest -out transaction_digest_rlp_encoding.go -decoder

nil/internal/types/tx_count_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type TxCount -out tx_count_rlp_encoding.go -decoder

nil/internal/types/async_context_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type AsyncContext -out async_context_rlp_encoding.go -decoder

nil/internal/types/async_response_payload_rlp_encoding.go: nil/internal/types/transaction.go
	$(TYPES_RLPGEN) -type AsyncResponsePayload -out async_response_payload_rlp_encoding.go -decoder

nil/internal/types/block_data_rlp_encoding.go: nil/internal/types/block.go
	$(TYPES_RLPGEN) -type BlockData -out block_data_rlp_encoding.go -decoder

nil/internal/types/block_rlp_encoding.go: nil/internal/types/block.go
	$(TYPES_RLPGEN) -type Block -out block_rlp_encoding.go -decoder

nil/internal/types/neighbor_rlp_encoding.go: nil/internal/types/collator.go
	$(TYPES_RLPGEN) -type Neighbor -out neighbor_rlp_encoding.go -decoder

nil/internal/types/collator_state_rlp_encoding.go: nil/internal/types/collator.go
	$(TYPES_RLPGEN) -type CollatorState -out collator_state_rlp_encoding.go -decoder

nil/internal/types/account_rlp_encoding.go: nil/internal/types/account.go
	$(TYPES_RLPGEN) -type SmartContract -out account_rlp_encoding.go -decoder

nil/internal/types/token_balance_rlp_encoding.go: nil/internal/types/account.go
	$(TYPES_RLPGEN) -type TokenBalance -out token_balance_rlp_encoding.go -decoder

nil/internal/types/version_info_rlp_encoding.go: nil/internal/types/version_info.go
	$(TYPES_RLPGEN) -type VersionInfo -out version_info_rlp_encoding.go -decoder

endif
