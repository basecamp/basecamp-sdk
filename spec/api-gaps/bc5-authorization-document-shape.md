---
gap: bc5-authorization-document-shape
status: partial-coverage
detected: 2026-08-05
sdk_demand: medium
bc3_pr: 9471
bc3_refs:
  introduced_in: BC3 #9471, the modern OAuth 2.1 stack (merged eac8b2b476)
  routes:
    - GET /authorization.json
    - GET /.well-known/oauth-authorization-server
    - GET /.well-known/oauth-protected-resource
  controllers:
    - app/controllers/authorizations_controller.rb
  views:
    - app/views/api/authorizations/show.json.jbuilder
  related_existing_api:
    - authorization.getInfo() / AuthorizationService (hand-written, not in openapi.json)
    - Oauth discovery (hand-written, SPEC §16)
---

# BC3 now serves its own `authorization.json`, and its shape is not Launchpad's

## What's missing

BC3 **#9471** ships a full OAuth 2.1 authorization server, and this brief is
about the **one half of it that diverges**. The discovery half does not: bc3's new
`.well-known/oauth-authorization-server` (RFC 8414) and
`.well-known/oauth-protected-resource` (RFC 9728) are the exact paths the SDK's
hand-written discovery already fetches, in every language that has it —
`go/pkg/basecamp/oauth/discovery.go:24-25`,
`typescript/src/oauth/discovery.ts:400,514`,
`python/src/basecamp/oauth/discovery.py:223,301`,
`ruby/lib/basecamp/oauth/discovery.rb:88`. Those match; no action.

What diverges is the **authorization document**. bc3 now draws `resource
:authorization, only: :show` and renders it from
`app/views/api/authorizations/show.json.jbuilder`, so a BC5 issuer serves its own
`GET /authorization.json` instead of the caller falling back to Launchpad. The
two documents are not the same shape:

| Field | Launchpad (what the SDK models) | bc3 `show.json.jbuilder` |
|---|---|---|
| `identity.id` | present | present |
| `identity.first_name` / `last_name` / `email_address` | present | **absent** |
| `accounts[].id` / `name` / `href` | present | present |
| `accounts[].product` | present (`"bc3"`, `"hey"`, …) | **absent** |
| `accounts[].app_href` | present | **absent** |
| `accounts[].resource` | absent | **present** (`Oauth::ResourceIndicator`) |
| `scope` | absent | present, but only for BC3-issued tokens |
| `expires_at` | ISO-8601 string | **integer epoch seconds** (`.to_i`) |

This is reachable today, not hypothetical. Ruby's
`Http#get_authorization_document` runs resource-first discovery against the
client's own base URL and, when a BC5 issuer is selected, fetches
`"#{issuer_origin}/authorization.json"` — bc3's document, by construction.
Launchpad is only the soft-fallback path.

Blast radius differs per SDK, and the untyped ones are the lucky ones:

- **Ruby** reaches the bc3 document and returns it as a raw `Hash`, so nothing
  raises — but `AuthorizationService#get`'s own `@example` shows
  `auth["identity"]["email_address"]`, which is `nil` there. Documentation is
  wrong, code is not.
- **TypeScript** is the exposed one. `Identity.emailAddress`, `firstName` and
  `lastName` are typed **required `string`** and would be `undefined` at runtime
  (`typescript/src/services/authorization.ts`, and the same mapping again in
  `typescript/src/oauth/identity.ts`). `filterProduct` filters on
  `a.product === …`, which matches nothing when `product` is absent — so the
  documented "pick the account with `product: bc3`" flow silently returns an
  empty list. Worst of the three: `new Date(raw.expires_at)` on an **integer**
  is interpreted as milliseconds, so an epoch-seconds value becomes a 1970 date
  rather than throwing. Both call sites currently default to Launchpad, so this
  bites only a caller who passes `endpoint:` — but that is the documented way to
  point at a BC5 issuer.
