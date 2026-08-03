/**
 * Tests for the TimesheetsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import { ListResult } from "../../src/pagination.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleEntry = (id = 1) => ({
  id,
  status: "active",
  visible_to_clients: false,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
  title: "2.5 hours",
  inherits_status: true,
  type: "Timesheet::Entry",
  url: `${BASE_URL}/timesheet_entries/${id}`,
  app_url: `https://3.basecamp.com/12345/timesheet_entries/${id}`,
  parent: { id: 10, title: "Write the docs", type: "Todo", url: "u", app_url: "a" },
  bucket: { id: 2085958499, name: "The Leto Laptop", type: "Project" },
  creator: { id: 1, name: "Victor Cooper" },
  date: "2024-01-15",
  hours: "2.5",
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

  describe("get", () => {
    it("should fetch a single timesheet entry", async () => {
      server.use(
        http.get(`${BASE_URL}/timesheet_entries/4001`, () =>
          HttpResponse.json(sampleEntry(4001))
        )
      );

      const entry = await client.timesheets.get(4001);
      expect(entry.id).toBe(4001);
      expect(entry.hours).toBe("2.5");
    });
  });

  describe("forProject", () => {
    it("should list a project's timesheet entries as a ListResult", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/2085958499/timesheet.json`, () =>
          HttpResponse.json([sampleEntry(4001), sampleEntry(4002)], {
            headers: { "X-Total-Count": "2" },
          })
        )
      );

      const result = await client.timesheets.forProject(2085958499);
      expect(result).toBeInstanceOf(ListResult);
      expect(result).toHaveLength(2);
      expect(result.meta.totalCount).toBe(2);
    });
  });

  describe("destroy", () => {
    it("should DELETE the flat entry URL and resolve on 204", async () => {
      let capturedMethod: string | undefined;
      let capturedPath: string | undefined;

      server.use(
        http.delete(`${BASE_URL}/timesheet_entries/4001`, ({ request }) => {
          capturedMethod = request.method;
          capturedPath = new URL(request.url).pathname;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.timesheets.destroy(4001)).resolves.toBeUndefined();
      expect(capturedMethod).toBe("DELETE");
      // Flat/unscoped: no /buckets/{bucketId} prefix, only the account segment.
      expect(capturedPath).toBe("/12345/timesheet_entries/4001");
    });

    it("should surface 403 as a forbidden BasecampError", async () => {
      server.use(
        // bc3's Timesheets::EntriesController#destroy does `head :forbidden`
        // when the caller may not archive or trash the entry — an empty body.
        http.delete(`${BASE_URL}/timesheet_entries/4001`, () =>
          new HttpResponse(null, { status: 403, statusText: "Forbidden" })
        )
      );

      const error = await client.timesheets.destroy(4001).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("forbidden");
      expect((error as BasecampError).httpStatus).toBe(403);
      expect((error as BasecampError).retryable).toBe(false);
    });

    it("should surface 404 as a not_found BasecampError", async () => {
      server.use(
        http.delete(`${BASE_URL}/timesheet_entries/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      const error = await client.timesheets.destroy(9999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("not_found");
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });
});
