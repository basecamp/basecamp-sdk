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
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleAssignment = {
  id: 9007199254741623,
  app_url: "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
  content: "Program the flux capacitor",
  starts_on: null,
  due_on: "2026-03-15",
  bucket: {
    id: 2085958504,
    name: "The Leto Laptop",
    app_url: "https://3.basecamp.com/195539477/buckets/2085958504",
  },
  completed: false,
  type: "todo",
  assignees: [
    {
      id: 1049715913,
      name: "Victor Cooper",
      avatar_url: "https://bc3-production-assets-cdn.basecamp-static.com/people/1049715913/avatar.jpg",
    },
  ],
  comments_count: 0,
  has_description: false,
  priority_recording_id: 9007199254741700,
  parent: {
    id: 9007199254741601,
    title: "Development tasks",
    app_url: "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601",
  },
  children: [],
};

describe("ReportsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  it("should return assignments grouped into priorities and non_priorities", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments.json`, () => {
        return HttpResponse.json({
          priorities: [sampleAssignment],
          non_priorities: [],
        });
      })
    );

    const result = await client.reports.assignments();

    expect(result.priorities).toHaveLength(1);
    expect(result.priorities[0]!.priority_recording_id).toBe(9007199254741700);
    expect(result.priorities[0]!.assignees[0]!.avatar_url).toBe(sampleAssignment.assignees[0]!.avatar_url);
    expect(result.non_priorities).toEqual([]);
  });

  it("should surface missing-auth assignments responses as not found errors", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments.json`, () => {
        return HttpResponse.json({ error: "Not found" }, { status: 404 });
      })
    );

    await expect(client.reports.assignments()).rejects.toMatchObject({
      code: "not_found",
      httpStatus: 404,
      message: "Not found",
    });
  });

  it("should return completed assignments", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments/completed.json`, () => {
        return HttpResponse.json([
          {
            ...sampleAssignment,
            completed: true,
            priority_recording_id: undefined,
          },
        ]);
      })
    );

    const result = await client.reports.completedAssignments();

    expect(result).toHaveLength(1);
    expect(result[0]!.completed).toBe(true);
    expect(result[0]!.assignees[0]!.avatar_url).toBe(sampleAssignment.assignees[0]!.avatar_url);
  });

  it("should surface missing-auth completed-assignment responses as not found errors", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments/completed.json`, () => {
        return HttpResponse.json({ error: "Not found" }, { status: 404 });
      })
    );

    await expect(client.reports.completedAssignments()).rejects.toMatchObject({
      code: "not_found",
      httpStatus: 404,
      message: "Not found",
    });
  });

  it("should send scope when fetching due assignments", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments/due.json`, ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get("scope")).toBe("due_tomorrow");
        return HttpResponse.json([
          {
            ...sampleAssignment,
            due_on: "2026-03-22",
          },
        ]);
      })
    );

    const result = await client.reports.dueAssignments({ scope: "due_tomorrow" });

    expect(result).toHaveLength(1);
    expect(result[0]!.due_on).toBe("2026-03-22");
    expect(result[0]!.assignees[0]!.avatar_url).toBe(sampleAssignment.assignees[0]!.avatar_url);
  });

  it("should surface invalid due-assignment scope errors as validation errors", async () => {
    server.use(
      http.get(`${BASE_URL}/my/assignments/due.json`, () => {
        return HttpResponse.json(
          {
            error: "Invalid scope 'invalid'. Valid options: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later",
          },
          { status: 400 }
        );
      })
    );

    try {
      await client.reports.dueAssignments({ scope: "invalid" });
      expect.unreachable("expected dueAssignments to throw");
    } catch (err) {
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("validation");
      expect((err as BasecampError).httpStatus).toBe(400);
      expect((err as BasecampError).message).toContain("Invalid scope 'invalid'");
    }
  });
});

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
