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
 * Both directions belong here, and only here. The three renderings the runtime
 * file covers are each checked both ways — nothing missing, and nothing extra —
 * but its "nothing extra" pass filters `Object.keys(client)`, which cannot see a
 * member that exists solely in the type system. So an interface property typed
 * as some existing service but never wired by a `defineService` call compiles
 * clean, passes every runtime test, and hands a consumer calling
 * `client.fanfares.list()` a TypeError. That direction is asserted below too.
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
import type { BaseService } from "../../src/services/base.js";

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

/**
 * The interface's own service accessors, for the other direction — derived, not
 * listed, since a literal would be maintained by whoever added the stray
 * property.
 *
 * "Assignable to `BaseService`" is what separates the accessors from the
 * openapi-fetch verbs the interface inherits and from the client's own members,
 * without naming any of them: `BaseService` declares its `client` and `hooks`
 * `protected`, so structural assignability to it requires deriving from it, and
 * nothing else on the interface does. It discriminates exactly — the filter
 * keeps the 53 accessors plus `authorization` and drops all 14 of `GET`, `PUT`,
 * `POST`, `DELETE`, `OPTIONS`, `HEAD`, `PATCH`, `TRACE`, `use`, `eject`,
 * `request`, `raw`, `hooks` and `downloadURL`. The `-?` strips optionality so an
 * optional member is judged on its declared type rather than on
 * `T | undefined`.
 */
type ServiceAccessorKey<T> = { [K in keyof T]-?: T[K] extends BaseService ? K : never }[keyof T];

/**
 * `authorization` is the one accessor with no generated service behind it: OAuth
 * is not in the OpenAPI spec, so `AuthorizationService` is hand-written. The
 * runtime file carries the same one-name exclusion. A second hand-written
 * service would fail the assertion below until it is named here, which is the
 * direction of failure to want — it forces the decision instead of quietly
 * widening the rule.
 */
type InterfaceAccessor = Exclude<ServiceAccessorKey<BasecampClient>, "authorization">;

/**
 * The reverse direction's own non-vacuity floor, and it needs one more than
 * anything else in this file: a mapped type that resolved to `never` — a renamed
 * `BaseService`, a filter that stopped discriminating in the other direction —
 * makes the `Exclude` below empty and the assertion trivially true. The same
 * three accessors the forward floor names, so a broken derivation cannot pass.
 */
type InterfaceRosterIsPopulated = "projects" extends InterfaceAccessor
  ? "myNotifications" extends InterfaceAccessor
    ? "gauges" extends InterfaceAccessor
      ? true
      : false
    : false
  : false;

export type InterfaceAccessorRosterIsPopulated = Expect<InterfaceRosterIsPopulated>;

/**
 * An interface property typed as a service but backed by no generated service
 * survives this `Exclude`, and is handed to `Expect` by name for the same reason
 * as above — `Type '"fanfares"' does not satisfy the constraint 'true'` says
 * which accessor the client will not actually carry.
 */
type BeyondGeneratedServices = Exclude<InterfaceAccessor, ExpectedAccessor>;

export type NoAccessorBeyondGeneratedServices = Expect<
  [BeyondGeneratedServices] extends [never] ? true : BeyondGeneratedServices
>;
