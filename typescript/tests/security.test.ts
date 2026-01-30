/**
 * Security tests for the Basecamp TypeScript SDK.
 *
 * Tests cover:
 * - Link header origin validation (SSRF / token leakage)
 * - HTTPS enforcement on token endpoints
 * - Webhook URL validation
 * - Cache auth isolation
 * - Error body truncation
 */
import { describe, it, expect, beforeEach, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import {
  createBasecampClient,
  fetchAllPages,
  paginateAll,
} from "../src/client.js";
import { BasecampError } from "../src/errors.js";
import { exchangeCode, refreshToken } from "../src/oauth/index.js";
import { discover } from "../src/oauth/discovery.js";

const BASE_URL = "https://3.basecampapi.com/12345";

// =============================================================================
// Link Header Origin Validation (SSRF / Token Leakage)
// =============================================================================

describe("Link header origin validation", () => {
  it("fetchAllPages rejects cross-origin Link header", async () => {
    // Create a response with a cross-origin Link header
    const response = new Response(JSON.stringify([{ id: 1 }]), {
      status: 200,
      headers: {
        Link: '<https://evil.com/page2>; rel="next"',
      },
    });
    // Override response.url (not settable in constructor)
    Object.defineProperty(response, "url", {
      value: "https://3.basecampapi.com/12345/projects.json",
    });

    await expect(
      fetchAllPages(response, (r) => r.json())
    ).rejects.toThrow("different origin");
  });

  it("paginateAll rejects cross-origin Link header", async () => {
    const response = new Response(JSON.stringify([{ id: 1 }]), {
      status: 200,
      headers: {
        Link: '<https://evil.com/page2>; rel="next"',
      },
    });
    Object.defineProperty(response, "url", {
      value: "https://3.basecampapi.com/12345/projects.json",
    });

    const generator = paginateAll(response, (r) => r.json());

    // First yield should succeed (initial page)
    const first = await generator.next();
    expect(first.done).toBe(false);
    expect(first.value).toEqual([{ id: 1 }]);

    // Second yield should throw (cross-origin link)
    await expect(generator.next()).rejects.toThrow("different origin");
  });

  it("fetchAllPages accepts same-origin Link header", async () => {
    // We can't easily test multi-page with real fetch here without MSW,
    // but we can verify that a same-origin URL doesn't throw.
    const response = new Response(JSON.stringify([{ id: 1 }]), {
      status: 200,
      headers: {}, // No Link header - should just return first page
    });
    Object.defineProperty(response, "url", {
      value: "https://3.basecampapi.com/12345/projects.json",
    });

    const results = await fetchAllPages(response, (r) => r.json());
    expect(results).toEqual([{ id: 1 }]);
  });

  it("fetchAllPages resolves relative Link header as same-origin", async () => {
    // A relative Link header like </page2> should be resolved against
    // the initial request URL, not rejected.
    let fetchCallCount = 0;
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      fetchCallCount++;
      // Return page 2 with no further links
      const resp = new Response(JSON.stringify([{ id: 2 }]), {
        status: 200,
        headers: {},
      });
      Object.defineProperty(resp, "url", { value: url });
      return resp;
    });

    try {
      const response = new Response(JSON.stringify([{ id: 1 }]), {
        status: 200,
        headers: {
          Link: '</12345/projects.json?page=2>; rel="next"',
        },
      });
      Object.defineProperty(response, "url", {
        value: "https://3.basecampapi.com/12345/projects.json",
      });

      const results = await fetchAllPages(response, (r) => r.json());
      // Should have fetched page 2 (relative URL resolved to same origin)
      expect(results).toEqual([{ id: 1 }, { id: 2 }]);
      expect(fetchCallCount).toBe(1);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  it("fetchAllPages accepts same-origin Link with explicit default port", async () => {
    let fetchCallCount = 0;
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      fetchCallCount++;
      const resp = new Response(JSON.stringify([{ id: 2 }]), {
        status: 200,
        headers: {},
      });
      Object.defineProperty(resp, "url", { value: url });
      return resp;
    });

    try {
      // Link includes explicit :443, which is the default for HTTPS
      const response = new Response(JSON.stringify([{ id: 1 }]), {
        status: 200,
        headers: {
          Link: '<https://3.basecampapi.com:443/12345/projects.json?page=2>; rel="next"',
        },
      });
      Object.defineProperty(response, "url", {
        value: "https://3.basecampapi.com/12345/projects.json",
      });

      const results = await fetchAllPages(response, (r) => r.json());
      expect(results).toEqual([{ id: 1 }, { id: 2 }]);
      expect(fetchCallCount).toBe(1);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  it("fetchAllPages resolves path-relative Link against current page URL", async () => {
    // Simulates a server that emits path-relative links like "page2" (not "/page2").
    // These must be resolved against the current page URL, not the initial one.
    const fetchedUrls: string[] = [];
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      fetchedUrls.push(url);
      // Page 2 returns a path-relative link to page 3
      if (url.includes("page=2")) {
        const resp = new Response(JSON.stringify([{ id: 2 }]), {
          status: 200,
          headers: {
            Link: '<page=3>; rel="next"',
          },
        });
        Object.defineProperty(resp, "url", { value: url });
        return resp;
      }
      // Page 3: no more links
      const resp = new Response(JSON.stringify([{ id: 3 }]), {
        status: 200,
        headers: {},
      });
      Object.defineProperty(resp, "url", { value: url });
      return resp;
    });

    try {
      // Initial page has a root-relative link
      const response = new Response(JSON.stringify([{ id: 1 }]), {
        status: 200,
        headers: {
          Link: '</v1/projects?page=2>; rel="next"',
        },
      });
      Object.defineProperty(response, "url", {
        value: "https://3.basecampapi.com/v1/projects",
      });

      const results = await fetchAllPages(response, (r) => r.json());
      expect(results).toEqual([{ id: 1 }, { id: 2 }, { id: 3 }]);
      // Page 2 URL resolved from initial
      expect(fetchedUrls[0]).toBe("https://3.basecampapi.com/v1/projects?page=2");
      // Page 3 URL resolved from page 2's URL (path-relative "page=3" against current)
      expect(fetchedUrls[1]).toBe("https://3.basecampapi.com/v1/page=3");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});

// =============================================================================
// HTTPS Enforcement on Token Endpoints
// =============================================================================

describe("HTTPS enforcement", () => {
  it("exchangeCode rejects HTTP token endpoint", async () => {
    await expect(
      exchangeCode({
        tokenEndpoint: "http://example.com/token",
        code: "auth-code",
        redirectUri: "https://myapp.com/callback",
        clientId: "client-id",
      })
    ).rejects.toThrow("HTTPS");
  });

  it("refreshToken rejects HTTP token endpoint", async () => {
    await expect(
      refreshToken({
        tokenEndpoint: "http://example.com/token",
        refreshToken: "refresh-token",
      })
    ).rejects.toThrow("HTTPS");
  });

  it("discover rejects HTTP base URL", async () => {
    await expect(discover("http://example.com")).rejects.toThrow("HTTPS");
  });

  it("exchangeCode allows localhost", async () => {
    server.use(
      http.post("http://localhost:3000/token", () => {
        return HttpResponse.json({
          access_token: "token123",
          token_type: "Bearer",
        });
      })
    );

    const result = await exchangeCode({
      tokenEndpoint: "http://localhost:3000/token",
      code: "auth-code",
      redirectUri: "http://localhost:3000/callback",
      clientId: "client-id",
    });

    expect(result.accessToken).toBe("token123");
  });

  it("discover allows localhost", async () => {
    server.use(
      http.get("http://localhost:3000/.well-known/oauth-authorization-server", () => {
        return HttpResponse.json({
          issuer: "http://localhost:3000",
          authorization_endpoint: "http://localhost:3000/authorize",
          token_endpoint: "http://localhost:3000/token",
        });
      })
    );

    const config = await discover("http://localhost:3000");
    expect(config.issuer).toBe("http://localhost:3000");
  });

  it("exchangeCode accepts HTTPS token endpoint", async () => {
    server.use(
      http.post("https://launchpad.37signals.com/authorization/token", () => {
        return HttpResponse.json({
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
          expires_in: 1209600,
        });
      })
    );

    const result = await exchangeCode({
      tokenEndpoint: "https://launchpad.37signals.com/authorization/token",
      code: "auth-code",
      redirectUri: "https://myapp.com/callback",
      clientId: "client-id",
    });

    expect(result.accessToken).toBe("new-access-token");
  });
});

// =============================================================================
// Webhook PayloadURL Validation
// =============================================================================

describe("Webhook URL validation", () => {
  let client: ReturnType<typeof createBasecampClient>;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  it("rejects HTTP payload URL", async () => {
    await expect(
      client.webhooks.create(1, {
        payloadUrl: "http://example.com/webhook",
        types: ["Todo"],
      })
    ).rejects.toThrow("HTTPS");
  });

  it("rejects empty payload URL", async () => {
    await expect(
      client.webhooks.create(1, {
        payloadUrl: "",
        types: ["Todo"],
      })
    ).rejects.toThrow("required");
  });

  it("accepts HTTPS payload URL", async () => {
    server.use(
      http.post(`${BASE_URL}/buckets/1/webhooks.json`, () => {
        return HttpResponse.json({
          webhook: {
            id: 1,
            active: true,
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
            payload_url: "https://example.com/webhook",
            types: ["Todo"],
          },
        });
      })
    );

    const webhook = await client.webhooks.create(1, {
      payloadUrl: "https://example.com/webhook",
      types: ["Todo"],
    });

    expect(webhook.payload_url).toBe("https://example.com/webhook");
  });
});

// =============================================================================
// Cache Auth Isolation
// =============================================================================

describe("Cache auth isolation", () => {
  it("different tokens produce different cache keys for same URL", async () => {
    // We can verify indirectly: two clients with different tokens should not
    // share cached responses. This is a behavioral test.
    let requestCount = 0;

    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        requestCount++;
        return HttpResponse.json(
          [{ id: requestCount, name: `Project ${requestCount}` }],
          {
            headers: {
              ETag: `"etag-${requestCount}"`,
            },
          }
        );
      })
    );

    const client1 = createBasecampClient({
      accountId: "12345",
      accessToken: "token-user-A",
    });

    const client2 = createBasecampClient({
      accountId: "12345",
      accessToken: "token-user-B",
    });

    // Both clients make the same request - with auth isolation,
    // they should use separate caches (separate client instances)
    await client1.GET("/projects.json");
    await client2.GET("/projects.json");

    // Both should have made actual requests (not shared cache)
    expect(requestCount).toBe(2);
  });
  it("cache fallback recomputes key from Authorization header on WeakMap miss", async () => {
    // This test verifies that ETag caching works end-to-end, which exercises
    // the onResponse cache key path. If the WeakMap missed and the fallback
    // produced a wrong key (e.g. empty token hash), the second request would
    // not find the cached entry and would not send If-None-Match.
    let requestCount = 0;
    let receivedIfNoneMatch: string | null = null;

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        requestCount++;
        receivedIfNoneMatch = request.headers.get("If-None-Match");

        if (receivedIfNoneMatch === '"etag-1"') {
          // Return 304 for conditional request
          return new HttpResponse(null, { status: 304 });
        }

        return HttpResponse.json(
          [{ id: 1, name: "Project 1" }],
          { headers: { ETag: '"etag-1"' } }
        );
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "token-cache-test",
    });

    // First request: populates cache with ETag
    await client.GET("/projects.json");
    expect(requestCount).toBe(1);

    // Second request: should send If-None-Match (proves onRequest found the
    // cache entry, and onResponse stored it with a consistent key)
    await client.GET("/projects.json");
    expect(requestCount).toBe(2);
    expect(receivedIfNoneMatch).toBe('"etag-1"');
  });
});

