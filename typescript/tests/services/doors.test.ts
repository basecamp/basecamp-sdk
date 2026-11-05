/**
 * Tests for the Doors service (external links), generated from the OpenAPI spec.
 *
 * Fixtures are bc3's documented external-link examples, shared under
 * spec/fixtures/doors/ and guarded by the fixture-completeness manifest. MSW
 * answers only the exact URL, so a wrong path fails here as it would 404 live.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { DoorsService } from "../../src/generated/services/doors.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient, normalizeUrlPath } from "../../src/client.js";
import doorFixture from "../../../spec/fixtures/doors/get.json" with { type: "json" };
import doorsFixture from "../../../spec/fixtures/doors/list.json" with { type: "json" };
import createRequest from "../../../spec/fixtures/doors/create-request.json" with { type: "json" };
import updateRequest from "../../../spec/fixtures/doors/update-request.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";

describe("DoorsService", () => {
  let service: DoorsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
    service = client.doors;
  });

  describe("list", () => {
    it("lists a project's external links from the bucket-scoped dock path", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/2085958504/dock/doors.json`, () => HttpResponse.json(doorsFixture)),
      );

      const result = await service.list(2085958504);

      expect(result).toHaveLength(1);
      expect(result[0]!.type).toBe("Door");
      expect(result[0]!.position).toBe(8);
    });

    it("throws forbidden for a 403 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/dock/doors.json`, () =>
          HttpResponse.json({ error: "Forbidden" }, { status: 403 }),
        ),
      );

      await expect(service.list(1)).rejects.toMatchObject({ code: "forbidden" });
    });
  });

  describe("get", () => {
    it("returns the full door shape, with url as the outside address", async () => {
      server.use(http.get(`${BASE_URL}/dock/doors/1069480290`, () => HttpResponse.json(doorFixture)));

      const door = await service.get(1069480290);

      expect(door.id).toBe(1069480290);
      expect(door.url).toBe("https://www.figma.com/file/abc123/Design-system");
      expect(door.service.code).toBe("figma");
      expect(door.description).toBe("<div>Shared Figma workspace</div>");
      expect(door.visible_to_clients).toBe(false);
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/dock/doors/9999`, () => HttpResponse.json({ error: "Not found" }, { status: 404 })),
      );

      await expect(service.get(9999)).rejects.toThrow(BasecampError);
      await expect(service.get(9999)).rejects.toMatchObject({ code: "not_found" });
    });
  });

  describe("create", () => {
    it("wraps the fields under door and puts visible_to_clients beside it", async () => {
      let received: unknown;
      server.use(
        http.post(`${BASE_URL}/buckets/2085958504/dock/doors.json`, async ({ request }) => {
          received = await request.json();
          return HttpResponse.json(doorFixture, { status: 201 });
        }),
      );

      const door = await service.create(2085958504, {
        door: createRequest.door,
        visibleToClients: createRequest.visible_to_clients,
      });

      expect(door.id).toBe(1069480290);
      expect(received).toEqual(createRequest);
    });

    it("omits visible_to_clients when not given, so the link stays private", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(`${BASE_URL}/buckets/1/dock/doors.json`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(doorFixture, { status: 201 });
        }),
      );

      await service.create(1, { door: { service: "other", url: "https://example.com" } });

      expect(received).toEqual({ door: { service: "other", url: "https://example.com" } });
    });

    it("surfaces a field-keyed 422 as a validation error", async () => {
      server.use(
        http.post(`${BASE_URL}/buckets/1/dock/doors.json`, () =>
          HttpResponse.json({ errors: { url: ["is invalid"] } }, { status: 422 }),
        ),
      );

      await expect(
        service.create(1, { door: { service: "other", url: "javascript:alert(1)" } }),
      ).rejects.toMatchObject({ code: "validation" });
    });
  });

  describe("update", () => {
    it("puts the partial change to the flat door path", async () => {
      let received: unknown;
      server.use(
        http.put(`${BASE_URL}/dock/doors/1069480290`, async ({ request }) => {
          received = await request.json();
          return HttpResponse.json(doorFixture);
        }),
      );

      await service.update(1069480290, {
        door: updateRequest.door,
        visibleToClients: updateRequest.visible_to_clients,
      });

      expect(received).toEqual(updateRequest);
    });

    it("can change only visible_to_clients", async () => {
      let received: unknown;
      server.use(
        http.put(`${BASE_URL}/dock/doors/1069480290`, async ({ request }) => {
          received = await request.json();
          return HttpResponse.json(doorFixture);
        }),
      );

      await service.update(1069480290, { visibleToClients: true });

      expect(received).toEqual({ visible_to_clients: true });
    });

    it("throws forbidden for a 403 response", async () => {
      server.use(
        http.put(`${BASE_URL}/dock/doors/1`, () => HttpResponse.json({ error: "Forbidden" }, { status: 403 })),
      );

      await expect(service.update(1, { door: { title: "x" } })).rejects.toMatchObject({ code: "forbidden" });
    });
  });

  describe("delete", () => {
    it("deletes the door and resolves on 204", async () => {
      let method: string | undefined;
      server.use(
        http.delete(`${BASE_URL}/dock/doors/1069480290`, ({ request }) => {
          method = request.method;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(service.delete(1069480290)).resolves.toBeUndefined();
      expect(method).toBe("DELETE");
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.delete(`${BASE_URL}/dock/doors/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 }),
        ),
      );

      await expect(service.delete(9999)).rejects.toMatchObject({ code: "not_found" });
    });
  });

  it("normalizes door URLs to the keys PATH_TO_OPERATION uses", () => {
    expect(normalizeUrlPath(`${BASE_URL}/dock/doors/1069480290`)).toBe("/{accountId}/dock/doors/{doorId}");
    expect(normalizeUrlPath(`${BASE_URL}/buckets/2085958504/dock/doors.json`)).toBe(
      "/{accountId}/buckets/{bucketId}/dock/doors.json",
    );
  });
});
