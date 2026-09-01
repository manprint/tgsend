SHELL := /bin/sh

GOLANGCI_LINT_VERSION := v2.13.2
GOVULNCHECK_VERSION := v1.1.4
VULN_GO_VERSION := go1.26.6
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
	project_dir=$$(mktemp -d); mkdir -p $$project_dir/project; tar --exclude='./.git' --exclude='./.serena' --exclude='./.tokensave' --exclude='./.tgsend' --exclude='./docs' --exclude='./bin' --exclude='./dist' --exclude='./coverage.out' -cf - . | tar -C $$project_dir/project -xf -; sed -i 's/^go 1\.27\.0$$/go 1.26.6/' $$project_dir/project/go.mod; tool_dir=$$(mktemp -d); GOBIN=$$tool_dir GOTOOLCHAIN=$(VULN_GO_VERSION) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); (cd $$project_dir/project && GOTOOLCHAIN=$(VULN_GO_VERSION) $$tool_dir/govulncheck ./...)

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
