package com.basecamp.sdk

import com.basecamp.sdk.generated.models.Person
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

/*
 * Mention helpers over Basecamp rich text.
 *
 * A mention in Basecamp rich text is a `<bc-attachment>` whose `sgid` attribute
 * is the mentioned person's `attachable_sgid` (`doc/api/sections/rich_text.md`,
 * "Inserting a mention"). BC3 renders the same tag back with
 * `content-type="application/vnd.basecamp.mention"` and an avatar figure inside
 * it, but the sgid is the only part of the markup that names the person on both
 * the write and the read side, so both helpers here work from it:
 *
 * - [mentionedPersonIds] reads the person ids a rich text names, by decoding the
 *   sgid of every `<bc-attachment>` and keeping the ones that point at a Person.
 * - [mentionMarkup] writes the tag for a person, from their `attachable_sgid`.
 *
 * An `attachable_sgid` is a Rails SignedGlobalID: a base64 payload, then `--`,
 * then an HMAC only BC3 can verify. The payload is an envelope carrying the
 * global id — `gid://bc3/Person/1049715915` — as a string, and that string is
 * what these helpers read. They do not (and cannot) verify the signature; what
 * they decode is the same person id BC3 renders into the mention's avatar, read
 * off content the API already served, and a caller that needs the id verified
 * reads the person back through `people.get`.
 *
 * That sets a trust boundary between the two sides. READING —
 * [mentionedPersonIds], [personIdFromSgid] — describes what a text says it
 * mentions, and unsigned is fine for description: the ids are reported, not
 * acted on as proof. WRITING — [withMentions],
 * `CommentsService.expandMentions` — never treats an unsigned id as proof that a
 * valid mention already exists: a forged or stale sgid in caller-supplied
 * content naming the right id would otherwise make the writer skip the
 * authoritative people read and post a tag Basecamp will not honour, so the
 * person is silently not mentioned. `CommentsService.expandMentions` therefore
 * resolves every requested person through `people.get` and deduplicates only
 * against the exact `attachable_sgid` string that read returned. The pure
 * helpers beneath it — [withMentions], [mentionMarkup] — take [Person] values
 * the caller built and can only check that an sgid is well-formed and names the
 * person it is given, never that it is authentic: hand them people the API
 * returned, not people assembled from content. Do not reuse the read-side
 * helpers to decide whether a write can be skipped.
 *
 * The markup is read as BC3 serves it: a sanitized tree of the tags
 * `doc/api/sections/rich_text.md` allows, which has no raw-text elements. The
 * tag walk skips comments and quoted attribute values but does not model
 * `<script>` or `<style>` content, which BC3 strips on write; a caller reading
 * mentions out of content it authored itself should not put a `bc-attachment`
 * inside such an element and expect it ignored.
 *
 * The envelope is decoded structurally, never searched as bytes, so a Person gid
 * that merely appears inside some other value — a Document gid built from one, a
 * purpose string that looks like one — is not a mention, and the envelope's
 * purpose must be `attachable`, the one BC3 accepts in rich text. Three
 * envelopes are read: Rails' current Marshal layout `{"_rails" => {"data" =>
 * gid, "pur" => purpose}}`, the older Marshal layout `{"gid" => gid, "purpose"
 * => …, "expires_at" => …}`, and the JSON spelling of either, which Rails' JSON
 * message serializer emits.
 */

/**
 * Returns the ids of the people [richText] mentions: the Person named by the
 * sgid of each `<bc-attachment>`, in document order, with repeats removed.
 * Attachments that are not mentions — files, images, embeds — are skipped, as is
 * any sgid that does not decode to a Person.
 *
 * This is the read side: a description of what the text says, from sgids whose
 * signatures cannot be checked here. Report it; do not treat an id in it as
 * proof that a valid mention exists (see the trust boundary above).
 *
 * Every `<bc-attachment>` in the text counts, including one inside a
 * `<blockquote>`: BC3 notifies quoted mentions too, so the read matches what the
 * server does with the write.
 */
