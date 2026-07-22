package basecamp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

// normalizePersonIds walks a JSON-decoded value and normalizes Person-shaped
// objects. When an object has a "personable_type" field and its "id" is a
// string, the id is coerced to a number (0 for non-numeric sentinels like
// "basecamp") and the original label is preserved as "system_label".
// Numeric overflow strings are left as-is (the downstream FlexibleInt64
// decoder will reject them).
func normalizePersonIds(v any) {
	switch val := v.(type) {
	case map[string]any:
		if _, hasPT := val["personable_type"]; hasPT {
			coercePersonID(val)
		}
		for _, child := range val {
			normalizePersonIds(child)
		}
	case []any:
		for _, item := range val {
			normalizePersonIds(item)
		}
	}
}

// coercePersonID rewrites a Person-shaped object's string "id" in place to the
// numeric form the wrapper Person.ID (a plain int64) can decode. A numeric
// string ("12345") becomes a json.Number; a non-numeric sentinel ("basecamp")
// collapses to 0 with the original label preserved as "system_label"; a numeric
// overflow string is left untouched so the decoder rejects it. Objects whose id
// is already a number (or absent) are left unchanged. This is the shared id
// rule used by both the personable_type-keyed pass and the notification
// creator/participants pass.
func coercePersonID(obj map[string]any) {
	idStr, ok := obj["id"].(string)
	if !ok {
		return
	}
	_, err := strconv.ParseInt(idStr, 10, 64)
	if err == nil {
		obj["id"] = json.Number(idStr) // numeric string — preserve as json.Number
		return
	}
	var numErr *strconv.NumError
	if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
		return // numeric overflow — leave as string, let decoder reject
	}
	// Non-numeric sentinel (e.g. "basecamp" for system-generated entities).
	obj["system_label"] = idStr
	obj["id"] = json.Number("0")
}

// normalizeEmbeddedPersonIds walks a JSON-decoded payload and coerces the string
// ids of people embedded under the well-known "creator" and "participants" keys,
// regardless of whether those person objects carry a "personable_type" field.
//
// BC3 serializes person ids as strings (the wire-format mismatch FlexibleInt64
// documents). Wrappers that embed *Person under these keys (Notification,
// Gauge, GaugeNeedle, ...) decode into Person.ID (a plain int64), which cannot
// unmarshal a JSON string, so an un-normalized string id fails the whole decode.
// The generic normalizePersonIds pass only fires on objects that have
// "personable_type"; embedded creator/participants people frequently omit it,
// so this pass targets them by their known structural position (the creator
// object and each participants element) and applies the same coercePersonID
// rule.
func normalizeEmbeddedPersonIds(v any) {
	switch val := v.(type) {
	case map[string]any:
		if creator, ok := val["creator"].(map[string]any); ok {
			coercePersonID(creator)
		}
		if participants, ok := val["participants"].([]any); ok {
			for _, p := range participants {
				if person, ok := p.(map[string]any); ok {
					coercePersonID(person)
				}
			}
		}
		for _, child := range val {
			normalizeEmbeddedPersonIds(child)
		}
	case []any:
		for _, item := range val {
			normalizeEmbeddedPersonIds(item)
		}
	}
}

// normalizeEmbeddedPeopleJSON normalizes a raw response that embeds *Person
// under "creator"/"participants" (notifications, gauges, gauge needles) before
// it is decoded onto the wrapper. It applies both the generic
// personable_type-keyed person normalization AND the embedded creator/participants
// pass, so embedded people with string ids decode into Person.ID (a plain int64)
// even when those person objects omit "personable_type".
//
// It short-circuits only when the body contains none of
// "personable_type", "creator", or "participants" — an embedded person id can be
// a string without any "personable_type" appearing in the body, so the
// personable_type-only guard would skip the very payloads this exists to fix.
func normalizeEmbeddedPeopleJSON(data []byte) ([]byte, error) {
	if !bytes.Contains(data, []byte(`"personable_type"`)) &&
		!bytes.Contains(data, []byte(`"creator"`)) &&
		!bytes.Contains(data, []byte(`"participants"`)) {
		return data, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return data, err
	}
	normalizePersonIds(raw)
	normalizeEmbeddedPersonIds(raw)
	return json.Marshal(raw)
}
