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

  ListRecentProjects: {
    fixtures: [],
    call: async (ctx) => {
      const result = await ctx.client.projects.listRecentProjects();
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

  ListRecordings: {
    fixtures: [],
    call: async (ctx) => {
      // Backs the type=Door external-links canary; validates the door shape
      // (external url + service + description) against the Recording schema.
      //
      // maxItems caps pagination: without it, requestPaginated would follow every
      // Link page for an account with many Door recordings — potentially a large
      // number of credentialed requests and an oversized snapshot. A handful of
      // rows is plenty to validate the Recording (Door) schema.
      const result = await ctx.client.recordings.list("Door", { maxItems: 5 });
      return { resolvedIds: {}, result };
    },
  },

  GetProgressReport: {
    fixtures: [],
    call: async (ctx) => {
      // maxItems caps pagination: the progress feed can be very long on an
      // active account, and requestPaginated would otherwise follow every Link
      // page. A small sample is enough to validate the TimelineEvent schema.
      const result = await ctx.client.reports.progress({ maxItems: 5 });
      return { resolvedIds: {}, result };
    },
  },

  GetBubbleUps: {
    fixtures: [],
    call: async (ctx) => {
      // maxItems caps pagination: bubble_ups is a paginated feed (50/page), so a
      // small sample validates the Notification schema without following every
      // Link page on an active account.
      const result = await ctx.client.myNotifications.bubbleUps({ maxItems: 5 });
      return { resolvedIds: {}, result };
    },
  },

  // Everything aggregates — flat family (one canary per group). The paginated
  // roots pass maxItems so live mode samples each account-wide feed instead of
  // following every Link page; the overdue lists are unpaginated (no options).
  GetEverythingMessages: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingMessages({ maxItems: 5 }) }),
  },
  GetEverythingComments: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingComments({ maxItems: 5 }) }),
  },
  GetEverythingCheckins: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingCheckins({ maxItems: 5 }) }),
  },
  GetEverythingForwards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingForwards({ maxItems: 5 }) }),
  },
  GetEverythingFiles: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingFiles({ maxItems: 5 }) }),
  },
  GetEverythingOverdueTodos: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingOverdueTodos() }),
  },
  GetEverythingOverdueCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingOverdueCards() }),
  },

  // Everything aggregates — bucket-grouped family (one canary per group). All
  // are Link-paginated, so each passes maxItems to sample rather than walk every
  // page in live mode.
  GetEverythingOpenTodos: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingOpenTodos({ maxItems: 5 }) }),
  },
  GetEverythingCompletedTodos: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingCompletedTodos({ maxItems: 5 }) }),
  },
  GetEverythingUnassignedTodos: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingUnassignedTodos({ maxItems: 5 }) }),
  },
  GetEverythingNoDueDateTodos: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingNoDueDateTodos({ maxItems: 5 }) }),
  },
  GetEverythingOpenCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingOpenCards({ maxItems: 5 }) }),
  },
  GetEverythingCompletedCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingCompletedCards({ maxItems: 5 }) }),
  },
  GetEverythingUnassignedCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingUnassignedCards({ maxItems: 5 }) }),
  },
  GetEverythingNoDueDateCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingNoDueDateCards({ maxItems: 5 }) }),
  },
  GetEverythingNotNowCards: {
    fixtures: [],
    call: async (ctx) => ({ resolvedIds: {}, result: await ctx.client.everything.everythingNotNowCards({ maxItems: 5 }) }),
  },

  GetCalendar: {
    // CALENDAR_ID resolves via env vars only (BASECAMP_BC5_CALENDAR_ID →
    // BASECAMP_CALENDAR_ID); unconfigured runs skip through the fixture
    // ladder. See fixtures.ts — no discovery path exists for calendar ids.
    fixtures: ["CALENDAR_ID"],
    call: async (ctx, ids) => {
      const result = await ctx.client.calendars.getCalendar(Number(ids.CALENDAR_ID));
      return { resolvedIds: ids, result };
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

  Search: {
    fixtures: [],
    call: async (ctx) => {
      // Drives the absorbed BC5 filter surface end to end: the type_names[]
      // array flows through the SDK's bracketed wire encoding, plus the since
      // time filter. New params are silently ignored on BC4, accepted on BC5.
      // Assertions prove 2xx + schema validity only (acceptance/decoding
      // coverage) — not that the backend actually respects the filter.
      //
      // maxItems caps pagination: without it, requestPaginated would follow
      // every Link page for a common term like "the" against a busy account —
      // potentially thousands of credentialed requests. A handful of results is
      // plenty to validate the SearchResult schema.
      const result = await ctx.client.search.search("the", {
        typeNames: ["Message"],
        since: "last_30_days",
        maxItems: 5,
      });
      return { resolvedIds: {}, result };
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
