package basecamp

// Mention helpers over Basecamp rich text.
//
// A mention in Basecamp rich text is a <bc-attachment> whose sgid attribute is
// the mentioned person's attachable_sgid (doc/api/sections/rich_text.md,
// "Inserting a mention"). BC3 renders the same tag back with
// content-type="application/vnd.basecamp.mention" and an avatar figure inside
// it, but the sgid is the only part of the markup that names the person on
// both the write and the read side, so both helpers here work from it:
//
//   - MentionedPersonIDs reads the person ids a rich text names, by decoding
//     the sgid of every <bc-attachment> and keeping the ones that point at a
//     Person.
//   - MentionMarkup writes the tag for a person, from their attachable_sgid.
//
// An attachable_sgid is a Rails SignedGlobalID: a base64 payload, then "--",
// then an HMAC only BC3 can verify. The payload is an envelope carrying the
// global id — "gid://bc3/Person/1049715915" — as a string, and that string is
// what these helpers read. They do not (and cannot) verify the signature; what
// they decode is the same person id BC3 renders into the mention's avatar, read
// off content the API already served, and a caller that needs the id verified
// reads the person back through People().Get.
//
// That sets a trust boundary between the two sides. READING — MentionedPersonIDs,
// PersonIDFromSGID — describes what a text says it mentions, and unsigned is
// fine for description: the ids are reported, not acted on as proof. WRITING —
// WithMentions, CommentsService.ExpandMentions — never treats an unsigned id as
// proof that a valid mention already exists: a forged or stale sgid in
// caller-supplied content naming the right id would otherwise make the writer
// skip the authoritative people read and post a tag Basecamp will not honour,
// so the person is silently not mentioned. CommentsService.ExpandMentions
// therefore resolves every requested person through People().Get and
// deduplicates only against the exact attachable_sgid string that read
// returned. The pure helpers beneath it — WithMentions, MentionMarkup — take
// Person values the caller built and can only check that an sgid is
// well-formed and names the person it is given, never that it is authentic:
// hand them people the API returned, not people assembled from content. Do
// not reuse the read-side helpers to decide whether a write can be skipped.
//
// The markup is read as BC3 serves it: a sanitized tree of the tags
// doc/api/sections/rich_text.md allows, which has no raw-text elements. The
// tag walk skips comments and quoted attribute values but does not model
// <script> or <style> content, which BC3 strips on write; a caller reading
// mentions out of content it authored itself should not put a bc-attachment
// inside such an element and expect it ignored.
//
// The envelope is decoded structurally, never searched as bytes, so a Person
// gid that merely appears inside some other value — a Document gid built from
// one, a purpose string that looks like one — is not a mention, and the
// envelope's purpose must be "attachable", the one BC3 accepts in rich text. Three
// envelopes are read: Rails' current Marshal layout
// {"_rails" => {"data" => gid, "pur" => purpose}}, the older Marshal layout
// {"gid" => gid, "purpose" => …, "expires_at" => …}, and the JSON spelling of
// either, which Rails' JSON message serializer emits.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

