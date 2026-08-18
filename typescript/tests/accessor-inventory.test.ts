/**
 * Accessor-roster inventory: every generated service must be *reachable*.
 *
 * `check-typescript-service-drift.sh` proves the generated service directory
 * matches the spec, and `make check-service-inventory-parity` proves the five
 * generators emitted the same service set. Neither asks whether the
 * hand-written wiring exposes what was generated — and TypeScript has four
 * hand-maintained renderings of one roster:
 *
 *   1. the `import` block in src/client.ts
 *   2. the `BasecampClient` interface properties
 *   3. the `defineService` calls
 *   4. the export blocks in src/index.ts
 *
 * A service missing from any of the first three is unreachable at runtime; a
 * service missing from the fourth is unreachable to a *consumer*, because
 * package.json exports only the package root and `./oauth`. Nothing fails in
 * either case: nothing references the missing accessor, so compilation is
 * clean. Python shipped `gauges` unreachable for about a year that way (#732,
 * #755). This is that question, asked of both surfaces.
 *
 * Two properties of the derivation are deliberate:
 *
 *   - The roster comes from the generated directory, never a literal. A literal
 *     would have to be edited by the same person who forgot the accessor.
 *   - Resolution goes through the modules, not their source text. Two
 *     `defineService` calls span multiple lines and one index.ts export is a
 *     single line, so a line-oriented regex over either file silently misses
 *     them.
 */

import { describe, expect, it } from "vitest";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

import { createBasecampClient } from "../src/client.js";
import * as generatedServices from "../src/generated/services/index.js";
import * as rootExports from "../src/index.js";
import { BaseService } from "../src/services/base.js";

const GENERATED_SERVICES_DIR = fileURLToPath(new URL("../src/generated/services", import.meta.url));

/**
 * Non-vacuity floor. Every assertion below is `it.each` over a derived roster,
 * so an import or path change that yielded an empty roster would make all of
 * them vacuously true. The floor is well under the real count (53) so it does
 * not become a second constant to maintain.
 */
const MIN_GENERATED_SERVICES = 40;

/** Services on the flat client that are not generated from the spec. */
const HAND_WRITTEN_ACCESSORS = new Set(["authorization"]);

/** `*Service` value exports of index.ts that are not generated services. */
const NON_SERVICE_EXPORTS = new Set(["BaseService", "AuthorizationService"]);

const toCamelCase = (kebab: string): string => kebab.replace(/-([a-z])/g, (_, c: string) => c.toUpperCase());

const toPascalCase = (kebab: string): string => {
  const camel = toCamelCase(kebab);
  return camel.charAt(0).toUpperCase() + camel.slice(1);
};

/** Every generated service module, by kebab-case basename. */
const GENERATED_MODULES: string[] = fs
  .readdirSync(GENERATED_SERVICES_DIR)
  .filter((f) => f.endsWith(".ts") && f !== "index.ts")
  .map((f) => f.slice(0, -".ts".length))
  .sort();

/** module basename -> { accessor on the client, exported class name }. */
const ROSTER = GENERATED_MODULES.map((module) => ({
  module,
  accessor: toCamelCase(module),
  className: `${toPascalCase(module)}Service`,
}));

const serviceClass = (className: string): Function => {
  const cls = (generatedServices as unknown as Record<string, unknown>)[className];
  // Asserted rather than assumed: the accessor and class names are derived from
  // the filename by one rule each, and this is where an irregular spelling the
  // generator might introduce surfaces as a named failure instead of an
  // `undefined` two assertions later.
  expect(typeof cls, `generated/services/index.ts does not export ${className}`).toBe("function");
  return cls as Function;
};

const newClient = () => createBasecampClient({ accountId: "12345", accessToken: "test-token" });

const propertyOf = (target: object, name: string): unknown => (target as Record<string, unknown>)[name];

describe("generated service reachability", () => {
  it("derives a non-empty roster from the generated directory", () => {
    expect(GENERATED_MODULES.length).toBeGreaterThan(MIN_GENERATED_SERVICES);
    // The generated barrel is emitted by the same loop that writes the modules,
    // so a disagreement here means the directory holds something the generator
    // did not produce.
    expect(Object.keys(generatedServices).length).toBe(GENERATED_MODULES.length);
  });

  describe("client.ts wiring", () => {
    it.each(ROSTER)("client.$accessor is a $className", ({ module, accessor, className }) => {
      const client = newClient();
      const cls = serviceClass(className);

      const service = propertyOf(client, accessor);
      expect(service, `BasecampClient has no \`${accessor}\`; generated/services/${module}.ts defines that service but client.ts does not wire it`).toBeDefined();
      // Presence alone would pass if `gauges` returned the wrong service, so
      // resolve it and check the class it actually yields. The six hand-written
      // composites (§18) subclass their generated service, so `instanceof`
      // holds for them too.
      expect(service).toBeInstanceOf(cls);
    });

    it("has no accessor without a generated service", () => {
      const client = newClient();
      // Every service accessor is a BaseService instance, which is what
      // separates them from the raw openapi-fetch verbs and the client's own
      // properties without naming those individually.
      const wired = Object.keys(client)
        .filter((key) => propertyOf(client, key) instanceof BaseService)
        .filter((key) => !HAND_WRITTEN_ACCESSORS.has(key))
        .sort();

      expect(wired).toEqual(ROSTER.map((r) => r.accessor).sort());
    });
  });

  describe("index.ts exports", () => {
    // Needs its own assertion: a missing export block is invisible to the
    // runtime checks above, which reach the service through client.ts. An
    // in-repo importer never notices, and only a consumer of the published
    // package does.
    it.each(ROSTER)("index.ts exports $className", ({ className }) => {
      const cls = serviceClass(className);
      const exported = propertyOf(rootExports, className);

      expect(exported, `src/index.ts does not export ${className}; consumers cannot import it through any supported path`).toBeDefined();
      // Six index.ts blocks export types only, their class coming from a
      // hand-written extension subclass exported separately — so the export is
      // required to *be* the generated class or to extend it, not to be
      // identical to it.
      const isTheServiceClass = exported === cls || (exported as { prototype?: unknown })?.prototype instanceof cls;
      expect(isTheServiceClass, `src/index.ts exports ${className}, but it is neither the generated class nor a subclass of it`).toBe(true);
    });

    it("exports no service class without a generated service", () => {
      const exportedServices = Object.keys(rootExports)
        .filter((name) => name.endsWith("Service") && !NON_SERVICE_EXPORTS.has(name))
        .sort();

      expect(exportedServices).toEqual(ROSTER.map((r) => r.className).sort());
    });
  });
});
