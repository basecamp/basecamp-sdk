package basecamp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// These tests pin the generated client's request-body replay contract for
// idempotent raw *WithBody operations. buildRequest recreates each attempt's
// request from the caller's single-use io.Reader, so without replay support a
// retry ships an empty (or mid-stream) body. doWithRetry snapshots the first
// finalized request's body once and replays THAT body on every later attempt.
// They drive the loop through an exported idempotent *WithBody op
// (UpdateMessageWithBody, a PUT flagged idempotent), staying outside
// pkg/generated per the repo rule that generated code carries no hand-written
// tests.

// attemptRecord captures what a fake transport observed for one attempt.
type attemptRecord struct {
	body        []byte // bytes read from req.Body on the wire
	getBodyNil  bool   // whether req.GetBody was nil
	getBodyData []byte // bytes produced by invoking req.GetBody (if present)
	contentLen  int64  // req.ContentLength
}

// recordingDoer reads each request's body (as a real transport would), records
// what it saw, and replies with the status returned by statusFor(attemptNumber).
func recordingDoer(records *[]attemptRecord, statusFor func(attempt int) int) generated.HttpRequestDoer {
	return doerFunc(func(req *http.Request) (*http.Response, error) {
		rec := attemptRecord{contentLen: req.ContentLength, getBodyNil: req.GetBody == nil}
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil {
				rec.getBodyData, _ = io.ReadAll(rc)
				_ = rc.Close()
			}
		}
		if req.Body != nil {
			rec.body, _ = io.ReadAll(req.Body)
			// net/http's Transport closes Request.Body after reading it; model
			// that here so the tests exercise realistic Close behavior.
			_ = req.Body.Close()
		}
		*records = append(*records, rec)
		return &http.Response{
			StatusCode: statusFor(len(*records)),
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
}

// newBodyReplayClient builds a generated client with the given transport and a
// fast idempotent retry config allowing maxAttempts.
func newBodyReplayClient(t *testing.T, doer generated.HttpRequestDoer, maxAttempts int) *generated.Client {
	t.Helper()
	client, err := generated.NewClient(
		"https://example.com",
		generated.WithHTTPClient(doer),
		generated.WithRetryConfig(fastRetryConfig(maxAttempts)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// plainReader is an io.Reader whose concrete type is not one http.NewRequest
// recognizes, so no GetBody snapshot is synthesized for it (the no-GetBody,
// single-use stream path). It is intentionally not an io.ReadCloser.
type plainReader struct{ r *bytes.Reader }

func newPlainReader(b []byte) *plainReader        { return &plainReader{r: bytes.NewReader(b)} }
func (p *plainReader) Read(b []byte) (int, error) { return p.r.Read(b) }

// closeTrackingReader is an io.ReadCloser that records whether Close was called,
// used to prove a discarded retry-editor body is closed rather than leaked.
type closeTrackingReader struct {
	r      io.Reader
	closed bool
}

func (c *closeTrackingReader) Read(b []byte) (int, error) { return c.r.Read(b) }
func (c *closeTrackingReader) Close() error               { c.closed = true; return nil }

// --- Red / contract proofs (must fail pre-fix, pass post-fix) ---

// (1) A 503→200 idempotent *WithBody with a single-use reader must ship the FULL
// body on the retry, and the finalized retry request must expose a working
// GetBody (307/308-redirect safety).
func TestWithBodyReplay_RetryShipsFullBody(t *testing.T) {
	const payload = `{"content":"the full replayable message body"}`
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 2 {
		t.Fatalf("made %d attempts, want 2", len(records))
	}
	if got := string(records[1].body); got != payload {
		t.Errorf("retry shipped body %q, want the full body %q", got, payload)
	}
	if records[1].getBodyNil {
		t.Error("retry request has a nil GetBody; a 307/308 redirect could not replay the body")
	}
	if got := string(records[1].getBodyData); got != payload {
		t.Errorf("retry request GetBody produced %q, want %q", got, payload)
	}
}

// (2) A partial-read network failure (transport reads a prefix, then errors)
// must restart at byte 0 on the next attempt, not resume mid-body.
func TestWithBodyReplay_PartialReadRestartsAtByteZero(t *testing.T) {
	const payload = `{"content":"partial-read restart proof body"}`
	var attempt2Body []byte
	var attempts int
	doer := doerFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			// Read only a prefix, then fail like a mid-stream network error.
			_, _ = io.ReadFull(req.Body, make([]byte, 5))
			_ = req.Body.Close()
			return nil, io.ErrUnexpectedEOF
		}
		attempt2Body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if got := string(attempt2Body); got != payload {
		t.Errorf("after a partial-read failure the retry shipped %q, want the full body %q", got, payload)
	}
}

// (3) A body with no GetBody snapshot is an unrewindable stream: it is NOT
// buffered or retried (which would risk unbounded memory or an uninterruptible
// read). The op makes a single attempt shipping the full body once, even under a
// retryable 503, rather than retrying with a drained body.
func TestWithBodyReplay_NoGetBodyStreamSingleAttempt(t *testing.T) {
	const payload = `{"content":"an unrewindable stream is sent once, not retried"}`
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusServiceUnavailable })
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/octet-stream", newPlainReader([]byte(payload)))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("a no-GetBody stream made %d attempts, want exactly 1 (unrewindable, not replayed)", len(records))
	}
	if got := string(records[0].body); got != payload {
		t.Errorf("the single attempt shipped %q, want the full body %q", got, payload)
	}
}

// (4) The retry attempt's ContentLength must reflect the replayed body, not the
// drained reader's zero length.
func TestWithBodyReplay_RetryContentLength(t *testing.T) {
	const payload = `{"content":"content-length restore proof"}`
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 2 {
		t.Fatalf("made %d attempts, want 2", len(records))
	}
	if want := int64(len(payload)); records[1].contentLen != want {
		t.Errorf("retry ContentLength = %d, want %d", records[1].contentLen, want)
	}
}

// (5) Attempt-1 fidelity: an editor that REPLACES req.Body (as a compress/
// encrypt editor does) without updating GetBody has its body sent verbatim on
// attempt 1 — never overwritten by the builder's stale GetBody. Because the
// finalized bytes can no longer be reproduced, the request is sent once (not
// retried with the wrong body).
func TestWithBodyReplay_EditorReplacingBodyIsSentThenNotRetried(t *testing.T) {
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusServiceUnavailable })
	client := newBodyReplayClient(t, doer, 3)

	editor := func(_ context.Context, req *http.Request) error {
		req.Body = io.NopCloser(strings.NewReader("EDITOR-TRANSFORMED-BODY"))
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("caller-original"), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1 (a replaced body can't be reproduced → single attempt)", len(records))
	}
	if got := string(records[0].body); got != "EDITOR-TRANSFORMED-BODY" {
		t.Errorf("attempt 1 shipped %q, want the editor-finalized body %q (attempt-1 fidelity, not the builder GetBody)", got, "EDITOR-TRANSFORMED-BODY")
	}
}

