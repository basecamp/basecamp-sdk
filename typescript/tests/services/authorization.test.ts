/**
 * Tests for the AuthorizationService
 */
import { describe, it, expect, beforeEach, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const LAUNCHPAD_URL = "https://launchpad.37signals.com/authorization.json";

const sampleAuthResponse = () => ({
  expires_at: "2024-03-01T12:00:00Z",
  identity: {
    id: 100,
    first_name: "Jane",
    last_name: "Doe",
    email_address: "jane@example.com",
  },
  accounts: [
    {
      id: 1,
      name: "Acme Corp",
      product: "bc3",
      href: "https://3.basecampapi.com/1",
      app_href: "https://3.basecamp.com/1",
    },
    {
      id: 2,
      name: "HEY Account",
      product: "hey",
      href: "https://3.basecampapi.com/2",
      app_href: "https://3.basecamp.com/2",
    },
  ],
});

/**
 * A BC5 (bc3) issuer's own authorization document, per
 * `app/views/api/authorizations/show.json.jbuilder`: identity id only, no
 * `product` or `app_href` on accounts, an RFC 8707 `resource` indicator instead,
 * a top-level `scope`, and `expires_at` as integer epoch *seconds*.
 */
const sampleBc5AuthResponse = () => ({
  identity: { id: 100 },
  accounts: [
    {
      id: 1,
      name: "Acme Corp",
      href: "https://bc5.example.com/1",
      resource: "urn:bc:account:1",
    },
    {
      id: 2,
      name: "Second Account",
      href: "https://bc5.example.com/2",
      resource: "urn:bc:account:2",
    },
  ],
  scope: "read write",
  // 2036-01-29T09:55:56Z as epoch seconds. Read as milliseconds: 1970-01-25.
  expires_at: 2085213356,
});

const BC5_URL = "https://bc5.example.com/authorization.json";

describe("AuthorizationService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("getInfo", () => {
    it("should return identity and accounts", async () => {
      server.use(
        http.get(LAUNCHPAD_URL, () => {
          return HttpResponse.json(sampleAuthResponse());
        })
      );

      const info = await client.authorization.getInfo();
      expect(info.identity.id).toBe(100);
      expect(info.identity.firstName).toBe("Jane");
      expect(info.identity.lastName).toBe("Doe");
      expect(info.identity.emailAddress).toBe("jane@example.com");
      expect(info.accounts).toHaveLength(2);
      expect(info.accounts[0]!.name).toBe("Acme Corp");
      expect(info.accounts[0]!.product).toBe("bc3");
      expect(info.expiresAt).toBeInstanceOf(Date);
    });

    it("should filter accounts by product", async () => {
      server.use(
        http.get(LAUNCHPAD_URL, () => {
          return HttpResponse.json(sampleAuthResponse());
        })
      );

      const info = await client.authorization.getInfo({ filterProduct: "bc3" });
      expect(info.accounts).toHaveLength(1);
      expect(info.accounts[0]!.product).toBe("bc3");
    });

    it("reports the filter as applied when the document carries products", async () => {
      server.use(http.get(LAUNCHPAD_URL, () => HttpResponse.json(sampleAuthResponse())));

      const info = await client.authorization.getInfo({ filterProduct: "bc3" });
      expect(info.productFilterApplied).toBe(true);
    });

    it("returns an empty list when a filterable document genuinely has no match", async () => {
      server.use(http.get(LAUNCHPAD_URL, () => HttpResponse.json(sampleAuthResponse())));

      const info = await client.authorization.getInfo({ filterProduct: "nope" });
      expect(info.accounts).toHaveLength(0);
      expect(info.productFilterApplied).toBe(true);
    });

    it("leaves productFilterApplied unset when no filter was requested", async () => {
      server.use(http.get(LAUNCHPAD_URL, () => HttpResponse.json(sampleAuthResponse())));

      const info = await client.authorization.getInfo();
      expect(info.productFilterApplied).toBeUndefined();
    });

    it("should throw on 401 error", async () => {
      server.use(
        http.get(LAUNCHPAD_URL, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      await expect(client.authorization.getInfo()).rejects.toThrow(BasecampError);
    });
  });

  // A BC5 issuer is reached exactly the way the docs say to reach one: by passing
  // `endpoint:`. Everything below is the shape that arrives when you do.
  describe("getInfo against a BC5 issuer's own document", () => {
    beforeEach(() => {
      server.use(http.get(BC5_URL, () => HttpResponse.json(sampleBc5AuthResponse())));
    });

    it("reads expires_at as epoch seconds, not milliseconds", async () => {
      const info = await client.authorization.getInfo({ endpoint: BC5_URL });

      // Read as milliseconds, 2085213356 lands in January 1970 — a wrong date
      // rather than an exception, which then reads as an expired credential.
      expect(info.expiresAt.getUTCFullYear()).toBe(2036);
      expect(info.expiresAt.toISOString()).toBe("2036-01-29T09:55:56.000Z");
    });

    it("does not empty the account list when no account carries a product", async () => {
      const info = await client.authorization.getInfo({
        endpoint: BC5_URL,
        filterProduct: "bc3",
      });

      // The filter cannot apply to a document with no `product` anywhere, so it
      // is reported inapplicable rather than silently matching nothing.
      expect(info.accounts).toHaveLength(2);
      expect(info.productFilterApplied).toBe(false);
      expect(info.accounts[0]!.href).toBe("https://bc5.example.com/1");
    });

    it("surfaces the resource indicator and scope, and omits Launchpad-only fields", async () => {
      const info = await client.authorization.getInfo({ endpoint: BC5_URL });

      expect(info.accounts[0]!.resource).toBe("urn:bc:account:1");
      expect(info.scope).toBe("read write");
      expect(info.identity.id).toBe(100);
      expect(info.identity.emailAddress).toBeUndefined();
      expect(info.accounts[0]!.product).toBeUndefined();
      expect(info.accounts[0]!.appHref).toBeUndefined();
    });
  });

  describe("getInfo endpoint validation", () => {
    it("rejects a non-HTTPS custom endpoint without sending the token", async () => {
      const originalFetch = globalThis.fetch;
      const fetchSpy = vi.fn();
      globalThis.fetch = fetchSpy as unknown as typeof globalThis.fetch;
      try {
        await expect(
          client.authorization.getInfo({ endpoint: "http://evil.example/authorization.json" })
        ).rejects.toThrow("HTTPS");
        expect(fetchSpy).not.toHaveBeenCalled();
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it("allows an HTTPS custom endpoint override", async () => {
      const customUrl = "https://custom.example.com/authorization.json";
      server.use(http.get(customUrl, () => HttpResponse.json(sampleAuthResponse())));
      const info = await client.authorization.getInfo({ endpoint: customUrl });
      expect(info.identity.id).toBe(100);
    });

    it("allows a localhost custom endpoint override", async () => {
      const localUrl = "http://localhost:3000/authorization.json";
      server.use(http.get(localUrl, () => HttpResponse.json(sampleAuthResponse())));
      const info = await client.authorization.getInfo({ endpoint: localUrl });
      expect(info.identity.id).toBe(100);
    });
  });
});
