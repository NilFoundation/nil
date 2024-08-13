.PHONY: ssz_mpt
ssz_mpt: nil/internal/mpt/node_encoding.go

nil/internal/mpt/node_encoding.go: nil/internal/mpt/node.go nil/internal/mpt/path.go nil/internal/mpt/generate.go
	cd nil/internal/mpt && go generate