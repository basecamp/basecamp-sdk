package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func clientCorrespondencesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "client_correspondences")
}

func loadClientCorrespondencesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(clientCorrespondencesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestClientCorrespondence_UnmarshalList(t *testing.T) {
	data := loadClientCorrespondencesFixture(t, "list.json")

	var correspondences []ClientCorrespondence
	if err := json.Unmarshal(data, &correspondences); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(correspondences) != 1 {
		t.Errorf("expected 1 correspondence, got %d", len(correspondences))
	}

	// Verify first correspondence
	c := correspondences[0]
	if c.ID != 1069479645 {
		t.Errorf("expected ID 1069479645, got %d", c.ID)
	}
	if c.Status != "active" {
		t.Errorf("expected status 'active', got %q", c.Status)
	}
	if c.Type != "Client::Correspondence" {
		t.Errorf("expected type 'Client::Correspondence', got %q", c.Type)
	}
	if c.Title != "Final deliverables and launch are right around the corner" {
		t.Errorf("expected title 'Final deliverables and launch are right around the corner', got %q", c.Title)
	}
	if c.Subject != "Final deliverables and launch are right around the corner" {
		t.Errorf("expected subject 'Final deliverables and launch are right around the corner', got %q", c.Subject)
	}
	if c.RepliesCount != 5 {
		t.Errorf("expected replies_count 5, got %d", c.RepliesCount)
	}
	if c.URL != "https://3.basecampapi.com/195539477/buckets/2085958500/client/correspondences/1069479645.json" {
		t.Errorf("unexpected URL: %q", c.URL)
	}
	if c.AppURL != "https://3.basecamp.com/195539477/buckets/2085958500/client/correspondences/1069479645" {
		t.Errorf("unexpected AppURL: %q", c.AppURL)
	}

	// Verify parent
	if c.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if c.Parent.ID != 1069479564 {
		t.Errorf("expected Parent.ID 1069479564, got %d", c.Parent.ID)
	}
	if c.Parent.Title != "The Clientside" {
		t.Errorf("expected Parent.Title 'The Clientside', got %q", c.Parent.Title)
	}
	if c.Parent.Type != "Client::Board" {
		t.Errorf("expected Parent.Type 'Client::Board', got %q", c.Parent.Type)
	}

	// Verify bucket
	if c.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if c.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", c.Bucket.ID)
	}
	if c.Bucket.Name != "The Leto Locator" {
		t.Errorf("expected Bucket.Name 'The Leto Locator', got %q", c.Bucket.Name)
	}

	// Verify creator
	if c.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if c.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", c.Creator.ID)
	}
	if c.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", c.Creator.Name)
	}
	if c.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if c.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", c.Creator.Company.Name)
	}
}

func TestClientCorrespondence_UnmarshalGet(t *testing.T) {
	data := loadClientCorrespondencesFixture(t, "get.json")

	var correspondence ClientCorrespondence
	if err := json.Unmarshal(data, &correspondence); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if correspondence.ID != 1069479566 {
		t.Errorf("expected ID 1069479566, got %d", correspondence.ID)
	}
	if correspondence.Status != "active" {
		t.Errorf("expected status 'active', got %q", correspondence.Status)
	}
	if correspondence.Type != "Client::Correspondence" {
		t.Errorf("expected type 'Client::Correspondence', got %q", correspondence.Type)
	}
	if correspondence.Title != "Project kickoff!" {
		t.Errorf("expected title 'Project kickoff!', got %q", correspondence.Title)
	}
	if correspondence.Subject != "Project kickoff!" {
		t.Errorf("expected subject 'Project kickoff!', got %q", correspondence.Subject)
	}
	if correspondence.RepliesCount != 5 {
		t.Errorf("expected replies_count 5, got %d", correspondence.RepliesCount)
	}

	// Verify timestamps are parsed
	if correspondence.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if correspondence.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator
	if correspondence.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if correspondence.Creator.ID != 1049715929 {
		t.Errorf("expected Creator.ID 1049715929, got %d", correspondence.Creator.ID)
	}
	if correspondence.Creator.Name != "Jay Edmonds" {
		t.Errorf("expected Creator.Name 'Jay Edmonds', got %q", correspondence.Creator.Name)
	}
	if correspondence.Creator.EmailAddress != "jay@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'jay@honchodesign.com', got %q", correspondence.Creator.EmailAddress)
	}
}
