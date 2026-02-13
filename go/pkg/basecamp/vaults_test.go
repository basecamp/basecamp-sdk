package basecamp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vaultsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "vaults")
}

func documentsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "documents")
}

func uploadsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "uploads")
}

func loadVaultsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(vaultsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func loadDocumentsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(documentsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func loadUploadsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(uploadsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

// Vault tests

func TestVault_UnmarshalGet(t *testing.T) {
	data := loadVaultsFixture(t, "get.json")

	var vault Vault
	if err := json.Unmarshal(data, &vault); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if vault.ID != 1069479098 {
		t.Errorf("expected ID 1069479098, got %d", vault.ID)
	}
	if vault.Title != "Docs & Files" {
		t.Errorf("expected title 'Docs & Files', got %q", vault.Title)
	}
	if vault.Type != "Vault" {
		t.Errorf("expected type 'Vault', got %q", vault.Type)
	}
	if vault.Status != "active" {
		t.Errorf("expected status 'active', got %q", vault.Status)
	}
	if vault.DocumentsCount != 2 {
		t.Errorf("expected documents_count 2, got %d", vault.DocumentsCount)
	}
	if vault.UploadsCount != 3 {
		t.Errorf("expected uploads_count 3, got %d", vault.UploadsCount)
	}
	if vault.VaultsCount != 1 {
		t.Errorf("expected vaults_count 1, got %d", vault.VaultsCount)
	}
	if vault.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if vault.Parent.Type != "Project" {
		t.Errorf("expected Parent.Type 'Project', got %q", vault.Parent.Type)
	}
	if vault.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if vault.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", vault.Bucket.ID)
	}
	if vault.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if vault.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", vault.Creator.Name)
	}
	if vault.DocumentsURL == "" {
		t.Error("expected non-empty DocumentsURL")
	}
	if vault.UploadsURL == "" {
		t.Error("expected non-empty UploadsURL")
	}
	if vault.VaultsURL == "" {
		t.Error("expected non-empty VaultsURL")
	}
}

func TestVault_UnmarshalList(t *testing.T) {
	data := loadVaultsFixture(t, "list.json")

	var vaults []Vault
	if err := json.Unmarshal(data, &vaults); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(vaults) != 2 {
		t.Errorf("expected 2 vaults, got %d", len(vaults))
	}

	// Verify first vault
	v1 := vaults[0]
	if v1.ID != 1069479200 {
		t.Errorf("expected ID 1069479200, got %d", v1.ID)
	}
	if v1.Title != "Design Assets" {
		t.Errorf("expected title 'Design Assets', got %q", v1.Title)
	}
	if v1.Position != 1 {
		t.Errorf("expected position 1, got %d", v1.Position)
	}
	if v1.VisibleToClients {
		t.Error("expected visible_to_clients to be false")
	}

	// Verify second vault
	v2 := vaults[1]
	if v2.ID != 1069479201 {
		t.Errorf("expected ID 1069479201, got %d", v2.ID)
	}
	if v2.Title != "Client Documents" {
		t.Errorf("expected title 'Client Documents', got %q", v2.Title)
	}
	if !v2.VisibleToClients {
		t.Error("expected visible_to_clients to be true")
	}
	if v2.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if v2.Parent.Type != "Vault" {
		t.Errorf("expected Parent.Type 'Vault', got %q", v2.Parent.Type)
	}
}

func TestCreateVaultRequest_Marshal(t *testing.T) {
	data := loadVaultsFixture(t, "create-request.json")

	var req CreateVaultRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Title != "New Folder" {
		t.Errorf("expected title 'New Folder', got %q", req.Title)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateVaultRequest: %v", err)
	}

	var roundtrip CreateVaultRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateVaultRequest_Marshal(t *testing.T) {
	data := loadVaultsFixture(t, "update-request.json")

	var req UpdateVaultRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Title != "Renamed Folder" {
		t.Errorf("expected title 'Renamed Folder', got %q", req.Title)
	}
}

func TestVault_TimestampParsing(t *testing.T) {
	data := loadVaultsFixture(t, "get.json")

	var vault Vault
	if err := json.Unmarshal(data, &vault); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if vault.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if vault.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if vault.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", vault.CreatedAt.Year())
	}
}

// Document tests

func TestDocument_UnmarshalGet(t *testing.T) {
	data := loadDocumentsFixture(t, "get.json")

	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if document.ID != 1069479300 {
		t.Errorf("expected ID 1069479300, got %d", document.ID)
	}
	if document.Title != "Project Overview" {
		t.Errorf("expected title 'Project Overview', got %q", document.Title)
	}
	if document.Type != "Document" {
		t.Errorf("expected type 'Document', got %q", document.Type)
	}
	if document.Status != "active" {
		t.Errorf("expected status 'active', got %q", document.Status)
	}
	if document.CommentsCount != 2 {
		t.Errorf("expected comments_count 2, got %d", document.CommentsCount)
	}
	if document.Content == "" {
		t.Error("expected non-empty Content")
	}
	if document.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if document.Parent.Type != "Vault" {
		t.Errorf("expected Parent.Type 'Vault', got %q", document.Parent.Type)
	}
	if document.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if document.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if document.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", document.Creator.Name)
	}
}

