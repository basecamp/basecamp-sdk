package basecamp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// This file is the oracle for "what does Go read an integer-shaped person id
// as", for this SDK and for the six ports held to it.
//
// THREE RULES LIVE IN THIS REPOSITORY FOR ONE QUESTION, AND ONLY TWO OF THEM
// ARE SUPPOSED TO AGREE.
//
//	Rule A  PersonIDFromSGID (mentions.go) walks the bytes and refuses anything
//	        outside 0..=9 BEFORE parsing, then refuses id <= 0. It therefore
//	        rejects the leading "+" the other two accept. This is correct at
//	        that site and is pinned as such by TestPersonIDGidRuleStaysApart:
//	        loosening it is the defect #886 closed, where
//	        gid://bc3/Person/+77 began naming person 77.
//	Rule B  types.FlexibleInt64 (flexible_int64.go) takes strconv.ParseInt
//	        whole, with no pre-walk.
//	Rule C  coercePersonID (normalize.go), the pre-decode normalizer. It is
//	        meant to be Rule B applied one layer earlier, and
//	        TestPersonIDNormalizeMatchesFlexible holds it to exactly that.
//
// So "match Go" is ambiguous until it says WHICH Go, and a port that unifies
// any two of these introduces a defect. The corpus below is the evidence: run
// it with ORACLE_OUT set (see TestPersonIDOracleDump) to re-derive the verdicts
// the ports pin, rather than copying a table out of a comment.

type idOutcome int

const (
	// idValue: ParseInt accepted, and the id is that number.
	idValue idOutcome = iota
	// idSentinel: ParseInt refused on syntax, so the id is Go's non-numeric
	// sentinel 0 and the original string is kept as system_label. 0 is the
	// SYSTEM ACTOR (LocalPerson, "basecamp", "campfire"), which is why a port
	// that lands here for a value Go reads as a number names the wrong actor.
	idSentinel
	// idRefused: ParseInt refused on range, so the string is left untouched
	// and the read fails rather than a sentinel being substituted.
	idRefused
)

type personIDCase struct {
	raw     string
	outcome idOutcome
	value   int64
}

