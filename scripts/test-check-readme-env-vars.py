#!/usr/bin/env python3
"""Self-test for the inventory-based check-readme-env-vars gate.

#611 replaced the six-language lexer with a pinned inventory. The failure
mode that matters now is silent agreement with a wrong table: if the gate
does not notice a phantom README row, or does not notice a pinned needle
vanishing from source, CI stays green while the documentation lies. Each
case below builds a throwaway tree, points the gate's module-level
constants at it, and asserts on the failures returned.

Lexer edge cases (heredocs, raw strings, comment nesting, import aliases)
are deliberately absent -- they tested a design this gate no longer has.
"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import shutil
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent

spec = importlib.util.spec_from_file_location("gate", HERE / "check-readme-env-vars.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

FAILURES: list[str] = []

TABLE = (
    "| Variable | Description |\n"
    "|----------|-------------|\n"
    "| `{var}` | documented |\n"
)

ROOT_TABLE = (
    "| Variable | Read by | Only when |\n"
    "|----------|---------|----------|\n"
    "| `{var}` | {readers} | always |\n"
)


def build(root: Path, files: dict[str, str]) -> None:
    for rel, body in files.items():
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")


def check(name: str, actual, expected) -> None:
    if actual == expected:
        print(f"  ok   {name}")
    else:
        print(f"  FAIL {name}\n         expected: {expected}\n         actual:   {actual}")
        FAILURES.append(name)


def run_gate(
    root: Path,
    inventory: dict[tuple[str, str], list[tuple[str, str]]],
    sdk_readmes: dict[str, str] | None = None,
) -> list[str]:
    """Run the gate against a synthetic repo, returning its failure list."""
    old = (gate.REPO, gate.INVENTORY, gate.SDK_READMES)
    gate.REPO = root
    gate.INVENTORY = inventory
    if sdk_readmes is not None:
        gate.SDK_READMES = sdk_readmes
    try:
        return gate.collect_failures(root)
    finally:
        gate.REPO, gate.INVENTORY, gate.SDK_READMES = old


def run_main(
    root: Path,
    inventory: dict[tuple[str, str], list[tuple[str, str]]],
    sdk_readmes: dict[str, str] | None = None,
) -> int:
    """Drive the shipped entrypoint, which is what the Makefile branches on."""
    old = (gate.REPO, gate.INVENTORY, gate.SDK_READMES)
    gate.REPO = root
    gate.INVENTORY = inventory
    if sdk_readmes is not None:
        gate.SDK_READMES = sdk_readmes
    try:
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return gate.main()
    finally:
        gate.REPO, gate.INVENTORY, gate.SDK_READMES = old


def tags(failures: list[str]) -> list[str]:
    """Collapse full messages to stable identity tags for assertions."""
    out: list[str] = []
    for failure in failures:
        if "has no pinned read sites" in failure:
            # inventory: (Go, BASECAMP_TOKEN) has no pinned read sites
            pair = failure.split("(", 1)[1].split(")", 1)[0]
            sdk, var = [p.strip() for p in pair.split(",", 1)]
            out.append(f"empty:{sdk}:{var}")
        elif failure.startswith("inventory:"):
            # inventory: Go read of BASECAMP_X lost site(s): ...
            parts = failure.split()
            # ["inventory:", "Go", "read", "of", "BASECAMP_X", ...]
            out.append(f"lost:{parts[1]}:{parts[4]}")
        elif "table names" in failure:
            # path:line: table names VAR, but (SDK, VAR) is not...
            sdk = failure.split("(", 1)[1].split(",", 1)[0]
            var = failure.split("table names ", 1)[1].split(",", 1)[0]
            out.append(f"forward:{sdk}:{var}")
        elif "credits " in failure:
            # README.md:line: credits VAR to SDK, but...
            after = failure.split("credits ", 1)[1]
            var = after.split(" to ", 1)[0]
            sdk = after.split(" to ", 1)[1].split(",", 1)[0]
            out.append(f"rootcredits:{sdk}:{var}")
        elif "omits it" in failure:
            sdk = failure.split(": ", 1)[1].split(" is inventoried", 1)[0]
            var = failure.split("reading ", 1)[1].split(",", 1)[0]
            out.append(f"rootomits:{sdk}:{var}")
        elif "names no SDK" in failure:
            out.append("rootnosdk")
        else:
            out.append(failure)
    return out


def main() -> int:
    tmp = Path(tempfile.mkdtemp(prefix="readme-env-vars-"))
    try:
        py_readme = {"Python": "python/README.md"}
        go_readme = {"Go": "go/README.md"}
        multi = {
            "Go": "go/README.md",
            "Python": "python/README.md",
            "Ruby": "ruby/README.md",
        }

        # 1. Consistent: table matches inventory, needle present.
        root = tmp / "consistent"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'base_url=os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("consistent inventory and table passes",
              tags(run_gate(root, inv, py_readme)), [])

        # 2. Phantom table row: README names a variable outside the set.
        root = tmp / "phantom"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_ACCOUNT_ID"),
            "python/src/basecamp/config.py":
                'base_url=os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("phantom table row fails",
              tags(run_gate(root, inv, py_readme)),
              ["forward:Python:BASECAMP_ACCOUNT_ID"])

        # 3. Lost read site: inventory member whose needle is gone.
        root = tmp / "lost-site"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py": "base_url = DEFAULT\n",
        })
        check("missing pinned needle fails",
              tags(run_gate(root, inv, py_readme)),
              ["lost:Python:BASECAMP_BASE_URL"])

        # 4. Missing file for a pinned site.
        root = tmp / "missing-file"
        inv = {
            ("Go", "BASECAMP_TOKEN"): [
                ("go/pkg/basecamp/auth.go", 'os.Getenv("BASECAMP_TOKEN")'),
            ],
        }
        build(root, {
            "go/README.md": TABLE.format(var="BASECAMP_TOKEN"),
        })
        check("missing pinned file fails",
              tags(run_gate(root, inv, go_readme)),
              ["lost:Go:BASECAMP_TOKEN"])

        # 5. Empty site list on an inventory member.
        root = tmp / "empty-sites"
        inv = {("Go", "BASECAMP_TOKEN"): []}
        build(root, {"go/README.md": "no table\n"})
        check("inventory pair with no sites fails",
              tags(run_gate(root, inv, go_readme)),
              ["empty:Go:BASECAMP_TOKEN"])

        # 6. Root README credits an SDK outside the inventory.
        root = tmp / "root-credit"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "README.md": ROOT_TABLE.format(
                var="BASECAMP_BASE_URL", readers="Python, Ruby"),
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
            "ruby/README.md": "no table\n",
        })
        check("root table crediting a non-inventory SDK fails",
              tags(run_gate(root, inv, {"Python": "python/README.md",
                                        "Ruby": "ruby/README.md"})),
              ["rootcredits:Ruby:BASECAMP_BASE_URL"])

        # 7. Root README omits an inventoried reader.
        root = tmp / "root-omit"
        inv = {
            ("Go", "BASECAMP_BASE_URL"): [
                ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_BASE_URL")'),
            ],
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "README.md": ROOT_TABLE.format(var="BASECAMP_BASE_URL", readers="Go"),
            "go/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "go/pkg/basecamp/config.go": 'os.Getenv("BASECAMP_BASE_URL")\n',
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("root table omitting an inventoried reader fails",
              tags(run_gate(root, inv, {"Go": "go/README.md",
                                        "Python": "python/README.md"})),
              ["rootomits:Python:BASECAMP_BASE_URL"])

        # 8. Root row with no SDK in the Read-by column.
        root = tmp / "root-nosdk"
        inv = {
            ("Python", "BASECAMP_TIMEOUT"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_TIMEOUT"'),
            ],
        }
        build(root, {
            "README.md": ROOT_TABLE.format(var="BASECAMP_TIMEOUT", readers="nobody"),
            "python/README.md": TABLE.format(var="BASECAMP_TIMEOUT"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_TIMEOUT", DEFAULT)\n',
        })
        check("root row naming no SDK fails",
              sorted(tags(run_gate(root, inv, py_readme))),
              ["rootnosdk", "rootomits:Python:BASECAMP_TIMEOUT"])

        # 9. An SDK with an empty inventory slice cannot table any variable.
        root = tmp / "no-env-sdk"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_TOKEN"),
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("no-env SDK table row fails as outside the set",
              tags(run_gate(root, inv, {
                  "Python": "python/README.md",
                  "TypeScript": "typescript/README.md",
              })),
              ["forward:TypeScript:BASECAMP_TOKEN"])

        # 10. Multiple needles for one pair: losing either fails.
        root = tmp / "multi-needle"
        inv = {
            ("Ruby", "BASECAMP_BASE_URL"): [
                ("ruby/lib/basecamp/config.rb", 'ENV.fetch("BASECAMP_BASE_URL"'),
                ("ruby/lib/basecamp/config.rb", 'ENV["BASECAMP_BASE_URL"]'),
            ],
        }
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "ruby/lib/basecamp/config.rb":
                'base_url: ENV.fetch("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("losing one of several pinned needles fails",
              tags(run_gate(root, inv, {"Ruby": "ruby/README.md"})),
              ["lost:Ruby:BASECAMP_BASE_URL"])

        root = tmp / "multi-needle-ok"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "ruby/lib/basecamp/config.rb":
                'base_url: ENV.fetch("BASECAMP_BASE_URL", DEFAULT)\n'
                'x = ENV["BASECAMP_BASE_URL"]\n',
        })
        check("all pinned needles present passes",
              tags(run_gate(root, inv, {"Ruby": "ruby/README.md"})), [])

        # 11. Fenced examples and prose mentioning BASECAMP_ACCOUNT_ID are
        #     not table rows, so they do not count as naming outside the set.
        root = tmp / "example-only"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "python/README.md":
                "```python\n"
                'client = Client(access_token=os.environ["BASECAMP_TOKEN"])\n'
                'account = client.for_account(os.environ["BASECAMP_ACCOUNT_ID"])\n'
                "```\n\n"
                + TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("example mentions outside a table do not fail",
              tags(run_gate(root, inv, py_readme)), [])

        # 12. Several variables in one root cell share one Read-by column.
        root = tmp / "root-multi-var"
        inv = {
            ("Go", "BASECAMP_CACHE_DIR"): [
                ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_CACHE_DIR")'),
            ],
            ("Go", "BASECAMP_CACHE_ENABLED"): [
                ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_CACHE_ENABLED")'),
            ],
        }
        build(root, {
            "README.md":
                "| Variable | Read by | Only when |\n"
                "|----------|---------|----------|\n"
                "| `BASECAMP_CACHE_ENABLED`, `BASECAMP_CACHE_DIR` | Go | load |\n",
            "go/README.md":
                "| Variable | Description |\n"
                "|----------|-------------|\n"
                "| `BASECAMP_CACHE_DIR` | cache dir |\n"
                "| `BASECAMP_CACHE_ENABLED` | cache on |\n",
            "go/pkg/basecamp/config.go":
                'os.Getenv("BASECAMP_CACHE_DIR")\n'
                'os.Getenv("BASECAMP_CACHE_ENABLED")\n',
        })
        check("root row with multiple variables and one reader passes",
              tags(run_gate(root, inv, go_readme)), [])

        # 13. Exact match: a comment that merely mentions the variable is not
        #     the pinned needle, so the site still counts as lost.
        root = tmp / "comment-is-not-needle"
        inv = {
            ("Go", "BASECAMP_TOKEN"): [
                ("go/pkg/basecamp/auth.go", 'os.Getenv("BASECAMP_TOKEN")'),
            ],
        }
        build(root, {
            "go/README.md": TABLE.format(var="BASECAMP_TOKEN"),
            "go/pkg/basecamp/auth.go":
                '// looks up BASECAMP_TOKEN somehow\nvar x = 1\n',
        })
        check("a bare mention is not the pinned read site",
              tags(run_gate(root, inv, go_readme)),
              ["lost:Go:BASECAMP_TOKEN"])

        # 14. Shipped inventory against the real checkout must pass, and
        #     report the live pair/site counts through main().
        live = tags(gate.collect_failures(gate.REPO))
        check("live inventory matches the checkout", live, [])

        # 15. main() exit codes — what make check / CI branch on.
        root = tmp / "main-ok"
        inv = {
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
        }
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("main() exits 0 on a consistent repo",
              run_main(root, inv, py_readme), 0)

        root = tmp / "main-phantom"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_PHANTOM"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
        })
        check("main() exits 1 on a phantom table row",
              run_main(root, inv, py_readme), 1)

        root = tmp / "main-lost"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py": "pass\n",
        })
        check("main() exits 1 when a pinned site is lost",
              run_main(root, inv, py_readme), 1)

        # 16. A multi-SDK inventory with a coherent root table passes.
        root = tmp / "multi-ok"
        inv = {
            ("Go", "BASECAMP_BASE_URL"): [
                ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_BASE_URL")'),
            ],
            ("Python", "BASECAMP_BASE_URL"): [
                ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
            ],
            ("Ruby", "BASECAMP_BASE_URL"): [
                ("ruby/lib/basecamp/config.rb", 'ENV["BASECAMP_BASE_URL"]'),
            ],
        }
        build(root, {
            "README.md": ROOT_TABLE.format(
                var="BASECAMP_BASE_URL", readers="Go, Ruby, Python"),
            "go/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "go/pkg/basecamp/config.go": 'os.Getenv("BASECAMP_BASE_URL")\n',
            "python/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "python/src/basecamp/config.py":
                'os.environ.get("BASECAMP_BASE_URL", DEFAULT)\n',
            "ruby/README.md": TABLE.format(var="BASECAMP_BASE_URL"),
            "ruby/lib/basecamp/config.rb": 'ENV["BASECAMP_BASE_URL"]\n',
        })
        check("multi-SDK root table matching inventory passes",
              tags(run_gate(root, inv, multi)), [])

    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if FAILURES:
        print(f"\n{len(FAILURES)} self-test failure(s): {', '.join(FAILURES)}",
              file=sys.stderr)
        return 1
    print("\ncheck-readme-env-vars self-test passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
