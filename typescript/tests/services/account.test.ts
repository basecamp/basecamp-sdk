/**
 * Tests for the AccountService — verifies Account.logo wire-shape deserialization.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("AccountService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  it("should not retry logo uploads when retries are disabled", async () => {
    let attempts = 0;
    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        attempts++;
        return attempts === 1
          ? HttpResponse.json({ error: "Unavailable" }, { status: 503, headers: { "Retry-After": "1" } })
          : new HttpResponse(null, { status: 204 });
      })
    );

    await expect(
      client.account.updateAccountLogo(new Blob(["data"]), "logo.png")
    ).rejects.toMatchObject({ httpStatus: 503 });
    expect(attempts).toBe(1);
  });

  describe("account", () => {
    it("should deserialize logo as an object with url", async () => {
      server.use(
        http.get(`${BASE_URL}/account.json`, () => {
          return HttpResponse.json({
            id: 3,
            name: "37signals",
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
            logo: { url: "https://3.basecampapi.com/2914079/account/logo?v=1650492527" },
          });
        })
      );

      const account = await client.account.account();
      expect(account.logo).toEqual({
        url: "https://3.basecampapi.com/2914079/account/logo?v=1650492527",
      });
    });

    it("should return undefined logo when absent", async () => {
      server.use(
        http.get(`${BASE_URL}/account.json`, () => {
          return HttpResponse.json({
            id: 3,
            name: "37signals",
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
          });
        })
      );

      const account = await client.account.account();
      expect(account.logo).toBeUndefined();
    });
  });
});
