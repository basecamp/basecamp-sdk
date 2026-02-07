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
      const projectId = 67890;
      const mockEntries = [
        {
          id: 1,
          date: "2024-01-15",
          hours: "2.0",
          bucket: { id: projectId, name: "Project X" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/timesheet.json`, () => {
          return HttpResponse.json(mockEntries);
        })
      );

      const entries = await client.timesheets.forProject(projectId);
      expect(entries).toHaveLength(1);
      expect(entries[0]!.hours).toBe("2.0");
    });

    it("should support filtering options", async () => {
      const projectId = 67890;

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/timesheet.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("from")).toBe("2024-02-01");
          expect(url.searchParams.get("person_id")).toBe("999");
          return HttpResponse.json([]);
        })
      );

      const entries = await client.timesheets.forProject(projectId, {
        from: "2024-02-01",
        personId: 999,
      });
      expect(entries).toHaveLength(0);
    });
  });

  describe("forRecording", () => {
    it("should return timesheet entries for a specific recording", async () => {
      const projectId = 67890;
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
          `${BASE_URL}/buckets/${projectId}/recordings/${recordingId}/timesheet.json`,
          () => {
            return HttpResponse.json(mockEntries);
          }
        )
      );

      const entries = await client.timesheets.forRecording(projectId, recordingId);
      expect(entries).toHaveLength(1);
      expect(entries[0]!.hours).toBe("1.5");
    });
  });
});
