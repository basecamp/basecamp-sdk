/**
 * Tests for the Events service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { EventsService } from "../../src/services/events.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("EventsService", () => {
  let service: EventsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.events;
  });

  describe("list", () => {
    it("should return events for a recording", async () => {
      const events = [
        {
          id: 9001,
          recording_id: 5001,
          action: "created",
          created_at: "2024-12-10T10:00:00Z",
          creator: { id: 1001, name: "Alice" },
        },
        {
          id: 9002,
          recording_id: 5001,
          action: "updated",
          created_at: "2024-12-11T14:30:00Z",
          creator: { id: 1002, name: "Bob" },
        },
        {
          id: 9003,
          recording_id: 5001,
          action: "assignment_changed",
          created_at: "2024-12-12T09:00:00Z",
          creator: { id: 1001, name: "Alice" },
          details: {
            added_person_ids: [1003, 1004],
            removed_person_ids: [],
          },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/5001/events.json`, () => {
          return HttpResponse.json({ events });
        })
      );

      const result = await service.list(123, 5001);

      expect(result).toHaveLength(3);
      expect(result[0].action).toBe("created");
      expect(result[1].action).toBe("updated");
      expect(result[2].action).toBe("assignment_changed");
      expect(result[2].details?.added_person_ids).toEqual([1003, 1004]);
    });

    it("should return empty array when no events", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/5001/events.json`, () => {
          return HttpResponse.json({ events: [] });
        })
      );

      const result = await service.list(123, 5001);

      expect(result).toEqual([]);
    });

    it("should include creator information", async () => {
      const events = [
        {
          id: 9001,
          recording_id: 5001,
          action: "created",
          created_at: "2024-12-10T10:00:00Z",
          creator: {
            id: 1001,
            name: "Alice Johnson",
            email_address: "alice@example.com",
            avatar_url: "https://example.com/avatar.png",
          },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/5001/events.json`, () => {
          return HttpResponse.json({ events });
        })
      );

      const result = await service.list(123, 5001);

      expect(result[0].creator?.name).toBe("Alice Johnson");
      expect(result[0].creator?.email_address).toBe("alice@example.com");
    });

    it("should include event details when present", async () => {
      const events = [
        {
          id: 9003,
          recording_id: 5001,
          action: "completed",
          created_at: "2024-12-12T09:00:00Z",
          creator: { id: 1001, name: "Alice" },
          details: {
            notified_recipient_ids: [1002, 1003],
          },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/5001/events.json`, () => {
          return HttpResponse.json({ events });
        })
      );

      const result = await service.list(123, 5001);

      expect(result[0].details?.notified_recipient_ids).toEqual([1002, 1003]);
    });

    it("should handle events with removed person IDs", async () => {
      const events = [
        {
          id: 9004,
          recording_id: 5001,
          action: "assignment_changed",
          created_at: "2024-12-13T11:00:00Z",
          creator: { id: 1001, name: "Alice" },
          details: {
            added_person_ids: [],
            removed_person_ids: [1003],
          },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/5001/events.json`, () => {
          return HttpResponse.json({ events });
        })
      );

      const result = await service.list(123, 5001);

      expect(result[0].details?.removed_person_ids).toEqual([1003]);
    });
  });
});
