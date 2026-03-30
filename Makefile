BINARY   := upkeep
INSTALL  := $(HOME)/bin/upkeep
GO       := go
GOFLAGS  :=

# Default target
.PHONY: all
all: build

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

# Run go vet + staticcheck (install staticcheck if missing)
.PHONY: lint
lint:
	$(GO) vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed — skipping (run: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# Remove the built binary
.PHONY: clean
clean:
	rm -f $(BINARY)

# Tidy go.mod / go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Dry-run: scan only, no updates
.PHONY: dry-run
dry-run: build
	./$(BINARY) --dry-run

# List all providers
.PHONY: list
list: build
	./$(BINARY) --list
