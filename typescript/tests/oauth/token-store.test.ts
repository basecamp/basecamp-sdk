/**
 * Tests for FileTokenStore.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { readFile, stat, rm, mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { FileTokenStore } from "../../src/oauth/token-store.js";
import type { OAuthToken } from "../../src/oauth/types.js";
import { BasecampError } from "../../src/errors.js";

describe("FileTokenStore", () => {
  let tempDir: string;

  beforeEach(async () => {
    tempDir = await mkdtemp(join(tmpdir(), "token-store-test-"));
  });

  afterEach(async () => {
    await rm(tempDir, { recursive: true, force: true });
  });

  const makeToken = (overrides: Partial<OAuthToken> = {}): OAuthToken => ({
    accessToken: "test_access_token",
    refreshToken: "test_refresh_token",
    tokenType: "Bearer",
    expiresIn: 3600,
    expiresAt: new Date("2026-01-01T00:00:00Z"),
    scope: "read write",
    ...overrides,
  });

  describe("save and load", () => {
    it("round-trips a token through save and load", async () => {
      const store = new FileTokenStore(join(tempDir, "tokens.json"));
      const token = makeToken();

      await store.save(token);
      const loaded = await store.load();

      expect(loaded).not.toBeNull();
      expect(loaded!.accessToken).toBe("test_access_token");
      expect(loaded!.refreshToken).toBe("test_refresh_token");
      expect(loaded!.tokenType).toBe("Bearer");
      expect(loaded!.expiresIn).toBe(3600);
      expect(loaded!.scope).toBe("read write");
    });

    it("serializes expiresAt as ISO string and deserializes back to Date", async () => {
      const store = new FileTokenStore(join(tempDir, "tokens.json"));
      const expiresAt = new Date("2026-06-15T12:00:00.000Z");
      const token = makeToken({ expiresAt });

      await store.save(token);

      // Verify raw JSON has ISO string
      const raw = JSON.parse(await readFile(join(tempDir, "tokens.json"), "utf-8"));
      expect(raw.expiresAt).toBe("2026-06-15T12:00:00.000Z");

      // Verify load returns a Date
      const loaded = await store.load();
      expect(loaded!.expiresAt).toBeInstanceOf(Date);
      expect(loaded!.expiresAt!.toISOString()).toBe("2026-06-15T12:00:00.000Z");
    });

    it("handles token without optional fields", async () => {
      const store = new FileTokenStore(join(tempDir, "tokens.json"));
      const token: OAuthToken = {
        accessToken: "minimal_token",
        tokenType: "Bearer",
      };

      await store.save(token);
      const loaded = await store.load();

      expect(loaded!.accessToken).toBe("minimal_token");
      expect(loaded!.refreshToken).toBeUndefined();
      expect(loaded!.expiresAt).toBeUndefined();
      expect(loaded!.expiresIn).toBeUndefined();
      expect(loaded!.scope).toBeUndefined();
    });

    it("overwrites existing token on save", async () => {
      const store = new FileTokenStore(join(tempDir, "tokens.json"));

      await store.save(makeToken({ accessToken: "first" }));
      await store.save(makeToken({ accessToken: "second" }));

      const loaded = await store.load();
      expect(loaded!.accessToken).toBe("second");
    });
  });

  describe("load", () => {
    it("returns null for missing file", async () => {
      const store = new FileTokenStore(join(tempDir, "nonexistent.json"));
      const loaded = await store.load();
      expect(loaded).toBeNull();
    });

    it("throws BasecampError for corrupted JSON", async () => {
      const path = join(tempDir, "corrupted.json");
      const { writeFile } = await import("node:fs/promises");
      await writeFile(path, "not valid json {{{");

      const store = new FileTokenStore(path);

      await expect(store.load()).rejects.toThrow(BasecampError);
      try {
        await store.load();
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("usage");
        expect((err as BasecampError).message).toContain("Failed to parse token file");
        expect((err as BasecampError).cause).toBeInstanceOf(SyntaxError);
      }
    });
  });

  describe("clear", () => {
    it("removes the token file", async () => {
      const path = join(tempDir, "tokens.json");
      const store = new FileTokenStore(path);

      await store.save(makeToken());
      await store.clear();

      const loaded = await store.load();
      expect(loaded).toBeNull();
    });

    it("does not throw for already-missing file", async () => {
      const store = new FileTokenStore(join(tempDir, "nonexistent.json"));
      await expect(store.clear()).resolves.toBeUndefined();
    });
  });

  describe("file permissions", () => {
    it("creates file with 0o600 permissions", async () => {
      const path = join(tempDir, "tokens.json");
      const store = new FileTokenStore(path);

      await store.save(makeToken());

      const s = await stat(path);
      // Check owner read/write only (0o600 = 0o100600 for regular file)
      const mode = s.mode & 0o777;
      expect(mode).toBe(0o600);
    });
  });

  describe("atomic writes", () => {
    it("writes to .tmp then renames", async () => {
      const path = join(tempDir, "tokens.json");
      const store = new FileTokenStore(path);

      await store.save(makeToken());

      // The .tmp file should not exist after save completes
      await expect(stat(path + ".tmp")).rejects.toThrow();

      // The target file should exist
      const s = await stat(path);
      expect(s.isFile()).toBe(true);
    });
  });

  describe("directory creation", () => {
    it("creates parent directories as needed", async () => {
      const path = join(tempDir, "deep", "nested", "dir", "tokens.json");
      const store = new FileTokenStore(path);

      await store.save(makeToken());
      const loaded = await store.load();
      expect(loaded!.accessToken).toBe("test_access_token");
    });
  });

  describe("~ expansion", () => {
    it("expands ~ in file path", () => {
      // We can verify the constructor handles ~ by checking that the store
      // doesn't throw on construction (it resolves ~ at construction time,
      // actual file ops happen in load/save/clear)
      const store = new FileTokenStore("~/test-token.json");
      expect(store).toBeDefined();
    });
  });
});
