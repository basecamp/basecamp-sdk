/**
 * Tests for OAuth callback server.
 *
 * Uses MSW passthrough for localhost requests to the callback server.
 */

import { describe, it, expect, afterEach, beforeEach } from "vitest";
import { http, passthrough } from "msw";
import { server } from "../setup.js";
import { startCallbackServer } from "../../src/oauth/callback-server.js";

describe("startCallbackServer", () => {
  let closeServer: (() => void) | undefined;

  beforeEach(() => {
    // Allow all localhost requests to pass through to the real callback server
    server.use(
      http.get(/^http:\/\/localhost:\d+\/.*/, () => passthrough()),
    );
  });

  afterEach(() => {
    closeServer?.();
    closeServer = undefined;
  });

  it("starts server and returns callback URL", async () => {
    const { url, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    expect(url).toMatch(/^http:\/\/localhost:\d+\/callback$/);
    const port = parseInt(new URL(url).port);
    expect(port).toBeGreaterThan(0);
  });

  it("extracts code and state from successful callback", async () => {
    const { url, waitForCallback, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    const callbackUrl = `${url}?code=auth_code_123&state=test_state`;
    const response = await fetch(callbackUrl);

    expect(response.status).toBe(200);
    const html = await response.text();
    expect(html).toContain("Authorization complete");

    const result = await waitForCallback();
    expect(result.code).toBe("auth_code_123");
    expect(result.state).toBe("test_state");
  });

  it("rejects on state mismatch", async () => {
    const { url, waitForCallback, close } = await startCallbackServer({
      port: 0,
      expectedState: "expected_state",
    });
    closeServer = close;

    const callbackUrl = `${url}?code=auth_code&state=wrong_state`;
    const response = await fetch(callbackUrl);
    expect(response.status).toBe(400);

    await expect(waitForCallback()).rejects.toThrow(/state mismatch/);
  });

  it("returns 400 when code or state is missing", async () => {
    const { url, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    const r1 = await fetch(`${url}?state=test_state`);
    expect(r1.status).toBe(400);

    const r2 = await fetch(`${url}?code=auth_code`);
    expect(r2.status).toBe(400);
  });

  it("returns 404 for non-callback paths", async () => {
    const { url, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    const baseUrl = url.replace("/callback", "");
    const r = await fetch(`${baseUrl}/other`);
    expect(r.status).toBe(404);
  });

  it("handles OAuth error responses", async () => {
    const { url, waitForCallback, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    const callbackUrl = `${url}?error=access_denied&error_description=User+denied+access`;
    const response = await fetch(callbackUrl);
    expect(response.status).toBe(400);

    await expect(waitForCallback()).rejects.toThrow(/User denied access/);
  });

  it("times out after specified duration", async () => {
    const { waitForCallback, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
      timeoutMs: 50,
    });
    closeServer = close;

    await expect(waitForCallback()).rejects.toThrow(/timed out/);
  });

  it("rejects when closed before callback received", async () => {
    const { waitForCallback, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
      timeoutMs: 60_000,
    });

    const promise = waitForCallback();
    close();
    closeServer = undefined;

    await expect(promise).rejects.toThrow(/closed before receiving callback/);
  });

  it("throws on port bind failure", async () => {
    const { url, close } = await startCallbackServer({
      port: 0,
      expectedState: "test_state",
    });
    closeServer = close;

    const port = parseInt(new URL(url).port);

    await expect(
      startCallbackServer({
        port,
        expectedState: "test_state_2",
      })
    ).rejects.toThrow(/already in use/);
  });
});
