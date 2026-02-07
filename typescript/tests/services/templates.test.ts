/**
 * Tests for the TemplatesService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No client-side validation (API validates)
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

    // Note: Client-side validation removed - generated services let API validate
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

    // Note: Client-side validation removed - generated services let API validate
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
            const body = await request.json() as { name: string; description?: string };
            expect(body.name).toBe("Q1 Campaign");
            return HttpResponse.json(mockConstruction);
          }
        )
      );

      const construction = await client.templates.createProject(templateId, {
        name: "Q1 Campaign",
        description: "Q1 marketing campaign",
      });
      expect(construction.id).toBe(789);
      expect(construction.status).toBe("pending");
    });

    // Note: Client-side validation removed - generated services let API validate
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
});
