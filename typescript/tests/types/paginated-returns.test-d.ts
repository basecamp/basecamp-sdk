/**
 * Type-level assertions for the return types of paginated service methods.
 *
 * Every generated method that calls `requestPaginated` resolves to a
 * `ListResult<T>` at runtime — an Array subclass carrying `.meta` — and its
 * JSDoc promises ".meta.totalCount". This file pins that promise in the type
 * system, so a generator change that drops the wrapper (issue #737: four
 * methods declared the bare `*ResponseContent` array because their entity had
 * no `TYPE_ALIASES` entry) fails the typecheck instead of shipping.
 *
 * There is no runtime here: the assertions are checked by `tsc`, via
 * `tsconfig.test.json` / `make ts-typecheck`. The `.test-d.ts` suffix keeps
 * vitest (`include: tests/**\/*.test.ts`) from collecting a file with no tests
 * in it.
 */
import type { ListMeta, ListResult } from "../../src/pagination.js";
import type { CheckinsService } from "../../src/generated/services/checkins.js";
import type { GaugesService } from "../../src/generated/services/gauges.js";
import type { ReportsService } from "../../src/generated/services/reports.js";
import type { SearchService } from "../../src/generated/services/search.js";
import type { TodosService } from "../../src/generated/services/todos.js";

/** Compiles only when `T` is `true`; anything else is a TS2344 constraint error. */
type Expect<T extends true> = T;

type AsyncMethod = (...args: never[]) => Promise<unknown>;

/** What awaiting a service method yields. */
type Returned<M extends AsyncMethod> = Awaited<ReturnType<M>>;

/**
 * `ListResult<T>` extends `Array<T>`, so this predicate has to be read in the
 * narrow direction: a plain `Gauge[]` lacks `.meta` and therefore does NOT
 * extend `ListResult<unknown>`, while any `ListResult<X>` does.
 */
type IsListResult<M extends AsyncMethod> = Returned<M> extends ListResult<unknown> ? true : false;

/** The list carries pagination metadata of the declared shape. */
type HasListMeta<M extends AsyncMethod> = Returned<M> extends { meta: ListMeta } ? true : false;

/**
 * The element type resolved to something concrete. `unknown extends T` is also
 * true for `any`, so both degenerate spellings are caught — this is what guards
 * against a `ListResult<unknown>` regression when an entity is unaliased.
 */
type ElementIsResolved<T> = unknown extends T ? false : true;

// -----------------------------------------------------------------------------
// The four methods from #737: paginated arrays whose entity has no TYPE_ALIASES
// entry (Gauge, GaugeNeedle, SearchResult, QuestionReminder).
// -----------------------------------------------------------------------------

export type ListGaugesIsListResult = Expect<IsListResult<GaugesService["listGauges"]>>;
export type ListGaugesHasMeta = Expect<HasListMeta<GaugesService["listGauges"]>>;
export type ListGaugesElement = Expect<ElementIsResolved<Returned<GaugesService["listGauges"]>[number]>>;

export type ListGaugeNeedlesIsListResult = Expect<IsListResult<GaugesService["listGaugeNeedles"]>>;
export type ListGaugeNeedlesHasMeta = Expect<HasListMeta<GaugesService["listGaugeNeedles"]>>;
export type ListGaugeNeedlesElement = Expect<
  ElementIsResolved<Returned<GaugesService["listGaugeNeedles"]>[number]>
>;

export type SearchIsListResult = Expect<IsListResult<SearchService["search"]>>;
export type SearchHasMeta = Expect<HasListMeta<SearchService["search"]>>;
export type SearchElement = Expect<ElementIsResolved<Returned<SearchService["search"]>[number]>>;

export type RemindersIsListResult = Expect<IsListResult<CheckinsService["reminders"]>>;
export type RemindersHasMeta = Expect<HasListMeta<CheckinsService["reminders"]>>;
export type RemindersElement = Expect<ElementIsResolved<Returned<CheckinsService["reminders"]>[number]>>;

// -----------------------------------------------------------------------------
// A method whose entity IS aliased, so the wrapper never depended on the
// fallback. Present so a regression that reaches every list method, not just
// the unaliased ones, is visible here too.
// -----------------------------------------------------------------------------

export type TodosListIsListResult = Expect<IsListResult<TodosService["list"]>>;
export type TodosListElement = Expect<ElementIsResolved<Returned<TodosService["list"]>[number]>>;

// -----------------------------------------------------------------------------
// Wrapped pagination: the list lives under a key of an object response. Its
// element type is resolved through the same TYPE_ALIASES lookup, whose miss
// spelled `ListResult<unknown>` before #737.
// -----------------------------------------------------------------------------

type PersonProgressEvents = Returned<ReportsService["personProgress"]>["events"];

export type PersonProgressIsListResult = Expect<PersonProgressEvents extends ListResult<unknown> ? true : false>;
export type PersonProgressElement = Expect<ElementIsResolved<PersonProgressEvents[number]>>;

// -----------------------------------------------------------------------------
// Negative control: the predicate has to be able to say "no". A single-entity
// method must NOT satisfy it — if this line ever compiles as `true`, the
// assertions above are vacuous.
// -----------------------------------------------------------------------------

export type SingleGaugeNeedleIsNotListResult = Expect<
  IsListResult<GaugesService["gaugeNeedle"]> extends false ? true : false
>;
