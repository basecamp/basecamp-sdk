package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	surfguard "github.com/basecamp/surfguard/go"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// These tests cover the address policy on the device-authorization and token
// endpoint POSTs — the requests that carry the client_id, device_code,
// authorization code, client secret, and refresh token to endpoints an
// authorization server's metadata may have chosen (#806).
//
// As in issuer_policy_test.go, the hit counters are the load-bearing part: an
// error alone proves the flow failed, not that the endpoint was never
// contacted, and "the internal host received the credentials, then something
// downstream complained" is precisely the outcome being prevented. Every
// endpoint serves a USABLE response, so a policy that failed to block would
// produce a successful login or exchange rather than a differently-worded
// failure.

// endpointHits counts requests to a mock authorization server's two endpoints.
type endpointHits struct {
	device atomic.Int64
	token  atomic.Int64
}

// bc5Endpoints starts a loopback authorization server — which the default
// policy refuses — serving a usable device-authorization response at /device
// and a usable token at /token, and returns its origin plus its counters.
func bc5Endpoints(t *testing.T) (string, *endpointHits) {
	t.Helper()
	hits := &endpointHits{}
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		hits.device.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceAuthBody)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		hits.token.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL, hits
}

// loopbackConfig is the already-selected *Config DiscoverFromResource would
// return for an issuer whose metadata named the mock server's endpoints.
func loopbackConfig(origin string) *Config {
	device := origin + "/device"
	return &Config{
		Issuer:                      origin,
		TokenEndpoint:               origin + "/token",
		DeviceAuthorizationEndpoint: &device,
		GrantTypesSupported:         []string{DeviceCodeGrantType},
	}
}

func noDisplay(DeviceAuthorization) {}

func noSleep(context.Context, time.Duration) error { return nil }

// assertBlockedEndpoint checks the refusal's shape: it matches surfguard's
// ErrBlocked, is coded api_error and not retryable, and is NOT a device-flow
// transport error (which would be retryable) — and the endpoint was never
// contacted.
func assertBlockedEndpoint(t *testing.T, err error, hits *atomic.Int64) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the endpoint to be refused, got nil error")
	}
	if !errors.Is(err, surfguard.ErrBlocked) {
		t.Errorf("errors.Is(err, surfguard.ErrBlocked) = false; err = %v", err)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) {
		t.Fatalf("expected a *basecamp.Error in the chain, got %T: %v", err, err)
	}
	if be.Code != basecamp.CodeAPI {
		t.Errorf("Code = %q, want %q", be.Code, basecamp.CodeAPI)
	}
	if be.Retryable {
		t.Errorf("Retryable = true; a policy refusal must not read as transient (err = %v)", err)
	}
	var dfe *DeviceFlowError
	if errors.As(err, &dfe) {
		t.Errorf("refusal was classified as device-flow %q; it must be the terminal policy verdict", dfe.Reason)
	}
	if hits != nil && hits.Load() != 0 {
		t.Errorf("the blocked endpoint was contacted %d time(s); the policy must refuse before any connect", hits.Load())
	}
}

// TestPerformDeviceLogin_BlockedEndpointIsNeverContacted is the core case: a
// selected config names endpoints in blocked space, and the flow must refuse
// them without opening a connection — before the client_id goes anywhere.
func TestPerformDeviceLogin_BlockedEndpointIsNeverContacted(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	_, err := PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
		WithDeviceSleep(noSleep))
	assertBlockedEndpoint(t, err, &hits.device)
	if hits.token.Load() != 0 {
		t.Errorf("token endpoint contacted %d time(s) after the device endpoint was refused", hits.token.Load())
	}
}

// TestPollDeviceToken_BlockedTokenEndpointTerminates guards the poll loop's
// classification. A dial refusal is neither a timeout (→ backoff and re-dial
// until the code expires) nor a transport failure (→ retryable): it must end
// the flow on the first attempt. The injected clock advances by each sleep, so
// a loop that wrongly kept polling would run out to expiry and fail with a
// recognizably wrong reason instead of hanging.
func TestPollDeviceToken_BlockedTokenEndpointTerminates(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	now := time.Now()
	var sleeps int
	clock := func() time.Time { return now }
	sleep := func(_ context.Context, d time.Duration) error {
		sleeps++
		now = now.Add(d)
		return nil
	}

	_, err := PollDeviceToken(context.Background(), origin+"/token", "basecamp-cli", testDeviceCode, 5, 900,
		WithDeviceClock(clock), WithDeviceSleep(sleep))
	assertBlockedEndpoint(t, err, &hits.token)
	// One wait precedes the first POST; a second would mean the loop backed off
	// and tried again.
	if sleeps != 1 {
		t.Errorf("the loop slept %d time(s); a policy refusal must terminate after the first attempt", sleeps)
	}
}

