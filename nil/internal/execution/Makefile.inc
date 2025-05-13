EXEC_RLPGEN := cd nil/internal/execution && $(RLPGEN_BIN)

RLP_EXECUTION_TARGETS = \
    nil/internal/execution/parent_block_serializable_rlp_encoding.go \
    nil/internal/execution/proposal_serializable_rlp_encoding.go

$(RLP_EXECUTION_TARGETS): rlp_types rlp_srlz | $(RLPGEN_BIN)

.PHONY: rlp_execution
rlp_execution: $(RLP_EXECUTION_TARGETS)

nil/internal/execution/parent_block_serializable_rlp_encoding.go: nil/internal/execution/proposal.go
	$(EXEC_RLPGEN) -type ParentBlockSerializable -out parent_block_serializable_rlp_encoding.go -decoder

nil/internal/execution/proposal_serializable_rlp_encoding.go: nil/internal/execution/proposal.go
	$(EXEC_RLPGEN) -type ProposalSerializable -out proposal_serializable_rlp_encoding.go -decoder