// personIDCorpus holds the shapes that DISCRIMINATE between the rules — a
// corpus is only evidence about what it contains, so a sweep of cases that
// cannot fail is not evidence at all. Groups, and what each is here to catch:
//
//   - a leading "+" and leading zeros: valid for ParseInt, INVALID in the JSON
//     number grammar, which is the whole of the card-34 defect, and refused by
//     Rule A, which is the deliberate disagreement.
//   - "010" is TEN. Ruby's Integer() detected octal and said eight — not a
//     refusal against an acceptance but two different people from one wire
//     value, either of which a caller could then mention.
//   - Unicode decimal digits, underscores and surrounding whitespace: accepted
//     by Python's int(), by Kotlin's toLongOrNull, and by an ICU-flavoured \d.
//     ParseInt accepts none of them.
//   - "18446744073709551615x" against "18446744073709551616x", one digit apart
//     and on opposite sides of the scan-order boundary: ParseUint checks the
//     magnitude INSIDE the loop, so the second overflows before the scan ever
//     reaches the junk. Syntax for the first, range for the second, and the
//     bound is u64::MAX rather than i64::MAX.
//   - 2^53 and its neighbours: real int64 ids that JavaScript's number cannot
//     hold, which is where the TypeScript port has to answer for itself.
var personIDCorpus = []personIDCase{
	{raw: "7", outcome: idValue, value: 7},
	{raw: "0", outcome: idValue, value: 0},
	{raw: "-0", outcome: idValue, value: 0},
	{raw: "+0", outcome: idValue, value: 0},
	{raw: "+7", outcome: idValue, value: 7},
	{raw: "-7", outcome: idValue, value: -7},
	{raw: "007", outcome: idValue, value: 7},
	{raw: "+007", outcome: idValue, value: 7},
	{raw: "-007", outcome: idValue, value: -7},
	{raw: "0009223372036854775807", outcome: idValue, value: 9223372036854775807},
	{raw: "0000000000000000000000009", outcome: idValue, value: 9},
	{raw: "", outcome: idSentinel},
	{raw: " ", outcome: idSentinel},
	{raw: "+", outcome: idSentinel},
	{raw: "-", outcome: idSentinel},
	{raw: " 7", outcome: idSentinel},
	{raw: "7 ", outcome: idSentinel},
	{raw: " 7 ", outcome: idSentinel},
	{raw: "\n7", outcome: idSentinel},
	{raw: "7\n", outcome: idSentinel},
	{raw: "\t7", outcome: idSentinel},
	{raw: "7\t", outcome: idSentinel},
	{raw: "1_0", outcome: idSentinel},
	{raw: "1_2", outcome: idSentinel},
	{raw: "0x10", outcome: idSentinel},
	{raw: "0b11", outcome: idSentinel},
	{raw: "0o17", outcome: idSentinel},
	{raw: "010", outcome: idValue, value: 10},
	{raw: "0X1F", outcome: idSentinel},
	{raw: "7x", outcome: idSentinel},
	{raw: "x7", outcome: idSentinel},
	{raw: "12.0", outcome: idSentinel},
	{raw: "1e3", outcome: idSentinel},
	{raw: "12,3", outcome: idSentinel},
	{raw: "basecamp", outcome: idSentinel},
	{raw: "campfire", outcome: idSentinel},
	{raw: "LocalPerson", outcome: idSentinel},
	{raw: "\uff11\uff12\uff13", outcome: idSentinel},
	{raw: "\uff17", outcome: idSentinel},
	{raw: "\u0660\u0661\u0662", outcome: idSentinel},
	{raw: "\u09ed", outcome: idSentinel},
	{raw: "\u06f7", outcome: idSentinel},
	{raw: "9223372036854775806", outcome: idValue, value: 9223372036854775806},
	{raw: "9223372036854775807", outcome: idValue, value: 9223372036854775807},
	{raw: "9223372036854775808", outcome: idRefused},
	{raw: "9223372036854775809", outcome: idRefused},
	{raw: "-9223372036854775807", outcome: idValue, value: -9223372036854775807},
	{raw: "-9223372036854775808", outcome: idValue, value: -9223372036854775808},
	{raw: "-9223372036854775809", outcome: idRefused},
	{raw: "18446744073709551614", outcome: idRefused},
	{raw: "18446744073709551615", outcome: idRefused},
	{raw: "18446744073709551616", outcome: idRefused},
	{raw: "18446744073709551615x", outcome: idSentinel},
	{raw: "18446744073709551616x", outcome: idRefused},
	{raw: "1844674407370955161x", outcome: idSentinel},
	{raw: "-18446744073709551615x", outcome: idSentinel},
	{raw: "-18446744073709551616x", outcome: idRefused},
	{raw: "99999999999999999999999", outcome: idRefused},
	{raw: "99999999999999999999999x", outcome: idRefused},
	{raw: "00000000000000000000018446744073709551616", outcome: idRefused},
	{raw: "0000000000000000000009223372036854775807", outcome: idValue, value: 9223372036854775807},
	{raw: "9007199254740991", outcome: idValue, value: 9007199254740991},
	{raw: "9007199254740992", outcome: idValue, value: 9007199254740992},
	{raw: "9007199254740993", outcome: idValue, value: 9007199254740993},
	{raw: "90071992547409931", outcome: idValue, value: 90071992547409931},
	{raw: "-9007199254740993", outcome: idValue, value: -9007199254740993},
	{raw: "+9223372036854775807", outcome: idValue, value: 9223372036854775807},
	{raw: "+9223372036854775808", outcome: idRefused},
	{raw: "00", outcome: idValue, value: 0},
	{raw: "0000", outcome: idValue, value: 0},
	{raw: "-00", outcome: idValue, value: 0},
	{raw: "\u0660", outcome: idSentinel},
	{raw: "\u09ed7", outcome: idSentinel},
	{raw: "7\u09ed", outcome: idSentinel},
}

