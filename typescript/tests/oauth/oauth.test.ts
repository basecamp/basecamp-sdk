/**
 * OAuth module tests.
 *
 * Tests discovery, token exchange, and token refresh functionality.
 */

import { describe, it, expect } from "vitest";
import { http, HttpResponse, delay } from "msw";
import { server } from "../setup.js";
import {
  discover,
  discoverLaunchpad,
  exchangeCode,
  refreshToken,
  isTokenExpired,
  LAUNCHPAD_BASE_URL,
  type OAuthToken,
} from "../../src/oauth/index.js";
import { BasecampError } from "../../src/errors.js";

describe("OAuth Discovery", () => {
  const mockDiscoveryResponse = {
    issuer: "https://launchpad.37signals.com",
    authorization_endpoint: "https://launchpad.37signals.com/authorization/new",
    token_endpoint: "https://launchpad.37signals.com/authorization/token",
    registration_endpoint: "https://launchpad.37signals.com/authorization/register",
    scopes_supported: ["read", "write"],
  };

  describe("discover", () => {
    it("fetches OAuth configuration from discovery endpoint", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json(mockDiscoveryResponse)
        )
      );

      const config = await discover("https://launchpad.37signals.com");

      expect(config.issuer).toBe("https://launchpad.37signals.com");
      expect(config.authorizationEndpoint).toBe(
        "https://launchpad.37signals.com/authorization/new"
      );
      expect(config.tokenEndpoint).toBe(
        "https://launchpad.37signals.com/authorization/token"
      );
      expect(config.registrationEndpoint).toBe(
        "https://launchpad.37signals.com/authorization/register"
      );
      expect(config.scopesSupported).toEqual(["read", "write"]);
    });

    it("parses code_challenge_methods_supported from discovery", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json({
            ...mockDiscoveryResponse,
            code_challenge_methods_supported: ["S256"],
          })
        )
      );

      const config = await discover("https://launchpad.37signals.com");

      expect(config.codeChallengeMethodsSupported).toEqual(["S256"]);
    });

    it("rejects a non-array code_challenge_methods_supported as api_error", async () => {
      // A bare string would be substring-matched during PKCE negotiation and
      // could falsely appear to advertise "S256"; it must be rejected.
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json({
            ...mockDiscoveryResponse,
            code_challenge_methods_supported: "S256",
          })
        )
      );

      await expect(discover("https://launchpad.37signals.com")).rejects.toMatchObject({
        code: "api_error",
      });
    });

    it("rejects code_challenge_methods_supported with a non-string element", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json({
            ...mockDiscoveryResponse,
            code_challenge_methods_supported: ["S256", 256],
          })
        )
      );

      await expect(discover("https://launchpad.37signals.com")).rejects.toMatchObject({
        code: "api_error",
      });
    });

    it("leaves codeChallengeMethodsSupported undefined when not in response", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json(mockDiscoveryResponse)
        )
      );

      const config = await discover("https://launchpad.37signals.com");

      expect(config.codeChallengeMethodsSupported).toBeUndefined();
    });

    it("normalizes a trailing slash for the fetch URL but binds against the raw issuer", async () => {
      // The trailing slash is dropped only for the well-known fetch (routing);
      // issuer binding is code-point-exact against the caller's raw string (RFC
      // 8414 §3.3, SPEC.md §16), so the AS must echo the trailing-slash issuer.
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json({ ...mockDiscoveryResponse, issuer: "https://launchpad.37signals.com/" })
        )
      );

      const config = await discover("https://launchpad.37signals.com/");

      expect(config.issuer).toBe("https://launchpad.37signals.com/");
    });

    it("throws BasecampError on HTTP error", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.text("Not Found", { status: 404 })
        )
      );

      try {
        await discover("https://launchpad.37signals.com");
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        // Non-2xx discovery responses are api_error (standardized), not network.
        expect((err as BasecampError).code).toBe("api_error");
        expect((err as BasecampError).httpStatus).toBe(404);
      }
    });

    it("truncates a large non-2xx body in the error message", async () => {
      const hugeBody = "x".repeat(200_000);
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.text(hugeBody, { status: 502 })
        )
      );

      try {
        await discover("https://launchpad.37signals.com");
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        // The body is capped at 500 chars (matching Go/Python/Ruby), so the message
        // never grows with the response body — no log spam / memory pressure.
        const msg = (err as BasecampError).message;
        expect(msg.length).toBeLessThan(600);
        expect(msg).toContain("...");
        expect(msg).not.toContain(hugeBody);
      }
    });

    it("throws BasecampError on invalid JSON response", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () =>
            HttpResponse.json({
              issuer: "https://launchpad.37signals.com",
              // Missing required fields
            })
        )
      );

      await expect(discover("https://launchpad.37signals.com")).rejects.toThrow(
        /missing required fields/
      );
    });

    it("throws BasecampError on network error", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.error()
        )
      );

      await expect(discover("https://launchpad.37signals.com")).rejects.toThrow(
        BasecampError
      );
    });
  });

  describe("discoverLaunchpad", () => {
    it("uses default Launchpad URL", async () => {
      server.use(
        http.get(
          `${LAUNCHPAD_BASE_URL}/.well-known/oauth-authorization-server`,
          () => HttpResponse.json(mockDiscoveryResponse)
        )
      );

      const config = await discoverLaunchpad();

      expect(config.issuer).toBe("https://launchpad.37signals.com");
    });
  });
});