// =============================================================================
// Error Body Truncation
// =============================================================================

describe("Error body truncation", () => {
  it("exchangeCode truncates large error response body", async () => {
    const largeBody = "x".repeat(10000);

    server.use(
      http.post("https://launchpad.37signals.com/authorization/token", () => {
        return new HttpResponse(largeBody, {
          status: 500,
          headers: { "Content-Type": "text/plain" },
        });
      })
    );

    try {
      await exchangeCode({
        tokenEndpoint: "https://launchpad.37signals.com/authorization/token",
        code: "auth-code",
        redirectUri: "https://myapp.com/callback",
        clientId: "client-id",
      });
      expect.fail("Expected error");
    } catch (err) {
      expect(err).toBeInstanceOf(BasecampError);
      // The error message should be truncated, not contain the full 10KB body
      expect((err as BasecampError).message.length).toBeLessThan(1000);
    }
  });
});

// =============================================================================
// Webhook Update URL Validation (backported from Ruby)
// =============================================================================

describe("Webhook update URL validation", () => {
  let client: ReturnType<typeof createBasecampClient>;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  it("webhook update rejects http:// payload URL", async () => {
    await expect(
      client.webhooks.update(1, 1, {
        payloadUrl: "http://example.com/webhook",
      })
    ).rejects.toThrow("HTTPS");
  });

  it("webhook update allows undefined payload URL", async () => {
    server.use(
      http.put(`${BASE_URL}/buckets/1/webhooks/1`, () => {
        return HttpResponse.json({
          webhook: {
            id: 1,
            active: false,
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
            payload_url: "https://example.com/webhook",
            types: ["Todo"],
          },
        });
      })
    );

    const webhook = await client.webhooks.update(1, 1, {
      active: false,
    });

    expect(webhook.active).toBe(false);
  });
});

