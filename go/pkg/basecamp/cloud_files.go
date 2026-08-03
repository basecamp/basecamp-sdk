package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// CloudFile represents a link to a file hosted on an external service
// (Dropbox, Google Drive, Figma, Notion, …) living inside a vault.
type CloudFile struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	// URL is the link on the EXTERNAL service — not this record's API URL.
	// BC3's cloud_files jbuilder renders the shared recording partial first and
	// then `json.(recording.recordable, :url, :service)`, which overwrites the
	// recording's url key with the recordable's. AppURL is still this record's
	// Basecamp URL.
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
	// Service describes the external service this file lives on, embedded so
	// callers can render its name, supporting text, and example URL without a
	// hard-coded catalogue.
	Service CloudFileService `json:"service"`
}

// CloudFileService is the external service a cloud file points at.
type CloudFileService struct {
	// Name is the display name ("Dropbox", "Google Docs").
	Name string `json:"name"`
	// ExampleURL is a representative URL, suitable as an input placeholder.
	ExampleURL string `json:"example_url"`
	// Code is the short identifier ("dropbox", "google_doc", "figma", "other").
	Code string `json:"code"`
	// ValidPatterns are the regular expressions a cloud file's URL is validated
	// against. A URL matching none of the selected service's patterns is a 422.
	ValidPatterns []string `json:"valid_patterns"`
	// SupportingText is a human-readable hint ("a file or folder on Dropbox").
	// Empty for services that declare none.
	SupportingText string `json:"supporting_text,omitempty"`
}

