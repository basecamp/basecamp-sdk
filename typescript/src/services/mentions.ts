/**
 * Mention helpers over Basecamp rich text.
 *
 * A mention in Basecamp rich text is a `<bc-attachment>` whose `sgid` attribute
 * is the mentioned person's `attachable_sgid` (doc/api/sections/rich_text.md,
 * "Inserting a mention"). BC3 renders the same tag back with
 * `content-type="application/vnd.basecamp.mention"` and an avatar figure inside
 * it, but the sgid is the only part of the markup that names the person on both
 * the write and the read side, so both helpers here work from it:
 *
 * - {@link mentionedPersonIds} reads the person ids a rich text names, by
 *   decoding the sgid of every `<bc-attachment>` and keeping the ones that
 *   point at a Person.
 * - {@link mentionMarkup} writes the tag for a person, from their
 *   `attachable_sgid`.
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
 * {@link mentionedPersonIds}, {@link personIdFromSGID} — describes what a text
 * says it mentions, and unsigned is fine for description: the ids are reported,
 * not acted on as proof. WRITING — {@link withMentions},
 * `CommentsService.expandMentions` — never treats an unsigned id as proof that
 * a valid mention already exists: a forged or stale sgid in caller-supplied
 * content naming the right id would otherwise make the writer skip the
 * authoritative people read and post a tag Basecamp will not honour, so the
 * person is silently not mentioned. `CommentsService.expandMentions` therefore
 * resolves every requested person through `people.get` and deduplicates only
 * against the exact `attachable_sgid` string that read returned. The pure
 * helpers beneath it — {@link withMentions}, {@link mentionMarkup} — take
 * Person values the caller built and can only check that an sgid is
 * well-formed and names the person it is given, never that it is authentic:
 * hand them people the API returned, not people assembled from content. Do not
 * reuse the read-side helpers to decide whether a write can be skipped.
 *
 * The markup is read as BC3 serves it: a sanitized tree of the tags
 * doc/api/sections/rich_text.md allows, which has no raw-text elements. The tag
 * walk skips comments and quoted attribute values but does not model `<script>`
 * or `<style>` content, which BC3 strips on write; a caller reading mentions
 * out of content it authored itself should not put a `bc-attachment` inside
 * such an element and expect it ignored.
 *
 * The envelope is decoded structurally, never searched as bytes, so a Person
 * gid that merely appears inside some other value — a Document gid built from
 * one, a purpose string that looks like one — is not a mention, and the
 * envelope's purpose must be `attachable`, the one BC3 accepts in rich text.
 * Three envelopes are read: Rails' current Marshal layout
 * `{"_rails" => {"data" => gid, "pur" => purpose}}`, the older Marshal layout
 * `{"gid" => gid, "purpose" => …, "expires_at" => …}`, and the JSON spelling of
 * either, which Rails' JSON message serializer emits.
 */

import { Errors } from "../errors.js";
import type { Person } from "../generated/services/people.js";

// =============================================================================
// Reading mentions
// =============================================================================

/**
 * The ids of the people a rich text mentions: the Person named by the sgid of
 * each `<bc-attachment>`, in document order, with repeats removed. Attachments
 * that are not mentions — files, images, embeds — are skipped, as is any sgid
 * that does not decode to a Person.
 *
 * This is the read side: a description of what the text says, from sgids whose
 * signatures cannot be checked here. Report it; do not treat an id in it as
 * proof that a valid mention exists (see the trust boundary above).
 *
 * Every `<bc-attachment>` in the text counts, including one inside a
 * `<blockquote>`: BC3 notifies quoted mentions too, so the read matches what
 * the server does with the write.
 */
export function mentionedPersonIds(richText: string): number[] {
  const ids: number[] = [];
  const seen = new Set<number>();
  for (const sgid of bcAttachmentSGIDs(richText)) {
    const id = personIdFromSGID(sgid);
    if (id === undefined || seen.has(id)) continue;
    seen.add(id);
    ids.push(id);
  }
  return ids;
}

/**
 * A GlobalID URL, split into host and path.
 *
 * The scheme is matched case-insensitively as a URL parser matches it; the host
 * may not carry whitespace, a URL delimiter, or a control character, which is
 * what a URL parser refuses there — the control range is the point of the
 * class, not an accident, hence the lint suppression.
 */
