root_sce = ./nil/services/synccommittee

.PHONY: generate_synccommittee_mocks
generate_synccommittee_mocks: \
	$(root_sce)/internal/api/task_handler_generated_mock.go \
	$(root_sce)/internal/api/task_state_change_handler_generated_mock.go \
	$(root_sce)/internal/api/task_request_handler_generated_mock.go \
	$(root_sce)/internal/scheduler/task_scheduler_generated_mock.go \
    $(root_sce)/internal/rollupcontract/eth_client_generated_mock.go \
    $(root_sce)/internal/srv/worker_generated_mock.go \
    $(root_sce)/internal/test_utils/op_context_generated_mock.go \
    $(root_sce)/prover/tracer/storage_getter_setter_generated_mock.go

$(root_sce)/internal/api/task_handler_generated_mock.go: \
	$(root_sce)/internal/api/task_handler.go \
	$(root_sce)/internal/types/prover_tasks.go
	go generate $(root_sce)/internal/api/task_handler.go

$(root_sce)/internal/api/task_request_handler_generated_mock.go: \
	$(root_sce)/internal/api/task_request_handler.go \
	$(root_sce)/internal/types/task_result.go \
	$(root_sce)/internal/types/prover_tasks.go
	go generate $(root_sce)/internal/api/task_request_handler.go

$(root_sce)/internal/api/task_state_change_handler_generated_mock.go: \
	$(root_sce)/internal/api/task_state_change_handler.go \
	$(root_sce)/internal/types/prover_tasks.go
	go generate $(root_sce)/internal/api/task_state_change_handler.go

$(root_sce)/internal/scheduler/task_scheduler_generated_mock.go: \
	$(root_sce)/internal/scheduler/task_scheduler.go \
	$(root_sce)/internal/api/task_request_handler.go \
	$(root_sce)/internal/srv/worker.go \
	$(root_sce)/internal/types/prover_tasks.go \
	$(root_sce)/internal/types/task_result.go \
	$(root_sce)/public/task_debug_api.go \
	$(root_sce)/public/task_view.go
	go generate $(root_sce)/internal/scheduler

$(root_sce)/internal/srv/worker_generated_mock.go: \
	$(root_sce)/internal/srv/worker.go
	go generate $(root_sce)/internal/srv/worker.go

$(root_sce)/internal/rollupcontract/eth_client_generated_mock.go: \
	$(root_sce)/internal/rollupcontract/eth_client.go
	go generate $(root_sce)/internal/rollupcontract

$(root_sce)/internal/test_utils/op_context_generated_mock.go: \
	./nil/internal/tracing/hooks.go
	go generate $(root_sce)/prover/tracer/internal/testutils/generate.go

$(root_sce)/prover/tracer/storage_getter_setter_generated_mock.go: \
    tracer_constants \
    pb_synccommittee \
    $(root_sce)/prover/tracer/storage_tracer.go
	go generate $(root_sce)/prover/tracer

.PHONY: synccommittee_types
synccommittee_types: \
    generate_contract_from_abi \
    generate_synccommittee_mocks \
	$(root_sce)/internal/types/tasktype_string.go \
	$(root_sce)/internal/types/proverresulttype_string.go \
	$(root_sce)/internal/types/taskstatus_string.go \
	$(root_sce)/internal/types/circuittype_string.go \
	$(root_sce)/internal/types/taskerrtype_string.go \
	$(root_sce)/public/taskdebugorder_string.go

$(root_sce)/internal/types/tasktype_string.go: $(root_sce)/internal/types/task_type.go
	go generate -run="TaskType" $(root_sce)/internal/types/generate.go
$(root_sce)/internal/types/proverresulttype_string.go: $(root_sce)/internal/types/task_result.go
	go generate -run="ProverResultType" $(root_sce)/internal/types/generate.go
$(root_sce)/internal/types/taskstatus_string.go: $(root_sce)/internal/types/task_status.go
	go generate -run="TaskStatus" $(root_sce)/internal/types/generate.go
$(root_sce)/internal/types/circuittype_string.go: $(root_sce)/internal/types/prover_tasks.go
	go generate -run="CircuitType" $(root_sce)/internal/types/generate.go
$(root_sce)/internal/types/taskerrtype_string.go: $(root_sce)/internal/types/errors.go
	go generate -run="TaskErrType" $(root_sce)/internal/types/generate.go
$(root_sce)/public/taskdebugorder_string.go: $(root_sce)/public/task_debug_api.go
	go generate $(root_sce)/public

.PHONY: pb_synccommittee
pb_synccommittee: $(root_sce)/prover/proto/traces.pb.go \
                  $(root_sce)/internal/types/proto/blob_transaction.pb.go

$(root_sce)/prover/proto/traces.pb.go: $(root_sce)/prover/proto/traces.proto
	protoc --proto_path=$(root_sce)/prover/proto --go_out=$(root_sce)/prover/ $(root_sce)/prover/proto/traces.proto $(root_sce)/prover/proto/verbose_mpt.proto


$(root_sce)/internal/types/proto/blob_transaction.pb.go: $(root_sce)/internal/types/proto/blob_transaction.proto
	protoc --go_out=$(root_sce)/internal/types/ $(root_sce)/internal/types/proto/blob_transaction.proto

.PHONY: generate_contract_from_abi
generate_contract_from_abi:
	go generate $(root_sce)/internal/rollupcontract/generate.go

.PHONY: tracer_constants
tracer_constants: $(root_sce)/prover/tracer/internal/constants/proto_hash_generated.go
$(root_sce)/prover/tracer/internal/constants/proto_hash_generated.go: \
	$(root_sce)/prover/proto/traces.proto \
	$(root_sce)/prover/tracer/internal/scripts/generate_constants.sh
	mkdir -p $(root_sce)/prover/tracer/internal/constants \
		&& cd $(root_sce)/prover/tracer/internal/constants \
		&& bash ../scripts/generate_constants.sh ProtoHash `cat ../../../proto/*.proto | sha256sum | cut -d ' ' -f 1` proto_hash_generated.go
