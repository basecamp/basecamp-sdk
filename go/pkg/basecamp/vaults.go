package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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

	resp, err := s.client.gen.GetVaultWithResponse(ctx, bucketID, vaultID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	vault := vaultFromGenerated(resp.JSON200.Vault)
	return &vault, nil
}

// List returns all subfolders (child vaults) in a vault.
// bucketID is the project ID, vaultID is the parent vault ID.
func (s *VaultsService) List(ctx context.Context, bucketID, vaultID int64) ([]Vault, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListVaultsWithResponse(ctx, bucketID, vaultID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	vaults := make([]Vault, 0, len(resp.JSON200.Vaults))
	for _, gv := range resp.JSON200.Vaults {
		vaults = append(vaults, vaultFromGenerated(gv))
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

	body := generated.CreateVaultJSONRequestBody{
		Title: req.Title,
	}

	resp, err := s.client.gen.CreateVaultWithResponse(ctx, bucketID, vaultID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	vault := vaultFromGenerated(resp.JSON200.Vault)
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

	body := generated.UpdateVaultJSONRequestBody{
		Title: req.Title,
	}

	resp, err := s.client.gen.UpdateVaultWithResponse(ctx, bucketID, vaultID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	vault := vaultFromGenerated(resp.JSON200.Vault)
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

	resp, err := s.client.gen.GetDocumentWithResponse(ctx, bucketID, documentID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	document := documentFromGenerated(resp.JSON200.Document)
	return &document, nil
}

// List returns all documents in a vault.
// bucketID is the project ID, vaultID is the vault ID.
func (s *DocumentsService) List(ctx context.Context, bucketID, vaultID int64) ([]Document, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListDocumentsWithResponse(ctx, bucketID, vaultID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	documents := make([]Document, 0, len(resp.JSON200.Documents))
	for _, gd := range resp.JSON200.Documents {
		documents = append(documents, documentFromGenerated(gd))
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

	body := generated.CreateDocumentJSONRequestBody{
		Title:   req.Title,
		Content: req.Content,
		Status:  req.Status,
	}

	resp, err := s.client.gen.CreateDocumentWithResponse(ctx, bucketID, vaultID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	document := documentFromGenerated(resp.JSON200.Document)
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

	body := generated.UpdateDocumentJSONRequestBody{
		Title:   req.Title,
		Content: req.Content,
	}

	resp, err := s.client.gen.UpdateDocumentWithResponse(ctx, bucketID, documentID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	document := documentFromGenerated(resp.JSON200.Document)
	return &document, nil
}

// Trash moves a document to the trash.
// bucketID is the project ID, documentID is the document ID.
// Trashed documents can be recovered from the trash.
func (s *DocumentsService) Trash(ctx context.Context, bucketID, documentID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, bucketID, documentID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
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

	resp, err := s.client.gen.GetUploadWithResponse(ctx, bucketID, uploadID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	upload := uploadFromGenerated(resp.JSON200.Upload)
	return &upload, nil
}

// List returns all uploads in a vault.
// bucketID is the project ID, vaultID is the vault ID.
func (s *UploadsService) List(ctx context.Context, bucketID, vaultID int64) ([]Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListUploadsWithResponse(ctx, bucketID, vaultID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	uploads := make([]Upload, 0, len(resp.JSON200.Uploads))
	for _, gu := range resp.JSON200.Uploads {
		uploads = append(uploads, uploadFromGenerated(gu))
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

	body := generated.UpdateUploadJSONRequestBody{
		Description: req.Description,
		BaseName:    req.BaseName,
	}

	resp, err := s.client.gen.UpdateUploadWithResponse(ctx, bucketID, uploadID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	upload := uploadFromGenerated(resp.JSON200.Upload)
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

	body := generated.CreateUploadJSONRequestBody{
		AttachableSgid: req.AttachableSGID,
		Description:    req.Description,
		BaseName:       req.BaseName,
	}

	resp, err := s.client.gen.CreateUploadWithResponse(ctx, bucketID, vaultID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	upload := uploadFromGenerated(resp.JSON200.Upload)
	return &upload, nil
}

// Trash moves an upload to the trash.
// bucketID is the project ID, uploadID is the upload ID.
// Trashed uploads can be recovered from the trash.
func (s *UploadsService) Trash(ctx context.Context, bucketID, uploadID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, bucketID, uploadID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// ListVersions returns all versions of an upload.
// bucketID is the project ID, uploadID is the upload ID.
func (s *UploadsService) ListVersions(ctx context.Context, bucketID, uploadID int64) ([]Upload, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListUploadVersionsWithResponse(ctx, bucketID, uploadID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	uploads := make([]Upload, 0, len(resp.JSON200.Uploads))
	for _, gu := range resp.JSON200.Uploads {
		uploads = append(uploads, uploadFromGenerated(gu))
	}
	return uploads, nil
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

	if gv.Id != nil {
		v.ID = *gv.Id
	}

	if gv.Parent.Id != nil || gv.Parent.Title != "" {
		v.Parent = &Parent{
			ID:     derefInt64(gv.Parent.Id),
			Title:  gv.Parent.Title,
			Type:   gv.Parent.Type,
			URL:    gv.Parent.Url,
			AppURL: gv.Parent.AppUrl,
		}
	}

	if gv.Bucket.Id != nil || gv.Bucket.Name != "" {
		v.Bucket = &Bucket{
			ID:   derefInt64(gv.Bucket.Id),
			Name: gv.Bucket.Name,
			Type: gv.Bucket.Type,
		}
	}

	if gv.Creator.Id != nil || gv.Creator.Name != "" {
		v.Creator = &Person{
			ID:           derefInt64(gv.Creator.Id),
			Name:         gv.Creator.Name,
			EmailAddress: gv.Creator.EmailAddress,
			AvatarURL:    gv.Creator.AvatarUrl,
			Admin:        gv.Creator.Admin,
			Owner:        gv.Creator.Owner,
		}
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
		CommentsURL:      gd.CommentsUrl,
		Position:         int(gd.Position),
		Content:          gd.Content,
		CreatedAt:        gd.CreatedAt,
		UpdatedAt:        gd.UpdatedAt,
	}

	if gd.Id != nil {
		d.ID = *gd.Id
	}

	if gd.Parent.Id != nil || gd.Parent.Title != "" {
		d.Parent = &Parent{
			ID:     derefInt64(gd.Parent.Id),
			Title:  gd.Parent.Title,
			Type:   gd.Parent.Type,
			URL:    gd.Parent.Url,
			AppURL: gd.Parent.AppUrl,
		}
	}

	if gd.Bucket.Id != nil || gd.Bucket.Name != "" {
		d.Bucket = &Bucket{
			ID:   derefInt64(gd.Bucket.Id),
			Name: gd.Bucket.Name,
			Type: gd.Bucket.Type,
		}
	}

	if gd.Creator.Id != nil || gd.Creator.Name != "" {
		d.Creator = &Person{
			ID:           derefInt64(gd.Creator.Id),
			Name:         gd.Creator.Name,
			EmailAddress: gd.Creator.EmailAddress,
			AvatarURL:    gd.Creator.AvatarUrl,
			Admin:        gd.Creator.Admin,
			Owner:        gd.Creator.Owner,
		}
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

	if gu.Id != nil {
		u.ID = *gu.Id
	}

	if gu.Parent.Id != nil || gu.Parent.Title != "" {
		u.Parent = &Parent{
			ID:     derefInt64(gu.Parent.Id),
			Title:  gu.Parent.Title,
			Type:   gu.Parent.Type,
			URL:    gu.Parent.Url,
			AppURL: gu.Parent.AppUrl,
		}
	}

	if gu.Bucket.Id != nil || gu.Bucket.Name != "" {
		u.Bucket = &Bucket{
			ID:   derefInt64(gu.Bucket.Id),
			Name: gu.Bucket.Name,
			Type: gu.Bucket.Type,
		}
	}

	if gu.Creator.Id != nil || gu.Creator.Name != "" {
		u.Creator = &Person{
			ID:           derefInt64(gu.Creator.Id),
			Name:         gu.Creator.Name,
			EmailAddress: gu.Creator.EmailAddress,
			AvatarURL:    gu.Creator.AvatarUrl,
			Admin:        gu.Creator.Admin,
			Owner:        gu.Creator.Owner,
		}
	}

	return u
}
