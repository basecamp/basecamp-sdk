import Foundation

/// Mention helpers over Basecamp rich text.
///
/// A mention in Basecamp rich text is a `<bc-attachment>` whose `sgid`
/// attribute is the mentioned person's `attachable_sgid`
/// (`doc/api/sections/rich_text.md`, "Inserting a mention"). BC3 renders the
/// same tag back with `content-type="application/vnd.basecamp.mention"` and an
/// avatar figure inside it, but the sgid is the only part of the markup that
/// names the person on both the write and the read side, so both halves here
/// work from it:
///
/// - ``personIds(in:)`` reads the person ids a rich text names, by decoding the
///   sgid of every `<bc-attachment>` and keeping the ones that point at a
///   Person.
/// - ``markup(for:)`` writes the tag for a person, from their
///   `attachable_sgid`.
///
/// An `attachable_sgid` is a Rails SignedGlobalID: a base64 payload, then
/// `--`, then an HMAC only BC3 can verify. The payload is an envelope carrying
/// the global id — `gid://bc3/Person/1049715915` — as a string, and that string
/// is what these helpers read. They do not (and cannot) verify the signature;
/// what they decode is the same person id BC3 renders into the mention's
/// avatar, read off content the API already served, and a caller that needs the
/// id verified reads the person back through `people.get(personId:)`.
///
/// That sets a trust boundary between the two sides. **Reading** —
/// ``personIds(in:)``, ``personId(fromAttachableSgid:)`` — describes what a text
/// says it mentions, and unsigned is fine for description: the ids are
/// reported, not acted on as proof. **Writing** — ``adding(_:to:)``,
/// `CommentsService.expandMentions(_:mentioning:)` — never treats an unsigned id
/// as proof that a valid mention already exists: a forged or stale sgid in
/// caller-supplied content naming the right id would otherwise make the writer
/// skip the authoritative people read and post a tag Basecamp will not honour,
/// so the person is silently not mentioned. `expandMentions` therefore resolves
/// every requested person through `people.get(personId:)` and deduplicates only
/// against the exact `attachable_sgid` string that read returned. The pure
/// helpers beneath it — ``adding(_:to:)``, ``markup(for:)`` — take `Person`
/// values the caller built and can only check that an sgid is well-formed and
/// names the person it is given, never that it is authentic: hand them people
/// the API returned, not people assembled from content. Do not reuse the
/// read-side helpers to decide whether a write can be skipped.
///
/// The markup is read as BC3 serves it: a sanitized tree of the tags
/// `doc/api/sections/rich_text.md` allows, which has no raw-text elements. The
/// tag walk skips comments and quoted attribute values but does not model
/// `<script>` or `<style>` content, which BC3 strips on write; a caller reading
/// mentions out of content it authored itself should not put a `bc-attachment`
/// inside such an element and expect it ignored.
///
/// The envelope is decoded structurally, never searched as bytes, so a Person
/// gid that merely appears inside some other value — a Document gid built from
/// one, a purpose string that looks like one — is not a mention, and the
/// envelope's purpose must be `attachable`, the one BC3 accepts in rich text.
/// Three envelopes are read: Rails' current Marshal layout
/// `{"_rails" => {"data" => gid, "pur" => purpose}}`, the older Marshal layout
/// `{"gid" => gid, "purpose" => …, "expires_at" => …}`, and the JSON spelling of
/// either, which Rails' JSON message serializer emits.
public enum Mentions {
    /// Returns the ids of the people a rich text mentions: the Person named by
    /// the sgid of each `<bc-attachment>`, in document order, with repeats
    /// removed. Attachments that are not mentions — files, images, embeds — are
    /// skipped, as is any sgid that does not decode to a Person.
    ///
    /// This is the read side: a description of what the text says, from sgids
    /// whose signatures cannot be checked here. Report it; do not treat an id in
    /// it as proof that a valid mention exists (see the trust boundary above).
    ///
    /// Every `<bc-attachment>` in the text counts, including one inside a
    /// `<blockquote>`: BC3 notifies quoted mentions too, so the read matches
    /// what the server does with the write.
    public static func personIds(in richText: String) -> [Int] {
        var ids: [Int] = []
        var seen = Set<Int>()
        for sgid in attachmentSgids(in: richText) {
            guard let id = personId(fromAttachableSgid: sgid) else { continue }
            guard seen.insert(id).inserted else { continue }
            ids.append(id)
        }
        return ids
    }

    /// Decodes the Person id an `attachable_sgid` names. Nil when the sgid does
    /// not decode, or names something other than a Person (a file attachment's
    /// sgid names an `ActiveStorage::Blob`).
    ///
    /// This reads the id out of the sgid's payload; it does not verify the
    /// sgid's signature, which only BC3 can. It is a read-side helper: never use
    /// its answer to decide that a write may skip the authoritative people read
    /// (see the trust boundary in the type comment).
    public static func personId(fromAttachableSgid sgid: String) -> Int? {
        guard let gid = globalId(fromSgid: sgid) else { return nil }
        return personId(fromGlobalId: gid)
    }

    /// Renders the `<bc-attachment>` that mentions a person, from their
    /// `attachable_sgid` — the write-side form in
    /// `doc/api/sections/rich_text.md`, which BC3 expands into the avatar figure
    /// on read.
    ///
    /// Throws a `.usage` error when the person carries no `attachable_sgid`,
    /// which is the case for a `Person` projection that came from somewhere
    /// other than a people read (a webhook payload, say), and when the sgid does
    /// not name the person it is given. That is all it can check: it cannot
    /// verify the signature, so the `Person` must come from the API — a
    /// `people.get(personId:)`, a recording's creator or assignees — not be
    /// assembled from an sgid found in content.
    public static func markup(for person: Person) throws -> String {
        let id = person.id.value
        guard let sgid = person.attachableSgid, !sgid.isEmpty else {
            throw BasecampError.usage(
                message: "person \(id) has no attachable_sgid to mention",
                hint: "read the person through people.get(personId:) to obtain one")
        }
        // Scalars, not Characters. A `Character` is a grapheme cluster, so a
        // quote followed by a combining mark is one Character that compares
        // unequal to `"` and walks straight past a Character-based test — and
        // the value is written into the attribute verbatim, with nothing else
        // validating the half of the sgid after the last `--`. Go's
        // `strings.ContainsAny` tests runes; this is the same test.
        if sgid.unicodeScalars.contains(where: { markupUnsafeScalars.contains($0) }) {
            throw BasecampError.usage(
                message: "person \(id) has a malformed attachable_sgid", hint: nil)
        }
        // The tag mentions whoever the sgid names. Refuse to write one that
        // names someone else — or a file — under this person's id.
        guard let named = personId(fromAttachableSgid: sgid), named == id else {
            throw BasecampError.usage(
                message: "person \(id)'s attachable_sgid does not name that person",
                hint: "read the person through people.get(personId:) to obtain their own")
        }
        return "<bc-attachment sgid=\"\(sgid)\"></bc-attachment>"
    }

