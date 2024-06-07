GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil


GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS)
GO_DBG_BUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build -tags $(BUILD_TAGS),debug,assert -gcflags=all="-N -l"  # see delve docs
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test -tags $(BUILD_TAGS),debug,assert $(GO_FLAGS) ./... -p 2

default: all

test:
	$(GOTEST) $(CMDARGS)

%.cmd:
	@# Note: $* is replaced by the command name
	@echo "Building $*"
	@cd ./cmd/$* && $(GOBUILD) -o $(GOBIN)/$*
	@echo "Run \"$(GOBIN)/$*\" to launch $*."

%.runcmd: %.cmd
	@$(GOBIN)/$* $(CMDARGS)

COMMANDS += nil nil_cli

$(COMMANDS): %: %.cmd

all: $(COMMANDS)

ssz:
	@echo "Generating SSZ code"
	pushd core/db && go generate && popd
	pushd core/types && go generate && popd
	pushd core/mpt && go generate && popd

compile-contracts:
	@echo "Generating contracts code"
	cd contracts && go generate

lint: lint-compiled-contracts
	go mod tidy
	gofumpt -l -w .
	gci write .
	golangci-lint run

lint-compiled-contracts:
	TMP_DIR=$$(mktemp -d); \
	contracts/generate.sh "$$TMP_DIR" && diff -ru contracts/compiled "$$TMP_DIR"; \
	d=$$?; \
	rm -rf "$$TMP_DIR"; \
	test $$d

clean:
	go clean -cache
	rm -fr build/*

solc:
	$(eval ARGS ?= --help)
	@GOPRIVATE="$(GOPRIVATE)" $(GO) run $(GO_FLAGS) tools/solc/bin/main.go $(ARGS)
