#!/usr/bin/env python3
"""Every generated TypeScript schema alias must be reachable from the root entry point.

`typescript/scripts/generate-services.ts` carries a hand-maintained `TYPE_ALIASES`
table. An entry there makes a generated service emit

    export type CloudFile = components["schemas"]["CloudFile"];

and use that name in its method signatures. The root re-export block in
`typescript/src/index.ts` is *also* hand-maintained, and nothing paired the two.

That pairing matters because `typescript/package.json` publishes an exhaustive
`exports` map — only `.` and `./oauth`. There is no subpath for
`./dist/generated/...`, so a consumer cannot deep-import a generated module even
though `src/generated` ships in `files`. An alias index.ts does not re-export is
therefore a type the consumer can *receive* but cannot *name*: the method
signature says `Promise<CloudFile>` and `CloudFile` is unwritable outside the
package. Nothing else catches it — the drift gate regenerates and diffs the
generated tree, which is self-consistent either way, and `tsc` is happy because
the type resolves fine *inside* the package.

Two ways to satisfy the gate, both correct:
  * re-export the alias from `typescript/src/index.ts`, or
  * drop its `TYPE_ALIASES` entry, so no service emits the name at all.

A name declared in more than one generated module (`TimelineEvent` is emitted by
both `reports.ts` and `timeline.ts`) needs exactly one re-export — re-exporting
it twice is a duplicate-identifier error — so this gate is satisfied by name,
not by declaration site.

Exit codes:
  0 = every emitted alias is reachable
  1 = at least one is unreachable
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
INDEX = ROOT / "typescript" / "src" / "index.ts"
SERVICES = ROOT / "typescript" / "src" / "generated" / "services"

# `export type X = components["schemas"][...]` — the alias form generate-services.ts
# emits for a TYPE_ALIASES entry. Deliberately not matching `export interface`:
# request/options interfaces are structural, declared inline, and not the subject.
ALIAS_RE = re.compile(r'^export type ([A-Za-z0-9_]+) = components\["schemas"\]', re.M)

# `export { a, type B, C as D } from "..."` — index.ts re-exports exclusively in
# this form (no `export *`), so the exported name is the local one, or the
# right-hand side of an `as` rename.
BLOCK_RE = re.compile(r'export\s*\{([^}]*)\}\s*from\s*"[^"]+"', re.S)


def strip_comments(source: str) -> str:
    """Blank out `//` and `/* */` comments, leaving string literals intact.

    Without this the scan reads commented-out code as live. A maintainer who
    comments out a re-export — `// export { type Notification } from "..."` —
    leaves the type genuinely unreachable while the gate still passes, which is
    the exact regression it exists to catch. `tsc` says nothing either, because
    the type still resolves inside the package.

    A full TypeScript parse would be the thorough answer and is not worth a
    dependency here: the only thing that must be right is which spans are code,
    and that needs string awareness (a `//` inside a module specifier is not a
    comment) but no grammar. Comments are replaced with spaces rather than
    deleted so nothing on either side is accidentally joined into one token.
    """
    out = []
    i, n = 0, len(source)
    quote = None  # active string delimiter, or None
    while i < n:
        ch = source[i]
        if quote:
            out.append(ch)
            if ch == "\\" and i + 1 < n:      # escape: copy the pair verbatim
                out.append(source[i + 1])
                i += 2
                continue
            if ch == quote:
                quote = None
            i += 1
            continue
        if ch in "\"'`":
            quote = ch
            out.append(ch)
            i += 1
            continue
        if ch == "/" and i + 1 < n and source[i + 1] == "/":
            while i < n and source[i] != "\n":
                out.append(" ")
                i += 1
            continue
        if ch == "/" and i + 1 < n and source[i + 1] == "*":
            end = source.find("*/", i + 2)
            end = n if end == -1 else end + 2
            out.append("".join(" " if c != "\n" else "\n" for c in source[i:end]))
            i = end
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def reexported_names(source: str) -> set[str]:
    names = set()
    for block in BLOCK_RE.findall(strip_comments(source)):
        for spec in block.split(","):
            spec = spec.strip()
            if not spec:
                continue
            spec = re.sub(r"^type\s+", "", spec)
            if " as " in spec:
                spec = spec.split(" as ")[-1]
            spec = spec.strip()
            if spec:
                names.add(spec)
    return names


def main() -> int:
    if not INDEX.is_file():
        print(f"ERROR: {INDEX} not found", file=sys.stderr)
        return 2
    if not SERVICES.is_dir():
        print(f"ERROR: {SERVICES} not found", file=sys.stderr)
        return 2

    exported = reexported_names(INDEX.read_text())

    # name -> the generated modules that declare it
    emitted: dict[str, list[str]] = {}
    for path in sorted(SERVICES.glob("*.ts")):
        if path.name == "index.ts":
            continue
        # Same comment strip on this side: a commented-out alias would otherwise
        # be scanned as emitted and fail the gate for a name nothing declares.
        for name in ALIAS_RE.findall(strip_comments(path.read_text())):
            emitted.setdefault(name, []).append(path.name)

    missing = {n: mods for n, mods in emitted.items() if n not in exported}

    if missing:
        print("ERROR: generated schema aliases are unreachable from the package entry point.")
        print("")
        for name in sorted(missing):
            mods = ", ".join(missing[name])
            print(f"  {name:<32} declared in {mods}")
        print("")
        print("Re-export each from typescript/src/index.ts (one re-export per name, even")
        print("when several modules declare it), or drop its TYPE_ALIASES entry in")
        print("typescript/scripts/generate-services.ts so nothing emits the name.")
        print("The package `exports` map permits no deep import, so an unreachable alias")
        print("is a return type consumers cannot write down.")
        return 1

    print(
        f"==> TypeScript entity exports OK: {len(emitted)} generated schema aliases, "
        f"all re-exported from src/index.ts."
    )
    return 0


# --------------------------------------------------------------------------
# Self-test. The live run only proves the gate can say yes. These drive the
# ways it could wrongly say yes, which is the failure mode that matters: a
# gate that passes on an unreachable type is worse than no gate, because it
# reports the guarantee it is not providing.
#
# The first case is not hypothetical — the gate shipped with it, and review
# caught it. Its raw-source scan read `// export { type Notification } from
# "..."` as a live re-export, so a maintainer could comment out an export and
# the gate stayed green while the type became unreachable.
# --------------------------------------------------------------------------

SELF_TEST_CASES = [
    (
        "live re-export is reachable",
        'export { Foo, type Bar } from "./generated/services/x.js";',
        {"Foo", "Bar"},
    ),
    (
        "single-line commented re-export is NOT reachable",
        '// export { type Bar } from "./generated/services/x.js";\n'
        'export { Foo } from "./generated/services/x.js";',
        {"Foo"},
    ),
    (
        "block-commented re-export is NOT reachable",
        '/* export { type Bar } from "./generated/services/x.js"; */\n'
        'export { Foo } from "./generated/services/x.js";',
        {"Foo"},
    ),
    (
        "multi-line commented re-export is NOT reachable",
        "// export {\n//   type Bar,\n// } from \"./generated/services/x.js\";\n"
        'export { Foo } from "./generated/services/x.js";',
        {"Foo"},
    ),
    (
        "`//` inside a module specifier is not a comment",
        'export { Foo } from "https://example.invalid/x.js";\n'
        'export { type Bar } from "./generated/services/x.js";',
        {"Foo", "Bar"},
    ),
    (
        "a rename exports the right-hand name",
        'export { Internal as Public } from "./generated/services/x.js";',
        {"Public"},
    ),
    (
        "a comment mentioning an export does not create one",
        "// Bar is deliberately internal; see the note in x.ts.\n"
        'export { Foo } from "./generated/services/x.js";',
        {"Foo"},
    ),
]

# `strip_comments` must not disturb the alias scan either: a commented-out
# alias declaration is not emitted, so demanding a re-export for it would fail
# the gate over a name nothing declares.
SELF_TEST_ALIAS_CASES = [
    (
        "live alias is emitted",
        'export type Bar = components["schemas"]["Bar"];',
        ["Bar"],
    ),
    (
        "commented alias is NOT emitted",
        '// export type Bar = components["schemas"]["Bar"];',
        [],
    ),
]


def self_test() -> int:
    failures = 0
    for name, source, want in SELF_TEST_CASES:
        got = reexported_names(source)
        ok = got == want
        failures += 0 if ok else 1
        print(f"  {'ok  ' if ok else 'FAIL'} {name}")
        if not ok:
            print(f"       want {sorted(want)}, got {sorted(got)}")
    for name, source, want in SELF_TEST_ALIAS_CASES:
        got = ALIAS_RE.findall(strip_comments(source))
        ok = got == want
        failures += 0 if ok else 1
        print(f"  {'ok  ' if ok else 'FAIL'} {name}")
        if not ok:
            print(f"       want {want}, got {got}")
    total = len(SELF_TEST_CASES) + len(SELF_TEST_ALIAS_CASES)
    if failures:
        print(f"entity-export gate self-test: {failures}/{total} FAILED")
        return 1
    print(f"entity-export gate self-test: all {total} cases passed.")
    return 0


if __name__ == "__main__":
    if "--self-test" in sys.argv[1:]:
        sys.exit(self_test())
    sys.exit(main())
