# Basecamp Go SDK Makefile

# Go parameters
GOCMD := go
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOMOD := $(GOCMD) mod

# Default target
.PHONY: all
all: check

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run go vet
.PHONY: vet
vet:
	$(GOVET) ./...

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) -s -w .

# Check formatting (for CI)
.PHONY: fmt-check
fmt-check:
	@test -z "$$($(GOFMT) -s -l . | tee /dev/stderr)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run ./...

# Run vulnerability scanner
.PHONY: vuln
vuln:
	@echo "Running govulncheck..."
	$(GOCMD) run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Verify dependencies
.PHONY: verify
verify:
	$(GOMOD) verify

# Clean build artifacts
.PHONY: clean
clean:
	rm -f coverage.out coverage.html

# Run all checks (local development gate)
.PHONY: check
check: fmt-check vet lint test

# CI target (same as check, explicit for clarity)
.PHONY: ci
ci: check vuln

# Release check - verify package is ready for release
.PHONY: release-check
release-check: check vuln
	@echo "Checking go.mod is tidy..."
	$(GOMOD) tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy. Run 'make tidy' and commit." && exit 1)
	@echo "Release checks passed."

# Show help
.PHONY: help
help:
	@echo "Basecamp Go SDK Makefile targets:"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run all tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  vet            Run go vet"
	@echo "  fmt            Format code"
	@echo "  fmt-check      Check code formatting"
	@echo "  lint           Run golangci-lint"
	@echo "  vuln           Run govulncheck for dependency vulnerabilities"
	@echo ""
	@echo "Dependencies:"
	@echo "  tidy           Tidy go.mod dependencies"
	@echo "  verify         Verify dependencies"
	@echo ""
	@echo "CI/Release:"
	@echo "  check          Run all checks (fmt-check, vet, lint, test)"
	@echo "  ci             Run CI checks (check + vuln)"
	@echo "  release-check  Verify package is ready for release"
	@echo ""
	@echo "Other:"
	@echo "  clean          Remove generated artifacts"
	@echo "  help           Show this help"
