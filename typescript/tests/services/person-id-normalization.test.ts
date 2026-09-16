/**
 * The pre-decode person-id normalizer (src/services/base.ts), against the
 * measured corpus.
 *
 * It runs the 74 rows through the REAL response pipeline — one notifications
 * response carrying 74 person-shaped creators, decoded by the real client —
 * rather than against the function directly, because the normalizer is what
 * every response body passes through and its whole job is what the caller ends
 * up holding. A row that "parses correctly" but is written into the body wrong
 * is the defect, not a detail.
 *
 * `system_label` and a numeric `id` are how this SDK spells Go's non-numeric
 * sentinel, and the danger runs one way: a real person handed back as `id: 0`
 * with a label IS `LocalPerson` to everything downstream. The corpus exists so
 * that direction cannot be re-entered by an edit that looks like a tidy-up.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { PERSON_ID_CORPUS, fitsNumber } from "../helpers/person-id-corpus.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("person id normalization", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  /**
   * Reads every corpus row back off one response.
   *
   * The people are `creator`s on notification unreads because that is a
   * response shape the SDK really decodes, and each carries `personable_type`,
   * which is what the normalizer keys on.
   */
  async function normalizedCreators(): Promise<Record<string, unknown>[]> {
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: PERSON_ID_CORPUS.map((row, index) => ({
            id: index + 1,
            title: `Notification ${index + 1}`,
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
            creator: { id: row.id, name: "Person", personable_type: "User" },
          })),
          reads: [],
          memories: [],
          bubble_ups_count: 0,
          scheduled_bubble_ups_count: 0,
        }),
      ),
    );

    const result = await client.myNotifications.myNotifications();
    expect(result.unreads).toBeDefined();
    expect(result.unreads!.length).toBe(PERSON_ID_CORPUS.length);
    // The creators come back typed as Person, whose `id` is a number — and
    // holding a string in it is precisely one of the outcomes under test, so
    // these are read as plain records.
    return result.unreads!.map((unread) => unread.creator as unknown as Record<string, unknown>);
  }

  it("writes each of the 74 measured rows the way coercePersonID does", async () => {
    expect(PERSON_ID_CORPUS.length).toBe(74);
    const creators = await normalizedCreators();

    for (const [index, row] of PERSON_ID_CORPUS.entries()) {
      const creator = creators[index];
      const where = JSON.stringify(row.id);

      if (row.go === "syntax") {
        // Go's non-numeric sentinel: id 0, the raw string kept as the label
        // (go/pkg/basecamp/normalize.go:66-67). `expect(...).toBe(0)` is
        // Object.is, so a `-0` from a `Number()`-based port fails here.
        expect(creator.id, where).toBe(0);
        expect(creator.system_label, where).toBe(row.id);
        continue;
      }

      if (row.go === "range") {
        // A range error leaves the string exactly as it arrived
        // (`:62-63`) so the reader refuses it. Writing 0 and a label here
        // would name the system actor for a string Go never read a value from.
        expect(creator.id, where).toBe(row.id);
        expect(creator.system_label, where).toBeUndefined();
        continue;
      }

      if (fitsNumber(row.value!)) {
        expect(creator.id, where).toBe(Number(row.value!));
        expect(creator.system_label, where).toBeUndefined();
        continue;
      }

      // RESIDUAL DIVERGENCE (11 rows). Go writes the int64; a JS `number`
      // cannot hold it, so the string is left in place — the RANGE treatment,
      // for the reasons argued at `personIdNumber` in src/person-id.ts. What
      // matters is that it is NOT rounded to a neighbouring person and NOT
      // collapsed to the system actor's 0.
      expect(creator.id, where).toBe(row.id);
      expect(creator.system_label, where).toBeUndefined();
    }
  });

  it("has exactly 11 rows JS cannot represent, and they are the large ones", () => {
    // Pinned as a count so the residual set cannot quietly grow: a change that
    // starts refusing representable ids has to come and edit this number.
    const residual = PERSON_ID_CORPUS.filter((row) => row.go === "value" && !fitsNumber(row.value!));
    expect(residual.length).toBe(11);
    expect(residual.every((row) => row.value! > 9007199254740991n || row.value! < -9007199254740991n)).toBe(true);
  });

  it("keeps a real person out of the system-actor sentinel", async () => {
    // The defect this corpus was written for, named on its own so a failure
    // reads as what it is. "+7" is person 7 to Go; it used to come back as
    // LocalPerson, because a `^-?\d+$` regex refuses the sign ParseInt takes.
    const creators = await normalizedCreators();
    const plusSeven = creators[PERSON_ID_CORPUS.findIndex((row) => row.id === "+7")];
    expect(plusSeven.id).toBe(7);
    expect(plusSeven.system_label).toBeUndefined();
  });

  it("leaves an object with no personable_type alone", async () => {
    // The normalizer keys on `personable_type`; a body without it reaches the
    // caller carrying its string id, which is the gap `personIdValue` in
    // mentions.ts closes on the read side.
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: [
            {
              id: 1,
              title: "No personable_type",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
              creator: { id: "+7", name: "Person" },
            },
          ],
          reads: [],
          memories: [],
          bubble_ups_count: 0,
          scheduled_bubble_ups_count: 0,
        }),
      ),
    );

    const result = await client.myNotifications.myNotifications();
    const creator = result.unreads![0].creator as unknown as Record<string, unknown>;
    expect(creator.id).toBe("+7");
    expect(creator.system_label).toBeUndefined();
  });
});
