GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GIT_TAG = $(shell git describe --tags 2>/dev/null || echo "0.1.0")
GIT_REVISION = $(shell git rev-list --count HEAD)

GO_FLAGS = -ldflags "-X '${PACKAGE}/common/version.gitCommit=$(GIT_COMMIT)' \
		     -X '${PACKAGE}/common/version.gitTag=$(GIT_TAG)' \
		     -X '${PACKAGE}/common/version.gitRevision=$(GIT_REVISION)'"
GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS)
GO_DBG_BUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build -tags $(BUILD_TAGS),debug,assert -gcflags=all="-N -l"  # see delve docs
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test -tags $(BUILD_TAGS),debug,assert,test $(GO_FLAGS) ./... -p 2

COMMANDS += nil nil_cli nil_load_generator exporter

all: $(COMMANDS)

.PHONY: test
test: compile-contracts ssz
	$(GOTEST) $(CMDARGS)

%.cmd:
	@# Note: $* is replaced by the command name
	@echo "Building $*"
	@cd ./nil/cmd/$* && $(GOBUILD) -o $(GOBIN)/$*
	@echo "Run \"$(GOBIN)/$*\" to launch $*."

%.runcmd: %.cmd
	@$(GOBIN)/$* $(CMDARGS)

$(COMMANDS): %: compile-contracts ssz %.cmd

include nil/internal/db/Makefile.inc
include nil/internal/mpt/Makefile.inc
include nil/internal/types/Makefile.inc

.PHONY: ssz
ssz: ssz_db ssz_mpt ssz_types

contracts/compiled/%.bin: $(wildcard nil/contracts/solidity/tests/*.sol) $(wildcard nil/contracts/solidity/*.sol)
	go generate nil/contracts/generate.go

compile-contracts: contracts/compiled/Faucet.bin contracts/compiled/Wallet.bin

lint: ssz
	GOPROXY= go mod tidy
	GOPROXY= go mod vendor
	gofumpt -l -w .
	gci write . --skip-generated --skip-vendor
	golangci-lint run

rpcspec:
	go run nil/cmd/spec_generator/spec_generator.go

clean:
	go clean -cache
	rm -fr build/*
	rm -fr contracts/compiled/*

solc:
	$(eval ARGS ?= --help)
	@GOPRIVATE="$(GOPRIVATE)" $(GO) run $(GO_FLAGS) nil/tools/solc/bin/main.go $(ARGS)
