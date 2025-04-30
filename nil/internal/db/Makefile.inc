DB_RLPGEN := cd nil/internal/db && $(RLPGEN_BIN)

RLP_DB_TARGET = nil/internal/db/block_hash_and_transaction_index_rlp_encoding.go

.PHONY: rlp_db rlp_types
rlp_db: $(RLP_DB_TARGET)

$(RLP_DB_TARGET): nil/internal/db/tables.go nil/common/hash.go nil/common/length.go nil/internal/types/transaction.go | $(RLPGEN_BIN)
	$(DB_RLPGEN) -type BlockHashAndTransactionIndex -out block_hash_and_transaction_index_rlp_encoding.go -decoder
