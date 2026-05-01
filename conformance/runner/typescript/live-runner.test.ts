/**
 * Live-mode conformance runner.
 *
 * Loads only mode="live" tests from conformance/tests/, dispatches each
 * through the SDK against a real Basecamp backend, captures raw wire
 * responses, validates them against the OpenAPI response schema, and
 * reports per-test pass/skip/fail.
 *
 * Gating: opt-in via BASECAMP_LIVE=1. Without it, the entire suite skips
 * — make check stays fully offline.
 *
 * This runner is the canonical wire-capturer (per §5f of the BC5-readiness
 * plan); other-language runners replay these snapshots in PR 3.
 */

import { describe, it, beforeAll, expect } from "vitest";
import { createBasecampClient } from "@37signals/basecamp";
import type { BasecampClient } from "@37signals/basecamp";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

import { installWireCapture, type WireSnapshot, type WirePage } from "./wire-capture.js";
import { validateResponse, type ValidationResult } from "./schema-validator.js";
import {
  LIVE_OPERATIONS,
  assertDispatchCoverage,
  FixtureMissingError,
  type DispatchResult,
} from "./live-dispatch.js";
import type { Backend, FixtureContext } from "./fixtures.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const TESTS_DIR = path.resolve(__dirname, "../../tests");

const LIVE_ENABLED = process.env.BASECAMP_LIVE === "1";
const TOKEN = process.env.BASECAMP_TOKEN;
const ACCOUNT_ID = process.env.BASECAMP_ACCOUNT_ID;
// BASECAMP_HOST is origin-only (e.g. https://3.basecampapi.com); we append
// /{accountId} to mirror createBasecampClient's default URL composition.
const HOST = process.env.BASECAMP_HOST?.replace(/\/$/, "");
const RECORD_DIR = process.env.LIVE_RECORD_DIR;
// Backend label namespaces snapshots so BC4 and BC5 runs don't collide.
const BACKEND: Backend = (process.env.BASECAMP_BACKEND ?? "unknown") as Backend;

interface LiveAssertion {
  type: "liveCallSucceeds" | "liveResponseFieldsRequired" | "liveResponseFieldsExpected" | "liveSchemaValidate";
  fields?: string[];
  enabled?: boolean;
}

interface LiveTestCase {
  mode: "live";
  name: string;
  description?: string;
  operation: string;
  fixtureIds?: Record<string, string>;
  liveAssertions: LiveAssertion[];
  tags?: string[];
}

interface RawTestCase {
  mode?: string;
  operation?: string;
}

function loadLiveTests(): { filename: string; tests: LiveTestCase[] }[] {
  const files = fs
    .readdirSync(TESTS_DIR)
    .filter((f) => f.endsWith(".json"))
    .sort();

  return files
    .map((filename) => {
      const content = fs.readFileSync(path.join(TESTS_DIR, filename), "utf-8");
      const all = JSON.parse(content) as RawTestCase[];
      const tests = all.filter((tc) => tc.mode === "live") as LiveTestCase[];
      return { filename, tests };
    })
    .filter((suite) => suite.tests.length > 0);
}

interface RunSummary {
  /** Field paths seen on raw wire bodies but not declared in the OpenAPI schema. */
  extrasObserved: Map<string, Set<string>>;
}

const summary: RunSummary = {
  extrasObserved: new Map(),
};

function recordExtras(operation: string, extras: string[]): void {
  if (extras.length === 0) return;
  let bucket = summary.extrasObserved.get(operation);
  if (!bucket) {
    bucket = new Set();
    summary.extrasObserved.set(operation, bucket);
  }
  for (const e of extras) bucket.add(e);
}

function persistSnapshot(testName: string, snapshot: WireSnapshot): void {
  if (!RECORD_DIR) return;
  const wireDir = path.join(RECORD_DIR, BACKEND, "wire");
  fs.mkdirSync(wireDir, { recursive: true });
  const safeName = testName.replace(/[^a-z0-9_-]+/gi, "_");
  const file = path.join(wireDir, `${safeName}.json`);
  fs.writeFileSync(file, JSON.stringify(snapshot, null, 2));
}

function checkRequiredFields(page: WirePage, fields: string[]): string[] {
  const errors: string[] = [];
  const body = page.body;
  for (const fieldPath of fields) {
    if (!fieldExists(body, fieldPath)) {
      errors.push(`required field absent: ${fieldPath}`);
    }
  }
  return errors;
}

function fieldExists(value: unknown, fieldPath: string): boolean {
  const parts = fieldPath.split(".");
  let cur: unknown = value;
  for (const part of parts) {
    if (cur === null || cur === undefined) return false;
    if (typeof cur !== "object") return false;
    if (Array.isArray(cur)) {
      // Array path means "every item must have this field". Empty arrays count as present.
      if (cur.length === 0) return true;
      const remaining = parts.slice(parts.indexOf(part)).join(".");
      return cur.every((item) => fieldExists(item, remaining));
    }
    if (!(part in (cur as Record<string, unknown>))) return false;
    cur = (cur as Record<string, unknown>)[part];
  }
  return true;
}

const LIVE_DESCRIBE = LIVE_ENABLED ? describe : describe.skip;

