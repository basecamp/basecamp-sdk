package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func campfiresFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "campfires")
}

func loadCampfiresFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(campfiresFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestCampfire_UnmarshalList(t *testing.T) {
	data := loadCampfiresFixture(t, "list.json")

	var campfires []Campfire
	if err := json.Unmarshal(data, &campfires); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(campfires) != 2 {
		t.Errorf("expected 2 campfires, got %d", len(campfires))
	}

	// Verify first campfire
	c1 := campfires[0]
	if c1.ID != 1069479345 {
		t.Errorf("expected ID 1069479345, got %d", c1.ID)
	}
	if c1.Status != "active" {
		t.Errorf("expected status 'active', got %q", c1.Status)
	}
	if c1.Type != "Chat::Transcript" {
		t.Errorf("expected type 'Chat::Transcript', got %q", c1.Type)
	}
	if c1.Title != "Campfire" {
		t.Errorf("expected title 'Campfire', got %q", c1.Title)
	}
	if c1.VisibleToClients != false {
		t.Errorf("expected VisibleToClients false, got true")
	}
	if c1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/chats/1069479345.json" {
		t.Errorf("unexpected URL: %q", c1.URL)
	}
	if c1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/chats/1069479345" {
		t.Errorf("unexpected AppURL: %q", c1.AppURL)
	}
	if c1.LinesURL != "https://3.basecampapi.com/195539477/buckets/2085958499/chats/1069479345/lines.json" {
		t.Errorf("unexpected LinesURL: %q", c1.LinesURL)
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
	if c1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", c1.Creator.ID)
	}
	if c1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", c1.Creator.Name)
	}

	// Verify second campfire
	c2 := campfires[1]
	if c2.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", c2.ID)
	}
	if c2.VisibleToClients != true {
		t.Errorf("expected VisibleToClients true, got false")
	}
	if c2.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil for second campfire")
	}
	if c2.Bucket.Name != "Marketing Campaign" {
		t.Errorf("expected Bucket.Name 'Marketing Campaign', got %q", c2.Bucket.Name)
	}
	if c2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second campfire")
	}
	if c2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", c2.Creator.Name)
	}
}

func TestCampfire_UnmarshalGet(t *testing.T) {
	data := loadCampfiresFixture(t, "get.json")

	var campfire Campfire
	if err := json.Unmarshal(data, &campfire); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if campfire.ID != 1069479345 {
		t.Errorf("expected ID 1069479345, got %d", campfire.ID)
	}
	if campfire.Status != "active" {
		t.Errorf("expected status 'active', got %q", campfire.Status)
	}
	if campfire.Type != "Chat::Transcript" {
		t.Errorf("expected type 'Chat::Transcript', got %q", campfire.Type)
	}
	if campfire.Title != "Campfire" {
		t.Errorf("expected title 'Campfire', got %q", campfire.Title)
	}

	// Verify timestamps are parsed
	if campfire.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if campfire.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if campfire.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if campfire.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", campfire.Creator.ID)
	}
	if campfire.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", campfire.Creator.Name)
	}
	if campfire.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", campfire.Creator.EmailAddress)
	}
	if campfire.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", campfire.Creator.Title)
	}
	if !campfire.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !campfire.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
	// Verify creator with company
	if campfire.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if campfire.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", campfire.Creator.Company.Name)
	}
}

