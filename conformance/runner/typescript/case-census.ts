/**
 * Case census (#602): every non-live fixture case must be accounted for by the
 * run.
 *
 *     registered cases  ==  every JSON case under conformance/tests,
 *                           recursively, whose mode != "live"
 *
 * The left side is what the runner actually registered with vitest. The right
 * side is counted by `countNonLiveCases` below — a SEPARATE walk and parse,
 * deliberately not the runner's own load path. That independence is the entire
 * point: a check fed by the load path can only confirm the load path agrees
 * with itself.
 *
 * Why `mode !== "live"` rather than `mode === "mock"`: all six runners select
 * with "mock unless told otherwise" (`isMockMode` here, and its five
 * equivalents), so a typo'd `mode: "moc"` is dropped by every runner at once
 * with nothing printed anywhere. Counting the expected side as "not explicitly
 * live" turns that silent divergence into arithmetic.
 *
 * Catches: an unrecognized `mode`; a fixture that failed to parse or was never
 * globbed (including one nested below `conformance/tests/`, which no runner
 * discovers — hence the recursive walk); a case dropped between load and
 * dispatch; a fixture emptied to `[]` (which the census REFUSES rather than
 * counts — see `countNonLiveCases`, and note that counting it would make this
 * bullet a lie); and any future skip channel that bypasses registration,
 * because registration is what it reads.
 *
 * The typo is not this check's alone to catch, and saying so is what keeps the
 * rest of the list honest: `make conformance-fixtures-check` validates
 * `conformance/tests/*.json` against `conformance/schema.json`, whose `mode` is
 * `enum: ["mock", "live"]`, so a typo in a TOP-LEVEL fixture fails there first
 * and this census is defense in depth for that one case. What that gate
 * structurally cannot see is everything else above — its glob is not recursive,
 * so a fixture nested below `conformance/tests/` is validated by nothing AND run
 * by nothing (verified: such a file passes the schema gate and fails this
 * census); a fixture truncated to `[]` is a valid array of zero cases; and a
 * case dropped between load and registration is not a fixture-format question at
 * all. Nor does that gate run when `make conformance-typescript` is invoked
 * alone.
 *
 * Does NOT catch the all-six case #602 names — one case every runner excludes
 * for its own reason, which leaves each runner's own census green. That needs
 * the six exclusion sets in one place, hence artifact plumbing across six CI
 * jobs; #602 stays open for it.
 *
 * TYPESCRIPT COUNTS REGISTRATIONS, NOT OUTCOMES, and that is deliberate. vitest
 * owns pass/fail/skip accounting and `it.skip` is how this runner expresses a
 * skip, so there are no counters to sum the way the other five runners do.
 * Asserting on the number of registered cases is the same invariant expressed
 * where vitest can see it: a case dropped at load never becomes an `it` at all.
 * The TS lane is also the one with no `SKIP: <name>` line, so a dropped case
 * leaves no trace in its output either.
 */

import * as fs from "node:fs";
import * as path from "node:path";

/**
 * Whether a fixture case's `mode` selects the mock runner.
 *
 * Absent means mock: live cases belong to live-runner.test.ts (the canonical
 * wire-capturer), and every other value is nobody's. Shared with the census
 * self-tests so the rule the load path applies is the rule under test, not a
 * copy of it.
 */
export function isMockMode(mode: string | undefined): boolean {
  return (mode ?? "mock") === "mock";
}

/** Every `*.json` file under `dir`, recursively, sorted by path. */
function fixtureFiles(dir: string): string[] {
  const found: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      found.push(...fixtureFiles(full));
    } else if (entry.isFile() && entry.name.endsWith(".json")) {
      found.push(full);
    }
  }
  return found.sort();
}

/**
 * Counts fixture cases whose mode is not `"live"`, recursively.
 *
 * Fail-closed in three places, each a way the count could certify nothing while
 * looking green: an unreadable tree, a fixture that does not parse, and a walk
 * that found no fixture files at all.
 */
export function countNonLiveCases(testsDir: string): number {
  const files = fixtureFiles(testsDir);
  if (files.length === 0) {
    throw new Error(`no *.json fixture files found under ${testsDir}`);
  }

  let cases = 0;
  for (const file of files) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(fs.readFileSync(file, "utf-8"));
    } catch (err) {
      throw new Error(`${file}: ${err instanceof Error ? err.message : String(err)}`);
    }
    if (!Array.isArray(parsed)) {
      throw new Error(`${file}: fixture is not a JSON array`);
    }
    // An emptied fixture is REFUSED, not counted as zero, and this is the one
    // rejection that carries the whole-file guarantee. It is the single
    // truncation both sides of the census read identically: the runner
    // registers nothing from the file and the census expects nothing, so the
    // two totals fall together and no mismatch ever appears. Counting it would
    // make "a fixture truncated to []" a claim this check cannot keep. A file
    // declaring no cases tests nothing, so refusing it costs nothing — and it
    // closes the same hole in conformance-fixtures-check, where an empty array
    // is a schema-valid list of zero items.
    if (parsed.length === 0) {
      throw new Error(
        `${file}: fixture declares no cases; delete the file or restore its cases`,
      );
    }
    // Only `mode` is read: the census must survive a fixture whose other fields
    // this runner cannot model, or it would report a failure for a case the run
    // itself handled fine.
    cases += parsed.filter((tc) => (tc as { mode?: string })?.mode !== "live").length;
  }
  return cases;
}

/**
 * Compares what the run registered against the census, returning null when they
 * agree and a message naming the short side otherwise.
 */
export function caseCountFailure(registered: number, expected: number): string | null {
  if (registered === expected) return null;
  if (registered < expected) {
    return (
      `case census: the run registered ${registered} case(s) but conformance/tests ` +
      `holds ${expected} non-live case(s) — ${expected - registered} executed by ` +
      "nothing. An unrecognized `mode`, a fixture that failed to parse or was never " +
      "globbed, or a case dropped between load and registration will do this."
    );
  }
  return (
    `case census: the run registered ${registered} case(s) but conformance/tests ` +
    `holds only ${expected} non-live case(s) — ${registered - expected} more than ` +
    "the fixtures declare."
  );
}