// oxlint-disable-next-line no-control-regex
const GID_URL = /^gid:\/\/([^/?#\s"<>\\^`{|}\u0000-\u001f\u007f]+)(\/[^?#]*)?(?:[?#][\s\S]*)?$/i;

/**
 * Whether an authority is one `url.Parse` would accept.
 *
 * The character class in {@link GID_URL} models what may not APPEAR in a host;
 * this models the one structural rule on top of it, which a character class
 * cannot express: everything after the last `:` must be a port, meaning digits
 * or nothing. Without it `gid://bc3:abc/Person/1` names a person here and fails
 * to parse in Go — a regex imitating a parser being wrong in the place a
 * character class cannot see. An IPv6 literal is bracketed, and the port rule
 * applies to what follows the bracket, exactly as `net/url` reads it.
 */
function isParsableHost(authority: string): boolean {
  // Userinfo comes off first, as `url.Parse` splits it at the LAST "@" before
  // looking at the host at all — otherwise the colon in "user:pass@bc3" reads
  // as a port and the gid is refused, where Go accepts it. A percent-escape is
  // legal in userinfo and not in the host, so the order matters twice over.
  const at = authority.lastIndexOf("@");
  const host = at < 0 ? authority : authority.slice(at + 1);

  // `net/url` refuses EVERY percent-escape in a host, valid or not: it is not a
  // decode-then-validate, so "b%41c3" is an error there rather than "bAc3".
  if (host.includes("%")) return false;

  if (host.startsWith("[")) {
    const close = host.indexOf("]");
    return close >= 0 && isOptionalPort(host.slice(close + 1));
  }
  const colon = host.lastIndexOf(":");
  return colon < 0 || isOptionalPort(host.slice(colon));
}

/**
 * The path with its percent-escapes resolved, or `undefined` when one of them
 * is malformed.
 *
 * `url.Parse` decodes the path and refuses an escape that is not two hex
 * digits, so `gid://bc3/Pe%72son/1` names a Person in Go and `…/1%zz` is a
 * parse error. Decoding here rather than refusing every escape is what keeps
 * the two agreeing — and it is the opposite of the rule for the HOST, where Go
 * refuses escapes outright. The escapes resolve to BYTES which are then read as
 * UTF-8, as Go reads them, so a multi-byte character spelled in escapes comes
 * back as itself rather than as its bytes.
 */
function percentDecodePath(path: string): string | undefined {
  if (!path.includes("%")) return path;
  const encoder = new TextEncoder();
  const bytes: number[] = [];
  for (let i = 0; i < path.length; ) {
    const code = path.charCodeAt(i);
    if (code !== 0x25 /* % */) {
      for (const byte of encoder.encode(path[i]!)) bytes.push(byte);
      i++;
      continue;
    }
    const high = hexDigit(path.charCodeAt(i + 1), true);
    const low = hexDigit(path.charCodeAt(i + 2), true);
    if (high < 0 || low < 0) return undefined;
    bytes.push(high * 16 + low);
    i += 3;
  }
  return new TextDecoder("utf-8").decode(new Uint8Array(bytes));
}

function isOptionalPort(port: string): boolean {
  return port === "" || (port.startsWith(":") && /^[0-9]*$/.test(port.slice(1)));
}

/**
 * The Person id an `attachable_sgid` names, or `undefined` when the sgid does
 * not decode, or names something other than a Person (a file attachment's sgid
 * names an `ActiveStorage::Blob`).
 *
 * This reads the id out of the sgid's payload; it does not verify the sgid's
 * signature, which only BC3 can. It is a read-side helper: never use its answer
 * to decide that a write may skip the authoritative people read (see the trust
 * boundary in the module comment).
 */
export function personIdFromSGID(sgid: string): number | undefined {
  const gid = globalIDFromSGID(sgid);
  if (gid === undefined) return undefined;

  // A GlobalID is `gid://<app>/<Model>/<id>`, optionally with a query — BC3
  // mints `gid://bc3/Person/1049715915?expires_in=`. The query and fragment are
  // stripped the way a URL parser strips them, and the path must then be
  // exactly `/<Model>/<id>`: no more, no less.
  const parsed = GID_URL.exec(gid);
  if (parsed === null) return undefined;
  if (!isParsableHost(parsed[1] ?? "")) return undefined;
  const path = percentDecodePath(parsed[2] ?? "");
  if (path === undefined) return undefined;
  const separator = path.indexOf("/", 1);
  if (separator < 0) return undefined;
  const model = path.slice(1, separator);
  const rawId = path.slice(separator + 1);
  if (model !== "Person" || rawId === "" || !/^[0-9]+$/.test(rawId)) return undefined;
  const id = Number(rawId);
  // A BC3 person id is well inside the safe range; one that is not cannot be
  // reported as a number without rounding it into a different person.
  if (!Number.isSafeInteger(id) || id <= 0) return undefined;
  return id;
}

/**
 * The `sgid` attribute of every `<bc-attachment>` in the text, in document
 * order.
 *
 * It walks the markup as a stream of tags rather than pattern-matching for one
 * tag name, so a `<bc-attachment>` inside an HTML comment or inside another
 * element's quoted attribute is not an element; and it tokenizes each tag's
 * attributes rather than pattern-matching them, so a `>` inside a quoted value
 * does not end the tag, an `sgid=` inside another attribute's value is not an
 * attribute, either quote style works, attribute order and case are free, the
 * first `sgid` attribute wins as in HTML, and the character references an sgid
 * can carry are decoded — a narrower set than a browser's, for the reason
 * {@link NAMED_ENTITIES} gives.
 */
function bcAttachmentSGIDs(text: string): string[] {
  const sgids: string[] = [];
  let pos = 0;
  while (pos < text.length) {
    const open = text.indexOf("<", pos);
    if (open < 0) break;
    pos = open + 1;

    if (text.startsWith("!--", pos)) {
      const stop = text.indexOf("-->", pos);
      if (stop < 0) return sgids; // an unterminated comment swallows the rest
      pos = stop + 3;
      continue;
    }
    const lead = text.charCodeAt(pos);
    if (lead === CH_BANG || lead === CH_QUESTION || lead === CH_SLASH) {
      const stop = text.indexOf(">", pos);
      if (stop < 0) return sgids;
      pos = stop + 1;
      continue;
    }

    let nameEnd = pos;
    while (nameEnd < text.length && isTagNameChar(text.charCodeAt(nameEnd))) nameEnd++;
    if (nameEnd === pos) continue; // a bare "<" in text

    const attrs = parseAttributes(text, nameEnd);
    if (!attrs.closed) return sgids; // an unterminated tag: nothing after it is markup
    if (text.slice(pos, nameEnd).toLowerCase() === "bc-attachment" && attrs.sgid !== "") {
      sgids.push(attrs.sgid);
    }
    pos = attrs.end;
  }
  return sgids;
}

const CH_TAB = 0x09;
const CH_NEWLINE = 0x0a;
const CH_FORM_FEED = 0x0c;
const CH_RETURN = 0x0d;
const CH_SPACE = 0x20;
const CH_BANG = 0x21;
const CH_DOUBLE_QUOTE = 0x22;
const CH_SINGLE_QUOTE = 0x27;
const CH_SLASH = 0x2f;
const CH_LT = 0x3c;
const CH_EQUALS = 0x3d;
const CH_GT = 0x3e;
const CH_QUESTION = 0x3f;

function isSpace(code: number): boolean {
  return (
    code === CH_SPACE ||
    code === CH_TAB ||
    code === CH_NEWLINE ||
    code === CH_RETURN ||
    code === CH_FORM_FEED
  );
}

/**
 * What may follow `<` in a tag name. The whole name is consumed, punctuation
 * included, so `<bc-attachment:preview` or `<bc-attachment_x` is its own name
 * and never compares equal to `bc-attachment`.
 */
function isTagNameChar(code: number): boolean {
  return (
    !isSpace(code) &&
    code !== CH_SLASH &&
    code !== CH_GT &&
    code !== CH_LT &&
    code !== CH_EQUALS &&
    code !== CH_DOUBLE_QUOTE &&
    code !== CH_SINGLE_QUOTE
  );
}

function isTagNameEnd(code: number): boolean {
  return isSpace(code) || code === CH_SLASH || code === CH_GT;
}

/** What {@link parseAttributes} read off one opening tag. */
interface TagAttributes {
  /** The decoded value of the first `sgid` attribute; `""` when there was none. */
  sgid: string;
  /** The index just past the closing `>`, or where the scan ran out. */
  end: number;
  /** Whether the tag was closed at all. */
  closed: boolean;
}

/**
 * Walks the attributes of an opening tag from `start` (just after the tag name)
 * to its closing `>`. The first `sgid` attribute wins, present-but-empty
 * included, as HTML resolves a repeated attribute.
 */
function parseAttributes(text: string, start: number): TagAttributes {
  let pos = start;
  let sgid = "";
  let sgidSeen = false;

  while (pos < text.length) {
    while (pos < text.length && (isSpace(text.charCodeAt(pos)) || text.charCodeAt(pos) === CH_SLASH)) {
      pos++;
    }
    if (pos >= text.length) return { sgid, end: pos, closed: false };
    if (text.charCodeAt(pos) === CH_GT) return { sgid, end: pos + 1, closed: true };

    const nameStart = pos;
    while (pos < text.length) {
      const code = text.charCodeAt(pos);
      if (isSpace(code) || code === CH_EQUALS || code === CH_GT || code === CH_SLASH) break;
      pos++;
    }
    const name = text.slice(nameStart, pos);

    while (pos < text.length && isSpace(text.charCodeAt(pos))) pos++;

    let value = "";
    if (pos < text.length && text.charCodeAt(pos) === CH_EQUALS) {
      pos++;
      while (pos < text.length && isSpace(text.charCodeAt(pos))) pos++;
      const quote = pos < text.length ? text.charCodeAt(pos) : -1;
      if (quote === CH_DOUBLE_QUOTE || quote === CH_SINGLE_QUOTE) {
        pos++;
        const closing = text.indexOf(String.fromCharCode(quote), pos);
        if (closing < 0) return { sgid, end: text.length, closed: false };
        value = text.slice(pos, closing);
        pos = closing + 1;
      } else {
        const valueStart = pos;
        while (pos < text.length) {
          const code = text.charCodeAt(pos);
          if (isSpace(code) || code === CH_GT) break;
          pos++;
        }
        value = text.slice(valueStart, pos);
      }
    }

    if (name === "") {
      // A stray "=" or quote where a name should be: step over it.
      pos++;
      continue;
    }
    if (!sgidSeen && name.toLowerCase() === "sgid") {
      sgidSeen = true;
      sgid = unescapeEntities(value);
    }
  }
  return { sgid, end: pos, closed: false };
}

/**
 * The named character references that can change what an sgid decodes to.
 *
 * NOT the full HTML5 table Go's `html.UnescapeString` carries. That table has
 * 2231 names; this is the 23 of them whose expansion consists entirely of
 * characters that can matter here — base64 alphabet, padding, or Go whitespace
 * (which {@link trimGoSpace} strips at either end). The subset was extracted
 * from Go's own `html/entity.go` by filtering on that property, not chosen by
 * eye.
 *
 * Every other name expands to something no base64 payload can contain, so Go
 * expands it and refuses the sgid while this leaves it literal and refuses the
 * sgid: different text, same verdict. The keys carry their terminating `;`
 * exactly as Go's table does, which is also why a name outside this subset can
 * never be shadowed by one inside it — `sol;` is not a prefix of `solb;`. The
 * one name Go lists twice, with and without the semicolon, is listed twice
 * here.
 *
 * Two rows come from Go's SECOND table, `entity2`, whose values are rune PAIRS:
 * `&fjlig;` is "fj" — two base64 letters — and `&ThickSpace;` is two spaces. A
 * subset derived by reading only the single-rune table misses them, which is
 * why the enumeration asks `html.UnescapeString` for every value rather than
 * parsing the table's literals.
 */
const NAMED_ENTITIES = new Map<string, string>([
  ["AMP;", "&"],
  ["AMP", "&"],
  ["GT;", ">"],
  ["GT", ">"],
  ["LT;", "<"],
  ["LT", "<"],
  ["MediumSpace;", "\u205f"],
  ["NewLine;", "\n"],
  ["NonBreakingSpace;", "\u00a0"],
  ["QUOT;", '"'],
  ["QUOT", '"'],
  ["Tab;", "\t"],
  ["ThickSpace;", "\u205f\u200a"],
  ["ThinSpace;", "\u2009"],
  ["UnderBar;", "_"],
  ["VeryThinSpace;", "\u200a"],
  ["amp;", "&"],
  ["amp", "&"],
  ["apos;", "'"],
  ["bne;", "=\u20e5"],
  ["emsp13;", "\u2004"],
  ["emsp14;", "\u2005"],
  ["emsp;", "\u2003"],
  ["ensp;", "\u2002"],
  ["equals;", "="],
  ["fjlig;", "fj"],
  ["gt;", ">"],
  ["gt", ">"],
  ["hairsp;", "\u200a"],
  ["lowbar;", "_"],
  ["lt;", "<"],
  ["lt", "<"],
  ["nbsp;", "\u00a0"],
  ["nbsp", "\u00a0"],
  ["numsp;", "\u2007"],
  ["plus;", "+"],
  ["puncsp;", "\u2008"],
  ["quot;", '"'],
  ["quot", '"'],
  ["sol;", "/"],
  ["thinsp;", "\u2009"],
]);

/**
 * The names in {@link NAMED_ENTITIES}, exposed so a test can hold the table to
 * the shape {@link unescapeEntity}'s search bound assumes: `[A-Za-z0-9]+` with
 * an optional `;`. A future row that broke that — a name with a hyphen, say —
 * would silently stop being findable rather than fail to compile.
 */
export function namedEntityNames(): string[] {
  return [...NAMED_ENTITIES.keys()];
}

/** What a character reference's name may be made of, in Go's table and here. */
function isEntityNameChar(code: number): boolean {
  return (
    (code >= 0x30 && code <= 0x39) ||
    (code >= 0x41 && code <= 0x5a) ||
    (code >= 0x61 && code <= 0x7a)
  );
}

/** The longest key in {@link NAMED_ENTITIES}, so the scan knows where to start. */
const LONGEST_ENTITY_NAME = Math.max(...[...NAMED_ENTITIES.keys()].map((name) => name.length));

/**
 * Windows-1252 for U+0080-U+009F, which is what Go substitutes for a numeric
 * reference in that range rather than emitting the C1 control.
 */
const WINDOWS_1252 = [
  0x20ac, 0x0081, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021, 0x02c6, 0x2030, 0x0160,
  0x2039, 0x0152, 0x008d, 0x017d, 0x008f, 0x0090, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022,
  0x2013, 0x2014, 0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, 0x009d, 0x017e, 0x0178,
];

const REPLACEMENT = "\ufffd";

/** Bounds the digits a numeric reference may claim before it is out of range anyway. */
const MAX_NUMERIC_DIGITS = 10;

/**
 * Decodes the character reference at `start` (where the text has `&`), Go's
 * way, returning the replacement text and the index just past what it consumed.
 *
 * The boundary rules are read off `html.UnescapeString` by measurement rather
 * than from its source, because they are not guessable: a decimal reference
 * needs two digits when no `;` follows it and one when it does (`&#6B` is
 * literal, `&#65B` is `"AB"`, `&#6;B` is a tab then `B`); a hex reference needs
 * only one digit either way; `&#x;` with no digits at all is U+FFFD while
 * `&#;` is literal; and a trailing `;` is consumed when present but never
 * required except in those two cases.
 */
function unescapeEntity(text: string, start: number): [string, number] {
  const literal = (): [string, number] => ["&", start + 1];
  if (text.charCodeAt(start + 1) !== 0x23 /* # */) {
    // Bound the search by the name characters actually PRESENT before trying
    // any of them. Every key in the table is `[A-Za-z0-9]+` with an optional
    // `;` (pinned by a test), so nothing longer than that run — plus the
    // semicolon, if one follows it — can match.
    //
    // Searching every length down from the longest key instead is correct but
    // costs a slice and a lookup per length for each `&`, whatever follows it:
    // a run of ampersands is the worst case, it is reachable from any attribute
    // an author writes, and it is invisible in an optimised test run. Measured
    // on 400k ampersands: 245ms before, and the shape stays linear either way,
    // so wall clock alone would not have shown it — the constant is the defect.
    let end = start + 1;
    while (end < text.length && isEntityNameChar(text.charCodeAt(end))) end++;
    const nameLength = end - (start + 1);
    if (nameLength === 0) return literal();
    const semicolon = end < text.length && text.charCodeAt(end) === 0x3b ? 1 : 0;

    for (let length = Math.min(nameLength + semicolon, LONGEST_ENTITY_NAME); length > 0; length--) {
      const replacement = NAMED_ENTITIES.get(text.slice(start + 1, start + 1 + length));
      if (replacement !== undefined) return [replacement, start + 1 + length];
    }
    return literal();
  }

  let pos = start + 2;
  const lead = text.charCodeAt(pos);
  const hex = lead === 0x78 || lead === 0x58; /* x X */
  if (hex) pos++;

  const digitsStart = pos;
  let value = 0;
  while (pos < text.length) {
    const digit = hexDigit(text.charCodeAt(pos), hex);
    if (digit < 0) break;
    if (pos - digitsStart < MAX_NUMERIC_DIGITS) value = value * (hex ? 16 : 10) + digit;
    else value = 0x110000; // out of range, and stays out
    pos++;
  }
  const digits = pos - digitsStart;
  const terminated = pos < text.length && text.charCodeAt(pos) === 0x3b; /* ; */

  if (digits === 0) {
    // `&#x;` is the one zero-digit form Go accepts, as the value zero.
    if (!hex || !terminated) return literal();
  } else if (!hex && digits === 1 && !terminated) {
    return literal();
  }
  if (terminated) pos++;

  if (value >= 0x80 && value <= 0x9f) return [String.fromCodePoint(WINDOWS_1252[value - 0x80]!), pos];
  if (value === 0 || value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff)) {
    return [REPLACEMENT, pos];
  }
  return [String.fromCodePoint(value), pos];
}

function hexDigit(code: number, hex: boolean): number {
  if (code >= 0x30 && code <= 0x39) return code - 0x30;
  if (!hex) return -1;
  if (code >= 0x61 && code <= 0x66) return code - 0x61 + 10;
  if (code >= 0x41 && code <= 0x46) return code - 0x41 + 10;
  return -1;
}

/** Decodes the character references an HTML attribute value may carry. */
function unescapeEntities(value: string): string {
  if (!value.includes("&")) return value;
  let out = "";
  let pos = 0;
  for (;;) {
    const amp = value.indexOf("&", pos);
    if (amp < 0) return out + value.slice(pos);
    out += value.slice(pos, amp);
    const [replacement, next] = unescapeEntity(value, amp);
    out += replacement;
    pos = next;
  }
}

// =============================================================================
// SignedGlobalID envelopes
// =============================================================================

/**
 * The SignedGlobalID purpose BC3 mints attachable sgids with
 * (doc/api/sections/rich_text.md: `attachable_sgid`). Pinned by the purpose
 * cases in the mention tests, so a rename upstream breaks a test here rather
 * than silently turning every mention invisible.
 */
const SGID_PURPOSE_ATTACHABLE = "attachable";

/**
 * Bounds the decoded sgid payload. A Person sgid's payload is under 200 bytes;
 * the cap keeps a hostile one from costing more than its own size to reject.
 */
const MAX_SGID_PAYLOAD_BYTES = 4096;

/**
 * The same bound on the base64 form (4/3 of the payload, plus padding), checked
 * before anything is allocated.
 */
const MAX_SGID_ENCODED_BYTES = Math.floor(MAX_SGID_PAYLOAD_BYTES / 3) * 4 + 4;

/**
 * Go's `unicode.IsSpace` set, which is what `strings.TrimSpace` trims.
 *
 * Neither a subset nor a superset of what `String.prototype.trim` strips, so
 * neither is usable as-is: JS omits U+0085 (NEL), which Go trims, and JS strips
 * U+FEFF (BOM), which Go does not — because the BOM is a format character, not
 * White_Space. Left to `trim()`, an sgid padded with NEL loses its mention here
 * and keeps it in Go, and one padded with a BOM gains a mention here that Go
 * refuses. Both directions matter: one vanishes a real mention, the other
 * invents one.
 */
const GO_SPACE = "\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000";
const GO_TRIM = new RegExp(`^[${GO_SPACE}]+|[${GO_SPACE}]+$`, "gu");

function trimGoSpace(value: string): string {
  return value.replace(GO_TRIM, "");
}

/**
 * The global id string an sgid's envelope carries.
 *
 * A signed sgid is `<payload>--<digest>`, and `-` is a base64url character, so
 * the payload itself may contain `--`. The separator is therefore the LAST one,
 * as Rails' own verifier reads it; the whole value is tried as a bare payload
 * when that fails, which is what an unsigned envelope — one that happens to
 * contain `--` included — needs.
 */
function globalIDFromSGID(sgid: string): string | undefined {
  const value = trimGoSpace(sgid);
  const separator = value.lastIndexOf("--");
  if (separator > 0) {
    const gid = envelopeGID(value.slice(0, separator));
    if (gid !== undefined) return gid;
  }
  return envelopeGID(value);
}

/** Decodes one base64 payload and returns the gid its envelope carries. */
function envelopeGID(payload: string): string | undefined {
  // The bound is applied to the encoded form first, so an oversized sgid costs
  // nothing to refuse — no normalization, no decode buffer.
  if (payload === "" || payload.length > MAX_SGID_ENCODED_BYTES) return undefined;

  const raw = decodeBase64(payload);
  if (raw === undefined || raw.length === 0 || raw.length > MAX_SGID_PAYLOAD_BYTES) return undefined;

  let envelope: unknown;
  if (raw.length >= 2 && raw[0] === 0x04 && raw[1] === 0x08) {
    envelope = unmarshalRuby(raw.subarray(2));
    if (envelope === MARSHAL_FAILED) return undefined;
  } else if (raw[0] === 0x7b /* { */) {
    try {
      // Non-fatal: Go's JSON decoder substitutes U+FFFD for invalid UTF-8
      // inside a string rather than refusing the document, so an envelope with
      // bad bytes in a field other than the gid decodes on both sides.
      envelope = JSON.parse(new TextDecoder("utf-8").decode(raw));
    } catch {
      return undefined;
    }
  } else {
    return undefined;
  }

  if (typeof envelope !== "object" || envelope === null || Array.isArray(envelope)) return undefined;
  const top = envelope as Record<string, unknown>;

  // A SignedGlobalID is bound to a purpose, and only an "attachable" one may be
  // placed in rich text: BC3 refuses any other, so a Person sgid minted for
  // bookmarking or reading is not a mention however valid its gid. Both layouts
  // carry the purpose; an envelope without one is not a Rails envelope.
  //
  // Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
  const rails = top["_rails"];
  if (typeof rails === "object" && rails !== null && !Array.isArray(rails)) {
    const inner = rails as Record<string, unknown>;
    if (inner["pur"] !== SGID_PURPOSE_ATTACHABLE) return undefined;
    const gid = inner["data"];
    return typeof gid === "string" && gid !== "" ? gid : undefined;
  }

  // Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
  if (top["purpose"] !== SGID_PURPOSE_ATTACHABLE) return undefined;
  const gid = top["gid"];
  return typeof gid === "string" && gid !== "" ? gid : undefined;
}

/**
 * Decodes a base64 payload in either alphabet.
 *
 * Rails' MessageVerifier emits base64url (current) or standard base64, and both
 * decode through the standard alphabet once the two symbols are mapped;
 * stripping the padding lets a truncated-but-valid payload through, and
 * `atob` accepts the unpadded form.
 */
function decodeBase64(payload: string): Uint8Array | undefined {
  // The order of these two is Go's, and it is observable. Go right-trims `=`
  // off the string as written and only then decodes, and its decoder ignores CR
  // and LF as it goes — so "MA==" decodes and "MA==\n" does NOT, the trim
  // having stopped at the newline and left an `=` the raw alphabet refuses.
  // Stripping the newlines first would quietly decode that one.
  //
  // The alphabet check afterwards is the other half of the parity: `atob`
  // ignores every ASCII whitespace character, where Go's decoder ignores only
  // CR and LF, so a space or a tab inside an sgid would decode here and be
  // refused there.
  const normalized = payload
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .replace(/=+$/, "")
    .replace(/[\r\n]/g, "");
  if (!/^[A-Za-z0-9+/]*$/.test(normalized)) return undefined;
  let binary: string;
  try {
    binary = atob(normalized);
  } catch {
    return undefined;
  }
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i) & 0xff;
  return bytes;
}

