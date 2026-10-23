package basecamp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// etagServer serves two pages of cards with fixed ETags and answers 304 to a
// matching If-None-Match, keeping Link and X-Total-Count on the 304 as bc3 does.
type etagServer struct {
	t   *testing.T
	url string

	mu          sync.Mutex
	ifNoneMatch []string // per request, in order: "page<N>:<If-None-Match>"
	version     int
}

func (s *etagServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	s.mu.Lock()
	s.ifNoneMatch = append(s.ifNoneMatch, "page"+page+":"+r.Header.Get("If-None-Match"))
	version := s.version
	s.mu.Unlock()

	etag := fmt.Sprintf(`"page%s-v%d"`, page, version)
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Total-Count", "3")
	if page == "1" {
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=2>; rel="next"`, s.url, r.URL.Path))
	}

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch page {
	case "1":
		fmt.Fprintf(w, `[{"id":1,"title":"one v%d"},{"id":2,"title":"two v%d"}]`, version, version)
	default:
		fmt.Fprintf(w, `[{"id":3,"title":"three v%d"}]`, version)
	}
}

func (s *etagServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.ifNoneMatch...)
	s.ifNoneMatch = nil
	return out
}

func (s *etagServer) change() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
}

func newETagServer(t *testing.T) *etagServer {
	t.Helper()
	s := &etagServer{t: t}
	server := httptest.NewServer(s)
	t.Cleanup(server.Close)
	s.url = server.URL
	return s
}

func etagCardsService(t *testing.T, s *etagServer, token string, opts ...ClientOption) *CardsService {
	t.Helper()
	cfg := DefaultConfig()
	cfg.BaseURL = s.url
	return NewClient(cfg, &StaticTokenProvider{Token: token}, opts...).ForAccount("99999").Cards()
}

func cardTitles(t *testing.T, result *CardListResult, err error) []string {
	t.Helper()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	titles := make([]string, len(result.Cards))
	for i, c := range result.Cards {
		titles[i] = c.Title
	}
	return titles
}

func assertStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s:\n got %q\nwant %q", label, got, want)
	}
}

func TestETagDoer_ListRevalidatesThePageItFetchedFirst(t *testing.T) {
	s := newETagServer(t)
	cards := etagCardsService(t, s, "token", WithCache(NewCache(t.TempDir())))
	ctx := context.Background()

	first, err := cards.List(ctx, 7, nil)
	assertStrings(t, "first list", cardTitles(t, first, err), []string{"one v0", "two v0", "three v0"})
	assertStrings(t, "first requests", s.requests(), []string{"page1:", "page2:"})

	// Both pages are unchanged, so both come back 304 and from the cache. Page 1
	// keeps its Link off the 304, so pagination still reaches page 2.
	again, err := cards.List(ctx, 7, nil)
	assertStrings(t, "revalidated list", cardTitles(t, again, err), []string{"one v0", "two v0", "three v0"})
	assertStrings(t, "revalidating requests", s.requests(), []string{`page1:"page1-v0"`, `page2:"page2-v0"`})
	if again.Meta.TotalCount != 3 {
		t.Errorf("TotalCount from a 304 = %d, want 3", again.Meta.TotalCount)
	}

	// A change answers 200 again, and that response replaces the cached one.
	s.change()
	changed, err := cards.List(ctx, 7, nil)
	assertStrings(t, "changed list", cardTitles(t, changed, err), []string{"one v1", "two v1", "three v1"})
	assertStrings(t, "requests after a change", s.requests(), []string{`page1:"page1-v0"`, `page2:"page2-v0"`})

	_, err = cards.List(ctx, 7, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	assertStrings(t, "requests after re-caching", s.requests(), []string{`page1:"page1-v1"`, `page2:"page2-v1"`})
}

func TestETagDoer_SinglePageFetchRevalidates(t *testing.T) {
	s := newETagServer(t)
	cards := etagCardsService(t, s, "token", WithCache(NewCache(t.TempDir())))
	ctx := context.Background()

	_, err := cards.List(ctx, 7, &CardListOptions{Page: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	again, err := cards.List(ctx, 7, &CardListOptions{Page: 1})
	assertStrings(t, "revalidated page", cardTitles(t, again, err), []string{"one v0", "two v0"})
	assertStrings(t, "requests", s.requests(), []string{"page1:", `page1:"page1-v0"`})
	if !again.Meta.Truncated {
		t.Error("Truncated from a 304 with a next Link = false, want true")
	}
}

func TestETagDoer_WithoutACacheRequestsAreUnconditional(t *testing.T) {
	s := newETagServer(t)
	cards := etagCardsService(t, s, "token")
	ctx := context.Background()

	for range 2 {
		if _, err := cards.List(ctx, 7, nil); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	assertStrings(t, "requests", s.requests(), []string{"page1:", "page2:", "page1:", "page2:"})
}

func TestETagDoer_CredentialsDoNotShareEntries(t *testing.T) {
	s := newETagServer(t)
	cache := NewCache(t.TempDir())
	ctx := context.Background()

	if _, err := etagCardsService(t, s, "token-a", WithCache(cache)).List(ctx, 7, nil); err != nil {
		t.Fatalf("List: %v", err)
	}
	s.requests()

	if _, err := etagCardsService(t, s, "token-b", WithCache(cache)).List(ctx, 7, nil); err != nil {
		t.Fatalf("List: %v", err)
	}
	assertStrings(t, "another credential's requests", s.requests(), []string{"page1:", "page2:"})
}

func TestETagDoer_ARemovedEntryIsNotRevalidated(t *testing.T) {
	s := newETagServer(t)
	dir := t.TempDir()
	cards := etagCardsService(t, s, "token", WithCache(NewCache(dir)))
	ctx := context.Background()

	if _, err := cards.List(ctx, 7, &CardListOptions{Page: 1}); err != nil {
		t.Fatalf("List: %v", err)
	}
	s.requests()
	if err := os.RemoveAll(filepath.Join(dir, "responses")); err != nil {
		t.Fatal(err)
	}

	again, err := cards.List(ctx, 7, &CardListOptions{Page: 1})
	assertStrings(t, "list", cardTitles(t, again, err), []string{"one v0", "two v0"})
	assertStrings(t, "requests", s.requests(), []string{"page1:"})
}

func TestETagDoer_SingleResourceGetRevalidates(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("If-None-Match"))
		mu.Unlock()
		w.Header().Set("ETag", `"card-v0"`)
		if r.Header.Get("If-None-Match") == `"card-v0"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":42,"title":"cached card"}`)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cards := NewClient(cfg, &StaticTokenProvider{Token: "token"}, WithCache(NewCache(t.TempDir()))).ForAccount("99999").Cards()

	for range 2 {
		card, err := cards.Get(context.Background(), 42)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if card.Title != "cached card" {
			t.Errorf("Title = %q, want %q", card.Title, "cached card")
		}
	}
	assertStrings(t, "requests", seen, []string{"", `"card-v0"`})
}

// sessionCookieAuth authenticates without an Authorization header, as a cookie or
// API-key strategy would.
type sessionCookieAuth struct{ session string }

func (a sessionCookieAuth) Authenticate(_ context.Context, req *http.Request) error {
	req.AddCookie(&http.Cookie{Name: "session", Value: a.session})
	return nil
}

func TestETagDoer_StrategiesWithoutAnAuthorizationHeaderAreNotCached(t *testing.T) {
	s := newETagServer(t)
	cache := NewCache(t.TempDir())
	ctx := context.Background()

	for _, session := range []string{"a", "b", "a"} {
		cards := etagCardsService(t, s, "unused", WithCache(cache), WithAuthStrategy(sessionCookieAuth{session}))
		if _, err := cards.List(ctx, 7, nil); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	// With nothing to scope a key by, session b would otherwise have revalidated
	// against, and been served, session a's cached pages.
	assertStrings(t, "requests", s.requests(), []string{"page1:", "page2:", "page1:", "page2:", "page1:", "page2:"})

	if _, err := os.Stat(filepath.Join(cache.dir, "responses")); !os.IsNotExist(err) {
		t.Errorf("expected nothing cached, stat responses: %v", err)
	}
}

func TestETagDoer_ABodyTooLargeToCacheIsReturnedWholeAndNotCached(t *testing.T) {
	title := strings.Repeat("x", int(MaxResponseBodyBytes))
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("If-None-Match"))
		mu.Unlock()
		w.Header().Set("ETag", `"huge"`)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":42,"title":%q}`, title)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cards := NewClient(cfg, &StaticTokenProvider{Token: "token"}, WithCache(NewCache(t.TempDir()))).ForAccount("99999").Cards()

	for range 2 {
		card, err := cards.Get(context.Background(), 42)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(card.Title) != len(title) {
			t.Fatalf("title length = %d, want %d", len(card.Title), len(title))
		}
	}
	assertStrings(t, "requests", seen, []string{"", ""})
}

// Two Cache values on one directory stand in for two processes sharing a cache
// (or two clients built with CacheEnabled): they share no lock, only files.
func TestCache_EntryNeverPairsOneResponsesETagWithAnothersBody(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Rename isn't atomic on Windows, and a reader's open handle can make it fail")
	}

	dir := t.TempDir()
	const key = "k"
	if err := NewCache(dir).Set(key, []byte("body-0"), "0"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var writes [2]atomic.Int64
	stop := make(chan struct{})
	for w := range writes {
		writer := NewCache(dir)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				v := fmt.Sprintf("%d-%d", w, i)
				if err := writer.Set(key, []byte("body-"+v), v); err != nil {
					t.Errorf("Set: %v", err)
					return
				}
				writes[w].Add(1)
			}
		}()
	}

	// Read only once both writers are publishing, and check that each kept
	// publishing while the reads ran.
	deadline := time.Now().Add(10 * time.Second)
	for (writes[0].Load() == 0 || writes[1].Load() == 0) && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	before := [2]int64{writes[0].Load(), writes[1].Load()}

	reader := NewCache(dir)
	torn := 0
	for range 2000 {
		etag, body := reader.entry(key)
		if string(body) != "body-"+etag {
			torn++
		}
	}
	during := [2]int64{writes[0].Load() - before[0], writes[1].Load() - before[1]}
	close(stop)
	wg.Wait()

	if during[0] == 0 || during[1] == 0 {
		t.Fatalf("writes during the reads = %v, want both writers publishing", during)
	}
	if torn > 0 {
		t.Errorf("%d of 2000 reads paired an ETag with another response's body", torn)
	}
	if etag, body := reader.entry(key); string(body) != "body-"+etag {
		t.Errorf("the writers left %q beside %q", etag, body)
	}
}
