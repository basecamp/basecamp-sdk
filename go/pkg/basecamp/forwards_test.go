package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func forwardsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "forwards")
}

func loadForwardsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(forwardsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestInbox_Unmarshal(t *testing.T) {
	data := loadForwardsFixture(t, "inbox.json")

	var inbox Inbox
	if err := json.Unmarshal(data, &inbox); err != nil {
		t.Fatalf("failed to unmarshal inbox.json: %v", err)
	}

	if inbox.ID != 1069479342 {
		t.Errorf("expected ID 1069479342, got %d", inbox.ID)
	}
	if inbox.Status != "active" {
		t.Errorf("expected status 'active', got %q", inbox.Status)
	}
	if inbox.Type != "Inbox" {
		t.Errorf("expected type 'Inbox', got %q", inbox.Type)
	}
	if inbox.Title != "Email Forwards" {
		t.Errorf("expected title 'Email Forwards', got %q", inbox.Title)
	}
	if inbox.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/inboxes/1069479342.json" {
		t.Errorf("unexpected URL: %q", inbox.URL)
	}
	if inbox.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/inboxes/1069479342" {
		t.Errorf("unexpected AppURL: %q", inbox.AppURL)
	}

	// Verify bucket
	if inbox.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if inbox.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", inbox.Bucket.ID)
	}
	if inbox.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", inbox.Bucket.Name)
	}

	// Verify creator
	if inbox.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if inbox.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", inbox.Creator.ID)
	}
	if inbox.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", inbox.Creator.Name)
	}

	// Verify timestamps are parsed
	if inbox.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if inbox.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestForward_UnmarshalList(t *testing.T) {
	data := loadForwardsFixture(t, "list.json")

	var forwards []Forward
	if err := json.Unmarshal(data, &forwards); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(forwards) != 2 {
		t.Errorf("expected 2 forwards, got %d", len(forwards))
	}

	// Verify first forward
	f1 := forwards[0]
	if f1.ID != 1069479380 {
		t.Errorf("expected ID 1069479380, got %d", f1.ID)
	}
	if f1.Status != "active" {
		t.Errorf("expected status 'active', got %q", f1.Status)
	}
	if f1.Type != "Inbox::Forward" {
		t.Errorf("expected type 'Inbox::Forward', got %q", f1.Type)
	}
	if f1.Subject != "Project proposal from client" {
		t.Errorf("expected subject 'Project proposal from client', got %q", f1.Subject)
	}
	if f1.From != "client@example.com" {
		t.Errorf("expected from 'client@example.com', got %q", f1.From)
	}
	if f1.Content != "<div>Hi team,<br><br>Please review the attached proposal for the new project.</div>" {
		t.Errorf("unexpected content: %q", f1.Content)
	}

	// Verify parent (inbox)
	if f1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if f1.Parent.ID != 1069479342 {
		t.Errorf("expected Parent.ID 1069479342, got %d", f1.Parent.ID)
	}
	if f1.Parent.Title != "Email Forwards" {
		t.Errorf("expected Parent.Title 'Email Forwards', got %q", f1.Parent.Title)
	}
	if f1.Parent.Type != "Inbox" {
		t.Errorf("expected Parent.Type 'Inbox', got %q", f1.Parent.Type)
	}

	// Verify bucket
	if f1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if f1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", f1.Bucket.ID)
	}

	// Verify creator
	if f1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if f1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", f1.Creator.ID)
	}
	if f1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", f1.Creator.Name)
	}

	// Verify second forward
	f2 := forwards[1]
	if f2.ID != 1069479390 {
		t.Errorf("expected ID 1069479390, got %d", f2.ID)
	}
	if f2.Subject != "Invoice #12345" {
		t.Errorf("expected subject 'Invoice #12345', got %q", f2.Subject)
	}
	if f2.From != "billing@vendor.com" {
		t.Errorf("expected from 'billing@vendor.com', got %q", f2.From)
	}
	if f2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second forward")
	}
	if f2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", f2.Creator.Name)
	}
	// Verify creator with company
	if f2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second forward")
	}
	if f2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", f2.Creator.Company.Name)
	}
}

