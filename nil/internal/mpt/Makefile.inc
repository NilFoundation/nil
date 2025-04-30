MPT_RLPGEN := cd nil/internal/mpt && $(RLPGEN_BIN)

RLP_MPT_TARGETS = \
    nil/internal/mpt/path_rlp_encoding.go \
    nil/internal/mpt/leaf_node_rlp_encoding.go \
    nil/internal/mpt/branch_node_rlp_encoding.go \
    nil/internal/mpt/extension_node_rlp_encoding.go

$(RLP_MPT_TARGETS): | $(RLPGEN_BIN)

.PHONY: rlp_mpt
rlp_mpt: $(RLP_MPT_TARGETS)

nil/internal/mpt/path_rlp_encoding.go: nil/internal/mpt/path.go
	$(MPT_RLPGEN) -type Path -out path_rlp_encoding.go -decoder

nil/internal/mpt/leaf_node_rlp_encoding.go: nil/internal/mpt/node.go nil/internal/mpt/path.go
	$(MPT_RLPGEN) -type LeafNode -out leaf_node_rlp_encoding.go -decoder

nil/internal/mpt/branch_node_rlp_encoding.go: nil/internal/mpt/node.go nil/internal/mpt/path.go
	$(MPT_RLPGEN) -type BranchNode -out branch_node_rlp_encoding.go -decoder

nil/internal/mpt/extension_node_rlp_encoding.go: nil/internal/mpt/node.go nil/internal/mpt/path.go
	$(MPT_RLPGEN) -type ExtensionNode -out extension_node_rlp_encoding.go -decoder
