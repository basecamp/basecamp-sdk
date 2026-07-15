package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// These tests drive the shared, data-only fixtures in
// conformance/oauth/fixtures with this harness's mock origins substituted for
// the {{...}} placeholders, so issuer / resource binding stays code-point-exact
// against the mocked hosts. See conformance/oauth/README.md.

type fixtureHop struct {
	Origin         string          `json:"origin"`
	Status         int             `json:"status"`
	TransportError bool            `json:"transportError"`
	Body           json.RawMessage `json:"body"`
	Oversized      bool            `json:"oversized"`
	RedirectTo     string          `json:"redirectTo"`
}

type fixtureExpect struct {
	Outcome            string `json:"outcome"`
	SelectedIssuer     string `json:"selectedIssuer"`
	FallbackReason     string `json:"fallbackReason"`
	Error              string `json:"error"`
	ErrorCategory      string `json:"errorCategory"`
	LaunchpadContacted *bool  `json:"launchpadContacted"`
}

type fixture struct {
	Name           string        `json:"name"`
	Operation      string        `json:"operation"`
	ResourceOrigin string        `json:"resourceOrigin"`
	IssuerOrigin   string        `json:"issuerOrigin"`
	ExpectedIssuer string        `json:"expectedIssuer"`
	Hop1           *fixtureHop   `json:"hop1"`
	Hop2           *fixtureHop   `json:"hop2"`
	Expect         fixtureExpect `json:"expect"`
}

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../go/pkg/basecamp/oauth/<file> → repo root is four levels up.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(root, "conformance", "oauth", "fixtures")
}

// hopResponder serves a single configured mock exchange for a role's server.
type hopResponder struct {
	mu  sync.Mutex
	hop *fixtureHop
}

func (h *hopResponder) set(hop *fixtureHop) {
	h.mu.Lock()
	h.hop = hop
	h.mu.Unlock()
}

func (h *hopResponder) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	hop := h.hop
	h.mu.Unlock()

	if hop == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if hop.Oversized {
		// A body far larger than any test cap; the bounded read must abort
		// before buffering it all.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 256*1024))
		return
	}
	if hop.RedirectTo != "" {
		status := hop.Status
		if status == 0 {
			status = http.StatusFound
		}
		w.Header().Set("Location", hop.RedirectTo)
		w.WriteHeader(status)
		return
	}
	status := hop.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if hop.Body != nil {
		_, _ = w.Write(hop.Body)
	}
}

