package basecamp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// The Documents read surface (Get, List, Create, Trash) and the Document type
// itself live in vaults.go, alongside the vault they hang off. This file holds
// the write surface, because that is where the endpoint's semantics bite:
//
// PUT /{accountId}/documents/{documentId} is a FULL REPLACE. BC3's
// DocumentsController#update runs
//
//	@recording.update! recording_attributes.merge(recordable: new_document)
//
// where new_document is Document.new(params.require(:document).permit(:title,
// :content)) — a brand-new recordable built from only the permitted params and
// swapped in wholesale. A field absent from the body is nil on the replacement,
// so a sparse PUT that omits Content ERASES it, and one that omits Title erases
// that too (the document then reads back as "Untitled", because Document#title
// falls back when blank). Neither attribute carries a presence validation, so
// NEITHER OMISSION IS A 422 — both are a 200 that quietly clears. What BC3 does
// require is the wrapping document object, so a body naming neither field is a
// 400.
//
// Hence the three-method surface: Update overlays onto the current state, Edit
// hands the caller that state, and Replace stays verbatim and destructive by
// design.

// UpdateDocumentRequest specifies the fields to set on a document, preserving
// everything the caller leaves unset.
//
// Set-detection is by zero value, as elsewhere in the Go SDK: an empty string
// reads as "unaddressed", not as "clear". To CLEAR a field, use Edit or
// Replace, where "" is unambiguous.
type UpdateDocumentRequest struct {
	// Title is the document title. Empty leaves the current title untouched.
	Title string `json:"title,omitempty"`
	// Content is the document body in HTML. Empty leaves the current body untouched.
	Content string `json:"content,omitempty"`
}

// ReplaceDocumentRequest specifies a document's complete new representation.
//
// This is the verbatim request: whatever it omits, the server clears. Neither
// field is required — BC3 presence-validates neither — but a request that
// carries neither is rejected with a 400, because BC3 requires the wrapping
// document object.
//
// Both fields are presence-bearing pointers rather than plain strings, because
// on a verbatim replace "absent" and "explicitly empty" are different requests
// and only one of them is legal on its own. A nil field is omitted from the
// body; a pointer to "" is SENT as "", which is the only way to say "clear both
// fields" — the all-nil body is the 400. (Their server EFFECT happens to
// coincide, since an omitted title and an empty one both read back as
// "Untitled", but the SDK must not collapse a distinction the wire makes.)
//
//	empty := ""
//	svc.Replace(ctx, id, &ReplaceDocumentRequest{Title: &empty, Content: &empty})
type ReplaceDocumentRequest struct {
	// Title is the document title. Nil omits it — the server clears it and the
	// document reads back as "Untitled". A pointer to "" sends it explicitly.
	Title *string `json:"title,omitempty"`
	// Content is the document body in HTML. Nil omits it — the server clears
	// it. A pointer to "" sends it explicitly.
	Content *string `json:"content,omitempty"`
}

// DocumentFields is a document's full writable state, handed to the Edit
// callback. The whole value is PUT back to the server, so clearing a field
// means setting it empty ("") — there is no third state. BC3's writable set on
// this endpoint is exactly {title, content}.
type DocumentFields struct {
	// Title is the document title. Set "" to clear; it then reads back as "Untitled".
	Title string
	// Content is the document body in HTML. Set "" to clear.
	Content string
}

// fieldsFromDocument derives a document's full writable state from a GET.
//
// Go decodes the response into the typed Document before this runs, so
// encoding/json has already rejected a wrong-TYPED field — the type guards the
// Python, Ruby and TypeScript composites carry (#576) have no Go analogue to
// write.
//
// A MISSING field is the hole encoding/json leaves open, and it matters here
// because Title is required. An absent "title" decodes to the string zero
// value, and the full-replace PUT would then send title:"" — blanking the real
// title on a call that only touched Content, which is #576's defect in the one
// shape a typed decoder does not catch. BC3 can never render title blank
// (Document#title is super.presence || "Untitled") and the spec marks it
// @required, so an empty title on a 2xx read is a malformed response rather
// than an empty title. Content is optional in the spec, so empty is genuinely
// empty and is left alone.
func fieldsFromDocument(d *Document) (*DocumentFields, error) {
	if strings.TrimSpace(d.Title) == "" {
		return nil, &Error{
			Code:    CodeAPI,
			Message: `GetDocument returned a document with no "title", but the field is required`,
			Hint: "The merge-safe Update/Edit resend this field verbatim, so a missing value " +
				"would blank the current one. Use Replace to write the record deliberately.",
			Retryable: false,
		}
	}
	return &DocumentFields{Title: d.Title, Content: d.Content}, nil
}

