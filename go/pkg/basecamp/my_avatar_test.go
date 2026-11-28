package basecamp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMyAvatarServer(t *testing.T, handler http.HandlerFunc) *PeopleService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").People()
}

func TestPeopleService_UpdateMyAvatarMultipart(t *testing.T) {
	svc := testMyAvatarServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/99999/my/avatar.json" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		f, header, err := r.FormFile("avatar")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "avatar-bytes" || header.Filename != "avatar.png" || header.Header.Get("Content-Type") != "image/png" {
			t.Fatal("multipart did not preserve the file")
		}
		if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.Value) != 0 {
			t.Fatal("unexpected multipart fields")
		}
		w.WriteHeader(204)
	})
	if err := svc.UpdateMyAvatar(context.Background(), strings.NewReader("avatar-bytes"), "avatar.png", "image/png"); err != nil {
		t.Fatal(err)
	}
}

func TestPeopleService_UpdateMyAvatarRetriesReplayBody(t *testing.T) {
	var bodies [][]byte
	svc := testMyAvatarServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	})
	if err := svc.UpdateMyAvatar(context.Background(), strings.NewReader("avatar-bytes"), "avatar.png", "image/png"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || string(bodies[0]) != string(bodies[1]) {
		t.Fatal("retry did not replay the exact multipart body")
	}
}

func TestPeopleService_UpdateMyAvatarRequiresFile(t *testing.T) {
	svc := testMyAvatarServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s %s", r.Method, r.URL)
	})
	cases := []struct {
		name        string
		avatar      io.Reader
		filename    string
		contentType string
	}{
		{"nil reader", nil, "avatar.png", "image/png"},
		{"no filename", strings.NewReader("x"), "", "image/png"},
		{"no content type", strings.NewReader("x"), "avatar.png", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateMyAvatar(context.Background(), tc.avatar, tc.filename, tc.contentType)
			var sdkErr *Error
			if !errors.As(err, &sdkErr) || sdkErr.Code != CodeUsage {
				t.Fatalf("expected a usage error, got %v", err)
			}
		})
	}
}

func TestPeopleService_UpdateMyAvatarValidationError(t *testing.T) {
	svc := testMyAvatarServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"error":"Avatar must be a PNG, JPEG, GIF, WebP, AVIF, HEIF, or HEIC image"}`))
	})
	err := svc.UpdateMyAvatar(context.Background(), strings.NewReader("not an image"), "notes.txt", "text/plain")
	if err == nil {
		t.Fatal("expected an error for 422")
	}
	if !strings.Contains(err.Error(), "Avatar must be") {
		t.Fatalf("error should carry the server's message, got %v", err)
	}
}

func TestPeopleService_DeleteMyAvatar(t *testing.T) {
	var calls int
	svc := testMyAvatarServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "DELETE" || r.URL.Path != "/99999/my/avatar.json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.WriteHeader(204)
	})
	if err := svc.DeleteMyAvatar(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}
