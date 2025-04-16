root_sc = ./nil/services/synccommittee

.PHONY: sync_committee_targets
sync_committee_targets: \
	sync_committee_contract_abi \
	sync_committee_types_stringer \
	sync_committee_generate_mocks \
	sync_committee_protobuf \
	sync_committee_tracer_constants

.PHONY: sync_committee_contract_abi
sync_committee_contract_abi: \
	$(root_sc)/core/rollupcontract/rollupcontract_abi_generated.go

$(root_sc)/core/rollupcontract/rollupcontract_abi_generated.go: \
	$(root_sc)/core/rollupcontract/abi.json
	go generate $(root_sc)/core/rollupcontract/generate.go

.PHONY: sync_committee_types_stringer
sync_committee_types_stringer: \
	$(root_sc)/internal/types/tasktype_string.go \
	$(root_sc)/internal/types/proverresulttype_string.go \
	$(root_sc)/internal/types/taskstatus_string.go \
	$(root_sc)/internal/types/circuittype_string.go \
	$(root_sc)/internal/types/taskerrtype_string.go \
	$(root_sc)/public/taskdebugorder_string.go

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
	$(root_sc)/internal/scheduler/task_scheduler_generated_mock.go \
	$(root_sc)/core/rollupcontract/eth_client_generated_mock.go \
	$(root_sc)/internal/srv/worker_generated_mock.go \
	$(root_sc)/internal/test_utils/op_context_generated_mock.go \
	$(root_sc)/prover/tracer/storage_getter_setter_generated_mock.go

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
	sync_committee_contract_abi \
	$(root_sc)/core/task_state_change_handler.go
	go generate $(root_sc)/core/task_state_change_handler.go

$(root_sc)/internal/scheduler/task_scheduler_generated_mock.go: \
	$(root_sc)/internal/scheduler/task_scheduler.go \
	$(root_sc)/internal/api/task_request_handler.go \
	$(root_sc)/internal/srv/worker.go \
	$(root_sc)/internal/types/prover_tasks.go \
	$(root_sc)/internal/types/task_result.go \
	$(root_sc)/public/task_debug_api.go \
	$(root_sc)/public/task_view.go
	go generate $(root_sc)/internal/scheduler

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

.PHONY: sync_committee_protobuf
sync_committee_protobuf: \
	$(root_sc)/prover/proto/traces.pb.go \
	$(root_sc)/internal/types/proto/blob_transaction.pb.go

$(root_sc)/prover/proto/traces.pb.go: $(root_sc)/prover/proto/traces.proto
	@echo "Generating traces.pb.go..."
	protoc \
		--proto_path=$(root_sc)/prover/proto \
		--go_out=$(root_sc)/prover/ $(root_sc)/prover/proto/traces.proto $(root_sc)/prover/proto/verbose_mpt.proto

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