// countingTransport counts requests to the real Launchpad host and answers them
// with canned metadata (so a genuine fallback would succeed), while forwarding
// everything else to the base transport. Hard cases must never reach Launchpad.
type countingTransport struct {
	base  http.RoundTripper
	count *int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.EqualFold(req.URL.Hostname(), "launchpad.37signals.com") {
		*t.count++
		body := `{"issuer":"https://launchpad.37signals.com","authorization_endpoint":"https://launchpad.37signals.com/authorization/new","token_endpoint":"https://launchpad.37signals.com/authorization/token"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    req,
		}, nil
	}
	return t.base.RoundTrip(req)
}

func sentinelFor(name string) error {
	switch name {
	case "ambiguous_issuers":
		return ErrAmbiguousIssuers
	case "expected_issuer_unavailable":
		return ErrExpectedIssuerUnavailable
	case "invalid_issuer_origin":
		return ErrInvalidIssuerOrigin
	case "as_fetch_failed":
		return ErrASFetchFailed
	case "issuer_mismatch":
		return ErrIssuerMismatch
	case "capability_unavailable":
		return ErrCapabilityUnavailable
	}
	return nil
}

func TestResourceFirstDiscoveryFixtures(t *testing.T) {
	dir := fixtureDir(t)
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
			runFixture(t, filepath.Join(dir, name))
		})
	}
}

func runFixture(t *testing.T, path string) {
	t.Helper()

	rawBytes, err := os.ReadFile(path) // #nosec G304 -- test fixture path
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	// Role servers. Created up front so their URLs are known before placeholder
	// substitution; unreferenced servers are simply never contacted.
	resourceResp := &hopResponder{}
	bc5Resp := &hopResponder{}
	issuerResp := &hopResponder{}
	resourceSrv := httptest.NewServer(resourceResp)
	defer resourceSrv.Close()
	bc5Srv := httptest.NewServer(bc5Resp)
	defer bc5Srv.Close()
	issuerSrv := httptest.NewServer(issuerResp)
	defer issuerSrv.Close()

	replacements := map[string]string{
		"{{RESOURCE_ORIGIN}}":  resourceSrv.URL,
		"{{BC5_ISSUER}}":       bc5Srv.URL,
		"{{ISSUER_ORIGIN}}":    issuerSrv.URL,
		"{{LAUNCHPAD_ORIGIN}}": LaunchpadBaseURL,
	}
	subbed := string(rawBytes)
	for ph, origin := range replacements {
		subbed = strings.ReplaceAll(subbed, ph, origin)
	}

	var fx fixture
	if err := json.Unmarshal([]byte(subbed), &fx); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", path, err)
	}

	// Bracketed IPv6 origins can't be served by httptest (it listens on
	// 127.0.0.1), so the IPv6 origin-root accept case is verified at the parser
	// boundary — the point of the fixture is that the transport parser accepts it
	// where a regex would fail.
	if strings.Contains(fx.ResourceOrigin, "[") && fx.Expect.Outcome == "selected" {
		got, err := requireOriginRoot(fx.ResourceOrigin, "resource origin")
		if err != nil {
			t.Fatalf("requireOriginRoot(%q) unexpected error: %v", fx.ResourceOrigin, err)
		}
		if got != fx.Expect.SelectedIssuer {
			t.Errorf("requireOriginRoot(%q) = %q, want %q", fx.ResourceOrigin, got, fx.Expect.SelectedIssuer)
		}
		return
	}

	// Wire up mock responses.
	oversized := false
	if fx.Hop1 != nil {
		resourceResp.set(fx.Hop1)
		oversized = oversized || fx.Hop1.Oversized
	}
	if fx.Hop2 != nil {
		switch fx.Hop2.Origin {
		case bc5Srv.URL:
			bc5Resp.set(fx.Hop2)
		case issuerSrv.URL:
			issuerResp.set(fx.Hop2)
		default:
			t.Fatalf("hop2 origin %q matches no role server", fx.Hop2.Origin)
		}
		oversized = oversized || fx.Hop2.Oversized
	}
	// A transport-level failure: close the resource server so the connection is
	// refused.
	if fx.Hop1 != nil && fx.Hop1.TransportError {
		resourceSrv.Close()
	}

	launchpadHits := 0
	client := &http.Client{Transport: &countingTransport{base: http.DefaultTransport, count: &launchpadHits}}
	d := NewDiscoverer(client)

	opts := []DiscoverOption{}
	if fx.ExpectedIssuer != "" {
		opts = append(opts, WithExpectedIssuer(fx.ExpectedIssuer))
	}
	if oversized {
		opts = append(opts, WithMaxBodyBytes(8*1024))
	}

	ctx := context.Background()
	var runErr error
	var result *DiscoveryResult
	switch fx.Operation {
	case "discoverFromResource":
		result, runErr = d.DiscoverFromResource(ctx, fx.ResourceOrigin, opts...)
	case "discoverProtectedResource":
		_, runErr = d.DiscoverProtectedResource(ctx, fx.ResourceOrigin, opts...)
	case "discover":
		_, runErr = d.Discover(ctx, fx.IssuerOrigin, opts...)
	default:
		t.Fatalf("unknown operation %q", fx.Operation)
	}

	switch fx.Expect.Outcome {
	case "raise":
		if runErr == nil {
			t.Fatalf("expected an error, got nil")
		}
		assertRaise(t, fx, runErr)
	case "fallback":
		if runErr != nil {
			t.Fatalf("expected fallback, got error: %v", runErr)
		}
		if result == nil || !result.IsFallback() {
			t.Fatalf("expected fallback result, got %+v", result)
		}
		if string(result.FallbackReason) != fx.Expect.FallbackReason {
			t.Errorf("fallback reason = %q, want %q", result.FallbackReason, fx.Expect.FallbackReason)
		}
		// A soft fallback hands off to the caller, whose next step is Launchpad
		// (oauth.LaunchpadBaseURL). Drive that hand-off so a launchpadContacted:true
		// fixture asserts the fallback path actually reaches Launchpad — the
		// counting transport serves canned metadata, so a genuine fallback succeeds.
		if fx.Expect.LaunchpadContacted != nil && *fx.Expect.LaunchpadContacted {
			if _, err := d.DiscoverLaunchpad(ctx); err != nil {
				t.Fatalf("Launchpad fallback discovery failed: %v", err)
			}
			if launchpadHits == 0 {
				t.Errorf("expected Launchpad to be contacted on fallback, but it was not")
			}
		}
	case "selected":
		if runErr != nil {
			t.Fatalf("expected selected, got error: %v", runErr)
		}
		if fx.Operation == "discoverFromResource" {
			if result == nil || result.IsFallback() || result.Config == nil {
				t.Fatalf("expected selected config, got %+v", result)
			}
			if fx.Expect.SelectedIssuer != "" && result.Issuer != fx.Expect.SelectedIssuer {
				t.Errorf("selected issuer = %q, want %q", result.Issuer, fx.Expect.SelectedIssuer)
			}
		}
		// discover / discoverProtectedResource: absence of an error is success.
	default:
		t.Fatalf("unknown expected outcome %q", fx.Expect.Outcome)
	}

	if fx.Expect.LaunchpadContacted != nil && !*fx.Expect.LaunchpadContacted {
		if launchpadHits != 0 {
			t.Errorf("Launchpad was contacted %d time(s); hard/selected case must not touch Launchpad", launchpadHits)
		}
	}
}

func TestRequireOriginRoot(t *testing.T) {
	accept := []struct{ in, want string }{
		{"https://api.example.com", "https://api.example.com"},
		{"https://api.example.com/", "https://api.example.com"},
		{"https://api.example.com:8443", "https://api.example.com:8443"},
		{"https://api.example.com:443", "https://api.example.com"}, // default port dropped
		{"https://h:443", "https://h"},                             // in-range port accepted
		{"http://localhost:3000", "http://localhost:3000"},
		{"http://[::1]:3000", "http://[::1]:3000"},
		{"http://127.0.0.1:9999", "http://127.0.0.1:9999"},
	}
	for _, tc := range accept {
		got, err := requireOriginRoot(tc.in, "origin")
		if err != nil {
			t.Errorf("requireOriginRoot(%q) error = %v, want nil", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("requireOriginRoot(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	reject := []string{
		"http://api.example.com",            // plain http, non-localhost
		"https://api.example.com:notaport",  // malformed port
		"https://h:99999",                   // numeric but out-of-range port
		"https://h:0",                       // port below the valid range
		"http://[::1]:notaport",             // malformed IPv6 port
		"https://api.example.com/tenant/1",  // path beyond "/"
		"https://api.example.com?x=1",       // query
		"https://api.example.com#frag",      // fragment
		"https://user:pass@api.example.com", // userinfo
		"ftp://api.example.com",             // non-http(s) scheme
		"not a url",                         // unparseable
	}
	for _, in := range reject {
		if _, err := requireOriginRoot(in, "origin"); err == nil {
			t.Errorf("requireOriginRoot(%q) = nil error, want usage error", in)
		} else {
			var be *basecamp.Error
			if !errors.As(err, &be) || be.Code != basecamp.CodeUsage {
				t.Errorf("requireOriginRoot(%q) error = %v, want usage", in, err)
			}
		}
	}
}

func TestDiscover_DeviceOnlyAS(t *testing.T) {
	var origin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issuer":"` + origin + `","token_endpoint":"` + origin + `/oauth/token",` +
			`"device_authorization_endpoint":"` + origin + `/oauth/device",` +
			`"grant_types_supported":["urn:ietf:params:oauth:grant-type:device_code","refresh_token"]}`))
	}))
	defer srv.Close()
	origin = srv.URL

	cfg, err := NewDiscoverer(srv.Client()).Discover(context.Background(), origin)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if cfg.AuthorizationEndpoint != nil {
		t.Errorf("AuthorizationEndpoint = %v, want nil (device-only AS omits it)", *cfg.AuthorizationEndpoint)
	}
	if cfg.DeviceAuthorizationEndpoint == nil || *cfg.DeviceAuthorizationEndpoint != origin+"/oauth/device" {
		t.Errorf("DeviceAuthorizationEndpoint = %v, want %q", cfg.DeviceAuthorizationEndpoint, origin+"/oauth/device")
	}
	found := false
	for _, g := range cfg.GrantTypesSupported {
		if g == "urn:ietf:params:oauth:grant-type:device_code" {
			found = true
		}
	}
	if !found {
		t.Errorf("GrantTypesSupported = %v, want device_code", cfg.GrantTypesSupported)
	}
}

