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
import { BasecampError } from "../../src/errors.js";
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

describe("ReportsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("myAssignments", () => {
    it("should return grouped priority and non-priority assignments", async () => {
      const mockAssignments = {
        priorities: [
          {
            id: 1,
            content: "Priority assignment",
            completed: false,
            type: "Todo",
            comments_count: 2,
            has_description: true,
            children: [
              {
                id: 2,
                content: "Nested assignment",
                completed: true,
                type: "Todo",
                comments_count: 0,
                has_description: false,
              },
            ],
          },
        ],
        non_priorities: [
          {
            id: 3,
            content: "Backlog assignment",
            completed: true,
            type: "Todo",
            comments_count: 1,
            has_description: false,
          },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/my/assignments.json`, () => {
          return HttpResponse.json(mockAssignments);
        })
      );

      const result = await client.reports.myAssignments();
      expect(result.priorities).toHaveLength(1);
      expect(result.priorities[0]!.content).toBe("Priority assignment");
      expect(result.priorities[0]!.children?.[0]!.content).toBe("Nested assignment");
      expect(result.non_priorities[0]!.completed).toBe(true);
    });

    it("should surface unauthorized errors", async () => {
      server.use(
        http.get(`${BASE_URL}/my/assignments.json`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      await expect(client.reports.myAssignments()).rejects.toThrow(BasecampError);
    });
  });

  describe("myAssignmentsCompleted", () => {
    it("should return completed assignments", async () => {
      const mockAssignments = [
        {
          id: 10,
          content: "Completed assignment",
          completed: true,
          type: "Todo",
          comments_count: 4,
          has_description: true,
        },
      ];

      server.use(
        http.get(`${BASE_URL}/my/assignments/completed.json`, () => {
          return HttpResponse.json(mockAssignments);
        })
      );

      const result = await client.reports.myAssignmentsCompleted();
      expect(result).toHaveLength(1);
      expect(result[0]!.content).toBe("Completed assignment");
      expect(result[0]!.completed).toBe(true);
    });

    it("should surface forbidden errors", async () => {
      server.use(
        http.get(`${BASE_URL}/my/assignments/completed.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      await expect(client.reports.myAssignmentsCompleted()).rejects.toThrow(BasecampError);
    });
  });

  describe("myAssignmentsDue", () => {
    it("should send the scope query parameter and return due assignments", async () => {
      const mockAssignments = [
        {
          id: 20,
          content: "Due assignment",
          due_on: "2024-04-03",
          completed: false,
          type: "Todo",
          comments_count: 0,
          has_description: false,
        },
      ];

      server.use(
        http.get(`${BASE_URL}/my/assignments/due.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("scope")).toBe("due_today");
          return HttpResponse.json(mockAssignments);
        })
      );

      const result = await client.reports.myAssignmentsDue({ scope: "due_today" });
      expect(result).toHaveLength(1);
      expect(result[0]!.due_on).toBe("2024-04-03");
    });

    it("should surface rate limit errors", async () => {
      server.use(
        http.get(`${BASE_URL}/my/assignments/due.json`, () => {
          return HttpResponse.json({ error: "Rate limited" }, { status: 429 });
        })
      );

      await expect(client.reports.myAssignmentsDue({ scope: "due_today" })).rejects.toThrow(BasecampError);
    });
  });
});
