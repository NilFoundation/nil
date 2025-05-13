SRLZ_RLPGEN := cd nil/internal/serialization && $(RLPGEN_BIN)

RLP_SRLZ_TARGETS = \
    nil/internal/serialization/key_value_rlp_encoding.go \
    nil/internal/serialization/map_holder_rlp_encoding.go

$(RLP_SRLZ_TARGETS): | $(RLPGEN_BIN)

.PHONY: rlp_srlz
rlp_srlz: $(RLP_SRLZ_TARGETS)

nil/internal/serialization/key_value_rlp_encoding.go: nil/internal/serialization/map.go
	$(SRLZ_RLPGEN) -type KeyValue -out key_value_rlp_encoding.go -decoder

nil/internal/serialization/map_holder_rlp_encoding.go: nil/internal/serialization/map.go
	$(SRLZ_RLPGEN) -type MapHolder -out map_holder_rlp_encoding.go -decoder
