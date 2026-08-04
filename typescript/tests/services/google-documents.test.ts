/**
 * Tests for the GoogleDocuments service (generated from the OpenAPI spec).
 *
 * The create path is the load-bearing assertion: bc3 draws google_documents
 * under `resources :vaults` only inside the bucket scope, so the create URL is
 * /buckets/{bucketId}/vaults/{vaultId}/google_documents.json while get and
 * update are flat and unscoped. MSW only answers the exact URL, so a wrong
 * spelling fails these tests the same way it would 404 in production.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { GoogleDocumentsService } from "../../src/generated/services/google-documents.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const googleDocument = {
  id: 2002,
  status: "active",
  visible_to_clients: false,
  created_at: "2022-11-22T08:30:00.000Z",
  updated_at: "2022-11-22T08:30:00.000Z",
  title: "Roadmap (draft)",
  inherits_status: true,
  type: "GoogleDocument",
  // The EXTERNAL link, not this record's API url — the google_documents
  // jbuilder renders the recording partial and then overwrites url with the
  // recordable's.
  url: "https://docs.google.com/document/d/abcd1234/edit",
  app_url: "https://3.basecamp.com/12345/buckets/2085958500/google_documents/2002",
  parent: { id: 3001, title: "Docs & Files", type: "Vault" },
  bucket: { id: 2085958500, name: "The Leto Laptop", type: "Project" },
  creator: { id: 5001, name: "Victor Cooper" },
  description: '<div dir="auto">Quarterly roadmap</div>',
  description_attachments: [],
  document_type: "doc",
};

describe("GoogleDocumentsService", () => {
  let service: GoogleDocumentsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
    service = client.googleDocuments;
  });

  describe("googleDocument", () => {
    it("returns a Google document by ID, with url as the external link", async () => {
      server.use(
        http.get(`${BASE_URL}/google_documents/2002`, () => HttpResponse.json(googleDocument)),
      );

      const result = await service.googleDocument(2002);

      expect(result.id).toBe(2002);
      expect(result.document_type).toBe("doc");
      expect(result.url).toBe("https://docs.google.com/document/d/abcd1234/edit");
      expect(result.url).not.toContain("basecampapi.com");
      expect(result.app_url).toContain("3.basecamp.com");
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/google_documents/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 }),
        ),
      );

      await expect(service.googleDocument(9999)).rejects.toThrow(BasecampError);

      try {
        await service.googleDocument(9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("createGoogleDocument", () => {
    it("posts to the bucket-scoped, vault-nested path", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.post(
          `${BASE_URL}/buckets/2085958500/vaults/3001/google_documents.json`,
          async ({ request }) => {
            received = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json(googleDocument, { status: 201 });
          },
        ),
      );

      const result = await service.createGoogleDocument(2085958500, 3001, {
        url: "https://docs.google.com/document/d/abcd1234/edit",
        documentType: "doc",
        title: "Roadmap",
      });

      expect(result.id).toBe(2002);
      expect(received).toMatchObject({
        url: "https://docs.google.com/document/d/abcd1234/edit",
        document_type: "doc",
      });
    });

    it("refuses the required replace fields before the wire", async () => {
      let called = false;
      server.use(
        http.post(`${BASE_URL}/buckets/2085958500/vaults/3001/google_documents.json`, () => {
          called = true;
          return HttpResponse.json(googleDocument, { status: 201 });
        }),
      );

      await expect(
        service.createGoogleDocument(2085958500, 3001, { url: "", documentType: "doc" }),
      ).rejects.toThrow(BasecampError);

      await expect(
        service.createGoogleDocument(2085958500, 3001, {
          url: "https://docs.google.com/document/d/abcd1234/edit",
          documentType: "",
        }),
      ).rejects.toThrow(BasecampError);

      expect(called).toBe(false);
    });

    it("surfaces bc3's document_type rejection as a validation error", async () => {
      server.use(
        http.post(`${BASE_URL}/buckets/2085958500/vaults/3001/google_documents.json`, () =>
          HttpResponse.json(
            { errors: { document_type: ["is not a valid document type"] } },
            { status: 422 },
          ),
        ),
      );

      try {
        await service.createGoogleDocument(2085958500, 3001, {
          url: "https://docs.google.com/document/d/abcd1234/edit",
          documentType: "bogus",
        });
        expect.unreachable("expected a validation error");
      } catch (err) {
        const e = err as BasecampError;
        expect(e).toBeInstanceOf(BasecampError);
        expect(e.code).toBe("validation");
        expect(e.httpStatus).toBe(422);
        expect(e.fieldErrors).toEqual({ document_type: ["is not a valid document type"] });
      }
    });
  });

  describe("updateGoogleDocument", () => {
    it("puts to the flat, unscoped path with the full representation", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.put(`${BASE_URL}/google_documents/2002`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...googleDocument, title: "Roadmap (revised)" });
        }),
      );

      const result = await service.updateGoogleDocument(2002, {
        url: "https://docs.google.com/document/d/abcd1234/edit",
        documentType: "doc",
        title: "Roadmap (revised)",
      });

      expect(result.title).toBe("Roadmap (revised)");
      expect(received).toMatchObject({ document_type: "doc" });
    });

    it("throws not_found for a 404 response", async () => {
      server.use(
        http.put(`${BASE_URL}/google_documents/9999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 }),
        ),
      );

      await expect(
        service.updateGoogleDocument(9999, {
          url: "https://docs.google.com/document/d/abcd1234/edit",
          documentType: "doc",
        }),
      ).rejects.toThrow(BasecampError);
    });
  });
});
