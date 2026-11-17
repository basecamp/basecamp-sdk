/**
 * Tests for the ReportsService and TimesheetsService (generated from OpenAPI spec)
 *
 * Note: In generated services, timesheet operations moved from ReportsService
 * to a dedicated TimesheetsService:
 * - reports.timesheet() -> timesheets.report()
 * - reports.projectTimesheet() -> timesheets.forProject()
 * - reports.recordingTimesheet() -> timesheets.forRecording()
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { readFileSync } from "node:fs";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("TimesheetsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("report", () => {
    it("should return account-wide timesheet entries", async () => {
      const mockEntries = [
        {
          id: 1,
          date: "2024-01-15",
          hours: "4.5",
          description: "Development work",
          creator: { id: 100, name: "John Doe" },
        },
        {
          id: 2,
          date: "2024-01-16",
          hours: "8.0",
          description: "Code review",
          creator: { id: 101, name: "Jane Smith" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/reports/timesheet.json`, () => {
          return HttpResponse.json(mockEntries);
        })
      );

      const entries = await client.timesheets.report();
      expect(entries).toHaveLength(2);
      expect(entries[0]!.hours).toBe("4.5");
      expect(entries[1]!.date).toBe("2024-01-16");
    });

    it("should support date range filtering", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/timesheet.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("from")).toBe("2024-01-01");
          expect(url.searchParams.get("to")).toBe("2024-01-31");
          return HttpResponse.json([]);
        })
      );

      const entries = await client.timesheets.report({
        from: "2024-01-01",
        to: "2024-01-31",
      });
      expect(entries).toHaveLength(0);
    });

    it("should follow the Link header across 50-per-page pages", async () => {
      const requests: string[] = [];
      server.use(
        http.get(`${BASE_URL}/reports/timesheet.json`, ({ request }) => {
          const page = new URL(request.url).searchParams.get("page");
          requests.push(page ?? "1");
          if (page === "2") {
            return HttpResponse.json([{ id: 1, date: "2024-01-01", hours: "1.0" }], {
              headers: { "X-Total-Count": "3" },
            });
          }
          return HttpResponse.json(
            [
              { id: 3, date: "2024-01-03", hours: "3.0" },
              { id: 2, date: "2024-01-02", hours: "2.0" },
            ],
            {
              headers: {
                "X-Total-Count": "3",
                Link: `<${BASE_URL}/reports/timesheet.json?page=2>; rel="next"`,
              },
            }
          );
        })
      );

      const entries = await client.timesheets.report();
      expect(requests).toEqual(["1", "2"]);
      expect(entries.map((e) => e.id)).toEqual([3, 2, 1]);
      expect(entries.meta).toEqual({ totalCount: 3, truncated: false });
    });

    it("should support person filtering", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/timesheet.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("person_id")).toBe("12345");
          return HttpResponse.json([]);
        })
      );

      const entries = await client.timesheets.report({ personId: 12345 });
      expect(entries).toHaveLength(0);
    });
  });

  describe("forProject", () => {
    it("should return timesheet entries for a specific project", async () => {
      const mockEntries = [
        {
          id: 1,
          date: "2024-01-15",
          hours: "2.0",
          bucket: { id: 123, name: "Project X" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/projects/456/timesheet.json`, () => {
          return HttpResponse.json(mockEntries);
        })
      );

      const entries = await client.timesheets.forProject(456);
      expect(entries).toHaveLength(1);
      expect(entries[0]!.hours).toBe("2.0");
    });

    it("should support filtering options", async () => {

      server.use(
        http.get(`${BASE_URL}/projects/456/timesheet.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("from")).toBe("2024-02-01");
          expect(url.searchParams.get("person_id")).toBe("999");
          return HttpResponse.json([]);
        })
      );

      const entries = await client.timesheets.forProject(456, {
        from: "2024-02-01",
        personId: 999,
      });
      expect(entries).toHaveLength(0);
    });
  });

  describe("forRecording", () => {
    it("should return timesheet entries for a specific recording", async () => {
      const recordingId = 11111;
      const mockEntries = [
        {
          id: 1,
          date: "2024-01-20",
          hours: "1.5",
          parent: { id: recordingId, title: "Important Task" },
        },
      ];

      server.use(
        http.get(
          `${BASE_URL}/recordings/${recordingId}/timesheet.json`,
          () => {
            return HttpResponse.json(mockEntries);
          }
        )
      );

      const entries = await client.timesheets.forRecording(recordingId);
      expect(entries).toHaveLength(1);
      expect(entries[0]!.hours).toBe("1.5");
    });
  });
});

/**
 * `GetUpcomingSchedule` renders BC3's reduced calendar partials
 * (`app/views/api/schedules/calendar/`), not the per-resource ones. Until #635
 * the spec declared the shared `ScheduleEntry` and a half-modelled `Assignable`
 * instead, which in TypeScript was a compile-time lie rather than a throw:
 * `title` typed non-optional and arrived `undefined`, while `content`,
 * `recurring`, `completed`, `repeating`, `completion_url` and `comments_count`
 * were unreachable through the types.
 *
 * The body here is the shared fixture, which
 * `make check-fixture-coverage` validates against the generated schema.
 */
