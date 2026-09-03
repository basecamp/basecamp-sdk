package oauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// recordingSleep is an injectable sleep seam that records requested waits and
// returns immediately, so tests exercise the interval schedule without delay.
type recordingSleep struct {
	waits []time.Duration
	// before, when set, runs before each recorded wait (e.g. to cancel a ctx).
	before func()
}

func (r *recordingSleep) fn(_ context.Context, d time.Duration) error {
	if r.before != nil {
		r.before()
	}
	r.waits = append(r.waits, d)
	return nil
}

// queueTokenResponses serves a fixed sequence of token-endpoint responses, one
// per poll (the last response repeats). It returns a pointer to the call count.
func queueTokenResponses(t *testing.T, responses []struct {
	status int
	body   map[string]any
}) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := calls
		if i >= len(responses) {
			i = len(responses) - 1
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responses[i].status)
		_ = json.NewEncoder(w).Encode(responses[i].body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// tlsClient returns an HTTP client that trusts the given test TLS server.
func tlsClient(srv *httptest.Server) *http.Client {
	return srv.Client()
}

const testDeviceCode = "dev-code-123"

var deviceAuthBody = map[string]any{
	"device_code":               testDeviceCode,
	"user_code":                 "WDJB-MJHT",
	"verification_uri":          "https://issuer.example/device",
	"verification_uri_complete": "https://issuer.example/device?user_code=WDJB-MJHT",
	"expires_in":                900,
	"interval":                  5,
}

var tokenBody = map[string]any{
	"access_token":  "device_access_token",
	"refresh_token": "device_refresh_token",
	"token_type":    "Bearer",
	"expires_in":    3600,
}

func TestRequestDeviceAuthorization_CompletionAfterCancelIsCancelled(t *testing.T) {
	// A custom RoundTripper that ignores the request context can complete a
	// VALID response after the caller cancelled. The success path must re-check
	// the parent context before handing back a usable device code — never a
	// device authorization returned after the caller asked to stop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	body, err := json.Marshal(deviceAuthBody)
	if err != nil {
		t.Fatalf("marshaling fixture: %v", err)
	}
	client := &http.Client{Transport: completeAfterCancelTransport{cancel: cancel, body: string(body)}}
	auth, err := RequestDeviceAuthorization(ctx, "https://device.example.com/oauth/device", "basecamp-cli",
		WithDeviceHTTPClient(client))
	if auth != nil {
		t.Fatal("device authorization must not be returned after cancellation")
	}
	var dfErr *DeviceFlowError
	if !errors.As(err, &dfErr) {
		t.Fatalf("expected *DeviceFlowError, got %T: %v", err, err)
	}
	if dfErr.Reason != DeviceFlowCancelled {
		t.Errorf("Reason = %v, want DeviceFlowCancelled", dfErr.Reason)
	}
}

func TestRequestDeviceAuthorization_CompletionAfterCancelBeatsAPIError(t *testing.T) {
	// Cancellation must also win over a completed NON-2XX: a context-ignoring
	// RoundTripper that cancels the caller and then returns a 500 must surface
	// cancelled, not the api_error classification.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &http.Client{Transport: completeAfterCancelStatusTransport{cancel: cancel, status: http.StatusInternalServerError}}
	_, err := RequestDeviceAuthorization(ctx, "https://device.example.com/oauth/device", "basecamp-cli",
		WithDeviceHTTPClient(client))
	var dfErr *DeviceFlowError
	if !errors.As(err, &dfErr) {
		t.Fatalf("expected *DeviceFlowError, got %T: %v", err, err)
	}
	if dfErr.Reason != DeviceFlowCancelled {
		t.Errorf("Reason = %v, want DeviceFlowCancelled", dfErr.Reason)
	}
}

func TestRequestDeviceAuthorization_OmitsScopeAndValidates(t *testing.T) {
	var sentScope string
	var scopePresent bool
	var sentClientID string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sentScope = r.PostForm.Get("scope")
		_, scopePresent = r.PostForm["scope"]
		sentClientID = r.PostForm.Get("client_id")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer srv.Close()

	auth, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scopePresent {
		t.Errorf("scope should be omitted when unset, got %q", sentScope)
	}
	if sentClientID != "basecamp-cli" {
		t.Errorf("client_id = %q, want basecamp-cli", sentClientID)
	}
	if auth.DeviceCode != testDeviceCode {
		t.Errorf("DeviceCode = %q", auth.DeviceCode)
	}
	if auth.UserCode != "WDJB-MJHT" {
		t.Errorf("UserCode = %q", auth.UserCode)
	}
	if auth.Interval != 5 {
		t.Errorf("Interval = %d, want 5", auth.Interval)
	}
}

func TestRequestDeviceAuthorization_VerificationURICompleteOptional(t *testing.T) {
	// Present → a non-nil pointer to the value; absent → nil (not ""), preserving
	// the optional distinction the cross-SDK contract requires.
	present := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer present.Close()
	auth, err := RequestDeviceAuthorization(context.Background(), present.URL, "basecamp-cli", WithDeviceHTTPClient(tlsClient(present)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.VerificationURIComplete == nil {
		t.Fatal("VerificationURIComplete = nil, want non-nil when present")
	}
	if *auth.VerificationURIComplete != "https://issuer.example/device?user_code=WDJB-MJHT" {
		t.Errorf("*VerificationURIComplete = %q", *auth.VerificationURIComplete)
	}

	body := map[string]any{}
	for k, v := range deviceAuthBody {
		if k != "verification_uri_complete" {
			body[k] = v
		}
	}
	absent := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer absent.Close()
	auth2, err := RequestDeviceAuthorization(context.Background(), absent.URL, "basecamp-cli", WithDeviceHTTPClient(tlsClient(absent)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth2.VerificationURIComplete != nil {
		t.Errorf("VerificationURIComplete = %v, want nil when absent", auth2.VerificationURIComplete)
	}
}

func TestRequestDeviceAuthorization_CallerCancellationIsCancelled(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer srv.Close()

	// A context already cancelled before the request: Do returns context.Canceled
	// without contacting the server, so the outcome must be DeviceFlowCancelled —
	// never a retryable transport failure (the SDK's own timeout stays transport).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RequestDeviceAuthorization(ctx, srv.URL, "basecamp-cli", WithDeviceHTTPClient(tlsClient(srv)))
	var dfErr *DeviceFlowError
	if !errors.As(err, &dfErr) {
		t.Fatalf("expected *DeviceFlowError, got %T: %v", err, err)
	}
	if dfErr.Reason != DeviceFlowCancelled {
		t.Errorf("Reason = %v, want DeviceFlowCancelled", dfErr.Reason)
	}
}

func TestRequestDeviceAuthorization_CancellationDuringBodyReadIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The body cancels ctx on its first read then errors, so the abort lands during
	// readBoundedBody (not at Do). The outcome must still be DeviceFlowCancelled.
	client := &http.Client{Transport: cancelOnReadTransport{cancel: cancel}}
	_, err := RequestDeviceAuthorization(ctx, "https://device.example.com/oauth/device", "basecamp-cli",
		WithDeviceHTTPClient(client))
	var dfErr *DeviceFlowError
	if !errors.As(err, &dfErr) {
		t.Fatalf("expected *DeviceFlowError, got %T: %v", err, err)
	}
	if dfErr.Reason != DeviceFlowCancelled {
		t.Errorf("Reason = %v, want DeviceFlowCancelled", dfErr.Reason)
	}
}

func TestRequestDeviceAuthorization_SendsScopeWhenSet(t *testing.T) {
	var sentScope string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sentScope = r.PostForm.Get("scope")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceScope("read write"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentScope != "read write" {
		t.Errorf("scope = %q, want %q", sentScope, "read write")
	}
}

func TestRequestDeviceAuthorization_SendsLoginHintWhenSet(t *testing.T) {
	var form url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceLoginHint("bot@example.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := form.Get("login_hint"); got != "bot@example.com" {
		t.Errorf("login_hint = %q, want %q", got, "bot@example.com")
	}

	_, err = RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := form["login_hint"]; present {
		t.Errorf("login_hint must be omitted when no hint is set, got %q", form.Get("login_hint"))
	}
}

func TestRequestDeviceAuthorization_DefaultsIntervalTo5(t *testing.T) {
	body := map[string]any{}
	for k, v := range deviceAuthBody {
		body[k] = v
	}
	delete(body, "interval")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	auth, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Interval != 5 {
		t.Errorf("Interval = %d, want 5 (default)", auth.Interval)
	}
}

func TestRequestDeviceAuthorization_RejectsNonPositiveExpiresIn(t *testing.T) {
	body := map[string]any{}
	for k, v := range deviceAuthBody {
		body[k] = v
	}
	body["expires_in"] = 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	assertBasecampCode(t, err, basecamp.CodeAPI)
}

func TestRequestDeviceAuthorization_RejectsMissingField(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user_code":        "X",
			"verification_uri": "https://issuer.example",
			"expires_in":       900,
		})
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	assertBasecampCode(t, err, basecamp.CodeAPI)
	// A malformed 2xx body's validation error carries the status, like the parse
	// failure and the token-poll raises.
	var be *basecamp.Error
	if !errors.As(err, &be) || be.HTTPStatus != http.StatusOK {
		t.Errorf("validation error should carry HTTPStatus=200, got %+v", err)
	}
}

func TestRequestDeviceAuthorization_ParseFailureCarriesHTTPStatus(t *testing.T) {
	// A 2xx body that is not valid JSON fails as api_error AND carries the HTTP
	// status (like the non-2xx raise and Python), so callers keep the status.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Fatalf("want api_error, got %v (%T)", err, err)
	}
	if be.HTTPStatus != http.StatusOK {
		t.Errorf("HTTPStatus = %d, want %d", be.HTTPStatus, http.StatusOK)
	}
}

func TestRequestDeviceAuthorization_AcceptsIntegerValuedFloatDurations(t *testing.T) {
	// A server sending 900.0 / 10.0 (integer-valued floats): *int decoding would
	// reject these, but the cross-SDK contract accepts whole-second floats.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":900.0,"interval":10.0}`))
	}))
	defer srv.Close()

	auth, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.ExpiresIn != 900 || auth.Interval != 10 {
		t.Errorf("ExpiresIn=%d Interval=%d, want 900/10", auth.ExpiresIn, auth.Interval)
	}
}

