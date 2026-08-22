package basecamp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- filenameFromURL ---

func Test_filenameFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"simple filename", "https://storage.3.basecamp.com/123/blobs/abc/download/logo.png", "logo.png"},
		{"encoded filename", "https://storage.3.basecamp.com/123/blobs/abc/download/my%20file.pdf", "my file.pdf"},
		{"trailing slash", "https://storage.3.basecamp.com/123/blobs/abc/download/", "download"},
		{"no path", "https://storage.3.basecamp.com", "download"},
		{"empty string", "", "download"},
		{"just slash", "https://storage.3.basecamp.com/", "download"},
		{"deep path", "https://example.com/a/b/c/report.csv", "report.csv"},
		{"with query", "https://example.com/path/file.txt?disposition=attachment", "file.txt"},
		{"invalid url", "://bad", "download"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filenameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("filenameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --- DownloadURL validation ---

func TestDownloadURL_Validation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "https://3.basecampapi.com"
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"relative path", "/blobs/abc/download/file.png"},
		{"no scheme", "storage.3.basecamp.com/blobs/abc/download/file.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ac.DownloadURL(context.Background(), tt.url)
			if err == nil {
				t.Fatal("expected ErrUsage")
			}
			var sdkErr *Error
			if !isSDKError(err, &sdkErr) || sdkErr.Code != CodeUsage {
				t.Errorf("expected usage error, got: %v", err)
			}
		})
	}
}

// --- URL rewriting ---

func TestDownloadURL_URLRewriting(t *testing.T) {
	var receivedPath, receivedQuery string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if receivedPath != "/999/blobs/abc/download/file.png" {
		t.Errorf("expected path /999/blobs/abc/download/file.png, got %q", receivedPath)
	}
	if receivedQuery != "" {
		t.Errorf("expected empty query, got %q", receivedQuery)
	}
}

func TestDownloadURL_HostAgnosticInputs(t *testing.T) {
	var receivedPath string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	origins := []string{
		"https://storage.3.basecamp.com",
		"https://basecamp-static.example.com",
		"https://3.basecampapi.com",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			receivedPath = ""
			result, err := ac.DownloadURL(context.Background(),
				origin+"/999/blobs/abc/download/file.png")
			if err != nil {
				t.Fatalf("unexpected error for origin %s: %v", origin, err)
			}
			defer result.Body.Close()
			io.Copy(io.Discard, result.Body)

			if receivedPath != "/999/blobs/abc/download/file.png" {
				t.Errorf("origin %s: expected path /999/blobs/abc/download/file.png, got %q", origin, receivedPath)
			}
		})
	}
}

func TestDownloadURL_QueryPreservation(t *testing.T) {
	var receivedQuery string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.png?disposition=attachment&foo=bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if receivedQuery != "disposition=attachment&foo=bar" {
		t.Errorf("expected query 'disposition=attachment&foo=bar', got %q", receivedQuery)
	}
}

// --- Redirect flow ---

func TestDownloadURL_Redirect(t *testing.T) {
	fileContent := "binary file data"

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "16")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Server.URL+"/bucket/signed-file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(http.DefaultTransport))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/photo.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != fileContent {
		t.Errorf("expected body %q, got %q", fileContent, string(body))
	}
	if result.ContentType != "image/png" {
		t.Errorf("expected Content-Type image/png, got %q", result.ContentType)
	}
	if result.ContentLength != 16 {
		t.Errorf("expected Content-Length 16, got %d", result.ContentLength)
	}
	if result.Filename != "photo.png" {
		t.Errorf("expected Filename photo.png, got %q", result.Filename)
	}
}

func TestDownloadURL_DirectDownload(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pdf-data"))
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "pdf-data" {
		t.Errorf("expected body 'pdf-data', got %q", string(body))
	}
	if result.ContentType != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %q", result.ContentType)
	}
}

