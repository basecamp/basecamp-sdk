package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func cardsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "cards")
}

func loadCardsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(cardsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestCardTable_Unmarshal(t *testing.T) {
	data := loadCardsFixture(t, "card_table.json")

	var cardTable CardTable
	if err := json.Unmarshal(data, &cardTable); err != nil {
		t.Fatalf("failed to unmarshal card_table.json: %v", err)
	}

	if cardTable.ID != 1069479345 {
		t.Errorf("expected ID 1069479345, got %d", cardTable.ID)
	}
	if cardTable.Status != "active" {
		t.Errorf("expected status 'active', got %q", cardTable.Status)
	}
	if cardTable.Title != "Development Board" {
		t.Errorf("expected title 'Development Board', got %q", cardTable.Title)
	}
	if cardTable.Type != "Kanban::Board" {
		t.Errorf("expected type 'Kanban::Board', got %q", cardTable.Type)
	}
	if cardTable.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/card_tables/1069479345.json" {
		t.Errorf("unexpected URL: %q", cardTable.URL)
	}
	if cardTable.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/card_tables/1069479345" {
		t.Errorf("unexpected AppURL: %q", cardTable.AppURL)
	}

	// Verify bucket
	if cardTable.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if cardTable.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", cardTable.Bucket.ID)
	}
	if cardTable.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", cardTable.Bucket.Name)
	}

	// Verify creator
	if cardTable.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if cardTable.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", cardTable.Creator.ID)
	}
	if cardTable.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", cardTable.Creator.Name)
	}

	// Verify lists (columns)
	if len(cardTable.Lists) != 3 {
		t.Fatalf("expected 3 lists, got %d", len(cardTable.Lists))
	}

	// Verify triage column
	triage := cardTable.Lists[0]
	if triage.ID != 1069479346 {
		t.Errorf("expected triage ID 1069479346, got %d", triage.ID)
	}
	if triage.Title != "Triage" {
		t.Errorf("expected triage title 'Triage', got %q", triage.Title)
	}
	if triage.Type != "Kanban::Triage" {
		t.Errorf("expected triage type 'Kanban::Triage', got %q", triage.Type)
	}
	if triage.CardsCount != 3 {
		t.Errorf("expected triage cards_count 3, got %d", triage.CardsCount)
	}

	// Verify in progress column
	inProgress := cardTable.Lists[1]
	if inProgress.Title != "In Progress" {
		t.Errorf("expected title 'In Progress', got %q", inProgress.Title)
	}
	if inProgress.Type != "Kanban::Column" {
		t.Errorf("expected type 'Kanban::Column', got %q", inProgress.Type)
	}
	if inProgress.Color != "blue" {
		t.Errorf("expected color 'blue', got %q", inProgress.Color)
	}
	if inProgress.Position != 1 {
		t.Errorf("expected position 1, got %d", inProgress.Position)
	}

	// Verify done column
	done := cardTable.Lists[2]
	if done.Title != "Done" {
		t.Errorf("expected title 'Done', got %q", done.Title)
	}
	if done.Type != "Kanban::DoneColumn" {
		t.Errorf("expected type 'Kanban::DoneColumn', got %q", done.Type)
	}
	if done.Color != "green" {
		t.Errorf("expected color 'green', got %q", done.Color)
	}
}

func TestCard_UnmarshalList(t *testing.T) {
	data := loadCardsFixture(t, "list.json")

	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(cards) != 2 {
		t.Errorf("expected 2 cards, got %d", len(cards))
	}

	// Verify first card
	c1 := cards[0]
	if c1.ID != 1069479350 {
		t.Errorf("expected ID 1069479350, got %d", c1.ID)
	}
	if c1.Title != "Implement user authentication" {
		t.Errorf("expected title 'Implement user authentication', got %q", c1.Title)
	}
	if c1.Type != "Kanban::Card" {
		t.Errorf("expected type 'Kanban::Card', got %q", c1.Type)
	}
	if c1.DueOn != "2024-02-01" {
		t.Errorf("expected due_on '2024-02-01', got %q", c1.DueOn)
	}
	if c1.Position != 1 {
		t.Errorf("expected position 1, got %d", c1.Position)
	}
	if c1.Completed {
		t.Error("expected completed to be false")
	}
	if c1.CommentsCount != 2 {
		t.Errorf("expected comments_count 2, got %d", c1.CommentsCount)
	}

	// Verify parent (column)
	if c1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if c1.Parent.ID != 1069479347 {
		t.Errorf("expected Parent.ID 1069479347, got %d", c1.Parent.ID)
	}
	if c1.Parent.Title != "In Progress" {
		t.Errorf("expected Parent.Title 'In Progress', got %q", c1.Parent.Title)
	}
	if c1.Parent.Type != "Kanban::Column" {
		t.Errorf("expected Parent.Type 'Kanban::Column', got %q", c1.Parent.Type)
	}

	// Verify assignees
	if len(c1.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(c1.Assignees))
	}
	if c1.Assignees[0].Name != "Annie Bryan" {
		t.Errorf("expected assignee name 'Annie Bryan', got %q", c1.Assignees[0].Name)
	}

	// Verify steps
	if len(c1.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(c1.Steps))
	}
	step1 := c1.Steps[0]
	if step1.Title != "Set up OAuth providers" {
		t.Errorf("expected step title 'Set up OAuth providers', got %q", step1.Title)
	}
	if !step1.Completed {
		t.Error("expected first step to be completed")
	}
	if step1.Completer == nil {
		t.Fatal("expected Completer to be non-nil")
	}
	if step1.Completer.Name != "Annie Bryan" {
		t.Errorf("expected Completer.Name 'Annie Bryan', got %q", step1.Completer.Name)
	}

	step2 := c1.Steps[1]
	if step2.Title != "Implement callback handlers" {
		t.Errorf("expected step title 'Implement callback handlers', got %q", step2.Title)
	}
	if step2.Completed {
		t.Error("expected second step to not be completed")
	}

	// Verify second card
	c2 := cards[1]
	if c2.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", c2.ID)
	}
	if c2.Title != "Design landing page" {
		t.Errorf("expected title 'Design landing page', got %q", c2.Title)
	}
	if len(c2.Steps) != 0 {
		t.Errorf("expected no steps, got %d", len(c2.Steps))
	}
}