    /// Returns content that mentions each of the given people, for posting as a
    /// comment or a Campfire line.
    ///
    /// A person whose exact `attachable_sgid` the content already carries is
    /// left alone, so passing the same person twice — or a person the author
    /// already mentioned with that sgid — never duplicates the mention; the rest
    /// are added at the start of the content, inside its first `<p>` or `<div>`
    /// when it opens with one, so they render on the first line rather than as a
    /// block of their own.
    ///
    /// This is the write side, and it deduplicates on the sgid string alone,
    /// never on the person id an existing tag's sgid decodes to: that id is
    /// unsigned, and a forged or stale tag naming the right person must not
    /// stand in for the real mention (see the trust boundary in the type
    /// comment). Every person needs their own `attachable_sgid`, and it must be
    /// one the API returned: this helper can check that an sgid is well-formed
    /// and names the person, not that it is authentic (see ``markup(for:)``).
    /// The account-bound `CommentsService.expandMentions(_:mentioning:)`
    /// resolves ids to people first and is the entry point that carries that
    /// guarantee.
    public static func adding(_ people: [Person], to content: String) throws -> String {
        // Keyed on UTF-8 bytes, not on `String`. Swift string equality is
        // canonical equivalence, so two sgids that differ byte for byte — a
        // combining sequence against its precomposed form — compare EQUAL, and
        // the dedupe would skip a mention Go adds. "Exact attachable_sgid" is a
        // byte rule; this is what makes it one.
        var present = Set(attachmentSgids(in: content).map { Array($0.utf8) })
        var tags: [String] = []
        for person in people {
            // Rendered before the dedupe check, not after: a person the content
            // already mentions is still refused when their own sgid is unusable,
            // so a caller cannot be told "nothing to do" about a broken Person.
            let tag = try markup(for: person)
            let sgid = person.attachableSgid ?? ""
            guard present.insert(Array(sgid.utf8)).inserted else { continue }
            tags.append(tag)
        }
        if tags.isEmpty { return content }

        let prefix = tags.joined(separator: " ") + " "
        let bytes = Array(content.utf8)
        guard let end = leadingBlockEnd(bytes) else { return prefix + content }
        // `end` is the index just past a `>`, so it is always a UTF-8 boundary.
        return String(decoding: bytes[0..<end], as: UTF8.self)
            + prefix
            + String(decoding: bytes[end...], as: UTF8.self)
    }
}

// MARK: - Markup scanning

/// Characters an sgid may not contain if it is to be written into an attribute
/// value verbatim. Checked rather than escaped: a real `attachable_sgid` is
/// base64url plus `--` plus hex, so one carrying markup is corrupt input, not
/// input to sanitize.
private let markupUnsafeScalars: Set<Unicode.Scalar> = ["\"", "'", "<", ">", "&"]

extension Mentions {
    /// Returns the `sgid` attribute of every `<bc-attachment>` in the text, in
    /// document order.
    ///
    /// It walks the markup as a stream of tags rather than pattern-matching for
    /// one tag name, so a `<bc-attachment>` inside an HTML comment or inside
    /// another element's quoted attribute is not an element; and it tokenizes
    /// each tag's attributes rather than pattern-matching them, so a `>` inside
    /// a quoted value does not end the tag, an `sgid=` inside another
    /// attribute's value is not an attribute, either quote style works,
    /// attribute order and case are free, the first `sgid` attribute wins as in
    /// HTML, and entity escapes in the value are decoded as a browser would.
    ///
    /// The walk is over UTF-8 bytes. Every character it branches on is ASCII,
    /// and a multi-byte scalar's continuation bytes are all ≥ 0x80, so no byte
    /// of one can be mistaken for a delimiter — and every slice it turns back
    /// into a `String` is cut at an ASCII boundary.
    static func attachmentSgids(in text: String) -> [String] {
        let bytes = Array(text.utf8)
        var sgids: [String] = []
        var pos = 0

        while pos < bytes.count {
            guard let open = bytes[pos...].firstIndex(of: asciiLessThan) else { break }
            pos = open + 1
            if hasPrefix(bytes, at: pos, "!--") {
                guard let stop = firstRange(of: "-->", in: bytes, from: pos) else {
                    return sgids  // an unterminated comment swallows the rest
                }
                pos = stop + 3
                continue
            }
            if pos < bytes.count,
                bytes[pos] == asciiBang || bytes[pos] == asciiQuestion || bytes[pos] == asciiSlash
            {
                guard let stop = bytes[pos...].firstIndex(of: asciiGreaterThan) else { return sgids }
                pos = stop + 1
                continue
            }

            var nameEnd = pos
            while nameEnd < bytes.count, isTagNameByte(bytes[nameEnd]) { nameEnd += 1 }
            if nameEnd == pos { continue }  // a bare "<" in text

            let (attributes, end, closed) = parseAttributes(bytes, from: nameEnd)
            guard closed else { return sgids }  // an unterminated tag: nothing after it is markup
            let name = String(decoding: bytes[pos..<nameEnd], as: UTF8.self)
            if name.caseInsensitiveCompare("bc-attachment") == .orderedSame,
                !attributes.sgid.isEmpty
            {
                sgids.append(attributes.sgid)
            }
            pos = end
        }
        return sgids
    }

    /// What ``parseAttributes(_:from:)`` reads off one opening tag.
    struct TagAttributes {
        /// The decoded value of the first `sgid` attribute; empty when there was
        /// none, or when the one there was is itself empty. The two do not need
        /// telling apart: an empty sgid names nobody either way.
        var sgid: String = ""
    }

    /// Walks the attributes of an opening tag from `start` (just after the tag
    /// name) to its closing `>`, returning what it found, the index after the
    /// `>`, and whether the tag was closed at all. The first `sgid` attribute
    /// wins, present-but-empty included, as HTML resolves a repeated attribute.
    static func parseAttributes(_ bytes: [UInt8], from start: Int) -> (
        attributes: TagAttributes, end: Int, closed: Bool
    ) {
        var attributes = TagAttributes()
        var sawSgid = false
        var pos = start

        while pos < bytes.count {
            while pos < bytes.count, isSpaceByte(bytes[pos]) || bytes[pos] == asciiSlash {
                pos += 1
            }
            if pos >= bytes.count { return (attributes, pos, false) }
            if bytes[pos] == asciiGreaterThan { return (attributes, pos + 1, true) }

            let nameStart = pos
            while pos < bytes.count, !isSpaceByte(bytes[pos]), bytes[pos] != asciiEquals,
                bytes[pos] != asciiGreaterThan, bytes[pos] != asciiSlash
            {
                pos += 1
            }
            let name = String(decoding: bytes[nameStart..<pos], as: UTF8.self)

            while pos < bytes.count, isSpaceByte(bytes[pos]) { pos += 1 }
            var value = ""
            if pos < bytes.count, bytes[pos] == asciiEquals {
                pos += 1
                while pos < bytes.count, isSpaceByte(bytes[pos]) { pos += 1 }
                if pos < bytes.count, bytes[pos] == asciiDoubleQuote || bytes[pos] == asciiSingleQuote {
                    let quote = bytes[pos]
                    pos += 1
                    guard let closing = bytes[pos...].firstIndex(of: quote) else {
                        return (attributes, bytes.count, false)
                    }
                    value = String(decoding: bytes[pos..<closing], as: UTF8.self)
                    pos = closing + 1
                } else {
                    let valueStart = pos
                    while pos < bytes.count, !isSpaceByte(bytes[pos]), bytes[pos] != asciiGreaterThan {
                        pos += 1
                    }
                    value = String(decoding: bytes[valueStart..<pos], as: UTF8.self)
                }
            }

            if name.isEmpty {
                // A stray "=" or quote where a name should be: step over it.
                pos += 1
                continue
            }
            if !sawSgid, name.caseInsensitiveCompare("sgid") == .orderedSame {
                sawSgid = true
                attributes.sgid = unescapeEntities(value)
            }
        }
        return (attributes, pos, false)
    }

