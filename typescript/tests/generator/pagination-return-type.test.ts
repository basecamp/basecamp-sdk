import { describe, it, expect, beforeEach } from "vitest";
import {
  buildReturnType,
  generateMethod,
  setSchemas,
  type ParsedOperation,
  type Schema,
} from "../../scripts/generate-services.js";

// Regression coverage for the return type of paginated operations (#737).
//
// `buildReturnType` resolves a friendly element name through the hand-maintained
// TYPE_ALIASES map, and everything below is about what happens when that lookup
// MISSES. Before the fix a missed lookup fell through to the raw response schema
// ref and dropped the `ListResult<>` wrapper entirely for the array form, and
// spelled `ListResult<unknown>` for the wrapped form.
//
// The type-level assertions in tests/types/paginated-returns.test-d.ts pin the
// four real operations that hit the array miss. They CANNOT reach the wrapped
// miss: every wrapped-pagination operation in the spec today (there is one)
// carries an aliased entity, so no generated signature would change if that
// branch regressed. These unit tests drive the function directly, so the branch
// is covered whether or not the spec ever grows such an operation.
//
// `WidgetThing` is deliberately fictional: an entity name that is not in
// TYPE_ALIASES and cannot quietly acquire an entry later, which would otherwise
// turn a miss case into a hit case without anyone noticing.

const schemas: Record<string, Schema> = {
  // Bare array response, entity absent from TYPE_ALIASES — the #737 shape.
  ListWidgetsResponseContent: {
    type: "array",
    items: { $ref: "#/components/schemas/WidgetThing" },
  },
  // Bare array response, entity present in TYPE_ALIASES.
  ListTodosResponseContent: {
    type: "array",
    items: { $ref: "#/components/schemas/Todo" },
  },
  // Bare array response whose items name no schema at all.
  ListAnonymousResponseContent: {
    type: "array",
    items: { type: "object" },
  },
  // Wrapped pagination, entity absent from TYPE_ALIASES.
  WidgetReportResponseContent: {
    type: "object",
    properties: {
      person: { $ref: "#/components/schemas/Person" },
      widgets: { type: "array", items: { $ref: "#/components/schemas/WidgetThing" } },
    },
  },
  // Wrapped pagination, entity present in TYPE_ALIASES — the shape the spec has.
  TimelineReportResponseContent: {
    type: "object",
    properties: {
      person: { $ref: "#/components/schemas/Person" },
      events: { type: "array", items: { $ref: "#/components/schemas/TimelineEvent" } },
    },
  },
  WidgetThing: { type: "object", properties: { id: { type: "integer" } } },
  Todo: { type: "object", properties: { id: { type: "integer" } } },
  Person: { type: "object", properties: { id: { type: "integer" } } },
  TimelineEvent: { type: "object", properties: { id: { type: "integer" } } },
};

const operation = (overrides: Partial<ParsedOperation>): ParsedOperation => ({
  operationId: "ListWidgets",
  methodName: "listWidgets",
  httpMethod: "GET",
  path: "/widgets.json",
  description: "List widgets.",
  pathParams: [],
  queryParams: [],
  bodyProperties: [],
  bodyRequired: false,
  returnsArray: false,
  returnsVoid: false,
  isMutation: false,
  resourceType: "widget",
  hasPagination: false,
  serviceName: "Widgets",
  ...overrides,
});