fun mentionedPersonIds(richText: String): List<Long> {
    val ids = mutableListOf<Long>()
    val seen = mutableSetOf<Long>()
    for (sgid in bcAttachmentSgids(richText)) {
        val id = personIdFromSgid(sgid) ?: continue
        if (seen.add(id)) ids.add(id)
    }
    return ids
}

/**
 * Decodes the Person id an `attachable_sgid` names, or null when the sgid does
 * not decode, or names something other than a Person (a file attachment's sgid
 * names an `ActiveStorage::Blob`).
 *
 * This reads the id out of the sgid's payload; it does not verify the sgid's
 * signature, which only BC3 can. It is a read-side helper: never use its answer
 * to decide that a write may skip the authoritative people read (see the trust
 * boundary above).
 */
fun personIdFromSgid(sgid: String): Long? {
    val gid = globalIdFromSgid(sgid) ?: return null
    val (model, rawId) = globalIdModelAndId(gid) ?: return null
    if (model != "Person" || rawId.isEmpty()) return null
    if (rawId.any { it < '0' || it > '9' }) return null
    val id = rawId.toLongOrNull() ?: return null
    return if (id > 0) id else null
}

/**
 * Renders the `<bc-attachment>` that mentions [person], from their
 * `attachable_sgid` — the write-side form in `doc/api/sections/rich_text.md`,
 * which BC3 expands into the avatar figure on read.
 *
 * Throws [BasecampException.Usage] when the person carries no
 * `attachable_sgid`, which is the case for a [Person] projection that came from
 * somewhere other than a people read (a webhook payload, say), and when the sgid
 * does not name the person it is given. That is all it can check: it cannot
 * verify the signature, so the [Person] must come from the API — a `people.get`,
 * a recording's creator or assignees — not be assembled from an sgid found in
 * content.
 */
fun mentionMarkup(person: Person): String {
    val sgid = person.attachableSgid
    if (sgid.isNullOrEmpty()) {
        throw BasecampException.Usage(
            "person ${person.id} has no attachable_sgid to mention",
            "read the person through people.get to obtain one",
        )
    }
    if (sgid.any { it in "\"'<>&" }) {
        throw BasecampException.Usage("person ${person.id} has a malformed attachable_sgid")
    }
    // The tag mentions whoever the sgid names. Refuse to write one that names
    // someone else — or a file — under this person's id.
    if (personIdFromSgid(sgid) != person.id) {
        throw BasecampException.Usage(
            "person ${person.id}'s attachable_sgid does not name that person",
            "read the person through people.get to obtain their own",
        )
    }
    return "<bc-attachment sgid=\"$sgid\"></bc-attachment>"
}

/**
 * Returns [content] mentioning each of [people], for posting as a comment or a
 * Campfire line. A person whose exact `attachable_sgid` the content already
 * carries is left alone, so passing the same person twice — or a person the
 * author already mentioned with that sgid — never duplicates the mention; the
 * rest are added at the start of the content, inside its first `<p>` or `<div>`
 * when it opens with one, so they render on the first line rather than as a
 * block of their own.
 *
 * This is the write side, and it deduplicates on the sgid string alone, never on
 * the person id an existing tag's sgid decodes to: that id is unsigned, and a
 * forged or stale tag naming the right person must not stand in for the real
 * mention (see the trust boundary above). Every person needs their own
 * `attachable_sgid`, and it must be one the API returned: this helper can check
 * that an sgid is well-formed and names the person, not that it is authentic
 * (see [mentionMarkup]). The account-bound `CommentsService.expandMentions`
 * resolves ids to people first and is the entry point that carries that
 * guarantee.
 */
fun withMentions(content: String, people: List<Person>): String {
    val present = bcAttachmentSgids(content).toMutableSet()
    val tags = mutableListOf<String>()
    for (person in people) {
        // Rendered before the duplicate check, so a person whose sgid is missing
        // or names someone else is refused whether or not the content already
        // carries that string.
        val tag = mentionMarkup(person)
        val sgid = person.attachableSgid ?: continue
        if (!present.add(sgid)) continue
        tags.add(tag)
    }
    if (tags.isEmpty()) return content
    val prefix = tags.joinToString(" ") + " "
    val end = leadingBlockEnd(content)
    return if (end >= 0) content.substring(0, end) + prefix + content.substring(end) else prefix + content
}

