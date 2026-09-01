/**
 * Tests for the errors module
 */
import { describe, it, expect } from "vitest";
import {
  BasecampError,
  Errors,
  errorFromParsedBody,
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
        usage: 1,
        not_found: 2,
        auth_required: 3,
        forbidden: 4,
        rate_limit: 5,
        network: 6,
        api_error: 7,
        ambiguous: 8,
        validation: 9,
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

  describe("ambiguous", () => {
    it("should create an ambiguous error", () => {
      const error = Errors.ambiguous("project", ["Project A", "Project B"]);
      expect(error.code).toBe("ambiguous");
      expect(error.exitCode).toBe(8);
      expect(error.message).toBe("Ambiguous project");
      expect(error.hint).toBe("Did you mean: Project A, Project B");
    });

    it("should use generic hint for many matches", () => {
      const error = Errors.ambiguous("todo", ["a", "b", "c", "d", "e", "f"]);
      expect(error.hint).toBe("Be more specific");
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

    // SPEC §6 step 5: the fixed code-bearing phrase, never the wire reason
    // phrase (which HTTP/2 does not carry at all).
    expect(error.code).toBe("api_error");
    expect(error.message).toBe("Request failed (HTTP 500)");
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

  it("renders the fixed code-bearing phrase for an unregistered status with an empty body", async () => {
    // 599 has no registered reason phrase, so the statusText fallback this
    // replaced yielded a blank message (SPEC §6 step 5).
    const response = new Response(null, { status: 599 });

    const error = await errorFromResponse(response);

    expect(error.code).toBe("api_error");
    expect(error.message).toBe("Request failed (HTTP 599)");
  });

  describe("field-keyed 422 bodies", () => {
    it("flattens field errors into the message and exposes the structured slot", async () => {
      const response = new Response(
        JSON.stringify({ errors: { color: ["is not a valid color"] } }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.code).toBe("validation");
      expect(error.message).toBe("color: is not a valid color");
      expect(error.fieldErrors).toEqual({ color: ["is not a valid color"] });
    });

    it("sorts fields lexicographically and joins multi-message fields", async () => {
      const response = new Response(
        JSON.stringify({
          errors: {
            name: ["can't be blank", "is too short"],
            color: ["is not a valid color"],
          },
        }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe(
        "color: is not a valid color, name: can't be blank; is too short"
      );
      expect(error.fieldErrors).toEqual({
        color: ["is not a valid color"],
        name: ["can't be blank", "is too short"],
      });
    });

    it("appends flattened field errors after a top-level error message", async () => {
      const response = new Response(
        JSON.stringify({
          error: "Validation failed",
          errors: { color: ["is not a valid color"] },
        }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("Validation failed (color: is not a valid color)");
    });

    it("extracts field errors on 400 responses too", async () => {
      const response = new Response(
        JSON.stringify({ errors: { color: ["is not a valid color"] } }),
        { status: 400 }
      );

      const error = await errorFromResponse(response);

      expect(error.code).toBe("validation");
      expect(error.message).toBe("color: is not a valid color");
      expect(error.fieldErrors).toEqual({ color: ["is not a valid color"] });
    });

    it("does not extract field errors outside validation statuses", async () => {
      const response = new Response(
        JSON.stringify({ errors: { color: ["is not a valid color"] } }),
        { status: 403, statusText: "Forbidden" }
      );

      const error = await errorFromResponse(response);

      expect(error.fieldErrors).toBeUndefined();
      expect(error.message).toBe("Request failed (HTTP 403)");
    });

    it("skips malformed entries and keeps only string messages", async () => {
      const response = new Response(
        JSON.stringify({
          errors: {
            color: "not an array",
            name: ["can't be blank"],
            empty: [],
            mixed: [42, "is invalid"],
          },
        }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("mixed: is invalid, name: can't be blank");
      expect(error.fieldErrors).toEqual({
        mixed: ["is invalid"],
        name: ["can't be blank"],
      });
    });

    it("falls back to the default message when errors is not a usable map", async () => {
      for (const errors of [{ color: "not an array" }, [], "nope", {}]) {
        const response = new Response(JSON.stringify({ errors }), {
          status: 422,
          statusText: "Unprocessable Entity",
        });

        const error = await errorFromResponse(response);

        expect(error.fieldErrors).toBeUndefined();
        expect(error.message).toBe("Request failed (HTTP 422)");
      }
    });

    it("truncates the composed message after flattening but keeps the raw slot", async () => {
      const long = "x".repeat(600);
      const response = new Response(
        JSON.stringify({ errors: { color: [long] } }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message.length).toBe(500);
      expect(error.message.startsWith("color: xxx")).toBe(true);
      expect(error.message.endsWith("...")).toBe(true);
      expect(error.fieldErrors).toEqual({ color: [long] });
    });

    it("survives a non-string error sibling alongside a usable errors map", async () => {
      const response = new Response(
        JSON.stringify({
          error: {},
          error_description: 42,
          errors: { color: ["is not a valid color"] },
        }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("color: is not a valid color");
      expect(error.fieldErrors).toEqual({ color: ["is not a valid color"] });
      expect(error.hint).toBeUndefined();
    });

    it("appends field errors after a message-key fallback", async () => {
      const response = new Response(
        JSON.stringify({
          message: "Validation failed",
          errors: { color: ["is not a valid color"] },
        }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("Validation failed (color: is not a valid color)");
      expect(error.fieldErrors).toEqual({ color: ["is not a valid color"] });
    });

    it("prefers the error key over the message key", async () => {
      const response = new Response(
        JSON.stringify({ error: "from error", message: "from message" }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("from error");
    });

    it("preserves a __proto__ field as an own property without prototype pollution", async () => {
      const response = new Response(
        '{"errors": {"__proto__": ["is reserved"], "color": ["is not a valid color"]}}',
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("__proto__: is reserved, color: is not a valid color");
      expect(Object.prototype.hasOwnProperty.call(error.fieldErrors, "__proto__")).toBe(true);
      expect(error.fieldErrors!["__proto__"]).toEqual(["is reserved"]);
      expect(error.fieldErrors!.color).toEqual(["is not a valid color"]);
      // The legacy prototype setter must not have fired: the map's prototype
      // is not the attacker-controlled array.
      expect(Array.isArray(Object.getPrototypeOf(error.fieldErrors))).toBe(false);
    });

    it("leaves plain 422 error bodies unchanged", async () => {
      const response = new Response(
        JSON.stringify({ error: "Name can't be blank" }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.message).toBe("Name can't be blank");
      expect(error.fieldErrors).toBeUndefined();
    });

    it("includes fieldErrors in toJSON", async () => {
      const response = new Response(
        JSON.stringify({ errors: { color: ["is not a valid color"] } }),
        { status: 422 }
      );

      const error = await errorFromResponse(response);

      expect(error.toJSON().fieldErrors).toEqual({ color: ["is not a valid color"] });
    });
  });

  describe("errorFromParsedBody (consumed-body path)", () => {
    // openapi-fetch consumes and parses the error body before the service
    // layer sees the response; the SDK must build the error from that parsed
    // value instead of re-reading the consumed stream.
    it("extracts the server message from an already-parsed body", () => {
      const response = new Response(null, { status: 422, statusText: "Unprocessable Entity" });

      const error = errorFromParsedBody(response, { error: "Name can't be blank" });

      expect(error.code).toBe("validation");
      expect(error.message).toBe("Name can't be blank");
    });

    it("flattens field-keyed 422 errors from an already-parsed body", () => {
      const response = new Response(null, { status: 422, statusText: "Unprocessable Entity" });

      const error = errorFromParsedBody(response, {
        errors: { color: ["is not a valid color"] },
      });

      expect(error.message).toBe("color: is not a valid color");
      expect(error.fieldErrors).toEqual({ color: ["is not a valid color"] });
    });
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

describe("bare field-map error bodies (SPEC §6 step 2)", () => {
  // webhooks_controller and chats/integrations_controller render
  // `json: @webhook.errors` at 400; lineup markers do the same at 422. The
  // field map is the whole body, with no "errors" wrapper.
  it.each([
    {
      name: "single field at 400",
      status: 400,
      body: { payload_url: ["is not a valid URL"] },
      message: "payload_url: is not a valid URL",
      fieldErrors: { payload_url: ["is not a valid URL"] },
    },
    {
      name: "multiple fields sort and join",
      status: 400,
      body: { types: ["is invalid"], payload_url: ["is not a valid URL", "is too long"] },
      message: "payload_url: is not a valid URL; is too long, types: is invalid",
      fieldErrors: {
        payload_url: ["is not a valid URL", "is too long"],
        types: ["is invalid"],
      },
    },
    {
      name: "lineup markers emit the bare map at 422",
      status: 422,
      body: { name: ["can't be blank"] },
      message: "name: can't be blank",
      fieldErrors: { name: ["can't be blank"] },
    },
  ])("$name", ({ status, body, message, fieldErrors }) => {
    const response = new Response(null, { status, statusText: "Bad Request" });

    const error = errorFromParsedBody(response, body);

    expect(error.code).toBe("validation");
    expect(error.message).toBe(message);
    expect({ ...error.fieldErrors }).toEqual(fieldErrors);
  });

  // The gate is all-or-nothing: with no "errors" key to declare intent, shape
  // is the only signal, so one non-conforming member means this is some other
  // JSON object.
  it.each([
    { name: "member is not an array", body: { id: 1 } },
    { name: "one member of several is not an array", body: { color: ["is invalid"], count: 3 } },
    { name: "member array is empty", body: { color: [] } },
    { name: "member array holds an empty string", body: { color: ["", "is invalid"] } },
    { name: "member array holds a non-string", body: { color: ["is invalid", 42] } },
    { name: "member array holds null", body: { color: [null] } },
    { name: "empty object", body: {} },
    { name: "JSON array body", body: [1, 2] },
  ])("does not treat $name as a field map", ({ body }) => {
    const response = new Response(null, { status: 400, statusText: "Bad Request" });

    const error = errorFromParsedBody(response, body);

    expect(error.fieldErrors).toBeUndefined();
    expect(error.message).toBe("Request failed (HTTP 400)");
  });

  // Only "errors" is excluded by name; a flat body's "error"/"message" is a
  // string, and the shape gate rejects a string-valued member — so these bodies
  // stay flat on shape, not on the key's name. The next test covers the other
  // half: array-valued "error"/"message" members ARE recognized as fields.
  it.each([
    { name: "error key", body: { error: "Webhook is invalid", payload_url: ["is bad"] }, message: "Webhook is invalid" },
    {
      name: "message key",
      body: { message: "Webhook is invalid", payload_url: ["is bad"] },
      message: "Webhook is invalid",
    },
    { name: "empty errors key", body: { errors: {}, payload_url: ["is bad"] }, message: "Request failed (HTTP 400)" },
  ])("stays flat for a string $name", ({ body, message }) => {
    const response = new Response(null, { status: 400, statusText: "Bad Request" });

    const error = errorFromParsedBody(response, body);

    expect(error.message).toBe(message);
    expect(error.fieldErrors).toBeUndefined();
  });

  // Only "errors" is reserved by name. A record whose validated attribute is
  // called "message" or "error" still gets its field map recognized: the flat
  // shape carries those keys as strings, which the gate rejects on shape alone.
  it.each([
    {
      name: "field named message",
      body: { message: ["can't be blank"] },
      message: "message: can't be blank",
      fieldErrors: { message: ["can't be blank"] },
    },
    {
      name: "field named error alongside another",
      body: { error: ["is invalid"], name: ["can't be blank"] },
      message: "error: is invalid, name: can't be blank",
      fieldErrors: { error: ["is invalid"], name: ["can't be blank"] },
    },
  ])("recognizes a $name", ({ body, message, fieldErrors }) => {
    const response = new Response(null, { status: 400, statusText: "Bad Request" });

    const error = errorFromParsedBody(response, body);

    expect(error.message).toBe(message);
    expect({ ...error.fieldErrors }).toEqual(fieldErrors);
  });

  it("is not extracted outside 400/422", () => {
    for (const status of [403, 404, 500]) {
      const response = new Response(null, { status, statusText: "Nope" });
      const error = errorFromParsedBody(response, { payload_url: ["is not a valid URL"] });
      expect(error.fieldErrors).toBeUndefined();
    }
  });

  it("treats __proto__ as an ordinary field name in a bare map", () => {
    const response = new Response(null, { status: 400, statusText: "Bad Request" });

    const error = errorFromParsedBody(response, JSON.parse('{"__proto__": ["is invalid"]}'));

    expect(error.message).toBe("__proto__: is invalid");
    expect(Object.getPrototypeOf(error.fieldErrors)).toBeNull();
    expect(Object.prototype.hasOwnProperty.call(error.fieldErrors, "__proto__")).toBe(true);
    expect(error.fieldErrors!["__proto__"]).toEqual(["is invalid"]);
    // The legacy prototype setter must not have fired: the map's prototype is
    // not the attacker-controlled array.
    expect(Array.isArray(Object.getPrototypeOf(error.fieldErrors))).toBe(false);
  });
});
