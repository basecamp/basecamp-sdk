package basecamp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// withHTTPClientRefused is the stable NewClient panic. Callers that passed
// WithHTTPClient still compile; they hit this string until they switch.
const withHTTPClientRefused = "basecamp: WithHTTPClient is no longer accepted; use WithTransport and WithTimeout"

// TestNewClient_RejectsWithHTTPClient is red while NewClient still returns
// after WithHTTPClient: the option is stored and then overwritten, so recover
// sees nil. It is green once construction panics with withHTTPClientRefused
// for both a nil and a non-nil *http.Client. The caller's CheckRedirect, Jar,
// and Transport are not copied onto a client NewClient returns, because
// NewClient does not return.
func TestNewClient_RejectsWithHTTPClient(t *testing.T) {
	caller := &http.Client{
		Timeout: 5 * time.Second,
		Jar:     refusingJar{},
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("caller transport ran")
			return nil, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			t.Fatal("caller CheckRedirect ran")
			return nil
		},
	}

	tests := []struct {
		name   string
		client *http.Client
	}{
		{name: "non-nil client", client: caller},
		{name: "nil client", client: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("NewClient returned after WithHTTPClient; want a panic")
				}
				if r != withHTTPClientRefused {
					t.Fatalf("panic = %v, want %q", r, withHTTPClientRefused)
				}
			}()

			NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
				&StaticTokenProvider{Token: "token"},
				WithHTTPClient(tt.client))
		})
	}
}

// TestNewClient_TransportOrder locks the layering NewClient already builds:
// *loggingTransport outermost, then any WithTransportWrapper, then the
// WithTransport RoundTripper or the default transport. A request records
// the inner two so the types are not only nested but actually called.
func TestNewClient_TransportOrder(t *testing.T) {
	t.Run("wrapper then transport", func(t *testing.T) {
		var seen []string
		base := &orderTransport{name: "transport", next: http.DefaultTransport, seen: &seen}
		wrapper := &orderWrapper{seen: &seen}
		client := newLayerClient(t, WithTransport(base), WithTransportWrapper(wrapper), WithTimeout(5*time.Second))

		if client.httpClient.Timeout != 5*time.Second {
			t.Fatalf("Timeout = %s, want 5s", client.httpClient.Timeout)
		}
		if client.httpClient.Jar != nil {
			t.Fatal("Jar = non-nil, want the client NewClient builds (no caller jar)")
		}
		outer, ok := client.httpClient.Transport.(*loggingTransport)
		if !ok {
			t.Fatalf("outermost = %T, want *loggingTransport", client.httpClient.Transport)
		}
		if outer.client != client {
			t.Fatal("logging transport is not bound to the client NewClient returned")
		}
		mid, ok := outer.inner.(*orderTransport)
		if !ok || mid.name != "wrapper" {
			t.Fatalf("under logging = %#v, want the WithTransportWrapper result", outer.inner)
		}
		if mid.next != base {
			t.Fatalf("under wrapper = %T, want the WithTransport value", mid.next)
		}
		if _, err := client.Get(context.Background(), "/ping"); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if strings.Join(seen, ",") != "wrapper,transport" {
			t.Fatalf("round-trip order = %v, want wrapper then transport", seen)
		}
	})

	t.Run("transport only", func(t *testing.T) {
		var seen []string
		base := &orderTransport{name: "transport", next: http.DefaultTransport, seen: &seen}
		client := newLayerClient(t, WithTransport(base))

		outer, ok := client.httpClient.Transport.(*loggingTransport)
		if !ok {
			t.Fatalf("outermost = %T, want *loggingTransport", client.httpClient.Transport)
		}
		if outer.inner != base {
			t.Fatalf("under logging = %T, want the WithTransport value", outer.inner)
		}
		if _, err := client.Get(context.Background(), "/ping"); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if strings.Join(seen, ",") != "transport" {
			t.Fatalf("round-trip order = %v, want transport", seen)
		}
	})

	t.Run("default transport", func(t *testing.T) {
		client := NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
			&StaticTokenProvider{Token: "token"})
		outer, ok := client.httpClient.Transport.(*loggingTransport)
		if !ok {
			t.Fatalf("outermost = %T, want *loggingTransport", client.httpClient.Transport)
		}
		if _, ok := outer.inner.(*http.Transport); !ok {
			t.Fatalf("under logging = %T, want *http.Transport", outer.inner)
		}
		if client.httpClient.Timeout != DefaultTimeout {
			t.Fatalf("Timeout = %s, want default %s", client.httpClient.Timeout, DefaultTimeout)
		}
	})
}

// TestNewClient_CheckRedirectStripsCrossOriginAuthorization locks the policy
// a copied caller CheckRedirect could drop. The next hop is compared with the
// original request (via[0]), which is what NewClient installs today.
func TestNewClient_CheckRedirectStripsCrossOriginAuthorization(t *testing.T) {
	client := NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
		&StaticTokenProvider{Token: "token"})
	if client.httpClient.CheckRedirect == nil {
		t.Fatal("CheckRedirect is nil")
	}

	original := redirectHop(t, "https://3.basecampapi.com/999/projects.json", "Bearer secret")

	cross := redirectHop(t, "https://evil.example/collect", "Bearer leaked")
	if err := client.httpClient.CheckRedirect(cross, []*http.Request{original}); err != nil {
		t.Fatalf("cross-origin hop: %v", err)
	}
	if got := cross.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q after a cross-origin hop, want it deleted", got)
	}

	// A later hop that returns to the original origin keeps the header.
	// Stripping against the previous hop instead of via[0] would fail this.
	home := redirectHop(t, "https://3.basecampapi.com/999/projects/2.json", "Bearer kept")
	foreign := redirectHop(t, "https://evil.example/bounce", "Bearer secret")
	if err := client.httpClient.CheckRedirect(home, []*http.Request{original, foreign}); err != nil {
		t.Fatalf("return hop: %v", err)
	}
	if got := home.Header.Get("Authorization"); got != "Bearer kept" {
		t.Fatalf("Authorization = %q on a same-origin hop, want it kept", got)
	}

	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = original
	}
	if err := client.httpClient.CheckRedirect(home, via); err == nil {
		t.Fatal("11th hop returned nil, want the 10-redirect stop")
	}
}

type orderTransport struct {
	name string
	next http.RoundTripper
	seen *[]string
}

func (o *orderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*o.seen = append(*o.seen, o.name)
	return o.next.RoundTrip(r)
}

type orderWrapper struct {
	seen *[]string
}

func (w *orderWrapper) WrapTransport(inner http.RoundTripper) http.RoundTripper {
	return &orderTransport{name: "wrapper", next: inner, seen: w.seen}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// refusingJar is a non-nil jar so a composed client would be observable.
type refusingJar struct{}

func (refusingJar) SetCookies(*url.URL, []*http.Cookie) {}
func (refusingJar) Cookies(*url.URL) []*http.Cookie     { return nil }

func newLayerClient(t *testing.T, opts ...ClientOption) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	return NewClient(cfg, &StaticTokenProvider{Token: "token"}, opts...)
}

func redirectHop(t *testing.T, rawURL, authorization string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Authorization", authorization)
	return req
}
