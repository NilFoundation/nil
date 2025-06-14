root_sc = ./nil/services/synccommittee
root_contracts = rollup-bridge-contracts/contracts

.PHONY: sync_committee_targets
sync_committee_targets: \
	sync_committee_types_stringer \
	sync_committee_generate_mocks \
	sync_committee_protobuf \
	sync_committee_tracer_constants \
	$(root_sc)/generate_rollup_contract_abi \
	$(root_sc)/generate_fee_updater_abi \
	$(root_sc)/compile_l2_bridge_state_getter_abi

.PHONY: $(root_sc)/compile_fee_updater_abi
$(root_sc)/compile_fee_updater_abi:
	solc $(root_contracts)/bridge/l1/interfaces/IFeeStorage.sol --abi --overwrite -o $(root_sc)/core/feeupdater --allow-paths .,$(root_contracts)/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json

.PHONY: $(root_sc)/generate_fee_updater_abi
$(root_sc)/generate_fee_updater_abi: $(root_sc)/compile_fee_updater_abi
	cd $(root_sc)/core/feeupdater && go run github.com/ethereum/go-ethereum/cmd/abigen --abi IFeeStorage.abi --pkg=feeupdater --out=./i_fee_storage_contract_abi_generated.go

.PHONY: $(root_sc)/compile_rollup_contract_abi
$(root_sc)/compile_rollup_contract_abi:
	solc $(root_contracts)/interfaces/INilRollup.sol --abi --overwrite -o $(root_sc)/core/rollupcontract --allow-paths .,$(root_contracts)/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json

.PHONY: $(root_sc)/generate_rollup_contract_abi
$(root_sc)/generate_rollup_contract_abi: $(root_sc)/compile_rollup_contract_abi
	cd $(root_sc)/core/rollupcontract && go run github.com/ethereum/go-ethereum/cmd/abigen --abi INilRollup.abi --pkg=rollupcontract --out=./i_rollup_contract_abi_generated.go

.PHONY: $(root_sc)/compile_l2_bridge_state_getter_abi
$(root_sc)/compile_l2_bridge_state_getter_abi:
	solc $(root_contracts)/bridge/l2/interfaces/IL2BridgeStateGetter.sol --abi --overwrite -o $(root_sc)/core/bridgecontract --allow-paths .,$(root_contracts)/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json

$(root_sc)/internal/l1client/eth_client_generated_mock.go:
	cd $(root_sc)/internal/l1client && bash ../../internal/scripts/generate_mock.sh EthClient

.PHONY: sync_committee_types_stringer
sync_committee_types_stringer: \
	$(root_sc)/core/batches/constraints/checkresult_string.go \
	$(root_sc)/internal/types/tasktype_string.go \
	$(root_sc)/internal/types/proverresulttype_string.go \
	$(root_sc)/internal/types/taskstatus_string.go \
	$(root_sc)/internal/types/circuittype_string.go \
	$(root_sc)/internal/types/taskerrtype_string.go \
	$(root_sc)/public/taskdebugorder_string.go

$(root_sc)/core/batches/constraints/checkresult_string.go: $(root_sc)/core/batches/constraints/result.go
	go generate -run="CheckResult" $(root_sc)/core/batches/constraints/result.go
$(root_sc)/internal/types/tasktype_string.go: $(root_sc)/internal/types/task_type.go
	go generate -run="TaskType" $(root_sc)/internal/types/generate.go
$(root_sc)/internal/types/proverresulttype_string.go: $(root_sc)/internal/types/task_result.go
	go generate -run="ProverResultType" $(root_sc)/internal/types/generate.go
$(root_sc)/internal/types/taskstatus_string.go: $(root_sc)/internal/types/task_status.go
	go generate -run="TaskStatus" $(root_sc)/internal/types/generate.go
$(root_sc)/internal/types/circuittype_string.go: $(root_sc)/internal/types/prover_tasks.go
	go generate -run="CircuitType" $(root_sc)/internal/types/generate.go
$(root_sc)/internal/types/taskerrtype_string.go: $(root_sc)/internal/types/errors.go
	go generate -run="TaskErrType" $(root_sc)/internal/types/generate.go
$(root_sc)/public/taskdebugorder_string.go: $(root_sc)/public/task_debug_api.go
	go generate $(root_sc)/public

