/**
 * Live-mode operation dispatch for the canary.
 *
 * Each entry in `LIVE_OPERATIONS` declares (a) which fixture-IDs the call
 * needs and (b) the SDK call itself. The runner pre-resolves fixture-IDs
 * outside the wire-capture window so discovery traffic (e.g. the
 * `ListProjects` call that backs PROJECT_ID resolution) doesn't bleed into
 * the snapshot for the actual operation under test.
 *
 * `LIVE_OPERATIONS` is the single source of truth for the coverage gate:
 * any operation referenced by a live test must appear here, or the runner
 * refuses to start.
 */

import type { BasecampClient } from "@37signals/basecamp";
import type { FixtureContext } from "./fixtures.js";

export interface DispatchResult {
  /** Resolved fixture-ID values, for diagnostics. */
  resolvedIds: Record<string, string>;
  /** SDK-decoded result (for downstream decode-success reporting). */
  result?: unknown;
}

export interface DispatchSpec {
  /**
   * Fixture-ID names this operation requires. Pre-resolved by the runner
   * before wire capture starts; missing fixtures cause the test to skip.
   */
  fixtures: readonly string[];
  /** The SDK call itself, executed under wire capture. */
  call: (ctx: FixtureContext, ids: Record<string, string>) => Promise<DispatchResult>;
}

export const LIVE_OPERATIONS: Record<string, DispatchSpec> = {
  ListProjects: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.projects.list();
      return { resolvedIds: {}, result };
    },
  },

  GetProject: {
    fixtures: ["PROJECT_ID"],
    call: async (ctx, ids) => {
      const result = await ctx.client.projects.get(Number(ids.PROJECT_ID));
      return { resolvedIds: ids, result };
    },
  },

  GetMyAssignments: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.myAssignments.myAssignments();
      return { resolvedIds: {}, result };
    },
  },

  GetMyCompletedAssignments: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.myAssignments.myCompletedAssignments();
      return { resolvedIds: {}, result };
    },
  },

  GetMyDueAssignments: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.myAssignments.myDueAssignments();
      return { resolvedIds: {}, result };
    },
  },

  GetMyNotifications: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.myNotifications.myNotifications();
      return { resolvedIds: {}, result };
    },
  },

  GetMyProfile: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.people.me();
      return { resolvedIds: {}, result };
    },
  },

  GetTodoset: {
    fixtures: ["TODOSET_ID"],
    call: async (ctx, ids) => {
      const result = await ctx.client.todosets.get(Number(ids.TODOSET_ID));
      return { resolvedIds: ids, result };
    },
  },

  ListTodolists: {
    fixtures: ["TODOSET_ID"],
    call: async (ctx, ids) => {
      const result = await ctx.client.todolists.list(Number(ids.TODOSET_ID));
      return { resolvedIds: ids, result };
    },
  },

  ListTodos: {
    fixtures: ["TODOLIST_ID"],
    call: async (ctx, ids) => {
      const result = await ctx.client.todos.list(Number(ids.TODOLIST_ID));
      return { resolvedIds: ids, result };
    },
  },
};

/**
 * Validate that every operation referenced in the fixture has a dispatch
 * case. Uses `Object.hasOwn` rather than `in` so inherited keys
 * (`toString`, `hasOwnProperty`, etc.) can't sneak past the gate.
 */
export function assertDispatchCoverage(operationsInFixture: string[]): void {
  const missing = operationsInFixture.filter((op) => !Object.hasOwn(LIVE_OPERATIONS, op));
  if (missing.length === 0) return;
  throw new Error(
    `Live runner is missing dispatch cases for: ${missing.join(", ")}. ` +
      `Add a DispatchSpec to LIVE_OPERATIONS in live-dispatch.ts.`,
  );
}
