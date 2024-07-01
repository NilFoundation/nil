.PHONY: ssz_mpt
ssz_mpt: core/mpt/node_encoding.go

core/mpt/node_encoding.go: core/mpt/node.go core/mpt/path.go
	cd core/mpt && go generate