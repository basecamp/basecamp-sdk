package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func peopleFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "people")
}

func loadPeopleFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(peopleFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestPerson_UnmarshalList(t *testing.T) {
	data := loadPeopleFixture(t, "list.json")

	var people []Person
	if err := json.Unmarshal(data, &people); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(people) != 2 {
		t.Errorf("expected 2 people, got %d", len(people))
	}

	// Verify first person
	p1 := people[0]
	if p1.ID != 1049715915 {
		t.Errorf("expected ID 1049715915, got %d", p1.ID)
	}
	if p1.Name != "Victor Cooper" {
		t.Errorf("expected name 'Victor Cooper', got %q", p1.Name)
	}
	if p1.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected email 'victor@honchodesign.com', got %q", p1.EmailAddress)
	}
	if p1.PersonableType != "User" {
		t.Errorf("expected personable_type 'User', got %q", p1.PersonableType)
	}
	if p1.Title != "Chief Strategist" {
		t.Errorf("expected title 'Chief Strategist', got %q", p1.Title)
	}
	if !p1.Admin {
		t.Error("expected admin to be true")
	}
	if !p1.Owner {
		t.Error("expected owner to be true")
	}
	if p1.Client {
		t.Error("expected client to be false")
	}
	if !p1.Employee {
		t.Error("expected employee to be true")
	}

	// Verify company
	if p1.Company == nil {
		t.Fatal("expected Company to be non-nil")
	}
	if p1.Company.ID != 1033447817 {
		t.Errorf("expected Company.ID 1033447817, got %d", p1.Company.ID)
	}
	if p1.Company.Name != "Honcho Design" {
		t.Errorf("expected Company.Name 'Honcho Design', got %q", p1.Company.Name)
	}

	// Verify second person
	p2 := people[1]
	if p2.ID != 1049715920 {
		t.Errorf("expected ID 1049715920, got %d", p2.ID)
	}
	if p2.Name != "Steve Marsh" {
		t.Errorf("expected name 'Steve Marsh', got %q", p2.Name)
	}
	if p2.Admin {
		t.Error("expected admin to be false for second person")
	}
}

func TestPerson_UnmarshalGet(t *testing.T) {
	data := loadPeopleFixture(t, "get.json")

	var person Person
	if err := json.Unmarshal(data, &person); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if person.ID != 1049715915 {
		t.Errorf("expected ID 1049715915, got %d", person.ID)
	}
	if person.Name != "Victor Cooper" {
		t.Errorf("expected name 'Victor Cooper', got %q", person.Name)
	}
	if person.Bio != "Don't let your dreams be dreams" {
		t.Errorf("expected bio 'Don't let your dreams be dreams', got %q", person.Bio)
	}
	if person.Location != "Chicago, IL" {
		t.Errorf("expected location 'Chicago, IL', got %q", person.Location)
	}
	if person.TimeZone != "America/Chicago" {
		t.Errorf("expected time_zone 'America/Chicago', got %q", person.TimeZone)
	}
	if person.AvatarURL == "" {
		t.Error("expected non-empty AvatarURL")
	}
	if !person.CanManageProjects {
		t.Error("expected can_manage_projects to be true")
	}
	if !person.CanManagePeople {
		t.Error("expected can_manage_people to be true")
	}
}

func TestPerson_UnmarshalPingable(t *testing.T) {
	data := loadPeopleFixture(t, "pingable.json")

	var people []Person
	if err := json.Unmarshal(data, &people); err != nil {
		t.Fatalf("failed to unmarshal pingable.json: %v", err)
	}

	if len(people) != 1 {
		t.Errorf("expected 1 person, got %d", len(people))
	}

	p := people[0]
	if p.ID != 1049715915 {
		t.Errorf("expected ID 1049715915, got %d", p.ID)
	}
	if !p.CanPing {
		t.Error("expected can_ping to be true")
	}
}

func TestUpdateProjectAccessRequest_Marshal(t *testing.T) {
	data := loadPeopleFixture(t, "update-access-request.json")

	var req UpdateProjectAccessRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-access-request.json: %v", err)
	}

	if len(req.Grant) != 1 || req.Grant[0] != 1049715920 {
		t.Errorf("expected grant [1049715920], got %v", req.Grant)
	}
	if len(req.Revoke) != 1 || req.Revoke[0] != 1049715925 {
		t.Errorf("expected revoke [1049715925], got %v", req.Revoke)
	}
	if len(req.Create) != 1 {
		t.Fatalf("expected 1 create entry, got %d", len(req.Create))
	}
	if req.Create[0].Name != "New Person" {
		t.Errorf("expected create name 'New Person', got %q", req.Create[0].Name)
	}
	if req.Create[0].EmailAddress != "new@example.com" {
		t.Errorf("expected create email 'new@example.com', got %q", req.Create[0].EmailAddress)
	}
	if req.Create[0].Title != "Developer" {
		t.Errorf("expected create title 'Developer', got %q", req.Create[0].Title)
	}
	if req.Create[0].CompanyName != "Acme Corp" {
		t.Errorf("expected create company_name 'Acme Corp', got %q", req.Create[0].CompanyName)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateProjectAccessRequest: %v", err)
	}

	var roundtrip UpdateProjectAccessRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if len(roundtrip.Grant) != len(req.Grant) || len(roundtrip.Revoke) != len(req.Revoke) {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateProjectAccessResponse_Unmarshal(t *testing.T) {
	data := loadPeopleFixture(t, "update-access-response.json")

	var resp UpdateProjectAccessResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal update-access-response.json: %v", err)
	}

	if len(resp.Granted) != 1 {
		t.Fatalf("expected 1 granted person, got %d", len(resp.Granted))
	}
	if resp.Granted[0].ID != 1049715920 {
		t.Errorf("expected granted ID 1049715920, got %d", resp.Granted[0].ID)
	}
	if resp.Granted[0].Name != "Steve Marsh" {
		t.Errorf("expected granted name 'Steve Marsh', got %q", resp.Granted[0].Name)
	}

	if len(resp.Revoked) != 1 {
		t.Fatalf("expected 1 revoked person, got %d", len(resp.Revoked))
	}
	if resp.Revoked[0].ID != 1049715925 {
		t.Errorf("expected revoked ID 1049715925, got %d", resp.Revoked[0].ID)
	}
	if resp.Revoked[0].Name != "Former Member" {
		t.Errorf("expected revoked name 'Former Member', got %q", resp.Revoked[0].Name)
	}
}

func TestCreatePersonRequest_Marshal(t *testing.T) {
	req := CreatePersonRequest{
		Name:         "Test User",
		EmailAddress: "test@example.com",
		Title:        "Engineer",
		CompanyName:  "Test Corp",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreatePersonRequest: %v", err)
	}

	var roundtrip CreatePersonRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.EmailAddress != req.EmailAddress {
		t.Errorf("expected email %q, got %q", req.EmailAddress, roundtrip.EmailAddress)
	}
	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
	if roundtrip.CompanyName != req.CompanyName {
		t.Errorf("expected company_name %q, got %q", req.CompanyName, roundtrip.CompanyName)
	}
}

func TestCreatePersonRequest_MarshalOmitsEmpty(t *testing.T) {
	req := CreatePersonRequest{
		Name:         "Test User",
		EmailAddress: "test@example.com",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreatePersonRequest: %v", err)
	}

	// Verify that empty optional fields are omitted
	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := data["title"]; ok {
		t.Error("expected title to be omitted when empty")
	}
	if _, ok := data["company_name"]; ok {
		t.Error("expected company_name to be omitted when empty")
	}
}
