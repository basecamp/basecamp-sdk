package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The documents write surface: the merge-safe Update, the read-modify-write
// Edit, and the verbatim Replace. The read surface (Get/List/Create/Trash) and
// the Document type stay tested in vaults_test.go, alongside the vault.
//
// PUT /documents/{id} is a FULL REPLACE — BC3 rebuilds the recordable from only
// the permitted params — so what these tests pin is which bytes reach the wire.
// A field the caller never mentioned is written on every one of these calls,
// and the writable set is exactly {title, content}: both optional, neither
// presence-validated, so an omitted title silently becomes "Untitled" and an
// omitted content is silently erased. Both are 200s, so nothing but the request
// body itself distinguishes a preserve from a clear.

// patchDocumentFixture returns the fixture JSON with the given fields replaced.
func patchDocumentFixture(t *testing.T, base []byte, patch map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	for k, v := range patch {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal patched fixture: %v", err)
	}
	return b
}

// capturedDocumentRequest records one request seen by testDocumentsCaptureServer.
type capturedDocumentRequest struct {
	method string
	path   string
	body   map[string]any
}

// testDocumentsCaptureServer serves getBody for GETs and putBody for PUTs while
// recording every request's method, path, and (for PUTs) decoded body.
// The extra hooks, when non-nil, are installed on the client.
func testDocumentsCaptureServer(t *testing.T, getBody, putBody []byte, hooks Hooks) (*DocumentsService, *[]capturedDocumentRequest) {
	t.Helper()
	reqs := &[]capturedDocumentRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := capturedDocumentRequest{method: r.Method, path: r.URL.Path}
		if r.Method == "PUT" {
			cr.body = decodeRequestBody(t, r)
		}
		*reqs = append(*reqs, cr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if r.Method == "GET" {
			w.Write(getBody)
		} else {
			w.Write(putBody)
		}
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	var opts []ClientOption
	if hooks != nil {
		opts = append(opts, WithHooks(hooks))
	}
	client := NewClient(cfg, token, opts...)
	return client.ForAccount("99999").Documents(), reqs
}

// The fixture's content, which every merge-safe call must carry back out
// untouched unless the caller says otherwise.
const fixtureDocumentContent = "<div>This document contains the project overview and key milestones.</div>"

// TestDocumentWriteRequests_WritableSetMatchesFixture pins the writable set of
// the document write surface — exactly {title, content}, per BC3's
// `params.require(:document).permit(:title, :content)` — against the wire
// fixture, for both the composite input and the verbatim request.
//
// Moved here from vaults_test.go (TestUpdateDocumentRequest_Marshal) when the
// write surface moved to documents.go and UpdateDocument became ReplaceDocument.
func TestDocumentWriteRequests_WritableSetMatchesFixture(t *testing.T) {
	data := loadDocumentsFixture(t, "update-request.json")

	// The composite input: the fields a caller may set on a merge-safe update.
	var update UpdateDocumentRequest
	if err := json.Unmarshal(data, &update); err != nil {
		t.Fatalf("failed to unmarshal update-request.json into UpdateDocumentRequest: %v", err)
	}
	if update.Title != "Updated Document Title" {
		t.Errorf("expected title 'Updated Document Title', got %q", update.Title)
	}
	if update.Content == "" {
		t.Error("expected non-empty Content")
	}

	// The verbatim request carries the same set, so the two are interchangeable
	// at the call site and a caller can move between them without rewriting.
	var replace ReplaceDocumentRequest
	if err := json.Unmarshal(data, &replace); err != nil {
		t.Fatalf("failed to unmarshal update-request.json into ReplaceDocumentRequest: %v", err)
	}
	// Replace's fields are presence-bearing pointers — on a verbatim replace,
	// absent and explicitly-empty are different requests — so the mirror check
	// dereferences rather than comparing the two shapes directly.
	if replace.Title == nil || *replace.Title != update.Title {
		t.Errorf("replace title %v does not mirror update title %q", replace.Title, update.Title)
	}
	if replace.Content == nil || *replace.Content != update.Content {
		t.Errorf("replace content %v does not mirror update content %q", replace.Content, update.Content)
	}

	// The fixture itself must not grow a third writable field without this
	// surface growing one too.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}
	if len(raw) != 2 {
		t.Errorf("expected the writable set to be exactly {title, content}, got %v", raw)
	}
	for _, key := range []string{"title", "content"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected %q in the writable set, got %v", key, raw)
		}
	}
}