// (6) Even when a body-replacing editor ALSO installs a matching GetBody, the
// request is conservatively sent once: the SDK cannot verify a replaced body's
// GetBody actually reproduces it, so it does not risk a wrong-body retry. The
// editor's body is still sent verbatim on attempt 1.
func TestWithBodyReplay_EditorReplacingBodyAndGetBodyIsSentThenNotRetried(t *testing.T) {
	const editorBody = "EDITOR-BODY-WITH-GETBODY"
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusServiceUnavailable })
	client := newBodyReplayClient(t, doer, 3)

	editor := func(_ context.Context, req *http.Request) error {
		req.Body = io.NopCloser(strings.NewReader(editorBody))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(editorBody)), nil }
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("caller"), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1 (a replaced body is not retried, even with a new GetBody)", len(records))
	}
	if got := string(records[0].body); got != editorBody {
		t.Errorf("attempt 1 shipped %q, want the editor body %q", got, editorBody)
	}
}

// (7) In-place mutation is undetectable: an editor that Resets the caller's
// reader (rather than replacing req.Body) still has its finalized body sent on
// attempt 1. Since the builder's body reference is unchanged, the request is
// (best-effort, net/http-redirect-consistent) retried via GetBody — the point
// under test is that attempt 1 ships the editor's finalized bytes, not the
// builder's stale snapshot.
func TestWithBodyReplay_EditorInPlaceMutationSentOnAttempt1(t *testing.T) {
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	caller := strings.NewReader("CALLER-ORIGINAL-BODY")
	editor := func(_ context.Context, _ *http.Request) error {
		caller.Reset("EDITOR-FINALIZED-BODY")
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", caller, editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if got := string(records[0].body); got != "EDITOR-FINALIZED-BODY" {
		t.Errorf("attempt 1 shipped %q, want the editor's in-place-mutated body %q (attempt-1 fidelity)", got, "EDITOR-FINALIZED-BODY")
	}
}

