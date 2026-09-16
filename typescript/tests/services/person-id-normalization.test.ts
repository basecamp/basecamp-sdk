/**
 * The pre-decode person-id normalizer (src/services/base.ts), against the
 * measured corpus.
 *
 * It runs the 74 rows through the REAL response pipeline — one notifications
 * response carrying 74 person-shaped objects in each of the four positions the
 * normalizer finds a person in — rather than against the function directly,
 * because the normalizer is what every response body passes through and its
 * whole job is what the caller ends up holding. A row that "parses correctly"
 * but is written into the body wrong is the defect, not a detail.
 *
 * The four positions are the two passes Go runs
 * (`go/pkg/basecamp/normalize.go:117-134`): an object carrying
 * `personable_type`, and — whether or not it carries one — a `creator` object, a
 * `participants` element, and the same `creator` nested deeper in the tree.
 * They are pinned together because they must land on the SAME table; the second
 * pass existing at all is the difference between `creator.id` reaching a caller
 * as `7` and reaching it as the string `"007"` in a field typed `number`.
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

/** Where a person can sit in a body, and how to read it back afterwards. */
const SHAPES = [
  ["personable_type person", (unread: Record<string, unknown>) => unread.person as Record<string, unknown>],
  ["bare creator", (unread: Record<string, unknown>) => unread.creator as Record<string, unknown>],
  [
    "participants element",
    (unread: Record<string, unknown>) => (unread.participants as Record<string, unknown>[])[0],
  ],
  [
    "creator nested under a comment",
    (unread: Record<string, unknown>) => (unread.comment as Record<string, unknown>).creator as Record<string, unknown>,
  ],
] as const;

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
   * Serves one notification unread per corpus row, with that row's id in all
   * four positions, and returns the unreads as plain records.
   *
   * Notification unreads because that is a response shape the SDK really
   * decodes. Only the first position carries `personable_type`: the other three
   * are found by structural position alone, which is the whole point of the
   * second pass.
   */
  async function normalizedUnreads(): Promise<Record<string, unknown>[]> {
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: PERSON_ID_CORPUS.map((row, index) => ({
            id: index + 1,
            title: `Notification ${index + 1}`,
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
            person: { id: row.id, name: "Person", personable_type: "User" },
            creator: { id: row.id, name: "Person" },
            participants: [{ id: row.id, name: "Person" }],
            comment: { creator: { id: row.id, name: "Person" } },
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
    // The unreads come back typed, and `creator.id` is typed as a number —
    // holding a string in it is precisely one of the outcomes under test, so
    // these are read as plain records.
    return result.unreads!.map((unread) => unread as unknown as Record<string, unknown>);
  }

  it("writes each of the 74 measured rows the way coercePersonID does, in all four positions", async () => {
    expect(PERSON_ID_CORPUS.length).toBe(74);
    const unreads = await normalizedUnreads();

    for (const [shape, read] of SHAPES) {
      for (const [index, row] of PERSON_ID_CORPUS.entries()) {
        const person = read(unreads[index]);
        const where = `${shape} ${JSON.stringify(row.id)}`;

        if (row.go === "syntax") {
          // Go's non-numeric sentinel: id 0, the raw string kept as the label
          // (go/pkg/basecamp/normalize.go:66-67). `expect(...).toBe(0)` is
          // Object.is, so a `-0` from a `Number()`-based port fails here.
          expect(person.id, where).toBe(0);
          expect(person.system_label, where).toBe(row.id);
          continue;
        }

        if (row.go === "range") {
          // A range error leaves the string exactly as it arrived (`:62-63`) so
          // the reader refuses it. Writing 0 and a label here would name the
          // system actor for a string Go read no value from at all.
          expect(person.id, where).toBe(row.id);
          expect(person.system_label, where).toBeUndefined();
          continue;
        }

        if (fitsNumber(row.value!)) {
          expect(person.id, where).toBe(Number(row.value!));
          expect(person.system_label, where).toBeUndefined();
          continue;
        }

        // RESIDUAL DIVERGENCE (11 rows). Go writes the int64; a JS `number`
        // cannot hold it, so the string is left in place — the RANGE treatment,
        // for the reasons argued at `personIdNumber` in src/person-id.ts. What
        // matters is that it is NOT rounded to a neighbouring person and NOT
        // collapsed to the system actor's 0.
        expect(person.id, where).toBe(row.id);
        expect(person.system_label, where).toBeUndefined();
      }
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
    const unreads = await normalizedUnreads();
    const plusSeven = unreads[PERSON_ID_CORPUS.findIndex((row) => row.id === "+7")];
    for (const [shape, read] of SHAPES) {
      expect(read(plusSeven).id, shape).toBe(7);
      expect(read(plusSeven).system_label, shape).toBeUndefined();
    }
  });

  it("finds an embedded person that carries no personable_type", async () => {
    // Go's second pass, targeting people by structural position
    // (`go/pkg/basecamp/normalize.go:83-104`), because embedded
    // creator/participants people frequently omit `personable_type` and the
    // first pass therefore skips exactly the payloads this exists to fix. Until
    // this pass existed, `"007"` reached the caller as the STRING "007" in a
    // field typed `number` — this SDK has no runtime decoder behind the
    // normalizer to convert it later, which is why the gap was observable here
    // and in none of the other six ports.
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: [
            {
              id: 1,
              title: "No personable_type anywhere",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
              creator: { id: "007", name: "Person" },
              participants: [{ id: "basecamp", name: "Basecamp" }, { id: "+7", name: "Person" }],
              comment: { creator: { id: "9223372036854775808", name: "Person" } },
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
    const unread = result.unreads![0] as unknown as Record<string, unknown>;

    // "007" is 7: leading zeros carry no meaning at base 10.
    expect((unread.creator as Record<string, unknown>).id).toBe(7);
    const participants = unread.participants as Record<string, unknown>[];
    expect(participants[0].id).toBe(0);
    expect(participants[0].system_label).toBe("basecamp");
    expect(participants[1].id).toBe(7);
    // Nested at depth, and still a range refusal that leaves the string.
    expect(((unread.comment as Record<string, unknown>).creator as Record<string, unknown>).id).toBe(
      "9223372036854775808",
    );
  });

  it("is idempotent over a person both passes find", async () => {
    // An object can be hit by both passes — a `creator` that also carries
    // `personable_type`. The second coercion has to be a no-op, which it is
    // because the id is no longer a string by then; a port that re-derived the
    // label from the already-rewritten id would set `system_label` to "0" here.
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: [
            {
              id: 1,
              title: "Both passes",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
              creator: { id: "basecamp", name: "Basecamp", personable_type: "LocalPerson" },
              participants: [{ id: "+7", name: "Person", personable_type: "User" }],
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
    const unread = result.unreads![0] as unknown as Record<string, unknown>;
    const creator = unread.creator as Record<string, unknown>;
    expect(creator.id).toBe(0);
    expect(creator.system_label).toBe("basecamp");
    expect((unread.participants as Record<string, unknown>[])[0].id).toBe(7);
  });

  it("leaves a creator or participants that is not a person shape alone", async () => {
    // Go's second pass asserts the types — `.(map[string]any)` for the creator,
    // `[]any` with a map per element for participants — and skips anything
    // else. A `creator` that is a string is not a person, and touching it would
    // be inventing structure the response never had.
    server.use(
      http.get(`${BASE_URL}/my/readings.json`, () =>
        HttpResponse.json({
          unreads: [
            {
              id: 1,
              title: "Not person shapes",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
              creator: "basecamp",
              participants: ["basecamp", 7, null],
            },
            {
              id: 2,
              title: "Participants is not an array",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
              participants: { id: "007" },
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
    const [first, second] = result.unreads!.map((unread) => unread as unknown as Record<string, unknown>);
    expect(first.creator).toBe("basecamp");
    expect(first.participants).toEqual(["basecamp", 7, null]);
    // A `participants` object is not a list of people — it is just an object,
    // and it is reached by the ordinary recursion, which finds no person in it.
    expect(second.participants).toEqual({ id: "007" });
  });
});