.PHONY: sync_committee_generate_mocks
sync_committee_generate_mocks: \
	$(root_sc)/internal/api/task_handler_generated_mock.go \
	$(root_sc)/internal/api/task_state_change_handler_generated_mock.go \
	$(root_sc)/internal/api/task_request_handler_generated_mock.go \
	$(root_sc)/core/state_reset_launcher_generated_mock.go \
	$(root_sc)/internal/l1client/eth_client_generated_mock.go \
	$(root_sc)/internal/srv/worker_generated_mock.go \
	$(root_sc)/internal/test_utils/op_context_generated_mock.go \
	$(root_sc)/prover/tracer/storage_getter_setter_generated_mock.go \
	$(root_sc)/core/rollupcontract/wrapper_generated_mock.go \
	$(root_sc)/core/feeupdater/contract_wrapper_generated_mock.go

$(root_sc)/internal/api/task_handler_generated_mock.go: \
	$(root_sc)/internal/api/task_handler.go \
	$(root_sc)/internal/types/prover_tasks.go
	go generate $(root_sc)/internal/api/task_handler.go

$(root_sc)/internal/api/task_request_handler_generated_mock.go: \
	$(root_sc)/internal/api/task_request_handler.go \
	$(root_sc)/internal/types/task_result.go \
	$(root_sc)/internal/types/prover_tasks.go
	go generate $(root_sc)/internal/api/task_request_handler.go

$(root_sc)/internal/api/task_state_change_handler_generated_mock.go: \
	$(root_sc)/internal/api/task_state_change_handler.go \
	$(root_sc)/internal/types/prover_tasks.go
	go generate $(root_sc)/internal/api/task_state_change_handler.go

$(root_sc)/core/state_reset_launcher_generated_mock.go: \
	$(root_sc)/generate_rollup_contract_abi \
	$(root_sc)/core/task_state_change_handler.go
	go generate $(root_sc)/core/task_state_change_handler.go

$(root_sc)/internal/srv/worker_generated_mock.go: \
	$(root_sc)/internal/srv/worker.go
	go generate $(root_sc)/internal/srv/worker.go

$(root_sc)/core/rollupcontract/eth_client_generated_mock.go: \
	$(root_sc)/core/rollupcontract/eth_client.go
	go generate $(root_sc)/core/rollupcontract

$(root_sc)/internal/test_utils/op_context_generated_mock.go: \
	./nil/internal/tracing/hooks.go
	go generate $(root_sc)/prover/tracer/internal/testutils/generate.go

$(root_sc)/prover/tracer/storage_getter_setter_generated_mock.go: \
	sync_committee_tracer_constants \
	sync_committee_protobuf \
	$(root_sc)/prover/tracer/storage_tracer.go
	go generate $(root_sc)/prover/tracer

$(root_sc)/core/rollupcontract/wrapper_generated_mock.go: \
	$(root_sc)/generate_rollup_contract_abi \
	$(root_sc)/core/rollupcontract/wrapper.go
	cd $(root_sc)/core/rollupcontract && bash ../../internal/scripts/generate_mock.sh Wrapper

$(root_sc)/core/feeupdater/contract_wrapper_generated_mock.go: \
	$(root_sc)/core/feeupdater/contract_wrapper.go \
	$(root_sc)/generate_fee_updater_abi
	cd $(root_sc)/core/feeupdater && bash ../../internal/scripts/generate_mock.sh NilGasPriceOracleContract

.PHONY: sync_committee_protobuf
sync_committee_protobuf: \
	$(root_sc)/prover/proto/traces.pb.go \
	$(root_sc)/internal/types/proto/blob_transaction.pb.go

$(root_sc)/prover/proto/traces.pb.go: $(root_sc)/prover/proto/traces.proto
	@echo "Generating traces.pb.go..."
	protoc \
		--proto_path=$(root_sc)/prover/proto \
		--go_out=$(root_sc)/prover/ $(root_sc)/prover/proto/traces.proto

$(root_sc)/internal/types/proto/blob_transaction.pb.go: $(root_sc)/internal/types/proto/blob_transaction.proto
	protoc --go_out=$(root_sc)/internal/types/ $(root_sc)/internal/types/proto/blob_transaction.proto

.PHONY: sync_committee_tracer_constants
sync_committee_tracer_constants: $(root_sc)/prover/tracer/internal/constants/proto_hash_generated.go
$(root_sc)/prover/tracer/internal/constants/proto_hash_generated.go: \
	$(root_sc)/prover/proto/traces.proto \
	$(root_sc)/prover/tracer/internal/scripts/generate_constants.sh
	mkdir -p $(root_sc)/prover/tracer/internal/constants \
		&& cd $(root_sc)/prover/tracer/internal/constants \
		&& bash ../scripts/generate_constants.sh ProtoHash `cat ../../../proto/*.proto | sha256sum | cut -d ' ' -f 1` proto_hash_generated.go