// (8) A large caller body with a native GetBody (a *bytes.Reader) replays fully
// on retry, regardless of size — the GetBody path never buffers, so it is not
// demoted to a single attempt.
func TestWithBodyReplay_LargeGetBodyBodyRetriesFully(t *testing.T) {
	payload := bytes.Repeat([]byte("z"), (1<<20)+512) // large; size is irrelevant to GetBody replay
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/octet-stream", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 2 {
		t.Fatalf("made %d attempts, want 2 (a large body with a valid GetBody must not be demoted)", len(records))
	}
	if !bytes.Equal(records[1].body, payload) {
		t.Errorf("retry shipped %d body bytes, want the full %d", len(records[1].body), len(payload))
	}
}

// (9) A well-behaved Digest/HMAC editor reads the body to sign via req.GetBody
// (not draining req.Body), and the digest it writes must match the body actually
// sent on both attempts. Attempt 1 sends req.Body and the retry replays GetBody;
// for an unmodified body these are byte-identical, so the digest matches.
func TestWithBodyReplay_SigningEditorViaGetBodyMatchesSentBody(t *testing.T) {
	const payload = `{"content":"digest must match the body actually sent, on every attempt"}`
	var attempts int32
	var mismatches []string
	doer := doerFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&attempts, 1)
		body, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		sum := sha256.Sum256(body)
		if want, got := hex.EncodeToString(sum[:]), req.Header.Get("X-Body-Digest"); want != got {
			mismatches = append(mismatches, fmt.Sprintf("attempt %d: sent %d body bytes with sha256 %s, but the editor's digest header was %s", n, len(body), want, got))
		}
		status := http.StatusOK
		if n == 1 {
			status = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	client := newBodyReplayClient(t, doer, 3)

	editor := func(_ context.Context, req *http.Request) error {
		if req.GetBody == nil {
			return fmt.Errorf("no GetBody to sign")
		}
		rc, err := req.GetBody()
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		sum := sha256.Sum256(b)
		req.Header.Set("X-Body-Digest", hex.EncodeToString(sum[:]))
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("made %d attempts, want 2", attempts)
	}
	if len(mismatches) > 0 {
		t.Errorf("editor digest did not match the sent body:\n  %s", strings.Join(mismatches, "\n  "))
	}
}

// (10) A GetBody snapshot failure must not fail an otherwise-sendable request:
// GetBody is only needed for replay, so a snapshot error disables retries and
// the request is still sent once with its valid body.
func TestWithBodyReplay_GetBodyFailureFallsBackToSingleAttempt(t *testing.T) {
	const payload = "VALID-BODY"
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusOK })
	client := newBodyReplayClient(t, doer, 3)

	editor := func(_ context.Context, req *http.Request) error {
		// A valid req.Body remains, but GetBody fails to snapshot.
		req.GetBody = func() (io.ReadCloser, error) { return nil, fmt.Errorf("snapshot failed") }
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody returned %v — a GetBody snapshot failure must not fail an otherwise-sendable request", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1 (GetBody failure disables retries)", len(records))
	}
	if got := string(records[0].body); got != payload {
		t.Errorf("shipped %q, want the valid body %q", got, payload)
	}
}

// (11) If an editor errors on a retry after installReplay has opened a fresh
// body (a custom GetBody may hold a file/pipe), that body must be closed — not
// leaked.
func TestWithBodyReplay_ClosesInstalledBodyOnEditorError(t *testing.T) {
	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	client := newBodyReplayClient(t, doer, 3)

	// Attempt 1 installs a custom GetBody returning close-tracking bodies; the
	// retry editor fails after installReplay has opened one of them.
	var created []*closeTrackingReader
	var editorCalls int
	editor := func(_ context.Context, req *http.Request) error {
		editorCalls++
		if editorCalls == 1 {
			req.GetBody = func() (io.ReadCloser, error) {
				ctr := &closeTrackingReader{r: strings.NewReader("REPLAY")}
				created = append(created, ctr)
				return ctr, nil
			}
			return nil
		}
		return fmt.Errorf("editor failure on retry")
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("caller"), editor)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected the retry editor error to propagate")
	}
	if len(created) == 0 {
		t.Fatal("custom GetBody was never invoked")
	}
	if last := created[len(created)-1]; !last.closed {
		t.Error("the replay body installed before the failing retry editor was not closed (leak)")
	}
}