func TestDownloadURL_RelativeLocation(t *testing.T) {
	// Use a single server: API handler returns a path-only Location,
	// and the same server handles the resolved path with file content.
	var resolvedHit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/resolved-path", func(w http.ResponseWriter, r *http.Request) {
		resolvedHit = true
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("resolved-data"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API handler: redirect with a relative (path-only) Location
		w.Header().Set("Location", "/resolved-path")
		w.WriteHeader(http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(http.DefaultTransport))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "resolved-data" {
		t.Errorf("expected body 'resolved-data', got %q", string(body))
	}
	if !resolvedHit {
		t.Error("expected /resolved-path handler to be hit after relative redirect")
	}
}

func TestDownloadURL_RedirectNoLocation(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound) // 302 with no Location header
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	ac := client.ForAccount("12345")

	_, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err == nil {
		t.Fatal("expected error for redirect with no Location")
	}
	if !strings.Contains(err.Error(), "no Location") {
		t.Errorf("expected 'no Location' in error, got: %v", err)
	}
}

// --- Error handling ---

func TestDownloadURL_APIError(t *testing.T) {
	// 502/503/504 exercise the retry loop (see TestDownloadURL_AuthHopRetriesOn503);
	// this table covers non-retryable error-mapping paths, including 500 which
	// fetchAPIDownload intentionally does not retry (matches the @retryable set
	// in Client.singleRequest).
	tests := []struct {
		name     string
		status   int
		wantCode string
	}{
		{"not found", http.StatusNotFound, CodeNotFound},
		{"forbidden", http.StatusForbidden, CodeForbidden},
		{"server error", http.StatusInternalServerError, CodeAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(tt.status)
			}))
			defer apiServer.Close()

			cfg := DefaultConfig()
			cfg.BaseURL = apiServer.URL
			token := &StaticTokenProvider{Token: "test-token"}
			client := NewClient(cfg, token)
			ac := client.ForAccount("12345")

			_, err := ac.DownloadURL(context.Background(),
				"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
			if err == nil {
				t.Fatal("expected error")
			}
			var sdkErr *Error
			if !isSDKError(err, &sdkErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			if sdkErr.Code != tt.wantCode {
				t.Errorf("expected code %q, got %q", tt.wantCode, sdkErr.Code)
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("expected 1 request (non-retryable), got %d", got)
			}
		})
	}
}

func TestDownloadURL_S3Error(t *testing.T) {
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Server.URL+"/bucket/file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(http.DefaultTransport))
	ac := client.ForAccount("12345")

	_, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.png")
	if err == nil {
		t.Fatal("expected error for S3 403")
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Errorf("expected 'status 403' in error, got: %v", err)
	}
}

// --- Auth header assertions ---

func TestDownloadURL_AuthHeaders(t *testing.T) {
	var apiAuthHeader, s3AuthHeader string

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s3AuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Location", s3Server.URL+"/bucket/file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(http.DefaultTransport))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if apiAuthHeader == "" {
		t.Error("expected Authorization header on API request")
	}
	if s3AuthHeader != "" {
		t.Errorf("expected no Authorization header on S3 request, got %q", s3AuthHeader)
	}
}

// --- Hook assertions ---

func TestDownloadURL_RequestHooksAPIOnly(t *testing.T) {
	var requestStartCount atomic.Int32
	var requestEndCount atomic.Int32

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Server.URL+"/bucket/file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	hooks := &testHooks{
		onRequestStart: func(ctx context.Context, info RequestInfo) context.Context {
			requestStartCount.Add(1)
			return ctx
		},
		onRequestEnd: func(ctx context.Context, info RequestInfo, result RequestResult) {
			requestEndCount.Add(1)
		},
	}

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(http.DefaultTransport), WithHooks(hooks))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if got := requestStartCount.Load(); got != 1 {
		t.Errorf("expected 1 OnRequestStart call (API leg only), got %d", got)
	}
	if got := requestEndCount.Load(); got != 1 {
		t.Errorf("expected 1 OnRequestEnd call (API leg only), got %d", got)
	}
}

