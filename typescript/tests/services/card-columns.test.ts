/**
 * Tests for the CardColumnsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleColumn = (id = 1) => ({
  id,
  title: "In Progress",
  description: "<p>Work in progress</p>",
  color: "blue",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("CardColumnsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single column", async () => {
      const columnId = 42;

      server.use(
        http.get(`${BASE_URL}/card_tables/columns/${columnId}`, () => {
          return HttpResponse.json(sampleColumn(columnId));
        })
      );

      const column = await client.cardColumns.get(columnId);
      expect(column.id).toBe(columnId);
      expect(column.title).toBe("In Progress");
    });
  });

  describe("create", () => {
    it("should create a column with title", async () => {
      const cardTableId = 200;

      server.use(
        http.post(`${BASE_URL}/card_tables/${cardTableId}/columns.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("New Column");
          return HttpResponse.json(sampleColumn(99), { status: 201 });
        })
      );

      const column = await client.cardColumns.create(cardTableId, {
        title: "New Column",
      });
      expect(column.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a column", async () => {
      const columnId = 42;

      server.use(
        http.put(`${BASE_URL}/card_tables/columns/${columnId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("Updated Column");
          return HttpResponse.json(sampleColumn(columnId));
        })
      );

      const column = await client.cardColumns.update(columnId, {
        title: "Updated Column",
      });
      expect(column.id).toBe(columnId);
    });
  });

  describe("move", () => {
    it("should move a column within a card table", async () => {
      const cardTableId = 200;

      server.use(
        http.post(`${BASE_URL}/card_tables/${cardTableId}/moves.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.source_id).toBe(10);
          expect(body.target_id).toBe(20);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.cardColumns.move(cardTableId, { sourceId: 10, targetId: 20 })
      ).resolves.toBeUndefined();
    });
  });

  describe("setColor", () => {
    it("should set the color of a column", async () => {
      const columnId = 42;

      server.use(
        http.put(`${BASE_URL}/card_tables/columns/${columnId}/color.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.color).toBe("green");
          return HttpResponse.json(sampleColumn(columnId));
        })
      );

      const column = await client.cardColumns.setColor(columnId, {
        color: "green",
      });
      expect(column.id).toBe(columnId);
    });
  });

  describe("enableOnHold", () => {
    it("should enable on-hold for a column", async () => {
      const columnId = 42;

      server.use(
        http.post(`${BASE_URL}/card_tables/columns/${columnId}/on_hold.json`, () => {
          return HttpResponse.json(sampleColumn(columnId));
        })
      );

      const column = await client.cardColumns.enableOnHold(columnId);
      expect(column.id).toBe(columnId);
    });
  });

  describe("disableOnHold", () => {
    it("should disable on-hold for a column", async () => {
      const columnId = 42;

      server.use(
        http.delete(`${BASE_URL}/card_tables/columns/${columnId}/on_hold.json`, () => {
          return HttpResponse.json(sampleColumn(columnId));
        })
      );

      const column = await client.cardColumns.disableOnHold(columnId);
      expect(column.id).toBe(columnId);
    });
  });
});
