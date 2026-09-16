# Event-feed srv2 digest and checkpoint flat-key vectors

The srv2 vector table is BC3's published contract (`doc/api/sections/event_feed.md`,
"Filter digests (srv2)", on bc3 master — the shipped feed). The eleven digests here were
additionally recomputed independently from the published canonicalization —
`SHA-256(canonical_json)[0:16]` — and match byte-for-byte. srv2 succeeded the pre-merge
srv1 scheme (a positional three-element array) when the feed shipped with more filter
dimensions than the array could name; any further change to the canonicalization or
algorithm ships as srv3 with new vectors.

## What lives here

One fixture file, `fixtures/srv2-vectors.json`, validated against `schema.json` by
`make event-feed-digest-fixtures-check`:

- **`srv2_vectors`** — the published eleven-vector table (empty set; single type; unsorted
  multi-list; three dimensions; `"01"` post-coercion dedup; the 100-id cap boundary;
  `performers`; `exclude_performers`; `actor_types`; and two `reasons` vectors, the inbox
  lane's dimension), each carrying the input filters, the exact canonical JSON, and the
  16-hex digest.
- **`flat_key_cases`** — `{origin, account_id, consumer_namespace, filters}` →
  `filter_key` (`srv2-<hex>`) → the compact RFC 8259 JSON-array flat key, covering origin
  canonicalization (lowercase scheme/host, default port stripped, non-default port
  preserved) and the loop-guard filter set an agent runs with.

Every SDK asserts every case (Go `digest_test.go` first; the other five as their
connectors land). The srv2 algorithm is **total over client-validated inputs** — catalog
membership is server-owned and never client-validated (SPEC §23 "Checkpoint Identity"),
so no quoted-string or non-ASCII vector exists here by design: the server rejects unknown
types, actor types, and reasons with the filter 400 before computing any digest. The
server's `self` literal never appears in canonical JSON either — it is resolved to an id
before digesting — which is why the `performers=9` vector is also the vector for
`performers=self` when the effective actor is person 9.

## Directory is a schema boundary

This directory holds exactly one shape (digest/flat-key vectors). Scenario scripts live
in the sibling `conformance/event-feed/`; operation-dispatch cases live in
`conformance/tests/`. A new shape gets a new sibling directory, never a second schema
here (the oauth/oauth-token precedent).