func TestRequestDeviceAuthorization_RejectsFractionalDurations(t *testing.T) {
	for _, body := range []string{
		`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":0.5}`,
		`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":900,"interval":2.5}`,
	} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
			WithDeviceHTTPClient(tlsClient(srv)))
		assertBasecampCode(t, err, basecamp.CodeAPI)
		srv.Close()
	}
}

func TestRequestDeviceAuthorization_RejectsOversizedDurations(t *testing.T) {
	// 1e100 is integer-valued, so whole-second checking alone would admit it —
	// and its int conversion is implementation-defined. The shared cross-SDK
	// ceiling (2147483 s) rejects it, and the first value past the boundary,
	// as api_error before any deadline arithmetic.
	for _, body := range []string{
		`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":1e100}`,
		`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":900,"interval":1e100}`,
		`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":2147484}`,
	} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
			WithDeviceHTTPClient(tlsClient(srv)))
		assertBasecampCode(t, err, basecamp.CodeAPI)
		srv.Close()
	}
}

func TestRequestDeviceAuthorization_AcceptsMaxDuration(t *testing.T) {
	// The 2147483 s ceiling itself is valid — the bound is inclusive.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"d","user_code":"u","verification_uri":"https://issuer.example/device","expires_in":2147483,"interval":2147483}`))
	}))
	defer srv.Close()

	auth, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(tlsClient(srv)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.ExpiresIn != maxDeviceSeconds || auth.Interval != maxDeviceSeconds {
		t.Errorf("ExpiresIn=%d Interval=%d, want %d/%d", auth.ExpiresIn, auth.Interval, maxDeviceSeconds, maxDeviceSeconds)
	}
}

func TestRequestDeviceAuthorization_RejectsInsecureEndpoint(t *testing.T) {
	_, err := RequestDeviceAuthorization(context.Background(), "http://insecure.example/device", "basecamp-cli")
	assertBasecampCode(t, err, basecamp.CodeUsage)
}

func TestRequestDeviceAuthorization_RequiresClientID(t *testing.T) {
	_, err := RequestDeviceAuthorization(context.Background(), "https://issuer.example/device", "")
	assertBasecampCode(t, err, basecamp.CodeValidation)
}

func TestPollDeviceToken_RejectsOutOfRangeDurations(t *testing.T) {
	// The exported entry point sanity-checks its duration inputs: a non-positive
	// value builds a past deadline and an oversized one overflows the internal
	// time.Duration(seconds) * time.Second math. Both surface as a usage error
	// before any request, mirroring the TS pollDeviceToken guard.
	cases := []struct {
		name              string
		interval, expires int
	}{
		{"expires zero", 5, 0},
		{"expires negative", 5, -1},
		{"expires oversized", 5, maxDeviceSeconds + 1},
		{"interval zero", 0, 900},
		{"interval negative", -1, 900},
		{"interval oversized", maxDeviceSeconds + 1, 900},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PollDeviceToken(context.Background(), "https://issuer.example/token",
				"basecamp-cli", testDeviceCode, tc.interval, tc.expires)
			assertBasecampCode(t, err, basecamp.CodeUsage)
		})
	}
}

func TestPollDeviceToken_PendingSlowDownToken(t *testing.T) {
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
		{http.StatusBadRequest, map[string]any{"error": "slow_down"}},
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
		{http.StatusOK, tokenBody},
	})
	sleep := &recordingSleep{}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	// Waits: 5s (pending), 5s (before slow_down), then +5 sustained → 10s, 10s.
	want := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 10 * time.Second}
	assertWaits(t, sleep.waits, want)
}

func TestPollDeviceToken_DoublesIntervalAfterTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()
	sleep := &recordingSleep{}

	// First attempt returns a network timeout; the rest hit the real server.
	base := tlsClient(srv)
	client := &http.Client{Transport: &timeoutOnceTransport{next: base.Transport}}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	if len(sleep.waits) < 2 {
		t.Fatalf("expected at least 2 waits, got %v", sleep.waits)
	}
	if sleep.waits[0] != 5*time.Second {
		t.Errorf("waits[0] = %v, want 5s", sleep.waits[0])
	}
	if sleep.waits[1] != 10*time.Second {
		t.Errorf("waits[1] = %v, want 10s (doubled after timeout)", sleep.waits[1])
	}
}

func TestPollDeviceToken_BackoffResetsAfterCompletedRoundTrip(t *testing.T) {
	// Two connection timeouts inflate the transient backoff (5→10→20); the next
	// completed round-trip (authorization_pending) must reset it to the
	// server-driven interval, so later waits return to 5s — not the inflated
	// 20s/40s a merged interval+backoff would produce.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
		{http.StatusOK, tokenBody},
	})
	sleep := &recordingSleep{}

	// The first two attempts return network timeouts; the rest hit the server.
	base := tlsClient(srv)
	client := &http.Client{Transport: &timeoutNTransport{next: base.Transport, n: 2}}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	// Waits: 5s, then timeout-doubled 10s and 20s, then back to the server
	// interval (5s) once round-trips complete.
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 5 * time.Second, 5 * time.Second}
	assertWaits(t, sleep.waits, want)
}