// CreateCloudFileRequest is the body for creating a cloud file in a vault.
type CreateCloudFileRequest struct {
	// URL is the link on the external service (required). It is validated
	// against the selected service's URL patterns, so it must be a real link
	// for that service.
	URL string `json:"url"`
	// Service is the short service identifier (required) — "dropbox",
	// "google_doc", "figma", … Use "other" for anything matching no recognized
	// service; it accepts any well-formed HTTPS URL.
	Service string `json:"service"`
	// Title is the cloud file's title (optional). Omitting it reads back as
	// "Untitled".
	Title string `json:"title,omitempty"`
	// Description is a rich-text description in HTML (optional).
	Description string `json:"description,omitempty"`
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

// UpdateCloudFileRequest is the body for replacing a cloud file.
//
// PUT /{accountId}/cloud_files/{cloudFileId} is a FULL REPLACE. BC3's
// CloudFilesController#update runs
//
//	@recording.update! recording_attributes.merge(recordable: new_cloud_file)
//
// where new_cloud_file is CloudFile.new(params.require(:cloud_file).permit(
// :title, :description, :url, :service)) — a brand-new recordable built from
// only the permitted params and swapped in wholesale. So a field absent from
// the body is nil on the replacement: omitting Description ERASES it, and
// omitting Title erases that too (the cloud file then reads back as
// "Untitled"). URL and Service carry validations, so they are required here
// rather than clearable — a request without them is a 422, not a silent wipe.
//
// Updating a drafted cloud file also publishes it. Subscribers are the one
// exception to omission-clears: a drafted cloud file keeps its current
// subscribers when the request addresses neither Subscriptions nor notify.
type UpdateCloudFileRequest struct {
	// URL is the link on the external service (required).
	URL string `json:"url"`
	// Service is the short service identifier (required).
	Service string `json:"service"`
	// Title is the cloud file's title. Omitting it clears it.
	Title string `json:"title,omitempty"`
	// Description is the rich-text description in HTML. Omitting it clears it.
	Description string `json:"description,omitempty"`
	// Subscriptions replaces the subscriber list when addressed; omit both this
	// and notify to leave a draft's subscribers alone.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
}

// CloudFilesService handles cloud file operations.
type CloudFilesService struct {
	client *AccountClient
}

// NewCloudFilesService creates a new CloudFilesService.
func NewCloudFilesService(client *AccountClient) *CloudFilesService {
	return &CloudFilesService{client: client}
}

// Get returns a cloud file by ID.
func (s *CloudFilesService) Get(ctx context.Context, cloudFileID int64) (result *CloudFile, err error) {
	op := OperationInfo{
		Service: "CloudFiles", Operation: "Get",
		ResourceType: "cloud_file", IsMutation: false,
		ResourceID: cloudFileID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCloudFileWithResponse(ctx, s.client.accountID, cloudFileID)
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

	cloudFile := cloudFileFromGenerated(*resp.JSON200)
	return &cloudFile, nil
}

// Create creates a new cloud file in a vault.
//
// The create route is bucket-scoped and nested under the vault:
// POST /{accountId}/buckets/{bucketId}/vaults/{vaultId}/cloud_files.json.
func (s *CloudFilesService) Create(ctx context.Context, bucketID, vaultID int64, req *CreateCloudFileRequest) (result *CloudFile, err error) {
	op := OperationInfo{
		Service: "CloudFiles", Operation: "Create",
		ResourceType: "cloud_file", IsMutation: true,
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
		err = ErrUsage("cloud file url is required")
		return nil, err
	}
	if req.Service == "" {
		err = ErrUsage("cloud file service is required")
		return nil, err
	}

	body := generated.CreateCloudFileJSONRequestBody{
		Url:              req.URL,
		Service:          req.Service,
		Title:            omitzero(req.Title),
		Description:      omitzero(req.Description),
		Subscriptions:    req.Subscriptions,
		VisibleToClients: req.VisibleToClients,
	}

	resp, err := s.client.parent.gen.CreateCloudFileWithResponse(ctx, s.client.accountID, bucketID, vaultID, body)
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

	cloudFile := cloudFileFromGenerated(*resp.JSON201)
	return &cloudFile, nil
}

// Update replaces a cloud file with the full representation in req.
//
// This is a REPLACE, not a patch: see UpdateCloudFileRequest for exactly which
// fields an omission clears.
func (s *CloudFilesService) Update(ctx context.Context, cloudFileID int64, req *UpdateCloudFileRequest) (result *CloudFile, err error) {
	op := OperationInfo{
		Service: "CloudFiles", Operation: "Update",
		ResourceType: "cloud_file", IsMutation: true,
		ResourceID: cloudFileID,
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
		err = ErrUsage("cloud file url is required")
		return nil, err
	}
	if req.Service == "" {
		err = ErrUsage("cloud file service is required")
		return nil, err
	}

	body := generated.UpdateCloudFileJSONRequestBody{
		Url:           req.URL,
		Service:       req.Service,
		Title:         omitzero(req.Title),
		Description:   omitzero(req.Description),
		Subscriptions: req.Subscriptions,
	}

	resp, err := s.client.parent.gen.UpdateCloudFileWithResponse(ctx, s.client.accountID, cloudFileID, body)
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

	cloudFile := cloudFileFromGenerated(*resp.JSON200)
	return &cloudFile, nil
}

// cloudFileFromGenerated converts a generated CloudFile to our clean type.
func cloudFileFromGenerated(gc generated.CloudFile) CloudFile {
	c := CloudFile{
		ID:               gc.Id,
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      deref(gc.BookmarkUrl),
		SubscriptionURL:  deref(gc.SubscriptionUrl),
		CommentsCount:    int(deref(gc.CommentsCount)),
		CommentsURL:      deref(gc.CommentsUrl),
		BoostsCount:      int(deref(gc.BoostsCount)),
		BoostsURL:        deref(gc.BoostsUrl),
		Position:         int(deref(gc.Position)),
		Description:      deref(gc.Description),
		Service: CloudFileService{
			Name:           gc.Service.Name,
			ExampleURL:     gc.Service.ExampleUrl,
			Code:           gc.Service.Code,
			ValidPatterns:  gc.Service.ValidPatterns,
			SupportingText: deref(gc.Service.SupportingText),
		},
	}

	if gc.Parent.Id != 0 || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     gc.Parent.Id,
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		creator := personFromGenerated(gc.Creator)
		c.Creator = &creator
	}

	c.DescriptionAttachments = richTextAttachmentsFromGenerated(gc.DescriptionAttachments)

	return c
}
