root_sce = nil/services/synccommittee
root_client = nil/client
root_vm = nil/internal/vm

.PHONY: generate_mocks
generate_mocks: \
	$(root_sce)/internal/api/task_handler_generated_mock.go \
	$(root_sce)/internal/api/task_state_change_handler_generated_mock.go \
	$(root_sce)/internal/api/task_request_handler_generated_mock.go \
	$(root_sce)/internal/scheduler/task_scheduler_generated_mock.go \
	$(root_client)/client_generated_mock.go \
	$(root_vm)/state_generated_mock.go

$(root_sce)/internal/api/task_handler_generated_mock.go: $(root_sce)/internal/api/task_handler.go $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/api && go generate

$(root_sce)/internal/api/task_request_handler_generated_mock.go: $(root_sce)/internal/api/task_request_handler.go $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/api && go generate

$(root_sce)/internal/api/task_state_change_handler_generated_mock.go: $(root_sce)/internal/api/task_state_change_handler.go $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/api && go generate

$(root_sce)/internal/scheduler/task_scheduler_generated_mock.go: \
	$(root_sce)/internal/scheduler/task_scheduler.go \
	$(root_sce)/internal/api/task_request_handler.go \
	$(root_sce)/internal/types/prover_tasks.go \
	$(root_sce)/public/task_debug_api.go \
	$(root_sce)/public/task_view.go
	cd $(root_sce)/internal/scheduler && go generate

$(root_client)/client_generated_mock.go: $(root_client)/client.go
	cd $(root_client) && go generate

$(root_vm)/state_generated_mock.go: $(root_vm)/interface.go
	cd $(root_vm) && go generate

.PHONY: synccommittee_types
synccommittee_types: \
	$(root_sce)/internal/types/tasktype_string.go \
	$(root_sce)/internal/types/proverresulttype_string.go \
	$(root_sce)/internal/types/taskstatus_string.go \
	$(root_sce)/internal/types/circuittype_string.go \
	$(root_sce)/public/taskdebugorder_string.go

$(root_sce)/internal/types/tasktype_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
$(root_sce)/internal/types/proverresulttype_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
$(root_sce)/internal/types/taskstatus_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
$(root_sce)/internal/types/circuittype_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
$(root_sce)/public/taskdebugorder_string.go: $(root_sce)/public/task_debug_api.go
	cd $(root_sce)/public && go generate

.PHONY: pb_synccommittee
pb_synccommittee: $(root_sce)/prover/proto/traces.pb.go

$(root_sce)/prover/proto/traces.pb.go: $(root_sce)/prover/proto/traces.proto
	protoc --go_out=$(root_sce)/prover/ $(root_sce)/prover/proto/traces.proto

.PHONY: tracer_constants
tracer_constants: $(root_sce)/prover/tracer/internal/constants/proto_hash_generated.go

$(root_sce)/prover/tracer/internal/constants/proto_hash_generated.go: \
	$(root_sce)/prover/proto/traces.proto \
	$(root_sce)/prover/tracer/internal/scripts/generate_constants.sh
	mkdir -p $(root_sce)/prover/tracer/internal/constants \
		&& cd $(root_sce)/prover/tracer/internal/constants \
		&& bash ../scripts/generate_constants.sh ProtoHash `sha256sum ../../../proto/traces.proto |  cut -d ' ' -f 1` proto_hash_generated.go
