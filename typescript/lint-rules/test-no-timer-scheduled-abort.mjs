/**
 * Self-test for the no-timer-scheduled-abort oxlint rule.
 *
 * oxlint has no RuleTester, so this drives the real binary over real fixture
 * files in a temp directory and asserts on the reported lines. That is a
 * feature rather than a compromise: it exercises plugin LOADING and rule
 * dispatch, which is precisely the path oxlint's alpha, non-semver JS plugin
 * API can break under a caret bump.
 *
 * Without this, such a break is a silent fail-open — the gate keeps exiting 0
 * while matching nothing. With it, the build reds. That is the whole reason it
 * runs alongside the rule rather than only when someone edits the rule.
 *
 * The two POSITIVE fixtures are the verbatim shapes this repo removed in #783
 * plus the one #715 removed, so "the gate is known to fail" is anchored to real
 * defects rather than to invented ones. The NEGATIVE fixtures are the seams
 * that replaced them, plus the shape a text selector gets wrong.
 */
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const CONFIG = join(HERE, "oxlintrc.json");

/** [name, source, expected 1-based lines that must report] */
const CASES = [
  [
    // #783, middleware-lifecycle.test.ts:244 — the form that survived #655's
    // sweep because its selector demanded a no-argument abort().
    "timer abort WITH a reason (the #655 survivor)",
    `const controller = new AbortController();
const reason = new Error("caller cancelled");
export const abortTimer = setTimeout(() => controller.abort(reason), 50);
`,
    [3],
  ],
  [
    // client.test.ts:586 before #715 — the one shape #655's selector did match.
    "timer abort with NO argument (what #655 itself caught)",
    `const controller = new AbortController();
export const abortTimer = setTimeout(() => controller.abort(), 50);
`,
    [2],
  ],
  [
    // Spelling variations the original rg could not have seen. If the rule
    // regressed to text matching, these are the first to go quiet.
    "block body, member-spelled timer, computed abort, nested function",
    `const controller = new AbortController();
export const a = globalThis.setTimeout(() => { controller.abort(); }, 10);
export const b = setInterval(function () { controller["abort"](); }, 10);
export const c = setTimeout(() => { const inner = () => controller.abort(); inner(); }, 10);
`,
    [2, 3, 4],
  ],
  [
    // The seams that replaced them: abort from inside the MSW handler, and
    // abort queued from the retry hook. Neither is timer-scheduled.
    "in-flight seams are allowed",
    `const controller = new AbortController();
const reason = new Error("x");
export const handler = async () => {
  controller.abort(reason);
  await new Promise((r) => setTimeout(r, 1000));
};
export const onRetry = () => {
  queueMicrotask(() => controller.abort(reason));
};
`,
    [],
  ],
  [
    // device.test.ts:1632-1640. A never-settling 60s display hook, with the
    // abort as a separate synchronous statement afterwards: the timer is the
    // thing being BEATEN, not the thing scheduling the abort. A proximity grep
    // flags this; the AST does not, and that difference is why this is a rule
    // and not a `make` target running rg.
    "a timer the abort races, rather than one that schedules it",
    `const controller = new AbortController();
export const display = () =>
  new Promise((resolve) => {
    setTimeout(() => {
      resolve();
    }, 60_000);
  });
controller.abort();
`,
    [],
  ],
  [
    // A stubbed sleep or fetch that aborts is deterministic — the abort lands
    // when the code under test calls it, not when a clock says so.
    "abort from a stubbed sleep or fetch is allowed",
    `const controller = new AbortController();
export const sleepStub = (_ms) => {
  controller.abort();
  return Promise.resolve();
};
`,
    [],
  ],
  [
    // The sanctioned escape hatch. If this stops working, the rule has no
    // legitimate exception path and becomes a reason to delete it.
    "an inline suppression with a reason is honored",
    `const controller = new AbortController();
// oxlint-disable-next-line basecamp-tests/no-timer-scheduled-abort -- this test's SUBJECT is a caller's own timer
export const t = setTimeout(() => controller.abort(), 50);
`,
    [],
  ],
];

const dir = mkdtempSync(join(tmpdir(), "abort-timer-rule-"));
const failures = [];

try {
  for (const [name, source, expected] of CASES) {
    const file = join(dir, "case.ts");
    writeFileSync(file, source);

    let output = "";
    try {
      output = execFileSync("npx", ["oxlint", "-c", CONFIG, file], {
        cwd: join(HERE, ".."),
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
      });
    } catch (error) {
      // oxlint exits 1 when it reports an error; that is a result, not a crash.
      // Anything else — a plugin that failed to load, a config it could not
      // parse — must fail loudly rather than read as "no findings".
      if (error.status !== 1) {
        failures.push(`${name}: oxlint exited ${error.status}\n${error.stdout ?? ""}${error.stderr ?? ""}`);
        continue;
      }
      output = `${error.stdout ?? ""}${error.stderr ?? ""}`;
    }

    const reported = [...output.matchAll(/case\.ts:(\d+):\d+: error basecamp-tests\(no-timer-scheduled-abort\)/g)]
      .map((m) => Number(m[1]))
      .sort((a, b) => a - b);

    const want = JSON.stringify(expected);
    const got = JSON.stringify(reported);
    if (want !== got) {
      failures.push(`${name}: expected reports on lines ${want}, got ${got}\n${output}`);
    }
  }
} finally {
  rmSync(dir, { recursive: true, force: true });
}

if (failures.length > 0) {
  console.error("no-timer-scheduled-abort self-test FAILED:\n");
  for (const failure of failures) console.error(`  - ${failure}\n`);
  process.exit(1);
}

console.log(`no-timer-scheduled-abort self-test: ${CASES.length} cases passed.`);
