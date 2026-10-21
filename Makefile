SHELL := /bin/sh

GOLANGCI_LINT_VERSION := v2.14.0
GOVULNCHECK_VERSION := v1.9.0
# Scan with the toolchain that builds releases (GO_VERSION in the workflows).
VULN_GO_VERSION := go1.27.3
GORELEASER_VERSION := v2.18.0
SYFT_VERSION := v1.51.1

.PHONY: build fmt fmt-check lint test test-e2e vuln test-container release-check release-snapshot verify

build:
	mkdir -p bin
	go build -o bin/tgsend ./cmd/tgsend

fmt:
	gofmt -w cmd internal

fmt-check:
	test -z "$$(gofmt -l cmd internal)"

lint:
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

test:
	go test -race ./...

test-e2e:
	go test -tags=e2e ./...

vuln:
	tool_dir=$$(mktemp -d); GOBIN=$$tool_dir GOTOOLCHAIN=$(VULN_GO_VERSION) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) && GOTOOLCHAIN=$(VULN_GO_VERSION) $$tool_dir/govulncheck ./...

test-container:
	sh test/container/smoke.sh

release-check:
	command -v goreleaser >/dev/null 2>&1
	test "$$(goreleaser --version | sed -n 's/^GitVersion:[[:space:]]*//p')" = "$(GORELEASER_VERSION:v%=%)"
	command -v syft >/dev/null 2>&1
	test "$$(syft version | sed -n 's/^Version:[[:space:]]*//p')" = "$(SYFT_VERSION:v%=%)"
	goreleaser check
	goreleaser release --snapshot --clean --skip=publish,docker
	sh scripts/check-release.sh dist

release-snapshot:
	goreleaser release --snapshot --clean

verify: build fmt-check lint test test-e2e vuln
