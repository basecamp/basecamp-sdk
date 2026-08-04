package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// GoogleDocument represents a link to a Google Workspace document (Docs,
// Sheets, Slides, …) living inside a vault.
type GoogleDocument struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	// URL is the Google Workspace document link — not this record's API URL.
	// Same recordable-overwrites-recording rendering as CloudFile.URL.
	URL             string  `json:"url"`
	AppURL          string  `json:"app_url"`
	BookmarkURL     string  `json:"bookmark_url"`
	SubscriptionURL string  `json:"subscription_url"`
	CommentsCount   int     `json:"comments_count"`
	CommentsURL     string  `json:"comments_url"`
	BoostsCount     int     `json:"boosts_count,omitempty"`
	BoostsURL       string  `json:"boosts_url,omitempty"`
	Position        int     `json:"position,omitempty"`
	Parent          *Parent `json:"parent,omitempty"`
	Bucket          *Bucket `json:"bucket,omitempty"`
	Creator         *Person `json:"creator,omitempty"`
	Description     string  `json:"description"`
	// DescriptionAttachments holds structured metadata for the downloadable
	// files embedded in the rich text Description. @required — the API always
	// sends this array (empty when the description has no inline files). No
	// omitempty, so on marshal a non-nil slice emits its elements ([] when
	// empty) and a nil slice emits null; the key is never dropped. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	// DocumentType is one of "doc", "sheet", "slide", "other".
	DocumentType string `json:"document_type"`
}

// CreateGoogleDocumentRequest is the body for creating a Google document in a
// vault.
type CreateGoogleDocumentRequest struct {
	// URL is the Google Workspace document link (required).
	URL string `json:"url"`
	// DocumentType is one of "doc", "sheet", "slide", "other" (required).
	// BC3 backs this with a Rails enum and rejects an unrecognized value up
	// front with the field-keyed 422
	// {"errors": {"document_type": ["is not a valid document type"]}}.
	DocumentType string `json:"document_type"`
	// Title is the document's title (optional). Omitting it reads back as
	// "Untitled".
	Title string `json:"title,omitempty"`
	// Description is a rich-text description in HTML (optional).
	Description string `json:"description,omitempty"`
	// Status is "active" to publish immediately or "drafted" to keep as a
	// draft (optional, defaults to drafted).
	Status string `json:"status,omitempty"`
	// Subscriptions controls who gets notified and subscribed.
	// nil: field omitted (server default). &[]int64{}: subscribe nobody.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
	// VisibleToClients sets client visibility at create time (optional,
	// tri-state). nil omits the field so the server applies its own default; a
	// non-nil value is sent verbatim. Applies only when creating directly in
	// the tool's vault — an item created inside a folder inherits the folder's
	// visibility. A client caller always creates client-visible records.
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}

// UpdateGoogleDocumentRequest is the body for replacing a Google document.
//
// PUT /{accountId}/google_documents/{googleDocumentId} is a FULL REPLACE. BC3's
// GoogleDocumentsController#update runs
//
//	@recording.update! recording_attributes.merge(recordable: new_google_document)
//
// where new_google_document is GoogleDocument.new(params.require(
// :google_document).permit(:title, :description, :url, :document_type)) — a
// brand-new recordable built from only the permitted params and swapped in
// wholesale. So a field absent from the body is nil on the replacement:
// omitting Description ERASES it, and omitting Title erases that too (the
// document then reads back as "Untitled"). URL and DocumentType are required
// here — DocumentType because an absent or unrecognized value is a 422, and URL
// because it carries a presence validation.
//
// Subscribers are the one exception to omission-clears: a drafted document
// keeps its current subscribers when the request addresses neither
// Subscriptions nor notify.
type UpdateGoogleDocumentRequest struct {
	// URL is the Google Workspace document link (required).
	URL string `json:"url"`
	// DocumentType is one of "doc", "sheet", "slide", "other" (required).
	DocumentType string `json:"document_type"`
	// Title is the document's title. Omitting it clears it.
	Title string `json:"title,omitempty"`
	// Description is the rich-text description in HTML. Omitting it clears it.
	Description string `json:"description,omitempty"`
	// Status is "active" or "drafted".
	Status string `json:"status,omitempty"`
	// Subscriptions replaces the subscriber list when addressed; omit both this
	// and notify to leave a draft's subscribers alone.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
}

// GoogleDocumentsService handles Google document operations.
type GoogleDocumentsService struct {
	client *AccountClient
}

// NewGoogleDocumentsService creates a new GoogleDocumentsService.
func NewGoogleDocumentsService(client *AccountClient) *GoogleDocumentsService {
	return &GoogleDocumentsService{client: client}
}

