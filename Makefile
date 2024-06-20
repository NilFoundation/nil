SHELL = bash # `pushd` is available only for bash
GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil


GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS)
GO_DBG_BUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build -tags $(BUILD_TAGS),debug,assert -gcflags=all="-N -l"  # see delve docs
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test -tags $(BUILD_TAGS),debug,assert $(GO_FLAGS) ./... -p 2

default: all

.PHONY: test
test: compile-contracts
	$(GOTEST) $(CMDARGS)

%.cmd:
	@# Note: $* is replaced by the command name
	@echo "Building $*"
	@cd ./cmd/$* && $(GOBUILD) -o $(GOBIN)/$*
	@echo "Run \"$(GOBIN)/$*\" to launch $*."

%.runcmd: %.cmd
	@$(GOBIN)/$* $(CMDARGS)

COMMANDS += nil nil_cli nil_load_generator

all: $(COMMANDS)

$(COMMANDS): %: compile-contracts %.cmd

ssz:
	@echo "Generating SSZ code"
	pushd core/db && go generate && popd
	pushd core/types && go generate && popd
	pushd core/mpt && go generate && popd

contracts/compiled/%.bin: $(wildcard contracts/solidity/tests/*.sol) $(wildcard contracts/solidity/*.sol)
	go generate contracts/generate.go

compile-contracts: contracts/compiled/Faucet.bin contracts/compiled/Wallet.bin

lint:
	go mod tidy
	gofumpt -l -w .
	gci write .
	golangci-lint run

rpcspec:
	go run cmd/spec_generator/spec_generator.go

clean:
	go clean -cache
	rm -fr build/*

solc:
	$(eval ARGS ?= --help)
	@GOPRIVATE="$(GOPRIVATE)" $(GO) run $(GO_FLAGS) tools/solc/bin/main.go $(ARGS)