// --- Markup scanning ---------------------------------------------------------

/**
 * Returns the `sgid` attribute of every `<bc-attachment>` in [text], in document
 * order. It walks the markup as a stream of tags rather than pattern-matching
 * for one tag name, so a `<bc-attachment>` inside an HTML comment or inside
 * another element's quoted attribute is not an element; and it tokenizes each
 * tag's attributes rather than pattern-matching them, so a `>` inside a quoted
 * value does not end the tag, an `sgid=` inside another attribute's value is not
 * an attribute, either quote style works, attribute order and case are free, the
 * first sgid attribute wins as in HTML, and entity escapes in the value are
 * decoded as a browser would.
 */
internal fun bcAttachmentSgids(text: String): List<String> {
    val sgids = mutableListOf<String>()
    var pos = 0
    while (pos < text.length) {
        val open = text.indexOf('<', pos)
        if (open < 0) break
        pos = open + 1
        if (text.startsWith("!--", pos)) {
            val stop = text.indexOf("-->", pos)
            if (stop < 0) return sgids // an unterminated comment swallows the rest
            pos = stop + 3
            continue
        }
        if (text.startsWith("!", pos) || text.startsWith("?", pos) || text.startsWith("/", pos)) {
            val stop = text.indexOf('>', pos)
            if (stop < 0) return sgids
            pos = stop + 1
            continue
        }
        var nameEnd = pos
        while (nameEnd < text.length && isTagNameChar(text[nameEnd])) nameEnd++
        if (nameEnd == pos) continue // a bare "<" in text
        val parsed = parseAttributes(text, nameEnd) ?: return sgids // unterminated tag
        val nameLength = nameEnd - pos
        if (nameLength == BC_ATTACHMENT.length &&
            text.regionMatches(pos, BC_ATTACHMENT, 0, nameLength, ignoreCase = true) &&
            parsed.sgid.isNotEmpty()
        ) {
            sgids.add(parsed.sgid)
        }
        pos = parsed.end
    }
    return sgids
}

private const val BC_ATTACHMENT = "bc-attachment"

/**
 * What may follow `<` in a tag name. The whole name is consumed, punctuation
 * included, so `<bc-attachment:preview` or `<bc-attachment_x` is its own name
 * and never compares equal to `bc-attachment`.
 */
private fun isTagNameChar(c: Char): Boolean =
    !isHtmlSpace(c) && c != '/' && c != '>' && c != '<' && c != '=' && c != '"' && c != '\''

private fun isTagNameEnd(c: Char): Boolean = isHtmlSpace(c) || c == '/' || c == '>'

private fun isHtmlSpace(c: Char): Boolean =
    c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\u000C'

/** What [parseAttributes] read off one opening tag. */
private class TagAttributes(val sgid: String, val end: Int)

/**
 * Walks the attributes of an opening tag from [pos] (just after the tag name) to
 * its closing `>`, returning the first `sgid` value found and the index after
 * the `>`, or null when the tag was never closed. The first sgid attribute wins,
 * present-but-empty included, as HTML resolves a repeated attribute.
 */