// =============================================================================
// Ruby Marshal (the subset a SignedGlobalID payload uses)
// =============================================================================

/**
 * The sentinel a failed Marshal decode returns.
 *
 * A unique object rather than `undefined` or a thrown error: `nil` is a value
 * the format can legitimately carry, so "failed" needs a token no payload can
 * produce, and the caller then treats the sgid as undecodable rather than
 * guessing.
 */
const MARSHAL_FAILED: unique symbol = Symbol("marshal failed");

/** Bounds nesting in a payload; an envelope is two deep. */
const RUBY_MARSHAL_MAX_DEPTH = 32;

type RubyValue = null | boolean | number | string | RubyValue[] | { [key: string]: RubyValue };

/**
 * Decodes the subset of Ruby's Marshal 4.8 format a SignedGlobalID payload uses
 * — nil, booleans, fixnums, strings (with their encoding ivars), symbols and
 * symbol links, arrays and hashes — into plain JS values. Anything else returns
 * {@link MARSHAL_FAILED}.
 */
function unmarshalRuby(data: Uint8Array): RubyValue | typeof MARSHAL_FAILED {
  const reader = new RubyMarshalReader(data);
  const value = reader.value(0);
  if (value === MARSHAL_FAILED) return MARSHAL_FAILED;
  // A Marshal dump is exactly one value; bytes after it are corruption.
  if (reader.pos !== data.length) return MARSHAL_FAILED;
  return value;
}