describe("Token Exchange", () => {
  const tokenEndpoint = "https://launchpad.37signals.com/authorization/token";

  const mockTokenResponse = {
    access_token: "test_access_token",
    refresh_token: "test_refresh_token",
    token_type: "Bearer",
    expires_in: 3600,
  };

  describe("exchangeCode", () => {
    it("never echoes the body when a token response fails to parse", async () => {
      // A syntactically-broken token body can still carry credential
      // material — the parse error must not echo any of it into the
      // message, where it would reach logs and exception telemetry.
      const secret = "sk-live-SUPERSECRET";
      server.use(
        http.post(tokenEndpoint, () => new HttpResponse(`{"access_token": "${secret}' oops`, { status: 200 }))
      );
      const err = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      }).catch((e) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect(err.code).toBe("api_error");
      expect(err.message).not.toContain(secret);
    });

    it("rejects a non-2xx null body as api_error with the status (never raw TypeError → network)", async () => {
      server.use(http.post(tokenEndpoint, () => HttpResponse.json(null, { status: 400 })));
      const err = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      }).catch((e) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect(err.code).toBe("api_error");
      expect(err.httpStatus).toBe(400);
    });

    it.each([
      ["null body", null],
      ["array body", []],
      ["numeric refresh_token", { access_token: "a", refresh_token: 123 }],
      ["numeric scope", { access_token: "a", scope: 7 }],
      ["fractional expires_in", { access_token: "a", expires_in: 3600.5 }],
      ["oversized expires_in", { access_token: "a", expires_in: 9_000_000_000_000_000_000 }],
    ])("rejects a malformed 200 token body (%s) as api_error with the HTTP status", async (_label, body) => {
      server.use(http.post(tokenEndpoint, () => HttpResponse.json(body as never)));
      const err = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      }).catch((e) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect(err.code).toBe("api_error");
      expect(err.httpStatus).toBe(200);
    });

    it("rejects a numeric access_token as api_error with the HTTP status", async () => {
      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json({ access_token: 123 })
        )
      );
      const err = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      }).catch((e) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect(err.code).toBe("api_error");
      expect(err.httpStatus).toBe(200);
    });

    it("rejects a present-but-empty token_type as api_error (null/absent default to Bearer)", async () => {
      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json({ access_token: "a", token_type: "" })
        )
      );
      const err = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      }).catch((e) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect(err.code).toBe("api_error");

      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json({ access_token: "a", token_type: null })
        )
      );
      const token = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
      });
      expect(token.tokenType).toBe("Bearer");
    });

    it("exchanges authorization code for tokens (standard format)", async () => {
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);

          expect(params.get("grant_type")).toBe("authorization_code");
          expect(params.get("code")).toBe("auth_code_123");
          expect(params.get("redirect_uri")).toBe("https://myapp.com/callback");
          expect(params.get("client_id")).toBe("my_client_id");
          expect(params.get("client_secret")).toBe("my_client_secret");

          return HttpResponse.json(mockTokenResponse);
        })
      );

      const token = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
        clientSecret: "my_client_secret",
      });

      expect(token.accessToken).toBe("test_access_token");
      expect(token.refreshToken).toBe("test_refresh_token");
      expect(token.tokenType).toBe("Bearer");
      expect(token.expiresIn).toBe(3600);
      expect(token.expiresAt).toBeInstanceOf(Date);
    });

    it("exchanges authorization code using legacy format", async () => {
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);

          expect(params.get("type")).toBe("web_server");
          expect(params.has("grant_type")).toBe(false);

          return HttpResponse.json(mockTokenResponse);
        })
      );

      const token = await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
        useLegacyFormat: true,
      });

      expect(token.accessToken).toBe("test_access_token");
    });

    it("includes PKCE code verifier when provided", async () => {
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);

          expect(params.get("code_verifier")).toBe("my_code_verifier");

          return HttpResponse.json(mockTokenResponse);
        })
      );

      await exchangeCode({
        tokenEndpoint,
        code: "auth_code_123",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client_id",
        codeVerifier: "my_code_verifier",
      });
    });

    it("validates required fields", async () => {
      await expect(
        exchangeCode({
          tokenEndpoint: "",
          code: "auth_code",
          redirectUri: "https://myapp.com/callback",
          clientId: "my_client",
        })
      ).rejects.toThrow("Token endpoint is required");

      await expect(
        exchangeCode({
          tokenEndpoint,
          code: "",
          redirectUri: "https://myapp.com/callback",
          clientId: "my_client",
        })
      ).rejects.toThrow("Authorization code is required");

      await expect(
        exchangeCode({
          tokenEndpoint,
          code: "auth_code",
          redirectUri: "",
          clientId: "my_client",
        })
      ).rejects.toThrow("Redirect URI is required");

      await expect(
        exchangeCode({
          tokenEndpoint,
          code: "auth_code",
          redirectUri: "https://myapp.com/callback",
          clientId: "",
        })
      ).rejects.toThrow("Client ID is required");
    });

    it("throws BasecampError on invalid_grant error", async () => {
      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json(
            {
              error: "invalid_grant",
              error_description: "The authorization code has expired",
            },
            { status: 400 }
          )
        )
      );

      try {
        await exchangeCode({
          tokenEndpoint,
          code: "expired_code",
          redirectUri: "https://myapp.com/callback",
          clientId: "my_client",
        });
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("auth_required");
        expect((err as BasecampError).message).toContain("authorization code has expired");
      }
    });

    it("throws BasecampError on 401 error", async () => {
      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json(
            { error: "invalid_client", error_description: "Invalid client credentials" },
            { status: 401 }
          )
        )
      );

      try {
        await exchangeCode({
          tokenEndpoint,
          code: "auth_code",
          redirectUri: "https://myapp.com/callback",
          clientId: "invalid_client",
          clientSecret: "wrong_secret",
        });
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("auth_required");
      }
    });
  });

  describe("refreshToken", () => {
    it("refreshes access token (standard format)", async () => {
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);

          expect(params.get("grant_type")).toBe("refresh_token");
          expect(params.get("refresh_token")).toBe("my_refresh_token");
          expect(params.get("client_id")).toBe("my_client_id");
          expect(params.get("client_secret")).toBe("my_client_secret");

          return HttpResponse.json(mockTokenResponse);
        })
      );

      const token = await refreshToken({
        tokenEndpoint,
        refreshToken: "my_refresh_token",
        clientId: "my_client_id",
        clientSecret: "my_client_secret",
      });

      expect(token.accessToken).toBe("test_access_token");
      expect(token.refreshToken).toBe("test_refresh_token");
    });

    it("refreshes token using legacy format", async () => {
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);

          expect(params.get("type")).toBe("refresh");
          expect(params.has("grant_type")).toBe(false);

          return HttpResponse.json(mockTokenResponse);
        })
      );

      const token = await refreshToken({
        tokenEndpoint,
        refreshToken: "my_refresh_token",
        useLegacyFormat: true,
      });

      expect(token.accessToken).toBe("test_access_token");
    });

    it("validates required fields", async () => {
      await expect(
        refreshToken({
          tokenEndpoint: "",
          refreshToken: "my_refresh_token",
        })
      ).rejects.toThrow("Token endpoint is required");

      await expect(
        refreshToken({
          tokenEndpoint,
          refreshToken: "",
        })
      ).rejects.toThrow("Refresh token is required");
    });

    it("sends resource when set and omits it when unset", async () => {
      let sawResource: string | null = null;
      let hasResource = true;
      server.use(
        http.post(tokenEndpoint, async ({ request }) => {
          const params = new URLSearchParams(await request.text());
          sawResource = params.get("resource");
          hasResource = params.has("resource");
          return HttpResponse.json(mockTokenResponse);
        })
      );

      await refreshToken({
        tokenEndpoint,
        refreshToken: "my_refresh_token",
        resource: "urn:bc:account:42",
      });
      expect(sawResource).toBe("urn:bc:account:42");

      await refreshToken({
        tokenEndpoint,
        refreshToken: "my_refresh_token",
      });
      expect(hasResource).toBe(false);
    });

    it("captures resource from the token response and treats null as absent", async () => {
      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json({ ...mockTokenResponse, resource: "urn:bc:account:42" })
        )
      );
      const token = await refreshToken({ tokenEndpoint, refreshToken: "my_refresh_token" });
      expect(token.resource).toBe("urn:bc:account:42");

      server.use(
        http.post(tokenEndpoint, () =>
          HttpResponse.json({ ...mockTokenResponse, resource: null })
        )
      );
      const nullToken = await refreshToken({ tokenEndpoint, refreshToken: "my_refresh_token" });
      expect(nullToken.resource).toBeUndefined();
    });

    it("rejects a present-but-empty or non-string resource as api_error", async () => {
      for (const resource of ["", 7]) {
        server.use(
          http.post(tokenEndpoint, () =>
            HttpResponse.json({ ...mockTokenResponse, resource })
          )
        );
        await expect(
          refreshToken({ tokenEndpoint, refreshToken: "my_refresh_token" })
        ).rejects.toThrow("resource must be a non-empty string");
      }
    });
  });
});

