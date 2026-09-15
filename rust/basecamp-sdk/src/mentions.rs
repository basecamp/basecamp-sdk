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
        if rest[..name_end].eq_ignore_ascii_case(b"bc-attachment")
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
        if sgid.is_none() && name.eq_ignore_ascii_case(b"sgid") {
            sgid = Some(unescape(&String::from_utf8_lossy(value)));
        }
    }
    None
}

/// The index just past the opening `<p …>` or `<div …>` tag a rich text starts with, or
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
        if bytes.len() < after || !bytes[start..after].eq_ignore_ascii_case(name) {
            continue;
        }
        if after < bytes.len() && !is_tag_name_end(bytes[after]) {
            continue;
        }
        return parse_attributes(bytes, after).map(|(_, end)| end);
    }
    None
}

/// The longest entity this reads: `&` plus a name or numeric body plus `;`. Go's scanner is
/// bounded the same way, by the longest name in its table. The bound is what keeps this
/// LINEAR — without it, `"&".repeat(n) + ";"` makes every failed parse rescan to the same
/// far semicolon and the walk is quadratic in an attribute an author controls.
const MAX_ENTITY_LENGTH: usize = 34;

/// Named references that produce a character an `attachable_sgid` can actually contain, plus
/// the five a serializer emits. Measured against Go's `html.UnescapeString`, not recalled.
///
/// The full HTML5 table is ~2200 entries and reproducing it would be its own liability. It
/// is not needed, and the reason is worth stating because it is what makes this equivalent
/// rather than merely smaller: an sgid is base64url plus `=` padding and the `--` separator,
/// so its alphabet is `A-Za-z0-9+/=_-`. A reference that yields a character INSIDE that
/// alphabet can change which person an sgid names, and every one of those is here. A
/// reference that yields a character outside it cannot: Go decodes it and gets an sgid with
/// a character no base64 payload may hold, which fails to decode and names nobody — and
/// this leaves it verbatim, which fails to decode and names nobody. Same answer, both ways.
///
/// Letters and digits have no named references at all. `&hyphen;` and `&dash;` are U+2010,
/// not ASCII `-`, so no named reference produces a hyphen — only a numeric one does.
const NAMED_ENTITIES: &[(&str, char)] = &[
    ("amp", '&'),
    ("apos", '\''),
    ("equals", '='),
    ("gt", '>'),
    ("lowbar", '_'),
    ("lt", '<'),
    ("plus", '+'),
    ("quot", '"'),
    ("sol", '/'),
    ("UnderBar", '_'),
];

/// Decodes the character references in an attribute value the way a browser — and Go's
/// `html.UnescapeString` — would.
///
/// Semicolon-less numeric references are decoded, because Go decodes them (`&#66` is `B`).
/// Anything this does not recognize is passed through verbatim, which is also what Go does
/// with an unknown entity.
fn unescape(value: &str) -> String {
    if !value.contains('&') {
        return value.to_string();
    }
    let mut out = String::with_capacity(value.len());
    let bytes = value.as_bytes();
    let mut pos = 0usize;
    while pos < bytes.len() {
        if bytes[pos] != b'&' {
            let next = bytes[pos..]
                .iter()
                .position(|byte| *byte == b'&')
                .map_or(bytes.len(), |offset| pos + offset);
            out.push_str(&value[pos..next]);
            pos = next;
            continue;
        }
        if let Some((decoded, length)) = entity_at(value, pos) {
            out.push(decoded);
            pos += length;
        } else {
            out.push('&');
            pos += 1;
        }
    }
    out
}

