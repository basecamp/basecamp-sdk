//! Mention helpers over Basecamp rich text.
//!
//! A mention in Basecamp rich text is a `<bc-attachment>` whose `sgid` attribute is the
//! mentioned person's `attachable_sgid` (`doc/api/sections/rich_text.md`, "Inserting a
//! mention"). BC3 renders the same tag back with
//! `content-type="application/vnd.basecamp.mention"` and an avatar figure inside it, but
//! the sgid is the only part of the markup that names the person on both the write and the
//! read side, so both helpers here work from it:
//!
//! - [`mentioned_person_ids`] reads the person ids a rich text names, by decoding the sgid
//!   of every `<bc-attachment>` and keeping the ones that point at a Person.
//! - [`mention_markup`] writes the tag for a person, from their `attachable_sgid`.
//!
//! An `attachable_sgid` is a Rails `SignedGlobalID`: a base64 payload, then `--`, then an
//! HMAC only BC3 can verify. The payload is an envelope carrying the global id —
//! `gid://bc3/Person/1049715915` — as a string, and that string is what these helpers read.
//! They do not (and cannot) verify the signature; what they decode is the same person id
//! BC3 renders into the mention's avatar, read off content the API already served, and a
//! caller that needs the id verified reads the person back through `people().get`.
//!
//! # The trust boundary
//!
//! READING — [`mentioned_person_ids`], [`person_id_from_sgid`] — describes what a text says
//! it mentions, and unsigned is fine for description: the ids are reported, not acted on as
//! proof. WRITING — [`with_mentions`],
//! [`CommentsService::expand_mentions`](crate::services::comments::CommentsService::expand_mentions)
//! — never treats an unsigned id as proof that a valid mention already exists: a forged or
//! stale sgid in caller-supplied content naming the right id would otherwise make the
//! writer skip the authoritative people read and post a tag Basecamp will not honour, so
//! the person is silently not mentioned. `expand_mentions` therefore resolves every
//! requested person through `people().get` and deduplicates only against the exact
//! `attachable_sgid` string that read returned. The pure helpers beneath it —
//! [`with_mentions`], [`mention_markup`] — take [`Person`] values the caller built and can
//! only check that an sgid is well-formed and names the person it is given, never that it
//! is authentic: hand them people the API returned, not people assembled from content. Do
//! not reuse the read-side helpers to decide whether a write can be skipped.
//!
//! # What the markup walk models
//!
//! The markup is read as BC3 serves it: a sanitized tree of the tags
//! `doc/api/sections/rich_text.md` allows, which has no raw-text elements. The tag walk
//! skips comments and quoted attribute values but does not model `<script>` or `<style>`
//! content, which BC3 strips on write; a caller reading mentions out of content it authored
//! itself should not put a `bc-attachment` inside such an element and expect it ignored.
//!
//! The envelope is decoded structurally, never searched as bytes, so a Person gid that
//! merely appears inside some other value — a Document gid built from one, a purpose string
//! that looks like one — is not a mention, and the envelope's purpose must be
//! `attachable`, the one BC3 accepts in rich text. Three envelopes are read: Rails' current
//! Marshal layout `{"_rails" => {"data" => gid, "pur" => purpose}}`, the older Marshal
//! layout `{"gid" => gid, "purpose" => …, "expires_at" => …}`, and the JSON spelling of
//! either, which Rails' JSON message serializer emits.

use std::borrow::Cow;
use std::collections::HashSet;

use serde_json::Value;

use crate::error::{Error, ErrorCode};
use crate::generated::types::Person;

/// The `SignedGlobalID` purpose BC3 mints attachable sgids with
/// (`doc/api/sections/rich_text.md`: `attachable_sgid`). Pinned by this module's purpose
/// tests, so a rename upstream breaks a test here rather than silently turning every
/// mention invisible.
const SGID_PURPOSE_ATTACHABLE: &str = "attachable";

/// Bounds the decoded sgid payload. A Person sgid's payload is under 200 bytes; the cap
/// keeps a hostile one from costing more than its own size to reject.
const MAX_SGID_PAYLOAD_BYTES: usize = 4096;

/// The same bound on the base64 form (4/3 of the payload, plus padding), checked before
/// anything is allocated.
const MAX_SGID_ENCODED_BYTES: usize = MAX_SGID_PAYLOAD_BYTES / 3 * 4 + 4;

/// The ids of the people a rich text mentions: the Person named by the sgid of each
/// `<bc-attachment>`, in document order, with repeats removed. Attachments that are not
/// mentions — files, images, embeds — are skipped, as is any sgid that does not decode to a
/// Person.
///
/// This is the read side: a description of what the text says, from sgids whose signatures
/// cannot be checked here. Report it; do not treat an id in it as proof that a valid
/// mention exists (see the trust boundary in the module docs).
///
/// Every `<bc-attachment>` in the text counts, including one inside a `<blockquote>`: BC3
/// notifies quoted mentions too, so the read matches what the server does with the write.
pub fn mentioned_person_ids(rich_text: &str) -> Vec<i64> {
    let mut ids = Vec::new();
    let mut seen = HashSet::new();
    for sgid in bc_attachment_sgids(rich_text) {
        let Some(id) = person_id_from_sgid(&sgid) else {
            continue;
        };
        if seen.insert(id) {
            ids.push(id);
        }
    }
    ids
}

/// The Person id an `attachable_sgid` names, or `None` when the sgid does not decode, or
/// names something other than a Person (a file attachment's sgid names an
/// `ActiveStorage::Blob`).
///
/// This reads the id out of the sgid's payload; it does not verify the sgid's signature,
/// which only BC3 can. It is a read-side helper: never use its answer to decide that a
/// write may skip the authoritative people read (see the trust boundary in the module
/// docs).
pub fn person_id_from_sgid(sgid: &str) -> Option<i64> {
    let gid = global_id_from_sgid(sgid)?;
    let (model, raw_id) = parse_global_id(&gid)?;
    if model != "Person" {
        return None;
    }
    let id: i64 = raw_id.parse().ok()?;
    (id > 0).then_some(id)
}

