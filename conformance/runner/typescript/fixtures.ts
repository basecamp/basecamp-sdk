/**
 * Fixture-ID resolution for live canary tests.
 *
 * Resolution ladder (per §5d of the BC5-readiness plan):
 *   1. Explicit per-backend env var, e.g. BASECAMP_BC4_PROJECT_ID
 *   2. Generic env var, e.g. BASECAMP_PROJECT_ID
 *   3. Discovery via the SDK (ListProjects → first project; etc.)
 *   4. Fall through: undefined; caller skips with skipReason.
 *
 * Resolution is cached per-backend so discovery only fires once per run.
 */

import type { BasecampClient } from "@37signals/basecamp";

export type Backend = "bc4" | "bc5" | "unknown";

export interface FixtureContext {
  client: BasecampClient;
  backend: Backend;
}

const cache = new Map<string, string | null>();

function cacheKey(backend: Backend, name: string): string {
  return `${backend}:${name}`;
}

function fromEnv(backend: Backend, name: string): string | undefined {
  const upper = name.toUpperCase();
  if (backend !== "unknown") {
    const explicit = process.env[`BASECAMP_${backend.toUpperCase()}_${upper}`];
    if (explicit) return explicit;
  }
  const generic = process.env[`BASECAMP_${upper}`];
  if (generic) return generic;
  return undefined;
}

/**
 * Resolve a fixture-ID by name. Returns the resolved string or undefined if
 * not resolvable; caller is responsible for the skip-with-reason path.
 *
 * Discovery walks:
 *   PROJECT_ID    → ListProjects → first project
 *   TODOSET_ID    → walk dock of resolved project, pick first todoset tool
 *   TODOLIST_ID   → ListTodolists for resolved todoset, pick first
 *   TODO_ID       → ListTodos for resolved todolist, pick first
 */
export async function resolveFixtureId(
  ctx: FixtureContext,
  name: string,
): Promise<string | undefined> {
  const key = cacheKey(ctx.backend, name);
  if (cache.has(key)) {
    const cached = cache.get(key);
    return cached ?? undefined;
  }

  const env = fromEnv(ctx.backend, name);
  if (env) {
    cache.set(key, env);
    return env;
  }

  let resolved: string | undefined;
  try {
    switch (name) {
      case "PROJECT_ID": {
        const projects = await ctx.client.projects.list({ maxItems: 1 });
        const first = projects[0] as { id?: number } | undefined;
        if (first?.id !== undefined) resolved = String(first.id);
        break;
      }
      case "TODOSET_ID": {
        const projectId = await resolveFixtureId(ctx, "PROJECT_ID");
        if (!projectId) break;
        const project = await ctx.client.projects.get(Number(projectId));
        const dock = (project as { dock?: Array<{ name?: string; id?: number }> }).dock ?? [];
        const todoset = dock.find((tool) => tool.name === "todoset");
        if (todoset?.id !== undefined) resolved = String(todoset.id);
        break;
      }
      case "TODOLIST_ID": {
        const todosetId = await resolveFixtureId(ctx, "TODOSET_ID");
        if (!todosetId) break;
        const todolists = await ctx.client.todolists.list(Number(todosetId), { maxItems: 1 });
        const first = todolists[0] as { id?: number } | undefined;
        if (first?.id !== undefined) resolved = String(first.id);
        break;
      }
      case "TODO_ID": {
        const todolistId = await resolveFixtureId(ctx, "TODOLIST_ID");
        if (!todolistId) break;
        const todos = await ctx.client.todos.list(Number(todolistId), { maxItems: 1 });
        const first = todos[0] as { id?: number } | undefined;
        if (first?.id !== undefined) resolved = String(first.id);
        break;
      }
    }
  } catch {
    // Discovery is best-effort; failures fall through to skip.
  }

  cache.set(key, resolved ?? null);
  return resolved;
}

/** Test-only helper: clear the cache between runs. */
export function _resetFixtureCache(): void {
  cache.clear();
}