describe("buildReturnType — paginated operations", () => {
  beforeEach(() => {
    setSchemas(schemas);
  });

  describe("bare array responses", () => {
    it("wraps an aliased entity in ListResult under its friendly name", () => {
      const returnType = buildReturnType(
        operation({ responseSchemaRef: "ListTodosResponseContent", returnsArray: true, hasPagination: true }),
        "Todos",
      );

      expect(returnType).toBe("ListResult<Todo>");
    });

    // The #737 regression: this returned the bare
    // `components["schemas"]["ListWidgetsResponseContent"]` array, so `.meta`
    // was invisible to the compiler on a value that carries it at runtime.
    it("keeps the ListResult wrapper when the entity has no alias", () => {
      const returnType = buildReturnType(
        operation({ responseSchemaRef: "ListWidgetsResponseContent", returnsArray: true, hasPagination: true }),
        "Widgets",
      );

      expect(returnType).toBe('ListResult<components["schemas"]["WidgetThing"]>');
    });

    // The wrapper is tied to pagination, not to arrays: an unpaginated array
    // returns no ListResult and must still fall back to the schema ref.
    it("leaves an unpaginated array as its response schema ref", () => {
      const returnType = buildReturnType(
        operation({ responseSchemaRef: "ListWidgetsResponseContent", returnsArray: true, hasPagination: false }),
        "Widgets",
      );

      expect(returnType).toBe('components["schemas"]["ListWidgetsResponseContent"]');
    });

    // The floor. No schema names the element, so there is nothing concrete to
    // put inside the wrapper — but the wrapper itself still survives, because
    // requestPaginated builds a ListResult either way.
    it("falls back to ListResult<unknown> when the items name no schema", () => {
      const returnType = buildReturnType(
        operation({ responseSchemaRef: "ListAnonymousResponseContent", returnsArray: true, hasPagination: true }),
        "Widgets",
      );

      expect(returnType).toBe("ListResult<unknown>");
    });
  });

  describe("wrapped pagination", () => {
    it("wraps an aliased entity in ListResult under its friendly name", () => {
      const returnType = buildReturnType(
        operation({
          responseSchemaRef: "TimelineReportResponseContent",
          returnsArray: false,
          hasPagination: true,
          paginationKey: "events",
        }),
        "Reports",
      );

      expect(returnType).toBe("{ person: Person; events: ListResult<TimelineEvent> }");
    });

    // Unreachable from the current spec, which is exactly why it is here: the
    // alias miss used to spell `ListResult<unknown>` and no generated signature
    // would move if it did so again.
    it("uses the item's schema ref when the entity has no alias", () => {
      const returnType = buildReturnType(
        operation({
          responseSchemaRef: "WidgetReportResponseContent",
          returnsArray: false,
          hasPagination: true,
          paginationKey: "widgets",
        }),
        "Reports",
      );

      expect(returnType).toBe('{ person: Person; widgets: ListResult<components["schemas"]["WidgetThing"]> }');
    });

    // A wrapped-paginated signature is emitted in two places that must name the
    // same element: the declared return type above, and the
    // `requestPaginatedWrapped<key, T>` type argument inside the method body.
    // `buildReturnType` cannot see the second one, and neither can anything
    // else in the repo — verified by mutation. Regressing that argument to
    // `unknown` and regenerating leaves `make ts-check` entirely green: drift
    // passes (the committed output was regenerated to match), both typecheck
    // projects pass (the generated method casts the result to its separately
    // built return type, so the disagreement never surfaces), and all tests
    // pass. `paginated-returns.test-d.ts` cannot reach it either — it pins the
    // four operations that hit the ARRAY miss.
    //
    // So the agreement is asserted here, on the emitted method, for both the
    // hit and the miss.
    it.each([
      { ref: "TimelineReportResponseContent", key: "events", element: "TimelineEvent" },
      { ref: "WidgetReportResponseContent", key: "widgets", element: 'components["schemas"]["WidgetThing"]' },
    ])("emits $element as both the declared element and the paginated type argument", ({ ref, key, element }) => {
      const op = operation({
        responseSchemaRef: ref,
        returnsArray: false,
        hasPagination: true,
        paginationKey: key,
      });

      expect(generateMethod(op, "Reports").join("\n")).toContain(
        `return this.requestPaginatedWrapped<"${key}", ${element}>(`,
      );
      expect(buildReturnType(op, "Reports")).toContain(`${key}: ListResult<${element}>`);
    });
  });
});
