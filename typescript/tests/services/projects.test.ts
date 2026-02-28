/**
 * Tests for the ProjectsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleProject = (id = 1) => ({
  id,
  name: "My Project",
  description: "<p>A cool project</p>",
  status: "active",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("ProjectsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list projects", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([sampleProject(1), sampleProject(2)]);
        })
      );

      const projects = await client.projects.list();
      expect(projects).toHaveLength(2);
      expect(projects[0]!.id).toBe(1);
      expect(projects[1]!.id).toBe(2);
    });

    it("should return empty array when no projects exist", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const projects = await client.projects.list();
      expect(projects).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should return a single project", async () => {
      const projectId = 42;

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}`, () => {
          return HttpResponse.json(sampleProject(projectId));
        })
      );

      const project = await client.projects.get(projectId);
      expect(project.id).toBe(projectId);
      expect(project.name).toBe("My Project");
    });

    it("should throw not_found for missing project", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.projects.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a project with name and description", async () => {
      server.use(
        http.post(`${BASE_URL}/projects.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.name).toBe("New Project");
          expect(body.description).toBe("<p>Details</p>");
          return HttpResponse.json(sampleProject(99), { status: 201 });
        })
      );

      const project = await client.projects.create({
        name: "New Project",
        description: "<p>Details</p>",
      });
      expect(project.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a project", async () => {
      const projectId = 42;

      server.use(
        http.put(`${BASE_URL}/projects/${projectId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.name).toBe("Updated Project");
          return HttpResponse.json(sampleProject(projectId));
        })
      );

      const project = await client.projects.update(projectId, {
        name: "Updated Project",
      });
      expect(project.id).toBe(projectId);
    });
  });

  describe("trash", () => {
    it("should trash a project", async () => {
      server.use(
        http.delete(`${BASE_URL}/projects/42`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.projects.trash(42)).resolves.toBeUndefined();
    });
  });
});
