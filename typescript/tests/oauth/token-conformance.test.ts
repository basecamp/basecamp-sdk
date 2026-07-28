/**
 * Drives the shared, data-only fixtures in conformance/oauth-token/fixtures:
 * one refresh round-trip per fixture, asserting the sent resource form
 * parameter and the response decode (round-trip, absent/null as unset,
 * present-empty/non-string rejected). Lifecycle preservation across a stored
 * credential is per-manager behavior, tested in token-manager.test.ts — not
 * here.
 */

import { readdirSync, readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { refreshToken } from "../../src/oauth/exchange.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const FIXTURE_DIR = join(HERE, "../../../conformance/oauth-token/fixtures");
const TOKEN_ENDPOINT = "https://issuer.token-fixtures.example/oauth/token";

interface TokenFixture {
  name: string;
  operation: string;
  request?: { resource?: string };
  response: { status?: number; body: Record<string, unknown> };
  expect: {
    outcome: "token" | "reject";
    resource?: string;
    resourceAbsent?: boolean;
    formResource?: string;
    formResourceAbsent?: boolean;
  };
}

const fixtureNames = readdirSync(FIXTURE_DIR)
  .filter((f) => f.endsWith(".json"))
  .sort();

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("conformance/oauth-token fixtures", () => {
  it("discovers at least one fixture", () => {
    // A proper test failure (not a describe-phase crash) if the fixture dir
    // moves or empties — it.each below silently runs zero cases otherwise.
    expect(fixtureNames.length).toBeGreaterThan(0);
  });

  it.each(fixtureNames)("%s", async (name) => {
    const fixture = JSON.parse(
      readFileSync(join(FIXTURE_DIR, name), "utf8")
    ) as TokenFixture;
    expect(fixture.operation).toBe("refreshToken");

    let sawResourceKey = false;
    let sentResource: string | null = null;
    server.use(
      http.post(TOKEN_ENDPOINT, async ({ request }) => {
        const params = new URLSearchParams(await request.text());
        sawResourceKey = params.has("resource");
        sentResource = params.get("resource");
        return HttpResponse.json(fixture.response.body, {
          status: fixture.response.status ?? 200,
        });
      })
    );

    const call = refreshToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      refreshToken: "refresh-token",
      clientId: "basecamp-cli",
      resource: fixture.request?.resource,
    });

    if (fixture.expect.outcome === "token") {
      const token = await call;
      if (fixture.expect.resource !== undefined) {
        expect(token.resource).toBe(fixture.expect.resource);
      }
      if (fixture.expect.resourceAbsent) {
        expect(token.resource).toBeUndefined();
      }
    } else {
      await expect(call).rejects.toMatchObject({ code: "api_error" });
    }

    if (fixture.expect.formResource !== undefined) {
      expect(sawResourceKey).toBe(true);
      expect(sentResource).toBe(fixture.expect.formResource);
    }
    if (fixture.expect.formResourceAbsent) {
      expect(sawResourceKey).toBe(false);
    }
  });
});