func TestPollDeviceToken_RedirectClassifiedByStatusWithoutDrainingBody(t *testing.T) {
	// A 3xx must fail by STATUS before the body is read. An oversized 3xx body
	// would trip the size cap if drained — the early status check surfaces it as a
	// redirect api_error instead (and, for a slow body, avoids a timeout+retry).
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://attacker.example/token")
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 2*1024*1024))
	}))
	defer srv.Close()

	sleep := &recordingSleep{}
	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	assertBasecampCode(t, err, "api_error")
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("want a redirect error, got %v", err)
	}
}

func TestPollDeviceToken_BackoffTracksGrownIntervalAfterSlowDown(t *testing.T) {
	// slow_down grows the interval 5→10; a following network timeout must double
	// from the GROWN interval (10→20), not the stale pre-slow_down 5 (which would
	// give 10 and poll too aggressively under combined throttling + timeouts).
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "slow_down"}},
		{http.StatusOK, tokenBody},
	})
	sleep := &recordingSleep{}
	base := tlsClient(srv)
	// Time out only the 2nd attempt — the poll right after slow_down.
	client := &http.Client{Transport: &timeoutOnAttemptTransport{next: base.Transport, attempt: 2}}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	// 5 (initial) → slow_down grows the interval to 10 → wait 10 → timeout doubles
	// the backoff from 10 to 20 → wait 20 → token. The stale-backoff bug gave 10.
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
	assertWaits(t, sleep.waits, want)
}

func TestPollDeviceToken_ExpiresAgainstInjectedClock(t *testing.T) {
	srv, calls := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
	})
	sleep := &recordingSleep{}

	// Clock: base at t0, then jumps past the 900s deadline on the first check.
	times := []time.Time{
		time.Unix(0, 0),
		time.Unix(1_000_000, 0),
	}
	idx := 0
	clock := func() time.Time {
		t := times[min(idx, len(times)-1)]
		idx++
		return t
	}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn), WithDeviceClock(clock))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v", err)
	}
	if dfe.Reason != DeviceFlowExpired {
		t.Errorf("Reason = %q, want expired", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeAuth {
		t.Errorf("Code = %q, want auth_required", dfe.Code())
	}
	if *calls != 0 {
		t.Errorf("expected no polls after expiry, got %d", *calls)
	}
}

func TestPollDeviceToken_AccessDenied(t *testing.T) {
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "access_denied"}},
	})
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v", err)
	}
	if dfe.Reason != DeviceFlowAccessDenied {
		t.Errorf("Reason = %q, want access_denied", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeAuth {
		t.Errorf("Code = %q, want auth_required", dfe.Code())
	}
}

func TestPollDeviceToken_ExpiredTokenError(t *testing.T) {
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "expired_token"}},
	})
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired), got %v", err)
	}
}

func TestPollDeviceToken_TransportRetryable(t *testing.T) {
	// A server that resets the connection produces a non-timeout transport error.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v", err)
	}
	if dfe.Reason != DeviceFlowTransport {
		t.Errorf("Reason = %q, want transport", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeNetwork {
		t.Errorf("Code = %q, want network", dfe.Code())
	}
	if !dfe.Retryable() {
		t.Error("transport error should be retryable")
	}
}

func TestPollDeviceToken_MalformedSuccessResponseIsAPIError(t *testing.T) {
	// A 2xx whose body is missing access_token is a server/api fault (api_error),
	// NOT a retryable transport error.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusOK, map[string]any{"token_type": "Bearer"}}, // no access_token
	})
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Fatalf("want a plain api_error, got DeviceFlowError(%q)", dfe.Reason)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Fatalf("want api_error, got %v (%T)", err, err)
	}
	if be.Retryable {
		t.Error("a malformed token response must not be retryable")
	}
}

// rawTokenServer serves a fixed raw 200 token-endpoint body, for cases a Go map
// cannot express (a JSON literal like 1e400 that json.Marshal rejects).
func rawTokenServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPollDeviceToken_RejectsMalformedTokenExpiresIn(t *testing.T) {
	// A 2xx whose expires_in cannot be a schedulable lifetime is a server/api
	// fault, never a token: 1e400/string/bool fail json.Unmarshal into int;
	// a negative or past-ceiling integer would overflow ExpiresAt arithmetic.
	for _, body := range []string{
		`{"access_token":"a","token_type":"Bearer","expires_in":1e400}`,
		`{"access_token":"a","token_type":"Bearer","expires_in":"3600"}`,
		`{"access_token":"a","token_type":"Bearer","expires_in":true}`,
		`{"access_token":"a","token_type":"Bearer","expires_in":1.5}`, // fractional: whole-second contract
		`{"access_token":"a","token_type":"Bearer","expires_in":-1}`,
		`{"access_token":"a","token_type":"Bearer","expires_in":2147483648}`,
	} {
		srv := rawTokenServer(t, body)
		sleep := &recordingSleep{}
		_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
			WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
		assertBasecampCode(t, err, basecamp.CodeAPI)
	}
}

func TestPollDeviceToken_AcceptsMaxTokenLifetime(t *testing.T) {
	// The 2147483647 s ceiling itself is valid — the bound is inclusive.
	srv := rawTokenServer(t, `{"access_token":"device_access_token","token_type":"Bearer","expires_in":2147483647}`)
	sleep := &recordingSleep{}
	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.ExpiresIn != maxTokenLifetimeSeconds {
		t.Errorf("ExpiresIn = %d, want %d", token.ExpiresIn, maxTokenLifetimeSeconds)
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set for a positive expires_in")
	}
}

func TestPollDeviceToken_RejectsZeroAndFractionalExpiresIn(t *testing.T) {
	// An explicit "expires_in":0 must be api_error, not silently treated as
	// absent (the old plain-int decode made 0 indistinguishable from omitted),
	// and a fractional lifetime is malformed per the cross-SDK whole-second rule.
	for _, body := range []string{
		`{"access_token":"device_access_token","token_type":"Bearer","expires_in":0}`,
		`{"access_token":"device_access_token","token_type":"Bearer","expires_in":3600.5}`,
	} {
		srv := rawTokenServer(t, body)
		sleep := &recordingSleep{}
		_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
			WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
		assertBasecampCode(t, err, basecamp.CodeAPI)
		srv.Close()
	}
}

func TestPollDeviceToken_AcceptsIntegerValuedFloatExpiresIn(t *testing.T) {
	// 3600.0 carries no fractional part — accepted per the cross-SDK contract
	// (the old decode into a plain int rejected it, unlike TS/Python/Ruby).
	srv := rawTokenServer(t, `{"access_token":"device_access_token","token_type":"Bearer","expires_in":3600.0}`)
	sleep := &recordingSleep{}
	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", token.ExpiresIn)
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set for a positive expires_in")
	}
}

func TestPollDeviceToken_RejectsExplicitEmptyTokenType(t *testing.T) {
	// An explicit "token_type":"" is malformed token metadata (api_error),
	// distinct from an absent field — the old plain-string decode coerced both
	// to Bearer. Uniform with Python/Ruby/TS/Kotlin.
	srv := rawTokenServer(t, `{"access_token":"device_access_token","token_type":"","expires_in":3600}`)
	sleep := &recordingSleep{}
	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	assertBasecampCode(t, err, basecamp.CodeAPI)
}

func TestPollDeviceToken_DefaultsAbsentTokenTypeToBearer(t *testing.T) {
	// Absent token_type defaults to Bearer — only an explicit empty string is
	// rejected.
	srv := rawTokenServer(t, `{"access_token":"device_access_token","expires_in":3600}`)
	sleep := &recordingSleep{}
	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", token.TokenType)
	}
}

func TestPollDeviceToken_AcceptsTokenWithoutExpiresIn(t *testing.T) {
	// Absent expires_in (RFC 6749 §5.1) is allowed — the token carries no expiry.
	srv := rawTokenServer(t, `{"access_token":"device_access_token","token_type":"Bearer"}`)
	sleep := &recordingSleep{}
	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.ExpiresIn != 0 {
		t.Errorf("ExpiresIn = %d, want 0 (absent)", token.ExpiresIn)
	}
	if !token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be zero when expires_in is absent")
	}
}

