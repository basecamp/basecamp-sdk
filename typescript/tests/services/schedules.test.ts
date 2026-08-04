/**
 * Tests for the Schedules service.
 *
 * Notes:
 * - Client-side checks: createEntry() rejects a missing summary, startsAt, or
 *   endsAt; the API validates the rest
 * - No domain-specific trashEntry() (use recordings.trash())
 * - The entry write surface is a triad: `replaceEntry` is the generated
 *   full-replace PUT, `updateEntry` and `editEntry` are the merge-safe
 *   composites layered over it in `services/schedules-extensions.ts`.
 *
 * `PUT /schedule_entries/{id}` is a full replace: `Schedules::EntriesController#update`
 * rebuilds the recordable from the submitted params, so a sparse PUT that omits
 * `description` erases it and one that omits `all_day` turns an all-day event
 * into a midnight-to-midnight timed one. Neither is a 422 — both are a `200`
 * that quietly clears, and only the next GET shows it.
 *
 * Three writable fields go the other way. BC3 seeds `participant_ids`, `url`
 * and `highlighted` from the existing recordable when the request does not
 * address them, so for those the SDK's job is to keep the key OFF the wire
 * unless the caller asked for it — and to never echo the response's `url` (the
 * entry's own API URL) into the request's `url` (the join link, read back as
 * `join_url`).
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { SchedulesService } from "../../src/services/schedules-extensions.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

/**
 * A GET-shaped ScheduleEntry carrying every field the triad reads.
 *
 * `url` and `join_url` are deliberately different values: `url` is the entry's
 * own Basecamp API URL and must never reach the request's `url` member, which
 * is the join link.
 */
const sampleEntry = (id = 4101, overrides: Record<string, unknown> = {}) => ({
  id,
  status: "active",
  type: "Schedule::Entry",
  title: "Team Meeting",
  url: `https://3.basecampapi.com/12345/buckets/1/schedule_entries/${id}.json`,
  app_url: `https://3.basecamp.com/12345/buckets/1/schedule_entries/${id}`,
  summary: "Team Meeting",
  description: "<div>Agenda in the doc.</div>",
  description_attachments: [],
  all_day: false,
  starts_at: "2026-06-05T06:00:00Z",
  ends_at: "2026-06-05T08:30:00Z",
  join_url: "https://meet.example.com/team",
  highlighted: true,
  participants: [
    { id: 1049715914, name: "Victor Cooper" },
    { id: 1049715915, name: "Annie Bryan" },
  ],
  ...overrides,
});

