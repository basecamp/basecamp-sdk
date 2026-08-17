/**
 * Tests for the ClientVisibilityService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import recordingFixture from "../../../spec/fixtures/recordings/get.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";

// Sourced from the shared, coverage-guarded fixture (spec/fixtures/manifest.yaml)
// so this helper cannot drift from the validated Recording shape; `id` is
// overridable per call.
const sampleRecording = (id = recordingFixture.id) => ({ ...recordingFixture, id });

describe("ClientVisibilityService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("setVisibility", () => {
    it("should set client visibility with visible_to_clients body", async () => {
      const recordingId = 42;

      server.use(
        http.put(`${BASE_URL}/recordings/${recordingId}/client_visibility.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.visible_to_clients).toBe(true);
          return HttpResponse.json(sampleRecording(recordingId));
        })
      );

      const recording = await client.clientVisibility.setVisibility(recordingId, {
        visibleToClients: true,
      });
      expect(recording.id).toBe(recordingId);
    });

    it("should set visibility to false", async () => {
      const recordingId = 42;

      server.use(
        http.put(`${BASE_URL}/recordings/${recordingId}/client_visibility.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.visible_to_clients).toBe(false);
          return HttpResponse.json({ ...sampleRecording(recordingId), visible_to_clients: false });
        })
      );

      const recording = await client.clientVisibility.setVisibility(recordingId, {
        visibleToClients: false,
      });
      expect(recording.id).toBe(recordingId);
    });
  });
});
