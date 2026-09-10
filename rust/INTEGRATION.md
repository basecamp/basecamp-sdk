# Rust SDK — integration checklist for the shared files

`rust/` is owned by the generator/runtime lane and is self-contained: `make -C rust check`
runs every gate the crate has. Everything a language needs *outside* its own directory —
root `Makefile` targets, parity readers, rosters, CI, docs — is enumerated here for the
integration owner to land in one commit, exactly as SPEC §21 and `rustB-contract.md` §2
list them. Nothing in this file has been applied to the shared files from the `rust/` lane.

## 1. Root `Makefile`

Add a `rs-*` block modelled on the Swift block, delegating to `rust/Makefile`:

```make
#------------------------------------------------------------------------------
# Rust SDK targets
#------------------------------------------------------------------------------

.PHONY: rs-build rs-test rs-lint rs-doc rs-deny rs-generate rs-check-drift rs-publish-check rs-check rs-clean

rs-build:
	@echo "==> Building Rust SDK..."
	$(MAKE) -C rust build

rs-test:
	@echo "==> Testing Rust SDK..."
	$(MAKE) -C rust test test-features

rs-lint:
	$(MAKE) -C rust lint

rs-doc:
	$(MAKE) -C rust doc

rs-deny:
	$(MAKE) -C rust deny

# Regenerate rust/basecamp-sdk/src/generated from openapi.json + behavior-model.json
rs-generate:
	@echo "==> Generating Rust SDK..."
	$(MAKE) -C rust generate

# Regenerate-and-diff freshness gate (the Swift model; see scripts/check-rust-service-drift.sh)
rs-check-drift:
	@./scripts/check-rust-service-drift.sh

rs-publish-check:
	$(MAKE) -C rust publish-check

rs-check: rs-lint rs-test rs-doc rs-deny rs-check-drift rs-publish-check

rs-clean:
	$(MAKE) -C rust clean
```

- `generate:` — add `@$(MAKE) rs-generate` **before** the `sync-api-version` lines (the
  generated `rust/basecamp-sdk/src/generated/mod.rs` carries `API_VERSION`, which
  `sync-api-version.sh` also rewrites; see §5).
- `check-targets` — add `rs-check-drift rs-check`.
- `clean:` — add `rs-clean`.
- `help:` — add the `rs-*` lines.
- `conformance-rust` / `conformance-runner-tests-rust` are W3's (`conformance/runner/rust`).

## 2. `scripts/check-rust-service-drift.sh` (new file)

The Swift model — regenerate into a temp dir and `diff -rq`, never in-place:

```bash
#!/bin/bash
# check-rust-service-drift.sh
#
# Verifies that the committed generated Rust artifacts are current by
# regenerating the whole generated/ tree into a temp directory and diffing.
# Generation needs only the Rust toolchain and rustfmt. It is non-mutating:
# the committed tree is never touched.
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected
#   2 = cargo not available

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

if ! command -v cargo >/dev/null 2>&1; then
  echo "ERROR: cargo is required for check-rust-service-drift.sh but was not found" >&2
  exit 2
fi

GENERATED_DIR="$ROOT_DIR/rust/basecamp-sdk/src/generated"
TMP_OUT="$(mktemp -d "${TMPDIR:-/tmp}/rust-service-drift.XXXXXX")"
trap 'rm -rf "$TMP_OUT"' EXIT

echo "==> Regenerating Rust SDK into a temp directory..."
(cd "$ROOT_DIR/rust" && cargo run -q -p basecamp-sdk-generator -- --root "$ROOT_DIR" --output "$TMP_OUT") > /dev/null

echo "==> Diffing against committed rust/basecamp-sdk/src/generated/ ..."
if ! diff -rq "$GENERATED_DIR" "$TMP_OUT" > /dev/null; then
  echo "ERROR: Generated Rust is out of date. Run 'make rs-generate'"
  diff -rq "$GENERATED_DIR" "$TMP_OUT" || true
  exit 1
fi

echo "No drift detected."
exit 0
```

(`cargo run -p basecamp-sdk-generator -- --check` does the same in-process and is what
`make -C rust generate-check` runs; the script exists so the root gate has the same shape
as the other five.)

## 3. Parity readers — one commit, all five together

All five are exact-source readers and fail on a partial roster, which is why none of them
was touched from the `rust/` lane. The shapes they read are stable now that all 262
operations are emitted.

### `scripts/check-retry-metadata-parity.py`

Reader (labelled, one operation per line — `metadata.rs` is `#[rustfmt::skip]`):