func TestDocumentsService_UpdateMergesUnsetFields(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	// Title-only update: content must be carried over from the GET. Omitting it
	// from the PUT would be a silent erase, not a preserve.
	document, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Title: "new title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if document.ID != 1069479300 {
		t.Errorf("expected ID 1069479300, got %d", document.ID)
	}

	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests (GET then PUT), got %d", len(*reqs))
	}
	if (*reqs)[0].method != "GET" || (*reqs)[1].method != "PUT" {
		t.Fatalf("expected GET then PUT, got %s then %s", (*reqs)[0].method, (*reqs)[1].method)
	}

	body := (*reqs)[1].body
	if body["title"] != "new title" {
		t.Errorf("expected title 'new title', got %v", body["title"])
	}
	if body["content"] != fixtureDocumentContent {
		t.Errorf("expected preserved content, got %v", body["content"])
	}
	// The writable set is exactly {title, content}; nothing else rides along.
	if len(body) != 2 {
		t.Errorf("expected exactly {title, content} in the body, got %v", body)
	}
}

func TestDocumentsService_UpdateMergesContentOnly(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	// The mirror case: a content-only update must preserve the title, which the
	// server would otherwise reset to "Untitled".
	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>new body</div>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["content"] != "<div>new body</div>" {
		t.Errorf("expected content '<div>new body</div>', got %v", body["content"])
	}
	if body["title"] != "Project Overview" {
		t.Errorf("expected preserved title 'Project Overview', got %v", body["title"])
	}
}

// TestDocumentsService_UpdateCannotClearWithEmptyString pins the Go zero-value
// guard: set-detection here is by zero value, as everywhere else in this SDK,
// so "" reads as "unaddressed", never as "clear". The fetched value has to go
// back out — a caller who wants the clear reaches for Edit or Replace.
func TestDocumentsService_UpdateCannotClearWithEmptyString(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Title:   "new title",
		Content: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	content, ok := body["content"]
	if !ok {
		t.Fatal("expected content present in the PUT body, but it was omitted")
	}
	if content != fixtureDocumentContent {
		t.Errorf("expected the fetched content to be resent (\"\" is unset, not a clear), got %v", content)
	}

	// And the same in the other direction: an empty Title leaves the current
	// title alone rather than letting the server reset it to "Untitled".
	svc2, reqs2 := testDocumentsCaptureServer(t, fixture, fixture, nil)
	if _, err := svc2.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>new body</div>",
		Title:   "",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title := (*reqs2)[len(*reqs2)-1].body["title"]; title != "Project Overview" {
		t.Errorf("expected the fetched title to be resent, got %v", title)
	}
}

func TestDocumentsService_UpdateNilRequestIsUsageError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479300, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil update request")
	}
	usageErr, ok := errors.AsType[*Error](err)
	if !ok || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	// Refused before the read-before-write, so not even the GET is spent.
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestDocumentsService_UpdateHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testDocumentsCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Update composes the public Get and Replace paths, so hooks see the two
	// wire operations rather than one synthetic composite.
	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Documents.Get" || ops[1] != "Documents.Replace" {
		t.Errorf("expected operations [Documents.Get Documents.Replace], got %v", ops)
	}
	if len(recorder.opEndCalls) != 2 {
		t.Errorf("expected 2 OnOperationEnd calls, got %d", len(recorder.opEndCalls))
	}
}

func TestDocumentsService_Edit(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	document, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		if f.Title != "Project Overview" {
			t.Errorf("expected Title from the GET, got %q", f.Title)
		}
		if f.Content != fixtureDocumentContent {
			t.Errorf("expected Content from the GET, got %q", f.Content)
		}
		f.Title = "🚨 " + f.Title
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if document.ID != 1069479300 {
		t.Errorf("expected ID 1069479300, got %d", document.ID)
	}

	if len(*reqs) != 2 || (*reqs)[0].method != "GET" || (*reqs)[1].method != "PUT" {
		t.Fatalf("expected GET then PUT, got %+v", *reqs)
	}
	// The full state goes back, not just the field the callback touched.
	body := (*reqs)[1].body
	if body["title"] != "🚨 Project Overview" {
		t.Errorf("expected prefixed title, got %v", body["title"])
	}
	if body["content"] != fixtureDocumentContent {
		t.Errorf("expected preserved content, got %v", body["content"])
	}
}

