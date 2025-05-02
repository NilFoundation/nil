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
generate_mocks: ssz_types
	@for p in $(shell grep -rl '//go:generate mockery' --exclude-dir=vendor | xargs -n1 dirname | sort -u); do \
		go generate -run "mockery" $$p; \
	done
