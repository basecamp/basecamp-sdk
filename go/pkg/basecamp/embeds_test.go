package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const slashedEmbedSGID = "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiI2dpZDovL2JjMy9FbWJlZC85OTg/ZXhwaXJlc19pbgY7AFRJIghwdXIGOwBUSSIPYXR0YWNoYWJsZQY7AFQ=--5d2b6e1c9a0f4e8b7c3d2a1f0e9d8c7b6a5f4e3d"

func loadEmbedsFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "fixtures", "embeds", name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func testEmbedsServer(t *testing.T, handler http.HandlerFunc, opts ...ClientOption) *EmbedsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, opts...)
	return client.ForAccount("99999").Embeds()
}

func TestEmbedsService_Create(t *testing.T) {
	var received map[string]any
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/99999/embeds.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(loadEmbedsFixture(t, "pending.json"))
	})

	embed, err := svc.Create(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received["url"] != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" || len(received) != 1 {
		t.Errorf("expected only the url in the body, got %v", received)
	}
	want := Embed{URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", Status: "pending", SGID: slashedEmbedSGID}
	if *embed != want {
		t.Errorf("expected %+v, got %+v", want, *embed)
	}
}

func TestEmbedsService_Create_ExistingEmbedAnswers200(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(loadEmbedsFixture(t, "active.json"))
	})

	embed, err := svc.Create(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embed.Status != "active" || embed.SGID != slashedEmbedSGID || embed.ContentType != "embed/youtube-video" || embed.Type != "video" {
		t.Errorf("unexpected embed %+v", *embed)
	}
	if embed.ImageURL != "" {
		t.Errorf("expected a null image_url to read as empty, got %q", embed.ImageURL)
	}
	if embed.Content != `<iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ"></iframe>` {
		t.Errorf("unexpected content %q", embed.Content)
	}
}

func TestEmbedsService_Create_EmptyURL(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("expected no request, got %s %s", r.Method, r.URL.Path)
	})

	_, err := svc.Create(context.Background(), "")
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestEmbedsService_Create_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		code    string
		message string
	}{
		"not embeddable": {422, `{"error":"Basecamp can't embed this URL. Link to it instead."}`, CodeValidation, "Basecamp can't embed this URL. Link to it instead."},
		"too many":       {429, `{"error":"Too many new embeds. Try again later."}`, CodeRateLimit, "Too many new embeds. Try again later."},
		"read-only":      {403, `{"error":"insufficient_scope"}`, CodeForbidden, "insufficient_scope"},
	} {
		t.Run(name, func(t *testing.T) {
			svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}, WithMaxRetries(1))

			embed, err := svc.Create(context.Background(), "https://example.com/page")
			if embed != nil {
				t.Errorf("expected no embed, got %+v", embed)
			}
			apiErr, ok := errors.AsType[*Error](err)
			if !ok {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			if apiErr.Code != tc.code || apiErr.HTTPStatus != tc.status || apiErr.Message != tc.message {
				t.Errorf("expected %s %d %q, got %s %d %q", tc.code, tc.status, tc.message, apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
			}
		})
	}
}

func TestEmbedsService_Create_RetriesAsIdempotent(t *testing.T) {
	attempts := 0
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(loadEmbedsFixture(t, "pending.json"))
	}, WithBaseDelay(1))

	embed, err := svc.Create(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 2 || embed.SGID != slashedEmbedSGID {
		t.Errorf("expected the retry to return the embed, got %d attempts and %+v", attempts, embed)
	}
}

func TestEmbedsService_Get_EscapesTheSGID(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.EscapedPath() != "/99999/embeds/BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiI2dpZDovL2JjMy9FbWJlZC85OTg%2FZXhwaXJlc19pbgY7AFRJIghwdXIGOwBUSSIPYXR0YWNoYWJsZQY7AFQ=--5d2b6e1c9a0f4e8b7c3d2a1f0e9d8c7b6a5f4e3d" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadEmbedsFixture(t, "active.json"))
	})

	embed, err := svc.Get(context.Background(), slashedEmbedSGID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embed.Status != "active" {
		t.Errorf("expected an active embed, got %+v", *embed)
	}
}

func TestEmbedsService_Get_SlashOnlySGIDStaysOneSegment(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/99999/embeds/a%2Fb+c=" {
			t.Errorf("unexpected path %s", r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadEmbedsFixture(t, "pending.json"))
	})

	if _, err := svc.Get(context.Background(), "a/b+c="); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmbedsService_Get_EmptySGID(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("expected no request, got %s %s", r.Method, r.URL.Path)
	})

	_, err := svc.Get(context.Background(), "")
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestEmbedsService_Get_NotFound(t *testing.T) {
	svc := testEmbedsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := svc.Get(context.Background(), "missing")
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound || apiErr.HTTPStatus != 404 {
		t.Errorf("expected not_found, got: %v", err)
	}
}