// TestDocumentsService_EditClearsContentPresentAndEmpty is the clear that
// Update cannot express. On a full-replace endpoint "" is how a clear is
// stated, and it has to REACH THE WIRE as a present key: omitting it would
// hand the clear back to the server's own rebuild — the same 200, but as an
// accident rather than an intent — and JSON null is out (SPEC §18).
func TestDocumentsService_EditClearsContentPresentAndEmpty(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		f.Content = ""
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	content, ok := body["content"]
	if !ok {
		t.Fatal("expected content present in the PUT body, but it was omitted")
	}
	if content != "" {
		t.Errorf("expected content \"\", got %v", content)
	}
	if content == nil {
		t.Error("expected content \"\", got JSON null")
	}
	// The untouched field still rides along in full.
	if body["title"] != "Project Overview" {
		t.Errorf("expected preserved title, got %v", body["title"])
	}
}

// TestDocumentsService_EditClearsTitlePresentAndEmpty is the same rule for the
// other field. BC3 presence-validates neither, so this is a 200 and the
// document reads back as "Untitled" — the clear is only visible in the body.
func TestDocumentsService_EditClearsTitlePresentAndEmpty(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		f.Title = ""
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	title, ok := body["title"]
	if !ok {
		t.Fatal("expected title present in the PUT body, but it was omitted")
	}
	if title != "" {
		t.Errorf("expected title \"\", got %v", title)
	}
	if body["content"] != fixtureDocumentContent {
		t.Errorf("expected preserved content, got %v", body["content"])
	}
}

// TestDocumentsService_EditCarriesAMissingFieldAsEmpty covers the GET that
// omits a writable field: Go's typed decode leaves it the zero value, and the
// full-replace body still states it rather than dropping the key.
func TestDocumentsService_EditCarriesAMissingFieldAsEmpty(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	getBody := patchDocumentFixture(t, fixture, map[string]any{"content": nil})
	svc, reqs := testDocumentsCaptureServer(t, getBody, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		if f.Content != "" {
			t.Errorf("expected empty Content for a null field, got %q", f.Content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	content, ok := body["content"]
	if !ok || content != "" {
		t.Errorf("expected content \"\" present in the body, got %v (present=%v)", content, ok)
	}
}

func TestDocumentsService_EditCallbackErrorAbortsWithoutPUT(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	wantErr := errors.New("nope")
	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		f.Title = "should never be written"
		f.Content = ""
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}

	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatalf("expected no PUT after a callback error, got %+v", r)
		}
	}
}

func TestDocumentsService_EditNilCallbackIsUsageError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479300, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil edit callback")
	}
	usageErr, ok := errors.AsType[*Error](err)
	if !ok || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestDocumentsService_EditHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testDocumentsCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Documents.Get" || ops[1] != "Documents.Replace" {
		t.Errorf("expected operations [Documents.Get Documents.Replace], got %v", ops)
	}
}

func TestDocumentsService_ReplaceSendsSparseVerbatim(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, recorder)

	replaceTitle := "the whole new document"
	document, err := svc.Replace(context.Background(), 1069479300, &ReplaceDocumentRequest{
		Title: &replaceTitle,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if document.ID != 1069479300 {
		t.Errorf("expected ID 1069479300, got %d", document.ID)
	}

	// No GET: replace is the server-native verbatim PUT.
	if len(*reqs) != 1 || (*reqs)[0].method != "PUT" {
		t.Fatalf("expected exactly one PUT, got %+v", *reqs)
	}
	body := (*reqs)[0].body
	if body["title"] != "the whole new document" {
		t.Errorf("expected title in body, got %v", body["title"])
	}
	// The unnamed field is omitted, and the server clears it. That is the sharp
	// edge Update and Edit exist to blunt.
	if _, ok := body["content"]; ok {
		t.Errorf("expected content omitted from a sparse replace, got %v", body["content"])
	}
	if len(body) != 1 {
		t.Errorf("expected exactly {title} in the body, got %v", body)
	}

	// Hooks observe a single Documents.Replace operation.
	if len(recorder.opStartCalls) != 1 ||
		recorder.opStartCalls[0].Service != "Documents" || recorder.opStartCalls[0].Operation != "Replace" {
		t.Errorf("expected single Documents.Replace operation, got %+v", recorder.opStartCalls)
	}
}

// TestDocumentsService_ReplaceEmptyRequestIsUsageError covers the one shape BC3
// rejects outright: `params.require(:document)` is synthesized from a flat body,
// so a body naming neither field carries no document object and is a 400. The
// SDK refuses it locally rather than spending a round-trip to learn that.
func TestDocumentsService_ReplaceEmptyRequestIsUsageError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Replace(context.Background(), 1069479300, &ReplaceDocumentRequest{})
	if err == nil {
		t.Fatal("expected usage error for a replace request naming neither field")
	}
	usageErr, ok := errors.AsType[*Error](err)
	if !ok || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
	// The body is built inside the hook envelope, so the refusal is observable.
	if len(recorder.opStartCalls) != 1 || len(recorder.opEndCalls) != 1 {
		t.Errorf("expected the usage error to be observable to hooks, got %d starts / %d ends",
			len(recorder.opStartCalls), len(recorder.opEndCalls))
	}
}

