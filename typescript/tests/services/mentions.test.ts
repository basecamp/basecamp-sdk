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

  it("refuses a malformed id, and an id that cannot be a number without rounding", () => {
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/12a"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/0"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/-5"))).toBeUndefined();
    expect(personIdFromSGID(legacySGID("gid://bc3/Person/90071992547409911"))).toBeUndefined();
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

  it("returns the content untouched when no people are given", () => {
    expect(withMentions("<div>On it.</div>", [])).toBe("<div>On it.</div>");
  });

  it("refuses the whole expansion when any person cannot be mentioned", () => {
    expect(() =>
      withMentions("<div>Hi</div>", [person(VICTOR, personSGID(VICTOR)), person(ANNIE, undefined)]),
    ).toThrow(BasecampError);
  });
});
