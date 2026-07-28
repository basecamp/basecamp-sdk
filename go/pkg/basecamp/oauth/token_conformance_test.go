package oauth

// Drives the shared, data-only fixtures in conformance/oauth-token/fixtures:
// one refresh round-trip per fixture, asserting the sent resource form
// parameter and the response decode (round-trip, absent/null as unset,
// present-empty/non-string rejected). Lifecycle preservation across a stored
// credential is per-manager behavior, tested in auth_test.go — not here.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type tokenFixture struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Operation   string `json:"operation"`
	Request     struct {
		Resource string `json:"resource"`
	} `json:"request"`
	Response struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	} `json:"response"`
	Expect struct {
		Outcome            string  `json:"outcome"`
		Resource           *string `json:"resource"`
		ResourceAbsent     bool    `json:"resourceAbsent"`
		FormResource       *string `json:"formResource"`
		FormResourceAbsent bool    `json:"formResourceAbsent"`
	} `json:"expect"`
}

func tokenFixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(root, "conformance", "oauth-token", "fixtures")
}

func TestOAuthTokenFixtures(t *testing.T) {
	dir := tokenFixtureDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading fixture dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("no fixtures found in %s", dir)
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name)) // #nosec G304 -- test fixture path
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			var fx tokenFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parsing fixture: %v", err)
			}
			if fx.Operation != "refreshToken" {
				t.Fatalf("unsupported operation %q", fx.Operation)
			}

			var sawResourceKey bool
			var sentResource string
			status := fx.Response.Status
			if status == 0 {
				status = http.StatusOK
			}
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				_, sawResourceKey = r.PostForm["resource"]
				sentResource = r.PostFormValue("resource")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write(fx.Response.Body)
			}))
			defer srv.Close()

			e := NewExchanger(srv.Client())
			token, err := e.Refresh(context.Background(), RefreshRequest{
				TokenEndpoint: srv.URL,
				RefreshToken:  "refresh-token",
				ClientID:      "basecamp-cli",
				Resource:      fx.Request.Resource,
			})

			switch fx.Expect.Outcome {
			case "token":
				if err != nil {
					t.Fatalf("expected token, got error: %v", err)
				}
				if fx.Expect.Resource != nil && token.Resource != *fx.Expect.Resource {
					t.Errorf("token.Resource = %q, want %q", token.Resource, *fx.Expect.Resource)
				}
				if fx.Expect.ResourceAbsent && token.Resource != "" {
					t.Errorf("token.Resource = %q, want unset", token.Resource)
				}
			case "reject":
				if err == nil {
					t.Fatal("expected rejection, got token")
				}
			default:
				t.Fatalf("unsupported outcome %q", fx.Expect.Outcome)
			}

			if fx.Expect.FormResource != nil {
				if !sawResourceKey || sentResource != *fx.Expect.FormResource {
					t.Errorf("form resource = %q (present=%v), want %q", sentResource, sawResourceKey, *fx.Expect.FormResource)
				}
			}
			if fx.Expect.FormResourceAbsent && sawResourceKey {
				t.Errorf("form resource key present, want absent")
			}
		})
	}
}
