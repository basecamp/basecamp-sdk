/**
 * Tests for the Vaults service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { VaultsService } from "../../src/services/vaults.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("VaultsService", () => {
  let service: VaultsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.vaults;
  });

  describe("get", () => {
    it("should return a vault by ID", async () => {
      const vault = {
        id: 1001,
        title: "Documents Folder",
        status: "active",
        visible_to_clients: false,
        documents_count: 5,
        uploads_count: 10,
        vaults_count: 2,
      };

      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001`, () => {
          return HttpResponse.json(vault);
        })
      );

      const result = await service.get(123, 1001);

      expect(result.id).toBe(1001);
      expect(result.title).toBe("Documents Folder");
      expect(result.documents_count).toBe(5);
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.get(123, 9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(123, 9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("list", () => {
    it("should return child vaults", async () => {
      const vaults = [
        { id: 2001, title: "Subfolder 1", status: "active" },
        { id: 2002, title: "Subfolder 2", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/vaults.json`, () => {
          return HttpResponse.json(vaults);
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toHaveLength(2);
      expect(result[0].title).toBe("Subfolder 1");
      expect(result[1].title).toBe("Subfolder 2");
    });

    it("should return empty array when no child vaults", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/vaults.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toEqual([]);
    });
  });

  describe("create", () => {
    it("should create a new vault", async () => {
      const newVault = {
        id: 3001,
        title: "New Folder",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/vaults.json`, () => {
          return HttpResponse.json(newVault);
        })
      );

      const result = await service.create(123, 1001, { title: "New Folder" });

      expect(result.id).toBe(3001);
      expect(result.title).toBe("New Folder");
    });

    it("should send title in request body", async () => {
      let capturedBody: { title?: string } | null = null;

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/vaults.json`, async ({ request }) => {
          capturedBody = (await request.json()) as { title?: string };
          return HttpResponse.json({ id: 1, title: "Test" });
        })
      );

      await service.create(123, 1001, { title: "My New Folder" });

      expect(capturedBody?.title).toBe("My New Folder");
    });

    it("should throw validation error when title is missing", async () => {
      await expect(service.create(123, 1001, { title: "" })).rejects.toThrow(BasecampError);

      try {
        await service.create(123, 1001, { title: "" });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("title");
      }
    });
  });

  describe("update", () => {
    it("should update an existing vault", async () => {
      const updatedVault = {
        id: 1001,
        title: "Renamed Folder",
        status: "active",
      };

      server.use(
        http.put(`${BASE_URL}/buckets/123/vaults/1001`, () => {
          return HttpResponse.json(updatedVault);
        })
      );

      const result = await service.update(123, 1001, { title: "Renamed Folder" });

      expect(result.title).toBe("Renamed Folder");
    });

    it("should send title in request body", async () => {
      let capturedBody: { title?: string } | null = null;

      server.use(
        http.put(`${BASE_URL}/buckets/123/vaults/1001`, async ({ request }) => {
          capturedBody = (await request.json()) as { title?: string };
          return HttpResponse.json({ id: 1001, title: "Updated" });
        })
      );

      await service.update(123, 1001, { title: "Updated Title" });

      expect(capturedBody?.title).toBe("Updated Title");
    });
  });
});
