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
import {
  requireSameOrigin,
  isSameOriginAllowingLocalhost,
  requireSecureEndpoint,
} from "../src/security.js";
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
// Pagination Page Cap (unbounded Link-following)
// =============================================================================

/**
 * `fetchAllPages` and `paginateAll` followed rel="next" under `while (true)`.
 * Same-origin validation bounds WHERE the loop can go, not how long it runs:
 * a Link header naming the page it was served from is same-origin and passes,
 * so "until no more pages exist" is never true and the call never returns.
 * Every other pagination loop in this SDK, and in the other five, carries a
 * page cap; these two — the exported helpers — did not.
 *
 * Every case below pins the EXACT number of fetches, not just that the call
 * returned. Termination on its own is too weak an assertion: the cap is applied
 * between consuming a page and reading its Link header, and a version that
 * tested it only in the `for` condition also terminates — while issuing one
 * extra request per call, to a URL taken from an attacker-influenceable header,
 * whose response is then discarded. `maxPages: 1` is where that off-by-one is
 * loudest: the initial page is handed in, so the correct number of fetches is
 * zero.
 *
 * The always-next mocks yield to the macrotask queue via `setTimeout(0)` so
 * that a regression to an unbounded loop fails on the suite timeout instead of
 * starving the event loop in microtasks, where no timer can interrupt it.
 */
