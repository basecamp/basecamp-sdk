---
gap: bc5-authorization-document-shape
status: covered-outside-spec
detected: 2026-08-05
sdk_demand: medium
bc3_pr: 12646
bc3_refs:
  introduced_in: "BC3 #9471, the modern OAuth 2.1 stack (merged eac8b2b476)"
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

The SDK half of this brief closed here. `status` stayed `partial-coverage` at
the time because the other half was bc3's and had not moved: bc3 did not yet
document its own authorization document, so `doc/api/sections/authentication.md`
described only Launchpad's payload and the two shapes were undeclared. That
half has since closed too — see "Closed (bc3 #12646)" below. Nothing here is
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
- `filterProduct` takes the third of the brief's three options: when the
  document carries at least one account and *none* of them carries a `product`,
  the filter is inapplicable, all accounts are returned, and
  `AuthorizationInfo.productFilterApplied` is `false`. When at least one does,
  the filter applies unchanged, so an empty result still means "nothing
  matched" — the two situations stay distinguishable, which returning `[]` for
  both did not allow. An **empty** account list reports `applied: true`:
  Launchpad returns one for an identity with no currently accessible accounts,
  the result is empty either way, and `false` there would assert "this issuer
  cannot filter by product" on no evidence.

**Go** — `AuthorizedAccount.Resource` and `AuthorizationInfo.Scope` added
(`,omitempty`), plus the same `FilterProduct` correction and
`ProductFilterApplied`. `ExpiresAt` was deliberately untouched here and closed
by **issue #662** on its own terms — not by pointerizing (`encoding/json`
allocates on any non-null token, so a wire `0` can never become a nil pointer
without struct-level machinery) but by a zero-time sentinel: `FlexTime` keeps
its value type, absent / `null` / `0` all decode to the zero time, the zero
marshals back as `null` instead of fabricating `0001-01-01T00:00:00Z`, and
`AuthorizationInfo.Expiry() (time.Time, bool)` is the documented front door.
The `isValueTime` guards were widened to recognize the named time wrappers,
plus a behavioral guard that every named wrapper marshals its zero as null —
the AST guards structurally cannot reach `ExpiresAt` (no generated
counterpart, bare tag), so the marshaling contract is what protects it.

One premise correction against bc3 source: a nil expiry rendering as `0` —
claimed for TS above — is unreachable today. BC3 tokens carry `null: false`
plus presence validation and are always set at mint (10-year PATs are the
"non-expiring" case and still carry real timestamps); legacy Signal tokens
lazily self-default (`expires_at ||= expires_in.from_now`). So `.to_i` has
never emitted `0` in production and Launchpad never omits the field either;
the `0` handling on both sides is defense-in-depth against the RFC 7591
collision (bc3's own `client_secret_expires_at: 0` means "never expires" — the
exact inverse of "expired at epoch"), not a live-bug fix.

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

## Closed (bc3 #12646)

bc3 **#12646** (merged `71b43f3d9fa`, deployed to production 2026-08-11) closed
the bc3 half:

- **`expires_at` converged on ISO 8601.** The view renders the raw `Time`
  instead of `.to_i` — the epoch integer was the only one in bc3's entire
  public JSON API, incidental to RFC 7662's mandatory `exp` written in the same
  PR. The self-referential controller-test assertions were replaced with
  shape assertions (`String` + `Time.iso8601` parse-back), proven failing
  against the old view. **No SDK change needed**: Go's `FlexTime` and TS's
  `parseExpiresAt` accept both spellings; Ruby passes the value through
  untyped; Python never reaches bc3's document at all (hardcoded to the
  Launchpad URL — see above), so nothing changes for it.
- **bc3 documents its own document.** `doc/api/sections/authentication.md`
  gained a "Get authorization from Basecamp" section — both token types, the
  RFC 8707 `resource` indicator, the `scope` presence rule (every
  Basecamp-issued token carries one, PATs included; legacy tokens predate
  scopes), the identity-id-only shape, the DPoP-bound request form, ISO 8601
  `expires_at`. Mirrored to the public repo via bc-api #435.

The status is `covered-outside-spec` — the registry's terminal state for
surfaces the architecture deliberately keeps outside the OpenAPI spec, added
for this entry as its first instance. No absorption PR will follow and none is
pending: `absorbed-in-sdk` can never apply (no `smithy_refs` can exist), and
the coverage claim points instead at the sanctioned hand-written surface named
in `bc3_refs.related_existing_api`, which the validator now requires for this
status.

Left open upstream, deliberately, as flagged on #12646:

- Whether bc3's document should carry Launchpad's `accounts[].product` /
  `app_href` — a product decision (resource indicators vs product selection).
- `href` renders `BC3.uri` (the web origin) while Launchpad's contract — and
  the doc example — use the account's API base. Raised on the PR with a
  recommended fix (`BC3.protocol` + `BC3.api_host`); the owner resolved the
  thread, so the call on if/when to converge it is made upstream, not here.

The wider question both instances belong to, named on the PR: which fields of
bc3's authorization document are contract-bound to mirror Launchpad's — one
decision, not per-field rounds.
