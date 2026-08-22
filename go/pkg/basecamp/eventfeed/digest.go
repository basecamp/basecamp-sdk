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
// published srv1 digest contract. The server wire format is the bare hex
// digest — exactly what the 409 body's position_digest/filters_digest carry;
// the server never emits this prefix.
const filterKeyNamespace = "srv1-"

// Digest returns the srv1 filter digest (SPEC.md §23 "Checkpoint Identity"):
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
// "srv1-" + the bare server digest.
func (f Filters) FilterKey() string {
	return filterKeyNamespace + f.Digest()
}

// canonicalJSON hand-builds the srv1 canonical filter form,
// "[" T "," B "," C "]", compact with no whitespace anywhere:
//
//	T = null if no types,   else a JSON array of the type strings, deduped,
//	    sorted bytewise-ascending over their UTF-8 encodings
//	B = null if no buckets, else a JSON array of integers: deduped after
//	    base-10 coercion, numerically ascending, canonical integer rendering
//	C = same as B, for creators
//
// The empty filter set is exactly [null,null,null]. The bytes are hand-built
// rather than json.Marshal'd because no language's default JSON emitter is
// load-bearing — encoding/json HTML-escapes <, >, and & by default.
func (f Filters) canonicalJSON() string {
	var b strings.Builder
	b.WriteByte('[')
	writeCanonicalTypes(&b, f.Types)
	b.WriteByte(',')
	writeCanonicalIDs(&b, f.Buckets)
	b.WriteByte(',')
	writeCanonicalIDs(&b, f.Creators)
	b.WriteByte(']')
	return b.String()
}

func writeCanonicalTypes(b *strings.Builder, types []string) {
	if len(types) == 0 {
		b.WriteString("null")
		return
	}
	sorted := slices.Clone(types)
	slices.Sort(sorted) // bytewise ascending over the UTF-8 encodings
	sorted = slices.Compact(sorted)
	b.WriteByte('[')
	for i, typ := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSONString(b, typ)
	}
	b.WriteByte(']')
}

func writeCanonicalIDs(b *strings.Builder, ids []int64) {
	if len(ids) == 0 {
		b.WriteString("null")
		return
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)             // numerically ascending
	sorted = slices.Compact(sorted) // dedup after coercion: "1" and "01" are one id
	b.WriteByte('[')
	for i, id := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		// Canonical integer rendering: no sign for positives, no leading
		// zeros, no fraction, no exponent.
		b.WriteString(strconv.FormatInt(id, 10))
	}
	b.WriteByte(']')
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