func codeForCategory(category string) string {
	switch category {
	case "usage":
		return basecamp.CodeUsage
	case "validation":
		return basecamp.CodeValidation
	case "api_error":
		return basecamp.CodeAPI
	case "network":
		return basecamp.CodeNetwork
	case "auth_required":
		return basecamp.CodeAuth
	default:
		return category
	}
}

func assertRaise(t *testing.T, fx fixture, err error) {
	t.Helper()

	// Cross-SDK coarse-category assertion: the thrown error must map to the
	// fixture's errorCategory via the shared basecamp.Error taxonomy.
	if fx.Expect.ErrorCategory != "" {
		var bce *basecamp.Error
		if !errors.As(err, &bce) {
			t.Fatalf("no basecamp.Error in chain for %v (%T)", err, err)
		}
		if want := codeForCategory(fx.Expect.ErrorCategory); bce.Code != want {
			t.Errorf("error category = %q, want %q (%v)", bce.Code, want, err)
		}
	}

	var be *basecamp.Error
	if fx.Expect.Error == "usage" {
		if !errors.As(err, &be) || be.Code != basecamp.CodeUsage {
			t.Errorf("expected usage error, got %v (%T)", err, err)
		}
		return
	}

	if fx.Operation == "discoverFromResource" {
		var se *SelectionError
		if !errors.As(err, &se) {
			t.Fatalf("expected *SelectionError, got %v (%T)", err, err)
		}
		want := sentinelFor(fx.Expect.Error)
		if want == nil {
			t.Fatalf("no sentinel mapped for error %q", fx.Expect.Error)
		}
		if !errors.Is(err, want) {
			t.Errorf("error %v is not %v", err, want)
		}
		return
	}

	// discover / discoverProtectedResource hard failures are api_error
	// (invalid_metadata and api_error both map to the api_error code).
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Errorf("expected api_error, got %v (%T)", err, err)
	}
}

