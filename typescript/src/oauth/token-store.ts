/**
 * Token persistence for OAuth 2.0 tokens.
 *
 * Provides a file-based token store with atomic writes and secure permissions.
 */

import { readFile, writeFile, rename, unlink, mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { homedir } from "node:os";
import type { OAuthToken } from "./types.js";

/**
 * Interface for persisting OAuth tokens.
 */
export interface TokenStore {
  /** Load a previously saved token, or null if none exists */
  load(): Promise<OAuthToken | null>;
  /** Save a token for later retrieval */
  save(token: OAuthToken): Promise<void>;
  /** Remove the stored token */
  clear(): Promise<void>;
}

/**
 * Serialized token format for JSON persistence.
 */
interface SerializedToken {
  accessToken: string;
  refreshToken?: string;
  tokenType: string;
  expiresIn?: number;
  expiresAt?: string; // ISO 8601
  scope?: string;
}

/**
 * Resolves ~ to the user's home directory.
 */
function expandHome(filePath: string): string {
  if (filePath.startsWith("~/")) {
    return filePath.replace("~", homedir());
  }
  if (filePath === "~") {
    return homedir();
  }
  return filePath;
}

/**
 * File-based token store with atomic writes and secure permissions.
 *
 * Tokens are stored as JSON with 0o600 permissions (owner read/write only).
 * Writes are atomic: data is written to a temporary file then renamed.
 *
 * @example
 * ```ts
 * const store = new FileTokenStore("~/.config/basecamp/tokens.json");
 *
 * // Save a token
 * await store.save(token);
 *
 * // Load it back
 * const loaded = await store.load();
 *
 * // Clear when done
 * await store.clear();
 * ```
 */
export class FileTokenStore implements TokenStore {
  private readonly filePath: string;

  constructor(filePath: string) {
    this.filePath = expandHome(filePath);
  }

  async load(): Promise<OAuthToken | null> {
    let raw: string;
    try {
      raw = await readFile(this.filePath, "utf-8");
    } catch (err: unknown) {
      if (err instanceof Error && "code" in err && (err as NodeJS.ErrnoException).code === "ENOENT") {
        return null;
      }
      throw err;
    }

    const data = JSON.parse(raw) as SerializedToken;

    return {
      accessToken: data.accessToken,
      refreshToken: data.refreshToken,
      tokenType: data.tokenType,
      expiresIn: data.expiresIn,
      expiresAt: data.expiresAt ? new Date(data.expiresAt) : undefined,
      scope: data.scope,
    };
  }

  async save(token: OAuthToken): Promise<void> {
    const serialized: SerializedToken = {
      accessToken: token.accessToken,
      refreshToken: token.refreshToken,
      tokenType: token.tokenType,
      expiresIn: token.expiresIn,
      expiresAt: token.expiresAt?.toISOString(),
      scope: token.scope,
    };

    const json = JSON.stringify(serialized, null, 2) + "\n";
    const tmpPath = this.filePath + ".tmp";

    // Ensure directory exists
    await mkdir(dirname(this.filePath), { recursive: true });

    // Atomic write: write to tmp, then rename
    await writeFile(tmpPath, json, { mode: 0o600 });
    await rename(tmpPath, this.filePath);
  }

  async clear(): Promise<void> {
    try {
      await unlink(this.filePath);
    } catch (err: unknown) {
      if (err instanceof Error && "code" in err && (err as NodeJS.ErrnoException).code === "ENOENT") {
        return; // Already gone
      }
      throw err;
    }
  }
}
