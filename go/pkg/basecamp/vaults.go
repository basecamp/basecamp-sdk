package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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
}

// VaultsService handles vault (folder) operations.
type VaultsService struct {
	client *Client
}

// NewVaultsService creates a new VaultsService.
func NewVaultsService(client *Client) *VaultsService {
	return &VaultsService{client: client}
}

// Get returns a vault by ID.
// bucketID is the project ID, vaultID is the vault ID.
func (s *VaultsService) Get(ctx context.Context, bucketID, vaultID int64) (*Vault, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d.json", bucketID, vaultID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var vault Vault
	if err := resp.UnmarshalData(&vault); err != nil {
		return nil, fmt.Errorf("failed to parse vault: %w", err)
	}

	return &vault, nil
}

// List returns all subfolders (child vaults) in a vault.
// bucketID is the project ID, vaultID is the parent vault ID.
func (s *VaultsService) List(ctx context.Context, bucketID, vaultID int64) ([]Vault, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/vaults.json", bucketID, vaultID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	vaults := make([]Vault, 0, len(results))
	for _, raw := range results {
		var v Vault
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("failed to parse vault: %w", err)
		}
		vaults = append(vaults, v)
	}

	return vaults, nil
}

// Create creates a new subfolder (child vault) in a vault.
// bucketID is the project ID, vaultID is the parent vault ID.
// Returns the created vault.
func (s *VaultsService) Create(ctx context.Context, bucketID, vaultID int64, req *CreateVaultRequest) (*Vault, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("vault title is required")
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/vaults.json", bucketID, vaultID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var vault Vault
	if err := resp.UnmarshalData(&vault); err != nil {
		return nil, fmt.Errorf("failed to parse vault: %w", err)
	}

	return &vault, nil
}

// Update updates an existing vault.
// bucketID is the project ID, vaultID is the vault ID.
// Returns the updated vault.
func (s *VaultsService) Update(ctx context.Context, bucketID, vaultID int64, req *UpdateVaultRequest) (*Vault, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d.json", bucketID, vaultID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var vault Vault
	if err := resp.UnmarshalData(&vault); err != nil {
		return nil, fmt.Errorf("failed to parse vault: %w", err)
	}

	return &vault, nil
}

// DocumentsService handles document operations.
type DocumentsService struct {
	client *Client
}

// NewDocumentsService creates a new DocumentsService.
func NewDocumentsService(client *Client) *DocumentsService {
	return &DocumentsService{client: client}
}

// Get returns a document by ID.
// bucketID is the project ID, documentID is the document ID.
func (s *DocumentsService) Get(ctx context.Context, bucketID, documentID int64) (*Document, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/documents/%d.json", bucketID, documentID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var document Document
	if err := resp.UnmarshalData(&document); err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return &document, nil
}

// List returns all documents in a vault.
// bucketID is the project ID, vaultID is the vault ID.
func (s *DocumentsService) List(ctx context.Context, bucketID, vaultID int64) ([]Document, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/documents.json", bucketID, vaultID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	documents := make([]Document, 0, len(results))
	for _, raw := range results {
		var d Document
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("failed to parse document: %w", err)
		}
		documents = append(documents, d)
	}

	return documents, nil
}

// Create creates a new document in a vault.
// bucketID is the project ID, vaultID is the vault ID.
// Returns the created document.
func (s *DocumentsService) Create(ctx context.Context, bucketID, vaultID int64, req *CreateDocumentRequest) (*Document, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("document title is required")
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/documents.json", bucketID, vaultID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var document Document
	if err := resp.UnmarshalData(&document); err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return &document, nil
}

// Update updates an existing document.
// bucketID is the project ID, documentID is the document ID.
// Returns the updated document.
func (s *DocumentsService) Update(ctx context.Context, bucketID, documentID int64, req *UpdateDocumentRequest) (*Document, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/documents/%d.json", bucketID, documentID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var document Document
	if err := resp.UnmarshalData(&document); err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return &document, nil
}

// Trash moves a document to the trash.
// bucketID is the project ID, documentID is the document ID.
// Trashed documents can be recovered from the trash.
func (s *DocumentsService) Trash(ctx context.Context, bucketID, documentID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, documentID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// UploadsService handles upload (file) operations.
type UploadsService struct {
	client *Client
}

// NewUploadsService creates a new UploadsService.
func NewUploadsService(client *Client) *UploadsService {
	return &UploadsService{client: client}
}

// Get returns an upload by ID.
// bucketID is the project ID, uploadID is the upload ID.
func (s *UploadsService) Get(ctx context.Context, bucketID, uploadID int64) (*Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/uploads/%d.json", bucketID, uploadID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var upload Upload
	if err := resp.UnmarshalData(&upload); err != nil {
		return nil, fmt.Errorf("failed to parse upload: %w", err)
	}

	return &upload, nil
}

// List returns all uploads in a vault.
// bucketID is the project ID, vaultID is the vault ID.
func (s *UploadsService) List(ctx context.Context, bucketID, vaultID int64) ([]Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/uploads.json", bucketID, vaultID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	uploads := make([]Upload, 0, len(results))
	for _, raw := range results {
		var u Upload
		if err := json.Unmarshal(raw, &u); err != nil {
			return nil, fmt.Errorf("failed to parse upload: %w", err)
		}
		uploads = append(uploads, u)
	}

	return uploads, nil
}

// Update updates an existing upload.
// bucketID is the project ID, uploadID is the upload ID.
// Returns the updated upload.
func (s *UploadsService) Update(ctx context.Context, bucketID, uploadID int64, req *UpdateUploadRequest) (*Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/uploads/%d.json", bucketID, uploadID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var upload Upload
	if err := resp.UnmarshalData(&upload); err != nil {
		return nil, fmt.Errorf("failed to parse upload: %w", err)
	}

	return &upload, nil
}

// Create creates a new upload in a vault.
// bucketID is the project ID, vaultID is the vault ID.
// The attachable_sgid must be obtained from the Create Attachment endpoint.
// Returns the created upload.
func (s *UploadsService) Create(ctx context.Context, bucketID, vaultID int64, req *CreateUploadRequest) (*Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.AttachableSGID == "" {
		return nil, ErrUsage("upload attachable_sgid is required")
	}

	path := fmt.Sprintf("/buckets/%d/vaults/%d/uploads.json", bucketID, vaultID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var upload Upload
	if err := resp.UnmarshalData(&upload); err != nil {
		return nil, fmt.Errorf("failed to parse upload: %w", err)
	}

	return &upload, nil
}

// Trash moves an upload to the trash.
// bucketID is the project ID, uploadID is the upload ID.
// Trashed uploads can be recovered from the trash.
func (s *UploadsService) Trash(ctx context.Context, bucketID, uploadID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, uploadID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// ListVersions returns all versions of an upload.
// bucketID is the project ID, uploadID is the upload ID.
func (s *UploadsService) ListVersions(ctx context.Context, bucketID, uploadID int64) ([]Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/uploads/%d/versions.json", bucketID, uploadID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var uploads []Upload
	if err := resp.UnmarshalData(&uploads); err != nil {
		return nil, err
	}
	return uploads, nil
}