describe("Token-Endpoint Transport Policy", () => {
  const tokenEndpoint = "https://launchpad.37signals.com/authorization/token";
  const attackerUrl = "https://attacker.example.com/steal";

  const callExchange = () =>
    exchangeCode({
      tokenEndpoint,
      code: "auth_code_123",
      redirectUri: "https://myapp.com/callback",
      clientId: "my_client_id",
    });
  const callRefresh = () =>
    refreshToken({
      tokenEndpoint,
      refreshToken: "my_refresh_token",
    });

  describe.each([
    ["exchangeCode", callExchange],
    ["refreshToken", callRefresh],
  ] as const)("%s", (_name, call) => {
    it.each([301, 302, 303, 307, 308])(
      "refuses a %d as api_error and never dials its Location",
      async (status) => {
        // SPEC §16 "Token-Endpoint Transport Policy": a redirect from the
        // token endpoint surfaces with its status, and the Location it names
        // is never dialled — a followed 307/308 would re-POST the credentials
        // wherever it points. A usable token behind the Location proves the
        // refusal is what stopped the chain, not a broken attacker handler.
        let attackerHits = 0;
        server.use(
          http.post(tokenEndpoint, () =>
            new HttpResponse(null, { status, headers: { Location: attackerUrl } })
          ),
          http.all(attackerUrl, () => {
            attackerHits += 1;
            return HttpResponse.json({ access_token: "stolen_token" });
          })
        );

        await expect(call()).rejects.toMatchObject({
          code: "api_error",
          httpStatus: status,
          message: expect.stringContaining("not followed"),
        });
        expect(attackerHits).toBe(0);
      }
    );
  });

  it("keeps 304 on the generic non-ok path, not the redirect refusal", async () => {
    // 304 is a cache validator, not a redirect-with-Location.
    server.use(http.post(tokenEndpoint, () => new HttpResponse(null, { status: 304 })));

    const err = await callRefresh().catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
    expect(err.httpStatus).toBe(304);
    expect(err.message).not.toContain("not followed");
  });

  it("refuses a browser opaqueredirect (status 0) as a not-followed redirect", async () => {
    // Browser fetch answers redirect: "manual" with an opaqueredirect — type
    // "opaqueredirect", status 0, no headers — instead of the 3xx Node hands
    // back. Without the type check it falls into the body reader and surfaces
    // as a generic missing-length/parse api_error.
    const opaque = {
      type: "opaqueredirect",
      status: 0,
      ok: false,
      headers: new Headers(),
      body: null,
    } as unknown as Response;
    const browserFetch: typeof globalThis.fetch = async () => opaque;

    const err = await refreshToken(
      { tokenEndpoint, refreshToken: "my_refresh_token" },
      { fetch: browserFetch }
    ).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
    expect(err.message).toContain("not followed");
    expect(err.httpStatus).toBeUndefined();
  });

  it.each([Number.NaN, Infinity, -5, 0])(
    "normalizes invalid timeoutMs %p to the default instead of instant-aborting",
    async (badTimeout) => {
      // An unclamped NaN/Infinity became setTimeout's ~1 ms delay — an
      // immediate abort masquerading as "Token request timed out".
      server.use(
        http.post(tokenEndpoint, async () => {
          await delay(50);
          return HttpResponse.json({ access_token: "tok", token_type: "Bearer" });
        })
      );

      const token = await refreshToken(
        { tokenEndpoint, refreshToken: "my_refresh_token" },
        { timeoutMs: badTimeout }
      );
      expect(token.accessToken).toBe("tok");
    }
  );

  it("bounds the exchange even when a custom fetch ignores its AbortSignal", async () => {
    // A never-settling fetch that ignores its signal must not hold the call
    // open past the timeout — the raceAbort wrapper rejects on the timer.
    const neverSettles: typeof globalThis.fetch = () => new Promise<Response>(() => {});

    await expect(
      refreshToken(
        { tokenEndpoint, refreshToken: "my_refresh_token" },
        { fetch: neverSettles, timeoutMs: 20 }
      )
    ).rejects.toMatchObject({
      code: "network",
      message: expect.stringContaining("timed out"),
    });
  });
});

