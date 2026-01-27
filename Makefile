# Basecamp SDK Makefile
#
# Orchestrates both Smithy spec and Go SDK

.PHONY: all check clean help

# Default: run all checks
all: check

#------------------------------------------------------------------------------
# Smithy targets
#------------------------------------------------------------------------------

.PHONY: smithy-validate smithy-build smithy-check smithy-clean behavior-model behavior-model-check

# Validate Smithy spec
smithy-validate:
	@echo "==> Validating Smithy spec..."
	cd spec && smithy validate

# Build OpenAPI from Smithy (also regenerates behavior model)
smithy-build: behavior-model
	@echo "==> Building OpenAPI from Smithy..."
	cd spec && smithy build
	cp spec/build/smithy/openapi/openapi/Basecamp.openapi.json openapi.json
	@echo "Updated openapi.json"

# Check that openapi.json is up to date
smithy-check: smithy-validate
	@echo "==> Checking OpenAPI freshness..."
	@cd spec && smithy build
	@diff -q openapi.json spec/build/smithy/openapi/openapi/Basecamp.openapi.json > /dev/null 2>&1 || \
		(echo "ERROR: openapi.json is out of date. Run 'make smithy-build'" && exit 1)
	@echo "openapi.json is up to date"

# Clean Smithy build artifacts
smithy-clean:
	rm -rf spec/build

# Generate behavior model from Smithy spec
behavior-model:
	@echo "==> Generating behavior model..."
	@cd spec && smithy build
	./scripts/generate-behavior-model
	@echo "Updated behavior-model.json"

# Check that behavior-model.json is up to date
behavior-model-check:
	@echo "==> Checking behavior model freshness..."
	@./scripts/generate-behavior-model spec/build/smithy/source/model/model.json behavior-model.json.tmp
	@diff -q behavior-model.json behavior-model.json.tmp > /dev/null 2>&1 || \
		(rm -f behavior-model.json.tmp && echo "ERROR: behavior-model.json is out of date. Run 'make behavior-model'" && exit 1)
	@rm -f behavior-model.json.tmp
	@echo "behavior-model.json is up to date"

#------------------------------------------------------------------------------
# Go SDK targets (delegates to go/Makefile)
#------------------------------------------------------------------------------

.PHONY: go-test go-lint go-check go-clean

go-test:
	@$(MAKE) -C go test

go-lint:
	@$(MAKE) -C go lint

go-check:
	@$(MAKE) -C go check

go-clean:
	@$(MAKE) -C go clean

#------------------------------------------------------------------------------
# TypeScript SDK targets
#------------------------------------------------------------------------------

.PHONY: ts-generate ts-build ts-test ts-typecheck ts-check ts-clean

# Generate TypeScript types and metadata from OpenAPI
ts-generate:
	@echo "==> Generating TypeScript SDK..."
	cd typescript && npm run generate

# Build TypeScript SDK
ts-build:
	@echo "==> Building TypeScript SDK..."
	cd typescript && npm run build

# Run TypeScript tests
ts-test:
	@echo "==> Running TypeScript tests..."
	cd typescript && npm run test

# Run TypeScript type checking
ts-typecheck:
	@echo "==> Type checking TypeScript SDK..."
	cd typescript && npm run typecheck

# Run all TypeScript checks
ts-check: ts-typecheck ts-test
	@echo "==> TypeScript SDK checks passed"

# Clean TypeScript build artifacts
ts-clean:
	@echo "==> Cleaning TypeScript SDK..."
	rm -rf typescript/dist typescript/node_modules

#------------------------------------------------------------------------------
# Conformance Test targets
#------------------------------------------------------------------------------

.PHONY: conformance conformance-go conformance-build

# Build conformance test runner
conformance-build:
	@echo "==> Building conformance test runner..."
	cd conformance/runner/go && go build -o conformance-runner .

# Run Go conformance tests
conformance-go: conformance-build
	@echo "==> Running Go conformance tests..."
	cd conformance/runner/go && ./conformance-runner

# Run all conformance tests
conformance: conformance-go
	@echo "==> Conformance tests passed"

#------------------------------------------------------------------------------
# Combined targets
#------------------------------------------------------------------------------

# Run all checks (Smithy + Go + TypeScript + Behavior Model + Conformance)
check: smithy-check behavior-model-check go-check ts-check conformance
	@echo "==> All checks passed"

# Clean all build artifacts
clean: smithy-clean go-clean ts-clean

# Help
help:
	@echo "Basecamp SDK Makefile"
	@echo ""
	@echo "Smithy:"
	@echo "  smithy-validate  Validate Smithy spec syntax"
	@echo "  smithy-build     Build OpenAPI from Smithy (updates openapi.json)"
	@echo "  smithy-check     Verify openapi.json is up to date"
	@echo "  smithy-clean     Remove Smithy build artifacts"
	@echo ""
	@echo "Behavior Model:"
	@echo "  behavior-model       Generate behavior-model.json from Smithy spec"
	@echo "  behavior-model-check Verify behavior-model.json is up to date"
	@echo ""
	@echo "Go SDK:"
	@echo "  go-test          Run Go tests"
	@echo "  go-lint          Run Go linter"
	@echo "  go-check         Run all Go checks"
	@echo "  go-clean         Remove Go build artifacts"
	@echo ""
	@echo "TypeScript SDK:"
	@echo "  ts-generate      Generate types and metadata from OpenAPI"
	@echo "  ts-build         Build TypeScript SDK"
	@echo "  ts-test          Run TypeScript tests"
	@echo "  ts-typecheck     Run TypeScript type checking"
	@echo "  ts-check         Run all TypeScript checks"
	@echo "  ts-clean         Remove TypeScript build artifacts"
	@echo ""
	@echo "Conformance:"
	@echo "  conformance      Run all conformance tests"
	@echo "  conformance-go   Run Go conformance tests"
	@echo "  conformance-build Build conformance test runner"
	@echo ""
	@echo "Combined:"
	@echo "  check            Run all checks (Smithy + Go + TypeScript + Conformance)"
	@echo "  clean            Remove all build artifacts"
	@echo "  help             Show this help"
