.PHONY: json_rpc_types
json_rpc_types: nil/services/rpc/jsonrpc/types_easyjson.go

nil/services/rpc/jsonrpc/types_easyjson.go: nil/services/rpc/jsonrpc/types.go ssz_types
	cd nil/services/rpc/jsonrpc && GOFLAGS=-mod=mod go run github.com/mailru/easyjson/easyjson -all types.go
