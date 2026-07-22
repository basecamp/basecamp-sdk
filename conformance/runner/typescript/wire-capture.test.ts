/**
 * Offline tests for the wire-capture layer.
 *
 * The capture wraps globalThis.fetch, so these tests stub the global with
 * scripted responses — no network, no MSW. The retry-collapse behavior is
 * exercised both at the capture layer directly and through the real SDK
 * retry middleware (which re-issues the identical request through global
 * fetch on 429, so each attempt hits the capture wrapper).
 */

import { describe, it, expect, afterEach } from "vitest";
import { createBasecampClient } from "@37signals/basecamp";

import { installWireCapture } from "./wire-capture.js";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function jsonResponse(
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

describe("installWireCapture — retry collapse", () => {
  it("keeps only the final attempt when consecutive captures re-fetch the same URL", async () => {
    const url = "https://example.test/999/projects/1.json";
    const responses = [
      jsonResponse(429, { error: "rate limited" }, { "Retry-After": "0" }),
      jsonResponse(200, { id: 1 }),
    ];
    globalThis.fetch = async () => responses.shift()!;

    const capture = installWireCapture();
    try {
      // Simulate the retry middleware's behavior: same URL fetched twice.
      await fetch(url);
      await fetch(url);
    } finally {
      capture.restore();
    }

    const snapshot = capture.drain();
    expect(snapshot.pages_count).toBe(1);
    expect(snapshot.pages[0].status).toBe(200);
    expect(snapshot.pages[0].body).toEqual({ id: 1 });
  });

  it("preserves distinct URLs as separate pages (pagination unaffected)", async () => {
    const bodies: Record<string, unknown> = {
      "https://example.test/999/projects.json": [{ id: 1 }],
      "https://example.test/999/projects.json?page=2": [{ id: 2 }],
    };
    globalThis.fetch = async (input) => jsonResponse(200, bodies[String(input)]);

    const capture = installWireCapture();
    try {
      await fetch("https://example.test/999/projects.json");
      await fetch("https://example.test/999/projects.json?page=2");
    } finally {
      capture.restore();
    }

    const snapshot = capture.drain();
    expect(snapshot.pages_count).toBe(2);
    expect(snapshot.pages[0].body).toEqual([{ id: 1 }]);
    expect(snapshot.pages[1].body).toEqual([{ id: 2 }]);
  });

  it("records one 200 page when the SDK retry middleware recovers from a 429", async () => {
    let calls = 0;
    globalThis.fetch = async () => {
      calls++;
      return calls === 1
        ? jsonResponse(429, { error: "slow down" }, { "Retry-After": "0" })
        : jsonResponse(200, { id: 42, name: "Canary Person" });
    };

    const capture = installWireCapture();
    let me: { id: number | string };
    try {
      const client = createBasecampClient({
        accountId: "999",
        accessToken: "test-token",
        baseUrl: "http://localhost:9876/999",
        enableRetry: true,
      });
      me = await client.people.me();
    } finally {
      capture.restore();
    }

    // The retry middleware must have re-entered global fetch for attempt 2,
    // and the SDK call itself recovered.
    expect(calls).toBe(2);
    expect(me.id).toBe(42);

    // Pre-fix, the snapshot recorded both attempts and pages[0].status was
    // 429 — failing liveCallSucceeds despite the successful call.
    const snapshot = capture.drain();
    expect(snapshot.pages_count).toBe(1);
    expect(snapshot.pages[0].status).toBe(200);
  });
});
