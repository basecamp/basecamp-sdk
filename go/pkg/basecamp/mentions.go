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
// then an HMAC only BC3 can verify. The payload is a Marshal dump of a hash
// carrying the global id as a plain string — "gid://bc3/Person/1049715915" —
// which is what these helpers read. They do not (and cannot) verify the
// signature; what they decode is the same person id BC3 renders into the
// mention's avatar, read off content the API already served, and a caller that
// needs the id verified reads the person back through People().Get.
//
// Both Rails payload layouts are read: the older {"gid","purpose","expires_at"}
// hash and the current {"_rails" => {"data","pur"}} one. The reader searches the
// decoded bytes for the gid string rather than parsing the Marshal structure, so
// a future serializer that still carries the gid verbatim — Rails' JSON
// serializer does — keeps working without a change here.

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// bcAttachmentSGID matches one <bc-attachment …> opening tag and captures its
// sgid attribute, whichever quote style it uses. Attribute order is free, the
// tag may span lines, and case is ignored: BC3 emits lowercase, but a caller
// may hand over content it authored itself.
var bcAttachmentSGID = regexp.MustCompile(`(?is)<bc-attachment\b[^>]*?\ssgid\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// personGID matches the global id of a Person inside a decoded sgid payload.
// The application name is not pinned to "bc3": the id is what matters, and the
// gid app name is a deployment detail.
var personGID = regexp.MustCompile(`gid://[A-Za-z0-9_.-]+/Person/([0-9]+)(?:[?#"'\x00-\x20]|$)`)

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
	for _, m := range bcAttachmentSGID.FindAllStringSubmatch(richText, -1) {
		sgid := m[1]
		if sgid == "" {
			sgid = m[2]
		}
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

// PersonIDFromSGID decodes the Person id an attachable_sgid names. The second
// result is false when the sgid does not decode, or names something other than
// a Person (a file attachment's sgid names an ActiveStorage::Blob).
//
// This reads the id out of the sgid's payload; it does not verify the sgid's
// signature, which only BC3 can. See the package comment in mentions.go.
func PersonIDFromSGID(sgid string) (int64, bool) {
	payload, _, _ := strings.Cut(strings.TrimSpace(sgid), "--")
	if payload == "" {
		return 0, false
	}
	// Rails' MessageVerifier emits either alphabet; base64url is current. Both
	// decode through the standard alphabet once the two symbols are mapped,
	// and stripping the padding lets a truncated-but-valid payload through.
	normalized := strings.NewReplacer("-", "+", "_", "/").Replace(payload)
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(normalized, "="))
	if err != nil {
		return 0, false
	}
	m := personGID.FindSubmatch(raw)
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(string(m[1]), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
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
	return `<bc-attachment sgid="` + person.AttachableSGID + `"></bc-attachment>`, nil
}

// leadingBlockTag matches an opening <p …> or <div …> at the very start of a
// rich text, so mentions can be placed inside the first block rather than as
// a bare prefix in front of it.
var leadingBlockTag = regexp.MustCompile(`(?is)^\s*<(?:p|div)\b[^>]*>`)

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
	if loc := leadingBlockTag.FindStringIndex(content); loc != nil {
		return content[:loc[1]] + prefix + content[loc[1]:], nil
	}
	return prefix + content, nil
}