// (12) A 503→200 with an EMPTY body (http.NoBody): a body-aware editor reads/
// signs the body on every attempt, so an explicitly empty body must be replayed
// as a non-nil http.NoBody rather than collapsed to nil (which would hand the
// retry editor a nil Body).
func TestWithBodyReplay_EmptyBodyReplayedNotNil(t *testing.T) {
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	var sawNil bool
	var editorCalls int
	editor := func(_ context.Context, req *http.Request) error {
		editorCalls++
		if req.Body == nil {
			sawNil = true
			return fmt.Errorf("retry editor observed nil Body")
		}
		_, _ = io.ReadAll(req.Body) // a body-aware editor reads/signs the (empty) body
		return nil
	}

	// strings.NewReader("") → http.NewRequest sets req.Body = http.NoBody.
	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(""), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if sawNil {
		t.Error("a body-aware editor observed a nil Body on retry; an empty body must be replayed as http.NoBody")
	}
	if len(records) != 2 {
		t.Fatalf("made %d attempts, want 2 (editor_calls=%d)", len(records), editorCalls)
	}
}

// (13) When a retry editor REPLACES req.Body, the replay body installed before
// the editor ran must not be orphaned — it is owned and closed regardless. Both
// the pre-editor body and the editor's replacement must end up closed.
func TestWithBodyReplay_RetryEditorReplacesBodyClosesPreEditor(t *testing.T) {
	var records []attemptRecord
	doer := recordingDoer(&records, func(attempt int) int {
		if attempt == 1 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	client := newBodyReplayClient(t, doer, 3)

	var created []*closeTrackingReader
	newTracked := func(s string) *closeTrackingReader {
		ctr := &closeTrackingReader{r: strings.NewReader(s)}
		created = append(created, ctr)
		return ctr
	}
	var editorCalls int
	editor := func(_ context.Context, req *http.Request) error {
		editorCalls++
		if editorCalls == 1 {
			// Make the replay source produce trackable bodies (so the pre-editor
			// body A is observable, not an opaque NopCloser).
			req.GetBody = func() (io.ReadCloser, error) { return newTracked("REPLAY"), nil }
			return nil
		}
		// Retry: replace req.Body, orphaning the pre-editor replay body.
		req.Body = newTracked("EDITOR-REPLACEMENT")
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("caller"), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 2 {
		t.Fatalf("made %d attempts, want 2", len(records))
	}
	for i, ctr := range created {
		if !ctr.closed {
			t.Errorf("replay body #%d was leaked (not closed) — a retry editor replacing req.Body orphaned it", i)
		}
	}
}

// (14) Same as (13) but the retry editor also ERRORS after replacing req.Body:
// both the pre-editor body and the replacement must be closed on the error path.
func TestWithBodyReplay_RetryEditorReplacesBodyThenErrorsClosesBoth(t *testing.T) {
	// recordingDoer closes req.Body like a real transport, so attempt 1's body is
	// released the normal way; the retry editor then errors before Client.Do.
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusServiceUnavailable })
	client := newBodyReplayClient(t, doer, 3)

	var created []*closeTrackingReader
	newTracked := func(s string) *closeTrackingReader {
		ctr := &closeTrackingReader{r: strings.NewReader(s)}
		created = append(created, ctr)
		return ctr
	}
	var editorCalls int
	editor := func(_ context.Context, req *http.Request) error {
		editorCalls++
		if editorCalls == 1 {
			req.GetBody = func() (io.ReadCloser, error) { return newTracked("REPLAY"), nil }
			return nil
		}
		req.Body = newTracked("EDITOR-REPLACEMENT")
		return fmt.Errorf("editor failure after replacing the body")
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("caller"), editor)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected the retry editor error to propagate")
	}
	if len(created) == 0 {
		t.Fatal("custom GetBody was never invoked")
	}
	for i, ctr := range created {
		if !ctr.closed {
			t.Errorf("replay body #%d was leaked (not closed) on the replace-then-error path", i)
		}
	}
}

