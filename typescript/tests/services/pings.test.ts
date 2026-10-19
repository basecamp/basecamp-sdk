/**
 * Tests for the PingsService (generated from OpenAPI spec)
 *
 * A Ping is a private conversation with its own Campfire. Its id is
 * its bucket's id, and its Campfire is named by `room_url`.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import pingFixture from "../../../spec/fixtures/pings/get.json" with { type: "json" };
import pingListFixture from "../../../spec/fixtures/pings/list.json" with { type: "json" };

// Sourced from the shared, coverage-guarded fixtures (spec/fixtures/manifest.yaml)
// so the wire shape stays tied to the spec; the fixtures' ids live on a
// different account, so the handlers match on the test account's own paths and
// the assertions read ids and URLs back from the fixture.
const BASE_URL = "https://3.basecampapi.com/12345";

describe("PingsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list the caller's Pings from the shared fixture", async () => {
      server.use(
        http.get(`${BASE_URL}/circles.json`, () => HttpResponse.json(pingListFixture))
      );

      const pings = await client.pings.list();
      expect(pings).toHaveLength(pingListFixture.length);
      expect(pings[0]!.id).toBe(pingListFixture[0]!.id);
      expect(pings[0]!.room_url).toBe(pingListFixture[0]!.room_url);
      expect(pings[0]!.people.length).toBeGreaterThan(0);
    });

    it("should return an empty list for a 204", async () => {
      server.use(
        http.get(`${BASE_URL}/circles.json`, () => new HttpResponse(null, { status: 204 }))
      );

      await expect(client.pings.list()).resolves.toEqual([]);
    });

    it("should throw forbidden when Pings are disabled", async () => {
      server.use(
        http.get(`${BASE_URL}/circles.json`, () =>
          HttpResponse.json({ error: "Pings have been disabled" }, { status: 403 })
        )
      );

      const err = await client.pings.list().catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("forbidden");
    });
  });

  describe("get", () => {
    it("should return a Ping by its bucket id", async () => {
      server.use(
        http.get(`${BASE_URL}/circles/${pingFixture.id}`, () => HttpResponse.json(pingFixture))
      );

      const ping = await client.pings.get(pingFixture.id);
      expect(ping.id).toBe(pingFixture.id);
      expect(ping.title).toBe("Launch crew");
      expect(ping.named).toBe(true);
      expect(ping.room_url).toBe(pingFixture.room_url);
      expect(ping.links.lines).toBe(pingFixture.links.lines);
    });

    it("should throw not_found for a bucket that is not a Ping", async () => {
      server.use(
        http.get(`${BASE_URL}/circles/999`, () =>
          HttpResponse.json({ error: "Record not found" }, { status: 404 })
        )
      );

      const err = await client.pings.get(999).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("not_found");
    });

    it("should throw forbidden when Pings are disabled", async () => {
      server.use(
        http.get(`${BASE_URL}/circles/999`, () =>
          HttpResponse.json({ error: "Pings have been disabled" }, { status: 403 })
        )
      );

      const err = await client.pings.get(999).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("forbidden");
    });
  });

  describe("create", () => {
    it("should start a Ping and omit new_group when it is not set", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(`${BASE_URL}/circles.json`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(pingFixture, { status: 201 });
        })
      );

      const ping = await client.pings.create({ userIds: [1049715915] });
      expect(ping.id).toBe(pingFixture.id);
      expect(received).toEqual({ user_ids: [1049715915] });
    });

    it("should send new_group when it is set", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(`${BASE_URL}/circles.json`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(pingFixture, { status: 201 });
        })
      );

      await client.pings.create({ userIds: [1049715915, 1049715916], newGroup: true });
      expect(received).toEqual({ user_ids: [1049715915, 1049715916], new_group: true });
    });

    it("should reject a missing userIds before any request", async () => {
      let reached = false;
      server.use(
        http.post(`${BASE_URL}/circles.json`, () => {
          reached = true;
          return HttpResponse.json(pingFixture, { status: 201 });
        })
      );

      const err = await client.pings
        .create({} as { userIds: number[] })
        .catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("validation");
      expect(reached).toBe(false);
    });

    it("should throw forbidden when Pings are disabled", async () => {
      server.use(
        http.post(`${BASE_URL}/circles.json`, () =>
          HttpResponse.json({ error: "Pings have been disabled" }, { status: 403 })
        )
      );

      const err = await client.pings.create({ userIds: [1] }).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("forbidden");
    });
  });
});
