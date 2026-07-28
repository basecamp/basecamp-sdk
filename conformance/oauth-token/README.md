# OAuth token wire-behavior fixtures

Data-only, cross-language fixtures for token-endpoint **wire behavior**: the
RFC 8707 `resource` echo on refresh requests and the decode rules for a token
response's `resource` member (round-trip, absent/JSON-null as unset,
present-empty/non-string rejected). See SPEC.md §16, "Token Response `resource`
Indicator".

A separate family from `conformance/oauth/` — that schema is discovery-only
and every discovery harness globs its whole fixtures directory, so token cases
here would break them.

Consumers:

- **Go** `go/pkg/basecamp/oauth/token_conformance_test.go`
- **TypeScript** `typescript/tests/oauth/token-conformance.test.ts`
- **Python** `python/tests/oauth/test_token_conformance.py`
- **Ruby** `ruby/test/basecamp/oauth_token_conformance_test.rb`
- **Kotlin** mirrors the scenarios in code (`OAuthTest.kt`), its established
  pattern for the discovery fixtures.

Schema-validated by `make oauth-token-fixtures-check` (part of `make
conformance`, so `make check` gates it).

**Deliberately out of scope:** response-omission *preservation* — carrying a
stored `resource` forward when a refresh response omits it — is lifecycle
behavior owned by token managers, tested per-manager (TS `TokenManager`, Go
`AuthManager`, basecamp-cli), not modeled as wire fixtures. 429 poll scheduling
likewise stays in per-SDK device tests.
