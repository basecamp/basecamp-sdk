import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import { createDownloadURL, filenameFromURL } from "../src/download.js";
import type { BasecampHooks, RequestInfo, RequestResult, OperationInfo } from "../src/hooks.js";
import { BasecampError } from "../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const API_ORIGIN = "https://3.basecampapi.com";
const S3_URL = "https://s3.amazonaws.com/bucket/signed-file.png";
/** Where a redirecting signed host would send hop 2 — and where hop 2 must never go. */
const THIRD_URL = "https://elsewhere.example.com/final/file.png";

function makeClient(hooks?: BasecampHooks, enableRetry?: boolean) {
  return createBasecampClient({
    accountId: "12345",
    accessToken: "test-token",
    baseUrl: BASE_URL,
    hooks,
    ...(enableRetry === undefined ? {} : { enableRetry }),
  });
}

/**
 * Direct factory construction: exercises the fixed hop-1 retry policy with a
 * millisecond backoff base (the internal test seam), so the retry tables run
 * without real one-second sleeps. Client-level tests keep the default base.
 */
function makeDownloadURL(overrides?: {
  hooks?: BasecampHooks;
  enableRetry?: boolean;
  requestTimeoutMs?: number;
}) {
  return createDownloadURL({
    authStrategy: {
      authenticate: async (headers) => {
        headers.set("Authorization", "Bearer test-token");
      },
    },
    userAgent: "basecamp-sdk-test",
    baseUrl: BASE_URL,
    hooks: overrides?.hooks,
    requestTimeoutMs: overrides?.requestTimeoutMs ?? 30_000,
    enableRetry: overrides?.enableRetry ?? true,
    retryBaseDelayMs: 1,
  });
}

// --- filenameFromURL ---

describe("filenameFromURL", () => {
  const cases: [string, string, string][] = [
    ["simple filename", "https://storage.3.basecamp.com/123/blobs/abc/download/logo.png", "logo.png"],
    ["encoded filename", "https://storage.3.basecamp.com/123/blobs/abc/download/my%20file.pdf", "my file.pdf"],
    ["trailing slash", "https://storage.3.basecamp.com/123/blobs/abc/download/", "download"],
    ["no path", "https://storage.3.basecamp.com", "download"],
    ["empty string", "", "download"],
    ["just slash", "https://storage.3.basecamp.com/", "download"],
    ["deep path", "https://example.com/a/b/c/report.csv", "report.csv"],
    ["with query", "https://example.com/path/file.txt?disposition=attachment", "file.txt"],
    ["invalid url", "://bad", "download"],
  ];

  it.each(cases)("%s: %s → %s", (_name, url, expected) => {
    expect(filenameFromURL(url)).toBe(expected);
  });
});

// --- downloadURL ---