    /// Returns the index just past the opening `<p …>` or `<div …>` tag a rich
    /// text starts with, or nil when it starts with anything else, so mentions
    /// can be placed inside the first block rather than as a bare prefix in
    /// front of it. The tag's attributes are scanned quote-aware: a `>` inside
    /// an attribute value does not end it.
    static func leadingBlockEnd(_ bytes: [UInt8]) -> Int? {
        var i = 0
        while i < bytes.count, isSpaceByte(bytes[i]) { i += 1 }
        for name in ["<p", "<div"] {
            guard hasPrefix(bytes, at: i, name) else { continue }
            let after = i + name.utf8.count
            if after < bytes.count, !isTagNameEndByte(bytes[after]) { continue }
            let (_, end, closed) = parseAttributes(bytes, from: after)
            return closed ? end : nil
        }
        return nil
    }
}

// MARK: - SGID decoding

extension Mentions {
    /// The SignedGlobalID purpose BC3 mints attachable sgids with
    /// (`doc/api/sections/rich_text.md`: `attachable_sgid`). Pinned by the
    /// purpose cases in `MentionsTests`, so a rename upstream breaks a test here
    /// rather than silently turning every mention invisible.
    static let attachablePurpose = "attachable"

    /// Bounds the decoded sgid payload. A Person sgid's payload is under 200
    /// bytes; the cap keeps a hostile one from costing more than its own size to
    /// reject.
    static let maxSgidPayloadBytes = 4096
    /// The same bound on the base64 form (4/3 of the payload, plus padding),
    /// checked before anything is allocated.
    static let maxSgidEncodedBytes = maxSgidPayloadBytes / 3 * 4 + 4

    /// Returns the global id string an sgid's envelope carries.
    ///
    /// A signed sgid is `<payload>--<digest>`, and `-` is a base64url character,
    /// so the payload itself may contain `--`. The separator is therefore the
    /// LAST one, as Rails' own verifier reads it; the whole value is tried as a
    /// bare payload when that fails, which is what an unsigned envelope — one
    /// that happens to contain `--` included — needs.
    static func globalId(fromSgid sgid: String) -> String? {
        let value = sgid.trimmingCharacters(in: goWhitespace)
        if let separator = value.range(of: "--", options: .backwards),
            separator.lowerBound != value.startIndex,
            let gid = envelopeGlobalId(String(value[..<separator.lowerBound]))
        {
            return gid
        }
        return envelopeGlobalId(value)
    }

    /// Decodes one base64 payload and returns the gid its envelope carries.
    static func envelopeGlobalId(_ payload: String) -> String? {
        // The bound is applied to the encoded form first, so an oversized sgid
        // costs nothing to refuse — no normalization, no decode buffer.
        guard !payload.isEmpty, payload.utf8.count <= maxSgidEncodedBytes else { return nil }
        // Rails' MessageVerifier emits either alphabet; base64url is current.
        // Both decode through the standard alphabet once the two symbols are
        // mapped, and stripping the padding lets a truncated-but-valid payload
        // through.
        // Decoded the way Go decodes it, in Go's order, because the order is
        // load-bearing: it maps the alphabet, then trims trailing `=`, then
        // hands the rest to `RawStdEncoding`, which ignores CR and LF and
        // refuses everything else — `=` included, wherever it sits.
        //
        // Doing it in any other order diverges. Stripping the newlines FIRST
        // turns `<payload>=\n` into `<payload>=`, trims the `=` as trailing, and
        // accepts an envelope Go rejects. And Foundation's decoder is not a
        // stand-in for `RawStdEncoding` either: it accepts an interior `=`
        // (`QQ=Q` decodes to two bytes there and is illegal in Go), so that is
        // refused explicitly rather than left to it.
        //
        // Exactly CR and LF are ignored, and nothing else: Go refuses a space or
        // a tab inside base64, so stripping whitespace generally would make this
        // the LENIENT side. Trailing bits need no handling — Go ignores a
        // non-zero final group and so does Foundation.
        var normalized = payload.replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        while normalized.hasSuffix("=") { normalized.removeLast() }
        // Byte-level: a CRLF is ONE Swift `Character`, so a Character-level
        // filter for "\r" or "\n" walks straight past the pair a line-wrapping
        // serializer emits.
        let stripped = String(
            decoding: normalized.utf8.filter { $0 != 0x0D && $0 != 0x0A }, as: UTF8.self)
        guard !stripped.utf8.contains(UInt8(ascii: "=")) else { return nil }

        guard let raw = decodeUnpaddedBase64(stripped), !raw.isEmpty,
            raw.count <= maxSgidPayloadBytes
        else { return nil }

        let bytes = [UInt8](raw)
        let envelope: [String: Any]
        if bytes.count >= 2, bytes[0] == 0x04, bytes[1] == 0x08 {
            guard let decoded = try? RubyMarshal.decode(Array(bytes.dropFirst(2))),
                let map = decoded as? [String: Any]
            else { return nil }
            envelope = map
        } else if bytes[0] == asciiOpenBrace {
            guard let decoded = try? JSONSerialization.jsonObject(with: raw),
                let map = decoded as? [String: Any]
            else { return nil }
            envelope = map
        } else {
            return nil
        }

        // A SignedGlobalID is bound to a purpose, and only an "attachable" one
        // may be placed in rich text: BC3 refuses any other, so a Person sgid
        // minted for bookmarking or reading is not a mention however valid its
        // gid. Both layouts carry the purpose; an envelope without one is not a
        // Rails envelope.
        //
        // Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
        if let rails = envelope["_rails"] as? [String: Any] {
            guard rails["pur"] as? String == attachablePurpose else { return nil }
            guard let gid = rails["data"] as? String, !gid.isEmpty else { return nil }
            return gid
        }
        // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
        guard envelope["purpose"] as? String == attachablePurpose else { return nil }
        guard let gid = envelope["gid"] as? String, !gid.isEmpty else { return nil }
        return gid
    }

