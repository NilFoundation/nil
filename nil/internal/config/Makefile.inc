CONFIG_RLPGEN := cd nil/internal/config && $(RLPGEN_BIN)

RLP_CONFIG_TARGETS = \
    nil/internal/config/list_validators_rlp_encoding.go \
    nil/internal/config/param_validators_rlp_encoding.go \
    nil/internal/config/validator_info_rlp_encoding.go \
    nil/internal/config/param_gas_price_rlp_encoding.go \
    nil/internal/config/param_l1_block_info_rlp_encoding.go

$(RLP_CONFIG_TARGETS): | $(RLPGEN_BIN)

.PHONY: rlp_config
rlp_config: $(RLP_CONFIG_TARGETS) rlp_types

nil/internal/config/list_validators_rlp_encoding.go: nil/internal/config/params.go
	$(CONFIG_RLPGEN) -type ListValidators -out list_validators_rlp_encoding.go -decoder

nil/internal/config/param_validators_rlp_encoding.go: nil/internal/config/params.go
	$(CONFIG_RLPGEN) -type ParamValidators -out param_validators_rlp_encoding.go -decoder

nil/internal/config/validator_info_rlp_encoding.go: nil/internal/config/params.go
	$(CONFIG_RLPGEN) -type ValidatorInfo -out validator_info_rlp_encoding.go -decoder

nil/internal/config/param_gas_price_rlp_encoding.go: nil/internal/config/params.go
	$(CONFIG_RLPGEN) -type ParamGasPrice -out param_gas_price_rlp_encoding.go -decoder

nil/internal/config/param_l1_block_info_rlp_encoding.go: nil/internal/config/params.go
	$(CONFIG_RLPGEN) -type ParamL1BlockInfo -out param_l1_block_info_rlp_encoding.go -decoder
