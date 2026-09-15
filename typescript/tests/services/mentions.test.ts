/**
 * Tests for the mention helpers (src/services/mentions.ts).
 *
 * The read side describes what a rich text says it mentions; the write side
 * renders a mention from a person the API returned. The trust boundary between
 * them — dedupe on the exact sgid, never on the id it decodes to — is pinned
 * here and again in comments-mentions.test.ts.
 */
import { describe, it, expect } from "vitest";
import {
  mentionedPersonIds,
  personIdFromSGID,
  mentionMarkup,
  withMentions,
} from "../../src/index.js";
import { namedEntityNames, unescapeEntities } from "../../src/services/mentions.js";
import { BasecampError } from "../../src/errors.js";
import type { Person } from "../../src/generated/services/people.js";
import { jsonSGID, legacySGID, personSGID, railsSGID } from "../helpers/sgid.js";

const VICTOR = 1049715914;
const ANNIE = 1049715915;

const person = (id: number, attachable_sgid: string | undefined): Person =>
  ({ id, name: `Person ${id}`, ...(attachable_sgid ? { attachable_sgid } : {}) }) as Person;

const attachment = (sgid: string): string => `<bc-attachment sgid="${sgid}"></bc-attachment>`;

describe("personIdFromSGID", () => {
  it("reads the id out of both Marshal layouts and the JSON spelling", () => {
    const gid = `gid://bc3/Person/${VICTOR}`;
    expect(personIdFromSGID(legacySGID(gid))).toBe(VICTOR);
    expect(personIdFromSGID(railsSGID(gid))).toBe(VICTOR);
    expect(personIdFromSGID(jsonSGID(gid))).toBe(VICTOR);
  });

  it("ignores the gid's query, which BC3 mints with", () => {
    expect(personIdFromSGID(personSGID(ANNIE))).toBe(ANNIE);
  });

  it("reads the envelope whether or not it is signed, and ignores the digest", () => {
    // The signature cannot be verified client-side, so it is not consulted: the
    // same payload under two digests names the same person, and a payload with
    // no digest at all is read through the whole-value fallback.
    const gid = `gid://bc3/Person/${VICTOR}`;
    expect(personIdFromSGID(legacySGID(gid, { unsigned: true }))).toBe(VICTOR);
    expect(personIdFromSGID(legacySGID(gid, { digest: "a".repeat(40) }))).toBe(VICTOR);
    expect(personIdFromSGID(legacySGID(gid, { digest: "b".repeat(40) }))).toBe(VICTOR);
  });

  it("trims each end on its own, whatever sits between them", () => {
    // The trim examines the boundary characters and nothing else. A port that
    // instead chose its whitespace alphabet from a validity test over the WHOLE
    // value loses a mention on this shape: a non-ASCII space in front, and
    // something odd in the digest half the separator throws away. Go decodes a
    // character at each end independently of everything between.
    //
    // Verified against Go over 200 combinations — nine non-ASCII spaces, four
    // kinds of stray content, five positions — and the same corpus run against
    // an ASCII-only trim diverges on 117 of them, so it reaches the class
    // rather than merely agreeing.
    const sgid = personSGID(VICTOR);
    for (const space of ["\u0085", "\u00a0", "\u1680", "\u2003", "\u202f", "\u3000"]) {
      for (const stray of ["\ufffd", "\ud83d\ude00", "\ud800", "\u00e9"]) {
        expect(personIdFromSGID(`${space}${sgid}${stray}`)).toBe(VICTOR);
        expect(personIdFromSGID(`${space}${sgid}${stray}${space}`)).toBe(VICTOR);
      }
    }
  });

  it("refuses a purpose BC3 does not accept in rich text", () => {
    expect(personIdFromSGID(legacySGID(`gid://bc3/Person/${VICTOR}`, { purpose: "readable" }))).toBeUndefined();
    expect(personIdFromSGID(railsSGID(`gid://bc3/Person/${VICTOR}`, { purpose: "bookmarkable" }))).toBeUndefined();
  });

  it("refuses a gid that names something other than a Person", () => {
    expect(personIdFromSGID(legacySGID("gid://bc3/ActiveStorage::Blob/9"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Recording/9"))).toBeUndefined();
  });

  it("refuses a gid that only contains a Person gid, rather than being one", () => {
    expect(personIdFromSGID(legacySGID(`gid://bc3/Document/gid://bc3/Person/${VICTOR}`))).toBeUndefined();
    expect(personIdFromSGID(legacySGID(`gid://bc3/Person/${VICTOR}/avatar`))).toBeUndefined();
  });

  it("reads a numeric reference the way Go's rune arithmetic does", () => {
    // Two rules, both measured against html.UnescapeString. There is no cap on
    // the digit count — an earlier version capped at ten and so truncated
    // "&#00000000065;", an ordinary way to write "A" rather than a hostile
    // input. And Go accumulates into an int32 that WRAPS, range-checking the
    // wrapped result, so 65 can be reached the long way round.
    const gid = `gid://bc3/Person/${VICTOR}`;
    const named = (value: string) =>
      mentionedPersonIds(`<bc-attachment sgid="${value}"></bc-attachment>`);
    const sgid = legacySGID(gid);

    // A zero-padded whitespace reference is consumed, so the trim erases it and
    // the person is still named; capping the digits would leave U+FFFD instead.
    expect(named(`&#9;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#00000000009;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#0000000000000000000009;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#00000000160;${sgid}`)).toEqual([VICTOR]);
    // The same value reached by wrapping an int32, which Go also resolves.
    expect(named(`&#4294967328;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#x100000020;${sgid}`)).toEqual([VICTOR]);
  });

  it("accepts and refuses the authorities net/url does", () => {
    const person = (gid: string) => personIdFromSGID(legacySGID(gid));
    const ok = (gid: string) => expect(person(gid)).toBe(VICTOR);
    const no = (gid: string) => expect(person(gid)).toBeUndefined();

    // An empty host is refused. Without this the read side INVENTS a mention,
    // which is the dangerous direction.
    no(`gid://@/Person/${VICTOR}`);
    no(`gid://user@/Person/${VICTOR}`);
    no(`gid://user:pass@/Person/${VICTOR}`);

    // Userinfo is split at the last "@" before the port is read.
    ok(`gid://user@bc3/Person/${VICTOR}`);
    ok(`gid://user:pass@bc3/Person/${VICTOR}`);
    no(`gid://\u00e9@bc3/Person/${VICTOR}`);
    no(`gid://u%zz@bc3/Person/${VICTOR}`);

    // Escapes: refused in a host only when they name an ASCII byte, with %25
    // exempt. Refusing all of them — as an earlier version did on a claim about
    // net/url that measurement contradicted — loses mentions Go reports.
    no(`gid://b%41c3/Person/${VICTOR}`);
    ok(`gid://b%C3%A9c3/Person/${VICTOR}`);
    ok(`gid://bc%253/Person/${VICTOR}`);
    no(`gid://b%zzc3/Person/${VICTOR}`);

    // The swept character set, including the four Go allows that look as though
    // it should not.
    for (const forbidden of [" ", "[", "\\", "^", "`", "{", "|", "}"]) {
      no(`gid://b${forbidden}c3/Person/${VICTOR}`);
    }
    for (const allowed of ['"', "<", ">", "]"]) {
      ok(`gid://b${allowed}c3/Person/${VICTOR}`);
    }

    // A control byte in the URL, but NOT in the fragment: Parse cuts the
    // fragment before it looks for one.
    no(`gid://bc3/Person/${VICTOR}\u0001`);
    no(`gid://bc3/Person/${VICTOR}?x=\u0001`);
    ok(`gid://bc3/Person/${VICTOR}#\u0001`);

    // The fragment IS unescaped, so a malformed escape there is an error; the
    // query is left raw, so the same escape there is not.
    no(`gid://bc3/Person/${VICTOR}#%zz`);
    no(`gid://bc3/Person/${VICTOR}#%2`);
    ok(`gid://bc3/Person/${VICTOR}?x=%zz`);

    // The path is percent-decoded, as url.Parse decodes it, and a malformed
    // escape there is refused.
    ok(`gid://bc3/Pe%72son/${VICTOR}`);
    ok(`gid://bc3/Person/104%39715914`);
    no(`gid://bc3/Person/${VICTOR}%zz`);

    // A bracketed literal is parsed, not shape-checked — six defects across
    // five ports have lived on these two lines, every one a look-like test
    // standing in for the reference's parse.
    ok(`gid://[::1]/Person/${VICTOR}`);
    ok(`gid://[::ffff:1.2.3.4]/Person/${VICTOR}`);
    no(`gid://[1.2.3.4]/Person/${VICTOR}`);
    // A dotted quad is legal only at the END of the address: with a trailing
    // "::" the quad sits in front of the compression.
    no(`gid://[1.2.3.4::]/Person/${VICTOR}`);
    no(`gid://[0:0.0.0.0::]/Person/${VICTOR}`);

    // The zone has its own character and escape rules, and they are not the
    // ones the rest of the URL uses.
    ok(`gid://[fe80::1%25eth0]/Person/${VICTOR}`);
    ok(`gid://[fe80::1%25e%20f]/Person/${VICTOR}`);
    ok(`gid://[fe80::1%25e%25f]/Person/${VICTOR}`);
    no(`gid://[fe80::1%25e f]/Person/${VICTOR}`);
    no(`gid://[fe80::1%25e[f]/Person/${VICTOR}`);
    no(`gid://[fe80::1%25e%2Ff]/Person/${VICTOR}`);
    no(`gid://[fe80::1%25e%00f]/Person/${VICTOR}`);
    no(`gid://[fe80::1%25]/Person/${VICTOR}`);

    // Four rules of the literal parser that a review found correct and held by
    // nothing: each mutation below left the whole suite green while making the
    // parser accept a gid Go refuses — the invent-a-mention direction. Every
    // pair is measured through the reference's own write path, the refusal
    // beside the neighbour it must not take with it.
    no(`gid://[::01.2.3.4]/Person/${VICTOR}`); // an octet may not carry a leading zero
    ok(`gid://[::1.2.3.4]/Person/${VICTOR}`);
    no(`gid://[1:2:3:4:5:6:7:8::]/Person/${VICTOR}`); // "::" must expand to a field
    ok(`gid://[1:2:3:4:5:6:7::]/Person/${VICTOR}`);
    no(`gid://[1:2:3:4:5:6::1.2.3.4]/Person/${VICTOR}`); // the quad is two of the eight
    ok(`gid://[1:2:3:4:5::1.2.3.4]/Person/${VICTOR}`);
    no(`gid://[12345::1]/Person/${VICTOR}`); // a group is at most four digits
    ok(`gid://[1234::1]/Person/${VICTOR}`);
    no(`gid://[bogus%25eth0]/Person/${VICTOR}`); // a zone does not make an address
    ok(`gid://[::1%25eth0]/Person/${VICTOR}`);
  });

  it("refuses a malformed id, and an id that cannot be a number without rounding", () => {
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/12a"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/0"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/-5"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/90071992547409911"))).toBeUndefined();
  });

  it("folds an attribute name the way EqualFold does, not the way toLowerCase does", () => {
    // `EqualFold("\u017Fgid", "sgid")` is TRUE — U+017F LATIN SMALL LETTER LONG S
    // shares a fold orbit with `s` — so markup written with one IS a mention the
    // reference reports, and `toLowerCase()` left it out: the vanishing
    // direction, a mention dropped rather than invented. The table is measured,
    // not recalled: sweeping every rune against the 26 ASCII letters and `-`
    // gives exactly two that reach ASCII from outside it, U+017F onto `s` and
    // U+212A KELVIN SIGN onto `k`.
    const sgid = personSGID(VICTOR);
    const withAttr = (name: string) => `<div><bc-attachment ${name}="${sgid}"></bc-attachment></div>`;
    expect(mentionedPersonIds(withAttr("sgid"))).toEqual([VICTOR]);
    expect(mentionedPersonIds(withAttr("SGID"))).toEqual([VICTOR]);
    expect(mentionedPersonIds(withAttr("\u017Fgid"))).toEqual([VICTOR]);
    // The table's other half, U+212A KELVIN SIGN onto `k`, is unobservable at
    // all three call sites — none of `sgid`, `bc-attachment`, `<p` or `<div`
    // contains a `k` — so it is asserted through the helper's own rule rather
    // than through a tag, and named here so nobody reads the tag rows as
    // covering it. (The row this replaces was `"SGI\u0044"`, which is the
    // string "SGID": byte-identical to the line above it and discriminating
    // nothing.)
    expect(mentionedPersonIds(withAttr("s\u212Aid"))).toEqual([]);
    expect(mentionedPersonIds(`<div><bc-atta\u212Ahment sgid="${sgid}"></bc-attachment></div>`)).toEqual([]);

    // And nothing else folds onto it: a Cyrillic ѕ, a sharp s, a full-width s
    // are all different letters to EqualFold, so the tag names nobody.
    //
    // The dotless ı and the dotted İ are the rows that matter, because they are
    // where a WIDER rule and the reference's part company: `"ı".toUpperCase()`
    // is "I", so a comparison written as "same when upper-cased" would accept
    // `sgıd`, and Go refuses it — measured. A guard that only refused
    // lookalikes whose case mapping already differs would have been silent
    // about that, which is how the first version of these rows passed against
    // both the orbit table and a too-wide rule.
    for (const lookalike of ["\u0455gid", "\u00dfgid", "\uff53gid", "\u1e69gid", "sg\u0131d", "sg\u0130d", "SG\u0130D"]) {
      expect(mentionedPersonIds(withAttr(lookalike)), lookalike).toEqual([]);
    }

    // The same rule on the tag name, and on the leading block `withMentions`
    // places a mention inside — all three sites use the reference's EqualFold.
    expect(mentionedPersonIds(`<div><BC-ATTACHMENT sgid="${sgid}"></BC-ATTACHMENT></div>`)).toEqual([VICTOR]);
    expect(withMentions("<DIV>hi</DIV>", [person(VICTOR, sgid)])).toBe(
      `<DIV>${attachment(sgid)} hi</DIV>`,
    );

    // The gid SCHEME keeps the other rule, ASCII-only, because url.Parse has
    // already excluded everything else by the time a scheme is read. One file,
    // two rules; carrying either answer to the other site is wrong.
    expect(personIdFromSGID(legacySGID(`GID://bc3/Person/${VICTOR}`))).toBe(VICTOR);
    expect(personIdFromSGID(legacySGID(`\u0262id://bc3/Person/${VICTOR}`))).toBeUndefined();
  });

  it("refuses a payload carrying whitespace the base64 alphabet does not", () => {
    // `atob` ignores every ASCII whitespace character; Go's decoder ignores
    // only CR and LF. Both sides must accept the same payloads, or a malformed
    // sgid reports a mention in one SDK and not in another.
    const sgid = personSGID(VICTOR);
    const at = (fill: string) => sgid.slice(0, 8) + fill + sgid.slice(8);
    expect(personIdFromSGID(at(" "))).toBeUndefined();
    expect(personIdFromSGID(at("\t"))).toBeUndefined();
    expect(personIdFromSGID(at("\n"))).toBe(VICTOR);
    expect(personIdFromSGID(at("\r"))).toBe(VICTOR);
  });

  it("matches the gid scheme case-insensitively and refuses a host that cannot be one", () => {
    expect(personIdFromSGID(legacySGID(`GID://bc3/Person/${VICTOR}`))).toBe(VICTOR);
    expect(personIdFromSGID(legacySGID(`gid://b c3/Person/${VICTOR}`))).toBeUndefined();
    expect(personIdFromSGID(legacySGID(`gid://bc3\u0000/Person/${VICTOR}`))).toBeUndefined();
  });

  it("accepts a final group whose unused bits are not zero, as Go's decoder does", () => {
    // Go decodes with base64.RawStdEncoding, which does NOT require the last
    // group's discarded bits to be zero — only Encoding.Strict() does. A
    // decoder stricter than that on this axis would make real mentions
    // disappear rather than error, so the leniency has to match in this
    // direction too.
    const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    // One character longer than the usual gid, so the payload does not land on
    // a whole group and there are discarded bits to set.
    const sgid = legacySGID(`gid://bc3/Person/${VICTOR}?expires_in=x`, { unsigned: true });
    const remainder = sgid.length % 4;
    expect(remainder).not.toBe(0);
    expect(personIdFromSGID(sgid)).toBe(VICTOR);

    // Keep the bits the decoder retains, set one of the bits it discards.
    const mask = remainder === 2 ? 0x0f : 0x03;
    const index = ALPHABET.indexOf(sgid[sgid.length - 1]!);
    const mutated = sgid.slice(0, -1) + ALPHABET[(index & ~mask) | 1]!;
    expect(mutated).not.toBe(sgid);
    expect(personIdFromSGID(mutated)).toBe(VICTOR);
  });

  it("strips padding the way Go does — a right-trim, before the newlines", () => {
    // Go right-trims `=` off the payload as written and only then decodes,
    // ignoring CR and LF as it goes. So a line break sitting between the
    // padding and the `--` separator names nobody in Go: the trim stops at the
    // newline and leaves an `=` the raw alphabet refuses, and the whole-value
    // fallback fails the same way. Stripping newlines before the trim would
    // decode that one and be leniently wrong in a place nothing else looks.
    //
    // Probed against `base64.RawStdEncoding` and Go's own `globalIDFromSGID`
    // rather than reasoned about: these five agree case for case.
    const unsigned = legacySGID(`gid://bc3/Person/${VICTOR}?expires_in=x`, { unsigned: true });
    const padding = "=".repeat((4 - (unsigned.length % 4)) % 4);
    expect(padding).not.toBe("");
    const digest = "--deadbeefdeadbeefdeadbeefdeadbeefdeadbeef";

    expect(personIdFromSGID(unsigned + padding + digest)).toBe(VICTOR);
    expect(personIdFromSGID(unsigned + padding + "\n" + digest)).toBeUndefined();
    expect(personIdFromSGID(unsigned + padding + "\r" + digest)).toBeUndefined();
    // A newline inside the payload is ignored, and the padding still trims.
    expect(
      personIdFromSGID(`${unsigned.slice(0, 4)}\n${unsigned.slice(4)}${padding}${digest}`),
    ).toBe(VICTOR);
    // Trailing whitespace on the whole sgid is trimmed before any of that, so
    // it reaches the payload handling as if it had never been there.
    expect(personIdFromSGID(unsigned + padding + "\n")).toBe(VICTOR);
  });

  it("refuses a payload whose length cannot be a base64 group", () => {
    // Both decoders reject a final group of one character.
    const sgid = legacySGID(`gid://bc3/Person/${VICTOR}`, { unsigned: true });
    expect(personIdFromSGID(sgid + "A")).toBeUndefined();
  });

  it("refuses what does not decode at all", () => {
    expect(personIdFromSGID("")).toBeUndefined();
    expect(personIdFromSGID("not base64 at all !!")).toBeUndefined();
    expect(personIdFromSGID(btoa("plain text, not an envelope"))).toBeUndefined();
    expect(personIdFromSGID(btoa("x".repeat(6000)))).toBeUndefined();
  });
});

