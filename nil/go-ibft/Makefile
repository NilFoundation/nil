.PHONY: lint
lint:
	golangci-lint run --config ../../.golangci.yml

.PHONY: gofumpt
gofumpt:
	gofumpt -l -w .

.PHONY: fixalign
fixalign:
	go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
	fieldalignment -fix $(filter-out $@,$(MAKECMDGOALS)) # the full package name (not path!)
