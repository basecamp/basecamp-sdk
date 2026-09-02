import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("BubbleUpsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("createBubbleUp", () => {
    it("schedules the bubble-up, sending 'at' on the wire (204)", async () => {
      let captured: unknown;
      server.use(
        http.post(`${BASE_URL}/recordings/900/bubble_up.json`, async ({ request }) => {
          captured = await request.json();
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.bubbleUps.createBubbleUp(900, { at: "2026-09-10T09:00:00Z" });
      expect(captured).toEqual({ at: "2026-09-10T09:00:00Z" });
    });

    it("omits 'at' from the wire when not supplied (204)", async () => {
      let captured: Record<string, unknown> | undefined;
      server.use(
        http.post(`${BASE_URL}/recordings/900/bubble_up.json`, async ({ request }) => {
          captured = (await request.json()) as Record<string, unknown>;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.bubbleUps.createBubbleUp(900, {});
      expect(captured).toBeDefined();
      expect(captured).not.toHaveProperty("at");
    });

    it("surfaces 403 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/900/bubble_up.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      const error = await client.bubbleUps.createBubbleUp(900, {}).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(403);
    });
  });

  describe("deleteBubbleUp", () => {
    it("pops the bubble-up (204)", async () => {
      let called = false;
      server.use(
        http.delete(`${BASE_URL}/recordings/900/bubble_up.json`, () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.bubbleUps.deleteBubbleUp(900);
      expect(called).toBe(true);
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.delete(`${BASE_URL}/recordings/999/bubble_up.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.bubbleUps.deleteBubbleUp(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });
});
