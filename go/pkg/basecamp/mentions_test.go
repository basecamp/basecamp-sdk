package basecamp

import (
	"encoding/base64"
	"reflect"
	"strconv"
	"strings"
	"testing"
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
		{"base64 of unrelated bytes", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Todo/1?expires_in")) + "--abc", 0, false},
		{"person gid with a longer path is not a person", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Person/12/extra")) + "--abc", 0, false},
		{"zero id", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Person/0?expires_in")) + "--abc", 0, false},
		{"id overflowing int64", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Person/99999999999999999999?expires_in")) + "--abc", 0, false},
		{"gid at end of payload", base64.StdEncoding.EncodeToString([]byte("gid://bc3/Person/77")) + "--abc", 77, true},
		{"different app name", base64.StdEncoding.EncodeToString([]byte("gid://basecamp/Person/78?expires_in")) + "--abc", 78, true},
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
