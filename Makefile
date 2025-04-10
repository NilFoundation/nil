GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil

# Default mode is debug
ifeq ($(MODE),release)
    TAGS = "$(BUILD_TAGS)"
else
    TAGS = "$(BUILD_TAGS),assert"
endif

GO_FLAGS =
GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS) -tags $(TAGS)
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test -tags $(BUILD_TAGS),debug,assert,test,goexperiment.synctest $(GO_FLAGS) ./... -p 2

SC_COMMANDS = sync_committee sync_committee_cli proof_provider prover nil_block_generator relayer
COMMANDS += nild nil nil-load-generator indexer cometa faucet journald_forwarder relay stresser $(SC_COMMANDS)

BINARY_NAMES := cometa=nil-cometa indexer=nil-indexer
get_bin_name = $(if $(filter $(1)=%,$(BINARY_NAMES)),$(patsubst $(1)=%,%,$(filter $(1)=%,$(BINARY_NAMES))),$(1))


all: $(COMMANDS)

.PHONY: generated
generated: ssz pb compile-contracts generate_mocks sync_committee_targets

.PHONY: test
test: generated
	$(GOTEST) $(CMDARGS)

%.cmd: generated
	@# Note: $* is replaced by the command name
	$(eval BINNAME := $(call get_bin_name,$*))
	@echo "Building $*"
	@cd ./nil/cmd/$* && $(GOBUILD) -o $(GOBIN)/$(BINNAME)
	@echo "Run \"$(GOBIN)/$(BINNAME)\" to launch $*."

%.runcmd: %.cmd
	@$(GOBIN)/$* $(CMDARGS)

$(COMMANDS): %: generated %.cmd

$(SC_COMMANDS:%=%.cmd): gen_rollup_contracts_bindings

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
pb: pb_rawapi pb_ibft

SOL_FILES := $(wildcard nil/contracts/solidity/tests/*.sol nil/contracts/solidity/*.sol)
BIN_FILES := $(patsubst nil/contracts/solidity/%.sol, contracts/compiled/%.bin, $(SOL_FILES))
CHECK_LOCKS_DIRECTORIES := ./nil/internal/network \
                           ./nil/internal/network/connection_manager \
                           ./nil/internal/collate
# TODO: Uncomment the line below when all checks have passed to run checklocks across all directories
# CHECK_LOCKS_DIRECTORIES := $(shell find ./nil -type f -name "*.go" | xargs -I {} dirname {} | sort -u)

.PHONY: compile-bins
compile-bins:
	cd nil/contracts && go generate generate.go

$(BIN_FILES): | compile-bins

compile-contracts: $(BIN_FILES)

golangci-lint: gen_rollup_contracts_bindings
	golangci-lint run

format: generated
	GOPROXY= go mod tidy
	GOPROXY= go mod vendor
	golangci-lint fmt

lint: format golangci-lint checklocks

checklocks: generated
	@export GOFLAGS="$$GOFLAGS -tags=test,goexperiment.synctest"; \
	for dir in $(CHECK_LOCKS_DIRECTORIES); do \
		echo ">> Checking locks correctness in $$dir"; \
		go run gvisor.dev/gvisor/tools/checklocks/cmd/checklocks "$$dir" || exit 1; \
	done

rpcspec:
	go run nil/cmd/spec_generator/spec_generator.go

clean:
	go clean -cache
	rm -fr build/*
	rm -fr contracts/compiled/*
