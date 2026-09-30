/**
 * Type-level assertions for the CardStep -> Subtask rename.
 *
 * The shape both SubtasksService and CardStepsService return was the
 * `CardStep` component until it was renamed to `Subtask`. `CardStep` stays
 * exported, from the package root and from both generated service modules, as a
 * deprecated alias of `Subtask`, so a caller that spells the old name keeps
 * compiling. This file pins that promise: the old and new spellings are
 * mutually assignable (the same type), both services return it, and the
 * `components["schemas"]` entry openapi-typescript emits for the retained
 * `CardStep` component resolves to it too.
 *
 * Checked by `tsc` via `tsconfig.test.json` / `make ts-typecheck`; the
 * `.test-d.ts` suffix keeps vitest from collecting it (see
 * paginated-returns.test-d.ts for why this is not a declaration file).
 */
import type { CardStep, Subtask } from "../../src/index.js";
import type {
  CardStep as CardStepFromCardSteps,
  CardStepsService,
} from "../../src/generated/services/card-steps.js";
import type { SubtasksService } from "../../src/generated/services/subtasks.js";
import type { components } from "../../src/generated/schema.js";

/** Compiles only when `T` is `true`; anything else is a TS2344 constraint error. */
type Expect<T extends true> = T;

/** Mutual assignability: the two spellings name the same type. */
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;

type Returned<M extends (...args: never[]) => Promise<unknown>> = Awaited<ReturnType<M>>;

export type SubtaskRenameAssertions = [
  Expect<Same<CardStep, Subtask>>,
  Expect<Same<CardStepFromCardSteps, Subtask>>,
  Expect<Same<components["schemas"]["CardStep"], Subtask>>,
  Expect<Same<Returned<CardStepsService["get"]>, CardStep>>,
  Expect<Same<Returned<SubtasksService["get"]>, CardStep>>,
];
