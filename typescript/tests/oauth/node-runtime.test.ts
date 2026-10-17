/**
 * Tests for the Node-only guard on the OAuth helpers that use Node built-ins.
 *
 * Outside Node, performInteractiveLogin, startCallbackServer and FileTokenStore
 * must fail at once with a `usage` error that names the cause, instead of
 * reaching discovery (a network error) or a stubbed built-in (a missing
 * function).
 */

import { describe, it, expect, vi, afterEach } from "vitest";
import { isNodeRuntime } from "../../src/oauth/node-runtime.js";
import { performInteractiveLogin } from "../../src/oauth/interactive-login.js";
import { startCallbackServer } from "../../src/oauth/callback-server.js";
import { FileTokenStore } from "../../src/oauth/token-store.js";
import { BasecampError } from "../../src/errors.js";

// What a React Native or browser bundle sees: no `process` at all, or the
// common `process` polyfill, which has an empty `versions`.
const RUNTIMES: Array<[string, unknown]> = [
  ["no process global (browser)", undefined],
  ["process polyfill without versions.node (React Native)", { env: { NODE_ENV: "production" }, versions: {} }],
];

/**
 * Runs `fn` with `globalThis.process` replaced. The guards are synchronous and
 * run before the first await, so the replacement only has to cover the call
 * itself; it is restored before anything else (vitest, msw) runs.
 */
function underRuntime<T>(proc: unknown, fn: () => T): T {
  const saved = Object.getOwnPropertyDescriptor(globalThis, "process")!;
  Object.defineProperty(globalThis, "process", { value: proc, configurable: true, writable: true });
  try {
    return fn();
  } finally {
    Object.defineProperty(globalThis, "process", saved);
  }
}

function expectNodeOnlyError(err: unknown, feature: string): void {
  expect(err).toBeInstanceOf(BasecampError);
  const e = err as BasecampError;
  expect(e.code).toBe("usage");
  expect(e.message).toBe(`${feature} requires Node.js and cannot run in React Native or a browser`);
  expect(e.hint).toContain('"React Native and browsers"');
}

describe("Node-only OAuth helpers", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("detects Node under the test runner", () => {
    expect(isNodeRuntime()).toBe(true);
  });

  for (const [label, proc] of RUNTIMES) {
    describe(label, () => {
      it("is not reported as Node", () => {
        expect(underRuntime(proc, () => isNodeRuntime())).toBe(false);
      });

      it("performInteractiveLogin rejects before discovery or opening a browser", async () => {
        const fetchSpy = vi.spyOn(globalThis, "fetch");
        const openBrowser = vi.fn(async () => {});
        const store = { load: vi.fn(async () => null), save: vi.fn(async () => {}), clear: vi.fn(async () => {}) };

        const pending = underRuntime(proc, () =>
          performInteractiveLogin({ clientId: "test_client_id", store, openBrowser })
        );

        const err = await pending.catch((e: unknown) => e);
        expectNodeOnlyError(err, "performInteractiveLogin (local HTTP callback server)");
        expect(fetchSpy).not.toHaveBeenCalled();
        expect(openBrowser).not.toHaveBeenCalled();
        expect(store.save).not.toHaveBeenCalled();
      });

      it("startCallbackServer rejects without listening", async () => {
        const pending = underRuntime(proc, () => startCallbackServer({ expectedState: "s" }));

        const err = await pending.catch((e: unknown) => e);
        expectNodeOnlyError(err, "startCallbackServer (local HTTP callback server)");
      });

      it("FileTokenStore throws from its constructor", () => {
        let err: unknown;
        underRuntime(proc, () => {
          try {
            new FileTokenStore("/tmp/never-written.json");
          } catch (e) {
            err = e;
          }
        });
        expectNodeOnlyError(err, "FileTokenStore (file-based token storage)");
      });
    });
  }
});