private fun parseAttributes(text: String, pos: Int): TagAttributes? {
    var p = pos
    var sgid = ""
    var sgidSeen = false
    while (p < text.length) {
        while (p < text.length && (isHtmlSpace(text[p]) || text[p] == '/')) p++
        if (p >= text.length) return null
        if (text[p] == '>') return TagAttributes(sgid, p + 1)
        val nameStart = p
        while (p < text.length && !isHtmlSpace(text[p]) && text[p] != '=' && text[p] != '>' && text[p] != '/') p++
        val name = text.substring(nameStart, p)
        while (p < text.length && isHtmlSpace(text[p])) p++
        var value = ""
        if (p < text.length && text[p] == '=') {
            p++
            while (p < text.length && isHtmlSpace(text[p])) p++
            if (p < text.length && (text[p] == '"' || text[p] == '\'')) {
                val quote = text[p]
                p++
                val closing = text.indexOf(quote, p)
                if (closing < 0) return null
                value = text.substring(p, closing)
                p = closing + 1
            } else {
                val valueStart = p
                while (p < text.length && !isHtmlSpace(text[p]) && text[p] != '>') p++
                value = text.substring(valueStart, p)
            }
        }
        if (name.isEmpty()) {
            // A stray "=" or quote where a name should be: step over it.
            p++
            continue
        }
        if (!sgidSeen && name.equals("sgid", ignoreCase = true)) {
            sgidSeen = true
            sgid = unescapeHtml(value)
        }
    }
    return null
}

/**
 * Decodes the character references a browser resolves inside an attribute value:
 * the five XML named entities and numeric references in either base.
 *
 * Deliberately narrower than the full HTML5 named-entity table, which runs to
 * more than two thousand names, none of which can appear in a well-formed sgid —
 * a base64url payload, `--`, and a hex digest. An unrecognized `&…;` is left as
 * written rather than guessed at, which is also what a browser does for a name it
 * does not know.
 */
private fun unescapeHtml(value: String): String {
    if ('&' !in value) return value
    val out = StringBuilder(value.length)
    var i = 0
    while (i < value.length) {
        val c = value[i]
        if (c != '&') {
            out.append(c)
            i++
            continue
        }
        val end = value.indexOf(';', i + 1)
        // A reference is at most "&#x10FFFF;"; anything longer is literal text.
        if (end < 0 || end - i > 10) {
            out.append(c)
            i++
            continue
        }
        val body = value.substring(i + 1, end)
        val decoded = when {
            body.equals("amp", ignoreCase = true) -> "&"
            body.equals("lt", ignoreCase = true) -> "<"
            body.equals("gt", ignoreCase = true) -> ">"
            body.equals("quot", ignoreCase = true) -> "\""
            body.equals("apos", ignoreCase = true) -> "'"
            body.startsWith("#x", ignoreCase = true) -> body.substring(2).toIntOrNull(16)?.let { codePointOrNull(it) }
            body.startsWith("#") -> body.substring(1).toIntOrNull()?.let { codePointOrNull(it) }
            else -> null
        }
        if (decoded == null) {
            out.append(c)
            i++
        } else {
            out.append(decoded)
            i = end + 1
        }
    }
    return out.toString()
}

/** A numeric reference's string, or null when it names no scalar value. */
private fun codePointOrNull(code: Int): String? {
    if (code <= 0 || code > 0x10FFFF) return null
    if (code in 0xD800..0xDFFF) return null // a lone surrogate is not a character
    if (code <= 0xFFFF) return code.toChar().toString()
    val v = code - 0x10000
    return charArrayOf((0xD800 + (v shr 10)).toChar(), (0xDC00 + (v and 0x3FF)).toChar()).concatToString()
}

/**
 * Returns the index just past the opening `<p …>` or `<div …>` tag [content]
 * starts with, or -1 when it starts with anything else, so mentions can be
 * placed inside the first block rather than as a bare prefix in front of it. The
 * tag's attributes are scanned quote-aware: a `>` inside an attribute value does
 * not end it.
 */
internal fun leadingBlockEnd(content: String): Int {
    var i = 0
    while (i < content.length && isHtmlSpace(content[i])) i++
    for (name in LEADING_BLOCK_TAGS) {
        if (content.length < i + name.length) continue
        if (!content.regionMatches(i, name, 0, name.length, ignoreCase = true)) continue
        val after = i + name.length
        if (after < content.length && !isTagNameEnd(content[after])) continue
        return parseAttributes(content, after)?.end ?: -1
    }
    return -1
}

private val LEADING_BLOCK_TAGS = listOf("<p", "<div")

// --- SignedGlobalID decoding -------------------------------------------------

