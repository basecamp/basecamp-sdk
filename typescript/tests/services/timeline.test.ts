/**
 * Tests for the TimelineService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleTimelineEntry = (id = 1) => ({
  id,
  action: "created",
  created_at: "2024-01-15T10:00:00Z",
  recording: { id: 200, title: "Some recording", type: "Todo" },
  creator: { id: 100, name: "Jane Doe" },
});

describe("TimelineService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("projectTimeline", () => {
    it("should return timeline entries for a project", async () => {
      const projectId = 100;

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/timeline.json`, () => {
          return HttpResponse.json([sampleTimelineEntry(1), sampleTimelineEntry(2)]);
        })
      );

      const entries = await client.timeline.projectTimeline(projectId);
      expect(entries).toHaveLength(2);
      expect(entries[0]!.id).toBe(1);
      expect(entries[1]!.id).toBe(2);
    });

    it("should return empty array when no timeline entries exist", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/100/timeline.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const entries = await client.timeline.projectTimeline(100);
      expect(entries).toHaveLength(0);
    });
  });
});
