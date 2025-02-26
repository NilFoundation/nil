GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil

GO_FLAGS =
GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS)
GO_DBG_BUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build -tags $(BUILD_TAGS),debug,assert -gcflags=all="-N -l"  # see delve docs
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test -tags $(BUILD_TAGS),debug,assert,test $(GO_FLAGS) ./... -p 2

SC_COMMANDS = sync_committee sync_committee_cli proof_provider prover nil_block_generator
COMMANDS += nild nil nil_load_generator exporter cometa faucet journald_forwarder $(SC_COMMANDS)

all: $(COMMANDS)

.PHONY: generated
generated: ssz pb compile-contracts generate_mocks synccommittee_types tracer_constants

.PHONY: test
test: generated
	$(GOTEST) $(CMDARGS)

%.cmd: generated
	@# Note: $* is replaced by the command name
	@echo "Building $*"
	@cd ./nil/cmd/$* && $(GOBUILD) -o $(GOBIN)/$*
	@echo "Run \"$(GOBIN)/$*\" to launch $*."

%.runcmd: %.cmd
	@$(GOBIN)/$* $(CMDARGS)

$(COMMANDS): %: generated %.cmd

include nil/common/sszx/Makefile.inc
include nil/internal/db/Makefile.inc
include nil/internal/mpt/Makefile.inc
include nil/internal/types/Makefile.inc
include nil/internal/config/Makefile.inc
include nil/internal/execution/Makefile.inc
include nil/services/rpc/rawapi/proto/Makefile.inc
include nil/go-ibft/messages/proto/Makefile.inc
include nil/Makefile.inc

.PHONY: ssz
ssz: ssz_sszx ssz_db ssz_mpt ssz_types ssz_config ssz_execution

.PHONY: pb
pb: pb_rawapi pb_ibft pb_synccommittee

SOL_FILES := $(wildcard nil/contracts/solidity/tests/*.sol nil/contracts/solidity/*.sol)
BIN_FILES := $(patsubst nil/contracts/solidity/%.sol, contracts/compiled/%.bin, $(SOL_FILES))
CHECK_LOCKS_DIRECTORIES := ./nil/internal/network
# TODO: Uncomment the line below when all checks have passed to run checklocks across all directories
# CHECK_LOCKS_DIRECTORIES := $(shell find ./nil -type f -name "*.go" | xargs -I {} dirname {} | sort -u)

.PHONY: compile-bins
compile-bins:
	cd nil/contracts && go generate generate.go

$(BIN_FILES): | compile-bins

compile-contracts: $(BIN_FILES)

lint: generated
	GOPROXY= go mod tidy
	GOPROXY= go mod vendor
	gofumpt -l -w .
	gci write . --skip-generated --skip-vendor
	golangci-lint run
	@for dir in $(CHECK_LOCKS_DIRECTORIES); do \
		echo "  >> Checking locks correctness in $$dir"; \
		GOFLAGS="-tags=test" go run gvisor.dev/gvisor/tools/checklocks/cmd/checklocks "$$dir" || exit 1; \
	done

rpcspec:
	go run nil/cmd/spec_generator/spec_generator.go

clean:
	go clean -cache
	rm -fr build/*
	rm -fr contracts/compiled/*

solc:
	$(eval ARGS ?= --help)
	@GOPRIVATE="$(GOPRIVATE)" $(GO) run $(GO_FLAGS) nil/tools/solc/bin/main.go $(ARGS)