describe("pagination page cap", () => {
  /** A page that always advertises another page after it. */
  function endlessPage(url: string, id: number, nextUrl: string): Response {
    const resp = new Response(JSON.stringify([{ id }]), {
      status: 200,
      headers: { Link: `<${nextUrl}>; rel="next"` },
    });
    Object.defineProperty(resp, "url", { value: url });
    return resp;
  }

  /** A terminal page: no Link header, so the natural end of a sequence. */
  function finalPage(url: string, id: number): Response {
    const resp = new Response(JSON.stringify([{ id }]), { status: 200 });
    Object.defineProperty(resp, "url", { value: url });
    return resp;
  }

  const firstOfEndless = () =>
    endlessPage(`${BASE_URL}/projects.json`, 1, `${BASE_URL}/projects.json?page=2`);

  /** Installs a fetch that never stops advertising a next page. */
  function installEndlessFetch(): { count: () => number; restore: () => void } {
    let fetchCallCount = 0;
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      fetchCallCount++;
      await new Promise((resolve) => setTimeout(resolve, 0));
      const page = fetchCallCount + 1;
      return endlessPage(url, page, `${BASE_URL}/projects.json?page=${page + 1}`);
    });
    return {
      count: () => fetchCallCount,
      restore: () => {
        globalThis.fetch = originalFetch;
      },
    };
  }

  describe("fetchAllPages", () => {
    it("makes no further request at all when maxPages is 1", async () => {
      const mock = installEndlessFetch();
      try {
        const results = await fetchAllPages(firstOfEndless(), (r) => r.json(), undefined, 1);

        expect(results).toEqual([{ id: 1 }]);
        // The initial response was supplied by the caller. One page consumed
        // means zero pages fetched — anything else is a request whose body is
        // thrown away.
        expect(mock.count()).toBe(0);
      } finally {
        mock.restore();
      }
    });

    it("consumes exactly maxPages pages against a server that never stops", async () => {
      const mock = installEndlessFetch();
      try {
        const results = await fetchAllPages(firstOfEndless(), (r) => r.json(), undefined, 3);

        expect(results).toEqual([{ id: 1 }, { id: 2 }, { id: 3 }]);
        expect(mock.count()).toBe(2);
      } finally {
        mock.restore();
      }
    });

    it("terminates on a Link header pointing at its own page", async () => {
      // The motivating case. Self-referential, same-origin, indistinguishable
      // from a legitimate link — only the cap ends it.
      const selfUrl = `${BASE_URL}/projects.json`;
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        await new Promise((resolve) => setTimeout(resolve, 0));
        expect(url).toBe(selfUrl);
        return endlessPage(selfUrl, 1, selfUrl);
      });

      try {
        const results = await fetchAllPages(
          endlessPage(selfUrl, 1, selfUrl),
          (r) => r.json(),
          undefined,
          3
        );

        expect(results).toEqual([{ id: 1 }, { id: 1 }, { id: 1 }]);
        expect(fetchCallCount).toBe(2);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it("stops at the natural end rather than at the cap", async () => {
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        return finalPage(url, 2);
      });

      try {
        // Generous cap, two-page sequence: the cap must not be what ends it.
        const results = await fetchAllPages(firstOfEndless(), (r) => r.json(), undefined, 100);

        expect(results).toEqual([{ id: 1 }, { id: 2 }]);
        expect(fetchCallCount).toBe(1);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it("stops at the natural end under the default cap", async () => {
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        return finalPage(url, 2);
      });

      try {
        const results = await fetchAllPages(firstOfEndless(), (r) => r.json());

        expect(results).toEqual([{ id: 1 }, { id: 2 }]);
        expect(fetchCallCount).toBe(1);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });
  });

  describe("paginateAll", () => {
    async function collect<T>(gen: AsyncGenerator<T[], void, unknown>): Promise<T[][]> {
      const pages: T[][] = [];
      for await (const page of gen) {
        pages.push(page);
      }
      return pages;
    }

    it("makes no further request at all when maxPages is 1", async () => {
      const mock = installEndlessFetch();
      try {
        const pages = await collect(paginateAll(firstOfEndless(), (r) => r.json(), undefined, 1));

        expect(pages).toEqual([[{ id: 1 }]]);
        expect(mock.count()).toBe(0);
      } finally {
        mock.restore();
      }
    });

    it("yields exactly maxPages pages against a server that never stops", async () => {
      const mock = installEndlessFetch();
      try {
        const pages = await collect(paginateAll(firstOfEndless(), (r) => r.json(), undefined, 3));

        expect(pages).toEqual([[{ id: 1 }], [{ id: 2 }], [{ id: 3 }]]);
        expect(mock.count()).toBe(2);
      } finally {
        mock.restore();
      }
    });

    it("terminates on a Link header pointing at its own page", async () => {
      const selfUrl = `${BASE_URL}/projects.json`;
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        await new Promise((resolve) => setTimeout(resolve, 0));
        expect(url).toBe(selfUrl);
        return endlessPage(selfUrl, 1, selfUrl);
      });

      try {
        const pages = await collect(
          paginateAll(endlessPage(selfUrl, 1, selfUrl), (r) => r.json(), undefined, 3)
        );

        expect(pages).toEqual([[{ id: 1 }], [{ id: 1 }], [{ id: 1 }]]);
        expect(fetchCallCount).toBe(2);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it("stops at the natural end rather than at the cap", async () => {
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        return finalPage(url, 2);
      });

      try {
        const pages = await collect(paginateAll(firstOfEndless(), (r) => r.json(), undefined, 100));

        expect(pages).toEqual([[{ id: 1 }], [{ id: 2 }]]);
        expect(fetchCallCount).toBe(1);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it("stops at the natural end under the default cap", async () => {
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        return finalPage(url, 2);
      });

      try {
        const pages = await collect(paginateAll(firstOfEndless(), (r) => r.json()));

        expect(pages).toEqual([[{ id: 1 }], [{ id: 2 }]]);
        expect(fetchCallCount).toBe(1);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });
  });

  // A cap is only a cap if the value is one. Every rejected value below breaks
  // the bound in a different direction, and each did so silently before
  // validation existed:
  //
  //   Infinity   — `page === maxPages` is never true, so the loop is unbounded
  //                and the cap does exactly nothing. The failure mode the cap
  //                was added to prevent, re-entered through the front door.
  //   2.5        — consumes 2 pages, then fetches a 3rd and discards it. That
  //                is the off-by-one this commit removed, resurfacing for
  //                non-integers, and it issues a request to a URL taken from
  //                an attacker-influenceable header.
  //   0, -1, NaN — consume ZERO pages, silently discarding a response the
  //                caller already fetched and handed in.
  //   MAX_VALUE   — unbounded again, and the reason the predicate is
  //                `isSafeInteger` and not `isInteger`:
  //                `Number.isInteger(Number.MAX_VALUE)` is `true`, so the
  //                obvious check lets it through. Past `2 ** 53` the counter
  //                stalls — `page++` on `2 ** 53` yields `2 ** 53` again,
  //                because the next integer is not representable — so a bound
  //                of `2 ** 53 + 2` is never reached.
  //   2**53       — terminates, in fact: the counter arrives from
  //                `2 ** 53 - 1` and breaks on equality before the next
  //                increment. It is rejected anyway, because
  //                MAX_SAFE_INTEGER is the honest edge of the guarantee that
  //                the counter can arrive at the bound at all. Deliberately
  //                conservative, and listed here so nobody "fixes" the
  //                predicate back to isInteger on the strength of this case.
  //
  // `Number.isSafeInteger(n) && n > 0` rejects all seven in one predicate.
  // SPEC.md §2 step 5: "Validate `max_pages > 0`. → `⊥ BasecampError(code:
  // "usage")` otherwise."
  describe("maxPages validation", () => {
    async function collect<T>(gen: AsyncGenerator<T[], void, unknown>): Promise<T[][]> {
      const pages: T[][] = [];
      for await (const page of gen) {
        pages.push(page);
      }
      return pages;
    }

    const INVALID: ReadonlyArray<[string, number]> = [
      ["zero", 0],
      ["negative", -1],
      ["NaN", NaN],
      ["Infinity", Infinity],
      ["a non-integer", 2.5],
      ["Number.MAX_VALUE", Number.MAX_VALUE],
      ["an unsafe integer", 2 ** 53],
    ];

    /**
     * Counts fetches without answering any. Validation has to happen before a
     * single request goes out — a throw that lands after one would be a
     * different bug wearing the same error, so every case below asserts zero.
     */
    function installCountingFetch(): { count: () => number; restore: () => void } {
      let fetchCallCount = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
        fetchCallCount++;
        return finalPage(url, 2);
      });
      return {
        count: () => fetchCallCount,
        restore: () => {
          globalThis.fetch = originalFetch;
        },
      };
    }

    describe("fetchAllPages", () => {
      it.each(INVALID)("rejects %s with a usage error and fetches nothing", async (_label, value) => {
        const mock = installCountingFetch();
        try {
          const error = await fetchAllPages(firstOfEndless(), (r) => r.json(), undefined, value)
            .then(() => null)
            .catch((e: unknown) => e);

          expect(error).toBeInstanceOf(BasecampError);
          expect((error as BasecampError).code).toBe("usage");
          // The whole sentence, not just the offending value: `toContain` on
          // the value alone is satisfied by any usage error that happens to
          // mention it, and would not notice the bound itself changing.
          expect((error as BasecampError).message).toBe(
            `maxPages must be a positive integer no larger than ${Number.MAX_SAFE_INTEGER}, got ${String(value)}`
          );
          expect(mock.count()).toBe(0);
        } finally {
          mock.restore();
        }
      });
    });

    describe("paginateAll", () => {
      // Asserted on the CALL, not on the first `.next()`. paginateAll validates
      // eagerly — it is a plain function that checks the cap and then returns
      // the generator, rather than an `async function*` whose body would not
      // run until something iterated it. A usage error is a programmer error,
      // and it should be raised where the programmer wrote the mistake, not
      // wherever the generator happens to be consumed later. This assertion is
      // what pins that choice: under lazy validation `paginateAll(...)` returns
      // a generator without throwing and `expect(...).toThrow` fails.
      it.each(INVALID)("rejects %s eagerly and fetches nothing", (_label, value) => {
        const mock = installCountingFetch();
        try {
          expect(() => paginateAll(firstOfEndless(), (r) => r.json(), undefined, value)).toThrow(
            BasecampError
          );

          let thrown: unknown;
          try {
            paginateAll(firstOfEndless(), (r) => r.json(), undefined, value);
          } catch (e: unknown) {
            thrown = e;
          }
          expect((thrown as BasecampError).code).toBe("usage");
          expect((thrown as BasecampError).message).toBe(
            `maxPages must be a positive integer no larger than ${Number.MAX_SAFE_INTEGER}, got ${String(value)}`
          );
          expect(mock.count()).toBe(0);
        } finally {
          mock.restore();
        }
      });
    });

    // The predicate must not be so eager it rejects the caps the SDK itself
    // uses. DEFAULT_MAX_PAGES is covered by the "under the default cap" tests
    // above; these pin the explicit end of the range.
    it.each([1, 2, 3, 100, 10_000])("accepts the valid cap %i", async (value) => {
      const mock = installCountingFetch();
      try {
        const results = await fetchAllPages(firstOfEndless(), (r) => r.json(), undefined, value);
        const pages = await collect(paginateAll(firstOfEndless(), (r) => r.json(), undefined, value));

        // A two-page sequence: page 1 is supplied, page 2 is terminal.
        const expected = value === 1 ? [{ id: 1 }] : [{ id: 1 }, { id: 2 }];
        expect(results).toEqual(expected);
        expect(pages).toEqual(expected.map((item) => [item]));
        expect(mock.count()).toBe(value === 1 ? 0 : 2);
      } finally {
        mock.restore();
      }
    });
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

  it("exchangeCode allows .localhost TLD (RFC 6761)", async () => {
    server.use(
      http.post("http://myapp.localhost:3000/token", () => {
        return HttpResponse.json({
          access_token: "token-localhost-tld",
          token_type: "Bearer",
        });
      })
    );

    const result = await exchangeCode({
      tokenEndpoint: "http://myapp.localhost:3000/token",
      code: "auth-code",
      redirectUri: "http://myapp.localhost:3000/callback",
      clientId: "client-id",
    });

    expect(result.accessToken).toBe("token-localhost-tld");
  });

  it("discover allows .localhost TLD (RFC 6761)", async () => {
    server.use(
      http.get("http://myapp.localhost:3000/.well-known/oauth-authorization-server", () => {
        return HttpResponse.json({
          issuer: "http://myapp.localhost:3000",
          authorization_endpoint: "http://myapp.localhost:3000/authorize",
          token_endpoint: "http://myapp.localhost:3000/token",
        });
      })
    );

    const config = await discover("http://myapp.localhost:3000");
    expect(config.issuer).toBe("http://myapp.localhost:3000");
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

  // Note: Generated services delegate URL validation to the API server.
  // Client-side validation was removed when migrating to generated services.

  it("sends HTTP payload URL to API (server validates)", async () => {
    server.use(
      http.post(`${BASE_URL}/buckets/1/webhooks.json`, () => {
        return HttpResponse.json({ error: "payload_url must use HTTPS" }, { status: 422 });
      })
    );

    await expect(
      client.webhooks.create(1, {
        payloadUrl: "http://example.com/webhook",
        types: ["Todo"],
      })
    ).rejects.toThrow();
  });

  it("sends empty payload URL to API (server validates)", async () => {
    server.use(
      http.post(`${BASE_URL}/buckets/1/webhooks.json`, () => {
        return HttpResponse.json({ error: "payload_url is required" }, { status: 422 });
      })
    );

    await expect(
      client.webhooks.create(1, {
        payloadUrl: "",
        types: ["Todo"],
      })
    ).rejects.toThrow();
  });

  it("accepts HTTPS payload URL", async () => {
    server.use(
      http.post(`${BASE_URL}/buckets/1/webhooks.json`, () => {
        return HttpResponse.json({
          id: 1,
          active: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          payload_url: "https://example.com/webhook",
          types: ["Todo"],
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
      enableCache: true,
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

  // Note: Generated services delegate URL validation to the API server.

  it("webhook update sends http:// payload URL to API (server validates)", async () => {
    server.use(
      http.put(`${BASE_URL}/webhooks/1`, () => {
        return HttpResponse.json({ error: "payload_url must use HTTPS" }, { status: 422 });
      })
    );

    await expect(
      client.webhooks.update(1, {
        payloadUrl: "http://example.com/webhook",
      })
    ).rejects.toThrow();
  });

  it("webhook update allows undefined payload URL", async () => {
    server.use(
      http.put(`${BASE_URL}/webhooks/1`, () => {
        return HttpResponse.json({
          id: 1,
          active: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          payload_url: "https://example.com/webhook",
          types: ["Todo"],
        });
      })
    );

    const webhook = await client.webhooks.update(1, {
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

  it("createBasecampClient allows .localhost TLD (RFC 6761)", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      baseUrl: "http://myapp.localhost:3000/12345",
    });
    expect(client).toBeDefined();
  });

  it("createBasecampClient allows nested .localhost subdomains", () => {
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      baseUrl: "http://api.myapp.localhost:3000/12345",
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
// isLocalhost Function
// =============================================================================

describe("isLocalhost", () => {
  it("returns true for 'localhost'", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("localhost")).toBe(true);
  });

  it("returns true for '127.0.0.1'", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("127.0.0.1")).toBe(true);
  });

  it("returns true for '::1' (IPv6 loopback)", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("::1")).toBe(true);
  });

  it("returns true for bracketed IPv6 loopback '[::1]' (as URL.hostname returns it)", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("[::1]")).toBe(true);
    // URL.hostname brackets IPv6 literals, so this is the value real callers pass.
    expect(isLocalhost(new URL("http://[::1]:8080/path").hostname)).toBe(true);
  });

  it("returns false for non-loopback bracketed IPv6", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("[2001:db8::1]")).toBe(false);
  });

  it("returns true for .localhost TLD (RFC 6761)", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("myapp.localhost")).toBe(true);
    expect(isLocalhost("api.localhost")).toBe(true);
    expect(isLocalhost("dev.api.localhost")).toBe(true);
  });

  it("returns false for external hosts", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("example.com")).toBe(false);
    expect(isLocalhost("api.example.com")).toBe(false);
    expect(isLocalhost("3.basecampapi.com")).toBe(false);
  });

  it("returns false for hosts that contain but don't end with localhost", async () => {
    const { isLocalhost } = await import("../src/security.js");
    expect(isLocalhost("localhost.example.com")).toBe(false);
    expect(isLocalhost("notlocalhost")).toBe(false);
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


// =============================================================================
// Same-Origin Credential Attachment (initial, caller-influenced request)
// =============================================================================

describe("Same-origin credential attachment", () => {
  const base = "https://3.basecampapi.com/12345";

  it("requireSameOrigin accepts a same-origin URL", () => {
    expect(() =>
      requireSameOrigin("https://3.basecampapi.com/12345/projects.json", base)
    ).not.toThrow();
  });

  it("requireSameOrigin accepts a localhost URL (dev carve-out)", () => {
    expect(() =>
      requireSameOrigin("http://localhost:3000/projects.json", base)
    ).not.toThrow();
  });

  it("requireSameOrigin rejects a foreign-origin URL", () => {
    expect(() =>
      requireSameOrigin("https://evil.example/steal.json", base)
    ).toThrow("different origin than the configured base URL");
  });

  it("requireSameOrigin throws a validation BasecampError", () => {
    let caught: unknown;
    try {
      requireSameOrigin("https://evil.example/steal.json", base);
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(BasecampError);
    expect((caught as BasecampError).code).toBe("validation");
  });

  it("isSameOriginAllowingLocalhost honors the localhost carve-out", () => {
    expect(isSameOriginAllowingLocalhost("https://3.basecampapi.com/x", base)).toBe(true);
    expect(isSameOriginAllowingLocalhost("http://127.0.0.1:9999/x", base)).toBe(true);
    expect(isSameOriginAllowingLocalhost("https://evil.example/x", base)).toBe(false);
  });

  it("isSameOriginAllowingLocalhost recognizes IPv6 loopback [::1]", () => {
    // URL.hostname brackets IPv6 literals; the carve-out must still match.
    expect(isSameOriginAllowingLocalhost("http://[::1]:8080/x", base)).toBe(true);
  });

  it("isSameOriginAllowingLocalhost limits the localhost carve-out to HTTP(S)", () => {
    // Credentials must fail closed on non-HTTP(S) schemes even for localhost.
    expect(isSameOriginAllowingLocalhost("ws://localhost:3000/x", base)).toBe(false);
    expect(isSameOriginAllowingLocalhost("ftp://localhost/x", base)).toBe(false);
  });

  it("guard errors truncate the caller-supplied URL", () => {
    const huge = "https://evil.example/" + "a".repeat(10_000);
    let caught: unknown;
    try {
      requireSameOrigin(huge, base);
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(BasecampError);
    expect((caught as BasecampError).message.length).toBeLessThan(700);
  });

  it("requireSecureEndpoint allows HTTPS anywhere and HTTP only for localhost", () => {
    expect(() => requireSecureEndpoint("https://launchpad.37signals.com/authorization.json", "endpoint")).not.toThrow();
    expect(() => requireSecureEndpoint("http://localhost:3000/authorization.json", "endpoint")).not.toThrow();
    expect(() => requireSecureEndpoint("http://evil.example/authorization.json", "endpoint")).toThrow("must use HTTPS");
    // Non-HTTP(S) schemes are rejected even for localhost.
    expect(() => requireSecureEndpoint("ws://localhost:3000/authorization.json", "endpoint")).toThrow("must use HTTPS");
  });

  it("guard fails closed before any request is sent to a foreign origin", () => {
    const originalFetch = globalThis.fetch;
    const fetchSpy = vi.fn();
    globalThis.fetch = fetchSpy as unknown as typeof globalThis.fetch;
    try {
      expect(() =>
        requireSameOrigin("https://evil.example/steal.json", base)
      ).toThrow();
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});

// =============================================================================
// Cross-Origin Redirect Authorization Stripping
// =============================================================================

describe("cross-origin redirect Authorization stripping", () => {
  it("fetch drops the bearer token when a redirect leaves the origin", async () => {
    // The SDK relies on the WHATWG fetch spec (undici) to strip Authorization
    // when following a cross-origin redirect, rather than stripping it
    // explicitly. This test pins that platform guarantee: if a runtime or
    // dependency bump ever regresses it, the token would silently leak to the
    // Location target — this turns that into a CI failure.
    let evilHit = false;
    let evilAuth: string | null = "unset";

    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        return new HttpResponse(null, {
          status: 302,
          headers: { Location: "https://evil.example/stolen" },
        });
      }),
      http.get("https://evil.example/stolen", ({ request }) => {
        evilHit = true;
        evilAuth = request.headers.get("Authorization");
        return HttpResponse.json([]);
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    await client.GET("/projects.json");

    expect(evilHit).toBe(true);
    expect(evilAuth).toBeNull();
  });
});
