"""OpenAPI schema walker for the wire-replay runner.

Pure-Python port of conformance/runner/typescript/schema-validator.ts. The
TS canary's Ajv-driven validation pipeline is replaced here by two focused
walkers:

  * `missing_required` — emits ``prefix/name`` paths for declared @required
    fields that are absent from the wire body.
  * `extras_seen`     — emits paths for fields present on the wire that the
    schema does not declare. Mirrors collectExtras semantics: object keys
    use ``.``, array elements use ``[]``.

Conventions kept exactly in sync with the TS implementation so cross-language
extras parity (PR 4 verification §5e) doesn't false-diff:
  * ``additionalProperties: false`` is ignored — forward-compat fields must
    not break the canary.
  * ``$ref`` chains resolve until non-ref or cycle. Both ``#/components/...``
    and ``openapi.json#/components/...`` ref forms are accepted.
  * Recursion depth bound 12.
  * For arrays: only recurse when ``schema.type == "array"`` and ``items``
    is set.
  * For non-object schemas (``type`` set and != ``"object"``), do not
    recurse — guards against descending into primitives.

No new dependencies — hand-rolled keeps semantics byte-identical to the TS
walker. ~80 LoC target.
"""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

_REF_RE = re.compile(r"^(?:openapi\.json)?#/components/schemas/(.+)$")
_MAX_DEPTH = 12


class SchemaWalker:
    def __init__(self, openapi_path: Path) -> None:
        self._doc: dict = json.loads(openapi_path.read_text())

    def find_response_schema(self, operation_id: str) -> dict | None:
        for path_item in self._doc.get("paths", {}).values():
            for op in path_item.values():
                if not isinstance(op, dict) or op.get("operationId") != operation_id:
                    continue
                responses = op.get("responses") or {}
                # Match TS preference order: 200 first, then any 2xx, then default.
                for code in ("200", "201", "202", "203", "204"):
                    schema = _schema_for(responses.get(code))
                    if schema is not None:
                        return schema
                for code, response in responses.items():
                    if re.fullmatch(r"2\d\d", str(code)):
                        schema = _schema_for(response)
                        if schema is not None:
                            return schema
                schema = _schema_for(responses.get("default"))
                if schema is not None:
                    return schema
        return None

    def missing_required(self, body: Any, schema: dict) -> list[str]:
        out: list[str] = []
        self._walk_required("", body, schema, 0, out)
        return out

    def extras_seen(self, body: Any, schema: dict) -> list[str]:
        out: list[str] = []
        seen: set[str] = set()
        self._walk_extras("", body, schema, 0, out, seen)
        return out

    def _resolve_ref(self, schema: Any) -> Any:
        seen: set[str] = set()
        current = schema
        while isinstance(current, dict):
            ref = current.get("$ref")
            if not isinstance(ref, str) or ref in seen:
                return current
            seen.add(ref)
            m = _REF_RE.match(ref)
            if not m:
                return current
            nxt = self._doc.get("components", {}).get("schemas", {}).get(m.group(1))
            if nxt is None:
                return current
            current = nxt
        return current

    def _walk_required(self, prefix: str, body: Any, schema: Any, depth: int, out: list[str]) -> None:
        if depth > _MAX_DEPTH or body is None:
            return
        s = self._resolve_ref(schema)
        if not isinstance(s, dict):
            return
        if s.get("type") == "array" and isinstance(body, list) and s.get("items") is not None:
            for i, item in enumerate(body):
                child = f"{prefix}[{i}]" if prefix else f"[{i}]"
                self._walk_required(child, item, s["items"], depth + 1, out)
            return
        if s.get("type") is not None and s.get("type") != "object":
            return
        if not isinstance(body, dict):
            return
        props = s.get("properties") or {}
        for name in s.get("required") or []:
            if name not in body:
                out.append(f"{prefix}/{name}" if prefix else name)
        for name, sub in props.items():
            if name in body:
                child = f"{prefix}/{name}" if prefix else name
                self._walk_required(child, body[name], sub, depth + 1, out)

    def _walk_extras(self, prefix: str, body: Any, schema: Any, depth: int, out: list[str], seen: set[str]) -> None:
        if depth > _MAX_DEPTH or body is None:
            return
        s = self._resolve_ref(schema)
        if not isinstance(s, dict):
            return
        if isinstance(body, list):
            if s.get("type") != "array" or s.get("items") is None:
                return
            child = f"{prefix}[]" if prefix else "[]"
            for item in body:
                self._walk_extras(child, item, s["items"], depth + 1, out, seen)
            return
        if not isinstance(body, dict):
            return
        if s.get("type") is not None and s.get("type") != "object":
            return
        props = s.get("properties") or {}
        for key, value in body.items():
            field_path = f"{prefix}.{key}" if prefix else key
            if key not in props:
                if field_path not in seen:
                    seen.add(field_path)
                    out.append(field_path)
            else:
                self._walk_extras(field_path, value, props[key], depth + 1, out, seen)


def _schema_for(response: Any) -> dict | None:
    if not isinstance(response, dict):
        return None
    schema = (response.get("content") or {}).get("application/json", {}).get("schema")
    return schema if isinstance(schema, dict) else None
