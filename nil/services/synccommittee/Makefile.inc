root = nil/services/synccommittee

.PHONY: generate_mocks
generate_mocks: $(root)/internal/api/task_handler_generated_mock.go $(root)/internal/api/task_request_handler_generated_mock.go

$(root)/internal/api/task_handler_generated_mock.go: $(root)/internal/api/task_handler.go $(root)/internal/types/prover_tasks.go
	cd $(root)/internal/api && go generate

$(root)/internal/api/task_request_handler_generated_mock.go: $(root)/internal/api/task_request_handler.go $(root)/internal/types/prover_tasks.go
	cd $(root)/internal/api && go generate
