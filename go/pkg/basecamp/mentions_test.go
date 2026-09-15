package basecamp

import (
	"encoding/base64"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// SGID fixtures. Every one is synthetic — the demo account's people from
// spec/fixtures, or ids that exist nowhere — and none carries a real signature.
//
// rubySGID* were produced by Ruby itself, not by Go, so the decoder is tested
// against what Rails writes rather than against this package's idea of it:
//
//	Base64.urlsafe_encode64(Marshal.dump({"_rails"=>{"data"=>"gid://bc3/Person/1049715915?expires_in","pur"=>"attachable"}}))
//
// (the payload layout Rails 7.1+ SignedGlobalID emits, as seen on BC3 today),
// with the "--<digest>" suffix appended by hand.
const (
	// The current Rails layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
	rubySGIDPerson = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiK2dpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNT9leHBpcmVzX2luBjsAVEkiCHB1cgY7AFRJIg9hdHRhY2hhYmxlBjsAVA==--0000000000000000000000000000000000000000"
	// The same layout naming a file blob: an attachment, not a mention.
	rubySGIDBlob = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiOGdpZDovL2JjMy9BY3RpdmVTdG9yYWdlOjpCbG9iLzEwNjk0ODAwMDA_ZXhwaXJlc19pbgY7AFRJIghwdXIGOwBUSSIPYXR0YWNoYWJsZQY7AFQ=--0000000000000000000000000000000000000000"
	// A person id above 2^53: the decoder must not lose precision.
	rubySGIDPersonWide = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiMWdpZDovL2JjMy9QZXJzb24vOTAwNzE5OTI1NDc0MDk5Mz9leHBpcmVzX2luBjsAVEkiCHB1cgY7AFRJIg9hdHRhY2hhYmxlBjsAVA==--0000000000000000000000000000000000000000"
	// The older Rails layout {"gid", "purpose", "expires_at"}, as carried by
	// spec/fixtures/people/get.json — the shape the SDK's shared fixtures use.
	fixtureSGIDPerson = "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"
	// spec/fixtures/todos/get.json's file attachment, an ActiveStorage::Blob.
	fixtureSGIDBlob = "BAh7CEkiCGdpZAY6BkVUSSIsZ2lkOi8vYmMzL0FjdGl2ZVN0b3JhZ2U6OkJsb2IvMTA2OTQ4MDAwMAY6BkVU--a1b2c3"
	// Adversarial payloads, also Ruby-produced: a Person gid appearing inside
	// another model's gid, and one appearing in the purpose field. Neither
	// names a person.
	rubySGIDPersonInsideDocument = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiK2dpZDovL2JjMy9Eb2N1bWVudC9naWQ6Ly9iYzMvUGVyc29uLzEyBjsAVEkiCHB1cgY7AFRJIg9hdHRhY2hhYmxlBjsAVA==--00"
	rubySGIDPersonInPurpose      = "BAh7CEkiCGdpZAY6BkVUSSIkZ2lkOi8vYmMzL0FjdGl2ZVN0b3JhZ2U6OkJsb2IvOQY7AFRJIgxwdXJwb3NlBjsAVEkiGGdpZDovL2JjMy9QZXJzb24vMTIGOwBUSSIPZXhwaXJlc19hdAY7AFQw--00"
	// The current layout with an extra key holding an array of every other
	// scalar the decoder supports.
	rubySGIDPersonExtraKeys = "BAh7B0kiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiK2dpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNT9leHBpcmVzX2luBjsAVEkiCHB1cgY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiCmV4dHJhBjsAVFsKaQZURjBJIgZzBjsAVA==--00"
	// Rails' JSON message serializer spelling of the current layout.
	jsonSGIDPerson = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNT9leHBpcmVzX2luIiwicHVyIjoiYXR0YWNoYWJsZSJ9fQ==--00"
	// The same person, Ruby-produced, minted for another purpose in each
	// layout: valid signed global ids, but not ones BC3 accepts in rich text.
	rubySGIDPersonReadable      = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiK2dpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNT9leHBpcmVzX2luBjsAVEkiCHB1cgY7AFRJIg1yZWFkYWJsZQY7AFQ=--00"
	rubySGIDPersonBookmarkOlder = "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg1ib29rbWFyawY7AFRJIg9leHBpcmVzX2F0BjsAVDA=--00"
)

// renderedMention is what BC3 serves back for a mention: the sgid, the
// content-type, and the avatar figure — the read-side shape from
// doc/api/sections/rich_text.md, with the demo person's id in the avatar.
func renderedMention(sgid string, personID int64, name string) string {
	return `<bc-attachment sgid="` + sgid + `" content-type="application/vnd.basecamp.mention"><figure>
  <img data-avatar-for-person-id="` + strconv.FormatInt(personID, 10) + `" alt="` + name + `" title="` + name + `" class="avatar" loading="lazy" decoding="async" src="https://example.invalid/avatar" width="20" height="20">
  <figcaption>
    ` + name + `
  </figcaption>
</figure></bc-attachment>`
}

// jsonEnvelope wraps a gid in the JSON spelling of Rails' current layout.
func jsonEnvelope(gid string) string {
	return base64.StdEncoding.EncodeToString([]byte(`{"_rails":{"data":"`+gid+`","pur":"attachable"}}`)) + "--00"
}

func TestPersonIDFromSGID(t *testing.T) {
	cases := []struct {
		name string
		sgid string
		want int64
		ok   bool
	}{
		{"ruby current layout", rubySGIDPerson, 1049715915, true},
		{"ruby current layout, wide id", rubySGIDPersonWide, 9007199254740993, true},
		{"fixture older layout", fixtureSGIDPerson, 1049715915, true},
		{"no signature suffix", strings.SplitN(rubySGIDPerson, "--", 2)[0], 1049715915, true},
		{"surrounding whitespace", "  " + fixtureSGIDPerson + "\n", 1049715915, true},
		{"file blob (ruby layout)", rubySGIDBlob, 0, false},
		{"file blob (fixture layout)", fixtureSGIDBlob, 0, false},
		{"empty", "", 0, false},
		{"only a signature", "--abc", 0, false},
		{"not base64", "!!not base64!!--abc", 0, false},
		{"raw bytes are not an envelope", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Person/1?expires_in")) + "--abc", 0, false},
		{"another model", jsonEnvelope("gid://bc3/Todo/1?expires_in"), 0, false},
		{"person gid with a longer path is not a person", jsonEnvelope("gid://bc3/Person/12/extra"), 0, false},
		{"zero id", jsonEnvelope("gid://bc3/Person/0?expires_in"), 0, false},
		{"id overflowing int64", jsonEnvelope("gid://bc3/Person/99999999999999999999?expires_in"), 0, false},
		{"gid without a query", jsonEnvelope("gid://bc3/Person/77"), 77, true},
		{"different app name", jsonEnvelope("gid://basecamp/Person/78?expires_in"), 78, true},
		{"json envelope (ruby)", jsonSGIDPerson, 1049715915, true},
		{"extra keys and scalars (ruby)", rubySGIDPersonExtraKeys, 1049715915, true},
		{"person gid inside a document gid (ruby)", rubySGIDPersonInsideDocument, 0, false},
		{"person gid in the purpose field (ruby)", rubySGIDPersonInPurpose, 0, false},
		{"json: person gid inside another gid", jsonEnvelope("gid://bc3/Document/gid://bc3/Person/12"), 0, false},
		{"json: gid under the wrong key", base64.StdEncoding.EncodeToString([]byte(`{"_rails":{"pur":"gid://bc3/Person/12"}}`)) + "--00", 0, false},
		{"json: a bare gid string is not an envelope", base64.StdEncoding.EncodeToString([]byte(`"gid://bc3/Person/12"`)) + "--00", 0, false},
		{"json: person gid with a query and fragment", jsonEnvelope("gid://bc3/Person/79?expires_in#x"), 79, true},
		{"json: wrong scheme", jsonEnvelope("https://bc3/Person/79"), 0, false},
		{"json: signed id", jsonEnvelope("gid://bc3/Person/abc"), 0, false},
		{"json: negative id", jsonEnvelope("gid://bc3/Person/-5"), 0, false},
		{"marshal: unsupported type is undecodable", "BAh" + base64.StdEncoding.EncodeToString([]byte{'o'}) + "--00", 0, false},
		{"marshal: truncated", strings.SplitN(rubySGIDPerson, "--", 2)[0][:40], 0, false},
		{"purpose other than attachable, current layout (ruby)", rubySGIDPersonReadable, 0, false},
		{"purpose other than attachable, older layout (ruby)", rubySGIDPersonBookmarkOlder, 0, false},
		{"json: purpose other than attachable", base64.StdEncoding.EncodeToString([]byte(`{"_rails":{"data":"gid://bc3/Person/80","pur":"bookmark"}}`)) + "--00", 0, false},
		{"json: purpose missing", base64.StdEncoding.EncodeToString([]byte(`{"_rails":{"data":"gid://bc3/Person/80"}}`)) + "--00", 0, false},
		{"json: older layout purpose missing", base64.StdEncoding.EncodeToString([]byte(`{"gid":"gid://bc3/Person/80"}`)) + "--00", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PersonIDFromSGID(tc.sgid)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("PersonIDFromSGID(%q) = (%d, %v), want (%d, %v)", tc.sgid, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestMentionedPersonIDs(t *testing.T) {
	victor := renderedMention(fixtureSGIDPerson, 1049715915, "Victor Cooper")
	annie := renderedMention(rubySGIDPerson, 1049715915, "Annie Bryan") // same id, another spelling
	wide := renderedMention(rubySGIDPersonWide, 9007199254740993, "Wide Id")

	cases := []struct {
		name string
		html string
		want []int64
	}{
		{"empty", "", nil},
		{"no attachments", "<div>Hello everyone!</div>", nil},
		{"one rendered mention", "<div>" + victor + " thanks!</div>", []int64{1049715915}},
		{"write-side bare tag", `<div><bc-attachment sgid="` + rubySGIDPerson + `"></bc-attachment> hi</div>`, []int64{1049715915}},
		{"single-quoted attribute", `<bc-attachment sgid='` + rubySGIDPerson + `'></bc-attachment>`, []int64{1049715915}},
		{"attribute order and case", `<BC-ATTACHMENT content-type="application/vnd.basecamp.mention"
   SGID = "` + rubySGIDPerson + `"></BC-ATTACHMENT>`, []int64{1049715915}},
		{"file attachment is not a mention", `<div>See <bc-attachment sgid="` + fixtureSGIDBlob + `" caption="schematic"></bc-attachment> and <bc-attachment sgid="` + rubySGIDBlob + `"></bc-attachment></div>`, nil},
		{"mixed, document order, deduped", "<div>" + wide + " " + victor + " and " + annie + ` <bc-attachment sgid="` + fixtureSGIDBlob + `"></bc-attachment></div>`, []int64{9007199254740993, 1049715915}},
		{"mention inside a blockquote counts", "<blockquote>" + victor + " said so</blockquote><div>agreed</div>", []int64{1049715915}},
		{"mention nested in another attachment's figure", `<bc-attachment sgid="` + fixtureSGIDBlob + `"><figure><figcaption>` + victor + `</figcaption></figure></bc-attachment>`, []int64{1049715915}},
		{"undecodable sgid is skipped", `<bc-attachment sgid="not-an-sgid"></bc-attachment>` + victor, []int64{1049715915}},
		{"sgid missing", `<bc-attachment content-type="application/vnd.basecamp.mention"></bc-attachment>`, nil},
		{"avatar id alone does not count", `<img data-avatar-for-person-id="1049715915" alt="x">`, nil},
		{"tag name prefix does not match", `<bc-attachments sgid="` + rubySGIDPerson + `"></bc-attachments>`, nil},
		{"a data-sgid attribute is not an sgid", `<bc-attachment data-sgid="` + rubySGIDPerson + `"></bc-attachment>`, nil},
		{"a > inside another attribute does not end the tag", `<bc-attachment title="a > b" sgid="` + rubySGIDPerson + `"></bc-attachment>`, []int64{1049715915}},
		{"an sgid= inside another attribute's value is not an attribute", `<bc-attachment caption='example sgid="` + rubySGIDBlob + `"' sgid="` + rubySGIDPerson + `"></bc-attachment>`, []int64{1049715915}},
		{"entity-escaped attribute value", `<bc-attachment sgid="` + strings.ReplaceAll(rubySGIDPerson, "=", "&#61;") + `"></bc-attachment>`, []int64{1049715915}},
		{"self-closing and unquoted", `<bc-attachment sgid=` + rubySGIDPerson + `/>`, []int64{1049715915}},
		{"unterminated tag is skipped", `<bc-attachment sgid="` + rubySGIDPerson, nil},
		{"unterminated quote is skipped", `<bc-attachment sgid="` + rubySGIDPerson + `></bc-attachment>`, nil},
		{"empty sgid", `<bc-attachment sgid=""></bc-attachment>`, nil},
		{"two attachments back to back", `<bc-attachment sgid="` + rubySGIDPersonWide + `"></bc-attachment><bc-attachment sgid="` + rubySGIDPerson + `"></bc-attachment>`, []int64{9007199254740993, 1049715915}},
		{"inside an HTML comment is not an element", `<!-- <bc-attachment sgid="` + rubySGIDPerson + `"></bc-attachment> --><div>hi</div>`, nil},
		{"inside another element's attribute is not an element", `<div title='<bc-attachment sgid="` + rubySGIDPerson + `">'>text</div>`, nil},
		{"inside a double-quoted attribute with a single-quoted sgid", `<div data-example="<bc-attachment sgid='` + rubySGIDPerson + `'>">x</div>`, nil},
		{"a real one after a commented one", `<!-- <bc-attachment sgid="` + rubySGIDPersonWide + `"> -->` + victor, []int64{1049715915}},
		{"first sgid attribute wins, empty included", `<bc-attachment sgid="" sgid="` + rubySGIDPerson + `"></bc-attachment>`, nil},
		{"first sgid attribute wins, valued", `<bc-attachment sgid="` + rubySGIDPerson + `" sgid="` + rubySGIDPersonWide + `"></bc-attachment>`, []int64{1049715915}},
		{"bare self-closing tag", `<bc-attachment/>` + victor, []int64{1049715915}},
		{"attribute without a value, then the sgid", `<bc-attachment hidden sgid="` + rubySGIDPerson + `"></bc-attachment>`, []int64{1049715915}},
		{"equals with no value", `<bc-attachment caption= sgid="` + rubySGIDPerson + `"></bc-attachment>`, nil},
		{"whitespace before the equals", `<bc-attachment sgid = "` + rubySGIDPerson + `"></bc-attachment>`, []int64{1049715915}},
		{"tag at the very end of the text", `<div>hi</div><bc-attachment`, nil},
		{"a > inside single quotes inside a double-quoted attribute", `<bc-attachment caption="it's a 'x > y' thing" sgid="` + rubySGIDPerson + `"></bc-attachment>`, []int64{1049715915}},
		{"a bare < in text does not derail the walk", `<div>1 < 2 and ` + victor + `</div>`, []int64{1049715915}},
		{"a closing tag and a doctype are skipped", `<!DOCTYPE html></p>` + victor, []int64{1049715915}},
		{"a tag whose name only starts with bc-attachment", `<bc-attachment:preview sgid="` + rubySGIDPerson + `"></bc-attachment:preview><bc-attachment_x sgid="` + rubySGIDPersonWide + `"/>`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MentionedPersonIDs(tc.html)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("MentionedPersonIDs = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPersonIDFromSGID_HostileMarshal checks that a payload lying about its
// own size is refused for the price of its bytes: a chain of arrays each
// claiming a million elements, a hash claiming more pairs than bytes remain,
// a negative instance-variable count, and a payload over the size cap.
func TestPersonIDFromSGID_HostileMarshal(t *testing.T) {
	sgid := func(raw []byte) string { return base64.StdEncoding.EncodeToString(raw) + "--00" }
	// Marshal packs 1048576 as 0x03 + three little-endian bytes.
	million := []byte{0x03, 0x00, 0x00, 0x10}
	var nested []byte
	nested = append(nested, 0x04, 0x08)
	for i := 0; i < 33; i++ {
		nested = append(nested, '[')
		nested = append(nested, million...)
	}
	nested = append(nested, make([]byte, 1024)...)
	cases := map[string][]byte{
		"nested arrays claiming a million elements each": nested,
		"hash claiming more pairs than bytes":            {0x04, 0x08, '{', 0x03, 0x00, 0x00, 0x10, 'I', '"', 0x06, 'a'},
		"negative ivar count on the envelope string":     {0x04, 0x08, 'I', '"', 0x06, 'x', 0xfa},
		"string claiming past the end":                   {0x04, 0x08, '"', 0x20, 'a'},
		"unsupported object type":                        {0x04, 0x08, 'o', ':', 0x06, 'X', 0x00},
		"truncated after the version":                    {0x04, 0x08},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				if id, ok := PersonIDFromSGID(sgid(raw)); ok {
					t.Errorf("accepted as person %d", id)
				}
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("decoding did not finish promptly")
			}
		})
	}
	t.Run("trailing bytes after a valid envelope", func(t *testing.T) {
		payload, _, _ := strings.Cut(rubySGIDPerson, "--")
		raw, err := base64.StdEncoding.DecodeString(strings.NewReplacer("-", "+", "_", "/").Replace(payload))
		if err != nil {
			t.Fatal(err)
		}
		if id, ok := PersonIDFromSGID(sgid(append(raw, "junk"...))); ok {
			t.Fatalf("accepted as person %d with trailing bytes", id)
		}
		if _, ok := PersonIDFromSGID(sgid(raw)); !ok {
			t.Fatal("the same envelope without the trailing bytes must decode")
		}
	})
	t.Run("encoded form over the size cap is refused before decoding", func(t *testing.T) {
		// Not even valid base64: the length alone refuses it.
		if _, ok := PersonIDFromSGID(strings.Repeat("!", maxSGIDEncodedBytes+1) + "--00"); ok {
			t.Fatal("accepted an oversized encoded sgid")
		}
	})
	t.Run("payload over the size cap", func(t *testing.T) {
		big := append([]byte{0x04, 0x08, '"'}, 0x02, 0x00, 0x20) // a string of 8192 bytes
		big = append(big, make([]byte, 8192)...)
		if _, ok := PersonIDFromSGID(sgid(big)); ok {
			t.Fatal("accepted an oversized payload")
		}
	})
	t.Run("a negative ivar count does not slip a valid envelope through", func(t *testing.T) {
		// Ruby's own envelope, then its trailing ivar count byte (0x06 = 1)
		// replaced by -1 (0xfa): the reader must refuse, not read zero ivars.
		payload, _, _ := strings.Cut(rubySGIDPerson, "--")
		raw, err := base64.StdEncoding.DecodeString(strings.NewReplacer("-", "+", "_", "/").Replace(payload))
		if err != nil {
			t.Fatal(err)
		}
		// The first ivar count follows the "_rails" key string: I " \x0b _rails <count>.
		idx := strings.Index(string(raw), "_rails") + len("_rails")
		if raw[idx] != 0x06 {
			t.Fatalf("fixture layout changed: byte %d = %#x", idx, raw[idx])
		}
		raw[idx] = 0xfa
		if id, ok := PersonIDFromSGID(sgid(raw)); ok {
			t.Fatalf("accepted as person %d with a negative ivar count", id)
		}
	})
}

func TestMentionMarkup(t *testing.T) {
	got, err := MentionMarkup(&Person{ID: 1049715915, AttachableSGID: fixtureSGIDPerson})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<bc-attachment sgid="` + fixtureSGIDPerson + `"></bc-attachment>`
	if got != want {
		t.Fatalf("MentionMarkup = %q, want %q", got, want)
	}

	if _, err := MentionMarkup(nil); err == nil {
		t.Fatal("expected an error for a nil person")
	}
	if _, err := MentionMarkup(&Person{ID: 5}); err == nil {
		t.Fatal("expected an error for a person without attachable_sgid")
	}
	if _, err := MentionMarkup(&Person{ID: 5, AttachableSGID: `x"><script>`}); err == nil {
		t.Fatal("expected an error for an sgid carrying markup")
	}
	if _, err := MentionMarkup(&Person{ID: 5, AttachableSGID: rubySGIDBlob}); err == nil {
		t.Fatal("expected an error for a file blob's sgid")
	}
	if _, err := MentionMarkup(&Person{ID: 5, AttachableSGID: fixtureSGIDPerson}); err == nil {
		t.Fatal("expected an error for an sgid naming another person")
	}
	if _, err := MentionMarkup(&Person{ID: 1049715915, AttachableSGID: rubySGIDPersonReadable}); err == nil {
		t.Fatal("expected an error for the right person's sgid minted for another purpose")
	}
}

func TestWithMentions(t *testing.T) {
	victor := Person{ID: 1049715915, Name: "Victor Cooper", AttachableSGID: fixtureSGIDPerson}
	wide := Person{ID: 9007199254740993, Name: "Wide Id", AttachableSGID: rubySGIDPersonWide}
	victorTag := `<bc-attachment sgid="` + fixtureSGIDPerson + `"></bc-attachment>`
	wideTag := `<bc-attachment sgid="` + rubySGIDPersonWide + `"></bc-attachment>`

	t.Run("prefix inside a leading paragraph", func(t *testing.T) {
		got, err := WithMentions(`<p dir="auto">on it</p>`, []Person{victor})
		if err != nil {
			t.Fatal(err)
		}
		if want := `<p dir="auto">` + victorTag + ` on it</p>`; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("prefix inside a leading div", func(t *testing.T) {
		got, err := WithMentions("<div>on it</div>", []Person{victor, wide})
		if err != nil {
			t.Fatal(err)
		}
		if want := "<div>" + victorTag + " " + wideTag + " on it</div>"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("a > inside the leading tag's attribute does not split it", func(t *testing.T) {
		got, err := WithMentions(`<div title="1 > 0">on it</div>`, []Person{victor})
		if err != nil {
			t.Fatal(err)
		}
		if want := `<div title="1 > 0">` + victorTag + ` on it</div>`; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("a tag that only starts like a block is a bare prefix", func(t *testing.T) {
		got, err := WithMentions(`<pre>code</pre>`, []Person{victor})
		if err != nil {
			t.Fatal(err)
		}
		if want := victorTag + ` <pre>code</pre>`; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("bare prefix for plain content", func(t *testing.T) {
		got, err := WithMentions("on it", []Person{victor})
		if err != nil {
			t.Fatal(err)
		}
		if want := victorTag + " on it"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("already mentioned is not repeated", func(t *testing.T) {
		content := "<div>" + renderedMention(rubySGIDPerson, 1049715915, "Victor Cooper") + " agreed</div>"
		got, err := WithMentions(content, []Person{victor})
		if err != nil {
			t.Fatal(err)
		}
		if got != content {
			t.Fatalf("content changed: %q", got)
		}
	})
	t.Run("same person twice is one mention", func(t *testing.T) {
		got, err := WithMentions("hi", []Person{victor, victor})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(got, "<bc-attachment") != 1 {
			t.Fatalf("expected one mention, got %q", got)
		}
	})
	t.Run("nobody to add leaves content untouched", func(t *testing.T) {
		got, err := WithMentions("<p>hi</p>", nil)
		if err != nil || got != "<p>hi</p>" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("missing sgid fails before any change", func(t *testing.T) {
		if _, err := WithMentions("hi", []Person{{ID: 9}}); err == nil {
			t.Fatal("expected an error")
		}
	})
	t.Run("round trip: written mentions read back", func(t *testing.T) {
		got, err := WithMentions("<div>ping</div>", []Person{victor, wide})
		if err != nil {
			t.Fatal(err)
		}
		if ids := MentionedPersonIDs(got); !reflect.DeepEqual(ids, []int64{1049715915, 9007199254740993}) {
			t.Fatalf("MentionedPersonIDs(WithMentions(...)) = %v", ids)
		}
	})
}
