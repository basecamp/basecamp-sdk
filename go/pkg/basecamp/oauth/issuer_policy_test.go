package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	surfguard "github.com/basecamp/surfguard/go"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// These tests cover the address policy on DiscoverFromResource's
// advertised-issuer hop — the one fetch whose destination a remote peer chose.
//
// The hit counters are the load-bearing part. An assertion that discovery
// returned an error proves only that discovery failed; it does not prove the
// issuer was never contacted, and "the internal host answered, then binding
// rejected it" is exactly the outcome the policy exists to prevent. Every
// blocked case therefore asserts the target's handler ran zero times.

// hitCounter is an http.Handler that records how many requests reached it and
// serves usable AS metadata, so a policy that failed to block would produce a
// SUCCESSFUL discovery rather than a differently-worded failure.
type hitCounter struct {
	hits   atomic.Int64
	issuer string
}

func (h *hitCounter) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.hits.Add(1)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"issuer":"` + h.issuer + `","token_endpoint":"` + h.issuer + `/oauth/token"}`))
}

// resourceAdvertising starts a hop-1 resource server whose metadata advertises
// the given authorization server, and returns its origin.
func resourceAdvertising(t *testing.T, advertised string) string {
	t.Helper()
	var origin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource":"` + origin + `","authorization_servers":["` + advertised + `"]}`))
	}))
	t.Cleanup(srv.Close)
	origin = srv.URL
	return origin
}

// bc5OnLoopback starts a hop-2 issuer server on loopback — which the default
// policy refuses — and returns its origin plus its hit counter.
func bc5OnLoopback(t *testing.T) (string, *hitCounter) {
	t.Helper()
	counter := &hitCounter{}
	srv := httptest.NewServer(counter)
	t.Cleanup(srv.Close)
	counter.issuer = srv.URL
	return srv.URL, counter
}

func assertBlockedIssuer(t *testing.T, err error, counter *hitCounter) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the advertised issuer to be refused, got nil error")
	}
	if !errors.Is(err, ErrInvalidIssuerOrigin) {
		t.Errorf("errors.Is(err, ErrInvalidIssuerOrigin) = false; err = %v", err)
	}
	if !errors.Is(err, surfguard.ErrBlocked) {
		t.Errorf("errors.Is(err, surfguard.ErrBlocked) = false; err = %v", err)
	}
	if errors.Is(err, ErrASFetchFailed) {
		t.Errorf("a policy refusal must not read as the transient as_fetch_failed; err = %v", err)
	}
	var se *SelectionError
	if !errors.As(err, &se) {
		t.Errorf("expected a *SelectionError, got %T", err)
	}
	var be *basecamp.Error
	if !errors.As(err, &be) || be.Code != basecamp.CodeAPI {
		t.Errorf("expected the api_error taxonomy code, got %v (%T)", err, err)
	}
	// A policy refusal is permanent. The dial failure is classified as a
	// retryable network error deep in the fetch, before anything knows why it
	// failed, and SelectionError inherits Retryable from its cause — so without
	// re-coding, callers would be told to retry a target they must stop talking
	// to.
	if be != nil && be.Retryable {
		t.Errorf("Retryable = true; a policy refusal must not read as transient (err = %v)", err)
	}
	if counter != nil && counter.hits.Load() != 0 {
		t.Errorf("the blocked issuer was contacted %d time(s); the policy must refuse before any connect", counter.hits.Load())
	}
}

// TestDiscoverFromResource_BlockedIssuerIsNeverContacted is the core case: a
// resource advertises an issuer in blocked space and the SDK must refuse it
// without ever opening a connection to it.
func TestDiscoverFromResource_BlockedIssuerIsNeverContacted(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	// The hop-1 client is the httptest one; the hop-2 client is the default,
	// policy-enforced one. That asymmetry is the feature under test.
	_, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
	assertBlockedIssuer(t, err, counter)
}

