/**
 * Tests for the TodolistGroupsService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No get() method (not in API spec)
 * - No update() method (not in API spec)
 * - No domain-specific trash() (use recordings.trash())
 * - No client-side validation (API validates)
 * - reposition() takes a request object, not bare number
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("TodolistGroupsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list all groups in a todolist", async () => {
      const projectId = 111;
      const todolistId = 222;
      const mockGroups = [
        { id: 1, name: "Phase 1", completed: false, completed_ratio: "3/10" },
        { id: 2, name: "Phase 2", completed: false, completed_ratio: "0/5" },
      ];

      server.use(
        http.get(
          `${BASE_URL}/buckets/${projectId}/todolists/${todolistId}/groups.json`,
          () => {
            return HttpResponse.json(mockGroups);
          }
        )
      );

      const groups = await client.todolistGroups.list(projectId, todolistId);
      expect(groups).toHaveLength(2);
      expect(groups[0]!.name).toBe("Phase 1");
      expect(groups[1]!.completed_ratio).toBe("0/5");
    });

    it("should return empty array when no groups exist", async () => {
      const projectId = 111;
      const todolistId = 222;

      server.use(
        http.get(
          `${BASE_URL}/buckets/${projectId}/todolists/${todolistId}/groups.json`,
          () => {
            return HttpResponse.json([]);
          }
        )
      );

      const groups = await client.todolistGroups.list(projectId, todolistId);
      expect(groups).toHaveLength(0);
    });
  });

  // Note: get() is not in the API spec - groups can only be listed, created, or repositioned;

  describe("create", () => {
    it("should create a new group in a todolist", async () => {
      const projectId = 111;
      const todolistId = 222;
      const mockGroup = {
        id: 444,
        name: "New Phase",
        type: "Todolist::Group",
        completed: false,
      };

      server.use(
        http.post(
          `${BASE_URL}/buckets/${projectId}/todolists/${todolistId}/groups.json`,
          async ({ request }) => {
            const body = await request.json() as { name: string };
            expect(body.name).toBe("New Phase");
            return HttpResponse.json(mockGroup);
          }
        )
      );

      const group = await client.todolistGroups.create(projectId, todolistId, {
        name: "New Phase",
      });
      expect(group.id).toBe(444);
      expect(group.name).toBe("New Phase");
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  // Note: update() is not in the API spec

  describe("reposition", () => {
    it("should change the position of a group", async () => {
      const projectId = 111;
      const groupId = 333;

      server.use(
        http.put(
          `${BASE_URL}/buckets/${projectId}/todolists/${groupId}/position.json`,
          async ({ request }) => {
            const body = await request.json() as { position: number };
            expect(body.position).toBe(1);
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      // Generated service takes a request object, not bare number
      await expect(
        client.todolistGroups.reposition(projectId, groupId, { position: 1 })
      ).resolves.toBeUndefined();
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  // Note: trash() is on RecordingsService, not TodolistGroupsService (spec-conformant)
  // Use client.recordings.trash(projectId, groupId) instead
});
