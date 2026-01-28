/**
 * Tests for the ToolsService
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("ToolsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should get a tool by ID", async () => {
      const projectId = 111;
      const toolId = 222;
      const mockTool = {
        id: toolId,
        name: "todoset",
        title: "To-dos",
        enabled: true,
        position: 1,
      };

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}`, () => {
          return HttpResponse.json({ tool: mockTool });
        })
      );

      const tool = await client.tools.get(projectId, toolId);
      expect(tool.id).toBe(toolId);
      expect(tool.name).toBe("todoset");
      expect(tool.title).toBe("To-dos");
    });

    it("should throw not_found error for non-existent tool", async () => {
      const projectId = 111;

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/dock/tools/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.tools.get(projectId, 999)).rejects.toThrow(BasecampError);
    });
  });

  describe("clone", () => {
    it("should clone a tool", async () => {
      const projectId = 111;
      const sourceToolId = 222;
      const mockTool = {
        id: 333,
        name: "todoset",
        title: "To-dos (Copy)",
        enabled: true,
        position: 5,
      };

      server.use(
        http.post(
          `${BASE_URL}/buckets/${projectId}/dock/tools/${sourceToolId}/clone.json`,
          () => {
            return HttpResponse.json({ tool: mockTool });
          }
        )
      );

      const tool = await client.tools.clone(projectId, sourceToolId);
      expect(tool.id).toBe(333);
      expect(tool.title).toBe("To-dos (Copy)");
    });
  });

  describe("update", () => {
    it("should update (rename) a tool", async () => {
      const projectId = 111;
      const toolId = 222;
      const mockTool = {
        id: toolId,
        name: "todoset",
        title: "Sprint Backlog",
        enabled: true,
      };

      server.use(
        http.put(
          `${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}`,
          async ({ request }) => {
            const body = await request.json() as { title: string };
            expect(body.title).toBe("Sprint Backlog");
            return HttpResponse.json({ tool: mockTool });
          }
        )
      );

      const tool = await client.tools.update(projectId, toolId, "Sprint Backlog");
      expect(tool.title).toBe("Sprint Backlog");
    });

    it("should throw validation error for missing title", async () => {
      await expect(client.tools.update(111, 222, "")).rejects.toThrow(
        "Tool title is required"
      );
    });
  });

  describe("delete", () => {
    it("should delete a tool", async () => {
      const projectId = 111;
      const toolId = 222;

      server.use(
        http.delete(`${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.tools.delete(projectId, toolId)).resolves.toBeUndefined();
    });
  });

  describe("enable", () => {
    it("should enable a tool on the dock", async () => {
      const projectId = 111;
      const toolId = 222;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}/position.json`,
          () => {
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(client.tools.enable(projectId, toolId)).resolves.toBeUndefined();
    });
  });

  describe("disable", () => {
    it("should disable a tool from the dock", async () => {
      const projectId = 111;
      const toolId = 222;

      server.use(
        http.delete(
          `${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}/position.json`,
          () => {
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(client.tools.disable(projectId, toolId)).resolves.toBeUndefined();
    });
  });

  describe("reposition", () => {
    it("should change the position of a tool on the dock", async () => {
      const projectId = 111;
      const toolId = 222;

      server.use(
        http.put(
          `${BASE_URL}/buckets/${projectId}/dock/tools/${toolId}/position.json`,
          async ({ request }) => {
            const body = await request.json() as { position: number };
            expect(body.position).toBe(1);
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(
        client.tools.reposition(projectId, toolId, 1)
      ).resolves.toBeUndefined();
    });

    it("should throw validation error for position less than 1", async () => {
      await expect(client.tools.reposition(111, 222, 0)).rejects.toThrow(
        "Position must be at least 1"
      );

      await expect(client.tools.reposition(111, 222, -5)).rejects.toThrow(
        "Position must be at least 1"
      );
    });
  });
});