- **Go** already tolerates the timestamp: `FlexTime.UnmarshalJSON`
  (`go/pkg/basecamp/authorization.go:25-43`) tries integer Unix first, then
  RFC 3339. Its string fields degrade to `""`.

  > **Correction.** This brief originally said Go's "endpoint is Launchpad-only
  > today". That was wrong. `GetInfoOptions.Endpoint` overrides it exactly as
  > TypeScript's `endpoint:` option does, so Go reaches bc3's document by the
  > same documented route — and `FilterProduct` therefore had the same
  > empty-list defect TypeScript did, which the "Go already tolerates it"
  > reading hid. Both are fixed together; see below.
- **Python** returns a raw `dict` from a hardcoded `LAUNCHPAD_AUTHORIZATION_URL`,
  so it neither reaches the document nor mistypes it.
- **Kotlin and Swift** have no authorization-document service at all.

The SDK is not *wrong* to be here — OAuth is deliberately outside the OpenAPI
spec (AGENTS.md), so none of this is generated and none of it drifted from
Smithy. `spec/bc3-route-allowlist.yml`'s `GET /authorization` entry keeps its
`out_of_scope` disposition for that reason; only its *reasoning* was corrected at
this repin, because it asserted the route is served solely from Launchpad, which
was true when written.

## Why it matters

This is the **first** request an SDK consumer makes after obtaining a token — it
is how they find their account `href` and therefore the base URL for everything
else. A shape mismatch here does not surface as a clean error; it surfaces as
`undefined` identity fields, an empty account list after a `product` filter, and
a token that looks like it expired in 1970. Each of those reads as a credential
problem rather than a schema problem, which is the expensive kind of bug.

It also matters for the direction of travel: BC5 issuers serving their own
authorization document is the intended end state, so the Launchpad shape is the
one that will become the exception. Modelling only Launchpad means the SDK is
typed against the *legacy* half of a migration bc3 has already made.

## Suggested API shape

The divergence is bc3's to settle first, and there are two defensible answers:

- **Converge on Launchpad's shape** — have `show.json.jbuilder` emit
  `first_name`, `last_name`, `email_address`, `product` and `app_href`, and
  `expires_at` as ISO-8601. Then one SDK type covers both issuers and this brief
  closes with no SDK change beyond a fixture.
- **Declare them deliberately different documents** — keep bc3's leaner shape
  (arguably correct: it drops the PII the docs already say not to use for
  identifying users, and `accounts[].resource` is the RFC 8707 indicator a
  Launchpad document has no reason to carry) and document both, so the SDK can
  model a union rather than guessing.

Either way `expires_at` should be settled explicitly, because an integer and an
ISO-8601 string are not distinguishable by a permissive parser without exactly
the kind of `FlexTime` special-casing Go already needed.

## Implementation notes for BC3

- Decide and document which of the two answers above applies. Today
  `doc/api/sections/authentication.md` documents only the Launchpad URL and its
  payload, so bc3's own document is undocumented API surface — renderable under
  `app/views/api`, absent from `doc/api`, which is why it never appears in
  `spec/bc3-routes.json`.
- If the shapes stay different, say so in `authentication.md` next to the
  existing example, including that `scope` appears only for BC3-issued tokens
  ("Legacy Signal tokens predate scopes"), so a consumer does not treat its
  absence as an error.
- `expires_at` as `.to_i` is the single most consequential line; if converging,
  change it to `to_formatted_s(:iso8601)` to match the documented example.

## SDK absorption plan when this lands

- Make `expires_at` parsing permissive in every typed SDK, not just Go: accept
  integer epoch seconds **and** RFC 3339. TypeScript's `new Date(...)` must
  branch on `typeof`, because its failure mode is a wrong date rather than an
  exception.
- Relax `Identity.firstName`/`lastName`/`emailAddress` and
  `AuthorizedAccount.product`/`appHref` to optional in TypeScript, and fix the
  two duplicated mappings (`services/authorization.ts` and `oauth/identity.ts`)
  together — they are the same shape written twice, which is how one of them
  would be missed.