func TestDocument_UnmarshalList(t *testing.T) {
	data := loadDocumentsFixture(t, "list.json")

	var documents []Document
	if err := json.Unmarshal(data, &documents); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(documents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(documents))
	}

	// Verify first document
	d1 := documents[0]
	if d1.ID != 1069479300 {
		t.Errorf("expected ID 1069479300, got %d", d1.ID)
	}
	if d1.Title != "Project Overview" {
		t.Errorf("expected title 'Project Overview', got %q", d1.Title)
	}
	if d1.Position != 1 {
		t.Errorf("expected position 1, got %d", d1.Position)
	}

	// Verify second document
	d2 := documents[1]
	if d2.ID != 1069479301 {
		t.Errorf("expected ID 1069479301, got %d", d2.ID)
	}
	if d2.Title != "Meeting Notes" {
		t.Errorf("expected title 'Meeting Notes', got %q", d2.Title)
	}
	if !d2.VisibleToClients {
		t.Error("expected visible_to_clients to be true")
	}
}

func TestCreateDocumentRequest_Marshal(t *testing.T) {
	data := loadDocumentsFixture(t, "create-request.json")

	var req CreateDocumentRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Title != "New Document" {
		t.Errorf("expected title 'New Document', got %q", req.Title)
	}
	if req.Content == "" {
		t.Error("expected non-empty Content")
	}
	if req.Status != "active" {
		t.Errorf("expected status 'active', got %q", req.Status)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateDocumentRequest: %v", err)
	}

	var roundtrip CreateDocumentRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title || roundtrip.Content != req.Content {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateDocumentRequest_Marshal(t *testing.T) {
	data := loadDocumentsFixture(t, "update-request.json")

	var req UpdateDocumentRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Title != "Updated Document Title" {
		t.Errorf("expected title 'Updated Document Title', got %q", req.Title)
	}
	if req.Content == "" {
		t.Error("expected non-empty Content")
	}
}

func TestDocument_TimestampParsing(t *testing.T) {
	data := loadDocumentsFixture(t, "get.json")

	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if document.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if document.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if document.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", document.CreatedAt.Year())
	}
}

// Upload tests

func TestUpload_UnmarshalGet(t *testing.T) {
	data := loadUploadsFixture(t, "get.json")

	var upload Upload
	if err := json.Unmarshal(data, &upload); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if upload.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", upload.ID)
	}
	if upload.Title != "logo.png" {
		t.Errorf("expected title 'logo.png', got %q", upload.Title)
	}
	if upload.Type != "Upload" {
		t.Errorf("expected type 'Upload', got %q", upload.Type)
	}
	if upload.Status != "active" {
		t.Errorf("expected status 'active', got %q", upload.Status)
	}
	if upload.Filename != "logo.png" {
		t.Errorf("expected filename 'logo.png', got %q", upload.Filename)
	}
	if upload.ContentType != "image/png" {
		t.Errorf("expected content_type 'image/png', got %q", upload.ContentType)
	}
	if upload.ByteSize != 245678 {
		t.Errorf("expected byte_size 245678, got %d", upload.ByteSize)
	}
	if upload.Width != 1024 {
		t.Errorf("expected width 1024, got %d", upload.Width)
	}
	if upload.Height != 768 {
		t.Errorf("expected height 768, got %d", upload.Height)
	}
	if upload.Description != "Company logo in high resolution" {
		t.Errorf("expected description 'Company logo in high resolution', got %q", upload.Description)
	}
	if upload.DownloadURL == "" {
		t.Error("expected non-empty DownloadURL")
	}
	if upload.CommentsCount != 1 {
		t.Errorf("expected comments_count 1, got %d", upload.CommentsCount)
	}
	if upload.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if upload.Parent.Type != "Vault" {
		t.Errorf("expected Parent.Type 'Vault', got %q", upload.Parent.Type)
	}
	if upload.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if upload.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if upload.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", upload.Creator.Name)
	}
}

