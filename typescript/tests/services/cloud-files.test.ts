/**
 * Tests for the CloudFiles service (generated from the OpenAPI spec).
 *
 * The create path is the load-bearing assertion here: bc3 draws cloud_files
 * under `resources :vaults` only inside the bucket scope, so the create URL is
 * /buckets/{bucketId}/vaults/{vaultId}/cloud_files.json while get and update
 * are flat and unscoped. MSW only answers the exact URL, so a wrong spelling
 * fails these tests the same way it would 404 in production.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { CloudFilesService } from "../../src/generated/services/cloud-files.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const cloudFile = {
  id: 1001,
  status: "active",
  visible_to_clients: false,
  created_at: "2022-11-22T08:30:00.000Z",
  updated_at: "2022-11-22T08:30:00.000Z",
  title: "Brand book draft",
  inherits_status: true,
  type: "CloudFile",
  url: "https://www.dropbox.com/s/abcd1234/brand-draft.pdf",
  app_url: "https://3.basecamp.com/12345/buckets/2085958500/cloud_files/1001",
  parent: { id: 3001, title: "Docs & Files", type: "Vault" },
  bucket: { id: 2085958500, name: "The Leto Laptop", type: "Project" },
  creator: { id: 5001, name: "Victor Cooper" },
  description: '<div dir="auto">Working draft</div>',
  description_attachments: [],
  service: {
    name: "Dropbox",
    example_url: "https://www.dropbox.com/s/abcd1234/example.pdf",
    code: "dropbox",
    valid_patterns: ["(.*?\\.)?dropbox\\.com(\\/.*)?"],
    supporting_text: "a file or folder on Dropbox",
  },
};


describe("CloudFilesService", () => {
  let service: CloudFilesService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
    service = client.cloudFiles;
  });

  describe("cloudFile", () => {
    it("returns a cloud file by ID, with url as the external link", async () => {
      server.use(http.get(`${BASE_URL}/cloud_files/1001`, () => HttpResponse.json(cloudFile)));

      const result = await service.cloudFile(1001);

      expect(result.id).toBe(1001);
      expect(result.title).toBe("Brand book draft");
      // url is the recordable's external link, not this record's API URL —
      // the jbuilder overwrites the recording partial's url key.
      expect(result.url).toBe("https://www.dropbox.com/s/abcd1234/brand-draft.pdf");
      expect(result.app_url).toContain("3.basecamp.com");
      expect(result.service.code).toBe("dropbox");
      expect(result.service.valid_patterns).toHaveLength(1);
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/cloud_files/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 }),
        ),
      );

      await expect(service.cloudFile(9999)).rejects.toThrow(BasecampError);

      try {
        await service.cloudFile(9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("createCloudFile", () => {
    it("posts to the bucket-scoped, vault-nested path", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(
          `${BASE_URL}/buckets/2085958500/vaults/3001/cloud_files.json`,
          async ({ request }) => {
            received = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json(cloudFile, { status: 201 });
          },
        ),
      );

      const result = await service.createCloudFile(2085958500, 3001, {
        url: "https://www.dropbox.com/s/abcd1234/brand.zip",
        service: "dropbox",
        title: "Brand assets",
      });

      expect(result.id).toBe(1001);
      expect(received).toMatchObject({
        url: "https://www.dropbox.com/s/abcd1234/brand.zip",
        service: "dropbox",
        title: "Brand assets",
      });
    });

    it("snake-cases visibleToClients onto the wire", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(
          `${BASE_URL}/buckets/2085958500/vaults/3001/cloud_files.json`,
          async ({ request }) => {
            received = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json(cloudFile, { status: 201 });
          },
        ),
      );

      await service.createCloudFile(2085958500, 3001, {
        url: "https://www.dropbox.com/s/abcd1234/brand.zip",
        service: "dropbox",
        visibleToClients: true,
      });

      expect(received?.visible_to_clients).toBe(true);
      expect(received).not.toHaveProperty("visibleToClients");
    });

    it("refuses the required replace fields before the wire", async () => {
      let called = false;
      server.use(
        http.post(`${BASE_URL}/buckets/2085958500/vaults/3001/cloud_files.json`, () => {
          called = true;
          return HttpResponse.json(cloudFile, { status: 201 });
        }),
      );

      await expect(
        service.createCloudFile(2085958500, 3001, { url: "", service: "dropbox" }),
      ).rejects.toThrow(BasecampError);

      await expect(
        service.createCloudFile(2085958500, 3001, {
          url: "https://www.dropbox.com/s/abcd1234/brand.zip",
          service: "",
        }),
      ).rejects.toThrow(BasecampError);

      expect(called).toBe(false);
    });

    it("surfaces the field-keyed 422 as a validation error", async () => {
      server.use(
        http.post(`${BASE_URL}/buckets/2085958500/vaults/3001/cloud_files.json`, () =>
          HttpResponse.json(
            { errors: { url: ["is not a valid Dropbox url"] } },
            { status: 422 },
          ),
        ),
      );

      try {
        await service.createCloudFile(2085958500, 3001, {
          url: "https://example.com/nope",
          service: "dropbox",
        });
        expect.unreachable("expected a validation error");
      } catch (err) {
        const e = err as BasecampError;
        expect(e).toBeInstanceOf(BasecampError);
        expect(e.code).toBe("validation");
        expect(e.fieldErrors).toEqual({ url: ["is not a valid Dropbox url"] });
      }
    });
  });

  describe("updateCloudFile", () => {
    it("puts to the flat, unscoped path with the full representation", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.put(`${BASE_URL}/cloud_files/1001`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...cloudFile, title: "Brand assets v2" });
        }),
      );

      const result = await service.updateCloudFile(1001, {
        url: "https://www.dropbox.com/s/abcd1234/brand-v2.zip",
        service: "dropbox",
        title: "Brand assets v2",
      });

      expect(result.title).toBe("Brand assets v2");
      expect(received).toMatchObject({
        url: "https://www.dropbox.com/s/abcd1234/brand-v2.zip",
        service: "dropbox",
      });
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.put(`${BASE_URL}/cloud_files/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 }),
        ),
      );

      await expect(
        service.updateCloudFile(9999, { url: "https://www.dropbox.com/s/a/b.pdf", service: "dropbox" }),
      ).rejects.toThrow(BasecampError);
    });
  });
});
