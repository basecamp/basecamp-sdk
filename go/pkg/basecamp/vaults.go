package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// VaultListOptions specifies options for listing vaults.
type VaultListOptions struct {
	// Limit is the maximum number of vaults to return.
	// If 0 (default), returns all vaults. Use a positive value to cap results.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// VaultListResult contains the results from listing vaults.
type VaultListResult struct {
	// Vaults is the list of vaults returned.
	Vaults []Vault
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// DocumentListOptions specifies options for listing documents.
type DocumentListOptions struct {
	// Limit is the maximum number of documents to return.
	// If 0 (default), returns all documents. Use a positive value to cap results.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// DocumentListResult contains the results from listing documents.
type DocumentListResult struct {
	// Documents is the list of documents returned.
	Documents []Document
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// UploadListOptions specifies options for listing uploads.
type UploadListOptions struct {
	// Limit is the maximum number of uploads to return.
	// If 0 (default), returns all uploads. Use a positive value to cap results.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// UploadListResult contains the results from listing uploads.
type UploadListResult struct {
	// Uploads is the list of uploads returned.
	Uploads []Upload
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// Vault represents a Basecamp vault (folder) in the Files tool.
type Vault struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	Position         int       `json:"position,omitempty"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	DocumentsCount   int       `json:"documents_count"`
	DocumentsURL     string    `json:"documents_url"`
	UploadsCount     int       `json:"uploads_count"`
	UploadsURL       string    `json:"uploads_url"`
	VaultsCount      int       `json:"vaults_count"`
	VaultsURL        string    `json:"vaults_url"`
}

// Document represents a Basecamp document in a vault.
type Document struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommentsCount    int       `json:"comments_count"`
	BoostsCount      int       `json:"boosts_count,omitempty"`
	CommentsURL      string    `json:"comments_url"`
	Position         int       `json:"position,omitempty"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
}

// Upload represents a Basecamp uploaded file in a vault.
type Upload struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommentsCount    int       `json:"comments_count"`
	BoostsCount      int       `json:"boosts_count,omitempty"`
	CommentsURL      string    `json:"comments_url"`
	Position         int       `json:"position,omitempty"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Description      string    `json:"description"`
	ContentType      string    `json:"content_type"`
	ByteSize         int64     `json:"byte_size"`
	Width            int       `json:"width,omitempty"`
	Height           int       `json:"height,omitempty"`
	DownloadURL      string    `json:"download_url"`
	Filename         string    `json:"filename"`
}

// UnmarshalJSON decodes an Upload from JSON, handling the BC3 API's
// float-encoded integer dimensions (e.g. "width": 1024.0). Delegates
// through the generated type (which uses FlexInt) so the public struct
// can keep plain int fields while remaining directly decodable from the
// API wire format.
func (u *Upload) UnmarshalJSON(data []byte) error {
	var gu generated.Upload
	if err := json.Unmarshal(data, &gu); err != nil {
		return err
	}
	*u = uploadFromGenerated(gu)
	return nil
}

// CreateVaultRequest specifies the parameters for creating a vault (folder).
type CreateVaultRequest struct {
	// Title is the vault name (required).
	Title string `json:"title"`
}

// UpdateVaultRequest specifies the parameters for updating a vault.
type UpdateVaultRequest struct {
	// Title is the vault name.
	Title string `json:"title,omitempty"`
}

// CreateDocumentRequest specifies the parameters for creating a document.
type CreateDocumentRequest struct {
	// Title is the document title (required).
	Title string `json:"title"`
	// Content is the document body in HTML (optional).
	Content string `json:"content,omitempty"`
	// Status is either "drafted" or "active" (optional, defaults to active).
	Status string `json:"status,omitempty"`
	// Subscriptions controls who gets notified and subscribed.
	// nil: field omitted (server default). &[]int64{}: subscribe nobody. &[]int64{1,2}: those people.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
}

// UpdateDocumentRequest specifies the parameters for updating a document.
type UpdateDocumentRequest struct {
	// Title is the document title.
	Title string `json:"title,omitempty"`
	// Content is the document body in HTML.
	Content string `json:"content,omitempty"`
}

// UpdateUploadRequest specifies the parameters for updating an upload.
type UpdateUploadRequest struct {
	// Description is the upload description.
	Description string `json:"description,omitempty"`
	// BaseName is the filename without extension.
	BaseName string `json:"base_name,omitempty"`
}

// CreateUploadRequest specifies the parameters for creating an upload.
type CreateUploadRequest struct {
	// AttachableSGID is the signed global ID for an uploaded attachment (required).
	// See the Create Attachment endpoint for how to upload files.
	AttachableSGID string `json:"attachable_sgid"`
	// Description is the upload description in HTML (optional).
	Description string `json:"description,omitempty"`
	// BaseName is the filename without extension (optional).
	BaseName string `json:"base_name,omitempty"`
	// Subscriptions controls who gets notified and subscribed.
	// nil: field omitted (server default). &[]int64{}: subscribe nobody. &[]int64{1,2}: those people.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
}

// VaultsService handles vault (folder) operations.
type VaultsService struct {
	client *AccountClient
}

// NewVaultsService creates a new VaultsService.
func NewVaultsService(client *AccountClient) *VaultsService {
	return &VaultsService{client: client}
}

// Get returns a vault by ID.
func (s *VaultsService) Get(ctx context.Context, vaultID int64) (result *Vault, err error) {
	op := OperationInfo{
		Service: "Vaults", Operation: "Get",
		ResourceType: "vault", IsMutation: false,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetVaultWithResponse(ctx, s.client.accountID, vaultID)
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

	vault := vaultFromGenerated(*resp.JSON200)
	return &vault, nil
}

// List returns all subfolders (child vaults) in a vault.
//
// By default, returns all vaults (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of vaults to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned VaultListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *VaultsService) List(ctx context.Context, vaultID int64, opts *VaultListOptions) (result *VaultListResult, err error) {
	op := OperationInfo{
		Service: "Vaults", Operation: "List",
		ResourceType: "vault", IsMutation: false,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListVaultsWithResponse(ctx, s.client.accountID, vaultID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var vaults []Vault
	if resp.JSON200 != nil {
		for _, gv := range *resp.JSON200 {
			vaults = append(vaults, vaultFromGenerated(gv))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &VaultListResult{Vaults: vaults, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for vaults), >0 = specific limit
	limit := 0 // default to all for vaults (structural index)
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(vaults) >= limit {
		return &VaultListResult{Vaults: vaults[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(vaults), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(vaults), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gv generated.Vault
		if err := json.Unmarshal(raw, &gv); err != nil {
			return nil, fmt.Errorf("failed to parse vault: %w", err)
		}
		vaults = append(vaults, vaultFromGenerated(gv))
	}

	return &VaultListResult{Vaults: vaults, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Create creates a new subfolder (child vault) in a vault.
// Returns the created vault.
func (s *VaultsService) Create(ctx context.Context, vaultID int64, req *CreateVaultRequest) (result *Vault, err error) {
	op := OperationInfo{
		Service: "Vaults", Operation: "Create",
		ResourceType: "vault", IsMutation: true,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("vault title is required")
		return nil, err
	}

	body := generated.CreateVaultJSONRequestBody{
		Title: req.Title,
	}

	resp, err := s.client.parent.gen.CreateVaultWithResponse(ctx, s.client.accountID, vaultID, body)
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

	vault := vaultFromGenerated(*resp.JSON201)
	return &vault, nil
}

// Update updates an existing vault.
// Returns the updated vault.
func (s *VaultsService) Update(ctx context.Context, vaultID int64, req *UpdateVaultRequest) (result *Vault, err error) {
	op := OperationInfo{
		Service: "Vaults", Operation: "Update",
		ResourceType: "vault", IsMutation: true,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateVaultJSONRequestBody{}
	if req.Title != "" {
		body.Title = req.Title
	}

	resp, err := s.client.parent.gen.UpdateVaultWithResponse(ctx, s.client.accountID, vaultID, body)
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

	vault := vaultFromGenerated(*resp.JSON200)
	return &vault, nil
}

// DocumentsService handles document operations.
type DocumentsService struct {
	client *AccountClient
}

// NewDocumentsService creates a new DocumentsService.
func NewDocumentsService(client *AccountClient) *DocumentsService {
	return &DocumentsService{client: client}
}

// Get returns a document by ID.
func (s *DocumentsService) Get(ctx context.Context, documentID int64) (result *Document, err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "Get",
		ResourceType: "document", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetDocumentWithResponse(ctx, s.client.accountID, documentID)
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

// List returns all documents in a vault.
//
// By default, returns all documents (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of documents to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned DocumentListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *DocumentsService) List(ctx context.Context, vaultID int64, opts *DocumentListOptions) (result *DocumentListResult, err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "List",
		ResourceType: "document", IsMutation: false,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListDocumentsWithResponse(ctx, s.client.accountID, vaultID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var documents []Document
	if resp.JSON200 != nil {
		for _, gd := range *resp.JSON200 {
			documents = append(documents, documentFromGenerated(gd))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &DocumentListResult{Documents: documents, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for documents), >0 = specific limit
	limit := 0 // default to all for documents
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(documents) >= limit {
		return &DocumentListResult{Documents: documents[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(documents), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(documents), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gd generated.Document
		if err := json.Unmarshal(raw, &gd); err != nil {
			return nil, fmt.Errorf("failed to parse document: %w", err)
		}
		documents = append(documents, documentFromGenerated(gd))
	}

	return &DocumentListResult{Documents: documents, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Create creates a new document in a vault.
// Returns the created document.
func (s *DocumentsService) Create(ctx context.Context, vaultID int64, req *CreateDocumentRequest) (result *Document, err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "Create",
		ResourceType: "document", IsMutation: true,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("document title is required")
		return nil, err
	}

	body := generated.CreateDocumentJSONRequestBody{
		Title:         req.Title,
		Content:       req.Content,
		Status:        req.Status,
		Subscriptions: req.Subscriptions,
	}

	resp, err := s.client.parent.gen.CreateDocumentWithResponse(ctx, s.client.accountID, vaultID, body)
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

	document := documentFromGenerated(*resp.JSON201)
	return &document, nil
}

// Update updates an existing document.
// Returns the updated document.
func (s *DocumentsService) Update(ctx context.Context, documentID int64, req *UpdateDocumentRequest) (result *Document, err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "Update",
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

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateDocumentJSONRequestBody{}
	if req.Title != "" {
		body.Title = req.Title
	}
	if req.Content != "" {
		body.Content = req.Content
	}

	resp, err := s.client.parent.gen.UpdateDocumentWithResponse(ctx, s.client.accountID, documentID, body)
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

// Trash moves a document to the trash.
// Trashed documents can be recovered from the trash.
func (s *DocumentsService) Trash(ctx context.Context, documentID int64) (err error) {
	op := OperationInfo{
		Service: "Documents", Operation: "Trash",
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

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, documentID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// UploadsService handles upload (file) operations.
type UploadsService struct {
	client *AccountClient
}

// NewUploadsService creates a new UploadsService.
func NewUploadsService(client *AccountClient) *UploadsService {
	return &UploadsService{client: client}
}

// Get returns an upload by ID.
func (s *UploadsService) Get(ctx context.Context, uploadID int64) (result *Upload, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "Get",
		ResourceType: "upload", IsMutation: false,
		ResourceID: uploadID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetUploadWithResponse(ctx, s.client.accountID, uploadID)
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

	upload := uploadFromGenerated(*resp.JSON200)
	return &upload, nil
}

// List returns all uploads in a vault.
//
// By default, returns all uploads (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of uploads to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned UploadListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *UploadsService) List(ctx context.Context, vaultID int64, opts *UploadListOptions) (result *UploadListResult, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "List",
		ResourceType: "upload", IsMutation: false,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListUploadsWithResponse(ctx, s.client.accountID, vaultID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var uploads []Upload
	if resp.JSON200 != nil {
		for _, gu := range *resp.JSON200 {
			uploads = append(uploads, uploadFromGenerated(gu))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &UploadListResult{Uploads: uploads, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for uploads), >0 = specific limit
	limit := 0 // default to all for uploads
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(uploads) >= limit {
		return &UploadListResult{Uploads: uploads[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(uploads), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(uploads), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gu generated.Upload
		if err := json.Unmarshal(raw, &gu); err != nil {
			return nil, fmt.Errorf("failed to parse upload: %w", err)
		}
		uploads = append(uploads, uploadFromGenerated(gu))
	}

	return &UploadListResult{Uploads: uploads, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Update updates an existing upload.
// Returns the updated upload.
func (s *UploadsService) Update(ctx context.Context, uploadID int64, req *UpdateUploadRequest) (result *Upload, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "Update",
		ResourceType: "upload", IsMutation: true,
		ResourceID: uploadID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateUploadJSONRequestBody{}
	if req.Description != "" {
		body.Description = req.Description
	}
	if req.BaseName != "" {
		body.BaseName = req.BaseName
	}

	resp, err := s.client.parent.gen.UpdateUploadWithResponse(ctx, s.client.accountID, uploadID, body)
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

	upload := uploadFromGenerated(*resp.JSON200)
	return &upload, nil
}

// Create creates a new upload in a vault.
// The attachable_sgid must be obtained from the Create Attachment endpoint.
// Returns the created upload.
func (s *UploadsService) Create(ctx context.Context, vaultID int64, req *CreateUploadRequest) (result *Upload, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "Create",
		ResourceType: "upload", IsMutation: true,
		ResourceID: vaultID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.AttachableSGID == "" {
		err = ErrUsage("upload attachable_sgid is required")
		return nil, err
	}

	body := generated.CreateUploadJSONRequestBody{
		AttachableSgid: req.AttachableSGID,
		Description:    req.Description,
		BaseName:       req.BaseName,
		Subscriptions:  req.Subscriptions,
	}

	resp, err := s.client.parent.gen.CreateUploadWithResponse(ctx, s.client.accountID, vaultID, body)
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

	upload := uploadFromGenerated(*resp.JSON201)
	return &upload, nil
}

// Trash moves an upload to the trash.
// Trashed uploads can be recovered from the trash.
func (s *UploadsService) Trash(ctx context.Context, uploadID int64) (err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "Trash",
		ResourceType: "upload", IsMutation: true,
		ResourceID: uploadID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, uploadID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// UploadVersionListOptions specifies options for listing upload versions.
type UploadVersionListOptions struct {
	// Limit is the maximum number of versions to return.
	// If 0 (default), returns all versions.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// UploadVersionListResult contains the results from listing upload versions.
type UploadVersionListResult struct {
	// Versions is the list of upload versions returned.
	Versions []Upload
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ListVersions returns all versions of an upload.
//
// Pagination options:
//   - Limit: maximum number of versions to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned UploadVersionListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *UploadsService) ListVersions(ctx context.Context, uploadID int64, opts *UploadVersionListOptions) (result *UploadVersionListResult, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "ListVersions",
		ResourceType: "upload", IsMutation: false,
		ResourceID: uploadID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListUploadVersionsWithResponse(ctx, s.client.accountID, uploadID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var versions []Upload
	if resp.JSON200 != nil {
		for _, gu := range *resp.JSON200 {
			versions = append(versions, uploadFromGenerated(gu))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &UploadVersionListResult{Versions: versions, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for versions)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(versions) >= limit {
		return &UploadVersionListResult{Versions: versions[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(versions), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(versions), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gu generated.Upload
		if err := json.Unmarshal(raw, &gu); err != nil {
			return nil, fmt.Errorf("failed to parse upload version: %w", err)
		}
		versions = append(versions, uploadFromGenerated(gu))
	}

	return &UploadVersionListResult{Versions: versions, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// DownloadResult contains the result from downloading an upload.
type DownloadResult struct {
	// Body is the file content. Caller must close this.
	Body io.ReadCloser
	// ContentType is the MIME type of the file.
	ContentType string
	// ContentLength is the size of the file in bytes (-1 if unknown).
	ContentLength int64
	// Filename is the name of the file.
	Filename string
}

// Download fetches the file content from an upload's download URL.
// The caller is responsible for closing the returned Body.
//
// This method first fetches the upload metadata to retrieve the download URL,
// then fetches the file content. The API returns download_url as an
// API-host URL that requires Bearer auth and 302-redirects to a signed
// S3 URL, so the first hop is authenticated and the second hop fetches
// the signed URL without auth.
func (s *UploadsService) Download(ctx context.Context, uploadID int64) (result *DownloadResult, err error) {
	op := OperationInfo{
		Service: "Uploads", Operation: "Download",
		ResourceType: "upload", IsMutation: false,
		ResourceID: uploadID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	upload, err := s.Get(ctx, uploadID)
	if err != nil {
		return nil, err
	}

	if upload.DownloadURL == "" {
		err = ErrUsage("upload has no download URL")
		return nil, err
	}

	result, err = s.client.parent.fetchAPIDownload(ctx, upload.DownloadURL)
	if err != nil {
		return nil, err
	}
	// Prefer the filename from upload metadata over URL-derived filename.
	if upload.Filename != "" {
		result.Filename = upload.Filename
	}
	return result, nil
}

// vaultFromGenerated converts a generated Vault to our clean Vault type.
func vaultFromGenerated(gv generated.Vault) Vault {
	v := Vault{
		Status:           gv.Status,
		VisibleToClients: gv.VisibleToClients,
		Title:            gv.Title,
		InheritsStatus:   gv.InheritsStatus,
		Type:             gv.Type,
		URL:              gv.Url,
		AppURL:           gv.AppUrl,
		BookmarkURL:      gv.BookmarkUrl,
		Position:         int(gv.Position),
		DocumentsCount:   int(gv.DocumentsCount),
		DocumentsURL:     gv.DocumentsUrl,
		UploadsCount:     int(gv.UploadsCount),
		UploadsURL:       gv.UploadsUrl,
		VaultsCount:      int(gv.VaultsCount),
		VaultsURL:        gv.VaultsUrl,
		CreatedAt:        gv.CreatedAt,
		UpdatedAt:        gv.UpdatedAt,
	}

	if gv.Id != 0 {
		v.ID = gv.Id
	}

	if gv.Parent.Id != 0 || gv.Parent.Title != "" {
		v.Parent = &Parent{
			ID:     gv.Parent.Id,
			Title:  gv.Parent.Title,
			Type:   gv.Parent.Type,
			URL:    gv.Parent.Url,
			AppURL: gv.Parent.AppUrl,
		}
	}

	if gv.Bucket.Id != 0 || gv.Bucket.Name != "" {
		v.Bucket = &Bucket{
			ID:   gv.Bucket.Id,
			Name: gv.Bucket.Name,
			Type: gv.Bucket.Type,
		}
	}

	if gv.Creator.Id != 0 || gv.Creator.Name != "" {
		creator := personFromGenerated(gv.Creator)
		v.Creator = &creator
	}

	return v
}

// documentFromGenerated converts a generated Document to our clean Document type.
func documentFromGenerated(gd generated.Document) Document {
	d := Document{
		Status:           gd.Status,
		VisibleToClients: gd.VisibleToClients,
		Title:            gd.Title,
		InheritsStatus:   gd.InheritsStatus,
		Type:             gd.Type,
		URL:              gd.Url,
		AppURL:           gd.AppUrl,
		BookmarkURL:      gd.BookmarkUrl,
		SubscriptionURL:  gd.SubscriptionUrl,
		CommentsCount:    int(gd.CommentsCount),
		BoostsCount:      int(gd.BoostsCount),
		CommentsURL:      gd.CommentsUrl,
		Position:         int(gd.Position),
		Content:          gd.Content,
		CreatedAt:        gd.CreatedAt,
		UpdatedAt:        gd.UpdatedAt,
	}

	if gd.Id != 0 {
		d.ID = gd.Id
	}

	if gd.Parent.Id != 0 || gd.Parent.Title != "" {
		d.Parent = &Parent{
			ID:     gd.Parent.Id,
			Title:  gd.Parent.Title,
			Type:   gd.Parent.Type,
			URL:    gd.Parent.Url,
			AppURL: gd.Parent.AppUrl,
		}
	}

	if gd.Bucket.Id != 0 || gd.Bucket.Name != "" {
		d.Bucket = &Bucket{
			ID:   gd.Bucket.Id,
			Name: gd.Bucket.Name,
			Type: gd.Bucket.Type,
		}
	}

	if gd.Creator.Id != 0 || gd.Creator.Name != "" {
		creator := personFromGenerated(gd.Creator)
		d.Creator = &creator
	}

	return d
}

// uploadFromGenerated converts a generated Upload to our clean Upload type.
func uploadFromGenerated(gu generated.Upload) Upload {
	u := Upload{
		Status:           gu.Status,
		VisibleToClients: gu.VisibleToClients,
		Title:            gu.Title,
		InheritsStatus:   gu.InheritsStatus,
		Type:             gu.Type,
		URL:              gu.Url,
		AppURL:           gu.AppUrl,
		BookmarkURL:      gu.BookmarkUrl,
		SubscriptionURL:  gu.SubscriptionUrl,
		CommentsCount:    int(gu.CommentsCount),
		BoostsCount:      int(gu.BoostsCount),
		CommentsURL:      gu.CommentsUrl,
		Position:         int(gu.Position),
		Description:      gu.Description,
		ContentType:      gu.ContentType,
		ByteSize:         gu.ByteSize,
		Width:            int(gu.Width),
		Height:           int(gu.Height),
		DownloadURL:      gu.DownloadUrl,
		Filename:         gu.Filename,
		CreatedAt:        gu.CreatedAt,
		UpdatedAt:        gu.UpdatedAt,
	}

	if gu.Id != 0 {
		u.ID = gu.Id
	}

	if gu.Parent.Id != 0 || gu.Parent.Title != "" {
		u.Parent = &Parent{
			ID:     gu.Parent.Id,
			Title:  gu.Parent.Title,
			Type:   gu.Parent.Type,
			URL:    gu.Parent.Url,
			AppURL: gu.Parent.AppUrl,
		}
	}

	if gu.Bucket.Id != 0 || gu.Bucket.Name != "" {
		u.Bucket = &Bucket{
			ID:   gu.Bucket.Id,
			Name: gu.Bucket.Name,
			Type: gu.Bucket.Type,
		}
	}

	if gu.Creator.Id != 0 || gu.Creator.Name != "" {
		creator := personFromGenerated(gu.Creator)
		u.Creator = &creator
	}

	return u
}