describe("mentionedPersonIds", () => {
  it("reads the sgid of each bc-attachment, in order, without repeats", () => {
    const content =
      `<div>${attachment(personSGID(VICTOR))} and ${attachment(personSGID(ANNIE))} ` +
      `and again ${attachment(personSGID(VICTOR))}</div>`;
    expect(mentionedPersonIds(content)).toEqual([VICTOR, ANNIE]);
  });

  it("counts a quoted mention, because BC3 notifies it", () => {
    expect(mentionedPersonIds(`<blockquote><p>${attachment(personSGID(VICTOR))}</p></blockquote>`)).toEqual([VICTOR]);
  });

  it("skips attachments that are not mentions", () => {
    const blob = attachment(legacySGID("gid://bc3/ActiveStorage::Blob/9"));
    expect(mentionedPersonIds(`<div>${blob}${attachment(personSGID(ANNIE))}</div>`)).toEqual([ANNIE]);
  });

  it("returns an empty list for content with no attachments", () => {
    expect(mentionedPersonIds("<div>Hello everyone!</div>")).toEqual([]);
    expect(mentionedPersonIds("")).toEqual([]);
  });

  it("does not read a bc-attachment inside an HTML comment", () => {
    expect(mentionedPersonIds(`<div><!-- ${attachment(personSGID(VICTOR))} --></div>`)).toEqual([]);
  });

  it("does not read a bc-attachment written inside another tag's attribute value", () => {
    const sgid = personSGID(VICTOR);
    expect(mentionedPersonIds(`<div title="<bc-attachment sgid='${sgid}'>">text</div>`)).toEqual([]);
  });

  it("does not read a tag whose name merely starts with bc-attachment", () => {
    expect(mentionedPersonIds(`<bc-attachment-preview sgid="${personSGID(VICTOR)}">`)).toEqual([]);
  });

  it("accepts either quote style, any attribute order, and any case", () => {
    const sgid = personSGID(VICTOR);
    expect(mentionedPersonIds(`<BC-Attachment content-type="x" SGID='${sgid}'></BC-Attachment>`)).toEqual([VICTOR]);
  });

  it("takes the first sgid when a tag repeats the attribute, as HTML does", () => {
    const content = `<bc-attachment sgid="${personSGID(VICTOR)}" sgid="${personSGID(ANNIE)}"></bc-attachment>`;
    expect(mentionedPersonIds(content)).toEqual([VICTOR]);
  });

  it("does not let a > inside a quoted value end the tag", () => {
    const sgid = personSGID(VICTOR);
    const content = `<bc-attachment alt="a > b" sgid="${sgid}"></bc-attachment>`;
    expect(mentionedPersonIds(content)).toEqual([VICTOR]);
  });

  it("decodes entity escapes in the attribute value", () => {
    const sgid = personSGID(VICTOR);
    const escaped = sgid.replace(/-/g, "&#45;");
    expect(mentionedPersonIds(`<bc-attachment sgid="${escaped}"></bc-attachment>`)).toEqual([VICTOR]);
  });

  it("decodes each character reference to the string the carried table decodes it to", () => {
    // Pins the DECODED STRING, not merely the verdict. A row asserting "names
    // nobody" is satisfied by any failure that also names nobody — including a
    // decoder that does nothing at all — so the rows guarding a specific
    // expansion have to compare the expansion.
    const expansions: [string, string][] = [
      ["&amp;", "&"],
      ["&amp", "&"],
      ["&AMP;", "&"],
      ["&nbsp;", "\u00a0"],
      ["&nbsp", "\u00a0"],
      ["&nbspBAh7", "\u00a0BAh7"],
      ["&ensp;", "\u2002"],
      ["&ThickSpace;", "\u205f\u200a"],
      ["&fjlig;", "fj"],
      ["&sol;", "/"],
      ["&plus;", "+"],
      ["&equals;", "="],
      ["&lowbar;", "_"],
      ["&Tab;", "\t"],
      ["&NewLine;", "\n"],
      // Not in the carried subset, and left exactly as written — Go expands all
      // four (`&eacute;` to "é", `&hyphen;` to U+2010, `&solb;` to U+29C4,
      // `&constructor;` to nothing, being no name at all). These are the same
      // deliberate difference as `&ltcc;` below, from the other direction: a
      // name the 24 do not carry. Neither direction can change a verdict,
      // because neither expansion can appear in a decodable payload.
      ["&eacute;", "&eacute;"],
      ["&hyphen;", "&hyphen;"],
      ["&constructor;", "&constructor;"],
      ["&solb;", "&solb;"],
      // Numeric boundaries.
      ["&#65;", "A"],
      ["&#65", "A"],
      ["&#6;", "\u0006"],
      ["&#6", "&#6"],
      ["&#6B", "&#6B"],
      ["&#66B", "BB"],
      ["&#x4", "\u0004"],
      ["&#x4B", "K"],
      ["&#x;", "\ufffd"],
      ["&#;", "&#;"],
      ["&#", "&#"],
      ["&#0;", "\ufffd"],
      ["&#128;", "\u20ac"],
      ["&#x9F;", "\u0178"],
      ["&#00000000065;", "A"],
      ["&#4294967361;", "A"],
      ["&#2147483648;", "\ufffd"],
      ["&#xD800;", "\ufffd"],
      ["&", "&"],
      ["&&&&", "&&&&"],
      ["plain", "plain"],
    ];
    for (const [input, expected] of expansions) {
      expect(unescapeEntities(input), `unescaping ${JSON.stringify(input)}`).toBe(expected);
    }

    // The SHADOWING form of the same deliberate difference, and the subtler of
    // the two: the rows above differ by a name this table does not carry, which
    // is plain to see, while this one differs on a name it does. It is the
    // price of carrying 24 names instead of 2231. Go matches the longest name
    // in its full table, so "&ltcc;" is U+2AA6; here the longest match is the
    // semicolon-less "lt", giving "<cc;". Only the nine semicolon-less legacy
    // names can shadow this way, every one of them expands to "&", "<", ">",
    // '"' or NBSP, and none of those can appear in a decodable payload — so the
    // strings differ and the verdict cannot. Asserted rather than omitted,
    // because a silent difference is how this stops being true.
    expect(unescapeEntities("&ltcc;")).toBe("<cc;");
    expect(mentionedPersonIds(`<bc-attachment sgid="&ltcc;${personSGID(VICTOR)}"></bc-attachment>`)).toEqual([]);
  });

  it("decodes character references the way Go's scanner does", () => {
    // Boundary rules measured against html.UnescapeString, not inferred: a
    // decimal reference needs two digits when no semicolon follows and one when
    // it does; a hex reference needs one either way; `&#x;` with no digits is
    // U+FFFD while `&#;` is literal; a trailing semicolon is consumed when
    // present. Each case here wraps a real sgid so the assertion is on the
    // verdict, which is what has to match.
    const sgid = personSGID(VICTOR);
    const named = (value: string) =>
      mentionedPersonIds(`<bc-attachment sgid="${value}"></bc-attachment>`);

    // Named references, semicolon-optional for the legacy set, and the
    // whitespace expansions that a Go-space trim then removes.
    expect(named(`&nbsp${sgid}`)).toEqual([VICTOR]);
    expect(named(`&nbsp;${sgid}`)).toEqual([VICTOR]);
    expect(named(`${sgid}&ThickSpace;`)).toEqual([VICTOR]);
    expect(named(`&NonBreakingSpace;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&Tab;${sgid}&NewLine;`)).toEqual([VICTOR]);

    // A reference that expands to something no payload can hold breaks it, as
    // it does in Go — and one outside the carried subset is left literal, which
    // breaks it the same way.
    expect(named(`&amp;${sgid}`)).toEqual([]);
    expect(named(`&eacute;${sgid}`)).toEqual([]);
    expect(named(`&constructor;${sgid}`)).toEqual([]);

    // Numeric boundaries. "&#66" is "B", so it extends the payload and breaks
    // it; the point is that it is CONSUMED rather than left literal, which the
    // one-digit forms are.
    expect(named(`&#9;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#10;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#32;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#x20;${sgid}`)).toEqual([VICTOR]);
    expect(named(`&#x9;${sgid}`)).toEqual([VICTOR]);
    // Unterminated forms break the payload, each for its own reason, and all
    // three agree with Go: `&#9` is one decimal digit with no semicolon and
    // stays literal; `&#x9` runs greedily on into the payload's own hex digits
    // and eats them; `&#` has no digits at all.
    expect(named(`&#9${sgid}`)).toEqual([]);
    expect(named(`&#x9${sgid}`)).toEqual([]);
    expect(named(`&#${sgid}`)).toEqual([]);
  });

  it("keeps the entity table to the shape the search bound assumes", () => {
    // unescapeEntity bounds its search by the run of name characters present,
    // which is only sound while every key is `[A-Za-z0-9]+` with an optional
    // `;`. A row that broke that — a name carrying a hyphen, say — would
    // silently stop being findable rather than fail to compile, so the
    // assumption is asserted rather than trusted.
    const names = namedEntityNames();
    expect(names.length).toBeGreaterThan(20);
    for (const name of names) {
      expect(name).toMatch(/^[A-Za-z0-9]+;?$/);
    }
  });

  it("consults the entity table once per ampersand, not once per table length", () => {
    // Two rewrites, and the second is the one that measures the bound.
    //
    // It began as a wall-clock ratio. An adversarial re-measurement put the
    // LINEAR ratio at 18.85 on a loaded box — past the quadratic reference the
    // threshold was set against — so no cutoff on that axis separated them.
    //
    // It then counted `String.prototype.slice` calls, which was countable but
    // not the thing: a reviewer showed the regression could be reintroduced
    // alongside a `slice` → `substring` swap and the count would stay flat,
    // while a second, incidental slice elsewhere in the scanner held the
    // non-vacuity floor up on its own. A test that survives the defect it names
    // is worse than the flaky one it replaced.
    //
    // The bound is a number of TABLE LOOKUPS, so the table is what to count.
    // `NAMED_ENTITIES.get` is reached once per length tried, whatever string
    // method slices the key, and nothing else in this path consults a Map.
    const native = Map.prototype.get;
    let lookups = 0;
    Map.prototype.get = function <K, V>(this: Map<K, V>, key: K): V | undefined {
      lookups++;
      return native.call(this, key) as V | undefined;
    };
    let perAmpersand: number;
    try {
      const n = 20_000;
      mentionedPersonIds(`<bc-attachment sgid="${"&a".repeat(n)}"></bc-attachment>`);
      perAmpersand = lookups / n;
    } finally {
      Map.prototype.get = native;
    }

    // "&a" offers one name character, so one length is tried and one lookup
    // made. Searching every length instead costs LONGEST_ENTITY_NAME of them —
    // the constant that grows with the TABLE, which is what must never return.
    expect(perAmpersand).toBeLessThan(2);
    // And not vacuous: a scan that consulted the table not at all would pass
    // the bound above while measuring an implementation that cannot work.
    expect(perAmpersand).toBeGreaterThanOrEqual(1);
  });

  it("stops at an unterminated comment or tag rather than guessing", () => {
    const sgid = personSGID(VICTOR);
    expect(mentionedPersonIds(`<div><!-- ${attachment(sgid)}`)).toEqual([]);
    expect(mentionedPersonIds(`<div><bc-attachment sgid="${sgid}`)).toEqual([]);
  });
});

