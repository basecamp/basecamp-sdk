# Basecamp SDK Makefile
#
# Orchestrates both Smithy spec and Go SDK

.PHONY: all check clean help setup tools provenance-sync provenance-check sync-status bump sync-spec-version sync-spec-version-check sync-api-version sync-api-version-check doc-constants-check release

# Default: run all checks
all: check

#------------------------------------------------------------------------------
# Smithy targets
#------------------------------------------------------------------------------

.PHONY: smithy-validate smithy-build smithy-check smithy-clean smithy-mapper behavior-model behavior-model-check

# Validate Smithy spec
smithy-validate: smithy-mapper
	@echo "==> Validating Smithy spec..."
	cd spec && smithy validate

# Build the custom Smithy OpenAPI mapper
smithy-mapper:
	@echo "==> Building Smithy OpenAPI mapper..."
	cd spec/smithy-bare-arrays && ./gradlew publishToMavenLocal --quiet

# Build OpenAPI from Smithy (also regenerates behavior model + syncs API version)
smithy-build: behavior-model smithy-mapper
	@$(MAKE) sync-spec-version
	@echo "==> Building OpenAPI from Smithy..."
	cd spec && smithy build
	cp spec/build/smithy/openapi/openapi/Basecamp.openapi.json openapi.json
	@echo "==> Post-processing OpenAPI for Go types..."
	./scripts/enhance-openapi-go-types.sh
	@echo "Updated openapi.json"
	@$(MAKE) sync-api-version

# Check that openapi.json is up to date
smithy-check: smithy-validate smithy-mapper
	@$(MAKE) sync-spec-version-check
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

.PHONY: bc3-routes bc3-route-parity test-bc3-route-parity bc3-routes-check

# Regenerate spec/bc3-routes.json — the vendored table of routes bc3 actually
# serves, extracted from its API docs at the pinned provenance revision.
# Needs a bc3 checkout; reads through `git show <pin>:` so the output is a pure
# function of the pin, not of whatever bc3's working tree happens to hold.
bc3-routes:
	@echo "==> Extracting bc3 routes at the pinned revision..."
	@./scripts/generate-bc3-routes
	@echo "Updated spec/bc3-routes.json"

# Compare the SDK's declared routes against bc3's, both directions. Offline:
# reads only the vendored table, so it runs in `make check` and in every CI job
# with no bc3 checkout and no secret. A gate that skips when its input is absent
# cannot prevent anything, and skipping is how two 404ing routes shipped.
bc3-route-parity:
	@echo "==> Checking bc3 route parity..."
	@./scripts/check-bc3-route-parity

# Drive that gate from outside with adversarial allowlists. The live run only
# exercises the VALID file, so nothing proves `modeled_as` — the one disposition
# asserting something is DONE, and the same class of claim that shipped
# ListForwards and RepositionTodolistGroup as 404s — rejects anything. Each case
# substitutes a REAL operationId for a REAL route via BC3_ROUTE_ALLOWLIST; the
# route tables are never faked.
test-bc3-route-parity:
	@ruby ./scripts/test-check-bc3-route-parity.rb

# Freshness gate for the vendored table: regenerate at the current pin and diff.
#
# Needs BC3_REPO_PATH, so it is NOT in `make check` and NOT in CI — no workflow
# checks out bc3 today, and inventing a secret for one is a provisioning
# decision, not a code change. Run it by hand when you repin. Tracked in #589.
#
# What still holds without it: bc3-route-parity verifies the table's recorded
# revision equals the provenance pin, and verifies a SHA-256 fingerprint of this
# generator plus the normalizer, so a changed extractor with a stale table fails
# offline. What it cannot catch is a hand-edited `source.revision` that matches
# the pin without regeneration — that needs bc3, hence #589.
#
# Generates into a real temp dir rather than a sibling .tmp file: an in-tree temp
# path races under `make -j` and can miss extra files.
bc3-routes-check:
	@echo "==> Checking bc3 route table freshness..."
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
		./scripts/generate-bc3-routes "$$tmp/bc3-routes.json" && \
		diff -q spec/bc3-routes.json "$$tmp/bc3-routes.json" > /dev/null 2>&1 || \
		{ echo "ERROR: spec/bc3-routes.json is out of date for the current pin. Run 'make bc3-routes'"; exit 1; }
	@echo "spec/bc3-routes.json is up to date"

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
BC3_REPO ?= basecamp/bc3

sync-status:
	@command -v gh > /dev/null 2>&1 || { echo "ERROR: gh CLI not found. Install: https://cli.github.com"; exit 1; }
	@command -v jq > /dev/null 2>&1 || { echo "ERROR: jq not found. Install: https://jqlang.github.io/jq/"; exit 1; }
	@gh auth status > /dev/null 2>&1 || { echo "ERROR: gh not authenticated. Run: gh auth login"; exit 1; }
	@BC3_REPO="$(BC3_REPO)" ./scripts/report-bc3-drift.sh \
		"$$(jq -r '.bc3.revision // empty' spec/api-provenance.json)" \
		"$$(jq -r '.bc3.branch // "master"' spec/api-provenance.json)" \
		"primary"
	@for COMPAT_KEY in $$(jq -r '.compatibility // {} | keys[]' spec/api-provenance.json); do \
		echo ""; \
		BC3_REPO="$(BC3_REPO)" ./scripts/report-bc3-drift.sh \
			"$$(jq -r --arg k "$$COMPAT_KEY" '.compatibility[$$k].revision // empty' spec/api-provenance.json)" \
			"$$(jq -r --arg k "$$COMPAT_KEY" '.compatibility[$$k].branch // "master"' spec/api-provenance.json)" \
			"compat"; \
	done

#------------------------------------------------------------------------------
# Version management
#------------------------------------------------------------------------------

# Bump SDK version across all languages: make bump VERSION=x.y.z
bump:
ifndef VERSION
	$(error VERSION is required. Usage: make bump VERSION=x.y.z)
endif
	@./scripts/bump-version.sh $(VERSION)

