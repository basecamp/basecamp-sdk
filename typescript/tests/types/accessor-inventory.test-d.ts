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
 * Three, because both of those compare only names. An accessor can be present,
 * backed by a generated service, and declared as the *wrong* one; the runtime
 * file cannot see that either, since the getter still returns the right class.
 * The declared type is checked against the generated service last, and that
 * assertion carries a measured account of what structural typing still lets
 * through.
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

/**
 * Both directions above compare *names*, and a name is not an API. Retype
 * `readonly clientApprovals: ClientRepliesService` and nothing said so far
 * moves: the accessor is still on the interface and still backed by a generated
 * service, so `MissingFromInterface` and `BeyondGeneratedServices` are both
 * still empty. What changed is everything a consumer can call on it, and the
 * runtime file cannot see it — the getter really does return a
 * `ClientApprovalsService`, so `toBeInstanceOf` is satisfied and 109 assertions
 * stay green while `client.clientApprovals.get(123)` stops typechecking outside
 * this repo.
 *
 * That is the identity axis the runtime file already asserts for the three
 * renderings it can reach (`expect(service).toBeInstanceOf(cls)`), asked of the
 * fourth. The generated service each accessor is *paired with* is derived by
 * inverting the same name rule, so nothing is listed here either.
 */
type GeneratedServiceFor<Accessor extends string> = {
  [Name in GeneratedServiceClassName]: AccessorName<Name> extends Accessor
    ? InstanceType<(typeof GeneratedServices)[Name]>
    : never;
}[GeneratedServiceClassName];

/**
 * Assignable to the generated service, and unchanged on the members it declares.
 *
 * The first clause cannot be identity: six accessors (§18's merge-safe
 * composites — todos, todolists, cards, documents, uploads, schedules) are
 * declared as hand-written subclasses that add methods to the generated service,
 * and the runtime file makes the same allowance for the index.ts exports
 * (`exported === cls || exported.prototype instanceof cls`). So extra members
 * are fine.
 *
 * On their own, extra members are also how a *different* service satisfies the
 * first clause — structural typing does not care that `ClientCorrespondences`
 * is a different service, only that it happens to carry a compatible `list` and
 * `get`. The second clause is what stops that without a roster of which six are
 * subclassed: restricted to the members the generated service declares, the two
 * must agree in both directions. A subclass agrees by definition, because it
 * inherits those members verbatim; an unrelated service agrees only if its
 * signatures are identical, not merely compatible.
 *
 * MEASURED, not assumed. Over all 53x52 ordered pairs of generated services,
 * plain assignability admits 45 wrong pairings; this rule admits 11. The
 * remainder is structural typing's floor, and one pair shows why it cannot be
 * driven to zero: `CardTablesService` and `MessageBoardsService` are a single
 * `get(bucketId, id)` each, over schema types that are themselves mutually
 * assignable, so the two are indistinguishable to any type-level rule whatsoever
 * — no assertion written here can tell `cardTables` from `messageBoards`. The
 * other nine are cross-domain pairs (`campfires`/`schedules`/`vaults` declared as
 * one of those two, `documents` as `clientApprovals`, `forwards` as
 * `clientCorrespondences`, `todolists` as `todolistGroups`) that survive for the
 * same reason: the target declares few enough members that a richer service
 * matches all of them exactly. The pairing Codex named — `clientApprovals`
 * declared as `ClientRepliesService` — is not among them and is pinned as a
 * floor below.
 */
type DeclaresItsGeneratedService<Declared, Generated> = Declared extends Generated
  ? Generated extends Pick<Declared, keyof Generated & keyof Declared>
    ? true
    : false
  : false;

/**
 * This direction's non-vacuity floor, and it takes three parts because the rule
 * above can degenerate in three ways. The mapping can resolve to `never` — every
 * accessor would then be reported, so that one fails loudly rather than
 * silently, but it is cheap to name. The rule can degenerate to `false`, which
 * the second part catches by pairing a service with itself. And the rule can
 * degenerate to `true`, which nothing else here would notice at all: the third
 * part pins the mutation the finding is about, asserting that
 * `ClientRepliesService` is *rejected* for `clientApprovals`.
 */
type IdentityRuleDiscriminates = [GeneratedServiceFor<"gauges">] extends [never]
  ? false
  : DeclaresItsGeneratedService<
        InstanceType<typeof GeneratedServices.GaugesService>,
        GeneratedServiceFor<"gauges">
      > extends true
    ? DeclaresItsGeneratedService<
          InstanceType<typeof GeneratedServices.ClientRepliesService>,
          GeneratedServiceFor<"clientApprovals">
        > extends false
      ? true
      : false
    : false;

export type AccessorIdentityRuleDiscriminates = Expect<IdentityRuleDiscriminates>;

/**
 * Any accessor whose declared type is not its generated service survives, and is
 * named by `Expect` for the same reason as above. Accessors absent from the
 * interface are skipped rather than reported twice — that is
 * `NoAccessorMissingFromBasecampClient`'s question, and `BasecampClient[K]` is
 * not even a legal lookup for them.
 */
type MisdeclaredAccessor = {
  [K in ExpectedAccessor]: K extends keyof BasecampClient
    ? DeclaresItsGeneratedService<BasecampClient[K], GeneratedServiceFor<K>> extends true
      ? never
      : K
    : never;
}[ExpectedAccessor];

export type EveryAccessorDeclaresItsGeneratedService = Expect<
  [MisdeclaredAccessor] extends [never] ? true : MisdeclaredAccessor
>;