/// The character reference beginning at `start` (which is an `&`), and how many bytes it
/// occupies. The scan is bounded by [`MAX_ENTITY_LENGTH`], so a failure costs a constant.
fn entity_at(value: &str, start: usize) -> Option<(char, usize)> {
    let end = value.len().min(start + MAX_ENTITY_LENGTH);
    let window = value.get(start..end)?;
    let body = window.strip_prefix('&')?;
    if let Some(digits) = body.strip_prefix('#') {
        let (radix, digits) = match digits.strip_prefix(['x', 'X']) {
            Some(hex) => (16, hex),
            None => (10, digits),
        };
        // Greedy, then optionally a ";". Go accepts both spellings.
        let taken = digits
            .find(|character: char| !character.is_digit(radix))
            .unwrap_or(digits.len());
        if taken == 0 {
            return None;
        }
        let code = u32::from_str_radix(&digits[..taken], radix).ok()?;
        let decoded = char::from_u32(code)?;
        let prefix = window.len() - body.len() + (body.len() - digits.len());
        let semicolon = usize::from(digits[taken..].starts_with(';'));
        return Some((decoded, prefix + taken + semicolon));
    }
    let name_end = body
        .find(|character: char| !character.is_ascii_alphanumeric())
        .unwrap_or(body.len());
    let name = &body[..name_end];
    let (_, decoded) = NAMED_ENTITIES
        .iter()
        .find(|(candidate, _)| *candidate == name)?;
    // A named reference needs its semicolon. Go's semicolon-less forms are a legacy table
    // whose every member yields a character outside an sgid's alphabet, so admitting them
    // could not change which person an sgid names.
    body[name_end..]
        .starts_with(';')
        .then(|| (*decoded, 1 + name_end + 1))
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

/// A `GlobalID`'s model and raw id, read the way `net/url` reads it — which is the parser
/// the reference implementation uses, so its answers are the contract.
///
/// Three of its behaviours have to be reproduced deliberately, because a hand-rolled split
/// gets each of them wrong in a way that matters:
///
/// - The scheme is ASCII case-insensitive (`GID://` parses).
/// - The path is percent-DECODED before it is read, so `gid://bc3/Person/%37%37` names
///   person 77.
/// - The authority is validated. `gid:// /Person/77` is not a URL at all — a space is
///   illegal in a host — and Go refuses it. Splitting on the first `/` instead would read
///   `" "` as the host and name person 77 that Go names nobody for, which is the dangerous
///   direction for a helper a connector admits events by.
///
/// The query and the fragment are cut FIRST, before the authority, because that is where
/// they begin: in `gid://bc3?x/Person/77` the `?` ends the host and everything after it is
/// the query, so the URL has no path at all.
fn parse_global_id(gid: &str) -> Option<(String, String)> {
    let after_scheme = strip_scheme(gid)?;
    let authority_and_path = after_scheme.split(['?', '#']).next().unwrap_or_default();
    let (host, path) = authority_and_path.split_once('/')?;
    if host.is_empty() || !is_valid_host(host) {
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

/// What follows `gid://`, matching the scheme case-insensitively as a URL parser does.
fn strip_scheme(gid: &str) -> Option<&str> {
    let (scheme, rest) = gid.split_once("://")?;
    scheme.eq_ignore_ascii_case("gid").then_some(rest)
}

/// Whether a host is one a URL parser would accept: no space, no control character, and
/// none of the delimiters that would have ended the authority.
fn is_valid_host(host: &str) -> bool {
    !host.bytes().any(|byte| {
        byte <= b' ' || byte == 0x7f || matches!(byte, b'/' | b'?' | b'#' | b'@' | b'\\')
    })
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
    #[test]
    fn the_gid_parse_answers_what_net_url_answers() {
        let go: &[(&str, Option<i64>)] = &[
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

    #[test]
    fn a_hostile_run_of_ampersands_costs_only_its_own_length() {
        // Quadratic here would be reachable from any attribute in content an author writes.
        let hostile = format!("<p sgid=\"{}\">x</p>", "&".repeat(200_000) + ";");
        let started = std::time::Instant::now();
        assert!(mentioned_person_ids(&hostile).is_empty());
        assert!(
            started.elapsed() < std::time::Duration::from_secs(2),
            "entity scanning is not linear: {:?}",
            started.elapsed()
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

    /// An unsigned JSON envelope, in the base64url spelling Rails emits.
    fn json_sgid(json: &str) -> String {
        const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
        let bytes = json.as_bytes();
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
