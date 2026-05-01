/**
 * Live-mode operation dispatch for the canary.
 *
 * Maps test-fixture `operation` strings to actual SDK calls, with fixture-ID
 * resolution applied. Returns the SDK's typed result; wire bytes are captured
 * separately via the global fetch wrapper in wire-capture.ts, so this module
 * only needs to drive the SDK.
 *
 * The exported `LIVE_OPERATIONS` set is the single source of truth for the
 * coverage gate: any operation referenced by a live test must appear here,
 * or the runner refuses to start.
 */

import type { BasecampClient } from "@37signals/basecamp";
import { resolveFixtureId, type FixtureContext } from "./fixtures.js";

export interface DispatchResult {
  /** Resolved fixture-ID values, for diagnostics. */
  resolvedIds: Record<string, string>;
  /** SDK-decoded result (for downstream decode-success reporting). */
  result?: unknown;
}

export class FixtureMissingError extends Error {
  constructor(public readonly fixtureName: string) {
    super(`Fixture ID ${fixtureName} not available`);
    this.name = "FixtureMissingError";
  }
}

async function need(ctx: FixtureContext, name: string, into: Record<string, string>): Promise<string> {
  const value = await resolveFixtureId(ctx, name);
  if (!value) throw new FixtureMissingError(name);
  into[name] = value;
  return value;
}

export type DispatchFn = (ctx: FixtureContext) => Promise<DispatchResult>;

export const LIVE_OPERATIONS: Record<string, DispatchFn> = {
  ListProjects: async (ctx) => {
    const result = await ctx.client.projects.list();
    return { resolvedIds: {}, result };
  },

  GetProject: async (ctx) => {
    const ids: Record<string, string> = {};
    const projectId = await need(ctx, "PROJECT_ID", ids);
    const result = await ctx.client.projects.get(Number(projectId));
    return { resolvedIds: ids, result };
  },

  GetMyAssignments: async (ctx) => {
    const result = await ctx.client.myAssignments.myAssignments();
    return { resolvedIds: {}, result };
  },

  GetMyCompletedAssignments: async (ctx) => {
    const result = await ctx.client.myAssignments.myCompletedAssignments();
    return { resolvedIds: {}, result };
  },

  GetMyDueAssignments: async (ctx) => {
    const result = await ctx.client.myAssignments.myDueAssignments();
    return { resolvedIds: {}, result };
  },

  GetMyNotifications: async (ctx) => {
    const result = await ctx.client.myNotifications.myNotifications();
    return { resolvedIds: {}, result };
  },

  GetMyProfile: async (ctx) => {
    const result = await ctx.client.people.me();
    return { resolvedIds: {}, result };
  },

  GetTodoset: async (ctx) => {
    const ids: Record<string, string> = {};
    const todosetId = await need(ctx, "TODOSET_ID", ids);
    const result = await ctx.client.todosets.get(Number(todosetId));
    return { resolvedIds: ids, result };
  },

  ListTodolists: async (ctx) => {
    const ids: Record<string, string> = {};
    const todosetId = await need(ctx, "TODOSET_ID", ids);
    const result = await ctx.client.todolists.list(Number(todosetId));
    return { resolvedIds: ids, result };
  },

  ListTodos: async (ctx) => {
    const ids: Record<string, string> = {};
    const todolistId = await need(ctx, "TODOLIST_ID", ids);
    const result = await ctx.client.todos.list(Number(todolistId));
    return { resolvedIds: ids, result };
  },
};

/**
 * Validate that every operation referenced in the fixture has a dispatch
 * case. Throws on first missing operation so the runner refuses to start
 * with incomplete coverage.
 */
export function assertDispatchCoverage(operationsInFixture: string[]): void {
  const missing = operationsInFixture.filter((op) => !(op in LIVE_OPERATIONS));
  if (missing.length === 0) return;
  throw new Error(
    `Live runner is missing dispatch cases for: ${missing.join(", ")}. ` +
      `Add a DispatchFn to LIVE_OPERATIONS in live-dispatch.ts.`,
  );
}
