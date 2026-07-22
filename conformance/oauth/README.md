# OAuth resource-first discovery fixtures

Data-only, cross-language fixtures for BC5's resource-first OAuth discovery
(SPEC.md §16 "Resource-First Discovery"). Unlike `conformance/tests/*.json`
(operation-dispatch, driven by the language runners), these describe discovery
**scenarios** — the two well-known documents a mock serves and the expected
selection/fallback/raise outcome.

## Validation

`schema.json` is the contract. `make oauth-fixtures-check` validates every
fixture against it with a pinned `check-jsonschema` (run through `uvx`, so no
global install is required).

## Placeholder substitution

Fixtures are host-agnostic. Each SDK's discovery test substitutes these tokens
with its own mock origins **before** driving the scenario, so issuer/resource
binding stays code-point-exact against whatever ephemeral host the mock listens
on:

| Placeholder | Meaning |
|---|---|
| `{{RESOURCE_ORIGIN}}` | The API/resource host (hop-1 `oauth-protected-resource`). |
| `{{ISSUER_ORIGIN}}` | A generic issuer host for direct `discover` (hop-2) cases. |
| `{{LAUNCHPAD_ORIGIN}}` | The Launchpad AS host (fallback target). |
| `{{BC5_ISSUER}}` | BC5's canonical issuer (web host). |

Literal origins (e.g. `http://[::1]:3000`, `https://attacker.example.com`) are
intentional and must **not** be substituted.

## Fields

See `schema.json` for the authoritative shape. Each fixture names an
`operation` (`discoverFromResource` | `discoverProtectedResource` | `discover`),
optional `hop1`/`hop2` mock exchanges, and an `expect` block. Hard cases set
`expect.launchpadContacted: false` — the harness must assert the Launchpad host
received **zero** requests.
