package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// testFieldErrorsAccount wires an httptest.Server to an AccountClient so service
// methods exercise the generated client and checkResponse end to end.
func testFieldErrorsAccount(t *testing.T, handler http.HandlerFunc) *AccountClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999")
}

// Webhook creation rejects with the unwrapped ActiveModel::Errors rendering at
// 400 (webhooks_controller.rb:31) — no "errors" wrapper anywhere in the body.
func TestWebhooksService_Create_BareFieldMap400(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"payload_url":["is not a valid URL"],"types":["is invalid"]}`))
	}).Webhooks()

	_, err := svc.Create(context.Background(), 1, &CreateWebhookRequest{
		PayloadURL: "https://example.com/hook",
		Types:      []string{"Comment"},
	})
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	wantMsg := "payload_url: is not a valid URL, types: is invalid"
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
	want := map[string][]string{
		"payload_url": {"is not a valid URL"},
		"types":       {"is invalid"},
	}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

// UpdateMyNote now declares FieldValidationError, matching what
// my/notes_controller.rb:19 actually renders.
func TestMyNotesService_Update_FieldKeyed422(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"content":["can't be blank"]}}`))
	}).MyNotes()

	_, err := svc.Update(context.Background(), "")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	if e.Message != "content: can't be blank" {
		t.Errorf("Message = %q, want flattened field errors", e.Message)
	}
	want := map[string][]string{"content": {"can't be blank"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

// Both folder writes render the field-keyed 422: stacks_controller.rb:51 for
// update (`render json: { errors: @stack.errors }`) and :27 for create (the same
// rendering from the RecordInvalid rescue). They therefore declare
// FieldValidationError, not ValidationError.
func TestFoldersService_Update_FieldKeyed422(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"name":["can't be blank"]}}`))
	}).Folders()

	_, err := svc.Update(context.Background(), 2085958513, "   ")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	if e.Message != "name: can't be blank" {
		t.Errorf("Message = %q, want flattened field errors", e.Message)
	}
	want := map[string][]string{"name": {"can't be blank"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

func TestFoldersService_Create_FieldKeyed422(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"name":["is invalid"]}}`))
	}).Folders()

	_, err := svc.Create(context.Background(), CreateFolderRequest{Name: "x"})
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	want := map[string][]string{"name": {"is invalid"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

// The wrapper flattens the body itself, so a wrapper-level assertion passes even
// when the *generated* 422 type is the wrong shape. This one goes at the
// generated client directly and pins the typed JSON422 field, which is the thing
// that silently broke: while these operations declared ValidationError, JSON422
// was a *ValidationErrorResponseContent whose required `error` member does not
// appear in the field-keyed body, so it decoded to the zero value and a caller
// reading resp.JSON422.Error got "" with no error of any kind.
func TestFoldersGenerated_JSON422_DecodesFieldKeyedBody(t *testing.T) {
	account := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"name":["can't be blank"]}}`))
	})
	ctx := context.Background()
	// generated.FieldErrorMap, not a bare map[string][]string — DeepEqual is
	// type-sensitive and the two print identically, so compare the named type.
	want := generated.FieldErrorMap{"name": {"can't be blank"}}

	updateResp, err := account.parent.gen.UpdateFolderWithResponse(ctx, account.accountID, 2085958513,
		generated.UpdateFolderJSONRequestBody{Name: "   "})
	if err != nil {
		t.Fatalf("UpdateFolder transport error: %v", err)
	}
	if updateResp.JSON422 == nil {
		t.Fatal("UpdateFolder JSON422 is nil — the 422 body did not decode into the typed field")
	}
	if !reflect.DeepEqual(updateResp.JSON422.Errors, want) {
		t.Errorf("UpdateFolder JSON422.Errors = %v, want %v", updateResp.JSON422.Errors, want)
	}

	createResp, err := account.parent.gen.CreateFolderWithResponse(ctx, account.accountID,
		generated.CreateFolderJSONRequestBody{})
	if err != nil {
		t.Fatalf("CreateFolder transport error: %v", err)
	}
	if createResp.JSON422 == nil {
		t.Fatal("CreateFolder JSON422 is nil — the 422 body did not decode into the typed field")
	}
	if !reflect.DeepEqual(createResp.JSON422.Errors, want) {
		t.Errorf("CreateFolder JSON422.Errors = %v, want %v", createResp.JSON422.Errors, want)
	}
}