func TestPollDeviceToken_NullOptionalFieldsAreAbsent(t *testing.T) {
	// JSON null for refresh_token/scope/token_type is absent per SPEC — the
	// token must be accepted with zero values (and Bearer default), never an
	// api_error.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{{http.StatusOK, map[string]any{
		"access_token":  "tok",
		"refresh_token": nil,
		"scope":         nil,
		"token_type":    nil,
	}}})
	sleep := &recordingSleep{}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.RefreshToken != "" || token.Scope != "" {
		t.Errorf("null optional fields must map to zero values, got %q/%q", token.RefreshToken, token.Scope)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer default", token.TokenType)
	}
}

func TestPollDeviceToken_CancelledViaContext(t *testing.T) {
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel on the first sleep, before any poll.
	sleep := &recordingSleep{before: cancel}

	_, err := PollDeviceToken(ctx, srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v", err)
	}
	if dfe.Reason != DeviceFlowCancelled {
		t.Errorf("Reason = %q, want cancelled", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeUsage {
		t.Errorf("Code = %q, want usage", dfe.Code())
	}
	if !errors.Is(err, context.Canceled) {
		t.Error("cancelled error should wrap context.Canceled")
	}
}

func TestPerformDeviceLogin_GuardsCapability(t *testing.T) {
	polled := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		polled = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL,
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{"refresh_token"}, // no device_code grant
	}

	_, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli", func(DeviceAuthorization) {},
		WithDeviceHTTPClient(tlsClient(srv)))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v", err)
	}
	if dfe.Reason != DeviceFlowUnavailable {
		t.Errorf("Reason = %q, want unavailable", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeValidation {
		t.Errorf("Code = %q, want validation", dfe.Code())
	}
	if polled {
		t.Error("must not poll when capability guard fails")
	}
}

func TestPerformDeviceLogin_FiresDisplayThenCompletes(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device" {
			_ = json.NewEncoder(w).Encode(deviceAuthBody)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	var displayed *DeviceAuthorization
	sleep := &recordingSleep{}

	token, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli",
		func(a DeviceAuthorization) { displayed = &a },
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if displayed == nil {
		t.Fatal("display hook was not called")
	}
	if displayed.UserCode != "WDJB-MJHT" {
		t.Errorf("displayed UserCode = %q", displayed.UserCode)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
}

func TestPerformDeviceLogin_NilConfigUnavailable(t *testing.T) {
	_, err := PerformDeviceLogin(context.Background(), nil, "basecamp-cli", func(DeviceAuthorization) {})
	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowUnavailable {
		t.Fatalf("want DeviceFlowError(unavailable), got %v", err)
	}
}

func TestPollDeviceToken_TokenEndpointDoesNotFollowRedirect(t *testing.T) {
	attackerHit := false
	attacker := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attackerHit = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenBody) // if chased, would masquerade as success
	}))
	defer attacker.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", attacker.URL)
		w.WriteHeader(http.StatusFound) // 302 → attacker
	}))
	defer srv.Close()
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(twoServerClient(srv, attacker)), WithDeviceSleep(sleep.fn))

	if attackerHit {
		t.Fatal("redirect was followed — attacker host was contacted")
	}
	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Fatalf("want a plain api_error, got DeviceFlowError(%q)", dfe.Reason)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Fatalf("want api_error for the unfollowed 302, got %v (%T)", err, err)
	}
}

func TestPollDeviceToken_RedirectWithPendingBodyIsAPIError(t *testing.T) {
	// Redirects are suppressed, so a 3xx reaches the classifier with its body
	// intact. A crafted {"error":"authorization_pending"} on a 302 must surface
	// as an api_error — not be mistaken for a pending poll that keeps the loop
	// running.
	srv, calls := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusFound, map[string]any{"error": "authorization_pending"}},
	})
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Fatalf("want a plain api_error, got DeviceFlowError(%q)", dfe.Reason)
	}
	assertBasecampCode(t, err, basecamp.CodeAPI)
	if *calls != 1 {
		t.Errorf("expected polling to stop after the redirect, got %d polls", *calls)
	}
}

func TestRequestDeviceAuthorization_DoesNotFollowRedirect(t *testing.T) {
	attackerHit := false
	attacker := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attackerHit = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer attacker.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", attacker.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
		WithDeviceHTTPClient(twoServerClient(srv, attacker)))

	if attackerHit {
		t.Fatal("redirect was followed — attacker host was contacted")
	}
	assertBasecampCode(t, err, basecamp.CodeAPI)
}

func TestPollDeviceToken_ClampsBackoffToDeadline(t *testing.T) {
	// Every poll times out, so the connection-timeout backoff escalates
	// (5→10→20→40→60). A clock that jumps close to the deadline must clamp each
	// subsequent wait to the remaining time rather than the escalating backoff,
	// and the flow must expire promptly instead of overshooting.
	client := &http.Client{Transport: timeoutAlwaysTransport{}}
	sleep := &recordingSleep{}

	base := time.Unix(0, 0)
	// Per iteration the loop reads the clock twice (remaining, then deadline
	// check); offsets in seconds, last value repeats.
	offsets := []int{0, 0, 0, 0, 0, 95, 95, 98, 98, 100}
	i := 0
	clock := func() time.Time {
		s := offsets[min(i, len(offsets)-1)]
		i++
		return base.Add(time.Duration(s) * time.Second)
	}

	_, err := PollDeviceToken(context.Background(), "https://issuer.example/token", "basecamp-cli",
		testDeviceCode, 5, 100,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn), WithDeviceClock(clock))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired), got %v", err)
	}
	if len(sleep.waits) < 4 {
		t.Fatalf("expected the backoff to escalate over several polls, got waits %v", sleep.waits)
	}
	// Absent clamping, the third and fourth waits would be 20s and 40s. Clamped to
	// the remaining time they must stay at or below the largest full-interval wait
	// (10s) and never exceed the remaining window.
	for idx, w := range sleep.waits {
		if w > 10*time.Second {
			t.Errorf("waits[%d] = %v exceeds the deadline-clamped bound (10s): %v", idx, w, sleep.waits)
		}
	}
}

func TestPollDeviceToken_ExpiredBeforeFirstWaitDoesNotSleep(t *testing.T) {
	// A slow caller between issuance and the first poll: the monotonic deadline is
	// already in the past when the loop first checks it. The check-before-wait
	// guard must return expired without sleeping a negative duration into the
	// injected seam.
	sleep := &recordingSleep{}
	base := time.Unix(0, 0)
	// clock call 0 anchors the deadline (t=0 → deadline 30s); call 1 (remaining)
	// reads t=60s, already past the deadline.
	offsets := []int{0, 60}
	i := 0
	clock := func() time.Time {
		s := offsets[min(i, len(offsets)-1)]
		i++
		return base.Add(time.Duration(s) * time.Second)
	}

	_, err := PollDeviceToken(context.Background(), "https://issuer.example/token", "basecamp-cli",
		testDeviceCode, 5, 30,
		WithDeviceSleep(sleep.fn), WithDeviceClock(clock))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired), got %v", err)
	}
	if len(sleep.waits) != 0 {
		t.Errorf("expected no sleep before expiry, got waits %v", sleep.waits)
	}
}

func TestPerformDeviceLogin_NilDisplayIsUsageError(t *testing.T) {
	// A nil display is the only mechanism that surfaces the verification URI
	// and user code: skipping it would mint a code nobody can approve and
	// poll until expiry. Reject as usage BEFORE any request.
	requests := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	_, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli", nil,
		WithDeviceHTTPClient(tlsClient(srv)))

	var bcErr *basecamp.Error
	if !errors.As(err, &bcErr) || bcErr.Code != basecamp.CodeUsage {
		t.Fatalf("want *basecamp.Error(usage) for nil display, got %T: %v", err, err)
	}
	if requests != 0 {
		t.Errorf("requests = %d, want 0 — the guard must reject before any request", requests)
	}
}

