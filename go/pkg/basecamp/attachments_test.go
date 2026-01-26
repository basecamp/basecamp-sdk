package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
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
