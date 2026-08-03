package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
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
type ReplaceDocumentRequest struct {
	// Title is the document title. Omitted (empty) clears it, and the document
	// then reads back as "Untitled".
	Title string `json:"title,omitempty"`
	// Content is the document body in HTML. Omitted (empty) clears it.
	Content string `json:"content,omitempty"`
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
	if d.Title == "" {
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

// fullBody serializes the full writable state for the replace transport.
//
// Both fields are ALWAYS sent, empties included, so a clear survives: on a
// full-replace endpoint "" is how a clear is expressed — never JSON null (SPEC
// §18 body compaction), and never by omission, which would hand the clear back
// to the server's own rebuild and read as an accident rather than an intent.
func (f *DocumentFields) fullBody() (map[string]any, error) {
	return map[string]any{
		"title":   f.Title,
		"content": f.Content,
	}, nil
}

// fetchDocument GETs the document the composites read their writable state from,
// normalizing a decode failure into the SPEC §6 shape.
//
// Go's json.Unmarshal is the typed guard the dynamic SDKs write by hand, and it
// rejects a wrong-typed field before this composite ever sees it — but it
// reports that as a raw *json.UnmarshalTypeError/*json.SyntaxError surfaced
// through the generated parser, which is not the shape SPEC §6 defines for a
// malformed 2xx body: callers switching on *Error would miss it entirely and it
// carries no hint. Wrap it, so a malformed response looks the same in every SDK
// (the Swift composite does this with DecodingError). A transport or HTTP error
// passes through untouched.
func (s *DocumentsService) fetchDocument(ctx context.Context, documentID int64) (*Document, error) {
	current, getErr := s.Get(ctx, documentID)
	if getErr == nil {
		return current, nil
	}

	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	if !errors.As(getErr, &typeErr) && !errors.As(getErr, &syntaxErr) {
		return nil, getErr
	}
	return nil, &Error{
		Code:    CodeAPI,
		Message: truncate(fmt.Sprintf("GetDocument returned a body that does not decode as a document: %v", getErr)),
		Hint: "The merge-safe Update/Edit resend this record's fields verbatim, so a malformed " +
			"response cannot be written back safely. Use Replace to write the record deliberately.",
		Retryable: false,
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

	current, err := s.fetchDocument(ctx, documentID)
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

	current, err := s.fetchDocument(ctx, documentID)
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
	return s.replaceDocument(ctx, documentID, func() (map[string]any, error) {
		if req == nil {
			return nil, ErrUsage("replace request is required")
		}
		body := map[string]any{}
		if req.Title != "" {
			body["title"] = req.Title
		}
		if req.Content != "" {
			body["content"] = req.Content
		}
		if len(body) == 0 {
			// BC3 does params.require(:document), which Rails wrap_parameters
			// synthesizes from a flat body — so a body naming neither field
			// carries no document object at all and is a 400. Refuse it here
			// rather than spend a round-trip discovering that.
			return nil, ErrUsage("replace request must set title or content; a body with neither is rejected by the server")
		}
		return body, nil
	})
}

// replaceDocument is the single transport behind Update, Edit and Replace. It
// owns the hook envelope and the one *WithBody call site.
//
// The body is a hand-marshaled map rather than the generated request struct:
// generated.ReplaceDocumentJSONRequestBody uses omitempty, which cannot express
// the always-send-empty semantics a full-replace composite needs (an empty
// title is a clear, and it has to reach the wire). SPEC §18 rule 1 sanctions
// exactly this carve-out — the generated wrapper still owns path, verb, content
// type and response decoding, and the operation identity still reaches hooks
// and retry.
func (s *DocumentsService) replaceDocument(ctx context.Context, documentID int64, buildBody func() (map[string]any, error)) (result *Document, err error) {
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
	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.parent.gen.ReplaceDocumentWithBodyWithResponse(
		ctx, s.client.accountID, documentID, "application/json", bodyReader)
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