// TestDiscoverFromResource_BlockedIssuerNamedByHostname is the same case one
// layer further out. The literal-address form above is refused by the shape
// check before the transport resolves anything; a NAME can only be judged after
// resolution, which is the path that has to hold for a real advertised issuer
// like "internal-sso.corp". "localhost" is the one name whose resolution is
// dependable in a test, and it lands in exactly the blocked space.
func TestDiscoverFromResource_BlockedIssuerNamedByHostname(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	named := strings.Replace(bc5, "127.0.0.1", "localhost", 1)
	if named == bc5 {
		t.Fatalf("httptest origin %q is not the expected loopback form", bc5)
	}
	resourceOrigin := resourceAdvertising(t, named)

	_, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
	assertBlockedIssuer(t, err, counter)
}

// TestDiscoverFromResource_RefusesLegacyNumericIssuer covers the spelling a
// syntax gate cannot see through: "2130706433" is 127.0.0.1 written as a bare
// integer, and it is a perfectly well-formed origin root.
func TestDiscoverFromResource_RefusesLegacyNumericIssuer(t *testing.T) {
	// https, so requireOriginRoot admits it and the refusal has to come from
	// the address policy rather than the pre-existing HTTPS gate.
	t.Run("https", func(t *testing.T) {
		resourceOrigin := resourceAdvertising(t, "https://2130706433/")
		_, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
		assertBlockedIssuer(t, err, nil)
	})

	// http on the same spelling is refused one layer earlier, by the origin-root
	// profile: "2130706433" is not a localhost spelling, so plain http is out.
	// Asserted so the two refusals stay distinguishable — this one is NOT
	// evidence that the address policy works.
	t.Run("http_refused_by_origin_root_not_the_policy", func(t *testing.T) {
		resourceOrigin := resourceAdvertising(t, "http://2130706433/")
		_, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
		if !errors.Is(err, ErrInvalidIssuerOrigin) {
			t.Fatalf("errors.Is(err, ErrInvalidIssuerOrigin) = false; err = %v", err)
		}
		if errors.Is(err, surfguard.ErrBlocked) {
			t.Errorf("expected the origin-root HTTPS gate, not the address policy; err = %v", err)
		}
	})
}

// TestDiscoverFromResource_ExpectedIssuerIsPoliciedToo pins the deliberate
// decision to apply the policy uniformly. WithExpectedIssuer names an issuer
// the caller chose, which could have been exempted — but a consumer may have
// computed that value from untrusted input, and a policy that applies only on
// one branch is the shape a bypass grows out of.
func TestDiscoverFromResource_ExpectedIssuerIsPoliciedToo(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	_, err := NewDiscoverer(nil).
		DiscoverFromResource(context.Background(), resourceOrigin, WithExpectedIssuer(bc5))
	assertBlockedIssuer(t, err, counter)
}

// TestDiscoverFromResource_TwoNonLaunchpadIssuersStillAmbiguous guards the
// order of operations: the policy runs AFTER selection, so it must not quietly
// turn an ambiguous advertisement into a single surviving candidate.
func TestDiscoverFromResource_TwoNonLaunchpadIssuersStillAmbiguous(t *testing.T) {
	blocked, blockedCounter := bc5OnLoopback(t)

	var resourceOrigin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource":"` + resourceOrigin +
			`","authorization_servers":["` + blocked + `","https://bc5.example"]}`))
	}))
	defer srv.Close()
	resourceOrigin = srv.URL

	_, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
	if !errors.Is(err, ErrAmbiguousIssuers) {
		t.Errorf("errors.Is(err, ErrAmbiguousIssuers) = false; err = %v", err)
	}
	if blockedCounter.hits.Load() != 0 {
		t.Errorf("an ambiguous advertisement must contact nothing; got %d hit(s)", blockedCounter.hits.Load())
	}
}

