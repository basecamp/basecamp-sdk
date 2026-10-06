package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// A token-endpoint refusal carries what the server said, typed: the RFC 6749
// error code, the HTTP status, and the wait a Retry-After names. A 429 is
// rate_limit — an abuse block's Retry-After can run to hours, and a caller
// that cannot read it resends into the block (SPEC §16).
func TestExchanger_Refresh_CarriesOAuthErrorAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		retryAfter string
		body       string
		wantCode   string
		wantOAuth  string
		wantWait   int
	}{
		{"429 abuse block", 429, "14400", `{"error":"too_many_requests","error_description":"Temporarily blocked due to repeated failures. Try again later."}`, basecamp.CodeRateLimit, "too_many_requests", 14400},
		{"429 bare", 429, "", ``, basecamp.CodeRateLimit, "", 0},
		{"429 HTTP-date", 429, time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat), `{"error":"too_many_requests"}`, basecamp.CodeRateLimit, "too_many_requests", 7200},
		{"400 invalid_grant", 400, "", `{"error":"invalid_grant","error_description":"Token has been revoked"}`, basecamp.CodeAuth, "invalid_grant", 0},
		{"400 invalid_request", 400, "", `{"error":"invalid_request"}`, basecamp.CodeAPI, "invalid_request", 0},
		{"503 with Retry-After", 503, "30", ``, basecamp.CodeAPI, "", 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			_, err := NewExchanger(server.Client()).Refresh(context.Background(), RefreshRequest{
				TokenEndpoint: server.URL,
				RefreshToken:  "refresh123",
				ClientID:      "basecamp-cli",
			})
			var bcErr *basecamp.Error
			if !errors.As(err, &bcErr) {
				t.Fatalf("error = %T %v, want *basecamp.Error", err, err)
			}
			if bcErr.Code != tc.wantCode || bcErr.HTTPStatus != tc.status {
				t.Errorf("error = %s/%d, want %s/%d", bcErr.Code, bcErr.HTTPStatus, tc.wantCode, tc.status)
			}
			if bcErr.OAuthError != tc.wantOAuth {
				t.Errorf("OAuthError = %q, want %q", bcErr.OAuthError, tc.wantOAuth)
			}
			// An HTTP-date is resolved against the clock, so allow a second's
			// drift between the header being written and being read.
			if bcErr.RetryAfter < tc.wantWait-1 || bcErr.RetryAfter > tc.wantWait {
				t.Errorf("RetryAfter = %d, want %d", bcErr.RetryAfter, tc.wantWait)
			}
		})
	}
}

// The device authorization endpoint is refused the same way: a 4xx's OAuth
// error code and description are read and typed, and a 429 is rate_limit
// carrying its Retry-After. Before, every non-2xx was a bare "status 429"
// api_error, so a person locked out by an abuse block was never told how
// long to wait. A refusal other than 429 stays api_error: it is a failure
// of the login being attempted, not a stale login to sign in again over.
func TestRequestDeviceAuthorization_CarriesOAuthErrorAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		retryAfter  string
		body        string
		wantCode    string
		wantOAuth   string
		wantWait    int
		wantMessage string
	}{
		{"429 abuse block", 429, "14400", `{"error":"too_many_requests","error_description":"Temporarily blocked due to repeated failures. Try again later."}`, basecamp.CodeRateLimit, "too_many_requests", 14400,
			"device authorization failed with status 429: too_many_requests - Temporarily blocked due to repeated failures. Try again later."},
		{"429 bare", 429, "60", ``, basecamp.CodeRateLimit, "", 60, "device authorization failed with status 429"},
		{"400 unauthorized_client", 400, "", `{"error":"unauthorized_client","error_description":"Client not authorized for device_code grant"}`, basecamp.CodeAPI, "unauthorized_client", 0,
			"device authorization failed with status 400: unauthorized_client - Client not authorized for device_code grant"},
		{"400 non-JSON", 400, "", `client_id=secret-thing`, basecamp.CodeAPI, "", 0, "device authorization failed with status 400"},
		{"503 dark launch", 503, "", ``, basecamp.CodeAPI, "", 0, "device authorization failed with status 503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := RequestDeviceAuthorization(context.Background(), srv.URL, "basecamp-cli",
				WithDeviceHTTPClient(tlsClient(srv)))
			var bcErr *basecamp.Error
			if !errors.As(err, &bcErr) {
				t.Fatalf("error = %T %v, want *basecamp.Error", err, err)
			}
			if bcErr.Code != tc.wantCode || bcErr.HTTPStatus != tc.status {
				t.Errorf("error = %s/%d, want %s/%d", bcErr.Code, bcErr.HTTPStatus, tc.wantCode, tc.status)
			}
			if bcErr.OAuthError != tc.wantOAuth {
				t.Errorf("OAuthError = %q, want %q", bcErr.OAuthError, tc.wantOAuth)
			}
			if bcErr.RetryAfter != tc.wantWait {
				t.Errorf("RetryAfter = %d, want %d", bcErr.RetryAfter, tc.wantWait)
			}
			if bcErr.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", bcErr.Message, tc.wantMessage)
			}
			// SPEC §9: only error and error_description are rendered.
			if strings.Contains(bcErr.Error(), "secret-thing") {
				t.Errorf("error = %q renders the response body", bcErr.Error())
			}
		})
	}
}
