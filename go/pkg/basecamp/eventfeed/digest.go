package eventfeed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// filterKeyNamespace is the SDK-side checkpoint-lineage namespace for the
// published srv2 digest contract. The server wire format is the bare hex
// digest — exactly what the 409 body's position_digest/filters_digest carry;
// the server never emits this prefix. Any change to the server's
// canonicalization or algorithm ships as srv3 with new vectors, and this
// namespace moves with it so an old lineage goes cold rather than resuming
// under a digest that no longer means the same filter set.
const filterKeyNamespace = "srv2-"

// Digest returns the srv2 filter digest (SPEC.md §23 "Checkpoint Identity"):
//
//	lowercase_hex(SHA-256(UTF-8(canonical_json)))[0:16]
//
// 16 hex characters = the digest's first 8 bytes. The algorithm is total over
// client-validated inputs: it is computed for any filter set that passes
// construction validation — catalog membership is server-owned and never
// client-validated.
func (f Filters) Digest() string {
	sum := sha256.Sum256([]byte(f.canonicalJSON()))
	return hex.EncodeToString(sum[:8])
}

// FilterKey returns the checkpoint-lineage filter key,
// "srv2-" + the bare server digest.
func (f Filters) FilterKey() string {
	return filterKeyNamespace + f.Digest()
}

// canonicalJSON hand-builds the srv2 canonical filter form: a JSON object
// keyed by dimension name, PRESENT dimensions only, keys sorted bytewise
// (actor_types, buckets, creators, exclude_performers, performers, reasons,
// types), compact with no whitespace anywhere. String dimensions are deduped
// and sorted bytewise-ascending over their UTF-8 encodings; id dimensions are
// deduped after base-10 coercion ("1" and "01" are one id) and sorted
// numerically ascending, canonical integer rendering. The empty filter set is
// exactly {}.
//
// Keying by name is what makes the scheme extension-stable: an absent
// dimension contributes no bytes, so a filter dimension introduced later
// never moves the digest of a set that does not use it. The bytes are
// hand-built rather than json.Marshal'd because no language's default JSON
// emitter is load-bearing — encoding/json HTML-escapes <, >, and & by
// default.
func (f Filters) canonicalJSON() string {
	var b strings.Builder
	b.WriteByte('{')
	c := canonicalWriter{b: &b}
	c.strings("actor_types", f.ActorTypes)
	c.ids("buckets", f.Buckets)
	c.ids("creators", f.Creators)
	c.ids("exclude_performers", f.ExcludePerformers)
	c.ids("performers", f.Performers)
	c.strings("reasons", f.Reasons)
	c.strings("types", f.Types)
	b.WriteByte('}')
	return b.String()
}

// canonicalWriter emits srv2 dimensions in the order it is called — the
// caller supplies the bytewise key order — separating members with commas
// and skipping absent dimensions entirely.
type canonicalWriter struct {
	b     *strings.Builder
	wrote bool
}

func (c *canonicalWriter) key(name string) {
	if c.wrote {
		c.b.WriteByte(',')
	}
	c.wrote = true
	writeJSONString(c.b, name)
	c.b.WriteByte(':')
}

func (c *canonicalWriter) strings(name string, values []string) {
	if len(values) == 0 {
		return
	}
	c.key(name)
	sorted := slices.Clone(values)
	slices.Sort(sorted) // bytewise ascending over the UTF-8 encodings
	sorted = slices.Compact(sorted)
	c.b.WriteByte('[')
	for i, v := range sorted {
		if i > 0 {
			c.b.WriteByte(',')
		}
		writeJSONString(c.b, v)
	}
	c.b.WriteByte(']')
}

func (c *canonicalWriter) ids(name string, ids []int64) {
	if len(ids) == 0 {
		return
	}
	c.key(name)
	sorted := slices.Clone(ids)
	slices.Sort(sorted)             // numerically ascending
	sorted = slices.Compact(sorted) // dedup after coercion: "1" and "01" are one id
	c.b.WriteByte('[')
	for i, id := range sorted {
		if i > 0 {
			c.b.WriteByte(',')
		}
		// Canonical integer rendering: no sign for positives, no leading
		// zeros, no fraction, no exponent.
		c.b.WriteString(strconv.FormatInt(id, 10))
	}
	c.b.WriteByte(']')
}

// writeJSONString emits s as an RFC 8259 minimally escaped JSON string: only
// `"`, `\`, and control characters U+0000–U+001F are escaped — no HTML
// escaping, no \uXXXX for non-control characters.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r < 0x20:
			switch r {
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				fmt.Fprintf(b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}
