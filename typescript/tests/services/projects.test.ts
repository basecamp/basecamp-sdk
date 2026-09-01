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

const sampleProject = (id = 1, starred = false) => ({
  id,
  name: "My Project",
  description: "<p>A cool project</p>",
  status: "active",
  start_date: "2024-01-01",
  end_date: "2024-03-31",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
  star_url: `${BASE_URL}/buckets/${id}/stars.json`,
  bookmarked: true,
  starred,
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
          return HttpResponse.json([sampleProject(1, true), sampleProject(2)]);
        })
      );

      const projects = await client.projects.list();
      expect(projects).toHaveLength(2);
      expect(projects[0]!.id).toBe(1);
      expect(projects[0]!.start_date).toBe("2024-01-01");
      expect(projects[1]!.id).toBe(2);
      // starred implies bookmarked, never the reverse: the second project is pinned but unstarred.
      expect(projects.map((p) => p.bookmarked)).toEqual([true, true]);
      expect(projects.map((p) => p.starred)).toEqual([true, false]);
      expect(projects[1]!.star_url).toBe(`${BASE_URL}/buckets/2/stars.json`);
    });

    it("should send no status param by default", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.has("status")).toBe(false);
          return HttpResponse.json([sampleProject(1)]);
        })
      );

      const projects = await client.projects.list();
      expect(projects).toHaveLength(1);
    });

    it("should forward an explicit active status — a server-accepted alias of the default", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("status")).toBe("active");
          return HttpResponse.json([sampleProject(1)]);
        })
      );

      const projects = await client.projects.list({ status: "active" });
      expect(projects).toHaveLength(1);
    });

    it("should forward the archived status filter", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("status")).toBe("archived");
          return HttpResponse.json([sampleProject(1)]);
        })
      );

      const projects = await client.projects.list({ status: "archived" });
      expect(projects).toHaveLength(1);
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
      expect(project.start_date).toBe("2024-01-01");
      expect(project.end_date).toBe("2024-03-31");
      expect(project.star_url).toBe(`${BASE_URL}/buckets/${projectId}/stars.json`);
      // starred implies bookmarked, never the reverse: pinned but unstarred is the discriminating case.
      expect(project.bookmarked).toBe(true);
      expect(project.starred).toBe(false);
    });

    it("should carry starred through alongside bookmarked", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/7`, () => {
          return HttpResponse.json(sampleProject(7, true));
        })
      );

      const project = await client.projects.get(7);
      expect(project.bookmarked).toBe(true);
      expect(project.starred).toBe(true);
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

  describe("listRecentProjects", () => {
    it("should list recently visited projects, most recent visit first", async () => {
      // The recently-visited list is the standard projection plus bookmarked
      // only — the wire omits starred here (BC3 #13043).
      const recent = [sampleProject(2), { ...sampleProject(1), bookmarked: false }].map(
        ({ starred: _starred, ...p }) => p
      );
      server.use(
        http.get(`${BASE_URL}/my/recent_projects.json`, () => {
          return HttpResponse.json(recent);
        })
      );

      const projects = await client.projects.listRecentProjects();
      expect(projects.map((p) => p.id)).toEqual([2, 1]);
      expect(projects.map((p) => p.bookmarked)).toEqual([true, false]);
      expect(projects.every((p) => p.starred === undefined)).toBe(true);
    });

    it("should return an empty array when nothing has been visited", async () => {
      server.use(
        http.get(`${BASE_URL}/my/recent_projects.json`, () => {
          return HttpResponse.json([]);
        })
      );

      await expect(client.projects.listRecentProjects()).resolves.toEqual([]);
    });
  });

  describe("recordProjectVisit", () => {
    it("should record a visit with a bodyless 204", async () => {
      server.use(
        http.post(`${BASE_URL}/projects/42/recent_visit.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.projects.recordProjectVisit(42)).resolves.toBeUndefined();
    });

    it("should throw not_found for an inaccessible project", async () => {
      server.use(
        http.post(`${BASE_URL}/projects/999/recent_visit.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.projects.recordProjectVisit(999)).rejects.toThrow(BasecampError);
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

  describe("archive", () => {
    it("should archive a project", async () => {
      server.use(
        http.put(`${BASE_URL}/projects/42/status/archived.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.projects.archive(42)).resolves.toBeUndefined();
    });

    // The admin pro pack can limit archiving to admins and the project's
    // creator; bc3 answers `head :forbidden` from
    // ensure_can_archive_or_trash_project.
    it("should surface a 403 when archiving is restricted", async () => {
      server.use(
        http.put(`${BASE_URL}/projects/42/status/archived.json`, () => {
          return new HttpResponse(null, { status: 403 });
        })
      );

      await expect(client.projects.archive(42)).rejects.toMatchObject({
        httpStatus: 403,
      });
    });
  });

  describe("unarchive", () => {
    it("should unarchive a project", async () => {
      server.use(
        http.put(`${BASE_URL}/projects/42/status/active.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.projects.unarchive(42)).resolves.toBeUndefined();
    });

    // The only behavioural evidence for ProjectLimitError. A 507 is an account
    // limit, so it maps to limit_exceeded and is NOT retryable — no backoff
    // frees a project slot (SPEC.md §6, step 11).
    it("should surface the 507 project limit as a non-retryable limit_exceeded", async () => {
      server.use(
        http.put(`${BASE_URL}/projects/42/status/active.json`, () => {
          return HttpResponse.json(
            { error: "The project limit for this account has been reached." },
            { status: 507 }
          );
        })
      );

      await expect(client.projects.unarchive(42)).rejects.toMatchObject({
        code: "limit_exceeded",
        httpStatus: 507,
        retryable: false,
      });
    });
  });
});
