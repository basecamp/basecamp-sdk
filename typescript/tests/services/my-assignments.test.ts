import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("MyAssignmentsService — Up Next writes", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("myAssignments", () => {
    it("decodes assignees with the full three-key person_minimal projection (#659)", async () => {
      server.use(
        http.get(`${BASE_URL}/my/assignments.json`, () =>
          HttpResponse.json({
            priorities: [
              {
                id: 1,
                content: "Priority task",
                // bc3 renders id, name and avatar_url unconditionally.
                assignees: [
                  {
                    id: 1049715914,
                    name: "Victor Cooper",
                    avatar_url: "https://example.com/avatar",
                  },
                ],
              },
            ],
            non_priorities: [],
          })
        )
      );

      const result = await client.myAssignments.myAssignments();
      const assignee = result.priorities![0]!.assignees![0]!;
      expect(assignee.id).toBe(1049715914);
      expect(assignee.name).toBe("Victor Cooper");
      expect(assignee.avatar_url).toBe("https://example.com/avatar");
    });
  });

  describe("prioritizeAssignment", () => {
    it("POSTs the recording id and accepts 204", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priorities.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body).toEqual({ id: 1069479801 });
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.myAssignments.prioritizeAssignment({ id: 1069479801 })).resolves.toBeUndefined();
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priorities.json`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      const error = await client.myAssignments.prioritizeAssignment({ id: 999 }).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("deprioritizeAssignment", () => {
    it("DELETEs the exact recording id and accepts 204", async () => {
      let called = false;
      server.use(
        http.delete(`${BASE_URL}/my/priorities/1069479801`, () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.myAssignments.deprioritizeAssignment(1069479801);
      expect(called).toBe(true);
    });

    it("surfaces 403 as BasecampError", async () => {
      server.use(
        http.delete(`${BASE_URL}/my/priorities/1069479801`, () =>
          HttpResponse.json({ error: "Forbidden" }, { status: 403 })
        )
      );

      const error = await client.myAssignments.deprioritizeAssignment(1069479801).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(403);
    });
  });

  describe("reorderUpNext", () => {
    it("POSTs source_id and position and accepts 204", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priority_moves.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body).toEqual({ source_id: 1069479801, position: 1 });
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.myAssignments.reorderUpNext({ sourceId: 1069479801, position: 1 });
    });

    it("surfaces the typed 400 (non-integer position) as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priority_moves.json`, () =>
          HttpResponse.json({ error: "Position must be an integer." }, { status: 400 })
        )
      );

      const error = await client.myAssignments
        .reorderUpNext({ sourceId: 1069479801, position: 2 })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(400);
    });

    it("surfaces the typed 422 (out of range) as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priority_moves.json`, () =>
          HttpResponse.json({ error: "Position must be between 1 and 3." }, { status: 422 })
        )
      );

      const error = await client.myAssignments
        .reorderUpNext({ sourceId: 1069479801, position: 99 })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });

    it("surfaces the bare bodyless 404 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/my/priority_moves.json`, () => new HttpResponse(null, { status: 404 }))
      );

      const error = await client.myAssignments
        .reorderUpNext({ sourceId: 999, position: 1 })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });
});
