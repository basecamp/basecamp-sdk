/**
 * Tests for interactive OAuth login flow.
 *
 * Uses MSW passthrough for localhost callback server requests.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { http as mswHttp, HttpResponse, passthrough } from "msw";
import { server } from "../setup.js";
import { performInteractiveLogin } from "../../src/oauth/interactive-login.js";
import { DiscoverySelectionError } from "../../src/oauth/discovery.js";
import type { TokenStore } from "../../src/oauth/token-store.js";
import type { OAuthToken } from "../../src/oauth/types.js";

const mockDiscoveryResponse = {
  issuer: "https://launchpad.37signals.com",
  authorization_endpoint: "https://launchpad.37signals.com/authorization/new",
  token_endpoint: "https://launchpad.37signals.com/authorization/token",
};

const mockDiscoveryResponseWithPKCE = {
  ...mockDiscoveryResponse,
  code_challenge_methods_supported: ["S256"],
};

const mockTokenResponse = {
  access_token: "new_access_token",
  refresh_token: "new_refresh_token",
  token_type: "Bearer",
  expires_in: 3600,
};

function createMockStore(): TokenStore {
  let stored: OAuthToken | null = null;
  return {
    load: vi.fn(async () => stored),
    save: vi.fn(async (token: OAuthToken) => { stored = token; }),
    clear: vi.fn(async () => { stored = null; }),
  };
}

describe("performInteractiveLogin", () => {
  beforeEach(() => {
    // Allow localhost requests to pass through to the real callback server
    server.use(
      mswHttp.get(/^http:\/\/localhost:\d+\/.*/, () => passthrough()),
    );
  });

  it("completes the full OAuth flow", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json(mockDiscoveryResponse)
      ),
      mswHttp.post(
        "https://launchpad.37signals.com/authorization/token",
        () => HttpResponse.json(mockTokenResponse)
      ),
    );

    const store = createMockStore();
    const statusMessages: string[] = [];

    const openBrowser = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      const callbackUrl = `${redirectUri}?code=test_auth_code&state=${state}`;
      await fetch(callbackUrl);
    });

    const token = await performInteractiveLogin({
      clientId: "test_client_id",
      clientSecret: "test_client_secret",
      store,
      openBrowser,
      onStatus: (msg) => statusMessages.push(msg),
    });

    expect(token.accessToken).toBe("new_access_token");
    expect(token.refreshToken).toBe("new_refresh_token");
    expect(openBrowser).toHaveBeenCalledOnce();
    expect(store.save).toHaveBeenCalledWith(token);
    expect(statusMessages).toContain("Authorization complete.");
  });

  it("falls back to manual visit when browser fails", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json(mockDiscoveryResponse)
      ),
      mswHttp.post(
        "https://launchpad.37signals.com/authorization/token",
        () => HttpResponse.json(mockTokenResponse)
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async () => { throw new Error("no browser"); });
    const promptForManualVisit = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      const callbackUrl = `${redirectUri}?code=manual_code&state=${state}`;
      await fetch(callbackUrl);
    });

    const token = await performInteractiveLogin({
      clientId: "test_client_id",
      store,
      openBrowser,
      promptForManualVisit,
    });

    expect(token.accessToken).toBe("new_access_token");
    expect(promptForManualVisit).toHaveBeenCalledOnce();
  });

  it("throws when browser fails and no manual visit prompt", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json(mockDiscoveryResponse)
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async () => { throw new Error("no browser"); });

    await expect(
      performInteractiveLogin({
        clientId: "test_client_id",
        store,
        openBrowser,
      })
    ).rejects.toThrow(/Failed to open browser/);
  });

  it("throws a typed capability_unavailable when the AS omits authorization_endpoint", async () => {
    // A device-only AS advertises no authorization_endpoint. The consumer-side
    // per-grant check must raise the typed DiscoverySelectionError reason, not a
    // generic validation error, so callers can distinguish it across SDKs.
    server.use(
      mswHttp.get(
        "https://device-only.example.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json({
          issuer: "https://device-only.example.com",
          token_endpoint: "https://device-only.example.com/token",
          device_authorization_endpoint: "https://device-only.example.com/device",
        })
      ),
    );

    const store = createMockStore();
    let caught: unknown;
    try {
      await performInteractiveLogin({
        clientId: "test_client_id",
        store,
        baseUrl: "https://device-only.example.com",
        openBrowser: vi.fn(),
      });
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(DiscoverySelectionError);
    expect((caught as DiscoverySelectionError).reason).toBe("capability_unavailable");
  });

  it("uses custom baseUrl for discovery", async () => {
    server.use(
      mswHttp.get(
        "https://custom.example.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json({
          ...mockDiscoveryResponse,
          // Issuer must bind to the host the metadata was fetched from (RFC 8414).
          issuer: "https://custom.example.com",
          authorization_endpoint: "https://custom.example.com/authorize",
          token_endpoint: "https://custom.example.com/token",
        })
      ),
      mswHttp.post(
        "https://custom.example.com/token",
        () => HttpResponse.json(mockTokenResponse)
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      expect(authUrl.origin).toBe("https://custom.example.com");

      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      await fetch(`${redirectUri}?code=custom_code&state=${state}`);
    });

    const token = await performInteractiveLogin({
      clientId: "test_client_id",
      store,
      baseUrl: "https://custom.example.com",
      openBrowser,
    });

    expect(token.accessToken).toBe("new_access_token");
  });

  it("includes PKCE code_verifier in token exchange", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json(mockDiscoveryResponseWithPKCE)
      ),
      mswHttp.post(
        "https://launchpad.37signals.com/authorization/token",
        async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);
          expect(params.get("code_verifier")).toBeTruthy();
          return HttpResponse.json(mockTokenResponse);
        }
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      expect(authUrl.searchParams.get("code_challenge")).toBeTruthy();
      expect(authUrl.searchParams.get("code_challenge_method")).toBe("S256");

      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      await fetch(`${redirectUri}?code=pkce_code&state=${state}`);
    });

    await performInteractiveLogin({
      clientId: "test_client_id",
      store,
      openBrowser,
    });
  });

  it("omits PKCE when server does not advertise S256", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json(mockDiscoveryResponse)
      ),
      mswHttp.post(
        "https://launchpad.37signals.com/authorization/token",
        async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);
          expect(params.has("code_verifier")).toBe(false);
          return HttpResponse.json(mockTokenResponse);
        }
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      expect(authUrl.searchParams.has("code_challenge")).toBe(false);
      expect(authUrl.searchParams.has("code_challenge_method")).toBe(false);

      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      await fetch(`${redirectUri}?code=no_pkce_code&state=${state}`);
    });

    await performInteractiveLogin({
      clientId: "test_client_id",
      store,
      openBrowser,
    });
  });

  it("omits PKCE when server advertises only unsupported methods", async () => {
    server.use(
      mswHttp.get(
        "https://launchpad.37signals.com/.well-known/oauth-authorization-server",
        () => HttpResponse.json({
          ...mockDiscoveryResponse,
          code_challenge_methods_supported: ["plain"],
        })
      ),
      mswHttp.post(
        "https://launchpad.37signals.com/authorization/token",
        async ({ request }) => {
          const body = await request.text();
          const params = new URLSearchParams(body);
          expect(params.has("code_verifier")).toBe(false);
          return HttpResponse.json(mockTokenResponse);
        }
      ),
    );

    const store = createMockStore();
    const openBrowser = vi.fn(async (url: string) => {
      const authUrl = new URL(url);
      expect(authUrl.searchParams.has("code_challenge")).toBe(false);
      expect(authUrl.searchParams.has("code_challenge_method")).toBe(false);

      const redirectUri = authUrl.searchParams.get("redirect_uri")!;
      const state = authUrl.searchParams.get("state")!;
      await fetch(`${redirectUri}?code=plain_only_code&state=${state}`);
    });

    await performInteractiveLogin({
      clientId: "test_client_id",
      store,
      openBrowser,
    });
  });
});