/**
 * The SignedGlobalID purpose BC3 mints attachable sgids with
 * (`doc/api/sections/rich_text.md`: `attachable_sgid`). Pinned by the purpose
 * cases in `MentionsTest`, so a rename upstream breaks a test here rather than
 * silently turning every mention invisible.
 */
private const val SGID_PURPOSE_ATTACHABLE = "attachable"

/**
 * Bounds the decoded sgid payload. A Person sgid's payload is under 200 bytes;
 * the cap keeps a hostile one from costing more than its own size to reject.
 */
private const val MAX_SGID_PAYLOAD_BYTES = 4096

/**
 * The same bound on the base64 form (4/3 of the payload, plus padding), checked
 * before anything is allocated.
 */
private const val MAX_SGID_ENCODED_BYTES = MAX_SGID_PAYLOAD_BYTES / 3 * 4 + 4

/** Bounds nesting in a Marshal payload; an envelope is two deep. */
private const val RUBY_MARSHAL_MAX_DEPTH = 32

/**
 * Returns the global id string an sgid's envelope carries.
 *
 * A signed sgid is `<payload>--<digest>`, and `-` is a base64url character, so
 * the payload itself may contain `--`. The separator is therefore the LAST one,
 * as Rails' own verifier reads it; the whole value is tried as a bare payload
 * when that fails, which is what an unsigned envelope — one that happens to
 * contain `--` included — needs.
 */
private fun globalIdFromSgid(sgid: String): String? {
    val value = sgid.trim()
    val i = value.lastIndexOf("--")
    if (i > 0) {
        envelopeGid(value.substring(0, i))?.let { return it }
    }
    return envelopeGid(value)
}

/** Decodes one base64 payload and returns the gid its envelope carries. */
private fun envelopeGid(payload: String): String? {
    // The bound is applied to the encoded form first, so an oversized sgid costs
    // nothing to refuse — no normalization, no decode buffer.
    if (payload.isEmpty() || payload.length > MAX_SGID_ENCODED_BYTES) return null
    // Rails' MessageVerifier emits either alphabet; base64url is current. Both
    // decode through the standard alphabet once the two symbols are mapped, and
    // stripping the padding lets a truncated-but-valid payload through.
    val normalized = payload.replace('-', '+').replace('_', '/').trimEnd('=')
    val raw = decodeBase64(normalized) ?: return null
    if (raw.isEmpty() || raw.size > MAX_SGID_PAYLOAD_BYTES) return null
    val envelope: Any? = when {
        raw.size >= 2 && raw[0] == MARSHAL_MAJOR && raw[1] == MARSHAL_MINOR ->
            RubyMarshalReader(raw, 2).readWhole() ?: return null
        raw[0] == '{'.code.toByte() ->
            runCatching { Json.parseToJsonElement(raw.decodeToString()) }.getOrNull()?.let { jsonToPlain(it) }
                ?: return null
        else -> return null
    }
    val top = envelope as? Map<*, *> ?: return null

    // A SignedGlobalID is bound to a purpose, and only an "attachable" one may be
    // placed in rich text: BC3 refuses any other, so a Person sgid minted for
    // bookmarking or reading is not a mention however valid its gid. Both layouts
    // carry the purpose; an envelope without one is not a Rails envelope.
    //
    // Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
    val rails = top["_rails"]
    if (rails is Map<*, *>) {
        if (rails["pur"] != SGID_PURPOSE_ATTACHABLE) return null
        return (rails["data"] as? String)?.takeIf { it.isNotEmpty() }
    }
    // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
    if (top["purpose"] != SGID_PURPOSE_ATTACHABLE) return null
    return (top["gid"] as? String)?.takeIf { it.isNotEmpty() }
}

private const val MARSHAL_MAJOR: Byte = 0x04
private const val MARSHAL_MINOR: Byte = 0x08

/**
 * Splits a GlobalID into its model and id.
 *
 * The scheme must be `gid` (case-insensitively, as a URL parser normalizes it),
 * the authority must be non-empty, and the path must be exactly `/<Model>/<id>`:
 * no more, no less. A query or fragment is stripped the way a URL parser strips
 * it, and percent escapes in the path are decoded, so the comparison is against
 * the path a parser would report rather than against its spelling.
 */
