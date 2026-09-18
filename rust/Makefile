# Basecamp Rust SDK Makefile — the crate-local recipes the root Makefile's rs-* targets call.

CARGO := cargo

.PHONY: all
all: check

# Regenerate basecamp-sdk/src/generated from ../openapi.json and ../behavior-model.json
# The declaration every generator reads is validated FIRST. This target is
# directly invokable, so the root Makefile's prerequisite does not cover it —
# the gate has to sit at each generation entry point, not only at the root
# wrappers. See scripts/check-generated-verbs.
.PHONY: generate
generate:
	@../scripts/check-generated-verbs
	@echo "Generating Rust types, routes, metadata and services from openapi.json..."
	$(CARGO) run -q -p basecamp-sdk-generator

# Fail when the checked-in generated code differs from what the generator emits
.PHONY: generate-check
generate-check:
	$(CARGO) run -q -p basecamp-sdk-generator -- --check

.PHONY: build
build:
	$(CARGO) build --workspace --locked

.PHONY: fmt
fmt:
	$(CARGO) fmt --all

.PHONY: fmt-check
fmt-check:
	$(CARGO) fmt --all -- --check

.PHONY: lint
lint: fmt-check
	$(CARGO) clippy --workspace --all-targets --all-features --locked -- -D warnings

.PHONY: test
test:
	$(CARGO) test --workspace --all-features --locked

# Every feature set the crate supports must build and pass its tests.
.PHONY: test-features
test-features:
	$(CARGO) test -p basecamp-sdk --no-default-features --locked
	$(CARGO) test -p basecamp-sdk --no-default-features --features oauth --locked
	$(CARGO) test -p basecamp-sdk --locked
	$(CARGO) test -p basecamp-sdk --all-features --locked

# Broken intra-doc links fail the build
.PHONY: doc
doc:
	RUSTDOCFLAGS="-D warnings" $(CARGO) doc -p basecamp-sdk --no-deps --all-features

.PHONY: deny
deny:
	$(CARGO) deny check

.PHONY: publish-check
publish-check:
	$(CARGO) publish -p basecamp-sdk --dry-run --locked

# The local development gate: what CI's test-rust job runs.
.PHONY: check
check: lint test test-features doc deny generate-check publish-check

.PHONY: clean
clean:
	$(CARGO) clean

.PHONY: help
help:
	@echo "Basecamp Rust SDK Makefile targets:"
	@echo ""
	@echo "  generate        Regenerate basecamp-sdk/src/generated from openapi.json"
	@echo "  generate-check  Verify basecamp-sdk/src/generated is up to date"
	@echo "  fmt             Format code"
	@echo "  fmt-check       Check formatting"
	@echo "  lint            fmt-check + clippy with warnings denied"
	@echo "  test            Run all tests"
	@echo "  test-features   Run the tests under every feature set"
	@echo "  doc             Build the crate docs"
	@echo "  deny            cargo deny check (advisories, licenses, bans, sources)"
	@echo "  publish-check   cargo publish --dry-run"
	@echo "  check           lint + test + test-features + doc + deny + generate-check + publish-check"
	@echo "  clean           Remove build artifacts"