// fullBody builds the full writable state for the replace transport.
//
// Both fields are ALWAYS sent, empties included, so a clear survives: on a
// full-replace endpoint "" is how a clear is expressed — never JSON null (SPEC
// §18 body compaction), and never by omission, which would hand the clear back
// to the server's own rebuild and read as an accident rather than an intent.
// The generated members are pointers, and a non-nil pointer to "" survives
// `omitempty`, so the always-send-empty contract holds through the generated
// request type.
func (f *DocumentFields) fullBody() (generated.ReplaceDocumentJSONRequestBody, error) {
	return generated.ReplaceDocumentJSONRequestBody{
		Title:   &f.Title,
		Content: &f.Content,
	}, nil
}

// bodyReadError marks an error as the response body failing to ARRIVE rather
// than failing to DECODE.
//
// The two are indistinguishable at the two call sites that parse a response by
// hand, because io.ReadAll(rsp.Body) is the FIRST statement of every generated
// Parse* function and its error comes back verbatim. So a truncated body, a
// reset connection or a bad chunk header reached documentDecodeError and
// scheduleEntryDecodeError by exactly the same path a JSON fault does, and got
// the same statusless CodeAPI with Retryable: false — a transient failure made
// to look permanent (#773).
//
// Inferring the origin from the error afterwards was tried and abandoned: there
// is no error set to match on. net/http reports bad chunk framing as a bare
// errors.New("invalid byte in chunk length"), the transport's automatic gunzip
// returns gzip.ErrHeader, and WithTransport lets a caller supply a body whose
// Read returns anything at all — none of them a net.Error, none of them a named
// type. Marking the read is what the call sites' doc comments already claim
// happens ("the origin is known by construction rather than guessed"), so the
// marker makes that true instead of approximating it.
type bodyReadError struct{ err error }

func (e *bodyReadError) Error() string { return e.err.Error() }
func (e *bodyReadError) Unwrap() error { return e.err }

// markedBody tags every read failure of the body it wraps.
type markedBody struct{ inner io.ReadCloser }

// Read forwards to the wrapped body and marks anything it fails with.
//
// io.EOF alone passes through untouched, and the comparison is deliberately
// against that exact value rather than errors.Is: io.ReadAll ends its loop on
// `err == io.EOF` and treats every other value as a failure, so matching its
// test exactly is what guarantees the marker covers precisely the errors
// io.ReadAll will hand back — including a wrapped EOF, which io.ReadAll would
// surface as an error too.
func (b *markedBody) Read(p []byte) (int, error) {
	n, err := b.inner.Read(p)
	if err != nil && err != io.EOF { //nolint:errorlint // matching io.ReadAll's own exact test, deliberately
		err = &bodyReadError{err: err}
	}
	return n, err
}

func (b *markedBody) Close() error { return b.inner.Close() }

// markBodyReadFailures wraps a response body so the two decode-error renderers
// can tell a failed read from a failed decode by construction. Call it on
// httpResp.Body before handing the response to a generated Parse* function.
func markBodyReadFailures(body io.ReadCloser) io.ReadCloser {
	return &markedBody{inner: body}
}

// documentDecodeError renders a response-decoder failure in the SPEC §6 shape.
//
// Go's json.Unmarshal is the typed guard the dynamic SDKs write by hand, and it
// rejects a wrong-typed field before a composite ever sees it — but it reports
// that as a raw decoder error, which callers switching on *Error would miss and
// which carries no hint. (The Swift composite does the same with DecodingError.)
//
// The decoder half is not classified by INSPECTING the error, deliberately.
// That inspection does not work in either direction: decoder errors are not
// enumerable (created_at/updated_at are time.Time, whose UnmarshalJSON returns
// *time.ParseError rather than an encoding/json sentinel, and
// content_attachments carries *types.FlexInt dimensions rejected with a plain
// fmt.Errorf that is no named type at all), and neither are the errors that
// precede a response — a gating hook, a token provider or a custom
// AuthStrategy may each return any sentinel they like. So DocumentsService.Get
// splits the request from the decode and calls this on the decode step only,
// where the origin is known by construction rather than guessed.
//
// The split needs one piece of help, because it is not as clean as it reads:
// io.ReadAll runs INSIDE the generated parse, so the decode step's errors are
// not all decode errors (#773). markBodyReadFailures above tags the ones that
// are not, at the read, and those pass through verbatim.
//
// The decoder's error is kept as Cause, not just interpolated into Message
// (#750). Every SDK renders it into the message; only the ones that keep it let
// a caller act on it, and Go's is the errors.As/errors.Is chain — a caller who
// wants to know whether the field was the wrong type or the body was not JSON
// at all reaches *json.UnmarshalTypeError or *json.SyntaxError through Unwrap
// instead of pattern-matching a sentence.
func documentDecodeError(err error) error {
	// A body that never finished arriving is not a malformed body. Unwrap the
	// marker so the caller sees the transport's own error, not our plumbing.
	if marked, ok := errors.AsType[*bodyReadError](err); ok {
		return marked.err
	}
	return &Error{
		Code:    CodeAPI,
		Message: truncate(fmt.Sprintf("GetDocument returned a body that does not decode as a document: %v", err)),
		Hint: "The merge-safe Update/Edit resend this record's fields verbatim, so a malformed " +
			"response cannot be written back safely. Use Replace to write the record deliberately.",
		Retryable: false,
		Cause:     err,
	}
}

