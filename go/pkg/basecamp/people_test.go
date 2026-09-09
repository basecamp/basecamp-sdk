package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestUpdateMyProfileRequest_Marshal(t *testing.T) {
	weekDay := FirstWeekDayMonday
	name := "Victor Cooper"
	title := "Chief Strategist"
	bio := "Don't let your dreams be dreams"
	location := "Chicago, IL"
	tz := "America/Chicago"
	req := UpdateMyProfileRequest{
		Name:         &name,
		Title:        &title,
		Bio:          &bio,
		Location:     &location,
		TimeZoneName: &tz,
		FirstWeekDay: &weekDay,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMyProfileRequest: %v", err)
	}

	var roundtrip UpdateMyProfileRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name == nil || *roundtrip.Name != *req.Name {
		t.Errorf("expected name %q, got %v", *req.Name, roundtrip.Name)
	}
	if roundtrip.Title == nil || *roundtrip.Title != *req.Title {
		t.Errorf("expected title %q, got %v", *req.Title, roundtrip.Title)
	}
	if roundtrip.FirstWeekDay == nil || *roundtrip.FirstWeekDay != *req.FirstWeekDay {
		t.Errorf("expected first_week_day %q, got %v", *req.FirstWeekDay, roundtrip.FirstWeekDay)
	}
}

