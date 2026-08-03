/**
 * Tests for the TodolistGroupsService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No get() method (not in API spec)
 * - No update() method (not in API spec)
 * - No domain-specific trash() (use recordings.trash())
 * - Client-side check: create() rejects a missing name; the API validates the rest
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
      const todolistId = 222;
      const mockGroups = [
        { id: 1, name: "Phase 1", completed: false, completed_ratio: "3/10" },
        { id: 2, name: "Phase 2", completed: false, completed_ratio: "0/5" },
      ];

      server.use(
        http.get(
          `${BASE_URL}/todolists/${todolistId}/groups.json`,
          () => {
            return HttpResponse.json(mockGroups);
          }
        )
      );

      const groups = await client.todolistGroups.list(todolistId);
      expect(groups).toHaveLength(2);
      expect(groups[0]!.name).toBe("Phase 1");
      expect(groups[1]!.completed_ratio).toBe("0/5");
    });

    it("should return empty array when no groups exist", async () => {
      const todolistId = 222;

      server.use(
        http.get(
          `${BASE_URL}/todolists/${todolistId}/groups.json`,
          () => {
            return HttpResponse.json([]);
          }
        )
      );

      const groups = await client.todolistGroups.list(todolistId);
      expect(groups).toHaveLength(0);
    });
  });

  // Note: get() is not in the API spec - groups can only be listed, created, or repositioned;

  describe("create", () => {
    it("should create a new group in a todolist", async () => {
      const todolistId = 222;
      // A group IS a to-do list on the wire: BC3's
      // todolists/groups/{index,show}.json.jbuilder render
      // todolists/_todolist.json.jbuilder, and the recording partial emits
      // `recordable_type`, which is "Todolist" for both variants. ("Todolist::Group"
      // is a *webhook* recording type — see ruby/lib/basecamp/webhooks/event.rb's
      // TODOLIST_GROUP and go/pkg/basecamp/webhook_event.go's
      // WebhookTypeTodolistGroup — and never appears in this payload.) So the
      // truthful stub carries a description and discriminates structurally:
      // group_position_url (parent is a Todolist), and no groups_url.
      const mockGroup = {
        id: 444,
        name: "New Phase",
        type: "Todolist",
        completed: false,
        description: "<div>Second half of the build</div>",
        description_attachments: [],
        group_position_url:
          "https://3.basecampapi.com/12345/buckets/1/todolists/groups/444/position.json",
      };

      server.use(
        http.post(
          `${BASE_URL}/todolists/${todolistId}/groups.json`,
          async ({ request }) => {
            const body = await request.json() as { name: string };
            expect(body.name).toBe("New Phase");
            return HttpResponse.json(mockGroup);
          }
        )
      );

      const group = await client.todolistGroups.create(todolistId, {
        name: "New Phase",
      });
      expect(group.id).toBe(444);
      expect(group.name).toBe("New Phase");
      // create carries the flat Todolist now (#544), description included — the
      // old group projection modelled none.
      expect(group.description).toBe("<div>Second half of the build</div>");
      expect(group.group_position_url).toBe(mockGroup.group_position_url);
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing name", async () => {
      await expect(
        client.todolistGroups.create(1, { name: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Name is required" });
    });
  });

  // Note: update() is not in the API spec

  describe("reposition", () => {
    it("should change the position of a group", async () => {
      const groupId = 333;

      server.use(
        http.put(
          `${BASE_URL}/todolists/groups/${groupId}/position.json`,
          async ({ request }) => {
            const body = await request.json() as { position: number };
            expect(body.position).toBe(1);
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      // Generated service takes a request object, not bare number
      await expect(
        client.todolistGroups.reposition(groupId, { position: 1 })
      ).resolves.toBeUndefined();
    });

  });

  // Note: trash() is on RecordingsService, not TodolistGroupsService (spec-conformant)
  // Use client.recordings.trash(groupId) instead
});