// Update sets the given fields on a document and preserves everything else:
// GETs the current document, overlays the explicitly-set request fields, and
// PUTs the full representation back.
//
// An unset (empty) field is untouched, guaranteed. Strings cannot be CLEARED
// through Update — "" is Go's unset marker here — so use Edit or Replace to
// clear one.
//
// Composes the public Get and Replace paths, so hooks observe both wire
// operations (Documents.Get then Documents.Replace) rather than a synthetic
// composite.
//
// Not atomic: there is no conditional-update signal on this endpoint, so a
// concurrent write between the GET and PUT is overwritten — last write wins for
// the whole representation, with a window of one round-trip. Use Replace to
// overwrite deliberately.
func (s *DocumentsService) Update(ctx context.Context, documentID int64, req *UpdateDocumentRequest) (*Document, error) {
	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	current, err := s.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}

	fields, err := fieldsFromDocument(current)
	if err != nil {
		return nil, err
	}
	if req.Title != "" {
		fields.Title = req.Title
	}
	if req.Content != "" {
		fields.Content = req.Content
	}

	return s.replaceDocument(ctx, documentID, fields.fullBody)
}

// Edit applies a read-modify-write callback to a document: GETs the current
// document, hands the callback its full writable state, and PUTs the whole
// thing back. Clearing a field means setting it empty ("") — an untouched field
// keeps its current value. If the callback returns an error, the edit aborts
// and nothing is written.
//
// Not atomic — see Update for the GET→PUT race.
func (s *DocumentsService) Edit(ctx context.Context, documentID int64, fn func(*DocumentFields) error) (*Document, error) {
	if fn == nil {
		return nil, ErrUsage("edit callback is required")
	}

	current, err := s.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}

	fields, err := fieldsFromDocument(current)
	if err != nil {
		return nil, err
	}
	if err := fn(fields); err != nil {
		return nil, err
	}

	return s.replaceDocument(ctx, documentID, fields.fullBody)
}

// Replace overwrites a document with the given representation verbatim: one
// PUT, no read-before-write. Every writable field the request omits is omitted
// from the body, and the server clears it.
//
// Sharp by construction. Use Update or Edit to preserve the fields the call
// does not name.
func (s *DocumentsService) Replace(ctx context.Context, documentID int64, req *ReplaceDocumentRequest) (*Document, error) {
	return s.replaceDocument(ctx, documentID, func() (generated.ReplaceDocumentJSONRequestBody, error) {
		if req == nil {
			return generated.ReplaceDocumentJSONRequestBody{}, ErrUsage("replace request is required")
		}
		body := generated.ReplaceDocumentJSONRequestBody{
			Title:   req.Title,
			Content: req.Content,
		}
		if req.Title == nil && req.Content == nil {
			// BC3 does params.require(:document), which Rails wrap_parameters
			// synthesizes from a flat body — so a body naming neither field
			// carries no document object at all and is a 400. Refuse it here
			// rather than spend a round-trip discovering that.
			return generated.ReplaceDocumentJSONRequestBody{}, ErrUsage("replace request must set title or content; a body naming neither is rejected by the server with a 400")
		}
		return body, nil
	})
}

// replaceDocument is the single transport behind Update, Edit and Replace. It
// owns the hook envelope and the one generated-client call site; buildBody
// runs inside the envelope so usage errors are observable to hooks.
func (s *DocumentsService) replaceDocument(ctx context.Context, documentID int64, buildBody func() (generated.ReplaceDocumentJSONRequestBody, error)) (result *Document, err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "Replace",
		ResourceType: "document", IsMutation: true,
		ResourceID: documentID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Built inside the envelope so a usage error is observable to hooks.
	body, err := buildBody()
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.ReplaceDocumentWithResponse(
		ctx, s.client.accountID, documentID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	document := documentFromGenerated(*resp.JSON200)
	return &document, nil
}