/// The `<bc-attachment>` that mentions a person, from their `attachable_sgid` — the
/// write-side form in `doc/api/sections/rich_text.md`, which BC3 expands into the avatar
/// figure on read.
///
/// It errors when the person carries no `attachable_sgid`, which is the case for a
/// [`Person`] projection that came from somewhere other than a people read (a webhook
/// payload, say), and when the sgid does not name the person it is given. That is all it
/// can check: it cannot verify the signature, so the [`Person`] must come from the API — a
/// `people().get`, a recording's creator or assignees — not be assembled from an sgid found
/// in content.
pub fn mention_markup(person: &Person) -> Result<String, Error> {
    let sgid = person.attachable_sgid.as_deref().unwrap_or_default();
    if sgid.is_empty() {
        return Err(Error::new(
            ErrorCode::Usage,
            format!("person {} has no attachable_sgid to mention", person.id),
        )
        .with_hint("read the person through people().get to obtain one"));
    }
    if sgid.contains(['"', '\'', '<', '>', '&']) {
        return Err(Error::new(
            ErrorCode::Usage,
            format!("person {} has a malformed attachable_sgid", person.id),
        ));
    }
    // The tag mentions whoever the sgid names. Refuse to write one that names someone else
    // — or a file — under this person's id.
    if person_id_from_sgid(sgid) != Some(person.id) {
        return Err(Error::new(
            ErrorCode::Usage,
            format!(
                "person {}'s attachable_sgid does not name that person",
                person.id
            ),
        )
        .with_hint("read the person through people().get to obtain their own"));
    }
    Ok(format!(r#"<bc-attachment sgid="{sgid}"></bc-attachment>"#))
}

/// Content that mentions each of the given people, for posting as a comment or a Campfire
/// line.
///
/// A person whose exact `attachable_sgid` the content already carries is left alone, so
/// passing the same person twice — or a person the author already mentioned with that sgid
/// — never duplicates the mention; the rest are added at the start of the content, inside
/// its first `<p>` or `<div>` when it opens with one, so they render on the first line
/// rather than as a block of their own.
///
/// This is the write side, and it deduplicates on the sgid string alone, never on the
/// person id an existing tag's sgid decodes to: that id is unsigned, and a forged or stale
/// tag naming the right person must not stand in for the real mention (see the trust
/// boundary in the module docs). Every person needs their own `attachable_sgid`, and it
/// must be one the API returned: this helper can check that an sgid is well-formed and
/// names the person, not that it is authentic (see [`mention_markup`]). The account-bound
/// [`CommentsService::expand_mentions`](crate::services::comments::CommentsService::expand_mentions)
/// resolves ids to people first and is the entry point that carries that guarantee.
pub fn with_mentions(content: &str, people: &[Person]) -> Result<String, Error> {
    let mut present: HashSet<String> = bc_attachment_sgids(content).into_iter().collect();
    let mut tags = Vec::new();
    for person in people {
        // Built before the dedupe check, so a person the helper cannot mention is refused
        // whether or not the content already carries their sgid.
        let tag = mention_markup(person)?;
        let sgid = person.attachable_sgid.clone().unwrap_or_default();
        if present.insert(sgid) {
            tags.push(tag);
        }
    }
    if tags.is_empty() {
        return Ok(content.to_string());
    }
    let prefix = format!("{} ", tags.join(" "));
    match leading_block_end(content) {
        Some(end) => Ok(format!("{}{prefix}{}", &content[..end], &content[end..])),
        None => Ok(format!("{prefix}{content}")),
    }
}

/// The `sgid` attribute of every `<bc-attachment>` in the text, in document order.
///
/// It walks the markup as a stream of tags rather than pattern-matching for one tag name,
/// so a `<bc-attachment>` inside an HTML comment or inside another element's quoted
/// attribute is not an element; and it tokenizes each tag's attributes rather than
/// pattern-matching them, so a `>` inside a quoted value does not end the tag, an `sgid=`
/// inside another attribute's value is not an attribute, either quote style works,
/// attribute order and case are free, the first `sgid` attribute wins as in HTML, and
/// entity escapes in the value are decoded as a browser would.
fn bc_attachment_sgids(text: &str) -> Vec<String> {
    let bytes = text.as_bytes();
    let mut sgids = Vec::new();
    let mut pos = 0usize;
    while pos < bytes.len() {
        let Some(offset) = bytes[pos..].iter().position(|byte| *byte == b'<') else {
            break;
        };
        pos += offset + 1;
        let rest = &bytes[pos..];
        if rest.starts_with(b"!--") {
            let Some(stop) = find(rest, b"-->") else {
                return sgids; // an unterminated comment swallows the rest
            };
            pos += stop + 3;
            continue;
        }
        if matches!(rest.first(), Some(b'!' | b'?' | b'/')) {
            let Some(stop) = rest.iter().position(|byte| *byte == b'>') else {
                return sgids;
            };
            pos += stop + 1;
            continue;
        }
        let name_end = rest
            .iter()
            .position(|byte| !is_tag_name_char(*byte))
            .unwrap_or(rest.len());
        if name_end == 0 {
            continue; // a bare "<" in text
        }
        let Some((attributes, end)) = parse_attributes(bytes, pos + name_end) else {
            return sgids; // an unterminated tag: nothing after it is markup
        };
        if equal_fold(&rest[..name_end], b"bc-attachment")
            && let Some(sgid) = attributes
            && !sgid.is_empty()
        {
            sgids.push(sgid);
        }
        pos = end;
    }
    sgids
}

/// What may follow `<` in a tag name. The whole name is consumed, punctuation included, so
/// `<bc-attachment:preview` or `<bc-attachment_x` is its own name and never compares equal
/// to `bc-attachment`.
fn is_tag_name_char(byte: u8) -> bool {
    !is_space(byte) && !matches!(byte, b'/' | b'>' | b'<' | b'=' | b'"' | b'\'')
}

fn is_tag_name_end(byte: u8) -> bool {
    is_space(byte) || matches!(byte, b'/' | b'>')
}

fn is_space(byte: u8) -> bool {
    matches!(byte, b' ' | b'\t' | b'\n' | b'\r' | 0x0c)
}

fn find(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack
        .windows(needle.len())
        .position(|window| window == needle)
}

/// Walks the attributes of an opening tag from `pos` (just after the tag name) to its
/// closing `>`, returning the decoded value of its first `sgid` attribute and the index
/// after the `>`, or `None` when the tag was never closed. The first `sgid` attribute wins,
/// present-but-empty included, as HTML resolves a repeated attribute.
fn parse_attributes(text: &[u8], mut pos: usize) -> Option<(Option<String>, usize)> {
    let mut sgid: Option<String> = None;
    while pos < text.len() {
        while pos < text.len() && (is_space(text[pos]) || text[pos] == b'/') {
            pos += 1;
        }
        if pos >= text.len() {
            return None;
        }
        if text[pos] == b'>' {
            return Some((sgid, pos + 1));
        }
        let name_start = pos;
        while pos < text.len() && !is_space(text[pos]) && !matches!(text[pos], b'=' | b'>' | b'/') {
            pos += 1;
        }
        let name = &text[name_start..pos];
        while pos < text.len() && is_space(text[pos]) {
            pos += 1;
        }
        let mut value: &[u8] = b"";
        if pos < text.len() && text[pos] == b'=' {
            pos += 1;
            while pos < text.len() && is_space(text[pos]) {
                pos += 1;
            }
            if pos < text.len() && matches!(text[pos], b'"' | b'\'') {
                let quote = text[pos];
                pos += 1;
                let closing = text[pos..].iter().position(|byte| *byte == quote)?;
                value = &text[pos..pos + closing];
                pos += closing + 1;
            } else {
                let value_start = pos;
                while pos < text.len() && !is_space(text[pos]) && text[pos] != b'>' {
                    pos += 1;
                }
                value = &text[value_start..pos];
            }
        }
        if name.is_empty() {
            // A stray "=" or quote where a name should be: step over it.
            pos += 1;
            continue;
        }
        if sgid.is_none() && equal_fold(name, b"sgid") {
            sgid = Some(unescape(&String::from_utf8_lossy(value)));
        }
    }
    None
}

/// The index just past the opening `<p …>` or `<div …>` tag a rich text starts with, or
/// `strings.EqualFold` against an ASCII needle, which is what the reference compares tag
/// and attribute names with — NOT an ASCII-only fold.
///
/// Go folds by Unicode simple case folding, and exactly two non-ASCII runes fold onto an
/// ASCII letter: U+017F LATIN SMALL LETTER LONG S onto `s`, and U+212A KELVIN SIGN onto
/// `k`. That was enumerated rather than recalled, and from the direction that does not
/// assume anything: every rune from U+0080 to U+10FFFF asked against every ASCII character
/// through `strings.EqualFold` itself. Walking the orbits out from the 128 ASCII code
/// points gives the same two, but only if folding is symmetric, which is a second thing to
/// be right about. So `<bc-attachment ſgid="…">` names a person in Go, and an ASCII-only
/// comparison silently reads it as an unknown attribute and names nobody.
///
/// Two of the three names compared here (`bc-attachment`, `<p`/`<div`) contain neither `s`
/// nor `k`, so an ASCII fold happens to agree on them today. They go through this function
/// anyway: that agreement is a property of the spelling, not of the rule, and it would go
/// away silently if a name ever gained an `s`.
///
/// A byte that is not valid UTF-8 cannot match: Go decodes it to U+FFFD, which folds to
/// nothing in ASCII.
fn equal_fold(text: &[u8], ascii_needle: &[u8]) -> bool {
    const LONG_S: &[u8] = &[0xc5, 0xbf]; // U+017F, in the orbit of `s`
    const KELVIN: &[u8] = &[0xe2, 0x84, 0xaa]; // U+212A, in the orbit of `k`
    let mut at = 0usize;
    for &wanted in ascii_needle {
        let rest = &text[at..];
        let Some(&byte) = rest.first() else {
            return false;
        };
        if byte < 0x80 {
            if !byte.eq_ignore_ascii_case(&wanted) {
                return false;
            }
            at += 1;
        } else if wanted.eq_ignore_ascii_case(&b's') && rest.starts_with(LONG_S) {
            at += LONG_S.len();
        } else if wanted.eq_ignore_ascii_case(&b'k') && rest.starts_with(KELVIN) {
            at += KELVIN.len();
        } else {
            return false;
        }
    }
    at == text.len()
}

/// `None` when it starts with anything else, so mentions can be placed inside the first
/// block rather than as a bare prefix in front of it. The tag's attributes are scanned
/// quote-aware: a `>` inside an attribute value does not end it.
fn leading_block_end(content: &str) -> Option<usize> {
    let bytes = content.as_bytes();
    let mut start = 0usize;
    while start < bytes.len() && is_space(bytes[start]) {
        start += 1;
    }
    for name in [b"<p".as_slice(), b"<div".as_slice()] {
        let after = start + name.len();
        if bytes.len() < after || !equal_fold(&bytes[start..after], name) {
            continue;
        }
        if after < bytes.len() && !is_tag_name_end(bytes[after]) {
            continue;
        }
        return parse_attributes(bytes, after).map(|(_, end)| end);
    }
    None
}

/// The longest name in [`VERDICT_RELEVANT_ENTITIES`] — `NonBreakingSpace;`, whose `;` is
/// part of the name. The leading `&` is accounted for separately in [`entity_at`]. Matching is
/// longest-first over that table rather than "consume the run of name characters", because
/// Go matches against its table the same way — which is what makes `&nbspBAh7…` resolve
/// there. A greedy name read swallows `nbspBAh7…` whole, matches nothing, and loses the
/// mention. The bound is also what keeps this LINEAR: without it, `"&".repeat(n) + ";"`
/// makes every failed parse rescan to the same far semicolon.
const MAX_ENTITY_NAME: usize = 17;

/// The named character references that can change which person an sgid names, with the
/// exact expansions `html.UnescapeString` produces — extracted from Go's own entity table
/// rather than recalled, and pinned by a differential test over all 2138 of its entries.
///
/// The full table is 2138 names and reproducing it would be its own liability. It is not
/// needed, and this is why — an sgid is base64url plus `=` padding and the `--` separator,
/// so exactly three classes of expansion can move the verdict:
///
/// 1. A character in the base64 alphabet. There are exactly FIVE such entities in the whole
///    HTML5 table, and all five are here.
/// 2. Whitespace, because the whole sgid is trimmed before it is read — so a space
///    expansion at either end is erased and the payload decodes, where a literal `&…` would
///    not. There are sixteen, and all sixteen are here, at their true code points rather
///    than folded, so Rust's `trim` (Unicode `White_Space`) erases exactly what Go's
///    `TrimSpace` (`unicode.IsSpace`) erases.
/// 3. CR and LF specifically, which a base64 decoder skips wherever they sit rather than
///    only at the ends. `&NewLine;` and `&Tab;` are therefore kept as themselves, never
///    folded to a space.
///
/// Every other expansion is a character that is neither in the alphabet nor whitespace, so
/// it kills the decode exactly as the literal `&name;` this leaves in its place does. Same
/// verdict, both ways — which the differential test asserts rather than assumes.
///
/// Go keeps a SECOND table for references that expand to two runes, and it matters twice:
/// `&fjlig;` is `f` + `j`, two alphabet characters — falsifying "letters and digits have no
/// named references" — and `&ThickSpace;` is two spaces. `&bne;` is the third with any
/// relevant rune (`=` plus a combining mark) and is deliberately absent: the mark is neither
/// alphabet nor whitespace, so it kills the decode wherever it lands, exactly as the literal
/// does.
///
/// The five standard references a serializer emits are here too, for the same reason they
/// are anywhere: they cost nothing and a reader expects them.
const VERDICT_RELEVANT_ENTITIES: &[(&str, &str)] = &[
    ("MediumSpace;", "\u{205f}"),
    ("NewLine;", "\u{a}"),
    ("NonBreakingSpace;", "\u{a0}"),
    ("Tab;", "\u{9}"),
    ("ThinSpace;", "\u{2009}"),
    ("UnderBar;", "\u{5f}"),
    ("ThickSpace;", "\u{205f}\u{200a}"),
    ("VeryThinSpace;", "\u{200a}"),
    ("amp", "\u{26}"),
    ("amp;", "\u{26}"),
    ("apos;", "\u{27}"),
    ("emsp;", "\u{2003}"),
    ("emsp13;", "\u{2004}"),
    ("emsp14;", "\u{2005}"),
    ("ensp;", "\u{2002}"),
    ("equals;", "\u{3d}"),
    ("fjlig;", "\u{66}\u{6a}"),
    ("gt", "\u{3e}"),
    ("gt;", "\u{3e}"),
    ("hairsp;", "\u{200a}"),
    ("lowbar;", "\u{5f}"),
    ("lt", "\u{3c}"),
    ("lt;", "\u{3c}"),
    ("nbsp", "\u{a0}"),
    ("nbsp;", "\u{a0}"),
    ("numsp;", "\u{2007}"),
    ("plus;", "\u{2b}"),
    ("puncsp;", "\u{2008}"),
    ("quot", "\u{22}"),
    ("quot;", "\u{22}"),
    ("sol;", "\u{2f}"),
    ("thinsp;", "\u{2009}"),
];

/// Decodes the character references in an attribute value the way Go's
/// `html.UnescapeString` does, for every reference that can change which person an sgid
/// names. Anything else is passed through verbatim, which is also what Go does with an
/// unknown entity — and, for a known one outside the three classes above, is
/// verdict-equivalent to what Go produces.
fn unescape(value: &str) -> String {
    if !value.contains('&') {
        return value.to_string();
    }
    let mut out = String::with_capacity(value.len());
    let mut rest = value;
    while let Some(start) = rest.find('&') {
        out.push_str(&rest[..start]);
        rest = &rest[start..];
        if let Some((decoded, length)) = entity_at(rest) {
            out.push_str(&decoded);
            rest = &rest[length..];
        } else {
            out.push('&');
            rest = &rest[1..];
        }
    }
    out.push_str(rest);
    out
}

/// The character reference at the start of `rest` (which begins with `&`), and how many
/// bytes it occupies.
fn entity_at(rest: &str) -> Option<(Cow<'static, str>, usize)> {
    if let Some(body) = rest.strip_prefix("&#") {
        return numeric_entity(body);
    }
    // Longest-first over the table, as Go matches: `&nbspBAh7…` is `nbsp` followed by text,
    // not a name called `nbspBAh7…`.
    let body = rest.get(1..)?;
    // Bounded by the NAME CHARACTERS actually present, not by the window. A table name is
    // `[A-Za-z0-9]+` and optionally its `;`, so nothing longer than that run can match —
    // and `&&&&…`, where the run is empty, costs one look instead of a full sweep of the
    // table at every length. That is the difference between a large linear constant and a
    // small one, on input an author controls.
    let run = body
        .bytes()
        .take_while(u8::is_ascii_alphanumeric)
        .count()
        .min(MAX_ENTITY_NAME);
    let semicolon = usize::from(body.as_bytes().get(run) == Some(&b';'));
    let limit = (run + semicolon).min(MAX_ENTITY_NAME);
    for length in (1..=limit).rev() {
        let Some(name) = body.get(..length) else {
            continue;
        };
        if let Some((_, decoded)) = VERDICT_RELEVANT_ENTITIES
            .iter()
            .find(|(candidate, _)| *candidate == name)
        {
            return Some((Cow::Borrowed(*decoded), 1 + length));
        }
    }
    None
}

/// What a numeric reference in `0x80..=0x9F` actually means. Those are not code points in
/// HTML — they are Windows-1252 bytes, and a decoder remaps them. `&#133;` is a horizontal
/// ellipsis, NOT U+0085 NEL.
///
/// Getting this wrong is not cosmetic here, and the mechanism is worth stating because it
/// is not obvious: U+0085 IS Unicode whitespace, so a decoder that reads 133 as a code
/// point produces a character the surrounding trim then deletes — and `<real sgid>&#133;`
/// resolves to a person Go refuses. That is the accepting direction, on the helper a
/// connector admits events by.
///
/// Read off `html.UnescapeString` by probing all thirty-two, not off a spec: the two are not
/// interchangeable here, and five of the thirty-two (0x81, 0x8D, 0x8F, 0x90, 0x9D) map to
/// themselves while the rest do not.
const C1_REPLACEMENTS: [char; 32] = [
    '\u{20ac}', '\u{81}', '\u{201a}', '\u{192}', '\u{201e}', '\u{2026}', '\u{2020}', '\u{2021}',
    '\u{2c6}', '\u{2030}', '\u{160}', '\u{2039}', '\u{152}', '\u{8d}', '\u{17d}', '\u{8f}',
    '\u{90}', '\u{2018}', '\u{2019}', '\u{201c}', '\u{201d}', '\u{2022}', '\u{2013}', '\u{2014}',
    '\u{2dc}', '\u{2122}', '\u{161}', '\u{203a}', '\u{153}', '\u{9d}', '\u{17e}', '\u{178}',
];

/// The character a numeric reference's value denotes, with the substitutions a decoder makes
/// — all measured against `html.UnescapeString` rather than assumed.
fn numeric_scalar(code: u32) -> char {
    const REPLACEMENT: char = '\u{fffd}';
    match code {
        0x80..=0x9f => C1_REPLACEMENTS[(code - 0x80) as usize],
        // Zero, the surrogates, and anything past the last scalar all become the
        // replacement character — `from_u32` already answers `None` for the last two, so
        // only zero needs naming.
        0 => REPLACEMENT,
        code => char::from_u32(code).unwrap_or(REPLACEMENT),
    }
}

/// A numeric character reference, on Go's boundary rather than the obvious one.
///
/// The reference is refused when its text — `&#`, the digits, and the `;` if there is one —
/// is three characters or fewer. That is why `&#9` and `&#9B` stay literal while `&#9;` is a
/// tab and `&#xB` is a vertical tab: the rule counts CHARACTERS, not digits, so a semicolon
/// is worth as much as a digit and the hex `x` is worth one on its own. `&#x;` is the odd
/// corner, four characters with no digits at all, and decodes to the replacement character.
///
/// Read off the real function by probing it. Nobody would derive this from its source, and a
/// port that counts digits gets `&#9;` wrong in the vanishing direction.
fn numeric_entity(body: &str) -> Option<(Cow<'static, str>, usize)> {
    let (radix, digits, prefix) = match body.strip_prefix(['x', 'X']) {
        Some(hex) => (16, hex, 3usize),
        None => (10, body, 2usize),
    };
    let taken = digits
        .find(|character: char| !character.is_digit(radix))
        .unwrap_or(digits.len());
    let semicolon = usize::from(digits[taken..].starts_with(';'));
    let consumed = prefix + taken + semicolon;
    if consumed <= 3 || (taken == 0 && (radix == 10 || semicolon == 0)) {
        return None;
    }
    // Go accumulates into a `rune`, which is an `int32`, and lets it WRAP — the value is
    // the low 32 bits, and the C1, surrogate and out-of-range checks then run on that. So
    // `&#x100000041;` is "A" there, not a replacement character. Mirrored rather than
    // clamped, because clamping only ever loses a mention: it can never name someone Go
    // does not, but it can miss someone Go finds.
    let mut accumulator: i32 = 0;
    let radix = i32::try_from(radix).ok()?;
    for digit in digits[..taken].chars() {
        let value = i32::try_from(digit.to_digit(36)?).ok()?;
        accumulator = accumulator.wrapping_mul(radix).wrapping_add(value);
    }
    #[allow(clippy::cast_sign_loss)] // the wrap is the point: this is Go's rune
    let code = accumulator as u32;
    Some((Cow::Owned(numeric_scalar(code).to_string()), consumed))
}

/// The global id string an sgid's envelope carries.
///
/// A signed sgid is `<payload>--<digest>`, and `-` is a base64url character, so the payload
/// itself may contain `--`. The separator is therefore the LAST one, as Rails' own verifier
/// reads it; the whole value is tried as a bare payload when that fails, which is what an
/// unsigned envelope — one that happens to contain `--` included — needs.
fn global_id_from_sgid(sgid: &str) -> Option<String> {
    let value = sgid.trim();
    if let Some(index) = value.rfind("--")
        && index > 0
        && let Some(gid) = envelope_gid(&value[..index])
    {
        return Some(gid);
    }
    envelope_gid(value)
}

/// Decodes one base64 payload and returns the gid its envelope carries.
fn envelope_gid(payload: &str) -> Option<String> {
    // The bound is applied to the encoded form first, so an oversized sgid costs nothing to
    // refuse — no normalization, no decode buffer.
    if payload.is_empty() || payload.len() > MAX_SGID_ENCODED_BYTES {
        return None;
    }
    // Rails' MessageVerifier emits either alphabet; base64url is current. Both decode
    // through the standard alphabet once the two symbols are mapped, and stripping the
    // padding lets a truncated-but-valid payload through.
    let normalized: String = payload
        .trim_end_matches('=')
        .chars()
        .map(|character| match character {
            '-' => '+',
            '_' => '/',
            other => other,
        })
        .collect();
    let raw = base64_decode(&normalized)?;
    if raw.is_empty() || raw.len() > MAX_SGID_PAYLOAD_BYTES {
        return None;
    }
    let envelope = if raw.starts_with(&[0x04, 0x08]) {
        unmarshal_ruby(&raw[2..]).ok()?
    } else if raw[0] == b'{' {
        serde_json::from_slice::<Value>(&raw).ok()?
    } else {
        return None;
    };
    let top = envelope.as_object()?;
    // A SignedGlobalID is bound to a purpose, and only an "attachable" one may be placed in
    // rich text: BC3 refuses any other, so a Person sgid minted for bookmarking or reading
    // is not a mention however valid its gid. Both layouts carry the purpose; an envelope
    // without one is not a Rails envelope.
    //
    // Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
    if let Some(rails) = top.get("_rails").and_then(Value::as_object) {
        if rails.get("pur").and_then(Value::as_str) != Some(SGID_PURPOSE_ATTACHABLE) {
            return None;
        }
        return rails
            .get("data")
            .and_then(Value::as_str)
            .filter(|gid| !gid.is_empty())
            .map(str::to_string);
    }
    // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
    if top.get("purpose").and_then(Value::as_str) != Some(SGID_PURPOSE_ATTACHABLE) {
        return None;
    }
    top.get("gid")
        .and_then(Value::as_str)
        .filter(|gid| !gid.is_empty())
        .map(str::to_string)
}

/// A `GlobalID`'s model and raw id, read the way `net/url` reads it — the parser the
/// reference implementation uses, so its answers are the contract.
///
/// The authority is where a hand-rolled split goes wrong, and it goes wrong in the
/// accepting direction, on a helper a connector admits events by. Every rule below was
/// measured against `url.Parse` rather than derived from RFC 3986, because the two differ:
///
/// - The scheme is ASCII case-insensitive.
/// - The path is percent-DECODED before it is read, so `gid://bc3/Person/%37%37` names 77.
/// - Userinfo before the last `@` is dropped, but only after being VALIDATED. `u s@bc3` and
///   `é@bc3` are parse errors, not hosts called `bc3`.
/// - What `u.Host` holds INCLUDES the port, so `gid://:8080/…` has a non-empty host and
///   resolves; stripping the port before the emptiness check refuses what Go accepts.
/// - A bracketed literal must really be one: the brackets are matched from the FRONT, the
///   content must parse as an IPv6 address and not as IPv4, and a `[` anywhere else is an
///   error. `[bad]`, `[192.0.2.1]` and `bc3[` are all refused.
/// - The host's character set is an ALLOWLIST. `<`, `"` and `;` are legal in a host; `^`,
///   `` ` ``, `{`, `|` and `}` are not.
///
/// The query and the fragment are cut first, before the authority, because that is where
/// they begin: in `gid://bc3?x/Person/77` the `?` ends the host and the URL has no path.
fn parse_global_id(gid: &str) -> Option<(String, String)> {
    // A URL parser refuses a control byte anywhere up to the fragment, whatever part it
    // lands in, so that check comes before the URL is taken apart at all.
    let (before_fragment, fragment) = match gid.split_once('#') {
        Some((before, fragment)) => (before, Some(fragment)),
        None => (gid, None),
    };
    if before_fragment
        .bytes()
        .any(|byte| byte < b' ' || byte == 0x7f)
    {
        return None;
    }
    // The fragment is discarded, but not before it is CHECKED: a URL parser unescapes it,
    // and a malformed escape there fails the whole parse. Nothing downstream can notice —
    // which is exactly why this was wrong, and wrong in the accepting direction, naming a
    // person for `gid://bc3/Person/77#%zz` where Go names nobody.
    //
    // The fragment's rule is its own and is NOT the host's: only "a `%` must be followed by
    // two hex digits". The query, cut in the same breath below, is not checked at all,
    // because a URL parser keeps it raw and never unescapes it — `?%zz` parses. Three
    // positions, three rules; the escape rule verified at one of them says nothing about
    // the others.
    if let Some(fragment) = fragment
        && !valid_escapes(fragment)
    {
        return None;
    }
    let after_scheme = strip_scheme(gid)?;
    let authority_and_path = after_scheme.split(['?', '#']).next().unwrap_or_default();
    let (authority, path) = authority_and_path.split_once('/')?;
    let host = parse_authority(authority)?;
    if host.is_empty() {
        return None;
    }
    let path = percent_decode(path)?;
    let (model, raw_id) = path.split_once('/')?;
    if model.is_empty()
        || raw_id.is_empty()
        || raw_id.contains('/')
        || !raw_id.bytes().all(|byte| byte.is_ascii_digit())
    {
        return None;
    }
    Some((model.to_string(), raw_id.to_string()))
}

/// Whether every `%` in a string introduces two hex digits — the plain unescape rule, with
/// none of the extra conditions the host and the zone add. A `%` in the last two bytes is
/// malformed for want of room.
fn valid_escapes(text: &str) -> bool {
    let bytes = text.as_bytes();
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'%' {
            if index + 2 >= bytes.len()
                || !bytes[index + 1].is_ascii_hexdigit()
                || !bytes[index + 2].is_ascii_hexdigit()
            {
                return false;
            }
            index += 3;
        } else {
            index += 1;
        }
    }
    true
}

/// What follows `gid://`, matching the scheme case-insensitively as a URL parser does.
fn strip_scheme(gid: &str) -> Option<&str> {
    let (scheme, rest) = gid.split_once("://")?;
    // ASCII-only here, and that is NOT an oversight to be tidied into `equal_fold` above.
    // A URL parser's scheme grammar admits only ASCII letters and digits, so a scheme
    // carrying `ı` or `İ` is not a scheme at all and the gid never matches. Folding it the
    // way a tag name is folded would resolve `gıd://bc3/Person/7` to a person. Two rules,
    // one file; the reference uses a different one in each place.
    scheme.eq_ignore_ascii_case("gid").then_some(rest)
}

/// The host an authority names, as `u.Host` holds it: userinfo validated and dropped, the
/// port kept, the rest percent-unescaped.
fn parse_authority(authority: &str) -> Option<String> {
    match authority.rfind('@') {
        Some(at) => {
            if !valid_userinfo(&authority[..at]) {
                return None;
            }
            parse_host(&authority[at + 1..])
        }
        None => parse_host(authority),
    }
}

/// Userinfo's own allowlist, which is wider than the host's: alphanumerics plus
/// `-._:~!$&'()*+,;=%@`. Anything else — a space, a quote, any non-ASCII — is a parse error
/// rather than something to discard.
fn valid_userinfo(userinfo: &str) -> bool {
    // Userinfo is unescaped as well as validated, so a `%` that does not introduce two hex
    // digits is a parse error rather than a literal percent.
    let bytes = userinfo.as_bytes();
    for (index, byte) in bytes.iter().enumerate() {
        if *byte == b'%'
            && !userinfo
                .get(index + 1..index + 3)
                .is_some_and(|hex| hex.bytes().all(|byte| byte.is_ascii_hexdigit()))
        {
            return false;
        }
    }
    userinfo.bytes().all(|byte| {
        byte.is_ascii_alphanumeric()
            || matches!(
                byte,
                b'-' | b'.'
                    | b'_'
                    | b':'
                    | b'~'
                    | b'!'
                    | b'$'
                    | b'&'
                    | b'\''
                    | b'('
                    | b')'
                    | b'*'
                    | b'+'
                    | b','
                    | b';'
                    | b'='
                    | b'%'
                    | b'@'
            )
    })
}

/// A host, with its optional port, validated and unescaped as `parseHost` does.
///
/// The bracketed branch validates the literal as an ADDRESS, and it does so whether or not
/// the literal carries a zone. An earlier version returned early on a zone and skipped the
/// check, on a premise its own comment stated and no probe had ever tested: `[::1%25eth0]`
/// parses under both hypotheses, because it is a valid address carrying a valid zone. What
/// distinguishes them is a zone on an INVALID body — `[not-an-address%25zone]` — which the
/// early return accepted and Go refuses.
fn parse_host(host: &str) -> Option<String> {
    let Some(open) = host.rfind('[') else {
        // No literal: an optional port, then the host's own escape and character rules.
        if let Some(colon) = host.rfind(':')
            && !valid_optional_port(&host[colon..])
        {
            return None;
        }
        return unescape_in(host, HostPart::Host);
    };
    if open > 0 {
        return None; // a bracket anywhere but the front is not an IP-literal
    }
    let close = host.rfind(']')?;
    let colon_port = &host[close + 1..];
    if !valid_optional_port(colon_port) {
        return None;
    }
    let unescaped_port = unescape_in(colon_port, HostPart::Host)?;
    let hostname = &host[1..close];
    // RFC 6874: `%25` introduces a zone identifier, and the zone's escape rule is its own —
    // wider than the host's, because a zone may spell out bytes the host may not.
    let unescaped = match hostname.find("%25") {
        Some(zone) => {
            let head = unescape_in(&hostname[..zone], HostPart::Host)?;
            let tail = unescape_in(&hostname[zone..], HostPart::Zone)?;
            format!("{head}{tail}")
        }
        None => unescape_in(hostname, HostPart::Host)?,
    };
    // Only a valid IPv6 address may be bracketed. The zone is split off first because the
    // address parser here does not take one, and an empty zone is refused as it is there.
    let (address, zone) = match unescaped.split_once('%') {
        Some((address, zone)) => (address, Some(zone)),
        None => (unescaped.as_str(), None),
    };
    if zone == Some("") {
        return None;
    }
    let address: std::net::IpAddr = address.parse().ok()?;
    if address.is_ipv4() {
        return None; // an IPv4 address in brackets is not an IP-literal
    }
    Some(format!("[{unescaped}]{unescaped_port}"))
}

/// Which escape rule applies: a host's, or the zone identifier's inside an IP-literal.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum HostPart {
    Host,
    Zone,
}