func TestUpdateMyProfileRequest_MarshalOmitsNil(t *testing.T) {
	title := "New Title"
	req := UpdateMyProfileRequest{
		Title: &title,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMyProfileRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := data["title"]; !ok {
		t.Error("expected title to be present")
	}
	for _, field := range []string{"name", "email_address", "bio", "location", "time_zone_name", "first_week_day", "time_format"} {
		if _, ok := data[field]; ok {
			t.Errorf("expected %s to be omitted when nil", field)
		}
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
	var data map[string]any
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

func testPeopleServer(t *testing.T, handler http.HandlerFunc) *PeopleService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.People()
}

func TestPeopleService_UpdateMyProfile(t *testing.T) {
	var receivedBody map[string]any
	var receivedMethod string
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(204)
	})

	title := "Chief Strategist"
	bio := "Don't let your dreams be dreams"
	location := "Chicago, IL"
	weekDay := FirstWeekDaySunday
	err := svc.UpdateMyProfile(context.Background(), &UpdateMyProfileRequest{
		Title:        &title,
		Bio:          &bio,
		Location:     &location,
		FirstWeekDay: &weekDay,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	if receivedBody["title"] != "Chief Strategist" {
		t.Errorf("expected title 'Chief Strategist', got %v", receivedBody["title"])
	}
	if receivedBody["bio"] != "Don't let your dreams be dreams" {
		t.Errorf("expected bio in body, got %v", receivedBody["bio"])
	}
	if receivedBody["location"] != "Chicago, IL" {
		t.Errorf("expected location in body, got %v", receivedBody["location"])
	}
	if receivedBody["first_week_day"] != "Sunday" {
		t.Errorf("expected first_week_day 'Sunday', got %v", receivedBody["first_week_day"])
	}
}

func TestPeopleService_UpdateMyProfilePartial(t *testing.T) {
	var receivedBody map[string]any
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(204)
	})

	title := "New Title"
	err := svc.UpdateMyProfile(context.Background(), &UpdateMyProfileRequest{
		Title: &title,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["title"] != "New Title" {
		t.Errorf("expected title 'New Title', got %v", receivedBody["title"])
	}
	for _, field := range []string{"name", "email_address", "bio", "location", "time_zone_name", "first_week_day", "time_format"} {
		if _, ok := receivedBody[field]; ok {
			t.Errorf("expected %q to be omitted from partial update, but it was present: %v", field, receivedBody[field])
		}
	}
}

func TestPeopleService_UpdateMyProfileClearsField(t *testing.T) {
	var receivedBody map[string]any
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(204)
	})

	emptyBio := ""
	err := svc.UpdateMyProfile(context.Background(), &UpdateMyProfileRequest{
		Bio: &emptyBio,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bio, ok := receivedBody["bio"]
	if !ok {
		t.Fatal("expected bio to be present in request body (empty string must not be omitted)")
	}
	if bio != "" {
		t.Errorf("expected bio to be empty string, got %v", bio)
	}
}

func TestOutOfOffice_Unmarshal(t *testing.T) {
	data := loadPeopleFixture(t, "out-of-office.json")

	var ooo OutOfOffice
	if err := json.Unmarshal(data, &ooo); err != nil {
		t.Fatalf("failed to unmarshal out-of-office.json: %v", err)
	}

	if !ooo.Enabled {
		t.Error("expected enabled to be true")
	}
	if !ooo.Ongoing {
		t.Error("expected ongoing to be true")
	}
	if ooo.StartDate != "2026-07-20" {
		t.Errorf("expected start_date '2026-07-20', got %q", ooo.StartDate)
	}
	if ooo.EndDate != "2026-07-26" {
		t.Errorf("expected end_date '2026-07-26', got %q", ooo.EndDate)
	}
	if ooo.BackOnDate != "2026-07-27" {
		t.Errorf("expected back_on_date '2026-07-27', got %q", ooo.BackOnDate)
	}
	if ooo.Person.ID != 1049715913 {
		t.Errorf("expected person id 1049715913, got %d", ooo.Person.ID)
	}
	if ooo.Person.Name != "Victor Cooper" {
		t.Errorf("expected person name 'Victor Cooper', got %q", ooo.Person.Name)
	}
	if ooo.Person.AvatarURL == "" {
		t.Error("expected non-empty person avatar_url")
	}
}

func TestOutOfOffice_UnmarshalDisabled(t *testing.T) {
	// person_minimal always renders all three keys — id, name, avatar_url —
	// even when out of office is disabled (#659). A stub without avatar_url
	// is a payload the API cannot produce.
	data := []byte(`{"person":{"id":1049715913,"name":"Victor Cooper","avatar_url":"https://example.com/avatar"},"enabled":false,"ongoing":false}`)

	var ooo OutOfOffice
	if err := json.Unmarshal(data, &ooo); err != nil {
		t.Fatalf("failed to unmarshal disabled out-of-office: %v", err)
	}

	if ooo.Enabled {
		t.Error("expected enabled to be false")
	}
	if ooo.Person.AvatarURL != "https://example.com/avatar" {
		t.Errorf("expected avatar_url to flow through to the wrapper, got %q", ooo.Person.AvatarURL)
	}
	if ooo.Ongoing {
		t.Error("expected ongoing to be false")
	}
	if ooo.StartDate != "" || ooo.EndDate != "" || ooo.BackOnDate != "" {
		t.Errorf("expected omitted dates to be empty, got start=%q end=%q back_on=%q",
			ooo.StartDate, ooo.EndDate, ooo.BackOnDate)
	}
}

func TestPeopleService_UpdateProjectClientAccess(t *testing.T) {
	var receivedMethod, receivedPath string
	var receivedBody map[string]any
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod, receivedPath = r.Method, r.URL.Path
		receivedBody = decodeRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"granted":[{"id":444,"name":"annie@example.com","email_address":"annie@example.com","client":true}],"revoked":[{"id":333,"name":"Former Client","client":true}]}`))
	})

	result, err := svc.UpdateProjectClientAccess(context.Background(), 42, &UpdateProjectClientAccessRequest{
		Revoke: []int64{333},
		Create: []CreateClientRequest{{EmailAddress: "annie@example.com", CompanyName: "Springfield Elementary"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "PUT" || receivedPath != "/99999/projects/42/people/client_users.json" {
		t.Errorf("request = %s %s, want PUT /99999/projects/42/people/client_users.json", receivedMethod, receivedPath)
	}
	if _, present := receivedBody["grant"]; present {
		t.Errorf("expected grant omitted when nil, got %v", receivedBody["grant"])
	}
	create, ok := receivedBody["create"].([]any)
	if !ok || len(create) != 1 {
		t.Fatalf("expected one create row, got %v", receivedBody["create"])
	}
	row := create[0].(map[string]any)
	if row["email_address"] != "annie@example.com" || row["company_name"] != "Springfield Elementary" {
		t.Errorf("unexpected create row %v", row)
	}
	if _, present := row["name"]; present {
		t.Errorf("expected name omitted when blank so bc3 defaults it, got %v", row["name"])
	}
	if len(result.Granted) != 1 || result.Granted[0].ID != 444 || !result.Granted[0].Client {
		t.Errorf("unexpected granted %+v", result.Granted)
	}
	if len(result.Revoked) != 1 || result.Revoked[0].ID != 333 {
		t.Errorf("unexpected revoked %+v", result.Revoked)
	}
}

func TestPeopleService_UpdateProjectClientAccessRequiresAChange(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected for an empty change set")
	})

	_, err := svc.UpdateProjectClientAccess(context.Background(), 42, &UpdateProjectClientAccessRequest{})
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
}

// Clients must be enabled on the project first; bc3 answers `head :forbidden`.
func TestPeopleService_UpdateProjectClientAccessForbiddenUntilEnabled(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := svc.UpdateProjectClientAccess(context.Background(), 42, &UpdateProjectClientAccessRequest{Grant: []int64{111}})
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

// An invalid create row rejects the whole batch with the offending addresses.
func TestPeopleService_UpdateProjectClientAccessInvalidRow(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"email_address":"not-an-address","messages":["Email address must be valid"]}]}`))
	})

	_, err := svc.UpdateProjectClientAccess(context.Background(), 42, &UpdateProjectClientAccessRequest{
		Create: []CreateClientRequest{{EmailAddress: "annie@example.com"}, {EmailAddress: "not-an-address"}},
	})
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
	if bcErr.HTTPStatus != 422 {
		t.Errorf("http status = %d, want 422", bcErr.HTTPStatus)
	}
}

