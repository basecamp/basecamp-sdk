/**
 * Tests for the TemplatesService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - Client-side checks: create() rejects a missing name, createProject() rejects a
 *   missing project; the API validates the rest
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("TemplatesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list all templates", async () => {
      const mockTemplates = [
        { id: 1, name: "Marketing Campaign", description: "Standard campaign template" },
        { id: 2, name: "Product Launch", description: "Product launch checklist" },
      ];

      server.use(
        http.get(`${BASE_URL}/templates.json`, () => {
          return HttpResponse.json(mockTemplates);
        })
      );

      const templates = await client.templates.list();
      expect(templates).toHaveLength(2);
      expect(templates[0]!.name).toBe("Marketing Campaign");
    });

    it("should return empty array when no templates exist", async () => {
      server.use(
        http.get(`${BASE_URL}/templates.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const templates = await client.templates.list();
      expect(templates).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should get a template by ID", async () => {
      const templateId = 123;
      const mockTemplate = {
        id: templateId,
        name: "Marketing Campaign",
        description: "Standard campaign template",
        status: "active",
      };

      server.use(
        http.get(`${BASE_URL}/templates/${templateId}`, () => {
          return HttpResponse.json(mockTemplate);
        })
      );

      const template = await client.templates.get(templateId);
      expect(template.id).toBe(templateId);
      expect(template.name).toBe("Marketing Campaign");
    });

    it("should throw not_found error for non-existent template", async () => {
      server.use(
        http.get(`${BASE_URL}/templates/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.templates.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a new template", async () => {
      const mockTemplate = {
        id: 456,
        name: "New Template",
        description: "A new template",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/templates.json`, async ({ request }) => {
          const body = await request.json() as { name: string; description?: string };
          expect(body.name).toBe("New Template");
          expect(body.description).toBe("A new template");
          return HttpResponse.json(mockTemplate);
        })
      );

      const template = await client.templates.create({
        name: "New Template",
        description: "A new template",
      });
      expect(template.id).toBe(456);
      expect(template.name).toBe("New Template");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing name", async () => {
      await expect(
        client.templates.create({ name: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Name is required" });
    });
  });

  describe("update", () => {
    it("should update an existing template", async () => {
      const templateId = 123;
      const mockTemplate = {
        id: templateId,
        name: "Updated Template",
        description: "Updated description",
      };

      server.use(
        http.put(`${BASE_URL}/templates/${templateId}`, async ({ request }) => {
          const body = await request.json() as { name: string };
          expect(body.name).toBe("Updated Template");
          return HttpResponse.json(mockTemplate);
        })
      );

      const template = await client.templates.update(templateId, {
        name: "Updated Template",
        description: "Updated description",
      });
      expect(template.name).toBe("Updated Template");
    });

  });

  describe("delete", () => {
    it("should delete a template", async () => {
      const templateId = 123;

      server.use(
        http.delete(`${BASE_URL}/templates/${templateId}`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.templates.delete(templateId)).resolves.toBeUndefined();
    });
  });

  describe("createProject", () => {
    it("should create a project from a template", async () => {
      const templateId = 123;
      const mockConstruction = {
        id: 789,
        status: "pending",
        url: "https://basecamp.com/constructions/789",
      };

      server.use(
        http.post(
          `${BASE_URL}/templates/${templateId}/project_constructions.json`,
          async ({ request }) => {
            const body = await request.json() as { project: { name: string; description?: string } };
            expect(body.project.name).toBe("Q1 Campaign");
            expect(body.project.description).toBe("Q1 marketing campaign");
            return HttpResponse.json(mockConstruction);
          }
        )
      );

      const construction = await client.templates.createProject(templateId, {
        project: { name: "Q1 Campaign", description: "Q1 marketing campaign" },
      });
      expect(construction.id).toBe(789);
      expect(construction.status).toBe("pending");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing project", async () => {
      await expect(
        client.templates.createProject(1, { project: undefined as never })
      ).rejects.toMatchObject({ code: "validation", message: "Project is required" });
    });
  });

  describe("getConstruction", () => {
    it("should get construction status", async () => {
      const templateId = 123;
      const constructionId = 789;
      const mockConstruction = {
        id: constructionId,
        status: "completed",
        url: "https://basecamp.com/constructions/789",
        project: {
          id: 1000,
          name: "Q1 Campaign",
        },
      };

      server.use(
        http.get(
          `${BASE_URL}/templates/${templateId}/project_constructions/${constructionId}`,
          () => {
            return HttpResponse.json(mockConstruction);
          }
        )
      );

      const construction = await client.templates.getConstruction(templateId, constructionId);
      expect(construction.status).toBe("completed");
      expect(construction.project?.id).toBe(1000);
    });
  });

  describe("template library", () => {
    it("gets the account template library", async () => {
      server.use(
        http.get(`${BASE_URL}/template_library.json`, () => {
          return HttpResponse.json({
            bucket: { id: 1, name: "To-do List Templates", type: "TemplateLibrary" },
            todoset: { id: 2, title: "To-do List Templates", type: "Todoset" },
            todolists: [{ id: 3, name: "Project kickoff" }],
          });
        })
      );

      const library = await client.templates.getLibrary();
      expect(library.bucket.type).toBe("TemplateLibrary");
      expect(library.todoset.id).toBe(2);
      expect(library.todolists[0]?.name).toBe("Project kickoff");
    });

    it("surfaces a forbidden template library read", async () => {
      server.use(
        http.get(`${BASE_URL}/template_library.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      await expect(client.templates.getLibrary()).rejects.toMatchObject({
        code: "forbidden",
        httpStatus: 403,
      });
    });

    it("starts a copy with destination and people confirmation", async () => {
      server.use(
        http.post(`${BASE_URL}/template_library/copies.json`, async ({ request }) => {
          expect(await request.json()).toEqual({
            template_recording_id: 3,
            destination_parent_id: 9,
            adding_people_confirmed: true,
          });
          return HttpResponse.json({
            id: 5,
            status: "pending",
            source_recording_id: 3,
            destination_parent_id: 9,
            url: `${BASE_URL}/template_library/copies/5.json`,
          }, { status: 201 });
        })
      );

      const copy = await client.templates.createLibraryCopy({
        templateRecordingId: 3,
        destinationParentId: 9,
        addingPeopleConfirmed: true,
      });
      expect(copy.id).toBe(5);
      expect(copy.status).toBe("pending");
      expect(copy.destination_todolist).toBeUndefined();
    });

    it("gets a completed copy with its destination to-do list", async () => {
      server.use(
        http.get(`${BASE_URL}/template_library/copies/5`, () => {
          return HttpResponse.json({
            id: 5,
            status: "completed",
            source_recording_id: 3,
            destination_parent_id: 9,
            url: `${BASE_URL}/template_library/copies/5.json`,
            destination_todolist: { id: 10, name: "Project kickoff" },
          });
        })
      );

      const copy = await client.templates.getLibraryCopy(5);
      expect(copy.status).toBe("completed");
      expect(copy.destination_todolist?.id).toBe(10);
    });

    it("surfaces a missing library copy", async () => {
      server.use(
        http.get(`${BASE_URL}/template_library/copies/404`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.templates.getLibraryCopy(404)).rejects.toMatchObject({
        code: "not_found",
        httpStatus: 404,
      });
    });

    it("surfaces people confirmation responses as validation errors", async () => {
      server.use(
        http.post(`${BASE_URL}/template_library/copies.json`, () => {
          return HttpResponse.json({
            error: "Adding people requires confirmation",
            people: [{ id: 4, name: "Victor", avatar_url: "https://example.test/avatar.png" }],
          }, { status: 422 });
        })
      );

      const error = await client.templates.createLibraryCopy({
        templateRecordingId: 3,
        destinationParentId: 9,
      }).catch((caught: unknown) => caught);

      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("validation");
      expect((error as BasecampError).httpStatus).toBe(422);
      expect((error as BasecampError).message).toBe("Adding people requires confirmation");
    });
  });
});