func TestCard_UnmarshalGet(t *testing.T) {
	data := loadCardsFixture(t, "get.json")

	var card Card
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if card.ID != 1069479350 {
		t.Errorf("expected ID 1069479350, got %d", card.ID)
	}
	if card.Title != "Implement user authentication" {
		t.Errorf("expected title 'Implement user authentication', got %q", card.Title)
	}
	if card.Type != "Kanban::Card" {
		t.Errorf("expected type 'Kanban::Card', got %q", card.Type)
	}

	// Verify timestamps
	if card.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if card.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if card.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if card.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", card.Creator.EmailAddress)
	}
	if card.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", card.Creator.Title)
	}

	// Verify assignees with details
	if len(card.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(card.Assignees))
	}
	assignee := card.Assignees[0]
	if assignee.EmailAddress != "annie@honchodesign.com" {
		t.Errorf("expected assignee email 'annie@honchodesign.com', got %q", assignee.EmailAddress)
	}

	// Verify steps
	if len(card.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(card.Steps))
	}
}

func TestCardColumn_Unmarshal(t *testing.T) {
	data := loadCardsFixture(t, "column.json")

	var column CardColumn
	if err := json.Unmarshal(data, &column); err != nil {
		t.Fatalf("failed to unmarshal column.json: %v", err)
	}

	if column.ID != 1069479347 {
		t.Errorf("expected ID 1069479347, got %d", column.ID)
	}
	if column.Title != "In Progress" {
		t.Errorf("expected title 'In Progress', got %q", column.Title)
	}
	if column.Type != "Kanban::Column" {
		t.Errorf("expected type 'Kanban::Column', got %q", column.Type)
	}
	if column.Color != "blue" {
		t.Errorf("expected color 'blue', got %q", column.Color)
	}
	if column.Position != 1 {
		t.Errorf("expected position 1, got %d", column.Position)
	}
	if column.Description != "Cards currently being worked on" {
		t.Errorf("expected description 'Cards currently being worked on', got %q", column.Description)
	}
	if column.CardsCount != 2 {
		t.Errorf("expected cards_count 2, got %d", column.CardsCount)
	}

	// Verify parent (card table)
	if column.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if column.Parent.ID != 1069479345 {
		t.Errorf("expected Parent.ID 1069479345, got %d", column.Parent.ID)
	}
	if column.Parent.Title != "Development Board" {
		t.Errorf("expected Parent.Title 'Development Board', got %q", column.Parent.Title)
	}
	if column.Parent.Type != "Kanban::Board" {
		t.Errorf("expected Parent.Type 'Kanban::Board', got %q", column.Parent.Type)
	}

	// Verify subscribers
	if len(column.Subscribers) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(column.Subscribers))
	}
	if column.Subscribers[0].Name != "Victor Cooper" {
		t.Errorf("expected subscriber name 'Victor Cooper', got %q", column.Subscribers[0].Name)
	}
}

