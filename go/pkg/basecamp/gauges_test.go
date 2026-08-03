package basecamp

// Gauge and GaugeNeedle decode directly into their public structs (no
// generated converter), so their rich text description_attachments arrays
// decode by invoking RichTextAttachment.UnmarshalJSON per element. These tests
// pin that decode path plus the two structures' differing presence contracts:
// GaugeNeedle's array is @required (nil-vs-empty preserved), while Gauge's is
// optional (omitempty) because the API renders it only when the gauge has
// needles.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	// Present but empty -> non-nil pointer to a zero-length slice.
	var empty Gauge
	if err := json.Unmarshal([]byte(`{"id": 7, "type": "Gauge", "description_attachments": []}`), &empty); err != nil {
		t.Fatalf("failed to unmarshal gauge with empty array: %v", err)
	}
	if empty.DescriptionAttachments == nil {
		t.Fatal("expected non-nil DescriptionAttachments pointer for server-sent []")
	}
	if len(*empty.DescriptionAttachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(*empty.DescriptionAttachments))
	}
}

// TestGauge_MarshalRoundTripsPresence pins the *[]RichTextAttachment choice on
// Gauge's optional, non-nullable array: an absent (nil-pointer) array is omitted
// on re-encode — never an invalid "description_attachments": null — while a
// present-but-empty array re-encodes as [], staying distinct from absent.
func TestGauge_MarshalRoundTripsPresence(t *testing.T) {
	// Absent (nil pointer): key omitted entirely.
	data, err := json.Marshal(Gauge{ID: 7})
	if err != nil {
		t.Fatalf("failed to marshal needle-less gauge: %v", err)
	}
	if strings.Contains(string(data), "description_attachments") {
		t.Errorf("expected absent description_attachments to be omitted, got %s", data)
	}

	// Present but empty (non-nil pointer to empty slice): encodes as [].
	empty := []RichTextAttachment{}
	data, err = json.Marshal(Gauge{ID: 7, DescriptionAttachments: &empty})
	if err != nil {
		t.Fatalf("failed to marshal gauge with empty attachments: %v", err)
	}
	if !strings.Contains(string(data), `"description_attachments":[]`) {
		t.Errorf("expected present-empty description_attachments to encode as [], got %s", data)
	}
}

// The gauge list surfaces took no options struct at all before #570, so `page`
// could not reach the wire and every call walked the whole collection. These
// tests pin the shape sibling services already had: a positive Page selects
// exactly that page (one request, no Link follow) and reports Truncated from
// the rel="next" Link it deliberately did not follow (SPEC §8).

func testGaugesServer(t *testing.T, handler http.HandlerFunc) *GaugesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").Gauges()
}

func TestGaugesService_List_PageSelectsOnePage(t *testing.T) {
	var requestCount int
	var sawPage string
	svc := testGaugesServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			sawPage = r.URL.Query().Get("page")
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/reports/gauges.json?page=4>; rel="next"`, r.Host))
			w.Header().Set("X-Total-Count", "9")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id": 1, "title": "One"}, {"id": 2, "title": "Two"}]`))
	})

	result, err := svc.List(context.Background(), &GaugeListOptions{Page: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawPage != "3" {
		t.Errorf("expected page=3 on the wire, got %q", sawPage)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request, got %d", requestCount)
	}
	if len(result.Gauges) != 2 {
		t.Errorf("expected 2 gauges, got %d", len(result.Gauges))
	}
	if result.Meta.TotalCount != 9 {
		t.Errorf("expected TotalCount 9, got %d", result.Meta.TotalCount)
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated=true: the selected page advertised a next page")
	}
}

func TestGaugesService_List_NoPageFollowsLinks(t *testing.T) {
	var requestCount int
	svc := testGaugesServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			if r.URL.Query().Get("page") != "" {
				t.Errorf("expected no page param, got %q", r.URL.Query().Get("page"))
			}
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/reports/gauges.json?page=2>; rel="next"`, r.Host))
			w.WriteHeader(200)
			w.Write([]byte(`[{"id": 1, "title": "One"}]`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`[{"id": 2, "title": "Two"}]`))
	})

	result, err := svc.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", requestCount)
	}
	if len(result.Gauges) != 2 {
		t.Errorf("expected 2 gauges, got %d", len(result.Gauges))
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated=false after the walk reached the last page")
	}
}

func TestGaugesService_ListNeedles_PageSelectsOnePage(t *testing.T) {
	var requestCount int
	var sawPage string
	svc := testGaugesServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			sawPage = r.URL.Query().Get("page")
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/projects/7/gauge/needles.json?page=3>; rel="next"`, r.Host))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id": 11, "type": "Gauge::Needle"}]`))
	})

	result, err := svc.ListNeedles(context.Background(), 7, &GaugeNeedleListOptions{Page: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawPage != "2" {
		t.Errorf("expected page=2 on the wire, got %q", sawPage)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request, got %d", requestCount)
	}
	if len(result.Needles) != 1 {
		t.Errorf("expected 1 needle, got %d", len(result.Needles))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated=true: the selected page advertised a next page")
	}
}

func TestGaugesService_ListNeedles_PinnedFinalPageIsNotTruncated(t *testing.T) {
	var requestCount int
	svc := testGaugesServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id": 11, "type": "Gauge::Needle"}]`))
	})

	result, err := svc.ListNeedles(context.Background(), 7, &GaugeNeedleListOptions{Page: 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request, got %d", requestCount)
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated=false: the pinned page carried no next link")
	}
}
