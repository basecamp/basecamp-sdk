/**
 * Tests for TokenManager.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { TokenManager } from "../../src/oauth/token-manager.js";
import type { TokenStore } from "../../src/oauth/token-store.js";
import type { OAuthToken, RefreshRequest } from "../../src/oauth/types.js";

function createMockStore(initialToken: OAuthToken | null = null): TokenStore {
  let stored = initialToken;
  return {
    load: vi.fn(async () => stored),
    save: vi.fn(async (token: OAuthToken) => { stored = token; }),
    clear: vi.fn(async () => { stored = null; }),
  };
}

function freshToken(overrides: Partial<OAuthToken> = {}): OAuthToken {
  return {
    accessToken: "fresh_access_token",
    refreshToken: "fresh_refresh_token",
    tokenType: "Bearer",
    expiresIn: 3600,
    expiresAt: new Date(Date.now() + 3600 * 1000),
    ...overrides,
  };
}

function expiredToken(overrides: Partial<OAuthToken> = {}): OAuthToken {
  return {
    accessToken: "expired_access_token",
    refreshToken: "the_refresh_token",
    tokenType: "Bearer",
    expiresIn: 3600,
    expiresAt: new Date(Date.now() - 1000),
    ...overrides,
  };
}

describe("TokenManager", () => {
  let mockRefreshFn: ReturnType<typeof vi.fn<(req: RefreshRequest) => Promise<OAuthToken>>>;

  beforeEach(() => {
    mockRefreshFn = vi.fn(async () => freshToken());
  });

  describe("getToken", () => {
    it("loads from store on first call", async () => {
      const token = freshToken();
      const store = createMockStore(token);
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      const result = await manager.getToken();

      expect(result).toBe("fresh_access_token");
      expect(store.load).toHaveBeenCalledOnce();
    });

    it("does not load from store again on subsequent calls", async () => {
      const store = createMockStore(freshToken());
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await manager.getToken();
      await manager.getToken();

      expect(store.load).toHaveBeenCalledOnce();
    });

    it("returns cached token when not expired", async () => {
      const store = createMockStore(freshToken());
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      const result = await manager.getToken();

      expect(result).toBe("fresh_access_token");
      expect(mockRefreshFn).not.toHaveBeenCalled();
    });

    it("refreshes expired token automatically", async () => {
      const store = createMockStore(expiredToken());
      const refreshed = freshToken({ accessToken: "refreshed_token" });
      mockRefreshFn.mockResolvedValue(refreshed);

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      const result = await manager.getToken();

      expect(result).toBe("refreshed_token");
      expect(mockRefreshFn).toHaveBeenCalledOnce();
      expect(store.save).toHaveBeenCalledWith(refreshed);
    });

    it("refreshes token within buffer window", async () => {
      // Token expires in 60 seconds, buffer is 120 seconds
      const token = freshToken({
        expiresAt: new Date(Date.now() + 60_000),
      });
      const store = createMockStore(token);
      mockRefreshFn.mockResolvedValue(freshToken({ accessToken: "within_buffer" }));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
        bufferSeconds: 120,
      });

      const result = await manager.getToken();

      expect(result).toBe("within_buffer");
      expect(mockRefreshFn).toHaveBeenCalled();
    });

    it("respects custom bufferSeconds", async () => {
      // Token expires in 60 seconds, buffer is only 10 seconds
      const token = freshToken({
        expiresAt: new Date(Date.now() + 60_000),
      });
      const store = createMockStore(token);

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
        bufferSeconds: 10,
      });

      const result = await manager.getToken();

      expect(result).toBe("fresh_access_token");
      expect(mockRefreshFn).not.toHaveBeenCalled();
    });

    it("passes correct parameters to refresh function", async () => {
      const store = createMockStore(expiredToken());
      mockRefreshFn.mockResolvedValue(freshToken());

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
        clientId: "my_client",
        clientSecret: "my_secret",
        useLegacyFormat: true,
      });

      await manager.getToken();

      expect(mockRefreshFn).toHaveBeenCalledWith({
        tokenEndpoint: "https://example.com/token",
        refreshToken: "the_refresh_token",
        clientId: "my_client",
        clientSecret: "my_secret",
        useLegacyFormat: true,
      });
    });
  });

  describe("forceRefresh", () => {
    it("refreshes even when token is not expired", async () => {
      const store = createMockStore(freshToken());
      mockRefreshFn.mockResolvedValue(freshToken({ accessToken: "forced_refresh" }));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      // First call loads from store
      await manager.getToken();

      const result = await manager.forceRefresh();

      expect(result.accessToken).toBe("forced_refresh");
      expect(mockRefreshFn).toHaveBeenCalledOnce();
    });

    it("throws when no refresh token available", async () => {
      const store = createMockStore(freshToken({ refreshToken: undefined }));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await manager.getToken();

      await expect(manager.forceRefresh()).rejects.toThrow("No refresh token available");
    });
  });

  describe("concurrent refresh deduplication", () => {
    it("deduplicates concurrent refresh calls", async () => {
      const store = createMockStore(expiredToken());
      let resolveRefresh!: (token: OAuthToken) => void;
      const pendingRefresh = new Promise<OAuthToken>((resolve) => {
        resolveRefresh = resolve;
      });
      mockRefreshFn.mockReturnValue(pendingRefresh);

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      // Fire three concurrent getToken calls
      const p1 = manager.getToken();
      const p2 = manager.getToken();
      const p3 = manager.getToken();

      // Resolve the pending refresh
      resolveRefresh(freshToken({ accessToken: "deduped" }));

      const [r1, r2, r3] = await Promise.all([p1, p2, p3]);

      expect(r1).toBe("deduped");
      expect(r2).toBe("deduped");
      expect(r3).toBe("deduped");
      expect(mockRefreshFn).toHaveBeenCalledOnce();
    });

    it("allows new refresh after previous one completes", async () => {
      const store = createMockStore(expiredToken());
      mockRefreshFn
        .mockResolvedValueOnce(expiredToken({ accessToken: "first_refresh" }))
        .mockResolvedValueOnce(freshToken({ accessToken: "second_refresh" }));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await manager.getToken(); // first refresh
      const result = await manager.getToken(); // second refresh (still expired)

      expect(result).toBe("second_refresh");
      expect(mockRefreshFn).toHaveBeenCalledTimes(2);
    });

    it("clears in-flight promise on refresh failure", async () => {
      const store = createMockStore(expiredToken());
      mockRefreshFn
        .mockRejectedValueOnce(new Error("network error"))
        .mockResolvedValueOnce(freshToken({ accessToken: "retry_success" }));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await expect(manager.getToken()).rejects.toThrow("network error");

      // Should try again, not return the cached rejection
      const result = await manager.getToken();
      expect(result).toBe("retry_success");
      expect(mockRefreshFn).toHaveBeenCalledTimes(2);
    });
  });

  describe("isExpired", () => {
    it("returns true when no token is loaded", () => {
      const store = createMockStore(null);
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      expect(manager.isExpired()).toBe(true);
    });

    it("returns false for fresh token after getToken()", async () => {
      const store = createMockStore(freshToken());
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await manager.getToken();

      expect(manager.isExpired()).toBe(false);
    });

    it("returns true for expired token after getToken() loads it", async () => {
      // Token expires in 30s, buffer is 120s, so it's "expired"
      const token = freshToken({ expiresAt: new Date(Date.now() + 30_000) });
      const store = createMockStore(token);
      // Make refresh fail so the expired token stays in place
      mockRefreshFn.mockRejectedValue(new Error("fail"));

      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
        bufferSeconds: 120,
      });

      try { await manager.getToken(); } catch { /* expected */ }

      expect(manager.isExpired()).toBe(true);
    });
  });

  describe("currentToken", () => {
    it("returns null before first load", () => {
      const store = createMockStore(freshToken());
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      expect(manager.currentToken).toBeNull();
    });

    it("returns loaded token after getToken()", async () => {
      const token = freshToken();
      const store = createMockStore(token);
      const manager = new TokenManager({
        store,
        refreshToken: mockRefreshFn,
        tokenEndpoint: "https://example.com/token",
      });

      await manager.getToken();

      expect(manager.currentToken).not.toBeNull();
      expect(manager.currentToken!.accessToken).toBe("fresh_access_token");
    });
  });
});