// Get returns a Google document by ID.
func (s *GoogleDocumentsService) Get(ctx context.Context, googleDocumentID int64) (result *GoogleDocument, err error) {
	op := OperationInfo{
		Service: "GoogleDocuments", Operation: "Get",
		ResourceType: "google_document", IsMutation: false,
		ResourceID: googleDocumentID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetGoogleDocumentWithResponse(ctx, s.client.accountID, googleDocumentID)
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

	googleDocument := googleDocumentFromGenerated(*resp.JSON200)
	return &googleDocument, nil
}

// Create creates a new Google document in a vault.
//
// The create route is bucket-scoped and nested under the vault:
// POST /{accountId}/buckets/{bucketId}/vaults/{vaultId}/google_documents.json.
func (s *GoogleDocumentsService) Create(ctx context.Context, bucketID, vaultID int64, req *CreateGoogleDocumentRequest) (result *GoogleDocument, err error) {
	op := OperationInfo{
		Service: "GoogleDocuments", Operation: "Create",
		ResourceType: "google_document", IsMutation: true,
		ResourceID: vaultID, ProjectID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.URL == "" {
		err = ErrUsage("google document url is required")
		return nil, err
	}
	if req.DocumentType == "" {
		err = ErrUsage("google document document_type is required")
		return nil, err
	}

	body := generated.CreateGoogleDocumentJSONRequestBody{
		Url:              req.URL,
		DocumentType:     req.DocumentType,
		Title:            omitzero(req.Title),
		Description:      omitzero(req.Description),
		Status:           omitzero(req.Status),
		Subscriptions:    req.Subscriptions,
		VisibleToClients: req.VisibleToClients,
	}

	resp, err := s.client.parent.gen.CreateGoogleDocumentWithResponse(ctx, s.client.accountID, bucketID, vaultID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	googleDocument := googleDocumentFromGenerated(*resp.JSON201)
	return &googleDocument, nil
}

// Update replaces a Google document with the full representation in req.
//
// This is a REPLACE, not a patch: see UpdateGoogleDocumentRequest for exactly
// which fields an omission clears.
func (s *GoogleDocumentsService) Update(ctx context.Context, googleDocumentID int64, req *UpdateGoogleDocumentRequest) (result *GoogleDocument, err error) {
	op := OperationInfo{
		Service: "GoogleDocuments", Operation: "Update",
		ResourceType: "google_document", IsMutation: true,
		ResourceID: googleDocumentID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.URL == "" {
		err = ErrUsage("google document url is required")
		return nil, err
	}
	if req.DocumentType == "" {
		err = ErrUsage("google document document_type is required")
		return nil, err
	}

	body := generated.UpdateGoogleDocumentJSONRequestBody{
		Url:           req.URL,
		DocumentType:  req.DocumentType,
		Title:         omitzero(req.Title),
		Description:   omitzero(req.Description),
		Status:        omitzero(req.Status),
		Subscriptions: req.Subscriptions,
	}

	resp, err := s.client.parent.gen.UpdateGoogleDocumentWithResponse(ctx, s.client.accountID, googleDocumentID, body)
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

	googleDocument := googleDocumentFromGenerated(*resp.JSON200)
	return &googleDocument, nil
}

// googleDocumentFromGenerated converts a generated GoogleDocument to our clean
// type.
func googleDocumentFromGenerated(gg generated.GoogleDocument) GoogleDocument {
	g := GoogleDocument{
		ID:               gg.Id,
		Status:           gg.Status,
		VisibleToClients: gg.VisibleToClients,
		CreatedAt:        gg.CreatedAt,
		UpdatedAt:        gg.UpdatedAt,
		Title:            gg.Title,
		InheritsStatus:   gg.InheritsStatus,
		Type:             gg.Type,
		URL:              gg.Url,
		AppURL:           gg.AppUrl,
		BookmarkURL:      deref(gg.BookmarkUrl),
		SubscriptionURL:  deref(gg.SubscriptionUrl),
		CommentsCount:    int(deref(gg.CommentsCount)),
		CommentsURL:      deref(gg.CommentsUrl),
		BoostsCount:      int(deref(gg.BoostsCount)),
		BoostsURL:        deref(gg.BoostsUrl),
		Position:         int(deref(gg.Position)),
		Description:      deref(gg.Description),
		DocumentType:     gg.DocumentType,
	}

	if gg.Parent.Id != 0 || gg.Parent.Title != "" {
		g.Parent = &Parent{
			ID:     gg.Parent.Id,
			Title:  gg.Parent.Title,
			Type:   gg.Parent.Type,
			URL:    gg.Parent.Url,
			AppURL: gg.Parent.AppUrl,
		}
	}

	if gg.Bucket.Id != 0 || gg.Bucket.Name != "" {
		g.Bucket = &Bucket{
			ID:   gg.Bucket.Id,
			Name: gg.Bucket.Name,
			Type: gg.Bucket.Type,
		}
	}

	if gg.Creator.Id != 0 || gg.Creator.Name != "" {
		creator := personFromGenerated(gg.Creator)
		g.Creator = &creator
	}

	g.DescriptionAttachments = richTextAttachmentsFromGenerated(gg.DescriptionAttachments)

	return g
}
