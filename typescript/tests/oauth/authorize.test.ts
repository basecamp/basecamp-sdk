/**
 * Tests for OAuth authorization URL construction.
 */

import { describe, it, expect } from "vitest";
import { buildAuthorizationUrl } from "../../src/oauth/authorize.js";
import { BasecampError } from "../../src/errors.js";

describe("buildAuthorizationUrl", () => {
  const baseParams = {
    authorizationEndpoint: "https://launchpad.37signals.com/authorization/new",
    clientId: "my_client_id",
    redirectUri: "http://localhost:14923/callback",
    state: "random_state_value",
  };

  it("constructs URL with required parameters", () => {
    const url = buildAuthorizationUrl(baseParams);

    expect(url.origin).toBe("https://launchpad.37signals.com");
    expect(url.pathname).toBe("/authorization/new");
    expect(url.searchParams.get("response_type")).toBe("code");
    expect(url.searchParams.get("client_id")).toBe("my_client_id");
    expect(url.searchParams.get("redirect_uri")).toBe("http://localhost:14923/callback");
    expect(url.searchParams.get("state")).toBe("random_state_value");
  });

  it("includes PKCE parameters when provided", () => {
    const url = buildAuthorizationUrl({
      ...baseParams,
      pkce: { verifier: "test_verifier", challenge: "test_challenge" },
    });

    expect(url.searchParams.get("code_challenge")).toBe("test_challenge");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
  });

  it("does not include PKCE params when pkce is absent", () => {
    const url = buildAuthorizationUrl(baseParams);

    expect(url.searchParams.has("code_challenge")).toBe(false);
    expect(url.searchParams.has("code_challenge_method")).toBe(false);
  });

  it("includes scope when provided", () => {
    const url = buildAuthorizationUrl({
      ...baseParams,
      scope: "read write",
    });

    expect(url.searchParams.get("scope")).toBe("read write");
  });

  it("does not include scope when absent", () => {
    const url = buildAuthorizationUrl(baseParams);
    expect(url.searchParams.has("scope")).toBe(false);
  });

  it("properly encodes special characters in parameters", () => {
    const url = buildAuthorizationUrl({
      ...baseParams,
      clientId: "client with spaces & special=chars",
      state: "state/with+special",
    });

    // URL should be valid and params should be properly encoded
    expect(url.searchParams.get("client_id")).toBe("client with spaces & special=chars");
    expect(url.searchParams.get("state")).toBe("state/with+special");
  });

  it("allows HTTP for localhost endpoints", () => {
    const url = buildAuthorizationUrl({
      ...baseParams,
      authorizationEndpoint: "http://localhost:3000/auth",
    });

    expect(url.protocol).toBe("http:");
    expect(url.hostname).toBe("localhost");
  });

  it("allows HTTP for 127.0.0.1 endpoints", () => {
    const url = buildAuthorizationUrl({
      ...baseParams,
      authorizationEndpoint: "http://127.0.0.1:3000/auth",
    });

    expect(url.protocol).toBe("http:");
  });

  it("throws BasecampError for non-HTTPS non-localhost endpoints", () => {
    expect(() =>
      buildAuthorizationUrl({
        ...baseParams,
        authorizationEndpoint: "http://example.com/auth",
      })
    ).toThrow(BasecampError);
  });

  it("throws BasecampError for invalid URLs", () => {
    expect(() =>
      buildAuthorizationUrl({
        ...baseParams,
        authorizationEndpoint: "not a url",
      })
    ).toThrow(BasecampError);
  });

  it("returns a URL instance", () => {
    const url = buildAuthorizationUrl(baseParams);
    expect(url).toBeInstanceOf(URL);
  });
});
