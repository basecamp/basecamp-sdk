package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func commentsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "comments")
}

func loadCommentsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(commentsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestComment_UnmarshalList(t *testing.T) {
	data := loadCommentsFixture(t, "list.json")

	var comments []Comment
	if err := json.Unmarshal(data, &comments); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(comments))
	}

	// Verify first comment
	c1 := comments[0]
	if c1.ID != 1069479352 {
		t.Errorf("expected ID 1069479352, got %d", c1.ID)
	}
	if c1.Status != "active" {
		t.Errorf("expected status 'active', got %q", c1.Status)
	}
	if c1.Type != "Comment" {
		t.Errorf("expected type 'Comment', got %q", c1.Type)
	}
	if c1.Content != "<div>Yeah! Great job everyone! Super excited to get going!</div>" {
		t.Errorf("unexpected content: %q", c1.Content)
	}
	if c1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/comments/1069479352.json" {
		t.Errorf("unexpected URL: %q", c1.URL)
	}
	if c1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/messages/1069479351#__recording_1069479352" {
		t.Errorf("unexpected AppURL: %q", c1.AppURL)
	}

	// Verify parent
	if c1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if c1.Parent.ID != 1069479351 {
		t.Errorf("expected Parent.ID 1069479351, got %d", c1.Parent.ID)
	}
	if c1.Parent.Title != "We won Leto!" {
		t.Errorf("expected Parent.Title 'We won Leto!', got %q", c1.Parent.Title)
	}
	if c1.Parent.Type != "Message" {
		t.Errorf("expected Parent.Type 'Message', got %q", c1.Parent.Type)
	}

	// Verify bucket
	if c1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if c1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", c1.Bucket.ID)
	}
	if c1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", c1.Bucket.Name)
	}
	if c1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", c1.Bucket.Type)
	}

	// Verify creator
	if c1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if c1.Creator.ID != 1049715923 {
		t.Errorf("expected Creator.ID 1049715923, got %d", c1.Creator.ID)
	}
	if c1.Creator.Name != "Andrew Wong" {
		t.Errorf("expected Creator.Name 'Andrew Wong', got %q", c1.Creator.Name)
	}

	// Verify second comment
	c2 := comments[1]
	if c2.ID != 1069479361 {
		t.Errorf("expected ID 1069479361, got %d", c2.ID)
	}
	if c2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second comment")
	}
	if c2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", c2.Creator.Name)
	}
	// Verify creator with company
	if c2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second comment")
	}
	if c2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", c2.Creator.Company.Name)
	}
}

func TestComment_UnmarshalGet(t *testing.T) {
	data := loadCommentsFixture(t, "get.json")

	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if comment.ID != 1069479361 {
		t.Errorf("expected ID 1069479361, got %d", comment.ID)
	}
	if comment.Status != "active" {
		t.Errorf("expected status 'active', got %q", comment.Status)
	}
	if comment.Type != "Comment" {
		t.Errorf("expected type 'Comment', got %q", comment.Type)
	}
	expectedContent := "<div>I just want to echo what just about everyone already said. This is a big one for us, and I can't wait to get going. I'll be spinning up the project shortly!</div>"
	if comment.Content != expectedContent {
		t.Errorf("unexpected content: %q", comment.Content)
	}

	// Verify timestamps are parsed
	if comment.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if comment.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if comment.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if comment.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", comment.Creator.ID)
	}
	if comment.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", comment.Creator.Name)
	}
	if comment.Creator.EmailAddress != "annie@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'annie@honchodesign.com', got %q", comment.Creator.EmailAddress)
	}
	if comment.Creator.Title != "Central Markets Manager" {
		t.Errorf("expected Creator.Title 'Central Markets Manager', got %q", comment.Creator.Title)
	}
	if !comment.Creator.Employee {
		t.Error("expected Creator.Employee to be true")
	}

	// ContentAttachments: the two-entry representative decodes directly into
	// the public Comment, so RichTextAttachment.UnmarshalJSON runs per element.
	// The image carries a float-spelled dimension (1024.0 -> 1024) and a plain
	// int height; the non-image blob carries null dimensions (-> nil).
	if len(comment.ContentAttachments) != 2 {
		t.Fatalf("expected 2 content attachments, got %d", len(comment.ContentAttachments))
	}
	img := comment.ContentAttachments[0]
	if img.ID != 1069480010 {
		t.Errorf("expected image attachment ID 1069480010, got %d", img.ID)
	}
	if img.Filename != "celebration.png" || img.ContentType != "image/png" {
		t.Errorf("unexpected image attachment: %+v", img)
	}
	if img.Width == nil || *img.Width != 1024 {
		t.Errorf("expected image Width 1024 (float-spelled 1024.0), got %v", img.Width)
	}
	if img.Height == nil || *img.Height != 768 {
		t.Errorf("expected image Height 768, got %v", img.Height)
	}
	if !img.Previewable {
		t.Error("expected image attachment to be previewable")
	}
	blob := comment.ContentAttachments[1]
	if blob.ID != 1069480011 || blob.ContentType != "application/pdf" {
		t.Errorf("unexpected blob attachment: %+v", blob)
	}
	if blob.Width != nil || blob.Height != nil {
		t.Errorf("expected nil dimensions for non-image blob, got width=%v height=%v", blob.Width, blob.Height)
	}
	if blob.Previewable {
		t.Error("expected non-image blob to not be previewable")
	}
}

func TestCreateCommentRequest_Marshal(t *testing.T) {
	req := CreateCommentRequest{
		Content: "<div><em>Wow!</em> That is cool.</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateCommentRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div><em>Wow!</em> That is cool.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}

	// Round-trip test
	var roundtrip CreateCommentRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
}

func TestUpdateCommentRequest_Marshal(t *testing.T) {
	req := UpdateCommentRequest{
		Content: "<div><em>No way!</em> That isn't cool at all.</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateCommentRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div><em>No way!</em> That isn't cool at all.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}

	// Round-trip test
	var roundtrip UpdateCommentRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
}

func TestComment_UnmarshalCreate(t *testing.T) {
	data := loadCommentsFixture(t, "create.json")

	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		t.Fatalf("failed to unmarshal create.json: %v", err)
	}

	if comment.ID != 1069479370 {
		t.Errorf("expected ID 1069479370, got %d", comment.ID)
	}
	if comment.Status != "active" {
		t.Errorf("expected status 'active', got %q", comment.Status)
	}
	if comment.Content != "<div><em>Wow!</em> That is cool.</div>" {
		t.Errorf("unexpected content: %q", comment.Content)
	}

	// Verify parent is set (comment is attached to a recording)
	if comment.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if comment.Parent.ID != 1069479351 {
		t.Errorf("expected Parent.ID 1069479351, got %d", comment.Parent.ID)
	}
}