func TestDocumentsService_ReplaceNilRequestIsUsageError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479300, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil replace request")
	}
	usageErr, ok := errors.AsType[*Error](err)
	if !ok || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

// TestDocumentsService_ReplaceSendsBothFieldsWhenBothSet is the shape Update
// and Edit always produce, exercised through the verbatim path.
func TestDocumentsService_ReplaceSendsBothFieldsWhenBothSet(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	bothTitle, bothContent := "Both", "<div>set</div>"
	_, err := svc.Replace(context.Background(), 1069479300, &ReplaceDocumentRequest{
		Title:   &bothTitle,
		Content: &bothContent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[0].body
	if body["title"] != "Both" || body["content"] != "<div>set</div>" {
		t.Errorf("expected both fields sent verbatim, got %v", body)
	}
}

// Document.title is @required in the spec, and BC3 can never render it blank
// (Document#title is super.presence || "Untitled"). encoding/json does not
// enforce required-ness, though: an absent "title" decodes to the string zero
// value, and the full-replace PUT would then send title:"" — blanking the real
// title on a call that only touched Content. That is #576's defect in the one
// shape a typed decoder does not catch, so the composite refuses it explicitly.
//
// The assertion that matters is the ORDERING: no PUT. A guard that fires after
// the PUT has already lost the field.
func TestDocumentsService_UpdateRefusesAMissingTitleBeforeWriting(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{"absent", map[string]any{"title": nil}},
		{"empty", map[string]any{"title": ""}},
		{"whitespace", map[string]any{"title": "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getBody := patchDocumentFixture(t, fixture, tc.patch)
			svc, reqs := testDocumentsCaptureServer(t, getBody, fixture, nil)

			_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
				Content: "<div>New body.</div>",
			})
			if err == nil {
				t.Fatal("expected the call to fail, but it succeeded")
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			// api_error, not usage: the value arrived in a successful API
			// response, so nothing the caller passed is at fault.
			if apiErr.Code != CodeAPI {
				t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
			}
			if apiErr.HTTPStatus != 0 {
				t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
			}
			if apiErr.Retryable {
				t.Error("re-requesting cannot repair a malformed body")
			}
			for _, r := range *reqs {
				if r.method == "PUT" {
					t.Fatalf("expected no PUT before the guard fired, got %+v", r)
				}
			}
		})
	}
}

func TestDocumentsService_EditRefusesAMissingTitleBeforeWriting(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	getBody := patchDocumentFixture(t, fixture, map[string]any{"title": nil})
	svc, reqs := testDocumentsCaptureServer(t, getBody, fixture, nil)

	called := false
	_, err := svc.Edit(context.Background(), 1069479300, func(f *DocumentFields) error {
		called = true
		f.Content = "<div>New body.</div>"
		return nil
	})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}
	if called {
		t.Error("the callback must not run on a malformed read")
	}
	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatalf("expected no PUT before the guard fired, got %+v", r)
		}
	}
}

// The table deliberately spans three different decoder error types, because
// fetchDocument classifies by EXCLUSION rather than by an allowlist: an
// allowlist of encoding/json sentinels leaked *time.ParseError, and adding that
// still leaked types.FlexInt's bare fmt.Errorf.
//
// json.Unmarshal is Go's answer to the hand-written type guards the dynamic
// SDKs carry, and it does refuse a wrong-typed field before the composite can
// write it back. But it reports that as a raw *json.UnmarshalTypeError, which
// is not the shape SPEC §6 defines for a malformed 2xx body: a caller switching
// on *Error would miss it entirely and it carries no hint. The composite
// normalizes it the way the Swift one normalizes DecodingError, so a malformed
// response looks the same in every SDK.
func TestDocumentsService_UpdateWrapsADecodeFailureAsStatuslessAPIError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{"title is a number", map[string]any{"title": 42}},
		{"content is an object", map[string]any{"content": map[string]any{"a": 1}}},
		// The two shapes an allowlist of encoding/json sentinels misses.
		// created_at is time.Time, so a bad timestamp is *time.ParseError.
		// A non-integral attachment dimension is rejected by
		// types.FlexInt.UnmarshalJSON itself, which returns a plain
		// fmt.Errorf that is no named type at all — the case that shows an
		// allowlist can never be completed, only extended. (A *string-typed*
		// height would surface as *json.UnmarshalTypeError from the nested
		// json.Unmarshal and so would not discriminate.)
		{"created_at is not a timestamp", map[string]any{"created_at": "not-a-timestamp"}},
		{"attachment height is non-integral", map[string]any{
			"content_attachments": []any{map[string]any{"height": 1024.5}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getBody := patchDocumentFixture(t, fixture, tc.patch)
			svc, reqs := testDocumentsCaptureServer(t, getBody, fixture, nil)

			_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
				Title: "Q3 Plan",
			})
			if err == nil {
				t.Fatal("expected the call to fail, but it succeeded")
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			if apiErr.Code != CodeAPI {
				t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
			}
			if apiErr.HTTPStatus != 0 {
				t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
			}
			if apiErr.Retryable {
				t.Error("re-requesting cannot repair a malformed body")
			}
			if apiErr.Hint == "" {
				t.Error("expected a hint naming the deliberate-overwrite escape hatch")
			}
			for _, r := range *reqs {
				if r.method == "PUT" {
					t.Fatalf("expected no PUT before the guard fired, got %+v", r)
				}
			}
		})
	}
}

