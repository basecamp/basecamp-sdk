package oauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