func TestCampfireLine_UnmarshalList(t *testing.T) {
	data := loadCampfiresFixture(t, "lines_list.json")

	var lines []CampfireLine
	if err := json.Unmarshal(data, &lines); err != nil {
		t.Fatalf("failed to unmarshal lines_list.json: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Verify first line
	l1 := lines[0]
	if l1.ID != 1069479350 {
		t.Errorf("expected ID 1069479350, got %d", l1.ID)
	}
	if l1.Status != "active" {
		t.Errorf("expected status 'active', got %q", l1.Status)
	}
	if l1.Type != "Chat::Lines::Text" {
		t.Errorf("expected type 'Chat::Lines::Text', got %q", l1.Type)
	}
	if l1.Content != "Hello everyone!" {
		t.Errorf("expected content 'Hello everyone!', got %q", l1.Content)
	}
	if l1.Title != "Hello everyone!" {
		t.Errorf("expected title 'Hello everyone!', got %q", l1.Title)
	}
	if l1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/chats/1069479345/lines/1069479350.json" {
		t.Errorf("unexpected URL: %q", l1.URL)
	}

	// Verify parent (campfire)
	if l1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if l1.Parent.ID != 1069479345 {
		t.Errorf("expected Parent.ID 1069479345, got %d", l1.Parent.ID)
	}
	if l1.Parent.Title != "Campfire" {
		t.Errorf("expected Parent.Title 'Campfire', got %q", l1.Parent.Title)
	}
	if l1.Parent.Type != "Chat::Transcript" {
		t.Errorf("expected Parent.Type 'Chat::Transcript', got %q", l1.Parent.Type)
	}

	// Verify bucket
	if l1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if l1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", l1.Bucket.ID)
	}
	if l1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", l1.Bucket.Name)
	}

	// Verify creator
	if l1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if l1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", l1.Creator.ID)
	}
	if l1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", l1.Creator.Name)
	}

	// Verify second line
	l2 := lines[1]
	if l2.ID != 1069479355 {
		t.Errorf("expected ID 1069479355, got %d", l2.ID)
	}
	if l2.Content != "Welcome to the project!" {
		t.Errorf("expected content 'Welcome to the project!', got %q", l2.Content)
	}
	if l2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second line")
	}
	if l2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", l2.Creator.Name)
	}
	// Verify creator with company
	if l2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second line")
	}
	if l2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", l2.Creator.Company.Name)
	}
}

func TestCampfireLine_UnmarshalGet(t *testing.T) {
	data := loadCampfiresFixture(t, "line_get.json")

	var line CampfireLine
	if err := json.Unmarshal(data, &line); err != nil {
		t.Fatalf("failed to unmarshal line_get.json: %v", err)
	}

	if line.ID != 1069479350 {
		t.Errorf("expected ID 1069479350, got %d", line.ID)
	}
	if line.Status != "active" {
		t.Errorf("expected status 'active', got %q", line.Status)
	}
	if line.Type != "Chat::Lines::Text" {
		t.Errorf("expected type 'Chat::Lines::Text', got %q", line.Type)
	}
	if line.Content != "Hello everyone!" {
		t.Errorf("expected content 'Hello everyone!', got %q", line.Content)
	}

	// Verify timestamps are parsed
	if line.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if line.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if line.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if line.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", line.Creator.ID)
	}
	if line.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", line.Creator.Name)
	}
	if line.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", line.Creator.EmailAddress)
	}
	if line.Creator.Bio != "Don't let your dreams be dreams" {
		t.Errorf("expected Creator.Bio 'Don't let your dreams be dreams', got %q", line.Creator.Bio)
	}
	if line.Creator.Location != "Chicago, IL" {
		t.Errorf("expected Creator.Location 'Chicago, IL', got %q", line.Creator.Location)
	}
}

func TestCreateCampfireLineRequest_Marshal(t *testing.T) {
	req := CreateCampfireLineRequest{
		Content: "Hello team!",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateCampfireLineRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "Hello team!" {
		t.Errorf("unexpected content: %v", data["content"])
	}

	// Round-trip test
	var roundtrip CreateCampfireLineRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
}

func TestChatbot_UnmarshalList(t *testing.T) {
	data := loadCampfiresFixture(t, "chatbots_list.json")

	var chatbots []Chatbot
	if err := json.Unmarshal(data, &chatbots); err != nil {
		t.Fatalf("failed to unmarshal chatbots_list.json: %v", err)
	}

	if len(chatbots) != 2 {
		t.Errorf("expected 2 chatbots, got %d", len(chatbots))
	}

	// Verify first chatbot (no command_url)
	c1 := chatbots[0]
	if c1.ID != 1049715958 {
		t.Errorf("expected ID 1049715958, got %d", c1.ID)
	}
	if c1.ServiceName != "Capistrano" {
		t.Errorf("expected ServiceName 'Capistrano', got %q", c1.ServiceName)
	}
	if c1.CommandURL != "" {
		t.Errorf("expected empty CommandURL, got %q", c1.CommandURL)
	}
	if c1.URL != "https://3.basecampapi.com/195539477/buckets/2085958497/chats/1069478933/integrations/1049715958.json" {
		t.Errorf("unexpected URL: %q", c1.URL)
	}
	if c1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958497/chats/1069478933/integrations/1049715958" {
		t.Errorf("unexpected AppURL: %q", c1.AppURL)
	}
	if c1.LinesURL != "https://3.basecampapi.com/195539477/integrations/B5JQYvHsNWCoDvYGZfH1xNR9/buckets/2085958497/chats/1069478933/lines" {
		t.Errorf("unexpected LinesURL: %q", c1.LinesURL)
	}

	// Verify timestamps are parsed
	if c1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if c1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify second chatbot (with command_url)
	c2 := chatbots[1]
	if c2.ID != 1049715959 {
		t.Errorf("expected ID 1049715959, got %d", c2.ID)
	}
	if c2.ServiceName != "deploy" {
		t.Errorf("expected ServiceName 'deploy', got %q", c2.ServiceName)
	}
	if c2.CommandURL != "https://example.com/deploy" {
		t.Errorf("expected CommandURL 'https://example.com/deploy', got %q", c2.CommandURL)
	}
}