LIVE_DESCRIBE("conformance live runner", () => {
  const suites = LIVE_ENABLED ? loadLiveTests() : [];
  let client: BasecampClient | null = null;

  beforeAll(() => {
    if (!LIVE_ENABLED) return;

    const missing: string[] = [];
    if (!TOKEN) missing.push("BASECAMP_TOKEN");
    if (!ACCOUNT_ID) missing.push("BASECAMP_ACCOUNT_ID");
    if (missing.length > 0) {
      throw new Error(
        `Live mode requires env vars: ${missing.join(", ")}. ` +
          `BASECAMP_HOST is origin-only (e.g. https://3.basecampapi.com); the runner appends /{accountId}.`,
      );
    }

    const allOperations = suites.flatMap((suite) => suite.tests.map((t) => t.operation));
    assertDispatchCoverage(allOperations);

    const baseUrl = HOST ? `${HOST}/${ACCOUNT_ID}` : undefined;
    client = createBasecampClient({
      accountId: ACCOUNT_ID!,
      accessToken: TOKEN!,
      baseUrl,
    });
  });

  if (!LIVE_ENABLED) {
    it.skip("BASECAMP_LIVE not set — live canary skipped", () => {});
    return;
  }

  for (const { filename, tests } of suites) {
    describe(`live/${filename}`, () => {
      for (const tc of tests) {
        it(tc.name, async () => {
          const dispatch = LIVE_OPERATIONS[tc.operation];
          // Coverage is enforced in beforeAll, but this guards against races.
          if (!dispatch) {
            throw new Error(`No dispatch for operation ${tc.operation}`);
          }
          if (!client) {
            throw new Error("Live client not constructed");
          }

          const ctx: FixtureContext = { client, backend: BACKEND };
          const capture = installWireCapture();
          let dispatchResult: DispatchResult | undefined;
          let dispatchError: Error | undefined;

          try {
            dispatchResult = await dispatch(ctx);
          } catch (err) {
            dispatchError = err instanceof Error ? err : new Error(String(err));
          } finally {
            capture.restore();
          }

          if (dispatchError instanceof FixtureMissingError) {
            // Per §5d: fall through, skip with skipReason.
            return Promise.reject(
              Object.assign(new Error(`SKIP: Fixture ID for \${${dispatchError.fixtureName}} not available`), {
                skip: true,
              }),
            );
          }

          const snapshot = capture.drain();
          persistSnapshot(tc.name, snapshot);

          if (dispatchError) {
            throw new Error(`SDK dispatch threw: ${dispatchError.message}`);
          }

          const failures: string[] = [];
          const assertions = tc.liveAssertions ?? [];
          // liveSchemaValidate defaults to enabled — ensure it runs even if absent.
          const hasExplicitSchema = assertions.some((a) => a.type === "liveSchemaValidate");
          const effective = hasExplicitSchema
            ? assertions
            : [...assertions, { type: "liveSchemaValidate" } as LiveAssertion];

          for (const a of effective) {
            if (a.enabled === false) continue;

            if (a.type === "liveCallSucceeds") {
              if (snapshot.pages_count === 0) {
                failures.push("liveCallSucceeds: no pages captured");
                continue;
              }
              const firstStatus = snapshot.pages[0].status;
              if (firstStatus < 200 || firstStatus >= 300) {
                failures.push(`liveCallSucceeds: first page returned HTTP ${firstStatus}`);
              }
            }

            if (a.type === "liveSchemaValidate") {
              const result = validatePages(tc.operation, snapshot.pages);
              recordExtras(tc.operation, result.extras);
              if (!result.ok) {
                for (const err of result.errors) failures.push(`liveSchemaValidate: ${err}`);
              }
            }

            if (a.type === "liveResponseFieldsRequired") {
              const fields = a.fields ?? [];
              for (let i = 0; i < snapshot.pages.length; i++) {
                const errors = checkRequiredFields(snapshot.pages[i], fields);
                for (const e of errors) failures.push(`liveResponseFieldsRequired (page ${i + 1}): ${e}`);
              }
            }

            if (a.type === "liveResponseFieldsExpected") {
              const fields = a.fields ?? [];
              for (let i = 0; i < snapshot.pages.length; i++) {
                const errors = checkRequiredFields(snapshot.pages[i], fields);
                for (const e of errors) {
                  // eslint-disable-next-line no-console
                  console.warn(`[live-canary] ${tc.name}: liveResponseFieldsExpected (page ${i + 1}) ${e}`);
                }
              }
            }
          }

          if (failures.length > 0) {
            throw new Error(failures.join("\n"));
          }
        });
      }
    });
  }
});

function validatePages(operation: string, pages: WirePage[]): ValidationResult {
  if (pages.length === 0) {
    return { ok: false, errors: ["no pages to validate"], extras: [] };
  }
  const errors: string[] = [];
  const extras = new Set<string>();
  let allOk = true;
  for (let i = 0; i < pages.length; i++) {
    const result = validateResponse(operation, pages[i].body);
    if (!result.ok) {
      allOk = false;
      for (const e of result.errors) errors.push(`page ${i + 1}: ${e}`);
    }
    for (const e of result.extras) extras.add(e);
  }
  return { ok: allOk, errors, extras: [...extras] };
}

// After-all summary: emit extras observed so absorption planning has signal.
if (LIVE_ENABLED) {
  // Vitest hooks at module scope: this fires after the file finishes.
  process.on("beforeExit", () => {
    if (summary.extrasObserved.size === 0) return;
    // eslint-disable-next-line no-console
    console.log("\n[live-canary] Extras observed (raw wire fields not in OpenAPI schema):");
    for (const [op, fields] of summary.extrasObserved) {
      // eslint-disable-next-line no-console
      console.log(`  ${op}: ${[...fields].sort().join(", ")}`);
    }
  });
}

// Suppress unused-import warnings for type-only imports under noUnusedLocals.
export const _types: typeof expect = expect;
