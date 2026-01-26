package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func clientRepliesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "client_replies")
}

func loadClientRepliesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(clientRepliesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestClientReply_UnmarshalList(t *testing.T) {
	data := loadClientRepliesFixture(t, "list.json")

	var replies []ClientReply
	if err := json.Unmarshal(data, &replies); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(replies) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies))
	}

	// Verify first reply
	r := replies[0]
	if r.ID != 1069479567 {
		t.Errorf("expected ID 1069479567, got %d", r.ID)
	}
	if r.Status != "active" {
		t.Errorf("expected status 'active', got %q", r.Status)
	}
	if r.Type != "Client::Reply" {
		t.Errorf("expected type 'Client::Reply', got %q", r.Type)
	}
	if r.Title != "Re: Project kickoff!" {
		t.Errorf("expected title 'Re: Project kickoff!', got %q", r.Title)
	}
	if !r.InheritsStatus {
		t.Error("expected inherits_status to be true")
	}
	if r.VisibleToClients {
		t.Error("expected visible_to_clients to be false")
	}
	if r.Content != "<div>Hi all - we're excited to get started too.</div>" {
		t.Errorf("unexpected content: %q", r.Content)
	}
	if r.URL != "https://3.basecampapi.com/195539477/buckets/2085958500/client/replies/1069479567.json" {
		t.Errorf("unexpected URL: %q", r.URL)
	}
	if r.AppURL != "https://3.basecamp.com/195539477/buckets/2085958500/client/correspondences/1069479566#__recording_1069479567" {
		t.Errorf("unexpected AppURL: %q", r.AppURL)
	}

	// Verify parent
	if r.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if r.Parent.ID != 1069479566 {
		t.Errorf("expected Parent.ID 1069479566, got %d", r.Parent.ID)
	}
	if r.Parent.Title != "Project kickoff!" {
		t.Errorf("expected Parent.Title 'Project kickoff!', got %q", r.Parent.Title)
	}
	if r.Parent.Type != "Client::Correspondence" {
		t.Errorf("expected Parent.Type 'Client::Correspondence', got %q", r.Parent.Type)
	}

	// Verify bucket
	if r.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if r.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", r.Bucket.ID)
	}
	if r.Bucket.Name != "The Leto Locator" {
		t.Errorf("expected Bucket.Name 'The Leto Locator', got %q", r.Bucket.Name)
	}

	// Verify creator
	if r.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if r.Creator.ID != 1049715941 {
		t.Errorf("expected Creator.ID 1049715941, got %d", r.Creator.ID)
	}
	if r.Creator.Name != "Stephen Early" {
		t.Errorf("expected Creator.Name 'Stephen Early', got %q", r.Creator.Name)
	}
	if r.Creator.EmailAddress != "stephen@letobrand.com" {
		t.Errorf("expected Creator.EmailAddress 'stephen@letobrand.com', got %q", r.Creator.EmailAddress)
	}
	if r.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if r.Creator.Company.Name != "Leto Brand" {
		t.Errorf("expected Creator.Company.Name 'Leto Brand', got %q", r.Creator.Company.Name)
	}
}

func TestClientReply_UnmarshalGet(t *testing.T) {
	data := loadClientRepliesFixture(t, "get.json")

	var reply ClientReply
	if err := json.Unmarshal(data, &reply); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if reply.ID != 1069479571 {
		t.Errorf("expected ID 1069479571, got %d", reply.ID)
	}
	if reply.Status != "active" {
		t.Errorf("expected status 'active', got %q", reply.Status)
	}
	if reply.Type != "Client::Reply" {
		t.Errorf("expected type 'Client::Reply', got %q", reply.Type)
	}
	if reply.Title != "Re: Project kickoff!" {
		t.Errorf("expected title 'Re: Project kickoff!', got %q", reply.Title)
	}

	// Verify timestamps are parsed
	if reply.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if reply.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator
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
	if reply.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if reply.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", reply.Creator.Company.Name)
	}

	// Verify content
	expectedContent := "<div>Hi Leto team, this it's Annie. I'll be your day to day contact for the project, so keep me on your speed dial (or speed email, perhaps more accurately!) Feel free to reach out to me with any questions at all, and I'll be posting up some outlines, timelines, etc. very shortly.</div>"
	if reply.Content != expectedContent {
		t.Errorf("unexpected content: %q", reply.Content)
	}

	// Verify parent
	if reply.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if reply.Parent.ID != 1069479566 {
		t.Errorf("expected Parent.ID 1069479566, got %d", reply.Parent.ID)
	}
	if reply.Parent.Type != "Client::Correspondence" {
		t.Errorf("expected Parent.Type 'Client::Correspondence', got %q", reply.Parent.Type)
	}
}