/// `validOptionalPort`: empty, or a `:` followed by digits only.
fn valid_optional_port(port: &str) -> bool {
    match port.strip_prefix(':') {
        None => port.is_empty(),
        Some(digits) => digits.bytes().all(|byte| byte.is_ascii_digit()),
    }
}

/// Percent-unescapes a host or a zone identifier and enforces the allowlist.
///
/// The two differ only in which escapes they admit, and the difference is not small. A HOST
/// refuses an escape whose first hex digit is below 8 unless the triple is `%25` — so `%63`
/// is an error while `%80`, `%C3%A9` and `%25` are not, because a host may percent-encode
/// its non-ASCII bytes and `%25` is how a literal `%` is written. A ZONE instead admits any
/// escape whose decoded byte it could have written directly, plus a space (RFC 6874 says
/// anything goes; Go restricts it to host-valid bytes, and then Windows puts spaces there).
/// So `%41`, `%20` and `%7E` are legal in a zone and illegal in a host — applying the host
/// rule to a zone loses addresses Go keeps.
fn unescape_in(host: &str, part: HostPart) -> Option<String> {
    let bytes = host.as_bytes();
    let mut out = Vec::with_capacity(bytes.len());
    let mut pos = 0usize;
    while pos < bytes.len() {
        match bytes[pos] {
            b'%' => {
                let triple = host.get(pos..pos + 3)?;
                let hex = &triple[1..];
                let value = u8::from_str_radix(hex, 16).ok()?;
                if !hex.bytes().all(|byte| byte.is_ascii_hexdigit()) {
                    return None;
                }
                let refused = match part {
                    HostPart::Host => unhex(bytes[pos + 1]) < 8 && triple != "%25",
                    HostPart::Zone => triple != "%25" && value != b' ' && !allowed_in_host(value),
                };
                if refused {
                    return None;
                }
                out.push(value);
                pos += 3;
            }
            byte if byte < 0x80 && !allowed_in_host(byte) => return None,
            byte => {
                out.push(byte);
                pos += 1;
            }
        }
    }
    // A host is a byte string, not text: `%80` is a legal escape and decodes to a byte no
    // UTF-8 sequence starts with. Only its EMPTINESS is read afterwards, so it is rendered
    // lossily rather than refused — refusing would lose a host Go keeps.
    Some(String::from_utf8_lossy(&out).into_owned())
}