```python
def from_rust() -> dict[str, tuple]:
    text = (ROOT / "rust/basecamp-sdk/src/generated/metadata.rs").read_text()
    # pub static GET_PROJECT: OperationMetadata = OperationMetadata { operation: "GetProject", idempotent: false, readonly: true, retry: RetryConfig { max_attempts: 3, base_delay_ms: 1000, backoff: Backoff::Exponential, retry_on: &[429, 503] } };
    pat = re.compile(
        r'operation:\s*"(?P<op>\w+)",\s*idempotent:\s*(?:true|false),\s*readonly:\s*(?:true|false),\s*'
        r"retry:\s*RetryConfig\s*\{\s*max_attempts:\s*(?P<max>\d+),\s*base_delay_ms:\s*(?P<delay>\d+),\s*"
        r"backoff:\s*Backoff::(?P<backoff>\w+),\s*retry_on:\s*&\[(?P<ro>[^\]]*)\]"
    )
    out: dict[str, tuple] = {}
    for m in pat.finditer(text):
        ro = tuple(int(x) for x in re.findall(r"\d+", m.group("ro")))
        out[m.group("op")] = (int(m.group("max")), int(m.group("delay")), m.group("backoff").lower(), ro)
    return out
```

`main()`: `errors += check_full_tuple("Rust    metadata.rs", from_rust(), model)`.

Consumer row (Rust consumes the full tuple; the loop lives in `client.rs`, the curve in
`retry.rs`):

```python
    ("Rust", "full tuple", "rust/basecamp-sdk/src/client.rs",
     ["retry.retry_on.contains", "effective_attempts(self.shared.config.max_retries, retry.max_attempts)", "backoff_with_jitter(retry, retry_index"], [],
     "client.rs: retry_on.contains(status), min(cap, max_attempts) via effective_attempts, backoff_with_jitter over base_delay_ms/backoff"),
    ("Rust", "full tuple", "rust/basecamp-sdk/src/retry.rs",
     ["config.base_delay_ms", "config.backoff", "Backoff::Exponential", "MAX_BACKOFF_DELAY_MS"], [],
     "retry.rs backoff_ms: base_delay_ms × curve, saturating at MAX_BACKOFF_DELAY_MS"),
```

Docstring: add `rust/basecamp-sdk/src/generated/metadata.rs (labelled)` to the full-tuple list.

### `scripts/check-idempotency-parity`

```bash
# ---- Rust (metadata.rs, labelled `operation: "Op", idempotent: true`) --------
grep -oE 'operation: "[[:alnum:]]+", idempotent: true' \
    rust/basecamp-sdk/src/generated/metadata.rs \
    | sed -E 's/operation: "([[:alnum:]]+)".*/\1/' | sort > "$WORK/rust"
compare "Rust idempotent set == $expected_idempotent" "$WORK/idempotentSet" "$WORK/rust"
```

Header comment: "six SDKs" → seven.

### `scripts/check-operation-assignment-parity`

A `SOURCES` entry. The Rust generator does not put the literal path in the call (it calls
`self.client.operation(&routes::CONST, …)`), so each generated method carries a rustdoc
line `/// \`GET /projects/{projectId}\` — …` emitted from the same `Operation` record the
route constant is emitted from; that line is the extraction point.

```ruby
  {
    id: "rust",
    path: "rust/basecamp-sdk/src/generated/services",
    ext: ".rs",
    drop: ["mod"],
    class_pattern: /^pub struct (\w+)<'a> \{/,
    calls: [
      # The generator writes the wire line into every method's rustdoc from the same
      # record the route constant is emitted from (rust/generator/src/emit/services.rs).
      { pattern: %r{^    /// `(?<method>GET|POST|PUT|DELETE|PATCH) (?<path>/[^`]*)` — } },
    ],
  },
