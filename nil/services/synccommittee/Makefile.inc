root = nil/services/synccommittee

.PHONY: generate_mocks
generate_mocks:
	cd $(root)/internal/api && go generate