private fun globalIdModelAndId(gid: String): Pair<String, String>? {
    val schemeEnd = gid.indexOf(':')
    if (schemeEnd != 3) return null
    if (!gid.regionMatches(0, "gid", 0, 3, ignoreCase = true)) return null
    if (!gid.startsWith("://", schemeEnd)) return null
    var rest = gid.substring(schemeEnd + 3)
    rest.indexOf('#').let { if (it >= 0) rest = rest.substring(0, it) }
    rest.indexOf('?').let { if (it >= 0) rest = rest.substring(0, it) }
    val slash = rest.indexOf('/')
    if (slash <= 0) return null // an empty authority names no app
    val path = percentDecode(rest.substring(slash + 1)) ?: return null
    val sep = path.indexOf('/')
    if (sep < 0) return null
    return path.substring(0, sep) to path.substring(sep + 1)
}

/** Decodes `%XX` escapes, or null when one is malformed (as a URL parser errors). */
private fun percentDecode(s: String): String? {
    if ('%' !in s) return s
    val out = StringBuilder(s.length)
    var i = 0
    while (i < s.length) {
        val c = s[i]
        if (c != '%') {
            out.append(c)
            i++
            continue
        }
        if (i + 2 >= s.length) return null
        val hex = s.substring(i + 1, i + 3).toIntOrNull(16) ?: return null
        out.append(hex.toChar())
        i += 3
    }
    return out.toString()
}

/** Decodes an unpadded standard-alphabet base64 string, or null when it is not one. */
private fun decodeBase64(s: String): ByteArray? {
    if (s.isEmpty()) return null
    val out = ByteArray(s.length * 3 / 4)
    var written = 0
    var buffer = 0
    var bits = 0
    for (c in s) {
        val v = base64Value(c)
        if (v < 0) return null
        buffer = (buffer shl 6) or v
        bits += 6
        if (bits >= 8) {
            bits -= 8
            out[written++] = ((buffer shr bits) and 0xFF).toByte()
        }
    }
    // A trailing group of six leftover bits cannot be part of any byte, and the
    // bits a smaller remainder carries must be zero — what a canonical encoder
    // writes before the padding this already stripped.
    if (bits >= 6) return null
    if (bits > 0 && (buffer and ((1 shl bits) - 1)) != 0) return null
    return if (written == out.size) out else out.copyOf(written)
}

private fun base64Value(c: Char): Int = when (c) {
    in 'A'..'Z' -> c - 'A'
    in 'a'..'z' -> c - 'a' + 26
    in '0'..'9' -> c - '0' + 52
    '+' -> 62
    '/' -> 63
    else -> -1
}

/** Projects a decoded JSON envelope onto the plain values the envelope reader walks. */
private fun jsonToPlain(element: JsonElement): Any? = when (element) {
    is JsonObject -> element.entries.associate { (k, v) -> k to jsonToPlain(v) }
    is JsonArray -> element.map { jsonToPlain(it) }
    is JsonPrimitive ->
        if (element.isString) {
            element.content
        } else {
            when (val raw = element.contentOrNull) {
                null, "null" -> null
                "true" -> true
                "false" -> false
                else -> raw.toLongOrNull() ?: raw
            }
        }
}

/**
 * Decodes the subset of Ruby's Marshal 4.8 format a SignedGlobalID payload uses —
 * nil, booleans, fixnums, strings (with their encoding ivars), symbols and symbol
 * links, arrays and hashes — into plain Kotlin values: `Map<String, Any?>`,
 * `List<Any?>`, [String], [Long], [Boolean], null. Anything else fails; the
 * caller then treats the sgid as undecodable rather than guessing.
 */
private class RubyMarshalReader(private val data: ByteArray, private var pos: Int) {
    private val symbols = mutableListOf<String>()

