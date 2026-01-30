/**
 * Tests for the TodolistGroupsService
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
      expect(groups).toEqual([]);
    });
  });

  describe("get", () => {
    it("should get a group by ID", async () => {
      const projectId = 111;
      const groupId = 333;
      const mockGroup = {
        id: groupId,
        name: "Phase 1",
        type: "Todolist::Group",
        completed: false,
        completed_ratio: "5/10",
      };

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/todolists/${groupId}`, () => {
          return HttpResponse.json({ group: mockGroup });
        })
      );

      const group = await client.todolistGroups.get(projectId, groupId);
      expect(group.id).toBe(groupId);
      expect(group.name).toBe("Phase 1");
    });

    it("should throw not_found when endpoint returns a todolist instead of group", async () => {
      const projectId = 111;
      const groupId = 333;
      const mockTodolist = {
        id: groupId,
        name: "Not a group",
        type: "Todolist",
      };

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/todolists/${groupId}`, () => {
          return HttpResponse.json({ todolist: mockTodolist });
        })
      );

      await expect(
        client.todolistGroups.get(projectId, groupId)
      ).rejects.toThrow("Todolist group not found");
    });
  });

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

    it("should throw validation error for missing name", async () => {
      await expect(
        client.todolistGroups.create(111, 222, { name: "" })
      ).rejects.toThrow("Group name is required");
    });
  });

  describe("update", () => {
    it("should update a group name", async () => {
      const projectId = 111;
      const groupId = 333;
      const mockGroup = {
        id: groupId,
        name: "Updated Phase Name",
        type: "Todolist::Group",
      };

      server.use(
        http.put(`${BASE_URL}/buckets/${projectId}/todolists/${groupId}`, async ({ request }) => {
          const body = await request.json() as { name: string };
          expect(body.name).toBe("Updated Phase Name");
          return HttpResponse.json({ group: mockGroup });
        })
      );

      const group = await client.todolistGroups.update(projectId, groupId, {
        name: "Updated Phase Name",
      });
      expect(group.name).toBe("Updated Phase Name");
    });
  });

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

      await expect(
        client.todolistGroups.reposition(projectId, groupId, 1)
      ).resolves.toBeUndefined();
    });

    it("should throw validation error for position less than 1", async () => {
      await expect(
        client.todolistGroups.reposition(111, 333, 0)
      ).rejects.toThrow("Position must be at least 1");

      await expect(
        client.todolistGroups.reposition(111, 333, -1)
      ).rejects.toThrow("Position must be at least 1");
    });
  });

  describe("trash", () => {
    it("should move a group to trash", async () => {
      const projectId = 111;
      const groupId = 333;

      server.use(
        http.put(
          `${BASE_URL}/buckets/${projectId}/recordings/${groupId}/status/trashed.json`,
          () => {
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(
        client.todolistGroups.trash(projectId, groupId)
      ).resolves.toBeUndefined();
    });
  });
});
