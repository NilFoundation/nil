.PHONY: json_transport_types
json_transport_types: nil/services/rpc/transport/json_easyjson.go

nil/services/rpc/transport/json_easyjson.go: nil/services/rpc/transport/json.go ssz_types
	cd nil/services/rpc/transport && GOFLAGS=-mod=mod go run github.com/mailru/easyjson/easyjson -all json.go
