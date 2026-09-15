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
// The envelope is decoded structurally, never searched as bytes, so a Person
// gid that merely appears inside some other value — a Document gid built from
// one, a purpose string that looks like one — is not a mention. Three
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

// bcAttachmentSGIDs returns the sgid attribute of every <bc-attachment> opening
// tag in the text, in document order. It tokenizes the tag's attributes rather
// than pattern-matching them, so a ">" inside a quoted attribute does not end
// the tag, an "sgid=" inside another attribute's value is not an attribute,
// either quote style works, attribute order and case are free, and entity
// escapes in the value are decoded as a browser would.
func bcAttachmentSGIDs(text string) []string {
	const tag = "<bc-attachment"
	var sgids []string
	for pos := 0; pos < len(text); {
		i := indexFold(text[pos:], tag)
		if i < 0 {
			break
		}
		start := pos + i + len(tag)
		// The tag name must end here: "<bc-attachments" is another element.
		if start < len(text) && !isTagNameEnd(text[start]) {
			pos = start
			continue
		}
		sgid, end, ok := parseAttributes(text, start)
		if ok && sgid != "" {
			sgids = append(sgids, sgid)
		}
		pos = end
	}
	return sgids
}

func isTagNameEnd(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '/' || c == '>'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// indexFold is strings.Index for ASCII, ignoring case.
func indexFold(s, substr string) int {
	n := len(substr)
	for i := 0; i+n <= len(s); i++ {
		if strings.EqualFold(s[i:i+n], substr) {
			return i
		}
	}
	return -1
}

// parseAttributes walks the attributes of an opening tag from pos (just after
// the tag name) to its closing ">", returning the decoded sgid attribute (the
// first one, when repeated), the index after the ">", and whether the tag was
// closed at all. An unterminated tag consumes the rest of the text.
func parseAttributes(text string, pos int) (sgid string, end int, ok bool) {
	for pos < len(text) {
		for pos < len(text) && (isSpace(text[pos]) || text[pos] == '/') {
			pos++
		}
		if pos >= len(text) {
			return sgid, pos, false
		}
		if text[pos] == '>' {
			return sgid, pos + 1, true
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
					return sgid, len(text), false
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
		if sgid == "" && strings.EqualFold(name, "sgid") {
			sgid = html.UnescapeString(value)
		}
	}
	return sgid, pos, false
}

// PersonIDFromSGID decodes the Person id an attachable_sgid names. The second
// result is false when the sgid does not decode, or names something other than
// a Person (a file attachment's sgid names an ActiveStorage::Blob).
//
// This reads the id out of the sgid's payload; it does not verify the sgid's
// signature, which only BC3 can. See the package comment in mentions.go.
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
func globalIDFromSGID(sgid string) (string, bool) {
	payload, _, _ := strings.Cut(strings.TrimSpace(sgid), "--")
	if payload == "" {
		return "", false
	}
	// Rails' MessageVerifier emits either alphabet; base64url is current. Both
	// decode through the standard alphabet once the two symbols are mapped,
	// and stripping the padding lets a truncated-but-valid payload through.
	normalized := strings.NewReplacer("-", "+", "_", "/").Replace(payload)
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(normalized, "="))
	if err != nil || len(raw) == 0 {
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
	// Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
	if rails, ok := top["_rails"].(map[string]any); ok {
		gid, ok := rails["data"].(string)
		return gid, ok && gid != ""
	}
	// Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
	gid, ok := top["gid"].(string)
	return gid, ok && gid != ""
}

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
	return v, nil
}

type rubyMarshalReader struct {
	data    []byte
	pos     int
	symbols []string
}

const rubyMarshalMaxDepth = 32

func (r *rubyMarshalReader) byte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("marshal: unexpected end of data")
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *rubyMarshalReader) bytes(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.data) {
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
	c := int64(int8(b))
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
		n, err := r.int()
		if err != nil {
			return nil, err
		}
		raw, err := r.bytes(int(n))
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case ':':
		n, err := r.int()
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
		n, err := r.int()
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
		n, err := r.int()
		if err != nil {
			return nil, err
		}
		if n < 0 || n > int64(len(r.data)) {
			return nil, fmt.Errorf("marshal: bad array length")
		}
		out := make([]any, 0, n)
		for i := int64(0); i < n; i++ {
			v, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case '{':
		n, err := r.int()
		if err != nil {
			return nil, err
		}
		if n < 0 || n > int64(len(r.data)) {
			return nil, fmt.Errorf("marshal: bad hash length")
		}
		out := make(map[string]any, n)
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
// somewhere other than a people read (a webhook payload, say).
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
	}
	return -1
}

// WithMentions returns content that mentions each of the given people, for
// posting as a comment or a Campfire line. People the content already mentions
// are left alone, so passing the same person twice — or a person the author
// already @-mentioned inline — never duplicates the mention; the rest are
// added at the start of the content, inside its first <p> or <div> when it
// opens with one, so they render on the first line rather than as a block of
// their own.
//
// Every person needs an attachable_sgid (see MentionMarkup); the account-bound
// CommentsService.ExpandMentions resolves ids to people first.
func WithMentions(content string, people []Person) (string, error) {
	already := map[int64]struct{}{}
	for _, id := range MentionedPersonIDs(content) {
		already[id] = struct{}{}
	}
	var tags []string
	for i := range people {
		p := &people[i]
		if _, done := already[p.ID]; done {
			continue
		}
		tag, err := MentionMarkup(p)
		if err != nil {
			return "", err
		}
		// Trust the id the caller passed only as far as the sgid agrees: the
		// tag names whoever the sgid names, so dedupe on that.
		if sgidID, ok := PersonIDFromSGID(p.AttachableSGID); ok {
			if _, done := already[sgidID]; done {
				continue
			}
			already[sgidID] = struct{}{}
		}
		already[p.ID] = struct{}{}
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
