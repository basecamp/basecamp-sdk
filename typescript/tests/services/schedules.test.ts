/**
 * Tests for the Schedules service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { SchedulesService } from "../../src/services/schedules.js";
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
        http.get(`${BASE_URL}/buckets/123/schedules/4001`, () => {
          return HttpResponse.json(schedule);
        })
      );

      const result = await service.get(123, 4001);

      expect(result.id).toBe(4001);
      expect(result.title).toBe("Schedule");
      expect(result.include_due_assignments).toBe(true);
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/schedules/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.get(123, 9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(123, 9999);
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
        http.get(`${BASE_URL}/buckets/123/schedules/4001/entries.json`, () => {
          return HttpResponse.json(entries);
        })
      );

      const result = await service.listEntries(123, 4001);

      expect(result).toHaveLength(2);
      expect(result[0].summary).toBe("Team Meeting");
      expect(result[1].summary).toBe("Project Review");
    });

    it("should return empty array when no entries", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/schedules/4001/entries.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.listEntries(123, 4001);

      expect(result).toEqual([]);
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
        http.get(`${BASE_URL}/buckets/123/schedule_entries/4101`, () => {
          return HttpResponse.json(entry);
        })
      );

      const result = await service.getEntry(123, 4101);

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
        http.post(`${BASE_URL}/buckets/123/schedules/4001/entries.json`, () => {
          return HttpResponse.json(newEntry);
        })
      );

      const result = await service.createEntry(123, 4001, {
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
        http.post(`${BASE_URL}/buckets/123/schedules/4001/entries.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 1, summary: "Test" });
        })
      );

      await service.createEntry(123, 4001, {
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

    it("should throw validation error when summary is missing", async () => {
      await expect(
        service.createEntry(123, 4001, {
          summary: "",
          startsAt: "2024-12-20T14:00:00Z",
          endsAt: "2024-12-20T15:00:00Z",
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.createEntry(123, 4001, {
          summary: "",
          startsAt: "2024-12-20T14:00:00Z",
          endsAt: "2024-12-20T15:00:00Z",
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("summary");
      }
    });

    it("should throw validation error when startsAt is missing", async () => {
      await expect(
        service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "",
          endsAt: "2024-12-20T15:00:00Z",
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "",
          endsAt: "2024-12-20T15:00:00Z",
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("starts_at");
      }
    });

    it("should throw validation error when endsAt is missing", async () => {
      await expect(
        service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "2024-12-20T14:00:00Z",
          endsAt: "",
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "2024-12-20T14:00:00Z",
          endsAt: "",
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("ends_at");
      }
    });

    it("should throw validation error for invalid startsAt format", async () => {
      await expect(
        service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "2024-12-20",
          endsAt: "2024-12-20T15:00:00Z",
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.createEntry(123, 4001, {
          summary: "Test",
          startsAt: "2024-12-20",
          endsAt: "2024-12-20T15:00:00Z",
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("RFC3339");
      }
    });
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
        http.put(`${BASE_URL}/buckets/123/schedule_entries/4101`, () => {
          return HttpResponse.json(updatedEntry);
        })
      );

      const result = await service.updateEntry(123, 4101, {
        summary: "Updated Meeting",
        startsAt: "2024-12-15T10:00:00Z",
        endsAt: "2024-12-15T11:00:00Z",
      });

      expect(result.summary).toBe("Updated Meeting");
    });

    it("should throw validation error for invalid date format", async () => {
      await expect(
        service.updateEntry(123, 4101, {
          startsAt: "invalid-date",
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.updateEntry(123, 4101, {
          startsAt: "invalid-date",
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("RFC3339");
      }
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
        http.get(`${BASE_URL}/buckets/123/schedule_entries/4101/occurrences/2024-12-22`, () => {
          return HttpResponse.json(entry);
        })
      );

      const result = await service.getEntryOccurrence(123, 4101, "2024-12-22");

      expect(result.starts_at).toBe("2024-12-22T09:00:00Z");
    });

    it("should throw validation error when date is missing", async () => {
      await expect(service.getEntryOccurrence(123, 4101, "")).rejects.toThrow(BasecampError);

      try {
        await service.getEntryOccurrence(123, 4101, "");
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("date");
      }
    });

    it("should throw validation error for invalid date format", async () => {
      await expect(
        service.getEntryOccurrence(123, 4101, "12-22-2024")
      ).rejects.toThrow(BasecampError);

      try {
        await service.getEntryOccurrence(123, 4101, "12-22-2024");
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("YYYY-MM-DD");
      }
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
        http.put(`${BASE_URL}/buckets/123/schedules/4001`, () => {
          return HttpResponse.json(schedule);
        })
      );

      const result = await service.updateSettings(123, 4001, {
        includeDueAssignments: false,
      });

      expect(result.include_due_assignments).toBe(false);
    });

    it("should send include_due_assignments in request body", async () => {
      let capturedBody: { include_due_assignments?: boolean } | null = null;

      server.use(
        http.put(`${BASE_URL}/buckets/123/schedules/4001`, async ({ request }) => {
          capturedBody = (await request.json()) as { include_due_assignments?: boolean };
          return HttpResponse.json({ id: 4001, title: "Schedule" });
        })
      );

      await service.updateSettings(123, 4001, { includeDueAssignments: true });

      expect(capturedBody?.include_due_assignments).toBe(true);
    });
  });

  describe("trashEntry", () => {
    it("should move a schedule entry to trash", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/4101/status/trashed.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.trashEntry(123, 4101)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent entry", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/trashed.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.trashEntry(123, 9999)).rejects.toThrow(BasecampError);
    });
  });
});
