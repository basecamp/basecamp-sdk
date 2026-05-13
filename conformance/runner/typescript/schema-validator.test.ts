/**
 * Offline tests for the live-canary schema validator.
 *
 * The live canary path itself requires real Basecamp credentials, so these
 * tests exercise the validator with crafted payloads to catch wiring bugs
 * (Ajv $ref resolution, extras-collection on arrays + nested objects, etc.)
 * without needing live access.
 */

import { describe, it, expect } from "vitest";
import { validateResponse } from "./schema-validator.js";

// =============================================================================
// ListProjects (array response — Project[])
// =============================================================================

const conformantProject = {
  id: 1,
  status: "active",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-02T00:00:00Z",
  name: "Test Project",
  url: "https://3.basecampapi.com/999/projects/1.json",
  app_url: "https://3.basecamp.com/999/projects/1",
};

describe("validateResponse — ListProjects (array root)", () => {
  it("compiles with $ref resolved against the registered OpenAPI doc", () => {
    // This is the exact bug the reviewer flagged: pre-fix, Ajv tried to
    // resolve "#/components/schemas/ListProjectsResponseContent" against
    // the fragment root and threw `can't resolve reference ... from id #`.
    const result = validateResponse("ListProjects", [conformantProject]);
    expect(result.errors).toEqual([]);
    expect(result.ok).toBe(true);
  });

  it("flags missing required fields", () => {
    const broken = { ...conformantProject } as Record<string, unknown>;
    delete broken.name;
    const result = validateResponse("ListProjects", [broken]);
    expect(result.ok).toBe(false);
    expect(result.errors.some((e) => e.includes("name"))).toBe(true);
  });

  it("permits extra fields without failing (forward-compat)", () => {
    const withExtras = { ...conformantProject, future_field: "BC5 addition" };
    const result = validateResponse("ListProjects", [withExtras]);
    expect(result.ok).toBe(true);
  });

  it("collects item-level extras with [] path prefix", () => {
    const withExtras = { ...conformantProject, future_field: "BC5 addition" };
    const result = validateResponse("ListProjects", [withExtras]);
    // Path convention: "[]" segment for array items, then ".field" for keys.
    expect(result.extras).toContain("[].future_field");
  });

  it("emits known-property paths so nested extras stay visible", () => {
    // Project has no nested object schemas exercised here; this asserts
    // that declared properties don't get reported as extras.
    const result = validateResponse("ListProjects", [conformantProject]);
    expect(result.extras).not.toContain("[].id");
    expect(result.extras).not.toContain("[].status");
  });
});

// =============================================================================
// GetMyNotifications (object response with array properties)
// =============================================================================

describe("validateResponse — GetMyNotifications (object root, array fields)", () => {
  it("validates an empty payload (all arrays absent)", () => {
    const result = validateResponse("GetMyNotifications", {});
    expect(result.ok).toBe(true);
  });

  it("validates with empty arrays", () => {
    const result = validateResponse("GetMyNotifications", {
      unreads: [],
      reads: [],
      memories: [],
      bubble_ups: [],
      scheduled_bubble_ups: [],
    });
    expect(result.ok).toBe(true);
  });

  it("collects extras at the root", () => {
    const result = validateResponse("GetMyNotifications", {
      unreads: [],
      hypothetical_new_top_level: 42,
    });
    expect(result.ok).toBe(true);
    expect(result.extras).toContain("hypothetical_new_top_level");
  });

  it("collects extras inside array-valued properties", () => {
    const minimalNotification = {
      id: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
    };
    const result = validateResponse("GetMyNotifications", {
      unreads: [{ ...minimalNotification, future_envelope_field: "BC5 addition" }],
    });
    expect(result.ok).toBe(true);
    // Path: <prefix>[].field — here `unreads[].future_envelope_field`.
    expect(result.extras).toContain("unreads[].future_envelope_field");
  });
});

// =============================================================================
// Non-200 success responses
// =============================================================================

describe("validateResponse — non-200 success responses", () => {
  it("resolves a schema for operations that return 201 (Create*)", () => {
    // CreateProject's success status is 201 Created. Pre-fix the lookup
    // checked only "200"/"default" and missed it.
    const result = validateResponse("CreateProject", {
      id: 1,
      status: "active",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
      name: "Test",
      url: "https://3.basecampapi.com/999/projects/1.json",
      app_url: "https://3.basecamp.com/999/projects/1",
    });
    // Either ok=true or field-level errors — but never the
    // "No response schema found" sentinel (which would prove the lookup
    // failed entirely).
    expect(result.errors.join("\n")).not.toContain("No response schema found");
  });
});

// =============================================================================
// Unknown operation
// =============================================================================

describe("validateResponse — error paths", () => {
  it("returns ok=false with a clear error for unknown operations", () => {
    const result = validateResponse("DoesNotExist", {});
    expect(result.ok).toBe(false);
    expect(result.errors[0]).toContain("DoesNotExist");
  });
});

// =============================================================================
// Bodyless 2xx (204 No Content)
// =============================================================================

describe("validateResponse — bodyless success responses", () => {
  it("returns ok=true for operations whose only success is 204 No Content", () => {
    // DeleteBoost is documented with only a 204 response — no JSON body
    // ever returns. Pre-fix, the lookup yielded no schema and the
    // validator returned ok=false, blocking such ops from the canary.
    const result = validateResponse("DeleteBoost", null);
    expect(result.ok).toBe(true);
    expect(result.errors).toEqual([]);
  });
});