class RubyMarshalReader {
  readonly #data: Uint8Array;
  readonly #symbols: string[] = [];
  readonly #decoder = new TextDecoder("utf-8", { fatal: false });
  pos = 0;

  constructor(data: Uint8Array) {
    this.#data = data;
  }

  #byte(): number | typeof MARSHAL_FAILED {
    if (this.pos >= this.#data.length) return MARSHAL_FAILED;
    return this.#data[this.pos++]!;
  }

  /**
   * Takes the next `n` bytes. The bound is checked against what remains, never
   * by adding `n` to the position: every length reaches here through
   * {@link #count}, which already rejected anything past the remaining bytes.
   */
  #bytes(n: number): Uint8Array | typeof MARSHAL_FAILED {
    if (n < 0 || n > this.#data.length - this.pos) return MARSHAL_FAILED;
    const slice = this.#data.subarray(this.pos, this.pos + n);
    this.pos += n;
    return slice;
  }

  /**
   * Reads Marshal's packed integer: 0 is 0; 1..4 and -1..-4 are a byte count
   * for a little-endian value; anything else is the value itself offset by 5.
   *
   * Every width the format allows is at most four bytes, so both branches stay
   * inside the double's exact-integer range and no bitwise 32-bit truncation is
   * involved.
   */
  #int(): number | typeof MARSHAL_FAILED {
    const lead = this.#byte();
    if (lead === MARSHAL_FAILED) return MARSHAL_FAILED;
    // The lead byte is a signed int8; widen it into its signed meaning.
    const c = lead > 127 ? lead - 256 : lead;
    if (c === 0) return 0;
    if (c > 4) return c - 5;
    if (c < -4) return c + 5;