func TestUpload_UnmarshalList(t *testing.T) {
	data := loadUploadsFixture(t, "list.json")

	var uploads []Upload
	if err := json.Unmarshal(data, &uploads); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(uploads) != 2 {
		t.Errorf("expected 2 uploads, got %d", len(uploads))
	}

	// Verify first upload (image with dimensions)
	u1 := uploads[0]
	if u1.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", u1.ID)
	}
	if u1.Filename != "logo.png" {
		t.Errorf("expected filename 'logo.png', got %q", u1.Filename)
	}
	if u1.ContentType != "image/png" {
		t.Errorf("expected content_type 'image/png', got %q", u1.ContentType)
	}
	if u1.Width != 1024 {
		t.Errorf("expected width 1024, got %d", u1.Width)
	}

	// Verify second upload (PDF without dimensions)
	u2 := uploads[1]
	if u2.ID != 1069479401 {
		t.Errorf("expected ID 1069479401, got %d", u2.ID)
	}
	if u2.Filename != "proposal.pdf" {
		t.Errorf("expected filename 'proposal.pdf', got %q", u2.Filename)
	}
	if u2.ContentType != "application/pdf" {
		t.Errorf("expected content_type 'application/pdf', got %q", u2.ContentType)
	}
	if u2.ByteSize != 1048576 {
		t.Errorf("expected byte_size 1048576, got %d", u2.ByteSize)
	}
	if u2.Width != 0 {
		t.Errorf("expected width 0 for PDF, got %d", u2.Width)
	}
	if !u2.VisibleToClients {
		t.Error("expected visible_to_clients to be true")
	}
}

func TestUpdateUploadRequest_Marshal(t *testing.T) {
	data := loadUploadsFixture(t, "update-request.json")

	var req UpdateUploadRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Description != "Updated description for the file" {
		t.Errorf("expected description 'Updated description for the file', got %q", req.Description)
	}
	if req.BaseName != "new_filename" {
		t.Errorf("expected base_name 'new_filename', got %q", req.BaseName)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateUploadRequest: %v", err)
	}

	var roundtrip UpdateUploadRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Description != req.Description || roundtrip.BaseName != req.BaseName {
		t.Error("round-trip mismatch")
	}
}

func TestUpload_TimestampParsing(t *testing.T) {
	data := loadUploadsFixture(t, "get.json")

	var upload Upload
	if err := json.Unmarshal(data, &upload); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if upload.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if upload.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if upload.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", upload.CreatedAt.Year())
	}
}

// UploadsService.Download tests

func TestUploadsService_Download_MissingDownloadURL(t *testing.T) {
	// Test that Download returns an error when the upload has no download URL
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return an upload without a download_url
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       1069479400,
			"title":    "logo.png",
			"filename": "logo.png",
			// Deliberately omit download_url
		})
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)

	ac := client.ForAccount("12345")
	_, err := ac.Uploads().Download(context.Background(), 999, 1069479400)

	if err == nil {
		t.Fatal("expected error for missing download URL")
	}
	if !strings.Contains(err.Error(), "no download URL") {
		t.Errorf("expected 'no download URL' error, got: %v", err)
	}
}

func TestUploadsService_Download_S3Error(t *testing.T) {
	// Test that Download handles non-200 responses from S3
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           1069479400,
			"title":        "logo.png",
			"filename":     "logo.png",
			"download_url": s3Server.URL + "/bucket/file.png",
		})
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithTransport(apiServer.Client().Transport))

	ac := client.ForAccount("12345")
	_, err := ac.Uploads().Download(context.Background(), 999, 1069479400)

	if err == nil {
		t.Fatal("expected error for S3 403 response")
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Errorf("expected 'status 403' error, got: %v", err)
	}
}

func TestUploadsService_Download_Success(t *testing.T) {
	// Test successful download with proper header extraction
	fileContent := "test file content"

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "17")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer s3Server.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           1069479400,
			"title":        "logo.png",
			"filename":     "logo.png",
			"download_url": s3Server.URL + "/bucket/file.png",
		})
	}))
	defer apiServer.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = apiServer.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithTransport(apiServer.Client().Transport))

	ac := client.ForAccount("12345")
	result, err := ac.Uploads().Download(context.Background(), 999, 1069479400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	if result.ContentType != "image/png" {
		t.Errorf("expected Content-Type 'image/png', got %q", result.ContentType)
	}
	if result.Filename != "logo.png" {
		t.Errorf("expected Filename 'logo.png', got %q", result.Filename)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(body) != fileContent {
		t.Errorf("expected body %q, got %q", fileContent, string(body))
	}
}