describe("mentionMarkup", () => {
  it("renders the write-side tag from the person's attachable_sgid", () => {
    const sgid = personSGID(VICTOR);
    expect(mentionMarkup(person(VICTOR, sgid))).toBe(`<bc-attachment sgid="${sgid}"></bc-attachment>`);
  });

  it("refuses a person with no attachable_sgid", () => {
    expect(() => mentionMarkup(person(VICTOR, undefined))).toThrow(BasecampError);
    try {
      mentionMarkup(person(VICTOR, undefined));
    } catch (err) {
      expect((err as BasecampError).code).toBe("usage");
      expect((err as BasecampError).message).toContain("no attachable_sgid");
    }
  });

  it("reads a string person id the way the reference's FlexibleInt64 does", () => {
    // Go's Person.Id is the one FlexibleInt64 in the generated model, and this
    // SDK's own normalizer only rewrites a string id when `personable_type` is
    // present — so a people read without it arrives here still carrying a
    // string, and comparing it to the number the sgid names refused a person
    // the reference mentions. Every row measured by decoding the same body
    // through generated.Person, not read off this code.
    const sgid = personSGID(VICTOR);
    const withId = (id: unknown): Person =>
      ({ id, name: "Person", attachable_sgid: sgid }) as unknown as Person;

    // "1049715914" and "+1049715914" are that number in Go.
    expect(mentionMarkup(withId(String(VICTOR)))).toBe(attachment(sgid));
    expect(mentionMarkup(withId(`+${VICTOR}`))).toBe(attachment(sgid));

    // A non-numeric string is Go's sentinel zero, and every one of them is a
    // refusal: the sgid side refuses a non-positive id outright (`id <= 0` in
    // PersonIDFromSGID), so nothing a sentinel could match ever decodes. Zero
    // is still what this reads, because that is what Go reads — the equality is
    // just unreachable, which the first draft of this test got wrong by
    // asserting an sgid naming person 0 would match it.
    for (const sentinel of ["basecamp", ` ${VICTOR}`, `${VICTOR}.0`]) {
      expect(() => mentionMarkup(withId(sentinel))).toThrow(/does not name that person/);
    }

    // The one string Go refuses outright, taking the people read with it. This
    // layer cannot fail a read that already returned, so it refuses the write.
    expect(() => mentionMarkup(withId("9223372036854775808"))).toThrow(/does not name that person/);
  });

  it("refuses an sgid that names someone else", () => {
    expect(() => mentionMarkup(person(VICTOR, personSGID(ANNIE)))).toThrow(/does not name that person/);
  });

  it("refuses an sgid that names a file rather than a person", () => {
    expect(() =>
      mentionMarkup(person(VICTOR, legacySGID("gid://bc3/ActiveStorage::Blob/9"))),
    ).toThrow(/does not name that person/);
  });

  it("refuses an sgid that decodes to nobody, even for a person carrying no id", () => {
    // The two halves of the check are separate: "names a different person"
    // alone would pass an undecodable sgid when neither side is a number, and
    // an unverifiable tag would be written.
    expect(() => mentionMarkup(person(VICTOR, "not-a-real-sgid-at-all"))).toThrow(
      /does not name that person/,
    );
    const anonymous = { name: "No id", attachable_sgid: "not-a-real-sgid-at-all" } as unknown as Person;
    expect(() => mentionMarkup(anonymous)).toThrow(BasecampError);
    expect(() => withMentions("<div>hi</div>", [anonymous])).toThrow(BasecampError);
  });

  it("refuses an attachable_sgid that is not a string, rather than throwing on it", () => {
    // Typed as a string, but it arrives off a response: a truthy non-string
    // passed the presence check, survived the markup regex — `test`
    // stringifies — and reached the parser, whose first character read threw a
    // raw TypeError out of a helper documented to raise a usage error.
    for (const sgid of [42, true, { sgid: "x" }, ["x"]]) {
      const person = { id: VICTOR, name: "P", attachable_sgid: sgid } as unknown as Person;
      const err = (() => {
        try {
          mentionMarkup(person);
          return undefined;
        } catch (e: unknown) {
          return e;
        }
      })();
      expect(err, JSON.stringify(sgid)).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("usage");
    }
  });

  it("refuses something that is not a person at all", () => {
    expect(() => mentionMarkup(null as unknown as Person)).toThrow(BasecampError);
    expect(() => mentionMarkup(undefined as unknown as Person)).toThrow(BasecampError);
  });

  it("leaves a character reference that names an Object.prototype member alone", () => {
    // The entity table must not answer for `constructor`/`toString`: a lookup
    // through Object.prototype would substitute a function into the value.
    expect(mentionedPersonIds(`<bc-attachment sgid="&constructor;"></bc-attachment>`)).toEqual([]);
    expect(mentionedPersonIds(`<bc-attachment sgid="&toString;"></bc-attachment>`)).toEqual([]);
  });

  it("reaches the same gid verdicts from the write side as from the read side", () => {
    // The gid parser has two entry points, and every differential run against
    // this port until now went through the read one. mentionMarkup checks that
    // a person's attachable_sgid names that person, so a parser STRICTER than
    // Go's does not merely fail to read a mention — it refuses to WRITE one for
    // a person Go would write, and expandMentions then fails the whole comment.
    // That is worse than a missed read, and it is the direction every
    // tightening of the host rules is a candidate for.
    const writes = (gid: string): boolean => {
      const sgid = legacySGID(gid);
      try {
        mentionMarkup(person(VICTOR, sgid));
        return true;
      } catch {
        return false;
      }
    };

    // Whatever the read side resolves, the write side must write — and the
    // assertion is tied to the read side rather than to a literal, so the two
    // cannot drift apart without this failing.
    for (const gid of [
      `gid://bc3/Person/${VICTOR}`,
      `gid://bc3/Person/${VICTOR}?expires_in=`,
      `gid://user:pass@bc3/Person/${VICTOR}`,
      `gid://b%C3%A9c3/Person/${VICTOR}`,
      `gid://[::1]/Person/${VICTOR}`,
      `gid://[fe80::1%25eth0]/Person/${VICTOR}`,
      `gid://bc3/Pe%72son/${VICTOR}`,
      `gid://b<c3/Person/${VICTOR}`,
      // and the ones neither side may resolve
      `gid://@/Person/${VICTOR}`,
      `gid://[not-an-ip]/Person/${VICTOR}`,
      `gid://b{c3/Person/${VICTOR}`,
      `gid://b%41c3/Person/${VICTOR}`,
      `gid://bc3/Person/${VICTOR}#%zz`,
    ]) {
      const resolves = personIdFromSGID(legacySGID(gid)) === VICTOR;
      expect(writes(gid), `write side disagrees with the read side for ${gid}`).toBe(resolves);
    }

    // Deriving both ends from the same parser pins DRIFT and is blind to
    // COMMON-MODE error: forcing isParsableHost or personIdFromSGID to a
    // constant leaves the loop above green, because both sides move together.
    // These rows are literal, one per verdict class, so the two failure modes
    // are covered by different assertions rather than by the same one twice.
    expect(writes(`gid://bc3/Person/${VICTOR}`)).toBe(true);
    expect(writes(`gid://[::1]/Person/${VICTOR}`)).toBe(true);
    expect(writes(`gid://@/Person/${VICTOR}`)).toBe(false);
    expect(writes(`gid://[not-an-ip]/Person/${VICTOR}`)).toBe(false);
    expect(writes(`gid://[fe80::1%25eth0]/Person/${VICTOR}`)).toBe(true);
    expect(writes(`gid://[fe80::1%25e f]/Person/${VICTOR}`)).toBe(false);
    expect(writes(`gid://[1.2.3.4::]/Person/${VICTOR}`)).toBe(false);
    expect(writes(`gid://bc3/Person/${VICTOR}#%zz`)).toBe(false);
    expect(writes(`gid://bc3/Pe%72son/${VICTOR}`)).toBe(true);
  });

  it("refuses an sgid carrying markup characters", () => {
    expect(() => mentionMarkup(person(VICTOR, `"><script>`))).toThrow(/malformed attachable_sgid/);
  });
});