describe("Response Size Limits", () => {
  const tokenEndpoint = "https://launchpad.37signals.com/authorization/token";

  it("rejects response with Content-Length exceeding limit", async () => {
    server.use(
      http.post(tokenEndpoint, () => {
        return new HttpResponse(JSON.stringify({ access_token: "test" }), {
          status: 200,
          headers: {
            "Content-Type": "application/json",
            "Content-Length": "99999999999", // ~100GB
          },
        });
      })
    );

    await expect(
      exchangeCode({
        tokenEndpoint,
        code: "auth_code",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client",
      })
    ).rejects.toThrow(/too large/);
  });

  it("treats non-numeric Content-Length as missing (security)", async () => {
    // A non-numeric Content-Length should not bypass size checks.
    // In non-streaming environments, this should fail closed.
    server.use(
      http.post(tokenEndpoint, () => {
        return new HttpResponse(JSON.stringify({ access_token: "test" }), {
          status: 200,
          headers: {
            "Content-Type": "application/json",
            "Content-Length": "abc123", // Invalid - not a number
          },
        });
      })
    );

    // Note: In a streaming environment (Node.js), this will succeed because
    // streaming can enforce the byte limit. In a non-streaming environment,
    // this would fail closed. The test verifies the response is either:
    // 1. Successfully parsed (streaming was available), OR
    // 2. Rejected with "no valid Content-Length" error (fail closed)
    try {
      const result = await exchangeCode({
        tokenEndpoint,
        code: "auth_code",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client",
      });
      // If it succeeds, streaming was available and the small body was read
      expect(result.accessToken).toBe("test");
    } catch (err) {
      // If it fails, it should be because we failed closed on invalid Content-Length
      expect((err as Error).message).toMatch(/no valid Content-Length/);
    }
  });

  it("treats negative Content-Length as missing", async () => {
    server.use(
      http.post(tokenEndpoint, () => {
        return new HttpResponse(JSON.stringify({ access_token: "test" }), {
          status: 200,
          headers: {
            "Content-Type": "application/json",
            "Content-Length": "-100",
          },
        });
      })
    );

    // Same behavior as non-numeric: either streaming succeeds or fail closed
    try {
      const result = await exchangeCode({
        tokenEndpoint,
        code: "auth_code",
        redirectUri: "https://myapp.com/callback",
        clientId: "my_client",
      });
      expect(result.accessToken).toBe("test");
    } catch (err) {
      expect((err as Error).message).toMatch(/no valid Content-Length/);
    }
  });
});