// TestPersonIDNormalizeMatchesFlexible is the property card 34 asked for: the
// wrapper's pre-decode normalizer accepts exactly what FlexibleInt64 accepts,
// reads it as the same number, and fails the read exactly where FlexibleInt64
// errors.
//
// It did not, before. coercePersonID stored a ParseInt-valid string verbatim as
// a json.Number, and json.Marshal validates that against the JSON NUMBER
// grammar — no leading "+", no leading zeros — so normalizeEmbeddedPeopleJSON
// failed, every caller fell back to the RAW body, and the raw body still has a
// JSON string where Person.ID is a plain int64. Thirteen of the rows below made
// the wrapper reject a whole response that FlexibleInt64 reads without
// complaint: "+0", "+7", "007", "+007", "-007", "0009223372036854775807",
// "0000000000000000000000009", "010",
// "0000000000000000000009223372036854775807", "+9223372036854775807", "00",
// "0000" and "-00". Divergences over the corpus: 13 before, 0 now.
func TestPersonIDNormalizeMatchesFlexible(t *testing.T) {
	for _, tc := range personIDCorpus {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			// Rule B, read directly.
			var flexible types.FlexibleInt64
			flexErr := json.Unmarshal([]byte(strconv.Quote(tc.raw)), &flexible)

			// Rule C, through the wrapper path exactly as decodeGaugePayload
			// and the notification decoders run it: normalize, fall back to the
			// raw body when normalization fails, then decode onto Person.
			body := []byte(`{"creator":{"id":` + strconv.Quote(tc.raw) + `,"name":"x"}}`)
			normalized, normalizeErr := normalizeEmbeddedPeopleJSON(body)
			if normalizeErr != nil {
				normalized = body
			}
			var envelope struct {
				Creator Person `json:"creator"`
			}
			decodeErr := json.Unmarshal(normalized, &envelope)

			switch tc.outcome {
			case idValue:
				if flexErr != nil {
					t.Fatalf("FlexibleInt64 refused %q: %v; want %d", tc.raw, flexErr, tc.value)
				}
				if int64(flexible) != tc.value {
					t.Errorf("FlexibleInt64(%q) = %d, want %d", tc.raw, int64(flexible), tc.value)
				}
				if decodeErr != nil {
					t.Fatalf("wrapper path refused %q: %v; want %d", tc.raw, decodeErr, tc.value)
				}
				if envelope.Creator.ID != tc.value {
					t.Errorf("wrapper path read %q as %d, want %d", tc.raw, envelope.Creator.ID, tc.value)
				}
				if envelope.Creator.SystemLabel != "" {
					t.Errorf("wrapper path labelled a numeric id %q as a sentinel (%q)", tc.raw, envelope.Creator.SystemLabel)
				}
			case idSentinel:
				if flexErr != nil {
					t.Fatalf("FlexibleInt64 refused %q: %v; want the sentinel 0", tc.raw, flexErr)
				}
				if int64(flexible) != 0 {
					t.Errorf("FlexibleInt64(%q) = %d, want the sentinel 0", tc.raw, int64(flexible))
				}
				if decodeErr != nil {
					t.Fatalf("wrapper path refused %q: %v; want the sentinel 0", tc.raw, decodeErr)
				}
				if envelope.Creator.ID != 0 {
					t.Errorf("wrapper path read %q as %d, want the sentinel 0", tc.raw, envelope.Creator.ID)
				}
				if envelope.Creator.SystemLabel != tc.raw {
					t.Errorf("wrapper path kept system_label %q for %q, want the original string", envelope.Creator.SystemLabel, tc.raw)
				}
			case idRefused:
				if flexErr == nil {
					t.Fatalf("FlexibleInt64 read %q as %d; want a range error", tc.raw, int64(flexible))
				}
				var numErr *strconv.NumError
				if !errors.As(flexErr, &numErr) || numErr.Err != strconv.ErrRange {
					t.Errorf("FlexibleInt64(%q) refused for %v, want strconv.ErrRange", tc.raw, flexErr)
				}
				// The string is LEFT ALONE — no sentinel substituted — so the
				// decoder is what refuses. That is the difference between a
				// failed read and an oversized id silently becoming the system
				// actor, and it is the direction that matters.
				if decodeErr == nil {
					t.Errorf("wrapper path read %q as %d (system_label %q); want the read to fail",
						tc.raw, envelope.Creator.ID, envelope.Creator.SystemLabel)
				}
			}
		})
	}
}

