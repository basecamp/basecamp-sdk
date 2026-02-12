# Basecamp SDK Makefile
#
# Orchestrates both Smithy spec and Go SDK

.PHONY: all check clean help provenance-sync provenance-check sync-status

# Default: run all checks
all: check

#------------------------------------------------------------------------------
# Smithy targets
#------------------------------------------------------------------------------

.PHONY: smithy-validate smithy-build smithy-check smithy-clean smithy-mapper behavior-model behavior-model-check

# Validate Smithy spec
smithy-validate:
	@echo "==> Validating Smithy spec..."
	cd spec && smithy validate

# Build the custom Smithy OpenAPI mapper
smithy-mapper:
	@echo "==> Building Smithy OpenAPI mapper..."
	cd spec/smithy-bare-arrays && ./gradlew publishToMavenLocal --quiet

# Build OpenAPI from Smithy (also regenerates behavior model)
smithy-build: behavior-model smithy-mapper
	@echo "==> Building OpenAPI from Smithy..."
	cd spec && smithy build
	cp spec/build/smithy/openapi/openapi/Basecamp.openapi.json openapi.json
	@echo "==> Post-processing OpenAPI for Go types..."
	./scripts/enhance-openapi-go-types.sh
	@echo "Updated openapi.json"

# Check that openapi.json is up to date
smithy-check: smithy-validate smithy-mapper
	@echo "==> Checking OpenAPI freshness..."
	@cd spec && smithy build
	@TMPFILE=$$(mktemp) && \
		cp spec/build/smithy/openapi/openapi/Basecamp.openapi.json "$$TMPFILE" && \
		./scripts/enhance-openapi-go-types.sh "$$TMPFILE" "$$TMPFILE" > /dev/null 2>&1 && \
		(diff -q openapi.json "$$TMPFILE" > /dev/null 2>&1 || \
			(rm -f "$$TMPFILE" && echo "ERROR: openapi.json is out of date. Run 'make smithy-build'" && exit 1)) && \
		rm -f "$$TMPFILE"
	@echo "openapi.json is up to date"

# Clean Smithy build artifacts
smithy-clean:
	rm -rf spec/build spec/smithy-bare-arrays/build spec/smithy-bare-arrays/.gradle

# Generate behavior model from Smithy spec
behavior-model: smithy-mapper
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

.PHONY: url-routes url-routes-check

# Generate url-routes.json from OpenAPI spec
url-routes:
	@echo "==> Generating URL routes..."
	./scripts/generate-url-routes
	@echo "Updated go/pkg/basecamp/url-routes.json"

# Check that url-routes.json is up to date
url-routes-check:
	@echo "==> Checking URL routes freshness..."
	@./scripts/generate-url-routes openapi.json go/pkg/basecamp/url-routes.json.tmp
	@diff -q go/pkg/basecamp/url-routes.json go/pkg/basecamp/url-routes.json.tmp > /dev/null 2>&1 || \
		(rm -f go/pkg/basecamp/url-routes.json.tmp && echo "ERROR: url-routes.json is out of date. Run 'make url-routes'" && exit 1)
	@rm -f go/pkg/basecamp/url-routes.json.tmp
	@echo "url-routes.json is up to date"

#------------------------------------------------------------------------------
# API Provenance targets
#------------------------------------------------------------------------------

# Copy api-provenance.json into Go package for go:embed
provenance-sync:
	@cp spec/api-provenance.json go/pkg/basecamp/api-provenance.json

# Check that the Go embedded provenance matches the canonical spec file
provenance-check:
	@diff -q spec/api-provenance.json go/pkg/basecamp/api-provenance.json > /dev/null 2>&1 || \
		(echo "ERROR: go/pkg/basecamp/api-provenance.json is out of date. Run 'make provenance-sync'" && exit 1)
	@echo "api-provenance.json is up to date"

# Show upstream changes since last spec sync (queries GitHub via gh CLI).
BC3_API_REPO ?= basecamp/bc3-api
BC3_REPO     ?= basecamp/bc3