    /**
     * Reads the payload's single value, requiring that nothing follows it.
     *
     * Returns null when the payload does not decode, and [NULL_MARKER] when it
     * decodes to Ruby nil — a bare null would conflate the two. Either way the
     * caller's `as? Map` rejects it; the marker keeps them readable apart.
     */
    fun readWhole(): Any? {
        val value = try {
            value(0)
        } catch (e: MarshalFormatException) {
            return null
        }
        // A Marshal dump is exactly one value; bytes after it are corruption.
        if (pos != data.size) return null
        return value ?: NULL_MARKER
    }

    private class MarshalFormatException(message: String) : Exception(message)

    private fun fail(message: String): Nothing = throw MarshalFormatException(message)

    private fun byte(): Int {
        if (pos >= data.size) fail("marshal: unexpected end of data")
        return data[pos++].toInt() and 0xFF
    }

    /**
     * Takes the next [n] bytes. The bound is checked against what remains, never
     * by adding [n] to the position, so a hostile length cannot overflow into a
     * passing check. Every length reaches here through [count], which already
     * rejected anything past the remaining bytes.
     */
    private fun bytes(n: Int): ByteArray {
        if (n < 0 || n > data.size - pos) fail("marshal: unexpected end of data")
        val slice = data.copyOfRange(pos, pos + n)
        pos += n
        return slice
    }

    /**
     * Reads Marshal's packed integer: 0 is 0; 1..4 and -1..-4 are a byte count
     * for a little-endian value; anything else is the value itself offset by 5.
     */
    private fun packedInt(): Long {
        val b = byte()
        // The lead byte is a signed int8; widen it into its signed meaning.
        val c = if (b > 127) (b - 256).toLong() else b.toLong()
        return when {
            c == 0L -> 0L
            c > 4 -> c - 5
            c < -4 -> c + 5
            c > 0 -> {
                val raw = bytes(c.toInt())
                var x = 0L
                for (i in raw.indices) x = x or ((raw[i].toLong() and 0xFF) shl (8 * i))
                x
            }
            else -> {
                val raw = bytes((-c).toInt())
                var x = -1L
                for (i in raw.indices) {
                    x = x and (0xFFL shl (8 * i)).inv()
                    x = x or ((raw[i].toLong() and 0xFF) shl (8 * i))
                }
                x
            }
        }
    }

    /**
     * Reads a length or count — string and symbol bytes, array elements, hash
     * pairs, ivar pairs — and rejects one that cannot be honest: negative, or
     * more than the bytes left (every element takes at least one byte).
     * Allocation follows what actually decodes, so a hostile count costs its own
     * bytes to refuse, never the capacity it claims.
     */
    private fun count(): Int {
        val n = packedInt()
        if (n < 0 || n > (data.size - pos).toLong()) fail("marshal: bad count $n")
        return n.toInt()
    }

    private fun value(depth: Int): Any? {
        if (depth > RUBY_MARSHAL_MAX_DEPTH) fail("marshal: nesting too deep")
        return when (val t = byte().toChar()) {
            '0' -> null
            'T' -> true
            'F' -> false
            'i' -> packedInt()
            '"' -> bytes(count()).decodeToString()
            ':' -> bytes(count()).decodeToString().also { symbols.add(it) }
            ';' -> {
                val idx = packedInt()
                if (idx < 0 || idx >= symbols.size) fail("marshal: bad symbol link")
                symbols[idx.toInt()]
            }
            'I' -> {
                // An object followed by its instance variables — a String's encoding.
                val inner = value(depth + 1)
                repeat(count()) {
                    value(depth + 1) // ivar name
                    value(depth + 1) // ivar value
                }
                inner
            }
            '[' -> {
                val n = count()
                val out = mutableListOf<Any?>()
                repeat(n) { out.add(value(depth + 1)) }
                out
            }
            '{' -> {
                val n = count()
                val out = mutableMapOf<String, Any?>()
                repeat(n) {
                    val k = value(depth + 1) as? String ?: fail("marshal: non-string hash key")
                    out[k] = value(depth + 1)
                }
                out
            }
            else -> fail("marshal: unsupported type '$t'")
        }
    }

    private companion object {
        val NULL_MARKER = Any()
    }
}
