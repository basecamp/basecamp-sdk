/**
 * Tests for the Schedules service (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No client-side validation (API validates)
 * - No domain-specific trashEntry() (use recordings.trash())
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { SchedulesService } from "../../src/generated/services/schedules.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

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
        })
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
        })
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
        { id: 4101, summary: "Team Meeting", starts_at: "2024-12-15T09:00:00Z" },
        { id: 4102, summary: "Project Review", starts_at: "2024-12-16T14:00:00Z" },
      ];

      server.use(
        http.get(`${BASE_URL}/schedules/4001/entries.json`, () => {
          return HttpResponse.json(entries);
        })
      );

      const result = await service.listEntries(4001);

      expect(result).toHaveLength(2);
      expect(result[0].summary).toBe("Team Meeting");
      expect(result[1].summary).toBe("Project Review");
    });

    it("should return empty array when no entries", async () => {
      server.use(
        http.get(`${BASE_URL}/schedules/4001/entries.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.listEntries(4001);

      expect(result).toHaveLength(0);
    });
  });

  describe("getEntry", () => {
    it("should return a schedule entry by ID", async () => {
      const entry = {
        id: 4101,
        summary: "Team Meeting",
        description: "<p>Weekly sync</p>",
        starts_at: "2024-12-15T09:00:00Z",
        ends_at: "2024-12-15T10:00:00Z",
        all_day: false,
      };

      server.use(
        http.get(`${BASE_URL}/schedule_entries/4101`, () => {
          return HttpResponse.json(entry);
        })
      );

      const result = await service.getEntry(4101);

      expect(result.id).toBe(4101);
      expect(result.summary).toBe("Team Meeting");
      expect(result.all_day).toBe(false);
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
        })
      );

      const result = await service.createEntry(4001, {
        summary: "New Event",
        startsAt: "2024-12-20T14:00:00Z",
        endsAt: "2024-12-20T15:00:00Z",
      });

      expect(result.id).toBe(4201);
      expect(result.summary).toBe("New Event");
    });

    it("should send all fields in request body", async () => {
      let capturedBody: Record<string, unknown> | null = null;

      server.use(
        http.post(`${BASE_URL}/schedules/4001/entries.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 1, summary: "Test" });
        })
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

    // Note: Client-side validation removed - generated services let API validate
  });

  describe("updateEntry", () => {
    it("should update an existing schedule entry", async () => {
      const updatedEntry = {
        id: 4101,
        summary: "Updated Meeting",
        starts_at: "2024-12-15T10:00:00Z",
        ends_at: "2024-12-15T11:00:00Z",
      };

      server.use(
        http.put(`${BASE_URL}/schedule_entries/4101`, () => {
          return HttpResponse.json(updatedEntry);
        })
      );

      const result = await service.updateEntry(4101, {
        summary: "Updated Meeting",
        startsAt: "2024-12-15T10:00:00Z",
        endsAt: "2024-12-15T11:00:00Z",
      });

      expect(result.summary).toBe("Updated Meeting");
    });

    // Note: Client-side validation removed - generated services let API validate
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
        http.get(`${BASE_URL}/schedule_entries/4101/occurrences/2024-12-22`, () => {
          return HttpResponse.json(entry);
        })
      );

      const result = await service.getEntryOccurrence(4101, "2024-12-22");

      expect(result.starts_at).toBe("2024-12-22T09:00:00Z");
    });

    // Note: Client-side validation removed - generated services let API validate
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
        })
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
          capturedBody = (await request.json()) as { include_due_assignments?: boolean };
          return HttpResponse.json({ id: 4001, title: "Schedule" });
        })
      );

      await service.updateSettings(4001, { includeDueAssignments: true });

      expect(capturedBody?.include_due_assignments).toBe(true);
    });
  });

  // Note: trashEntry() is on RecordingsService, not SchedulesService (spec-conformant)
  // Use client.recordings.trash(entryId) instead
});