describe("Token Expiration", () => {
  describe("isTokenExpired", () => {
    it("returns false for token without expiration", () => {
      const token: OAuthToken = {
        accessToken: "test",
        tokenType: "Bearer",
      };

      expect(isTokenExpired(token)).toBe(false);
    });

    it("returns false for non-expired token", () => {
      const token: OAuthToken = {
        accessToken: "test",
        tokenType: "Bearer",
        expiresAt: new Date(Date.now() + 3600 * 1000), // 1 hour from now
      };

      expect(isTokenExpired(token)).toBe(false);
    });

    it("returns true for expired token", () => {
      const token: OAuthToken = {
        accessToken: "test",
        tokenType: "Bearer",
        expiresAt: new Date(Date.now() - 1000), // 1 second ago
      };

      expect(isTokenExpired(token)).toBe(true);
    });

    it("returns true for token expiring within buffer", () => {
      const token: OAuthToken = {
        accessToken: "test",
        tokenType: "Bearer",
        expiresAt: new Date(Date.now() + 30 * 1000), // 30 seconds from now
      };

      // Default buffer is 60 seconds
      expect(isTokenExpired(token)).toBe(true);
    });

    it("respects custom buffer", () => {
      const token: OAuthToken = {
        accessToken: "test",
        tokenType: "Bearer",
        expiresAt: new Date(Date.now() + 30 * 1000), // 30 seconds from now
      };

      // 10 second buffer - should not be expired yet
      expect(isTokenExpired(token, 10)).toBe(false);

      // 60 second buffer - should be considered expired
      expect(isTokenExpired(token, 60)).toBe(true);
    });
  });
});
