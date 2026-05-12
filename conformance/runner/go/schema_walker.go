package main

// Pure-Go port of conformance/runner/typescript/schema-validator.ts.
//
// Walks parsed JSON against the OpenAPI response schema, surfacing:
//   - MissingRequired: slash-separated paths for required fields absent
//     from the body (e.g. "owner/id")
//   - ExtrasSeen: dotted paths for fields present on the wire but not
//     declared (e.g. "unreads[].new_field")
//
// Conventions match the TS walker (and the Python/Ruby ports under
// conformance/runner/{python,ruby}/) so cross-language extras parity diffs
// (PR 4 §Verification) don't false-fire:
//   - Required-walk object paths use "/" (e.g. "owner/id"); extras-walk
//     object paths use "." (e.g. "owner.new_field"). The two streams use
//     distinct separators so they're visually distinguishable in tooling.
//   - Required walk uses "[i]" element segments; extras walk uses "[]"
//     to dedupe item-level extras across an array
//   - "$ref" chains resolve until a non-ref schema or a cycle. Both
//     "#/components/schemas/X" and "openapi.json#/components/schemas/X"
//     are accepted.
//   - additionalProperties:false is intentionally ignored — extras are
//     reported but do not fail validation (forward-compat).
//   - Recursion depth bound 12 as a cycle guard.
//
// No new dependencies: hand-rolled keeps semantics identical to the TS
// walker. After json.Unmarshal into any, JSON objects become
// map[string]any and arrays become []any; both branches dispatch on that.

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
)

const maxDepth = 12

var refRE = regexp.MustCompile(`^(?:openapi\.json)?#/components/schemas/(.+)$`)

type SchemaWalker struct {
	doc map[string]any
}

func NewSchemaWalker(openapiPath string) (*SchemaWalker, error) {
	raw, err := os.ReadFile(openapiPath)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return &SchemaWalker{doc: doc}, nil
}

// FindResponseSchema returns the response schema for operationID, or nil
// when none is found. Match TS preference order: 200, then any 2xx, then
// "default".
func (w *SchemaWalker) FindResponseSchema(operationID string) map[string]any {
	paths, _ := w.doc["paths"].(map[string]any)
	for _, pathName := range sortedKeys(paths) {
		pathItem, _ := paths[pathName].(map[string]any)
		for _, method := range sortedKeys(pathItem) {
			op, ok := pathItem[method].(map[string]any)
			if !ok {
				continue
			}
			if id, _ := op["operationId"].(string); id != operationID {
				continue
			}
			responses, _ := op["responses"].(map[string]any)
			for _, code := range []string{"200", "201", "202", "203", "204", "default"} {
				if s := schemaFor(responses[code]); s != nil {
					return s
				}
			}
			for _, code := range sortedKeys(responses) {
				if len(code) == 3 && code[0] == '2' && isDigit(code[1]) && isDigit(code[2]) {
					if s := schemaFor(responses[code]); s != nil {
						return s
					}
				}
			}
		}
	}
	return nil
}

// MissingRequired returns slash-separated path strings for required fields
// absent from body (e.g. "owner/id").
func (w *SchemaWalker) MissingRequired(body any, schema map[string]any) []string {
	return w.walkRequired("", body, schema, 0)
}

// ExtrasSeen returns dotted-path strings for fields present on the wire
// but not declared in the schema. Recurses through known properties so
// item-level extras on lists surface (e.g. "unreads[].new_field").
func (w *SchemaWalker) ExtrasSeen(body any, schema map[string]any) []string {
	return w.walkExtras("", body, schema, 0)
}

