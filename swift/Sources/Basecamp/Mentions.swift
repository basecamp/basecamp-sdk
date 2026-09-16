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
        let value = Array(sgid.trimmingCharacters(in: goWhitespace).utf8)
        // `strings.LastIndex` is a byte scan and so is this. `range(of:)`
        // without `.literal` searches grapheme clusters, so a `--` whose second
        // dash carries a combining mark is invisible to it and plain to Go —
        // and then the two sides split the envelope at DIFFERENT places.
        // `<payload>--x--\u{0301}` is the shape: Go splits at the last `--` and
        // reads a payload that is not base64, this splits at the first and reads
        // a person. That is the accepting direction on the read side, and the
        // write side's gate is "does this sgid name this person", so it renders
        // a tag Go refuses to write.
        if let separator = lastIndexOfSeparator(value), separator > 0,
            let gid = envelopeGlobalId(value[..<separator])
        {
            return gid
        }
        return envelopeGlobalId(value[...])
    }

    /// The last `--` in the value, as `strings.LastIndex` finds it.
    private static func lastIndexOfSeparator(_ value: [UInt8]) -> Int? {
        guard value.count >= 2 else { return nil }
        var index = value.count - 2
        while index >= 0 {
            if value[index] == asciiDash, value[index + 1] == asciiDash { return index }
            index -= 1
        }
        return nil
    }

    /// Decodes one base64 payload and returns the gid its envelope carries.
    static func envelopeGlobalId(_ payload: ArraySlice<UInt8>) -> String? {
        // The bound is applied to the encoded form first, so an oversized sgid
        // costs nothing to refuse — no normalization, no decode buffer.
        guard !payload.isEmpty, payload.count <= maxSgidEncodedBytes else { return nil }
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
        //
        // Byte-level throughout, and each of the three steps had to be: a CRLF
        // is ONE Swift `Character`, so a Character-level filter for "\r" or
        // "\n" walks past the pair a line-wrapping serializer emits; and
        // `replacingOccurrences` searches grapheme clusters, so a `-` carrying a
        // combining mark is a byte Go maps to `+` and Foundation leaves alone.
        var normalized = payload.map { byte -> UInt8 in
            switch byte {
            case asciiDash: return asciiPlus
            case asciiUnderscore: return asciiSlash
            default: return byte
            }
        }
        while normalized.last == asciiEquals { normalized.removeLast() }
        let stripped = normalized.filter { $0 != 0x0D && $0 != 0x0A }
        guard !stripped.contains(asciiEquals) else { return nil }

        guard let raw = decodeUnpaddedBase64(stripped), !raw.isEmpty,
            raw.count <= maxSgidPayloadBytes
        else { return nil }

        let bytes = [UInt8](raw)
        let envelope: [String: Any]
        // Only the JSON branch smuggles a byte-order mark past the parser, so
        // only the JSON branch decodes one back. A Marshal envelope whose gid
        // legitimately carries the sentinels is Go's answer as it stands, and
        // running the inverse over it would rewrite bytes nothing encoded.
        let isJSON = !(bytes.count >= 2 && bytes[0] == 0x04 && bytes[1] == 0x08)
        if bytes.count >= 2, bytes[0] == 0x04, bytes[1] == 0x08 {
            guard let decoded = try? RubyMarshal.decode(Array(bytes.dropFirst(2))),
                let map = decoded as? [String: Any]
            else { return nil }
            envelope = map
        } else if bytes[0] == asciiOpenBrace {
            guard let json = jsonEnvelopeBytes(bytes),
                let decoded = try? JSONSerialization.jsonObject(with: json),
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
            return isJSON ? bomDecoded(gid) : gid
        }
        // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
        guard envelope["purpose"] as? String == attachablePurpose else { return nil }
        guard let gid = envelope["gid"] as? String, !gid.isEmpty else { return nil }
        return isJSON ? bomDecoded(gid) : gid
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
    ///   5. Fixing 4 introduced a FOURTH alphabet and left it unwritten: an
    ///      escape inside a zone follows the mirror of the host's rule, not no
    ///      rule, so `[::1%25%C3%A9]` names nobody in Go and named person 1
    ///      here. 171 of the 256 byte values went that way — and the sweep
    ///      added in 4 could not see it, because none of its shapes put a second
    ///      escape inside a zone.
    ///   6. The control-character check was run on the whole gid where Go runs
    ///      it on the gid with the fragment already cut off, so every control
    ///      after a `#` lost a mention Go reports. Checking more than Go reads
    ///      as the safe direction and is not one.
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
        // No ASCII control character — but only BEFORE the fragment, because
        // that is where Go's is. `url.Parse` cuts the fragment off the raw URL
        // first and runs `stringContainsCTLByte` on what is left, so
        // `gid://bc3/Person/1#\u{01}` is person 1 there; the query is NOT cut
        // first, so `gid://bc3/Person/1?\u{01}` is nobody. Checking the whole
        // string reads as the safer choice and is not one: it loses 34 real
        // mentions per position and guards nothing, since the fragment is
        // discarded either way.
        //
        // The check itself matters where it does apply. Go hands the gid to
        // `net/url`, which refuses a control outright; a parser that strips tab,
        // CR and LF before parsing — the WHATWG rule Foundation follows, and the
        // one that bit the Python port — reads `gid://bc3/Person/104\n9715915`
        // as a clean person id where Go reads nothing. The digits-only check
        // below already catches that exact shape, but the host and scheme are
        // not digits, and the write side's only authenticity-adjacent gate is
        // "does this sgid name this person": a parser more forgiving than Go's
        // renders a mention tag Go refuses to write.
        let beforeFragment = bytes[..<(bytes.firstIndex(of: asciiHash) ?? bytes.endIndex)]
        guard !beforeFragment.contains(where: { $0 < 0x20 || $0 == 0x7F }) else { return nil }
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

/// The bytes to hand `JSONSerialization`, reconciled with `encoding/json` in
/// both directions — nil when Go's scanner would refuse the document outright.
///
/// **Two substitutions Go makes and Foundation will not.** `json.Unmarshal` does
/// not reject a string it cannot read as UTF-8; it substitutes U+FFFD, for an
/// invalid byte sequence and for a `\uXXXX` escape naming an unpaired surrogate
/// alike. `JSONSerialization` refuses the whole DOCUMENT for either, so a single
/// stray byte anywhere in the envelope — in a field this code never reads —
/// loses an sgid Go decodes.
///
/// One detail of the first is NOT reproduced, deliberately. Go's `unquoteBytes`
/// calls `utf8.DecodeRune`, which yields one U+FFFD per invalid BYTE, while
/// Swift's decoder applies the Unicode maximal-subpart rule and collapses a
/// truncated-but-valid prefix into one. The two therefore disagree about how
/// many replacement characters a mangled sequence becomes. It is not observable
/// at any entry point — U+FFFD's bytes are all ≥ 0x80, so they are never a
/// delimiter, a hex digit, a decimal digit or part of `gid`, `://` or `Person`,
/// and nothing downstream is length-sensitive over them — and a 2,289-shape
/// sweep carrying raw invalid UTF-8 inside the gid found no answer that differs.
/// Reproducing Go's byte-at-a-time rule would mean hand-decoding UTF-8 here to
/// fix something no caller can see.
///
/// **And one leniency Foundation has that Go does not**: a trailing comma.
/// `{"a":1,}` is a document Go refuses and `JSONSerialization` accepts, which is
/// the ACCEPTING direction — an sgid read here and nowhere else — so it is
/// refused explicitly rather than left to the parser. That is also the only
/// JSON5-ish leniency measured: unquoted keys, single quotes, `NaN`, comments,
/// hex and octal literals, `\x` escapes and trailing junk are all refused on
/// both sides.
///
/// The walk is over unicode scalars rather than `Character`s on purpose: a
/// quote or a backslash followed by a combining mark is one `Character` and
/// would hide the string boundary from a `Character`-level scan, which is the
/// same trap the gid parser was in.
///
/// **One divergence is left, knowingly.** `JSONSerialization` refuses nesting
/// deeper than 511, so an envelope nested deeper than that is an sgid Go decodes
/// and this does not. The window is narrow and worth stating exactly rather than
/// by citing the scanner's own 10,000 limit, which never applies: the 4,096-byte
/// payload bound refuses the input first, and the deepest envelope that fits is
/// 2,016 arrays. So the gap is depths 512 through 2,016 — the vanishing
/// direction, a payload nobody mints, and closing it would mean replacing the
/// parser rather than pre-processing for it. Measured on
/// swift-corelibs-foundation; the macOS `NSJSONSerialization` this ships against
/// may differ, which is the other reason not to build on it.
private func jsonEnvelopeBytes(_ raw: [UInt8]) -> Data? {
    // Decoding as UTF-8 is what performs the first substitution: an invalid
    // sequence becomes U+FFFD here exactly as it does in `unquote`.
    let scalars = Array(String(decoding: raw, as: UTF8.self).unicodeScalars)
    var out = String.UnicodeScalarView()
    out.reserveCapacity(scalars.count)
    var inString = false
    var index = 0
    while index < scalars.count {
        let scalar = scalars[index]
        guard inString else {
            // Outside a string a byte-order mark is a syntax error to Go's
            // scanner — "invalid character '\u{FEFF}'" — wherever it sits.
            if scalar == "\u{FEFF}" { return nil }
            if scalar == "\"" { inString = true }
            if scalar == "," {
                // Go's scanner requires a value after a comma; Foundation does
                // not. Refusing here keeps the two agreeing without depending on
                // which Foundation is underneath.
                var next = index + 1
                while next < scalars.count, isJSONWhitespace(scalars[next]) { next += 1 }
                if next < scalars.count, scalars[next] == "}" || scalars[next] == "]" {
                    return nil
                }
            }
            out.append(scalar)
            index += 1
            continue
        }
        if scalar == "\"" {
            inString = false
            out.append(scalar)
            index += 1
            continue
        }
        if scalar == bomSentinelLead || scalar == "\u{FEFF}" {
            out.append(contentsOf: bomEncoded(scalar))
            index += 1
            continue
        }
        guard scalar == "\\", index + 1 < scalars.count else {
            out.append(scalar)
            index += 1
            continue
        }
        // Any other escape is two scalars, `\\` included — consuming both is
        // what stops `\\u0041`, a literal backslash then `u0041`, being read as
        // an escape.
        guard scalars[index + 1] == "u", index + 5 < scalars.count,
            let value = hexEscapeValue(scalars[(index + 2)...(index + 5)])
        else {
            out.append(scalar)
            out.append(scalars[index + 1])
            index += 2
            continue
        }
        // A valid pair is passed through unchanged. Nothing at any entry point
        // can currently tell that from rewriting it to two U+FFFD — a surrogate
        // pair always decodes to a non-ASCII character, so it is never a
        // delimiter, a digit or part of a keyword, and nothing downstream is
        // length-sensitive over it. It is here because it is what Go does, and
        // because the next reader of the envelope may not have that property.
        if value >= 0xD800, value <= 0xDBFF, index + 11 < scalars.count,
            scalars[index + 6] == "\\", scalars[index + 7] == "u",
            let low = hexEscapeValue(scalars[(index + 8)...(index + 11)]),
            low >= 0xDC00, low <= 0xDFFF
        {
            out.append(contentsOf: scalars[index...(index + 11)])
            index += 12
            continue
        }
        if value >= 0xD800, value <= 0xDFFF {
            out.append(contentsOf: "\\ufffd".unicodeScalars)
            index += 6
            continue
        }
        // An ESCAPED mark decodes to the same scalar the parser then eats, so it
        // takes the same substitution.
        if value == 0xFEFF || value == 0xE000 {
            out.append(contentsOf: bomEncoded(Unicode.Scalar(UInt32(value))!))
            index += 6
            continue
        }
        out.append(contentsOf: scalars[index...(index + 5)])
        index += 6
    }
    return Data(String(out).utf8)
}

/// U+FEFF, written so that no JSON parser can silently remove it.
///
/// `JSONSerialization` eats a byte-order mark from a string it decodes, and
/// `encoding/json` keeps it — so `{"\u{FEFF}_rails": …}` is an envelope Go reads
/// as neither layout and this read as a mention, on the write side too, where
/// `markup(for:)` rendered a tag Go refuses to write. It is the ONLY scalar that
/// does this: disabling the substitution and sweeping every scalar in
/// 0…0x1FFFF at nine head-of-token positions leaves nine diverging shapes, all
/// of them U+FEFF.
///
/// Escaping it as `\uFEFF` was the first fix and it was a BET: it holds on
/// swift-corelibs-foundation and CI showed it does NOT hold on Darwin, which
/// removes the mark from the escape as well. So the mark is not handed to the
/// parser at all. It is encoded into private-use scalars — U+FEFF becomes
/// U+E000 U+E001, and a literal U+E000 doubles to U+E000 U+E000 so the encoding
/// stays reversible — and decoded again out of the one string this reads back.
/// Nothing here depends on how a parser treats U+FEFF, which is the only form
/// of this fix that can be verified from the platform I can run.
private let bomSentinelLead: Unicode.Scalar = "\u{E000}"
private let bomSentinelMark: Unicode.Scalar = "\u{E001}"

private func bomEncoded(_ scalar: Unicode.Scalar) -> [Unicode.Scalar] {
    scalar == "\u{FEFF}" ? [bomSentinelLead, bomSentinelMark] : [bomSentinelLead, bomSentinelLead]
}

/// The inverse, applied to the gid a decoded envelope carries so it is the
/// bytes Go read rather than the bytes this had to smuggle past the parser.
private func bomDecoded(_ text: String) -> String {
    guard text.unicodeScalars.contains(bomSentinelLead) else { return text }
    var out = String.UnicodeScalarView()
    var scalars = Array(text.unicodeScalars)[...]
    while let first = scalars.first {
        if first == bomSentinelLead, scalars.count > 1 {
            let second = scalars[scalars.startIndex + 1]
            if second == bomSentinelMark || second == bomSentinelLead {
                out.append(second == bomSentinelMark ? "\u{FEFF}" : bomSentinelLead)
                scalars = scalars.dropFirst(2)
                continue
            }
        }
        out.append(first)
        scalars = scalars.dropFirst()
    }
    return String(out)
}

/// The four bytes `encoding/json`'s scanner skips between tokens.
private func isJSONWhitespace(_ scalar: Unicode.Scalar) -> Bool {
    scalar == " " || scalar == "\t" || scalar == "\n" || scalar == "\r"
}

/// The value of a four-digit `\uXXXX` escape, or nil when it is not four hex
/// digits — which both parsers refuse, so it is passed through unchanged.
private func hexEscapeValue(_ digits: ArraySlice<Unicode.Scalar>) -> Int? {
    var value = 0
    for scalar in digits {
        guard scalar.isASCII, let digit = asciiDigitValue(UInt8(scalar.value), hex: true) else {
            return nil
        }
        value = value * 16 + Int(digit)
    }
    return value
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
private let asciiDash = UInt8(ascii: "-")
private let asciiPlus = UInt8(ascii: "+")
private let asciiUnderscore = UInt8(ascii: "_")
private let asciiAmpersand = UInt8(ascii: "&")
private let asciiSemicolon = UInt8(ascii: ";")
private let asciiLowerX = UInt8(ascii: "x")
private let asciiUpperX = UInt8(ascii: "X")

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
private func decodeUnpaddedBase64(_ value: [UInt8]) -> Data? {
    let remainder = value.count % 4
    if remainder == 1 { return nil }
    var padded = value
    padded.append(
        contentsOf: repeatElement(asciiEquals, count: remainder == 0 ? 0 : 4 - remainder))
    return Data(base64Encoded: Data(padded))
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
/// Walked as UTF-8 BYTES, like `html.unescapeEntity` and unlike every earlier
/// version of this. A `Character` is a grapheme cluster, so a combining mark on
/// a digit makes `5` + U+0301 ONE Character that is not a digit — and the scan
/// stopped a digit early, decoding the same reference to a different character
/// than Go does. `&#455\u{0301};` is `Ǉ` in Go and was `-` here, which is the
/// sgid SEPARATOR: `<payload>-&#455\u{0301};x` named nobody in Go and person 7
/// here. It reaches the write side too, where `adding(_:to:)` dedupes against
/// the content's decoded sgids, so the same divergence silently drops a mention
/// the reference inserts — the outcome the package comment forbids.
///
/// The comment below said "a digit is an ASCII BYTE, tested the way Go tests
/// it", and that was true of the digit TEST and false of the walk around it.
private func unescapeEntities(_ value: String) -> String {
    let bytes = Array(value.utf8)
    guard bytes.contains(asciiAmpersand) else { return value }

    var out: [UInt8] = []
    out.reserveCapacity(bytes.count)
    var index = 0
    while index < bytes.count {
        guard bytes[index] == asciiAmpersand else {
            out.append(bytes[index])
            index += 1
            continue
        }
        guard let (replacement, end) = entityAt(bytes, from: index + 1) else {
            out.append(asciiAmpersand)
            index += 1
            continue
        }
        out.append(contentsOf: replacement)
        index = end
    }
    return String(decoding: out, as: UTF8.self)
}

/// Reads one character reference starting just after its `&`, returning the
/// expansion and the index after it. Nil when there is no reference there.
private func entityAt(_ bytes: [UInt8], from start: Int) -> ([UInt8], Int)? {
    // Numeric, ported from Go's `unescapeEntity` rather than from a reading of
    // what it ought to do — the boundary is not guessable. `&#9x` is LITERAL
    // (only one character consumed after `&#`) while `&#10x` is a newline and
    // `&#x9x` is a tab, because the `x` counts toward the same index. And the
    // value is not the character: 0x80–0x9F are remapped through Windows-1252
    // (`&#133;` is an ellipsis, NOT the NEL that would have been trimmed), and
    // NUL, the surrogates and anything past U+10FFFF become U+FFFD.
    //
    // A digit is an ASCII byte and so is the WALK, which is the harder half:
    // see the note on `unescapeEntities`. `Character` also has a
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
    if start < bytes.count, bytes[start] == asciiHash {
        var consumed = 2  // Go's `i`, counting the "&" and the "#"
        var cursor = start + 1
        var hex = false
        if cursor < bytes.count, bytes[cursor] == asciiLowerX || bytes[cursor] == asciiUpperX {
            hex = true
            cursor += 1
            consumed += 1
        }

        var value: Int32 = 0
        while cursor < bytes.count {
            let c = bytes[cursor]
            cursor += 1
            consumed += 1
            if let digit = asciiDigitValue(c, hex: hex) {
                value = value &* (hex ? 16 : 10) &+ digit
                continue
            }
            if c != asciiSemicolon {
                consumed -= 1
                cursor -= 1
            }
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
        return (Array(String(scalar).utf8), cursor)
    }

    // Named. A name is alphanumeric — `emsp13` is one — and is matched against
    // the TABLE, longest entry first, rather than by consuming the longest run
    // of name characters and demanding a semicolon after it. That distinction is
    // the whole of `&nbspBAh7…`: the run there is `nbspBAh`, which is not a
    // name, but `nbsp` is, and Go expands it.
    var cursor = start
    while cursor < bytes.count, isEntityNameByte(bytes[cursor]) { cursor += 1 }
    // Go's `entityName` carries the terminating semicolon when there is one, and
    // the table is keyed that way, so the lookup is one hash rather than a
    // lookup plus a rule about semicolons.
    var nameEnd = cursor
    if cursor < bytes.count, bytes[cursor] == asciiSemicolon { nameEnd = cursor + 1 }
    guard start < nameEnd else { return nil }

    if let replacement = entityTable[String(decoding: bytes[start..<nameEnd], as: UTF8.self)] {
        return (Array(replacement.utf8), nameEnd)
    }
    // On a miss, Go retries the name's own PREFIXES without a terminator,
    // longest first, bounded by `longestEntityWithoutSemicolon` — six, so a
    // forty-character run costs six hashes rather than forty. `&notit;` is the
    // shape: the name misses, `notit` misses, `noti` misses, `not` hits, and the
    // `it;` is left in the text.
    var length = nameEnd - start - 1
    if length > longestEntityWithoutSemicolon { length = longestEntityWithoutSemicolon }
    while length > 1 {
        if let replacement = entityTable[
            String(decoding: bytes[start..<(start + length)], as: UTF8.self)]
        {
            return (Array(replacement.utf8), start + length)
        }
        length -= 1
    }
    return nil
}

/// What may appear in a character reference's name: ASCII alphanumerics, as
/// `unescapeEntity` reads them.
private func isEntityNameByte(_ c: UInt8) -> Bool {
    switch c {
    case asciiZero...asciiNine, UInt8(ascii: "A")...UInt8(ascii: "Z"),
        UInt8(ascii: "a")...UInt8(ascii: "z"):
        return true
    default:
        return false
    }
}

/// What Go's `unicode.IsSpace` calls whitespace — the set `strings.TrimSpace`
/// uses, and so the set that decides whether a reference expanded at either end
/// of an sgid is erased before the decode. Module-wide, because the composite's
/// routing trims the same way and reached for `CharacterSet.whitespacesAndNewlines`
/// instead, which is this set PLUS U+200B.
///
/// Spelled out rather than taken from `CharacterSet.whitespacesAndNewlines`,
/// which is not the same set: it contains U+200B ZERO WIDTH SPACE, which Go does
/// not, so `&#8203;` before a payload was trimmed here and left in place there —
/// an sgid that named a person here and nobody in Go.
let goWhitespace: CharacterSet = {
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

/// Go's entity tables, as `html.UnescapeString` resolves them.
///
/// 2,229 rows, generated by running every name in `$GOROOT/src/html/entity.go`
/// through `html.UnescapeString` — through the FUNCTION rather than by parsing
/// the table, which is how the two-rune entries (`&fjlig;` is "fj") come out
/// right, and how `&nGt;` and `&nLt;` come to be ABSENT: they are in Go's source
/// and Go does not expand them.
///
/// This was once a 41-row subset, on the argument that every other reference
/// expands to something the base64 alphabet does not contain, so the envelope
/// fails to decode whether it was expanded or left verbatim — same answer,
/// reached differently. That is true of the DECODE and false of the write side:
/// `adding(_:to:)` compares the content's decoded sgid byte-for-byte against the
/// person's, and Go's expansion DELETES the reference's bytes where a narrower
/// table keeps them. `<payload>--&not` decodes to `<payload>--¬` in Go and to
/// itself here, so a mention Go recognises as already present was added a second
/// time. 194 of 212 measured shapes, one per name Go expands and the subset did
/// not.
///
/// The keys carry their own semicolon, exactly as Go's do, which is why both
/// `amp` and `amp;` appear: `unescapeEntity` looks up the name WITH its
/// terminator, and only falls back to a semicolon-less prefix when that misses.
///
/// Rows are terminated by U+0002 rather than by the newline they are written on,
/// and that is not a style choice. Two of Go's 2,229 expansions ARE control
/// characters — `&NewLine;` is U+000A and `&Tab;` is U+0009 — so a row-per-line
/// format cannot represent them: the value ends the row that carries it.
/// `entityTable["NewLine;"]` was the empty string for exactly that reason, and
/// the count came out 2,229 either way, so every count-based check passed while
/// the table was wrong. Spelling the value `\u{A}` does not help; it is the same
/// scalar. The format has to stop depending on the value.
let entityTable: [String: String] = {
    var table = [String: String](minimumCapacity: 2229)
    for row in entityTableRows.split(separator: "\u{2}", omittingEmptySubsequences: true) {
        // The newline the row is written on belongs to the layout, not to a name.
        let trimmed = row.drop(while: { $0 == "\n" })
        guard let separator = trimmed.firstIndex(of: "\u{1}") else { continue }
        table[String(trimmed[..<separator])] = String(trimmed[trimmed.index(after: separator)...])
    }
    return table
}()

/// The longest name Go expands without a terminating semicolon, which bounds the
/// prefix descent: `longestEntityWithoutSemicolon`.
private let longestEntityWithoutSemicolon = 6

private let entityTableRows = """
AElig\u{1}\u{C6}\u{2}
AElig;\u{1}\u{C6}\u{2}
AMP\u{1}&\u{2}
AMP;\u{1}&\u{2}
Aacute\u{1}\u{C1}\u{2}
Aacute;\u{1}\u{C1}\u{2}
Abreve;\u{1}\u{102}\u{2}
Acirc\u{1}\u{C2}\u{2}
Acirc;\u{1}\u{C2}\u{2}
Acy;\u{1}\u{410}\u{2}
Afr;\u{1}\u{1D504}\u{2}
Agrave\u{1}\u{C0}\u{2}
Agrave;\u{1}\u{C0}\u{2}
Alpha;\u{1}\u{391}\u{2}
Amacr;\u{1}\u{100}\u{2}
And;\u{1}\u{2A53}\u{2}
Aogon;\u{1}\u{104}\u{2}
Aopf;\u{1}\u{1D538}\u{2}
ApplyFunction;\u{1}\u{2061}\u{2}
Aring\u{1}\u{C5}\u{2}
Aring;\u{1}\u{C5}\u{2}
Ascr;\u{1}\u{1D49C}\u{2}
Assign;\u{1}\u{2254}\u{2}
Atilde\u{1}\u{C3}\u{2}
Atilde;\u{1}\u{C3}\u{2}
Auml\u{1}\u{C4}\u{2}
Auml;\u{1}\u{C4}\u{2}
Backslash;\u{1}\u{2216}\u{2}
Barv;\u{1}\u{2AE7}\u{2}
Barwed;\u{1}\u{2306}\u{2}
Bcy;\u{1}\u{411}\u{2}
Because;\u{1}\u{2235}\u{2}
Bernoullis;\u{1}\u{212C}\u{2}
Beta;\u{1}\u{392}\u{2}
Bfr;\u{1}\u{1D505}\u{2}
Bopf;\u{1}\u{1D539}\u{2}
Breve;\u{1}\u{2D8}\u{2}
Bscr;\u{1}\u{212C}\u{2}
Bumpeq;\u{1}\u{224E}\u{2}
CHcy;\u{1}\u{427}\u{2}
COPY\u{1}\u{A9}\u{2}
COPY;\u{1}\u{A9}\u{2}
Cacute;\u{1}\u{106}\u{2}
Cap;\u{1}\u{22D2}\u{2}
CapitalDifferentialD;\u{1}\u{2145}\u{2}
Cayleys;\u{1}\u{212D}\u{2}
Ccaron;\u{1}\u{10C}\u{2}
Ccedil\u{1}\u{C7}\u{2}
Ccedil;\u{1}\u{C7}\u{2}
Ccirc;\u{1}\u{108}\u{2}
Cconint;\u{1}\u{2230}\u{2}
Cdot;\u{1}\u{10A}\u{2}
Cedilla;\u{1}\u{B8}\u{2}
CenterDot;\u{1}\u{B7}\u{2}
Cfr;\u{1}\u{212D}\u{2}
Chi;\u{1}\u{3A7}\u{2}
CircleDot;\u{1}\u{2299}\u{2}
CircleMinus;\u{1}\u{2296}\u{2}
CirclePlus;\u{1}\u{2295}\u{2}
CircleTimes;\u{1}\u{2297}\u{2}
ClockwiseContourIntegral;\u{1}\u{2232}\u{2}
CloseCurlyDoubleQuote;\u{1}\u{201D}\u{2}
CloseCurlyQuote;\u{1}\u{2019}\u{2}
Colon;\u{1}\u{2237}\u{2}
Colone;\u{1}\u{2A74}\u{2}
Congruent;\u{1}\u{2261}\u{2}
Conint;\u{1}\u{222F}\u{2}
ContourIntegral;\u{1}\u{222E}\u{2}
Copf;\u{1}\u{2102}\u{2}
Coproduct;\u{1}\u{2210}\u{2}
CounterClockwiseContourIntegral;\u{1}\u{2233}\u{2}
Cross;\u{1}\u{2A2F}\u{2}
Cscr;\u{1}\u{1D49E}\u{2}
Cup;\u{1}\u{22D3}\u{2}
CupCap;\u{1}\u{224D}\u{2}
DD;\u{1}\u{2145}\u{2}
DDotrahd;\u{1}\u{2911}\u{2}
DJcy;\u{1}\u{402}\u{2}
DScy;\u{1}\u{405}\u{2}
DZcy;\u{1}\u{40F}\u{2}
Dagger;\u{1}\u{2021}\u{2}
Darr;\u{1}\u{21A1}\u{2}
Dashv;\u{1}\u{2AE4}\u{2}
Dcaron;\u{1}\u{10E}\u{2}
Dcy;\u{1}\u{414}\u{2}
Del;\u{1}\u{2207}\u{2}
Delta;\u{1}\u{394}\u{2}
Dfr;\u{1}\u{1D507}\u{2}
DiacriticalAcute;\u{1}\u{B4}\u{2}
DiacriticalDot;\u{1}\u{2D9}\u{2}
DiacriticalDoubleAcute;\u{1}\u{2DD}\u{2}
DiacriticalGrave;\u{1}`\u{2}
DiacriticalTilde;\u{1}\u{2DC}\u{2}
Diamond;\u{1}\u{22C4}\u{2}
DifferentialD;\u{1}\u{2146}\u{2}
Dopf;\u{1}\u{1D53B}\u{2}
Dot;\u{1}\u{A8}\u{2}
DotDot;\u{1}\u{20DC}\u{2}
DotEqual;\u{1}\u{2250}\u{2}
DoubleContourIntegral;\u{1}\u{222F}\u{2}
DoubleDot;\u{1}\u{A8}\u{2}
DoubleDownArrow;\u{1}\u{21D3}\u{2}
DoubleLeftArrow;\u{1}\u{21D0}\u{2}
DoubleLeftRightArrow;\u{1}\u{21D4}\u{2}
DoubleLeftTee;\u{1}\u{2AE4}\u{2}
DoubleLongLeftArrow;\u{1}\u{27F8}\u{2}
DoubleLongLeftRightArrow;\u{1}\u{27FA}\u{2}
DoubleLongRightArrow;\u{1}\u{27F9}\u{2}
DoubleRightArrow;\u{1}\u{21D2}\u{2}
DoubleRightTee;\u{1}\u{22A8}\u{2}
DoubleUpArrow;\u{1}\u{21D1}\u{2}
DoubleUpDownArrow;\u{1}\u{21D5}\u{2}
DoubleVerticalBar;\u{1}\u{2225}\u{2}
DownArrow;\u{1}\u{2193}\u{2}
DownArrowBar;\u{1}\u{2913}\u{2}
DownArrowUpArrow;\u{1}\u{21F5}\u{2}
DownBreve;\u{1}\u{311}\u{2}
DownLeftRightVector;\u{1}\u{2950}\u{2}
DownLeftTeeVector;\u{1}\u{295E}\u{2}
DownLeftVector;\u{1}\u{21BD}\u{2}
DownLeftVectorBar;\u{1}\u{2956}\u{2}
DownRightTeeVector;\u{1}\u{295F}\u{2}
DownRightVector;\u{1}\u{21C1}\u{2}
DownRightVectorBar;\u{1}\u{2957}\u{2}
DownTee;\u{1}\u{22A4}\u{2}
DownTeeArrow;\u{1}\u{21A7}\u{2}
Downarrow;\u{1}\u{21D3}\u{2}
Dscr;\u{1}\u{1D49F}\u{2}
Dstrok;\u{1}\u{110}\u{2}
ENG;\u{1}\u{14A}\u{2}
ETH\u{1}\u{D0}\u{2}
ETH;\u{1}\u{D0}\u{2}
Eacute\u{1}\u{C9}\u{2}
Eacute;\u{1}\u{C9}\u{2}
Ecaron;\u{1}\u{11A}\u{2}
Ecirc\u{1}\u{CA}\u{2}
Ecirc;\u{1}\u{CA}\u{2}
Ecy;\u{1}\u{42D}\u{2}
Edot;\u{1}\u{116}\u{2}
Efr;\u{1}\u{1D508}\u{2}
Egrave\u{1}\u{C8}\u{2}
Egrave;\u{1}\u{C8}\u{2}
Element;\u{1}\u{2208}\u{2}
Emacr;\u{1}\u{112}\u{2}
EmptySmallSquare;\u{1}\u{25FB}\u{2}
EmptyVerySmallSquare;\u{1}\u{25AB}\u{2}
Eogon;\u{1}\u{118}\u{2}
Eopf;\u{1}\u{1D53C}\u{2}
Epsilon;\u{1}\u{395}\u{2}
Equal;\u{1}\u{2A75}\u{2}
EqualTilde;\u{1}\u{2242}\u{2}
Equilibrium;\u{1}\u{21CC}\u{2}
Escr;\u{1}\u{2130}\u{2}
Esim;\u{1}\u{2A73}\u{2}
Eta;\u{1}\u{397}\u{2}
Euml\u{1}\u{CB}\u{2}
Euml;\u{1}\u{CB}\u{2}
Exists;\u{1}\u{2203}\u{2}
ExponentialE;\u{1}\u{2147}\u{2}
Fcy;\u{1}\u{424}\u{2}
Ffr;\u{1}\u{1D509}\u{2}
FilledSmallSquare;\u{1}\u{25FC}\u{2}
FilledVerySmallSquare;\u{1}\u{25AA}\u{2}
Fopf;\u{1}\u{1D53D}\u{2}
ForAll;\u{1}\u{2200}\u{2}
Fouriertrf;\u{1}\u{2131}\u{2}
Fscr;\u{1}\u{2131}\u{2}
GJcy;\u{1}\u{403}\u{2}
GT\u{1}>\u{2}
GT;\u{1}>\u{2}
Gamma;\u{1}\u{393}\u{2}
Gammad;\u{1}\u{3DC}\u{2}
Gbreve;\u{1}\u{11E}\u{2}
Gcedil;\u{1}\u{122}\u{2}
Gcirc;\u{1}\u{11C}\u{2}
Gcy;\u{1}\u{413}\u{2}
Gdot;\u{1}\u{120}\u{2}
Gfr;\u{1}\u{1D50A}\u{2}
Gg;\u{1}\u{22D9}\u{2}
Gopf;\u{1}\u{1D53E}\u{2}
GreaterEqual;\u{1}\u{2265}\u{2}
GreaterEqualLess;\u{1}\u{22DB}\u{2}
GreaterFullEqual;\u{1}\u{2267}\u{2}
GreaterGreater;\u{1}\u{2AA2}\u{2}
GreaterLess;\u{1}\u{2277}\u{2}
GreaterSlantEqual;\u{1}\u{2A7E}\u{2}
GreaterTilde;\u{1}\u{2273}\u{2}
Gscr;\u{1}\u{1D4A2}\u{2}
Gt;\u{1}\u{226B}\u{2}
HARDcy;\u{1}\u{42A}\u{2}
Hacek;\u{1}\u{2C7}\u{2}
Hat;\u{1}^\u{2}
Hcirc;\u{1}\u{124}\u{2}
Hfr;\u{1}\u{210C}\u{2}
HilbertSpace;\u{1}\u{210B}\u{2}
Hopf;\u{1}\u{210D}\u{2}
HorizontalLine;\u{1}\u{2500}\u{2}
Hscr;\u{1}\u{210B}\u{2}
Hstrok;\u{1}\u{126}\u{2}
HumpDownHump;\u{1}\u{224E}\u{2}
HumpEqual;\u{1}\u{224F}\u{2}
IEcy;\u{1}\u{415}\u{2}
IJlig;\u{1}\u{132}\u{2}
IOcy;\u{1}\u{401}\u{2}
Iacute\u{1}\u{CD}\u{2}
Iacute;\u{1}\u{CD}\u{2}
Icirc\u{1}\u{CE}\u{2}
Icirc;\u{1}\u{CE}\u{2}
Icy;\u{1}\u{418}\u{2}
Idot;\u{1}\u{130}\u{2}
Ifr;\u{1}\u{2111}\u{2}
Igrave\u{1}\u{CC}\u{2}
Igrave;\u{1}\u{CC}\u{2}
Im;\u{1}\u{2111}\u{2}
Imacr;\u{1}\u{12A}\u{2}
ImaginaryI;\u{1}\u{2148}\u{2}
Implies;\u{1}\u{21D2}\u{2}
Int;\u{1}\u{222C}\u{2}
Integral;\u{1}\u{222B}\u{2}
Intersection;\u{1}\u{22C2}\u{2}
InvisibleComma;\u{1}\u{2063}\u{2}
InvisibleTimes;\u{1}\u{2062}\u{2}
Iogon;\u{1}\u{12E}\u{2}
Iopf;\u{1}\u{1D540}\u{2}
Iota;\u{1}\u{399}\u{2}
Iscr;\u{1}\u{2110}\u{2}
Itilde;\u{1}\u{128}\u{2}
Iukcy;\u{1}\u{406}\u{2}
Iuml\u{1}\u{CF}\u{2}
Iuml;\u{1}\u{CF}\u{2}
Jcirc;\u{1}\u{134}\u{2}
Jcy;\u{1}\u{419}\u{2}
Jfr;\u{1}\u{1D50D}\u{2}
Jopf;\u{1}\u{1D541}\u{2}
Jscr;\u{1}\u{1D4A5}\u{2}
Jsercy;\u{1}\u{408}\u{2}
Jukcy;\u{1}\u{404}\u{2}
KHcy;\u{1}\u{425}\u{2}
KJcy;\u{1}\u{40C}\u{2}
Kappa;\u{1}\u{39A}\u{2}
Kcedil;\u{1}\u{136}\u{2}
Kcy;\u{1}\u{41A}\u{2}
Kfr;\u{1}\u{1D50E}\u{2}
Kopf;\u{1}\u{1D542}\u{2}
Kscr;\u{1}\u{1D4A6}\u{2}
LJcy;\u{1}\u{409}\u{2}
LT\u{1}<\u{2}
LT;\u{1}<\u{2}
Lacute;\u{1}\u{139}\u{2}
Lambda;\u{1}\u{39B}\u{2}
Lang;\u{1}\u{27EA}\u{2}
Laplacetrf;\u{1}\u{2112}\u{2}
Larr;\u{1}\u{219E}\u{2}
Lcaron;\u{1}\u{13D}\u{2}
Lcedil;\u{1}\u{13B}\u{2}
Lcy;\u{1}\u{41B}\u{2}
LeftAngleBracket;\u{1}\u{27E8}\u{2}
LeftArrow;\u{1}\u{2190}\u{2}
LeftArrowBar;\u{1}\u{21E4}\u{2}
LeftArrowRightArrow;\u{1}\u{21C6}\u{2}
LeftCeiling;\u{1}\u{2308}\u{2}
LeftDoubleBracket;\u{1}\u{27E6}\u{2}
LeftDownTeeVector;\u{1}\u{2961}\u{2}
LeftDownVector;\u{1}\u{21C3}\u{2}
LeftDownVectorBar;\u{1}\u{2959}\u{2}
LeftFloor;\u{1}\u{230A}\u{2}
LeftRightArrow;\u{1}\u{2194}\u{2}
LeftRightVector;\u{1}\u{294E}\u{2}
LeftTee;\u{1}\u{22A3}\u{2}
LeftTeeArrow;\u{1}\u{21A4}\u{2}
LeftTeeVector;\u{1}\u{295A}\u{2}
LeftTriangle;\u{1}\u{22B2}\u{2}
LeftTriangleBar;\u{1}\u{29CF}\u{2}
LeftTriangleEqual;\u{1}\u{22B4}\u{2}
LeftUpDownVector;\u{1}\u{2951}\u{2}
LeftUpTeeVector;\u{1}\u{2960}\u{2}
LeftUpVector;\u{1}\u{21BF}\u{2}
LeftUpVectorBar;\u{1}\u{2958}\u{2}
LeftVector;\u{1}\u{21BC}\u{2}
LeftVectorBar;\u{1}\u{2952}\u{2}
Leftarrow;\u{1}\u{21D0}\u{2}
Leftrightarrow;\u{1}\u{21D4}\u{2}
LessEqualGreater;\u{1}\u{22DA}\u{2}
LessFullEqual;\u{1}\u{2266}\u{2}
LessGreater;\u{1}\u{2276}\u{2}
LessLess;\u{1}\u{2AA1}\u{2}
LessSlantEqual;\u{1}\u{2A7D}\u{2}
LessTilde;\u{1}\u{2272}\u{2}
Lfr;\u{1}\u{1D50F}\u{2}
Ll;\u{1}\u{22D8}\u{2}
Lleftarrow;\u{1}\u{21DA}\u{2}
Lmidot;\u{1}\u{13F}\u{2}
LongLeftArrow;\u{1}\u{27F5}\u{2}
LongLeftRightArrow;\u{1}\u{27F7}\u{2}
LongRightArrow;\u{1}\u{27F6}\u{2}
Longleftarrow;\u{1}\u{27F8}\u{2}
Longleftrightarrow;\u{1}\u{27FA}\u{2}
Longrightarrow;\u{1}\u{27F9}\u{2}
Lopf;\u{1}\u{1D543}\u{2}
LowerLeftArrow;\u{1}\u{2199}\u{2}
LowerRightArrow;\u{1}\u{2198}\u{2}
Lscr;\u{1}\u{2112}\u{2}
Lsh;\u{1}\u{21B0}\u{2}
Lstrok;\u{1}\u{141}\u{2}
Lt;\u{1}\u{226A}\u{2}
Map;\u{1}\u{2905}\u{2}
Mcy;\u{1}\u{41C}\u{2}
MediumSpace;\u{1}\u{205F}\u{2}
Mellintrf;\u{1}\u{2133}\u{2}
Mfr;\u{1}\u{1D510}\u{2}
MinusPlus;\u{1}\u{2213}\u{2}
Mopf;\u{1}\u{1D544}\u{2}
Mscr;\u{1}\u{2133}\u{2}
Mu;\u{1}\u{39C}\u{2}
NJcy;\u{1}\u{40A}\u{2}
Nacute;\u{1}\u{143}\u{2}
Ncaron;\u{1}\u{147}\u{2}
Ncedil;\u{1}\u{145}\u{2}
Ncy;\u{1}\u{41D}\u{2}
NegativeMediumSpace;\u{1}\u{200B}\u{2}
NegativeThickSpace;\u{1}\u{200B}\u{2}
NegativeThinSpace;\u{1}\u{200B}\u{2}
NegativeVeryThinSpace;\u{1}\u{200B}\u{2}
NestedGreaterGreater;\u{1}\u{226B}\u{2}
NestedLessLess;\u{1}\u{226A}\u{2}
NewLine;\u{1}\n\u{2}
Nfr;\u{1}\u{1D511}\u{2}
NoBreak;\u{1}\u{2060}\u{2}
NonBreakingSpace;\u{1}\u{A0}\u{2}
Nopf;\u{1}\u{2115}\u{2}
Not;\u{1}\u{2AEC}\u{2}
NotCongruent;\u{1}\u{2262}\u{2}
NotCupCap;\u{1}\u{226D}\u{2}
NotDoubleVerticalBar;\u{1}\u{2226}\u{2}
NotElement;\u{1}\u{2209}\u{2}
NotEqual;\u{1}\u{2260}\u{2}
NotEqualTilde;\u{1}\u{2242}\u{338}\u{2}
NotExists;\u{1}\u{2204}\u{2}
NotGreater;\u{1}\u{226F}\u{2}
NotGreaterEqual;\u{1}\u{2271}\u{2}
NotGreaterFullEqual;\u{1}\u{2267}\u{338}\u{2}
NotGreaterGreater;\u{1}\u{226B}\u{338}\u{2}
NotGreaterLess;\u{1}\u{2279}\u{2}
NotGreaterSlantEqual;\u{1}\u{2A7E}\u{338}\u{2}
NotGreaterTilde;\u{1}\u{2275}\u{2}
NotHumpDownHump;\u{1}\u{224E}\u{338}\u{2}
NotHumpEqual;\u{1}\u{224F}\u{338}\u{2}
NotLeftTriangle;\u{1}\u{22EA}\u{2}
NotLeftTriangleBar;\u{1}\u{29CF}\u{338}\u{2}
NotLeftTriangleEqual;\u{1}\u{22EC}\u{2}
NotLess;\u{1}\u{226E}\u{2}
NotLessEqual;\u{1}\u{2270}\u{2}
NotLessGreater;\u{1}\u{2278}\u{2}
NotLessLess;\u{1}\u{226A}\u{338}\u{2}
NotLessSlantEqual;\u{1}\u{2A7D}\u{338}\u{2}
NotLessTilde;\u{1}\u{2274}\u{2}
NotNestedGreaterGreater;\u{1}\u{2AA2}\u{338}\u{2}
NotNestedLessLess;\u{1}\u{2AA1}\u{338}\u{2}
NotPrecedes;\u{1}\u{2280}\u{2}
NotPrecedesEqual;\u{1}\u{2AAF}\u{338}\u{2}
NotPrecedesSlantEqual;\u{1}\u{22E0}\u{2}
NotReverseElement;\u{1}\u{220C}\u{2}
NotRightTriangle;\u{1}\u{22EB}\u{2}
NotRightTriangleBar;\u{1}\u{29D0}\u{338}\u{2}
NotRightTriangleEqual;\u{1}\u{22ED}\u{2}
NotSquareSubset;\u{1}\u{228F}\u{338}\u{2}
NotSquareSubsetEqual;\u{1}\u{22E2}\u{2}
NotSquareSuperset;\u{1}\u{2290}\u{338}\u{2}
NotSquareSupersetEqual;\u{1}\u{22E3}\u{2}
NotSubset;\u{1}\u{2282}\u{20D2}\u{2}
NotSubsetEqual;\u{1}\u{2288}\u{2}
NotSucceeds;\u{1}\u{2281}\u{2}
NotSucceedsEqual;\u{1}\u{2AB0}\u{338}\u{2}
NotSucceedsSlantEqual;\u{1}\u{22E1}\u{2}
NotSucceedsTilde;\u{1}\u{227F}\u{338}\u{2}
NotSuperset;\u{1}\u{2283}\u{20D2}\u{2}
NotSupersetEqual;\u{1}\u{2289}\u{2}
NotTilde;\u{1}\u{2241}\u{2}
NotTildeEqual;\u{1}\u{2244}\u{2}
NotTildeFullEqual;\u{1}\u{2247}\u{2}
NotTildeTilde;\u{1}\u{2249}\u{2}
NotVerticalBar;\u{1}\u{2224}\u{2}
Nscr;\u{1}\u{1D4A9}\u{2}
Ntilde\u{1}\u{D1}\u{2}
Ntilde;\u{1}\u{D1}\u{2}
Nu;\u{1}\u{39D}\u{2}
OElig;\u{1}\u{152}\u{2}
Oacute\u{1}\u{D3}\u{2}
Oacute;\u{1}\u{D3}\u{2}
Ocirc\u{1}\u{D4}\u{2}
Ocirc;\u{1}\u{D4}\u{2}
Ocy;\u{1}\u{41E}\u{2}
Odblac;\u{1}\u{150}\u{2}
Ofr;\u{1}\u{1D512}\u{2}
Ograve\u{1}\u{D2}\u{2}
Ograve;\u{1}\u{D2}\u{2}
Omacr;\u{1}\u{14C}\u{2}
Omega;\u{1}\u{3A9}\u{2}
Omicron;\u{1}\u{39F}\u{2}
Oopf;\u{1}\u{1D546}\u{2}
OpenCurlyDoubleQuote;\u{1}\u{201C}\u{2}
OpenCurlyQuote;\u{1}\u{2018}\u{2}
Or;\u{1}\u{2A54}\u{2}
Oscr;\u{1}\u{1D4AA}\u{2}
Oslash\u{1}\u{D8}\u{2}
Oslash;\u{1}\u{D8}\u{2}
Otilde\u{1}\u{D5}\u{2}
Otilde;\u{1}\u{D5}\u{2}
Otimes;\u{1}\u{2A37}\u{2}
Ouml\u{1}\u{D6}\u{2}
Ouml;\u{1}\u{D6}\u{2}
OverBar;\u{1}\u{203E}\u{2}
OverBrace;\u{1}\u{23DE}\u{2}
OverBracket;\u{1}\u{23B4}\u{2}
OverParenthesis;\u{1}\u{23DC}\u{2}
PartialD;\u{1}\u{2202}\u{2}
Pcy;\u{1}\u{41F}\u{2}
Pfr;\u{1}\u{1D513}\u{2}
Phi;\u{1}\u{3A6}\u{2}
Pi;\u{1}\u{3A0}\u{2}
PlusMinus;\u{1}\u{B1}\u{2}
Poincareplane;\u{1}\u{210C}\u{2}
Popf;\u{1}\u{2119}\u{2}
Pr;\u{1}\u{2ABB}\u{2}
Precedes;\u{1}\u{227A}\u{2}
PrecedesEqual;\u{1}\u{2AAF}\u{2}
PrecedesSlantEqual;\u{1}\u{227C}\u{2}
PrecedesTilde;\u{1}\u{227E}\u{2}
Prime;\u{1}\u{2033}\u{2}
Product;\u{1}\u{220F}\u{2}
Proportion;\u{1}\u{2237}\u{2}
Proportional;\u{1}\u{221D}\u{2}
Pscr;\u{1}\u{1D4AB}\u{2}
Psi;\u{1}\u{3A8}\u{2}
QUOT\u{1}\"\u{2}
QUOT;\u{1}\"\u{2}
Qfr;\u{1}\u{1D514}\u{2}
Qopf;\u{1}\u{211A}\u{2}
Qscr;\u{1}\u{1D4AC}\u{2}
RBarr;\u{1}\u{2910}\u{2}
REG\u{1}\u{AE}\u{2}
REG;\u{1}\u{AE}\u{2}
Racute;\u{1}\u{154}\u{2}
Rang;\u{1}\u{27EB}\u{2}
Rarr;\u{1}\u{21A0}\u{2}
Rarrtl;\u{1}\u{2916}\u{2}
Rcaron;\u{1}\u{158}\u{2}
Rcedil;\u{1}\u{156}\u{2}
Rcy;\u{1}\u{420}\u{2}
Re;\u{1}\u{211C}\u{2}
ReverseElement;\u{1}\u{220B}\u{2}
ReverseEquilibrium;\u{1}\u{21CB}\u{2}
ReverseUpEquilibrium;\u{1}\u{296F}\u{2}
Rfr;\u{1}\u{211C}\u{2}
Rho;\u{1}\u{3A1}\u{2}
RightAngleBracket;\u{1}\u{27E9}\u{2}
RightArrow;\u{1}\u{2192}\u{2}
RightArrowBar;\u{1}\u{21E5}\u{2}
RightArrowLeftArrow;\u{1}\u{21C4}\u{2}
RightCeiling;\u{1}\u{2309}\u{2}
RightDoubleBracket;\u{1}\u{27E7}\u{2}
RightDownTeeVector;\u{1}\u{295D}\u{2}
RightDownVector;\u{1}\u{21C2}\u{2}
RightDownVectorBar;\u{1}\u{2955}\u{2}
RightFloor;\u{1}\u{230B}\u{2}
RightTee;\u{1}\u{22A2}\u{2}
RightTeeArrow;\u{1}\u{21A6}\u{2}
RightTeeVector;\u{1}\u{295B}\u{2}
RightTriangle;\u{1}\u{22B3}\u{2}
RightTriangleBar;\u{1}\u{29D0}\u{2}
RightTriangleEqual;\u{1}\u{22B5}\u{2}
RightUpDownVector;\u{1}\u{294F}\u{2}
RightUpTeeVector;\u{1}\u{295C}\u{2}
RightUpVector;\u{1}\u{21BE}\u{2}
RightUpVectorBar;\u{1}\u{2954}\u{2}
RightVector;\u{1}\u{21C0}\u{2}
RightVectorBar;\u{1}\u{2953}\u{2}
Rightarrow;\u{1}\u{21D2}\u{2}
Ropf;\u{1}\u{211D}\u{2}
RoundImplies;\u{1}\u{2970}\u{2}
Rrightarrow;\u{1}\u{21DB}\u{2}
Rscr;\u{1}\u{211B}\u{2}
Rsh;\u{1}\u{21B1}\u{2}
RuleDelayed;\u{1}\u{29F4}\u{2}
SHCHcy;\u{1}\u{429}\u{2}
SHcy;\u{1}\u{428}\u{2}
SOFTcy;\u{1}\u{42C}\u{2}
Sacute;\u{1}\u{15A}\u{2}
Sc;\u{1}\u{2ABC}\u{2}
Scaron;\u{1}\u{160}\u{2}
Scedil;\u{1}\u{15E}\u{2}
Scirc;\u{1}\u{15C}\u{2}
Scy;\u{1}\u{421}\u{2}
Sfr;\u{1}\u{1D516}\u{2}
ShortDownArrow;\u{1}\u{2193}\u{2}
ShortLeftArrow;\u{1}\u{2190}\u{2}
ShortRightArrow;\u{1}\u{2192}\u{2}
ShortUpArrow;\u{1}\u{2191}\u{2}
Sigma;\u{1}\u{3A3}\u{2}
SmallCircle;\u{1}\u{2218}\u{2}
Sopf;\u{1}\u{1D54A}\u{2}
Sqrt;\u{1}\u{221A}\u{2}
Square;\u{1}\u{25A1}\u{2}
SquareIntersection;\u{1}\u{2293}\u{2}
SquareSubset;\u{1}\u{228F}\u{2}
SquareSubsetEqual;\u{1}\u{2291}\u{2}
SquareSuperset;\u{1}\u{2290}\u{2}
SquareSupersetEqual;\u{1}\u{2292}\u{2}
SquareUnion;\u{1}\u{2294}\u{2}
Sscr;\u{1}\u{1D4AE}\u{2}
Star;\u{1}\u{22C6}\u{2}
Sub;\u{1}\u{22D0}\u{2}
Subset;\u{1}\u{22D0}\u{2}
SubsetEqual;\u{1}\u{2286}\u{2}
Succeeds;\u{1}\u{227B}\u{2}
SucceedsEqual;\u{1}\u{2AB0}\u{2}
SucceedsSlantEqual;\u{1}\u{227D}\u{2}
SucceedsTilde;\u{1}\u{227F}\u{2}
SuchThat;\u{1}\u{220B}\u{2}
Sum;\u{1}\u{2211}\u{2}
Sup;\u{1}\u{22D1}\u{2}
Superset;\u{1}\u{2283}\u{2}
SupersetEqual;\u{1}\u{2287}\u{2}
Supset;\u{1}\u{22D1}\u{2}
THORN\u{1}\u{DE}\u{2}
THORN;\u{1}\u{DE}\u{2}
TRADE;\u{1}\u{2122}\u{2}
TSHcy;\u{1}\u{40B}\u{2}
TScy;\u{1}\u{426}\u{2}
Tab;\u{1}\t\u{2}
Tau;\u{1}\u{3A4}\u{2}
Tcaron;\u{1}\u{164}\u{2}
Tcedil;\u{1}\u{162}\u{2}
Tcy;\u{1}\u{422}\u{2}
Tfr;\u{1}\u{1D517}\u{2}
Therefore;\u{1}\u{2234}\u{2}
Theta;\u{1}\u{398}\u{2}
ThickSpace;\u{1}\u{205F}\u{200A}\u{2}
ThinSpace;\u{1}\u{2009}\u{2}
Tilde;\u{1}\u{223C}\u{2}
TildeEqual;\u{1}\u{2243}\u{2}
TildeFullEqual;\u{1}\u{2245}\u{2}
TildeTilde;\u{1}\u{2248}\u{2}
Topf;\u{1}\u{1D54B}\u{2}
TripleDot;\u{1}\u{20DB}\u{2}
Tscr;\u{1}\u{1D4AF}\u{2}
Tstrok;\u{1}\u{166}\u{2}
Uacute\u{1}\u{DA}\u{2}
Uacute;\u{1}\u{DA}\u{2}
Uarr;\u{1}\u{219F}\u{2}
Uarrocir;\u{1}\u{2949}\u{2}
Ubrcy;\u{1}\u{40E}\u{2}
Ubreve;\u{1}\u{16C}\u{2}
Ucirc\u{1}\u{DB}\u{2}
Ucirc;\u{1}\u{DB}\u{2}
Ucy;\u{1}\u{423}\u{2}
Udblac;\u{1}\u{170}\u{2}
Ufr;\u{1}\u{1D518}\u{2}
Ugrave\u{1}\u{D9}\u{2}
Ugrave;\u{1}\u{D9}\u{2}
Umacr;\u{1}\u{16A}\u{2}
UnderBar;\u{1}_\u{2}
UnderBrace;\u{1}\u{23DF}\u{2}
UnderBracket;\u{1}\u{23B5}\u{2}
UnderParenthesis;\u{1}\u{23DD}\u{2}
Union;\u{1}\u{22C3}\u{2}
UnionPlus;\u{1}\u{228E}\u{2}
Uogon;\u{1}\u{172}\u{2}
Uopf;\u{1}\u{1D54C}\u{2}
UpArrow;\u{1}\u{2191}\u{2}
UpArrowBar;\u{1}\u{2912}\u{2}
UpArrowDownArrow;\u{1}\u{21C5}\u{2}
UpDownArrow;\u{1}\u{2195}\u{2}
UpEquilibrium;\u{1}\u{296E}\u{2}
UpTee;\u{1}\u{22A5}\u{2}
UpTeeArrow;\u{1}\u{21A5}\u{2}
Uparrow;\u{1}\u{21D1}\u{2}
Updownarrow;\u{1}\u{21D5}\u{2}
UpperLeftArrow;\u{1}\u{2196}\u{2}
UpperRightArrow;\u{1}\u{2197}\u{2}
Upsi;\u{1}\u{3D2}\u{2}
Upsilon;\u{1}\u{3A5}\u{2}
Uring;\u{1}\u{16E}\u{2}
Uscr;\u{1}\u{1D4B0}\u{2}
Utilde;\u{1}\u{168}\u{2}
Uuml\u{1}\u{DC}\u{2}
Uuml;\u{1}\u{DC}\u{2}
VDash;\u{1}\u{22AB}\u{2}
Vbar;\u{1}\u{2AEB}\u{2}
Vcy;\u{1}\u{412}\u{2}
Vdash;\u{1}\u{22A9}\u{2}
Vdashl;\u{1}\u{2AE6}\u{2}
Vee;\u{1}\u{22C1}\u{2}
Verbar;\u{1}\u{2016}\u{2}
Vert;\u{1}\u{2016}\u{2}
VerticalBar;\u{1}\u{2223}\u{2}
VerticalLine;\u{1}|\u{2}
VerticalSeparator;\u{1}\u{2758}\u{2}
VerticalTilde;\u{1}\u{2240}\u{2}
VeryThinSpace;\u{1}\u{200A}\u{2}
Vfr;\u{1}\u{1D519}\u{2}
Vopf;\u{1}\u{1D54D}\u{2}
Vscr;\u{1}\u{1D4B1}\u{2}
Vvdash;\u{1}\u{22AA}\u{2}
Wcirc;\u{1}\u{174}\u{2}
Wedge;\u{1}\u{22C0}\u{2}
Wfr;\u{1}\u{1D51A}\u{2}
Wopf;\u{1}\u{1D54E}\u{2}
Wscr;\u{1}\u{1D4B2}\u{2}
Xfr;\u{1}\u{1D51B}\u{2}
Xi;\u{1}\u{39E}\u{2}
Xopf;\u{1}\u{1D54F}\u{2}
Xscr;\u{1}\u{1D4B3}\u{2}
YAcy;\u{1}\u{42F}\u{2}
YIcy;\u{1}\u{407}\u{2}
YUcy;\u{1}\u{42E}\u{2}
Yacute\u{1}\u{DD}\u{2}
Yacute;\u{1}\u{DD}\u{2}
Ycirc;\u{1}\u{176}\u{2}
Ycy;\u{1}\u{42B}\u{2}
Yfr;\u{1}\u{1D51C}\u{2}
Yopf;\u{1}\u{1D550}\u{2}
Yscr;\u{1}\u{1D4B4}\u{2}
Yuml;\u{1}\u{178}\u{2}
ZHcy;\u{1}\u{416}\u{2}
Zacute;\u{1}\u{179}\u{2}
Zcaron;\u{1}\u{17D}\u{2}
Zcy;\u{1}\u{417}\u{2}
Zdot;\u{1}\u{17B}\u{2}
ZeroWidthSpace;\u{1}\u{200B}\u{2}
Zeta;\u{1}\u{396}\u{2}
Zfr;\u{1}\u{2128}\u{2}
Zopf;\u{1}\u{2124}\u{2}
Zscr;\u{1}\u{1D4B5}\u{2}
aacute\u{1}\u{E1}\u{2}
aacute;\u{1}\u{E1}\u{2}
abreve;\u{1}\u{103}\u{2}
ac;\u{1}\u{223E}\u{2}
acE;\u{1}\u{223E}\u{333}\u{2}
acd;\u{1}\u{223F}\u{2}
acirc\u{1}\u{E2}\u{2}
acirc;\u{1}\u{E2}\u{2}
acute\u{1}\u{B4}\u{2}
acute;\u{1}\u{B4}\u{2}
acy;\u{1}\u{430}\u{2}
aelig\u{1}\u{E6}\u{2}
aelig;\u{1}\u{E6}\u{2}
af;\u{1}\u{2061}\u{2}
afr;\u{1}\u{1D51E}\u{2}
agrave\u{1}\u{E0}\u{2}
agrave;\u{1}\u{E0}\u{2}
alefsym;\u{1}\u{2135}\u{2}
aleph;\u{1}\u{2135}\u{2}
alpha;\u{1}\u{3B1}\u{2}
amacr;\u{1}\u{101}\u{2}
amalg;\u{1}\u{2A3F}\u{2}
amp\u{1}&\u{2}
amp;\u{1}&\u{2}
and;\u{1}\u{2227}\u{2}
andand;\u{1}\u{2A55}\u{2}
andd;\u{1}\u{2A5C}\u{2}
andslope;\u{1}\u{2A58}\u{2}
andv;\u{1}\u{2A5A}\u{2}
ang;\u{1}\u{2220}\u{2}
ange;\u{1}\u{29A4}\u{2}
angle;\u{1}\u{2220}\u{2}
angmsd;\u{1}\u{2221}\u{2}
angmsdaa;\u{1}\u{29A8}\u{2}
angmsdab;\u{1}\u{29A9}\u{2}
angmsdac;\u{1}\u{29AA}\u{2}
angmsdad;\u{1}\u{29AB}\u{2}
angmsdae;\u{1}\u{29AC}\u{2}
angmsdaf;\u{1}\u{29AD}\u{2}
angmsdag;\u{1}\u{29AE}\u{2}
angmsdah;\u{1}\u{29AF}\u{2}
angrt;\u{1}\u{221F}\u{2}
angrtvb;\u{1}\u{22BE}\u{2}
angrtvbd;\u{1}\u{299D}\u{2}
angsph;\u{1}\u{2222}\u{2}
angst;\u{1}\u{C5}\u{2}
angzarr;\u{1}\u{237C}\u{2}
aogon;\u{1}\u{105}\u{2}
aopf;\u{1}\u{1D552}\u{2}
ap;\u{1}\u{2248}\u{2}
apE;\u{1}\u{2A70}\u{2}
apacir;\u{1}\u{2A6F}\u{2}
ape;\u{1}\u{224A}\u{2}
apid;\u{1}\u{224B}\u{2}
apos;\u{1}'\u{2}
approx;\u{1}\u{2248}\u{2}
approxeq;\u{1}\u{224A}\u{2}
aring\u{1}\u{E5}\u{2}
aring;\u{1}\u{E5}\u{2}
ascr;\u{1}\u{1D4B6}\u{2}
ast;\u{1}*\u{2}
asymp;\u{1}\u{2248}\u{2}
asympeq;\u{1}\u{224D}\u{2}
atilde\u{1}\u{E3}\u{2}
atilde;\u{1}\u{E3}\u{2}
auml\u{1}\u{E4}\u{2}
auml;\u{1}\u{E4}\u{2}
awconint;\u{1}\u{2233}\u{2}
awint;\u{1}\u{2A11}\u{2}
bNot;\u{1}\u{2AED}\u{2}
backcong;\u{1}\u{224C}\u{2}
backepsilon;\u{1}\u{3F6}\u{2}
backprime;\u{1}\u{2035}\u{2}
backsim;\u{1}\u{223D}\u{2}
backsimeq;\u{1}\u{22CD}\u{2}
barvee;\u{1}\u{22BD}\u{2}
barwed;\u{1}\u{2305}\u{2}
barwedge;\u{1}\u{2305}\u{2}
bbrk;\u{1}\u{23B5}\u{2}
bbrktbrk;\u{1}\u{23B6}\u{2}
bcong;\u{1}\u{224C}\u{2}
bcy;\u{1}\u{431}\u{2}
bdquo;\u{1}\u{201E}\u{2}
becaus;\u{1}\u{2235}\u{2}
because;\u{1}\u{2235}\u{2}
bemptyv;\u{1}\u{29B0}\u{2}
bepsi;\u{1}\u{3F6}\u{2}
bernou;\u{1}\u{212C}\u{2}
beta;\u{1}\u{3B2}\u{2}
beth;\u{1}\u{2136}\u{2}
between;\u{1}\u{226C}\u{2}
bfr;\u{1}\u{1D51F}\u{2}
bigcap;\u{1}\u{22C2}\u{2}
bigcirc;\u{1}\u{25EF}\u{2}
bigcup;\u{1}\u{22C3}\u{2}
bigodot;\u{1}\u{2A00}\u{2}
bigoplus;\u{1}\u{2A01}\u{2}
bigotimes;\u{1}\u{2A02}\u{2}
bigsqcup;\u{1}\u{2A06}\u{2}
bigstar;\u{1}\u{2605}\u{2}
bigtriangledown;\u{1}\u{25BD}\u{2}
bigtriangleup;\u{1}\u{25B3}\u{2}
biguplus;\u{1}\u{2A04}\u{2}
bigvee;\u{1}\u{22C1}\u{2}
bigwedge;\u{1}\u{22C0}\u{2}
bkarow;\u{1}\u{290D}\u{2}
blacklozenge;\u{1}\u{29EB}\u{2}
blacksquare;\u{1}\u{25AA}\u{2}
blacktriangle;\u{1}\u{25B4}\u{2}
blacktriangledown;\u{1}\u{25BE}\u{2}
blacktriangleleft;\u{1}\u{25C2}\u{2}
blacktriangleright;\u{1}\u{25B8}\u{2}
blank;\u{1}\u{2423}\u{2}
blk12;\u{1}\u{2592}\u{2}
blk14;\u{1}\u{2591}\u{2}
blk34;\u{1}\u{2593}\u{2}
block;\u{1}\u{2588}\u{2}
bne;\u{1}=\u{20E5}\u{2}
bnequiv;\u{1}\u{2261}\u{20E5}\u{2}
bnot;\u{1}\u{2310}\u{2}
bopf;\u{1}\u{1D553}\u{2}
bot;\u{1}\u{22A5}\u{2}
bottom;\u{1}\u{22A5}\u{2}
bowtie;\u{1}\u{22C8}\u{2}
boxDL;\u{1}\u{2557}\u{2}
boxDR;\u{1}\u{2554}\u{2}
boxDl;\u{1}\u{2556}\u{2}
boxDr;\u{1}\u{2553}\u{2}
boxH;\u{1}\u{2550}\u{2}
boxHD;\u{1}\u{2566}\u{2}
boxHU;\u{1}\u{2569}\u{2}
boxHd;\u{1}\u{2564}\u{2}
boxHu;\u{1}\u{2567}\u{2}
boxUL;\u{1}\u{255D}\u{2}
boxUR;\u{1}\u{255A}\u{2}
boxUl;\u{1}\u{255C}\u{2}
boxUr;\u{1}\u{2559}\u{2}
boxV;\u{1}\u{2551}\u{2}
boxVH;\u{1}\u{256C}\u{2}
boxVL;\u{1}\u{2563}\u{2}
boxVR;\u{1}\u{2560}\u{2}
boxVh;\u{1}\u{256B}\u{2}
boxVl;\u{1}\u{2562}\u{2}
boxVr;\u{1}\u{255F}\u{2}
boxbox;\u{1}\u{29C9}\u{2}
boxdL;\u{1}\u{2555}\u{2}
boxdR;\u{1}\u{2552}\u{2}
boxdl;\u{1}\u{2510}\u{2}
boxdr;\u{1}\u{250C}\u{2}
boxh;\u{1}\u{2500}\u{2}
boxhD;\u{1}\u{2565}\u{2}
boxhU;\u{1}\u{2568}\u{2}
boxhd;\u{1}\u{252C}\u{2}
boxhu;\u{1}\u{2534}\u{2}
boxminus;\u{1}\u{229F}\u{2}
boxplus;\u{1}\u{229E}\u{2}
boxtimes;\u{1}\u{22A0}\u{2}
boxuL;\u{1}\u{255B}\u{2}
boxuR;\u{1}\u{2558}\u{2}
boxul;\u{1}\u{2518}\u{2}
boxur;\u{1}\u{2514}\u{2}
boxv;\u{1}\u{2502}\u{2}
boxvH;\u{1}\u{256A}\u{2}
boxvL;\u{1}\u{2561}\u{2}
boxvR;\u{1}\u{255E}\u{2}
boxvh;\u{1}\u{253C}\u{2}
boxvl;\u{1}\u{2524}\u{2}
boxvr;\u{1}\u{251C}\u{2}
bprime;\u{1}\u{2035}\u{2}
breve;\u{1}\u{2D8}\u{2}
brvbar\u{1}\u{A6}\u{2}
brvbar;\u{1}\u{A6}\u{2}
bscr;\u{1}\u{1D4B7}\u{2}
bsemi;\u{1}\u{204F}\u{2}
bsim;\u{1}\u{223D}\u{2}
bsime;\u{1}\u{22CD}\u{2}
bsol;\u{1}\\\u{2}
bsolb;\u{1}\u{29C5}\u{2}
bsolhsub;\u{1}\u{27C8}\u{2}
bull;\u{1}\u{2022}\u{2}
bullet;\u{1}\u{2022}\u{2}
bump;\u{1}\u{224E}\u{2}
bumpE;\u{1}\u{2AAE}\u{2}
bumpe;\u{1}\u{224F}\u{2}
bumpeq;\u{1}\u{224F}\u{2}
cacute;\u{1}\u{107}\u{2}
cap;\u{1}\u{2229}\u{2}
capand;\u{1}\u{2A44}\u{2}
capbrcup;\u{1}\u{2A49}\u{2}
capcap;\u{1}\u{2A4B}\u{2}
capcup;\u{1}\u{2A47}\u{2}
capdot;\u{1}\u{2A40}\u{2}
caps;\u{1}\u{2229}\u{FE00}\u{2}
caret;\u{1}\u{2041}\u{2}
caron;\u{1}\u{2C7}\u{2}
ccaps;\u{1}\u{2A4D}\u{2}
ccaron;\u{1}\u{10D}\u{2}
ccedil\u{1}\u{E7}\u{2}
ccedil;\u{1}\u{E7}\u{2}
ccirc;\u{1}\u{109}\u{2}
ccups;\u{1}\u{2A4C}\u{2}
ccupssm;\u{1}\u{2A50}\u{2}
cdot;\u{1}\u{10B}\u{2}
cedil\u{1}\u{B8}\u{2}
cedil;\u{1}\u{B8}\u{2}
cemptyv;\u{1}\u{29B2}\u{2}
cent\u{1}\u{A2}\u{2}
cent;\u{1}\u{A2}\u{2}
centerdot;\u{1}\u{B7}\u{2}
cfr;\u{1}\u{1D520}\u{2}
chcy;\u{1}\u{447}\u{2}
check;\u{1}\u{2713}\u{2}
checkmark;\u{1}\u{2713}\u{2}
chi;\u{1}\u{3C7}\u{2}
cir;\u{1}\u{25CB}\u{2}
cirE;\u{1}\u{29C3}\u{2}
circ;\u{1}\u{2C6}\u{2}
circeq;\u{1}\u{2257}\u{2}
circlearrowleft;\u{1}\u{21BA}\u{2}
circlearrowright;\u{1}\u{21BB}\u{2}
circledR;\u{1}\u{AE}\u{2}
circledS;\u{1}\u{24C8}\u{2}
circledast;\u{1}\u{229B}\u{2}
circledcirc;\u{1}\u{229A}\u{2}
circleddash;\u{1}\u{229D}\u{2}
cire;\u{1}\u{2257}\u{2}
cirfnint;\u{1}\u{2A10}\u{2}
cirmid;\u{1}\u{2AEF}\u{2}
cirscir;\u{1}\u{29C2}\u{2}
clubs;\u{1}\u{2663}\u{2}
clubsuit;\u{1}\u{2663}\u{2}
colon;\u{1}:\u{2}
colone;\u{1}\u{2254}\u{2}
coloneq;\u{1}\u{2254}\u{2}
comma;\u{1},\u{2}
commat;\u{1}@\u{2}
comp;\u{1}\u{2201}\u{2}
compfn;\u{1}\u{2218}\u{2}
complement;\u{1}\u{2201}\u{2}
complexes;\u{1}\u{2102}\u{2}
cong;\u{1}\u{2245}\u{2}
congdot;\u{1}\u{2A6D}\u{2}
conint;\u{1}\u{222E}\u{2}
copf;\u{1}\u{1D554}\u{2}
coprod;\u{1}\u{2210}\u{2}
copy\u{1}\u{A9}\u{2}
copy;\u{1}\u{A9}\u{2}
copysr;\u{1}\u{2117}\u{2}
crarr;\u{1}\u{21B5}\u{2}
cross;\u{1}\u{2717}\u{2}
cscr;\u{1}\u{1D4B8}\u{2}
csub;\u{1}\u{2ACF}\u{2}
csube;\u{1}\u{2AD1}\u{2}
csup;\u{1}\u{2AD0}\u{2}
csupe;\u{1}\u{2AD2}\u{2}
ctdot;\u{1}\u{22EF}\u{2}
cudarrl;\u{1}\u{2938}\u{2}
cudarrr;\u{1}\u{2935}\u{2}
cuepr;\u{1}\u{22DE}\u{2}
cuesc;\u{1}\u{22DF}\u{2}
cularr;\u{1}\u{21B6}\u{2}
cularrp;\u{1}\u{293D}\u{2}
cup;\u{1}\u{222A}\u{2}
cupbrcap;\u{1}\u{2A48}\u{2}
cupcap;\u{1}\u{2A46}\u{2}
cupcup;\u{1}\u{2A4A}\u{2}
cupdot;\u{1}\u{228D}\u{2}
cupor;\u{1}\u{2A45}\u{2}
cups;\u{1}\u{222A}\u{FE00}\u{2}
curarr;\u{1}\u{21B7}\u{2}
curarrm;\u{1}\u{293C}\u{2}
curlyeqprec;\u{1}\u{22DE}\u{2}
curlyeqsucc;\u{1}\u{22DF}\u{2}
curlyvee;\u{1}\u{22CE}\u{2}
curlywedge;\u{1}\u{22CF}\u{2}
curren\u{1}\u{A4}\u{2}
curren;\u{1}\u{A4}\u{2}
curvearrowleft;\u{1}\u{21B6}\u{2}
curvearrowright;\u{1}\u{21B7}\u{2}
cuvee;\u{1}\u{22CE}\u{2}
cuwed;\u{1}\u{22CF}\u{2}
cwconint;\u{1}\u{2232}\u{2}
cwint;\u{1}\u{2231}\u{2}
cylcty;\u{1}\u{232D}\u{2}
dArr;\u{1}\u{21D3}\u{2}
dHar;\u{1}\u{2965}\u{2}
dagger;\u{1}\u{2020}\u{2}
daleth;\u{1}\u{2138}\u{2}
darr;\u{1}\u{2193}\u{2}
dash;\u{1}\u{2010}\u{2}
dashv;\u{1}\u{22A3}\u{2}
dbkarow;\u{1}\u{290F}\u{2}
dblac;\u{1}\u{2DD}\u{2}
dcaron;\u{1}\u{10F}\u{2}
dcy;\u{1}\u{434}\u{2}
dd;\u{1}\u{2146}\u{2}
ddagger;\u{1}\u{2021}\u{2}
ddarr;\u{1}\u{21CA}\u{2}
ddotseq;\u{1}\u{2A77}\u{2}
deg\u{1}\u{B0}\u{2}
deg;\u{1}\u{B0}\u{2}
delta;\u{1}\u{3B4}\u{2}
demptyv;\u{1}\u{29B1}\u{2}
dfisht;\u{1}\u{297F}\u{2}
dfr;\u{1}\u{1D521}\u{2}
dharl;\u{1}\u{21C3}\u{2}
dharr;\u{1}\u{21C2}\u{2}
diam;\u{1}\u{22C4}\u{2}
diamond;\u{1}\u{22C4}\u{2}
diamondsuit;\u{1}\u{2666}\u{2}
diams;\u{1}\u{2666}\u{2}
die;\u{1}\u{A8}\u{2}
digamma;\u{1}\u{3DD}\u{2}
disin;\u{1}\u{22F2}\u{2}
div;\u{1}\u{F7}\u{2}
divide\u{1}\u{F7}\u{2}
divide;\u{1}\u{F7}\u{2}
divideontimes;\u{1}\u{22C7}\u{2}
divonx;\u{1}\u{22C7}\u{2}
djcy;\u{1}\u{452}\u{2}
dlcorn;\u{1}\u{231E}\u{2}
dlcrop;\u{1}\u{230D}\u{2}
dollar;\u{1}$\u{2}
dopf;\u{1}\u{1D555}\u{2}
dot;\u{1}\u{2D9}\u{2}
doteq;\u{1}\u{2250}\u{2}
doteqdot;\u{1}\u{2251}\u{2}
dotminus;\u{1}\u{2238}\u{2}
dotplus;\u{1}\u{2214}\u{2}
dotsquare;\u{1}\u{22A1}\u{2}
doublebarwedge;\u{1}\u{2306}\u{2}
downarrow;\u{1}\u{2193}\u{2}
downdownarrows;\u{1}\u{21CA}\u{2}
downharpoonleft;\u{1}\u{21C3}\u{2}
downharpoonright;\u{1}\u{21C2}\u{2}
drbkarow;\u{1}\u{2910}\u{2}
drcorn;\u{1}\u{231F}\u{2}
drcrop;\u{1}\u{230C}\u{2}
dscr;\u{1}\u{1D4B9}\u{2}
dscy;\u{1}\u{455}\u{2}
dsol;\u{1}\u{29F6}\u{2}
dstrok;\u{1}\u{111}\u{2}
dtdot;\u{1}\u{22F1}\u{2}
dtri;\u{1}\u{25BF}\u{2}
dtrif;\u{1}\u{25BE}\u{2}
duarr;\u{1}\u{21F5}\u{2}
duhar;\u{1}\u{296F}\u{2}
dwangle;\u{1}\u{29A6}\u{2}
dzcy;\u{1}\u{45F}\u{2}
dzigrarr;\u{1}\u{27FF}\u{2}
eDDot;\u{1}\u{2A77}\u{2}
eDot;\u{1}\u{2251}\u{2}
eacute\u{1}\u{E9}\u{2}
eacute;\u{1}\u{E9}\u{2}
easter;\u{1}\u{2A6E}\u{2}
ecaron;\u{1}\u{11B}\u{2}
ecir;\u{1}\u{2256}\u{2}
ecirc\u{1}\u{EA}\u{2}
ecirc;\u{1}\u{EA}\u{2}
ecolon;\u{1}\u{2255}\u{2}
ecy;\u{1}\u{44D}\u{2}
edot;\u{1}\u{117}\u{2}
ee;\u{1}\u{2147}\u{2}
efDot;\u{1}\u{2252}\u{2}
efr;\u{1}\u{1D522}\u{2}
eg;\u{1}\u{2A9A}\u{2}
egrave\u{1}\u{E8}\u{2}
egrave;\u{1}\u{E8}\u{2}
egs;\u{1}\u{2A96}\u{2}
egsdot;\u{1}\u{2A98}\u{2}
el;\u{1}\u{2A99}\u{2}
elinters;\u{1}\u{23E7}\u{2}
ell;\u{1}\u{2113}\u{2}
els;\u{1}\u{2A95}\u{2}
elsdot;\u{1}\u{2A97}\u{2}
emacr;\u{1}\u{113}\u{2}
empty;\u{1}\u{2205}\u{2}
emptyset;\u{1}\u{2205}\u{2}
emptyv;\u{1}\u{2205}\u{2}
emsp13;\u{1}\u{2004}\u{2}
emsp14;\u{1}\u{2005}\u{2}
emsp;\u{1}\u{2003}\u{2}
eng;\u{1}\u{14B}\u{2}
ensp;\u{1}\u{2002}\u{2}
eogon;\u{1}\u{119}\u{2}
eopf;\u{1}\u{1D556}\u{2}
epar;\u{1}\u{22D5}\u{2}
eparsl;\u{1}\u{29E3}\u{2}
eplus;\u{1}\u{2A71}\u{2}
epsi;\u{1}\u{3B5}\u{2}
epsilon;\u{1}\u{3B5}\u{2}
epsiv;\u{1}\u{3F5}\u{2}
eqcirc;\u{1}\u{2256}\u{2}
eqcolon;\u{1}\u{2255}\u{2}
eqsim;\u{1}\u{2242}\u{2}
eqslantgtr;\u{1}\u{2A96}\u{2}
eqslantless;\u{1}\u{2A95}\u{2}
equals;\u{1}=\u{2}
equest;\u{1}\u{225F}\u{2}
equiv;\u{1}\u{2261}\u{2}
equivDD;\u{1}\u{2A78}\u{2}
eqvparsl;\u{1}\u{29E5}\u{2}
erDot;\u{1}\u{2253}\u{2}
erarr;\u{1}\u{2971}\u{2}
escr;\u{1}\u{212F}\u{2}
esdot;\u{1}\u{2250}\u{2}
esim;\u{1}\u{2242}\u{2}
eta;\u{1}\u{3B7}\u{2}
eth\u{1}\u{F0}\u{2}
eth;\u{1}\u{F0}\u{2}
euml\u{1}\u{EB}\u{2}
euml;\u{1}\u{EB}\u{2}
euro;\u{1}\u{20AC}\u{2}
excl;\u{1}!\u{2}
exist;\u{1}\u{2203}\u{2}
expectation;\u{1}\u{2130}\u{2}
exponentiale;\u{1}\u{2147}\u{2}
fallingdotseq;\u{1}\u{2252}\u{2}
fcy;\u{1}\u{444}\u{2}
female;\u{1}\u{2640}\u{2}
ffilig;\u{1}\u{FB03}\u{2}
fflig;\u{1}\u{FB00}\u{2}
ffllig;\u{1}\u{FB04}\u{2}
ffr;\u{1}\u{1D523}\u{2}
filig;\u{1}\u{FB01}\u{2}
fjlig;\u{1}fj\u{2}
flat;\u{1}\u{266D}\u{2}
fllig;\u{1}\u{FB02}\u{2}
fltns;\u{1}\u{25B1}\u{2}
fnof;\u{1}\u{192}\u{2}
fopf;\u{1}\u{1D557}\u{2}
forall;\u{1}\u{2200}\u{2}
fork;\u{1}\u{22D4}\u{2}
forkv;\u{1}\u{2AD9}\u{2}
fpartint;\u{1}\u{2A0D}\u{2}
frac12\u{1}\u{BD}\u{2}
frac12;\u{1}\u{BD}\u{2}
frac13;\u{1}\u{2153}\u{2}
frac14\u{1}\u{BC}\u{2}
frac14;\u{1}\u{BC}\u{2}
frac15;\u{1}\u{2155}\u{2}
frac16;\u{1}\u{2159}\u{2}
frac18;\u{1}\u{215B}\u{2}
frac23;\u{1}\u{2154}\u{2}
frac25;\u{1}\u{2156}\u{2}
frac34\u{1}\u{BE}\u{2}
frac34;\u{1}\u{BE}\u{2}
frac35;\u{1}\u{2157}\u{2}
frac38;\u{1}\u{215C}\u{2}
frac45;\u{1}\u{2158}\u{2}
frac56;\u{1}\u{215A}\u{2}
frac58;\u{1}\u{215D}\u{2}
frac78;\u{1}\u{215E}\u{2}
frasl;\u{1}\u{2044}\u{2}
frown;\u{1}\u{2322}\u{2}
fscr;\u{1}\u{1D4BB}\u{2}
gE;\u{1}\u{2267}\u{2}
gEl;\u{1}\u{2A8C}\u{2}
gacute;\u{1}\u{1F5}\u{2}
gamma;\u{1}\u{3B3}\u{2}
gammad;\u{1}\u{3DD}\u{2}
gap;\u{1}\u{2A86}\u{2}
gbreve;\u{1}\u{11F}\u{2}
gcirc;\u{1}\u{11D}\u{2}
gcy;\u{1}\u{433}\u{2}
gdot;\u{1}\u{121}\u{2}
ge;\u{1}\u{2265}\u{2}
gel;\u{1}\u{22DB}\u{2}
geq;\u{1}\u{2265}\u{2}
geqq;\u{1}\u{2267}\u{2}
geqslant;\u{1}\u{2A7E}\u{2}
ges;\u{1}\u{2A7E}\u{2}
gescc;\u{1}\u{2AA9}\u{2}
gesdot;\u{1}\u{2A80}\u{2}
gesdoto;\u{1}\u{2A82}\u{2}
gesdotol;\u{1}\u{2A84}\u{2}
gesl;\u{1}\u{22DB}\u{FE00}\u{2}
gesles;\u{1}\u{2A94}\u{2}
gfr;\u{1}\u{1D524}\u{2}
gg;\u{1}\u{226B}\u{2}
ggg;\u{1}\u{22D9}\u{2}
gimel;\u{1}\u{2137}\u{2}
gjcy;\u{1}\u{453}\u{2}
gl;\u{1}\u{2277}\u{2}
glE;\u{1}\u{2A92}\u{2}
gla;\u{1}\u{2AA5}\u{2}
glj;\u{1}\u{2AA4}\u{2}
gnE;\u{1}\u{2269}\u{2}
gnap;\u{1}\u{2A8A}\u{2}
gnapprox;\u{1}\u{2A8A}\u{2}
gne;\u{1}\u{2A88}\u{2}
gneq;\u{1}\u{2A88}\u{2}
gneqq;\u{1}\u{2269}\u{2}
gnsim;\u{1}\u{22E7}\u{2}
gopf;\u{1}\u{1D558}\u{2}
grave;\u{1}`\u{2}
gscr;\u{1}\u{210A}\u{2}
gsim;\u{1}\u{2273}\u{2}
gsime;\u{1}\u{2A8E}\u{2}
gsiml;\u{1}\u{2A90}\u{2}
gt\u{1}>\u{2}
gt;\u{1}>\u{2}
gtcc;\u{1}\u{2AA7}\u{2}
gtcir;\u{1}\u{2A7A}\u{2}
gtdot;\u{1}\u{22D7}\u{2}
gtlPar;\u{1}\u{2995}\u{2}
gtquest;\u{1}\u{2A7C}\u{2}
gtrapprox;\u{1}\u{2A86}\u{2}
gtrarr;\u{1}\u{2978}\u{2}
gtrdot;\u{1}\u{22D7}\u{2}
gtreqless;\u{1}\u{22DB}\u{2}
gtreqqless;\u{1}\u{2A8C}\u{2}
gtrless;\u{1}\u{2277}\u{2}
gtrsim;\u{1}\u{2273}\u{2}
gvertneqq;\u{1}\u{2269}\u{FE00}\u{2}
gvnE;\u{1}\u{2269}\u{FE00}\u{2}
hArr;\u{1}\u{21D4}\u{2}
hairsp;\u{1}\u{200A}\u{2}
half;\u{1}\u{BD}\u{2}
hamilt;\u{1}\u{210B}\u{2}
hardcy;\u{1}\u{44A}\u{2}
harr;\u{1}\u{2194}\u{2}
harrcir;\u{1}\u{2948}\u{2}
harrw;\u{1}\u{21AD}\u{2}
hbar;\u{1}\u{210F}\u{2}
hcirc;\u{1}\u{125}\u{2}
hearts;\u{1}\u{2665}\u{2}
heartsuit;\u{1}\u{2665}\u{2}
hellip;\u{1}\u{2026}\u{2}
hercon;\u{1}\u{22B9}\u{2}
hfr;\u{1}\u{1D525}\u{2}
hksearow;\u{1}\u{2925}\u{2}
hkswarow;\u{1}\u{2926}\u{2}
hoarr;\u{1}\u{21FF}\u{2}
homtht;\u{1}\u{223B}\u{2}
hookleftarrow;\u{1}\u{21A9}\u{2}
hookrightarrow;\u{1}\u{21AA}\u{2}
hopf;\u{1}\u{1D559}\u{2}
horbar;\u{1}\u{2015}\u{2}
hscr;\u{1}\u{1D4BD}\u{2}
hslash;\u{1}\u{210F}\u{2}
hstrok;\u{1}\u{127}\u{2}
hybull;\u{1}\u{2043}\u{2}
hyphen;\u{1}\u{2010}\u{2}
iacute\u{1}\u{ED}\u{2}
iacute;\u{1}\u{ED}\u{2}
ic;\u{1}\u{2063}\u{2}
icirc\u{1}\u{EE}\u{2}
icirc;\u{1}\u{EE}\u{2}
icy;\u{1}\u{438}\u{2}
iecy;\u{1}\u{435}\u{2}
iexcl\u{1}\u{A1}\u{2}
iexcl;\u{1}\u{A1}\u{2}
iff;\u{1}\u{21D4}\u{2}
ifr;\u{1}\u{1D526}\u{2}
igrave\u{1}\u{EC}\u{2}
igrave;\u{1}\u{EC}\u{2}
ii;\u{1}\u{2148}\u{2}
iiiint;\u{1}\u{2A0C}\u{2}
iiint;\u{1}\u{222D}\u{2}
iinfin;\u{1}\u{29DC}\u{2}
iiota;\u{1}\u{2129}\u{2}
ijlig;\u{1}\u{133}\u{2}
imacr;\u{1}\u{12B}\u{2}
image;\u{1}\u{2111}\u{2}
imagline;\u{1}\u{2110}\u{2}
imagpart;\u{1}\u{2111}\u{2}
imath;\u{1}\u{131}\u{2}
imof;\u{1}\u{22B7}\u{2}
imped;\u{1}\u{1B5}\u{2}
in;\u{1}\u{2208}\u{2}
incare;\u{1}\u{2105}\u{2}
infin;\u{1}\u{221E}\u{2}
infintie;\u{1}\u{29DD}\u{2}
inodot;\u{1}\u{131}\u{2}
int;\u{1}\u{222B}\u{2}
intcal;\u{1}\u{22BA}\u{2}
integers;\u{1}\u{2124}\u{2}
intercal;\u{1}\u{22BA}\u{2}
intlarhk;\u{1}\u{2A17}\u{2}
intprod;\u{1}\u{2A3C}\u{2}
iocy;\u{1}\u{451}\u{2}
iogon;\u{1}\u{12F}\u{2}
iopf;\u{1}\u{1D55A}\u{2}
iota;\u{1}\u{3B9}\u{2}
iprod;\u{1}\u{2A3C}\u{2}
iquest\u{1}\u{BF}\u{2}
iquest;\u{1}\u{BF}\u{2}
iscr;\u{1}\u{1D4BE}\u{2}
isin;\u{1}\u{2208}\u{2}
isinE;\u{1}\u{22F9}\u{2}
isindot;\u{1}\u{22F5}\u{2}
isins;\u{1}\u{22F4}\u{2}
isinsv;\u{1}\u{22F3}\u{2}
isinv;\u{1}\u{2208}\u{2}
it;\u{1}\u{2062}\u{2}
itilde;\u{1}\u{129}\u{2}
iukcy;\u{1}\u{456}\u{2}
iuml\u{1}\u{EF}\u{2}
iuml;\u{1}\u{EF}\u{2}
jcirc;\u{1}\u{135}\u{2}
jcy;\u{1}\u{439}\u{2}
jfr;\u{1}\u{1D527}\u{2}
jmath;\u{1}\u{237}\u{2}
jopf;\u{1}\u{1D55B}\u{2}
jscr;\u{1}\u{1D4BF}\u{2}
jsercy;\u{1}\u{458}\u{2}
jukcy;\u{1}\u{454}\u{2}
kappa;\u{1}\u{3BA}\u{2}
kappav;\u{1}\u{3F0}\u{2}
kcedil;\u{1}\u{137}\u{2}
kcy;\u{1}\u{43A}\u{2}
kfr;\u{1}\u{1D528}\u{2}
kgreen;\u{1}\u{138}\u{2}
khcy;\u{1}\u{445}\u{2}
kjcy;\u{1}\u{45C}\u{2}
kopf;\u{1}\u{1D55C}\u{2}
kscr;\u{1}\u{1D4C0}\u{2}
lAarr;\u{1}\u{21DA}\u{2}
lArr;\u{1}\u{21D0}\u{2}
lAtail;\u{1}\u{291B}\u{2}
lBarr;\u{1}\u{290E}\u{2}
lE;\u{1}\u{2266}\u{2}
lEg;\u{1}\u{2A8B}\u{2}
lHar;\u{1}\u{2962}\u{2}
lacute;\u{1}\u{13A}\u{2}
laemptyv;\u{1}\u{29B4}\u{2}
lagran;\u{1}\u{2112}\u{2}
lambda;\u{1}\u{3BB}\u{2}
lang;\u{1}\u{27E8}\u{2}
langd;\u{1}\u{2991}\u{2}
langle;\u{1}\u{27E8}\u{2}
lap;\u{1}\u{2A85}\u{2}
laquo\u{1}\u{AB}\u{2}
laquo;\u{1}\u{AB}\u{2}
larr;\u{1}\u{2190}\u{2}
larrb;\u{1}\u{21E4}\u{2}
larrbfs;\u{1}\u{291F}\u{2}
larrfs;\u{1}\u{291D}\u{2}
larrhk;\u{1}\u{21A9}\u{2}
larrlp;\u{1}\u{21AB}\u{2}
larrpl;\u{1}\u{2939}\u{2}
larrsim;\u{1}\u{2973}\u{2}
larrtl;\u{1}\u{21A2}\u{2}
lat;\u{1}\u{2AAB}\u{2}
latail;\u{1}\u{2919}\u{2}
late;\u{1}\u{2AAD}\u{2}
lates;\u{1}\u{2AAD}\u{FE00}\u{2}
lbarr;\u{1}\u{290C}\u{2}
lbbrk;\u{1}\u{2772}\u{2}
lbrace;\u{1}{\u{2}
lbrack;\u{1}[\u{2}
lbrke;\u{1}\u{298B}\u{2}
lbrksld;\u{1}\u{298F}\u{2}
lbrkslu;\u{1}\u{298D}\u{2}
lcaron;\u{1}\u{13E}\u{2}
lcedil;\u{1}\u{13C}\u{2}
lceil;\u{1}\u{2308}\u{2}
lcub;\u{1}{\u{2}
lcy;\u{1}\u{43B}\u{2}
ldca;\u{1}\u{2936}\u{2}
ldquo;\u{1}\u{201C}\u{2}
ldquor;\u{1}\u{201E}\u{2}
ldrdhar;\u{1}\u{2967}\u{2}
ldrushar;\u{1}\u{294B}\u{2}
ldsh;\u{1}\u{21B2}\u{2}
le;\u{1}\u{2264}\u{2}
leftarrow;\u{1}\u{2190}\u{2}
leftarrowtail;\u{1}\u{21A2}\u{2}
leftharpoondown;\u{1}\u{21BD}\u{2}
leftharpoonup;\u{1}\u{21BC}\u{2}
leftleftarrows;\u{1}\u{21C7}\u{2}
leftrightarrow;\u{1}\u{2194}\u{2}
leftrightarrows;\u{1}\u{21C6}\u{2}
leftrightharpoons;\u{1}\u{21CB}\u{2}
leftrightsquigarrow;\u{1}\u{21AD}\u{2}
leftthreetimes;\u{1}\u{22CB}\u{2}
leg;\u{1}\u{22DA}\u{2}
leq;\u{1}\u{2264}\u{2}
leqq;\u{1}\u{2266}\u{2}
leqslant;\u{1}\u{2A7D}\u{2}
les;\u{1}\u{2A7D}\u{2}
lescc;\u{1}\u{2AA8}\u{2}
lesdot;\u{1}\u{2A7F}\u{2}
lesdoto;\u{1}\u{2A81}\u{2}
lesdotor;\u{1}\u{2A83}\u{2}
lesg;\u{1}\u{22DA}\u{FE00}\u{2}
lesges;\u{1}\u{2A93}\u{2}
lessapprox;\u{1}\u{2A85}\u{2}
lessdot;\u{1}\u{22D6}\u{2}
lesseqgtr;\u{1}\u{22DA}\u{2}
lesseqqgtr;\u{1}\u{2A8B}\u{2}
lessgtr;\u{1}\u{2276}\u{2}
lesssim;\u{1}\u{2272}\u{2}
lfisht;\u{1}\u{297C}\u{2}
lfloor;\u{1}\u{230A}\u{2}
lfr;\u{1}\u{1D529}\u{2}
lg;\u{1}\u{2276}\u{2}
lgE;\u{1}\u{2A91}\u{2}
lhard;\u{1}\u{21BD}\u{2}
lharu;\u{1}\u{21BC}\u{2}
lharul;\u{1}\u{296A}\u{2}
lhblk;\u{1}\u{2584}\u{2}
ljcy;\u{1}\u{459}\u{2}
ll;\u{1}\u{226A}\u{2}
llarr;\u{1}\u{21C7}\u{2}
llcorner;\u{1}\u{231E}\u{2}
llhard;\u{1}\u{296B}\u{2}
lltri;\u{1}\u{25FA}\u{2}
lmidot;\u{1}\u{140}\u{2}
lmoust;\u{1}\u{23B0}\u{2}
lmoustache;\u{1}\u{23B0}\u{2}
lnE;\u{1}\u{2268}\u{2}
lnap;\u{1}\u{2A89}\u{2}
lnapprox;\u{1}\u{2A89}\u{2}
lne;\u{1}\u{2A87}\u{2}
lneq;\u{1}\u{2A87}\u{2}
lneqq;\u{1}\u{2268}\u{2}
lnsim;\u{1}\u{22E6}\u{2}
loang;\u{1}\u{27EC}\u{2}
loarr;\u{1}\u{21FD}\u{2}
lobrk;\u{1}\u{27E6}\u{2}
longleftarrow;\u{1}\u{27F5}\u{2}
longleftrightarrow;\u{1}\u{27F7}\u{2}
longmapsto;\u{1}\u{27FC}\u{2}
longrightarrow;\u{1}\u{27F6}\u{2}
looparrowleft;\u{1}\u{21AB}\u{2}
looparrowright;\u{1}\u{21AC}\u{2}
lopar;\u{1}\u{2985}\u{2}
lopf;\u{1}\u{1D55D}\u{2}
loplus;\u{1}\u{2A2D}\u{2}
lotimes;\u{1}\u{2A34}\u{2}
lowast;\u{1}\u{2217}\u{2}
lowbar;\u{1}_\u{2}
loz;\u{1}\u{25CA}\u{2}
lozenge;\u{1}\u{25CA}\u{2}
lozf;\u{1}\u{29EB}\u{2}
lpar;\u{1}(\u{2}
lparlt;\u{1}\u{2993}\u{2}
lrarr;\u{1}\u{21C6}\u{2}
lrcorner;\u{1}\u{231F}\u{2}
lrhar;\u{1}\u{21CB}\u{2}
lrhard;\u{1}\u{296D}\u{2}
lrm;\u{1}\u{200E}\u{2}
lrtri;\u{1}\u{22BF}\u{2}
lsaquo;\u{1}\u{2039}\u{2}
lscr;\u{1}\u{1D4C1}\u{2}
lsh;\u{1}\u{21B0}\u{2}
lsim;\u{1}\u{2272}\u{2}
lsime;\u{1}\u{2A8D}\u{2}
lsimg;\u{1}\u{2A8F}\u{2}
lsqb;\u{1}[\u{2}
lsquo;\u{1}\u{2018}\u{2}
lsquor;\u{1}\u{201A}\u{2}
lstrok;\u{1}\u{142}\u{2}
lt\u{1}<\u{2}
lt;\u{1}<\u{2}
ltcc;\u{1}\u{2AA6}\u{2}
ltcir;\u{1}\u{2A79}\u{2}
ltdot;\u{1}\u{22D6}\u{2}
lthree;\u{1}\u{22CB}\u{2}
ltimes;\u{1}\u{22C9}\u{2}
ltlarr;\u{1}\u{2976}\u{2}
ltquest;\u{1}\u{2A7B}\u{2}
ltrPar;\u{1}\u{2996}\u{2}
ltri;\u{1}\u{25C3}\u{2}
ltrie;\u{1}\u{22B4}\u{2}
ltrif;\u{1}\u{25C2}\u{2}
lurdshar;\u{1}\u{294A}\u{2}
luruhar;\u{1}\u{2966}\u{2}
lvertneqq;\u{1}\u{2268}\u{FE00}\u{2}
lvnE;\u{1}\u{2268}\u{FE00}\u{2}
mDDot;\u{1}\u{223A}\u{2}
macr\u{1}\u{AF}\u{2}
macr;\u{1}\u{AF}\u{2}
male;\u{1}\u{2642}\u{2}
malt;\u{1}\u{2720}\u{2}
maltese;\u{1}\u{2720}\u{2}
map;\u{1}\u{21A6}\u{2}
mapsto;\u{1}\u{21A6}\u{2}
mapstodown;\u{1}\u{21A7}\u{2}
mapstoleft;\u{1}\u{21A4}\u{2}
mapstoup;\u{1}\u{21A5}\u{2}
marker;\u{1}\u{25AE}\u{2}
mcomma;\u{1}\u{2A29}\u{2}
mcy;\u{1}\u{43C}\u{2}
mdash;\u{1}\u{2014}\u{2}
measuredangle;\u{1}\u{2221}\u{2}
mfr;\u{1}\u{1D52A}\u{2}
mho;\u{1}\u{2127}\u{2}
micro\u{1}\u{B5}\u{2}
micro;\u{1}\u{B5}\u{2}
mid;\u{1}\u{2223}\u{2}
midast;\u{1}*\u{2}
midcir;\u{1}\u{2AF0}\u{2}
middot\u{1}\u{B7}\u{2}
middot;\u{1}\u{B7}\u{2}
minus;\u{1}\u{2212}\u{2}
minusb;\u{1}\u{229F}\u{2}
minusd;\u{1}\u{2238}\u{2}
minusdu;\u{1}\u{2A2A}\u{2}
mlcp;\u{1}\u{2ADB}\u{2}
mldr;\u{1}\u{2026}\u{2}
mnplus;\u{1}\u{2213}\u{2}
models;\u{1}\u{22A7}\u{2}
mopf;\u{1}\u{1D55E}\u{2}
mp;\u{1}\u{2213}\u{2}
mscr;\u{1}\u{1D4C2}\u{2}
mstpos;\u{1}\u{223E}\u{2}
mu;\u{1}\u{3BC}\u{2}
multimap;\u{1}\u{22B8}\u{2}
mumap;\u{1}\u{22B8}\u{2}
nGg;\u{1}\u{22D9}\u{338}\u{2}
nGtv;\u{1}\u{226B}\u{338}\u{2}
nLeftarrow;\u{1}\u{21CD}\u{2}
nLeftrightarrow;\u{1}\u{21CE}\u{2}
nLl;\u{1}\u{22D8}\u{338}\u{2}
nLtv;\u{1}\u{226A}\u{338}\u{2}
nRightarrow;\u{1}\u{21CF}\u{2}
nVDash;\u{1}\u{22AF}\u{2}
nVdash;\u{1}\u{22AE}\u{2}
nabla;\u{1}\u{2207}\u{2}
nacute;\u{1}\u{144}\u{2}
nang;\u{1}\u{2220}\u{20D2}\u{2}
nap;\u{1}\u{2249}\u{2}
napE;\u{1}\u{2A70}\u{338}\u{2}
napid;\u{1}\u{224B}\u{338}\u{2}
napos;\u{1}\u{149}\u{2}
napprox;\u{1}\u{2249}\u{2}
natur;\u{1}\u{266E}\u{2}
natural;\u{1}\u{266E}\u{2}
naturals;\u{1}\u{2115}\u{2}
nbsp\u{1}\u{A0}\u{2}
nbsp;\u{1}\u{A0}\u{2}
nbump;\u{1}\u{224E}\u{338}\u{2}
nbumpe;\u{1}\u{224F}\u{338}\u{2}
ncap;\u{1}\u{2A43}\u{2}
ncaron;\u{1}\u{148}\u{2}
ncedil;\u{1}\u{146}\u{2}
ncong;\u{1}\u{2247}\u{2}
ncongdot;\u{1}\u{2A6D}\u{338}\u{2}
ncup;\u{1}\u{2A42}\u{2}
ncy;\u{1}\u{43D}\u{2}
ndash;\u{1}\u{2013}\u{2}
ne;\u{1}\u{2260}\u{2}
neArr;\u{1}\u{21D7}\u{2}
nearhk;\u{1}\u{2924}\u{2}
nearr;\u{1}\u{2197}\u{2}
nearrow;\u{1}\u{2197}\u{2}
nedot;\u{1}\u{2250}\u{338}\u{2}
nequiv;\u{1}\u{2262}\u{2}
nesear;\u{1}\u{2928}\u{2}
nesim;\u{1}\u{2242}\u{338}\u{2}
nexist;\u{1}\u{2204}\u{2}
nexists;\u{1}\u{2204}\u{2}
nfr;\u{1}\u{1D52B}\u{2}
ngE;\u{1}\u{2267}\u{338}\u{2}
nge;\u{1}\u{2271}\u{2}
ngeq;\u{1}\u{2271}\u{2}
ngeqq;\u{1}\u{2267}\u{338}\u{2}
ngeqslant;\u{1}\u{2A7E}\u{338}\u{2}
nges;\u{1}\u{2A7E}\u{338}\u{2}
ngsim;\u{1}\u{2275}\u{2}
ngt;\u{1}\u{226F}\u{2}
ngtr;\u{1}\u{226F}\u{2}
nhArr;\u{1}\u{21CE}\u{2}
nharr;\u{1}\u{21AE}\u{2}
nhpar;\u{1}\u{2AF2}\u{2}
ni;\u{1}\u{220B}\u{2}
nis;\u{1}\u{22FC}\u{2}
nisd;\u{1}\u{22FA}\u{2}
niv;\u{1}\u{220B}\u{2}
njcy;\u{1}\u{45A}\u{2}
nlArr;\u{1}\u{21CD}\u{2}
nlE;\u{1}\u{2266}\u{338}\u{2}
nlarr;\u{1}\u{219A}\u{2}
nldr;\u{1}\u{2025}\u{2}
nle;\u{1}\u{2270}\u{2}
nleftarrow;\u{1}\u{219A}\u{2}
nleftrightarrow;\u{1}\u{21AE}\u{2}
nleq;\u{1}\u{2270}\u{2}
nleqq;\u{1}\u{2266}\u{338}\u{2}
nleqslant;\u{1}\u{2A7D}\u{338}\u{2}
nles;\u{1}\u{2A7D}\u{338}\u{2}
nless;\u{1}\u{226E}\u{2}
nlsim;\u{1}\u{2274}\u{2}
nlt;\u{1}\u{226E}\u{2}
nltri;\u{1}\u{22EA}\u{2}
nltrie;\u{1}\u{22EC}\u{2}
nmid;\u{1}\u{2224}\u{2}
nopf;\u{1}\u{1D55F}\u{2}
not\u{1}\u{AC}\u{2}
not;\u{1}\u{AC}\u{2}
notin;\u{1}\u{2209}\u{2}
notinE;\u{1}\u{22F9}\u{338}\u{2}
notindot;\u{1}\u{22F5}\u{338}\u{2}
notinva;\u{1}\u{2209}\u{2}
notinvb;\u{1}\u{22F7}\u{2}
notinvc;\u{1}\u{22F6}\u{2}
notni;\u{1}\u{220C}\u{2}
notniva;\u{1}\u{220C}\u{2}
notnivb;\u{1}\u{22FE}\u{2}
notnivc;\u{1}\u{22FD}\u{2}
npar;\u{1}\u{2226}\u{2}
nparallel;\u{1}\u{2226}\u{2}
nparsl;\u{1}\u{2AFD}\u{20E5}\u{2}
npart;\u{1}\u{2202}\u{338}\u{2}
npolint;\u{1}\u{2A14}\u{2}
npr;\u{1}\u{2280}\u{2}
nprcue;\u{1}\u{22E0}\u{2}
npre;\u{1}\u{2AAF}\u{338}\u{2}
nprec;\u{1}\u{2280}\u{2}
npreceq;\u{1}\u{2AAF}\u{338}\u{2}
nrArr;\u{1}\u{21CF}\u{2}
nrarr;\u{1}\u{219B}\u{2}
nrarrc;\u{1}\u{2933}\u{338}\u{2}
nrarrw;\u{1}\u{219D}\u{338}\u{2}
nrightarrow;\u{1}\u{219B}\u{2}
nrtri;\u{1}\u{22EB}\u{2}
nrtrie;\u{1}\u{22ED}\u{2}
nsc;\u{1}\u{2281}\u{2}
nsccue;\u{1}\u{22E1}\u{2}
nsce;\u{1}\u{2AB0}\u{338}\u{2}
nscr;\u{1}\u{1D4C3}\u{2}
nshortmid;\u{1}\u{2224}\u{2}
nshortparallel;\u{1}\u{2226}\u{2}
nsim;\u{1}\u{2241}\u{2}
nsime;\u{1}\u{2244}\u{2}
nsimeq;\u{1}\u{2244}\u{2}
nsmid;\u{1}\u{2224}\u{2}
nspar;\u{1}\u{2226}\u{2}
nsqsube;\u{1}\u{22E2}\u{2}
nsqsupe;\u{1}\u{22E3}\u{2}
nsub;\u{1}\u{2284}\u{2}
nsubE;\u{1}\u{2AC5}\u{338}\u{2}
nsube;\u{1}\u{2288}\u{2}
nsubset;\u{1}\u{2282}\u{20D2}\u{2}
nsubseteq;\u{1}\u{2288}\u{2}
nsubseteqq;\u{1}\u{2AC5}\u{338}\u{2}
nsucc;\u{1}\u{2281}\u{2}
nsucceq;\u{1}\u{2AB0}\u{338}\u{2}
nsup;\u{1}\u{2285}\u{2}
nsupE;\u{1}\u{2AC6}\u{338}\u{2}
nsupe;\u{1}\u{2289}\u{2}
nsupset;\u{1}\u{2283}\u{20D2}\u{2}
nsupseteq;\u{1}\u{2289}\u{2}
nsupseteqq;\u{1}\u{2AC6}\u{338}\u{2}
ntgl;\u{1}\u{2279}\u{2}
ntilde\u{1}\u{F1}\u{2}
ntilde;\u{1}\u{F1}\u{2}
ntlg;\u{1}\u{2278}\u{2}
ntriangleleft;\u{1}\u{22EA}\u{2}
ntrianglelefteq;\u{1}\u{22EC}\u{2}
ntriangleright;\u{1}\u{22EB}\u{2}
ntrianglerighteq;\u{1}\u{22ED}\u{2}
nu;\u{1}\u{3BD}\u{2}
num;\u{1}#\u{2}
numero;\u{1}\u{2116}\u{2}
numsp;\u{1}\u{2007}\u{2}
nvDash;\u{1}\u{22AD}\u{2}
nvHarr;\u{1}\u{2904}\u{2}
nvap;\u{1}\u{224D}\u{20D2}\u{2}
nvdash;\u{1}\u{22AC}\u{2}
nvge;\u{1}\u{2265}\u{20D2}\u{2}
nvgt;\u{1}>\u{20D2}\u{2}
nvinfin;\u{1}\u{29DE}\u{2}
nvlArr;\u{1}\u{2902}\u{2}
nvle;\u{1}\u{2264}\u{20D2}\u{2}
nvlt;\u{1}<\u{20D2}\u{2}
nvltrie;\u{1}\u{22B4}\u{20D2}\u{2}
nvrArr;\u{1}\u{2903}\u{2}
nvrtrie;\u{1}\u{22B5}\u{20D2}\u{2}
nvsim;\u{1}\u{223C}\u{20D2}\u{2}
nwArr;\u{1}\u{21D6}\u{2}
nwarhk;\u{1}\u{2923}\u{2}
nwarr;\u{1}\u{2196}\u{2}
nwarrow;\u{1}\u{2196}\u{2}
nwnear;\u{1}\u{2927}\u{2}
oS;\u{1}\u{24C8}\u{2}
oacute\u{1}\u{F3}\u{2}
oacute;\u{1}\u{F3}\u{2}
oast;\u{1}\u{229B}\u{2}
ocir;\u{1}\u{229A}\u{2}
ocirc\u{1}\u{F4}\u{2}
ocirc;\u{1}\u{F4}\u{2}
ocy;\u{1}\u{43E}\u{2}
odash;\u{1}\u{229D}\u{2}
odblac;\u{1}\u{151}\u{2}
odiv;\u{1}\u{2A38}\u{2}
odot;\u{1}\u{2299}\u{2}
odsold;\u{1}\u{29BC}\u{2}
oelig;\u{1}\u{153}\u{2}
ofcir;\u{1}\u{29BF}\u{2}
ofr;\u{1}\u{1D52C}\u{2}
ogon;\u{1}\u{2DB}\u{2}
ograve\u{1}\u{F2}\u{2}
ograve;\u{1}\u{F2}\u{2}
ogt;\u{1}\u{29C1}\u{2}
ohbar;\u{1}\u{29B5}\u{2}
ohm;\u{1}\u{3A9}\u{2}
oint;\u{1}\u{222E}\u{2}
olarr;\u{1}\u{21BA}\u{2}
olcir;\u{1}\u{29BE}\u{2}
olcross;\u{1}\u{29BB}\u{2}
oline;\u{1}\u{203E}\u{2}
olt;\u{1}\u{29C0}\u{2}
omacr;\u{1}\u{14D}\u{2}
omega;\u{1}\u{3C9}\u{2}
omicron;\u{1}\u{3BF}\u{2}
omid;\u{1}\u{29B6}\u{2}
ominus;\u{1}\u{2296}\u{2}
oopf;\u{1}\u{1D560}\u{2}
opar;\u{1}\u{29B7}\u{2}
operp;\u{1}\u{29B9}\u{2}
oplus;\u{1}\u{2295}\u{2}
or;\u{1}\u{2228}\u{2}
orarr;\u{1}\u{21BB}\u{2}
ord;\u{1}\u{2A5D}\u{2}
order;\u{1}\u{2134}\u{2}
orderof;\u{1}\u{2134}\u{2}
ordf\u{1}\u{AA}\u{2}
ordf;\u{1}\u{AA}\u{2}
ordm\u{1}\u{BA}\u{2}
ordm;\u{1}\u{BA}\u{2}
origof;\u{1}\u{22B6}\u{2}
oror;\u{1}\u{2A56}\u{2}
orslope;\u{1}\u{2A57}\u{2}
orv;\u{1}\u{2A5B}\u{2}
oscr;\u{1}\u{2134}\u{2}
oslash\u{1}\u{F8}\u{2}
oslash;\u{1}\u{F8}\u{2}
osol;\u{1}\u{2298}\u{2}
otilde\u{1}\u{F5}\u{2}
otilde;\u{1}\u{F5}\u{2}
otimes;\u{1}\u{2297}\u{2}
otimesas;\u{1}\u{2A36}\u{2}
ouml\u{1}\u{F6}\u{2}
ouml;\u{1}\u{F6}\u{2}
ovbar;\u{1}\u{233D}\u{2}
par;\u{1}\u{2225}\u{2}
para\u{1}\u{B6}\u{2}
para;\u{1}\u{B6}\u{2}
parallel;\u{1}\u{2225}\u{2}
parsim;\u{1}\u{2AF3}\u{2}
parsl;\u{1}\u{2AFD}\u{2}
part;\u{1}\u{2202}\u{2}
pcy;\u{1}\u{43F}\u{2}
percnt;\u{1}%\u{2}
period;\u{1}.\u{2}
permil;\u{1}\u{2030}\u{2}
perp;\u{1}\u{22A5}\u{2}
pertenk;\u{1}\u{2031}\u{2}
pfr;\u{1}\u{1D52D}\u{2}
phi;\u{1}\u{3C6}\u{2}
phiv;\u{1}\u{3D5}\u{2}
phmmat;\u{1}\u{2133}\u{2}
phone;\u{1}\u{260E}\u{2}
pi;\u{1}\u{3C0}\u{2}
pitchfork;\u{1}\u{22D4}\u{2}
piv;\u{1}\u{3D6}\u{2}
planck;\u{1}\u{210F}\u{2}
planckh;\u{1}\u{210E}\u{2}
plankv;\u{1}\u{210F}\u{2}
plus;\u{1}+\u{2}
plusacir;\u{1}\u{2A23}\u{2}
plusb;\u{1}\u{229E}\u{2}
pluscir;\u{1}\u{2A22}\u{2}
plusdo;\u{1}\u{2214}\u{2}
plusdu;\u{1}\u{2A25}\u{2}
pluse;\u{1}\u{2A72}\u{2}
plusmn\u{1}\u{B1}\u{2}
plusmn;\u{1}\u{B1}\u{2}
plussim;\u{1}\u{2A26}\u{2}
plustwo;\u{1}\u{2A27}\u{2}
pm;\u{1}\u{B1}\u{2}
pointint;\u{1}\u{2A15}\u{2}
popf;\u{1}\u{1D561}\u{2}
pound\u{1}\u{A3}\u{2}
pound;\u{1}\u{A3}\u{2}
pr;\u{1}\u{227A}\u{2}
prE;\u{1}\u{2AB3}\u{2}
prap;\u{1}\u{2AB7}\u{2}
prcue;\u{1}\u{227C}\u{2}
pre;\u{1}\u{2AAF}\u{2}
prec;\u{1}\u{227A}\u{2}
precapprox;\u{1}\u{2AB7}\u{2}
preccurlyeq;\u{1}\u{227C}\u{2}
preceq;\u{1}\u{2AAF}\u{2}
precnapprox;\u{1}\u{2AB9}\u{2}
precneqq;\u{1}\u{2AB5}\u{2}
precnsim;\u{1}\u{22E8}\u{2}
precsim;\u{1}\u{227E}\u{2}
prime;\u{1}\u{2032}\u{2}
primes;\u{1}\u{2119}\u{2}
prnE;\u{1}\u{2AB5}\u{2}
prnap;\u{1}\u{2AB9}\u{2}
prnsim;\u{1}\u{22E8}\u{2}
prod;\u{1}\u{220F}\u{2}
profalar;\u{1}\u{232E}\u{2}
profline;\u{1}\u{2312}\u{2}
profsurf;\u{1}\u{2313}\u{2}
prop;\u{1}\u{221D}\u{2}
propto;\u{1}\u{221D}\u{2}
prsim;\u{1}\u{227E}\u{2}
prurel;\u{1}\u{22B0}\u{2}
pscr;\u{1}\u{1D4C5}\u{2}
psi;\u{1}\u{3C8}\u{2}
puncsp;\u{1}\u{2008}\u{2}
qfr;\u{1}\u{1D52E}\u{2}
qint;\u{1}\u{2A0C}\u{2}
qopf;\u{1}\u{1D562}\u{2}
qprime;\u{1}\u{2057}\u{2}
qscr;\u{1}\u{1D4C6}\u{2}
quaternions;\u{1}\u{210D}\u{2}
quatint;\u{1}\u{2A16}\u{2}
quest;\u{1}?\u{2}
questeq;\u{1}\u{225F}\u{2}
quot\u{1}\"\u{2}
quot;\u{1}\"\u{2}
rAarr;\u{1}\u{21DB}\u{2}
rArr;\u{1}\u{21D2}\u{2}
rAtail;\u{1}\u{291C}\u{2}
rBarr;\u{1}\u{290F}\u{2}
rHar;\u{1}\u{2964}\u{2}
race;\u{1}\u{223D}\u{331}\u{2}
racute;\u{1}\u{155}\u{2}
radic;\u{1}\u{221A}\u{2}
raemptyv;\u{1}\u{29B3}\u{2}
rang;\u{1}\u{27E9}\u{2}
rangd;\u{1}\u{2992}\u{2}
range;\u{1}\u{29A5}\u{2}
rangle;\u{1}\u{27E9}\u{2}
raquo\u{1}\u{BB}\u{2}
raquo;\u{1}\u{BB}\u{2}
rarr;\u{1}\u{2192}\u{2}
rarrap;\u{1}\u{2975}\u{2}
rarrb;\u{1}\u{21E5}\u{2}
rarrbfs;\u{1}\u{2920}\u{2}
rarrc;\u{1}\u{2933}\u{2}
rarrfs;\u{1}\u{291E}\u{2}
rarrhk;\u{1}\u{21AA}\u{2}
rarrlp;\u{1}\u{21AC}\u{2}
rarrpl;\u{1}\u{2945}\u{2}
rarrsim;\u{1}\u{2974}\u{2}
rarrtl;\u{1}\u{21A3}\u{2}
rarrw;\u{1}\u{219D}\u{2}
ratail;\u{1}\u{291A}\u{2}
ratio;\u{1}\u{2236}\u{2}
rationals;\u{1}\u{211A}\u{2}
rbarr;\u{1}\u{290D}\u{2}
rbbrk;\u{1}\u{2773}\u{2}
rbrace;\u{1}}\u{2}
rbrack;\u{1}]\u{2}
rbrke;\u{1}\u{298C}\u{2}
rbrksld;\u{1}\u{298E}\u{2}
rbrkslu;\u{1}\u{2990}\u{2}
rcaron;\u{1}\u{159}\u{2}
rcedil;\u{1}\u{157}\u{2}
rceil;\u{1}\u{2309}\u{2}
rcub;\u{1}}\u{2}
rcy;\u{1}\u{440}\u{2}
rdca;\u{1}\u{2937}\u{2}
rdldhar;\u{1}\u{2969}\u{2}
rdquo;\u{1}\u{201D}\u{2}
rdquor;\u{1}\u{201D}\u{2}
rdsh;\u{1}\u{21B3}\u{2}
real;\u{1}\u{211C}\u{2}
realine;\u{1}\u{211B}\u{2}
realpart;\u{1}\u{211C}\u{2}
reals;\u{1}\u{211D}\u{2}
rect;\u{1}\u{25AD}\u{2}
reg\u{1}\u{AE}\u{2}
reg;\u{1}\u{AE}\u{2}
rfisht;\u{1}\u{297D}\u{2}
rfloor;\u{1}\u{230B}\u{2}
rfr;\u{1}\u{1D52F}\u{2}
rhard;\u{1}\u{21C1}\u{2}
rharu;\u{1}\u{21C0}\u{2}
rharul;\u{1}\u{296C}\u{2}
rho;\u{1}\u{3C1}\u{2}
rhov;\u{1}\u{3F1}\u{2}
rightarrow;\u{1}\u{2192}\u{2}
rightarrowtail;\u{1}\u{21A3}\u{2}
rightharpoondown;\u{1}\u{21C1}\u{2}
rightharpoonup;\u{1}\u{21C0}\u{2}
rightleftarrows;\u{1}\u{21C4}\u{2}
rightleftharpoons;\u{1}\u{21CC}\u{2}
rightrightarrows;\u{1}\u{21C9}\u{2}
rightsquigarrow;\u{1}\u{219D}\u{2}
rightthreetimes;\u{1}\u{22CC}\u{2}
ring;\u{1}\u{2DA}\u{2}
risingdotseq;\u{1}\u{2253}\u{2}
rlarr;\u{1}\u{21C4}\u{2}
rlhar;\u{1}\u{21CC}\u{2}
rlm;\u{1}\u{200F}\u{2}
rmoust;\u{1}\u{23B1}\u{2}
rmoustache;\u{1}\u{23B1}\u{2}
rnmid;\u{1}\u{2AEE}\u{2}
roang;\u{1}\u{27ED}\u{2}
roarr;\u{1}\u{21FE}\u{2}
robrk;\u{1}\u{27E7}\u{2}
ropar;\u{1}\u{2986}\u{2}
ropf;\u{1}\u{1D563}\u{2}
roplus;\u{1}\u{2A2E}\u{2}
rotimes;\u{1}\u{2A35}\u{2}
rpar;\u{1})\u{2}
rpargt;\u{1}\u{2994}\u{2}
rppolint;\u{1}\u{2A12}\u{2}
rrarr;\u{1}\u{21C9}\u{2}
rsaquo;\u{1}\u{203A}\u{2}
rscr;\u{1}\u{1D4C7}\u{2}
rsh;\u{1}\u{21B1}\u{2}
rsqb;\u{1}]\u{2}
rsquo;\u{1}\u{2019}\u{2}
rsquor;\u{1}\u{2019}\u{2}
rthree;\u{1}\u{22CC}\u{2}
rtimes;\u{1}\u{22CA}\u{2}
rtri;\u{1}\u{25B9}\u{2}
rtrie;\u{1}\u{22B5}\u{2}
rtrif;\u{1}\u{25B8}\u{2}
rtriltri;\u{1}\u{29CE}\u{2}
ruluhar;\u{1}\u{2968}\u{2}
rx;\u{1}\u{211E}\u{2}
sacute;\u{1}\u{15B}\u{2}
sbquo;\u{1}\u{201A}\u{2}
sc;\u{1}\u{227B}\u{2}
scE;\u{1}\u{2AB4}\u{2}
scap;\u{1}\u{2AB8}\u{2}
scaron;\u{1}\u{161}\u{2}
sccue;\u{1}\u{227D}\u{2}
sce;\u{1}\u{2AB0}\u{2}
scedil;\u{1}\u{15F}\u{2}
scirc;\u{1}\u{15D}\u{2}
scnE;\u{1}\u{2AB6}\u{2}
scnap;\u{1}\u{2ABA}\u{2}
scnsim;\u{1}\u{22E9}\u{2}
scpolint;\u{1}\u{2A13}\u{2}
scsim;\u{1}\u{227F}\u{2}
scy;\u{1}\u{441}\u{2}
sdot;\u{1}\u{22C5}\u{2}
sdotb;\u{1}\u{22A1}\u{2}
sdote;\u{1}\u{2A66}\u{2}
seArr;\u{1}\u{21D8}\u{2}
searhk;\u{1}\u{2925}\u{2}
searr;\u{1}\u{2198}\u{2}
searrow;\u{1}\u{2198}\u{2}
sect\u{1}\u{A7}\u{2}
sect;\u{1}\u{A7}\u{2}
semi;\u{1};\u{2}
seswar;\u{1}\u{2929}\u{2}
setminus;\u{1}\u{2216}\u{2}
setmn;\u{1}\u{2216}\u{2}
sext;\u{1}\u{2736}\u{2}
sfr;\u{1}\u{1D530}\u{2}
sfrown;\u{1}\u{2322}\u{2}
sharp;\u{1}\u{266F}\u{2}
shchcy;\u{1}\u{449}\u{2}
shcy;\u{1}\u{448}\u{2}
shortmid;\u{1}\u{2223}\u{2}
shortparallel;\u{1}\u{2225}\u{2}
shy\u{1}\u{AD}\u{2}
shy;\u{1}\u{AD}\u{2}
sigma;\u{1}\u{3C3}\u{2}
sigmaf;\u{1}\u{3C2}\u{2}
sigmav;\u{1}\u{3C2}\u{2}
sim;\u{1}\u{223C}\u{2}
simdot;\u{1}\u{2A6A}\u{2}
sime;\u{1}\u{2243}\u{2}
simeq;\u{1}\u{2243}\u{2}
simg;\u{1}\u{2A9E}\u{2}
simgE;\u{1}\u{2AA0}\u{2}
siml;\u{1}\u{2A9D}\u{2}
simlE;\u{1}\u{2A9F}\u{2}
simne;\u{1}\u{2246}\u{2}
simplus;\u{1}\u{2A24}\u{2}
simrarr;\u{1}\u{2972}\u{2}
slarr;\u{1}\u{2190}\u{2}
smallsetminus;\u{1}\u{2216}\u{2}
smashp;\u{1}\u{2A33}\u{2}
smeparsl;\u{1}\u{29E4}\u{2}
smid;\u{1}\u{2223}\u{2}
smile;\u{1}\u{2323}\u{2}
smt;\u{1}\u{2AAA}\u{2}
smte;\u{1}\u{2AAC}\u{2}
smtes;\u{1}\u{2AAC}\u{FE00}\u{2}
softcy;\u{1}\u{44C}\u{2}
sol;\u{1}/\u{2}
solb;\u{1}\u{29C4}\u{2}
solbar;\u{1}\u{233F}\u{2}
sopf;\u{1}\u{1D564}\u{2}
spades;\u{1}\u{2660}\u{2}
spadesuit;\u{1}\u{2660}\u{2}
spar;\u{1}\u{2225}\u{2}
sqcap;\u{1}\u{2293}\u{2}
sqcaps;\u{1}\u{2293}\u{FE00}\u{2}
sqcup;\u{1}\u{2294}\u{2}
sqcups;\u{1}\u{2294}\u{FE00}\u{2}
sqsub;\u{1}\u{228F}\u{2}
sqsube;\u{1}\u{2291}\u{2}
sqsubset;\u{1}\u{228F}\u{2}
sqsubseteq;\u{1}\u{2291}\u{2}
sqsup;\u{1}\u{2290}\u{2}
sqsupe;\u{1}\u{2292}\u{2}
sqsupset;\u{1}\u{2290}\u{2}
sqsupseteq;\u{1}\u{2292}\u{2}
squ;\u{1}\u{25A1}\u{2}
square;\u{1}\u{25A1}\u{2}
squarf;\u{1}\u{25AA}\u{2}
squf;\u{1}\u{25AA}\u{2}
srarr;\u{1}\u{2192}\u{2}
sscr;\u{1}\u{1D4C8}\u{2}
ssetmn;\u{1}\u{2216}\u{2}
ssmile;\u{1}\u{2323}\u{2}
sstarf;\u{1}\u{22C6}\u{2}
star;\u{1}\u{2606}\u{2}
starf;\u{1}\u{2605}\u{2}
straightepsilon;\u{1}\u{3F5}\u{2}
straightphi;\u{1}\u{3D5}\u{2}
strns;\u{1}\u{AF}\u{2}
sub;\u{1}\u{2282}\u{2}
subE;\u{1}\u{2AC5}\u{2}
subdot;\u{1}\u{2ABD}\u{2}
sube;\u{1}\u{2286}\u{2}
subedot;\u{1}\u{2AC3}\u{2}
submult;\u{1}\u{2AC1}\u{2}
subnE;\u{1}\u{2ACB}\u{2}
subne;\u{1}\u{228A}\u{2}
subplus;\u{1}\u{2ABF}\u{2}
subrarr;\u{1}\u{2979}\u{2}
subset;\u{1}\u{2282}\u{2}
subseteq;\u{1}\u{2286}\u{2}
subseteqq;\u{1}\u{2AC5}\u{2}
subsetneq;\u{1}\u{228A}\u{2}
subsetneqq;\u{1}\u{2ACB}\u{2}
subsim;\u{1}\u{2AC7}\u{2}
subsub;\u{1}\u{2AD5}\u{2}
subsup;\u{1}\u{2AD3}\u{2}
succ;\u{1}\u{227B}\u{2}
succapprox;\u{1}\u{2AB8}\u{2}
succcurlyeq;\u{1}\u{227D}\u{2}
succeq;\u{1}\u{2AB0}\u{2}
succnapprox;\u{1}\u{2ABA}\u{2}
succneqq;\u{1}\u{2AB6}\u{2}
succnsim;\u{1}\u{22E9}\u{2}
succsim;\u{1}\u{227F}\u{2}
sum;\u{1}\u{2211}\u{2}
sung;\u{1}\u{266A}\u{2}
sup1\u{1}\u{B9}\u{2}
sup1;\u{1}\u{B9}\u{2}
sup2\u{1}\u{B2}\u{2}
sup2;\u{1}\u{B2}\u{2}
sup3\u{1}\u{B3}\u{2}
sup3;\u{1}\u{B3}\u{2}
sup;\u{1}\u{2283}\u{2}
supE;\u{1}\u{2AC6}\u{2}
supdot;\u{1}\u{2ABE}\u{2}
supdsub;\u{1}\u{2AD8}\u{2}
supe;\u{1}\u{2287}\u{2}
supedot;\u{1}\u{2AC4}\u{2}
suphsol;\u{1}\u{27C9}\u{2}
suphsub;\u{1}\u{2AD7}\u{2}
suplarr;\u{1}\u{297B}\u{2}
supmult;\u{1}\u{2AC2}\u{2}
supnE;\u{1}\u{2ACC}\u{2}
supne;\u{1}\u{228B}\u{2}
supplus;\u{1}\u{2AC0}\u{2}
supset;\u{1}\u{2283}\u{2}
supseteq;\u{1}\u{2287}\u{2}
supseteqq;\u{1}\u{2AC6}\u{2}
supsetneq;\u{1}\u{228B}\u{2}
supsetneqq;\u{1}\u{2ACC}\u{2}
supsim;\u{1}\u{2AC8}\u{2}
supsub;\u{1}\u{2AD4}\u{2}
supsup;\u{1}\u{2AD6}\u{2}
swArr;\u{1}\u{21D9}\u{2}
swarhk;\u{1}\u{2926}\u{2}
swarr;\u{1}\u{2199}\u{2}
swarrow;\u{1}\u{2199}\u{2}
swnwar;\u{1}\u{292A}\u{2}
szlig\u{1}\u{DF}\u{2}
szlig;\u{1}\u{DF}\u{2}
target;\u{1}\u{2316}\u{2}
tau;\u{1}\u{3C4}\u{2}
tbrk;\u{1}\u{23B4}\u{2}
tcaron;\u{1}\u{165}\u{2}
tcedil;\u{1}\u{163}\u{2}
tcy;\u{1}\u{442}\u{2}
tdot;\u{1}\u{20DB}\u{2}
telrec;\u{1}\u{2315}\u{2}
tfr;\u{1}\u{1D531}\u{2}
there4;\u{1}\u{2234}\u{2}
therefore;\u{1}\u{2234}\u{2}
theta;\u{1}\u{3B8}\u{2}
thetasym;\u{1}\u{3D1}\u{2}
thetav;\u{1}\u{3D1}\u{2}
thickapprox;\u{1}\u{2248}\u{2}
thicksim;\u{1}\u{223C}\u{2}
thinsp;\u{1}\u{2009}\u{2}
thkap;\u{1}\u{2248}\u{2}
thksim;\u{1}\u{223C}\u{2}
thorn\u{1}\u{FE}\u{2}
thorn;\u{1}\u{FE}\u{2}
tilde;\u{1}\u{2DC}\u{2}
times\u{1}\u{D7}\u{2}
times;\u{1}\u{D7}\u{2}
timesb;\u{1}\u{22A0}\u{2}
timesbar;\u{1}\u{2A31}\u{2}
timesd;\u{1}\u{2A30}\u{2}
tint;\u{1}\u{222D}\u{2}
toea;\u{1}\u{2928}\u{2}
top;\u{1}\u{22A4}\u{2}
topbot;\u{1}\u{2336}\u{2}
topcir;\u{1}\u{2AF1}\u{2}
topf;\u{1}\u{1D565}\u{2}
topfork;\u{1}\u{2ADA}\u{2}
tosa;\u{1}\u{2929}\u{2}
tprime;\u{1}\u{2034}\u{2}
trade;\u{1}\u{2122}\u{2}
triangle;\u{1}\u{25B5}\u{2}
triangledown;\u{1}\u{25BF}\u{2}
triangleleft;\u{1}\u{25C3}\u{2}
trianglelefteq;\u{1}\u{22B4}\u{2}
triangleq;\u{1}\u{225C}\u{2}
triangleright;\u{1}\u{25B9}\u{2}
trianglerighteq;\u{1}\u{22B5}\u{2}
tridot;\u{1}\u{25EC}\u{2}
trie;\u{1}\u{225C}\u{2}
triminus;\u{1}\u{2A3A}\u{2}
triplus;\u{1}\u{2A39}\u{2}
trisb;\u{1}\u{29CD}\u{2}
tritime;\u{1}\u{2A3B}\u{2}
trpezium;\u{1}\u{23E2}\u{2}
tscr;\u{1}\u{1D4C9}\u{2}
tscy;\u{1}\u{446}\u{2}
tshcy;\u{1}\u{45B}\u{2}
tstrok;\u{1}\u{167}\u{2}
twixt;\u{1}\u{226C}\u{2}
twoheadleftarrow;\u{1}\u{219E}\u{2}
twoheadrightarrow;\u{1}\u{21A0}\u{2}
uArr;\u{1}\u{21D1}\u{2}
uHar;\u{1}\u{2963}\u{2}
uacute\u{1}\u{FA}\u{2}
uacute;\u{1}\u{FA}\u{2}
uarr;\u{1}\u{2191}\u{2}
ubrcy;\u{1}\u{45E}\u{2}
ubreve;\u{1}\u{16D}\u{2}
ucirc\u{1}\u{FB}\u{2}
ucirc;\u{1}\u{FB}\u{2}
ucy;\u{1}\u{443}\u{2}
udarr;\u{1}\u{21C5}\u{2}
udblac;\u{1}\u{171}\u{2}
udhar;\u{1}\u{296E}\u{2}
ufisht;\u{1}\u{297E}\u{2}
ufr;\u{1}\u{1D532}\u{2}
ugrave\u{1}\u{F9}\u{2}
ugrave;\u{1}\u{F9}\u{2}
uharl;\u{1}\u{21BF}\u{2}
uharr;\u{1}\u{21BE}\u{2}
uhblk;\u{1}\u{2580}\u{2}
ulcorn;\u{1}\u{231C}\u{2}
ulcorner;\u{1}\u{231C}\u{2}
ulcrop;\u{1}\u{230F}\u{2}
ultri;\u{1}\u{25F8}\u{2}
umacr;\u{1}\u{16B}\u{2}
uml\u{1}\u{A8}\u{2}
uml;\u{1}\u{A8}\u{2}
uogon;\u{1}\u{173}\u{2}
uopf;\u{1}\u{1D566}\u{2}
uparrow;\u{1}\u{2191}\u{2}
updownarrow;\u{1}\u{2195}\u{2}
upharpoonleft;\u{1}\u{21BF}\u{2}
upharpoonright;\u{1}\u{21BE}\u{2}
uplus;\u{1}\u{228E}\u{2}
upsi;\u{1}\u{3C5}\u{2}
upsih;\u{1}\u{3D2}\u{2}
upsilon;\u{1}\u{3C5}\u{2}
upuparrows;\u{1}\u{21C8}\u{2}
urcorn;\u{1}\u{231D}\u{2}
urcorner;\u{1}\u{231D}\u{2}
urcrop;\u{1}\u{230E}\u{2}
uring;\u{1}\u{16F}\u{2}
urtri;\u{1}\u{25F9}\u{2}
uscr;\u{1}\u{1D4CA}\u{2}
utdot;\u{1}\u{22F0}\u{2}
utilde;\u{1}\u{169}\u{2}
utri;\u{1}\u{25B5}\u{2}
utrif;\u{1}\u{25B4}\u{2}
uuarr;\u{1}\u{21C8}\u{2}
uuml\u{1}\u{FC}\u{2}
uuml;\u{1}\u{FC}\u{2}
uwangle;\u{1}\u{29A7}\u{2}
vArr;\u{1}\u{21D5}\u{2}
vBar;\u{1}\u{2AE8}\u{2}
vBarv;\u{1}\u{2AE9}\u{2}
vDash;\u{1}\u{22A8}\u{2}
vangrt;\u{1}\u{299C}\u{2}
varepsilon;\u{1}\u{3F5}\u{2}
varkappa;\u{1}\u{3F0}\u{2}
varnothing;\u{1}\u{2205}\u{2}
varphi;\u{1}\u{3D5}\u{2}
varpi;\u{1}\u{3D6}\u{2}
varpropto;\u{1}\u{221D}\u{2}
varr;\u{1}\u{2195}\u{2}
varrho;\u{1}\u{3F1}\u{2}
varsigma;\u{1}\u{3C2}\u{2}
varsubsetneq;\u{1}\u{228A}\u{FE00}\u{2}
varsubsetneqq;\u{1}\u{2ACB}\u{FE00}\u{2}
varsupsetneq;\u{1}\u{228B}\u{FE00}\u{2}
varsupsetneqq;\u{1}\u{2ACC}\u{FE00}\u{2}
vartheta;\u{1}\u{3D1}\u{2}
vartriangleleft;\u{1}\u{22B2}\u{2}
vartriangleright;\u{1}\u{22B3}\u{2}
vcy;\u{1}\u{432}\u{2}
vdash;\u{1}\u{22A2}\u{2}
vee;\u{1}\u{2228}\u{2}
veebar;\u{1}\u{22BB}\u{2}
veeeq;\u{1}\u{225A}\u{2}
vellip;\u{1}\u{22EE}\u{2}
verbar;\u{1}|\u{2}
vert;\u{1}|\u{2}
vfr;\u{1}\u{1D533}\u{2}
vltri;\u{1}\u{22B2}\u{2}
vnsub;\u{1}\u{2282}\u{20D2}\u{2}
vnsup;\u{1}\u{2283}\u{20D2}\u{2}
vopf;\u{1}\u{1D567}\u{2}
vprop;\u{1}\u{221D}\u{2}
vrtri;\u{1}\u{22B3}\u{2}
vscr;\u{1}\u{1D4CB}\u{2}
vsubnE;\u{1}\u{2ACB}\u{FE00}\u{2}
vsubne;\u{1}\u{228A}\u{FE00}\u{2}
vsupnE;\u{1}\u{2ACC}\u{FE00}\u{2}
vsupne;\u{1}\u{228B}\u{FE00}\u{2}
vzigzag;\u{1}\u{299A}\u{2}
wcirc;\u{1}\u{175}\u{2}
wedbar;\u{1}\u{2A5F}\u{2}
wedge;\u{1}\u{2227}\u{2}
wedgeq;\u{1}\u{2259}\u{2}
weierp;\u{1}\u{2118}\u{2}
wfr;\u{1}\u{1D534}\u{2}
wopf;\u{1}\u{1D568}\u{2}
wp;\u{1}\u{2118}\u{2}
wr;\u{1}\u{2240}\u{2}
wreath;\u{1}\u{2240}\u{2}
wscr;\u{1}\u{1D4CC}\u{2}
xcap;\u{1}\u{22C2}\u{2}
xcirc;\u{1}\u{25EF}\u{2}
xcup;\u{1}\u{22C3}\u{2}
xdtri;\u{1}\u{25BD}\u{2}
xfr;\u{1}\u{1D535}\u{2}
xhArr;\u{1}\u{27FA}\u{2}
xharr;\u{1}\u{27F7}\u{2}
xi;\u{1}\u{3BE}\u{2}
xlArr;\u{1}\u{27F8}\u{2}
xlarr;\u{1}\u{27F5}\u{2}
xmap;\u{1}\u{27FC}\u{2}
xnis;\u{1}\u{22FB}\u{2}
xodot;\u{1}\u{2A00}\u{2}
xopf;\u{1}\u{1D569}\u{2}
xoplus;\u{1}\u{2A01}\u{2}
xotime;\u{1}\u{2A02}\u{2}
xrArr;\u{1}\u{27F9}\u{2}
xrarr;\u{1}\u{27F6}\u{2}
xscr;\u{1}\u{1D4CD}\u{2}
xsqcup;\u{1}\u{2A06}\u{2}
xuplus;\u{1}\u{2A04}\u{2}
xutri;\u{1}\u{25B3}\u{2}
xvee;\u{1}\u{22C1}\u{2}
xwedge;\u{1}\u{22C0}\u{2}
yacute\u{1}\u{FD}\u{2}
yacute;\u{1}\u{FD}\u{2}
yacy;\u{1}\u{44F}\u{2}
ycirc;\u{1}\u{177}\u{2}
ycy;\u{1}\u{44B}\u{2}
yen\u{1}\u{A5}\u{2}
yen;\u{1}\u{A5}\u{2}
yfr;\u{1}\u{1D536}\u{2}
yicy;\u{1}\u{457}\u{2}
yopf;\u{1}\u{1D56A}\u{2}
yscr;\u{1}\u{1D4CE}\u{2}
yucy;\u{1}\u{44E}\u{2}
yuml\u{1}\u{FF}\u{2}
yuml;\u{1}\u{FF}\u{2}
zacute;\u{1}\u{17A}\u{2}
zcaron;\u{1}\u{17E}\u{2}
zcy;\u{1}\u{437}\u{2}
zdot;\u{1}\u{17C}\u{2}
zeetrf;\u{1}\u{2128}\u{2}
zeta;\u{1}\u{3B6}\u{2}
zfr;\u{1}\u{1D537}\u{2}
zhcy;\u{1}\u{436}\u{2}
zigrarr;\u{1}\u{21DD}\u{2}
zopf;\u{1}\u{1D56B}\u{2}
zscr;\u{1}\u{1D4CF}\u{2}
zwj;\u{1}\u{200D}\u{2}
zwnj;\u{1}\u{200C}\u{2}
"""

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
/// and return the decoded bytes. Non-ASCII bytes pass through LITERALLY in both,
/// which is why `b%C3%A9c3` and a literal `béc3` are both hosts Go accepts.
///
/// What they do with an ESCAPE is where they part, and it is not that one is
/// stricter than the other — each refuses what the other allows:
///
///   * `encodeHost` refuses an escape that names an ASCII byte, because in a
///     host an escape exists only to spell a byte you could not write. `a%41b`
///     is not a host. `%25` is excepted, which is how RFC 6874 opens a zone.
///   * `encodeZone` refuses an escape that names a byte you COULD have written
///     — the mirror rule, "you can escape in a zone but not to introduce a byte
///     you could not just write directly" — so `%25`, `%20` (Windows puts
///     spaces in zone names) and an escape naming a literal host byte are the
///     whole of what it allows. `[::1%25%C3%A9]` names nobody in Go.
///
/// Reading that second rule as simply absent accepts 171 of the 256 byte values
/// behind a zone, every one of them a gid Go refuses.
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
        if zone {
            guard byte == 0x25 || byte == 0x20 || isHostByte(UInt8(byte)) else { return nil }
        } else {
            guard byte >= 0x80 || byte == 0x25 else { return nil }
        }
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