    const width = c > 0 ? c : -c;
    const raw = this.#bytes(width);
    if (raw === MARSHAL_FAILED) return MARSHAL_FAILED;
    let unsigned = 0;
    for (let i = 0; i < raw.length; i++) unsigned += raw[i]! * 2 ** (8 * i);
    // A negative packed integer is the same little-endian run sign-extended
    // with 0xff bytes, which is exactly `unsigned - 256**width`.
    return c > 0 ? unsigned : unsigned - 2 ** (8 * width);
  }

  /**
   * Reads a length or count — string and symbol bytes, array elements, hash
   * pairs, ivar pairs — and rejects one that cannot be honest: negative, or
   * more than the bytes left (every element takes at least one byte).
   * Allocation follows what actually decodes, so a hostile count costs its own
   * bytes to refuse, never the capacity it claims.
   */
  #count(): number | typeof MARSHAL_FAILED {
    const n = this.#int();
    if (n === MARSHAL_FAILED) return MARSHAL_FAILED;
    if (n < 0 || n > this.#data.length - this.pos) return MARSHAL_FAILED;
    return n;
  }

  #string(): string | typeof MARSHAL_FAILED {
    const n = this.#count();
    if (n === MARSHAL_FAILED) return MARSHAL_FAILED;
    const raw = this.#bytes(n);
    if (raw === MARSHAL_FAILED) return MARSHAL_FAILED;
    return this.#decoder.decode(raw);
  }

  value(depth: number): RubyValue | typeof MARSHAL_FAILED {
    if (depth > RUBY_MARSHAL_MAX_DEPTH) return MARSHAL_FAILED;
    const type = this.#byte();
    if (type === MARSHAL_FAILED) return MARSHAL_FAILED;

    switch (type) {
      case 0x30: // '0'
        return null;
      case 0x54: // 'T'
        return true;
      case 0x46: // 'F'
        return false;
      case 0x69: // 'i'
        return this.#int();
      case 0x22: // '"'
        return this.#string();
      case 0x3a: {
        // ':' — a symbol, which later occurrences link back to by index.
        const symbol = this.#string();
        if (symbol === MARSHAL_FAILED) return MARSHAL_FAILED;
        this.#symbols.push(symbol);
        return symbol;
      }
      case 0x3b: {
        // ';' — a symbol link.
        const index = this.#int();
        if (index === MARSHAL_FAILED) return MARSHAL_FAILED;
        if (index < 0 || index >= this.#symbols.length) return MARSHAL_FAILED;
        return this.#symbols[index]!;
      }
      case 0x49: {
        // 'I' — an object followed by its instance variables (a String's encoding).
        const inner = this.value(depth + 1);
        if (inner === MARSHAL_FAILED) return MARSHAL_FAILED;
        const n = this.#count();
        if (n === MARSHAL_FAILED) return MARSHAL_FAILED;
        for (let i = 0; i < n; i++) {
          if (this.value(depth + 1) === MARSHAL_FAILED) return MARSHAL_FAILED; // ivar name
          if (this.value(depth + 1) === MARSHAL_FAILED) return MARSHAL_FAILED; // ivar value
        }
        return inner;
      }
      case 0x5b: {
        // '[' — an array.
        const n = this.#count();
        if (n === MARSHAL_FAILED) return MARSHAL_FAILED;
        const out: RubyValue[] = [];
        for (let i = 0; i < n; i++) {
          const element = this.value(depth + 1);
          if (element === MARSHAL_FAILED) return MARSHAL_FAILED;
          out.push(element);
        }
        return out;
      }
      case 0x7b: {
        // '{' — a hash.
        const n = this.#count();
        if (n === MARSHAL_FAILED) return MARSHAL_FAILED;
        const out: { [key: string]: RubyValue } = Object.create(null) as { [key: string]: RubyValue };
        for (let i = 0; i < n; i++) {
          const key = this.value(depth + 1);
          if (key === MARSHAL_FAILED) return MARSHAL_FAILED;
          const value = this.value(depth + 1);
          if (value === MARSHAL_FAILED) return MARSHAL_FAILED;
          if (typeof key !== "string") return MARSHAL_FAILED;
          out[key] = value;
        }
        return out;
      }
      default:
        return MARSHAL_FAILED;
    }
  }
}