func (w *SchemaWalker) walkRequired(prefix string, body, schema any, depth int) []string {
	if depth > maxDepth || body == nil {
		return nil
	}
	resolved := w.resolveRef(schema)
	s, ok := resolved.(map[string]any)
	if !ok {
		return nil
	}

	if arr, isArr := body.([]any); isArr {
		typ, _ := s["type"].(string)
		if typ != "array" || s["items"] == nil {
			return nil
		}
		var missing []string
		for i, item := range arr {
			child := joinIndex(prefix, i)
			missing = append(missing, w.walkRequired(child, item, s["items"], depth+1)...)
		}
		return missing
	}

	obj, isObj := body.(map[string]any)
	if !isObj {
		return nil
	}
	if typ, present := s["type"].(string); present && typ != "object" {
		return nil
	}

	props, _ := s["properties"].(map[string]any)
	required, _ := s["required"].([]any)
	var missing []string
	// Required-field paths use `/` as the separator (walkExtras uses `.`)
	// so the two streams are visually distinct in tooling and consistent
	// across Ruby/Python/Go/Kotlin walkers.
	for _, r := range required {
		name, _ := r.(string)
		if _, ok := obj[name]; !ok {
			missing = append(missing, joinSlash(prefix, name))
		}
	}
	// Recurse into present known props. Sort keys: cross-language
	// comparable output (TS preserves JSON parse order; Go map iteration
	// is randomized, so sorted keys keep diffs stable).
	for _, name := range sortedKeys(props) {
		if value, ok := obj[name]; ok {
			missing = append(missing, w.walkRequired(joinSlash(prefix, name), value, props[name], depth+1)...)
		}
	}
	return missing
}

func (w *SchemaWalker) walkExtras(prefix string, body, schema any, depth int) []string {
	if depth > maxDepth || body == nil {
		return nil
	}
	resolved := w.resolveRef(schema)
	s, ok := resolved.(map[string]any)
	if !ok {
		return nil
	}

	if arr, isArr := body.([]any); isArr {
		typ, _ := s["type"].(string)
		if typ != "array" || s["items"] == nil {
			return nil
		}
		// Per-array dedup mirrors TS collectExtras's `new Set` for arrays.
		seen := map[string]struct{}{}
		var out []string
		child := joinBrackets(prefix)
		for _, item := range arr {
			for _, e := range w.walkExtras(child, item, s["items"], depth+1) {
				if _, dup := seen[e]; !dup {
					seen[e] = struct{}{}
					out = append(out, e)
				}
			}
		}
		return out
	}

	obj, isObj := body.(map[string]any)
	if !isObj {
		return nil
	}
	if typ, present := s["type"].(string); present && typ != "object" {
		return nil
	}

	props, _ := s["properties"].(map[string]any)
	var extras []string
	// Sort body keys: stable cross-language output. TS preserves JSON parse
	// order, which equals insertion order; Go map iteration is randomized,
	// so we sort. PR 4 cross-lang extras parity diff compares deduped sets
	// anyway, so this only affects the per-page ordering inside the snapshot.
	for _, key := range sortedKeys(obj) {
		fieldPath := joinDot(prefix, key)
		if _, known := props[key]; known {
			extras = append(extras, w.walkExtras(fieldPath, obj[key], props[key], depth+1)...)
		} else {
			extras = append(extras, fieldPath)
		}
	}
	return extras
}

// resolveRef follows $ref chains until a non-ref schema or a cycle.
// Accepts "#/components/schemas/X" and "openapi.json#/components/schemas/X".
func (w *SchemaWalker) resolveRef(schema any) any {
	seen := map[string]struct{}{}
	current := schema
	for {
		m, ok := current.(map[string]any)
		if !ok {
			return current
		}
		ref, ok := m["$ref"].(string)
		if !ok {
			return current
		}
		if _, dup := seen[ref]; dup {
			return current
		}
		seen[ref] = struct{}{}
		match := refRE.FindStringSubmatch(ref)
		if match == nil {
			return current
		}
		components, _ := w.doc["components"].(map[string]any)
		schemas, _ := components["schemas"].(map[string]any)
		next, ok := schemas[match[1]]
		if !ok {
			return current
		}
		current = next
	}
}

func schemaFor(response any) map[string]any {
	resp, ok := response.(map[string]any)
	if !ok {
		return nil
	}
	content, _ := resp["content"].(map[string]any)
	appJSON, _ := content["application/json"].(map[string]any)
	schema, _ := appJSON["schema"].(map[string]any)
	return schema
}

func sortedKeys(m map[string]any) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func joinDot(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func joinSlash(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func joinIndex(prefix string, i int) string {
	suffix := "[" + itoa(i) + "]"
	if prefix == "" {
		return suffix
	}
	return prefix + suffix
}

func joinBrackets(prefix string) string {
	if prefix == "" {
		return "[]"
	}
	return prefix + "[]"
}

// itoa avoids importing strconv just for one use.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