func TestForward_UnmarshalGet(t *testing.T) {
	data := loadForwardsFixture(t, "get.json")

	var forward Forward
	if err := json.Unmarshal(data, &forward); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if forward.ID != 1069479380 {
		t.Errorf("expected ID 1069479380, got %d", forward.ID)
	}
	if forward.Status != "active" {
		t.Errorf("expected status 'active', got %q", forward.Status)
	}
	if forward.Type != "Inbox::Forward" {
		t.Errorf("expected type 'Inbox::Forward', got %q", forward.Type)
	}
	if forward.Subject != "Project proposal from client" {
		t.Errorf("expected subject 'Project proposal from client', got %q", forward.Subject)
	}
	if forward.From != "client@example.com" {
		t.Errorf("expected from 'client@example.com', got %q", forward.From)
	}

	// Verify timestamps are parsed
	if forward.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if forward.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if forward.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if forward.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", forward.Creator.ID)
	}
	if forward.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", forward.Creator.Name)
	}
	if forward.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", forward.Creator.EmailAddress)
	}
	if forward.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", forward.Creator.Title)
	}
	if !forward.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !forward.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
}

func TestForwardReply_UnmarshalList(t *testing.T) {
	data := loadForwardsFixture(t, "replies_list.json")

	var replies []ForwardReply
	if err := json.Unmarshal(data, &replies); err != nil {
		t.Fatalf("failed to unmarshal replies_list.json: %v", err)
	}

	if len(replies) != 2 {
		t.Errorf("expected 2 replies, got %d", len(replies))
	}

	// Verify first reply
	r1 := replies[0]
	if r1.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", r1.ID)
	}
	if r1.Status != "active" {
		t.Errorf("expected status 'active', got %q", r1.Status)
	}
	if r1.Type != "Inbox::Forward::Reply" {
		t.Errorf("expected type 'Inbox::Forward::Reply', got %q", r1.Type)
	}
	if r1.Content != "<div>Thanks for forwarding this. I'll review and get back to you by EOD.</div>" {
		t.Errorf("unexpected content: %q", r1.Content)
	}

	// Verify parent (forward)
	if r1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if r1.Parent.ID != 1069479380 {
		t.Errorf("expected Parent.ID 1069479380, got %d", r1.Parent.ID)
	}
	if r1.Parent.Title != "Project proposal from client" {
		t.Errorf("expected Parent.Title 'Project proposal from client', got %q", r1.Parent.Title)
	}
	if r1.Parent.Type != "Inbox::Forward" {
		t.Errorf("expected Parent.Type 'Inbox::Forward', got %q", r1.Parent.Type)
	}

	// Verify bucket
	if r1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if r1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", r1.Bucket.ID)
	}

	// Verify creator
	if r1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if r1.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", r1.Creator.ID)
	}
	if r1.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", r1.Creator.Name)
	}

	// Verify second reply
	r2 := replies[1]
	if r2.ID != 1069479410 {
		t.Errorf("expected ID 1069479410, got %d", r2.ID)
	}
	if r2.Content != "<div>Looks good! I've approved the proposal. Let's schedule a kickoff call.</div>" {
		t.Errorf("unexpected content: %q", r2.Content)
	}
	if r2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second reply")
	}
	if r2.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", r2.Creator.Name)
	}
}

func TestForwardReply_UnmarshalGet(t *testing.T) {
	data := loadForwardsFixture(t, "reply_get.json")

	var reply ForwardReply
	if err := json.Unmarshal(data, &reply); err != nil {
		t.Fatalf("failed to unmarshal reply_get.json: %v", err)
	}

	if reply.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", reply.ID)
	}
	if reply.Status != "active" {
		t.Errorf("expected status 'active', got %q", reply.Status)
	}
	if reply.Type != "Inbox::Forward::Reply" {
		t.Errorf("expected type 'Inbox::Forward::Reply', got %q", reply.Type)
	}
	if reply.Content != "<div>Thanks for forwarding this. I'll review and get back to you by EOD.</div>" {
		t.Errorf("unexpected content: %q", reply.Content)
	}

	// Verify timestamps are parsed
	if reply.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if reply.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if reply.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if reply.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", reply.Creator.ID)
	}
	if reply.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", reply.Creator.Name)
	}
	if reply.Creator.EmailAddress != "annie@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'annie@honchodesign.com', got %q", reply.Creator.EmailAddress)
	}
	if reply.Creator.Title != "Project Manager" {
		t.Errorf("expected Creator.Title 'Project Manager', got %q", reply.Creator.Title)
	}
	if reply.Creator.Owner {
		t.Error("expected Creator.Owner to be false")
	}
	if reply.Creator.Admin {
		t.Error("expected Creator.Admin to be false")
	}
	// Verify creator with company
	if reply.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if reply.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", reply.Creator.Company.Name)
	}
}

func TestCreateForwardReplyRequest_Marshal(t *testing.T) {
	req := CreateForwardReplyRequest{
		Content: "<div>This is my reply to the forwarded email.</div>",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateForwardReplyRequest: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if result["content"] != "<div>This is my reply to the forwarded email.</div>" {
		t.Errorf("unexpected content: %v", result["content"])
	}
}