describe("withMentions", () => {
  it("places the mentions inside the content's first block", () => {
    const sgid = personSGID(VICTOR);
    expect(withMentions("<div>On it.</div>", [person(VICTOR, sgid)])).toBe(
      `<div>${attachment(sgid)} On it.</div>`,
    );
    expect(withMentions('<p class="x">On it.</p>', [person(VICTOR, sgid)])).toBe(
      `<p class="x">${attachment(sgid)} On it.</p>`,
    );
  });

  it("prefixes content that does not open with a block", () => {
    const sgid = personSGID(VICTOR);
    expect(withMentions("On it.", [person(VICTOR, sgid)])).toBe(`${attachment(sgid)} On it.`);
    expect(withMentions("<blockquote>On it.</blockquote>", [person(VICTOR, sgid)])).toBe(
      `${attachment(sgid)} <blockquote>On it.</blockquote>`,
    );
  });

  it("adds one tag per person, in order", () => {
    const victor = personSGID(VICTOR);
    const annie = personSGID(ANNIE);
    expect(withMentions("<div>Hi</div>", [person(VICTOR, victor), person(ANNIE, annie)])).toBe(
      `<div>${attachment(victor)} ${attachment(annie)} Hi</div>`,
    );
  });

  it("adds nothing for a person whose exact sgid the content already carries", () => {
    const sgid = personSGID(VICTOR);
    const content = `<div>${attachment(sgid)} On it.</div>`;
    expect(withMentions(content, [person(VICTOR, sgid)])).toBe(content);
    expect(withMentions(content, [person(VICTOR, sgid), person(VICTOR, sgid)])).toBe(content);
  });

  it("still mentions a person whose id an existing tag names under a DIFFERENT sgid", () => {
    // The trust boundary: an sgid's signature cannot be verified here, so a
    // stale or forged tag naming the right id must not suppress the real
    // mention. Deduping by person id would drop the second tag and silently
    // leave the person unmentioned.
    const stale = personSGID(VICTOR, { digest: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" });
    const real = personSGID(VICTOR);
    expect(stale).not.toBe(real);
    expect(withMentions(`<div>${attachment(stale)} On it.</div>`, [person(VICTOR, real)])).toBe(
      `<div>${attachment(real)} ${attachment(stale)} On it.</div>`,
    );
  });

  it("never lets a character reference expand to nothing (the mechanism)", () => {
    // A decoder that maps anything to the empty string is exactly what makes
    // the write-side dedupe suppressible: unescape(<real sgid> + <suffix>) can
    // only equal <real sgid> if the suffix decoded to nothing. HTML5 drops a
    // numeric reference naming a C0 control to ""; Go emits the character, and
    // so does this. Pinned at the decoder so a later simplification toward the
    // HTML5 reading fails here rather than silently at the write side.
    const sgid = personSGID(VICTOR);
    expect(mentionedPersonIds(attachment(sgid))).toEqual([VICTOR]);
    for (const reference of ["&#1;", "&#0;", "&#x1;", "&#8;", "&#31;", "&#127;", "&#x0;"]) {
      // Prefixed, so the reference lands in the payload rather than the digest:
      // if it expanded to nothing the payload would be untouched and the person
      // would still be named.
      expect(mentionedPersonIds(attachment(reference + sgid))).toEqual([]);
    }
    // The contrast, measured against Go: a reference naming WHITESPACE does
    // leave the person named, because the trim then removes it. So "the
    // reference survived" is being asserted above, not "any prefix breaks it".
    for (const reference of ["&#9;", "&#10;", "&#11;", "&#12;", "&#13;", "&#32;"]) {
      expect(mentionedPersonIds(attachment(reference + sgid))).toEqual([VICTOR]);
    }
  });

  it("still writes a mention the content only appears to carry (the consequence)", () => {
    // The same attack from the write side. The dedupe is deliberately an exact
    // string match against the sgid the people read returned, so anything that
    // unescapes to exactly that string suppresses the tag.
    const sgid = personSGID(VICTOR);
    const author = person(VICTOR, sgid);
    const tags = (content: string) => content.match(/<bc-attachment /g)?.length ?? 0;

    // The control row, without which this test would pass just as happily if
    // the dedupe had been broken to never fire: here the content genuinely does
    // carry the mention, and exactly one tag must come back.
    expect(tags(withMentions(attachment(sgid), [author]))).toBe(1);

    for (const reference of ["&#1;", "&#0;", "&#x1;", "&#127;"]) {
      const disguised = attachment(sgid + reference);
      expect(tags(withMentions(disguised, [author]))).toBe(2);
    }
  });

  it("does not trim on the write side, where the read side does", () => {
    // The two paths genuinely differ, and the difference is load-bearing.
    //
    // READ: globalIDFromSGID trims Go-space before decoding, so a whitespace
    // reference around the value is erased and the person is still named.
    // WRITE: the dedupe compares the unescaped attribute value with NO trim, so
    // the same value is not the authoritative sgid and the tag is added.
    //
    // Measured against the reference's own WithMentions, not inferred. Applying
    // the read side's trim here "for consistency" is precisely the suppression
    // bug: it would let an appended whitespace reference collapse into the
    // authoritative sgid and silently drop the mention.
    const sgid = personSGID(VICTOR);
    const author = person(VICTOR, sgid);
    const tags = (content: string) => content.match(/<bc-attachment /g)?.length ?? 0;

    for (const whitespace of ["&#11;", "&#9;", "&#32;", "&nbsp;", "\t", " "]) {
      // Read side: erased by the trim, so the person is still named.
      expect(mentionedPersonIds(attachment(whitespace + sgid))).toEqual([VICTOR]);
      // Write side: not erased, so the tag is added rather than deduped away.
      expect(tags(withMentions(attachment(sgid + whitespace), [author]))).toBe(2);
    }

    // And the control on the other side of it: with no whitespace at all the
    // value IS the authoritative sgid, and the dedupe fires.
    expect(tags(withMentions(attachment(sgid), [author]))).toBe(1);
  });

  it("returns the content untouched when no people are given", () => {
    expect(withMentions("<div>On it.</div>", [])).toBe("<div>On it.</div>");
  });

  it("refuses the whole expansion when any person cannot be mentioned", () => {
    expect(() =>
      withMentions("<div>Hi</div>", [person(VICTOR, personSGID(VICTOR)), person(ANNIE, undefined)]),
    ).toThrow(BasecampError);
  });
});
