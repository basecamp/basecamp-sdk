/**
 * Tests for the AuthorizationService
 */
import { describe, it, expect, beforeEach } from "vitest";
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

    it("should throw on 401 error", async () => {
      server.use(
        http.get(LAUNCHPAD_URL, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      await expect(client.authorization.getInfo()).rejects.toThrow(BasecampError);
    });
  });
});
