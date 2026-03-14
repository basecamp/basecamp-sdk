import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse, passthrough } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import { filenameFromURL } from "../src/download.js";
import type { BasecampHooks, RequestInfo, RequestResult, OperationInfo, OperationResult } from "../src/hooks.js";
import { BasecampError } from "../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const API_ORIGIN = "https://3.basecampapi.com";
const S3_URL = "https://s3.amazonaws.com/bucket/signed-file.png";

function makeClient(hooks?: BasecampHooks) {
  return createBasecampClient({
    accountId: "12345",
    accessToken: "test-token",
    baseUrl: BASE_URL,
    hooks,
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

    it("does not retry on 429", async () => {
      let attempts = 0;

      server.use(
        http.get(`${API_ORIGIN}/*`, () => {
          attempts++;
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "1" },
          });
        }),
      );

      const client = makeClient();
      await expect(
        client.downloadURL("https://storage.3.basecamp.com/999/blobs/abc/download/file.txt"),
      ).rejects.toMatchObject({ code: "rate_limit" });

      expect(attempts).toBe(1);
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

    it("fires onRequestEnd with statusCode 0 on network failure", async () => {
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

      const client = makeClient(hooks);
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