// The decoder's own error stays REACHABLE, not just quoted (#750).
//
// documentDecodeError interpolates it into Message, which tells a human what
// happened and leaves a caller nothing to act on: deciding "was the field the
// wrong type, or was the body not JSON at all" from a sentence means matching
// substrings, which is the mechanism #750 removed from Swift. Cause + Unwrap put
// the decoder's own error back in reach of errors.As, so the classification and
// the detail are both available and neither is parsed out of prose.
func TestDocumentsService_UpdateKeepsTheDecoderErrorReachable(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	getBody := patchDocumentFixture(t, fixture, map[string]any{"title": 42})
	svc, _ := testDocumentsCaptureServer(t, getBody, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "Q3 Plan"})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}

	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("expected the decoder's error to be reachable through Unwrap, got %v", err)
	}
	if typeErr.Field != "title" {
		t.Errorf("expected the decoder to name the offending field, got %q", typeErr.Field)
	}
	// The classification must survive alongside it: a chain that only reports
	// the decoder error would have replaced the SPEC §6 shape, not enriched it.
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeAPI {
		t.Fatalf("expected a statusless api_error over the decoder error, got %T: %v", err, err)
	}
}

// A 200 whose Content-Length promises 4096 bytes and then stops after thirty:
// the wire shape of a connection dropped mid-body. net/http's transfer.go turns
// an early EOF on a Content-Length body into io.ErrUnexpectedEOF.
const truncatedResponse = "HTTP/1.1 200 OK\r\n" +
	"Content-Type: application/json\r\n" +
	"Content-Length: 4096\r\n" +
	"\r\n" +
	`{"id":1069479300,"title":"Q3 P`

// A 200 whose chunked framing is corrupt. net/http reports that as a bare
// errors.New("invalid byte in chunk length") — no named type, not a net.Error,
// not io.ErrUnexpectedEOF, and so invisible to any deny-list of transport error
// TYPES. It shares that property with the transport's automatic gunzip
// (gzip.ErrHeader) and with anything a WithTransport-supplied body chooses to
// return, which is why the origin is marked at the read rather than inferred
// from the error afterwards.
const chunkedGarbageResponse = "HTTP/1.1 200 OK\r\n" +
	"Content-Type: application/json\r\n" +
	"Transfer-Encoding: chunked\r\n" +
	"\r\n" +
	"zz\r\n"

// brokenBodyServer answers every request by hijacking the connection, writing
// raw verbatim, and hanging up. It also counts the PUTs it saw, so a caller can
// prove no write-back followed a failed read.
//
// Hijacking is the only way to stage a body that fails mid-read: a normal
// handler's ResponseWriter derives Content-Length from what was actually
// written and re-frames chunked output itself, so it cannot be made to
// contradict itself.
//
// Shared with schedules_test.go — the two composites read through the same two
// generated Parse* functions, and #773 is a property of both.
func brokenBodyServer(t *testing.T, raw string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	puts := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts.Add(1)
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server does not support hijacking, so a mid-body failure cannot be staged")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack failed: %v", err)
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString(raw)
		_ = buf.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv, puts
}

// brokenBodyClient points a client at a brokenBodyServer serving raw.
func brokenBodyClient(t *testing.T, raw string) (*AccountClient, *atomic.Int64) {
	t.Helper()
	srv, puts := brokenBodyServer(t, raw)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	return NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999"), puts
}

