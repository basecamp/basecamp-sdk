// Refuse to run the conformance suite against a stale or missing SDK build.
//
// The runner consumes typescript/ through a `file:` link, and that package's
// exports point at typescript/dist — so the suite tests whatever was last built,
// not whatever is in src. The pretest hook used to rebuild on every run, which
// made staleness impossible but also meant the conformance target installed and
// built inside a directory shared with ts-check; `npm ci` there deletes
// node_modules mid-run and races the TypeScript checks under `make -j` (#612).
//
// So the build moved into the Make dependency graph (conformance-typescript
// depends on ts-build) and this took its place: a read-only freshness assertion
// that writes nothing and touches no shared state. `make conformance-typescript`
// satisfies it by construction; a bare `npm test` in this directory now fails
// with instructions instead of silently reporting green against stale code.

import { existsSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

// Overridable so scripts/test-assert-sdk-built can drive this at synthetic
// trees — including the deleted-source case, which cannot be staged in the real
// checkout without destroying it.
const SDK = process.env.CONFORMANCE_SDK_ROOT ?? "../../../typescript";
const ENTRY = join(SDK, "dist", "index.js");

// The marker `npm run postbuild` writes once tsc has emitted AND src/generated
// has been copied. dist/index.js is not proof of a successful build: tsconfig
// does not set noEmitOnError, so tsc can emit a partial dist and still exit
// non-zero, which skips postbuild entirely. The entry point then looks newer
// than every source while dist/generated is missing or stale. Compare against
// the marker, which only a completed build leaves behind.
const MARKER = join(SDK, "dist", ".build-complete");

function die(reason) {
  console.error(`Conformance runner: ${reason}`);
  console.error("Run: make conformance-typescript   (or make ts-build, then npm test here)");
  process.exit(1);
}

if (!existsSync(ENTRY)) die(`SDK is not built — ${ENTRY} is missing.`);

if (!existsSync(MARKER)) {
  die(
    `SDK build did not complete — ${MARKER} is missing. dist/ exists, so a build ` +
      "started; tsc emitting and then failing leaves exactly this state.",
  );
}

// Newest mtime anywhere in the SDK's sources, plus the inputs that change what
// tsc emits. A dist older than any of them cannot reflect the current tree.
//
// Directory mtimes count, not just file mtimes. Deleting a source leaves every
// remaining file older than dist while dist still carries the deleted module —
// no extant file records the change, but the parent directory's mtime does.
// Additions and renames land the same way.
function newestMtime(path) {
  let newest = 0;
  const stack = [path];
  while (stack.length > 0) {
    const current = stack.pop();
    let stats;
    try {
      stats = statSync(current);
    } catch {
      continue;
    }
    if (stats.mtimeMs > newest) newest = stats.mtimeMs;
    if (stats.isDirectory()) {
      for (const entry of readdirSync(current)) stack.push(join(current, entry));
    }
  }
  return newest;
}

const sourceMtime = Math.max(
  newestMtime(join(SDK, "src")),
  newestMtime(join(SDK, "tsconfig.json")),
  newestMtime(join(SDK, "package.json")),
  // The lockfile too: a dependency update can change the compiler itself, and
  // therefore the emitted JavaScript and declarations, without any source file
  // being touched. dist would look current while being built by another tsc.
  newestMtime(join(SDK, "package-lock.json")),
);
const builtMtime = statSync(MARKER).mtimeMs;

if (sourceMtime > builtMtime) {
  die(
    "SDK build is stale — sources are newer than dist/index.js, so the suite " +
      "would report against code that is no longer in the tree.",
  );
}