// TestWithIssuerPolicy_AdmitsLoopback exercises the first escape hatch, in the
// spelling the docs prescribe: AllowLoopback is the one derivation that pierces
// the IANASpecialUse tables.
func TestWithIssuerPolicy_AdmitsLoopback(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	result, err := NewDiscoverer(nil, WithIssuerPolicy(DefaultIssuerPolicy().AllowLoopback())).
		DiscoverFromResource(context.Background(), resourceOrigin)
	if err != nil {
		t.Fatalf("DiscoverFromResource() error = %v, want nil", err)
	}
	if result.Config == nil || result.Issuer != bc5 {
		t.Fatalf("want the loopback issuer selected, got %+v", result)
	}
	if counter.hits.Load() != 1 {
		t.Errorf("issuer hits = %d, want 1", counter.hits.Load())
	}
}

// TestWithIssuerPolicy_AllowUnderSpecialUseDoesNotReadmitPrivateSpace pins the
// surfguard precedence the WithIssuerPolicy doc turns on, because it is the
// thing an operator would most plausibly get wrong: Allow re-admits space the
// DEFAULT tables refuse, not space the IANASpecialUse tables refuse, and all of
// RFC 1918 is in the latter. An on-premises deployment that reached for
// DefaultIssuerPolicy().Allow(...) would be silently still-blocked.
func TestWithIssuerPolicy_AllowUnderSpecialUseDoesNotReadmitPrivateSpace(t *testing.T) {
	private := netip.MustParseAddr("10.4.1.2")
	prefix := netip.MustParsePrefix("10.4.0.0/16")

	if !DefaultIssuerPolicy().Allow(prefix).Blocked(private) {
		t.Error("DefaultIssuerPolicy().Allow(10.4.0.0/16) admits 10.4.1.2; the WithIssuerPolicy doc must be corrected")
	}
	// The spelling the doc prescribes instead: drop IANASpecialUse, keep the
	// default deny tables, allow the one range.
	onPrem := surfguard.Policy{}.AllowAllPorts().Allow(prefix)
	if onPrem.Blocked(private) {
		t.Error("the documented on-premises policy refuses 10.4.1.2; the WithIssuerPolicy doc must be corrected")
	}
	if !onPrem.Blocked(netip.MustParseAddr("10.9.9.9")) {
		t.Error("the documented on-premises policy admits private space outside the allowed prefix")
	}
}

// TestWithIssuerHTTPClient_CarriesHopTwo exercises the second escape hatch: the
// caller's own client carries hop 2, and enforcement travels with it.
func TestWithIssuerHTTPClient_CarriesHopTwo(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	// A plain client — no surfguard transport at all. This is the "enforcement
	// becomes yours" contract stated in the option's doc.
	result, err := NewDiscoverer(nil, WithIssuerHTTPClient(&http.Client{})).
		DiscoverFromResource(context.Background(), resourceOrigin)
	if err != nil {
		t.Fatalf("DiscoverFromResource() error = %v, want nil", err)
	}
	if result.Config == nil || result.Issuer != bc5 {
		t.Fatalf("want the loopback issuer selected, got %+v", result)
	}
	if counter.hits.Load() != 1 {
		t.Errorf("issuer hits = %d, want 1", counter.hits.Load())
	}
}

// TestWithIssuerHTTPClient_TakesPrecedenceOverPolicy pins the documented
// precedence, so a caller passing both cannot read the combination as additive.
func TestWithIssuerHTTPClient_TakesPrecedenceOverPolicy(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	// The policy here would block loopback; the client does not. The client wins.
	result, err := NewDiscoverer(nil,
		WithIssuerPolicy(DefaultIssuerPolicy()),
		WithIssuerHTTPClient(&http.Client{}),
	).DiscoverFromResource(context.Background(), resourceOrigin)
	if err != nil {
		t.Fatalf("DiscoverFromResource() error = %v, want nil (the client supersedes the policy)", err)
	}
	if result.Config == nil || counter.hits.Load() != 1 {
		t.Errorf("want the issuer contacted once through the supplied client; hits = %d, result = %+v", counter.hits.Load(), result)
	}
}