    /// Parses `gid://<app>/Person/<id>` and returns the id.
    ///
    /// Hand-parsed rather than handed to `URL`: a GlobalID path is exactly
    /// `/<Model>/<id>` — no more, no less — and the whole point is to refuse
    /// anything else rather than to be lenient about it.
    ///
    /// It disagrees with Go's `url.Parse` in exactly one mechanism NOW, and the
    /// history belongs here because that sentence has been false four times, and
    /// each time the sweep that "proved" it was blind to the counterexample by
    /// construction.
    ///
    ///   1. It named one shape of a mechanism that has three.
    ///   2. It claimed the disagreement ran in the stricter direction only —
    ///      true of the 747-shape path-and-scheme sweep it cited, whose shapes
    ///      carry no fragment, and false of the parser, which truncated at `#`
    ///      without looking while Go unescapes the fragment and refuses the
    ///      whole URL on a malformed escape there. `gid://bc3/Person/1#%zz`
    ///      named a person here and nobody in Go.
    ///   3. The fragment check that fixed it was written over `Character`s, and
    ///      a `Character` is a grapheme cluster: `#%\u{0301}zz` hides the `%`
    ///      from an equality test and walked straight past it, while
    ///      `#\u{0301}` hides the `#` so no fragment was found at all. No sweep
    ///      of ASCII shapes can see either.
    ///   4. The authority was checked against an alphabet of "printable ASCII"
    ///      rather than against Go's, which is three alphabets: `unescape`'s for
    ///      a host, `validUserinfo`'s for userinfo, and — inside brackets —
    ///      `netip.ParseAddr`, so `[notanip]` and even `[1.2.3.4]` are hosts Go
    ///      refuses. Ninety-four shapes of a byte sweep went the accepting way.
    ///
    /// A comment whose scope is narrower than its claim reads as the claim, and
    /// a sweep whose alphabet is narrower than the parser's input proves less
    /// than it appears to. The parser now reads UTF-8 bytes, which is what
    /// `net/url` reads.
    ///
    /// What remains is one mechanism, stated as a mechanism rather than as a
    /// list of shapes: Go reads
    /// `u.Path`, which is percent-DECODED, and this reads the path as written.
    /// So every gid whose path spells a structural character through an escape
    /// names a Person there and nobody here — the model name
    /// (`gid://bc3/Pers%6Fn/1`), the id (`gid://bc3/Person/%31`), and the
    /// separator between them (`gid://bc3/Person%2F1`) alike. An earlier version
    /// of this comment named only the first and read as though that were the
    /// whole of it, which is the shape of comment that stops a reader checking.
    ///
    /// The disagreement runs in that direction ONLY, and
    /// `testTheGidPathDisagreesWithGoInOneDirectionOnly` is where that is a
    /// property rather than a claim: it builds the same 747-shape cross product
    /// — nine spellings of the scheme, three prefixes, twenty-eight paths — and
    /// compares what this parser accepts against the 44 shapes Go names a
    /// person for, as a SET. Anything this accepted and Go did not would show up
    /// as an extra element. There is none, and that is the half that matters:
    /// the accepting direction is the one that would have this SDK act on a gid
    /// Go rejects.
    ///
    /// Refusing is the right way to differ here. BC3 mints the literal form, an
    /// encoded model name is not something a real sgid carries, and the read
    /// side of these helpers is the one place an attacker-supplied envelope is
    /// parsed.
    static func personId(fromGlobalId gid: String) -> Int? {
        // Parsed as UTF-8 BYTES, which is what `net/url` parses, and the reason
        // is not tidiness. A Swift `Character` is a grapheme cluster, so `%`
        // followed by a combining acute is ONE Character equal to neither `"%"`
        // nor anything else a delimiter search looks for. Reading this through
        // `Character` was wrong in both directions at once:
        // `gid://bc3/Person/1#%\u{0301}zz` walked past the fragment's escape
        // check and named person 1 where Go refuses the whole URL, and
        // `gid://bc3/Person/1#\u{0301}` hid the `#` inside a cluster so no
        // fragment was found at all and a gid Go reads as person 1 named nobody
        // here. Delimiters are ASCII bytes; they are found as bytes.
        let bytes = Array(gid.utf8)
        // No ASCII control character anywhere in it. Go hands the gid to
        // `net/url`, which refuses one outright; a parser that strips tab, CR
        // and LF before parsing — the WHATWG rule Foundation follows, and the
        // one that bit the Python port — reads `gid://bc3/Person/104\n9715915`
        // as a clean person id where Go reads nothing. The digits-only check
        // below already catches that exact shape, but the host and scheme are
        // not digits, and the write side's only authenticity-adjacent gate is
        // "does this sgid name this person": a parser more forgiving than Go's
        // renders a mention tag Go refuses to write.
        guard !bytes.contains(where: { $0 < 0x20 || $0 == 0x7F }) else { return nil }
        // Go's `getScheme` admits only ASCII in a scheme, so an ASCII fold is
        // the whole of it — a non-ASCII spelling leaves Go with no scheme
        // rather than with a scheme that folds to `gid`.
        guard let schemeEnd = firstRange(of: "://", in: bytes, from: 0) else { return nil }
        guard schemeEnd == 3, hasPrefix(bytes, at: 0, "gid") else { return nil }
        var rest = bytes[(schemeEnd + 3)...]
        // The fragment and the query are kept out of the path, but they are not
        // equivalent and truncating at the first of either gets one of them
        // wrong. Go splits the fragment off the whole URL and UNESCAPES it, so a
        // malformed escape there refuses the entire gid —
        // `gid://bc3/Person/1#%zz` names nobody in Go. It keeps `RawQuery` raw
        // and validates nothing, so `gid://bc3/Person/1?%zz` is a gid Go reads.
        // Truncating at `#` without looking accepted the first, which is the
        // permissive direction: a gid Go refuses, read here as a mention.
        if let hash = rest.firstIndex(of: asciiHash) {
            guard isWellFormedPercentEscaping(rest[(hash + 1)...]) else { return nil }
            rest = rest[..<hash]
        }
        if let question = rest.firstIndex(of: asciiQuestion) { rest = rest[..<question] }
        guard let hostEnd = rest.firstIndex(of: asciiSlash), hostEnd != rest.startIndex else { return nil }
        // The authority is checked against what `net/url` accepts, which is the
        // parser Go hands the gid to. Both directions matter and an allowlist
        // gets one of them wrong: too loose and `gid://@/Person/1` names a
        // person here that Go refuses — the write side would render a tag Go
        // will not write; too strict and `gid://b%C3%A9c3/Person/1`, a host Go
        // accepts, names nobody here and a real mention is lost.
        guard isValidGlobalIdAuthority(rest[..<hostEnd]) else { return nil }

        let path = rest[(hostEnd + 1)...]
        guard let modelEnd = path.firstIndex(of: asciiSlash) else { return nil }
        guard path[..<modelEnd].elementsEqual("Person".utf8) else { return nil }

        let rawId = path[(modelEnd + 1)...]
        guard !rawId.isEmpty, rawId.allSatisfy({ $0 >= asciiZero && $0 <= asciiNine }) else { return nil }
        guard let id = Int(String(decoding: rawId, as: UTF8.self)), id > 0 else { return nil }
        return id
    }
}

// MARK: - Byte helpers

