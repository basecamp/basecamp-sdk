# Event-feed srv1 digest and checkpoint flat-key vectors

**PROVISIONAL(`8be5c67de5`).** The srv1 vector table is BC3's published contract, verified
at that branch head; it freezes only when BC3 regenerates vectors at its merge-time gate
(SPEC §23 "Provenance", class 1). The five digests here were additionally recomputed
independently from the published canonicalization — `SHA-256(canonical_json)[0:16]` —
and match byte-for-byte.

## What lives here

One fixture file, `fixtures/srv1-vectors.json`, validated against `schema.json` by
`make event-feed-digest-fixtures-check`:

- **`srv1_vectors`** — the published five-vector table (empty set; single type; unsorted
  multi-list; `"01"` post-coercion dedup; the 100-id cap boundary), each carrying the
  input filters, the exact canonical JSON, and the 16-hex digest.
- **`flat_key_cases`** — `{origin, account_id, consumer_namespace, filters}` →
  `filter_key` (`srv1-<hex>`) → the compact RFC 8259 JSON-array flat key, covering origin
  canonicalization (lowercase scheme/host, default port stripped, non-default port
  preserved).

Every SDK asserts every case (Go `digest_test.go` first; the other five as their
connectors land). The srv1 algorithm is **total over client-validated inputs** — catalog
membership is server-owned and never client-validated (SPEC §23 "Checkpoint Identity"),
so no quoted-string or non-ASCII vector exists here by design: the server rejects unknown
types with the filter 400 before computing any digest.

## Directory is a schema boundary

This directory holds exactly one shape (digest/flat-key vectors). Scenario scripts live
in the sibling `conformance/event-feed/`; operation-dispatch cases live in
`conformance/tests/`. A new shape gets a new sibling directory, never a second schema
here (the oauth/oauth-token precedent).