// (15) Empty-body transfer framing must be identical across attempts. net/http
// keys request framing off body identity: a wrapped http.NoBody has
// ContentLength 0 but is not the sentinel, so Request.outgoingLength() reports
// -1 and the retry would switch to chunked Transfer-Encoding. This runs through
// a REAL net/http Transport (httptest.Server) — a fake doer cannot observe
// transfer framing.
func TestWithBodyReplay_EmptyBodyIdenticalFramingAcrossRetries(t *testing.T) {
	type framing struct {
		contentLength    string
		transferEncoding string
		reqContentLength int64
	}
	var framings []framing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		framings = append(framings, framing{
			contentLength:    r.Header.Get("Content-Length"),
			transferEncoding: strings.Join(r.TransferEncoding, ","),
			reqContentLength: r.ContentLength,
		})
		if len(framings) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // retryable
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// A real *http.Client (no fake doer), so the real Transport computes framing.
	client, err := generated.NewClient(server.URL, generated.WithRetryConfig(fastRetryConfig(3)))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(framings) != 2 {
		t.Fatalf("made %d requests, want 2", len(framings))
	}
	if framings[0] != framings[1] {
		t.Errorf("empty-body transfer framing differs across attempts:\n  attempt 1: %+v\n  attempt 2: %+v", framings[0], framings[1])
	}
	if framings[1].transferEncoding == "chunked" {
		t.Error("retry used chunked Transfer-Encoding for an empty body while attempt 1 did not (http.NoBody sentinel not preserved)")
	}
}

// --- Controls (pass both pre- and post-fix) ---

// Happy path: a body with a native GetBody snapshot on a first-try success is
// sent once, unchanged.
func TestWithBodyReplay_HappyPathSingleAttempt(t *testing.T) {
	const payload = `{"content":"happy path body"}`
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusOK })
	client := newBodyReplayClient(t, doer, 3)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1", len(records))
	}
	if got := string(records[0].body); got != payload {
		t.Errorf("shipped body %q, want %q", got, payload)
	}
}

// Retry-ineligible (single attempt): a no-GetBody body is streamed untouched —
// not replayed — so no GetBody snapshot is synthesized.
func TestWithBodyReplay_RetryIneligibleBuffersNothing(t *testing.T) {
	const payload = `{"content":"streamed untouched body"}`
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusOK })
	// maxAttempts == 1 → retry-ineligible.
	client := newBodyReplayClient(t, doer, 1)

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", newPlainReader([]byte(payload)))
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1", len(records))
	}
	if !records[0].getBodyNil {
		t.Error("a retry-ineligible no-GetBody body had a GetBody synthesized; it should stream untouched")
	}
	if got := string(records[0].body); got != payload {
		t.Errorf("shipped body %q, want %q", got, payload)
	}
}

// Retry-ineligible with a native GetBody caller: a non-retrying call must send
// whatever body the editors finalized (here, an editor that replaced req.Body),
// NOT the caller's GetBody snapshot. The retry-ineligible check runs before the
// GetBody normalization, so no rewrite happens on a single-attempt request.
func TestWithBodyReplay_RetryIneligibleHonorsEditorBody(t *testing.T) {
	var records []attemptRecord
	doer := recordingDoer(&records, func(int) int { return http.StatusOK })
	// maxAttempts == 1 → retry-ineligible.
	client := newBodyReplayClient(t, doer, 1)

	editor := func(_ context.Context, req *http.Request) error {
		req.Body = io.NopCloser(strings.NewReader("EDITOR-BODY"))
		return nil
	}

	resp, err := client.UpdateMessageWithBody(context.Background(), "1", 100, "application/json", strings.NewReader("CALLER-BODY"), editor)
	if err != nil {
		t.Fatalf("UpdateMessageWithBody: %v", err)
	}
	_ = resp.Body.Close()

	if len(records) != 1 {
		t.Fatalf("made %d attempts, want 1", len(records))
	}
	if got := string(records[0].body); got != "EDITOR-BODY" {
		t.Errorf("retry-ineligible call shipped %q, want the editor body %q (no GetBody normalization when not retrying)", got, "EDITOR-BODY")
	}
}
