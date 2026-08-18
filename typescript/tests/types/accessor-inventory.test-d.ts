/**
 * Type-level half of the accessor-roster inventory.
 *
 * `tests/accessor-inventory.test.ts` covers three of TypeScript's four
 * hand-maintained renderings of the service roster — the imports and
 * `defineService` calls in client.ts (a client it constructs would not carry
 * the accessor) and the export blocks in index.ts. It cannot cover the fourth.
 * The `BasecampClient` interface exists only in the type system: the factory
 * returns `client as BasecampClient`, so a property missing from the interface
 * changes nothing at runtime and no runtime assertion can observe it. What it
 * changes is the consumer's experience — `client.gauges` stops typechecking for
 * everyone outside this repo, while every in-repo test stays green.
 *
 * That rendering is not unguarded today, and the guard should not be sold as
 * more than it is: all 53 accessors are reached through the typed client
 * somewhere under tests/, so removing one from the interface fails
 * `tsc -p tsconfig.test.json` with a TS2339 at those call sites (verified by
 * doing it). But that coverage is incidental — it holds only while every
 * service's tests happen to go through `client.<accessor>` rather than
 * constructing the service directly, which is a property of how each test was
 * written, not something anything asserts. This file states the invariant
 * instead, for all 53 and for every service added later.
 *
 * The roster is derived in the type system from the generated barrel. The
 * derivation is the same one the runtime test performs on filenames, expressed
 * as a type: strip the `Service` suffix from each exported class name and
 * lowercase the first character. `HillChartsService` -> `hillCharts`,
 * `MyNotificationsService` -> `myNotifications`.
 *
 * There is no runtime here; `tsc` checks it via `tsconfig.test.json` / `make
 * ts-typecheck`. The `.test-d.ts` suffix keeps vitest (`include:
 * tests/**\/*.test.ts`) from collecting a file with no tests in it, and — as
 * tests/types/paginated-returns.test-d.ts documents at length — a name ending
 * `-d.ts` is not a declaration file, so the inherited `skipLibCheck: true`
 * never reaches these assertions.
 */

import type { BasecampClient } from "../../src/client.js";
import type * as GeneratedServices from "../../src/generated/services/index.js";

/** Compiles only when `T` is `true`; anything else is a TS2344 constraint error. */
type Expect<T extends true> = T;

/** Every class the generated barrel exports, as a union of names. */
type GeneratedServiceClassName = keyof typeof GeneratedServices & string;

/** `"HillChartsService"` -> `"hillCharts"`. */
type AccessorName<Name extends string> = Name extends `${infer Base}Service` ? Uncapitalize<Base> : never;

type ExpectedAccessor = AccessorName<GeneratedServiceClassName>;

/**
 * Non-vacuity floor, in the only currency available here: if the barrel import
 * resolved to nothing, `ExpectedAccessor` would be `never`, `Missing` would be
 * `never` too, and the assertion below would pass while checking nothing. Three
 * accessors are named — one plain, one multi-word, and `gauges`, the service
 * that shipped unreachable in Python (#732) — so the union has to be populated
 * and correctly transformed for this file to compile at all.
 */
type RosterIsPopulated = "projects" extends ExpectedAccessor
  ? "myNotifications" extends ExpectedAccessor
    ? "gauges" extends ExpectedAccessor
      ? true
      : false
    : false
  : false;

export type AccessorRosterIsPopulated = Expect<RosterIsPopulated>;

/**
 * Any generated service whose accessor is absent from the interface survives
 * the `Exclude`. The surviving union is then handed to `Expect` rather than a
 * bare `false`, so the TS2344 message names the missing accessors — `Type
 * '"gauges"' does not satisfy the constraint 'true'` rather than `Type
 * 'false'`. `[X] extends [never]` is the non-distributing spelling, which is
 * what makes the empty case answer `true` for a union of any size.
 */
type MissingFromInterface = Exclude<ExpectedAccessor, keyof BasecampClient>;

export type NoAccessorMissingFromBasecampClient = Expect<
  [MissingFromInterface] extends [never] ? true : MissingFromInterface
>;
