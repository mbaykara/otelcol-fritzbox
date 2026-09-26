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
check: generate-check builder-check lint test-race vuln build ## Run all Go checks CI runs.

.PHONY: generate
generate: ## Regenerate code from metadata.yaml.
	$(GO) generate ./receiver/...

.PHONY: generate-check
generate-check: generate ## Fail if generated code is stale.
	@git diff --exit-code -- $(GENERATED) || \
		(echo "Generated files are stale. Run 'make generate' and commit the result."; exit 1)

.PHONY: builder-check
builder-check: ## Fail if example/builder-config.yaml drifts from go.mod.
	./scripts/check-builder-config.sh

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

IMAGE ?= otelcol-fritzbox:dev

.PHONY: image
image: ## Build the container image for the host platform.
	docker buildx build --build-arg VERSION=$(shell git describe --tags --always --dirty) -t $(IMAGE) --load .

.PHONY: image-smoke
image-smoke: image ## Build the image and smoke-test it.
	./scripts/image-smoke.sh $(IMAGE)

KIND_CLUSTER ?= otelcol-fritzbox
# A dedicated kubeconfig keeps chart-test away from your current context.
KIND_KUBECONFIG ?= $(CURDIR)/bin/kind-kubeconfig

.PHONY: chart-lint
chart-lint: build ## Lint the Helm chart and validate every rendered collector config.
	helm lint --strict charts/otelcol-fritzbox
	ct lint --config ct.yaml --charts charts/otelcol-fritzbox
	./scripts/chart-render-test.sh

.PHONY: chart-test
chart-test: image ## Install the chart into a kind cluster and run helm tests.
	@mkdir -p $(dir $(KIND_KUBECONFIG))
	kind get clusters | grep -qx $(KIND_CLUSTER) || \
		kind create cluster --name $(KIND_CLUSTER) --kubeconfig $(KIND_KUBECONFIG) --wait 90s
	kind export kubeconfig --name $(KIND_CLUSTER) --kubeconfig $(KIND_KUBECONFIG)
	kind load docker-image $(IMAGE) --name $(KIND_CLUSTER)
	KUBECONFIG=$(KIND_KUBECONFIG) ct install --config ct.yaml --charts charts/otelcol-fritzbox \
		--helm-extra-set-args "--set image.repository=$(firstword $(subst :, ,$(IMAGE))) --set image.tag=$(lastword $(subst :, ,$(IMAGE))) --set image.pullPolicy=Never"

.PHONY: clean
clean: ## Remove build output.
	rm -rf bin dist