// TestWithoutIssuerPolicy_RestoresPrePolicyBehavior exercises the third escape
// hatch and, with it, the pre-change baseline: without the policy, a loopback
// issuer advertised by a remote peer is contacted and selected.
func TestWithoutIssuerPolicy_RestoresPrePolicyBehavior(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	result, err := NewDiscoverer(nil, WithoutIssuerPolicy()).
		DiscoverFromResource(context.Background(), resourceOrigin)
	if err != nil {
		t.Fatalf("DiscoverFromResource() error = %v, want nil", err)
	}
	if result.Config == nil || result.Issuer != bc5 {
		t.Fatalf("want the loopback issuer selected, got %+v", result)
	}
	if counter.hits.Load() != 1 {
		t.Errorf("issuer hits = %d, want 1", counter.hits.Load())
	}
}

// TestIssuerPolicyTogglesAreLastWins pins the composition semantics of the two
// mutually exclusive toggles. Options are appended by wrappers that cannot see
// each other, so the resolution must depend on ORDER rather than on which
// option is more permissive — otherwise a WithoutIssuerPolicy buried in a
// shared default silently defeats a WithIssuerPolicy a wrapper appends after
// it, and the hop runs unprotected while the code reads as though it is not.
func TestIssuerPolicyTogglesAreLastWins(t *testing.T) {
	// Off then on: the policy must be back in force, so the loopback issuer is
	// refused and never contacted.
	t.Run("policy_last_wins", func(t *testing.T) {
		bc5, counter := bc5OnLoopback(t)
		resourceOrigin := resourceAdvertising(t, bc5)

		_, err := NewDiscoverer(nil, WithoutIssuerPolicy(), WithIssuerPolicy(DefaultIssuerPolicy())).
			DiscoverFromResource(context.Background(), resourceOrigin)
		assertBlockedIssuer(t, err, counter)
	})

	// On then off: the later WithoutIssuerPolicy must win, so the same issuer
	// is reached. Without this direction the test would pass under a rule of
	// "the strictest option wins", which is not the semantics being pinned.
	t.Run("off_last_wins", func(t *testing.T) {
		bc5, counter := bc5OnLoopback(t)
		resourceOrigin := resourceAdvertising(t, bc5)

		result, err := NewDiscoverer(nil, WithIssuerPolicy(DefaultIssuerPolicy()), WithoutIssuerPolicy()).
			DiscoverFromResource(context.Background(), resourceOrigin)
		if err != nil {
			t.Fatalf("DiscoverFromResource() error = %v, want nil", err)
		}
		if result.Issuer != bc5 {
			t.Fatalf("want the loopback issuer selected, got %+v", result)
		}
		if counter.hits.Load() != 1 {
			t.Errorf("issuer hits = %d, want 1", counter.hits.Load())
		}
	})
}

// TestDefaultIssuerClientIsShared pins the resource invariant: the default
// issuer client owns a transport, and a Discoverer has no Close, so building one
// per Discoverer would leak a connection pool per construction.
func TestDefaultIssuerClientIsShared(t *testing.T) {
	a := NewDiscoverer(nil)
	b := NewDiscoverer(&http.Client{})
	if a.issuerClient != b.issuerClient {
		t.Error("two default Discoverers hold different issuer clients; the default must be shared")
	}

	// A caller-supplied policy must NOT land on the shared client, or one
	// caller's Allow would widen every other Discoverer in the process.
	own := NewDiscoverer(nil, WithIssuerPolicy(DefaultIssuerPolicy().AllowLoopback()))
	if own.issuerClient == a.issuerClient {
		t.Error("WithIssuerPolicy reused the shared issuer client; a caller policy must get its own transport")
	}
}

// TestSharedIssuerClientIsNeverMutated is the safety property that makes sharing
// legal. fetchDiscoveryDocument needs redirects suppressed, and it gets there by
// COPYING the client — if it ever set CheckRedirect in place, every Discoverer
// in the process would be reconfigured by whichever one ran first.
func TestSharedIssuerClientIsNeverMutated(t *testing.T) {
	shared := NewDiscoverer(nil).issuerClient
	if shared.CheckRedirect != nil || shared.Timeout != 0 {
		t.Fatalf("shared issuer client did not start clean: CheckRedirect=%v Timeout=%v", shared.CheckRedirect != nil, shared.Timeout)
	}

	// Drive a real discovery through it (blocked, but it reaches the fetch).
	bc5, _ := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)
	_, _ = NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)

	if shared.CheckRedirect != nil {
		t.Error("the shared issuer client was mutated in place; noRedirectClient must copy")
	}
}