// TestSelectionError_PreservesCauseStatus guards finding B: a SelectionError
// wrapping an AS 5xx (or a network cause) must expose the cause's HTTP status and
// retryability to an errors.As(&basecamp.Error) consumer, not a stripped
// {Code, Message} view — while keeping the taxonomy Code and errors.Is sentinel
// matching intact.
func TestSelectionError_PreservesCauseStatus(t *testing.T) {
	cause := &basecamp.Error{Code: basecamp.CodeAPI, Message: "boom", HTTPStatus: 503, Retryable: true, RequestID: "req-1"}
	se := newSelectionError(ErrASFetchFailed, "authorization server metadata fetch failed", cause)

	var be *basecamp.Error
	if !errors.As(se, &be) {
		t.Fatal("expected a *basecamp.Error in the chain")
	}
	if be.Code != basecamp.CodeAPI {
		t.Errorf("Code = %q, want %q", be.Code, basecamp.CodeAPI)
	}
	if be.HTTPStatus != 503 {
		t.Errorf("HTTPStatus = %d, want 503 (cause status must not be stripped)", be.HTTPStatus)
	}
	if !be.Retryable {
		t.Error("Retryable = false, want true (cause retryability must not be stripped)")
	}
	if be.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want %q", be.RequestID, "req-1")
	}
	if !errors.Is(se, ErrASFetchFailed) {
		t.Error("errors.Is(se, ErrASFetchFailed) = false, want true")
	}
}

// TestSelectionError_NoCauseStillCoded confirms that when there is no cause
// carrying a *basecamp.Error (e.g. an ambiguous-issuer selection), the
// taxonomy-coded view is still present with the right Code and a zero status.
func TestSelectionError_NoCauseStillCoded(t *testing.T) {
	se := newSelectionError(ErrAmbiguousIssuers, "multiple non-Launchpad issuers advertised", nil)

	var be *basecamp.Error
	if !errors.As(se, &be) {
		t.Fatal("expected a *basecamp.Error in the chain")
	}
	if be.Code != basecamp.CodeAPI {
		t.Errorf("Code = %q, want %q", be.Code, basecamp.CodeAPI)
	}
	if be.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0", be.HTTPStatus)
	}
	if !errors.Is(se, ErrAmbiguousIssuers) {
		t.Error("errors.Is(se, ErrAmbiguousIssuers) = false, want true")
	}
}

// TestDiscoverFromResource_ASFetchFailurePreservesStatus is the end-to-end form
// of finding B: a committed BC5 issuer whose AS metadata hop returns 503 must
// surface that status through DiscoverFromResource's *SelectionError.
func TestDiscoverFromResource_ASFetchFailurePreservesStatus(t *testing.T) {
	bc5 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
	}))
	defer bc5.Close()

	var resourceOrigin string
	resource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource":"` + resourceOrigin + `","authorization_servers":["` + bc5.URL + `"]}`))
	}))
	defer resource.Close()
	resourceOrigin = resource.URL

	_, err := NewDiscoverer(resource.Client()).DiscoverFromResource(context.Background(), resourceOrigin)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrASFetchFailed) {
		t.Errorf("errors.Is(err, ErrASFetchFailed) = false; err = %v", err)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) {
		t.Fatalf("expected *basecamp.Error in chain, got %T", err)
	}
	if be.HTTPStatus != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus = %d, want %d", be.HTTPStatus, http.StatusServiceUnavailable)
	}
	if be.Code != basecamp.CodeAPI {
		t.Errorf("Code = %q, want api_error", be.Code)
	}
}