// assertUnclassifiedReadFailure pins what a failed body read owes the caller: it
// reaches them as itself, and it is NOT the statusless non-retryable api_error
// that a malformed body gets. Nothing wrote back afterwards, either — a read
// that failed cannot be merge-safely echoed.
func assertUnclassifiedReadFailure(t *testing.T, err error, puts *atomic.Int64) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}
	if apiErr, ok := errors.AsType[*Error](err); ok {
		t.Fatalf("expected the read failure verbatim, got a %q *Error (retryable=%v): %v",
			apiErr.Code, apiErr.Retryable, err)
	}
	if n := puts.Load(); n != 0 {
		t.Fatalf("expected no write-back after a failed read, got %d PUT(s)", n)
	}
}

// #773: the document read's decode step sees four different failures, and only
// two of them are the decoder's, because io.ReadAll runs INSIDE
// ParseGetDocumentResponse. Two are the body failing to ARRIVE (cases 1 and 4)
// and two are the body failing to DECODE (cases 2 and 3). All four reach
// documentDecodeError by the same path, so what tells them apart is the marker
// markBodyReadFailures puts on the read itself — not anything about the errors.
// They are one test rather than four so a later "simplification" has to face
// all of them at once.
//
// Cases 3 and 4 are the load-bearing pair, and they fail in opposite
// directions. That pairing is the point: either one alone rules out a single
// bad gate, but together they rule out inferring the origin from the error's
// type AT ALL, in either direction.
//
//   - An ALLOW-LIST of the decoder (*json.SyntaxError, *json.UnmarshalTypeError,
//     everything else verbatim) breaks case 3, because created_at is a time.Time
//     and time.Time.UnmarshalJSON returns *time.ParseError, which is not an
//     encoding/json type at all.
//   - A DENY-LIST of the transport (io.ErrUnexpectedEOF, net.Error, the context
//     sentinels — classify everything else) breaks case 4, because net/http
//     reports corrupt chunk framing as a bare errors.New with no type to match.
//
// Both were tried on this branch. Neither survives both cases, because both
// INFER the origin from the error's type, and neither error set is closed
// enough to infer from. Marking the origin at the read is what passes all four.
func TestDocumentsService_GetSeparatesTransportFailuresFromDecodeFailures(t *testing.T) {
	// 1. The body never finished arriving. There is nothing malformed about it —
	// re-requesting is exactly the right move — so the read error has to survive
	// as itself rather than becoming a statusless, permanently non-retryable
	// api_error.
	t.Run("a body that stops mid-read stays the transport's error", func(t *testing.T) {
		ac, puts := brokenBodyClient(t, truncatedResponse)

		_, err := ac.Documents().Update(context.Background(), 1069479300,
			&UpdateDocumentRequest{Title: "Q3 Plan"})
		assertUnclassifiedReadFailure(t, err, puts)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected the read failure to reach the caller as itself, got %T: %v", err, err)
		}
	})

	// 2. The body arrived whole and is not JSON. Re-requesting cannot repair it,
	// and the merge-safe composites would write it back, so this keeps the
	// statusless api_error — with the decoder's own error still reachable.
	t.Run("a malformed body is the statusless api_error", func(t *testing.T) {
		fixture := loadDocumentsFixture(t, "get.json")
		svc, reqs := testDocumentsCaptureServer(t, []byte(`{"id":1069479300,`), fixture, nil)

		_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "Q3 Plan"})
		if err == nil {
			t.Fatal("expected the call to fail, but it succeeded")
		}
		assertStatuslessDocumentAPIError(t, err)
		if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
			t.Errorf("expected the JSON error to be reachable through Cause, got %v", err)
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Error("a body that arrived whole must not be reported as a truncated read")
		}
		for _, r := range *reqs {
			if r.method == "PUT" {
				t.Fatalf("expected no PUT after a decode failure, got %+v", r)
			}
		}
	})

	// 3. Neither: valid JSON, decoded by a type encoding/json does not own. An
	// allow-list of encoding/json sentinels drops the §6 classification here, so
	// this case is the one that decides the shape of the gate.
	t.Run("a decoder error that is neither is still the statusless api_error", func(t *testing.T) {
		fixture := loadDocumentsFixture(t, "get.json")
		getBody := patchDocumentFixture(t, fixture, map[string]any{"created_at": "not-a-date"})
		svc, _ := testDocumentsCaptureServer(t, getBody, fixture, nil)

		_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "Q3 Plan"})
		if err == nil {
			t.Fatal("expected the call to fail, but it succeeded")
		}
		// Pin the premise as well as the conclusion: if created_at ever stops
		// being a time.Time this case silently stops testing what it claims to.
		if _, ok := errors.AsType[*time.ParseError](err); !ok {
			t.Fatalf("expected a *time.ParseError from created_at, got %T: %v", err, err)
		}
		assertStatuslessDocumentAPIError(t, err)
	})

	// 4. A read failure with no type to match on: corrupt chunk framing, which
	// net/http reports as a bare errors.New. It is the mirror of case 3 — where
	// an allow-list of decoder types drops a decode failure, a deny-list of
	// transport types drops THIS, and stamps a transient failure permanently
	// non-retryable exactly as #773 described. Raised by Copilot on PR #779.
	t.Run("a read failure with no matchable type stays the transport's error", func(t *testing.T) {
		ac, puts := brokenBodyClient(t, chunkedGarbageResponse)

		_, err := ac.Documents().Update(context.Background(), 1069479300,
			&UpdateDocumentRequest{Title: "Q3 Plan"})
		assertUnclassifiedReadFailure(t, err, puts)
		assertOutsideEveryTransportDenyList(t, err)
	})
}