sync-status:
	@command -v gh > /dev/null 2>&1 || { echo "ERROR: gh CLI not found. Install: https://cli.github.com"; exit 1; }
	@gh auth status > /dev/null 2>&1 || { echo "ERROR: gh not authenticated. Run: gh auth login"; exit 1; }
	@REV=$$(jq -r '.bc3_api.revision // empty' spec/api-provenance.json); \
	if [ -z "$$REV" ]; then \
		echo "==> bc3-api: no baseline revision set"; \
	else \
		echo "==> bc3-api changes since last sync ($$(echo $$REV | cut -c1-7)):"; \
		gh api "repos/$(BC3_API_REPO)/compare/$$REV...HEAD" \
			--jq '[.files[] | select(.filename | startswith("sections/"))] | if length == 0 then "  (no changes in sections/)" else .[] | "  " + .status[:1] + " " + .filename end'; \
	fi
	@echo ""
	@REV=$$(jq -r '.bc3.revision // empty' spec/api-provenance.json); \
	if [ -z "$$REV" ]; then \
		echo "==> bc3: no baseline revision set"; \
	else \
		echo "==> bc3 API changes since last sync ($$(echo $$REV | cut -c1-7)):"; \
		gh api "repos/$(BC3_REPO)/compare/$$REV...HEAD" \
			--jq '[.files[] | select(.filename | startswith("app/controllers/"))] | if length == 0 then "  (no changes in app/controllers/)" else .[] | "  " + .status[:1] + " " + .filename end'; \
	fi

#------------------------------------------------------------------------------
# Go SDK targets (delegates to go/Makefile)
#------------------------------------------------------------------------------

.PHONY: go-test go-lint go-check go-clean go-check-drift

go-test:
	@$(MAKE) -C go test

go-lint:
	@$(MAKE) -C go lint

go-check:
	@$(MAKE) -C go check

go-clean:
	@$(MAKE) -C go clean

# Check for drift between generated client and service layer
go-check-drift:
	@echo "==> Checking service layer drift..."
	@./scripts/check-service-drift.sh

#------------------------------------------------------------------------------
# TypeScript SDK targets
#------------------------------------------------------------------------------

.PHONY: ts-generate ts-generate-services ts-build ts-test ts-typecheck ts-check ts-clean

# Generate TypeScript types and metadata from OpenAPI
ts-generate:
	@echo "==> Generating TypeScript SDK..."
	cd typescript && npm run generate

# Generate TypeScript services from OpenAPI
ts-generate-services:
	@echo "==> Generating TypeScript services..."
	cd typescript && npx tsx scripts/generate-services.ts

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
# Ruby SDK targets
#------------------------------------------------------------------------------

.PHONY: rb-generate rb-generate-services rb-build rb-test rb-check rb-doc rb-clean

# Generate Ruby types and metadata from OpenAPI
rb-generate:
	@echo "==> Generating Ruby SDK types and metadata..."
	cd ruby && ruby scripts/generate-metadata.rb > lib/basecamp/generated/metadata.json
	cd ruby && ruby scripts/generate-types.rb > lib/basecamp/generated/types.rb
	@echo "Generated lib/basecamp/generated/metadata.json and types.rb"

# Generate Ruby services from OpenAPI
rb-generate-services:
	@echo "==> Generating Ruby services..."
	cd ruby && ruby scripts/generate-services.rb

# Build Ruby SDK (install deps)
rb-build:
	@echo "==> Building Ruby SDK..."
	cd ruby && bundle install

# Run Ruby tests
rb-test:
	@echo "==> Running Ruby tests..."
	cd ruby && bundle exec rake test

# Run all Ruby checks
rb-check: rb-test
	@echo "==> Running Ruby linter..."
	cd ruby && bundle exec rubocop
	@echo "==> Ruby SDK checks passed"

# Generate Ruby documentation
rb-doc:
	@echo "==> Generating Ruby documentation..."
	cd ruby && bundle exec rake doc
	@echo "Documentation generated in ruby/doc/"

# Clean Ruby build artifacts
rb-clean:
	@echo "==> Cleaning Ruby SDK..."
	rm -rf ruby/.bundle ruby/vendor ruby/doc ruby/coverage

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
# Swift SDK targets (delegates to swift/Makefile)
#------------------------------------------------------------------------------

