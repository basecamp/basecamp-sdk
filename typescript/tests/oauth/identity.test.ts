/**
 * Tests for OAuth identity discovery.
 */

import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { discoverIdentity } from "../../src/oauth/identity.js";
import { BasecampError } from "../../src/errors.js";

const mockAuthorizationResponse = {
  expires_at: "2026-06-15T12:00:00Z",
  identity: {
    id: 12345,
    first_name: "Jane",
    last_name: "Doe",
    email_address: "jane@example.com",
  },
  accounts: [
    {
      id: 99999,
      name: "Acme Corp",
      product: "bc3",
      href: "https://3.basecampapi.com/99999",
      app_href: "https://3.basecamp.com/99999",
      hidden: false,
      expired: false,
      featured: true,
    },
    {
      id: 88888,
      name: "Side Project",
      product: "bc3",
      href: "https://3.basecampapi.com/88888",
      app_href: "https://3.basecamp.com/88888",
      hidden: false,
      expired: false,
    },
  ],
};

describe("discoverIdentity", () => {
  it("fetches identity and accounts from Launchpad", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test_token");
        return HttpResponse.json(mockAuthorizationResponse);
      })
    );

    const info = await discoverIdentity("test_token");

    expect(info.identity.id).toBe(12345);
    expect(info.identity.firstName).toBe("Jane");
    expect(info.identity.lastName).toBe("Doe");
    expect(info.identity.emailAddress).toBe("jane@example.com");
    expect(info.expiresAt).toBeInstanceOf(Date);
    expect(info.expiresAt.toISOString()).toBe("2026-06-15T12:00:00.000Z");
  });

  it("returns all accounts with correct field mapping", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", () =>
        HttpResponse.json(mockAuthorizationResponse)
      )
    );

    const info = await discoverIdentity("test_token");

    expect(info.accounts).toHaveLength(2);
    expect(info.accounts[0]).toEqual({
      id: 99999,
      name: "Acme Corp",
      product: "bc3",
      href: "https://3.basecampapi.com/99999",
      appHref: "https://3.basecamp.com/99999",
      hidden: false,
      expired: false,
      featured: true,
    });
    expect(info.accounts[1]!.id).toBe(88888);
  });

  it("accepts an async token provider function", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer async_token");
        return HttpResponse.json(mockAuthorizationResponse);
      })
    );

    const info = await discoverIdentity(async () => "async_token");

    expect(info.identity.id).toBe(12345);
  });

  it("throws BasecampError on 401 response", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", () =>
        HttpResponse.json({ error: "unauthorized" }, { status: 401 })
      )
    );

    try {
      await discoverIdentity("bad_token");
      expect.fail("Should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("auth");
      expect((err as BasecampError).httpStatus).toBe(401);
    }
  });

  it("wraps network errors in BasecampError", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", () =>
        HttpResponse.error()
      )
    );

    try {
      await discoverIdentity("test_token");
      expect.fail("Should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("network");
    }
  });

  it("throws BasecampError on server error", async () => {
    server.use(
      http.get("https://launchpad.37signals.com/authorization.json", () =>
        HttpResponse.text("Internal Server Error", { status: 500 })
      )
    );

    try {
      await discoverIdentity("test_token");
      expect.fail("Should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("api_error");
      expect((err as BasecampError).httpStatus).toBe(500);
    }
  });
});