// assertOutsideEveryTransportDenyList pins this case's PREMISE: the framing
// error carries none of the marks a type-based gate could have recognised. If
// net/http ever gives it a named type or makes it a net.Error, this case
// silently stops being the thing it was added to prove and has to be restaged.
func assertOutsideEveryTransportDenyList(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, io.ErrUnexpectedEOF) {
		t.Error("this case only proves anything while the framing error is NOT io.ErrUnexpectedEOF")
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		t.Error("this case only proves anything while the framing error is NOT a net.Error")
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Error("this case only proves anything while the framing error is NOT a context sentinel")
	}
}

// assertStatuslessDocumentAPIError pins the SPEC §6 shape the merge-safe
// composites depend on: an api_error with no HTTP status, not retryable, and
// carrying the escape-hatch hint.
func assertStatuslessDocumentAPIError(t *testing.T, err error) {
	t.Helper()
	apiErr, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Code != CodeAPI {
		t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
	}
	if apiErr.HTTPStatus != 0 {
		t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
	}
	if apiErr.Retryable {
		t.Error("re-requesting cannot repair a malformed body")
	}
	if apiErr.Hint == "" {
		t.Error("expected a hint naming the deliberate-overwrite escape hatch")
	}
}

// A transport or HTTP error must pass through untouched — the wrapper is for
// decode failures only, and swallowing everything would hide a 404 behind a
// "does not decode" message.
func TestDocumentsService_UpdatePassesNonDecodeErrorsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"Not Found"}`)
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	svc := client.ForAccount("999").Documents()

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "Q3 Plan"})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected the 404 to survive, got HTTP %d (%s)", apiErr.HTTPStatus, apiErr.Message)
	}
}

// An all-nil request is the 400 (BC3 requires the wrapping document object), but
// a request naming both fields as "" is a legal full replacement that clears
// both. Zero-value guards conflated the two and made the clear unreachable from
// Go's raw path; presence-bearing pointers keep them distinct. Raised by Codex
// review on #601.
func TestDocumentsService_ReplaceSendsExplicitEmptyStrings(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, nil)

	empty := ""
	if _, err := svc.Replace(context.Background(), 1069479300, &ReplaceDocumentRequest{
		Title:   &empty,
		Content: &empty,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*reqs) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(*reqs))
	}
	body := (*reqs)[0].body
	for _, key := range []string{"title", "content"} {
		value, ok := body[key]
		if !ok {
			t.Fatalf("expected %q present-and-empty in the body, got %+v", key, body)
		}
		if value != "" {
			t.Errorf("expected %q to be the empty string, got %#v", key, value)
		}
	}
}

// The decode-failure normalizer has to cover every error the response decoder
// can produce, not just the two encoding/json sentinels. Document carries
// time.Time fields, and time.Time.UnmarshalJSON reports a bad timestamp as
// *time.ParseError — which is neither *json.UnmarshalTypeError nor
// *json.SyntaxError, so a two-type allowlist would leak it raw.
//
// Those three are structurally the complete set for this model: json.Unmarshal
// reports a wrong type as *json.UnmarshalTypeError and malformed JSON as
// *json.SyntaxError, and the only field type on Document with its own
// UnmarshalJSON is time.Time.
func TestDocumentsService_UpdateNormalizesABadTimestamp(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	getBody := patchDocumentFixture(t, fixture, map[string]any{"created_at": "not-a-timestamp"})
	svc, reqs := testDocumentsCaptureServer(t, getBody, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>New body.</div>",
	})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Code != CodeAPI {
		t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
	}
	if apiErr.HTTPStatus != 0 {
		t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
	}
	if apiErr.Retryable {
		t.Error("re-requesting cannot repair a malformed body")
	}
	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatalf("expected no PUT after a decode failure, got %+v", r)
		}
	}
}

// A GatingHooks implementation rejects the composite's GET before any request
// is made — circuit breakers and bulkheads are explicitly allowed to do that
// with an ordinary sentinel. That decision is local, so it must reach the
// caller verbatim: classifying it as a malformed response would break
// errors.Is and blame the server for a choice the client made.
func TestDocumentsService_UpdatePreservesAGateError(t *testing.T) {
	errGated := errors.New("circuit open")
	fixture := loadDocumentsFixture(t, "get.json")
	hooks := &testGatingHooks{
		onGate: func(ctx context.Context, op OperationInfo) (context.Context, error) {
			if op.Operation == "Get" {
				return ctx, errGated
			}
			return ctx, nil
		},
	}
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, hooks)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>New body.</div>",
	})
	if !errors.Is(err, errGated) {
		t.Fatalf("expected the gate sentinel to survive errors.Is, got %T: %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("a gate rejection must issue no request, got %+v", *reqs)
	}
}

// gatingHooks refuses the operation before any request is made — a circuit
// breaker or bulkhead, which SPEC §12 explicitly permits.
type documentGatingHooks struct {
	recordingHooks
	err error
}

func (h *documentGatingHooks) OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error) {
	return ctx, h.err
}

// A gate error is not a decode error. The classifier lives below the gate, at
// the one call site whose only origins are the transport and the decoder, so a
// gating hook's own sentinel travels back untouched and errors.Is still works.
// Wrapping it would have reported a malformed body for a request that never ran.
func TestDocumentsService_UpdatePreservesAGatingHookError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	sentinel := errors.New("circuit breaker open")
	hooks := &documentGatingHooks{err: sentinel}
	svc, reqs := testDocumentsCaptureServer(t, fixture, fixture, hooks)

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{Title: "Q3 Plan"})
	if err == nil {
		t.Fatal("expected the gate to refuse the call")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the gate's own error to survive, got %T: %v", err, err)
	}
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Code == CodeAPI {
		t.Error("a gate refusal must not be reported as a malformed response")
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests when the gate refuses, got %+v", *reqs)
	}
}

// failingAuth is an AuthStrategy that refuses to sign the request, standing in
// for a token refresh or keyring failure.
type failingAuth struct{ err error }

func (a failingAuth) Authenticate(context.Context, *http.Request) error { return a.err }

// The authEditor runs inside the generated client, per request, so an auth
// failure surfaces from the same call the response decoder does — with no HTTP
// response behind it. It must keep its own taxonomy: reporting a credential
// failure as malformed document JSON leaves callers unable to recognize it.
func TestDocumentsService_UpdatePreservesAnAuthError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	reqs := &[]capturedDocumentRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, capturedDocumentRequest{method: r.Method, path: r.URL.Path})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	authErr := ErrAuth("token refresh failed")
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"},
		WithAuthStrategy(failingAuth{err: authErr}))
	svc := client.ForAccount("99999").Documents()

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>New body.</div>",
	})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Code != CodeAuth {
		t.Fatalf("expected the auth taxonomy to survive, got code %q (message %q)", apiErr.Code, apiErr.Message)
	}
	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatalf("expected no PUT after an auth failure, got %+v", r)
		}
	}
}

// AuthStrategy.Authenticate permits ANY error, and BearerAuth propagates a
// token provider's error unchanged — so an auth failure is not reliably an
// *Error. Splitting the request from the decode is what makes this work:
// nothing inspects the error, so an ordinary sentinel survives errors.Is.
func TestDocumentsService_UpdatePreservesAnArbitraryAuthError(t *testing.T) {
	fixture := loadDocumentsFixture(t, "get.json")
	reqs := &[]capturedDocumentRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, capturedDocumentRequest{method: r.Method, path: r.URL.Path})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	}))
	t.Cleanup(server.Close)

	sentinel := errors.New("keyring locked")
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"},
		WithAuthStrategy(failingAuth{err: sentinel}))
	svc := client.ForAccount("99999").Documents()

	_, err := svc.Update(context.Background(), 1069479300, &UpdateDocumentRequest{
		Content: "<div>New body.</div>",
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the auth sentinel to survive errors.Is, got %T: %v", err, err)
	}
	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatalf("expected no PUT after an auth failure, got %+v", r)
		}
	}
}