// MentionedPersonIDs returns the ids of the people a rich text mentions: the
// Person named by the sgid of each <bc-attachment>, in document order, with
// repeats removed. Attachments that are not mentions — files, images, embeds —
// are skipped, as is any sgid that does not decode to a Person.
//
// This is the read side: a description of what the text says, from sgids
// whose signatures cannot be checked here. Report it; do not treat an id in
// it as proof that a valid mention exists (see the trust boundary above).
//
// Every <bc-attachment> in the text counts, including one inside a
// <blockquote>: BC3 notifies quoted mentions too, so the read matches what the
// server does with the write.
func MentionedPersonIDs(richText string) []int64 {
	var ids []int64
	seen := map[int64]struct{}{}
	for _, sgid := range bcAttachmentSGIDs(richText) {
		id, ok := PersonIDFromSGID(sgid)
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// bcAttachmentSGIDs returns the sgid attribute of every <bc-attachment> in the
// text, in document order. It walks the markup as a stream of tags rather
// than pattern-matching for one tag name, so a <bc-attachment> inside an HTML
// comment or inside another element's quoted attribute is not an element; and
// it tokenizes each tag's attributes rather than pattern-matching them, so a
// ">" inside a quoted value does not end the tag, an "sgid=" inside another
// attribute's value is not an attribute, either quote style works, attribute
// order and case are free, the first sgid attribute wins as in HTML, and
// entity escapes in the value are decoded as a browser would.
func bcAttachmentSGIDs(text string) []string {
	var sgids []string
	for pos := 0; pos < len(text); {
		i := strings.IndexByte(text[pos:], '<')
		if i < 0 {
			break
		}
		pos += i + 1
		rest := text[pos:]
		switch {
		case strings.HasPrefix(rest, "!--"):
			stop := strings.Index(rest, "-->")
			if stop < 0 {
				return sgids // an unterminated comment swallows the rest
			}
			pos += stop + 3
			continue
		case strings.HasPrefix(rest, "!"), strings.HasPrefix(rest, "?"), strings.HasPrefix(rest, "/"):
			stop := strings.IndexByte(rest, '>')
			if stop < 0 {
				return sgids
			}
			pos += stop + 1
			continue
		}
		nameEnd := 0
		for nameEnd < len(rest) && isTagNameChar(rest[nameEnd]) {
			nameEnd++
		}
		if nameEnd == 0 {
			continue // a bare "<" in text
		}
		attrs, end, ok := parseAttributes(text, pos+nameEnd)
		if !ok {
			return sgids // an unterminated tag: nothing after it is markup
		}
		if strings.EqualFold(rest[:nameEnd], "bc-attachment") && attrs.sgid != "" {
			sgids = append(sgids, attrs.sgid)
		}
		pos = end
	}
	return sgids
}

// isTagNameChar is what may follow "<" in a tag name. The whole name is
// consumed, punctuation included, so "<bc-attachment:preview" or
// "<bc-attachment_x" is its own name and never compares equal to
// "bc-attachment".
func isTagNameChar(c byte) bool {
	return !isSpace(c) && c != '/' && c != '>' && c != '<' && c != '=' && c != '"' && c != '\''
}

func isTagNameEnd(c byte) bool {
	return isSpace(c) || c == '/' || c == '>'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// tagAttributes is what parseAttributes reads off one opening tag.
type tagAttributes struct {
	sgid     string // the decoded value of the first sgid attribute
	sgidSeen bool   // whether an sgid attribute was present at all
}

// parseAttributes walks the attributes of an opening tag from pos (just after
// the tag name) to its closing ">", returning what it found, the index after
// the ">", and whether the tag was closed at all. The first sgid attribute
// wins, present-but-empty included, as HTML resolves a repeated attribute.
func parseAttributes(text string, pos int) (attrs tagAttributes, end int, ok bool) {
	for pos < len(text) {
		for pos < len(text) && (isSpace(text[pos]) || text[pos] == '/') {
			pos++
		}
		if pos >= len(text) {
			return attrs, pos, false
		}
		if text[pos] == '>' {
			return attrs, pos + 1, true
		}
		nameStart := pos
		for pos < len(text) && !isSpace(text[pos]) && text[pos] != '=' && text[pos] != '>' && text[pos] != '/' {
			pos++
		}
		name := text[nameStart:pos]
		for pos < len(text) && isSpace(text[pos]) {
			pos++
		}
		value := ""
		if pos < len(text) && text[pos] == '=' {
			pos++
			for pos < len(text) && isSpace(text[pos]) {
				pos++
			}
			if pos < len(text) && (text[pos] == '"' || text[pos] == '\'') {
				quote := text[pos]
				pos++
				closing := strings.IndexByte(text[pos:], quote)
				if closing < 0 {
					return attrs, len(text), false
				}
				value = text[pos : pos+closing]
				pos += closing + 1
			} else {
				valueStart := pos
				for pos < len(text) && !isSpace(text[pos]) && text[pos] != '>' {
					pos++
				}
				value = text[valueStart:pos]
			}
		}
		if name == "" {
			// A stray "=" or quote where a name should be: step over it.
			pos++
			continue
		}
		if !attrs.sgidSeen && strings.EqualFold(name, "sgid") {
			attrs.sgidSeen = true
			attrs.sgid = html.UnescapeString(value)
		}
	}
	return attrs, pos, false
}

// PersonIDFromSGID decodes the Person id an attachable_sgid names. The second
// result is false when the sgid does not decode, or names something other than
// a Person (a file attachment's sgid names an ActiveStorage::Blob).
//
// This reads the id out of the sgid's payload; it does not verify the sgid's
// signature, which only BC3 can. It is a read-side helper: never use its
// answer to decide that a write may skip the authoritative people read (see
// the trust boundary in the package comment).
func PersonIDFromSGID(sgid string) (int64, bool) {
	gid, ok := globalIDFromSGID(sgid)
	if !ok {
		return 0, false
	}
	u, err := url.Parse(gid)
	if err != nil || u.Scheme != "gid" || u.Host == "" {
		return 0, false
	}
	// A GlobalID path is exactly "/<Model>/<id>": no more, no less.
	model, rawID, found := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if !found || model != "Person" || rawID == "" {
		return 0, false
	}
	for i := 0; i < len(rawID); i++ {
		if rawID[i] < '0' || rawID[i] > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// globalIDFromSGID returns the global id string an sgid's envelope carries.
//
// A signed sgid is "<payload>--<digest>", and "-" is a base64url character,
// so the payload itself may contain "--". The separator is therefore the
// LAST one, as Rails' own verifier reads it; the whole value is tried as a
// bare payload when that fails, which is what an unsigned envelope — one
// that happens to contain "--" included — needs.
func globalIDFromSGID(sgid string) (string, bool) {
	value := strings.TrimSpace(sgid)
	if i := strings.LastIndex(value, "--"); i > 0 {
		if gid, ok := envelopeGID(value[:i]); ok {
			return gid, true
		}
	}
	return envelopeGID(value)
}

// envelopeGID decodes one base64 payload and returns the gid its envelope
// carries.
func envelopeGID(payload string) (string, bool) {
	// The bound is applied to the encoded form first, so an oversized sgid
	// costs nothing to refuse — no normalization, no decode buffer.
	if payload == "" || len(payload) > maxSGIDEncodedBytes {
		return "", false
	}
	// Rails' MessageVerifier emits either alphabet; base64url is current. Both
	// decode through the standard alphabet once the two symbols are mapped,
	// and stripping the padding lets a truncated-but-valid payload through.
	normalized := strings.NewReplacer("-", "+", "_", "/").Replace(payload)
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(normalized, "="))
	if err != nil || len(raw) == 0 || len(raw) > maxSGIDPayloadBytes {
		return "", false
	}
	var envelope any
	switch {
	case len(raw) >= 2 && raw[0] == 0x04 && raw[1] == 0x08:
		envelope, err = unmarshalRuby(raw[2:])
		if err != nil {
			return "", false
		}
	case raw[0] == '{':
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return "", false
		}
	default:
		return "", false
	}
	top, ok := envelope.(map[string]any)
	if !ok {
		return "", false
	}
	// A SignedGlobalID is bound to a purpose, and only an "attachable" one
	// may be placed in rich text: BC3 refuses any other, so a Person sgid
	// minted for bookmarking or reading is not a mention however valid its
	// gid. Both layouts carry the purpose; an envelope without one is not a
	// Rails envelope.
	//
	// Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
	if rails, ok := top["_rails"].(map[string]any); ok {
		if pur, _ := rails["pur"].(string); pur != sgidPurposeAttachable {
			return "", false
		}
		gid, ok := rails["data"].(string)
		return gid, ok && gid != ""
	}
	// Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
	if purpose, _ := top["purpose"].(string); purpose != sgidPurposeAttachable {
		return "", false
	}
	gid, ok := top["gid"].(string)
	return gid, ok && gid != ""
}

// sgidPurposeAttachable is the SignedGlobalID purpose BC3 mints attachable
// sgids with (doc/api/sections/rich_text.md: attachable_sgid). Pinned by
// TestPersonIDFromSGID's purpose cases, so a rename upstream breaks a test
// here rather than silently turning every mention invisible.
const sgidPurposeAttachable = "attachable"

// unmarshalRuby decodes the subset of Ruby's Marshal 4.8 format a
// SignedGlobalID payload uses — nil, booleans, fixnums, strings (with their
// encoding ivars), symbols and symbol links, arrays and hashes — into plain Go
// values: map[string]any, []any, string, int64, bool, nil. Anything else is an
// error; the caller then treats the sgid as undecodable rather than guessing.
func unmarshalRuby(data []byte) (any, error) {
	r := &rubyMarshalReader{data: data}
	v, err := r.value(0)
	if err != nil {
		return nil, err
	}
	if r.pos != len(data) {
		// A Marshal dump is exactly one value; bytes after it are corruption.
		return nil, fmt.Errorf("marshal: %d trailing bytes", len(data)-r.pos)
	}
	return v, nil
}

type rubyMarshalReader struct {
	data    []byte
	pos     int
	symbols []string
}

const (
	// rubyMarshalMaxDepth bounds nesting in a payload; an envelope is two deep.
	rubyMarshalMaxDepth = 32
	// maxSGIDPayloadBytes bounds the decoded sgid payload. A Person sgid's
	// payload is under 200 bytes; the cap keeps a hostile one from costing
	// more than its own size to reject.
	maxSGIDPayloadBytes = 4096
	// maxSGIDEncodedBytes is the same bound on the base64 form (4/3 of the
	// payload, plus padding), checked before anything is allocated.
	maxSGIDEncodedBytes = maxSGIDPayloadBytes/3*4 + 4
)

func (r *rubyMarshalReader) byte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("marshal: unexpected end of data")
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

// bytes takes the next n bytes. The bound is checked against what remains,
// never by adding n to the position: on a 32-bit target r.pos+n overflows
// for a hostile length near MaxInt32 and would pass a naive check into a
// slice panic. Every length reaches here through count(), which already
// rejected anything past the remaining bytes as an int64, so the int
// conversion upstream cannot have truncated either.
func (r *rubyMarshalReader) bytes(n int) ([]byte, error) {
	if n < 0 || n > len(r.data)-r.pos {
		return nil, fmt.Errorf("marshal: unexpected end of data")
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

// int reads Marshal's packed integer: 0 is 0; 1..4 and -1..-4 are a byte count
// for a little-endian value; anything else is the value itself offset by 5.
func (r *rubyMarshalReader) int() (int64, error) {
	b, err := r.byte()
	if err != nil {
		return 0, err
	}
	// Marshal's packed-integer lead byte is a signed int8. Sign-extend it
	// arithmetically: this widens the byte into its signed meaning, it does
	// not narrow anything, so there is no overflow to guard.
	c := int64(b)
	if c > 127 {
		c -= 256
	}
	switch {
	case c == 0:
		return 0, nil
	case c > 4:
		return c - 5, nil
	case c < -4:
		return c + 5, nil
	case c > 0:
		raw, err := r.bytes(int(c))
		if err != nil {
			return 0, err
		}
		var x int64
		for i, v := range raw {
			x |= int64(v) << (8 * i)
		}
		return x, nil
	default:
		raw, err := r.bytes(int(-c))
		if err != nil {
			return 0, err
		}
		x := int64(-1)
		for i, v := range raw {
			x &^= 0xff << (8 * i)
			x |= int64(v) << (8 * i)
		}
		return x, nil
	}
}

// count reads a length or count — string and symbol bytes, array elements,
// hash pairs, ivar pairs — and rejects one that cannot be honest: negative,
// or more than the bytes left (every element takes at least one byte). The
// comparison is made on the int64 against the remaining length, before any
// conversion to int and never by adding to the position, so neither a
// 32-bit truncation nor an overflow-by-addition can let a hostile length
// through. Allocation follows what actually decodes, so a hostile count
// costs its own bytes to refuse, never the capacity it claims.
func (r *rubyMarshalReader) count() (int64, error) {
	n, err := r.int()
	if err != nil {
		return 0, err
	}
	if n < 0 || n > int64(len(r.data)-r.pos) {
		return 0, fmt.Errorf("marshal: bad count %d", n)
	}
	return n, nil
}

func (r *rubyMarshalReader) value(depth int) (any, error) {
	if depth > rubyMarshalMaxDepth {
		return nil, fmt.Errorf("marshal: nesting too deep")
	}
	t, err := r.byte()
	if err != nil {
		return nil, err
	}
	switch t {
	case '0':
		return nil, nil
	case 'T':
		return true, nil
	case 'F':
		return false, nil
	case 'i':
		return r.int()
	case '"':
		n, err := r.count()
		if err != nil {
			return nil, err
		}
		raw, err := r.bytes(int(n))
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case ':':
		n, err := r.count()
		if err != nil {
			return nil, err
		}
		raw, err := r.bytes(int(n))
		if err != nil {
			return nil, err
		}
		sym := string(raw)
		r.symbols = append(r.symbols, sym)
		return sym, nil
	case ';':
		idx, err := r.int()
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx >= int64(len(r.symbols)) {
			return nil, fmt.Errorf("marshal: bad symbol link")
		}
		return r.symbols[idx], nil
	case 'I':
		// An object followed by its instance variables — a String's encoding.
		inner, err := r.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n, err := r.count()
		if err != nil {
			return nil, err
		}
		for i := int64(0); i < n; i++ {
			if _, err := r.value(depth + 1); err != nil { // ivar name
				return nil, err
			}
			if _, err := r.value(depth + 1); err != nil { // ivar value
				return nil, err
			}
		}
		return inner, nil
	case '[':
		n, err := r.count()
		if err != nil {
			return nil, err
		}
		var out []any //nolint:prealloc // grown as elements decode, never sized from the claim
		for i := int64(0); i < n; i++ {
			v, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case '{':
		n, err := r.count()
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		for i := int64(0); i < n; i++ {
			k, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			v, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("marshal: non-string hash key")
			}
			out[key] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("marshal: unsupported type %q", t)
	}
}

// MentionMarkup renders the <bc-attachment> that mentions a person, from their
// attachable_sgid — the write-side form in doc/api/sections/rich_text.md, which
// BC3 expands into the avatar figure on read. It errors when the person carries
// no attachable_sgid, which is the case for a Person projection that came from
// somewhere other than a people read (a webhook payload, say), and when the
// sgid does not name the person it is given. That is all it can check: it
// cannot verify the signature, so the Person must come from the API — a
// People().Get, a recording's creator or assignees — not be assembled from an
// sgid found in content.
func MentionMarkup(person *Person) (string, error) {
	if person == nil {
		return "", &Error{Code: CodeUsage, Message: "cannot mention a nil person"}
	}
	if person.AttachableSGID == "" {
		return "", &Error{Code: CodeUsage, Message: fmt.Sprintf("person %d has no attachable_sgid to mention", person.ID), Hint: "read the person through People().Get to obtain one"}
	}
	if strings.ContainsAny(person.AttachableSGID, "\"'<>&") {
		return "", &Error{Code: CodeUsage, Message: fmt.Sprintf("person %d has a malformed attachable_sgid", person.ID)}
	}
	// The tag mentions whoever the sgid names. Refuse to write one that names
	// someone else — or a file — under this person's id.
	if id, ok := PersonIDFromSGID(person.AttachableSGID); !ok || id != person.ID {
		return "", &Error{Code: CodeUsage, Message: fmt.Sprintf("person %d's attachable_sgid does not name that person", person.ID), Hint: "read the person through People().Get to obtain their own"}
	}
	return `<bc-attachment sgid="` + person.AttachableSGID + `"></bc-attachment>`, nil
}

// leadingBlockEnd returns the index just past the opening <p …> or <div …>
// tag a rich text starts with, or -1 when it starts with anything else, so
// mentions can be placed inside the first block rather than as a bare prefix
// in front of it. The tag's attributes are scanned quote-aware: a ">" inside
// an attribute value does not end it.
func leadingBlockEnd(content string) int {
	i := 0
	for i < len(content) && isSpace(content[i]) {
		i++
	}
	for _, name := range []string{"<p", "<div"} {
		if len(content) < i+len(name) || !strings.EqualFold(content[i:i+len(name)], name) {
			continue
		}
		after := i + len(name)
		if after < len(content) && !isTagNameEnd(content[after]) {
			continue
		}
		if _, end, ok := parseAttributes(content, after); ok {
			return end
		}
		return -1
	}
	return -1
}

// WithMentions returns content that mentions each of the given people, for
// posting as a comment or a Campfire line. A person whose exact
// attachable_sgid the content already carries is left alone, so passing the
// same person twice — or a person the author already mentioned with that
// sgid — never duplicates the mention; the rest are added at the start of the
// content, inside its first <p> or <div> when it opens with one, so they
// render on the first line rather than as a block of their own.
//
// This is the write side, and it deduplicates on the sgid string alone, never
// on the person id an existing tag's sgid decodes to: that id is unsigned, and
// a forged or stale tag naming the right person must not stand in for the
// real mention (see the trust boundary in the package comment). Every person
// needs their own attachable_sgid, and it must be one the API returned: this
// helper can check that an sgid is well-formed and names the person, not that
// it is authentic (see MentionMarkup). The account-bound
// CommentsService.ExpandMentions resolves ids to people first and is the
// entry point that carries that guarantee.
func WithMentions(content string, people []Person) (string, error) {
	present := map[string]struct{}{}
	for _, sgid := range bcAttachmentSGIDs(content) {
		present[sgid] = struct{}{}
	}
	var tags []string
	for i := range people {
		p := &people[i]
		tag, err := MentionMarkup(p)
		if err != nil {
			return "", err
		}
		if _, done := present[p.AttachableSGID]; done {
			continue
		}
		present[p.AttachableSGID] = struct{}{}
		tags = append(tags, tag)
	}
	if len(tags) == 0 {
		return content, nil
	}
	prefix := strings.Join(tags, " ") + " "
	if end := leadingBlockEnd(content); end >= 0 {
		return content[:end] + prefix + content[end:], nil
	}
	return prefix + content, nil
}