fn unhex(byte: u8) -> u8 {
    match byte {
        b'0'..=b'9' => byte - b'0',
        b'a'..=b'f' => byte - b'a' + 10,
        b'A'..=b'F' => byte - b'A' + 10,
        _ => 255,
    }
}

/// The ASCII bytes a host may carry unescaped. An allowlist, not a denylist: `<`, `"` and
/// `;` are in it and `^`, `` ` ``, `{`, `|`, `}` are not, which a denylist gets backwards.
fn allowed_in_host(byte: u8) -> bool {
    byte.is_ascii_alphanumeric()
        || matches!(
            byte,
            b'-' | b'_'
                | b'.'
                | b'~'
                | b'!'
                | b'$'
                | b'&'
                | b'\''
                | b'('
                | b')'
                | b'*'
                | b'+'
                | b','
                | b';'
                | b'='
                | b':'
                | b'['
                | b']'
                | b'<'
                | b'>'
                | b'"'
        )
}

/// Percent-decodes a URL path, refusing a truncated or non-hex escape as a parser would.
/// The result must still be text; a decode that is not UTF-8 names nobody.
fn percent_decode(path: &str) -> Option<String> {
    if !path.contains('%') {
        return Some(path.to_string());
    }
    let bytes = path.as_bytes();
    let mut out = Vec::with_capacity(bytes.len());
    let mut pos = 0usize;
    while pos < bytes.len() {
        if bytes[pos] == b'%' {
            let hex = path.get(pos + 1..pos + 3)?;
            if !hex.bytes().all(|byte| byte.is_ascii_hexdigit()) {
                return None;
            }
            out.push(u8::from_str_radix(hex, 16).ok()?);
            pos += 3;
        } else {
            out.push(bytes[pos]);
            pos += 1;
        }
    }
    String::from_utf8(out).ok()
}

/// Decodes standard-alphabet base64 without padding, matching Go's `RawStdEncoding` — the
/// decoder the reference implementation reads sgids with — in both directions.
///
/// Matching it EXACTLY is the requirement, not being strict and not being lenient. This is
/// the read side of a mention: a payload BC3 minted that Go decodes and this refuses does
/// not surface as an error anywhere, it makes a real mention silently vanish. So two
/// deliberate leniencies are copied from Go rather than tightened:
///
/// - Line breaks are SKIPPED, and only line breaks. Go's `decodeQuantum` steps over `\r`
///   and `\n` wherever they appear; a space is still an error there and here.
/// - The final group's unused bits are NOT required to be zero. Go's non-strict decoder
///   ignores them (`RawStdEncoding.DecodeString("QR")` is `[65]`, not an error), and only
///   `.Strict()` would reject them.
///
/// What Go does reject is copied too: a final group of six leftover bits is a corrupt
/// payload, not a truncated byte.
///
/// Rolling this by hand keeps the read side free of a dependency the crate only enables
/// under the `oauth` feature — and, more to the point, free of an engine whose defaults are
/// strict on exactly these two axes.
fn base64_decode(input: &str) -> Option<Vec<u8>> {
    let mut out = Vec::with_capacity(input.len() / 4 * 3);
    let mut accumulator: u32 = 0;
    let mut bits = 0u32;
    for byte in input.bytes() {
        let value = match byte {
            b'A'..=b'Z' => byte - b'A',
            b'a'..=b'z' => byte - b'a' + 26,
            b'0'..=b'9' => byte - b'0' + 52,
            b'+' => 62,
            b'/' => 63,
            b'\n' | b'\r' => continue,
            _ => return None,
        };
        // Only the bits not yet emitted are kept. Rust's `<<` discards what leaves the top
        // and checks the shift AMOUNT rather than the value, so an unmasked accumulator
        // would neither panic nor mis-decode — `>> bits & 0xff` reads only the sextets just
        // added. Masking anyway, because "the high bits are stale but never read" is a
        // property a reader has to reconstruct, and `bits` never exceeds 13 here.
        accumulator = ((accumulator << 6) | u32::from(value)) & 0x3fff;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push(u8::try_from((accumulator >> bits) & 0xff).ok()?);
        }
    }
    (bits < 6).then_some(out)
}

/// Bounds nesting in a payload; an envelope is two deep.
const RUBY_MARSHAL_MAX_DEPTH: u32 = 32;

/// Decodes the subset of Ruby's Marshal 4.8 format a `SignedGlobalID` payload uses — nil,
/// booleans, fixnums, strings (with their encoding ivars), symbols and symbol links, arrays
/// and hashes — into [`Value`]. Anything else is an error; the caller then treats the sgid
/// as undecodable rather than guessing.
fn unmarshal_ruby(data: &[u8]) -> Result<Value, &'static str> {
    let mut reader = MarshalReader {
        data,
        pos: 0,
        symbols: Vec::new(),
    };
    let value = reader.value(0)?;
    if reader.pos != data.len() {
        // A Marshal dump is exactly one value; bytes after it are corruption.
        return Err("marshal: trailing bytes");
    }
    Ok(value)
}

struct MarshalReader<'a> {
    data: &'a [u8],
    pos: usize,
    symbols: Vec<String>,
}

