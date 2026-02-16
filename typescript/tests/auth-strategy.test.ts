/**
 * AuthStrategy tests
 */
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { BearerAuth, bearerAuth } from "../src/auth-strategy.js";
import type { AuthStrategy } from "../src/auth-strategy.js";
import { createBasecampClient } from "../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("BearerAuth", () => {
  it("sets Authorization header with static token", async () => {
    const auth = bearerAuth("my-token");
    const headers = new Headers();
    await auth.authenticate(headers);
    expect(headers.get("Authorization")).toBe("Bearer my-token");
  });

  it("sets Authorization header with dynamic token", async () => {
    const auth = bearerAuth(async () => "dynamic-token");
    const headers = new Headers();
    await auth.authenticate(headers);
    expect(headers.get("Authorization")).toBe("Bearer dynamic-token");
  });

  it("is an instance of BearerAuth", () => {
    const auth = bearerAuth("token");
    expect(auth).toBeInstanceOf(BearerAuth);
  });
});

describe("Custom AuthStrategy", () => {
  it("can implement cookie-based auth", async () => {
    const cookieAuth: AuthStrategy = {
      async authenticate(headers: Headers) {
        headers.set("Cookie", "session=abc123");
      },
    };
    const headers = new Headers();
    await cookieAuth.authenticate(headers);
    expect(headers.get("Cookie")).toBe("session=abc123");
    expect(headers.get("Authorization")).toBeNull();
  });

  it("works with createBasecampClient via auth option", async () => {
    let capturedRequest: Request | null = null;

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        capturedRequest = request;
        return HttpResponse.json([]);
      })
    );

    const customAuth: AuthStrategy = {
      async authenticate(headers: Headers) {
        headers.set("X-Custom-Auth", "custom-value");
      },
    };

    const client = createBasecampClient({
      accountId: "12345",
      auth: customAuth,
    });

    await client.GET("/projects.json");

    expect(capturedRequest?.headers.get("X-Custom-Auth")).toBe("custom-value");
    expect(capturedRequest?.headers.get("Authorization")).toBeNull();
  });
});

describe("createBasecampClient auth validation", () => {
  it("throws when neither auth nor accessToken is provided", () => {
    expect(() =>
      createBasecampClient({ accountId: "12345" })
    ).toThrow("Either 'auth' or 'accessToken' is required");
  });

  it("throws when both auth and accessToken are provided", () => {
    expect(() =>
      createBasecampClient({
        accountId: "12345",
        accessToken: "token",
        auth: bearerAuth("token"),
      })
    ).toThrow("Provide either 'auth' or 'accessToken', not both");
  });

  it("accepts accessToken for backward compatibility", async () => {
    let capturedRequest: Request | null = null;

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        capturedRequest = request;
        return HttpResponse.json([]);
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "compat-token",
    });

    await client.GET("/projects.json");

    expect(capturedRequest?.headers.get("Authorization")).toBe(
      "Bearer compat-token"
    );
  });

  it("accepts auth option with BearerAuth", async () => {
    let capturedRequest: Request | null = null;

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        capturedRequest = request;
        return HttpResponse.json([]);
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      auth: bearerAuth("auth-option-token"),
    });

    await client.GET("/projects.json");

    expect(capturedRequest?.headers.get("Authorization")).toBe(
      "Bearer auth-option-token"
    );
  });
});
