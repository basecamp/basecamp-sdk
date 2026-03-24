package basecamp

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testAccountServer creates an httptest.Server and an AccountService wired to it.
func testAccountServer(t *testing.T, handler http.HandlerFunc) *AccountService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Account()
}

func TestAccountService_UpdateLogo(t *testing.T) {
	svc := testAccountServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/account/logo.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify multipart content type
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("failed to parse Content-Type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("expected multipart/form-data, got %s", mediaType)
		}

		// Parse the multipart body
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("failed to read multipart part: %v", err)
		}
		if part.FormName() != "logo" {
			t.Errorf("expected form name 'logo', got %q", part.FormName())
		}
		if part.FileName() != "test-logo.png" {
			t.Errorf("expected filename 'test-logo.png', got %q", part.FileName())
		}
		if part.Header.Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type 'image/png', got %q", part.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(part)
		if string(body) != "fake-png-data" {
			t.Errorf("unexpected body: %q", string(body))
		}

		w.WriteHeader(204)
	})

	err := svc.UpdateLogo(context.Background(), strings.NewReader("fake-png-data"), "test-logo.png", "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountService_UpdateLogo_401WithoutRefresh(t *testing.T) {
	attempts := 0
	svc := testAccountServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"expired token"}`))
			return
		}
		w.WriteHeader(204)
	})

	err := svc.UpdateLogo(context.Background(), strings.NewReader("fake-png-data"), "logo.png", "image/png")
	// StaticTokenProvider doesn't implement AuthManager, so no refresh happens — first 401 is returned.
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestAccountService_UpdateLogo_ValidationErrors(t *testing.T) {
	svc := testAccountServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	ctx := context.Background()

	if err := svc.UpdateLogo(ctx, nil, "logo.png", "image/png"); err == nil {
		t.Error("expected error for nil logo")
	}
	if err := svc.UpdateLogo(ctx, strings.NewReader("data"), "", "image/png"); err == nil {
		t.Error("expected error for empty filename")
	}
	if err := svc.UpdateLogo(ctx, strings.NewReader("data"), "logo.png", ""); err == nil {
		t.Error("expected error for empty content type")
	}
}

func TestAccountService_UpdateLogo_SanitizesInputs(t *testing.T) {
	svc := testAccountServer(t, func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, _ := mime.ParseMediaType(ct)
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("failed to read part: %v", err)
		}

		// Filename should have CRLF stripped and quotes escaped
		if strings.ContainsAny(part.FileName(), "\r\n") {
			t.Errorf("filename contains CRLF: %q", part.FileName())
		}

		// Content-Type header should have CRLF stripped
		partCT := part.Header.Get("Content-Type")
		if strings.ContainsAny(partCT, "\r\n") {
			t.Errorf("Content-Type contains CRLF: %q", partCT)
		}

		w.WriteHeader(204)
	})

	err := svc.UpdateLogo(
		context.Background(),
		strings.NewReader("data"),
		"evil\r\nContent-Type: text/html\r\n\r\nname.png",
		"image/png\r\nX-Injected: true",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
