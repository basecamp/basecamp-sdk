package basecamp

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TestGeneratedParsers_BodylessSuccess pins the behavior of the generated
// response parsers for bodyless success statuses across all three shapes the
// spec produces: bodyless-200, bodyless-201, and bodyless-204.
//
// It exists to prove the oapi-codegen v2.7.1 -> v2.8.0 upgrade's only
// behavioral change — v2.8's explicit `case rsp.StatusCode == N: break // No
// content-type` inserted into 40 bodyless response parsers — is a no-op for the
// success path: the parser returns no error, preserves the raw body, populates
// HTTPResponse, and sets none of the typed error fields. The existing
// conformance suite already covers a bodyless-204 (TrashProject); this adds the
// bodyless-200 (MarkAsRead) and bodyless-201 (EnableTool) that the conformance
// dispatcher does not exercise, so all three shapes are proven.
func TestGeneratedParsers_BodylessSuccess(t *testing.T) {
	// A bodyless success response carries no Content-Type and an empty body,
	// exactly as Rails returns for these operations.
	newResp := func(status int) *http.Response {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		}
	}

	t.Run("MarkAsRead 200", func(t *testing.T) {
		raw := newResp(http.StatusOK)
		defer func() { _ = raw.Body.Close() }()
		resp, err := generated.ParseMarkAsReadResponse(raw)
		if err != nil {
			t.Fatalf("ParseMarkAsReadResponse returned error: %v", err)
		}
		if resp.HTTPResponse == nil || resp.HTTPResponse.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTPResponse with status 200, got %+v", resp.HTTPResponse)
		}
		if len(resp.Body) != 0 {
			t.Fatalf("expected empty body, got %q", resp.Body)
		}
		if resp.JSON401 != nil || resp.JSON403 != nil || resp.JSON429 != nil || resp.JSON500 != nil {
			t.Fatalf("expected no typed error field populated on 200, got %+v", resp)
		}
	})

	t.Run("EnableTool 201", func(t *testing.T) {
		raw := newResp(http.StatusCreated)
		defer func() { _ = raw.Body.Close() }()
		resp, err := generated.ParseEnableToolResponse(raw)
		if err != nil {
			t.Fatalf("ParseEnableToolResponse returned error: %v", err)
		}
		if resp.HTTPResponse == nil || resp.HTTPResponse.StatusCode != http.StatusCreated {
			t.Fatalf("expected HTTPResponse with status 201, got %+v", resp.HTTPResponse)
		}
		if len(resp.Body) != 0 {
			t.Fatalf("expected empty body, got %q", resp.Body)
		}
		if resp.JSON401 != nil || resp.JSON403 != nil || resp.JSON422 != nil || resp.JSON429 != nil || resp.JSON500 != nil {
			t.Fatalf("expected no typed error field populated on 201, got %+v", resp)
		}
	})

	t.Run("TrashProject 204", func(t *testing.T) {
		raw := newResp(http.StatusNoContent)
		defer func() { _ = raw.Body.Close() }()
		resp, err := generated.ParseTrashProjectResponse(raw)
		if err != nil {
			t.Fatalf("ParseTrashProjectResponse returned error: %v", err)
		}
		if resp.HTTPResponse == nil || resp.HTTPResponse.StatusCode != http.StatusNoContent {
			t.Fatalf("expected HTTPResponse with status 204, got %+v", resp.HTTPResponse)
		}
		if len(resp.Body) != 0 {
			t.Fatalf("expected empty body, got %q", resp.Body)
		}
		if resp.JSON401 != nil || resp.JSON403 != nil || resp.JSON404 != nil || resp.JSON500 != nil {
			t.Fatalf("expected no typed error field populated on 204, got %+v", resp)
		}
	})
}