func TestDownloadURL_OperationHooks(t *testing.T) {
	var opStartCalled, opEndCalled bool
	var capturedOp OperationInfo

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer apiServer.Close()

	hooks := &testHooks{
		onOperationStart: func(ctx context.Context, op OperationInfo) context.Context {
			opStartCalled = true
			capturedOp = op
			return ctx
		},
		onOperationEnd: func(ctx context.Context, op OperationInfo, err error, d time.Duration) {
			opEndCalled = true
		},
	}

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithHooks(hooks))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if !opStartCalled {
		t.Error("expected OnOperationStart to be called")
	}
	if !opEndCalled {
		t.Error("expected OnOperationEnd to be called")
	}
	if capturedOp.Service != "Account" {
		t.Errorf("expected Service 'Account', got %q", capturedOp.Service)
	}
	if capturedOp.Operation != "DownloadURL" {
		t.Errorf("expected Operation 'DownloadURL', got %q", capturedOp.Operation)
	}
	if capturedOp.ResourceType != "download" {
		t.Errorf("expected ResourceType 'download', got %q", capturedOp.ResourceType)
	}
	if capturedOp.IsMutation {
		t.Error("expected IsMutation to be false")
	}
}

func TestDownloadURL_GateRejection(t *testing.T) {
	var httpHit bool
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	hooks := &testGatingHooks{
		testHooks: testHooks{},
		onGate: func(ctx context.Context, op OperationInfo) (context.Context, error) {
			return ctx, ErrUsage("gate rejected")
		},
	}

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithHooks(hooks))
	ac := client.ForAccount("12345")

	_, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err == nil {
		t.Fatal("expected gate rejection error")
	}
	if httpHit {
		t.Error("expected no HTTP request after gate rejection")
	}
}

// --- UploadsService.Download regression: second leg assertions ---

func TestDownload_SecondLegNoAuth(t *testing.T) {
	var secondLegHit atomic.Bool
	var s3AuthHeader atomic.Value
	s3AuthHeader.Store("")

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondLegHit.Store(true)
		s3AuthHeader.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pixels"))
	}))
	defer s3Server.Close()

	mux := http.NewServeMux()
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	metadataBody, downloadPath := loadUploadFixture(t, apiServer.URL)
	mux.HandleFunc("/12345/uploads/1069479400",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(metadataBody)
		})
	mux.HandleFunc(downloadPath,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Location", s3Server.URL+"/bucket/file.png")
			w.WriteHeader(http.StatusFound)
		})

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(apiServer.Client().Transport))
	ac := client.ForAccount("12345")

	result, err := ac.Uploads().Download(context.Background(), 1069479400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if !secondLegHit.Load() {
		t.Fatal("second leg (signed URL fetch) was never reached")
	}
	if auth := s3AuthHeader.Load().(string); auth != "" {
		t.Errorf("expected no Authorization header on S3 request, got %q", auth)
	}
}

func TestDownload_SecondLegNoTimeout(t *testing.T) {
	// Verify the bare client used for signed downloads has no client-level timeout.
	// Use a fresh context.Background() so no preexisting deadline can confuse the check.
	var secondLegHit atomic.Bool

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondLegHit.Store(true)
		// Assert no deadline on the request context — proves Timeout: 0 on the bare client
		if _, hasDeadline := r.Context().Deadline(); hasDeadline {
			t.Error("expected no deadline on S3 request context (bare client should have Timeout: 0)")
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pixels"))
	}))
	defer s3Server.Close()

	mux := http.NewServeMux()
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	metadataBody, downloadPath := loadUploadFixture(t, apiServer.URL)
	mux.HandleFunc("/12345/uploads/1069479400",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(metadataBody)
		})
	mux.HandleFunc(downloadPath,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Location", s3Server.URL+"/bucket/file.png")
			w.WriteHeader(http.StatusFound)
		})

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(apiServer.Client().Transport))
	ac := client.ForAccount("12345")

	// Use context.Background() explicitly — no preexisting deadline
	result, err := ac.Uploads().Download(context.Background(), 1069479400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	if !secondLegHit.Load() {
		t.Fatal("second leg (signed URL fetch) was never reached — no-deadline assertion did not run")
	}
}