func TestPerformDeviceLogin_CancelDuringDisplayBeatsExpiry(t *testing.T) {
	// A display hook that both cancels the flow and consumes the whole code
	// lifetime (a prompt closing in response to cancellation) must surface
	// cancelled, not expired.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{}
		for k, v := range deviceAuthBody {
			body[k] = v
		}
		body["expires_in"] = 10
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	// Clock: t0 at issuance, then past the 10s lifetime once display returns.
	times := []time.Time{time.Unix(0, 0), time.Unix(100, 0)}
	idx := 0
	clock := func() time.Time {
		tm := times[min(idx, len(times)-1)]
		idx++
		return tm
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := PerformDeviceLogin(ctx, config, "basecamp-cli",
		func(DeviceAuthorization) { cancel() },
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceClock(clock))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowCancelled {
		t.Fatalf("want DeviceFlowError(cancelled) — cancellation inside display wins over expiry — got %v", err)
	}
}

func TestPerformDeviceLogin_RechecksDeadlineAfterDisplay(t *testing.T) {
	polled := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device" {
			body := map[string]any{}
			for k, v := range deviceAuthBody {
				body[k] = v
			}
			body["expires_in"] = 10
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		polled = true
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	// Clock: t0 when the code is issued, then jumps past its 10s lifetime by the
	// time the display hook returns.
	times := []time.Time{time.Unix(0, 0), time.Unix(100, 0)}
	idx := 0
	clock := func() time.Time {
		t := times[min(idx, len(times)-1)]
		idx++
		return t
	}

	displayed := false
	_, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli",
		func(DeviceAuthorization) { displayed = true },
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceClock(clock))

	if !displayed {
		t.Fatal("display hook was not called")
	}
	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired) after a slow display hook, got %v", err)
	}
	if polled {
		t.Error("must not poll the token endpoint once the code has expired")
	}
}

func TestRequestDeviceAuthorization_OversizedBodyIsAPIError(t *testing.T) {
	// A body past the size cap is a server/api fault, not a retryable transport
	// failure and not the "too large" mislabel applied to every read failure.
	client := &http.Client{Transport: largeBodyTransport{n: maxTokenResponseBytes + 1}}

	_, err := RequestDeviceAuthorization(context.Background(), "https://issuer.example/device", "basecamp-cli",
		WithDeviceHTTPClient(client))

	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Fatalf("want a plain api_error, got DeviceFlowError(%q)", dfe.Reason)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Fatalf("want api_error, got %v (%T)", err, err)
	}
	if be.Retryable {
		t.Error("an oversized device authorization response must not be retryable")
	}
	if !errors.Is(err, errBodyTooLarge) {
		t.Error("oversized body error should wrap errBodyTooLarge")
	}
}

func TestRequestDeviceAuthorization_GenuineReadFailureIsTransport(t *testing.T) {
	// A real read failure (not an overflow) must surface as a retryable transport
	// error, NOT be mislabeled "too large".
	client := &http.Client{Transport: errBodyTransport{err: io.ErrUnexpectedEOF}}

	_, err := RequestDeviceAuthorization(context.Background(), "https://issuer.example/device", "basecamp-cli",
		WithDeviceHTTPClient(client))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) {
		t.Fatalf("want *DeviceFlowError, got %v (%T)", err, err)
	}
	if dfe.Reason != DeviceFlowTransport {
		t.Errorf("Reason = %q, want transport", dfe.Reason)
	}
	if dfe.Code() != basecamp.CodeNetwork {
		t.Errorf("Code = %q, want network", dfe.Code())
	}
	if !dfe.Retryable() {
		t.Error("a genuine read failure should be retryable")
	}
	if strings.Contains(err.Error(), "too large") {
		t.Errorf("read failure must not be mislabeled 'too large': %v", err)
	}
	if errors.Is(err, errBodyTooLarge) {
		t.Error("a genuine read failure must not wrap errBodyTooLarge")
	}
}

func TestPollDeviceToken_OversizedBodyIsAPIErrorNotRetryable(t *testing.T) {
	// A token-endpoint body past the size cap is a server/api fault (api_error,
	// non-retryable), NOT a retryable transport failure.
	client := &http.Client{Transport: largeBodyTransport{n: maxTokenResponseBytes + 1}}
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), "https://issuer.example/token", "basecamp-cli",
		testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Fatalf("want a plain api_error, got DeviceFlowError(%q)", dfe.Reason)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Fatalf("want api_error, got %v (%T)", err, err)
	}
	if be.Retryable {
		t.Error("an oversized token response must not be retryable")
	}
	// The poll path converts to a coded api_error via ErrAPI (a string message,
	// like the malformed-2xx path), so assert the size-cap classification shows
	// through the message rather than via errors.Is.
	if !strings.Contains(be.Message, "size cap") {
		t.Errorf("api_error message should identify the size-cap overflow, got %q", be.Message)
	}
}

func TestPerformDeviceLogin_AnchorsExpiryAtResponseReceipt(t *testing.T) {
	// SPEC §16: the deadline is clock.now() + expires_in taken AFTER
	// RequestDeviceAuthorization returns — a 6s request leg with expires_in 5
	// must NOT expire the fresh code client-side; expiry past receipt is
	// arbitrated by the server (expired_token). The handler runs on the
	// server goroutine, so the clock state is atomic.
	var nowSec atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device" {
			nowSec.Store(6)
			body := map[string]any{}
			for k, v := range deviceAuthBody {
				body[k] = v
			}
			body["expires_in"] = 5
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	clock := func() time.Time { return time.Unix(nowSec.Load(), 0) }
	sleep := &recordingSleep{}

	token, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli",
		func(DeviceAuthorization) {},
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceClock(clock), WithDeviceSleep(sleep.fn))

	if err != nil {
		t.Fatalf("request-leg latency must not shrink the code window: %v", err)
	}
	if token == nil || token.AccessToken == "" {
		t.Fatal("expected a token from the receipt-anchored flow")
	}
}

func TestPerformDeviceLogin_ChargesDisplayTimeAgainstPollDeadline(t *testing.T) {
	// The display hook consumes most (but not all) of the code's lifetime. The
	// poll must inherit the REMAINING window, not a fresh full one — so with the
	// remainder nearly gone it expires without ever polling. Under the pre-fix
	// behavior (full expires_in re-anchored after display) the same clock would
	// leave a large window and the token endpoint would be polled.
	polled := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device" {
			body := map[string]any{}
			for k, v := range deviceAuthBody {
				body[k] = v
			}
			body["expires_in"] = 100
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		polled = true
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	// Clock reads, in order: (1) issuance anchor, (2) remaining after display,
	// (3) wait clamp, (4) post-sleep deadline check. The poll loop inherits the
	// exact issuance-anchored deadline (no re-anchor read). The display burns 99
	// of the 100s window, then the final read crosses the deadline.
	offsets := []int{0, 99, 99, 100}
	idx := 0
	clock := func() time.Time {
		s := offsets[min(idx, len(offsets)-1)]
		idx++
		return time.Unix(int64(s), 0)
	}
	sleep := &recordingSleep{}

	displayed := false
	_, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli",
		func(DeviceAuthorization) { displayed = true },
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceClock(clock), WithDeviceSleep(sleep.fn))

	if !displayed {
		t.Fatal("display hook was not called")
	}
	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired) after display consumed the window, got %v", err)
	}
	if polled {
		t.Error("must not poll: display time should have exhausted the remaining lifetime")
	}
}