// TestExchanger_BlockedTokenEndpointIsNeverContacted covers the highest-value
// request: the one carrying the authorization code and client secret, or the
// refresh token.
func TestExchanger_BlockedTokenEndpointIsNeverContacted(t *testing.T) {
	origin, hits := bc5Endpoints(t)
	e := NewExchanger(nil)

	t.Run("exchange", func(t *testing.T) {
		_, err := e.Exchange(context.Background(), ExchangeRequest{
			TokenEndpoint: origin + "/token",
			Code:          "code",
			RedirectURI:   "http://localhost/callback",
			ClientID:      "client",
			ClientSecret:  "secret",
		})
		assertBlockedEndpoint(t, err, &hits.token)
	})

	t.Run("refresh", func(t *testing.T) {
		_, err := e.Refresh(context.Background(), RefreshRequest{
			TokenEndpoint: origin + "/token",
			RefreshToken:  "refresh",
		})
		assertBlockedEndpoint(t, err, &hits.token)
	})
}

// TestEndpointPolicy_RefusesLegacyNumericSpelling covers the spelling the
// scheme gate cannot see through: "2130706433" is 127.0.0.1 as a bare integer,
// and https on it satisfies RequireSecureEndpoint. No server is needed — the
// point is that nothing is dialed.
func TestEndpointPolicy_RefusesLegacyNumericSpelling(t *testing.T) {
	const endpoint = "https://2130706433/oauth/token"

	t.Run("device", func(t *testing.T) {
		_, err := RequestDeviceAuthorization(context.Background(), endpoint, "basecamp-cli")
		assertBlockedEndpoint(t, err, nil)
	})

	t.Run("exchange", func(t *testing.T) {
		_, err := NewExchanger(nil).Refresh(context.Background(), RefreshRequest{
			TokenEndpoint: endpoint,
			RefreshToken:  "refresh",
		})
		assertBlockedEndpoint(t, err, nil)
	})
}

// TestWithDevicePolicy_AdmitsLoopback exercises the device-flow escape hatch in
// the spelling the docs prescribe, and proves the whole grant runs through it:
// both endpoints are reached exactly once and a token comes back.
func TestWithDevicePolicy_AdmitsLoopback(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	token, err := PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
		WithDevicePolicy(DefaultIssuerPolicy().AllowLoopback()), WithDeviceSleep(noSleep))
	if err != nil {
		t.Fatalf("PerformDeviceLogin() error = %v, want nil", err)
	}
	if token == nil || token.AccessToken != "tok" {
		t.Fatalf("token = %+v, want the mock's", token)
	}
	if hits.device.Load() != 1 || hits.token.Load() != 1 {
		t.Errorf("hits = device:%d token:%d, want 1 and 1", hits.device.Load(), hits.token.Load())
	}
}

// TestWithExchangerPolicy_AdmitsLoopback is the Exchanger's escape hatch.
func TestWithExchangerPolicy_AdmitsLoopback(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	token, err := NewExchanger(nil, WithExchangerPolicy(DefaultIssuerPolicy().AllowLoopback())).
		Refresh(context.Background(), RefreshRequest{TokenEndpoint: origin + "/token", RefreshToken: "refresh"})
	if err != nil {
		t.Fatalf("Refresh() error = %v, want nil", err)
	}
	if token.AccessToken != "tok" || hits.token.Load() != 1 {
		t.Errorf("token = %+v, hits = %d; want the mock's token reached once", token, hits.token.Load())
	}
}

// TestEndpointPolicy_CallerClientTakesPrecedence pins the documented precedence
// on both surfaces: a client the caller hands us is theirs, enforcement
// included, and a policy passed alongside it is not additive.
func TestEndpointPolicy_CallerClientTakesPrecedence(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	t.Run("device", func(t *testing.T) {
		// The policy here would block loopback; the plain client does not.
		_, err := PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
			WithDevicePolicy(DefaultIssuerPolicy()), WithDeviceHTTPClient(&http.Client{}), WithDeviceSleep(noSleep))
		if err != nil {
			t.Fatalf("PerformDeviceLogin() error = %v, want nil (the client supersedes the policy)", err)
		}
		if hits.device.Load() != 1 {
			t.Errorf("device hits = %d, want 1 through the supplied client", hits.device.Load())
		}
	})

	t.Run("exchanger", func(t *testing.T) {
		before := hits.token.Load()
		_, err := NewExchanger(&http.Client{}, WithExchangerPolicy(DefaultIssuerPolicy())).
			Refresh(context.Background(), RefreshRequest{TokenEndpoint: origin + "/token", RefreshToken: "refresh"})
		if err != nil {
			t.Fatalf("Refresh() error = %v, want nil (the client supersedes the policy)", err)
		}
		if hits.token.Load() != before+1 {
			t.Errorf("token hits = %d, want %d through the supplied client", hits.token.Load(), before+1)
		}
	})

	// And http.DefaultClient is the documented spelling of "no policy".
	t.Run("default_client_disables", func(t *testing.T) {
		before := hits.token.Load()
		_, err := NewExchanger(http.DefaultClient).
			Refresh(context.Background(), RefreshRequest{TokenEndpoint: origin + "/token", RefreshToken: "refresh"})
		if err != nil {
			t.Fatalf("Refresh() on http.DefaultClient error = %v, want nil", err)
		}
		if hits.token.Load() != before+1 {
			t.Errorf("token hits = %d, want %d", hits.token.Load(), before+1)
		}
	})
}