private let asciiLessThan = UInt8(ascii: "<")
private let asciiGreaterThan = UInt8(ascii: ">")
private let asciiSlash = UInt8(ascii: "/")
private let asciiEquals = UInt8(ascii: "=")
private let asciiBang = UInt8(ascii: "!")
private let asciiQuestion = UInt8(ascii: "?")
private let asciiDoubleQuote = UInt8(ascii: "\"")
private let asciiSingleQuote = UInt8(ascii: "'")
private let asciiOpenBrace = UInt8(ascii: "{")
private let asciiHash = UInt8(ascii: "#")
private let asciiAt = UInt8(ascii: "@")
private let asciiColon = UInt8(ascii: ":")
private let asciiPercent = UInt8(ascii: "%")
private let asciiOpenBracket = UInt8(ascii: "[")
private let asciiCloseBracket = UInt8(ascii: "]")
private let asciiZero = UInt8(ascii: "0")
private let asciiNine = UInt8(ascii: "9")
private let asciiDot = UInt8(ascii: ".")
private let asciiTwo = UInt8(ascii: "2")
private let asciiFive = UInt8(ascii: "5")

private func isSpaceByte(_ c: UInt8) -> Bool {
    c == 0x20 || c == 0x09 || c == 0x0A || c == 0x0D || c == 0x0C
}

/// What may follow `<` in a tag name. The whole name is consumed, punctuation
/// included, so `<bc-attachment:preview` or `<bc-attachment_x` is its own name
/// and never compares equal to `bc-attachment`.
private func isTagNameByte(_ c: UInt8) -> Bool {
    !isSpaceByte(c) && c != asciiSlash && c != asciiGreaterThan && c != asciiLessThan
        && c != asciiEquals && c != asciiDoubleQuote && c != asciiSingleQuote
}

private func isTagNameEndByte(_ c: UInt8) -> Bool {
    isSpaceByte(c) || c == asciiSlash || c == asciiGreaterThan
}

/// Case-insensitive ASCII prefix test at a byte offset.
private func hasPrefix(_ bytes: [UInt8], at index: Int, _ prefix: String) -> Bool {
    let needle = Array(prefix.utf8)
    guard index >= 0, index + needle.count <= bytes.count else { return false }
    for (offset, expected) in needle.enumerated() where lowercasedAscii(bytes[index + offset]) != lowercasedAscii(expected) {
        return false
    }
    return true
}

private func lowercasedAscii(_ c: UInt8) -> UInt8 {
    (c >= 65 && c <= 90) ? c + 32 : c
}

/// Index of the first occurrence of an ASCII needle at or after `from`.
private func firstRange(of needle: String, in bytes: [UInt8], from: Int) -> Int? {
    let pattern = Array(needle.utf8)
    guard !pattern.isEmpty, from <= bytes.count - pattern.count else { return nil }
    var i = from
    // Compared byte by byte rather than by slicing into a fresh Array, which
    // allocated once per byte scanned.
    outer: while i + pattern.count <= bytes.count {
        for (offset, expected) in pattern.enumerated() where bytes[i + offset] != expected {
            i += 1
            continue outer
        }
        return i
    }
    return nil
}

/// Decodes base64 whose padding has been stripped, in either alphabet's
/// standard-mapped form. Returns nil for anything `Data(base64Encoded:)` would
/// refuse, a length that cannot be a base64 encoding included.
private func decodeUnpaddedBase64(_ value: String) -> Data? {
    let remainder = value.utf8.count % 4
    if remainder == 1 { return nil }
    let padded = value + String(repeating: "=", count: remainder == 0 ? 0 : 4 - remainder)
    return Data(base64Encoded: padded)
}

/// Decodes the HTML character references that can appear in an attribute value.
///
/// Narrower than Go's `html.UnescapeString`, and narrow along a line that cannot
/// change an outcome. The value this is applied to is an `attachable_sgid` — so
/// after the decode it is trimmed, split on the last `--`, and base64-decoded.
/// Only two kinds of reference can change what that produces:
///
///   * one that expands to a character the base64 alphabet contains, and
///   * one that expands to WHITESPACE, because the trim then erases it — which
///     is how `sgid="&nbspBAh7…"` names a person in Go and would name nobody
///     under a table that did not know `&nbsp`.
///
/// Every other reference expands to something the alphabet does not contain, so
/// the envelope fails to decode whether it was expanded or left verbatim: same
/// answer, reached differently. The table below is exactly those two kinds, and
/// every entry in it was read off `html.UnescapeString` rather than guessed —
/// which is how `&hyphen;` and `&dash;` are absent (both name U+2010, not ASCII
/// `-`) and how `&ThickSpace;` comes to be two scalars.
private func unescapeEntities(_ value: String) -> String {
    guard value.contains("&") else { return value }

    var out = ""
    out.reserveCapacity(value.count)
    var rest = Substring(value)
    while let amp = rest.firstIndex(of: "&") {
        out += rest[..<amp]
        let after = rest.index(after: amp)
        guard let (replacement, end) = entityAt(rest, from: after) else {
            out.append("&")
            rest = rest[after...]
            continue
        }
        out += replacement
        rest = rest[end...]
    }
    out += rest
    return out
}

