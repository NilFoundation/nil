root = nil/services/synccommittee

.PHONY: generate_mocks
generate_mocks: $(root)/internal/api/task_request_handler_generated_mock.go

$(root)/internal/api/task_request_handler_generated_mock.go:
	cd $(root)/internal/api && go generate
