# Basecamp Go SDK Makefile

# Go parameters
GOCMD := go
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOMOD := $(GOCMD) mod
GOGEN := $(GOCMD) generate

# Default target
.PHONY: all
all: check

# Generate code from OpenAPI spec
.PHONY: generate
generate:
	@echo "Generating client code from OpenAPI spec..."
	$(GOGEN) ./...

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

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -run=^$$ -bench=. -benchmem ./pkg/basecamp/

# Run benchmarks with CPU profile for PGO
.PHONY: bench-pgo
bench-pgo:
	@echo "Running benchmarks to generate CPU profile for PGO..."
	$(GOTEST) -run=^$$ -bench=. -benchtime=5s -cpuprofile=pkg/basecamp/default.pgo ./pkg/basecamp/
	@echo "Profile saved to pkg/basecamp/default.pgo"
	@echo "Commit this file to enable PGO in builds"

# Run benchmarks and compare with baseline (requires benchstat)
.PHONY: bench-compare
bench-compare:
	@if [ ! -f bench-baseline.txt ]; then \
		echo "No baseline found. Run 'make bench-baseline' first."; \
		exit 1; \
	fi
	$(GOTEST) -run=^$$ -bench=. -benchmem -count=6 ./pkg/basecamp/ > bench-current.txt
	benchstat bench-baseline.txt bench-current.txt

# Save benchmark baseline for future comparisons
.PHONY: bench-baseline
bench-baseline:
	$(GOTEST) -run=^$$ -bench=. -benchmem -count=6 ./pkg/basecamp/ > bench-baseline.txt
	@echo "Baseline saved to bench-baseline.txt"

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

# Clean build artifacts (preserves committed pkg/basecamp/default.pgo)
.PHONY: clean
clean:
	rm -f coverage.out coverage.html
	rm -f bench-baseline.txt bench-current.txt
	rm -f pkg/generated/*.gen.go

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
	@echo "Code Generation:"
	@echo "  generate       Generate client code from OpenAPI spec using oapi-codegen"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run all tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo ""
	@echo "Benchmarks:"
	@echo "  bench          Run all benchmarks"
	@echo "  bench-pgo      Run benchmarks and generate PGO profile (default.pgo)"
	@echo "  bench-baseline Save benchmark baseline for comparisons"
	@echo "  bench-compare  Compare current benchmarks against baseline"
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