describe("performInteractiveLogin — resource-first fatal-after-selection", () => {
  const RESOURCE = "https://api.basecamp-test.example";
  const BC5 = "https://bc5.basecamp-test.example";
  const LAUNCHPAD = "https://launchpad.37signals.com";

  let launchpadContacted = false;

  beforeEach(() => {
    launchpadContacted = false;
    server.use(
      mswHttp.get(`${LAUNCHPAD}/.well-known/oauth-authorization-server`, () => {
        launchpadContacted = true;
        return HttpResponse.json({
          issuer: LAUNCHPAD,
          authorization_endpoint: `${LAUNCHPAD}/authorization/new`,
          token_endpoint: `${LAUNCHPAD}/authorization/token`,
        });
      }),
    );
  });

  function advertiseBc5() {
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: RESOURCE, authorization_servers: [BC5, LAUNCHPAD] }),
      ),
    );
  }

  it("rejects and never contacts Launchpad when the committed AS returns 500", async () => {
    advertiseBc5();
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ error: "boom" }, { status: 500 }),
      ),
    );

    const openBrowser = vi.fn(async () => {});
    await expect(
      performInteractiveLogin({
        clientId: "basecamp-cli",
        store: createMockStore(),
        resourceBaseUrl: RESOURCE,
        openBrowser,
      }),
    ).rejects.toMatchObject({ code: "api_error" });

    expect(openBrowser).not.toHaveBeenCalled();
    expect(launchpadContacted).toBe(false);
  });

  it("rejects and never contacts Launchpad on an issuer-binding mismatch", async () => {
    advertiseBc5();
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({
          issuer: "https://impostor.example.com",
          authorization_endpoint: `${BC5}/oauth/authorize`,
          token_endpoint: `${BC5}/oauth/token`,
        }),
      ),
    );

    const openBrowser = vi.fn(async () => {});
    await expect(
      performInteractiveLogin({
        clientId: "basecamp-cli",
        store: createMockStore(),
        resourceBaseUrl: RESOURCE,
        openBrowser,
      }),
    ).rejects.toMatchObject({ code: "api_error" });

    expect(openBrowser).not.toHaveBeenCalled();
    expect(launchpadContacted).toBe(false);
  });

  it("rejects when baseUrl and resourceBaseUrl are both supplied", async () => {
    await expect(
      performInteractiveLogin({
        clientId: "basecamp-cli",
        store: createMockStore(),
        baseUrl: LAUNCHPAD,
        resourceBaseUrl: RESOURCE,
        openBrowser: vi.fn(async () => {}),
      }),
    ).rejects.toMatchObject({ code: "usage" });
  });
});
