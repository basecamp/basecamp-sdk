"""Fixture for the stale-module sweep in the Python service generator (#757).

Nothing else executes the deletion branch. `check-python-service-drift.sh`
regenerates into a fresh temp directory, so the sweep never has a corpse to
find there, and a live `make generate` only ever confirms the current roster —
the branch that unlinks a file runs in no other test. These cases build a
directory holding every kind of file the sweep must tell apart, run the real
thing over it, and assert on what is left on disk.

What the sweep deletes is `the previous barrel's modules - this run's output`,
so the cases are really about that record: it is read before `__init__.py` is
overwritten, it is the only thing that nominates a file for deletion, and no
file's *contents* are consulted at any point. Two cases exist to pin exactly
that. A dropped module carrying an obsolete preamble is still swept — the defect
in the content predicate this replaced, where a commit that renamed a service and
touched the preamble in the same breath stranded the module beyond the reach of
rerunning the generator. And a module carrying a perfectly current preamble that
the barrel never named is *not* swept, which is the residual `remove_stale_files`
documents rather than a case that got away.

The wrong-directory case is answered by the mechanism now instead of by a second
clause in a predicate: `generated/__init__.py` is the empty package marker, so
pointing `--output` one directory too high yields an empty drop list and
`types.py` is never a candidate. That is asserted against the real tree, not a
mock of it.

`main` gets its own case, because the record can be read correctly and the wiring
still be wrong: it proves the sweep is reached after the emit loop and that the
barrel is read before it is rewritten.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
import generate_services  # noqa: E402
from generate_services import (  # noqa: E402
    SERVICE_MODULE_BASE_IMPORT,
    SERVICE_MODULE_MARKER,
    SERVICE_PACKAGE,
    previously_emitted_modules,
    remove_stale_files,
)

REPO_ROOT = Path(__file__).parent.parent.parent
OPENAPI = REPO_ROOT / "openapi.json"
GENERATED_DIR = REPO_ROOT / "python" / "src" / "basecamp" / "generated"

# A module named by no mapping, so `main` cannot emit it: the corpse a renamed
# or dropped service leaves behind.
STALE_MODULE = "fanfares.py"

# The same, but its preamble is one an older revision of this generator wrote.
# Reachable in a single commit — drop a service and reword the `@generated` line
# or move the base module, and the file left on disk carries yesterday's text
# while every constant in the generator carries today's.
STALE_OLD_PREAMBLE_MODULE = "carrier_pigeons.py"

# A module whose preamble is exactly current but which no barrel ever named.
# Kept, and that is the documented residual: the drop list can only be too short.
UNRECORDED_MODULE = "ghost.py"

# A module `main` does emit, so the "written by this run" guard has something to
# protect.
EMITTED_MODULE = "projects.py"


def service_module(class_name: str) -> str:
    """A module shaped like one this generator emits today."""
    return "\n".join(
        [
            SERVICE_MODULE_MARKER,
            "",
            "from __future__ import annotations",
            "",
            SERVICE_MODULE_BASE_IMPORT,
            "",
            "",
            f"class {class_name}Service(BaseService):",
            "    pass",
            "",
        ]
    )


def service_module_with_old_preamble(class_name: str) -> str:
    """A module shaped like one an *earlier* revision of this generator emitted.

    Neither line the retired content predicate matched on appears here, which the
    case below asserts rather than assumes — a fixture that drifted back into the
    current spelling would pass for the wrong reason.
    """
    return "\n".join(
        [
            "# @generated - do not edit",
            "",
            "from basecamp.generated.service_base import BaseService",
            "",
            "",
            f"class {class_name}Service(BaseService):",
            "    pass",
            "",
        ]
    )


def barrel(*modules: str) -> str:
    """The barrel shape `generate_init_file` emits, naming the given modules."""
    lines = [SERVICE_MODULE_MARKER, ""]
    for module in modules:
        cls = "".join(part.capitalize() for part in module.split("_"))
        lines.append(f"from {SERVICE_PACKAGE}.{module} import {cls}Service, Async{cls}Service")
    lines.append("")
    return "\n".join(lines)


def populate(output_dir: Path) -> None:
    """Write one file of every kind the sweep has to distinguish."""
    # 1. A generated service module nothing emits any more.
    (output_dir / STALE_MODULE).write_text(service_module("Fanfares"), encoding="utf-8")

    # 2. The same, wearing a preamble from before some earlier refactor.
    (output_dir / STALE_OLD_PREAMBLE_MODULE).write_text(
        service_module_with_old_preamble("CarrierPigeons"), encoding="utf-8"
    )

    # 3. A generated service module that is still emitted.
    (output_dir / EMITTED_MODULE).write_text(service_module("Projects"), encoding="utf-8")

    # 4. The hand-written base modules, which live under `generated/` by
    #    exception (AGENTS.md Hard Rule 1).
    for base in ("_base.py", "_async_base.py"):
        (output_dir / base).write_text(
            "from __future__ import annotations\n\n\nclass BaseService:\n    pass\n",
            encoding="utf-8",
        )

    # 5. A hand-written module with no marker at all.
    (output_dir / "hand_written.py").write_text(
        '"""Not generated by anything."""\n\nVALUE = 1\n',
        encoding="utf-8",
    )

    # 6. A module indistinguishable by content from a generated one, absent from
    #    the barrel.
    (output_dir / UNRECORDED_MODULE).write_text(service_module("Ghost"), encoding="utf-8")

    # 7. The barrel the previous run left behind: the record the sweep reads.
    #    It names the two stale modules and the emitted one, and — like the real
    #    thing — neither base module nor either of the hand-written files.
    (output_dir / "__init__.py").write_text(
        barrel(
            STALE_MODULE.removesuffix(".py"),
            STALE_OLD_PREAMBLE_MODULE.removesuffix(".py"),
            EMITTED_MODULE.removesuffix(".py"),
        ),
        encoding="utf-8",
    )

    # 8. The types module, one directory up in the tracked tree but right here if
    #    `--output` names the parent.
    (output_dir / "types.py").write_text(
        f"{SERVICE_MODULE_MARKER}\n\nfrom __future__ import annotations\n\nfrom typing import TypedDict\n",
        encoding="utf-8",
    )


def names_on_disk(output_dir: Path) -> set[str]:
    return {p.name for p in output_dir.glob("*.py")}


@pytest.fixture()
def swept(tmp_path: Path) -> Path:
    """A populated directory, after a real sweep that emitted two of its files."""
    populate(tmp_path)
    # Same order as `main`: read the record, then act on it.
    previous = previously_emitted_modules(tmp_path)
    remove_stale_files(tmp_path, previous, [EMITTED_MODULE, "__init__.py"])
    return tmp_path


def test_the_drop_list_is_populated(swept: Path):
    # Non-vacuity floor for every "kept" case below: an empty drop list deletes
    # nothing, so all of them would hold against a sweep that had read no record
    # at all. `swept` has already overwritten nothing, so the barrel still reads.
    assert previously_emitted_modules(swept) >= {"fanfares", "carrier_pigeons", "projects"}


def test_removes_a_module_the_barrel_named_and_this_run_did_not_emit(swept: Path):
    assert STALE_MODULE not in names_on_disk(swept)


def test_removes_a_stale_module_wearing_an_obsolete_preamble(swept: Path):
    # The defect the content predicate had. Drop a service in the same commit
    # that rewords the preamble and the constants necessarily match the new
    # output while the module left on disk matches the old — unrecognisable,
    # unswept, and not fixable by rerunning the generator.
    source = service_module_with_old_preamble("CarrierPigeons")
    assert SERVICE_MODULE_MARKER not in source
    assert SERVICE_MODULE_BASE_IMPORT not in source

    assert STALE_OLD_PREAMBLE_MODULE not in names_on_disk(swept)


def test_keeps_a_module_this_run_emitted(swept: Path):
    # The barrel names it too, so only membership in the emitted set can save it.
    assert EMITTED_MODULE in names_on_disk(swept)


def test_keeps_the_hand_written_base_modules(swept: Path):
    assert {"_base.py", "_async_base.py"} <= names_on_disk(swept)


def test_keeps_an_unmarked_hand_written_module(swept: Path):
    assert "hand_written.py" in names_on_disk(swept)


def test_keeps_the_barrel(swept: Path):
    assert "__init__.py" in names_on_disk(swept)


def test_keeps_a_generated_looking_module_the_barrel_never_named(swept: Path):
    # Byte-identical in shape to the stale module that was just deleted, and kept,
    # because content is not consulted in either direction. This is the residual
    # `remove_stale_files` documents: a corpse from before the sweep existed, or
    # from a run whose barrel was lost, still wants one manual `rm`.
    assert UNRECORDED_MODULE in names_on_disk(swept)
    assert (swept / UNRECORDED_MODULE).read_text(encoding="utf-8") == service_module("Ghost")


def test_the_real_barrel_names_exactly_the_real_roster():
    # The record the sweep depends on, read from the tracked tree rather than a
    # fixture: every service module on disk is named by the barrel and vice
    # versa, so a drop list derived from it can neither miss a live module nor
    # nominate one.
    services_dir = GENERATED_DIR / "services"
    on_disk = {p.stem for p in services_dir.glob("*.py") if not p.name.startswith("_")}
    assert on_disk
    assert previously_emitted_modules(services_dir) == on_disk


def test_a_directory_one_level_too_high_yields_no_drop_list():
    # `--output .../generated` instead of `.../generated/services` is a plausible
    # slip: the flag exists, `check-python-service-drift.sh` passes it, and the
    # default is spelled out in full beside it. The package marker there imports
    # nothing, so there is no drop list and `types.py` is never a candidate — the
    # hazard is answered by the mechanism, with no clause to maintain.
    assert (GENERATED_DIR / "types.py").is_file()
    assert previously_emitted_modules(GENERATED_DIR) == set()


def test_an_unparseable_barrel_sweeps_nothing(tmp_path: Path):
    populate(tmp_path)
    (tmp_path / "__init__.py").write_text("from basecamp.generated.services. import (\n", encoding="utf-8")

    previous = previously_emitted_modules(tmp_path)
    remove_stale_files(tmp_path, previous, [EMITTED_MODULE, "__init__.py"])

    assert previous == set()
    assert STALE_MODULE in names_on_disk(tmp_path)


def test_a_missing_barrel_sweeps_nothing(tmp_path: Path):
    populate(tmp_path)
    (tmp_path / "__init__.py").unlink()

    previous = previously_emitted_modules(tmp_path)
    remove_stale_files(tmp_path, previous, [EMITTED_MODULE])

    assert previous == set()
    assert STALE_MODULE in names_on_disk(tmp_path)


def test_main_sweeps_stale_modules_after_emitting(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    populate(tmp_path)

    monkeypatch.setattr(
        sys,
        "argv",
        ["generate_services.py", "--openapi", str(OPENAPI), "--output", str(tmp_path)],
    )
    generate_services.main()

    remaining = names_on_disk(tmp_path)
    # Both corpses are gone, which also proves the barrel was read before it was
    # rewritten: `main` overwrites `__init__.py` with one that names neither.
    assert STALE_MODULE not in remaining
    assert STALE_OLD_PREAMBLE_MODULE not in remaining
    # The emit loop's own output is intact, and so is everything the sweep is
    # not entitled to touch.
    assert EMITTED_MODULE in remaining
    assert {"_base.py", "_async_base.py", "hand_written.py", "types.py", "__init__.py"} <= remaining
    assert UNRECORDED_MODULE in remaining
    # Non-vacuity floor. Every name above was already on disk before `main` ran,
    # so a `main` that emitted nothing would satisfy them all: this run must have
    # produced the real roster, and rewritten the placeholder standing in for one
    # of its modules, for "emitted this run" to mean anything here. The floor sits
    # well under the real count so it is not a second constant to maintain.
    assert len(remaining) > 40
    assert (tmp_path / EMITTED_MODULE).read_text(encoding="utf-8") != service_module("Projects")
