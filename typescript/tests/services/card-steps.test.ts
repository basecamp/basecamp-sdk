/**
 * Tests for the CardStepsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleStep = (id = 1) => ({
  id,
  title: "Review code",
  completed: false,
  due_on: "2024-03-01",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("CardStepsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("create", () => {
    it("should create a step with title", async () => {
      const cardId = 42;

      server.use(
        http.post(`${BASE_URL}/card_tables/cards/${cardId}/steps.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("New step");
          return HttpResponse.json(sampleStep(99), { status: 201 });
        })
      );

      const step = await client.cardSteps.create(cardId, {
        title: "New step",
      });
      expect(step.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a step", async () => {
      const stepId = 42;

      server.use(
        http.put(`${BASE_URL}/card_tables/steps/${stepId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("Updated step");
          return HttpResponse.json(sampleStep(stepId));
        })
      );

      const step = await client.cardSteps.update(stepId, {
        title: "Updated step",
      });
      expect(step.id).toBe(stepId);
    });
  });

  describe("reposition", () => {
    it("should reposition a step within a card", async () => {
      const cardId = 42;

      server.use(
        http.post(`${BASE_URL}/card_tables/cards/${cardId}/positions.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.source_id).toBe(10);
          expect(body.position).toBe(2);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.cardSteps.reposition(cardId, { sourceId: 10, position: 2 })
      ).resolves.toBeUndefined();
    });
  });

  describe("setCompletion", () => {
    it("should set step completion status", async () => {
      const stepId = 42;

      server.use(
        http.put(`${BASE_URL}/card_tables/steps/${stepId}/completions.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.completion).toBe("on");
          return HttpResponse.json(sampleStep(stepId));
        })
      );

      const step = await client.cardSteps.setCompletion(stepId, {
        completion: "on",
      });
      expect(step.id).toBe(stepId);
    });
  });
});
