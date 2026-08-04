#!/usr/bin/env python3
"""check-retry-metadata-parity — assert per-operation retry metadata matches the
source of truth across every SDK that emits it, and record which fields each SDK
actually consumes at runtime.

Source of truth: behavior-model.json `operations[opId].retry`:
    { "max": int, "base_delay_ms": int, "backoff": str, "retry_on": [int, ...] }

This guard is DISTINCT from the regenerate-and-diff freshness gates. Those prove
each emitter reproduces its committed bytes (no stale/hand-edit). This proves the
emitted VALUES equal behavior-model.json — catching a generator that reproducibly
emits the wrong retry semantics.

Two acceptance criteria:

  (1) Metadata parity (static). Every file that emits the per-op retry block must
      carry, for all 225 operations, values equal to behavior-model.json:
        * Full tuple (max, base_delay_ms, backoff, retry_on):
            - python/src/basecamp/generated/metadata.json   (snake_case)
            - ruby/lib/basecamp/generated/metadata.json     (camelCase)
            - typescript/src/generated/metadata.ts          (camelCase)
            - kotlin/.../generated/Metadata.kt              (positional)
            - swift/Sources/Basecamp/Generated/Metadata.swift (labelled)
        * max + retry_on:
            - go/pkg/generated/client.gen.go operationRetryMax + operationRetryOn maps

  (2) Runtime consumption (TOKEN-SMOKE classification, NOT behavioral proof).
      Which emitted fields are read at runtime, per SDK, checked by the presence
      of the actual usage-form token in each consuming source (and the absence of
      a token for inert fields). This catches gross removal of a consumption site
      but does NOT prove the field governs a retry decision — a token surviving
      in a comment would pass. Behavioral proof lives in each SDK's retry tests.
      Fields emitted but never read are guarded for PARITY only (criterion 1) and
      classified emitted-but-runtime-inert — NOT claimed as runtime parity:
        * TypeScript / Swift / Kotlin: consume the full tuple.
        * Go / Python / Ruby:          consume `max` (per-op ceiling) AND
                                       `retry_on` (the status gate); base_delay
                                       and backoff are emitted-but-inert. Ruby
                                       applies both on its governed GET path
                                       only — mutations never retry there.

Exit non-zero on any parity mismatch.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def fail(msg: str) -> None:
    print(f"  ✗ {msg}")


# --- source of truth ---------------------------------------------------------


def load_model() -> dict[str, tuple]:
    d = json.loads((ROOT / "behavior-model.json").read_text())
    out: dict[str, tuple] = {}
    for op, meta in d["operations"].items():
        r = meta.get("retry")
        if not r:
            continue
        out[op] = (r["max"], r["base_delay_ms"], r["backoff"], tuple(r["retry_on"]))
    return out


# --- emitter extractors: each returns {opId: (max, base_delay_ms, backoff, retry_on)} ---


def _from_json_ops(path: Path, ops_key: str | None, camel: bool) -> dict[str, tuple]:
    d = json.loads(path.read_text())
    ops = d[ops_key] if ops_key else d
    mx, bd, ro = ("maxAttempts", "baseDelayMs", "retryOn") if camel else ("max", "base_delay_ms", "retry_on")
    out: dict[str, tuple] = {}
    for op, meta in ops.items():
        if not isinstance(meta, dict) or "retry" not in meta:
            continue
        r = meta["retry"]
        out[op] = (r[mx], r[bd], r["backoff"], tuple(r[ro]))
    return out


def from_python() -> dict[str, tuple]:
    return _from_json_ops(ROOT / "python/src/basecamp/generated/metadata.json", None, camel=False)


def from_ruby() -> dict[str, tuple]:
    return _from_json_ops(ROOT / "ruby/lib/basecamp/generated/metadata.json", "operations", camel=True)


def from_typescript() -> dict[str, tuple]:
    text = (ROOT / "typescript/src/generated/metadata.ts").read_text()
    # The metadata value is a JSON object literal preceded by TS interface
    # declarations. Anchor on the `metadata` const, then extract the balanced
    # braces of its object and json.loads it.
    decl = re.search(r"const metadata\b[^=]*=\s*", text)
    if not decl:
        raise ValueError("metadata const not found in metadata.ts")
    start = text.index("{", decl.end())
    depth = 0
    for i in range(start, len(text)):
        if text[i] == "{":
            depth += 1
        elif text[i] == "}":
            depth -= 1
            if depth == 0:
                end = i + 1
                break
    else:
        raise ValueError("unbalanced braces in metadata.ts")
    d = json.loads(text[start:end])
    ops = d["operations"]
    out: dict[str, tuple] = {}
    for op, meta in ops.items():
        if "retry" not in meta:
            continue
        r = meta["retry"]
        out[op] = (r["maxAttempts"], r["baseDelayMs"], r["backoff"], tuple(r["retryOn"]))
    return out


def from_kotlin() -> dict[str, tuple]:
    text = (ROOT / "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/Metadata.kt").read_text()
    # "OpId" to OperationConfig(<bool>, RetryConfig(<max>, <delay>L, "<backoff>", setOf(<ints>))),
    pat = re.compile(
        r'"(?P<op>\w+)"\s+to\s+OperationConfig\([^,]+,\s*RetryConfig\('
        r"(?P<max>\d+),\s*(?P<delay>\d+)L,\s*\"(?P<backoff>\w+)\",\s*setOf\((?P<ro>[^)]*)\)\)"
    )
    out: dict[str, tuple] = {}
    for m in pat.finditer(text):
        ro = tuple(int(x) for x in re.findall(r"\d+", m.group("ro")))
        out[m.group("op")] = (int(m.group("max")), int(m.group("delay")), m.group("backoff"), ro)
    return out


def from_swift() -> dict[str, tuple]:
    text = (ROOT / "swift/Sources/Basecamp/Generated/Metadata.swift").read_text()
    # "OpId": RetryConfig(maxAttempts: N, baseDelayMs: N, backoff: .exponential, retryOn: [429, 503]),
    pat = re.compile(
        r'"(?P<op>\w+)"\s*:\s*RetryConfig\(maxAttempts:\s*(?P<max>\d+),\s*'
        r"baseDelayMs:\s*(?P<delay>\d+),\s*backoff:\s*\.(?P<backoff>\w+),\s*retryOn:\s*\[(?P<ro>[^\]]*)\]\)"
    )
    out: dict[str, tuple] = {}
    for m in pat.finditer(text):
        ro = tuple(int(x) for x in re.findall(r"\d+", m.group("ro")))
        out[m.group("op")] = (int(m.group("max")), int(m.group("delay")), m.group("backoff"), ro)
    return out


def from_go_max() -> dict[str, int]:
    # Go emits the per-op retry ceiling as a separate `operationRetryMax` map
    # (kept off the exported OperationMetadata struct to avoid a source break).
    text = (ROOT / "go/pkg/generated/client.gen.go").read_text()
    block = re.search(r"var operationRetryMax = map\[string\]int\{(.*?)\n\}", text, re.DOTALL)
    if not block:
        raise ValueError("operationRetryMax map not found in client.gen.go")
    pat = re.compile(r'"(?P<op>\w+)":\s*(?P<max>\d+),')
    return {m.group("op"): int(m.group("max")) for m in pat.finditer(block.group(1))}


def from_go_retry_on() -> dict[str, tuple]:
    # Go emits the declared retryable status set as `operationRetryOn`, a
    # sibling of operationRetryMax and likewise kept off the exported
    # OperationMetadata struct.
    text = (ROOT / "go/pkg/generated/client.gen.go").read_text()
    block = re.search(r"var operationRetryOn = map\[string\]\[\]int\{(.*?)\n\}", text, re.DOTALL)
    if not block:
        raise ValueError("operationRetryOn map not found in client.gen.go")
    pat = re.compile(r'"(?P<op>\w+)":\s*\{(?P<ro>[0-9,\s]*)\},')
    return {
        m.group("op"): tuple(int(x) for x in re.findall(r"\d+", m.group("ro")))
        for m in pat.finditer(block.group(1))
    }


# --- checks ------------------------------------------------------------------


def check_full_tuple(name: str, emitted: dict[str, tuple], model: dict[str, tuple]) -> int:
    errors = 0
    missing = sorted(set(model) - set(emitted))
    extra = sorted(set(emitted) - set(model))
    if missing:
        fail(f"{name}: {len(missing)} operation(s) missing a retry block, e.g. {missing[:3]}")
        errors += len(missing)
    if extra:
        fail(f"{name}: {len(extra)} operation(s) not in behavior-model.json, e.g. {extra[:3]}")
        errors += len(extra)
    for op in sorted(set(model) & set(emitted)):
        if emitted[op] != model[op]:
            fail(f"{name}: {op} retry {emitted[op]} != model {model[op]}")
            errors += 1
    if errors == 0:
        print(f"  ✓ {name}: {len(emitted)} operations match behavior-model.json (full tuple)")
    return errors


def check_max_only(name: str, emitted: dict[str, int], model: dict[str, tuple]) -> int:
    errors = 0
    missing = sorted(set(model) - set(emitted))
    extra = sorted(set(emitted) - set(model))
    if missing:
        fail(f"{name}: {len(missing)} operation(s) missing RetryMax, e.g. {missing[:3]}")
        errors += len(missing)
    if extra:
        # A stale RetryMax entry for an op no longer in behavior-model.json must
        # fail, so the emitter cannot retain obsolete retry metadata unnoticed.
        fail(f"{name}: {len(extra)} operation(s) not in behavior-model.json, e.g. {extra[:3]}")
        errors += len(extra)
    for op in sorted(set(model) & set(emitted)):
        if emitted[op] != model[op][0]:
            fail(f"{name}: {op} RetryMax {emitted[op]} != model max {model[op][0]}")
            errors += 1
    if errors == 0:
        print(f"  ✓ {name}: {len(emitted)} operations match behavior-model.json (max only)")
    return errors


# (sdk, level, consuming file, required tokens, forbidden tokens, note). Tokens
# are the ACTUAL usage forms (e.g. `retryOn.includes`, not bare `retryOn`) so
# deleting the behavior — not just renaming a field — trips the check. This is a
# TOKEN-SMOKE classification, NOT a behavioral proof: it catches gross removal of
# a consumption site but cannot prove the field governs a retry decision (a token
# surviving in a comment would pass). Behavioral proof lives in each SDK's own
# retry test suite (go generated_*retry_test.go, python tests/test_http.py,
# swift RetryTests.swift, typescript tests/client.test.ts +
# tests/middleware-lifecycle.test.ts, the conformance runners).
RUNTIME_CONSUMPTION = [
    ("TypeScript", "full tuple", "typescript/src/services/base.ts",
     ["retryOn.includes", "maxAttempts", "baseDelayMs ??"], [],
     "base.ts: retryOn.includes(status), attempt vs maxAttempts, baseDelayMs backoff"),
    ("TypeScript", "full tuple", "typescript/src/client.ts",
     ["getRetryConfigForRequest", "retryConfig.maxAttempts", "executeWithRetry"], [],
     "client.ts createRetryingFetch resolves the per-operation tuple and hands it to executeWithRetry"),
    ("TypeScript", "full tuple", "typescript/src/retry.ts",
     ["config.retryOn.includes", "attempt >= config.maxAttempts", "calculateBackoffDelay", "config.baseDelayMs"], [],
     "retry.ts executeWithRetry: retryOn.includes(status), attempt >= maxAttempts, backoff from baseDelayMs"),
    ("Swift", "full tuple", "swift/Sources/Basecamp/HTTP/HTTPClient.swift",
     ["retryOn.contains", "< maxAttempts", "baseDelayMs"], [],
     "HTTPClient.swift: retryOn.contains(status), attempt < maxAttempts, baseDelayMs"),
    ("Kotlin", "full tuple", "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/http/BasecampHttpClient.kt",
     ["opRetry.retryOn", "opRetry?.maxRetries", "opRetry?.baseDelayMs", "minOf(", "coerceAtLeast(1)"], [],
     "BasecampHttpClient.kt: status in opRetry.retryOn, min(caller cap, opRetry.maxRetries), baseDelayMs"),
    ("Go", "max + retry_on", "go/templates/client.tmpl",
     ["opMax < maxAttempts", "operationRetryMax[operationId]",
      "operationRetryOn[operationId]", "isRetryableStatus(resp.StatusCode, operationId)"],
     ["RetryBaseDelayMs"],
     "doWithRetry applies min(client cap, operationRetryMax) and gates status retry on operationRetryOn; base_delay/backoff emitted-but-inert"),
    ("Python", "max + retry_on", "python/src/basecamp/_http.py",
     ['.get("retry", {}).get("max")', 'retry.get("retry_on")', "_is_retryable_error"], [],
     "_request_with_retry applies min(client cap, retry.max) and gates status retry on the declared retry_on; other fields emitted-but-inert"),
    ("Ruby", "max + retry_on", "ruby/lib/basecamp/http.rb",
     ['fetch("maxAttempts")', 'fetch("retryOn")', "declared.include?", "operation_retry"], [],
     "http.rb retries governed GETs with min(caller cap, maxAttempts) and gates status retry on the declared retryOn; base_delay/backoff emitted-but-inert"),
]


def check_retry_on_only(name: str, emitted: dict[str, tuple], model: dict[str, tuple]) -> int:
    errors = 0
    missing = sorted(set(model) - set(emitted))
    extra = sorted(set(emitted) - set(model))
    if missing:
        fail(f"{name}: {len(missing)} operation(s) missing a retryOn set, e.g. {missing[:3]}")
        errors += len(missing)
    if extra:
        fail(f"{name}: {len(extra)} operation(s) not in behavior-model.json, e.g. {extra[:3]}")
        errors += len(extra)
    for op in sorted(set(model) & set(emitted)):
        if emitted[op] != model[op][3]:
            fail(f"{name}: {op} retryOn {emitted[op]} != model {model[op][3]}")
            errors += 1
    if errors == 0:
        print(f"  ✓ {name}: {len(emitted)} operations match behavior-model.json (retry_on)")
    return errors


def check_runtime_consumption() -> int:
    errors = 0
    for sdk, level, rel, required, forbidden, note in RUNTIME_CONSUMPTION:
        text = (ROOT / rel).read_text()
        missing = [t for t in required if t not in text]
        present_forbidden = [t for t in forbidden if t in text]
        if missing:
            fail(f"{sdk}: {rel} is missing expected usage token(s) {missing} — a consumption site was removed")
            errors += len(missing)
        if present_forbidden:
            fail(f"{sdk}: consuming file {rel} references {present_forbidden}, contradicting 'consumes {level}'")
            errors += len(present_forbidden)
        if not missing and not present_forbidden:
            print(f"  ✓ {sdk:<10} consumes {level:<10} — {note}")
    return errors


def main() -> int:
    model = load_model()
    print(f"Source of truth: behavior-model.json ({len(model)} operations with a retry block)\n")

    print("Criterion 1 — static metadata parity:")
    errors = 0
    errors += check_full_tuple("Python  metadata.json", from_python(), model)
    errors += check_full_tuple("Ruby    metadata.json", from_ruby(), model)
    errors += check_full_tuple("TypeScript metadata.ts", from_typescript(), model)
    errors += check_full_tuple("Kotlin  Metadata.kt", from_kotlin(), model)
    errors += check_full_tuple("Swift   Metadata.swift", from_swift(), model)
    errors += check_max_only("Go      operationRetryMax", from_go_max(), model)
    errors += check_retry_on_only("Go      operationRetryOn", from_go_retry_on(), model)

    print("\nCriterion 2 — runtime consumption (token-smoke classification, not behavioral proof):")
    errors += check_runtime_consumption()

    if errors:
        print(f"\nFAIL: {errors} retry-metadata parity error(s).")
        return 1
    print("\nOK: retry metadata is consistent across all SDKs and matches behavior-model.json.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
