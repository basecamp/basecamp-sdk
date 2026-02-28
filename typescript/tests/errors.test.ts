/**
 * Tests for the errors module
 */
import { describe, it, expect } from "vitest";
import {
  BasecampError,
  Errors,
  errorFromResponse,
  isBasecampError,
  isErrorCode,
} from "../src/errors.js";

describe("BasecampError", () => {
  describe("constructor", () => {
    it("should create an error with required fields", () => {
      const error = new BasecampError("auth_required", "Test message");

      expect(error.name).toBe("BasecampError");
      expect(error.code).toBe("auth_required");
      expect(error.message).toBe("Test message");
      expect(error.retryable).toBe(false);
    });

    it("should create an error with all options", () => {
      const cause = new Error("Original error");
      const error = new BasecampError("rate_limit", "Rate limited", {
        hint: "Slow down",
        httpStatus: 429,
        retryable: true,
        retryAfter: 30,
        requestId: "req-123",
        cause,
      });

      expect(error.code).toBe("rate_limit");
      expect(error.hint).toBe("Slow down");
      expect(error.httpStatus).toBe(429);
      expect(error.retryable).toBe(true);
      expect(error.retryAfter).toBe(30);
      expect(error.requestId).toBe("req-123");
      expect(error.cause).toBe(cause);
    });
  });

  describe("exitCode", () => {
    it("should return correct exit codes for each error type", () => {
      const codes: Record<string, number> = {
        validation: 1,
        not_found: 2,
        auth_required: 3,
        forbidden: 4,
        rate_limit: 5,
        network: 6,
        api_error: 7,
      };

      for (const [code, expected] of Object.entries(codes)) {
        const error = new BasecampError(code as any, "test");
        expect(error.exitCode).toBe(expected);
      }
    });
  });

  describe("toJSON", () => {
    it("should serialize to JSON correctly", () => {
      const error = new BasecampError("not_found", "Todo not found", {
        hint: "Check the ID",
        httpStatus: 404,
        requestId: "req-456",
      });

      const json = error.toJSON();

      expect(json).toEqual({
        name: "BasecampError",
        code: "not_found",
        message: "Todo not found",
        hint: "Check the ID",
        httpStatus: 404,
        retryable: false,
        retryAfter: undefined,
        requestId: "req-456",
      });
    });
  });

  describe("instanceof check", () => {
    it("should be an instance of Error", () => {
      const error = new BasecampError("auth_required", "Test");
      expect(error).toBeInstanceOf(Error);
      expect(error).toBeInstanceOf(BasecampError);
    });
  });
});

describe("Errors factory", () => {
  describe("auth_required", () => {
    it("should create an auth error", () => {
      const error = Errors.auth();
      expect(error.code).toBe("auth_required");
      expect(error.httpStatus).toBe(401);
      expect(error.hint).toContain("access token");
    });

    it("should accept custom hint", () => {
      const error = Errors.auth("Custom hint");
      expect(error.hint).toBe("Custom hint");
    });

    it("should accept cause", () => {
      const cause = new Error("Original");
      const error = Errors.auth("hint", cause);
      expect(error.cause).toBe(cause);
    });
  });

  describe("forbidden", () => {
    it("should create a forbidden error", () => {
      const error = Errors.forbidden();
      expect(error.code).toBe("forbidden");
      expect(error.httpStatus).toBe(403);
    });
  });

  describe("notFound", () => {
    it("should create a not found error with resource name", () => {
      const error = Errors.notFound("Todo");
      expect(error.code).toBe("not_found");
      expect(error.message).toBe("Todo not found");
      expect(error.httpStatus).toBe(404);
    });

    it("should include resource ID in message", () => {
      const error = Errors.notFound("Todo", 12345);
      expect(error.message).toBe("Todo 12345 not found");
    });

    it("should accept string IDs", () => {
      const error = Errors.notFound("Project", "abc-123");
      expect(error.message).toBe("Project abc-123 not found");
    });
  });

  describe("rateLimit", () => {
    it("should create a rate limit error", () => {
      const error = Errors.rateLimit();
      expect(error.code).toBe("rate_limit");
      expect(error.httpStatus).toBe(429);
      expect(error.retryable).toBe(true);
    });

    it("should include retry after seconds", () => {
      const error = Errors.rateLimit(30);
      expect(error.retryAfter).toBe(30);
      expect(error.hint).toBe("Retry after 30 seconds");
    });
  });

  describe("validation", () => {
    it("should create a validation error", () => {
      const error = Errors.validation("Invalid input");
      expect(error.code).toBe("validation");
      expect(error.message).toBe("Invalid input");
      expect(error.httpStatus).toBe(400);
    });

    it("should accept custom hint", () => {
      const error = Errors.validation("Invalid email", "Must be a valid email address");
      expect(error.hint).toBe("Must be a valid email address");
    });
  });

  describe("network", () => {
    it("should create a network error", () => {
      const error = Errors.network("Connection refused");
      expect(error.code).toBe("network");
      expect(error.retryable).toBe(true);
      expect(error.hint).toContain("network connection");
    });
  });

  describe("apiError", () => {
    it("should create a generic API error", () => {
      const error = Errors.apiError("Something went wrong", 500);
      expect(error.code).toBe("api_error");
      expect(error.httpStatus).toBe(500);
    });

    it("should accept additional options", () => {
      const error = Errors.apiError("Server error", 503, {
        retryable: true,
        hint: "Try again later",
        requestId: "req-789",
      });
      expect(error.retryable).toBe(true);
      expect(error.hint).toBe("Try again later");
      expect(error.requestId).toBe("req-789");
    });
  });
});

