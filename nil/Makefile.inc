include nil/services/synccommittee/Makefile.inc

root_client = nil/client
root_vm = nil/internal/vm
root_db = nil/internal/db

.PHONY: generate_mocks
generate_mocks: \
	$(root_client)/client_generated_mock.go \
	$(root_vm)/state_generated_mock.go \
	$(root_db)/rwtx_generated_mock.go \
	$(root_db)/db_generated_mock.go

$(root_client)/client_generated_mock.go: $(root_client)/client.go
	cd $(root_client) && go generate

$(root_vm)/state_generated_mock.go: $(root_vm)/interface.go
	cd $(root_vm) && go generate

$(root_db)/rwtx_generated_mock.go: $(root_db)/kv.go ssz_types
	cd $(root_db) && go generate -run="rwtx_" kv.go

$(root_db)/db_generated_mock.go: $(root_db)/kv.go ssz_types
	cd $(root_db) && go generate -run="db_" kv.go
