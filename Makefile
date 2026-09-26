# Developer and CI entry points. `make check` runs everything CI runs for Go
# code; run it before opening a pull request.

GO ?= go

GOLANGCI_LINT_VERSION ?= v2.14.0
GOVULNCHECK_VERSION   ?= v1.8.0

# Tools run through `go run` so they are built with the toolchain from go.mod.
# Override with a local binary, e.g. `make lint GOLANGCI_LINT=golangci-lint`.
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOVULNCHECK   ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

GENERATED := receiver/fritzbox/internal/metadata \
	receiver/fritzbox/generated_component_test.go \
	receiver/fritzbox/generated_package_test.go \
	receiver/fritzbox/documentation.md

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: check
check: generate-check lint test-race vuln build ## Run all Go checks CI runs.

.PHONY: generate
generate: ## Regenerate code from metadata.yaml.
	$(GO) generate ./receiver/...

.PHONY: generate-check
generate-check: generate ## Fail if generated code is stale.
	@git diff --exit-code -- $(GENERATED) || \
		(echo "Generated files are stale. Run 'make generate' and commit the result."; exit 1)

.PHONY: fmt
fmt: ## Format Go code.
	$(GOLANGCI_LINT) fmt ./...

.PHONY: lint
lint: ## Run golangci-lint.
	$(GOLANGCI_LINT) run ./...

.PHONY: test
test: ## Run unit and integration tests.
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run tests with the race detector, uncached.
	$(GO) test -race -count=1 ./...

.PHONY: vuln
vuln: ## Scan for known vulnerabilities in called code.
	$(GOVULNCHECK) ./...

.PHONY: build
build: ## Build the collector binary into bin/.
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/otelcol-fritzbox ./cmd/otelcol-fritzbox

.PHONY: clean
clean: ## Remove build output.
	rm -rf bin dist