```

`canonical_service` already strips a trailing `service` (`ProjectsService` → `projects`).
The `XParams` structs in the same files are `pub struct ListTodosParams {` — no `<'a>` — so
the class pattern does not match them. Success line: "all 6 generated SDKs".

### `scripts/check-service-inventory-parity`

Two entries: the services directory and the accessor file.

```ruby
  {
    id: "rust",
    path: "rust/basecamp-sdk/src/generated/services",
    kind: :directory,
    ext: ".rs",
    drop: ["mod"],
    spelling: :snake,
  },
  {
    id: "rust-accessors",
    path: "rust/basecamp-sdk/src/generated/accessors.rs",
    kind: :file,
    pattern: /^    pub fn ([a-z][a-z0-9_]*)\(&self\) -> services::[a-z0-9_]+::[A-Za-z0-9]+Service<'_> \{/,
    spelling: :snake,
  },
```

Bump the prose ("six SDKs", "eight renderings" → ten).

### `scripts/check-deprecation-parity`

Rust is in the compiler-warning class (`#[deprecated(note = "…")]`), like Kotlin:

```bash
# ---- Rust (compiler warning: #[deprecated]) ---------------------------------
RS_SVC=rust/basecamp-sdk/src/generated/services/search.rs
assert_contains "$RS_SVC" '#[deprecated(note = "prefer type_names[].")]' "Rust: search 'type' param marker"
assert_contains "$RS_SVC" '#[deprecated(note = "prefer bucket_ids[].")]' "Rust: search 'bucket_id' param marker"
assert_contains "$RS_SVC" '#[deprecated(note = "prefer creator_ids[].")]' "Rust: search 'creator_id' param marker"
assert_count "$RS_SVC" '#\[deprecated\(' 3 "Rust: exactly three param markers (controls clean)"
RS_TYPES=rust/basecamp-sdk/src/generated/types.rs
# rustfmt wraps the long note onto its own line, so the attribute and its note are two lines.
assert_count "$RS_TYPES" '^#\[deprecated\($' 1 "Rust: the ClientSide struct is marked"
assert_count "$RS_TYPES" '^    #\[deprecated\($' 1 "Rust: the Project.clientside field is marked"
assert_count "$RS_TYPES" 'note = "This shape is deprecated since 2024-01: Use Client Visibility feature instead"' 2 "Rust: both markers carry the reason"
assert_contains "$RS_SVC" '#[allow(deprecated)]' "Rust: search method reads its deprecated params under an allow"
assert_field_undeprecated "$RS_SVC" "pub type_names: Option<Vec<String>>," "Rust control: type_names replacement is present and unmarked"
```

Add `rust/basecamp-sdk/src/generated` to the doubled-marker `grep -rIn` list.

## 4. `scripts/check-readme-env-vars.py`

`SDKS["Rust"]`: readme `rust/basecamp-sdk/README.md`, source `rust/basecamp-sdk/src`,
suffixes `.rs`, comment style `//` and `/* */` (no nesting), string quotes `"` only
(plus raw `r"…"`/`r#"…"#`), env read pattern `env::var\(\s*"(?P<name>[A-Z_][A-Z0-9_]*)"`.
The only reads are in `config.rs::from_env` (`BASECAMP_BASE_URL`, `BASECAMP_TIMEOUT`,
`BASECAMP_MAX_RETRIES`), all named in the README's "Environment variables" section. The root
README sentence about which SDKs read no env vars is unaffected (Rust reads three).

## 5. `scripts/sync-api-version.sh` and `scripts/bump-version.sh`

- `sync-api-version.sh`: `sedi "s/^pub const API_VERSION: &str = \".*\";/pub const API_VERSION: \&str = \"$API_VERSION\";/" rust/basecamp-sdk/src/generated/mod.rs` (the generator writes the same value, so `make generate`'s second `sync-api-version` is a no-op for Rust). `sync-api-version-check`: grep the same line.
- `bump-version.sh`: version file `rust/basecamp-sdk/Cargo.toml` (`^version = "…"` inside `[package]`, first match only — the workspace manifest carries no version); then `(cd rust && cargo update -p basecamp-sdk --offline)` to restamp `rust/Cargo.lock`, and the same in `conformance/runner/rust` once W3 lands. Bump the hardcoded counts in its final echo (+1 version file, +2 lockfiles). `release:` guard: `cargo metadata --no-deps --format-version 1 --manifest-path rust/Cargo.toml | jq -r '.packages[] | select(.name=="basecamp-sdk") | .version'`.
- `scripts/assert-lockfiles-unchanged`: add `Cargo.lock` to the `find` names and the pathspec.

## 6. CI (`.github/workflows`) — W4 owns these; what the crate expects

- `test.yml` job `test-rust`: matrix `{ toolchain: [1.88, stable] }`; `dtolnay/rust-toolchain` at the matrix value (`stable` = `rust/rust-toolchain.toml`, i.e. 1.98.1); `Swatinem/rust-cache` with `workspaces: rust`; steps in order: `cargo fmt --all --check` (stable), `cargo clippy --workspace --all-targets --all-features --locked -- -D warnings` (stable), `cargo test --workspace --all-features --locked` (both), `cargo test -p basecamp-sdk --no-default-features --locked` (both), `RUSTDOCFLAGS="-D warnings" cargo doc -p basecamp-sdk --no-deps --all-features` (stable), `EmbarkStudios/cargo-deny-action` with `manifest-path: rust/Cargo.toml` (stable), `scripts/check-rust-service-drift.sh`, `cargo publish -p basecamp-sdk --dry-run --locked` (stable), `obi1kenobi/cargo-semver-checks-action` (`manifest-path: rust/basecamp-sdk/Cargo.toml`, `continue-on-error: true` until a crates.io baseline exists), then W3's runner and `scripts/assert-lockfiles-unchanged --verify-clean`.
- `security.yml`: `cargo-deny` job over `rust/Cargo.toml` (advisories + licenses; `deny.toml` is checked in).
- `dependabot.yml`: `cargo` at `/rust` (groups `tokio-ecosystem`, `serde`, `cargo-dependencies`; prefix `deps(rust):`; labels `dependencies, rust`) and `/conformance/runner/rust`.
- `labeler.yml`: `rust: - changed-files: - any-glob-to-any-file: "rust/**"`.
- `codeql.yml`: no Rust analyzer in the matrix builder — record the gap in SECURITY.md; clippy + cargo-deny cover it.

## 7. Docs — what is true about the crate today

- Crate README: `rust/basecamp-sdk/README.md` (crates.io landing page; `#![doc = include_str!]` in `lib.rs`, so every fence is a doctest — keep `rust,no_run` on network snippets).
- Root README rows: language `Rust` → `rust/`, package `basecamp-sdk` (crates.io; publish is gated on the Trusted Publishing bootstrap), docs `https://docs.rs/basecamp-sdk`; install line `cargo add basecamp-sdk`; MSRV 1.88; feature matrix column: retries ✓, pagination ✓, ETag cache ✗ (follow-up card), hooks ✓, OAuth ✓ (device flow with `login_hint`), webhooks ✓ (signature verify), download ✓, event feed ✗ (§23 deferred).
- SPEC.md rows: §7 caps — client cap `max_retries` (total attempts, `0` legal), per-op ceiling honoured, backoff overflow: log-domain saturating (`retry.rs`); §8 page param — `XParams.page`, pinned page never followed (following is explicit via `next_page`/`collect_all`); §9 header table — `basecamp-sdk-rust/{VERSION} (api:{API_VERSION})`; §10 integer width — `i64` ids, `Person.id` via `flexible_i64`, `FlexInt` dimensions via `flex_int`; §9 truncation unit — characters; §16 OAuth applicability — device flow, PKCE, exchange, refresh, discovery (see the oauth module report); §21 gate table — `rs-check`, runner `conformance/runner/rust`; Appendix F — §17 ETag cache not shipped, §23 event feed not shipped (feature `event-feed` reserved), `on_paginate` hook omitted (allowed).
- SECURITY.md: "all seven implementations"; `### Rust` — rustls by default (`native-tls` additive opt-in), PKCE S256, HTTPS enforced with the localhost carve-out, sensitive headers redacted, signed download URLs never rendered (origin only), 401 refresh row: "at most once per request, budget-gated before `refresh()`, concurrent 401s coalesce into one refresh".
- AGENTS.md: architecture row `Rust | reqwest via HttpClient trait | rust/basecamp-sdk/src/generated/services/*.rs`; infra rows (`client.rs`/`retry.rs`/`pagination.rs`/`hooks.rs`; OAuth `rust/basecamp-sdk/src/oauth/`; composites `rust/basecamp-sdk/src/services/{todos,todolists,documents,schedules,cards,uploads}.rs`); Hard Rule 2's TAG_TO_SERVICE list gains `rust/generator/names.toml`; "all 7 workflows" → 8.
- CONTRIBUTING.md: prerequisites row `Rust | 1.88+ (MSRV), rust-toolchain.toml pins the dev toolchain; cargo-deny`; build block `cd rust && make check`; generator line `rust/generator` (`make rs-generate`).
- MIGRATING.md `# Unreleased`: `### Rust: new SDK`.

## 8. Local deviations from `rustD-devex-standard.md` (decided by the plan, recorded here)

- `rust/` is a two-member workspace (`basecamp-sdk` + `generator`), not a single crate: the generator is a Rust binary and must live somewhere Cargo can build it.
- MSRV is fixed at `1.88` (the plan's decision), not the standard's `1.85`; `clippy.toml` `msrv` matches.
- `native-tls` is an additive feature beside `rustls-tls` (Codex #12 in the plan); `deny.toml` therefore does not ban `openssl-sys`.
- `[workspace.lints]` carries the standard's tables plus two pedantic allows with reasons (`struct_excessive_bools`, `return_self_not_must_use`).