func TestChatbot_UnmarshalGet(t *testing.T) {
	data := loadCampfiresFixture(t, "chatbot_get.json")

	var chatbot Chatbot
	if err := json.Unmarshal(data, &chatbot); err != nil {
		t.Fatalf("failed to unmarshal chatbot_get.json: %v", err)
	}

	if chatbot.ID != 1049715958 {
		t.Errorf("expected ID 1049715958, got %d", chatbot.ID)
	}
	if chatbot.ServiceName != "Capistrano" {
		t.Errorf("expected ServiceName 'Capistrano', got %q", chatbot.ServiceName)
	}
	if chatbot.CommandURL != "https://example.com/command" {
		t.Errorf("expected CommandURL 'https://example.com/command', got %q", chatbot.CommandURL)
	}
	if chatbot.URL != "https://3.basecampapi.com/195539477/buckets/2085958497/chats/1069478933/integrations/1049715958.json" {
		t.Errorf("unexpected URL: %q", chatbot.URL)
	}
	if chatbot.AppURL != "https://3.basecamp.com/195539477/buckets/2085958497/chats/1069478933/integrations/1049715958" {
		t.Errorf("unexpected AppURL: %q", chatbot.AppURL)
	}
	if chatbot.LinesURL != "https://3.basecampapi.com/195539477/integrations/B5JQYvHsNWCoDvYGZfH1xNR9/buckets/2085958497/chats/1069478933/lines" {
		t.Errorf("unexpected LinesURL: %q", chatbot.LinesURL)
	}

	// Verify timestamps are parsed
	if chatbot.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if chatbot.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestCreateChatbotRequest_Marshal(t *testing.T) {
	req := CreateChatbotRequest{
		ServiceName: "mybot",
		CommandURL:  "https://example.com/webhook",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateChatbotRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["service_name"] != "mybot" {
		t.Errorf("unexpected service_name: %v", data["service_name"])
	}
	if data["command_url"] != "https://example.com/webhook" {
		t.Errorf("unexpected command_url: %v", data["command_url"])
	}

	// Test without command_url
	reqNoURL := CreateChatbotRequest{
		ServiceName: "simplebot",
	}
	outNoURL, err := json.Marshal(reqNoURL)
	if err != nil {
		t.Fatalf("failed to marshal CreateChatbotRequest without command_url: %v", err)
	}

	var dataNoURL map[string]interface{}
	if err := json.Unmarshal(outNoURL, &dataNoURL); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if dataNoURL["service_name"] != "simplebot" {
		t.Errorf("unexpected service_name: %v", dataNoURL["service_name"])
	}
	if _, exists := dataNoURL["command_url"]; exists {
		t.Errorf("command_url should be omitted when empty, got: %v", dataNoURL["command_url"])
	}
}

func TestUpdateChatbotRequest_Marshal(t *testing.T) {
	req := UpdateChatbotRequest{
		ServiceName: "updatedbot",
		CommandURL:  "https://example.com/updated",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateChatbotRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["service_name"] != "updatedbot" {
		t.Errorf("unexpected service_name: %v", data["service_name"])
	}
	if data["command_url"] != "https://example.com/updated" {
		t.Errorf("unexpected command_url: %v", data["command_url"])
	}
}