func TestPerformDeviceLogin_ExactDeadlineNoWholeSecondRounding(t *testing.T) {
	// A sub-second remainder after display must NOT round up to a whole extra
	// second nor re-anchor at a later clock read: the poll loop inherits the
	// EXACT issuance-anchored deadline, so the clamped 500ms wait lands on
	// expiry and the endpoint is never polled. The pre-fix behavior (remaining
	// rounded up to 1s, then a fresh deadline anchored inside PollDeviceToken)
	// would have left a full-second window and polled once.
	polled := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device" {
			body := map[string]any{}
			for k, v := range deviceAuthBody {
				body[k] = v
			}
			body["expires_in"] = 100
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		polled = true
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()

	endpoint := srv.URL + "/device"
	config := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	// Clock reads: (1) issuance anchor (deadline = 100s), (2) remaining after a
	// display that burned 99.5s, (3) wait clamp (500ms left), (4) post-sleep
	// deadline check exactly at expiry.
	offsetsMs := []int64{0, 99_500, 99_500, 100_000}
	idx := 0
	clock := func() time.Time {
		ms := offsetsMs[min(idx, len(offsetsMs)-1)]
		idx++
		return time.UnixMilli(ms)
	}
	sleep := &recordingSleep{}

	_, err := PerformDeviceLogin(context.Background(), config, "basecamp-cli",
		func(DeviceAuthorization) {},
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceClock(clock), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired) at the exact issuance deadline, got %v", err)
	}
	if polled {
		t.Error("must not poll: the sub-second remainder ends at the exact deadline")
	}
	assertWaits(t, sleep.waits, []time.Duration{500 * time.Millisecond})
}

// --- helpers ---

func assertWaits(t *testing.T, got, want []time.Duration) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("waits = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("waits = %v, want %v", got, want)
		}
	}
}

func assertBasecampCode(t *testing.T, err error, code string) {
	t.Helper()
	var be *basecamp.Error
	if !errors.As(err, &be) {
		t.Fatalf("want *basecamp.Error, got %v", err)
	}
	if be.Code != code {
		t.Errorf("Code = %q, want %q", be.Code, code)
	}
}

// twoServerClient returns an HTTP client that trusts both test servers, so a
// followed redirect would actually reach the second (attacker) host — making
// "attacker never contacted" a meaningful assertion.
func twoServerClient(a, b *httptest.Server) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(a.Certificate())
	pool.AddCert(b.Certificate())
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}}
}

// timeoutAlwaysTransport returns a net timeout on every RoundTrip, driving the
// connection-timeout backoff path deterministically without a live server.
type timeoutAlwaysTransport struct{}

func (timeoutAlwaysTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: timeoutError{}}
}

// timeoutOnceTransport returns a net timeout on the first RoundTrip, then
// delegates to next. It makes the connection-timeout backoff path deterministic.
type timeoutOnceTransport struct {
	next  http.RoundTripper
	fired bool
}

func (t *timeoutOnceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.fired {
		t.fired = true
		return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: timeoutError{}}
	}
	return t.next.RoundTrip(req)
}

// timeoutNTransport returns a net timeout on the first n RoundTrips, then
// delegates to next. It drives repeated-timeout backoff paths deterministically.
type timeoutNTransport struct {
	next http.RoundTripper
	n    int
}

func (t *timeoutNTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.n > 0 {
		t.n--
		return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: timeoutError{}}
	}
	return t.next.RoundTrip(req)
}

// timeoutOnAttemptTransport returns a net timeout on the Nth RoundTrip only,
// delegating every other attempt — for a timeout at a specific point in the poll
// sequence (e.g. immediately after a slow_down).
type timeoutOnAttemptTransport struct {
	next    http.RoundTripper
	attempt int
	count   int
}

func (t *timeoutOnAttemptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.count++
	if t.count == t.attempt {
		return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: timeoutError{}}
	}
	return t.next.RoundTrip(req)
}

// timeoutError is a net.Error reporting a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "simulated connection timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

// largeBodyTransport returns a 200 response whose body is n bytes, driving the
// bounded-read overflow path without a live server.
type largeBodyTransport struct{ n int64 }

func (t largeBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(make([]byte, t.n))),
		Request:    req,
	}, nil
}

// cancelOnReadTransport returns a 200 whose body cancels the caller's context on
// its first read and then fails — simulating a caller abort landing after headers
// arrive but while the body is still streaming. Do returns the response normally
// (ctx is live at Do time); the abort surfaces only during readBoundedBody.
type cancelOnReadTransport struct{ cancel context.CancelFunc }

func (t cancelOnReadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       cancelOnReadBody(t),
		Request:    req,
	}, nil
}

// completeAfterCancelTransport ignores the request context entirely: it cancels
// the caller's context and then completes a VALID response anyway, modeling a
// custom RoundTripper that does not honor cancellation.
type completeAfterCancelTransport struct {
	cancel context.CancelFunc
	body   string
}

func (t completeAfterCancelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.cancel()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}

// completeAfterCancelStatusTransport cancels the caller's context and then
// completes a response with the given status anyway, modeling a custom
// RoundTripper that does not honor cancellation.
type completeAfterCancelStatusTransport struct {
	cancel context.CancelFunc
	status int
}

func (t completeAfterCancelStatusTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.cancel()
	return &http.Response{
		StatusCode: t.status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader("{}")),
		Request:    req,
	}, nil
}

type cancelOnReadBody struct{ cancel context.CancelFunc }

func (b cancelOnReadBody) Read([]byte) (int, error) {
	b.cancel()
	return 0, errors.New("read aborted")
}
func (b cancelOnReadBody) Close() error { return nil }

// errBodyTransport returns a 200 response whose body read fails with err before
// any cap is reached, driving the genuine-I/O-failure path.
type errBodyTransport struct{ err error }

func (t errBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       errReadCloser(t),
		Request:    req,
	}, nil
}

// errReadCloser is a ReadCloser whose Read always fails with err.
type errReadCloser struct{ err error }

func (e errReadCloser) Read([]byte) (int, error) { return 0, e.err }
func (e errReadCloser) Close() error             { return nil }

func TestPollDeviceToken_ExpiresAtIsWallClock(t *testing.T) {
	// The injected clock is a polling-deadline seam only. A token's public
	// ExpiresAt must anchor to wall time (like exchange.go) — with an
	// epoch-anchored injected clock the old cfg.clock() anchoring produced an
	// ExpiresAt near 1970, an already-expired token.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{{http.StatusOK, tokenBody}})
	sleep := &recordingSleep{}
	epochClock := func() time.Time { return time.Unix(0, 0) }

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn), WithDeviceClock(epochClock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.ExpiresAt.Before(time.Now().Add(30 * time.Minute)) {
		t.Errorf("ExpiresAt = %v, want ~1h from wall-clock now (not the injected clock)", token.ExpiresAt)
	}
}

func TestPollDeviceToken_Non200SuccessIsTerminal(t *testing.T) {
	// RFC 8628/6749 token responses are exactly 200 (SPEC §16). A nonstandard
	// 201/202 carrying an access_token must not complete polling — it is a
	// terminal api_error, never an accepted token.
	for _, status := range []int{201, 202} {
		srv, _ := queueTokenResponses(t, []struct {
			status int
			body   map[string]any
		}{{status, tokenBody}})
		sleep := &recordingSleep{}

		_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
			WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
		assertBasecampCode(t, err, basecamp.CodeAPI)
	}
}

func TestPollDeviceToken_ProtocolErrorsOnlyOn4xx(t *testing.T) {
	// OAuth protocol states are recognized only on a 4xx: a nonstandard 2xx or
	// a 5xx carrying a crafted authorization_pending body must terminate as
	// api_error, never extend polling.
	for _, status := range []int{201, 202, 500} {
		srv, calls := queueTokenResponses(t, []struct {
			status int
			body   map[string]any
		}{{status, map[string]any{"error": "authorization_pending"}}})
		sleep := &recordingSleep{}

		_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
			WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
		assertBasecampCode(t, err, basecamp.CodeAPI)
		if *calls != 1 {
			t.Errorf("status %d: polled %d times, want exactly 1 (no retry)", status, *calls)
		}
	}
}

