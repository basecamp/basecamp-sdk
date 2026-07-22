import { describe, it, expect, beforeEach } from "vitest";
import { generateExampleValue, setSchemas, type Schema } from "../../scripts/generate-services.js";

// Regression coverage for object-valued body members. Before the fix, object refs
// (e.g. a `project`/`gauge` envelope) fell through to the '"example"' scalar and the
// generator emitted invalid examples like `{ project: "example" }`.
describe("generateExampleValue — object-valued body members", () => {
  const schemas: Record<string, Schema> = {
    ProjectConstructionAttributes: {
      type: "object",
      properties: { name: { type: "string" }, description: { type: "string" } },
      required: ["name"],
    },
    GaugeTogglePayload: {
      type: "object",
      properties: { enabled: { type: "boolean" } },
      required: ["enabled"],
    },
    GaugeNeedlePayload: {
      type: "object",
      properties: {
        position: { type: "integer", format: "int32" },
        color: { type: "string" },
      },
      required: ["position"],
    },
    OutOfOfficePayload: {
      type: "object",
      properties: {
        start_date: { type: "string" },
        end_date: { type: "string" },
      },
      required: ["start_date", "end_date"],
    },
    FirstWeekDay: {
      type: "string",
    },
  };

  beforeEach(() => {
    setSchemas(schemas);
  });

  it("renders a nested object using the member's required children (name)", () => {
    const value = generateExampleValue("project", 'components["schemas"]["ProjectConstructionAttributes"]', undefined, {
      $ref: "#/components/schemas/ProjectConstructionAttributes",
    });
    expect(value).toBe('{ name: "My example" }');
  });

  it("renders a non-name object using the child's actual type (enabled: boolean)", () => {
    const value = generateExampleValue("gauge", 'components["schemas"]["GaugeTogglePayload"]', undefined, {
      $ref: "#/components/schemas/GaugeTogglePayload",
    });
    expect(value).toBe("{ enabled: true }");
  });

  it("renders integer children as numbers and omits optional members", () => {
    const value = generateExampleValue("gaugeNeedle", 'components["schemas"]["GaugeNeedlePayload"]', undefined, {
      $ref: "#/components/schemas/GaugeNeedlePayload",
    });
    expect(value).toBe("{ position: 1 }");
  });

  it("keeps nested schema member keys raw (snake_case), not camel-cased", () => {
    // The nested object is typed as the component schema and forwarded to the wire
    // unchanged, so its keys must match the schema exactly (start_date, not startDate).
    const value = generateExampleValue("outOfOffice", 'components["schemas"]["OutOfOfficePayload"]', undefined, {
      $ref: "#/components/schemas/OutOfOfficePayload",
    });
    expect(value).toBe('{ start_date: "example", end_date: "example" }');
  });

  it("leaves a string ref (FirstWeekDay) as a scalar, not an object", () => {
    const value = generateExampleValue("firstWeekDay", 'components["schemas"]["FirstWeekDay"]', undefined, {
      $ref: "#/components/schemas/FirstWeekDay",
    });
    expect(value).toBe('"example"');
  });
});
