import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

// The list shape: base folder fields, no `projects`. Note `type` is "Stack",
// not "Folder" — the product was renamed, the wire was not.
function sampleFolder(id: number, name = "Client work") {
  return {
    id,
    name,
    type: "Stack",
    created_at: "2026-07-27T10:16:49.312Z",
    updated_at: "2026-07-27T10:16:49.325Z",
    bucket_ids: [201, 202],
    is_emoji_only_name: false,
    star_url: `${BASE_URL}/buckets/${id}/stars.json`,
    gauges_url: null,
    color: null,
    image_url: null,
    url: `${BASE_URL}/stacks/${id}.json`,
  };
}

function sampleProject(id: number, name: string) {
  return {
    id,
    status: "active",
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-02T00:00:00Z",
    name,
    description: "",
    purpose: "topic",
    clients_enabled: false,
    bookmark_url: `${BASE_URL}/my/bookmarks/abc.json`,
    url: `${BASE_URL}/projects/${id}.json`,
    app_url: `https://3.basecamp.com/12345/projects/${id}`,
  };
}

// The get/create/update shape: the base folder plus the expanded projects.
function sampleFolderWithProjects(id: number, name = "Client work") {
  return {
    ...sampleFolder(id, name),
    projects: [sampleProject(201, "Refresh"), sampleProject(202, "Nike promotion")],
  };
}

describe("FoldersService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("listFolders", () => {
    it("lists the folders as a bare array, without expanded projects", async () => {
      server.use(
        http.get(`${BASE_URL}/stacks.json`, () => {
          return HttpResponse.json([sampleFolder(1), sampleFolder(2, "Personal")]);
        })
      );

      const result = await client.folders.listFolders();
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(1);
      expect(result[0].type).toBe("Stack");
      expect(result[0].bucket_ids).toEqual([201, 202]);
      // The list item type has no `projects` member at all.
      expect("projects" in result[0]).toBe(false);
    });

    it("decodes the always-present, often-null gauges_url/color/image_url", async () => {
      server.use(
        http.get(`${BASE_URL}/stacks.json`, () => {
          return HttpResponse.json([sampleFolder(1)]);
        })
      );

      const [folder] = await client.folders.listFolders();
      expect(folder.gauges_url).toBeNull();
      expect(folder.color).toBeNull();
      expect(folder.image_url).toBeNull();
    });

    it("surfaces 401 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/stacks.json`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      const error = await client.folders.listFolders().catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(401);
    });
  });

  describe("getFolder", () => {
    it("returns the folder with its grouped projects expanded", async () => {
      server.use(
        http.get(`${BASE_URL}/stacks/1`, () => {
          return HttpResponse.json(sampleFolderWithProjects(1));
        })
      );

      const folder = await client.folders.getFolder(1);
      expect(folder.id).toBe(1);
      expect(folder.projects).toHaveLength(2);
      expect(folder.projects[0].name).toBe("Refresh");
      // bucket_ids and projects describe the same relationship.
      expect(folder.bucket_ids).toEqual(folder.projects.map((p) => p.id));
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/stacks/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.folders.getFolder(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("createFolder", () => {
    it("sends project_ids and returns 201 with the expanded folder", async () => {
      let body: unknown;
      server.use(
        http.post(`${BASE_URL}/stacks.json`, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(sampleFolderWithProjects(7), { status: 201 });
        })
      );

      const folder = await client.folders.createFolder({
        name: "Client work",
        projectIds: [201, 202],
      });
      expect(body).toEqual({ name: "Client work", project_ids: [201, 202] });
      expect(folder.id).toBe(7);
      expect(folder.projects).toHaveLength(2);
    });

    it("surfaces the zero-write 404 when a project id is unreachable", async () => {
      server.use(
        http.post(`${BASE_URL}/stacks.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.folders
        .createFolder({ name: "Mixed", projectIds: [201, 999999999] })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("updateFolder", () => {
    it("renames the folder and returns it with projects", async () => {
      let body: unknown;
      server.use(
        http.put(`${BASE_URL}/stacks/1`, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(sampleFolderWithProjects(1, "Active client work"));
        })
      );

      const folder = await client.folders.updateFolder(1, { name: "Active client work" });
      expect(body).toEqual({ name: "Active client work" });
      expect(folder.name).toBe("Active client work");
      expect(folder.projects).toHaveLength(2);
    });

    it("surfaces 422 for a blank name", async () => {
      server.use(
        http.put(`${BASE_URL}/stacks/1`, () => {
          return HttpResponse.json({ errors: { name: ["can't be blank"] } }, { status: 422 });
        })
      );

      // A blank name never reaches the wire — the generated guard rejects it first.
      const guarded = await client.folders.updateFolder(1, { name: "" }).catch((e: unknown) => e);
      expect(guarded).toBeInstanceOf(BasecampError);

      const error = await client.folders
        .updateFolder(1, { name: "   " })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });
  });

  describe("deleteFolder", () => {
    it("deletes the folder (204)", async () => {
      let called = false;
      server.use(
        http.delete(`${BASE_URL}/stacks/1`, () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.folders.deleteFolder(1);
      expect(called).toBe(true);
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.delete(`${BASE_URL}/stacks/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.folders.deleteFolder(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });
});