- Make `filterProduct` explicit about absent `product`: either skip filtering
  with a documented warning or treat a missing `product` as non-matching by
  design. Silently returning an empty list is the current behaviour and the worst
  of the three.
- Correct Ruby's `AuthorizationService#get` `@example`, which shows
  `email_address`.
- Add fixtures for **both** documents and assert the SDK handles each, including
  the epoch-seconds `expires_at`. One fixture will not catch this class of bug.
- Consider whether Kotlin and Swift should gain an authorization-document
  surface; they have discovery but no way to read the document discovery points
  them at.

## What shipped (#681)

The SDK half of this brief is closed. `status` stays `partial-coverage` because
the other half is bc3's and has not moved: bc3 still does not document its own
authorization document, so `doc/api/sections/authentication.md` describes only
Launchpad's payload and the two shapes are still undeclared. Nothing here is
generated — OAuth is outside the OpenAPI spec by design — so there are no
`smithy_refs` to point at and `absorbed-in-sdk` would fail
`scripts/validate-api-gaps.rb` for exactly that reason.

**TypeScript** — the two duplicated mappings became one shared parser,
`typescript/src/oauth/authorization-document.ts`, imported by both
`services/authorization.ts` and `oauth/identity.ts`. It is a leaf module: it
imports nothing from the SDK, so `identity.ts` taking a *value* dependency on it
adds no runtime edge to the known `base.ts`/`client.ts` cycle. In it:

- `expires_at` branches on `typeof`. A number is epoch **seconds**
  (`new Date(n * 1000)`); v0.12.0's `new Date(n)` read it as milliseconds and
  produced a 1970 date. `0` — bc3's `.to_i` rendering of a nil expiry, never
  `null` — parses to an Invalid Date rather than to 1970.
- `Identity.firstName`/`lastName`/`emailAddress` and
  `AuthorizedAccount.product`/`appHref` are optional; `resource` and a
  top-level `scope` are new. This is a **breaking type change** and has a
  `MIGRATING.md` entry.
- `filterProduct` takes the third of the brief's three options: when *no*
  account carries a `product`, the filter is inapplicable, all accounts are
  returned, and `AuthorizationInfo.productFilterApplied` is `false`. When at
  least one does, the filter applies unchanged, so an empty result still means
  "nothing matched" — the two situations stay distinguishable, which returning
  `[]` for both did not allow.

**Go** — `AuthorizedAccount.Resource` and `AuthorizationInfo.Scope` added
(`,omitempty`), plus the same `FilterProduct` correction and
`ProductFilterApplied`. `ExpiresAt` is deliberately untouched: **issue #662**
owns pointerizing it (it currently fabricates `0001-01-01T00:00:00Z`) and
widening the `isValueTime` guard, and `FlexTime` already decodes both spellings,
so the brief's Go timestamp ask was met before this. Doing it here would collide
with a breaking change #662 should make on its own terms.

**Ruby** — `AuthorizationService#get`'s `@example` no longer reads
`identity.email_address`, which is `nil` against bc3. The sharper fix is in the
tests: `test_helper.rb`'s `sample_authorization` is Launchpad-shaped and the
BC5-issuer tests were being served it, so *the tests proving the SDK reaches
bc3's issuer were feeding it Launchpad's body* and would have passed even if the
SDK could not read a BC5 document. A `sample_bc5_authorization` helper now backs
those, plus one test pinning the three BC5-only fields through the round trip.

**Python, Kotlin, Swift — considered, no change.** Recorded here so the next
reader does not re-derive it. Python returns a raw `dict` from a hardcoded
`LAUNCHPAD_AUTHORIZATION_URL`: it neither reaches bc3's document nor types it, so
there is nothing to relax and no reachable defect. Kotlin and Swift have no
authorization-document surface at all — the last bullet above, giving them one,
is still open and is a feature, not a fix.

Both fixtures asked for by the absorption plan exist: the pre-existing
Launchpad-shaped ones and new BC5-shaped ones in all three SDKs that have a
typed or tested surface, each asserting the epoch-seconds `expires_at`.