describe("downloadURL", () => {
  describe("validation", () => {
    it("rejects empty URL", async () => {
      const client = makeClient();
      await expect(client.downloadURL("")).rejects.toThrow(BasecampError);
      await expect(client.downloadURL("")).rejects.toMatchObject({ code: "usage" });
    });

    it("rejects relative path", async () => {
      const client = makeClient();
      await expect(client.downloadURL("/blobs/abc/download/file.png")).rejects.toThrow(BasecampError);
      await expect(client.downloadURL("/blobs/abc/download/file.png")).rejects.toMatchObject({ code: "usage" });
    });

    it("rejects non-absolute URL", async () => {
      const client = makeClient();
      await expect(client.downloadURL("storage.3.basecamp.com/blobs/abc/download/file.png")).rejects.toThrow(BasecampError);
      await expect(client.downloadURL("storage.3.basecamp.com/blobs/abc/download/file.png")).rejects.toMatchObject({ code: "usage" });
    });
  });

  describe("URL rewriting", () => {
    it("rewrites URL to API server", async () => {
      let receivedPath = "";

      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          receivedPath = new URL(request.url).pathname;
          return new HttpResponse("content", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.png",
      );
      result.body.cancel();

      expect(receivedPath).toBe("/999/blobs/abc/download/file.png");
    });

    it("handles various host origins", async () => {
      let receivedPath = "";

      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          receivedPath = new URL(request.url).pathname;
          return new HttpResponse("ok", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const client = makeClient();
      const origins = [
        "https://storage.3.basecamp.com",
        "https://basecamp-static.example.com",
        "https://3.basecampapi.com",
      ];

      for (const origin of origins) {
        receivedPath = "";
        const result = await client.downloadURL(
          `${origin}/999/blobs/abc/download/file.png`,
        );
        result.body.cancel();
        expect(receivedPath).toBe("/999/blobs/abc/download/file.png");
      }
    });

    it("preserves query parameters", async () => {
      let receivedQuery = "";

      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          receivedQuery = new URL(request.url).search;
          return new HttpResponse("ok", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.png?disposition=attachment&foo=bar",
      );
      result.body.cancel();

      expect(receivedQuery).toBe("?disposition=attachment&foo=bar");
    });
  });

  describe("redirect flow", () => {
    it("follows 302 redirect to signed URL", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, () => {
          return new HttpResponse("binary file data", {
            headers: {
              "Content-Type": "image/png",
              "Content-Length": "16",
            },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/photo.png",
      );

      const reader = result.body.getReader();
      const chunks: Uint8Array[] = [];
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
      }
      const body = new TextDecoder().decode(Buffer.concat(chunks));

      expect(body).toBe("binary file data");
      expect(result.contentType).toBe("image/png");
      expect(result.contentLength).toBe(16);
      expect(result.filename).toBe("photo.png");
    });

    it("handles direct download (200 without redirect)", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse("pdf-data", {
            headers: { "Content-Type": "application/pdf" },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf",
      );

      const reader = result.body.getReader();
      const chunks: Uint8Array[] = [];
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
      }
      const body = new TextDecoder().decode(Buffer.concat(chunks));

      expect(body).toBe("pdf-data");
      expect(result.contentType).toBe("application/pdf");
    });

    it("resolves relative Location header", async () => {
      let resolvedHit = false;

      server.use(
        http.get(`${API_ORIGIN}/999/blobs/abc/download/file.txt`, () => {
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: "/resolved-path" },
          });
        }),
        http.get(`${API_ORIGIN}/resolved-path`, () => {
          resolvedHit = true;
          return new HttpResponse("resolved-data", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.txt",
      );

      const reader = result.body.getReader();
      const chunks: Uint8Array[] = [];
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
      }
      const body = new TextDecoder().decode(Buffer.concat(chunks));

      expect(body).toBe("resolved-data");
      expect(resolvedHit).toBe(true);
    });

    it("errors on redirect without Location header", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, { status: 302 });
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.txt"),
      ).rejects.toThrow(/no Location/);
    });
  });

  describe("error handling", () => {
    it.each([
      ["not found", 404, "not_found"],
      ["forbidden", 403, "forbidden"],
      ["server error", 500, "api_error"],
    ] as const)("handles %s (%d) → %s", async (_name, status, expectedCode) => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, { status });
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.txt"),
      ).rejects.toMatchObject({ code: expectedCode });
    });

    it("handles S3 error after redirect", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, () => {
          return new HttpResponse(null, { status: 403 });
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.png"),
      ).rejects.toThrow(BasecampError);
    });

    it("refuses a redirect on the signed second hop", async () => {
      // SPEC §14 "Hop-2 Redirect Policy": the signed URL is the one destination
      // the API host named. A redirect from it surfaces with its status, and the
      // Location it names is never dialled (#805). Before hop 2 passed
      // `redirect: "manual"`, fetch followed this chain and the caller
      // received "SECRET" as the file.
      let thirdHits = 0;
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, { status: 302, headers: { Location: S3_URL } });
        }),
        http.get(S3_URL, () => {
          return new HttpResponse(null, { status: 302, headers: { Location: THIRD_URL } });
        }),
        http.get(THIRD_URL, () => {
          thirdHits += 1;
          return HttpResponse.text("SECRET");
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.png"),
      ).rejects.toMatchObject({
        code: "api_error",
        httpStatus: 302,
        message: expect.stringContaining("not followed"),
      });
      expect(thirdHits).toBe(0);
    });

    it("handles signed-download network failure after successful redirect", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, () => {
          return HttpResponse.error();
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.png"),
      ).rejects.toMatchObject({ code: "network" });
    });

  });

  describe("credential-bearing values are never rendered (SPEC §9)", () => {
    // A caller-supplied download URL can smuggle a signed query through the
    // origin rewrite into hop 1, and the signed hop-2 URL is a credential
    // outright — so download transport errors carry fixed messages with no
    // cause, and hop-1 hooks see origin+path only.
    const SIGNED_RAW_URL =
      "https://storage.3.basecamp.com/999/blobs/abc/download/file.png?verifier=SECRET#frag";
    const SIGNED_S3_URL = `${S3_URL}?X-Amz-Signature=SECRET`;
    const PROJECTED_URL = `${API_ORIGIN}/999/blobs/abc/download/file.png`;

    it("hop-1 network error carries a fixed message and no cause — to the caller and to hooks", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () => HttpResponse.error()),
      );

      const hookErrors: (Error | undefined)[] = [];
      const hooks: BasecampHooks = {
        onRequestEnd: (_info, result) => {
          hookErrors.push(result.error);
        },
      };

      const downloadURL = makeDownloadURL({ enableRetry: false, hooks });
      let caught: unknown;
      try {
        await downloadURL(SIGNED_RAW_URL);
      } catch (err) {
        caught = err;
      }
      const error = caught as BasecampError;
      expect(error).toBeInstanceOf(BasecampError);
      expect(error.code).toBe("network");
      expect(error.message).toBe("Network error");
      expect(error.cause).toBeUndefined();

      expect(hookErrors).toHaveLength(1);
      const hookError = hookErrors[0] as BasecampError;
      expect(hookError).toBeInstanceOf(BasecampError);
      expect(hookError.message).toBe("Network error");
      expect(hookError.cause).toBeUndefined();
    });

    it("retried hop-1 network failures reach onRequestEnd and onRetry projected", async () => {
      let attempts = 0;
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          if (attempts === 1) {
            return HttpResponse.error();
          }
          return new HttpResponse("content", { headers: { "Content-Type": "text/plain" } });
        }),
      );

      const endErrors: (Error | undefined)[] = [];
      const retryErrors: Error[] = [];
      const hooks: BasecampHooks = {
        onRequestEnd: (_info, result) => {
          endErrors.push(result.error);
        },
        onRetry: (_info, _attempt, error) => {
          retryErrors.push(error);
        },
      };

      const downloadURL = makeDownloadURL({ hooks });
      const result = await downloadURL(SIGNED_RAW_URL);
      result.body.cancel();

      expect(endErrors).toHaveLength(2);
      expect(endErrors[0]).toBeInstanceOf(BasecampError);
      expect(endErrors[0]!.message).toBe("Network error");
      expect(endErrors[0]!.cause).toBeUndefined();
      expect(endErrors[1]).toBeUndefined();
      expect(retryErrors).toHaveLength(1);
      expect(retryErrors[0]).toBeInstanceOf(BasecampError);
      expect(retryErrors[0]!.message).toBe("Network error");
    });

    it("status retries still hand onRetry the status error, not a network projection", async () => {
      let attempts = 0;
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, { status: 503 });
          }
          return new HttpResponse("content", { headers: { "Content-Type": "text/plain" } });
        }),
      );

      const retryErrors: Error[] = [];
      const hooks: BasecampHooks = {
        onRetry: (_info, _attempt, error) => {
          retryErrors.push(error);
        },
      };

      const downloadURL = makeDownloadURL({ hooks });
      const result = await downloadURL(SIGNED_RAW_URL);
      result.body.cancel();

      expect(retryErrors).toHaveLength(1);
      expect(retryErrors[0]!.message).toContain("HTTP 503");
    });

    it("a signed Location that fails URL construction renders the fixed token", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () =>
          new HttpResponse(null, {
            status: 302,
            headers: { Location: "https://[invalid/bucket/file?X-Amz-Signature=SECRET" },
          })),
      );

      const client = makeClient();
      let caught: unknown;
      try {
        await client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.png");
      } catch (err) {
        caught = err;
      }
      const error = caught as BasecampError;
      expect(error).toBeInstanceOf(BasecampError);
      expect(error.code).toBe("api_error");
      expect(error.message).toBe("redirect to undialable download URL: unparsable");
    });

    it("hop-2 network error carries a fixed message and no cause", async () => {
      server.use(
        http.get(`${API_ORIGIN}/*`, () =>
          new HttpResponse(null, { status: 302, headers: { Location: SIGNED_S3_URL } })),
        http.get(S3_URL, () => HttpResponse.error()),
      );

      const client = makeClient();
      let caught: unknown;
      try {
        await client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.png");
      } catch (err) {
        caught = err;
      }
      const error = caught as BasecampError;
      expect(error).toBeInstanceOf(BasecampError);
      expect(error.code).toBe("network");
      expect(error.message).toBe("Download failed");
      expect(error.cause).toBeUndefined();
    });

    it("hop-1 hook URLs carry no query or fragment while the wire keeps the query", async () => {
      let wireQuery: string | null = null;
      const urls: string[] = [];
      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          wireQuery = new URL(request.url).search;
          return new HttpResponse("content", { headers: { "Content-Type": "text/plain" } });
        }),
      );

      const hooks: BasecampHooks = {
        onRequestStart: (info) => {
          urls.push(info.url);
        },
        onRequestEnd: (info) => {
          urls.push(info.url);
        },
      };

      const downloadURL = makeDownloadURL({ hooks });
      const result = await downloadURL(SIGNED_RAW_URL);
      result.body.cancel();

      expect(wireQuery).toBe("?verifier=SECRET");
      expect(urls).toEqual([PROJECTED_URL, PROJECTED_URL]);
    });

    it("onRetry sees the projected URL too", async () => {
      let attempts = 0;
      const retryURLs: string[] = [];
      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, { status: 503 });
          }
          return new HttpResponse("content", { headers: { "Content-Type": "text/plain" } });
        }),
      );

      const hooks: BasecampHooks = {
        onRetry: (info) => {
          retryURLs.push(info.url);
        },
      };

      const downloadURL = makeDownloadURL({ hooks });
      const result = await downloadURL(SIGNED_RAW_URL);
      result.body.cancel();

      expect(retryURLs).toEqual([PROJECTED_URL]);
    });
  });

  describe("hop-1 retry policy (SPEC §14)", () => {
    const RAW_URL = "https://storage.3.basecamp.com/999/blobs/abc/download/file.png";

    // The COMPLETE declared retry set, pinned status by status: hop 1 retries
    // {429, 502, 503, 504} (the shared conformance fixtures cover 429/503).
    it.each([[429], [502], [503], [504]] as const)(
      "retries hop 1 on %d, then follows the redirect",
      async (status) => {
        let apiAttempts = 0;

        server.use(
          http.get(`${API_ORIGIN}/*`, () => {
            apiAttempts++;
            if (apiAttempts === 1) {
              return new HttpResponse(null, { status });
            }
            return new HttpResponse(null, {
              status: 302,
              headers: { Location: S3_URL },
            });
          }),
          http.get(S3_URL, () => {
            return new HttpResponse("data", {
              headers: { "Content-Type": "application/octet-stream" },
            });
          }),
        );

        const downloadURL = makeDownloadURL();
        const result = await downloadURL(RAW_URL);
        result.body.cancel();

        expect(apiAttempts).toBe(2);
      },
    );

    it("never retries 500 — deliberately outside the declared set", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          return new HttpResponse(null, { status: 500 });
        }),
      );

      const downloadURL = makeDownloadURL();
      await expect(downloadURL(RAW_URL)).rejects.toMatchObject({ code: "api_error" });

      expect(attempts).toBe(1);
    });

    it("retries hop 1 on a network error", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          if (attempts === 1) {
            return HttpResponse.error();
          }
          return new HttpResponse("content", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const downloadURL = makeDownloadURL();
      const result = await downloadURL(RAW_URL);
      result.body.cancel();

      expect(attempts).toBe(2);
    });

    it("exhausts the three-attempt budget and surfaces the final error", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          return new HttpResponse(null, { status: 503 });
        }),
      );

      const downloadURL = makeDownloadURL();
      await expect(downloadURL(RAW_URL)).rejects.toThrow(BasecampError);

      expect(attempts).toBe(3);
    });

    it("makes exactly one attempt when retry is disabled", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          return new HttpResponse(null, { status: 503 });
        }),
      );

      const downloadURL = makeDownloadURL({ enableRetry: false });
      await expect(downloadURL(RAW_URL)).rejects.toThrow(BasecampError);

      expect(attempts).toBe(1);
    });

    it("treats a per-attempt timeout abort as terminal — no retry after abort", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, async () => {
          attempts++;
          await new Promise((resolve) => setTimeout(resolve, 200));
          return new HttpResponse(null, { status: 503 });
        }),
      );

      const downloadURL = makeDownloadURL({ requestTimeoutMs: 50 });
      await expect(downloadURL(RAW_URL)).rejects.toMatchObject({ code: "network" });

      expect(attempts).toBe(1);
    });

    it("sends Authorization on every hop-1 attempt and never on hop 2", async () => {
      const apiAuthHeaders: (string | null)[] = [];
      let s3AuthHeader: string | null = "unset";
      let apiAttempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          apiAttempts++;
          apiAuthHeaders.push(request.headers.get("Authorization"));
          if (apiAttempts === 1) {
            return new HttpResponse(null, { status: 503 });
          }
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, ({ request }) => {
          s3AuthHeader = request.headers.get("Authorization");
          return new HttpResponse("data", {
            headers: { "Content-Type": "application/octet-stream" },
          });
        }),
      );

      const downloadURL = makeDownloadURL();
      const result = await downloadURL(RAW_URL);
      result.body.cancel();

      expect(apiAuthHeaders).toEqual(["Bearer test-token", "Bearer test-token"]);
      expect(s3AuthHeader).toBeNull();
    });

    it("fires balanced start/end hooks and one onRetry per backoff", async () => {
      const events: { kind: string; attempt: number }[] = [];
      let apiAttempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          apiAttempts++;
          if (apiAttempts < 3) {
            return new HttpResponse(null, { status: 503 });
          }
          return new HttpResponse("content", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const hooks: BasecampHooks = {
        onRequestStart: (info) => {
          events.push({ kind: "start", attempt: info.attempt });
        },
        onRequestEnd: (info) => {
          events.push({ kind: "end", attempt: info.attempt });
        },
        onRetry: (_info, upcomingAttempt) => {
          events.push({ kind: "retry", attempt: upcomingAttempt });
        },
      };

      const downloadURL = makeDownloadURL({ hooks });
      const result = await downloadURL(RAW_URL);
      result.body.cancel();

      const byKind = (kind: string) =>
        events.filter((e) => e.kind === kind).map((e) => e.attempt);
      expect(byKind("start")).toEqual([1, 2, 3]);
      expect(byKind("end")).toEqual([1, 2, 3]);
      // onRetry's argument is the UPCOMING attempt (SPEC §7 attempt semantics).
      expect(byKind("retry")).toEqual([2, 3]);
    });

    it("honors Retry-After on 429 at the client level", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "1" },
            });
          }
          return new HttpResponse("content", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const client = makeClient();
      const start = performance.now();
      const result = await client.downloadURL(RAW_URL);
      result.body.cancel();
      const elapsed = performance.now() - start;

      expect(attempts).toBe(2);
      // Node timers may fire marginally early; require all but a sliver of
      // the requested second so a dropped Retry-After (millisecond backoff
      // would miss by ~999ms) still fails loudly.
      expect(elapsed).toBeGreaterThanOrEqual(990);
    });
  });

  describe("auth headers", () => {
    it("sends auth on API leg, not on S3 leg", async () => {
      let apiAuthHeader = "";
      let s3AuthHeader = "";

      server.use(
        http.get(`${API_ORIGIN}/*`, ({ request }) => {
          apiAuthHeader = request.headers.get("Authorization") ?? "";
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, ({ request }) => {
          s3AuthHeader = request.headers.get("Authorization") ?? "";
          return new HttpResponse("data", {
            headers: { "Content-Type": "application/octet-stream" },
          });
        }),
      );

      const client = makeClient();
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.png",
      );
      result.body.cancel();

      expect(apiAuthHeader).toBe("Bearer test-token");
      expect(s3AuthHeader).toBe("");
    });
  });

  describe("hooks", () => {
    it("fires operation hooks once", async () => {
      let opStartCount = 0;
      let opEndCount = 0;
      let capturedOp: OperationInfo | null = null;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse("ok", {
            headers: { "Content-Type": "text/plain" },
          });
        }),
      );

      const hooks: BasecampHooks = {
        onOperationStart: (info) => {
          opStartCount++;
          capturedOp = info;
        },
        onOperationEnd: () => {
          opEndCount++;
        },
      };

      const client = makeClient(hooks);
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.txt",
      );
      result.body.cancel();

      expect(opStartCount).toBe(1);
      expect(opEndCount).toBe(1);
      expect(capturedOp).toMatchObject({
        service: "Client",
        operation: "DownloadURL",
        resourceType: "download",
        isMutation: false,
      });
    });

    it("fires request hooks for API leg only", async () => {
      let reqStartCount = 0;
      let reqEndCount = 0;
      let capturedReqInfo: RequestInfo | null = null;
      let capturedReqResult: RequestResult | null = null;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: S3_URL },
          });
        }),
        http.get(S3_URL, () => {
          return new HttpResponse("data", {
            headers: { "Content-Type": "application/octet-stream" },
          });
        }),
      );

      const hooks: BasecampHooks = {
        onRequestStart: (info) => {
          reqStartCount++;
          capturedReqInfo = info;
        },
        onRequestEnd: (info, result) => {
          reqEndCount++;
          capturedReqResult = result;
        },
      };

      const client = makeClient(hooks);
      const result = await client.downloadURL(
        "https://storage.3.basecamp.com/999/blobs/abc/download/file.png",
      );
      result.body.cancel();

      expect(reqStartCount).toBe(1);
      expect(reqEndCount).toBe(1);
      expect(capturedReqInfo).toMatchObject({
        method: "GET",
        attempt: 1,
      });
      expect(capturedReqInfo!.url).toContain(API_ORIGIN);
      expect(capturedReqResult).toMatchObject({
        statusCode: 302,
        fromCache: false,
      });
    });

    it("fires onRequestEnd on error responses", async () => {
      let reqEndCount = 0;
      let capturedResult: RequestResult | null = null;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return new HttpResponse(null, { status: 404 });
        }),
      );

      const hooks: BasecampHooks = {
        onRequestStart: () => {},
        onRequestEnd: (_info, result) => {
          reqEndCount++;
          capturedResult = result;
        },
      };

      const client = makeClient(hooks);
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.txt"),
      ).rejects.toThrow(BasecampError);

      expect(reqEndCount).toBe(1);
      expect(capturedResult!.statusCode).toBe(404);
    });

    it("fires onRequestEnd with statusCode 0 on network failure (retry disabled)", async () => {
      let reqStartCount = 0;
      let reqEndCount = 0;
      let capturedResult: RequestResult | null = null;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          return HttpResponse.error();
        }),
      );

      const hooks: BasecampHooks = {
        onRequestStart: () => {
          reqStartCount++;
        },
        onRequestEnd: (_info, result) => {
          reqEndCount++;
          capturedResult = result;
        },
      };

      // Retry disabled: exactly one hop-1 attempt (SPEC §14 attempt budget),
      // so the hook contract is pinned per attempt without real backoff. The
      // retry-enabled hook shape is pinned by the balanced-hooks test above.
      const client = makeClient(hooks, false);
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.txt"),
      ).rejects.toMatchObject({ code: "network" });

      expect(reqStartCount).toBe(1);
      expect(reqEndCount).toBe(1);
      expect(capturedResult!.statusCode).toBe(0);
      expect(capturedResult!.error).toBeDefined();
    });
  });
});