/// Reads one character reference starting just after its `&`, returning the
/// expansion and the index after it. Nil when there is no reference there.
private func entityAt(_ text: Substring, from start: Substring.Index) -> (String, Substring.Index)? {
    // Numeric, ported from Go's `unescapeEntity` rather than from a reading of
    // what it ought to do — the boundary is not guessable. `&#9x` is LITERAL
    // (only one character consumed after `&#`) while `&#10x` is a newline and
    // `&#x9x` is a tab, because the `x` counts toward the same index. And the
    // value is not the character: 0x80–0x9F are remapped through Windows-1252
    // (`&#133;` is an ellipsis, NOT the NEL that would have been trimmed), and
    // NUL, the surrogates and anything past U+10FFFF become U+FFFD.
    //
    // A digit is an ASCII BYTE, tested the way Go tests it. `Character` has a
    // `hexDigitValue`, and reaching for it is the obvious move and wrong: it
    // accepts the fullwidth forms U+FF10–U+FF19 and U+FF21–U+FF26/U+FF41–U+FF46,
    // which Go and HTML5 both refuse. That is not a curiosity — it re-opens the
    // suppression attack, because `sgid="&#x４２;<rest of a real sgid>"` would
    // decode HERE to the authoritative sgid and nowhere else, so the write-side
    // dedupe would skip the real mention while BC3, parsing HTML5, sees only the
    // decorated tag and mentions nobody.
    //
    // The accumulator is Int32 and it WRAPS, because Go's is a `rune` and Go's
    // wraps: `&#x100000041;` is `A` there, not a refusal, and a wrap that lands
    // back inside the valid range is a character Go writes. Latching to U+FFFD
    // instead reported one mention fewer than the contract on 357 of 20,000
    // fuzzed inputs.
    if start < text.endIndex, text[start] == "#" {
        var consumed = 2  // Go's `i`, counting the "&" and the "#"
        var cursor = text.index(after: start)
        var hex = false
        if cursor < text.endIndex, text[cursor] == "x" || text[cursor] == "X" {
            hex = true
            cursor = text.index(after: cursor)
            consumed += 1
        }

        var value: Int32 = 0
        while cursor < text.endIndex {
            let c = text[cursor]
            cursor = text.index(after: cursor)
            consumed += 1
            if let digit = asciiDigitValue(c, hex: hex) {
                value = value &* (hex ? 16 : 10) &+ digit
                continue
            }
            if c != ";" { consumed -= 1; cursor = text.index(before: cursor) }
            break
        }
        guard consumed > 3 else { return nil }  // "No characters matched."

        var scalarValue = value
        if scalarValue >= 0x80, scalarValue <= 0x9F {
            scalarValue = windows1252Replacements[Int(scalarValue - 0x80)]
        } else if scalarValue == 0 || (scalarValue >= 0xD800 && scalarValue <= 0xDFFF)
            || scalarValue > 0x10_FFFF || scalarValue < 0
        {
            // The negative arm is Go's too: a wrapped-negative rune reaches
            // `utf8.EncodeRune`, which writes U+FFFD for anything out of range.
            scalarValue = 0xFFFD
        }
        guard let scalar = Unicode.Scalar(UInt32(scalarValue)) else { return nil }
        return (String(Character(scalar)), cursor)
    }

    // Named. A name is alphanumeric — `emsp13` is one — and is matched against
    // the TABLE, longest entry first, rather than by consuming the longest run
    // of name characters and demanding a semicolon after it. That distinction is
    // the whole of `&nbspBAh7…`: the run there is `nbspBAh`, which is not a
    // name, but `nbsp` is, and Go expands it.
    var cursor = start
    var length = 0
    // Counted, not re-measured: `String.distance` is O(k), so calling it in the
    // loop condition makes a bounded scan quadratic in its own bound.
    while cursor < text.endIndex, length < maxEntityNameLength, text[cursor].isASCII,
        text[cursor].isLetter || text[cursor].isNumber
    {
        cursor = text.index(after: cursor)
        length += 1
    }
    guard start < cursor else { return nil }

    // With its semicolon, any name in the table.
    if cursor < text.endIndex, text[cursor] == ";",
        let replacement = namedEntities[String(text[start..<cursor])]
    {
        return (replacement, text.index(after: cursor))
    }
    // Without one, only the legacy names allow it — longest first, and no
    // longer than the longest legacy name. Go bounds the same descent with
    // `longestEntityWithoutSemicolon`, so trying every prefix of a 24-character
    // run would be 24 allocations and hashes per `&` where Go does at most six.
    var candidate = cursor
    var candidateLength = length
    while candidateLength > longestLegacyEntityName {
        candidate = text.index(before: candidate)
        candidateLength -= 1
    }
    while candidate > start {
        if let replacement = legacyEntities[String(text[start..<candidate])] {
            return (replacement, candidate)
        }
        candidate = text.index(before: candidate)
    }
    return nil
}

/// Longer than any name in the table below, and short enough that a stray `&`
/// before a long run of letters costs nothing to refuse.
private let maxEntityNameLength = 24

/// The longest name in ``legacyEntities``, which bounds the no-semicolon
/// descent the way Go's `longestEntityWithoutSemicolon` bounds its own.
private let longestLegacyEntityName = 4

/// What Go's `unicode.IsSpace` calls whitespace — the set `strings.TrimSpace`
/// uses, and so the set that decides whether a reference expanded at either end
/// of an sgid is erased before the decode.
///
/// Spelled out rather than taken from `CharacterSet.whitespacesAndNewlines`,
/// which is not the same set: it contains U+200B ZERO WIDTH SPACE, which Go does
/// not, so `&#8203;` before a payload was trimmed here and left in place there —
/// an sgid that named a person here and nobody in Go.
private let goWhitespace: CharacterSet = {
    var set = CharacterSet()
    for scalar in [0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x20, 0x85, 0xA0, 0x1680, 0x2028, 0x2029,
        0x202F, 0x205F, 0x3000]
    {
        set.insert(Unicode.Scalar(UInt32(scalar))!)
    }
    set.insert(charactersIn: Unicode.Scalar(0x2000)!...Unicode.Scalar(0x200A)!)
    return set
}()

/// A numeric reference's digit, tested as an ASCII byte the way Go tests it.
/// Nil for everything else — including the fullwidth digit forms that
/// `Character.hexDigitValue` accepts and Go refuses. The `asciiValue` guard is
/// what refuses them: a fullwidth digit has none.
private func asciiDigitValue(_ c: Character, hex: Bool) -> Int32? {
    guard let byte = c.asciiValue else { return nil }
    return asciiDigitValue(byte, hex: hex)
}

/// The same table, reached from a byte. One table, so a percent escape and a
/// numeric character reference cannot drift apart about what a hex digit is.
private func asciiDigitValue(_ byte: UInt8, hex: Bool) -> Int32? {
    switch byte {
    case UInt8(ascii: "0")...UInt8(ascii: "9"):
        return Int32(byte - UInt8(ascii: "0"))
    case UInt8(ascii: "a")...UInt8(ascii: "f") where hex:
        return Int32(byte - UInt8(ascii: "a")) + 10
    case UInt8(ascii: "A")...UInt8(ascii: "F") where hex:
        return Int32(byte - UInt8(ascii: "A")) + 10
    default:
        return nil
    }
}

/// Go's `replacementTable`: what a numeric reference in 0x80–0x9F becomes.
private let windows1252Replacements: [Int32] = [
    0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
    0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
    0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
    0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
]

/// The references whose expansion is a base64-alphabet character or whitespace —
/// the only two kinds that can change what an sgid decodes to.
///
/// Not assembled from memory. Every name in Go's table was run through
/// `html.UnescapeString` and kept if its expansion is all-whitespace or contains
/// a base64 character: 2,231 names in, 24 rows out, and those 24 are here with
/// the values that call gave them.
///
/// Through the FUNCTION, not by parsing the table — that distinction is
/// load-bearing. Go keeps a second table for two-rune expansions, so a
/// classification of the single-rune one misses `&fjlig;` ("fj", two alphabet
/// characters) and `&bne;`. It is also how `&hyphen;` and `&dash;` are known to
/// be absent: both name U+2010 rather than ASCII `-`.
private let namedEntities: [String: String] = [
    // The five a serializer emits, which the tag walk has to see through.
    "amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'",
    "AMP": "&", "LT": "<", "GT": ">", "QUOT": "\"",
    // Base64 punctuation. There is no named reference for ASCII `-`:
    // `&hyphen;` and `&dash;` are U+2010.
    "plus": "+", "sol": "/", "equals": "=", "lowbar": "_", "UnderBar": "_",
    // Go keeps a SECOND table for two-rune expansions, and two of its entries
    // land here. `&fjlig;` is the one that can change an answer on its own — two
    // alphabet characters — and it is invisible to anyone who classifies the
    // single-rune table alone.
    "fjlig": "fj", "bne": "=\u{20E5}",
    // Whitespace, which the trim erases at either end of the value.
    "Tab": "\u{09}", "NewLine": "\u{0A}",
    "nbsp": "\u{A0}", "NonBreakingSpace": "\u{A0}",
    "ensp": "\u{2002}", "emsp": "\u{2003}", "emsp13": "\u{2004}", "emsp14": "\u{2005}",
    "numsp": "\u{2007}", "puncsp": "\u{2008}",
    "thinsp": "\u{2009}", "ThinSpace": "\u{2009}",
    "hairsp": "\u{200A}", "VeryThinSpace": "\u{200A}",
    "MediumSpace": "\u{205F}", "ThickSpace": "\u{205F}\u{200A}",
]

