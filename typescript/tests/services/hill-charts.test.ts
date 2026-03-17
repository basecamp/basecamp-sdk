/**
 * Tests for the HillChartsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleHillChart = () => ({
  enabled: true,
  stale: false,
  updated_at: "2026-03-11T06:38:12.167Z",
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
      expect(result.dots).toHaveLength(1);
      expect(result.dots[0].label).toBe("Background and research");
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
      expect(result.dots).toHaveLength(2);
      expect(result.dots[1].label).toBe("Design mockups");
      expect(result.dots[1].position).toBe(42);
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
});
