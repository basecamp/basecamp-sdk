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

    // A control byte anywhere in the URL, not merely in the authority.
    no(`gid://bc3/Person/${VICTOR}\u0001`);
    no(`gid://bc3/Person/${VICTOR}?x=\u0001`);
  });

  it("refuses a malformed id, and an id that cannot be a number without rounding", () => {
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/12a"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/0"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/-5"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/90071992547409911"))).toBeUndefined();
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

  it("decodes each character reference to the string Go decodes it to", () => {
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
      // Not in the carried subset, and left exactly as written.
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

    // The one place the text deliberately differs from Go's, and it is the
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

  it("scans a run of ampersands linearly", () => {
    // A ratio, not a wall clock: an absolute threshold measures the machine,
    // and this ran on CI. Quadratic would be near 16x for 4x the input; linear
    // is near 4x. The floor keeps a fast machine from making it vacuous by
    // timing two runs that both round to nothing.
    //
    // What this CANNOT catch is a regression to searching every table length
    // per "&" — that is linear too, just with a constant about 1.7x larger.
    // The bound itself is pinned by the table-shape test above.
    const scan = (n: number): number => {
      const input = `<bc-attachment sgid="${"&".repeat(n)}"></bc-attachment>`;
      for (let i = 0; i < 3; i++) mentionedPersonIds(input);
      let best = Infinity;
      for (let r = 0; r < 5; r++) {
        const start = performance.now();
        mentionedPersonIds(input);
        best = Math.min(best, performance.now() - start);
      }
      return best;
    };

    const small = scan(50_000);
    const large = scan(200_000);
    // Only meaningful once the smaller run is measurable at all.
    if (small < 0.5) return;
    // 10, not 8. Measured worst case over 120 trials with the CPU four times
    // oversubscribed: 4.42. Quadratic would be near 16, so 10 keeps the
    // discrimination while leaving room for a loaded CI box — this is the only
    // wall-clock assertion in the suite, and one that fails for reasons
    // unrelated to the code is worse than none.
    expect(large / small).toBeLessThan(10);
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