/// The subset Go expands without a terminating semicolon. HTML5's legacy list is
/// longer; the rest expand to characters the alphabet does not contain and the
/// trim does not remove, so including them could not change an answer.
private let legacyEntities: [String: String] = [
    "amp": "&", "lt": "<", "gt": ">", "quot": "\"",
    "AMP": "&", "LT": "<", "GT": ">", "QUOT": "\"",
    "nbsp": "\u{A0}",
]

/// Whether `net/url` would accept this authority.
///
/// Ported from what Go's parser does rather than from a charset that looked
/// about right, and the WHOLE authority is checked rather than just the host —
/// both of those because both were measured wrong first. Each rule below is a
/// row in `testTheAuthorityMatchesWhatGoAccepts`:
///
///   * **Userinfo** may carry an escape naming any byte, but a MALFORMED escape,
///     a space or a control refuses the gid: `bad%zz@bc3` and `us er@bc3` name
///     nobody in Go.
///   * **The host** is what remains after any userinfo, and it may not be empty
///     — which is why `gid://user@/Person/1` names nobody. The emptiness test is
///     on the host WITH its port, as Go's is, so `gid://:8080/Person/1` is a
///     host Go accepts.
///   * **In the host** a percent escape is allowed only when it names a
///     non-ASCII byte, `%25` excepted. `b%C3%A9c3` decodes to `béc3` and is a
///     host Go ACCEPTS; `a%41b` is not. In userinfo the same escape is fine.
///   * **A port** is a `:` and then digits, and nothing else: `bc3:8080`,
///     `bc3:` and even `bc3:99999999999` are accepted, `bc3:notaport` is not.
///     Go splits at the LAST colon, which is why `bc3:80:80` is a host as well.
///   * **A bracketed literal** carries its port after the `]`.
///
/// Everything else — `"`, `<`, `>`, `]`, `_` — Go accepts, and so must this, or
/// a gid Go reads as a mention names nobody here. The host is never read beyond
/// this check; the only job is to agree with Go about which gids exist.
///
/// Every one of these reads UTF-8 bytes rather than `Character`s, for the reason
/// the parser above does: a delimiter hidden in a grapheme cluster is a
/// delimiter this would not find and `net/url` would.
///
/// The rules are `parseAuthority` and `parseHost`, ported rather than
/// approximated, because an approximation was measured wrong in the accepting
/// direction ninety-four ways over a byte sweep of the authority: a bracketed
/// host accepted whatever was between the brackets where Go requires an IPv6
/// address, and both the host and the userinfo accepted ASCII that Go's
/// `unescape` and `validUserinfo` refuse.
private func isValidGlobalIdAuthority(_ authority: ArraySlice<UInt8>) -> Bool {
    var host = authority
    if let at = authority.lastIndex(of: asciiAt) {
        guard isValidUserinfo(authority[..<at]) else { return false }
        host = authority[(at + 1)...]
    }
    guard !host.isEmpty else { return false }

    // `parseHost` looks for the LAST `[`: at 0 this is an IP literal, anywhere
    // else it is malformed, and a second one puts it anywhere else.
    if let openBracket = host.lastIndex(of: asciiOpenBracket) {
        guard openBracket == host.startIndex else { return false }
        // A bracketed IP literal carries its port outside the brackets.
        guard let close = host.lastIndex(of: asciiCloseBracket) else { return false }
        guard isValidOptionalPort(host[(close + 1)...]) else { return false }
        return isValidIPLiteral(host[(host.startIndex + 1)..<close])
    }
    if let colon = host.lastIndex(of: asciiColon) {
        guard isValidOptionalPort(host[colon...]) else { return false }
        host = host[..<colon]
    }
    // Deliberately NOT re-checked for emptiness: Go's non-empty test is on
    // `u.Host`, which still carries the port, so `gid://:8080/Person/1` is a
    // host Go accepts and names a person for.
    return unescapedHostBytes(host, zone: false) != nil
}

/// Go's `validUserinfo` followed by `unescape(_, encodeUserPassword)`: an
/// allowlist of ASCII, and well-formed escapes. It is an allowlist over RUNES,
/// so every non-ASCII byte fails it — `gid://bé@bc3/Person/1` names nobody in
/// Go, while `gid://bé c3/Person/1`… is not a host at all, and
/// `gid://b%C3%A9c3/Person/1` IS a host Go reads. The three are not one rule.
private func isValidUserinfo(_ userinfo: ArraySlice<UInt8>) -> Bool {
    var index = userinfo.startIndex
    while index < userinfo.endIndex {
        let c = userinfo[index]
        guard c < 0x80, isUserinfoByte(c) else { return false }
        guard c == asciiPercent else {
            index += 1
            continue
        }
        guard percentEscapedByte(userinfo, at: index) != nil else { return false }
        index += 3
    }
    return true
}

/// `unescape(_, encodeHost)` and `unescape(_, encodeZone)`, which both refuse a
/// malformed escape and any ASCII byte the host grammar requires to be escaped,
/// and return the decoded bytes. `encodeHost` additionally refuses an escape
/// that names an ASCII byte — `a%41b` is not a host — with `%25` excepted, which
/// is how RFC 6874 introduces a zone. Non-ASCII bytes pass through either way,
/// which is why `b%C3%A9c3` and a literal `béc3` are both hosts Go accepts.
private func unescapedHostBytes(_ text: ArraySlice<UInt8>, zone: Bool) -> [UInt8]? {
    var decoded: [UInt8] = []
    decoded.reserveCapacity(text.count)
    var index = text.startIndex
    while index < text.endIndex {
        let c = text[index]
        guard c == asciiPercent else {
            guard c >= 0x80 || isHostByte(c) else { return nil }
            decoded.append(c)
            index += 1
            continue
        }
        guard let byte = percentEscapedByte(text, at: index) else { return nil }
        guard zone || byte >= 0x80 || byte == 0x25 else { return nil }
        decoded.append(UInt8(byte))
        index += 3
    }
    return decoded
}

/// What `parseHost` does with what is between the brackets: split the RFC 6874
/// zone off at the first `%25`, unescape the two halves under their own rules,
/// and require the result to be an IPv6 address. Anything else — `[notanip]`,
/// `[]`, `[::1]]`, and an IPv4 literal like `[1.2.3.4]`, which `parseHost`
/// refuses outright — is not a host, and a gid whose host is not a host names
/// nobody.
private func isValidIPLiteral(_ content: ArraySlice<UInt8>) -> Bool {
    var decoded: [UInt8]
    if let zoneStart = indexOfPercent25(content) {
        guard let head = unescapedHostBytes(content[..<zoneStart], zone: false),
            let tail = unescapedHostBytes(content[zoneStart...], zone: true)
        else { return false }
        decoded = head + tail
    } else {
        guard let whole = unescapedHostBytes(content, zone: false) else { return false }
        decoded = whole
    }
    // `netip.ParseAddr` dispatches on the first of `.`, `:` or `%`: a dot means
    // IPv4, which `parseHost` then refuses because only an IPv6 address may be
    // bracketed; a percent before any colon is "missing IPv6 address"; nothing
    // at all is "unable to parse IP". Only a colon reaches the IPv6 parser.
    for byte in decoded {
        if byte == asciiColon { return isIPv6Address(decoded) }
        if byte == asciiDot || byte == asciiPercent { return false }
    }
    return false
}