// The signed hop follows no redirect (SPEC §14 "Hop-2 Redirect Policy"). A
// storage host that answers the presigned GET with a 3xx is surfaced with that
// status, and the Location it names is never dialled — the body a third server
// would have returned must not reach the caller as if it were the file (#805).
// Before CheckRedirect was set on the bare client, net/http followed this
// chain and the caller received "SECRET".
func TestDownload_SecondLegRefusesRedirect(t *testing.T) {
	var thirdHits atomic.Int32
	thirdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thirdHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("SECRET"))
	}))
	defer thirdServer.Close()

	var signedHits atomic.Int32
	signedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signedHits.Add(1)
		w.Header().Set("Location", thirdServer.URL+"/elsewhere/file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer signedServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", signedServer.URL+"/bucket/signed-file.png")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithTransport(http.DefaultTransport))
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/photo.png")
	if err == nil {
		_ = result.Body.Close()
		t.Fatal("expected the signed hop's redirect to be refused, got a result")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != http.StatusFound {
		t.Errorf("expected HTTPStatus 302, got %d", apiErr.HTTPStatus)
	}
	if !strings.Contains(apiErr.Message, "not followed") {
		t.Errorf("expected the message to name the refusal, got %q", apiErr.Message)
	}
	if got := signedHits.Load(); got != 1 {
		t.Errorf("expected exactly one request to the signed host, got %d", got)
	}
	if got := thirdHits.Load(); got != 0 {
		t.Errorf("the redirect target was dialled %d time(s); hop 2 must not follow", got)
	}
}

// --- Auth-hop retry behavior ---

