/**
 * Tests for the ToolsService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - update() and reposition() take request objects, not bare params
 * - No client-side validation (API validates)
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
      const toolId = 222;
      const mockTool = {
        id: toolId,
        name: "todoset",
        title: "To-dos",
        enabled: true,
        position: 1,
      };

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(mockTool);
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.id).toBe(toolId);
      expect(tool.name).toBe("todoset");
      expect(tool.title).toBe("To-dos");
    });

    it("should throw not_found error for non-existent tool", async () => {

      server.use(
        http.get(`${BASE_URL}/dock/tools/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.tools.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("clone", () => {
    it("should clone a tool", async () => {
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
          `${BASE_URL}/dock/tools.json`,
          async ({ request }) => {
            const body = await request.json() as { source_recording_id: number };
            expect(body.source_recording_id).toBe(sourceToolId);
            return HttpResponse.json(mockTool, { status: 201 });
          }
        )
      );

      const tool = await client.tools.clone({ sourceRecordingId: sourceToolId });
      expect(tool.id).toBe(333);
      expect(tool.title).toBe("To-dos (Copy)");
    });
  });

  describe("update", () => {
    it("should update (rename) a tool", async () => {
      const toolId = 222;
      const mockTool = {
        id: toolId,
        name: "todoset",
        title: "Sprint Backlog",
        enabled: true,
      };

      server.use(
        http.put(
          `${BASE_URL}/dock/tools/${toolId}`,
          async ({ request }) => {
            const body = await request.json() as { title: string };
            expect(body.title).toBe("Sprint Backlog");
            return HttpResponse.json(mockTool);
          }
        )
      );

      // Generated service takes a request object
      const tool = await client.tools.update(toolId, { title: "Sprint Backlog" });
      expect(tool.title).toBe("Sprint Backlog");
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  describe("delete", () => {
    it("should delete a tool", async () => {
      const toolId = 222;

      server.use(
        http.delete(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.tools.delete(toolId)).resolves.toBeUndefined();
    });
  });

  describe("enable", () => {
    it("should enable a tool on the dock", async () => {
      const toolId = 222;

      server.use(
        http.post(
          `${BASE_URL}/recordings/${toolId}/position.json`,
          () => {
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(client.tools.enable(toolId)).resolves.toBeUndefined();
    });
  });

  describe("disable", () => {
    it("should disable a tool from the dock", async () => {
      const toolId = 222;

      server.use(
        http.delete(
          `${BASE_URL}/recordings/${toolId}/position.json`,
          () => {
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      await expect(client.tools.disable(toolId)).resolves.toBeUndefined();
    });
  });

  describe("reposition", () => {
    it("should change the position of a tool on the dock", async () => {
      const toolId = 222;

      server.use(
        http.put(
          `${BASE_URL}/recordings/${toolId}/position.json`,
          async ({ request }) => {
            const body = await request.json() as { position: number };
            expect(body.position).toBe(1);
            return new HttpResponse(null, { status: 204 });
          }
        )
      );

      // Generated service takes a request object
      await expect(
        client.tools.reposition(toolId, { position: 1 })
      ).resolves.toBeUndefined();
    });

    // Note: Client-side validation removed - generated services let API validate
  });
});