// =============================================================================
// Writing mentions
// =============================================================================

/**
 * The `<bc-attachment>` that mentions a person, rendered from their
 * `attachable_sgid` — the write-side form in doc/api/sections/rich_text.md,
 * which BC3 expands into the avatar figure on read.
 *
 * It throws when the person carries no `attachable_sgid`, which is the case for
 * a Person projection that came from somewhere other than a people read (a
 * webhook payload, say), and when the sgid does not name the person it is
 * given. That is all it can check: it cannot verify the signature, so the
 * Person must come from the API — a `people.get`, a recording's creator or
 * assignees — not be assembled from an sgid found in content.
 *
 * @throws {BasecampError} `usage` when the person cannot be mentioned.
 */
export function mentionMarkup(person: Person): string {
  if (person === null || typeof person !== "object") {
    throw Errors.usage("cannot mention a person that is not a person object");
  }
  const sgid = person.attachable_sgid;
  if (!sgid) {
    throw Errors.usage(
      `person ${person.id} has no attachable_sgid to mention`,
      "read the person through people.get to obtain one",
    );
  }
  if (/["'<>&]/.test(sgid)) {
    throw Errors.usage(`person ${person.id} has a malformed attachable_sgid`);
  }
  // The tag mentions whoever the sgid names. Refuse to write one that names
  // someone else — or a file — under this person's id, and refuse one that
  // names nobody at all. The two halves are separate on purpose: comparing only
  // "names a different person" would pass an undecodable sgid for a person
  // projection carrying no id, since neither side would be a number.
  const named = personIdFromSGID(sgid);
  if (named === undefined || named !== person.id) {
    throw Errors.usage(
      `person ${person.id}'s attachable_sgid does not name that person`,
      "read the person through people.get to obtain their own",
    );
  }
  return `<bc-attachment sgid="${sgid}"></bc-attachment>`;
}

/**
 * The index just past the opening `<p …>` or `<div …>` tag a rich text starts
 * with, or `-1` when it starts with anything else, so mentions can be placed
 * inside the first block rather than as a bare prefix in front of it. The tag's
 * attributes are scanned quote-aware: a `>` inside an attribute value does not
 * end it.
 */
function leadingBlockEnd(content: string): number {
  let i = 0;
  while (i < content.length && isSpace(content.charCodeAt(i))) i++;

  for (const name of ["<p", "<div"]) {
    if (content.length < i + name.length) continue;
    if (content.slice(i, i + name.length).toLowerCase() !== name) continue;
    const after = i + name.length;
    if (after < content.length && !isTagNameEnd(content.charCodeAt(after))) continue;
    const attrs = parseAttributes(content, after);
    return attrs.closed ? attrs.end : -1;
  }
  return -1;
}

/**
 * Content that mentions each of the given people, for posting as a comment or a
 * Campfire line.
 *
 * A person whose exact `attachable_sgid` the content already carries is left
 * alone, so passing the same person twice — or a person the author already
 * mentioned with that sgid — never duplicates the mention; the rest are added
 * at the start of the content, inside its first `<p>` or `<div>` when it opens
 * with one, so they render on the first line rather than as a block of their
 * own.
 *
 * This is the write side, and it deduplicates on the sgid string alone, never
 * on the person id an existing tag's sgid decodes to: that id is unsigned, and
 * a forged or stale tag naming the right person must not stand in for the real
 * mention (see the trust boundary in the module comment). Every person needs
 * their own `attachable_sgid`, and it must be one the API returned: this helper
 * can check that an sgid is well-formed and names the person, not that it is
 * authentic (see {@link mentionMarkup}). The account-bound
 * `CommentsService.expandMentions` resolves ids to people first and is the
 * entry point that carries that guarantee.
 *
 * @throws {BasecampError} `usage` when a person cannot be mentioned.
 */
export function withMentions(content: string, people: readonly Person[]): string {
  const present = new Set(bcAttachmentSGIDs(content));
  const tags: string[] = [];
  for (const person of people) {
    // Every person is validated, duplicates included: a caller passing an
    // unmentionable person twice hears about it either way.
    const tag = mentionMarkup(person);
    const sgid = person.attachable_sgid!;
    if (present.has(sgid)) continue;
    present.add(sgid);
    tags.push(tag);
  }
  if (tags.length === 0) return content;

  const prefix = tags.join(" ") + " ";
  const end = leadingBlockEnd(content);
  return end >= 0 ? content.slice(0, end) + prefix + content.slice(end) : prefix + content;
}