# Tag and push a global release: make release VERSION=x.y.z
release:
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=x.y.z)
endif
	@echo "Releasing v$(VERSION)..."
	@# Verify version constants match
	@grep -qF 'Version = "$(VERSION)"' go/pkg/basecamp/version.go || \
		{ echo "ERROR: Go version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF '"version": "$(VERSION)"' typescript/package.json || \
		{ echo "ERROR: TypeScript version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF 'VERSION = "$(VERSION)"' ruby/lib/basecamp/version.rb || \
		{ echo "ERROR: Ruby version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF 'const val VERSION = "$(VERSION)"' kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/BasecampConfig.kt || \
		{ echo "ERROR: Kotlin version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF 'version = "$(VERSION)"' kotlin/sdk/build.gradle.kts || \
		{ echo "ERROR: Kotlin Gradle project version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF 'public static let version = "$(VERSION)"' swift/Sources/Basecamp/BasecampConfig.swift || \
		{ echo "ERROR: Swift version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@grep -qF 'VERSION = "$(VERSION)"' python/src/basecamp/_version.py || \
		{ echo "ERROR: Python version does not match $(VERSION). Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@# Verify lockfiles are frozen against their manifests
	@cd python && uv lock --check || \
		{ echo "ERROR: python/uv.lock is stale. Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@test "$$(jq -r '.packages["../../../typescript"].version' conformance/runner/typescript/package-lock.json)" = "$(VERSION)" || \
		{ echo "ERROR: conformance/runner/typescript/package-lock.json records a stale SDK version. Run 'make bump VERSION=$(VERSION)' first."; exit 1; }
	@git diff --quiet && git diff --cached --quiet || \
		{ echo "ERROR: Working tree has uncommitted changes. Commit first."; exit 1; }
	@# Verify we're on main — release tags must be on the default branch
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$BRANCH" != "main" ]; then \
		echo "ERROR: Must be on main branch to release (currently on $$BRANCH)."; exit 1; \
	fi
	@# Push main first — release workflows verify the tag commit is reachable from origin/main
	git push origin main
	git tag "v$(VERSION)"
	git push origin "v$(VERSION)"
	@echo "Pushed v$(VERSION) — all SDK release workflows will trigger."

# Sync Smithy service version from spec/api-provenance.json
sync-spec-version:
	@./scripts/sync-spec-version.sh

# Check that the Smithy service version matches spec/api-provenance.json
sync-spec-version-check:
	@echo "==> Checking Smithy service version freshness..."
	@command -v jq > /dev/null 2>&1 || { echo "ERROR: jq not found. Install jq to run sync-spec-version-check (used by 'make check')."; exit 1; }
	@BC3_DATE=$$(jq -r '.bc3.date' spec/api-provenance.json); \
		SMITHY_VER=$$(sed -n 's/^  version: "\(.*\)"/\1/p' spec/basecamp.smithy | head -1); \
		if [ -z "$$BC3_DATE" ] || [ "$$BC3_DATE" = "null" ]; then echo "ERROR: Could not read bc3.date from spec/api-provenance.json"; exit 1; fi; \
		if [ "$$SMITHY_VER" != "$$BC3_DATE" ]; then echo "ERROR: Smithy service version is out of date. Run 'make sync-spec-version'"; exit 1; fi
	@echo "Smithy service version is up to date"

# Sync API_VERSION constants from openapi.json info.version
sync-api-version:
	@./scripts/sync-api-version.sh

# Check that API_VERSION constants match openapi.json info.version
sync-api-version-check:
	@echo "==> Checking API version freshness..."
	@command -v jq > /dev/null 2>&1 || { echo "ERROR: jq not found. Install jq to run sync-api-version-check (used by 'make check')."; exit 1; }
	@API_VER=$$(jq -r '.info.version' openapi.json); \
	ok=true; \
	grep -q "const APIVersion = \"$$API_VER\"" go/pkg/basecamp/version.go || ok=false; \
	grep -q "export const API_VERSION = \"$$API_VER\"" typescript/src/client.ts || ok=false; \
	grep -q "API_VERSION = \"$$API_VER\"" ruby/lib/basecamp/version.rb || ok=false; \
	grep -q "const val API_VERSION = \"$$API_VER\"" kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/BasecampConfig.kt || ok=false; \
	grep -q "public static let apiVersion = \"$$API_VER\"" swift/Sources/Basecamp/BasecampConfig.swift || ok=false; \
	grep -q "API_VERSION = \"$$API_VER\"" python/src/basecamp/_version.py || ok=false; \
	if [ "$$ok" = false ]; then echo "ERROR: API_VERSION constants are out of date. Run 'make sync-api-version'"; exit 1; fi
	@echo "API version constants are up to date"

# Check the constants restated in prose (API_VERSION, bc3 provenance pin,
# SPEC §19's assertion-type table) against their machine-readable sources.
# Only HTML-comment-marked spans are checked; spec/doc-constants.json commits
# the exact per-file marker count so neither deleting a marker nor quietly
# adding an unrecorded one can silence the gate.
# The live run only ever proves the gate can say yes, so the self-test follows:
# it crafts each failure mode and asserts the gate rejects it.
doc-constants-check:
	@echo "==> Checking documentation constants..."
	@./scripts/check-doc-constants.sh
	@ruby ./scripts/test-doc-constants.rb

#------------------------------------------------------------------------------
# Go SDK targets (delegates to go/Makefile)
#------------------------------------------------------------------------------

.PHONY: go-test go-lint go-check go-clean go-check-drift go-check-wrapper-drift go-check-generated-drift

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

# Check that the committed generated Go client is current. Regenerates
# client.gen.go (oapi-codegen + normalization) into a temp location and diffs;
# non-mutative and safe under `make -j`. Distinct from go-check-drift
# (operation-level coverage) and go-check-wrapper-drift (field-level): this is
# output freshness of the generated file itself.
go-check-generated-drift:
	@echo "==> Checking generated Go client drift..."
	@./scripts/check-go-generated-drift.sh

# Check for field-level drift between generated structs and hand-written
# wrappers in go/pkg/basecamp/. Sibling of go-check-drift; that check is
# operation-level, this one is field-level.
go-check-wrapper-drift:
	@echo "==> Running wrapper-drift checker tests..."
	@go test ./scripts/check-wrapper-drift/
	@echo "==> Checking wrapper field-level drift..."
	@go run ./scripts/check-wrapper-drift/

.PHONY: auth-routable-check

# Check that hop-2-only primitives are not called outside the authenticated
# download path. Guards against regressing the @basecampAuthRoutableUrl contract.
auth-routable-check:
	@echo "==> Checking auth-routable consumer invariants..."
	@./scripts/check-auth-routable-consumers.sh

#------------------------------------------------------------------------------
# TypeScript SDK targets
#------------------------------------------------------------------------------

.PHONY: ts-install ts-generate ts-generate-services ts-build ts-test ts-typecheck ts-check ts-check-drift ts-clean

TS_NODE_STAMP := typescript/node_modules/.install-stamp

$(TS_NODE_STAMP): typescript/package-lock.json typescript/package.json
	@echo "==> Installing TypeScript dependencies..."
	cd typescript && npm ci
	@touch $(TS_NODE_STAMP)

ts-install: $(TS_NODE_STAMP)

ts-generate: ts-install
ts-generate-services: ts-install
ts-build: ts-install
ts-test: ts-install
ts-typecheck: ts-install

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

# Check that committed generated TypeScript artifacts are current. Regenerates
# the whole src/generated/ tree (stripped OpenAPI, schema, metadata,
# path-mapping, services) into a temp project and diffs; non-mutative and safe
# under `make -j`.
ts-check-drift: ts-install
	@echo "==> Checking TypeScript generated code drift..."
	@./scripts/check-typescript-service-drift.sh

# Run all TypeScript checks
ts-check: ts-check-drift ts-typecheck ts-test
	@echo "==> TypeScript SDK checks passed"

# Clean TypeScript build artifacts
ts-clean:
	@echo "==> Cleaning TypeScript SDK..."
	rm -rf typescript/dist typescript/node_modules

#------------------------------------------------------------------------------
# Ruby SDK targets
#------------------------------------------------------------------------------

.PHONY: rb-generate rb-generate-services rb-build rb-test rb-check rb-check-drift rb-doc rb-clean

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
RB_STAMP := ruby/.bundle/.install-stamp

$(RB_STAMP): ruby/Gemfile ruby/Gemfile.lock ruby/basecamp-sdk.gemspec
	@echo "==> Installing Ruby dependencies..."
	cd ruby && bundle install
	@mkdir -p $(dir $(RB_STAMP))
	@touch $(RB_STAMP)

rb-build: $(RB_STAMP)

# Run Ruby tests
rb-test: rb-build
	@echo "==> Running Ruby tests..."
	cd ruby && bundle exec rake test

# Check that committed generated Ruby artifacts are current. Regenerates
# metadata.json, types.rb, and the service files and diffs; non-mutative. The
# metadata/types diff canonicalizes the embedded generation timestamp.
rb-check-drift:
	@echo "==> Checking Ruby generated code drift..."
	@./scripts/check-ruby-service-drift.sh

# Run all Ruby checks
rb-check: rb-check-drift rb-test
	@echo "==> Running Ruby linter..."
	cd ruby && bundle exec rubocop
	@echo "==> Ruby SDK checks passed"

# Generate Ruby documentation
rb-doc: rb-build
	@echo "==> Generating Ruby documentation..."
	cd ruby && bundle exec rake doc
	@echo "Documentation generated in ruby/doc/"

# Clean Ruby build artifacts
rb-clean:
	@echo "==> Cleaning Ruby SDK..."
	rm -rf ruby/.bundle ruby/vendor ruby/doc ruby/coverage

#------------------------------------------------------------------------------
# Python SDK targets
#------------------------------------------------------------------------------

.PHONY: py-generate py-generate-services py-build py-test py-typecheck py-check py-check-drift py-clean

py-generate: py-generate-services
	cd python && uv run python scripts/generate_types.py
	cd python && uv run python scripts/generate_metadata.py
	cd python && uv run ruff format src/basecamp/generated/

py-generate-services:
	cd python && uv run python scripts/generate_services.py

py-build:
	cd python && uv sync --dev

py-test:
	cd python && uv run pytest --cov --cov-report=term-missing --cov-fail-under=60

py-typecheck:
	cd python && uv run mypy src/basecamp/ --ignore-missing-imports

py-check: py-check-drift py-test py-typecheck
	cd python && uv run ruff check src/ tests/
	cd python && uv run ruff format --check src/ tests/

py-check-drift:
	@echo "==> Checking Python service drift..."
	@./scripts/check-python-service-drift.sh

py-clean:
	rm -rf python/dist python/.pytest_cache python/src/*.egg-info python/.venv

#------------------------------------------------------------------------------
# Conformance Test targets
#------------------------------------------------------------------------------

.PHONY: conformance conformance-runner-tests conformance-runner-tests-go conformance-runner-tests-python conformance-runner-tests-ruby conformance-runner-tests-kotlin conformance-runner-tests-swift check-runner-test-reachability conformance-go conformance-go-replay conformance-kotlin conformance-kotlin-replay conformance-typescript conformance-typescript-live conformance-ruby conformance-ruby-replay conformance-python conformance-python-replay conformance-swift conformance-build conformance-live conformance-canary oauth-fixtures-check oauth-token-fixtures-check conformance-fixtures-check

# NOTE: conformance-swift and conformance-runner-tests-swift are defined in the
# Swift SDK targets section below — their IS_MACOS conditional must parse after
# that variable is defined.

# Pinned validator for the data-only OAuth discovery fixtures. Run via uvx so the
# version is reproducible without a global install; the schema is separate from
# the operation-dispatch conformance/schema.json (unusable for OAuth data).
CHECK_JSONSCHEMA_VERSION := 0.35.0

# Validate OAuth resource-first discovery fixtures against their JSON Schema.
oauth-fixtures-check:
	@echo "==> Validating OAuth discovery fixtures..."
	uvx --from 'check-jsonschema==$(CHECK_JSONSCHEMA_VERSION)' check-jsonschema \
		--schemafile conformance/oauth/schema.json conformance/oauth/fixtures/*.json

# Validate OAuth token wire-behavior fixtures (RFC 8707 resource echo/decode)
# against their JSON Schema. A separate family from conformance/oauth/ — that
# schema is discovery-only and every discovery harness globs its whole fixtures
# directory, so token cases must live here.
oauth-token-fixtures-check:
	@echo "==> Validating OAuth token fixtures..."
	uvx --from 'check-jsonschema==$(CHECK_JSONSCHEMA_VERSION)' check-jsonschema \
		--schemafile conformance/oauth-token/schema.json conformance/oauth-token/fixtures/*.json

# Validate every conformance/tests/*.json entry against conformance/schema.json.
# This is the AUTHORITATIVE enforcement of the per-case schema — including the
# mockResponses oneOf (exactly one of status or networkError:true). The runners
# don't schema-validate fixtures, so without this a malformed fixture (e.g.
# {status:204, networkError:false}) would only be caught, if at all, by each
# runner's looser runtime backstop. tests.schema.json wraps schema.json as an
# array so check-jsonschema validates each element of the array-shaped files.
#
# Schema validation checks the fixture FORMAT, not that a mock body still
# decodes into the generated models. The runners enforce that (Kotlin/Swift
# fail loudly on a body that no longer matches) — except where a fixture
# declares errorRaised, which deliberately switches that policy off. The
# control-sibling gate below is what keeps those fixtures honest.
#
# The metaschema pass runs FIRST because validating fixtures against a schema
# that is not itself a valid schema proves nothing. Draft 2020-12 requires every
# value under `properties` to be a schema object or boolean, so an annotation
# like `$comment` placed there declares a property of that name with a string
# for a schema — accepted silently by the fixture pass, rejected by any
# validator that meta-validates first.
#
# The self-test runs after the live check for the same reason the gate exists:
# pointed only at the valid fixture set, the gate proves it can say yes and
# nothing else. Several of its rejections — a kill case answering 204 above all
# — are invisible to both the schema pass and every runner, so a regression that
# removed them would show up as a clean `make conformance` and nowhere else.
conformance-fixtures-check:
	@echo "==> Validating conformance schemas against the JSON Schema metaschema..."
	uvx --from 'check-jsonschema==$(CHECK_JSONSCHEMA_VERSION)' check-jsonschema \
		--check-metaschema conformance/schema.json conformance/tests.schema.json
	@echo "==> Validating conformance fixtures against schema.json..."
	uvx --from 'check-jsonschema==$(CHECK_JSONSCHEMA_VERSION)' check-jsonschema \
		--schemafile conformance/tests.schema.json conformance/tests/*.json
	@echo "==> Checking errorRaised fixtures have body-pinning control siblings..."
	python3 conformance/check_kill_case_controls.py
	@echo "==> Self-testing the control-sibling gate's rejections..."
	@python3 conformance/test_check_kill_case_controls.py

# Unit-test the runners' own assertion helpers.
#
# The runners are test harnesses, but their assertion logic is code like any
# other, and its bounds branches never execute against a fixture that passes.
# #563 shipped a delayBetweenRequests check that vacuously passed when the gap
# it named did not exist; nothing caught it because every committed fixture
# supplied the gap. These pin the branches the fixtures cannot reach.
#
# errorRaised (#576) is the same shape: every fixture declaring it is one the
# SDK does refuse, so its failing branch is unreachable from conformance/tests/
# and a handler that accepted everything would look green in all six runners.
#
# Every recipe below DISCOVERS its suites; none names a test file. #572: the
# Python and Ruby lines used to name `test_delay_gaps.py` / `delay_gaps_test.rb`
# explicitly, so `test_replay_runner.py` and `replay_runner_test.rb` sat in the
# tree executed by nothing — the same "assertion code nothing exercises" shape
# these targets exist to close. Adding a runner test must be enough to run it.
# `scripts/check-runner-test-reachability` enforces both halves: no recipe (here
# or in CI) may name a test file, and every test-bearing file must sit where its
# language's discovery finds it.
#
# Split per language so CI's per-language jobs can call the make target for
# their own toolchain — one definition of what "run the runner tests" means,
# instead of a second enumeration in .github/workflows/test.yml.
conformance-runner-tests: conformance-runner-tests-go conformance-runner-tests-python conformance-runner-tests-ruby conformance-runner-tests-kotlin conformance-runner-tests-swift
	@echo "==> Conformance runner unit tests passed"

conformance-runner-tests-go:
	@echo "==> Running Go conformance runner unit tests..."
	cd conformance/runner/go && go test ./...

# Bare `pytest` discovers test_*.py and *_test.py under the runner directory.
# Collecting nothing is exit 5, so an empty suite fails rather than green-passes.
conformance-runner-tests-python:
	@echo "==> Running Python conformance runner unit tests..."
	cd conformance/runner/python && uv sync --quiet && uv run python -m pytest -q

# Ruby has no runner-level test task, so discovery is hand-rolled here.
#
# It RECURSES, via find. `go test ./...`, pytest and vitest all walk
# subdirectories; a top-level `for f in *_test.rb` did not, which made Ruby the
# one arm where a test file's PLACEMENT silently decided whether it ran.
# conformance/runner/ruby/nested/probe_test.rb was executed by nothing while
# scripts/check-runner-test-reachability — which fnmatched basenames over a
# recursive find — certified it reachable. Recursing here is what makes that
# check's basename model true. The check derives this scope back out of the
# recipe below (see its `ruby_discovery_scope`), so reverting to a top-level
# glob re-arms the placement tooth rather than reopening the hole.
#
# vendor/ and .bundle/ are pruned: `bundle install --path` puts third-party gem
# suites there, and those are not ours to run.
#
# It aborts when discovery matches nothing: a rename that empties it must fail
# loudly, not report success over zero files.
conformance-runner-tests-ruby:
	@echo "==> Running Ruby conformance runner unit tests..."
	@cd conformance/runner/ruby && bundle install --quiet && \
	files=$$(find . \( -name vendor -o -name .bundle \) -prune -o \
		-type f -name '*_test.rb' -print | sort); \
	if [ -z "$$files" ]; then \
		echo "ERROR: no *_test.rb files found under conformance/runner/ruby" >&2; \
		exit 1; \
	fi; \
	for f in $$files; do \
		echo "  --> $$f"; \
		bundle exec ruby "$$f" || exit 1; \
	done

conformance-runner-tests-kotlin:
	@echo "==> Running Kotlin conformance runner unit tests..."
	cd kotlin && ./gradlew --quiet :conformance:test

# Build conformance test runner
conformance-build:
	@echo "==> Building conformance test runner..."
	cd conformance/runner/go && go build -o conformance-runner .

# Run Go conformance tests
conformance-go: conformance-build
	@echo "==> Running Go conformance tests..."
	cd conformance/runner/go && ./conformance-runner

# Run Go wire-replay against snapshots written by the TS live runner.
# Required env: WIRE_REPLAY_DIR, BASECAMP_BACKEND. Opt-in: not in `make check`.
conformance-go-replay:
	@echo "==> Running Go wire-replay runner..."
	@test -n "$$WIRE_REPLAY_DIR" || (echo "WIRE_REPLAY_DIR is required" >&2; exit 1)
	@test -n "$$BASECAMP_BACKEND" || (echo "BASECAMP_BACKEND is required" >&2; exit 1)
	cd conformance/runner/go && go run .

# Run Kotlin conformance tests
conformance-kotlin:
	@echo "==> Running Kotlin conformance tests..."
	cd kotlin && ./gradlew :conformance:run

# Run Kotlin wire-replay against snapshots written by the TS live runner.
# Required env: WIRE_REPLAY_DIR, BASECAMP_BACKEND. Opt-in: not in `make check`.
conformance-kotlin-replay:
	@echo "==> Running Kotlin wire-replay runner..."
	@test -n "$$WIRE_REPLAY_DIR" || (echo "WIRE_REPLAY_DIR is required" >&2; exit 1)
	@test -n "$$BASECAMP_BACKEND" || (echo "BASECAMP_BACKEND is required" >&2; exit 1)
	cd kotlin && ./gradlew --quiet :conformance:runReplay

# Run TypeScript conformance tests.
#
# Depends on ts-build rather than reinstalling the SDK from the runner's own
# pretest hook. The runner consumes typescript/ through a `file:` link, so it
# needs the SDK installed and built — but typescript/node_modules is SHARED with
# ts-check, and `npm ci` deletes node_modules before it installs. A reinstall
# driven from here would race a sibling TypeScript target under `make -j` and
# delete dependencies out from under it. Routing through ts-build puts the
# install behind the ts-install stamp, which make serialises for us; the runner's
# own `npm ci` then touches only its private node_modules.
conformance-typescript: ts-build
	@echo "==> Running TypeScript conformance tests..."
	cd conformance/runner/typescript && npm ci && npm test

# Run TypeScript live canary against a real Basecamp backend.
#
# Required env: BASECAMP_LIVE=1, BASECAMP_TOKEN, BASECAMP_ACCOUNT_ID.
# Optional env: BASECAMP_HOST (origin only, e.g. https://3.basecampapi.com —
# runner appends /{accountId}); BASECAMP_BACKEND=bc4|bc5 to namespace
# snapshots; LIVE_RECORD_DIR to persist wire snapshots for downstream
# replay/compare. Opt-in: not invoked by `make check`.
conformance-typescript-live: ts-build
	@echo "==> Running TypeScript live canary..."
	cd conformance/runner/typescript && npm ci && BASECAMP_LIVE=1 npm test

# Run Ruby conformance tests
conformance-ruby:
	@echo "==> Running Ruby conformance tests..."
	cd conformance/runner/ruby && bundle install --quiet && ruby runner.rb

# Run Ruby wire-replay against snapshots written by the TS live runner.
# Required env: WIRE_REPLAY_DIR, BASECAMP_BACKEND. Opt-in: not in `make check`.
conformance-ruby-replay:
	@echo "==> Running Ruby wire-replay runner..."
	@test -n "$$WIRE_REPLAY_DIR" || (echo "WIRE_REPLAY_DIR is required" >&2; exit 1)
	@test -n "$$BASECAMP_BACKEND" || (echo "BASECAMP_BACKEND is required" >&2; exit 1)
	cd conformance/runner/ruby && bundle install --quiet && ruby replay-runner.rb

# Run Python conformance tests
conformance-python:
	@echo "==> Running Python conformance tests..."
	cd conformance/runner/python && uv sync && uv run python runner.py

# Run Python wire-replay against snapshots written by the TS live runner.
# Required env: WIRE_REPLAY_DIR, BASECAMP_BACKEND. Opt-in: not in `make check`.
conformance-python-replay:
	@echo "==> Running Python wire-replay runner..."
	@test -n "$$WIRE_REPLAY_DIR" || (echo "WIRE_REPLAY_DIR is required" >&2; exit 1)
	@test -n "$$BASECAMP_BACKEND" || (echo "BASECAMP_BACKEND is required" >&2; exit 1)
	cd conformance/runner/python && uv sync && uv run python replay_runner.py

# Run all conformance tests
conformance: oauth-fixtures-check oauth-token-fixtures-check conformance-fixtures-check conformance-runner-tests conformance-go conformance-kotlin conformance-typescript conformance-ruby conformance-python conformance-swift
	@echo "==> Conformance tests passed"

# Orchestrate one canary pass against a single backend:
#   1. TS captures canonical wire snapshots (live HTTP).
#   2. Each replay runner decodes those snapshots through its SDK + walks raw JSON.
#
# Required env (passed through to children):
#   BASECAMP_TOKEN, BASECAMP_ACCOUNT_ID, BASECAMP_BACKEND, LIVE_RECORD_DIR
# (BASECAMP_LIVE=1 is set by the conformance-typescript-live recipe itself.)
#
# The four replay runners run sequentially after the TS capture completes
# (the per-language replays need the wire snapshots TS just wrote). They
# read from `$$LIVE_RECORD_DIR/$$BASECAMP_BACKEND/wire/` and write to
# `$$LIVE_RECORD_DIR/$$BASECAMP_BACKEND/decode/<lang>/`. Failures in any
# stage fail the orchestrator.
#
# check-replay-decoder-parity runs FIRST, as a prerequisite: a fixture
# operation missing from a replay decoder map makes the replay half of this
# target impossible, and the runners only discover that after the ~30-minute
# live capture has already finished. The static check answers the same question
# in 0.3s. (#553 — the four maps were 20 operations behind and nothing said so.)
#
# Opt-in target: not invoked by `make check`.
conformance-live: check-replay-decoder-parity
	@test -n "$$LIVE_RECORD_DIR" || (echo "LIVE_RECORD_DIR is required" >&2; exit 1)
	@test -n "$$BASECAMP_BACKEND" || (echo "BASECAMP_BACKEND is required" >&2; exit 1)
	@test -n "$$BASECAMP_TOKEN" || (echo "BASECAMP_TOKEN is required" >&2; exit 1)
	@test -n "$$BASECAMP_ACCOUNT_ID" || (echo "BASECAMP_ACCOUNT_ID is required" >&2; exit 1)
	@# Canonicalize LIVE_RECORD_DIR before fanning out: the capture and
	@# replay recipes each `cd` into their runner directory, so a relative
	@# path (e.g. the documented tmp/live-canary) would scatter snapshots
	@# under conformance/runner/*/ while the pairwise compare reads from
	@# the repo root — missing-snapshot errors instead of a comparison.
	@LRD="$$LIVE_RECORD_DIR"; case "$$LRD" in /*) ;; *) LRD="$$(pwd)/$$LRD" ;; esac; \
	echo "==> conformance-live: capturing canonical wire snapshots (TypeScript)..." && \
	LIVE_RECORD_DIR="$$LRD" $(MAKE) conformance-typescript-live && \
	echo "==> Running cross-language wire-replay against just-captured snapshots..." && \
	WIRE_REPLAY_DIR="$$LRD" $(MAKE) conformance-ruby-replay && \
	WIRE_REPLAY_DIR="$$LRD" $(MAKE) conformance-python-replay && \
	WIRE_REPLAY_DIR="$$LRD" $(MAKE) conformance-go-replay && \
	WIRE_REPLAY_DIR="$$LRD" $(MAKE) conformance-kotlin-replay
	@echo "==> conformance-live: capture + replay complete for backend $$BASECAMP_BACKEND"

# Top-level live canary: a single-backend conformance pass against production
# BC5. BC5 replaced BC4 in production, so there is one live backend — capture
# BC5 wire snapshots through the typed surface, schema-validate, and 4-language
# decode-replay. The former pairwise BC4↔BC5 comparison is retired (no live BC4
# to compare against); see COORDINATION.md.
#
# Required env:
#   BASECAMP_TOKEN, BASECAMP_ACCOUNT_ID, BASECAMP_HOST (production BC5 origin)
# Optional env:
#   LIVE_RECORD_DIR (snapshot root; defaults to tmp/live-canary)
#
# Snapshot dirs are namespaced by backend label:
# $$LIVE_RECORD_DIR/bc5/{wire,decode}/. The label is also the fixture-override
# prefix (BASECAMP_BC5_* — see conformance/runner/typescript/fixtures.ts). This
# canary always uses `bc5` (production runs BC5); the pass below pins it as a
# sub-make command-line variable so a caller's `make BASECAMP_BACKEND=... ` can't
# retarget the snapshot/fixture namespace.
conformance-canary:
	@test -n "$$BASECAMP_TOKEN" || (echo "BASECAMP_TOKEN is required" >&2; exit 2)
	@test -n "$$BASECAMP_ACCOUNT_ID" || (echo "BASECAMP_ACCOUNT_ID is required" >&2; exit 2)
	@test -n "$$BASECAMP_HOST" || (echo "BASECAMP_HOST is required (production BC5 origin, e.g. https://3.basecampapi.com)" >&2; exit 2)
	@# Defaults resolve in the recipe shell, NOT via target-specific `?=`:
	@# make 3.81 (macOS /usr/bin/make) leaks target-specific `?=` into a
	@# global override that clobbers the caller's environment to empty for
	@# every OTHER target's recipe — breaking conformance-live's
	@# required-env guards whenever this makefile is loaded.
	@#
	@# The rm -rf lives on its own recipe line, separate from the $(MAKE)
	@# invocations: under `make -n`, lines containing $(MAKE) are still
	@# executed (with -n propagated to the sub-make), so folding the rm into
	@# that line would delete snapshots during a dry-run.
	@# Guard against a catastrophic rm -rf: strip ALL trailing slashes (a
	@# single strip would let '///' through as '//' ≈ root), then refuse
	@# "", ".", "/" (repo checkout / filesystem root) and any '..' PATH
	@# SEGMENT (leading, trailing, interior, or the whole path); finally,
	@# canonicalize an EXISTING dir (cd + pwd -P) into a separate GUARD
	@# variable for the checkout-ancestor check, so non-canonical
	@# spellings ('/x//y', symlinks into the checkout) can't slip past the
	@# string compare — a failed cd empties the guard value, which the
	@# ancestor pattern then refuses. The rm itself uses the ORIGINAL
	@# path: when LIVE_RECORD_DIR is a symlink, rm removes the link, not
	@# the tree it points to. A '..'
	@# inside a segment (tmp/live-canary..pr308) is benign and allowed.
	@LRD="$${LIVE_RECORD_DIR:-tmp/live-canary}"; LRD_ORIG="$$LRD"; \
	while [ "$${LRD%/}" != "$$LRD" ]; do LRD="$${LRD%/}"; done; \
	case "$$LRD" in \
	  ""|"."|"/") echo "ERROR: refusing rm -rf on unsafe LIVE_RECORD_DIR='$$LRD_ORIG'" >&2; exit 2 ;; \
	  ".."|"../"*|*"/.."|*"/../"*) echo "ERROR: refusing rm -rf on LIVE_RECORD_DIR with a '..' path segment: '$$LRD_ORIG'" >&2; exit 2 ;; \
	esac; \
	LRD_CHECK="$$LRD"; \
	if [ -d "$$LRD" ]; then LRD_CHECK="$$(cd "$$LRD" && pwd -P)"; fi; \
	case "$$LRD_CHECK" in \
	  "/") echo "ERROR: refusing rm -rf on LIVE_RECORD_DIR='$$LRD_ORIG' — it canonicalizes to the filesystem root" >&2; exit 2 ;; \
	esac; \
	case "$$(pwd -P)/" in \
	  "$$LRD_CHECK"/*) echo "ERROR: refusing rm -rf on LIVE_RECORD_DIR='$$LRD_ORIG' — it is the repo checkout or one of its ancestors" >&2; exit 2 ;; \
	esac; \
	rm -rf -- "$$LRD"
	@echo "==> conformance-canary: production BC5 pass"
	@# BASECAMP_BACKEND=bc5 is passed as a sub-make command-line variable (not a
	@# recipe-shell env prefix): a caller-supplied `make BASECAMP_BACKEND=bc4
	@# conformance-canary` would otherwise propagate and override an env prefix,
	@# capturing production traffic under the wrong label. A command-line variable
	@# on the recursive make wins over that inherited override.
	@LRD="$${LIVE_RECORD_DIR:-tmp/live-canary}"; \
	BASECAMP_LIVE=1 LIVE_RECORD_DIR="$$LRD" $(MAKE) BASECAMP_BACKEND=bc5 conformance-live

#------------------------------------------------------------------------------
# Kotlin SDK targets
#------------------------------------------------------------------------------

.PHONY: kt-generate-services kt-build kt-test kt-check kt-check-drift kt-check-generated-drift kt-clean gradle-stop

# Generate Kotlin services from OpenAPI
kt-generate-services:
	@echo "==> Generating Kotlin services..."
	cd kotlin && ./gradlew :generator:run --args="--openapi ../openapi.json --behavior ../behavior-model.json --output sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"

# Build Kotlin SDK
kt-build:
	@echo "==> Building Kotlin SDK..."
	cd kotlin && ./gradlew :basecamp-sdk:build

# Run Kotlin tests
#
# :generator:test covers the emitters themselves — notably the append-only
# constructor-order rule for options classes, which the generated output alone
# cannot demonstrate.
kt-test:
	@echo "==> Running Kotlin tests..."
	cd kotlin && ./gradlew :basecamp-sdk:check :generator:test

# Run all Kotlin checks
kt-check: kt-test
	@echo "==> Kotlin SDK checks passed"

# Check for drift between generated Kotlin services and OpenAPI spec.
#
# NOTE: this is an operation-level *coverage* check (operationId sets match),
# NOT a regenerate-and-diff freshness gate like check-{python,ruby,typescript}-
# service-drift.sh or check-go-generated-drift.sh. It stays in the default
# `make check` because it is fast (jq/grep, no JVM). The authoritative
# regenerate-and-diff freshness gate is kt-check-generated-drift below.
kt-check-drift:
	@echo "==> Checking Kotlin service drift..."
	@./scripts/check-kotlin-service-drift.sh

# Regenerate-and-diff freshness gate for the whole generated Kotlin tree — the
# Kotlin sibling of check-{python,ruby,swift,typescript}-service-drift.sh and
# check-go-generated-drift.sh, completing 6-SDK parity. Non-mutating: it
# regenerates into a temp dir and diffs, so it detects both missing and extra
# files. Deliberately NOT part of the default `make check`: a `:generator:run`
# is a heavy JVM/Gradle-daemon startup, so it runs as its own target and in CI
# (the test-kotlin job's "Check generated code drift" step) rather than adding
# seconds of local latency to every `make check`.
kt-check-generated-drift:
	@echo "==> Checking Kotlin generated drift (regenerate-and-diff)..."
	@./scripts/check-kotlin-generated-drift.sh

# Clean Kotlin build artifacts
kt-clean:
	@echo "==> Cleaning Kotlin SDK..."
	cd kotlin && ./gradlew clean

# Stop any lingering Gradle daemons
gradle-stop:
	-cd kotlin && ./gradlew --stop
	-cd spec/smithy-bare-arrays && ./gradlew --stop

#------------------------------------------------------------------------------
# Swift SDK targets (delegates to swift/Makefile)
#------------------------------------------------------------------------------

HAS_SWIFT := $(shell command -v swift 2>/dev/null)
IS_MACOS  := $(filter Darwin,$(shell uname -s))

.PHONY: swift-build swift-test swift-check swift-check-drift swift-clean swift-generate

# Build Swift SDK (macOS only — SDK requires Apple platforms)
swift-build:
ifdef IS_MACOS
	@$(MAKE) -C swift build
else
	@echo "SKIP: swift-build (macOS only)"
endif

# Run Swift tests (macOS only)
swift-test:
ifdef IS_MACOS
	@$(MAKE) -C swift test
else
	@echo "SKIP: swift-test (macOS only)"
endif

# Run all Swift checks (macOS only)
swift-check:
ifdef IS_MACOS
	@$(MAKE) -C swift check
else
	@echo "SKIP: swift-check (macOS only)"
endif

# Run Swift conformance tests (macOS only — the SDK requires Apple platforms).
# Defined here rather than in the conformance section so the IS_MACOS ifdef
# parses after the variable is defined above.
conformance-swift:
ifdef IS_MACOS
	@echo "==> Running Swift conformance tests..."
	cd conformance/runner/swift && swift run ConformanceRunner
else
	@echo "SKIP: conformance-swift (macOS only)"
endif

# Unit-test the Swift runner's own assertion helpers (macOS only). Same reason
# as the other five: the bounds branches never execute against a fixture that
# passes, so a vacuous assertion survives a fully green conformance run.
# Reached from conformance-runner-tests, which is platform-agnostic and defers
# the gate to this target. `swift test` discovers the whole Tests/ tree — no
# file is named here (#572).
conformance-runner-tests-swift:
ifdef IS_MACOS
	@echo "==> Running Swift conformance runner unit tests..."
	cd conformance/runner/swift && swift test
else
	@echo "SKIP: conformance-runner-tests-swift (macOS only)"
endif

# Regenerate Swift SDK services from OpenAPI spec (needs swift on any platform)
swift-generate:
ifdef HAS_SWIFT
	@$(MAKE) -C swift generate
else
	$(error swift is required for swift-generate but was not found)
endif

# Check committed generated Swift is current (needs swift on any platform, NOT
# just macOS — generation only needs the toolchain, unlike swift-check's
# build/test which require Apple platforms). Non-mutating regenerate + diff.
swift-check-drift:
ifdef HAS_SWIFT
	@echo "==> Checking Swift service drift..."
	@./scripts/check-swift-service-drift.sh
else
	@echo "SKIP: swift-check-drift (swift toolchain not found)"
endif

# Clean Swift build artifacts
swift-clean:
ifdef HAS_SWIFT
	@$(MAKE) -C swift clean
else
	rm -rf swift/.build
endif

#------------------------------------------------------------------------------
# GitHub Actions lint targets
#------------------------------------------------------------------------------

.PHONY: lint-actions

# Lint GitHub Actions workflows (requires actionlint + zizmor)
lint-actions:
	@command -v actionlint >/dev/null || (echo "Install actionlint: go install github.com/rhysd/actionlint/cmd/actionlint@latest" && exit 1)
	@command -v zizmor >/dev/null || (echo "Install zizmor: https://docs.zizmor.sh/installation/" && exit 1)
	actionlint
	zizmor .

#------------------------------------------------------------------------------
# Setup & tool installation
#------------------------------------------------------------------------------

.PHONY: setup tools

# One-command setup for a fresh clone: install runtimes + dev tools
setup:
	@command -v mise >/dev/null 2>&1 || { echo "ERROR: mise not found. Install: https://mise.jdx.dev"; exit 1; }
	mise install
	mise exec -- $(MAKE) tools

# Pinned tool versions — update these when bumping tools
SMITHY_CLI_VERSION    := 1.68.0
GOLANGCI_LINT_VERSION := v2.11.4
ACTIONLINT_VERSION    := v1.7.11

# Install development tools and prerequisites
tools:
	@echo "==> Installing Smithy CLI..."
	@command -v smithy >/dev/null 2>&1 || { \
		if command -v brew >/dev/null 2>&1; then brew tap smithy-lang/tap && brew install smithy-cli; \
		elif [ "$$(uname -s)" = "Linux" ]; then \
			command -v curl >/dev/null 2>&1 || { echo "ERROR: curl is required"; exit 1; }; \
			command -v unzip >/dev/null 2>&1 || { echo "ERROR: unzip is required"; exit 1; }; \
			ARCH=$$(uname -m); \
			case "$$ARCH" in x86_64) SUFFIX=linux-x86_64;; aarch64) SUFFIX=linux-aarch64;; *) echo "Unsupported arch: $$ARCH" && exit 1;; esac; \
			TMPDIR=$$(mktemp -d) && \
			trap 'rm -rf "$$TMPDIR"' EXIT && \
			echo "Downloading smithy-cli-$$SUFFIX..." && \
			curl -fsSL "https://github.com/smithy-lang/smithy/releases/download/$(SMITHY_CLI_VERSION)/smithy-cli-$$SUFFIX.zip" -o "$$TMPDIR/smithy.zip" && \
			unzip -qo "$$TMPDIR/smithy.zip" -d "$$TMPDIR" && \
			sudo "$$TMPDIR/smithy-cli-$$SUFFIX/install"; \
		else echo "Install Smithy CLI: https://smithy.io/2.0/guides/smithy-cli/cli_installation.html" && exit 1; \
		fi; \
	}
	@echo "==> Installing Go tools..."
	@command -v go >/dev/null 2>&1 || { echo "ERROR: go not found. Run 'make setup' or install Go first."; exit 1; }
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
	@command -v zizmor >/dev/null 2>&1 || { \
		echo "==> Installing zizmor..."; \
		if command -v brew >/dev/null 2>&1; then brew install zizmor; \
		elif command -v pacman >/dev/null 2>&1; then sudo pacman -S --noconfirm zizmor; \
		elif command -v pip3 >/dev/null 2>&1; then pip3 install zizmor; \
		else echo "Install zizmor: https://docs.zizmor.sh/installation/" && exit 1; \
		fi; \
	}
	@command -v jq >/dev/null 2>&1 || { \
		echo "==> Installing jq..."; \
		if command -v brew >/dev/null 2>&1; then brew install jq; \
		elif command -v pacman >/dev/null 2>&1; then sudo pacman -S --noconfirm jq; \
		elif command -v apt-get >/dev/null 2>&1; then sudo apt-get install -y jq; \
		else echo "ERROR: jq is required. Install via your package manager." && exit 1; \
		fi; \
	}
	@command -v node >/dev/null 2>&1 || echo "NOTE: node/npm is required for the TypeScript SDK"
	@command -v ruby >/dev/null 2>&1 || echo "NOTE: ruby/bundler is required for the Ruby SDK"
	@command -v swift >/dev/null 2>&1 || echo "NOTE: swift is optional (macOS: xcode-select --install, Arch: yay -S swift-bin)"
	@echo "==> Done"

#------------------------------------------------------------------------------
# Spec-shape lints
#------------------------------------------------------------------------------

.PHONY: check-bucket-flat-parity validate-api-gaps check-deprecation-parity kt-check-optional-arrays-and-scalars go-check-optional-pointers test-enhance-request-reachability check-fixture-coverage check-idempotency-parity check-write-semantics-parity check-retry-metadata-parity check-runner-test-reachability check-replay-decoder-parity check-readme-env-vars test-check-readme-env-vars check-npm-lockfile-readonly test-check-npm-lockfile-readonly

# Verify every bucket-scoped GET list operation has a flat-path counterpart
# (or is justified in spec/bucket-scoped-allowlist.txt). Cross-project SDK
# consumers shouldn't need to enumerate projects to reach account-wide data.
check-bucket-flat-parity:
	@./scripts/check-bucket-flat-parity.sh

# Verify @deprecated propagates to all six SDKs in the right signal class
# (compiler=Kotlin; editor=TS/Go; doc-only=Ruby/Python/Swift), that the clean
# controls stay unmarked, and that no doubled "Deprecated: Deprecated:" slips in.
# Depends on rb-build: the Ruby YARD-registry sub-check runs `bundle exec`, so
# the gems (YARD) must be installed first — otherwise a direct invocation or a
# parallel `make -j check` that schedules this before rb-check would fail on a
# clean checkout.
check-deprecation-parity: rb-build
	@./scripts/check-deprecation-parity

# Validate spec/api-gaps/ entry frontmatter, required body sections, and allowlist.
validate-api-gaps:
	@./scripts/validate-api-gaps.sh

# Fixture-completeness guard: every spec/fixtures/manifest.yaml target validates
# against its schema (required-field presence + type/nullability), every covered
# schema keeps a concrete active representative, and every rich-text emitter is
# accounted for — so a new required field on a covered schema is forced into a
# fixture. Reuses the conformance schema-walker. The self-test asserts the guard
# rejects each crafted failure mode (the live check only exercises the valid set).
check-fixture-coverage:
	@./scripts/check-fixture-coverage.sh
	@ruby ./scripts/test-check-fixture-coverage.rb

# Presence contract for generated Kotlin ARRAY and PRIMITIVE SCALAR properties:
# optional -> `T? = null`, required -> `T`, required-and-nullable -> `T?` with no
# default, and no zero-value sentinel defaults (`= emptyList()`, `= 0`, `= false`,
# `= ""`). Pins the Kotlin scalar fix (#424) and optional-array fix (#433).
# Object/$ref/enum properties are deliberately out of scope — hence the name.
kt-check-optional-arrays-and-scalars:
	@./scripts/check-kotlin-optional-arrays-and-scalars.sh

# Verify every optional (omitempty) field in the generated Go client can
# represent absence: pointer, slice, map, or interface — no value-typed
# zero-value sentinels (SPEC.md §10). No waiver list; the type classifier is
# the policy. Pins the #436 fix.
go-check-optional-pointers:
	@./scripts/check-go-optional-pointers

# Drive the enhancer from outside with synthetic specs whose correct answer is
# known independently: direct, one-hop, two-hop, and unused request-body
# components. The enhancer's own self-check computes the same closure it
# validates, so it cannot catch a bug in that closure — this can.
test-enhance-request-reachability:
	@./scripts/test-enhance-request-reachability

# Verify idempotency classification is identical across all six SDKs and matches
# behavior-model.json (the naturally-idempotent mutations; Go additionally folds
# in the read-only ops). Bash+jq — runs anywhere, enforced in CI.
check-idempotency-parity:
	@./scripts/check-idempotency-parity

# Verify every x-basecamp-write-semantics extension in openapi.json equals the
# matching `write` clause in behavior-model.json, both directions. The two come
# from different tools, and generate-behavior-model builds its clause key by
# key, so a trait field it was never taught about is dropped silently. Bash+jq.
check-write-semantics-parity:
	@echo "==> Checking write-semantics parity..."
	@./scripts/check-write-semantics-parity

# Verify every SDK's emitted per-operation retry metadata equals
# behavior-model.json, and that each SDK still consumes the fields it claims to.
# Python3 — runs anywhere, enforced in CI (test-go job).
check-retry-metadata-parity:
	@python3 ./scripts/check-retry-metadata-parity.py

# Verify every conformance-runner test file is actually reachable from the
# discovery its language's `conformance-runner-tests-*` recipe performs, and
# that no recipe (Makefile or CI) names an individual test file. #572: two
# replay-runner suites sat in the tree for months executed by nothing because
# the recipes enumerated filenames. Bash+grep — runs anywhere, enforced in CI.
check-runner-test-reachability:
	@./scripts/check-runner-test-reachability

# Verify every live operation in conformance/tests/live-my-surface.json has a
# decoder in all five dispatch tables (TS LIVE_OPERATIONS plus the four replay
# runners). The runners' own coverage gates only fire during a live canary, and
# that canary skips whenever its secrets are unset — which is how four tables
# sat 20 operations behind the fixture with CI green (#553). Bash+jq, 0.3s.
check-replay-decoder-parity:
	@./scripts/check-replay-decoder-parity

# Verify the README environment-variable tables against the SDK sources, both
# directions: nothing documented that isn't read, nothing read that isn't
# documented. Nothing here is generated, so prose drift is otherwise silent —
# a phantom BASECAMP_ACCOUNT_ID row and an XDG_CACHE_HOME credited to Ruby both
# shipped. Python3 — runs anywhere, enforced in CI (spec-gates job).
check-readme-env-vars:
	@python3 ./scripts/check-readme-env-vars.py

# Drive that gate from outside with synthetic repos whose correct answer is
# known: single- and double-quoted reads, doc-comment examples, test source
# sets, and a checkout nested under a directory named Tests. Its failure mode is
# silent — a read pattern that misses reports "nothing reads this", which looks
# like a README bug rather than a gate bug — so it needs its own tests.
test-check-readme-env-vars:
	@python3 ./scripts/test-check-readme-env-vars.py

# Assert nothing `make check` runs can rewrite an npm lockfile. `npm install`
# writes package-lock.json back, and *what* it writes depends on the npm version
# running it, not the platform — npm >= 11.11.0 records a `libc` array on
# Linux-only optional dependencies and every npm below that drops it (bisected:
# 11.10.0 none, 11.11.0 eighteen). The pinned toolchain (10.9.8), CI's node 24
# npm (11.5.1), and Dependabot's npm (11.19.x) straddle that
# threshold, so a single `npm install` in a lifecycle script made the lockfile
# oscillate by whoever ran it last (#612). `npm ci` installs from the lockfile
# without writing it, and fails loudly on the package.json drift `npm install`
# would have absorbed silently.
#
# An ALLOWLIST of permitted invocations, not a denylist of writers: npm accepts
# any unambiguous prefix (`npm in`, `npm ins`) and tolerates flags before the
# subcommand (`npm --prefix ../x install`), so a denylist cannot enumerate the
# writers. Covers package.json lifecycle scripts, Makefile recipes, and
# scripts/, exempting the one deliberate writer (scripts/bump-version.sh).
# Bash+jq, static, 0.2s.
check-npm-lockfile-readonly:
	@./scripts/check-npm-lockfile-readonly

# Drive that gate from outside with synthetic repos whose correct answer is
# known. Its live run only ever exercises the passing case, so nothing there
# proves it rejects anything — and its first cut was a denylist that four
# ordinary spellings walked straight through (`npm in`, `npm ins`,
# `npm --prefix <path> install`, `npm --prefix=<path> install`). Those four are
# pinned here, alongside the deliberate bump-version.sh exemption and a control
# proving the same content is rejected under any other name.
test-check-npm-lockfile-readonly:
	@./scripts/test-check-npm-lockfile-readonly

#------------------------------------------------------------------------------
# Combined targets
#------------------------------------------------------------------------------

.PHONY: generate

# Regenerate every machine-derived artifact in the repo, in dependency order.
# Run after editing spec/basecamp.smithy or spec/api-provenance.json.
# Sequential phases via sub-makes so language generators don't run in
# parallel against a stale openapi.json under `make -j`.
generate:
	@$(MAKE) smithy-build
	@$(MAKE) behavior-model url-routes provenance-sync
	@$(MAKE) ts-generate ts-generate-services \
	         rb-generate rb-generate-services \
	         py-generate \
	         kt-generate-services \
	         swift-generate
	@$(MAKE) -C go generate
	@echo "==> Generation complete"

# Run all checks (Smithy + Go + TypeScript + Ruby + Kotlin + Swift + Python + Behavior Model + Conformance + Provenance + Actions lint)
check: lint-actions sync-spec-version-check smithy-check behavior-model-check provenance-check sync-api-version-check doc-constants-check url-routes-check bc3-route-parity test-bc3-route-parity go-check-drift go-check-wrapper-drift go-check-generated-drift auth-routable-check kt-check-drift swift-check-drift go-check ts-check rb-check kt-check swift-check py-check conformance check-bucket-flat-parity validate-api-gaps check-deprecation-parity check-fixture-coverage kt-check-optional-arrays-and-scalars go-check-optional-pointers test-enhance-request-reachability check-idempotency-parity check-write-semantics-parity check-retry-metadata-parity check-runner-test-reachability check-replay-decoder-parity check-readme-env-vars test-check-readme-env-vars check-npm-lockfile-readonly test-check-npm-lockfile-readonly
	@echo "==> All checks passed"

# Clean all build artifacts
clean: smithy-clean go-clean ts-clean rb-clean kt-clean swift-clean py-clean

# Help
help:
	@echo "Basecamp SDK Makefile"
	@echo ""
	@echo "Smithy:"
	@echo "  smithy-validate  Validate Smithy spec syntax"
	@echo "  smithy-mapper    Build custom OpenAPI mapper JAR"
	@echo "  smithy-build     Build OpenAPI from Smithy (updates openapi.json)"
	@echo "  smithy-check     Verify openapi.json is up to date"
	@echo "  sync-spec-version        Sync Smithy service version from provenance"
	@echo "  sync-spec-version-check  Verify Smithy service version matches provenance"
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
	@echo "  go-check-drift           Check service layer drift vs generated client (operation-level)"
	@echo "  go-check-wrapper-drift   Check wrapper struct drift vs generated structs (field-level)"
	@echo "  go-check-generated-drift Check generated client.gen.go is current (regenerate + diff)"
	@echo "  go-clean         Remove Go build artifacts"
	@echo ""
	@echo "TypeScript SDK:"
	@echo "  ts-generate           Generate types and metadata from OpenAPI"
	@echo "  ts-generate-services  Generate service classes from OpenAPI"
	@echo "  ts-build              Build TypeScript SDK"
	@echo "  ts-test               Run TypeScript tests"
	@echo "  ts-typecheck          Run TypeScript type checking"
	@echo "  ts-check-drift        Check generated src/generated/ is current (regenerate + diff)"
	@echo "  ts-check              Run all TypeScript checks"
	@echo "  ts-clean              Remove TypeScript build artifacts"
	@echo ""
	@echo "Kotlin SDK:"
	@echo "  kt-generate-services Generate service classes from OpenAPI"
	@echo "  kt-build             Build Kotlin SDK"
	@echo "  kt-test              Run Kotlin tests"
	@echo "  kt-check             Run all Kotlin checks"
	@echo "  kt-check-drift       Check service drift vs OpenAPI spec (fast coverage)"
	@echo "  kt-check-generated-drift  Regenerate-and-diff freshness gate (heavy; CI + on demand)"
	@echo "  kt-clean             Remove Kotlin build artifacts"
	@echo "  gradle-stop          Stop any lingering Gradle daemons"
	@echo ""
	@echo "Swift SDK:"
	@echo "  swift-generate   Generate service classes from OpenAPI"
	@echo "  swift-build      Build Swift SDK"
	@echo "  swift-test       Run Swift tests"
	@echo "  swift-check      Run all Swift checks"
	@echo "  swift-check-drift  Check generated Swift is current (any OS with swift)"
	@echo "  swift-clean      Remove Swift build artifacts"
	@echo ""
	@echo "Conformance:"
	@echo "  conformance                Run all conformance tests"
	@echo "  conformance-go             Run Go conformance tests"
	@echo "  conformance-go-replay      Decode TS-captured wire snapshots through Go SDK"
	@echo "  conformance-kotlin         Run Kotlin conformance tests"
	@echo "  conformance-kotlin-replay  Decode TS-captured wire snapshots through Kotlin SDK"
	@echo "  conformance-typescript     Run TypeScript conformance tests"
	@echo "  conformance-typescript-live Run TypeScript live canary against a real backend"
	@echo "  conformance-ruby           Run Ruby conformance tests"
	@echo "  conformance-ruby-replay    Decode TS-captured wire snapshots through Ruby SDK"
	@echo "  conformance-python         Run Python conformance tests"
	@echo "  conformance-python-replay  Decode TS-captured wire snapshots through Python SDK"
	@echo "  conformance-swift          Run Swift conformance tests (macOS only)"
	@echo "  conformance-runner-tests   Unit-test every runner's own assertion helpers"
	@echo "  conformance-runner-tests-<lang>  ...for one of go|python|ruby|kotlin|swift"
	@echo "  conformance-build          Build Go conformance test runner"
	@echo "  oauth-fixtures-check       Validate OAuth discovery fixtures against their schema"
	@echo "  oauth-token-fixtures-check Validate OAuth token wire-behavior fixtures against their schema"
	@echo "  conformance-fixtures-check Validate conformance/tests fixtures against schema.json"
	@echo "  check-runner-test-reachability  Assert every runner test file is reachable from discovery"
	@echo "  check-replay-decoder-parity  Assert all five replay/dispatch tables cover the live fixture"
	@echo ""
	@echo "Ruby SDK:"
	@echo "  rb-generate          Generate types and metadata from OpenAPI"
	@echo "  rb-generate-services Generate service classes from OpenAPI"
	@echo "  rb-build             Build Ruby SDK (install deps)"
	@echo "  rb-test              Run Ruby tests (with coverage)"
	@echo "  rb-check-drift       Check generated metadata/types/services are current (regenerate + diff)"
	@echo "  rb-check             Run all Ruby checks"
	@echo "  rb-doc               Generate YARD documentation"
	@echo "  rb-clean             Remove Ruby build artifacts"
	@echo ""
	@echo "Python SDK:"
	@echo "  py-generate          Generate types and metadata from OpenAPI"
	@echo "  py-generate-services Generate service classes from OpenAPI"
	@echo "  py-build             Build Python SDK (install deps)"
	@echo "  py-test              Run Python tests"
	@echo "  py-check             Run all Python checks"
	@echo "  py-check-drift       Check service drift vs OpenAPI spec"
	@echo "  py-clean             Remove Python build artifacts"
	@echo ""
	@echo "Provenance:"
	@echo "  provenance-sync  Copy provenance into Go package for go:embed"
	@echo "  provenance-check Verify Go embedded provenance is up to date"
	@echo "  sync-status      Show upstream changes since last spec sync"
	@echo ""
	@echo "Version & Release:"
	@echo "  bump VERSION=x.y.z       Bump SDK version across all languages"
	@echo "  sync-api-version         Sync API_VERSION from openapi.json"
	@echo "  sync-api-version-check   Verify API_VERSION constants are up to date"
	@echo "  doc-constants-check      Verify marked doc constants match their sources"
	@echo "  release VERSION=x.y.z    Tag and push a global release (triggers all SDK releases)"
	@echo ""
	@echo "GitHub Actions:"
	@echo "  lint-actions     Lint GitHub Actions workflows (actionlint + zizmor)"
	@echo ""
	@echo "Setup:"
	@echo "  setup            One-command setup (mise install + tools)"
	@echo "  tools            Install development tools (smithy, golangci-lint, actionlint, zizmor)"
	@echo ""
	@echo "Combined:"
	@echo "  generate         Regenerate every machine-derived artifact (Smithy + per-language SDKs + provenance)"
	@echo "  check            Run all checks (Smithy + behavior-model/drift + Go + TypeScript + Ruby + Swift + Kotlin + Python + Conformance + Provenance + API version sync + parity lint + api-gaps + fixture-coverage + kt-optional-arrays + Actions lint)"
	@echo "  clean            Remove all build artifacts"
	@echo "  help             Show this help"