// TestDefaultPolicyClientIsSharedAcrossSurfaces pins the resource invariant
// from #804 now that three surfaces default to the policy client: neither a
// device-flow call nor an Exchanger has a Close, so each must reuse the one
// shared transport rather than own a pool nothing can reclaim — and a
// caller's own policy must NOT land on that shared transport.
func TestDefaultPolicyClientIsSharedAcrossSurfaces(t *testing.T) {
	shared := sharedPolicyClient()

	// suppressRedirects copies the client, so compare the transport, which is
	// what owns the pool.
	if got := newDeviceConfig(nil).httpClient.Transport; got != shared.Transport {
		t.Error("a default device-flow config does not ride the shared policy transport")
	}
	if NewExchanger(nil).httpClient != shared {
		t.Error("a default Exchanger does not hold the shared policy client")
	}
	if NewDiscoverer(nil).issuerClient != shared {
		t.Error("a default Discoverer does not hold the shared policy client")
	}

	// A caller's policy gets its own transport, and exactly one per option
	// value: PerformDeviceLogin hands the same opts to RequestDeviceAuthorization
	// and to the poll, so a transport per newDeviceConfig would be two pools
	// per login.
	opt := WithDevicePolicy(DefaultIssuerPolicy().AllowLoopback())
	first := newDeviceConfig([]DeviceOption{opt}).httpClient.Transport
	second := newDeviceConfig([]DeviceOption{opt}).httpClient.Transport
	if first == shared.Transport {
		t.Error("WithDevicePolicy reused the shared transport; a caller policy must get its own")
	}
	if first != second {
		t.Error("WithDevicePolicy built a transport per call; it must build one per option")
	}
	if NewExchanger(nil, WithExchangerPolicy(DefaultIssuerPolicy().AllowLoopback())).httpClient == shared {
		t.Error("WithExchangerPolicy reused the shared client; a caller policy must get its own")
	}
}

// TestSharedPolicyClientIsNeverMutatedByEndpointFlows is the safety property
// that makes sharing legal on the new surfaces: the device flow needs redirects
// suppressed, and it gets there by copying — if suppressRedirects ever set
// CheckRedirect in place, every Discoverer and Exchanger in the process would
// be reconfigured by whichever device login ran first.
func TestSharedPolicyClientIsNeverMutatedByEndpointFlows(t *testing.T) {
	shared := sharedPolicyClient()
	if shared.CheckRedirect != nil || shared.Timeout != 0 {
		t.Fatalf("shared policy client did not start clean: CheckRedirect=%v Timeout=%v", shared.CheckRedirect != nil, shared.Timeout)
	}

	origin, _ := bc5Endpoints(t)
	_, _ = PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
		WithDeviceSleep(noSleep))
	_, _ = NewExchanger(nil).Refresh(context.Background(),
		RefreshRequest{TokenEndpoint: origin + "/token", RefreshToken: "refresh"})

	if shared.CheckRedirect != nil || shared.Timeout != 0 {
		t.Error("the shared policy client was mutated in place; the endpoint flows must copy before configuring")
	}
}

// TestEndpointPolicyIsolation is the behavioral form of the sharing concern: a
// permissive device login must not widen a default one that runs after it.
func TestEndpointPolicyIsolation(t *testing.T) {
	origin, hits := bc5Endpoints(t)

	if _, err := PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
		WithDevicePolicy(DefaultIssuerPolicy().AllowLoopback()), WithDeviceSleep(noSleep)); err != nil {
		t.Fatalf("the permissive login should reach loopback: %v", err)
	}
	deviceHits, tokenHits := hits.device.Load(), hits.token.Load()

	_, err := PerformDeviceLogin(context.Background(), loopbackConfig(origin), "basecamp-cli", noDisplay,
		WithDeviceSleep(noSleep))
	assertBlockedEndpoint(t, err, nil)
	if hits.device.Load() != deviceHits || hits.token.Load() != tokenHits {
		t.Errorf("a default login contacted the endpoints after a permissive one; the permissive policy leaked")
	}
}
