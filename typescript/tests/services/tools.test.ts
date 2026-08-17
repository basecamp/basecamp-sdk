/**
 * Tests for the ToolsService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - update() and reposition() take request objects, not bare params
 * - Request fields follow the generated OpenAPI shapes
 * - Client-side checks: create() rejects a missing toolType, update() rejects a
 *   missing title; the API validates the rest
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import toolFixture from "../../../spec/fixtures/tools/get.json" with { type: "json" };
import createdToolFixture from "../../../spec/fixtures/tools/create.json" with { type: "json" };
import updatedToolFixture from "../../../spec/fixtures/tools/update.json" with { type: "json" };
import disabledToolFixture from "../../../spec/fixtures/tools/disabled.json" with { type: "json" };
import nestedVaultToolFixture from "../../../spec/fixtures/tools/nested_vault.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";

// A dock tool's projection is the BARE recordings/recording partial:
// app/views/api/docks/tools/show.json.jbuilder is one line — `json.partial!
// "recordings/recording", recording: @recording` — and adds nothing. Unlike
// Todoset/Questionnaire, whose own recordable partials add it, a tool response
// therefore carries no `name`, and no `enabled` at all. The fabricated stub
// bodies these tests used to carry (`name: "todoset", enabled: true`) are
// exactly how `name`/`enabled` stayed @required through six SDKs (#650), so the
// stubs below are sourced from the shared, coverage-guarded fixtures
// (spec/fixtures/manifest.yaml) and cannot drift back.
const sampleTool = (id = toolFixture.id) => ({ ...toolFixture, id });

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

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(sampleTool(toolId));
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.id).toBe(toolId);
      expect(tool.title).toBe("Chat");
      expect(tool.type).toBe("Chat::Transcript");
      expect(tool.visible_to_clients).toBe(false);
      expect(tool.inherits_status).toBe(true);
      expect(tool.status).toBe("active");
      expect(tool.bookmark_url).toBe(toolFixture.bookmark_url);
      // Chat::Transcript overrides Recordable#subscribable?, so the partial's
      // `if recording.subscribable?` branch renders subscription_url here.
      expect(tool.subscription_url).toBe(toolFixture.subscription_url);
      // Present because the tool is on the dock (`recording.positioned?`).
      expect(tool.position).toBe(5);
      expect(tool.creator.id).toBe(toolFixture.creator.id);
      expect(tool.creator.name).toBe("Victor Cooper");
      expect(tool.bucket?.id).toBe(toolFixture.bucket.id);
      expect(tool.bucket?.type).toBe("Project");
    });

    // Regression guard for #650. `name` and `enabled` were @required on Tool,
    // yet BC3 emits neither key on ANY tool response — so this test's body is
    // not an edge case, it is every real response. The old stubs fabricated
    // both keys and asserted `tool.name`, which is why the bug never went red.
    it("accepts a response carrying neither name nor enabled", async () => {
      const toolId = 222;

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(sampleTool(toolId));
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.name).toBeUndefined();
      expect(tool.enabled).toBeUndefined();
      // The keys BC3 does emit still flow through intact.
      expect(tool.id).toBe(toolId);
      expect(tool.title).toBe("Chat");
      expect(tool.type).toBe("Chat::Transcript");
      expect(tool.inherits_status).toBe(true);
      expect(tool.creator.id).toBe(toolFixture.creator.id);
    });

    // A disabled tool is removed from the dock but NOT deleted, so
    // `recording.positioned?` is false and `position` is absent entirely —
    // absence of `position`, not `enabled: false`, is the disabled signal.
    // This one is also a Vault, which does not override Recordable#subscribable?
    // (default false), so `subscription_url` is absent too.
    it("accepts a disabled tool with no position and no subscription_url", async () => {
      const toolId = disabledToolFixture.id;

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(disabledToolFixture);
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.position).toBeUndefined();
      expect(tool.subscription_url).toBeUndefined();
      expect(tool.enabled).toBeUndefined();
      expect(tool.name).toBeUndefined();
      expect(tool.type).toBe("Vault");
      expect(tool.title).toBe("Docs & Files");
      expect(tool.bookmark_url).toBe(disabledToolFixture.bookmark_url);
    });

    // `parent` is emitted only when `!recording.docked?`. The dock-tool lookup
    // scopes by recordable TYPE (Recordable::CORE_GROUPS["dock_tools"] includes
    // Vault) rather than by dock membership, so a vault nested inside another
    // vault resolves through GET /dock/tools/:id and does carry a parent.
    it("accepts a nested vault carrying a parent", async () => {
      const toolId = nestedVaultToolFixture.id;

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(nestedVaultToolFixture);
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.parent?.id).toBe(nestedVaultToolFixture.parent.id);
      expect(tool.parent?.type).toBe("Vault");
      expect(tool.parent?.title).toBe("Docs & Files");
      expect(tool.type).toBe("Vault");
      expect(tool.title).toBe("Contracts");
      expect(tool.name).toBeUndefined();
      expect(tool.enabled).toBeUndefined();
    });

    // The converse: a docked tool has no parent at all.
    it("accepts a docked tool with no parent", async () => {
      const toolId = 222;

      server.use(
        http.get(`${BASE_URL}/dock/tools/${toolId}`, () => {
          return HttpResponse.json(sampleTool(toolId));
        })
      );

      const tool = await client.tools.get(toolId);
      expect(tool.parent).toBeUndefined();
      expect(tool.position).toBe(5);
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

  describe("create", () => {
    it("should create a tool in a bucket", async () => {
      const bucketId = 456;
      const toolType = createdToolFixture.type;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/dock/tools.json`,
          async ({ request }) => {
            const body = await request.json() as { tool_type: string; title: string };
            expect(body.tool_type).toBe(toolType);
            expect(body.title).toBe("Q&A Chat");
            return HttpResponse.json(createdToolFixture, { status: 201 });
          }
        )
      );

      const tool = await client.tools.create(bucketId, { toolType, title: "Q&A Chat" });
      expect(tool.id).toBe(createdToolFixture.id);
      expect(tool.title).toBe("Q&A Chat");
      expect(tool.type).toBe("Chat::Transcript");
      expect(tool.visible_to_clients).toBe(true);
      expect(tool.position).toBe(6);
      expect(tool.creator.id).toBe(createdToolFixture.creator.id);
      // The create projection is the same bare partial as get: no name, no enabled.
      expect(tool.name).toBeUndefined();
      expect(tool.enabled).toBeUndefined();
    });

    it("omits title from the request body when not provided", async () => {
      const bucketId = 456;
      const toolType = createdToolFixture.type;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/dock/tools.json`,
          async ({ request }) => {
            const body = await request.json() as Record<string, unknown>;
            expect(body).toEqual({ tool_type: toolType });
            return HttpResponse.json(createdToolFixture, { status: 201 });
          }
        )
      );

      const tool = await client.tools.create(bucketId, { toolType });
      expect(tool.id).toBe(createdToolFixture.id);
    });

    it("requires a tool type", async () => {
      await expect(
        client.tools.create(456, { toolType: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Tool type is required" });
    });

    // visibleToClients is tri-state: undefined omits the key, true/false are sent
    // verbatim. An explicit false must reach the wire (not be dropped). Only
    // Chat::Transcript and Kanban::Board honor it; all other tool types ignore it.
    it("should send visible_to_clients tri-state in request body", async () => {
      const bucketId = 456;
      const toolType = "Chat::Transcript";
      let capturedBody: Record<string, unknown> = {};

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/dock/tools.json`,
          async ({ request }) => {
            capturedBody = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json(createdToolFixture, { status: 201 });
          }
        )
      );

      await client.tools.create(bucketId, { toolType });
      expect("visible_to_clients" in capturedBody).toBe(false);

      await client.tools.create(bucketId, { toolType, visibleToClients: true });
      expect(capturedBody.visible_to_clients).toBe(true);

      await client.tools.create(bucketId, { toolType, visibleToClients: false });
      expect("visible_to_clients" in capturedBody).toBe(true);
      expect(capturedBody.visible_to_clients).toBe(false);
    });
  });

  describe("update", () => {
    it("should update (rename) a tool", async () => {
      const toolId = updatedToolFixture.id;

      server.use(
        http.put(
          `${BASE_URL}/dock/tools/${toolId}`,
          async ({ request }) => {
            const body = await request.json() as { title: string };
            expect(body.title).toBe("Team Chat");
            return HttpResponse.json(updatedToolFixture);
          }
        )
      );

      // Generated service takes a request object
      const tool = await client.tools.update(toolId, { title: "Team Chat" });
      expect(tool.title).toBe("Team Chat");
      expect(tool.type).toBe("Chat::Transcript");
      expect(tool.inherits_status).toBe(true);
      expect(tool.position).toBe(5);
      // The update projection is the same bare partial as get: no name, no enabled.
      expect(tool.name).toBeUndefined();
      expect(tool.enabled).toBeUndefined();
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing title", async () => {
      await expect(
        client.tools.update(1, { title: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Title is required" });
    });
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

  });
});