// TestIssuerPolicyIsolationAcrossDiscoverers is the behavioral form of the same
// concern, and the one that would catch aliasing that pointer identity cannot:
// a Discoverer that widened its policy must not widen a default one built either
// before or after it.
func TestIssuerPolicyIsolationAcrossDiscoverers(t *testing.T) {
	bc5, counter := bc5OnLoopback(t)
	resourceOrigin := resourceAdvertising(t, bc5)

	before := NewDiscoverer(nil)
	permissive := NewDiscoverer(nil, WithIssuerPolicy(DefaultIssuerPolicy().AllowLoopback()))

	if _, err := permissive.DiscoverFromResource(context.Background(), resourceOrigin); err != nil {
		t.Fatalf("the permissive Discoverer should reach loopback: %v", err)
	}
	if counter.hits.Load() != 1 {
		t.Fatalf("permissive issuer hits = %d, want 1", counter.hits.Load())
	}

	after := NewDiscoverer(nil)
	for name, d := range map[string]*Discoverer{"before": before, "after": after} {
		t.Run(name, func(t *testing.T) {
			hitsBefore := counter.hits.Load()
			_, err := d.DiscoverFromResource(context.Background(), resourceOrigin)
			assertBlockedIssuer(t, err, nil)
			if got := counter.hits.Load(); got != hitsBefore {
				t.Errorf("a default Discoverer contacted the issuer (%d new hit(s)); the permissive policy leaked", got-hitsBefore)
			}
		})
	}
}

// TestIssuerPolicyDoesNotReachCallerNamedOrigins is the regression guard on the
// scope of the change. Hop 1 and Discover target origins the CALLER named, so
// they keep the caller's client and must still reach loopback with the policy
// at its default.
func TestIssuerPolicyDoesNotReachCallerNamedOrigins(t *testing.T) {
	t.Run("discover", func(t *testing.T) {
		var origin string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issuer":"` + origin + `","token_endpoint":"` + origin + `/oauth/token"}`))
		}))
		defer srv.Close()
		origin = srv.URL

		cfg, err := NewDiscoverer(srv.Client()).Discover(context.Background(), origin)
		if err != nil {
			t.Fatalf("Discover() on loopback error = %v, want nil", err)
		}
		if cfg.Issuer != origin {
			t.Errorf("Issuer = %q, want %q", cfg.Issuer, origin)
		}
	})

	// Hop 1 of DiscoverFromResource: reaching a loopback resource server is
	// proven by the soft fallback, which is only reachable once hop 1 has
	// returned parsed metadata.
	t.Run("hop1", func(t *testing.T) {
		resourceOrigin := resourceAdvertising(t, LaunchpadBaseURL)

		result, err := NewDiscoverer(nil).DiscoverFromResource(context.Background(), resourceOrigin)
		if err != nil {
			t.Fatalf("DiscoverFromResource() error = %v, want nil", err)
		}
		if result.FallbackReason != FallbackNoASAdvertised {
			t.Errorf("fallback reason = %q, want %q — hop 1 did not reach the loopback resource server",
				result.FallbackReason, FallbackNoASAdvertised)
		}
	})

	t.Run("protected_resource", func(t *testing.T) {
		resourceOrigin := resourceAdvertising(t, LaunchpadBaseURL)

		md, err := NewDiscoverer(nil).DiscoverProtectedResource(context.Background(), resourceOrigin)
		if err != nil {
			t.Fatalf("DiscoverProtectedResource() on loopback error = %v, want nil", err)
		}
		if md.Resource != resourceOrigin {
			t.Errorf("Resource = %q, want %q", md.Resource, resourceOrigin)
		}
	})
}
