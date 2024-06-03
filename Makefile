GO ?= go
GOBIN = $(CURDIR)/build/bin
GOPRIVATE = github.com/NilFoundation
PACKAGE = github.com/NilFoundation/nil


GOBUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build $(GO_FLAGS)
GO_DBG_BUILD = GOPRIVATE="$(GOPRIVATE)" $(GO) build -tags $(BUILD_TAGS),debug -gcflags=all="-N -l"  # see delve docs
GOTEST = GOPRIVATE="$(GOPRIVATE)" GODEBUG=cgocheck=0 $(GO) test $(GO_FLAGS) ./... -p 2

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

lint:
	go mod tidy
	gofumpt -l -w .
	gci write .
	golangci-lint run

clean:
	go clean -cache
	rm -fr build/*

solc:
	$(eval ARGS ?= --help)
	@GOPRIVATE="$(GOPRIVATE)" $(GO) run $(GO_FLAGS) tools/solc/bin/main.go $(ARGS)