func TestDownloadURL_AuthHopRetriesOn503(t *testing.T) {
	fileContent := "retried-ok"
	var attempts atomic.Int32

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"try again"}`))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer apiServer.Close()

	baseDelay := 30 * time.Millisecond
	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithMaxRetries(3),
		WithBaseDelay(baseDelay),
		WithMaxJitter(time.Millisecond),
	)
	ac := client.ForAccount("12345")

	start := time.Now()
	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
	// Backoff between attempts: baseDelay then 2*baseDelay. Require at least 3*baseDelay
	// total so a regression that flattened the second delay back to baseDelay fails.
	if elapsed < 3*baseDelay {
		t.Errorf("expected elapsed >= %v, got %v", 3*baseDelay, elapsed)
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != fileContent {
		t.Errorf("expected body %q, got %q", fileContent, string(body))
	}
}

func TestDownloadURL_AuthHopRetriesOn429WithRetryAfter(t *testing.T) {
	fileContent := "rate-limit-ok"
	var attempts atomic.Int32

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Location", s3Server.URL+"/bucket/file.pdf")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	// Use a tiny BaseDelay so, if Retry-After is ignored, the test would finish
	// well under 1s and surface the regression.
	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithMaxRetries(3),
		WithBaseDelay(10*time.Millisecond),
		WithMaxJitter(time.Millisecond),
		WithTransport(http.DefaultTransport),
	)
	ac := client.ForAccount("12345")

	start := time.Now()
	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if got := attempts.Load(); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
	if elapsed < time.Second {
		t.Errorf("expected elapsed >= 1s (Retry-After), got %v", elapsed)
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != fileContent {
		t.Errorf("expected body %q, got %q", fileContent, string(body))
	}
}

// TestDownloadURL_RetryWaitChecksCancellationBeforeTheTimer is the download
// loop's copy of TestClient_RetryWaitChecksCancellationBeforeTheTimer: the same
// select, the same fire-OnRetry-then-wait order, and so the same coin flip when
// a hook cancels there and the delay has already elapsed. The reasoning — and
// why the contract can only be asserted over N runs rather than forced — is in
// that test; the assertion is the same: ctx.Err() itself, and the transport
// handed exactly one request.
func TestDownloadURL_RetryWaitChecksCancellationBeforeTheTimer(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer apiServer.Close()

	const runs = 64
	for run := range runs {
		ctx, cancel := context.WithCancel(context.Background())
		hooks := &cancelOnRetryHooks{cancel: cancel}
		cfg := DefaultConfig()
		cfg.BaseURL = apiServer.URL
		client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"},
			WithMaxRetries(3),
			// Zero backoff, so the timer is ready before the wait begins; see
			// the Client test for why that is the honest stand-in.
			WithBaseDelay(0),
			WithMaxJitter(time.Nanosecond),
			WithTransport(http.DefaultTransport),
			WithHooks(hooks),
		)
		ac := client.ForAccount("12345")

		_, err := ac.DownloadURL(ctx, "https://storage.3.basecamp.com/999/blobs/abc/download/file.png")
		cancel()

		if err != context.Canceled {
			t.Fatalf("run %d: DownloadURL returned %v, want ctx.Err() itself (context.Canceled) — "+
				"the loop went round again after the caller cancelled at the retry boundary", run, err)
		}
		if got := hooks.requests.Load(); got != 1 {
			t.Fatalf("run %d: the transport was handed %d requests, want 1 — "+
				"a cancellation delivered before the wait must not be followed by another attempt", run, got)
		}
	}
}

// flakyRoundTripper fails the first N RoundTrips with a synthetic network error,
// then delegates to inner. Used to prove network-error retry without relying on
// OS-level connection-reset semantics (which Go's transport sometimes retries
// on its own for idempotent requests).
type flakyRoundTripper struct {
	inner    http.RoundTripper
	failures atomic.Int32
	limit    int32
}

func (t *flakyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failures.Add(1) <= t.limit {
		return nil, errors.New("synthetic network failure")
	}
	return t.inner.RoundTrip(req)
}

func TestDownloadURL_AuthHopRetriesOnNetworkError(t *testing.T) {
	fileContent := "net-ok"
	var serverHits atomic.Int32

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer apiServer.Close()

	flaky := &flakyRoundTripper{inner: apiServer.Client().Transport, limit: 1}

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithTransport(flaky),
		WithMaxRetries(3),
		WithBaseDelay(10*time.Millisecond),
		WithMaxJitter(time.Millisecond),
	)
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if got := flaky.failures.Load(); got != 2 {
		t.Errorf("expected 2 RoundTrip calls, got %d", got)
	}
	if got := serverHits.Load(); got != 1 {
		t.Errorf("expected 1 successful request to reach the server, got %d", got)
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != fileContent {
		t.Errorf("expected body %q, got %q", fileContent, string(body))
	}
}

// TestDownloadURL_AuthHopDeclaredRetrySet pins the COMPLETE declared hop-1
// retry set (SPEC §14) status by status. The 503 and 429 cases above assert the
// timing properties (backoff growth, Retry-After); this table asserts
// membership, including the carve-out: 500 sits deliberately outside the set,
// so it is attempted exactly once and surfaces as a non-retryable API error.
func TestDownloadURL_AuthHopDeclaredRetrySet(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantAttempts int32
		wantErr      bool
	}{
		{"429 is retried", http.StatusTooManyRequests, 2, false},
		{"502 is retried", http.StatusBadGateway, 2, false},
		{"503 is retried", http.StatusServiceUnavailable, 2, false},
		{"504 is retried", http.StatusGatewayTimeout, 2, false},
		{"500 is outside the set", http.StatusInternalServerError, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileContent := "declared-set-ok"
			var attempts atomic.Int32

			s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/pdf")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(fileContent))
			}))
			defer s3Server.Close()

			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) == 1 {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(`{"error":"transient"}`))
					return
				}
				w.Header().Set("Location", s3Server.URL+"/bucket/file.pdf")
				w.WriteHeader(http.StatusFound)
			}))
			defer apiServer.Close()

			cfg := DefaultConfig()
			cfg.BaseURL = apiServer.URL
			token := &StaticTokenProvider{Token: "test-token"}
			client := NewClient(cfg, token,
				WithMaxRetries(3),
				WithBaseDelay(time.Millisecond),
				WithMaxJitter(time.Millisecond),
				WithTransport(http.DefaultTransport),
			)
			ac := client.ForAccount("12345")

			result, err := ac.DownloadURL(context.Background(),
				"https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf")

			if tt.wantErr {
				if err == nil {
					result.Body.Close()
					t.Fatalf("expected an error for %d", tt.status)
				}
				var sdkErr *Error
				if !isSDKError(err, &sdkErr) || sdkErr.Retryable {
					t.Errorf("expected a non-retryable SDK error, got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				defer result.Body.Close()
				body, _ := io.ReadAll(result.Body)
				if string(body) != fileContent {
					t.Errorf("expected body %q, got %q", fileContent, string(body))
				}
			}

			if got := attempts.Load(); got != tt.wantAttempts {
				t.Errorf("expected %d attempts, got %d", tt.wantAttempts, got)
			}
		})
	}
}

// TestDownloadURL_AuthHopAuthOnEveryAttemptNeverOnHop2 pins the credential
// boundary across a retry: every authenticated-hop attempt carries
// Authorization — the retried one included, since each attempt re-runs the auth
// strategy — and the signed hop never does.
func TestDownloadURL_AuthHopAuthOnEveryAttemptNeverOnHop2(t *testing.T) {
	var mu sync.Mutex
	var hop1Auth []string
	var hop2Auth []string
	var attempts atomic.Int32

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hop2Auth = append(hop2Auth, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hop1Auth = append(hop1Auth, r.Header.Get("Authorization"))
		mu.Unlock()
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Location", s3Server.URL+"/bucket/file.pdf")
		w.WriteHeader(http.StatusFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithMaxRetries(3),
		WithBaseDelay(time.Millisecond),
		WithMaxJitter(time.Millisecond),
		WithTransport(http.DefaultTransport),
	)
	ac := client.ForAccount("12345")

	result, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/doc.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()
	io.Copy(io.Discard, result.Body)

	mu.Lock()
	defer mu.Unlock()
	if len(hop1Auth) != 2 {
		t.Fatalf("expected 2 authenticated-hop attempts, got %d", len(hop1Auth))
	}
	for i, auth := range hop1Auth {
		if auth != "Bearer test-token" {
			t.Errorf("hop-1 attempt %d: expected %q, got %q", i+1, "Bearer test-token", auth)
		}
	}
	if len(hop2Auth) != 1 {
		t.Fatalf("expected 1 signed-hop request, got %d", len(hop2Auth))
	}
	if hop2Auth[0] != "" {
		t.Errorf("expected no Authorization header on the signed hop, got %q", hop2Auth[0])
	}
}

func TestDownloadURL_AuthHopNoRetryOn404(t *testing.T) {
	var attempts atomic.Int32

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithMaxRetries(3),
		WithBaseDelay(time.Millisecond),
	)
	ac := client.ForAccount("12345")

	_, err := ac.DownloadURL(context.Background(),
		"https://storage.3.basecamp.com/999/blobs/abc/download/file.txt")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var sdkErr *Error
	if !isSDKError(err, &sdkErr) || sdkErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("expected 1 attempt on 404, got %d", got)
	}
}

// --- test helpers ---

// isSDKError extracts an *Error from err via errors.As.
func isSDKError(err error, target **Error) bool {
	return errors.As(err, target)
}

// testHooks is a configurable Hooks implementation for testing.
type testHooks struct {
	onOperationStart func(ctx context.Context, op OperationInfo) context.Context
	onOperationEnd   func(ctx context.Context, op OperationInfo, err error, d time.Duration)
	onRequestStart   func(ctx context.Context, info RequestInfo) context.Context
	onRequestEnd     func(ctx context.Context, info RequestInfo, result RequestResult)
}

func (h *testHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	if h.onOperationStart != nil {
		return h.onOperationStart(ctx, op)
	}
	return ctx
}

func (h *testHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, d time.Duration) {
	if h.onOperationEnd != nil {
		h.onOperationEnd(ctx, op, err, d)
	}
}

func (h *testHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	if h.onRequestStart != nil {
		return h.onRequestStart(ctx, info)
	}
	return ctx
}

func (h *testHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	if h.onRequestEnd != nil {
		h.onRequestEnd(ctx, info, result)
	}
}

func (h *testHooks) OnRetry(context.Context, RequestInfo, int, error) {}

// testGatingHooks extends testHooks with gating.
type testGatingHooks struct {
	testHooks
	onGate func(ctx context.Context, op OperationInfo) (context.Context, error)
}

func (h *testGatingHooks) OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error) {
	if h.onGate != nil {
		return h.onGate(ctx, op)
	}
	return ctx, nil
}
