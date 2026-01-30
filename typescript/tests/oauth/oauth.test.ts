/**
 * OAuth module tests.
 *
 * Tests discovery, token exchange, and token refresh functionality.
 */

import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
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

    it("normalizes trailing slash in base URL", async () => {
      server.use(
        http.get(
          "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
          () => HttpResponse.json(mockDiscoveryResponse)
        )
      );

      const config = await discover("https://launchpad.37signals.com/");

      expect(config.issuer).toBe("https://launchpad.37signals.com");
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
        expect((err as BasecampError).code).toBe("network");
        expect((err as BasecampError).httpStatus).toBe(404);
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
        expect((err as BasecampError).code).toBe("auth");
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
        expect((err as BasecampError).code).toBe("auth");
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
