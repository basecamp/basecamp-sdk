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
        if sgid.contains(where: { markupUnsafeCharacters.contains($0) }) {
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
        var present = Set(attachmentSgids(in: content))
        var tags: [String] = []
        for person in people {
            // Rendered before the dedupe check, not after: a person the content
            // already mentions is still refused when their own sgid is unusable,
            // so a caller cannot be told "nothing to do" about a broken Person.
            let tag = try markup(for: person)
            let sgid = person.attachableSgid ?? ""
            guard present.insert(sgid).inserted else { continue }
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
private let markupUnsafeCharacters: Set<Character> = ["\"", "'", "<", ">", "&"]

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
        let value = sgid.trimmingCharacters(in: .whitespacesAndNewlines)
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
        var normalized = payload.replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        while normalized.hasSuffix("=") { normalized.removeLast() }
        guard let raw = decodeUnpaddedBase64(normalized), !raw.isEmpty,
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
    static func personId(fromGlobalId gid: String) -> Int? {
        guard let schemeEnd = gid.range(of: "://") else { return nil }
        guard gid[gid.startIndex..<schemeEnd.lowerBound].lowercased() == "gid" else { return nil }
        var rest = Substring(gid[schemeEnd.upperBound...])
        // Drop the query and fragment a URL parser would keep out of the path.
        if let cut = rest.firstIndex(where: { $0 == "?" || $0 == "#" }) { rest = rest[..<cut] }
        guard let hostEnd = rest.firstIndex(of: "/"), hostEnd != rest.startIndex else { return nil }

        let path = rest[rest.index(after: hostEnd)...]
        guard let modelEnd = path.firstIndex(of: "/") else { return nil }
        guard path[..<modelEnd] == "Person" else { return nil }

        let rawId = path[path.index(after: modelEnd)...]
        guard !rawId.isEmpty, rawId.allSatisfy({ $0.isASCII && $0.isNumber }) else { return nil }
        guard let id = Int(rawId), id > 0 else { return nil }
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
    while i + pattern.count <= bytes.count {
        if Array(bytes[i..<(i + pattern.count)]) == pattern { return i }
        i += 1
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
/// Deliberately narrower than a full HTML5 named-reference table: the value this
/// is applied to is an `attachable_sgid`, which is base64url plus `--` plus hex.
/// The five named references below are the ones a serializer actually emits, and
/// the numeric forms cover the rest. An unrecognized `&…;` is left verbatim,
/// which is also what a browser does with one.
private func unescapeEntities(_ value: String) -> String {
    guard value.contains("&") else { return value }

    var out = ""
    out.reserveCapacity(value.count)
    var rest = Substring(value)
    while let amp = rest.firstIndex(of: "&") {
        out += rest[..<amp]
        let after = rest.index(after: amp)
        guard let semicolon = rest[after...].firstIndex(of: ";"),
            rest.distance(from: after, to: semicolon) <= 8
        else {
            out.append("&")
            rest = rest[after...]
            continue
        }
        let body = rest[after..<semicolon]
        if let replacement = entityReplacement(String(body)) {
            out.append(replacement)
        } else {
            out += "&\(body);"
        }
        rest = rest[rest.index(after: semicolon)...]
    }
    out += rest
    return out
}

private func entityReplacement(_ body: String) -> Character? {
    switch body {
    case "amp": return "&"
    case "lt": return "<"
    case "gt": return ">"
    case "quot": return "\""
    case "apos": return "'"
    default: break
    }
    guard body.hasPrefix("#") else { return nil }
    let digits = body.dropFirst()
    let value: UInt32?
    if digits.first == "x" || digits.first == "X" {
        value = UInt32(digits.dropFirst(), radix: 16)
    } else {
        value = UInt32(digits, radix: 10)
    }
    guard let value, let scalar = Unicode.Scalar(value) else { return nil }
    return Character(scalar)
}
