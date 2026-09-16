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
            return bomDecoded(gid)
        }
        // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
        guard envelope["purpose"] as? String == attachablePurpose else { return nil }
        guard let gid = envelope["gid"] as? String, !gid.isEmpty else { return nil }
        return bomDecoded(gid)
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
/// `markup(for:)` rendered a tag Go refuses to write.
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
private let entityTable: [String: String] = {
    var table = [String: String](minimumCapacity: 2229)
    for row in entityTableRows.split(separator: "\n", omittingEmptySubsequences: true) {
        guard let separator = row.firstIndex(of: "\u{1}") else { continue }
        table[String(row[..<separator])] = String(row[row.index(after: separator)...])
    }
    return table
}()

/// The longest name Go expands without a terminating semicolon, which bounds the
/// prefix descent: `longestEntityWithoutSemicolon`.
private let longestEntityWithoutSemicolon = 6

private let entityTableRows = """
AElig\u{1}\u{C6}
AElig;\u{1}\u{C6}
AMP\u{1}&
AMP;\u{1}&
Aacute\u{1}\u{C1}
Aacute;\u{1}\u{C1}
Abreve;\u{1}\u{102}
Acirc\u{1}\u{C2}
Acirc;\u{1}\u{C2}
Acy;\u{1}\u{410}
Afr;\u{1}\u{1D504}
Agrave\u{1}\u{C0}
Agrave;\u{1}\u{C0}
Alpha;\u{1}\u{391}
Amacr;\u{1}\u{100}
And;\u{1}\u{2A53}
Aogon;\u{1}\u{104}
Aopf;\u{1}\u{1D538}
ApplyFunction;\u{1}\u{2061}
Aring\u{1}\u{C5}
Aring;\u{1}\u{C5}
Ascr;\u{1}\u{1D49C}
Assign;\u{1}\u{2254}
Atilde\u{1}\u{C3}
Atilde;\u{1}\u{C3}
Auml\u{1}\u{C4}
Auml;\u{1}\u{C4}
Backslash;\u{1}\u{2216}
Barv;\u{1}\u{2AE7}
Barwed;\u{1}\u{2306}
Bcy;\u{1}\u{411}
Because;\u{1}\u{2235}
Bernoullis;\u{1}\u{212C}
Beta;\u{1}\u{392}
Bfr;\u{1}\u{1D505}
Bopf;\u{1}\u{1D539}
Breve;\u{1}\u{2D8}
Bscr;\u{1}\u{212C}
Bumpeq;\u{1}\u{224E}
CHcy;\u{1}\u{427}
COPY\u{1}\u{A9}
COPY;\u{1}\u{A9}
Cacute;\u{1}\u{106}
Cap;\u{1}\u{22D2}
CapitalDifferentialD;\u{1}\u{2145}
Cayleys;\u{1}\u{212D}
Ccaron;\u{1}\u{10C}
Ccedil\u{1}\u{C7}
Ccedil;\u{1}\u{C7}
Ccirc;\u{1}\u{108}
Cconint;\u{1}\u{2230}
Cdot;\u{1}\u{10A}
Cedilla;\u{1}\u{B8}
CenterDot;\u{1}\u{B7}
Cfr;\u{1}\u{212D}
Chi;\u{1}\u{3A7}
CircleDot;\u{1}\u{2299}
CircleMinus;\u{1}\u{2296}
CirclePlus;\u{1}\u{2295}
CircleTimes;\u{1}\u{2297}
ClockwiseContourIntegral;\u{1}\u{2232}
CloseCurlyDoubleQuote;\u{1}\u{201D}
CloseCurlyQuote;\u{1}\u{2019}
Colon;\u{1}\u{2237}
Colone;\u{1}\u{2A74}
Congruent;\u{1}\u{2261}
Conint;\u{1}\u{222F}
ContourIntegral;\u{1}\u{222E}
Copf;\u{1}\u{2102}
Coproduct;\u{1}\u{2210}
CounterClockwiseContourIntegral;\u{1}\u{2233}
Cross;\u{1}\u{2A2F}
Cscr;\u{1}\u{1D49E}
Cup;\u{1}\u{22D3}
CupCap;\u{1}\u{224D}
DD;\u{1}\u{2145}
DDotrahd;\u{1}\u{2911}
DJcy;\u{1}\u{402}
DScy;\u{1}\u{405}
DZcy;\u{1}\u{40F}
Dagger;\u{1}\u{2021}
Darr;\u{1}\u{21A1}
Dashv;\u{1}\u{2AE4}
Dcaron;\u{1}\u{10E}
Dcy;\u{1}\u{414}
Del;\u{1}\u{2207}
Delta;\u{1}\u{394}
Dfr;\u{1}\u{1D507}
DiacriticalAcute;\u{1}\u{B4}
DiacriticalDot;\u{1}\u{2D9}
DiacriticalDoubleAcute;\u{1}\u{2DD}
DiacriticalGrave;\u{1}`
DiacriticalTilde;\u{1}\u{2DC}
Diamond;\u{1}\u{22C4}
DifferentialD;\u{1}\u{2146}
Dopf;\u{1}\u{1D53B}
Dot;\u{1}\u{A8}
DotDot;\u{1}\u{20DC}
DotEqual;\u{1}\u{2250}
DoubleContourIntegral;\u{1}\u{222F}
DoubleDot;\u{1}\u{A8}
DoubleDownArrow;\u{1}\u{21D3}
DoubleLeftArrow;\u{1}\u{21D0}
DoubleLeftRightArrow;\u{1}\u{21D4}
DoubleLeftTee;\u{1}\u{2AE4}
DoubleLongLeftArrow;\u{1}\u{27F8}
DoubleLongLeftRightArrow;\u{1}\u{27FA}
DoubleLongRightArrow;\u{1}\u{27F9}
DoubleRightArrow;\u{1}\u{21D2}
DoubleRightTee;\u{1}\u{22A8}
DoubleUpArrow;\u{1}\u{21D1}
DoubleUpDownArrow;\u{1}\u{21D5}
DoubleVerticalBar;\u{1}\u{2225}
DownArrow;\u{1}\u{2193}
DownArrowBar;\u{1}\u{2913}
DownArrowUpArrow;\u{1}\u{21F5}
DownBreve;\u{1}\u{311}
DownLeftRightVector;\u{1}\u{2950}
DownLeftTeeVector;\u{1}\u{295E}
DownLeftVector;\u{1}\u{21BD}
DownLeftVectorBar;\u{1}\u{2956}
DownRightTeeVector;\u{1}\u{295F}
DownRightVector;\u{1}\u{21C1}
DownRightVectorBar;\u{1}\u{2957}
DownTee;\u{1}\u{22A4}
DownTeeArrow;\u{1}\u{21A7}
Downarrow;\u{1}\u{21D3}
Dscr;\u{1}\u{1D49F}
Dstrok;\u{1}\u{110}
ENG;\u{1}\u{14A}
ETH\u{1}\u{D0}
ETH;\u{1}\u{D0}
Eacute\u{1}\u{C9}
Eacute;\u{1}\u{C9}
Ecaron;\u{1}\u{11A}
Ecirc\u{1}\u{CA}
Ecirc;\u{1}\u{CA}
Ecy;\u{1}\u{42D}
Edot;\u{1}\u{116}
Efr;\u{1}\u{1D508}
Egrave\u{1}\u{C8}
Egrave;\u{1}\u{C8}
Element;\u{1}\u{2208}
Emacr;\u{1}\u{112}
EmptySmallSquare;\u{1}\u{25FB}
EmptyVerySmallSquare;\u{1}\u{25AB}
Eogon;\u{1}\u{118}
Eopf;\u{1}\u{1D53C}
Epsilon;\u{1}\u{395}
Equal;\u{1}\u{2A75}
EqualTilde;\u{1}\u{2242}
Equilibrium;\u{1}\u{21CC}
Escr;\u{1}\u{2130}
Esim;\u{1}\u{2A73}
Eta;\u{1}\u{397}
Euml\u{1}\u{CB}
Euml;\u{1}\u{CB}
Exists;\u{1}\u{2203}
ExponentialE;\u{1}\u{2147}
Fcy;\u{1}\u{424}
Ffr;\u{1}\u{1D509}
FilledSmallSquare;\u{1}\u{25FC}
FilledVerySmallSquare;\u{1}\u{25AA}
Fopf;\u{1}\u{1D53D}
ForAll;\u{1}\u{2200}
Fouriertrf;\u{1}\u{2131}
Fscr;\u{1}\u{2131}
GJcy;\u{1}\u{403}
GT\u{1}>
GT;\u{1}>
Gamma;\u{1}\u{393}
Gammad;\u{1}\u{3DC}
Gbreve;\u{1}\u{11E}
Gcedil;\u{1}\u{122}
Gcirc;\u{1}\u{11C}
Gcy;\u{1}\u{413}
Gdot;\u{1}\u{120}
Gfr;\u{1}\u{1D50A}
Gg;\u{1}\u{22D9}
Gopf;\u{1}\u{1D53E}
GreaterEqual;\u{1}\u{2265}
GreaterEqualLess;\u{1}\u{22DB}
GreaterFullEqual;\u{1}\u{2267}
GreaterGreater;\u{1}\u{2AA2}
GreaterLess;\u{1}\u{2277}
GreaterSlantEqual;\u{1}\u{2A7E}
GreaterTilde;\u{1}\u{2273}
Gscr;\u{1}\u{1D4A2}
Gt;\u{1}\u{226B}
HARDcy;\u{1}\u{42A}
Hacek;\u{1}\u{2C7}
Hat;\u{1}^
Hcirc;\u{1}\u{124}
Hfr;\u{1}\u{210C}
HilbertSpace;\u{1}\u{210B}
Hopf;\u{1}\u{210D}
HorizontalLine;\u{1}\u{2500}
Hscr;\u{1}\u{210B}
Hstrok;\u{1}\u{126}
HumpDownHump;\u{1}\u{224E}
HumpEqual;\u{1}\u{224F}
IEcy;\u{1}\u{415}
IJlig;\u{1}\u{132}
IOcy;\u{1}\u{401}
Iacute\u{1}\u{CD}
Iacute;\u{1}\u{CD}
Icirc\u{1}\u{CE}
Icirc;\u{1}\u{CE}
Icy;\u{1}\u{418}
Idot;\u{1}\u{130}
Ifr;\u{1}\u{2111}
Igrave\u{1}\u{CC}
Igrave;\u{1}\u{CC}
Im;\u{1}\u{2111}
Imacr;\u{1}\u{12A}
ImaginaryI;\u{1}\u{2148}
Implies;\u{1}\u{21D2}
Int;\u{1}\u{222C}
Integral;\u{1}\u{222B}
Intersection;\u{1}\u{22C2}
InvisibleComma;\u{1}\u{2063}
InvisibleTimes;\u{1}\u{2062}
Iogon;\u{1}\u{12E}
Iopf;\u{1}\u{1D540}
Iota;\u{1}\u{399}
Iscr;\u{1}\u{2110}
Itilde;\u{1}\u{128}
Iukcy;\u{1}\u{406}
Iuml\u{1}\u{CF}
Iuml;\u{1}\u{CF}
Jcirc;\u{1}\u{134}
Jcy;\u{1}\u{419}
Jfr;\u{1}\u{1D50D}
Jopf;\u{1}\u{1D541}
Jscr;\u{1}\u{1D4A5}
Jsercy;\u{1}\u{408}
Jukcy;\u{1}\u{404}
KHcy;\u{1}\u{425}
KJcy;\u{1}\u{40C}
Kappa;\u{1}\u{39A}
Kcedil;\u{1}\u{136}
Kcy;\u{1}\u{41A}
Kfr;\u{1}\u{1D50E}
Kopf;\u{1}\u{1D542}
Kscr;\u{1}\u{1D4A6}
LJcy;\u{1}\u{409}
LT\u{1}<
LT;\u{1}<
Lacute;\u{1}\u{139}
Lambda;\u{1}\u{39B}
Lang;\u{1}\u{27EA}
Laplacetrf;\u{1}\u{2112}
Larr;\u{1}\u{219E}
Lcaron;\u{1}\u{13D}
Lcedil;\u{1}\u{13B}
Lcy;\u{1}\u{41B}
LeftAngleBracket;\u{1}\u{27E8}
LeftArrow;\u{1}\u{2190}
LeftArrowBar;\u{1}\u{21E4}
LeftArrowRightArrow;\u{1}\u{21C6}
LeftCeiling;\u{1}\u{2308}
LeftDoubleBracket;\u{1}\u{27E6}
LeftDownTeeVector;\u{1}\u{2961}
LeftDownVector;\u{1}\u{21C3}
LeftDownVectorBar;\u{1}\u{2959}
LeftFloor;\u{1}\u{230A}
LeftRightArrow;\u{1}\u{2194}
LeftRightVector;\u{1}\u{294E}
LeftTee;\u{1}\u{22A3}
LeftTeeArrow;\u{1}\u{21A4}
LeftTeeVector;\u{1}\u{295A}
LeftTriangle;\u{1}\u{22B2}
LeftTriangleBar;\u{1}\u{29CF}
LeftTriangleEqual;\u{1}\u{22B4}
LeftUpDownVector;\u{1}\u{2951}
LeftUpTeeVector;\u{1}\u{2960}
LeftUpVector;\u{1}\u{21BF}
LeftUpVectorBar;\u{1}\u{2958}
LeftVector;\u{1}\u{21BC}
LeftVectorBar;\u{1}\u{2952}
Leftarrow;\u{1}\u{21D0}
Leftrightarrow;\u{1}\u{21D4}
LessEqualGreater;\u{1}\u{22DA}
LessFullEqual;\u{1}\u{2266}
LessGreater;\u{1}\u{2276}
LessLess;\u{1}\u{2AA1}
LessSlantEqual;\u{1}\u{2A7D}
LessTilde;\u{1}\u{2272}
Lfr;\u{1}\u{1D50F}
Ll;\u{1}\u{22D8}
Lleftarrow;\u{1}\u{21DA}
Lmidot;\u{1}\u{13F}
LongLeftArrow;\u{1}\u{27F5}
LongLeftRightArrow;\u{1}\u{27F7}
LongRightArrow;\u{1}\u{27F6}
Longleftarrow;\u{1}\u{27F8}
Longleftrightarrow;\u{1}\u{27FA}
Longrightarrow;\u{1}\u{27F9}
Lopf;\u{1}\u{1D543}
LowerLeftArrow;\u{1}\u{2199}
LowerRightArrow;\u{1}\u{2198}
Lscr;\u{1}\u{2112}
Lsh;\u{1}\u{21B0}
Lstrok;\u{1}\u{141}
Lt;\u{1}\u{226A}
Map;\u{1}\u{2905}
Mcy;\u{1}\u{41C}
MediumSpace;\u{1}\u{205F}
Mellintrf;\u{1}\u{2133}
Mfr;\u{1}\u{1D510}
MinusPlus;\u{1}\u{2213}
Mopf;\u{1}\u{1D544}
Mscr;\u{1}\u{2133}
Mu;\u{1}\u{39C}
NJcy;\u{1}\u{40A}
Nacute;\u{1}\u{143}
Ncaron;\u{1}\u{147}
Ncedil;\u{1}\u{145}
Ncy;\u{1}\u{41D}
NegativeMediumSpace;\u{1}\u{200B}
NegativeThickSpace;\u{1}\u{200B}
NegativeThinSpace;\u{1}\u{200B}
NegativeVeryThinSpace;\u{1}\u{200B}
NestedGreaterGreater;\u{1}\u{226B}
NestedLessLess;\u{1}\u{226A}
NewLine;\u{1}\n
Nfr;\u{1}\u{1D511}
NoBreak;\u{1}\u{2060}
NonBreakingSpace;\u{1}\u{A0}
Nopf;\u{1}\u{2115}
Not;\u{1}\u{2AEC}
NotCongruent;\u{1}\u{2262}
NotCupCap;\u{1}\u{226D}
NotDoubleVerticalBar;\u{1}\u{2226}
NotElement;\u{1}\u{2209}
NotEqual;\u{1}\u{2260}
NotEqualTilde;\u{1}\u{2242}\u{338}
NotExists;\u{1}\u{2204}
NotGreater;\u{1}\u{226F}
NotGreaterEqual;\u{1}\u{2271}
NotGreaterFullEqual;\u{1}\u{2267}\u{338}
NotGreaterGreater;\u{1}\u{226B}\u{338}
NotGreaterLess;\u{1}\u{2279}
NotGreaterSlantEqual;\u{1}\u{2A7E}\u{338}
NotGreaterTilde;\u{1}\u{2275}
NotHumpDownHump;\u{1}\u{224E}\u{338}
NotHumpEqual;\u{1}\u{224F}\u{338}
NotLeftTriangle;\u{1}\u{22EA}
NotLeftTriangleBar;\u{1}\u{29CF}\u{338}
NotLeftTriangleEqual;\u{1}\u{22EC}
NotLess;\u{1}\u{226E}
NotLessEqual;\u{1}\u{2270}
NotLessGreater;\u{1}\u{2278}
NotLessLess;\u{1}\u{226A}\u{338}
NotLessSlantEqual;\u{1}\u{2A7D}\u{338}
NotLessTilde;\u{1}\u{2274}
NotNestedGreaterGreater;\u{1}\u{2AA2}\u{338}
NotNestedLessLess;\u{1}\u{2AA1}\u{338}
NotPrecedes;\u{1}\u{2280}
NotPrecedesEqual;\u{1}\u{2AAF}\u{338}
NotPrecedesSlantEqual;\u{1}\u{22E0}
NotReverseElement;\u{1}\u{220C}
NotRightTriangle;\u{1}\u{22EB}
NotRightTriangleBar;\u{1}\u{29D0}\u{338}
NotRightTriangleEqual;\u{1}\u{22ED}
NotSquareSubset;\u{1}\u{228F}\u{338}
NotSquareSubsetEqual;\u{1}\u{22E2}
NotSquareSuperset;\u{1}\u{2290}\u{338}
NotSquareSupersetEqual;\u{1}\u{22E3}
NotSubset;\u{1}\u{2282}\u{20D2}
NotSubsetEqual;\u{1}\u{2288}
NotSucceeds;\u{1}\u{2281}
NotSucceedsEqual;\u{1}\u{2AB0}\u{338}
NotSucceedsSlantEqual;\u{1}\u{22E1}
NotSucceedsTilde;\u{1}\u{227F}\u{338}
NotSuperset;\u{1}\u{2283}\u{20D2}
NotSupersetEqual;\u{1}\u{2289}
NotTilde;\u{1}\u{2241}
NotTildeEqual;\u{1}\u{2244}
NotTildeFullEqual;\u{1}\u{2247}
NotTildeTilde;\u{1}\u{2249}
NotVerticalBar;\u{1}\u{2224}
Nscr;\u{1}\u{1D4A9}
Ntilde\u{1}\u{D1}
Ntilde;\u{1}\u{D1}
Nu;\u{1}\u{39D}
OElig;\u{1}\u{152}
Oacute\u{1}\u{D3}
Oacute;\u{1}\u{D3}
Ocirc\u{1}\u{D4}
Ocirc;\u{1}\u{D4}
Ocy;\u{1}\u{41E}
Odblac;\u{1}\u{150}
Ofr;\u{1}\u{1D512}
Ograve\u{1}\u{D2}
Ograve;\u{1}\u{D2}
Omacr;\u{1}\u{14C}
Omega;\u{1}\u{3A9}
Omicron;\u{1}\u{39F}
Oopf;\u{1}\u{1D546}
OpenCurlyDoubleQuote;\u{1}\u{201C}
OpenCurlyQuote;\u{1}\u{2018}
Or;\u{1}\u{2A54}
Oscr;\u{1}\u{1D4AA}
Oslash\u{1}\u{D8}
Oslash;\u{1}\u{D8}
Otilde\u{1}\u{D5}
Otilde;\u{1}\u{D5}
Otimes;\u{1}\u{2A37}
Ouml\u{1}\u{D6}
Ouml;\u{1}\u{D6}
OverBar;\u{1}\u{203E}
OverBrace;\u{1}\u{23DE}
OverBracket;\u{1}\u{23B4}
OverParenthesis;\u{1}\u{23DC}
PartialD;\u{1}\u{2202}
Pcy;\u{1}\u{41F}
Pfr;\u{1}\u{1D513}
Phi;\u{1}\u{3A6}
Pi;\u{1}\u{3A0}
PlusMinus;\u{1}\u{B1}
Poincareplane;\u{1}\u{210C}
Popf;\u{1}\u{2119}
Pr;\u{1}\u{2ABB}
Precedes;\u{1}\u{227A}
PrecedesEqual;\u{1}\u{2AAF}
PrecedesSlantEqual;\u{1}\u{227C}
PrecedesTilde;\u{1}\u{227E}
Prime;\u{1}\u{2033}
Product;\u{1}\u{220F}
Proportion;\u{1}\u{2237}
Proportional;\u{1}\u{221D}
Pscr;\u{1}\u{1D4AB}
Psi;\u{1}\u{3A8}
QUOT\u{1}\"
QUOT;\u{1}\"
Qfr;\u{1}\u{1D514}
Qopf;\u{1}\u{211A}
Qscr;\u{1}\u{1D4AC}
RBarr;\u{1}\u{2910}
REG\u{1}\u{AE}
REG;\u{1}\u{AE}
Racute;\u{1}\u{154}
Rang;\u{1}\u{27EB}
Rarr;\u{1}\u{21A0}
Rarrtl;\u{1}\u{2916}
Rcaron;\u{1}\u{158}
Rcedil;\u{1}\u{156}
Rcy;\u{1}\u{420}
Re;\u{1}\u{211C}
ReverseElement;\u{1}\u{220B}
ReverseEquilibrium;\u{1}\u{21CB}
ReverseUpEquilibrium;\u{1}\u{296F}
Rfr;\u{1}\u{211C}
Rho;\u{1}\u{3A1}
RightAngleBracket;\u{1}\u{27E9}
RightArrow;\u{1}\u{2192}
RightArrowBar;\u{1}\u{21E5}
RightArrowLeftArrow;\u{1}\u{21C4}
RightCeiling;\u{1}\u{2309}
RightDoubleBracket;\u{1}\u{27E7}
RightDownTeeVector;\u{1}\u{295D}
RightDownVector;\u{1}\u{21C2}
RightDownVectorBar;\u{1}\u{2955}
RightFloor;\u{1}\u{230B}
RightTee;\u{1}\u{22A2}
RightTeeArrow;\u{1}\u{21A6}
RightTeeVector;\u{1}\u{295B}
RightTriangle;\u{1}\u{22B3}
RightTriangleBar;\u{1}\u{29D0}
RightTriangleEqual;\u{1}\u{22B5}
RightUpDownVector;\u{1}\u{294F}
RightUpTeeVector;\u{1}\u{295C}
RightUpVector;\u{1}\u{21BE}
RightUpVectorBar;\u{1}\u{2954}
RightVector;\u{1}\u{21C0}
RightVectorBar;\u{1}\u{2953}
Rightarrow;\u{1}\u{21D2}
Ropf;\u{1}\u{211D}
RoundImplies;\u{1}\u{2970}
Rrightarrow;\u{1}\u{21DB}
Rscr;\u{1}\u{211B}
Rsh;\u{1}\u{21B1}
RuleDelayed;\u{1}\u{29F4}
SHCHcy;\u{1}\u{429}
SHcy;\u{1}\u{428}
SOFTcy;\u{1}\u{42C}
Sacute;\u{1}\u{15A}
Sc;\u{1}\u{2ABC}
Scaron;\u{1}\u{160}
Scedil;\u{1}\u{15E}
Scirc;\u{1}\u{15C}
Scy;\u{1}\u{421}
Sfr;\u{1}\u{1D516}
ShortDownArrow;\u{1}\u{2193}
ShortLeftArrow;\u{1}\u{2190}
ShortRightArrow;\u{1}\u{2192}
ShortUpArrow;\u{1}\u{2191}
Sigma;\u{1}\u{3A3}
SmallCircle;\u{1}\u{2218}
Sopf;\u{1}\u{1D54A}
Sqrt;\u{1}\u{221A}
Square;\u{1}\u{25A1}
SquareIntersection;\u{1}\u{2293}
SquareSubset;\u{1}\u{228F}
SquareSubsetEqual;\u{1}\u{2291}
SquareSuperset;\u{1}\u{2290}
SquareSupersetEqual;\u{1}\u{2292}
SquareUnion;\u{1}\u{2294}
Sscr;\u{1}\u{1D4AE}
Star;\u{1}\u{22C6}
Sub;\u{1}\u{22D0}
Subset;\u{1}\u{22D0}
SubsetEqual;\u{1}\u{2286}
Succeeds;\u{1}\u{227B}
SucceedsEqual;\u{1}\u{2AB0}
SucceedsSlantEqual;\u{1}\u{227D}
SucceedsTilde;\u{1}\u{227F}
SuchThat;\u{1}\u{220B}
Sum;\u{1}\u{2211}
Sup;\u{1}\u{22D1}
Superset;\u{1}\u{2283}
SupersetEqual;\u{1}\u{2287}
Supset;\u{1}\u{22D1}
THORN\u{1}\u{DE}
THORN;\u{1}\u{DE}
TRADE;\u{1}\u{2122}
TSHcy;\u{1}\u{40B}
TScy;\u{1}\u{426}
Tab;\u{1}\t
Tau;\u{1}\u{3A4}
Tcaron;\u{1}\u{164}
Tcedil;\u{1}\u{162}
Tcy;\u{1}\u{422}
Tfr;\u{1}\u{1D517}
Therefore;\u{1}\u{2234}
Theta;\u{1}\u{398}
ThickSpace;\u{1}\u{205F}\u{200A}
ThinSpace;\u{1}\u{2009}
Tilde;\u{1}\u{223C}
TildeEqual;\u{1}\u{2243}
TildeFullEqual;\u{1}\u{2245}
TildeTilde;\u{1}\u{2248}
Topf;\u{1}\u{1D54B}
TripleDot;\u{1}\u{20DB}
Tscr;\u{1}\u{1D4AF}
Tstrok;\u{1}\u{166}
Uacute\u{1}\u{DA}
Uacute;\u{1}\u{DA}
Uarr;\u{1}\u{219F}
Uarrocir;\u{1}\u{2949}
Ubrcy;\u{1}\u{40E}
Ubreve;\u{1}\u{16C}
Ucirc\u{1}\u{DB}
Ucirc;\u{1}\u{DB}
Ucy;\u{1}\u{423}
Udblac;\u{1}\u{170}
Ufr;\u{1}\u{1D518}
Ugrave\u{1}\u{D9}
Ugrave;\u{1}\u{D9}
Umacr;\u{1}\u{16A}
UnderBar;\u{1}_
UnderBrace;\u{1}\u{23DF}
UnderBracket;\u{1}\u{23B5}
UnderParenthesis;\u{1}\u{23DD}
Union;\u{1}\u{22C3}
UnionPlus;\u{1}\u{228E}
Uogon;\u{1}\u{172}
Uopf;\u{1}\u{1D54C}
UpArrow;\u{1}\u{2191}
UpArrowBar;\u{1}\u{2912}
UpArrowDownArrow;\u{1}\u{21C5}
UpDownArrow;\u{1}\u{2195}
UpEquilibrium;\u{1}\u{296E}
UpTee;\u{1}\u{22A5}
UpTeeArrow;\u{1}\u{21A5}
Uparrow;\u{1}\u{21D1}
Updownarrow;\u{1}\u{21D5}
UpperLeftArrow;\u{1}\u{2196}
UpperRightArrow;\u{1}\u{2197}
Upsi;\u{1}\u{3D2}
Upsilon;\u{1}\u{3A5}
Uring;\u{1}\u{16E}
Uscr;\u{1}\u{1D4B0}
Utilde;\u{1}\u{168}
Uuml\u{1}\u{DC}
Uuml;\u{1}\u{DC}
VDash;\u{1}\u{22AB}
Vbar;\u{1}\u{2AEB}
Vcy;\u{1}\u{412}
Vdash;\u{1}\u{22A9}
Vdashl;\u{1}\u{2AE6}
Vee;\u{1}\u{22C1}
Verbar;\u{1}\u{2016}
Vert;\u{1}\u{2016}
VerticalBar;\u{1}\u{2223}
VerticalLine;\u{1}|
VerticalSeparator;\u{1}\u{2758}
VerticalTilde;\u{1}\u{2240}
VeryThinSpace;\u{1}\u{200A}
Vfr;\u{1}\u{1D519}
Vopf;\u{1}\u{1D54D}
Vscr;\u{1}\u{1D4B1}
Vvdash;\u{1}\u{22AA}
Wcirc;\u{1}\u{174}
Wedge;\u{1}\u{22C0}
Wfr;\u{1}\u{1D51A}
Wopf;\u{1}\u{1D54E}
Wscr;\u{1}\u{1D4B2}
Xfr;\u{1}\u{1D51B}
Xi;\u{1}\u{39E}
Xopf;\u{1}\u{1D54F}
Xscr;\u{1}\u{1D4B3}
YAcy;\u{1}\u{42F}
YIcy;\u{1}\u{407}
YUcy;\u{1}\u{42E}
Yacute\u{1}\u{DD}
Yacute;\u{1}\u{DD}
Ycirc;\u{1}\u{176}
Ycy;\u{1}\u{42B}
Yfr;\u{1}\u{1D51C}
Yopf;\u{1}\u{1D550}
Yscr;\u{1}\u{1D4B4}
Yuml;\u{1}\u{178}
ZHcy;\u{1}\u{416}
Zacute;\u{1}\u{179}
Zcaron;\u{1}\u{17D}
Zcy;\u{1}\u{417}
Zdot;\u{1}\u{17B}
ZeroWidthSpace;\u{1}\u{200B}
Zeta;\u{1}\u{396}
Zfr;\u{1}\u{2128}
Zopf;\u{1}\u{2124}
Zscr;\u{1}\u{1D4B5}
aacute\u{1}\u{E1}
aacute;\u{1}\u{E1}
abreve;\u{1}\u{103}
ac;\u{1}\u{223E}
acE;\u{1}\u{223E}\u{333}
acd;\u{1}\u{223F}
acirc\u{1}\u{E2}
acirc;\u{1}\u{E2}
acute\u{1}\u{B4}
acute;\u{1}\u{B4}
acy;\u{1}\u{430}
aelig\u{1}\u{E6}
aelig;\u{1}\u{E6}
af;\u{1}\u{2061}
afr;\u{1}\u{1D51E}
agrave\u{1}\u{E0}
agrave;\u{1}\u{E0}
alefsym;\u{1}\u{2135}
aleph;\u{1}\u{2135}
alpha;\u{1}\u{3B1}
amacr;\u{1}\u{101}
amalg;\u{1}\u{2A3F}
amp\u{1}&
amp;\u{1}&
and;\u{1}\u{2227}
andand;\u{1}\u{2A55}
andd;\u{1}\u{2A5C}
andslope;\u{1}\u{2A58}
andv;\u{1}\u{2A5A}
ang;\u{1}\u{2220}
ange;\u{1}\u{29A4}
angle;\u{1}\u{2220}
angmsd;\u{1}\u{2221}
angmsdaa;\u{1}\u{29A8}
angmsdab;\u{1}\u{29A9}
angmsdac;\u{1}\u{29AA}
angmsdad;\u{1}\u{29AB}
angmsdae;\u{1}\u{29AC}
angmsdaf;\u{1}\u{29AD}
angmsdag;\u{1}\u{29AE}
angmsdah;\u{1}\u{29AF}
angrt;\u{1}\u{221F}
angrtvb;\u{1}\u{22BE}
angrtvbd;\u{1}\u{299D}
angsph;\u{1}\u{2222}
angst;\u{1}\u{C5}
angzarr;\u{1}\u{237C}
aogon;\u{1}\u{105}
aopf;\u{1}\u{1D552}
ap;\u{1}\u{2248}
apE;\u{1}\u{2A70}
apacir;\u{1}\u{2A6F}
ape;\u{1}\u{224A}
apid;\u{1}\u{224B}
apos;\u{1}'
approx;\u{1}\u{2248}
approxeq;\u{1}\u{224A}
aring\u{1}\u{E5}
aring;\u{1}\u{E5}
ascr;\u{1}\u{1D4B6}
ast;\u{1}*
asymp;\u{1}\u{2248}
asympeq;\u{1}\u{224D}
atilde\u{1}\u{E3}
atilde;\u{1}\u{E3}
auml\u{1}\u{E4}
auml;\u{1}\u{E4}
awconint;\u{1}\u{2233}
awint;\u{1}\u{2A11}
bNot;\u{1}\u{2AED}
backcong;\u{1}\u{224C}
backepsilon;\u{1}\u{3F6}
backprime;\u{1}\u{2035}
backsim;\u{1}\u{223D}
backsimeq;\u{1}\u{22CD}
barvee;\u{1}\u{22BD}
barwed;\u{1}\u{2305}
barwedge;\u{1}\u{2305}
bbrk;\u{1}\u{23B5}
bbrktbrk;\u{1}\u{23B6}
bcong;\u{1}\u{224C}
bcy;\u{1}\u{431}
bdquo;\u{1}\u{201E}
becaus;\u{1}\u{2235}
because;\u{1}\u{2235}
bemptyv;\u{1}\u{29B0}
bepsi;\u{1}\u{3F6}
bernou;\u{1}\u{212C}
beta;\u{1}\u{3B2}
beth;\u{1}\u{2136}
between;\u{1}\u{226C}
bfr;\u{1}\u{1D51F}
bigcap;\u{1}\u{22C2}
bigcirc;\u{1}\u{25EF}
bigcup;\u{1}\u{22C3}
bigodot;\u{1}\u{2A00}
bigoplus;\u{1}\u{2A01}
bigotimes;\u{1}\u{2A02}
bigsqcup;\u{1}\u{2A06}
bigstar;\u{1}\u{2605}
bigtriangledown;\u{1}\u{25BD}
bigtriangleup;\u{1}\u{25B3}
biguplus;\u{1}\u{2A04}
bigvee;\u{1}\u{22C1}
bigwedge;\u{1}\u{22C0}
bkarow;\u{1}\u{290D}
blacklozenge;\u{1}\u{29EB}
blacksquare;\u{1}\u{25AA}
blacktriangle;\u{1}\u{25B4}
blacktriangledown;\u{1}\u{25BE}
blacktriangleleft;\u{1}\u{25C2}
blacktriangleright;\u{1}\u{25B8}
blank;\u{1}\u{2423}
blk12;\u{1}\u{2592}
blk14;\u{1}\u{2591}
blk34;\u{1}\u{2593}
block;\u{1}\u{2588}
bne;\u{1}=\u{20E5}
bnequiv;\u{1}\u{2261}\u{20E5}
bnot;\u{1}\u{2310}
bopf;\u{1}\u{1D553}
bot;\u{1}\u{22A5}
bottom;\u{1}\u{22A5}
bowtie;\u{1}\u{22C8}
boxDL;\u{1}\u{2557}
boxDR;\u{1}\u{2554}
boxDl;\u{1}\u{2556}
boxDr;\u{1}\u{2553}
boxH;\u{1}\u{2550}
boxHD;\u{1}\u{2566}
boxHU;\u{1}\u{2569}
boxHd;\u{1}\u{2564}
boxHu;\u{1}\u{2567}
boxUL;\u{1}\u{255D}
boxUR;\u{1}\u{255A}
boxUl;\u{1}\u{255C}
boxUr;\u{1}\u{2559}
boxV;\u{1}\u{2551}
boxVH;\u{1}\u{256C}
boxVL;\u{1}\u{2563}
boxVR;\u{1}\u{2560}
boxVh;\u{1}\u{256B}
boxVl;\u{1}\u{2562}
boxVr;\u{1}\u{255F}
boxbox;\u{1}\u{29C9}
boxdL;\u{1}\u{2555}
boxdR;\u{1}\u{2552}
boxdl;\u{1}\u{2510}
boxdr;\u{1}\u{250C}
boxh;\u{1}\u{2500}
boxhD;\u{1}\u{2565}
boxhU;\u{1}\u{2568}
boxhd;\u{1}\u{252C}
boxhu;\u{1}\u{2534}
boxminus;\u{1}\u{229F}
boxplus;\u{1}\u{229E}
boxtimes;\u{1}\u{22A0}
boxuL;\u{1}\u{255B}
boxuR;\u{1}\u{2558}
boxul;\u{1}\u{2518}
boxur;\u{1}\u{2514}
boxv;\u{1}\u{2502}
boxvH;\u{1}\u{256A}
boxvL;\u{1}\u{2561}
boxvR;\u{1}\u{255E}
boxvh;\u{1}\u{253C}
boxvl;\u{1}\u{2524}
boxvr;\u{1}\u{251C}
bprime;\u{1}\u{2035}
breve;\u{1}\u{2D8}
brvbar\u{1}\u{A6}
brvbar;\u{1}\u{A6}
bscr;\u{1}\u{1D4B7}
bsemi;\u{1}\u{204F}
bsim;\u{1}\u{223D}
bsime;\u{1}\u{22CD}
bsol;\u{1}\\
bsolb;\u{1}\u{29C5}
bsolhsub;\u{1}\u{27C8}
bull;\u{1}\u{2022}
bullet;\u{1}\u{2022}
bump;\u{1}\u{224E}
bumpE;\u{1}\u{2AAE}
bumpe;\u{1}\u{224F}
bumpeq;\u{1}\u{224F}
cacute;\u{1}\u{107}
cap;\u{1}\u{2229}
capand;\u{1}\u{2A44}
capbrcup;\u{1}\u{2A49}
capcap;\u{1}\u{2A4B}
capcup;\u{1}\u{2A47}
capdot;\u{1}\u{2A40}
caps;\u{1}\u{2229}\u{FE00}
caret;\u{1}\u{2041}
caron;\u{1}\u{2C7}
ccaps;\u{1}\u{2A4D}
ccaron;\u{1}\u{10D}
ccedil\u{1}\u{E7}
ccedil;\u{1}\u{E7}
ccirc;\u{1}\u{109}
ccups;\u{1}\u{2A4C}
ccupssm;\u{1}\u{2A50}
cdot;\u{1}\u{10B}
cedil\u{1}\u{B8}
cedil;\u{1}\u{B8}
cemptyv;\u{1}\u{29B2}
cent\u{1}\u{A2}
cent;\u{1}\u{A2}
centerdot;\u{1}\u{B7}
cfr;\u{1}\u{1D520}
chcy;\u{1}\u{447}
check;\u{1}\u{2713}
checkmark;\u{1}\u{2713}
chi;\u{1}\u{3C7}
cir;\u{1}\u{25CB}
cirE;\u{1}\u{29C3}
circ;\u{1}\u{2C6}
circeq;\u{1}\u{2257}
circlearrowleft;\u{1}\u{21BA}
circlearrowright;\u{1}\u{21BB}
circledR;\u{1}\u{AE}
circledS;\u{1}\u{24C8}
circledast;\u{1}\u{229B}
circledcirc;\u{1}\u{229A}
circleddash;\u{1}\u{229D}
cire;\u{1}\u{2257}
cirfnint;\u{1}\u{2A10}
cirmid;\u{1}\u{2AEF}
cirscir;\u{1}\u{29C2}
clubs;\u{1}\u{2663}
clubsuit;\u{1}\u{2663}
colon;\u{1}:
colone;\u{1}\u{2254}
coloneq;\u{1}\u{2254}
comma;\u{1},
commat;\u{1}@
comp;\u{1}\u{2201}
compfn;\u{1}\u{2218}
complement;\u{1}\u{2201}
complexes;\u{1}\u{2102}
cong;\u{1}\u{2245}
congdot;\u{1}\u{2A6D}
conint;\u{1}\u{222E}
copf;\u{1}\u{1D554}
coprod;\u{1}\u{2210}
copy\u{1}\u{A9}
copy;\u{1}\u{A9}
copysr;\u{1}\u{2117}
crarr;\u{1}\u{21B5}
cross;\u{1}\u{2717}
cscr;\u{1}\u{1D4B8}
csub;\u{1}\u{2ACF}
csube;\u{1}\u{2AD1}
csup;\u{1}\u{2AD0}
csupe;\u{1}\u{2AD2}
ctdot;\u{1}\u{22EF}
cudarrl;\u{1}\u{2938}
cudarrr;\u{1}\u{2935}
cuepr;\u{1}\u{22DE}
cuesc;\u{1}\u{22DF}
cularr;\u{1}\u{21B6}
cularrp;\u{1}\u{293D}
cup;\u{1}\u{222A}
cupbrcap;\u{1}\u{2A48}
cupcap;\u{1}\u{2A46}
cupcup;\u{1}\u{2A4A}
cupdot;\u{1}\u{228D}
cupor;\u{1}\u{2A45}
cups;\u{1}\u{222A}\u{FE00}
curarr;\u{1}\u{21B7}
curarrm;\u{1}\u{293C}
curlyeqprec;\u{1}\u{22DE}
curlyeqsucc;\u{1}\u{22DF}
curlyvee;\u{1}\u{22CE}
curlywedge;\u{1}\u{22CF}
curren\u{1}\u{A4}
curren;\u{1}\u{A4}
curvearrowleft;\u{1}\u{21B6}
curvearrowright;\u{1}\u{21B7}
cuvee;\u{1}\u{22CE}
cuwed;\u{1}\u{22CF}
cwconint;\u{1}\u{2232}
cwint;\u{1}\u{2231}
cylcty;\u{1}\u{232D}
dArr;\u{1}\u{21D3}
dHar;\u{1}\u{2965}
dagger;\u{1}\u{2020}
daleth;\u{1}\u{2138}
darr;\u{1}\u{2193}
dash;\u{1}\u{2010}
dashv;\u{1}\u{22A3}
dbkarow;\u{1}\u{290F}
dblac;\u{1}\u{2DD}
dcaron;\u{1}\u{10F}
dcy;\u{1}\u{434}
dd;\u{1}\u{2146}
ddagger;\u{1}\u{2021}
ddarr;\u{1}\u{21CA}
ddotseq;\u{1}\u{2A77}
deg\u{1}\u{B0}
deg;\u{1}\u{B0}
delta;\u{1}\u{3B4}
demptyv;\u{1}\u{29B1}
dfisht;\u{1}\u{297F}
dfr;\u{1}\u{1D521}
dharl;\u{1}\u{21C3}
dharr;\u{1}\u{21C2}
diam;\u{1}\u{22C4}
diamond;\u{1}\u{22C4}
diamondsuit;\u{1}\u{2666}
diams;\u{1}\u{2666}
die;\u{1}\u{A8}
digamma;\u{1}\u{3DD}
disin;\u{1}\u{22F2}
div;\u{1}\u{F7}
divide\u{1}\u{F7}
divide;\u{1}\u{F7}
divideontimes;\u{1}\u{22C7}
divonx;\u{1}\u{22C7}
djcy;\u{1}\u{452}
dlcorn;\u{1}\u{231E}
dlcrop;\u{1}\u{230D}
dollar;\u{1}$
dopf;\u{1}\u{1D555}
dot;\u{1}\u{2D9}
doteq;\u{1}\u{2250}
doteqdot;\u{1}\u{2251}
dotminus;\u{1}\u{2238}
dotplus;\u{1}\u{2214}
dotsquare;\u{1}\u{22A1}
doublebarwedge;\u{1}\u{2306}
downarrow;\u{1}\u{2193}
downdownarrows;\u{1}\u{21CA}
downharpoonleft;\u{1}\u{21C3}
downharpoonright;\u{1}\u{21C2}
drbkarow;\u{1}\u{2910}
drcorn;\u{1}\u{231F}
drcrop;\u{1}\u{230C}
dscr;\u{1}\u{1D4B9}
dscy;\u{1}\u{455}
dsol;\u{1}\u{29F6}
dstrok;\u{1}\u{111}
dtdot;\u{1}\u{22F1}
dtri;\u{1}\u{25BF}
dtrif;\u{1}\u{25BE}
duarr;\u{1}\u{21F5}
duhar;\u{1}\u{296F}
dwangle;\u{1}\u{29A6}
dzcy;\u{1}\u{45F}
dzigrarr;\u{1}\u{27FF}
eDDot;\u{1}\u{2A77}
eDot;\u{1}\u{2251}
eacute\u{1}\u{E9}
eacute;\u{1}\u{E9}
easter;\u{1}\u{2A6E}
ecaron;\u{1}\u{11B}
ecir;\u{1}\u{2256}
ecirc\u{1}\u{EA}
ecirc;\u{1}\u{EA}
ecolon;\u{1}\u{2255}
ecy;\u{1}\u{44D}
edot;\u{1}\u{117}
ee;\u{1}\u{2147}
efDot;\u{1}\u{2252}
efr;\u{1}\u{1D522}
eg;\u{1}\u{2A9A}
egrave\u{1}\u{E8}
egrave;\u{1}\u{E8}
egs;\u{1}\u{2A96}
egsdot;\u{1}\u{2A98}
el;\u{1}\u{2A99}
elinters;\u{1}\u{23E7}
ell;\u{1}\u{2113}
els;\u{1}\u{2A95}
elsdot;\u{1}\u{2A97}
emacr;\u{1}\u{113}
empty;\u{1}\u{2205}
emptyset;\u{1}\u{2205}
emptyv;\u{1}\u{2205}
emsp13;\u{1}\u{2004}
emsp14;\u{1}\u{2005}
emsp;\u{1}\u{2003}
eng;\u{1}\u{14B}
ensp;\u{1}\u{2002}
eogon;\u{1}\u{119}
eopf;\u{1}\u{1D556}
epar;\u{1}\u{22D5}
eparsl;\u{1}\u{29E3}
eplus;\u{1}\u{2A71}
epsi;\u{1}\u{3B5}
epsilon;\u{1}\u{3B5}
epsiv;\u{1}\u{3F5}
eqcirc;\u{1}\u{2256}
eqcolon;\u{1}\u{2255}
eqsim;\u{1}\u{2242}
eqslantgtr;\u{1}\u{2A96}
eqslantless;\u{1}\u{2A95}
equals;\u{1}=
equest;\u{1}\u{225F}
equiv;\u{1}\u{2261}
equivDD;\u{1}\u{2A78}
eqvparsl;\u{1}\u{29E5}
erDot;\u{1}\u{2253}
erarr;\u{1}\u{2971}
escr;\u{1}\u{212F}
esdot;\u{1}\u{2250}
esim;\u{1}\u{2242}
eta;\u{1}\u{3B7}
eth\u{1}\u{F0}
eth;\u{1}\u{F0}
euml\u{1}\u{EB}
euml;\u{1}\u{EB}
euro;\u{1}\u{20AC}
excl;\u{1}!
exist;\u{1}\u{2203}
expectation;\u{1}\u{2130}
exponentiale;\u{1}\u{2147}
fallingdotseq;\u{1}\u{2252}
fcy;\u{1}\u{444}
female;\u{1}\u{2640}
ffilig;\u{1}\u{FB03}
fflig;\u{1}\u{FB00}
ffllig;\u{1}\u{FB04}
ffr;\u{1}\u{1D523}
filig;\u{1}\u{FB01}
fjlig;\u{1}fj
flat;\u{1}\u{266D}
fllig;\u{1}\u{FB02}
fltns;\u{1}\u{25B1}
fnof;\u{1}\u{192}
fopf;\u{1}\u{1D557}
forall;\u{1}\u{2200}
fork;\u{1}\u{22D4}
forkv;\u{1}\u{2AD9}
fpartint;\u{1}\u{2A0D}
frac12\u{1}\u{BD}
frac12;\u{1}\u{BD}
frac13;\u{1}\u{2153}
frac14\u{1}\u{BC}
frac14;\u{1}\u{BC}
frac15;\u{1}\u{2155}
frac16;\u{1}\u{2159}
frac18;\u{1}\u{215B}
frac23;\u{1}\u{2154}
frac25;\u{1}\u{2156}
frac34\u{1}\u{BE}
frac34;\u{1}\u{BE}
frac35;\u{1}\u{2157}
frac38;\u{1}\u{215C}
frac45;\u{1}\u{2158}
frac56;\u{1}\u{215A}
frac58;\u{1}\u{215D}
frac78;\u{1}\u{215E}
frasl;\u{1}\u{2044}
frown;\u{1}\u{2322}
fscr;\u{1}\u{1D4BB}
gE;\u{1}\u{2267}
gEl;\u{1}\u{2A8C}
gacute;\u{1}\u{1F5}
gamma;\u{1}\u{3B3}
gammad;\u{1}\u{3DD}
gap;\u{1}\u{2A86}
gbreve;\u{1}\u{11F}
gcirc;\u{1}\u{11D}
gcy;\u{1}\u{433}
gdot;\u{1}\u{121}
ge;\u{1}\u{2265}
gel;\u{1}\u{22DB}
geq;\u{1}\u{2265}
geqq;\u{1}\u{2267}
geqslant;\u{1}\u{2A7E}
ges;\u{1}\u{2A7E}
gescc;\u{1}\u{2AA9}
gesdot;\u{1}\u{2A80}
gesdoto;\u{1}\u{2A82}
gesdotol;\u{1}\u{2A84}
gesl;\u{1}\u{22DB}\u{FE00}
gesles;\u{1}\u{2A94}
gfr;\u{1}\u{1D524}
gg;\u{1}\u{226B}
ggg;\u{1}\u{22D9}
gimel;\u{1}\u{2137}
gjcy;\u{1}\u{453}
gl;\u{1}\u{2277}
glE;\u{1}\u{2A92}
gla;\u{1}\u{2AA5}
glj;\u{1}\u{2AA4}
gnE;\u{1}\u{2269}
gnap;\u{1}\u{2A8A}
gnapprox;\u{1}\u{2A8A}
gne;\u{1}\u{2A88}
gneq;\u{1}\u{2A88}
gneqq;\u{1}\u{2269}
gnsim;\u{1}\u{22E7}
gopf;\u{1}\u{1D558}
grave;\u{1}`
gscr;\u{1}\u{210A}
gsim;\u{1}\u{2273}
gsime;\u{1}\u{2A8E}
gsiml;\u{1}\u{2A90}
gt\u{1}>
gt;\u{1}>
gtcc;\u{1}\u{2AA7}
gtcir;\u{1}\u{2A7A}
gtdot;\u{1}\u{22D7}
gtlPar;\u{1}\u{2995}
gtquest;\u{1}\u{2A7C}
gtrapprox;\u{1}\u{2A86}
gtrarr;\u{1}\u{2978}
gtrdot;\u{1}\u{22D7}
gtreqless;\u{1}\u{22DB}
gtreqqless;\u{1}\u{2A8C}
gtrless;\u{1}\u{2277}
gtrsim;\u{1}\u{2273}
gvertneqq;\u{1}\u{2269}\u{FE00}
gvnE;\u{1}\u{2269}\u{FE00}
hArr;\u{1}\u{21D4}
hairsp;\u{1}\u{200A}
half;\u{1}\u{BD}
hamilt;\u{1}\u{210B}
hardcy;\u{1}\u{44A}
harr;\u{1}\u{2194}
harrcir;\u{1}\u{2948}
harrw;\u{1}\u{21AD}
hbar;\u{1}\u{210F}
hcirc;\u{1}\u{125}
hearts;\u{1}\u{2665}
heartsuit;\u{1}\u{2665}
hellip;\u{1}\u{2026}
hercon;\u{1}\u{22B9}
hfr;\u{1}\u{1D525}
hksearow;\u{1}\u{2925}
hkswarow;\u{1}\u{2926}
hoarr;\u{1}\u{21FF}
homtht;\u{1}\u{223B}
hookleftarrow;\u{1}\u{21A9}
hookrightarrow;\u{1}\u{21AA}
hopf;\u{1}\u{1D559}
horbar;\u{1}\u{2015}
hscr;\u{1}\u{1D4BD}
hslash;\u{1}\u{210F}
hstrok;\u{1}\u{127}
hybull;\u{1}\u{2043}
hyphen;\u{1}\u{2010}
iacute\u{1}\u{ED}
iacute;\u{1}\u{ED}
ic;\u{1}\u{2063}
icirc\u{1}\u{EE}
icirc;\u{1}\u{EE}
icy;\u{1}\u{438}
iecy;\u{1}\u{435}
iexcl\u{1}\u{A1}
iexcl;\u{1}\u{A1}
iff;\u{1}\u{21D4}
ifr;\u{1}\u{1D526}
igrave\u{1}\u{EC}
igrave;\u{1}\u{EC}
ii;\u{1}\u{2148}
iiiint;\u{1}\u{2A0C}
iiint;\u{1}\u{222D}
iinfin;\u{1}\u{29DC}
iiota;\u{1}\u{2129}
ijlig;\u{1}\u{133}
imacr;\u{1}\u{12B}
image;\u{1}\u{2111}
imagline;\u{1}\u{2110}
imagpart;\u{1}\u{2111}
imath;\u{1}\u{131}
imof;\u{1}\u{22B7}
imped;\u{1}\u{1B5}
in;\u{1}\u{2208}
incare;\u{1}\u{2105}
infin;\u{1}\u{221E}
infintie;\u{1}\u{29DD}
inodot;\u{1}\u{131}
int;\u{1}\u{222B}
intcal;\u{1}\u{22BA}
integers;\u{1}\u{2124}
intercal;\u{1}\u{22BA}
intlarhk;\u{1}\u{2A17}
intprod;\u{1}\u{2A3C}
iocy;\u{1}\u{451}
iogon;\u{1}\u{12F}
iopf;\u{1}\u{1D55A}
iota;\u{1}\u{3B9}
iprod;\u{1}\u{2A3C}
iquest\u{1}\u{BF}
iquest;\u{1}\u{BF}
iscr;\u{1}\u{1D4BE}
isin;\u{1}\u{2208}
isinE;\u{1}\u{22F9}
isindot;\u{1}\u{22F5}
isins;\u{1}\u{22F4}
isinsv;\u{1}\u{22F3}
isinv;\u{1}\u{2208}
it;\u{1}\u{2062}
itilde;\u{1}\u{129}
iukcy;\u{1}\u{456}
iuml\u{1}\u{EF}
iuml;\u{1}\u{EF}
jcirc;\u{1}\u{135}
jcy;\u{1}\u{439}
jfr;\u{1}\u{1D527}
jmath;\u{1}\u{237}
jopf;\u{1}\u{1D55B}
jscr;\u{1}\u{1D4BF}
jsercy;\u{1}\u{458}
jukcy;\u{1}\u{454}
kappa;\u{1}\u{3BA}
kappav;\u{1}\u{3F0}
kcedil;\u{1}\u{137}
kcy;\u{1}\u{43A}
kfr;\u{1}\u{1D528}
kgreen;\u{1}\u{138}
khcy;\u{1}\u{445}
kjcy;\u{1}\u{45C}
kopf;\u{1}\u{1D55C}
kscr;\u{1}\u{1D4C0}
lAarr;\u{1}\u{21DA}
lArr;\u{1}\u{21D0}
lAtail;\u{1}\u{291B}
lBarr;\u{1}\u{290E}
lE;\u{1}\u{2266}
lEg;\u{1}\u{2A8B}
lHar;\u{1}\u{2962}
lacute;\u{1}\u{13A}
laemptyv;\u{1}\u{29B4}
lagran;\u{1}\u{2112}
lambda;\u{1}\u{3BB}
lang;\u{1}\u{27E8}
langd;\u{1}\u{2991}
langle;\u{1}\u{27E8}
lap;\u{1}\u{2A85}
laquo\u{1}\u{AB}
laquo;\u{1}\u{AB}
larr;\u{1}\u{2190}
larrb;\u{1}\u{21E4}
larrbfs;\u{1}\u{291F}
larrfs;\u{1}\u{291D}
larrhk;\u{1}\u{21A9}
larrlp;\u{1}\u{21AB}
larrpl;\u{1}\u{2939}
larrsim;\u{1}\u{2973}
larrtl;\u{1}\u{21A2}
lat;\u{1}\u{2AAB}
latail;\u{1}\u{2919}
late;\u{1}\u{2AAD}
lates;\u{1}\u{2AAD}\u{FE00}
lbarr;\u{1}\u{290C}
lbbrk;\u{1}\u{2772}
lbrace;\u{1}{
lbrack;\u{1}[
lbrke;\u{1}\u{298B}
lbrksld;\u{1}\u{298F}
lbrkslu;\u{1}\u{298D}
lcaron;\u{1}\u{13E}
lcedil;\u{1}\u{13C}
lceil;\u{1}\u{2308}
lcub;\u{1}{
lcy;\u{1}\u{43B}
ldca;\u{1}\u{2936}
ldquo;\u{1}\u{201C}
ldquor;\u{1}\u{201E}
ldrdhar;\u{1}\u{2967}
ldrushar;\u{1}\u{294B}
ldsh;\u{1}\u{21B2}
le;\u{1}\u{2264}
leftarrow;\u{1}\u{2190}
leftarrowtail;\u{1}\u{21A2}
leftharpoondown;\u{1}\u{21BD}
leftharpoonup;\u{1}\u{21BC}
leftleftarrows;\u{1}\u{21C7}
leftrightarrow;\u{1}\u{2194}
leftrightarrows;\u{1}\u{21C6}
leftrightharpoons;\u{1}\u{21CB}
leftrightsquigarrow;\u{1}\u{21AD}
leftthreetimes;\u{1}\u{22CB}
leg;\u{1}\u{22DA}
leq;\u{1}\u{2264}
leqq;\u{1}\u{2266}
leqslant;\u{1}\u{2A7D}
les;\u{1}\u{2A7D}
lescc;\u{1}\u{2AA8}
lesdot;\u{1}\u{2A7F}
lesdoto;\u{1}\u{2A81}
lesdotor;\u{1}\u{2A83}
lesg;\u{1}\u{22DA}\u{FE00}
lesges;\u{1}\u{2A93}
lessapprox;\u{1}\u{2A85}
lessdot;\u{1}\u{22D6}
lesseqgtr;\u{1}\u{22DA}
lesseqqgtr;\u{1}\u{2A8B}
lessgtr;\u{1}\u{2276}
lesssim;\u{1}\u{2272}
lfisht;\u{1}\u{297C}
lfloor;\u{1}\u{230A}
lfr;\u{1}\u{1D529}
lg;\u{1}\u{2276}
lgE;\u{1}\u{2A91}
lhard;\u{1}\u{21BD}
lharu;\u{1}\u{21BC}
lharul;\u{1}\u{296A}
lhblk;\u{1}\u{2584}
ljcy;\u{1}\u{459}
ll;\u{1}\u{226A}
llarr;\u{1}\u{21C7}
llcorner;\u{1}\u{231E}
llhard;\u{1}\u{296B}
lltri;\u{1}\u{25FA}
lmidot;\u{1}\u{140}
lmoust;\u{1}\u{23B0}
lmoustache;\u{1}\u{23B0}
lnE;\u{1}\u{2268}
lnap;\u{1}\u{2A89}
lnapprox;\u{1}\u{2A89}
lne;\u{1}\u{2A87}
lneq;\u{1}\u{2A87}
lneqq;\u{1}\u{2268}
lnsim;\u{1}\u{22E6}
loang;\u{1}\u{27EC}
loarr;\u{1}\u{21FD}
lobrk;\u{1}\u{27E6}
longleftarrow;\u{1}\u{27F5}
longleftrightarrow;\u{1}\u{27F7}
longmapsto;\u{1}\u{27FC}
longrightarrow;\u{1}\u{27F6}
looparrowleft;\u{1}\u{21AB}
looparrowright;\u{1}\u{21AC}
lopar;\u{1}\u{2985}
lopf;\u{1}\u{1D55D}
loplus;\u{1}\u{2A2D}
lotimes;\u{1}\u{2A34}
lowast;\u{1}\u{2217}
lowbar;\u{1}_
loz;\u{1}\u{25CA}
lozenge;\u{1}\u{25CA}
lozf;\u{1}\u{29EB}
lpar;\u{1}(
lparlt;\u{1}\u{2993}
lrarr;\u{1}\u{21C6}
lrcorner;\u{1}\u{231F}
lrhar;\u{1}\u{21CB}
lrhard;\u{1}\u{296D}
lrm;\u{1}\u{200E}
lrtri;\u{1}\u{22BF}
lsaquo;\u{1}\u{2039}
lscr;\u{1}\u{1D4C1}
lsh;\u{1}\u{21B0}
lsim;\u{1}\u{2272}
lsime;\u{1}\u{2A8D}
lsimg;\u{1}\u{2A8F}
lsqb;\u{1}[
lsquo;\u{1}\u{2018}
lsquor;\u{1}\u{201A}
lstrok;\u{1}\u{142}
lt\u{1}<
lt;\u{1}<
ltcc;\u{1}\u{2AA6}
ltcir;\u{1}\u{2A79}
ltdot;\u{1}\u{22D6}
lthree;\u{1}\u{22CB}
ltimes;\u{1}\u{22C9}
ltlarr;\u{1}\u{2976}
ltquest;\u{1}\u{2A7B}
ltrPar;\u{1}\u{2996}
ltri;\u{1}\u{25C3}
ltrie;\u{1}\u{22B4}
ltrif;\u{1}\u{25C2}
lurdshar;\u{1}\u{294A}
luruhar;\u{1}\u{2966}
lvertneqq;\u{1}\u{2268}\u{FE00}
lvnE;\u{1}\u{2268}\u{FE00}
mDDot;\u{1}\u{223A}
macr\u{1}\u{AF}
macr;\u{1}\u{AF}
male;\u{1}\u{2642}
malt;\u{1}\u{2720}
maltese;\u{1}\u{2720}
map;\u{1}\u{21A6}
mapsto;\u{1}\u{21A6}
mapstodown;\u{1}\u{21A7}
mapstoleft;\u{1}\u{21A4}
mapstoup;\u{1}\u{21A5}
marker;\u{1}\u{25AE}
mcomma;\u{1}\u{2A29}
mcy;\u{1}\u{43C}
mdash;\u{1}\u{2014}
measuredangle;\u{1}\u{2221}
mfr;\u{1}\u{1D52A}
mho;\u{1}\u{2127}
micro\u{1}\u{B5}
micro;\u{1}\u{B5}
mid;\u{1}\u{2223}
midast;\u{1}*
midcir;\u{1}\u{2AF0}
middot\u{1}\u{B7}
middot;\u{1}\u{B7}
minus;\u{1}\u{2212}
minusb;\u{1}\u{229F}
minusd;\u{1}\u{2238}
minusdu;\u{1}\u{2A2A}
mlcp;\u{1}\u{2ADB}
mldr;\u{1}\u{2026}
mnplus;\u{1}\u{2213}
models;\u{1}\u{22A7}
mopf;\u{1}\u{1D55E}
mp;\u{1}\u{2213}
mscr;\u{1}\u{1D4C2}
mstpos;\u{1}\u{223E}
mu;\u{1}\u{3BC}
multimap;\u{1}\u{22B8}
mumap;\u{1}\u{22B8}
nGg;\u{1}\u{22D9}\u{338}
nGtv;\u{1}\u{226B}\u{338}
nLeftarrow;\u{1}\u{21CD}
nLeftrightarrow;\u{1}\u{21CE}
nLl;\u{1}\u{22D8}\u{338}
nLtv;\u{1}\u{226A}\u{338}
nRightarrow;\u{1}\u{21CF}
nVDash;\u{1}\u{22AF}
nVdash;\u{1}\u{22AE}
nabla;\u{1}\u{2207}
nacute;\u{1}\u{144}
nang;\u{1}\u{2220}\u{20D2}
nap;\u{1}\u{2249}
napE;\u{1}\u{2A70}\u{338}
napid;\u{1}\u{224B}\u{338}
napos;\u{1}\u{149}
napprox;\u{1}\u{2249}
natur;\u{1}\u{266E}
natural;\u{1}\u{266E}
naturals;\u{1}\u{2115}
nbsp\u{1}\u{A0}
nbsp;\u{1}\u{A0}
nbump;\u{1}\u{224E}\u{338}
nbumpe;\u{1}\u{224F}\u{338}
ncap;\u{1}\u{2A43}
ncaron;\u{1}\u{148}
ncedil;\u{1}\u{146}
ncong;\u{1}\u{2247}
ncongdot;\u{1}\u{2A6D}\u{338}
ncup;\u{1}\u{2A42}
ncy;\u{1}\u{43D}
ndash;\u{1}\u{2013}
ne;\u{1}\u{2260}
neArr;\u{1}\u{21D7}
nearhk;\u{1}\u{2924}
nearr;\u{1}\u{2197}
nearrow;\u{1}\u{2197}
nedot;\u{1}\u{2250}\u{338}
nequiv;\u{1}\u{2262}
nesear;\u{1}\u{2928}
nesim;\u{1}\u{2242}\u{338}
nexist;\u{1}\u{2204}
nexists;\u{1}\u{2204}
nfr;\u{1}\u{1D52B}
ngE;\u{1}\u{2267}\u{338}
nge;\u{1}\u{2271}
ngeq;\u{1}\u{2271}
ngeqq;\u{1}\u{2267}\u{338}
ngeqslant;\u{1}\u{2A7E}\u{338}
nges;\u{1}\u{2A7E}\u{338}
ngsim;\u{1}\u{2275}
ngt;\u{1}\u{226F}
ngtr;\u{1}\u{226F}
nhArr;\u{1}\u{21CE}
nharr;\u{1}\u{21AE}
nhpar;\u{1}\u{2AF2}
ni;\u{1}\u{220B}
nis;\u{1}\u{22FC}
nisd;\u{1}\u{22FA}
niv;\u{1}\u{220B}
njcy;\u{1}\u{45A}
nlArr;\u{1}\u{21CD}
nlE;\u{1}\u{2266}\u{338}
nlarr;\u{1}\u{219A}
nldr;\u{1}\u{2025}
nle;\u{1}\u{2270}
nleftarrow;\u{1}\u{219A}
nleftrightarrow;\u{1}\u{21AE}
nleq;\u{1}\u{2270}
nleqq;\u{1}\u{2266}\u{338}
nleqslant;\u{1}\u{2A7D}\u{338}
nles;\u{1}\u{2A7D}\u{338}
nless;\u{1}\u{226E}
nlsim;\u{1}\u{2274}
nlt;\u{1}\u{226E}
nltri;\u{1}\u{22EA}
nltrie;\u{1}\u{22EC}
nmid;\u{1}\u{2224}
nopf;\u{1}\u{1D55F}
not\u{1}\u{AC}
not;\u{1}\u{AC}
notin;\u{1}\u{2209}
notinE;\u{1}\u{22F9}\u{338}
notindot;\u{1}\u{22F5}\u{338}
notinva;\u{1}\u{2209}
notinvb;\u{1}\u{22F7}
notinvc;\u{1}\u{22F6}
notni;\u{1}\u{220C}
notniva;\u{1}\u{220C}
notnivb;\u{1}\u{22FE}
notnivc;\u{1}\u{22FD}
npar;\u{1}\u{2226}
nparallel;\u{1}\u{2226}
nparsl;\u{1}\u{2AFD}\u{20E5}
npart;\u{1}\u{2202}\u{338}
npolint;\u{1}\u{2A14}
npr;\u{1}\u{2280}
nprcue;\u{1}\u{22E0}
npre;\u{1}\u{2AAF}\u{338}
nprec;\u{1}\u{2280}
npreceq;\u{1}\u{2AAF}\u{338}
nrArr;\u{1}\u{21CF}
nrarr;\u{1}\u{219B}
nrarrc;\u{1}\u{2933}\u{338}
nrarrw;\u{1}\u{219D}\u{338}
nrightarrow;\u{1}\u{219B}
nrtri;\u{1}\u{22EB}
nrtrie;\u{1}\u{22ED}
nsc;\u{1}\u{2281}
nsccue;\u{1}\u{22E1}
nsce;\u{1}\u{2AB0}\u{338}
nscr;\u{1}\u{1D4C3}
nshortmid;\u{1}\u{2224}
nshortparallel;\u{1}\u{2226}
nsim;\u{1}\u{2241}
nsime;\u{1}\u{2244}
nsimeq;\u{1}\u{2244}
nsmid;\u{1}\u{2224}
nspar;\u{1}\u{2226}
nsqsube;\u{1}\u{22E2}
nsqsupe;\u{1}\u{22E3}
nsub;\u{1}\u{2284}
nsubE;\u{1}\u{2AC5}\u{338}
nsube;\u{1}\u{2288}
nsubset;\u{1}\u{2282}\u{20D2}
nsubseteq;\u{1}\u{2288}
nsubseteqq;\u{1}\u{2AC5}\u{338}
nsucc;\u{1}\u{2281}
nsucceq;\u{1}\u{2AB0}\u{338}
nsup;\u{1}\u{2285}
nsupE;\u{1}\u{2AC6}\u{338}
nsupe;\u{1}\u{2289}
nsupset;\u{1}\u{2283}\u{20D2}
nsupseteq;\u{1}\u{2289}
nsupseteqq;\u{1}\u{2AC6}\u{338}
ntgl;\u{1}\u{2279}
ntilde\u{1}\u{F1}
ntilde;\u{1}\u{F1}
ntlg;\u{1}\u{2278}
ntriangleleft;\u{1}\u{22EA}
ntrianglelefteq;\u{1}\u{22EC}
ntriangleright;\u{1}\u{22EB}
ntrianglerighteq;\u{1}\u{22ED}
nu;\u{1}\u{3BD}
num;\u{1}#
numero;\u{1}\u{2116}
numsp;\u{1}\u{2007}
nvDash;\u{1}\u{22AD}
nvHarr;\u{1}\u{2904}
nvap;\u{1}\u{224D}\u{20D2}
nvdash;\u{1}\u{22AC}
nvge;\u{1}\u{2265}\u{20D2}
nvgt;\u{1}>\u{20D2}
nvinfin;\u{1}\u{29DE}
nvlArr;\u{1}\u{2902}
nvle;\u{1}\u{2264}\u{20D2}
nvlt;\u{1}<\u{20D2}
nvltrie;\u{1}\u{22B4}\u{20D2}
nvrArr;\u{1}\u{2903}
nvrtrie;\u{1}\u{22B5}\u{20D2}
nvsim;\u{1}\u{223C}\u{20D2}
nwArr;\u{1}\u{21D6}
nwarhk;\u{1}\u{2923}
nwarr;\u{1}\u{2196}
nwarrow;\u{1}\u{2196}
nwnear;\u{1}\u{2927}
oS;\u{1}\u{24C8}
oacute\u{1}\u{F3}
oacute;\u{1}\u{F3}
oast;\u{1}\u{229B}
ocir;\u{1}\u{229A}
ocirc\u{1}\u{F4}
ocirc;\u{1}\u{F4}
ocy;\u{1}\u{43E}
odash;\u{1}\u{229D}
odblac;\u{1}\u{151}
odiv;\u{1}\u{2A38}
odot;\u{1}\u{2299}
odsold;\u{1}\u{29BC}
oelig;\u{1}\u{153}
ofcir;\u{1}\u{29BF}
ofr;\u{1}\u{1D52C}
ogon;\u{1}\u{2DB}
ograve\u{1}\u{F2}
ograve;\u{1}\u{F2}
ogt;\u{1}\u{29C1}
ohbar;\u{1}\u{29B5}
ohm;\u{1}\u{3A9}
oint;\u{1}\u{222E}
olarr;\u{1}\u{21BA}
olcir;\u{1}\u{29BE}
olcross;\u{1}\u{29BB}
oline;\u{1}\u{203E}
olt;\u{1}\u{29C0}
omacr;\u{1}\u{14D}
omega;\u{1}\u{3C9}
omicron;\u{1}\u{3BF}
omid;\u{1}\u{29B6}
ominus;\u{1}\u{2296}
oopf;\u{1}\u{1D560}
opar;\u{1}\u{29B7}
operp;\u{1}\u{29B9}
oplus;\u{1}\u{2295}
or;\u{1}\u{2228}
orarr;\u{1}\u{21BB}
ord;\u{1}\u{2A5D}
order;\u{1}\u{2134}
orderof;\u{1}\u{2134}
ordf\u{1}\u{AA}
ordf;\u{1}\u{AA}
ordm\u{1}\u{BA}
ordm;\u{1}\u{BA}
origof;\u{1}\u{22B6}
oror;\u{1}\u{2A56}
orslope;\u{1}\u{2A57}
orv;\u{1}\u{2A5B}
oscr;\u{1}\u{2134}
oslash\u{1}\u{F8}
oslash;\u{1}\u{F8}
osol;\u{1}\u{2298}
otilde\u{1}\u{F5}
otilde;\u{1}\u{F5}
otimes;\u{1}\u{2297}
otimesas;\u{1}\u{2A36}
ouml\u{1}\u{F6}
ouml;\u{1}\u{F6}
ovbar;\u{1}\u{233D}
par;\u{1}\u{2225}
para\u{1}\u{B6}
para;\u{1}\u{B6}
parallel;\u{1}\u{2225}
parsim;\u{1}\u{2AF3}
parsl;\u{1}\u{2AFD}
part;\u{1}\u{2202}
pcy;\u{1}\u{43F}
percnt;\u{1}%
period;\u{1}.
permil;\u{1}\u{2030}
perp;\u{1}\u{22A5}
pertenk;\u{1}\u{2031}
pfr;\u{1}\u{1D52D}
phi;\u{1}\u{3C6}
phiv;\u{1}\u{3D5}
phmmat;\u{1}\u{2133}
phone;\u{1}\u{260E}
pi;\u{1}\u{3C0}
pitchfork;\u{1}\u{22D4}
piv;\u{1}\u{3D6}
planck;\u{1}\u{210F}
planckh;\u{1}\u{210E}
plankv;\u{1}\u{210F}
plus;\u{1}+
plusacir;\u{1}\u{2A23}
plusb;\u{1}\u{229E}
pluscir;\u{1}\u{2A22}
plusdo;\u{1}\u{2214}
plusdu;\u{1}\u{2A25}
pluse;\u{1}\u{2A72}
plusmn\u{1}\u{B1}
plusmn;\u{1}\u{B1}
plussim;\u{1}\u{2A26}
plustwo;\u{1}\u{2A27}
pm;\u{1}\u{B1}
pointint;\u{1}\u{2A15}
popf;\u{1}\u{1D561}
pound\u{1}\u{A3}
pound;\u{1}\u{A3}
pr;\u{1}\u{227A}
prE;\u{1}\u{2AB3}
prap;\u{1}\u{2AB7}
prcue;\u{1}\u{227C}
pre;\u{1}\u{2AAF}
prec;\u{1}\u{227A}
precapprox;\u{1}\u{2AB7}
preccurlyeq;\u{1}\u{227C}
preceq;\u{1}\u{2AAF}
precnapprox;\u{1}\u{2AB9}
precneqq;\u{1}\u{2AB5}
precnsim;\u{1}\u{22E8}
precsim;\u{1}\u{227E}
prime;\u{1}\u{2032}
primes;\u{1}\u{2119}
prnE;\u{1}\u{2AB5}
prnap;\u{1}\u{2AB9}
prnsim;\u{1}\u{22E8}
prod;\u{1}\u{220F}
profalar;\u{1}\u{232E}
profline;\u{1}\u{2312}
profsurf;\u{1}\u{2313}
prop;\u{1}\u{221D}
propto;\u{1}\u{221D}
prsim;\u{1}\u{227E}
prurel;\u{1}\u{22B0}
pscr;\u{1}\u{1D4C5}
psi;\u{1}\u{3C8}
puncsp;\u{1}\u{2008}
qfr;\u{1}\u{1D52E}
qint;\u{1}\u{2A0C}
qopf;\u{1}\u{1D562}
qprime;\u{1}\u{2057}
qscr;\u{1}\u{1D4C6}
quaternions;\u{1}\u{210D}
quatint;\u{1}\u{2A16}
quest;\u{1}?
questeq;\u{1}\u{225F}
quot\u{1}\"
quot;\u{1}\"
rAarr;\u{1}\u{21DB}
rArr;\u{1}\u{21D2}
rAtail;\u{1}\u{291C}
rBarr;\u{1}\u{290F}
rHar;\u{1}\u{2964}
race;\u{1}\u{223D}\u{331}
racute;\u{1}\u{155}
radic;\u{1}\u{221A}
raemptyv;\u{1}\u{29B3}
rang;\u{1}\u{27E9}
rangd;\u{1}\u{2992}
range;\u{1}\u{29A5}
rangle;\u{1}\u{27E9}
raquo\u{1}\u{BB}
raquo;\u{1}\u{BB}
rarr;\u{1}\u{2192}
rarrap;\u{1}\u{2975}
rarrb;\u{1}\u{21E5}
rarrbfs;\u{1}\u{2920}
rarrc;\u{1}\u{2933}
rarrfs;\u{1}\u{291E}
rarrhk;\u{1}\u{21AA}
rarrlp;\u{1}\u{21AC}
rarrpl;\u{1}\u{2945}
rarrsim;\u{1}\u{2974}
rarrtl;\u{1}\u{21A3}
rarrw;\u{1}\u{219D}
ratail;\u{1}\u{291A}
ratio;\u{1}\u{2236}
rationals;\u{1}\u{211A}
rbarr;\u{1}\u{290D}
rbbrk;\u{1}\u{2773}
rbrace;\u{1}}
rbrack;\u{1}]
rbrke;\u{1}\u{298C}
rbrksld;\u{1}\u{298E}
rbrkslu;\u{1}\u{2990}
rcaron;\u{1}\u{159}
rcedil;\u{1}\u{157}
rceil;\u{1}\u{2309}
rcub;\u{1}}
rcy;\u{1}\u{440}
rdca;\u{1}\u{2937}
rdldhar;\u{1}\u{2969}
rdquo;\u{1}\u{201D}
rdquor;\u{1}\u{201D}
rdsh;\u{1}\u{21B3}
real;\u{1}\u{211C}
realine;\u{1}\u{211B}
realpart;\u{1}\u{211C}
reals;\u{1}\u{211D}
rect;\u{1}\u{25AD}
reg\u{1}\u{AE}
reg;\u{1}\u{AE}
rfisht;\u{1}\u{297D}
rfloor;\u{1}\u{230B}
rfr;\u{1}\u{1D52F}
rhard;\u{1}\u{21C1}
rharu;\u{1}\u{21C0}
rharul;\u{1}\u{296C}
rho;\u{1}\u{3C1}
rhov;\u{1}\u{3F1}
rightarrow;\u{1}\u{2192}
rightarrowtail;\u{1}\u{21A3}
rightharpoondown;\u{1}\u{21C1}
rightharpoonup;\u{1}\u{21C0}
rightleftarrows;\u{1}\u{21C4}
rightleftharpoons;\u{1}\u{21CC}
rightrightarrows;\u{1}\u{21C9}
rightsquigarrow;\u{1}\u{219D}
rightthreetimes;\u{1}\u{22CC}
ring;\u{1}\u{2DA}
risingdotseq;\u{1}\u{2253}
rlarr;\u{1}\u{21C4}
rlhar;\u{1}\u{21CC}
rlm;\u{1}\u{200F}
rmoust;\u{1}\u{23B1}
rmoustache;\u{1}\u{23B1}
rnmid;\u{1}\u{2AEE}
roang;\u{1}\u{27ED}
roarr;\u{1}\u{21FE}
robrk;\u{1}\u{27E7}
ropar;\u{1}\u{2986}
ropf;\u{1}\u{1D563}
roplus;\u{1}\u{2A2E}
rotimes;\u{1}\u{2A35}
rpar;\u{1})
rpargt;\u{1}\u{2994}
rppolint;\u{1}\u{2A12}
rrarr;\u{1}\u{21C9}
rsaquo;\u{1}\u{203A}
rscr;\u{1}\u{1D4C7}
rsh;\u{1}\u{21B1}
rsqb;\u{1}]
rsquo;\u{1}\u{2019}
rsquor;\u{1}\u{2019}
rthree;\u{1}\u{22CC}
rtimes;\u{1}\u{22CA}
rtri;\u{1}\u{25B9}
rtrie;\u{1}\u{22B5}
rtrif;\u{1}\u{25B8}
rtriltri;\u{1}\u{29CE}
ruluhar;\u{1}\u{2968}
rx;\u{1}\u{211E}
sacute;\u{1}\u{15B}
sbquo;\u{1}\u{201A}
sc;\u{1}\u{227B}
scE;\u{1}\u{2AB4}
scap;\u{1}\u{2AB8}
scaron;\u{1}\u{161}
sccue;\u{1}\u{227D}
sce;\u{1}\u{2AB0}
scedil;\u{1}\u{15F}
scirc;\u{1}\u{15D}
scnE;\u{1}\u{2AB6}
scnap;\u{1}\u{2ABA}
scnsim;\u{1}\u{22E9}
scpolint;\u{1}\u{2A13}
scsim;\u{1}\u{227F}
scy;\u{1}\u{441}
sdot;\u{1}\u{22C5}
sdotb;\u{1}\u{22A1}
sdote;\u{1}\u{2A66}
seArr;\u{1}\u{21D8}
searhk;\u{1}\u{2925}
searr;\u{1}\u{2198}
searrow;\u{1}\u{2198}
sect\u{1}\u{A7}
sect;\u{1}\u{A7}
semi;\u{1};
seswar;\u{1}\u{2929}
setminus;\u{1}\u{2216}
setmn;\u{1}\u{2216}
sext;\u{1}\u{2736}
sfr;\u{1}\u{1D530}
sfrown;\u{1}\u{2322}
sharp;\u{1}\u{266F}
shchcy;\u{1}\u{449}
shcy;\u{1}\u{448}
shortmid;\u{1}\u{2223}
shortparallel;\u{1}\u{2225}
shy\u{1}\u{AD}
shy;\u{1}\u{AD}
sigma;\u{1}\u{3C3}
sigmaf;\u{1}\u{3C2}
sigmav;\u{1}\u{3C2}
sim;\u{1}\u{223C}
simdot;\u{1}\u{2A6A}
sime;\u{1}\u{2243}
simeq;\u{1}\u{2243}
simg;\u{1}\u{2A9E}
simgE;\u{1}\u{2AA0}
siml;\u{1}\u{2A9D}
simlE;\u{1}\u{2A9F}
simne;\u{1}\u{2246}
simplus;\u{1}\u{2A24}
simrarr;\u{1}\u{2972}
slarr;\u{1}\u{2190}
smallsetminus;\u{1}\u{2216}
smashp;\u{1}\u{2A33}
smeparsl;\u{1}\u{29E4}
smid;\u{1}\u{2223}
smile;\u{1}\u{2323}
smt;\u{1}\u{2AAA}
smte;\u{1}\u{2AAC}
smtes;\u{1}\u{2AAC}\u{FE00}
softcy;\u{1}\u{44C}
sol;\u{1}/
solb;\u{1}\u{29C4}
solbar;\u{1}\u{233F}
sopf;\u{1}\u{1D564}
spades;\u{1}\u{2660}
spadesuit;\u{1}\u{2660}
spar;\u{1}\u{2225}
sqcap;\u{1}\u{2293}
sqcaps;\u{1}\u{2293}\u{FE00}
sqcup;\u{1}\u{2294}
sqcups;\u{1}\u{2294}\u{FE00}
sqsub;\u{1}\u{228F}
sqsube;\u{1}\u{2291}
sqsubset;\u{1}\u{228F}
sqsubseteq;\u{1}\u{2291}
sqsup;\u{1}\u{2290}
sqsupe;\u{1}\u{2292}
sqsupset;\u{1}\u{2290}
sqsupseteq;\u{1}\u{2292}
squ;\u{1}\u{25A1}
square;\u{1}\u{25A1}
squarf;\u{1}\u{25AA}
squf;\u{1}\u{25AA}
srarr;\u{1}\u{2192}
sscr;\u{1}\u{1D4C8}
ssetmn;\u{1}\u{2216}
ssmile;\u{1}\u{2323}
sstarf;\u{1}\u{22C6}
star;\u{1}\u{2606}
starf;\u{1}\u{2605}
straightepsilon;\u{1}\u{3F5}
straightphi;\u{1}\u{3D5}
strns;\u{1}\u{AF}
sub;\u{1}\u{2282}
subE;\u{1}\u{2AC5}
subdot;\u{1}\u{2ABD}
sube;\u{1}\u{2286}
subedot;\u{1}\u{2AC3}
submult;\u{1}\u{2AC1}
subnE;\u{1}\u{2ACB}
subne;\u{1}\u{228A}
subplus;\u{1}\u{2ABF}
subrarr;\u{1}\u{2979}
subset;\u{1}\u{2282}
subseteq;\u{1}\u{2286}
subseteqq;\u{1}\u{2AC5}
subsetneq;\u{1}\u{228A}
subsetneqq;\u{1}\u{2ACB}
subsim;\u{1}\u{2AC7}
subsub;\u{1}\u{2AD5}
subsup;\u{1}\u{2AD3}
succ;\u{1}\u{227B}
succapprox;\u{1}\u{2AB8}
succcurlyeq;\u{1}\u{227D}
succeq;\u{1}\u{2AB0}
succnapprox;\u{1}\u{2ABA}
succneqq;\u{1}\u{2AB6}
succnsim;\u{1}\u{22E9}
succsim;\u{1}\u{227F}
sum;\u{1}\u{2211}
sung;\u{1}\u{266A}
sup1\u{1}\u{B9}
sup1;\u{1}\u{B9}
sup2\u{1}\u{B2}
sup2;\u{1}\u{B2}
sup3\u{1}\u{B3}
sup3;\u{1}\u{B3}
sup;\u{1}\u{2283}
supE;\u{1}\u{2AC6}
supdot;\u{1}\u{2ABE}
supdsub;\u{1}\u{2AD8}
supe;\u{1}\u{2287}
supedot;\u{1}\u{2AC4}
suphsol;\u{1}\u{27C9}
suphsub;\u{1}\u{2AD7}
suplarr;\u{1}\u{297B}
supmult;\u{1}\u{2AC2}
supnE;\u{1}\u{2ACC}
supne;\u{1}\u{228B}
supplus;\u{1}\u{2AC0}
supset;\u{1}\u{2283}
supseteq;\u{1}\u{2287}
supseteqq;\u{1}\u{2AC6}
supsetneq;\u{1}\u{228B}
supsetneqq;\u{1}\u{2ACC}
supsim;\u{1}\u{2AC8}
supsub;\u{1}\u{2AD4}
supsup;\u{1}\u{2AD6}
swArr;\u{1}\u{21D9}
swarhk;\u{1}\u{2926}
swarr;\u{1}\u{2199}
swarrow;\u{1}\u{2199}
swnwar;\u{1}\u{292A}
szlig\u{1}\u{DF}
szlig;\u{1}\u{DF}
target;\u{1}\u{2316}
tau;\u{1}\u{3C4}
tbrk;\u{1}\u{23B4}
tcaron;\u{1}\u{165}
tcedil;\u{1}\u{163}
tcy;\u{1}\u{442}
tdot;\u{1}\u{20DB}
telrec;\u{1}\u{2315}
tfr;\u{1}\u{1D531}
there4;\u{1}\u{2234}
therefore;\u{1}\u{2234}
theta;\u{1}\u{3B8}
thetasym;\u{1}\u{3D1}
thetav;\u{1}\u{3D1}
thickapprox;\u{1}\u{2248}
thicksim;\u{1}\u{223C}
thinsp;\u{1}\u{2009}
thkap;\u{1}\u{2248}
thksim;\u{1}\u{223C}
thorn\u{1}\u{FE}
thorn;\u{1}\u{FE}
tilde;\u{1}\u{2DC}
times\u{1}\u{D7}
times;\u{1}\u{D7}
timesb;\u{1}\u{22A0}
timesbar;\u{1}\u{2A31}
timesd;\u{1}\u{2A30}
tint;\u{1}\u{222D}
toea;\u{1}\u{2928}
top;\u{1}\u{22A4}
topbot;\u{1}\u{2336}
topcir;\u{1}\u{2AF1}
topf;\u{1}\u{1D565}
topfork;\u{1}\u{2ADA}
tosa;\u{1}\u{2929}
tprime;\u{1}\u{2034}
trade;\u{1}\u{2122}
triangle;\u{1}\u{25B5}
triangledown;\u{1}\u{25BF}
triangleleft;\u{1}\u{25C3}
trianglelefteq;\u{1}\u{22B4}
triangleq;\u{1}\u{225C}
triangleright;\u{1}\u{25B9}
trianglerighteq;\u{1}\u{22B5}
tridot;\u{1}\u{25EC}
trie;\u{1}\u{225C}
triminus;\u{1}\u{2A3A}
triplus;\u{1}\u{2A39}
trisb;\u{1}\u{29CD}
tritime;\u{1}\u{2A3B}
trpezium;\u{1}\u{23E2}
tscr;\u{1}\u{1D4C9}
tscy;\u{1}\u{446}
tshcy;\u{1}\u{45B}
tstrok;\u{1}\u{167}
twixt;\u{1}\u{226C}
twoheadleftarrow;\u{1}\u{219E}
twoheadrightarrow;\u{1}\u{21A0}
uArr;\u{1}\u{21D1}
uHar;\u{1}\u{2963}
uacute\u{1}\u{FA}
uacute;\u{1}\u{FA}
uarr;\u{1}\u{2191}
ubrcy;\u{1}\u{45E}
ubreve;\u{1}\u{16D}
ucirc\u{1}\u{FB}
ucirc;\u{1}\u{FB}
ucy;\u{1}\u{443}
udarr;\u{1}\u{21C5}
udblac;\u{1}\u{171}
udhar;\u{1}\u{296E}
ufisht;\u{1}\u{297E}
ufr;\u{1}\u{1D532}
ugrave\u{1}\u{F9}
ugrave;\u{1}\u{F9}
uharl;\u{1}\u{21BF}
uharr;\u{1}\u{21BE}
uhblk;\u{1}\u{2580}
ulcorn;\u{1}\u{231C}
ulcorner;\u{1}\u{231C}
ulcrop;\u{1}\u{230F}
ultri;\u{1}\u{25F8}
umacr;\u{1}\u{16B}
uml\u{1}\u{A8}
uml;\u{1}\u{A8}
uogon;\u{1}\u{173}
uopf;\u{1}\u{1D566}
uparrow;\u{1}\u{2191}
updownarrow;\u{1}\u{2195}
upharpoonleft;\u{1}\u{21BF}
upharpoonright;\u{1}\u{21BE}
uplus;\u{1}\u{228E}
upsi;\u{1}\u{3C5}
upsih;\u{1}\u{3D2}
upsilon;\u{1}\u{3C5}
upuparrows;\u{1}\u{21C8}
urcorn;\u{1}\u{231D}
urcorner;\u{1}\u{231D}
urcrop;\u{1}\u{230E}
uring;\u{1}\u{16F}
urtri;\u{1}\u{25F9}
uscr;\u{1}\u{1D4CA}
utdot;\u{1}\u{22F0}
utilde;\u{1}\u{169}
utri;\u{1}\u{25B5}
utrif;\u{1}\u{25B4}
uuarr;\u{1}\u{21C8}
uuml\u{1}\u{FC}
uuml;\u{1}\u{FC}
uwangle;\u{1}\u{29A7}
vArr;\u{1}\u{21D5}
vBar;\u{1}\u{2AE8}
vBarv;\u{1}\u{2AE9}
vDash;\u{1}\u{22A8}
vangrt;\u{1}\u{299C}
varepsilon;\u{1}\u{3F5}
varkappa;\u{1}\u{3F0}
varnothing;\u{1}\u{2205}
varphi;\u{1}\u{3D5}
varpi;\u{1}\u{3D6}
varpropto;\u{1}\u{221D}
varr;\u{1}\u{2195}
varrho;\u{1}\u{3F1}
varsigma;\u{1}\u{3C2}
varsubsetneq;\u{1}\u{228A}\u{FE00}
varsubsetneqq;\u{1}\u{2ACB}\u{FE00}
varsupsetneq;\u{1}\u{228B}\u{FE00}
varsupsetneqq;\u{1}\u{2ACC}\u{FE00}
vartheta;\u{1}\u{3D1}
vartriangleleft;\u{1}\u{22B2}
vartriangleright;\u{1}\u{22B3}
vcy;\u{1}\u{432}
vdash;\u{1}\u{22A2}
vee;\u{1}\u{2228}
veebar;\u{1}\u{22BB}
veeeq;\u{1}\u{225A}
vellip;\u{1}\u{22EE}
verbar;\u{1}|
vert;\u{1}|
vfr;\u{1}\u{1D533}
vltri;\u{1}\u{22B2}
vnsub;\u{1}\u{2282}\u{20D2}
vnsup;\u{1}\u{2283}\u{20D2}
vopf;\u{1}\u{1D567}
vprop;\u{1}\u{221D}
vrtri;\u{1}\u{22B3}
vscr;\u{1}\u{1D4CB}
vsubnE;\u{1}\u{2ACB}\u{FE00}
vsubne;\u{1}\u{228A}\u{FE00}
vsupnE;\u{1}\u{2ACC}\u{FE00}
vsupne;\u{1}\u{228B}\u{FE00}
vzigzag;\u{1}\u{299A}
wcirc;\u{1}\u{175}
wedbar;\u{1}\u{2A5F}
wedge;\u{1}\u{2227}
wedgeq;\u{1}\u{2259}
weierp;\u{1}\u{2118}
wfr;\u{1}\u{1D534}
wopf;\u{1}\u{1D568}
wp;\u{1}\u{2118}
wr;\u{1}\u{2240}
wreath;\u{1}\u{2240}
wscr;\u{1}\u{1D4CC}
xcap;\u{1}\u{22C2}
xcirc;\u{1}\u{25EF}
xcup;\u{1}\u{22C3}
xdtri;\u{1}\u{25BD}
xfr;\u{1}\u{1D535}
xhArr;\u{1}\u{27FA}
xharr;\u{1}\u{27F7}
xi;\u{1}\u{3BE}
xlArr;\u{1}\u{27F8}
xlarr;\u{1}\u{27F5}
xmap;\u{1}\u{27FC}
xnis;\u{1}\u{22FB}
xodot;\u{1}\u{2A00}
xopf;\u{1}\u{1D569}
xoplus;\u{1}\u{2A01}
xotime;\u{1}\u{2A02}
xrArr;\u{1}\u{27F9}
xrarr;\u{1}\u{27F6}
xscr;\u{1}\u{1D4CD}
xsqcup;\u{1}\u{2A06}
xuplus;\u{1}\u{2A04}
xutri;\u{1}\u{25B3}
xvee;\u{1}\u{22C1}
xwedge;\u{1}\u{22C0}
yacute\u{1}\u{FD}
yacute;\u{1}\u{FD}
yacy;\u{1}\u{44F}
ycirc;\u{1}\u{177}
ycy;\u{1}\u{44B}
yen\u{1}\u{A5}
yen;\u{1}\u{A5}
yfr;\u{1}\u{1D536}
yicy;\u{1}\u{457}
yopf;\u{1}\u{1D56A}
yscr;\u{1}\u{1D4CE}
yucy;\u{1}\u{44E}
yuml\u{1}\u{FF}
yuml;\u{1}\u{FF}
zacute;\u{1}\u{17A}
zcaron;\u{1}\u{17E}
zcy;\u{1}\u{437}
zdot;\u{1}\u{17C}
zeetrf;\u{1}\u{2128}
zeta;\u{1}\u{3B6}
zfr;\u{1}\u{1D537}
zhcy;\u{1}\u{436}
zigrarr;\u{1}\u{21DD}
zopf;\u{1}\u{1D56B}
zscr;\u{1}\u{1D4CF}
zwj;\u{1}\u{200D}
zwnj;\u{1}\u{200C}
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
