package generated_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

const (
	testToken     = "sentinel-token"
	testAccountID = "1"
	accountPath   = "/" + testAccountID + "/account.json"
)

// recorder records the Authorization header of every request a test server
// serves, in arrival order.
type recorder struct {
	mu   sync.Mutex
	seen []string
}

func (r *recorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, req.Header.Get("Authorization"))
}

func (r *recorder) headers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seen...)
}

func newServer(t *testing.T, rec *recorder, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func redirectTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accountPath {
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		okJSON(w, r)
	}
}

// asLocalhost readdresses an httptest server (bound to 127.0.0.1) as
// "localhost", so a redirect to it crosses the hostname boundary that
// net/http's shouldCopyHeaderOnRedirect compares.
func asLocalhost(t *testing.T, server *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return "http://localhost:" + u.Port()
}

func withAuth() generated.ClientOption {
	return generated.WithAuthTransport(&generated.StaticTokenProvider{Token: testToken}, "auth-transport-test")
}

// newClient builds a client from opts in the given order, so each test states
// where WithAuthTransport sits relative to the options it interacts with.
func newClient(t *testing.T, server string, opts ...generated.ClientOption) *generated.Client {
	t.Helper()
	client, err := generated.NewClient(server, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func getAccount(t *testing.T, client *generated.Client) {
	t.Helper()
	resp, err := client.GetAccount(context.Background(), testAccountID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	resp.Body.Close()
}

func assertHeaders(t *testing.T, name string, rec *recorder, want ...string) {
	t.Helper()
	got := rec.headers()
	if !slices.Equal(got, want) {
		t.Errorf("%s saw Authorization %q, want %q", name, got, want)
	}
}

func TestAuthTransport_CrossOriginRedirectDropsAuthorization(t *testing.T) {
	var apiRec, foreignRec recorder
	foreign := newServer(t, &foreignRec, okJSON)
	api := newServer(t, &apiRec, redirectTo(asLocalhost(t, foreign)+"/steal"))

	getAccount(t, newClient(t, api.URL, withAuth()))

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken)
	assertHeaders(t, "foreign host", &foreignRec, "")
}

// Same hostname, different port: net/http copies Authorization across this
// redirect on its own, so only a strict same-origin check keeps it off.
func TestAuthTransport_CrossPortRedirectDropsAuthorization(t *testing.T) {
	var apiRec, foreignRec recorder
	foreign := newServer(t, &foreignRec, okJSON)
	api := newServer(t, &apiRec, redirectTo(foreign.URL+"/steal"))

	getAccount(t, newClient(t, api.URL, withAuth()))

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken)
	assertHeaders(t, "foreign port", &foreignRec, "")
}

func TestAuthTransport_SameOriginRedirectKeepsAuthorization(t *testing.T) {
	var apiRec recorder
	api := newServer(t, &apiRec, redirectTo("/moved"))

	getAccount(t, newClient(t, api.URL, withAuth()))

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken, "Bearer "+testToken)
}

func TestAuthTransport_OriginTracksBaseURLSetAfterAuth(t *testing.T) {
	var apiRec recorder
	api := newServer(t, &apiRec, okJSON)

	getAccount(t, newClient(t, "http://stale.invalid", withAuth(), generated.WithBaseURL(api.URL)))

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken)
}

func directGet(t *testing.T, transport *generated.AuthTransport, target string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestAuthTransport_NilOriginAttachesNothing(t *testing.T) {
	var apiRec recorder
	api := newServer(t, &apiRec, okJSON)

	directGet(t, &generated.AuthTransport{
		TokenProvider: &generated.StaticTokenProvider{Token: testToken},
	}, api.URL+accountPath)

	assertHeaders(t, "api", &apiRec, "")
}

func TestAuthTransport_StaticOriginBoundsAttachment(t *testing.T) {
	var apiRec, otherRec recorder
	api := newServer(t, &apiRec, okJSON)
	other := newServer(t, &otherRec, okJSON)
	transport := &generated.AuthTransport{
		TokenProvider: &generated.StaticTokenProvider{Token: testToken},
		Origin:        generated.StaticOrigin(api.URL),
	}

	directGet(t, transport, api.URL+accountPath)
	directGet(t, transport, other.URL+accountPath)

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken)
	assertHeaders(t, "other origin", &otherRec, "")
}

func TestWithAuthTransport_PreservesCallerClient(t *testing.T) {
	var apiRec recorder
	api := newServer(t, &apiRec, redirectTo("/moved"))
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	errNoRedirects := errors.New("caller refuses redirects")
	caller := &http.Client{
		Timeout: 7 * time.Second,
		Jar:     jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errNoRedirects
		},
	}

	client := newClient(t, api.URL, generated.WithHTTPClient(caller), withAuth(), generated.WithRetryConfig(generated.RetryConfig{MaxRetries: 1}))

	httpClient, ok := client.Client.(*http.Client)
	if !ok {
		t.Fatalf("client is %T, want *http.Client", client.Client)
	}
	if httpClient.Timeout != caller.Timeout {
		t.Errorf("Timeout = %v, want %v", httpClient.Timeout, caller.Timeout)
	}
	if httpClient.Jar != jar {
		t.Error("Jar was not preserved")
	}
	resp, err := client.GetAccount(context.Background(), testAccountID)
	if err == nil {
		resp.Body.Close()
	}
	if !errors.Is(err, errNoRedirects) {
		t.Errorf("caller CheckRedirect was not preserved: err = %v", err)
	}
	if caller.Transport != nil {
		t.Error("caller's http.Client was mutated")
	}
}

// A caller-supplied http.Client with no CheckRedirect gets the same-origin
// strip, which also covers an Authorization header a request editor placed on
// the outer request.
func TestWithAuthTransport_DefaultCheckRedirectStripsOuterAuthorization(t *testing.T) {
	var apiRec, foreignRec recorder
	foreign := newServer(t, &foreignRec, okJSON)
	api := newServer(t, &apiRec, redirectTo(foreign.URL+"/steal"))
	outerAuth := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer outer")
		return nil
	}

	client := newClient(t, api.URL, generated.WithHTTPClient(&http.Client{}), withAuth(), generated.WithRequestEditorFn(outerAuth))
	getAccount(t, client)

	assertHeaders(t, "api", &apiRec, "Bearer "+testToken)
	assertHeaders(t, "foreign port", &foreignRec, "")
}