func TestPollDeviceToken_RequestTimeoutClampedToRemainingLifetime(t *testing.T) {
	// Near expiry the per-request budget must shrink to the remaining code
	// lifetime: with 15s left, a 30s default request timeout would let a
	// stalled POST blow through the monotonic deadline.
	var captured time.Duration
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if d, ok := req.Context().Deadline(); ok {
			captured = time.Until(d)
		}
		return nil, errors.New("stop after capturing the deadline")
	})

	// Scripted clock: anchor t=0, lifetime 20s; first wait 5s → 15s remaining
	// at POST time.
	times := []time.Time{time.Unix(0, 0), time.Unix(0, 0), time.Unix(5, 0)}
	idx := 0
	clock := func() time.Time {
		v := times[min(idx, len(times)-1)]
		idx++
		return v
	}
	sleep := &recordingSleep{}

	_, _ = PollDeviceToken(context.Background(), "https://issuer.example/token", "basecamp-cli", testDeviceCode, 5, 20,
		WithDeviceHTTPClient(&http.Client{Transport: transport}), WithDeviceSleep(sleep.fn), WithDeviceClock(clock))

	if captured <= 0 || captured > 15*time.Second+500*time.Millisecond {
		t.Errorf("request deadline = %v from now, want ≤ ~15s (clamped to remaining lifetime, not the 30s default)", captured)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPollDeviceToken_TerminalStatusClassifiedWithoutDrainingBody(t *testing.T) {
	// Statuses outside 200 and 4xx are terminal WITHOUT their bodies. An
	// oversized body on a 201/500 would trip the size cap if drained — the
	// early status check surfaces the status api_error instead (and, for a
	// slow body, avoids a timeout+retry-until-expiry misclassification).
	for _, status := range []int{201, 500} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 2*1024*1024))
		}))
		t.Cleanup(srv.Close)

		sleep := &recordingSleep{}
		_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
			WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
		assertBasecampCode(t, err, "api_error")
		if !strings.Contains(err.Error(), fmt.Sprintf("status %d", status)) {
			t.Errorf("want a status-%d error (not a size-cap error), got %v", status, err)
		}
	}
}

func TestNewDeviceConfig_OversizedTimeoutFallsBackToDefault(t *testing.T) {
	// A huge positive timeout (math.MaxInt64 ≈ 292y) would hold a stalled
	// request open effectively forever — it must fall back to the default like
	// zero/negative values, mirroring the other SDKs' 3600 s ceilings.
	for _, d := range []time.Duration{time.Duration(math.MaxInt64), maxDeviceRequestTimeout + time.Second} {
		cfg := newDeviceConfig([]DeviceOption{WithDeviceTimeout(d)})
		if cfg.timeout != defaultDeviceRequestTimeout {
			t.Errorf("timeout %v: got %v, want the %v default", d, cfg.timeout, defaultDeviceRequestTimeout)
		}
	}
	// The ceiling itself is valid (inclusive bound).
	cfg := newDeviceConfig([]DeviceOption{WithDeviceTimeout(maxDeviceRequestTimeout)})
	if cfg.timeout != maxDeviceRequestTimeout {
		t.Errorf("ceiling: got %v, want %v", cfg.timeout, maxDeviceRequestTimeout)
	}
}

func TestPerformDeviceLogin_EmptyEndpointIsUnavailable(t *testing.T) {
	// Present-but-empty must behave exactly like absent: unavailable, and no
	// request is ever made (the counting server would record one).
	srv, calls := countingHTTPServer(t)
	empty := ""
	cfg := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &empty,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	_, err := PerformDeviceLogin(context.Background(), cfg, "basecamp-cli", func(DeviceAuthorization) {},
		WithDeviceHTTPClient(tlsClient(srv)))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowUnavailable {
		t.Fatalf("want DeviceFlowError(unavailable), got %v", err)
	}
	if *calls != 0 {
		t.Errorf("no request may be made for an unavailable capability, got %d", *calls)
	}
}

// countingHTTPServer records how many requests it receives and 404s them.
func countingHTTPServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestPerformDeviceLogin_CancelAfterAuthorizationNeverReachesDisplay(t *testing.T) {
	// A ctx cancelled while the authorization request is in flight — here the
	// mock endpoint cancels it as it serves the code pair, modeling an
	// injected transport that ignores request cancellation — must surface as
	// cancelled without ever invoking the display hook.
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	})
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	endpoint := srv.URL + "/device"
	cfg := &Config{
		Issuer:                      srv.URL,
		TokenEndpoint:               srv.URL + "/token",
		DeviceAuthorizationEndpoint: &endpoint,
		GrantTypesSupported:         []string{DeviceCodeGrantType, "refresh_token"},
	}

	displayed := 0
	_, err := PerformDeviceLogin(ctx, cfg, "basecamp-cli", func(DeviceAuthorization) { displayed++ },
		WithDeviceHTTPClient(tlsClient(srv)))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowCancelled {
		t.Fatalf("want DeviceFlowError(cancelled), got %v", err)
	}
	if displayed != 0 {
		t.Errorf("display fired %d times for a cancelled flow, want 0", displayed)
	}
}

// contextIgnoringTransport completes requests via the base transport but with
// the context stripped, modeling an injected RoundTripper that ignores request
// cancellation.
type contextIgnoringTransport struct {
	base   http.RoundTripper
	cancel context.CancelFunc
}

func (t *contextIgnoringTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Cancel the caller's ctx mid-request, then complete anyway.
	t.cancel()
	return t.base.RoundTrip(req.Clone(context.Background()))
}

func TestPollDeviceToken_CancelledDuringIgnoredTransportBeats200(t *testing.T) {
	// A transport that ignores cancellation can return a 200 after the caller
	// cancelled — the loop must surface cancelled, never the token.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{{http.StatusOK, tokenBody}})

	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: &contextIgnoringTransport{base: tlsClient(srv).Transport, cancel: cancel}}
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(ctx, srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowCancelled {
		t.Fatalf("want DeviceFlowError(cancelled), got %v", err)
	}
}

func TestPollDeviceToken_CancelledDuringIgnoredTransportBeatsTerminalError(t *testing.T) {
	// Cancellation must also win over a TERMINAL error completed after the
	// caller cancelled: a context-ignoring transport returning access_denied
	// (or any 4xx/malformed response) post-cancellation must surface
	// cancelled, not the terminal classification.
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{{http.StatusBadRequest, map[string]any{"error": "access_denied"}}})

	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: &contextIgnoringTransport{base: tlsClient(srv).Transport, cancel: cancel}}
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(ctx, srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(client), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowCancelled {
		t.Fatalf("want DeviceFlowError(cancelled), got %v", err)
	}
}

func TestPollDeviceToken_CapturesResource(t *testing.T) {
	srv, _ := queueTokenResponses(t, []struct {
		status int
		body   map[string]any
	}{
		{http.StatusOK, map[string]any{
			"access_token":  "device_access_token",
			"refresh_token": "device_refresh_token",
			"resource":      "urn:bc:account:42",
		}},
	})
	sleep := &recordingSleep{}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.Resource != "urn:bc:account:42" {
		t.Errorf("Resource = %q, want urn:bc:account:42", token.Resource)
	}
}

func TestPollDeviceToken_ResourceValidation(t *testing.T) {
	cases := []struct {
		name     string
		resource any
		wantErr  bool
	}{
		{"null resource is absent", nil, false},
		{"empty resource is malformed", "", true},
		{"non-string resource is malformed", 7, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"access_token": "device_access_token", "resource": tc.resource}
			srv, _ := queueTokenResponses(t, []struct {
				status int
				body   map[string]any
			}{{http.StatusOK, body}})
			sleep := &recordingSleep{}

			token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
				WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
			if tc.wantErr {
				assertBasecampCode(t, err, basecamp.CodeAPI)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token.Resource != "" {
				t.Errorf("Resource = %q, want unset", token.Resource)
			}
		})
	}
}

