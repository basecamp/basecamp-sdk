/**
 * Tests for the HillChartsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import hillChartFixture from "../../../spec/fixtures/hill_charts/get.json" with { type: "json" };
import versionsFixture from "../../../spec/fixtures/hill_charts/versions.json" with { type: "json" };
import versionFixture from "../../../spec/fixtures/hill_charts/version.json" with { type: "json" };
import versionGoneFixture from "../../../spec/fixtures/hill_charts/version_todolist_gone.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleHillChart = () => ({
  enabled: true,
  stale: false,
  updated_at: "2026-03-11T06:38:12.167Z",
  app_update_url: `${BASE_URL}/buckets/100/todosets/42/hill/edit`,
  app_versions_url: `https://3.basecamp.com/12345/buckets/100/todosets/42/hill/versions`,
  dots: [
    {
      id: 1069479424,
      label: "Background and research",
      color: "blue",
      position: 0,
      url: `${BASE_URL}/buckets/100/todolists/1069479424.json`,
      app_url: "https://3.basecamp.com/12345/buckets/100/todolists/1069479424",
    },
  ],
});

const sampleHillChartSettings = () => ({
  enabled: true,
  stale: false,
  updated_at: "2026-03-11T07:00:00.000Z",
  app_update_url: `${BASE_URL}/buckets/100/todosets/42/hill/edit`,
  dots: [
    {
      id: 1069479424,
      label: "Background and research",
      color: "blue",
      position: 0,
      url: `${BASE_URL}/buckets/100/todolists/1069479424.json`,
      app_url: "https://3.basecamp.com/12345/buckets/100/todolists/1069479424",
    },
    {
      id: 1069479573,
      label: "Design mockups",
      color: "green",
      position: 42,
      url: `${BASE_URL}/buckets/100/todolists/1069479573.json`,
      app_url: "https://3.basecamp.com/12345/buckets/100/todolists/1069479573",
    },
  ],
});

describe("HillChartsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return the hill chart for a todoset", async () => {
      const todosetId = 42;

      server.use(
        http.get(`${BASE_URL}/todosets/${todosetId}/hill.json`, () => {
          return HttpResponse.json(sampleHillChart());
        })
      );

      const result = await client.hillCharts.get(todosetId);
      expect(result.enabled).toBe(true);
      expect(result.stale).toBe(false);
      // `dots` is optional in the generated schema. Assert presence first so a
      // response that omits it fails here, then read through the narrowed
      // local rather than repeating `?.` on every dot.
      expect(result.dots).toBeDefined();
      const dots = result.dots!;
      expect(dots).toHaveLength(1);
      expect(dots[0].label).toBe("Background and research");
      expect(result.app_versions_url).toBe(`https://3.basecamp.com/12345/buckets/100/todosets/42/hill/versions`);
    });

    it("carries versions_url when the chart has progress updates", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1069479829/hill.json`, () => HttpResponse.json(hillChartFixture))
      );

      const result = await client.hillCharts.get(1069479829);
      expect(result.versions_url).toBe(
        "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json"
      );
    });

    it("should throw not_found for missing todoset", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/999/hill.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.hillCharts.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("updateSettings", () => {
    it("should update hill chart settings", async () => {
      const todosetId = 42;
      let capturedBody: unknown;

      server.use(
        http.put(`${BASE_URL}/todosets/${todosetId}/hills/settings.json`, async ({ request }) => {
          capturedBody = await request.json();
          return HttpResponse.json(sampleHillChartSettings());
        })
      );

      const result = await client.hillCharts.updateSettings(todosetId, {
        tracked: [1069479573],
        untracked: [1069479511],
      });
      expect(capturedBody).toEqual({ tracked: [1069479573], untracked: [1069479511] });
      expect(result.enabled).toBe(true);
      // See the note in `get`: `dots` is optional, so assert presence first.
      expect(result.dots).toBeDefined();
      const dots = result.dots!;
      expect(dots).toHaveLength(2);
      expect(dots[1].label).toBe("Design mockups");
      expect(dots[1].position).toBe(42);
    });

    it("should throw not_found for missing todoset", async () => {
      server.use(
        http.put(`${BASE_URL}/todosets/999/hills/settings.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(
        client.hillCharts.updateSettings(999, { tracked: [1] })
      ).rejects.toThrow(BasecampError);
    });
  });

  describe("listVersions", () => {
    it("lists a hill chart's progress updates", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1069479829/hills/versions.json`, () =>
          HttpResponse.json(versionsFixture, { headers: { "X-Total-Count": "1" } })
        )
      );

      const result = await client.hillCharts.listVersions(1069479829);
      expect(result).toHaveLength(1);
      expect(result.meta.totalCount).toBe(1);
      const version = result[0];
      expect(version.id).toBe(1069480194);
      expect(version.type).toBe("Hill::Version");
      expect(version.description).toContain("Research is wrapping up.");
      expect(version.description_attachments).toEqual([]);
      expect(version.comments_url).toBe(
        "https://3.basecampapi.com/195539477/buckets/2085958505/recordings/1069480194/comments.json"
      );
      expect(version.dots).toHaveLength(1);
      expect(version.dots[0]).toMatchObject({ id: 1069479863, label: "Background and research", position: 40 });
    });

    it("follows the Link header to the next page", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1/hills/versions.json`, ({ request }) => {
          const page = new URL(request.url).searchParams.get("page");
          if (page === "2") return HttpResponse.json(versionsFixture);
          return HttpResponse.json(versionsFixture, {
            headers: { Link: `<${BASE_URL}/todosets/1/hills/versions.json?page=2>; rel="next"` },
          });
        })
      );

      const result = await client.hillCharts.listVersions(1);
      expect(result).toHaveLength(2);
    });

    it("should throw not_found for missing todoset", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/999/hills/versions.json`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      await expect(client.hillCharts.listVersions(999)).rejects.toMatchObject({ code: "not_found" });
    });
  });

  describe("getVersion", () => {
    it("gets one progress update", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1069479829/hills/versions/1069480194`, () => HttpResponse.json(versionFixture))
      );

      const version = await client.hillCharts.getVersion(1069479829, 1069480194);
      expect(version.id).toBe(1069480194);
      expect(version.creator.name).toBe("Victor Cooper");
      expect(version.parent.type).toBe("Todoset");
      expect(version.dots[0].position).toBe(40);
    });

    it("reads a dot whose todolist is gone", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1069479829/hills/versions/1069480194`, () => HttpResponse.json(versionGoneFixture))
      );

      const version = await client.hillCharts.getVersion(1069479829, 1069480194);
      expect(version.dots).toHaveLength(2);
      const gone = version.dots[1];
      expect(gone.id).toBeNull();
      expect(gone.url).toBeUndefined();
      expect(gone.app_url).toBeUndefined();
      expect(gone.label).toBe("Old research list");
    });

    it("should throw not_found for a missing progress update", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/1/hills/versions/999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      await expect(client.hillCharts.getVersion(1, 999)).rejects.toMatchObject({ code: "not_found" });
    });
  });
});
