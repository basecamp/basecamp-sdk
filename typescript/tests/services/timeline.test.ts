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

    it("should decode additive activity-timeline fields: avatars_sample, data, and heterogeneous attachments", async () => {
      const projectId = 100;

      const fixture = [
        {
          id: 1,
          created_at: "2024-03-15T10:30:00Z",
          kind: "chat_transcript_rollup",
          avatars_sample: [
            "https://3.basecampapi.com/1/people/aaa/avatar",
            "https://3.basecampapi.com/1/people/bbb/avatar",
          ],
        },
        {
          id: 2,
          created_at: "2024-03-15T10:31:00Z",
          kind: "schedule_entry_created",
          avatars_sample: [],
          data: {
            all_day: true,
            starts_at: "2025-10-30",
            ends_at: "2025-10-30",
          },
        },
        {
          id: 3,
          created_at: "2024-03-15T10:32:00Z",
          kind: "upload_created",
          avatars_sample: [],
          attachments: [
            {
              id: 900,
              type: "Upload",
              status: "active",
              visible_to_clients: false,
              title: "Diagram",
              filename: "diagram.png",
              content_type: "image/png",
              byte_size: 20480,
              width: 1024.0,
              height: 768.0,
              url: "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
              app_url: "https://3.basecamp.com/1/buckets/2/uploads/900",
              download_url: "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
              app_download_url: "https://3.basecamp.com/1/buckets/2/uploads/900/download",
            },
          ],
        },
        {
          id: 4,
          created_at: "2024-03-15T10:33:00Z",
          kind: "comment_created",
          avatars_sample: [],
          attachments: [
            {
              id: 500,
              attachable_sgid: "sgid-attachable-500",
              sgid: "sgid-500",
              status_url: "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
              caption: "See attached",
              filename: "notes.pdf",
              content_type: "application/pdf",
              byte_size: 4096,
              key: "blobkey500",
              width: null,
              height: null,
              previewable: true,
              download_url: "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
              preview_url: "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
              thumbnail_url: "https://3.basecampapi.com/1/blobs/blobkey500/previews/card",
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/timeline.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const events = await client.timeline.projectTimeline(projectId);
      expect(events).toHaveLength(4);

      // avatars_sample is a non-empty array of avatar URLs
      const avatars = (events[0] as any).avatars_sample;
      expect(avatars).toHaveLength(2);
      expect(avatars[0]).toBe("https://3.basecampapi.com/1/people/aaa/avatar");

      // data payload: all-day schedule entry with date-only start/end
      const data = (events[1] as any).data;
      expect(data.all_day).toBe(true);
      expect(data.starts_at).toBe("2025-10-30");
      expect(data.ends_at).toBe("2025-10-30");

      // attachments variant 1: full Upload recording
      const upload = (events[2] as any).attachments[0];
      expect(upload.type).toBe("Upload");
      expect(upload.filename).toBe("diagram.png");
      expect(upload.app_download_url).toBe("https://3.basecamp.com/1/buckets/2/uploads/900/download");
      expect(upload.width).toBe(1024);

      // attachments variant 2: rich-text attachment/blob partial
      const blob = (events[3] as any).attachments[0];
      expect(blob.attachable_sgid).toBe("sgid-attachable-500");
      expect(blob.caption).toBe("See attached");
      expect(blob.key).toBe("blobkey500");
      expect(blob.previewable).toBe(true);
      expect(blob.width).toBeNull();
    });
  });
});