// New addresses beyond the account's user limit answer a bare 429; nobody is invited.
func TestPeopleService_UpdateProjectClientAccessSeatLimit(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := svc.UpdateProjectClientAccess(context.Background(), 42, &UpdateProjectClientAccessRequest{
		Create: []CreateClientRequest{{EmailAddress: "annie@example.com"}},
	})
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeRateLimit {
		t.Fatalf("expected rate_limit error, got %v", err)
	}
	if bcErr.HTTPStatus != 429 {
		t.Errorf("http status = %d, want 429", bcErr.HTTPStatus)
	}
}

func TestPeopleService_EnableProjectClients(t *testing.T) {
	var receivedMethod, receivedPath string
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod, receivedPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clients_enabled":true}`))
	})

	result, err := svc.EnableProjectClients(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "POST" || receivedPath != "/99999/projects/42/client_enablement.json" {
		t.Errorf("request = %s %s, want POST /99999/projects/42/client_enablement.json", receivedMethod, receivedPath)
	}
	if !result.ClientsEnabled {
		t.Error("expected clients_enabled true")
	}
}

// bc3 refuses to enable clients on a project that can't have them.
func TestPeopleService_EnableProjectClientsForbidden(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := svc.EnableProjectClients(context.Background(), 42)
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestPeopleService_DisableProjectClients(t *testing.T) {
	var receivedMethod, receivedPath string
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod, receivedPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clients_enabled":false}`))
	})

	result, err := svc.DisableProjectClients(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "DELETE" || receivedPath != "/99999/projects/42/client_enablement.json" {
		t.Errorf("request = %s %s, want DELETE /99999/projects/42/client_enablement.json", receivedMethod, receivedPath)
	}
	if result.ClientsEnabled {
		t.Error("expected clients_enabled false")
	}
}

// Disabling is refused while the project still has client users.
func TestPeopleService_DisableProjectClientsForbiddenWithClients(t *testing.T) {
	svc := testPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := svc.DisableProjectClients(context.Background(), 42)
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