impl MarshalReader<'_> {
    fn byte(&mut self) -> Result<u8, &'static str> {
        let byte = *self
            .data
            .get(self.pos)
            .ok_or("marshal: unexpected end of data")?;
        self.pos += 1;
        Ok(byte)
    }

    /// The next `n` bytes. The bound is checked against what remains, never by adding `n`
    /// to the position, so no length can overflow its way past the check.
    fn bytes(&mut self, n: usize) -> Result<&[u8], &'static str> {
        if n > self.data.len() - self.pos {
            return Err("marshal: unexpected end of data");
        }
        let taken = &self.data[self.pos..self.pos + n];
        self.pos += n;
        Ok(taken)
    }

    /// Marshal's packed integer: 0 is 0; 1..4 and -1..-4 are a byte count for a
    /// little-endian value; anything else is the value itself offset by 5.
    fn int(&mut self) -> Result<i64, &'static str> {
        let lead = i64::from(i8::from_ne_bytes([self.byte()?]));
        match lead {
            0 => Ok(0),
            c if c > 4 => Ok(c - 5),
            c if c < -4 => Ok(c + 5),
            c if c > 0 => {
                let raw = self.bytes(usize::try_from(c).unwrap_or_default())?;
                let mut value: i64 = 0;
                for (index, byte) in raw.iter().enumerate() {
                    value |= i64::from(*byte) << (8 * index);
                }
                Ok(value)
            }
            c => {
                let raw = self.bytes(usize::try_from(-c).unwrap_or_default())?;
                let mut value: i64 = -1;
                for (index, byte) in raw.iter().enumerate() {
                    value &= !(0xff_i64 << (8 * index));
                    value |= i64::from(*byte) << (8 * index);
                }
                Ok(value)
            }
        }
    }

    /// A length or count — string and symbol bytes, array elements, hash pairs, ivar pairs
    /// — refusing one that cannot be honest: negative, or more than the bytes left (every
    /// element takes at least one byte). Allocation follows what actually decodes, so a
    /// hostile count costs its own bytes to refuse, never the capacity it claims.
    fn count(&mut self) -> Result<usize, &'static str> {
        let n = self.int()?;
        let remaining = i64::try_from(self.data.len() - self.pos).unwrap_or(i64::MAX);
        if n < 0 || n > remaining {
            return Err("marshal: bad count");
        }
        usize::try_from(n).map_err(|_| "marshal: bad count")
    }

    fn value(&mut self, depth: u32) -> Result<Value, &'static str> {
        if depth > RUBY_MARSHAL_MAX_DEPTH {
            return Err("marshal: nesting too deep");
        }
        match self.byte()? {
            b'0' => Ok(Value::Null),
            b'T' => Ok(Value::Bool(true)),
            b'F' => Ok(Value::Bool(false)),
            b'i' => Ok(Value::from(self.int()?)),
            b'"' => {
                let n = self.count()?;
                let raw = self.bytes(n)?;
                Ok(Value::String(String::from_utf8_lossy(raw).into_owned()))
            }
            b':' => {
                let n = self.count()?;
                let symbol = String::from_utf8_lossy(self.bytes(n)?).into_owned();
                self.symbols.push(symbol.clone());
                Ok(Value::String(symbol))
            }
            b';' => {
                let index = self.int()?;
                let symbol = usize::try_from(index)
                    .ok()
                    .and_then(|index| self.symbols.get(index))
                    .ok_or("marshal: bad symbol link")?;
                Ok(Value::String(symbol.clone()))
            }
            b'I' => {
                // An object followed by its instance variables — a String's encoding.
                let inner = self.value(depth + 1)?;
                let n = self.count()?;
                for _ in 0..n {
                    self.value(depth + 1)?; // ivar name
                    self.value(depth + 1)?; // ivar value
                }
                Ok(inner)
            }
            b'[' => {
                let n = self.count()?;
                let mut items = Vec::new();
                for _ in 0..n {
                    items.push(self.value(depth + 1)?);
                }
                Ok(Value::Array(items))
            }
            b'{' => {
                let n = self.count()?;
                let mut map = serde_json::Map::new();
                for _ in 0..n {
                    let key = self.value(depth + 1)?;
                    let value = self.value(depth + 1)?;
                    let Value::String(key) = key else {
                        return Err("marshal: non-string hash key");
                    };
                    map.insert(key, value);
                }
                Ok(Value::Object(map))
            }
            _ => Err("marshal: unsupported type"),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The digest of the decode below, measured on rustc 1.98.0. The MSRV job builds on 1.88
    /// and runs this too; if the two toolchains ever decode differently, that job fails here.
    const DECODE_DIGEST: u64 = 419_185_206_463_269_138;

    /// `{"gid" => "gid://bc3/Person/1049715915?expires_in", "purpose" => "attachable",
    /// "expires_at" => nil}`, Marshal 4.8, as BC3 mints it.
    const ANNIE_SGID: &str = "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aeb392ebf54ffd820e45f27add22bae3a8c7da56";

    fn person(id: i64, sgid: Option<&str>) -> Person {
        Person {
            id,
            attachable_sgid: sgid.map(str::to_string),
            ..Default::default()
        }
    }

    #[test]
    fn a_marshal_envelope_names_its_person() {
        assert_eq!(person_id_from_sgid(ANNIE_SGID), Some(1_049_715_915));
    }

    #[test]
    fn a_json_envelope_names_its_person_in_both_layouts() {
        let current =
            json_sgid(r#"{"_rails":{"data":"gid://bc3/Person/77","pur":"attachable","exp":null}}"#);
        let older = json_sgid(r#"{"gid":"gid://bc3/Person/77","purpose":"attachable"}"#);
        assert_eq!(person_id_from_sgid(&current), Some(77));
        assert_eq!(person_id_from_sgid(&older), Some(77));
    }

    #[test]
    fn a_purpose_other_than_attachable_is_not_a_mention() {
        let readable = json_sgid(r#"{"gid":"gid://bc3/Person/77","purpose":"readable"}"#);
        let rails = json_sgid(r#"{"_rails":{"data":"gid://bc3/Person/77","pur":"readable"}}"#);
        let none = json_sgid(r#"{"gid":"gid://bc3/Person/77"}"#);
        assert_eq!(person_id_from_sgid(&readable), None);
        assert_eq!(person_id_from_sgid(&rails), None);
        assert_eq!(person_id_from_sgid(&none), None);
    }

    /// Every row measured against `net/url` + the Go helper, not recalled.
    ///
    /// The authority rows are the ones worth keeping: a hand-rolled split gets userinfo, a
    /// non-numeric port and a percent-escape wrong in DIFFERENT directions, and three of
    /// them name a person Go names nobody for.
    #[test]
    fn the_gid_parse_answers_what_net_url_answers() {
        let go: &[(&str, Option<i64>)] = &[
            // Userinfo before the last `@` is dropped; a URL parser accepts these.
            ("gid://user@bc3/Person/77", Some(77)),
            ("gid://user:pw@bc3/Person/77", Some(77)),
            ("gid://a@b@bc3/Person/77", Some(77)),
            ("gid://@bc3/Person/77", Some(77)),
            // A port must be digits. `bc3:xx` is not a host with a funny name, it is a
            // parse error; an empty port is fine.
            ("gid://bc3:8080/Person/77", Some(77)),
            ("gid://bc3:/Person/77", Some(77)),
            ("gid://bc3:xx/Person/77", None),
            ("gid://[::1]/Person/77", Some(77)),
            // `net/url` refuses EVERY percent-escape in a host — measured, not assumed —
            // so there is nothing to decode. Decoding first and validating after is the
            // natural design and admits `b%63%33` as `bc3`, which Go refuses.
            ("gid://b%63%33/Person/77", None),
            ("gid://%20/Person/77", None),
            // A control byte anywhere up to the fragment is refused; inside it is not.
            // Written as the JSON escape the envelope actually carries, so the control
            // byte reaches the parser rather than breaking the envelope around it.
            (r"gid://bc3/Person/77?\u0001", None),
            (r"gid://bc3/Person/77#\u0001", Some(77)),
            // Userinfo is VALIDATED, not merely discarded: its own allowlist is wider than
            // the host's, but a space, a non-ASCII byte or a malformed escape is a parse
            // error rather than something to drop.
            ("gid://user:pw@bc3/Person/77", Some(77)),
            ("gid://%40@bc3/Person/77", Some(77)),
            ("gid://u s@bc3/Person/77", None),
            ("gid://u%s@bc3/Person/77", None),
            // A bracketed literal must really be one: matched from the FRONT, parsed as an
            // address, and refused if it is IPv4. A `[` anywhere else is not a literal.
            ("gid://[::1]/Person/77", Some(77)),
            ("gid://[::1]:80/Person/77", Some(77)),
            ("gid://[::1%25eth0]/Person/77", Some(77)),
            ("gid://[bad]/Person/77", None),
            ("gid://[192.0.2.1]/Person/77", None),
            ("gid://[::1/Person/77", None),
            ("gid://bc3[/Person/77", None),
            ("gid://a:1]/Person/77", None),
            // A bracketed authority that PARSES but is not an address is its own shape,
            // distinct from a malformed one: three ports have now accepted some form of
            // bracketed text as an IPv6 host without parsing it.
            ("gid://[not-an-ip]/Person/1", None),
            ("gid://[abc]/Person/1", None),
            ("gid://[1]/Person/1", None),
            ("gid://[::1::2]/Person/1", None),
            ("gid://[]/Person/1", None),
            ("gid://[::ffff:192.0.2.1]/Person/1", Some(1)),
            ("gid://[fe80::1%25eth0]:8080/Person/1", Some(1)),
            ("gid://[::1]:/Person/1", Some(1)),
            // A ZONE does not excuse the literal from being an address. These are the rows
            // that separate "validated unconditionally" from "skipped when a zone is
            // present"; `[::1%25eth0]` above cannot, because it parses either way. Every
            // answer here is Go's, measured, not predicted — the accepting direction was a
            // real defect on this line and the probe that was supposed to have caught it
            // could only ever agree with the premise it came from.
            ("gid://[not-an-address%25zone]/Person/77", None),
            ("gid://[abc%25z]/Person/77", None),
            ("gid://[1%25z]/Person/77", None),
            ("gid://[bad%25x]/Person/77", None),
            ("gid://[%25]/Person/77", None),
            // An EMPTY zone is refused: `ParseAddr` will not take one.
            ("gid://[::1%25]/Person/77", None),
            // IPv4 in brackets stays refused with a zone on it, and the IPv4-mapped IPv6
            // form stays accepted — the check is on what the address IS, not how it reads.
            ("gid://[192.0.2.1%25eth0]/Person/77", None),
            ("gid://[::ffff:192.0.2.1%25eth0]/Person/77", Some(77)),
            // The zone's escape rule is its OWN, and admits escapes a host refuses: `%41`
            // and `%20` both have a first hex digit below 8. Verifying that rule at the
            // host position says nothing about this one.
            ("gid://[::1%25%41]/Person/77", Some(77)),
            ("gid://[::1%25a%20b]/Person/77", Some(77)),
            // But not every escape: a zone still refuses one whose byte it could not have
            // written directly.
            ("gid://[::1%25a%2fb]/Person/77", None),
            ("gid://[::1%25%01]/Person/77", None),
            // IPvFuture is a real RFC 3986 production and `netip.ParseAddr` does not
            // implement it, so Go refuses it. A port that checks whether the brackets LOOK
            // like a literal instead of parsing one accepts these.
            ("gid://[v7.x]/Person/77", None),
            ("gid://[V7.x]/Person/77", None),
            ("gid://[vz.x]/Person/77", None),
            ("gid://[v1.fe80::a+en1]/Person/77", None),
            // An empty host stays empty after userinfo is stripped — but `u.Host` carries
            // the port, so an empty NAME with a port is not an empty host.
            ("gid://@/Person/1", None),
            ("gid://:@/Person/1", None),
            ("gid://%20@/Person/1", None),
            ("gid:///Person/1", None),
            ("gid://@:/Person/1", Some(1)),
            ("gid://@:8080/Person/1", Some(1)),
            // The host is what `u.Host` holds, which INCLUDES the port — so an empty name
            // with a port is not an empty host.
            ("gid://:8080/Person/77", Some(77)),
            ("gid://:/Person/77", Some(77)),
            // An escape is refused only when its first hex digit is below 8 and the triple
            // is not `%25`. A blanket refusal loses every one of these.
            ("gid://%25/Person/77", Some(77)),
            ("gid://bc%80/Person/77", Some(77)),
            ("gid://%C3%A9/Person/77", Some(77)),
            ("gid://%bc3/Person/77", Some(77)),
            ("gid://b%63%33/Person/77", None),
            ("gid://bc3%/Person/77", None),
            // The FRAGMENT is discarded, but a URL parser unescapes it first, so a
            // malformed escape there fails the whole parse — and nothing downstream can
            // notice, which is why this went unnoticed in the accepting direction. The
            // QUERY, cut in the same breath, is never unescaped and so is never checked.
            // Three positions, three rules.
            ("gid://bc3/Person/77#%zz", None),
            ("gid://bc3/Person/77#%", None),
            ("gid://bc3/Person/77#%2", None),
            ("gid://bc3/Person/77#x%0gy", None),
            ("gid://bc3/Person/77?ok#%zz", None),
            ("gid://bc3/Person/77#a#%zz", None),
            ("gid://bc3/Person/77#%41", Some(77)),
            ("gid://bc3/Person/77#%ff", Some(77)),
            ("gid://bc3/Person/77#ok", Some(77)),
            ("gid://bc3/Person/77#", Some(77)),
            ("gid://bc3/Person/77?%zz", Some(77)),
            ("gid://bc3/Person/77?x=%zz", Some(77)),
            ("gid://bc3/Person/77?%", Some(77)),
            // The host's character set is an allowlist, and these four are on opposite
            // sides of it from what a denylist would guess.
            ("gid://b<3/Person/77", Some(77)),
            ("gid://b;3/Person/77", Some(77)),
            ("gid://b^3/Person/77", None),
            ("gid://b|3/Person/77", None),
            ("gid://bc3/Person/77", Some(77)),
            // The path is percent-decoded before it is read.
            ("gid://bc3/Person/%37%37", Some(77)),
            ("gid://bc3/Pe%72son/77", Some(77)),
            // The scheme is ASCII case-insensitive.
            ("GID://bc3/Person/77", Some(77)),
            ("Gid://bc3/Person/77", Some(77)),
            // A space is illegal in a host, so this is not a URL at all. Splitting on the
            // first `/` would read `" "` as the host and name a person Go names none for.
            ("gid:// /Person/77", None),
            ("gid://b c3/Person/77", None),
            ("gid:///Person/77", None),
            // A truncated or non-hex escape is not a path.
            ("gid://bc3/Person/%3", None),
            ("gid://bc3/Person/%zz", None),
            ("gid://bc3/Person/77/extra", None),
            ("gid://bc3/Person/12x", None),
        ];
        for (gid, expected) in go {
            let sgid = json_sgid(&format!(r#"{{"gid":"{gid}","purpose":"attachable"}}"#));
            assert_eq!(person_id_from_sgid(&sgid), *expected, "{gid}");
        }
    }

    /// Pinned against `html.UnescapeString`, measured. The rule is to agree with Go on every
    /// character an sgid can hold, and to leave the rest alone exactly as Go leaves an
    /// unknown entity alone.
    #[test]
    fn entities_decode_where_go_decodes_them() {
        let go = [
            ("&equals;", "="),
            ("&sol;", "/"),
            ("&plus;", "+"),
            ("&lowbar;", "_"),
            ("&UnderBar;", "_"),
            ("&amp;", "&"),
            ("&quot;", "\""),
            ("&#66;", "B"),
            // Go accepts a numeric reference without its semicolon.
            ("&#66", "B"),
            ("&#x42;", "B"),
            ("&#x42", "B"),
            // The boundary counts CHARACTERS from `&`, not digits, so a semicolon is worth
            // as much as a digit and the hex `x` is worth one on its own. A port that
            // counts digits gets the first of these wrong in the vanishing direction.
            ("&#9;", "\t"),
            ("&#9", "&#9"),
            ("&#9B", "&#9B"),
            ("&#1;", "\u{1}"),
            ("&#12", "\u{c}"),
            ("&#x9", "\t"),
            ("&#xB", "\u{b}"),
            // 0x80..=0x9F are Windows-1252 BYTES, not code points: `&#133;` is a horizontal
            // ellipsis, not U+0085 NEL. Reading them as code points produces a character
            // the surrounding trim then deletes, resolving a person Go refuses.
            ("&#133;", "\u{2026}"),
            ("&#x85;", "\u{2026}"),
            ("&#128;", "\u{20ac}"),
            ("&#159;", "\u{178}"),
            // Five of the thirty-two map to themselves; the rest do not.
            ("&#129;", "\u{81}"),
            ("&#141;", "\u{8d}"),
            ("&#157;", "\u{9d}"),
            // U+00A0 is just past the remapped range and is itself.
            ("&#160;", "\u{a0}"),
            // Zero, the surrogates and anything past the last scalar are the replacement
            // character — not a refusal, and not a NUL.
            ("&#0;", "\u{fffd}"),
            ("&#0", "&#0"),
            ("&#55296;", "\u{fffd}"),
            ("&#1114112;", "\u{fffd}"),
            ("&#x;", "\u{fffd}"),
            ("&#;", "&#;"),
            ("&#1114111;", "\u{10ffff}"),
            // A value past 2^32 WRAPS, because Go accumulates into an int32 and the range
            // checks then run on the wrapped value. Clamping to the replacement character
            // only ever loses a mention Go finds.
            ("&#x100000041;", "A"),
            ("&#4294967361;", "A"),
            ("&#x100000020;", " "),
            ("&#x10000000000000041;", "A"),
            // An unknown entity is left verbatim, in Go too.
            ("&unknownthing;", "&unknownthing;"),
            ("&", "&"),
            ("&;", "&;"),
            ("a&b", "a&b"),
        ];
        for (input, expected) in go {
            assert_eq!(unescape(input), expected, "{input:?}");
        }
    }

    #[test]
    fn an_sgid_whose_base64_characters_are_escaped_still_names_its_person() {
        // `=` padding written as `&equals;` decodes in Go, so it must here: a narrower
        // reader would not error, it would drop the mention and — on the write side —
        // add a duplicate tag for a person the content already mentions.
        let escaped = ANNIE_SGID.replace('=', "&equals;");
        let markup = format!(r#"<div><bc-attachment sgid="{escaped}"></bc-attachment></div>"#);
        assert_eq!(mentioned_person_ids(&markup), vec![1_049_715_915_i64]);

        // And the write side sees the same string the read side does, so the person the
        // content already mentions is not mentioned twice. This is the half a narrower
        // decoder got wrong in the other direction: not unescaping made the escaped
        // spelling look like a DIFFERENT sgid, and a second tag went out for someone who
        // was already there.
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        let expanded = with_mentions(&markup, std::slice::from_ref(&annie)).unwrap();
        assert_eq!(expanded.matches("bc-attachment sgid=").count(), 1);
    }

    /// Quadratic entity scanning is reachable from any attribute in content an author
    /// writes, so it is worth a test — but asserted by SCALING, not by a wall clock. An
    /// absolute bound is a different number in a debug build, on a shared CI runner, and on
    /// a laptop; the first version of this passed locally in release and failed CI in debug,
    /// which told me about my own test rather than about the code.
    #[test]
    fn a_hostile_run_of_ampersands_stays_linear() {
        let elapsed = |n: usize| {
            let hostile = format!("<p sgid=\"{}\">x</p>", "&".repeat(n) + ";");
            let started = std::time::Instant::now();
            assert!(mentioned_person_ids(&hostile).is_empty());
            started.elapsed()
        };
        let small = elapsed(20_000);
        let large = elapsed(80_000);
        // Four times the input: linear lands near 4x, quadratic near 16x. The floor keeps a
        // near-zero `small` from making the ratio meaningless on a fast machine.
        let budget = small.max(std::time::Duration::from_micros(200)) * 10;
        assert!(
            large < budget,
            "entity scanning is not linear: {small:?} for 20k, {large:?} for 80k"
        );
    }

    #[test]
    fn a_query_or_fragment_never_smuggles_a_path_past_the_authority() {
        // `gid://bc3?x/Person/77` has no path: a URL parser puts all of it in the query, so
        // it names nobody. Reading `bc3?x` as a host would make it a mention of person 77.
        for gid in [
            "gid://bc3?x/Person/77",
            "gid://bc3#x/Person/77",
            "gid://?/Person/77",
            "gid://bc3?/Person/77",
        ] {
            let sgid = json_sgid(&format!(r#"{{"gid":"{gid}","purpose":"attachable"}}"#));
            assert_eq!(person_id_from_sgid(&sgid), None, "{gid}");
        }
        // The query BC3 itself mints, after a real path, still decodes.
        let real = json_sgid(r#"{"gid":"gid://bc3/Person/77?expires_in","purpose":"attachable"}"#);
        assert_eq!(person_id_from_sgid(&real), Some(77));
    }

    #[test]
    fn a_gid_that_is_not_a_person_is_not_a_mention() {
        let blob =
            json_sgid(r#"{"gid":"gid://bc3/ActiveStorage::Blob/12","purpose":"attachable"}"#);
        let nested =
            json_sgid(r#"{"gid":"gid://bc3/Document/gid://bc3/Person/77","purpose":"attachable"}"#);
        let not_numeric = json_sgid(r#"{"gid":"gid://bc3/Person/12x","purpose":"attachable"}"#);
        let extra_segment =
            json_sgid(r#"{"gid":"gid://bc3/Person/77/extra","purpose":"attachable"}"#);
        assert_eq!(person_id_from_sgid(&blob), None);
        assert_eq!(person_id_from_sgid(&nested), None);
        assert_eq!(person_id_from_sgid(&not_numeric), None);
        assert_eq!(person_id_from_sgid(&extra_segment), None);
    }

    #[test]
    fn junk_and_oversized_sgids_decode_to_nothing() {
        assert_eq!(person_id_from_sgid(""), None);
        assert_eq!(person_id_from_sgid("--"), None);
        assert_eq!(person_id_from_sgid("not base64 at all!"), None);
        assert_eq!(
            person_id_from_sgid(&"A".repeat(MAX_SGID_ENCODED_BYTES + 1)),
            None
        );
    }

    #[test]
    fn mentions_are_read_in_document_order_without_repeats() {
        let other = json_sgid(r#"{"gid":"gid://bc3/Person/88","purpose":"attachable"}"#);
        let blob = json_sgid(r#"{"gid":"gid://bc3/ActiveStorage::Blob/9","purpose":"attachable"}"#);
        let content = format!(
            r#"<div><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment> hi <bc-attachment sgid="{other}"></bc-attachment><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment><bc-attachment sgid="{blob}"></bc-attachment></div>"#
        );
        assert_eq!(mentioned_person_ids(&content), vec![1_049_715_915_i64, 88]);
    }

    #[test]
    fn a_quoted_mention_counts_and_a_commented_one_does_not() {
        let quoted = format!(
            r#"<blockquote><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment></blockquote>"#
        );
        let commented =
            format!(r#"<div><!-- <bc-attachment sgid="{ANNIE_SGID}"></bc-attachment> --></div>"#);
        let in_an_attribute =
            format!(r#"<div title="<bc-attachment sgid=&quot;{ANNIE_SGID}&quot;>">x</div>"#);
        assert_eq!(mentioned_person_ids(&quoted), vec![1_049_715_915_i64]);
        assert!(mentioned_person_ids(&commented).is_empty());
        assert!(mentioned_person_ids(&in_an_attribute).is_empty());
    }

    #[test]
    fn attribute_tokenizing_survives_quotes_case_and_order() {
        let cases = [
            format!("<BC-ATTACHMENT SGID='{ANNIE_SGID}'></bc-attachment>"),
            format!(r#"<bc-attachment caption="a > b" sgid="{ANNIE_SGID}"></bc-attachment>"#),
            format!(r#"<bc-attachment data-x="sgid=nope" sgid={ANNIE_SGID}></bc-attachment>"#),
            format!(r#"<bc-attachment sgid="{ANNIE_SGID}" sgid="later"></bc-attachment>"#),
        ];
        for case in &cases {
            assert_eq!(
                mentioned_person_ids(case),
                vec![1_049_715_915_i64],
                "{case}"
            );
        }
    }

    #[test]
    fn a_tag_name_that_merely_starts_with_the_name_is_not_one() {
        let near = format!(r#"<bc-attachment_x sgid="{ANNIE_SGID}"></bc-attachment_x>"#);
        assert!(mentioned_person_ids(&near).is_empty());
    }

    /// Names are compared by Unicode simple case folding, as the reference compares them,
    /// and exactly two non-ASCII runes reach an ASCII letter: U+017F with `s` and U+212A
    /// with `k`. So `ſgid` IS the sgid attribute. Measured against the Go function, which
    /// names Annie for the first of these; an ASCII-only fold named nobody.
    ///
    /// The scheme is the counter-case in the same file: a URL parser's scheme grammar
    /// admits only ASCII, so `gıd` is not `gid` however it folds. Getting one rule right is
    /// not getting the other right, and the two live a few hundred lines apart.
    #[test]
    fn a_name_folds_the_way_the_reference_folds_it_and_the_scheme_does_not() {
        let long_s = format!("<bc-attachment \u{17f}gid=\"{ANNIE_SGID}\"></bc-attachment>");
        assert_eq!(mentioned_person_ids(&long_s), vec![1_049_715_915_i64]);
        let upper = format!("<BC-ATTACHMENT SGID=\"{ANNIE_SGID}\"></BC-ATTACHMENT>");
        assert_eq!(mentioned_person_ids(&upper), vec![1_049_715_915_i64]);
        // Not every non-ASCII letter folds onto an ASCII one; only those two do.
        for miss in [
            "\u{131}gid",
            "\u{130}gid",
            "\u{17f}\u{17f}gid",
            "égid",
            "zgid",
        ] {
            let markup = format!("<bc-attachment {miss}=\"{ANNIE_SGID}\"></bc-attachment>");
            assert!(
                mentioned_person_ids(&markup).is_empty(),
                "{miss} is not the sgid attribute"
            );
        }
        // A byte that is not valid UTF-8 folds to nothing: Go decodes it to U+FFFD.
        let broken = [
            b"<bc-attachment \xc5gid=\"".as_slice(),
            ANNIE_SGID.as_bytes(),
            b"\"></bc-attachment>",
        ]
        .concat();
        assert!(mentioned_person_ids(&String::from_utf8_lossy(&broken)).is_empty());
        // And the scheme, which is ASCII or it is not a scheme at all.
        for scheme in ["g\u{131}d", "G\u{130}D", "\u{131}d"] {
            let payload = json_sgid(&format!(
                r#"{{"gid":"{scheme}://bc3/Person/77","purpose":"attachable"}}"#
            ));
            assert_eq!(person_id_from_sgid(&payload), None, "{scheme} is not gid");
        }
        for scheme in ["gid", "GID", "Gid"] {
            let payload = json_sgid(&format!(
                r#"{{"gid":"{scheme}://bc3/Person/77","purpose":"attachable"}}"#
            ));
            assert_eq!(person_id_from_sgid(&payload), Some(77), "{scheme} is gid");
        }
    }

    #[test]
    fn an_unterminated_tag_ends_the_walk() {
        let unterminated = format!(
            r#"<bc-attachment sgid="{ANNIE_SGID}"></bc-attachment><bc-attachment sgid="oops"#
        );
        assert_eq!(mentioned_person_ids(&unterminated), vec![1_049_715_915_i64]);
    }

    #[test]
    fn markup_needs_an_sgid_that_names_the_person() {
        assert!(mention_markup(&person(1, None)).is_err());
        assert!(mention_markup(&person(1, Some(""))).is_err());
        assert!(mention_markup(&person(1, Some("a\"b"))).is_err());
        // The sgid is Annie's, the person is not.
        assert!(mention_markup(&person(1, Some(ANNIE_SGID))).is_err());
        assert_eq!(
            mention_markup(&person(1_049_715_915, Some(ANNIE_SGID))).unwrap(),
            format!(r#"<bc-attachment sgid="{ANNIE_SGID}"></bc-attachment>"#)
        );
    }

    #[test]
    fn mentions_go_inside_the_first_block_or_in_front_of_bare_text() {
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        let tag = mention_markup(&annie).unwrap();
        assert_eq!(
            with_mentions("<div>On it.</div>", std::slice::from_ref(&annie)).unwrap(),
            format!("<div>{tag} On it.</div>")
        );
        assert_eq!(
            with_mentions(r#"<p class="x">On it.</p>"#, std::slice::from_ref(&annie)).unwrap(),
            format!(r#"<p class="x">{tag} On it.</p>"#)
        );
        assert_eq!(
            with_mentions("On it.", std::slice::from_ref(&annie)).unwrap(),
            format!("{tag} On it.")
        );
        assert_eq!(
            with_mentions("<span>On it.</span>", std::slice::from_ref(&annie)).unwrap(),
            format!("{tag} <span>On it.</span>")
        );
    }

    #[test]
    fn the_same_sgid_is_never_written_twice() {
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        let content = format!(r#"<div><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment></div>"#);
        assert_eq!(
            with_mentions(&content, std::slice::from_ref(&annie)).unwrap(),
            content
        );
        let twice = with_mentions("Hi", &[annie.clone(), annie]).unwrap();
        assert_eq!(twice.matches("bc-attachment sgid=").count(), 1);
    }

    /// The SUPPRESSION direction: content crafted so that its unescape equals the sgid the
    /// people read returned, making the write-side dedupe fire and skip a mention that
    /// should be written. It is the `&fjlig;` machinery inverted — that one made the dedupe
    /// MISS and emit a duplicate; this one makes it HIT and emit nothing.
    ///
    /// The Python and Ruby ports were vulnerable through `&#1;`: an HTML5 decoder drops a
    /// numeric reference naming a C0 control to the empty string, so `<real sgid>&#1;`
    /// unescaped to exactly the real sgid and the mention was silently suppressed. Go emits
    /// the control character, the strings differ, and the mention is written.
    ///
    /// This decoder cannot do it, and the reason is structural rather than lucky: no
    /// expansion in the table is empty, a numeric reference always yields a character or is
    /// left literal, and a literal `&…` is not empty either. So the unescape of
    /// `<sgid><anything non-empty>` is never `<sgid>`. Measured across 2,452 suffixes —
    /// every entity name in both of Go's tables, every C0 control and DEL in six spellings,
    /// surrogates, the BOM and the zero-width characters — against Go's own `WithMentions`:
    /// both write the mention in all 2,452, and the empty-suffix control below is what
    /// proves the assertion is not vacuous.
    /// The reference accumulates a numeric reference into an int32 and says in its own
    /// comment that it does not check for overflow, so every later test runs on the WRAPPED
    /// value and `&#x100000041;` is `A`. A guard the reference deliberately omits is a
    /// divergence when you add it: clamping or a checked accumulate can never name someone
    /// Go does not, but it suppresses a mention Go finds, which is the direction that reads
    /// as safe and so does not get noticed.
    ///
    /// `entities_decode_where_go_decodes_them` pins the decoded character. This pins the
    /// consequence, which is the thing that matters: an sgid one of whose characters is
    /// written as an overflowing reference still names its person. 261 such cases were
    /// measured against the Go function, 240 of them resolving to a person, so the branch
    /// is known to have been reached and not merely included.
    #[test]
    fn an_overflowing_numeric_reference_still_completes_the_sgid_it_sits_in() {
        let head = &ANNIE_SGID[..1];
        let tail = &ANNIE_SGID[2..];
        // ANNIE_SGID[1] is 'A' (0x41); each of these wraps an int32 back onto it.
        for reference in [
            "&#x100000041;",
            "&#4294967361;",
            "&#x000000000100000041;",
            "&#x2100000041;",
        ] {
            let markup =
                format!(r#"<bc-attachment sgid="{head}{reference}{tail}"></bc-attachment>"#);
            assert_eq!(
                mentioned_person_ids(&markup),
                vec![1_049_715_915_i64],
                "{reference} must decode to A and complete the sgid"
            );
        }
    }

    /// A digit is an ASCII digit at every position that reads one. Another port shipped a
    /// fullwidth digit reaching a numeric parse and suppressing a real mention, so this
    /// sweeps the positions rather than the one predicate: the person id, both spellings of
    /// a numeric character reference, the port, and a bracketed literal.
    ///
    /// Recorded honestly, because the measurement is weaker than it looks: 59 cases against
    /// the Go function diverge nowhere, but making either predicate Unicode-aware ALSO
    /// diverges nowhere. What actually refuses these is the ASCII-only `parse` and
    /// `to_digit` downstream of both. The predicates are belt and braces, and this test
    /// holds the composite behaviour so that removing either layer is still caught.
    #[test]
    fn a_digit_is_an_ascii_digit_everywhere_a_digit_is_read() {
        // One representative from each of several scripts, plus the shapes that are
        // "numeric" to Unicode without being digits at all.
        for digits in [
            "\u{ff10}\u{ff17}",
            "\u{660}\u{667}",
            "\u{966}\u{96d}",
            "\u{1d7ce}\u{1d7d5}",
        ] {
            let seven = digits.chars().nth(1).unwrap();
            let payload = json_sgid(&format!(
                r#"{{"gid":"gid://bc3/Person/{seven}{seven}","purpose":"attachable"}}"#
            ));
            assert_eq!(
                person_id_from_sgid(&payload),
                None,
                "{seven} is not a digit"
            );
            // Mixed with a real ASCII digit, which is how this hides.
            let mixed = json_sgid(&format!(
                r#"{{"gid":"gid://bc3/Person/7{seven}","purpose":"attachable"}}"#
            ));
            assert_eq!(person_id_from_sgid(&mixed), None, "7{seven} is not 7");
            // A numeric character reference spelt with one.
            let head = &ANNIE_SGID[..1];
            let tail = &ANNIE_SGID[2..];
            for reference in [format!("&#{seven}{seven};"), format!("&#x{seven}{seven};")] {
                let markup =
                    format!(r#"<bc-attachment sgid="{head}{reference}{tail}"></bc-attachment>"#);
                assert!(
                    mentioned_person_ids(&markup).is_empty(),
                    "{reference} is not a numeric reference"
                );
            }
            // And the port, which a URL parser reads as digits too.
            let ported = json_sgid(&format!(
                r#"{{"gid":"gid://bc3:{seven}0/Person/77","purpose":"attachable"}}"#
            ));
            assert_eq!(person_id_from_sgid(&ported), None, "{seven}0 is not a port");
        }
        // The ASCII spellings of all four still resolve, so the assertions above are about
        // the digits and not about the shapes carrying them.
        assert_eq!(
            person_id_from_sgid(&json_sgid(
                r#"{"gid":"gid://bc3:80/Person/77","purpose":"attachable"}"#
            )),
            Some(77)
        );
        let ascii = format!(
            r#"<bc-attachment sgid="{}&#x41;{}"></bc-attachment>"#,
            &ANNIE_SGID[..1],
            &ANNIE_SGID[2..]
        );
        assert_eq!(mentioned_person_ids(&ascii), vec![1_049_715_915_i64]);
    }

    #[test]
    fn a_crafted_suffix_cannot_suppress_a_real_mention() {
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        let content_with = |suffix: &str| {
            format!(r#"<div><bc-attachment sgid="{ANNIE_SGID}{suffix}"></bc-attachment>hi</div>"#)
        };
        let tags = |suffix: &str| {
            with_mentions(&content_with(suffix), std::slice::from_ref(&annie))
                .unwrap()
                .matches("bc-attachment sgid=")
                .count()
        };

        // The control: with nothing appended the content really does already carry the
        // mention, so the dedupe SHOULD fire. Without this row the rest proves nothing.
        assert_eq!(tags(""), 1, "the dedupe fires when it should");

        for suffix in [
            "&#1;",
            "&#1",
            "&#x1;",
            "&#01;",
            "&#001;",
            "&#0;",
            "&#127;",
            "&#xd800;",
            "&#xfeff;",
            "&#8203;",
            "&#x110000;",
            "&#;",
            "&#",
            "&#x",
            "&",
            "&&",
            "&;",
            "&NewLine;",
            "&Tab;",
            "&nbsp;",
            "&nbsp",
            "&ThickSpace;",
            "&fjlig;",
            "&amp;",
            "&equals;",
            "&unknownthing;",
        ] {
            assert_eq!(
                tags(suffix),
                2,
                "a real mention was suppressed by {suffix:?}"
            );
        }
    }

    #[test]
    fn dedupe_is_on_the_sgid_string_not_on_the_person_id_it_decodes_to() {
        // A stale tag naming the right person: the sgid differs, so the real mention is
        // still written. Deduplicating by person id would suppress it.
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        let stale = json_sgid(r#"{"gid":"gid://bc3/Person/1049715915","purpose":"attachable"}"#);
        let content = format!(r#"<div><bc-attachment sgid="{stale}"></bc-attachment></div>"#);
        let expanded = with_mentions(&content, std::slice::from_ref(&annie)).unwrap();
        assert!(expanded.contains(ANNIE_SGID));
        assert_eq!(expanded.matches("bc-attachment sgid=").count(), 2);
    }

    #[test]
    fn one_person_without_an_sgid_fails_the_whole_expansion() {
        let annie = person(1_049_715_915, Some(ANNIE_SGID));
        assert!(with_mentions("Hi", &[annie, person(2, None)]).is_err());
    }

    #[test]
    fn a_marshal_payload_that_is_not_one_value_is_refused() {
        // Two values back to back: valid prefix, trailing bytes.
        assert!(unmarshal_ruby(b"0T").is_err());
        assert!(unmarshal_ruby(b"c").is_err()); // an unsupported type byte
        assert!(unmarshal_ruby(b"\"\x7f").is_err()); // a count past the data
    }

    /// Pinned against what `base64.RawStdEncoding.DecodeString` actually answers, measured
    /// rather than remembered. A divergence in EITHER direction is a bug: stricter than Go
    /// makes a real mention vanish, looser names a person Go does not.
    #[test]
    fn base64_matches_go_raw_std_encoding_in_both_directions() {
        let go = [
            ("QQ", Some(vec![b'A'])),
            // Non-zero trailing bits: Go's non-strict decoder ignores them.
            ("QR", Some(vec![b'A'])),
            ("QUJD", Some(vec![b'A', b'B', b'C'])),
            ("QUJ", Some(vec![b'A', b'B'])),
            ("QUK", Some(vec![b'A', b'B'])),
            // Line breaks are stepped over wherever they appear.
            ("QU\nJD", Some(vec![b'A', b'B', b'C'])),
            ("QU\r\nJD", Some(vec![b'A', b'B', b'C'])),
            // A six-bit leftover group is corrupt, and a space is not whitespace Go skips.
            ("QUJDR", None),
            ("A", None),
            ("QU JD", None),
            ("~~~~", None),
        ];
        for (input, expected) in go {
            assert_eq!(base64_decode(input), expected, "{input:?}");
        }
    }

    /// Both halves measured against the Go helper, not assumed.
    /// A payload far longer than the accumulator is wide, decoded byte-exactly. Go reports
    /// 109 bytes for this one, and a real `<bc-attachment>` carries exactly this shape, so
    /// an accumulator that lost or mangled a bit past 32 would show here.
    #[test]
    fn a_long_payload_decodes_byte_for_byte_past_the_accumulator_width() {
        let payload = ANNIE_SGID.split("--").next().unwrap();
        let decoded = base64_decode(
            &payload
                .trim_end_matches('=')
                .replace('-', "+")
                .replace('_', "/"),
        )
        .expect("the payload decodes");
        assert_eq!(decoded.len(), 109, "as Go's RawStdEncoding reports");
        assert_eq!(&decoded[..2], &[0x04, 0x08], "a Marshal 4.8 header");
        assert!(
            String::from_utf8_lossy(&decoded).contains("gid://bc3/Person/1049715915"),
            "the gid survives the whole decode"
        );
        // And the full read path over an attacker-sized value terminates without panicking:
        // the bound refuses it, rather than the loop running away.
        let oversized = "A".repeat(MAX_SGID_ENCODED_BYTES + 1);
        assert_eq!(person_id_from_sgid(&oversized), None);
    }

    #[test]
    fn a_line_wrapped_sgid_decodes_exactly_where_go_decodes_it() {
        // A break INSIDE the payload: Go steps over it and names the person, so this must
        // too — refusing would raise no error anywhere, it would drop a real mention.
        let split = format!("{}\n{}", &ANNIE_SGID[..20], &ANNIE_SGID[20..]);
        assert_eq!(person_id_from_sgid(&split), Some(1_049_715_915));

        // A break immediately BEFORE the digest separator names nobody, in Go too, and it
        // is the ONLY placement that does. The whole value is trimmed first, so a break at
        // either end is gone before anything looks at it; this one is interior, where the
        // trim cannot reach, and it leaves the payload ending `==` — which the padding
        // strip, being a right-trim, then cannot reach either. A raw (unpadded) decoder
        // rejects the surviving `=`. Two rules interacting, not either alone.
        let before_digest = ANNIE_SGID.replacen("--", "\n--", 1);
        assert_eq!(person_id_from_sgid(&before_digest), None);
    }

    /// A DIFFERENTIAL table: every row is what `basecamp.PersonIDFromSGID` — the real Go
    /// function, not a transcription of it — answered for the same input, captured by
    /// piping both implementations the same list and diffing.
    ///
    /// The axis here is not what the decoder skips, it is WHERE THE TRIM SITS relative to
    /// the padding strip and the separator split. Go's order is: trim the whole value,
    /// split on the LAST `--`, right-trim the padding, then decode with an encoding that
    /// has no padding character. Four other ports got that order wrong in one direction or
    /// the other, and neither the shared fixture nor a leniency table can see it: a break
    /// between the padding and the separator survives the whole-value trim and then blocks
    /// the padding trim, so the `=` reaches a decoder that refuses it — while a break at
    /// either END of the value is gone before anything looks at it.
    ///
    /// `\\n`, `\\r`, `\\s` and `\\t` in the inputs are escapes this test expands, so the
    /// rows read the same here as they were fed to Go.
    #[test]
    fn the_sgid_read_agrees_with_the_go_function_row_for_row() {
        const ANNIE: i64 = 1_049_715_915;
        let cases: &[(&str, Option<i64>)] = &[
            // baseline signed
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // break between pad and sep
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==\\n--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                None,
            ),
            // CR between pad and sep
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==\\r--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                None,
            ),
            // break amid the padding
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA=\\n=--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                None,
            ),
            // break before the padding
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA\\n==--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // break inside the payload
            (
                "BAh7CEkiCGdpZAY6BkVU\\nSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // break after the separator
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--\\n919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // trailing break whole value
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102\\n",
                Some(ANNIE),
            ),
            // leading break whole value
            (
                "\\nBAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // spaces both ends
            (
                "\\s\\sBAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102\\s\\s",
                Some(ANNIE),
            ),
            // tab trailing
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102\\t",
                Some(ANNIE),
            ),
            // unsigned bare
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==",
                Some(ANNIE),
            ),
            // unsigned trailing break
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==\\n",
                Some(ANNIE),
            ),
            // unsigned leading break
            (
                "\\nBAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==",
                Some(ANNIE),
            ),
            // unsigned break amid pad
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA=\\n=",
                None,
            ),
            // unsigned break before pad
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA\\n==",
                Some(ANNIE),
            ),
            // unsigned break in payload
            (
                "BAh7CEkiCGdpZAY6BkVU\\nSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==",
                Some(ANNIE),
            ),
            // padding stripped entirely
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA--919d2c8b11ff403eefcab9db42dd26846d0c3102",
                Some(ANNIE),
            ),
            // double separator
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102--extra",
                None,
            ),
            // separator only
            (
                "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--",
                Some(ANNIE),
            ),
        ];
        for (escaped, expected) in cases {
            let sgid = expand(escaped);
            assert_eq!(person_id_from_sgid(&sgid), *expected, "{escaped}");
        }
    }

    /// Expands the escapes the differential table is written with.
    fn expand(value: &str) -> String {
        let bytes = value.as_bytes();
        let mut out = String::new();
        let mut index = 0;
        while index < bytes.len() {
            if bytes[index] == b'\\' && index + 1 < bytes.len() {
                let replacement = match bytes[index + 1] {
                    b'n' => Some('\n'),
                    b'r' => Some('\r'),
                    b's' => Some(' '),
                    b't' => Some('\t'),
                    _ => None,
                };
                if let Some(character) = replacement {
                    out.push(character);
                    index += 2;
                    continue;
                }
            }
            out.push(char::from(bytes[index]));
            index += 1;
        }
        out
    }

    /// A DIGEST over a wide, deterministic input space, pinned to a constant.
    ///
    /// The differential corpora behind the tables above were all measured on one toolchain,
    /// and this crate is built on two: `Rust msrv (1.88)` and `Rust stable`. A difference
    /// between them in `char::to_digit`, in `str::trim`'s Unicode tables, or in anything
    /// else these paths lean on would be invisible to a sweep taken on either one alone.
    ///
    /// This runs in both jobs and fails in whichever disagrees. It is deliberately a digest
    /// rather than a table: the point is not to state what each of several thousand inputs
    /// answers, which the tables above already do for the cases that carry meaning, but to
    /// notice if ANY of them starts answering differently. The tables say what is right;
    /// this says nothing changed.
    #[test]
    fn the_decode_is_identical_on_every_toolchain_that_builds_this() {
        // FNV-1a over the answers, so the assertion is one number and needs no dependency.
        let mut digest: u64 = 0xcbf2_9ce4_8422_2325;
        let mut feed = |text: &str| {
            for byte in text.as_bytes() {
                digest ^= u64::from(*byte);
                digest = digest.wrapping_mul(0x0000_0100_0000_01b3);
            }
            digest ^= 0xff;
            digest = digest.wrapping_mul(0x0000_0100_0000_01b3);
        };

        // Numeric references across the interesting ranges, in six spellings each.
        for code in (0u32..0x0300).chain([0xd7ff, 0xd800, 0xdfff, 0xe000, 0x0010_ffff, 0x0011_0000])
        {
            for spelling in [
                format!("&#{code};"),
                format!("&#{code}"),
                format!("&#x{code:x};"),
                format!("&#x{code:x}"),
                format!("&#0{code};"),
                format!("&#X{code:X};"),
            ] {
                feed(&unescape(&spelling));
            }
        }
        // Every entity in the table, with and without its semicolon, and as a prefix.
        for (name, _) in VERDICT_RELEVANT_ENTITIES {
            feed(&unescape(&format!("&{name}")));
            feed(&unescape(&format!("&{}", name.trim_end_matches(';'))));
            feed(&unescape(&format!("&{name}tail")));
        }
        // The trim and the whole read, over every separator and near-separator.
        for code in (0u32..0x3001).step_by(7) {
            let Some(character) = char::from_u32(code) else {
                continue;
            };
            let sgid = format!("{character}{ANNIE_SGID}{character}");
            feed(match person_id_from_sgid(&sgid) {
                Some(_) => "mention",
                None => "none",
            });
        }

        assert_eq!(
            digest, DECODE_DIGEST,
            "the decode differs on this toolchain; the pinned tables above say which rows are \
             right, this only says something moved"
        );
    }

    /// A non-ASCII space at one end and a stray character in the digest half — the shape
    /// that broke a sibling port, where the trim chose its alphabet from whether the WHOLE
    /// value was well-formed and so fell back to ASCII when anything anywhere was not.
    ///
    /// Rust cannot reach the byte-level version of that: `person_id_from_sgid` takes a
    /// `&str`, so an ill-formed byte cannot be handed to it, and the one
    /// `from_utf8_lossy` on this path slices a `&str` at ASCII quote boundaries, where the
    /// result is already well-formed and nothing is ever substituted. `str::trim` decodes a
    /// character from each end independently, as Go's `TrimSpace` does, and the two agree
    /// on every separator here.
    ///
    /// Measured against the Go function over 224 cases — twenty-four space characters in
    /// seven positions, and seven non-space characters each paired with five different
    /// leading spaces — with every case base64-framed, because a corpus written one case
    /// per line silently splits the ones containing U+000A, U+000D, U+2028 or U+2029 and
    /// reports a clean run it never actually made.
    #[test]
    fn a_space_at_one_end_and_a_stray_in_the_digest_agree_with_go() {
        const ANNIE: i64 = 1_049_715_915;
        let payload = ANNIE_SGID.split("--").next().unwrap().to_string();
        let digest = ANNIE_SGID.split("--").nth(1).unwrap().to_string();
        let signed = |lead: &str, stray: &str| {
            format!("{lead}{payload}--{}{stray}{}", &digest[..10], &digest[10..])
        };

        // A space at an end is trimmed and the payload decodes, whatever sits in the
        // digest — the separator throws that half away.
        for lead in ["\u{20}", "\u{a0}", "\u{2009}", "\u{3000}", "\u{85}"] {
            for stray in ["\u{fffd}", "\u{200b}", "\u{feff}", "\u{b7}"] {
                assert_eq!(
                    person_id_from_sgid(&signed(lead, stray)),
                    Some(ANNIE),
                    "lead {lead:?} stray {stray:?}"
                );
            }
        }

        // Every separator Go trims, this trims: the ASCII set, NEL, the line separators,
        // and the Unicode spaces.
        for space in [
            "\u{20}", "\u{9}", "\u{a}", "\u{d}", "\u{b}", "\u{c}", "\u{85}", "\u{a0}", "\u{1680}",
            "\u{2000}", "\u{2009}", "\u{200a}", "\u{202f}", "\u{205f}", "\u{3000}", "\u{2028}",
            "\u{2029}",
        ] {
            assert_eq!(
                person_id_from_sgid(&format!("{space}{ANNIE_SGID}")),
                Some(ANNIE),
                "leading {space:?}"
            );
            assert_eq!(
                person_id_from_sgid(&format!("{space}{ANNIE_SGID}{space}")),
                Some(ANNIE),
                "both ends {space:?}"
            );
        }

        // A non-space at an end is NOT trimmed, so it stays in the payload and names
        // nobody — including the replacement character, which a lossy decode would have
        // put there and which must not be mistaken for a separator.
        for stray in ["\u{fffd}", "\u{200b}", "\u{2060}", "\u{feff}", "\u{b7}"] {
            assert_eq!(
                person_id_from_sgid(&format!("{stray}{ANNIE_SGID}")),
                None,
                "leading {stray:?}"
            );
        }

        // And a space between the padding and the separator still names nobody: the trim
        // cannot reach it, and it blocks the padding strip.
        assert_eq!(person_id_from_sgid(&signed("", "")), Some(ANNIE));
        assert_eq!(
            person_id_from_sgid(&format!("{payload}\u{a0}--{digest}")),
            None
        );
    }

    /// The other half of the same rule, and the half a defensive port gets wrong: Go trims
    /// the WHOLE sgid before parsing it, so whitespace at either end is not a reason to
    /// refuse. Refusing here would raise no error — it would drop a real mention.
    #[test]
    fn whitespace_around_the_whole_sgid_is_trimmed_before_anything_reads_it() {
        let unsigned = ANNIE_SGID.split("--").next().unwrap().to_string();
        for (name, sgid) in [
            ("trailing LF", format!("{ANNIE_SGID}\n")),
            ("trailing CRLF", format!("{ANNIE_SGID}\r\n")),
            ("trailing spaces", format!("{ANNIE_SGID}  ")),
            ("leading LF", format!("\n{ANNIE_SGID}")),
            ("surrounded", format!("  {ANNIE_SGID}\n")),
            // The unsigned fallback takes the same path.
            ("unsigned with a trailing LF", format!("{unsigned}\n")),
            ("unsigned bare", unsigned.clone()),
        ] {
            assert_eq!(person_id_from_sgid(&sgid), Some(1_049_715_915), "{name}");
        }
    }

    /// A KNOWN, DELIBERATE divergence, pinned so it is a decision rather than a surprise.
    ///
    /// Go's `encoding/json` replaces a malformed UTF-8 byte or an unpaired surrogate escape
    /// with U+FFFD and carries on, so an envelope whose damage sits in a member the decode
    /// ignores still yields its person. `serde_json` refuses the document instead, and the
    /// sgid names nobody.
    ///
    /// It runs in ONE direction only, measured rather than assumed: Go answers and this
    /// declines, never the reverse. So this port never names a person Go does not; it
    /// declines a few Go would name. On a helper a connector admits events by, that is the
    /// direction to fail in, and it is why this is pinned rather than chased: matching
    /// byte-for-byte needs a lossy re-encode of the raw bytes AND a surrogate-replacing
    /// JSON reader — two substitutions of someone else's parser — to accept input BC3 does
    /// not mint.
    ///
    /// What decides it is NOT which member the damage lands in. An earlier version of this
    /// comment said the divergence appears only in members the decode ignores, and that is
    /// wrong: a reviewer produced `{"gid":"gid://bc3/Person/77?x=<lone surrogate>", …}`,
    /// where the damage is inside the gid — a member that is very much read — and Go still
    /// answers 77, because U+FFFD lands in the query, which nothing parses. The rule is
    /// that the divergence appears wherever the substitution would not have changed the
    /// parse anyway. Both shapes are pinned below.
    #[test]
    fn a_malformed_byte_in_a_json_envelope_declines_where_go_substitutes() {
        let gid = br#""gid":"gid://bc3/Person/77","purpose":"attachable""#;
        // Damage in a member neither side reads: Go replaces and answers, this declines.
        for (name, damaged) in [
            ("a raw continuation byte", &b"\x80"[..]),
            ("a raw 0xff, which is in no UTF-8 sequence", &b"\xff"[..]),
            ("an overlong NUL", &b"\xc0\x80"[..]),
            ("a surrogate encoded as UTF-8", &b"\xed\xa0\x80"[..]),
            ("a truncated three-byte sequence", &b"\xe2\x82"[..]),
            ("a code point above the maximum", &b"\xf4\x90\x80\x80"[..]),
            ("an unpaired high surrogate escape", &br"\ud800"[..]),
            ("an unpaired low surrogate escape", &br"\udfff"[..]),
        ] {
            for (position, envelope) in [
                (
                    "an ignored value",
                    [b"{".as_slice(), gid, br#","x":""#, damaged, br#""}"#].concat(),
                ),
                (
                    "an ignored key",
                    [b"{".as_slice(), gid, b",\"", damaged, br#"":"y"}"#].concat(),
                ),
            ] {
                assert_eq!(
                    person_id_from_sgid(&json_sgid_bytes(&envelope)),
                    None,
                    "{name} in {position}: Go answers 77 here; this port declines, and the \
                     divergence is deliberate"
                );
            }
        }
        // The same envelopes undamaged DO answer, so the assertion above is about the
        // malformed byte and not about the envelope shape.
        assert_eq!(
            person_id_from_sgid(&json_sgid_bytes(
                &[b"{".as_slice(), gid, br#","x":"ok"}"#].concat()
            )),
            Some(77)
        );
        // And valid non-ASCII is not the divergence: an emoji, as UTF-8 or as a correct
        // surrogate PAIR, is carried by both.
        for valid in [&b"\xf0\x9f\x98\x80"[..], &br"\ud83d\ude00"[..]] {
            assert_eq!(
                person_id_from_sgid(&json_sgid_bytes(
                    &[b"{".as_slice(), gid, br#","x":""#, valid, br#""}"#].concat()
                )),
                Some(77)
            );
        }
        // Damage INSIDE the gid, where the substitution would not have changed the parse:
        // Go answers 77 for all of these, and this declines. Same direction, a member the
        // decode reads.
        for (position, damaged_gid) in [
            ("the query", &br"gid://bc3/Person/77?x=\ud800"[..]),
            ("the query, raw", &b"gid://bc3/Person/77?x=\xff"[..]),
            ("the fragment", &br"gid://bc3/Person/77#\ud800"[..]),
            ("the fragment, raw", &b"gid://bc3/Person/77#\xff"[..]),
            ("the host", &b"gid://bc\xff3/Person/77"[..]),
        ] {
            let envelope = [
                br#"{"gid":""#.as_slice(),
                damaged_gid,
                br#"","purpose":"attachable"}"#,
            ]
            .concat();
            assert_eq!(
                person_id_from_sgid(&json_sgid_bytes(&envelope)),
                None,
                "damage in {position}: Go answers 77, this declines"
            );
        }
        // The contrast that makes this a JSON-reader difference and not a decoding one:
        // the SAME damage carried by a Marshal 4.8 envelope — the layout BC3's own sgids
        // actually use — diverges nowhere, because Marshal carries bytes and nothing
        // substitutes anything. Both of these answer 77 in Go and here.
        for marshal in [
            // {"gid" => "gid://bc3/Person/77", "purpose" => "attachable", "x" => "\xff"}
            "BAh7CEkiCGdpZAY6BkVUSSIYZ2lkOi8vYmMzL1BlcnNvbi83NwY6BkVUSSIMcHVycG9zZQY6BkVUSSIPYXR0YWNoYWJsZQY6BkVUSSIGeAY6BkVUSSIG_wY6BkVU",
            // {"gid" => "gid://bc3/Person/77?x=\xff", "purpose" => "attachable"}
            "BAh7B0kiCGdpZAY6BkVUSSIcZ2lkOi8vYmMzL1BlcnNvbi83Nz94Pf8GOgZFVEkiDHB1cnBvc2UGOgZFVEkiD2F0dGFjaGFibGUGOgZFVA",
        ] {
            assert_eq!(person_id_from_sgid(marshal), Some(77));
        }
        // Where the substitution DOES change the parse, both answer nobody. These use a
        // U+FFFD that is already THERE, in valid JSON, rather than a malformed byte: a
        // malformed byte makes serde_json refuse the document before the id is ever looked
        // at, so it cannot show that U+FFFD is not a digit. Spelt this way the rows reach
        // the check they name, and Go reaches it too.
        for (position, envelope) in [
            (
                "the id",
                r#"{"gid":"gid://bc3/Person/77\ufffd","purpose":"attachable"}"#,
            ),
            (
                "the model name",
                r#"{"gid":"gid://bc3/Pers\ufffdon/77","purpose":"attachable"}"#,
            ),
            (
                "the purpose",
                r#"{"gid":"gid://bc3/Person/77","purpose":"attach\ufffdable"}"#,
            ),
        ] {
            assert_eq!(
                person_id_from_sgid(&json_sgid(envelope)),
                None,
                "U+FFFD in {position}"
            );
        }
        // But U+FFFD in the HOST is a legal host character, so both sides still answer —
        // which is why "damage in a member that is read" was the wrong way to say this.
        assert_eq!(
            person_id_from_sgid(&json_sgid(
                r#"{"gid":"gid://bc\ufffd3/Person/77","purpose":"attachable"}"#
            )),
            Some(77)
        );
    }

    /// An unsigned JSON envelope, in the base64url spelling Rails emits.
    fn json_sgid(json: &str) -> String {
        json_sgid_bytes(json.as_bytes())
    }

    /// The same, over bytes, so an envelope carrying a malformed one can be built at all.
    fn json_sgid_bytes(bytes: &[u8]) -> String {
        const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
        let mut out = String::new();
        for chunk in bytes.chunks(3) {
            let mut buffer = [0u8; 3];
            buffer[..chunk.len()].copy_from_slice(chunk);
            let packed =
                (u32::from(buffer[0]) << 16) | (u32::from(buffer[1]) << 8) | u32::from(buffer[2]);
            let encoded = 4 - (3 - chunk.len());
            for index in 0..encoded {
                let slot = usize::try_from((packed >> (18 - 6 * index)) & 0x3f).unwrap_or_default();
                out.push(char::from(ALPHABET[slot]));
            }
        }
        out
    }
}