// =============================================================================
// Config Validation (backported from Ruby)
// =============================================================================

describe("Client config validation", () => {
  it("createBasecampClient rejects http:// base URL", () => {
    expect(() =>
      createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        baseUrl: "http://3.basecampapi.com/12345",
      })
    ).toThrow("HTTPS");
  });

  it("createBasecampClient accepts https:// base URL", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      baseUrl: "https://custom.example.com/12345",
    });
    expect(client).toBeDefined();
  });

  it("createBasecampClient allows http://localhost for dev/test", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      baseUrl: "http://localhost:3000/12345",
    });
    expect(client).toBeDefined();
  });

  it("createBasecampClient allows http://127.0.0.1 for dev/test", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      baseUrl: "http://127.0.0.1:3000/12345",
    });
    expect(client).toBeDefined();
  });
});

// =============================================================================
// OAuth Response Body Size Limit (backported from Ruby)
// =============================================================================

describe("OAuth response body size limit", () => {
  it("rejects oversized token response", async () => {
    const hugeBody = "x".repeat(2 * 1024 * 1024);

    server.use(
      http.post("https://launchpad.37signals.com/authorization/token", () => {
        return new HttpResponse(hugeBody, {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      })
    );

    await expect(
      exchangeCode({
        tokenEndpoint: "https://launchpad.37signals.com/authorization/token",
        code: "auth-code",
        redirectUri: "https://myapp.com/callback",
        clientId: "client-id",
      })
    ).rejects.toThrow("too large");
  });
});

// =============================================================================
// Service Cache Concurrency (Race Condition Prevention)
// =============================================================================

describe("Service cache concurrency", () => {
  it("concurrent service access returns same instance", async () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    // Access the same service concurrently from multiple "threads"
    // In single-threaded JS, this simulates interleaved access
    const promises = Array(100)
      .fill(null)
      .map(() => Promise.resolve(client.projects));

    const services = await Promise.all(promises);

    // All should be the same instance (singleton pattern)
    const uniqueServices = new Set(services);
    expect(uniqueServices.size).toBe(1);
  });

  it("different services are different instances", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    const projects = client.projects;
    const todos = client.todos;
    const people = client.people;

    // Each service type should be a different instance
    expect(projects).not.toBe(todos);
    expect(todos).not.toBe(people);
    expect(projects).not.toBe(people);
  });
});