private func indexOfPercent25(_ text: ArraySlice<UInt8>) -> Int? {
    var index = text.startIndex
    while index + 2 < text.endIndex {
        if text[index] == asciiPercent, text[index + 1] == asciiTwo, text[index + 2] == asciiFive {
            return index
        }
        index += 1
    }
    return nil
}

/// `netip.parseIPv6`, ported as the predicate this needs. The shape of the
/// grammar is the point: at most one `::`, which must stand for at least one
/// zero group; at most four hex digits a group; an embedded IPv4 tail only in
/// the final two groups; a zone after `%` that may not be empty; and no
/// trailing anything.
private func isIPv6Address(_ input: [UInt8]) -> Bool {
    var s = input[...]
    // The zone is split off at the FIRST `%`, not the last, so everything after
    // it is zone however many more percents it carries. `00::%25::%25` is a
    // host Go reads for that reason alone.
    if let percent = input.firstIndex(of: asciiPercent) {
        // A zone was named, so it may not be empty.
        guard percent + 1 < input.endIndex else { return false }
        s = input[..<percent]
    }

    var filled = 0
    var ellipsis = -1
    if s.count >= 2, s[s.startIndex] == asciiColon, s[s.startIndex + 1] == asciiColon {
        ellipsis = 0
        s = s[(s.startIndex + 2)...]
        if s.isEmpty { return true }
    }

    while filled < 16 {
        var digits = 0
        var group: UInt32 = 0
        while s.startIndex + digits < s.endIndex,
            let value = asciiDigitValue(s[s.startIndex + digits], hex: true)
        {
            group = (group << 4) + UInt32(value)
            digits += 1
            if digits > 4 { return false }
        }
        guard digits > 0 else { return false }

        if s.startIndex + digits < s.endIndex, s[s.startIndex + digits] == asciiDot {
            // An embedded IPv4 tail fills the last two groups and ends the address.
            guard ellipsis >= 0 || filled == 12 else { return false }
            guard filled + 4 <= 16 else { return false }
            guard isIPv4Address(s) else { return false }
            filled += 4
            s = s[s.endIndex...]
            break
        }

        filled += 2
        s = s[(s.startIndex + digits)...]
        if s.isEmpty { break }

        guard s[s.startIndex] == asciiColon else { return false }
        guard s.count > 1 else { return false }
        s = s[(s.startIndex + 1)...]

        if s[s.startIndex] == asciiColon {
            guard ellipsis < 0 else { return false }
            ellipsis = filled
            s = s[(s.startIndex + 1)...]
            if s.isEmpty { break }
        }
    }

    guard s.isEmpty else { return false }
    if filled < 16 { return ellipsis >= 0 }
    return ellipsis < 0
}

/// `netip.parseIPv4`: exactly four dot-separated decimal octets, none empty,
/// none above 255, and none written with a leading zero.
private func isIPv4Address(_ text: ArraySlice<UInt8>) -> Bool {
    var value = 0
    var separators = 0
    var digits = 0
    var index = text.startIndex
    while index < text.endIndex {
        let c = text[index]
        if c >= asciiZero && c <= asciiNine {
            if digits == 1 && value == 0 { return false }
            value = value * 10 + Int(c - asciiZero)
            digits += 1
            if value > 255 { return false }
        } else if c == asciiDot {
            guard index != text.startIndex, index != text.endIndex - 1 else { return false }
            guard text[index - 1] != asciiDot else { return false }
            guard separators != 3 else { return false }
            separators += 1
            value = 0
            digits = 0
        } else {
            return false
        }
        index += 1
    }
    return separators == 3
}

/// The ASCII a host or a zone may spell literally — Go's `shouldEscape` for
/// `encodeHost`, which is wider than the RFC's reg-name because it carries the
/// port and the brackets inside `u.Host`.
private func isHostByte(_ c: UInt8) -> Bool {
    switch c {
    case UInt8(ascii: "a")...UInt8(ascii: "z"), UInt8(ascii: "A")...UInt8(ascii: "Z"),
        asciiZero...asciiNine:
        return true
    case UInt8(ascii: "-"), UInt8(ascii: "_"), asciiDot, UInt8(ascii: "~"):
        return true
    case asciiBang, UInt8(ascii: "$"), UInt8(ascii: "&"), asciiSingleQuote,
        UInt8(ascii: "("), UInt8(ascii: ")"), UInt8(ascii: "*"), UInt8(ascii: "+"),
        UInt8(ascii: ","), UInt8(ascii: ";"), asciiEquals, asciiColon,
        asciiOpenBracket, asciiCloseBracket, asciiLessThan, asciiGreaterThan,
        asciiDoubleQuote:
        return true
    default:
        return false
    }
}

/// Go's `validUserinfo` allowlist.
private func isUserinfoByte(_ c: UInt8) -> Bool {
    switch c {
    case UInt8(ascii: "a")...UInt8(ascii: "z"), UInt8(ascii: "A")...UInt8(ascii: "Z"),
        asciiZero...asciiNine:
        return true
    case UInt8(ascii: "-"), asciiDot, UInt8(ascii: "_"), asciiColon, UInt8(ascii: "~"),
        asciiBang, UInt8(ascii: "$"), UInt8(ascii: "&"), asciiSingleQuote,
        UInt8(ascii: "("), UInt8(ascii: ")"), UInt8(ascii: "*"), UInt8(ascii: "+"),
        UInt8(ascii: ","), UInt8(ascii: ";"), asciiEquals, asciiPercent, asciiAt:
        return true
    default:
        return false
    }
}

/// Whether every `%` in `text` introduces a well-formed `%XX`. Go unescapes a
/// fragment, and `unescape` errors on a malformed escape, which refuses the
/// whole URL — so this is what stands between a gid Go rejects and a mention.
private func isWellFormedPercentEscaping(_ text: ArraySlice<UInt8>) -> Bool {
    var index = text.startIndex
    while index < text.endIndex {
        guard text[index] == asciiPercent else {
            index += 1
            continue
        }
        guard percentEscapedByte(text, at: index) != nil else { return false }
        index += 3
    }
    return true
}

/// The byte a `%XX` at `index` names, or nil when the escape is malformed.
private func percentEscapedByte(_ text: ArraySlice<UInt8>, at index: Int) -> Int? {
    guard index + 2 < text.endIndex else { return nil }
    guard let high = asciiDigitValue(text[index + 1], hex: true),
        let low = asciiDigitValue(text[index + 2], hex: true)
    else { return nil }
    return Int(high) * 16 + Int(low)
}

/// Go's `validOptionalPort`: empty, or a colon followed by digits and nothing
/// else. A bare colon is valid, and there is no range check.
private func isValidOptionalPort(_ port: ArraySlice<UInt8>) -> Bool {
    guard !port.isEmpty else { return true }
    guard port.first == asciiColon else { return false }
    return port.dropFirst().allSatisfy { $0 >= asciiZero && $0 <= asciiNine }
}