describe("errorFromResponse", () => {
  it("should create auth error from 401 response", async () => {
    const response = new Response(JSON.stringify({ error: "Unauthorized" }), {
      status: 401,
      statusText: "Unauthorized",
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("auth_required");
    expect(error.httpStatus).toBe(401);
    expect(error.message).toBe("Unauthorized");
  });

  it("should create forbidden error from 403 response", async () => {
    const response = new Response(JSON.stringify({ error: "Forbidden" }), {
      status: 403,
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("forbidden");
    expect(error.httpStatus).toBe(403);
  });

  it("should create not found error from 404 response", async () => {
    const response = new Response(JSON.stringify({ error: "Not found" }), {
      status: 404,
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("not_found");
    expect(error.httpStatus).toBe(404);
  });

  it("should create rate limit error from 429 response", async () => {
    const response = new Response(null, {
      status: 429,
      headers: { "Retry-After": "60" },
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("rate_limit");
    expect(error.httpStatus).toBe(429);
    expect(error.retryable).toBe(true);
    expect(error.retryAfter).toBe(60);
  });

  it("should create validation error from 400 response", async () => {
    const response = new Response(
      JSON.stringify({ error: "Bad request", error_description: "Missing field" }),
      { status: 400 }
    );

    const error = await errorFromResponse(response);

    expect(error.code).toBe("validation");
    expect(error.httpStatus).toBe(400);
    expect(error.hint).toBe("Missing field");
  });

  it("should create validation error from 422 response", async () => {
    const response = new Response(JSON.stringify({ error: "Unprocessable" }), {
      status: 422,
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("validation");
    expect(error.httpStatus).toBe(422);
  });

  it("should create retryable API error from 5xx response", async () => {
    const response = new Response(JSON.stringify({ error: "Internal error" }), {
      status: 500,
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("api_error");
    expect(error.httpStatus).toBe(500);
    expect(error.retryable).toBe(true);
  });

  it("should include requestId when provided", async () => {
    const response = new Response(null, { status: 500 });

    const error = await errorFromResponse(response, "req-abc");

    expect(error.requestId).toBe("req-abc");
  });

  it("should handle non-JSON response body", async () => {
    const response = new Response("Plain text error", {
      status: 500,
      statusText: "Internal Server Error",
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("api_error");
    expect(error.message).toBe("Internal Server Error");
  });

  it("should handle empty response body", async () => {
    const response = new Response(null, {
      status: 503,
      statusText: "Service Unavailable",
    });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("api_error");
    expect(error.retryable).toBe(true);
  });

  it("should parse Retry-After as HTTP-date", async () => {
    const futureDate = new Date(Date.now() + 120000).toUTCString();
    const response = new Response(null, {
      status: 429,
      headers: { "Retry-After": futureDate },
    });

    const error = await errorFromResponse(response);

    expect(error.retryAfter).toBeGreaterThan(100);
    expect(error.retryAfter).toBeLessThanOrEqual(120);
  });

  it("should truncate large error messages to 500 chars", async () => {
    const largeMessage = "x".repeat(1000);
    const response = new Response(JSON.stringify({ error: largeMessage }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });

    const error = await errorFromResponse(response);

    expect(error.message.length).toBeLessThanOrEqual(500);
    expect(error.message).toMatch(/\.\.\.$/); // Ends with ...
  });

  it("should truncate large error_description to 500 chars", async () => {
    const largeDescription = "y".repeat(1000);
    const response = new Response(
      JSON.stringify({ error: "Bad request", error_description: largeDescription }),
      {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }
    );

    const error = await errorFromResponse(response);

    expect(error.hint).toBeDefined();
    expect(error.hint!.length).toBeLessThanOrEqual(500);
    expect(error.hint).toMatch(/\.\.\.$/); // Ends with ...
  });
});

describe("isBasecampError", () => {
  it("should return true for BasecampError", () => {
    const error = new BasecampError("auth_required", "test");
    expect(isBasecampError(error)).toBe(true);
  });

  it("should return false for regular Error", () => {
    const error = new Error("test");
    expect(isBasecampError(error)).toBe(false);
  });

  it("should return false for non-errors", () => {
    expect(isBasecampError("string")).toBe(false);
    expect(isBasecampError(null)).toBe(false);
    expect(isBasecampError(undefined)).toBe(false);
    expect(isBasecampError({ code: "auth_required" })).toBe(false);
  });
});

describe("isErrorCode", () => {
  it("should return true for matching error code", () => {
    const error = new BasecampError("not_found", "test");
    expect(isErrorCode(error, "not_found")).toBe(true);
  });

  it("should return false for non-matching error code", () => {
    const error = new BasecampError("auth_required", "test");
    expect(isErrorCode(error, "not_found")).toBe(false);
  });

  it("should return false for non-BasecampError", () => {
    const error = new Error("test");
    expect(isErrorCode(error, "auth_required")).toBe(false);
  });
});
