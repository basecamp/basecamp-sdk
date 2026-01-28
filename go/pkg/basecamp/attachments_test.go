package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func attachmentsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "attachments")
}

func loadAttachmentsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(attachmentsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestAttachmentResponse_Unmarshal(t *testing.T) {
	data := loadAttachmentsFixture(t, "create.json")

	var resp AttachmentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal create.json: %v", err)
	}

	expectedSGID := "BAh2CEkiCGdpZAY6BkVUSSIsZ2lkOi8vYmMzL0F0dGFjaG1lbnQvNzM4NDcyNj9leHBpcmVzX2luBjsAVEkiDHB1cnBvc2UGOwBUSSIPYXR0YWNoYWJsZQY7AFRJIg9leHBpcmVzX2F0BjsAVDA=--13982201abe18044c897e32979c7dccfe8add9c1"
	if resp.AttachableSGID != expectedSGID {
		t.Errorf("expected attachable_sgid %q, got %q", expectedSGID, resp.AttachableSGID)
	}
}

func TestAttachmentResponse_Marshal(t *testing.T) {
	resp := AttachmentResponse{
		AttachableSGID: "test-sgid-12345",
	}

	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal AttachmentResponse: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["attachable_sgid"] != "test-sgid-12345" {
		t.Errorf("unexpected attachable_sgid: %v", data["attachable_sgid"])
	}
}

func TestAttachmentsService_Create_ContentType(t *testing.T) {
	// Test that the Content-Type header is set correctly when creating attachments.
	// This verifies that the passed contentType parameter is used as the request's
	// Content-Type header, not overwritten by any other value.
	tests := []struct {
		name        string
		contentType string
	}{
		{"image/png", "image/png"},
		{"image/jpeg", "image/jpeg"},
		{"application/pdf", "application/pdf"},
		{"text/plain", "text/plain"},
		{"application/octet-stream", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedContentType string

			// Create a test server that captures the Content-Type header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedContentType = r.Header.Get("Content-Type")

				// Return a successful attachment response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"attachable_sgid": "test-sgid-123",
				})
			}))
			defer server.Close()

			// Create a client pointing to the test server
			cfg := DefaultConfig()
			cfg.BaseURL = server.URL
			token := &StaticTokenProvider{Token: "test-token"}
			client := NewClient(cfg, token)

			// Create an attachment with the test content type
			_, err := client.ForAccount("12345").Attachments().Create(
				context.Background(),
				"test.file",
				tt.contentType,
				strings.NewReader("test file content"),
			)
			if err != nil {
				t.Fatalf("Create failed: %v", err)
			}

			// Verify the Content-Type header matches what we passed
			if capturedContentType != tt.contentType {
				t.Errorf("Content-Type header mismatch: expected %q, got %q", tt.contentType, capturedContentType)
			}
		})
	}
}