// TestPersonIDGidRuleStaysApart pins the disagreement between Rule A and Rule B
// as a fact rather than an accident, in BOTH directions.
//
// A reader who tightens FlexibleInt64 to match the gid walk makes the same
// mistake as one who loosens the walk to match FlexibleInt64, and the second
// reintroduces the defect #886 closed. The rows below are the ones where the
// two genuinely answer differently about a REAL person — a sentinel 0 matches
// no sgid, so the many rows where Rule B reads 0 and Rule A refuses are the
// same answer by two routes and are not the interesting set.
func TestPersonIDGidRuleStaysApart(t *testing.T) {
	cases := []struct {
		raw  string
		flex int64 // what FlexibleInt64 reads
		why  string
	}{
		{"+7", 7, "the sign ParseInt takes and the digit walk refuses — #886's +77"},
		{"+007", 7, "sign and leading zeros together"},
		{"+9223372036854775807", 9223372036854775807, "the sign survives all the way to int64 max"},
		{"-7", -7, "the walk refuses the sign; PersonIDFromSGID also refuses id <= 0"},
		{"-9223372036854775808", -9223372036854775808, "int64 min is a fine FlexibleInt64 and never a person"},
	}
	for _, tc := range cases {
		var flexible types.FlexibleInt64
		if err := json.Unmarshal([]byte(strconv.Quote(tc.raw)), &flexible); err != nil {
			t.Fatalf("FlexibleInt64(%q): %v", tc.raw, err)
		}
		if int64(flexible) != tc.flex {
			t.Errorf("FlexibleInt64(%q) = %d, want %d (%s)", tc.raw, int64(flexible), tc.flex, tc.why)
		}
		if id, ok := PersonIDFromSGID(personGIDSGID(tc.raw)); ok {
			t.Errorf("PersonIDFromSGID(gid://bc3/Person/%s) = %d, want a refusal (%s)", tc.raw, id, tc.why)
		}
	}

	// And Rule A is not simply the stricter of the two: leading zeros pass its
	// digit walk, so it agrees with FlexibleInt64 on these. A port that
	// "hardens" the walk by refusing them diverges from the reference just as
	// surely as one that loosens it.
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{"007", 7},
		{"010", 10},
		{"0009223372036854775807", 9223372036854775807},
	} {
		id, ok := PersonIDFromSGID(personGIDSGID(tc.raw))
		if !ok || id != tc.want {
			t.Errorf("PersonIDFromSGID(gid://bc3/Person/%s) = (%d, %v), want (%d, true)", tc.raw, id, ok, tc.want)
		}
	}
}

// personGIDSGID builds the unsigned attachable envelope PersonIDFromSGID reads,
// carrying raw verbatim as the Person's id.
func personGIDSGID(raw string) string {
	envelope, err := json.Marshal(map[string]any{
		"gid":     "gid://bc3/Person/" + raw,
		"purpose": "attachable",
	})
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(envelope)
}

// TestPersonIDOracleDump writes the corpus's verdicts, as the real Go code
// answers them, to the file named by ORACLE_OUT. It is how the ports' tables
// were derived and how they can be re-derived after any change here:
//
//	ORACLE_OUT=/tmp/oracle.json go test ./pkg/basecamp/ -run TestPersonIDOracleDump
//
// Skipped otherwise; it asserts nothing, on purpose. The assertions are above.
func TestPersonIDOracleDump(t *testing.T) {
	out := os.Getenv("ORACLE_OUT")
	if out == "" {
		t.Skip("set ORACLE_OUT to write the oracle table")
	}
	type verdict struct {
		Raw          string `json:"raw"`
		GIDOK        bool   `json:"gid_ok"`
		GIDID        int64  `json:"gid_id"`
		FlexErr      string `json:"flex_err,omitempty"`
		FlexValue    int64  `json:"flex_value"`
		NormalizeErr string `json:"normalize_err,omitempty"`
		DecodeErr    string `json:"decode_err,omitempty"`
		WrapperID    int64  `json:"wrapper_id"`
		WrapperLabel string `json:"wrapper_label,omitempty"`
	}
	results := make([]verdict, 0, len(personIDCorpus))
	for _, tc := range personIDCorpus {
		v := verdict{Raw: tc.raw}
		v.GIDID, v.GIDOK = PersonIDFromSGID(personGIDSGID(tc.raw))

		var flexible types.FlexibleInt64
		if err := json.Unmarshal([]byte(strconv.Quote(tc.raw)), &flexible); err != nil {
			v.FlexErr = err.Error()
		} else {
			v.FlexValue = int64(flexible)
		}

		body := []byte(`{"creator":{"id":` + strconv.Quote(tc.raw) + `,"name":"x"}}`)
		normalized, normalizeErr := normalizeEmbeddedPeopleJSON(body)
		if normalizeErr != nil {
			v.NormalizeErr = normalizeErr.Error()
			normalized = body
		}
		var envelope struct {
			Creator Person `json:"creator"`
		}
		if err := json.Unmarshal(normalized, &envelope); err != nil {
			v.DecodeErr = err.Error()
		} else {
			v.WrapperID = envelope.Creator.ID
			v.WrapperLabel = envelope.Creator.SystemLabel
		}
		results = append(results, v)
	}
	encoded, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}