describe("SchedulesService", () => {
  let service: SchedulesService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.schedules;
  });

  describe("get", () => {
    it("should return a schedule by ID", async () => {
      const schedule = {
        id: 4001,
        title: "Schedule",
        status: "active",
        include_due_assignments: true,
        entries_count: 15,
      };

      server.use(
        http.get(`${BASE_URL}/schedules/4001`, () => {
          return HttpResponse.json(schedule);
        }),
      );

      const result = await service.get(4001);

      expect(result.id).toBe(4001);
      expect(result.title).toBe("Schedule");
      expect(result.include_due_assignments).toBe(true);
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/schedules/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.get(9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("listEntries", () => {
    it("should return schedule entries", async () => {
      const entries = [
        {
          id: 4101,
          summary: "Team Meeting",
          starts_at: "2024-12-15T09:00:00Z",
          description_attachments: [],
        },
        {
          id: 4102,
          summary: "Project Review",
          starts_at: "2024-12-16T14:00:00Z",
          description_attachments: [],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/schedules/4001/entries.json`, () => {
          return HttpResponse.json(entries);
        }),
      );

      const result = await service.listEntries(4001);

      expect(result).toHaveLength(2);
      expect(result[0].summary).toBe("Team Meeting");
      expect(result[1].summary).toBe("Project Review");
      // The required description_attachments array round-trips through the list.
      expect(result[0].description_attachments).toEqual([]);
    });

    it("should return empty array when no entries", async () => {
      server.use(
        http.get(`${BASE_URL}/schedules/4001/entries.json`, () => {
          return HttpResponse.json([]);
        }),
      );

      const result = await service.listEntries(4001);

      expect(result).toHaveLength(0);
    });
  });

  describe("getEntry", () => {
    it("should return a schedule entry by ID", async () => {
      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          return HttpResponse.json(sampleEntry());
        }),
      );

      const result = await service.getEntry(4101);

      expect(result.id).toBe(4101);
      expect(result.summary).toBe("Team Meeting");
      expect(result.all_day).toBe(false);
      // The join link and the entry's own API URL are separate keys.
      expect(result.join_url).toBe("https://meet.example.com/team");
      expect(result.url).toContain("basecampapi.com");
    });
  });

  describe("createEntry", () => {
    it("should create a new schedule entry", async () => {
      const newEntry = {
        id: 4201,
        summary: "New Event",
        starts_at: "2024-12-20T14:00:00Z",
        ends_at: "2024-12-20T15:00:00Z",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/schedules/4001/entries.json`, () => {
          return HttpResponse.json(newEntry);
        }),
      );

      const result = await service.createEntry(4001, {
        summary: "New Event",
        startsAt: "2024-12-20T14:00:00Z",
        endsAt: "2024-12-20T15:00:00Z",
      });

      expect(result.id).toBe(4201);
      expect(result.summary).toBe("New Event");
    });

    it("should pass subscriptions in request body", async () => {
      server.use(
        http.post(
          `${BASE_URL}/schedules/4001/entries.json`,
          async ({ request }) => {
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.subscriptions).toEqual([111, 222]);
            return HttpResponse.json({ id: 4202, summary: "Test" });
          },
        ),
      );

      const result = await service.createEntry(4001, {
        summary: "Quiet Event",
        startsAt: "2024-12-20T14:00:00Z",
        endsAt: "2024-12-20T15:00:00Z",
        subscriptions: [111, 222],
      });
      expect(result.id).toBe(4202);
    });

    it("should send all fields in request body", async () => {
      let capturedBody: Record<string, unknown> | null = null;

      server.use(
        http.post(
          `${BASE_URL}/schedules/4001/entries.json`,
          async ({ request }) => {
            capturedBody = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json({ id: 1, summary: "Test" });
          },
        ),
      );

      await service.createEntry(4001, {
        summary: "Test Event",
        startsAt: "2024-12-20T14:00:00Z",
        endsAt: "2024-12-20T15:00:00Z",
        description: "<p>Description</p>",
        participantIds: [1001, 1002],
        allDay: true,
        notify: true,
      });

      expect(capturedBody?.summary).toBe("Test Event");
      expect(capturedBody?.starts_at).toBe("2024-12-20T14:00:00Z");
      expect(capturedBody?.ends_at).toBe("2024-12-20T15:00:00Z");
      expect(capturedBody?.description).toBe("<p>Description</p>");
      expect(capturedBody?.participant_ids).toEqual([1001, 1002]);
      expect(capturedBody?.all_day).toBe(true);
      expect(capturedBody?.notify).toBe(true);
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing summary, startsAt, or endsAt", async () => {
      await expect(
        service.createEntry(1, { summary: "", startsAt: "2026-01-01T09:00:00Z", endsAt: "2026-01-01T10:00:00Z" })
      ).rejects.toMatchObject({ code: "validation", message: "Summary is required" });
      await expect(
        service.createEntry(1, { summary: "Standup", startsAt: "", endsAt: "2026-01-01T10:00:00Z" })
      ).rejects.toMatchObject({ code: "validation", message: "Starts at is required" });
      await expect(
        service.createEntry(1, { summary: "Standup", startsAt: "2026-01-01T09:00:00Z", endsAt: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Ends at is required" });
    });
  });

  describe("replaceEntry", () => {
    it("sends the sparse request verbatim with no GET", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleEntry());
        }),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      const result = await service.replaceEntry(4101, {
        summary: "Team Meeting",
        startsAt: "2026-06-05T06:00:00Z",
        endsAt: "2026-06-05T08:30:00Z",
      });

      expect(result.id).toBe(4101);
      // replace is the deliberate overwrite — it reads nothing first.
      expect(requests).toEqual(["PUT"]);
      expect(putBody.summary).toBe("Team Meeting");
      // Unaddressed carve-outs never appear: BC3 preserves them, and a body
      // that named them with null/[]/false would clear what it is holding.
      expect(putBody).not.toHaveProperty("participant_ids");
      expect(putBody).not.toHaveProperty("url");
      expect(putBody).not.toHaveProperty("highlighted");
      // The unaddressed full-state fields ARE cleared by the server. That is
      // the whole reason `updateEntry`/`editEntry` exist.
      expect(putBody).not.toHaveProperty("description");
      expect(putBody).not.toHaveProperty("all_day");
    });

    it("sends explicitly empty carve-outs", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.replaceEntry(4101, {
        summary: "Team Meeting",
        startsAt: "2026-06-05T06:00:00Z",
        endsAt: "2026-06-05T08:30:00Z",
        participantIds: [],
        url: "",
        highlighted: false,
      });

      expect(putBody.participant_ids).toEqual([]);
      expect(putBody.url).toBe("");
      expect(putBody.highlighted).toBe(false);
    });

    it("rejects a missing startsAt or endsAt", async () => {
      await expect(
        service.replaceEntry(4101, { startsAt: "", endsAt: "2026-06-05T08:30:00Z" })
      ).rejects.toMatchObject({ code: "validation", message: "Starts at is required" });
      await expect(
        service.replaceEntry(4101, { startsAt: "2026-06-05T06:00:00Z", endsAt: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Ends at is required" });
    });

    it("surfaces a 404 as BasecampError", async () => {
      server.use(
        http.put(`${BASE_URL}/schedule_entries/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(
        service.replaceEntry(9999, {
          startsAt: "2026-06-05T06:00:00Z",
          endsAt: "2026-06-05T08:30:00Z",
        }),
      ).rejects.toThrow(BasecampError);
    });
  });

  describe("updateEntry", () => {
    it("merges: a summary-only update preserves the times, description and all-day flag", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleEntry());
        }),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry(4101, { summary: "Team Meeting & Kickoff" }));
        }),
      );

      const entry = await service.updateEntry(4101, { summary: "Team Meeting & Kickoff" });

      expect(entry.id).toBe(4101);
      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody.summary).toBe("Team Meeting & Kickoff");
      // The four the caller never mentioned ride back verbatim. A sparse PUT
      // here would have been a silent 200 that erased them.
      expect(putBody.starts_at).toBe("2026-06-05T06:00:00Z");
      expect(putBody.ends_at).toBe("2026-06-05T08:30:00Z");
      expect(putBody.description).toBe("<div>Agenda in the doc.</div>");
      expect(putBody.all_day).toBe(false);
    });

    it("never echoes join_url, highlighted or participants from the read-back", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { summary: "Team Meeting & Kickoff" });

      // BC3 preserves all three server-side, so resending them is redundant at
      // best and wrong if the read raced a change. The `url` case is worse than
      // redundant: the response's `url` is the entry's own API URL, so echoing
      // it would write that into the join link.
      expect(putBody).not.toHaveProperty("participant_ids");
      expect(putBody).not.toHaveProperty("url");
      expect(putBody).not.toHaveProperty("highlighted");
      expect(Object.keys(putBody).sort()).toEqual([
        "all_day",
        "description",
        "ends_at",
        "starts_at",
        "summary",
      ]);
    });

    it("sends a caller-addressed join link and highlight, leaving participants alone", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () =>
          HttpResponse.json(sampleEntry(4101, { join_url: null, highlighted: false })),
        ),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, {
        url: "https://meet.example.com/new-room",
        highlighted: true,
      });

      // The caller's join link goes on the wire under the REQUEST spelling
      // `url`, even though the response returns it as `join_url`.
      expect(putBody.url).toBe("https://meet.example.com/new-room");
      expect(putBody.highlighted).toBe(true);
      // The carve-outs are independent, not all-or-nothing.
      expect(putBody).not.toHaveProperty("participant_ids");
      // Full state still rides along.
      expect(putBody.summary).toBe("Team Meeting");
      expect(putBody.starts_at).toBe("2026-06-05T06:00:00Z");
    });

    it("sends an explicitly empty join link, empty participant list and false highlight", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { url: "", highlighted: false, participantIds: [] });

      // Addressed and cleared, not absent: a `??`, `||` or truthiness test that
      // dropped these would hand the clear back to BC3's carve-out, which
      // preserves instead.
      expect(putBody).toHaveProperty("url");
      expect(putBody.url).toBe("");
      expect(putBody).toHaveProperty("highlighted");
      expect(putBody.highlighted).toBe(false);
      expect(putBody).toHaveProperty("participant_ids");
      expect(putBody.participant_ids).toEqual([]);
    });

    it("clears the description with an explicitly-passed empty string", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { description: "" });

      // A clear is an empty string, never an omission and never JSON null.
      expect(putBody).toHaveProperty("description");
      expect(putBody.description).toBe("");
      expect(putBody.summary).toBe("Team Meeting");
    });

    it("round-trips an all-day entry's bare dates and true all_day verbatim", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () =>
          HttpResponse.json(
            sampleEntry(4101, {
              all_day: true,
              starts_at: "2026-06-05",
              ends_at: "2026-06-06",
            }),
          ),
        ),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { summary: "Offsite" });

      // The wire value is a bare date for an all-day entry and a timestamp
      // otherwise. Parsing into a Date would normalise the two spellings into
      // one and rewrite the entry's shape.
      expect(putBody.all_day).toBe(true);
      expect(putBody.starts_at).toBe("2026-06-05");
      expect(putBody.ends_at).toBe("2026-06-06");
    });

    it("sends notify only when addressed", async () => {
      const bodies: Record<string, unknown>[] = [];

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          bodies.push((await request.json()) as Record<string, unknown>);
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { summary: "Quiet" });
      await service.updateEntry(4101, { summary: "Loud", notify: true });
      await service.updateEntry(4101, { summary: "Quiet again", notify: false });

      // A directive, not state: never seeded, and `false` is an address.
      expect(bodies[0]).not.toHaveProperty("notify");
      expect(bodies[1]?.notify).toBe(true);
      expect(bodies[2]).toHaveProperty("notify");
      expect(bodies[2]?.notify).toBe(false);
    });

    it("hooks observe the wire operations GetScheduleEntry then ReplaceScheduleEntry", async () => {
      const operations: string[] = [];
      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            operations.push(info.operation);
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
      );

      await hookedClient.schedules.updateEntry(4101, { summary: "observed" });

      // The composite is not a synthetic operation: hooks see the two real ones.
      expect(operations).toEqual(["GetScheduleEntry", "ReplaceScheduleEntry"]);
    });
  });

  describe("editEntry", () => {
    it("hands the callback current state and PUTs the full state back", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleEntry());
        }),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      const entry = await service.editEntry(4101, (e) => {
        expect(e.summary).toBe("Team Meeting");
        expect(e.startsAt).toBe("2026-06-05T06:00:00Z");
        expect(e.endsAt).toBe("2026-06-05T08:30:00Z");
        expect(e.description).toBe("<div>Agenda in the doc.</div>");
        expect(e.allDay).toBe(false);
        // The carve-outs are seeded for READING. `url` comes from `join_url`,
        // never from the response's own `url`.
        expect(e.url).toBe("https://meet.example.com/team");
        expect(e.highlighted).toBe(true);
        expect(e.participantIds).toEqual([1049715914, 1049715915]);
        expect(e.notify).toBeUndefined();
        e.summary = `🚨 ${e.summary}`;
      });

      expect(entry.id).toBe(4101);
      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody.summary).toBe("🚨 Team Meeting");
      expect(putBody.description).toBe("<div>Agenda in the doc.</div>");
      expect(putBody.all_day).toBe(false);
    });

    it("leaves untouched carve-outs off the wire even though it read them", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        // Reading a carve-out is not addressing it.
        expect(e.url).toBe("https://meet.example.com/team");
        e.summary = "Team Sync";
      });

      expect(putBody.summary).toBe("Team Sync");
      expect(putBody).not.toHaveProperty("participant_ids");
      expect(putBody).not.toHaveProperty("url");
      expect(putBody).not.toHaveProperty("highlighted");
    });

    // The reason the contract is setter-invocation dirty tracking rather than a
    // snapshot diff. A value comparison would conclude nothing changed and omit
    // both, handing the write back to BC3's carve-out. Intent is not
    // recoverable from a value.
    it("sends a carve-out assigned the same value the read returned", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        e.url = e.url;
        e.highlighted = e.highlighted;
      });

      expect(putBody).toHaveProperty("url");
      expect(putBody.url).toBe("https://meet.example.com/team");
      expect(putBody).toHaveProperty("highlighted");
      expect(putBody.highlighted).toBe(true);
      // Still independent: participants were not assigned.
      expect(putBody).not.toHaveProperty("participant_ids");
    });

    it("sends a participant list reassigned to its own read-back value", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        e.participantIds = e.participantIds;
      });

      expect(putBody.participant_ids).toEqual([1049715914, 1049715915]);
    });

    it("clears the join link, the highlight and the participants with explicit empties", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        e.url = "";
        e.highlighted = false;
        e.participantIds = [];
      });

      expect(putBody.url).toBe("");
      expect(putBody.highlighted).toBe(false);
      expect(putBody.participant_ids).toEqual([]);
    });

    it("clears the description by setting it empty — present-and-empty in the PUT body", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        e.description = "";
      });

      // Present and empty, not omitted: on a full-replace endpoint an omission
      // is the server's own clear-by-default and reads as an accident.
      expect(putBody).toHaveProperty("description");
      expect(putBody.description).toBe("");
      expect(putBody.summary).toBe("Team Meeting");
      expect(putBody.starts_at).toBe("2026-06-05T06:00:00Z");
      expect(putBody.ends_at).toBe("2026-06-05T08:30:00Z");
      expect(putBody.all_day).toBe(false);
      expect(putBody).not.toHaveProperty("participant_ids");
      expect(putBody).not.toHaveProperty("url");
      expect(putBody).not.toHaveProperty("highlighted");
    });

    it("sends notify only when the callback assigns it", async () => {
      const bodies: Record<string, unknown>[] = [];

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          bodies.push((await request.json()) as Record<string, unknown>);
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        e.summary = "Quiet";
      });
      await service.editEntry(4101, (e) => {
        e.notify = false;
      });

      expect(bodies[0]).not.toHaveProperty("notify");
      expect(bodies[1]).toHaveProperty("notify");
      expect(bodies[1]?.notify).toBe(false);
    });

    it("aborts without a PUT when the callback throws", async () => {
      let putCount = 0;

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, () => {
          putCount++;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await expect(
        service.editEntry(4101, () => {
          throw new Error("abort");
        }),
      ).rejects.toThrow("abort");
      expect(putCount).toBe(0);
    });

    it("supports async callbacks", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, async (e) => {
        e.description = await Promise.resolve("<div>async agenda</div>");
      });

      expect(putBody.description).toBe("<div>async agenda</div>");
      expect(putBody.summary).toBe("Team Meeting");
    });

    it("hooks observe the wire operations GetScheduleEntry then ReplaceScheduleEntry", async () => {
      const operations: string[] = [];
      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            operations.push(info.operation);
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
        http.put(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(sampleEntry())),
      );

      await hookedClient.schedules.editEntry(4101, (e) => {
        e.summary = "observed";
      });

      expect(operations).toEqual(["GetScheduleEntry", "ReplaceScheduleEntry"]);
    });
  });

  describe("getEntryOccurrence", () => {
    it("should return a specific occurrence", async () => {
      const entry = {
        id: 4101,
        summary: "Weekly Meeting",
        starts_at: "2024-12-22T09:00:00Z",
        ends_at: "2024-12-22T10:00:00Z",
      };

      server.use(
        http.get(
          `${BASE_URL}/schedule_entries/4101/occurrences/2024-12-22`,
          () => {
            return HttpResponse.json(entry);
          },
        ),
      );

      const result = await service.getEntryOccurrence(4101, "2024-12-22");

      expect(result.starts_at).toBe("2024-12-22T09:00:00Z");
    });

  });

  describe("updateSettings", () => {
    it("should update schedule settings", async () => {
      const schedule = {
        id: 4001,
        title: "Schedule",
        include_due_assignments: false,
      };

      server.use(
        http.put(`${BASE_URL}/schedules/4001`, () => {
          return HttpResponse.json(schedule);
        }),
      );

      const result = await service.updateSettings(4001, {
        includeDueAssignments: false,
      });

      expect(result.include_due_assignments).toBe(false);
    });

    it("should send include_due_assignments in request body", async () => {
      let capturedBody: { include_due_assignments?: boolean } | null = null;

      server.use(
        http.put(`${BASE_URL}/schedules/4001`, async ({ request }) => {
          capturedBody = (await request.json()) as {
            include_due_assignments?: boolean;
          };
          return HttpResponse.json({ id: 4001, title: "Schedule" });
        }),
      );

      await service.updateSettings(4001, { includeDueAssignments: true });

      expect(capturedBody?.include_due_assignments).toBe(true);
    });
  });

  // --- #576: a malformed GET field must never reach the full-replace PUT ----
  //
  // `updateEntry`/`editEntry` GET the entry, read each writable field, and PUT
  // the FULL representation back, so every value read is written -- including
  // one the caller never mentioned. `?? ""` coalesces only null and undefined,
  // so it rules out *erasure* while leaving *corruption* wide open.
  //
  // TypeScript has no runtime decoder to catch this -- `schema.d.ts` is erased
  // at build time, so `ScheduleEntry` is a compile-time claim nothing
  // validates. That places this composite with Python and Ruby, not with Go and
  // Swift.
  //
  // The assertion that matters is the ORDERING: `requests` must be ["GET"] --
  // exactly one request. A guard that fires after the PUT has already lost the
  // field.
  describe("malformed writable fields (#576)", () => {
    const malformed: [string, unknown][] = [
      ["false", false],
      ["zero", 0],
      ["empty array", []],
      ["empty object", {}],
      ["number", 42],
      ["true", true],
      ["array", ["x"]],
      ["object", { a: 1 }],
    ];

    const writableStrings = ["summary", "starts_at", "ends_at", "description"] as const;

    // Serve a GET carrying `body` and a PUT that records that it happened.
    const serve = (body: unknown, requests: string[]) => {
      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/schedule_entries/4101`, () => {
          requests.push("PUT");
          return HttpResponse.json(sampleEntry());
        }),
      );
    };

    const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
      promise.then(
        () => {
          throw new Error("expected the call to reject, but it resolved");
        },
        (error: unknown) => error,
      );

    // Asserting only the message is vacuous about the taxonomy: a wrong `code`
    // satisfies it. The value arrived in a successful API response, so this is
    // `api_error` -- the caller passed nothing wrong.
    const expectResponseError = (error: unknown, pattern: RegExp, requests: string[]) => {
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(pattern);
      expect(requests).toEqual(["GET"]);
    };

    for (const field of writableStrings) {
      it.each(malformed)(
        `updateEntry refuses a %s ${field} before writing`,
        async (_label, value) => {
          const requests: string[] = [];
          serve(sampleEntry(4101, { [field]: value }), requests);

          const error = await rejection(service.updateEntry(4101, { summary: "New summary" }));
          expectResponseError(
            error,
            new RegExp(`Schedule entry field "${field}" is not a string`),
            requests,
          );
        },
      );

      it(`editEntry refuses a malformed ${field} before writing`, async () => {
        const requests: string[] = [];
        serve(sampleEntry(4101, { [field]: 42 }), requests);

        const error = await rejection(
          service.editEntry(4101, (e) => {
            e.summary = "New summary";
          }),
        );
        expectResponseError(
          error,
          new RegExp(`Schedule entry field "${field}" is not a string`),
          requests,
        );
      });
    }

    // `all_day` is @required and NOT NULL DEFAULT false, and every partial
    // emits it. It needs the boolean guard specifically: the value it most
    // needs to admit is `false`, which every truthiness idiom would read as
    // missing -- and defaulting a missing value to `false` converts an all-day
    // event into a midnight-to-midnight timed one on a call that only changed
    // the summary.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("updateEntry refuses a %s all_day before writing", async (_label, value) => {
      const requests: string[] = [];
      const body: Record<string, unknown> = sampleEntry(4101, { all_day: value });
      if (value === undefined) delete body["all_day"];
      serve(body, requests);

      const error = await rejection(service.updateEntry(4101, { summary: "New summary" }));
      expectResponseError(error, /Schedule entry field "all_day" is required/, requests);
    });

    it.each([
      ["string", "yes"],
      ["zero", 0],
      ["one", 1],
      ["array", []],
      ["object", {}],
    ])("updateEntry refuses a %s all_day before writing", async (_label, value) => {
      const requests: string[] = [];
      serve(sampleEntry(4101, { all_day: value }), requests);

      const error = await rejection(service.updateEntry(4101, { summary: "New summary" }));
      expectResponseError(error, /Schedule entry field "all_day" is not a boolean/, requests);
    });

    it.each([
      ["absent", undefined],
      ["null", null],
    ])("editEntry refuses a %s all_day before writing", async (_label, value) => {
      const requests: string[] = [];
      const body: Record<string, unknown> = sampleEntry(4101, { all_day: value });
      if (value === undefined) delete body["all_day"];
      serve(body, requests);

      const error = await rejection(
        service.editEntry(4101, (e) => {
          e.summary = "New summary";
        }),
      );
      expectResponseError(error, /Schedule entry field "all_day" is required/, requests);
    });

    // A legitimately `false` all_day is NOT malformed. The guard must admit the
    // value it exists to protect.
    it("accepts a false all_day and resends it", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () =>
          HttpResponse.json(sampleEntry(4101, { all_day: false })),
        ),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { summary: "New summary" });

      expect(putBody).toHaveProperty("all_day");
      expect(putBody.all_day).toBe(false);
    });

    // `summary` is @required and `Schedule::Entry#summary` is
    // `super.presence || "Untitled"`, so BC3 can never render it blank. An
    // absent, null or blank summary in a 2xx body is a MALFORMED RESPONSE, not
    // an empty summary -- and coalescing it to "" would blank the real summary
    // on a call that only touched the description.
    //
    // `starts_at`/`ends_at` are @required and NOT NULL, so they get the same
    // treatment.
    const requiredStrings = ["summary", "starts_at", "ends_at"] as const;

    for (const field of requiredStrings) {
      it.each([
        ["absent", undefined],
        ["null", null],
        ["blank", ""],
        // BC3 blanks via `presence`, whose blank case includes whitespace-only.
        ["whitespace", "   "],
      ])(`updateEntry refuses a %s ${field} before writing`, async (_label, value) => {
        const requests: string[] = [];
        const body: Record<string, unknown> = sampleEntry(4101, { [field]: value });
        if (value === undefined) delete body[field];
        serve(body, requests);

        const error = await rejection(
          service.updateEntry(4101, { description: "<div>New agenda.</div>" }),
        );
        expectResponseError(
          error,
          new RegExp(`Schedule entry field "${field}" is required`),
          requests,
        );
      });

      it.each([
        ["absent", undefined],
        ["null", null],
        ["blank", ""],
        ["whitespace", "   "],
      ])(`editEntry refuses a %s ${field} before writing`, async (_label, value) => {
        const requests: string[] = [];
        const body: Record<string, unknown> = sampleEntry(4101, { [field]: value });
        if (value === undefined) delete body[field];
        serve(body, requests);

        const error = await rejection(
          service.editEntry(4101, (e) => {
            e.description = "<div>New agenda.</div>";
          }),
        );
        expectResponseError(
          error,
          new RegExp(`Schedule entry field "${field}" is required`),
          requests,
        );
      });
    }

    // The other half of the rule: for an OPTIONAL field, absent and null are
    // not malformed, they are empty. Guarding types must not turn a
    // legitimately blank description into an error.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("treats a %s description as genuinely empty", async (_label, value) => {
      let putBody: Record<string, unknown> = {};
      const body: Record<string, unknown> = sampleEntry(4101, { description: value });
      if (value === undefined) delete body["description"];

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(body)),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.updateEntry(4101, { summary: "set by the caller" });

      expect(putBody["description"]).toBe("");
      expect(putBody["summary"]).toBe("set by the caller");
    });

    // `join_url`, `highlighted` and `participants` are optional in the
    // response -- the entry partial emits them, the reduced calendar partial
    // does not -- so their absence must not break an edit that never touches
    // them.
    it("seeds the edit carve-outs to empty when the response omits them", async () => {
      let putBody: Record<string, unknown> = {};
      const body: Record<string, unknown> = sampleEntry();
      delete body["join_url"];
      delete body["highlighted"];
      delete body["participants"];

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => HttpResponse.json(body)),
        http.put(`${BASE_URL}/schedule_entries/4101`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleEntry());
        }),
      );

      await service.editEntry(4101, (e) => {
        expect(e.url).toBe("");
        expect(e.highlighted).toBe(false);
        expect(e.participantIds).toEqual([]);
        e.summary = "New summary";
      });

      expect(putBody).not.toHaveProperty("url");
      expect(putBody).not.toHaveProperty("highlighted");
      expect(putBody).not.toHaveProperty("participant_ids");
    });

    it("editEntry refuses a non-boolean highlighted before writing", async () => {
      const requests: string[] = [];
      serve(sampleEntry(4101, { highlighted: "yes" }), requests);

      const error = await rejection(
        service.editEntry(4101, (e) => {
          e.summary = "New summary";
        }),
      );
      expectResponseError(error, /Schedule entry field "highlighted" is not a boolean/, requests);
    });

    it("editEntry refuses a malformed participants list before writing", async () => {
      const requests: string[] = [];
      serve(sampleEntry(4101, { participants: [{ name: "No ID" }] }), requests);

      const error = await rejection(
        service.editEntry(4101, (e) => {
          e.summary = "New summary";
        }),
      );
      expectResponseError(error, /Schedule entry field "participants"\[0\] has no "id"/, requests);
    });

    // One level up from the field guards: a successful GET can return a
    // scalar, an array or null, and reading a property off null throws a raw
    // TypeError instead of the documented statusless api_error.
    it.each([
      ["array", []],
      ["string", "entry"],
      ["number", 42],
      ["null", null],
      ["boolean", true],
    ])("updateEntry refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(service.updateEntry(4101, { summary: "New summary" }));
      expectResponseError(error, /GetScheduleEntry returned/, requests);
    });

    it.each([
      ["array", []],
      ["null", null],
    ])("editEntry refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(
        service.editEntry(4101, (e) => {
          e.summary = "New summary";
        }),
      );
      expectResponseError(error, /GetScheduleEntry returned/, requests);
    });
  });

  // Note: trashEntry() is on RecordingsService, not SchedulesService (spec-conformant)
  // Use client.recordings.trash(entryId) instead
});