// queueTokenResponses429 serves a fixed sequence of token-endpoint responses
// with an optional Retry-After header per response (the last repeats).
func queueTokenResponses429(t *testing.T, responses []struct {
	status     int
	body       map[string]any
	retryAfter string
}) *httptest.Server {
	t.Helper()
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := min(calls, len(responses)-1)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if responses[i].retryAfter != "" {
			w.Header().Set("Retry-After", responses[i].retryAfter)
		}
		w.WriteHeader(responses[i].status)
		_ = json.NewEncoder(w).Encode(responses[i].body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

var tooManyRequestsBody = map[string]any{"error": "too_many_requests"}

func TestPollDeviceToken_RetriesAfter429WithRetryAfterOverride(t *testing.T) {
	srv := queueTokenResponses429(t, []struct {
		status     int
		body       map[string]any
		retryAfter string
	}{
		{http.StatusTooManyRequests, tooManyRequestsBody, "30"},
		{http.StatusOK, tokenBody, ""},
	})
	sleep := &recordingSleep{}

	token, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "device_access_token" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	// Initial 5s wait, then the one-shot max(interval, Retry-After) = 30s.
	assertWaits(t, sleep.waits, []time.Duration{5 * time.Second, 30 * time.Second})
}

func TestParseRetryAfterSeconds_ASCIIOWSOnly(t *testing.T) {
	// RFC 9110: delta-seconds is 1*DIGIT and optional whitespace is ONLY
	// SP/HTAB. Unicode whitespace (NBSP above all) must not be trimmed into
	// validity — strings.TrimSpace would accept NBSP+"30" as 30.
	for _, tc := range []struct {
		header string
		want   int
	}{
		{" 30 ", 30},
		{"\t30\t", 30},
		{" \t30\t ", 30},
		{"\u00a030", 0},
		{"30\u00a0", 0},
		{"\u200930", 0},
		{"\n30\n", 0},
		// A representable over-ceiling delta clamps (the wait rule clips to
		// the remaining lifetime); an unrepresentable one stays malformed.
		{"2147484", maxDeviceSeconds},
		{"99999999999999999999", 0},
	} {
		if got := parseRetryAfterSeconds(tc.header); got != tc.want {
			t.Errorf("parseRetryAfterSeconds(%q) = %d, want %d", tc.header, got, tc.want)
		}
	}
}

func TestPollDeviceToken_429MalformedRetryAfterFallsBackToInterval(t *testing.T) {
	// The final case runs NBSP+"30" end-to-end through a real HTTP round trip
	// (Go passes obs-text header bytes through unmodified).
	// "99999999999": 11 significant digits — representable in an int, but past
	// the shared 10-digit bound, so it must fall back like TS/Python/Ruby.
	for _, header := range []string{"", "abc", "1.5", "-1", "0", "99999999999999999999", "99999999999", "+30", "\u00a030"} {
		t.Run("header="+header, func(t *testing.T) {
			srv := queueTokenResponses429(t, []struct {
				status     int
				body       map[string]any
				retryAfter string
			}{
				{http.StatusTooManyRequests, tooManyRequestsBody, header},
				{http.StatusOK, tokenBody, ""},
			})
			sleep := &recordingSleep{}

			_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
				WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertWaits(t, sleep.waits, []time.Duration{5 * time.Second, 5 * time.Second})
		})
	}
}

func TestPollDeviceToken_429DuplicateRetryAfterFallsBackToInterval(t *testing.T) {
	// Duplicate Retry-After field lines are ambiguous — Header.Get would
	// silently take the first ("30") even though the combined field is
	// malformed; the override must fall back to the current interval.
	step := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if step == 0 {
			step++
			w.Header().Add("Retry-After", "30")
			w.Header().Add("Retry-After", "bogus")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(tooManyRequestsBody)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenBody)
	}))
	defer srv.Close()
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertWaits(t, sleep.waits, []time.Duration{5 * time.Second, 5 * time.Second})
}

func TestPollDeviceToken_429RetryAfterOverrideDecaysAfterOneWait(t *testing.T) {
	srv := queueTokenResponses429(t, []struct {
		status     int
		body       map[string]any
		retryAfter string
	}{
		{http.StatusTooManyRequests, tooManyRequestsBody, "30"},
		{http.StatusBadRequest, map[string]any{"error": "authorization_pending"}, ""},
		{http.StatusOK, tokenBody, ""},
	})
	sleep := &recordingSleep{}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5s initial, 30s one-shot override, then back to the 5s interval — the
	// override never inflates the slow_down-driven cadence.
	assertWaits(t, sleep.waits, []time.Duration{5 * time.Second, 30 * time.Second, 5 * time.Second})
}

func TestPollDeviceToken_429WrongPairStaysTerminal(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   map[string]any
	}{
		{"429 without too_many_requests", http.StatusTooManyRequests, map[string]any{"error": "rate_limited"}},
		{"429 parroting authorization_pending", http.StatusTooManyRequests, map[string]any{"error": "authorization_pending"}},
		{"429 parroting slow_down", http.StatusTooManyRequests, map[string]any{"error": "slow_down"}},
		{"too_many_requests on 400", http.StatusBadRequest, tooManyRequestsBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := queueTokenResponses429(t, []struct {
				status     int
				body       map[string]any
				retryAfter string
			}{{tc.status, tc.body, "30"}})
			sleep := &recordingSleep{}

			_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
				WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))
			assertBasecampCode(t, err, basecamp.CodeAPI)
		})
	}
}

func TestPollDeviceToken_429WaitClampedToExpiry(t *testing.T) {
	srv := queueTokenResponses429(t, []struct {
		status     int
		body       map[string]any
		retryAfter string
	}{{http.StatusTooManyRequests, tooManyRequestsBody, "3600"}})
	sleep := &recordingSleep{}

	// Scripted monotonic clock: deadline anchors at t=0 with a 20s lifetime.
	// The second iteration's huge Retry-After override must clamp to the 14s
	// remaining, and the post-wait check then expires the flow.
	times := []time.Time{
		time.Unix(0, 0),  // deadline anchor
		time.Unix(0, 0),  // iter 1 remaining
		time.Unix(5, 0),  // iter 1 post-wait check
		time.Unix(6, 0),  // iter 2 remaining
		time.Unix(20, 0), // iter 2 post-wait check → expired
	}
	idx := 0
	clock := func() time.Time {
		v := times[min(idx, len(times)-1)]
		idx++
		return v
	}

	_, err := PollDeviceToken(context.Background(), srv.URL, "basecamp-cli", testDeviceCode, 5, 20,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn), WithDeviceClock(clock))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowExpired {
		t.Fatalf("want DeviceFlowError(expired), got %v", err)
	}
	assertWaits(t, sleep.waits, []time.Duration{5 * time.Second, 14 * time.Second})
}

func TestPollDeviceToken_CancellationDuring429Wait(t *testing.T) {
	srv := queueTokenResponses429(t, []struct {
		status     int
		body       map[string]any
		retryAfter string
	}{{http.StatusTooManyRequests, tooManyRequestsBody, "30"}})

	ctx, cancel := context.WithCancel(context.Background())
	waitCount := 0
	sleep := &recordingSleep{before: func() {
		waitCount++
		if waitCount == 2 {
			cancel() // cancel during the post-429 override wait
		}
	}}

	_, err := PollDeviceToken(ctx, srv.URL, "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceHTTPClient(tlsClient(srv)), WithDeviceSleep(sleep.fn))

	var dfe *DeviceFlowError
	if !errors.As(err, &dfe) || dfe.Reason != DeviceFlowCancelled {
		t.Fatalf("want DeviceFlowError(cancelled), got %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled error should wrap context.Canceled, got %v", err)
	}
}