describe("ReportsService.upcoming", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  const fixture = JSON.parse(
    readFileSync(
      new URL("../../../spec/fixtures/schedules/upcoming.json", import.meta.url),
      "utf8",
    ),
  );

  it("types and returns the reduced calendar projection", async () => {
    let seenUrl = "";
    server.use(
      http.get(`${BASE_URL}/reports/schedules/upcoming.json`, ({ request }) => {
        seenUrl = request.url;
        return HttpResponse.json(fixture);
      }),
    );

    const result = await client.reports.upcoming("2026-06-01", "2026-06-30");

    // Both bounds are required, so they are positional parameters rather than an
    // options bag and always reach the query string.
    expect(seenUrl).toContain("window_starts_on=2026-06-01");
    expect(seenUrl).toContain("window_ends_on=2026-06-30");

    const entry = result.schedule_entries[0]!;
    expect(entry.summary).toBe("Team Meeting");
    // Emitted only by the calendar partial, and the flag that separates the two
    // entry arrays.
    expect(entry.recurring).toBe(false);
    // id + name only — no `type`, which is what broke a strict decode against
    // TodoBucket.
    expect(entry.bucket).toEqual({ id: 2085958499, name: "The Leto Laptop" });
    expect(entry.creator.avatar_url).toBeTruthy();

    const occurrence = result.recurring_schedule_entry_occurrences[0]!;
    expect(occurrence.recurring).toBe(true);
    expect(occurrence.all_day).toBe(true);
    // An all-day entry reads back as a bare date, not a timestamp.
    expect(occurrence.starts_at).toBe("2026-06-08");

    // BC3 spells the item text `content`. The retired schema declared `title`.
    const [todo, card] = result.assignables;
    expect(todo!.content).toBe("Ship the hardware");
    expect(todo!.type).toBe("todo");
    expect(todo!.parent.title).toBe("Launch: Hardware");
    expect(todo!.completed).toBe(true);
    expect(todo!.completion?.creator.name).toBe("Steve Marsh");

    // Kanban::Card and Step both define starts_on as a literal nil to duck-type
    // Todo, and the partial reads it unconditionally.
    expect(card!.starts_on).toBeNull();
    expect(card!.due_on).toBeNull();
    // The partial's one conditional key: absent, not null.
    expect(card!.completion).toBeUndefined();
    // Non-to-dos get a `_path` helper, which emits no host.
    expect(card!.completion_url).toBe("/999/buckets/2085958499/steps/1069479526/completions.json");
  });

  it("decodes an empty window as three empty arrays", async () => {
    server.use(
      http.get(`${BASE_URL}/reports/schedules/upcoming.json`, () =>
        HttpResponse.json({
          schedule_entries: [],
          recurring_schedule_entry_occurrences: [],
          assignables: [],
        }),
      ),
    );

    const result = await client.reports.upcoming("2026-01-01", "2026-01-31");

    expect(result.schedule_entries).toEqual([]);
    expect(result.recurring_schedule_entry_occurrences).toEqual([]);
    expect(result.assignables).toEqual([]);
  });
});

describe("ReportsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("overdue", () => {
    it("should fetch only the page asked for, without following the next link", async () => {
      const requests: string[] = [];
      server.use(
        http.get(`${BASE_URL}/reports/todos/overdue.json`, ({ request }) => {
          requests.push(new URL(request.url).search);
          return HttpResponse.json(
            {
              under_a_week_late: [{ id: 16 }],
              over_a_week_late: [],
              over_a_month_late: [],
              over_three_months_late: [],
            },
            { headers: { Link: `<${BASE_URL}/reports/todos/overdue.json?page=3>; rel="next"` } }
          );
        })
      );

      const report = await client.reports.overdue({ page: 2 });
      expect(requests).toEqual(["?page=2"]);
      expect(report.under_a_week_late![0]!.id).toBe(16);
    });
  });
});

describe("ReportsService.assigned", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  it("follows the Link header across 50-per-page pages, keeping the person and grouping", async () => {
    const requests: string[] = [];
    server.use(
      http.get(`${BASE_URL}/reports/todos/assigned/7`, ({ request }) => {
        const page = new URL(request.url).searchParams.get("page");
        requests.push(page ?? "1");
        const wrapper = { person: { id: 7, name: "Jane" }, grouped_by: "bucket" };
        if (page === "2") {
          return HttpResponse.json({ ...wrapper, todos: [{ id: 3 }] }, { headers: { "X-Total-Count": "3" } });
        }
        return HttpResponse.json(
          { ...wrapper, todos: [{ id: 1 }, { id: 2 }] },
          {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/reports/todos/assigned/7?page=2>; rel="next"`,
            },
          }
        );
      })
    );

    const report = await client.reports.assigned(7);
    expect(requests).toEqual(["1", "2"]);
    expect(report.person!.name).toBe("Jane");
    expect(report.grouped_by).toBe("bucket");
    expect(report.todos.map((t) => t.id)).toEqual([1, 2, 3]);
    expect(report.todos.meta).toEqual({ totalCount: 3, truncated: false });
  });
});
