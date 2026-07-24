package basecamp

// Gauge and GaugeNeedle decode directly into their public structs (no
// generated converter), so their rich text description_attachments arrays
// decode by invoking RichTextAttachment.UnmarshalJSON per element. These tests
// pin that decode path plus the two structures' differing presence contracts:
// GaugeNeedle's array is @required (nil-vs-empty preserved), while Gauge's is
// optional (omitempty) because the API renders it only when the gauge has
// needles.

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGaugeNeedle_DecodesDescriptionAttachments proves the direct-decode path:
// a GaugeNeedle body with a populated description_attachments array decodes,
// invoking RichTextAttachment.UnmarshalJSON on each element so a float-spelled
// dimension (1024.0 -> 1024) and a null dimension (-> nil) decode faithfully.
func TestGaugeNeedle_DecodesDescriptionAttachments(t *testing.T) {
	body := []byte(`{
		"id": 42,
		"type": "Gauge::Needle",
		"description": "<div>Progress update with files</div>",
		"description_attachments": [
			{
				"id": 1069480030,
				"sgid": "BAh7needle1",
				"filename": "chart.png",
				"content_type": "image/png",
				"byte_size": 40960,
				"download_url": "https://example.com/download/chart.png",
				"width": 1024.0,
				"height": 768,
				"previewable": true,
				"preview_url": "https://example.com/preview/chart.png",
				"thumbnail_url": "https://example.com/thumb/chart.png"
			},
			{
				"id": 1069480031,
				"sgid": "BAh7needle2",
				"filename": "notes.pdf",
				"content_type": "application/pdf",
				"byte_size": 81920,
				"download_url": "https://example.com/download/notes.pdf",
				"width": null,
				"height": null,
				"previewable": false,
				"preview_url": "https://example.com/preview/notes.pdf",
				"thumbnail_url": "https://example.com/thumb/notes.pdf"
			}
		]
	}`)

	var needle GaugeNeedle
	if err := json.Unmarshal(body, &needle); err != nil {
		t.Fatalf("failed to unmarshal GaugeNeedle: %v", err)
	}
	if len(needle.DescriptionAttachments) != 2 {
		t.Fatalf("expected 2 description attachments, got %d", len(needle.DescriptionAttachments))
	}
	img := needle.DescriptionAttachments[0]
	if img.ID != 1069480030 || img.Filename != "chart.png" || img.ContentType != "image/png" {
		t.Errorf("unexpected image attachment: %+v", img)
	}
	if img.Width == nil || *img.Width != 1024 {
		t.Errorf("expected image Width 1024 (float-spelled 1024.0), got %v", img.Width)
	}
	if img.Height == nil || *img.Height != 768 {
		t.Errorf("expected image Height 768, got %v", img.Height)
	}
	blob := needle.DescriptionAttachments[1]
	if blob.ID != 1069480031 || blob.Width != nil || blob.Height != nil {
		t.Errorf("expected non-image blob with nil dimensions, got %+v", blob)
	}
}

// TestGauge_DescriptionAttachments_PresenceContract pins Gauge's optional
// array: an absent key stays nil, a server-sent [] decodes to a non-nil
// zero-length slice.
func TestGauge_DescriptionAttachments_PresenceContract(t *testing.T) {
	// Absent key -> nil.
	var absent Gauge
	if err := json.Unmarshal([]byte(`{"id": 7, "type": "Gauge"}`), &absent); err != nil {
		t.Fatalf("failed to unmarshal needle-less gauge: %v", err)
	}
	if absent.DescriptionAttachments != nil {
		t.Errorf("expected nil DescriptionAttachments for absent key, got %v", absent.DescriptionAttachments)
	}

	// Present but empty -> non-nil zero-length slice.
	var empty Gauge
	if err := json.Unmarshal([]byte(`{"id": 7, "type": "Gauge", "description_attachments": []}`), &empty); err != nil {
		t.Fatalf("failed to unmarshal gauge with empty array: %v", err)
	}
	if empty.DescriptionAttachments == nil {
		t.Error("expected non-nil DescriptionAttachments for server-sent []")
	}
	if len(empty.DescriptionAttachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(empty.DescriptionAttachments))
	}
}

// TestGauge_MarshalOmitsAbsentAttachments pins the omitempty choice on Gauge's
// optional, non-nullable array: a nil array must be omitted on re-encode, never
// emitted as an invalid "description_attachments": null.
func TestGauge_MarshalOmitsAbsentAttachments(t *testing.T) {
	data, err := json.Marshal(Gauge{ID: 7})
	if err != nil {
		t.Fatalf("failed to marshal gauge: %v", err)
	}
	if strings.Contains(string(data), "description_attachments") {
		t.Errorf("expected absent (nil) description_attachments to be omitted, got %s", data)
	}
}
