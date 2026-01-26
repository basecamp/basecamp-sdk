package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func clientApprovalsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "client_approvals")
}

func loadClientApprovalsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(clientApprovalsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestClientApproval_UnmarshalList(t *testing.T) {
	data := loadClientApprovalsFixture(t, "list.json")

	var approvals []ClientApproval
	if err := json.Unmarshal(data, &approvals); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(approvals) != 1 {
		t.Errorf("expected 1 approval, got %d", len(approvals))
	}

	// Verify first approval
	a := approvals[0]
	if a.ID != 1069479654 {
		t.Errorf("expected ID 1069479654, got %d", a.ID)
	}
	if a.Status != "active" {
		t.Errorf("expected status 'active', got %q", a.Status)
	}
	if a.Type != "Client::Approval" {
		t.Errorf("expected type 'Client::Approval', got %q", a.Type)
	}
	if a.Title != "Business card" {
		t.Errorf("expected title 'Business card', got %q", a.Title)
	}
	if a.Subject != "Business card" {
		t.Errorf("expected subject 'Business card', got %q", a.Subject)
	}
	if a.ApprovalStatus != "pending" {
		t.Errorf("expected approval_status 'pending', got %q", a.ApprovalStatus)
	}
	if a.URL != "https://3.basecampapi.com/195539477/buckets/2085958500/client/approvals/1069479654.json" {
		t.Errorf("unexpected URL: %q", a.URL)
	}
	if a.AppURL != "https://3.basecamp.com/195539477/buckets/2085958500/client/approvals/1069479654" {
		t.Errorf("unexpected AppURL: %q", a.AppURL)
	}

	// Verify parent
	if a.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if a.Parent.ID != 1069479564 {
		t.Errorf("expected Parent.ID 1069479564, got %d", a.Parent.ID)
	}
	if a.Parent.Title != "The Clientside" {
		t.Errorf("expected Parent.Title 'The Clientside', got %q", a.Parent.Title)
	}
	if a.Parent.Type != "Client::Board" {
		t.Errorf("expected Parent.Type 'Client::Board', got %q", a.Parent.Type)
	}

	// Verify bucket
	if a.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if a.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", a.Bucket.ID)
	}
	if a.Bucket.Name != "The Leto Locator" {
		t.Errorf("expected Bucket.Name 'The Leto Locator', got %q", a.Bucket.Name)
	}

	// Verify creator
	if a.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if a.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", a.Creator.ID)
	}
	if a.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", a.Creator.Name)
	}

	// Verify approver
	if a.Approver == nil {
		t.Fatal("expected Approver to be non-nil")
	}
	if a.Approver.ID != 1049715942 {
		t.Errorf("expected Approver.ID 1049715942, got %d", a.Approver.ID)
	}
	if a.Approver.Name != "Miranda Grant" {
		t.Errorf("expected Approver.Name 'Miranda Grant', got %q", a.Approver.Name)
	}
	if a.Approver.PersonableType != "Client" {
		t.Errorf("expected Approver.PersonableType 'Client', got %q", a.Approver.PersonableType)
	}
}

func TestClientApproval_UnmarshalGet(t *testing.T) {
	data := loadClientApprovalsFixture(t, "get.json")

	var approval ClientApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if approval.ID != 1069479651 {
		t.Errorf("expected ID 1069479651, got %d", approval.ID)
	}
	if approval.Status != "active" {
		t.Errorf("expected status 'active', got %q", approval.Status)
	}
	if approval.Type != "Client::Approval" {
		t.Errorf("expected type 'Client::Approval', got %q", approval.Type)
	}
	if approval.Title != "New logo for the website" {
		t.Errorf("expected title 'New logo for the website', got %q", approval.Title)
	}
	if approval.ApprovalStatus != "approved" {
		t.Errorf("expected approval_status 'approved', got %q", approval.ApprovalStatus)
	}

	// Verify timestamps are parsed
	if approval.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if approval.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify approver
	if approval.Approver == nil {
		t.Fatal("expected Approver to be non-nil")
	}
	if approval.Approver.Name != "Stephen Early" {
		t.Errorf("expected Approver.Name 'Stephen Early', got %q", approval.Approver.Name)
	}

	// Verify responses
	if len(approval.Responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(approval.Responses))
	}

	resp := approval.Responses[0]
	if resp.ID != 1069479653 {
		t.Errorf("expected Response.ID 1069479653, got %d", resp.ID)
	}
	if resp.Type != "Client::Approval::Response" {
		t.Errorf("expected Response.Type 'Client::Approval::Response', got %q", resp.Type)
	}
	if !resp.Approved {
		t.Error("expected Response.Approved to be true")
	}
	if resp.Creator == nil {
		t.Fatal("expected Response.Creator to be non-nil")
	}
	if resp.Creator.Name != "Beth Allen" {
		t.Errorf("expected Response.Creator.Name 'Beth Allen', got %q", resp.Creator.Name)
	}
}

func TestClientApprovalResponse_Unmarshal(t *testing.T) {
	// Test that ClientApprovalResponse can be unmarshalled standalone
	jsonData := `{
		"id": 123,
		"status": "active",
		"type": "Client::Approval::Response",
		"content": "<div>Approved!</div>",
		"approved": true
	}`

	var resp ClientApprovalResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != 123 {
		t.Errorf("expected ID 123, got %d", resp.ID)
	}
	if !resp.Approved {
		t.Error("expected Approved to be true")
	}
}
