include nil/services/synccommittee/Makefile.inc
include nil/services/relayer/Makefile.inc

root_client = nil/client
root_vm = nil/internal/vm
root_db = nil/internal/db
root_rollup = nil/services/rollup

.PHONY: gen_rollup_contracts_bindings
gen_rollup_contracts_bindings: \
	$(root_relayer)/compile_abi \
	$(root_relayer)/embed_l1_abi \
	$(root_relayer)/generate_l1_abi \
	$(root_relayer)/embed_l2_abi \
	$(root_relayer)/gen_l1_mocks \
	$(root_relayer)/gen_l2_mocks

.PHONY: generate_mocks
generate_mocks: \
	$(root_client)/client_generated_mock.go \
	$(root_vm)/state_generated_mock.go \
	$(root_db)/rwtx_generated_mock.go \
	$(root_db)/db_generated_mock.go \
	$(root_rollup)/l1_fetcher_generated_mock.go \

$(root_client)/client_generated_mock.go: $(root_client)/client.go ssz_types
	cd $(root_client) && go generate

$(root_vm)/state_generated_mock.go: $(root_vm)/interface.go ssz_types
	cd $(root_vm) && go generate

$(root_db)/rwtx_generated_mock.go: $(root_db)/kv.go ssz_types
	cd $(root_db) && go generate -run="rwtx_" kv.go

$(root_db)/db_generated_mock.go: $(root_db)/kv.go ssz_types
	cd $(root_db) && go generate -run="db_" kv.go

$(root_rollup)/l1_fetcher_generated_mock.go: $(root_rollup)/l1_fetcher.go
	cd $(root_rollup) && go generate l1_fetcher.go
