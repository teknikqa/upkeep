BINARY   := upkeep
INSTALL  := $(HOME)/bin/upkeep
GO       := go
GOFLAGS  :=

# Default target
.PHONY: all
all: build

# Show available targets with descriptions
.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all             Build the binary (default)"
	@echo "  help            Show this help message"
	@echo "  setup           Set up development environment (git hooks)"
	@echo "  build           Build the binary in the current directory"
	@echo "  install         Build and install to ~/bin/upkeep"
	@echo "  test            Run all tests"
	@echo "  test-verbose    Run tests with verbose output"
	@echo "  lint            Run go vet + golangci-lint"
	@echo "  fmt             Format all Go source files"
	@echo "  tidy            Tidy go.mod / go.sum"
	@echo "  coverage        Run tests with coverage and generate HTML report"
	@echo "  clean           Remove built binary and coverage output"
	@echo "  ci              Run full CI pipeline locally (fmt, lint, test, build)"
	@echo "  check           Alias for ci"
	@echo "  govulncheck     Run govulncheck for known vulnerabilities"
	@echo "  release-dry-run GoReleaser dry-run (snapshot, no publish)"
	@echo "  dry-run         Scan only, no updates"
	@echo "  list            List all providers"

# Set up development environment (git hooks, etc.)
.PHONY: setup
setup:
	@ln -sf ../../.git-hooks/pre-commit .git/hooks/pre-commit
	@ln -sf ../../.git-hooks/commit-msg .git/hooks/commit-msg
	@echo "Git hooks installed (.git-hooks → .git/hooks)"

# Build the binary in the current directory
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

# Install the binary to ~/bin/upkeep
.PHONY: install
install: build
	@mkdir -p $(dir $(INSTALL))
	cp $(BINARY) $(INSTALL)
	@echo "Installed to $(INSTALL)"

# Run all tests
.PHONY: test
test:
	$(GO) test ./... -timeout 120s

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	$(GO) test ./... -v -timeout 120s

# Run go vet + golangci-lint
.PHONY: lint
lint:
	$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed — skipping (see https://golangci-lint.run/docs/install/)"; \
	fi

# Remove the built binary and coverage output
.PHONY: clean
clean:
	rm -f $(BINARY) coverage.out coverage.html

# Tidy go.mod / go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Run tests with coverage and generate HTML report
.PHONY: coverage
coverage:
	$(GO) test ./... -timeout 120s -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Format all Go source files
.PHONY: fmt
fmt:
	$(GO) fmt ./...

# GoReleaser dry-run (snapshot, no publish)
.PHONY: release-dry-run
release-dry-run:
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed — skipping (see https://goreleaser.com/install/)"; \
	fi

# Run the full CI pipeline locally: fmt, lint, test, build
.PHONY: ci
ci: fmt lint test build

# Run all quality checks: fmt, lint, test, build
# Equivalent to ci — use this as a pre-commit / pre-PR sanity check.
.PHONY: check
check: fmt lint test build
	@echo "All checks passed."

# Run govulncheck for known vulnerabilities in dependencies.
# Install with: go install golang.org/x/vuln/cmd/govulncheck@latest
.PHONY: govulncheck
govulncheck:
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed — skipping (run: go install golang.org/x/vuln/cmd/govulncheck@latest)"; \
	fi

# Dry-run: scan only, no updates
.PHONY: dry-run
dry-run: build
	./$(BINARY) --dry-run

# List all providers
.PHONY: list
list: build
	./$(BINARY) --list
