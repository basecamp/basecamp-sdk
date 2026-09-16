/**
 * The typed person-id decode (src/services/base.ts `decodeResponsePersonIds`),
 * through the real generated services.
 *
 * Go reads a person id as `types.FlexibleInt64` at every field whose schema is
 * `Person` (go/pkg/types/flexible_int64.go:27-65), because every generated
 * service decodes through `Parse<Op>Response`. This SDK has no decoder, so the
 * decode is replayed at the sites `src/generated/person-id-sites.ts` lists per
 * operation — generated from the `x-go-type` marker, never from key names.
 *
 * The negative half matters as much as the positive one: `UpcomingSchedulePerson`,
 * `MyAssignmentAssignee` and `TemplateLibraryConfirmationPerson` are plain
 * `int64` in Go, where a string is a decode error rather than a person, so a
 * string there must reach the caller exactly as it arrived.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError, PeopleConfirmationRequiredError } from "../../src/errors.js";
import { PERSON_ID_SITES } from "../../src/generated/person-id-sites.js";

const BASE_URL = "https://3.basecampapi.com/12345";

type Rec = Record<string, unknown>;

async function refusal(promise: Promise<unknown>): Promise<BasecampError> {
  const error = await promise.then(
    () => undefined,
    (err: unknown) => err,
  );
  expect(error).toBeInstanceOf(BasecampError);
  return error as BasecampError;
}

function expectMalformed(error: BasecampError, pattern: RegExp): void {
  expect(error.code).toBe("api_error");
  expect(error.retryable).toBe(false);
  expect(error.httpStatus).toBeUndefined();
  expect(error.message).toMatch(pattern);
}

describe("person id typed decode", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({ accountId: "12345", accessToken: "test-token", enableRetry: false });
  });

  function serveComment(creator: unknown): void {
    server.use(http.get(`${BASE_URL}/comments/1`, () => HttpResponse.json({ id: 1, content: "c", creator })));
  }

  describe("the generated site table", () => {
    it("keys sites by operation and leaves the plain-int64 people out", () => {
      expect(PERSON_ID_SITES.GetComment).toEqual(["creator"]);
      expect(PERSON_ID_SITES.GetPersonProgress).toEqual(["events.[].attachments.[].creator", "events.[].creator", "person"]);
      expect(PERSON_ID_SITES.ListTodos).toContain("[].steps.[].assignees.[]");
      // Their people are UpcomingSchedulePerson / MyAssignmentAssignee /
      // OutOfOfficePerson: none of these operations holds a `Person` at all.
      for (const op of ["GetUpcomingSchedule", "GetMyAssignments", "GetMyDueAssignments", "GetMyCompletedAssignments", "DisableOutOfOffice"]) {
        expect(Object.hasOwn(PERSON_ID_SITES, op), op).toBe(false);
      }
    });
  });

  describe("single object read (GetComment)", () => {
    it("converts an untagged string id at a Person site", async () => {
      serveComment({ id: "7", name: "Jane" });
      const comment = await client.comments.get(1);
      expect(comment.creator!.id).toBe(7);
      expect((comment.creator as unknown as Rec).system_label).toBeUndefined();
    });

    it("reads the ParseInt grammar, not Number()", async () => {
      for (const [raw, expected] of [["+5", 5], ["007", 7], ["-5", -5]] as const) {
        serveComment({ id: raw, name: "Jane" });
        expect((await client.comments.get(1)).creator!.id, raw).toBe(expected);
      }
    });

    it("writes 0 on a SYNTAX refusal, without the normalizer's system_label", async () => {
      // FlexibleInt64 writes 0 and nothing else (flexible_int64.go:46); the
      // label is the normalizer's, and only for a person carrying
      // personable_type.
      for (const raw of ["basecamp", " 12", "1e3", "18446744073709551615x"]) {
        serveComment({ id: raw, name: "Basecamp" });
        const creator = (await client.comments.get(1)).creator as unknown as Rec;
        expect(creator.id, raw).toBe(0);
        expect(creator.system_label, raw).toBeUndefined();
      }
      serveComment({ id: "basecamp", name: "Basecamp", personable_type: "LocalPerson" });
      const tagged = (await client.comments.get(1)).creator as unknown as Rec;
      expect(tagged.id).toBe(0);
      expect(tagged.system_label).toBe("basecamp");
    });

    it("refuses the read on a RANGE refusal, tagged or not", async () => {
      for (const creator of [
        { id: "9223372036854775808", name: "Overflow" },
        { id: "18446744073709551616x", name: "Overflow" },
        { id: "9223372036854775808", name: "Overflow", personable_type: "User" },
      ]) {
        serveComment(creator);
        const error = await refusal(client.comments.get(1));
        expectMalformed(error, /GetComment returned a person id at creator that overflows int64/);
      }
    });

    it("refuses an id that is not an int64 at all", async () => {
      for (const id of [null, true, 1.5, [7], { n: 7 }, 1e300]) {
        serveComment({ id, name: "Jane" });
        const error = await refusal(client.comments.get(1));
        expectMalformed(error, /GetComment returned a person id at creator that is not a valid int64/);
      }
    });

    it("leaves a string past 2^53 in place, the personIdNumber residual", async () => {
      serveComment({ id: "9007199254740993", name: "Big" });
      expect((await client.comments.get(1)).creator!.id as unknown).toBe("9007199254740993");
    });

    it("leaves a person that has no id, is null, or is not an object alone", async () => {
      for (const creator of [{ name: "No id" }, null, "basecamp", 7]) {
        serveComment(creator);
        expect((await client.comments.get(1)).creator as unknown).toEqual(creator);
      }
    });
  });

  describe("plain-int64 people stay strict", () => {
    it("leaves GetUpcomingSchedule's creator, participants and assignees as they arrived", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/schedules/upcoming.json`, () =>
          HttpResponse.json({
            schedule_entries: [{ id: 1, creator: { id: "7", name: "A" }, participants: [{ id: "9223372036854775808", name: "B" }] }],
            assignables: [{ id: 2, assignees: [{ id: "7", name: "C" }] }],
            recurring_schedule_entry_occurrences: [],
          }),
        ),
      );
      const report = (await client.reports.upcoming("2024-01-01", "2024-01-31")) as unknown as {
        schedule_entries: { creator: Rec; participants: Rec[] }[];
        assignables: { assignees: Rec[] }[];
      };
      expect(report.schedule_entries[0]!.creator.id).toBe("7");
      expect(report.schedule_entries[0]!.participants[0]!.id).toBe("9223372036854775808");
      expect(report.assignables[0]!.assignees[0]!.id).toBe("7");
    });

    it("leaves GetMyAssignments' assignees as they arrived", async () => {
      server.use(
        http.get(`${BASE_URL}/my/assignments.json`, () =>
          HttpResponse.json({
            priorities: [{ id: 1, content: "t", assignees: [{ id: "7", name: "A" }] }],
            non_priorities: [{ id: 2, content: "u", assignees: [{ id: "basecamp", name: "B" }] }],
          }),
        ),
      );
      const result = (await client.myAssignments.myAssignments()) as unknown as {
        priorities: { assignees: Rec[] }[];
        non_priorities: { assignees: Rec[] }[];
      };
      expect(result.priorities[0]!.assignees[0]!.id).toBe("7");
      expect(result.non_priorities[0]!.assignees[0]!.id).toBe("basecamp");
    });

    it("does not reach TemplateLibraryConfirmationPerson in a 422 body", async () => {
      // The confirmation people ride an error response, which no typed decode
      // touches; their reader stays the strict positive-integer one, so a string
      // id is not a confirmation person.
      server.use(
        http.post(`${BASE_URL}/template_library/copies.json`, () =>
          HttpResponse.json(
            { error: "Adding people requires confirmation", people: [{ id: "7", name: "V", avatar_url: "https://example.test/a.png" }] },
            { status: 422 },
          ),
        ),
      );
      const error = await refusal(client.templates.createLibraryCopy({ templateRecordingId: 3, destinationParentId: 9 }));
      expect(error).not.toBeInstanceOf(PeopleConfirmationRequiredError);
      expect(error.code).toBe("validation");
    });
  });

  describe("paginated reads", () => {
    it("decodes a bare-array list on its FOLLOWED page, and refuses there too", async () => {
      let secondPageCreator: unknown = { id: "8", name: "Second" };
      server.use(
        http.get(`${BASE_URL}/recordings/5/comments.json`, ({ request }) => {
          if (new URL(request.url).searchParams.get("page") === "2") {
            return HttpResponse.json([{ id: 2, content: "b", creator: secondPageCreator }]);
          }
          return HttpResponse.json([{ id: 1, content: "a", creator: { id: "007", name: "First" } }], {
            headers: { Link: `<${BASE_URL}/recordings/5/comments.json?page=2>; rel="next"` },
          });
        }),
      );

      const comments = await client.comments.list(5);
      expect(comments.map((c) => c.creator!.id)).toEqual([7, 8]);

      secondPageCreator = { id: "9223372036854775808", name: "Overflow" };
      const error = await refusal(client.comments.list(5));
      expectMalformed(error, /ListComments returned a person id at \[\]\.creator that overflows int64/);
    });

    it("decodes nested array sites on every page (ListTodos steps.[].assignees)", async () => {
      server.use(
        http.get(`${BASE_URL}/todolists/3/todos.json`, ({ request }) => {
          const todo = (id: number, assignee: string) => ({
            id,
            content: "t",
            assignees: [{ id: assignee, name: "A" }],
            completion_subscribers: [{ id: assignee, name: "S" }],
            steps: [{ id: id * 10, assignees: [{ id: assignee, name: "S" }], completer: { id: "basecamp", name: "B" } }],
          });
          if (new URL(request.url).searchParams.get("page") === "2") return HttpResponse.json([todo(2, "+8")]);
          return HttpResponse.json([todo(1, "7")], {
            headers: { Link: `<${BASE_URL}/todolists/3/todos.json?page=2>; rel="next"` },
          });
        }),
      );

      const todos = (await client.todos.list(3)) as unknown as {
        assignees: Rec[];
        completion_subscribers: Rec[];
        steps: { assignees: Rec[]; completer: Rec }[];
      }[];
      expect(todos.map((t) => t.assignees[0]!.id)).toEqual([7, 8]);
      expect(todos.map((t) => t.completion_subscribers[0]!.id)).toEqual([7, 8]);
      expect(todos.map((t) => t.steps[0]!.assignees[0]!.id)).toEqual([7, 8]);
      expect(todos.map((t) => t.steps[0]!.completer.id)).toEqual([0, 0]);
      expect(todos[0]!.steps[0]!.completer.system_label).toBeUndefined();
    });

    it("decodes a wrapped paginated response on the wrapper and on every followed page (GetPersonProgress)", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/users/progress/9.json`, ({ request }) => {
          if (new URL(request.url).searchParams.get("page") === "2") {
            return HttpResponse.json({
              person: { id: "9", name: "P" },
              events: [{ id: 2, creator: { id: "12", name: "E2" }, attachments: [{ creator: { id: "13", name: "A2" } }] }],
            });
          }
          return HttpResponse.json(
            {
              person: { id: "9", name: "P" },
              events: [{ id: 1, creator: { id: "10", name: "E1" }, attachments: [{ creator: { id: "11", name: "A1" } }] }],
            },
            { headers: { Link: `<${BASE_URL}/reports/users/progress/9.json?page=2>; rel="next"` } },
          );
        }),
      );

      const progress = await client.reports.personProgress(9);
      expect(progress.person.id).toBe(9);
      const events = [...progress.events] as unknown as { creator: Rec; attachments: { creator: Rec }[] }[];
      expect(events.map((e) => e.creator.id)).toEqual([10, 12]);
      expect(events.map((e) => e.attachments[0]!.creator.id)).toEqual([11, 13]);
    });

    it("refuses a RANGE id on a wrapped response's followed page", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/users/progress/9.json`, ({ request }) => {
          if (new URL(request.url).searchParams.get("page") === "2") {
            return HttpResponse.json({ person: { id: 9, name: "P" }, events: [{ id: 2, creator: { id: "99999999999999999999", name: "X" } }] });
          }
          return HttpResponse.json(
            { person: { id: 9, name: "P" }, events: [{ id: 1, creator: { id: 10, name: "E1" } }] },
            { headers: { Link: `<${BASE_URL}/reports/users/progress/9.json?page=2>; rel="next"` } },
          );
        }),
      );
      const error = await refusal(client.reports.personProgress(9));
      expectMalformed(error, /GetPersonProgress returned a person id at events\.\[\]\.creator that overflows int64/);
    });
  });

  it("decodes a nested object site (GetCardTable lists.[].subscribers.[])", async () => {
    server.use(
      http.get(`${BASE_URL}/card_tables/4`, () =>
        HttpResponse.json({
          id: 4,
          creator: { id: "1", name: "C" },
          subscribers: [{ id: "2", name: "S" }],
          lists: [{ id: 40, creator: { id: "3", name: "L" }, subscribers: [{ id: "4", name: "LS" }, null, { name: "no id" }] }],
        }),
      ),
    );
    const table = (await client.cardTables.get(4)) as unknown as {
      creator: Rec;
      subscribers: Rec[];
      lists: { creator: Rec; subscribers: (Rec | null)[] }[];
    };
    expect(table.creator.id).toBe(1);
    expect(table.subscribers[0]!.id).toBe(2);
    expect(table.lists[0]!.creator.id).toBe(3);
    expect(table.lists[0]!.subscribers).toEqual([{ id: 4, name: "LS" }, null, { name: "no id" }]);
  });

  it("has no double effect on a gauge list, where both the positional pass and the decode run", async () => {
    server.use(
      http.get(`${BASE_URL}/projects/1/gauge/needles.json`, ({ request }) => {
        if (new URL(request.url).searchParams.get("page") === "2") {
          return HttpResponse.json([{ id: 2, description: "second", creator: { id: "basecamp", name: "B" } }]);
        }
        return HttpResponse.json([{ id: 1, description: "first", creator: { id: "007", name: "Padded" } }], {
          headers: { Link: `<${BASE_URL}/projects/1/gauge/needles.json?page=2>; rel="next"` },
        });
      }),
    );
    const needles = [...(await client.gauges.listGaugeNeedles(1))] as unknown as { creator: Rec }[];
    expect(needles[0]!.creator).toEqual({ id: 7, name: "Padded" });
    // The positional pass wrote 0 and the label; the decode saw a number and
    // left it, rather than relabelling "0".
    expect(needles[1]!.creator).toEqual({ id: 0, name: "B", system_label: "basecamp" });
  });
});