.PHONY: swift-build swift-test swift-check swift-clean

# Build Swift SDK
swift-build:
	@$(MAKE) -C swift build

# Run Swift tests
swift-test:
	@$(MAKE) -C swift test

# Run all Swift checks
swift-check:
	@$(MAKE) -C swift check

# Clean Swift build artifacts
swift-clean:
	@$(MAKE) -C swift clean

#------------------------------------------------------------------------------
# Combined targets
#------------------------------------------------------------------------------

# Run all checks (Smithy + Go + TypeScript + Ruby + Swift + Behavior Model + Conformance + Provenance)
check: smithy-check behavior-model-check provenance-check go-check-drift go-check ts-check rb-check swift-check conformance
	@echo "==> All checks passed"

# Clean all build artifacts
clean: smithy-clean go-clean ts-clean rb-clean swift-clean

# Help
help:
	@echo "Basecamp SDK Makefile"
	@echo ""
	@echo "Smithy:"
	@echo "  smithy-validate  Validate Smithy spec syntax"
	@echo "  smithy-mapper    Build custom OpenAPI mapper JAR"
	@echo "  smithy-build     Build OpenAPI from Smithy (updates openapi.json)"
	@echo "  smithy-check     Verify openapi.json is up to date"
	@echo "  smithy-clean     Remove Smithy build artifacts"
	@echo ""
	@echo "Behavior Model:"
	@echo "  behavior-model       Generate behavior-model.json from Smithy spec"
	@echo "  behavior-model-check Verify behavior-model.json is up to date"
	@echo ""
	@echo "URL Routes:"
	@echo "  url-routes           Generate url-routes.json from OpenAPI spec"
	@echo "  url-routes-check     Verify url-routes.json is up to date"
	@echo ""
	@echo "Go SDK:"
	@echo "  go-test          Run Go tests"
	@echo "  go-lint          Run Go linter"
	@echo "  go-check         Run all Go checks"
	@echo "  go-check-drift   Check service layer drift vs generated client"
	@echo "  go-clean         Remove Go build artifacts"
	@echo ""
	@echo "TypeScript SDK:"
	@echo "  ts-generate           Generate types and metadata from OpenAPI"
	@echo "  ts-generate-services  Generate service classes from OpenAPI"
	@echo "  ts-build              Build TypeScript SDK"
	@echo "  ts-test               Run TypeScript tests"
	@echo "  ts-typecheck          Run TypeScript type checking"
		@echo "  ts-check              Run all TypeScript checks"
	@echo "  ts-clean              Remove TypeScript build artifacts"
	@echo ""
	@echo "Swift SDK:"
	@echo "  swift-build      Build Swift SDK"
	@echo "  swift-test       Run Swift tests"
	@echo "  swift-check      Run all Swift checks"
	@echo "  swift-clean      Remove Swift build artifacts"
	@echo ""
	@echo "Conformance:"
	@echo "  conformance      Run all conformance tests"
	@echo "  conformance-go   Run Go conformance tests"
	@echo "  conformance-build Build conformance test runner"
	@echo ""
	@echo "Ruby SDK:"
	@echo "  rb-generate          Generate types and metadata from OpenAPI"
	@echo "  rb-generate-services Generate service classes from OpenAPI"
	@echo "  rb-build             Build Ruby SDK (install deps)"
	@echo "  rb-test              Run Ruby tests (with coverage)"
	@echo "  rb-check             Run all Ruby checks"
	@echo "  rb-doc               Generate YARD documentation"
	@echo "  rb-clean             Remove Ruby build artifacts"
	@echo ""
	@echo "Provenance:"
	@echo "  provenance-sync  Copy provenance into Go package for go:embed"
	@echo "  provenance-check Verify Go embedded provenance is up to date"
	@echo "  sync-status      Show upstream changes since last spec sync"
	@echo ""
	@echo "Combined:"
	@echo "  check            Run all checks (Smithy + Go + TypeScript + Ruby + Swift + Conformance + Provenance)"
	@echo "  clean            Remove all build artifacts"
	@echo "  help             Show this help"