func TestCardStep_Unmarshal(t *testing.T) {
	data := loadCardsFixture(t, "step.json")

	var step CardStep
	if err := json.Unmarshal(data, &step); err != nil {
		t.Fatalf("failed to unmarshal step.json: %v", err)
	}

	if step.ID != 1069479360 {
		t.Errorf("expected ID 1069479360, got %d", step.ID)
	}
	if step.Title != "Set up OAuth providers" {
		t.Errorf("expected title 'Set up OAuth providers', got %q", step.Title)
	}
	if step.Type != "Kanban::Step" {
		t.Errorf("expected type 'Kanban::Step', got %q", step.Type)
	}
	if step.Position != 1 {
		t.Errorf("expected position 1, got %d", step.Position)
	}
	if step.DueOn != "2024-01-20" {
		t.Errorf("expected due_on '2024-01-20', got %q", step.DueOn)
	}
	if !step.Completed {
		t.Error("expected completed to be true")
	}
	if step.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be non-nil")
	}

	// Verify parent (card)
	if step.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if step.Parent.ID != 1069479350 {
		t.Errorf("expected Parent.ID 1069479350, got %d", step.Parent.ID)
	}
	if step.Parent.Type != "Kanban::Card" {
		t.Errorf("expected Parent.Type 'Kanban::Card', got %q", step.Parent.Type)
	}

	// Verify completer
	if step.Completer == nil {
		t.Fatal("expected Completer to be non-nil")
	}
	if step.Completer.Name != "Annie Bryan" {
		t.Errorf("expected Completer.Name 'Annie Bryan', got %q", step.Completer.Name)
	}

	// Verify assignees
	if len(step.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(step.Assignees))
	}
	if step.Assignees[0].Name != "Annie Bryan" {
		t.Errorf("expected assignee name 'Annie Bryan', got %q", step.Assignees[0].Name)
	}
}

func TestCreateCardRequest_Marshal(t *testing.T) {
	req := CreateCardRequest{
		Title:   "New feature card",
		Content: "<div>Description of the feature</div>",
		DueOn:   "2024-03-01",
		Notify:  true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateCardRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "New feature card" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["content"] != "<div>Description of the feature</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["due_on"] != "2024-03-01" {
		t.Errorf("unexpected due_on: %v", data["due_on"])
	}
	if data["notify"] != true {
		t.Errorf("unexpected notify: %v", data["notify"])
	}
}

func TestCreateCardRequest_MarshalMinimal(t *testing.T) {
	req := CreateCardRequest{
		Title: "Simple card",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateCardRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Simple card" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if _, ok := data["content"]; ok {
		t.Error("expected content to be omitted")
	}
	if _, ok := data["due_on"]; ok {
		t.Error("expected due_on to be omitted")
	}
	if _, ok := data["notify"]; ok {
		t.Error("expected notify to be omitted")
	}
}

func TestUpdateCardRequest_Marshal(t *testing.T) {
	req := UpdateCardRequest{
		Title:       "Updated title",
		Content:     "<div>Updated content</div>",
		DueOn:       "2024-04-01",
		AssigneeIDs: []int64{1049715914, 1049715915},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateCardRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Updated title" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["content"] != "<div>Updated content</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["due_on"] != "2024-04-01" {
		t.Errorf("unexpected due_on: %v", data["due_on"])
	}

	assigneeIDs, ok := data["assignee_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected assignee_ids to be an array, got %T", data["assignee_ids"])
	}
	if len(assigneeIDs) != 2 {
		t.Errorf("expected 2 assignee IDs, got %d", len(assigneeIDs))
	}
}

func TestMoveCardRequest_Marshal(t *testing.T) {
	req := MoveCardRequest{
		ColumnID: 1069479348,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal MoveCardRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["column_id"] != float64(1069479348) {
		t.Errorf("unexpected column_id: %v", data["column_id"])
	}
}

func TestCreateColumnRequest_Marshal(t *testing.T) {
	req := CreateColumnRequest{
		Title:       "Review",
		Description: "Cards ready for review",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateColumnRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Review" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["description"] != "Cards ready for review" {
		t.Errorf("unexpected description: %v", data["description"])
	}
}

func TestSetColumnColorRequest_Marshal(t *testing.T) {
	req := SetColumnColorRequest{
		Color: "purple",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SetColumnColorRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["color"] != "purple" {
		t.Errorf("unexpected color: %v", data["color"])
	}
}

func TestMoveColumnRequest_Marshal(t *testing.T) {
	req := MoveColumnRequest{
		SourceID: 1069479347,
		TargetID: 1069479348,
		Position: 1,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal MoveColumnRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["source_id"] != float64(1069479347) {
		t.Errorf("unexpected source_id: %v", data["source_id"])
	}
	if data["target_id"] != float64(1069479348) {
		t.Errorf("unexpected target_id: %v", data["target_id"])
	}
	if data["position"] != float64(1) {
		t.Errorf("unexpected position: %v", data["position"])
	}
}

func TestCreateStepRequest_Marshal(t *testing.T) {
	req := CreateStepRequest{
		Title:     "Write tests",
		DueOn:     "2024-02-15",
		Assignees: "1049715914,1049715915",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateStepRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Write tests" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["due_on"] != "2024-02-15" {
		t.Errorf("unexpected due_on: %v", data["due_on"])
	}
	if data["assignees"] != "1049715914,1049715915" {
		t.Errorf("unexpected assignees: %v", data["assignees"])
	}
}

func TestUpdateStepRequest_Marshal(t *testing.T) {
	req := UpdateStepRequest{
		Title: "Updated step title",
		DueOn: "2024-02-20",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateStepRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Updated step title" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["due_on"] != "2024-02-20" {
		t.Errorf("unexpected due_on: %v", data["due_on"])
	}
	if _, ok := data["assignees"]; ok {
		t.Error("expected assignees to be omitted")
	}
}
