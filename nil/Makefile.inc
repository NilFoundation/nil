root_sce = nil/services/synccommittee
root_client = nil/client

.PHONY: generate_mocks
generate_mocks: $(root_sce)/internal/api/task_handler_generated_mock.go $(root_sce)/internal/api/task_request_handler_generated_mock.go $(root_client)/client_generated_mock.go

$(root_sce)/internal/api/task_handler_generated_mock.go: $(root_sce)/internal/api/task_handler.go $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/api && go generate

$(root_sce)/internal/api/task_request_handler_generated_mock.go: $(root_sce)/internal/api/task_request_handler.go $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/api && go generate

$(root_client)/client_generated_mock.go: $(root_client)/client.go
	cd $(root_client) && go generate

.PHONY: synccommittee_types
synccommittee_types: $(root_sce)/internal/types/tasktype_string.go $(root_sce)/internal/types/proverresulttype_string.go

$(root_sce)/internal/types/tasktype_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
$(root_sce)/internal/types/proverresulttype_string.go: $(root_sce)/internal/types/prover_tasks.go
	cd $(root_sce)/internal/types && go generate