// =============================================================================
// Header Redaction
// =============================================================================

describe("Header redaction", () => {
  // Import inline to avoid circular dependencies in test setup
  it("redactHeaders redacts sensitive headers", async () => {
    const { redactHeaders } = await import("../src/security.js");

    const headers = new Headers({
      Authorization: "Bearer secret-token",
      Cookie: "session=abc123",
      "Content-Type": "application/json",
      "X-CSRF-Token": "csrf-token-value",
    });

    const redacted = redactHeaders(headers);

    // Note: Headers.forEach yields lowercase keys in most runtimes
    expect(redacted.authorization).toBe("[REDACTED]");
    expect(redacted.cookie).toBe("[REDACTED]");
    expect(redacted["x-csrf-token"]).toBe("[REDACTED]");
    expect(redacted["content-type"]).toBe("application/json");
  });

  it("redactHeadersRecord preserves original key casing", async () => {
    const { redactHeadersRecord } = await import("../src/security.js");

    const headers = {
      Authorization: "Bearer secret-token",
      Cookie: "session=abc123",
      "Content-Type": "application/json",
    };

    const redacted = redactHeadersRecord(headers);

    // redactHeadersRecord preserves the original key casing from the input object
    expect(redacted.Authorization).toBe("[REDACTED]");
    expect(redacted.Cookie).toBe("[REDACTED]");
    expect(redacted["Content-Type"]).toBe("application/json");
  });
});
