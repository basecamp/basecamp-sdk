package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// unmarshalCheckinsWithNumbers is an alias for the shared unmarshalWithNumbers helper.
var unmarshalCheckinsWithNumbers = unmarshalWithNumbers

func checkinsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "checkins")
}

func loadCheckinsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(checkinsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestQuestionnaire_Unmarshal(t *testing.T) {
	data := loadCheckinsFixture(t, "questionnaire.json")

	var q Questionnaire
	if err := json.Unmarshal(data, &q); err != nil {
		t.Fatalf("failed to unmarshal questionnaire.json: %v", err)
	}

	if q.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", q.ID)
	}
	if q.Status != "active" {
		t.Errorf("expected status 'active', got %q", q.Status)
	}
	if q.Type != "Questionnaire" {
		t.Errorf("expected type 'Questionnaire', got %q", q.Type)
	}
	if q.Title != "Automatic Check-ins" {
		t.Errorf("expected title 'Automatic Check-ins', got %q", q.Title)
	}
	if q.Name != "Automatic Check-ins" {
		t.Errorf("expected name 'Automatic Check-ins', got %q", q.Name)
	}
	if q.QuestionsCount != 2 {
		t.Errorf("expected questions_count 2, got %d", q.QuestionsCount)
	}
	if q.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/questionnaires/1069479400.json" {
		t.Errorf("unexpected URL: %q", q.URL)
	}
	if q.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/questionnaires/1069479400" {
		t.Errorf("unexpected AppURL: %q", q.AppURL)
	}
	if q.QuestionsURL != "https://3.basecampapi.com/195539477/buckets/2085958499/questionnaires/1069479400/questions.json" {
		t.Errorf("unexpected QuestionsURL: %q", q.QuestionsURL)
	}

	// Verify bucket
	if q.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if q.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", q.Bucket.ID)
	}
	if q.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", q.Bucket.Name)
	}

	// Verify creator
	if q.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if q.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", q.Creator.ID)
	}
	if q.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", q.Creator.Name)
	}

	// Verify timestamps are parsed
	if q.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if q.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestQuestion_UnmarshalList(t *testing.T) {
	data := loadCheckinsFixture(t, "questions_list.json")

	var questions []Question
	if err := json.Unmarshal(data, &questions); err != nil {
		t.Fatalf("failed to unmarshal questions_list.json: %v", err)
	}

	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}

	// Verify first question
	q1 := questions[0]
	if q1.ID != 1069479410 {
		t.Errorf("expected ID 1069479410, got %d", q1.ID)
	}
	if q1.Status != "active" {
		t.Errorf("expected status 'active', got %q", q1.Status)
	}
	if q1.Type != "Question" {
		t.Errorf("expected type 'Question', got %q", q1.Type)
	}
	if q1.Title != "What did you work on today?" {
		t.Errorf("expected title 'What did you work on today?', got %q", q1.Title)
	}
	if q1.Paused {
		t.Error("expected paused to be false")
	}
	if q1.AnswersCount != 5 {
		t.Errorf("expected answers_count 5, got %d", q1.AnswersCount)
	}
	if q1.AnswersURL != "https://3.basecampapi.com/195539477/buckets/2085958499/questions/1069479410/answers.json" {
		t.Errorf("unexpected AnswersURL: %q", q1.AnswersURL)
	}

	// Verify schedule
	if q1.Schedule == nil {
		t.Fatal("expected Schedule to be non-nil")
	}
	if q1.Schedule.Frequency != "every_day" {
		t.Errorf("expected Schedule.Frequency 'every_day', got %q", q1.Schedule.Frequency)
	}
	if len(q1.Schedule.Days) != 5 {
		t.Errorf("expected 5 days, got %d", len(q1.Schedule.Days))
	}
	if q1.Schedule.Hour == nil || *q1.Schedule.Hour != 17 {
		t.Errorf("expected Schedule.Hour 17, got %v", q1.Schedule.Hour)
	}
	if q1.Schedule.Minute == nil || *q1.Schedule.Minute != 0 {
		t.Errorf("expected Schedule.Minute 0, got %v", q1.Schedule.Minute)
	}

	// Verify parent (questionnaire)
	if q1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if q1.Parent.ID != 1069479400 {
		t.Errorf("expected Parent.ID 1069479400, got %d", q1.Parent.ID)
	}
	if q1.Parent.Type != "Questionnaire" {
		t.Errorf("expected Parent.Type 'Questionnaire', got %q", q1.Parent.Type)
	}

	// Verify bucket
	if q1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if q1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", q1.Bucket.ID)
	}

	// Verify creator
	if q1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if q1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", q1.Creator.Name)
	}

	// Verify second question
	q2 := questions[1]
	if q2.ID != 1069479420 {
		t.Errorf("expected ID 1069479420, got %d", q2.ID)
	}
	if q2.Title != "What's your plan for next week?" {
		t.Errorf("expected title 'What's your plan for next week?', got %q", q2.Title)
	}
	if q2.Schedule == nil {
		t.Fatal("expected Schedule to be non-nil for second question")
	}
	if q2.Schedule.Frequency != "every_week" {
		t.Errorf("expected Schedule.Frequency 'every_week', got %q", q2.Schedule.Frequency)
	}
	// Verify week_interval is parsed correctly
	if q2.Schedule.WeekInterval == nil {
		t.Fatal("expected Schedule.WeekInterval to be non-nil for weekly question")
	}
	if *q2.Schedule.WeekInterval != 1 {
		t.Errorf("expected Schedule.WeekInterval 1, got %d", *q2.Schedule.WeekInterval)
	}
	if q2.Schedule.StartDate != "2022-10-29" {
		t.Errorf("expected Schedule.StartDate '2022-10-29', got %q", q2.Schedule.StartDate)
	}
	// Verify creator with company
	if q2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second question")
	}
	if q2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", q2.Creator.Name)
	}
	if q2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second question")
	}
	if q2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", q2.Creator.Company.Name)
	}
}

func TestQuestion_UnmarshalGet(t *testing.T) {
	data := loadCheckinsFixture(t, "question.json")

	var question Question
	if err := json.Unmarshal(data, &question); err != nil {
		t.Fatalf("failed to unmarshal question.json: %v", err)
	}

	if question.ID != 1069479410 {
		t.Errorf("expected ID 1069479410, got %d", question.ID)
	}
	if question.Status != "active" {
		t.Errorf("expected status 'active', got %q", question.Status)
	}
	if question.Type != "Question" {
		t.Errorf("expected type 'Question', got %q", question.Type)
	}
	if question.Title != "What did you work on today?" {
		t.Errorf("expected title 'What did you work on today?', got %q", question.Title)
	}

	// Verify timestamps are parsed
	if question.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if question.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify schedule
	if question.Schedule == nil {
		t.Fatal("expected Schedule to be non-nil")
	}
	if question.Schedule.Frequency != "every_day" {
		t.Errorf("expected Schedule.Frequency 'every_day', got %q", question.Schedule.Frequency)
	}
	if question.Schedule.StartDate != "2022-10-28" {
		t.Errorf("expected Schedule.StartDate '2022-10-28', got %q", question.Schedule.StartDate)
	}
	// Verify null fields are nil pointers
	if question.Schedule.WeekInstance != nil {
		t.Errorf("expected Schedule.WeekInstance to be nil, got %v", *question.Schedule.WeekInstance)
	}
	if question.Schedule.EndDate != "" {
		t.Errorf("expected Schedule.EndDate to be empty, got %q", question.Schedule.EndDate)
	}
}

func TestQuestionAnswer_UnmarshalList(t *testing.T) {
	data := loadCheckinsFixture(t, "answers_list.json")

	var answers []QuestionAnswer
	if err := json.Unmarshal(data, &answers); err != nil {
		t.Fatalf("failed to unmarshal answers_list.json: %v", err)
	}

	if len(answers) != 2 {
		t.Errorf("expected 2 answers, got %d", len(answers))
	}

	// Verify first answer
	a1 := answers[0]
	if a1.ID != 1069479450 {
		t.Errorf("expected ID 1069479450, got %d", a1.ID)
	}
	if a1.Status != "active" {
		t.Errorf("expected status 'active', got %q", a1.Status)
	}
	if a1.Type != "Question::Answer" {
		t.Errorf("expected type 'Question::Answer', got %q", a1.Type)
	}
	if a1.Title != "What did you work on today?" {
		t.Errorf("expected title 'What did you work on today?', got %q", a1.Title)
	}
	if a1.Content != "<div>Worked on the new landing page design and reviewed PRs.</div>" {
		t.Errorf("unexpected content: %q", a1.Content)
	}
	if a1.GroupOn != "2022-10-28" {
		t.Errorf("expected group_on '2022-10-28', got %q", a1.GroupOn)
	}
	if a1.CommentsCount != 2 {
		t.Errorf("expected comments_count 2, got %d", a1.CommentsCount)
	}
	if a1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/question_answers/1069479450.json" {
		t.Errorf("unexpected URL: %q", a1.URL)
	}

	// Verify parent (question)
	if a1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if a1.Parent.ID != 1069479410 {
		t.Errorf("expected Parent.ID 1069479410, got %d", a1.Parent.ID)
	}
	if a1.Parent.Type != "Question" {
		t.Errorf("expected Parent.Type 'Question', got %q", a1.Parent.Type)
	}

	// Verify bucket
	if a1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if a1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", a1.Bucket.ID)
	}

	// Verify creator
	if a1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if a1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", a1.Creator.Name)
	}

	// Verify second answer
	a2 := answers[1]
	if a2.ID != 1069479460 {
		t.Errorf("expected ID 1069479460, got %d", a2.ID)
	}
	if a2.Content != "<div>Fixed authentication bugs and updated documentation.</div>" {
		t.Errorf("unexpected content: %q", a2.Content)
	}
	if a2.CommentsCount != 0 {
		t.Errorf("expected comments_count 0, got %d", a2.CommentsCount)
	}
	if a2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second answer")
	}
	if a2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", a2.Creator.Name)
	}
}

func TestQuestionAnswer_UnmarshalGet(t *testing.T) {
	data := loadCheckinsFixture(t, "answer.json")

	var answer QuestionAnswer
	if err := json.Unmarshal(data, &answer); err != nil {
		t.Fatalf("failed to unmarshal answer.json: %v", err)
	}

	if answer.ID != 1069479450 {
		t.Errorf("expected ID 1069479450, got %d", answer.ID)
	}
	if answer.Status != "active" {
		t.Errorf("expected status 'active', got %q", answer.Status)
	}
	if answer.Type != "Question::Answer" {
		t.Errorf("expected type 'Question::Answer', got %q", answer.Type)
	}
	if answer.Title != "What did you work on today?" {
		t.Errorf("expected title 'What did you work on today?', got %q", answer.Title)
	}
	expectedContent := "<div>Worked on the new landing page design and reviewed PRs.</div>"
	if answer.Content != expectedContent {
		t.Errorf("unexpected content: %q", answer.Content)
	}
	if answer.GroupOn != "2022-10-28" {
		t.Errorf("expected group_on '2022-10-28', got %q", answer.GroupOn)
	}

	// Verify timestamps are parsed
	if answer.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if answer.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if answer.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if answer.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", answer.Creator.ID)
	}
	if answer.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", answer.Creator.Name)
	}
	if answer.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", answer.Creator.EmailAddress)
	}
	if answer.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", answer.Creator.Title)
	}
	if !answer.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !answer.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
}

func TestCreateQuestionRequest_Marshal(t *testing.T) {
	req := CreateQuestionRequest{
		Title: "What are you working on?",
		Schedule: &QuestionSchedule{
			Frequency: "every_day",
			Days:      []int{1, 2, 3, 4, 5},
			Hour:      intPtr(17),
			Minute:    intPtr(0),
		},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateQuestionRequest: %v", err)
	}

	data, err := unmarshalCheckinsWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "What are you working on?" {
		t.Errorf("unexpected title: %v", data["title"])
	}

	schedule, ok := data["schedule"].(map[string]any)
	if !ok {
		t.Fatal("expected schedule to be a map")
	}
	if schedule["frequency"] != "every_day" {
		t.Errorf("unexpected frequency: %v", schedule["frequency"])
	}
	hour, _ := schedule["hour"].(json.Number).Int64()
	if hour != 17 {
		t.Errorf("unexpected hour: %v", schedule["hour"])
	}
	minute, _ := schedule["minute"].(json.Number).Int64()
	if minute != 0 {
		t.Errorf("unexpected minute: %v", schedule["minute"])
	}

	days, ok := schedule["days"].([]any)
	if !ok {
		t.Fatal("expected days to be an array")
	}
	if len(days) != 5 {
		t.Errorf("expected 5 days, got %d", len(days))
	}

	// Round-trip test
	var roundtrip CreateQuestionRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
	if roundtrip.Schedule == nil {
		t.Fatal("expected Schedule to be non-nil after round-trip")
	}
	if roundtrip.Schedule.Frequency != req.Schedule.Frequency {
		t.Errorf("expected frequency %q, got %q", req.Schedule.Frequency, roundtrip.Schedule.Frequency)
	}
}

func TestUpdateQuestionRequest_Marshal(t *testing.T) {
	paused := true
	req := UpdateQuestionRequest{
		Title: "Updated question text",
		Schedule: &QuestionSchedule{
			Frequency: "every_week",
			Days:      []int{5},
			Hour:      intPtr(16),
			Minute:    intPtr(30),
		},
		Paused: &paused,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateQuestionRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Updated question text" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["paused"] != true {
		t.Errorf("unexpected paused: %v", data["paused"])
	}

	schedule, ok := data["schedule"].(map[string]any)
	if !ok {
		t.Fatal("expected schedule to be a map")
	}
	if schedule["frequency"] != "every_week" {
		t.Errorf("unexpected frequency: %v", schedule["frequency"])
	}
}

func TestUpdateQuestionRequest_MarshalPartial(t *testing.T) {
	// Test with only title
	req := UpdateQuestionRequest{
		Title: "Just updating title",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateQuestionRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Just updating title" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	// Optional fields should be omitted
	if _, ok := data["schedule"]; ok {
		t.Error("expected schedule to be omitted")
	}
	if _, ok := data["paused"]; ok {
		t.Error("expected paused to be omitted")
	}
}

func TestCreateAnswerRequest_Marshal(t *testing.T) {
	req := CreateAnswerRequest{
		Content: "<div>Working on the new feature.</div>",
		GroupOn: "2022-10-28",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateAnswerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div>Working on the new feature.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["group_on"] != "2022-10-28" {
		t.Errorf("unexpected group_on: %v", data["group_on"])
	}

	// Round-trip test
	var roundtrip CreateAnswerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
	if roundtrip.GroupOn != req.GroupOn {
		t.Errorf("expected group_on %q, got %q", req.GroupOn, roundtrip.GroupOn)
	}
}

func TestCreateAnswerRequest_MarshalMinimal(t *testing.T) {
	// Test with only required field
	req := CreateAnswerRequest{
		Content: "<div>My answer</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateAnswerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div>My answer</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	// Optional field with omitempty should not be present
	if _, ok := data["group_on"]; ok {
		t.Error("expected group_on to be omitted")
	}
}

func TestUpdateAnswerRequest_Marshal(t *testing.T) {
	req := UpdateAnswerRequest{
		Content: "<div>Updated: Today I finished the API documentation.</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateAnswerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div>Updated: Today I finished the API documentation.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	// group_on with omitempty should be absent when empty
	if _, ok := data["group_on"]; ok {
		t.Error("expected group_on to be omitted when empty")
	}

	// Round-trip test
	var roundtrip UpdateAnswerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
}

func TestUpdateAnswerRequest_MarshalWithGroupOn(t *testing.T) {
	req := UpdateAnswerRequest{
		Content: "<div>Updated answer.</div>",
		GroupOn: "2024-01-22",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateAnswerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div>Updated answer.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["group_on"] != "2024-01-22" {
		t.Errorf("expected group_on '2024-01-22', got %v", data["group_on"])
	}
}

func testCheckinsServer(t *testing.T, handler http.HandlerFunc) *CheckinsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Checkins()
}

func TestCheckinsService_UpdateQuestionPartial(t *testing.T) {
	fixture := loadCheckinsFixture(t, "question.json")
	var receivedBody map[string]any
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
		Title: "New question title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["title"] != "New question title" {
		t.Errorf("expected title 'New question title', got %v", receivedBody["title"])
	}

	for _, field := range []string{"schedule", "paused"} {
		if _, ok := receivedBody[field]; ok {
			t.Errorf("expected %q to be omitted from partial update, but it was present: %v", field, receivedBody[field])
		}
	}
}

func TestCheckinsService_UpdateQuestionPartialSchedule(t *testing.T) {
	fixture := loadCheckinsFixture(t, "question.json")
	var receivedBody map[string]any
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	// Update only the schedule end_date — other schedule fields must not leak
	_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
		Schedule: &QuestionSchedule{
			EndDate: "2025-06-30",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schedRaw, ok := receivedBody["schedule"]
	if !ok {
		t.Fatal("expected schedule to be present")
	}
	sched, ok := schedRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected schedule to be a map, got %T", schedRaw)
	}

	if sched["end_date"] != "2025-06-30" {
		t.Errorf("expected end_date '2025-06-30', got %v", sched["end_date"])
	}

	// Zero-valued schedule fields must NOT be present
	for _, field := range []string{"frequency", "days", "hour", "minute", "start_date"} {
		if _, ok := sched[field]; ok {
			t.Errorf("expected schedule.%q to be omitted, but it was present: %v", field, sched[field])
		}
	}
}

func TestCheckinsService_UpdateQuestionEmptySchedule(t *testing.T) {
	fixture := loadCheckinsFixture(t, "question.json")
	var receivedBody map[string]any
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	// Non-nil but entirely empty Schedule must not leak as {}
	_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
		Title:    "New title",
		Schedule: &QuestionSchedule{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := receivedBody["schedule"]; ok {
		t.Errorf("expected schedule to be omitted for empty struct, but it was present: %v", receivedBody["schedule"])
	}
}

// TestCheckinsService_QuestionBodyBytes pins the exact bytes CreateQuestion and
// UpdateQuestion put on the wire. The expectations were captured from the
// questionScheduleToMap implementation before both moved to the generated
// request types (#653): the test is the invariant, the swap is the change.
// Byte-level equality is deliberate — key order, the explicit-empty `days: []`,
// and the presence or absence of every omitted member are the contract being
// preserved.
func TestCheckinsService_QuestionBodyBytes(t *testing.T) {
	intp := func(v int) *int { return &v }
	boolp := func(v bool) *bool { return &v }

	for _, tc := range []struct {
		name string
		call func(svc *CheckinsService) error
		want string
	}{
		{
			name: "create full schedule",
			call: func(svc *CheckinsService) error {
				_, err := svc.CreateQuestion(context.Background(), 777, &CreateQuestionRequest{
					Title: "What did you work on today?",
					Schedule: &QuestionSchedule{
						Frequency:     "on_certain_days",
						Days:          []int{1, 2, 3, 4, 5},
						Hour:          intp(17),
						Minute:        intp(0),
						WeekInstance:  intp(1),
						WeekInterval:  intp(2),
						MonthInterval: intp(3),
						StartDate:     "2025-01-01",
						EndDate:       "2025-06-30",
					},
					VisibleToClients: boolp(true),
				})
				return err
			},
			want: `{"schedule":{"days":[1,2,3,4,5],"end_date":"2025-06-30","frequency":"on_certain_days","hour":17,"minute":0,"month_interval":3,"start_date":"2025-01-01","week_instance":1,"week_interval":2},"title":"What did you work on today?","visible_to_clients":true}`,
		},
		{
			name: "create explicit empty days",
			call: func(svc *CheckinsService) error {
				_, err := svc.CreateQuestion(context.Background(), 777, &CreateQuestionRequest{
					Title: "Standup?",
					Schedule: &QuestionSchedule{
						Frequency: "every_day",
						Days:      []int{},
					},
				})
				return err
			},
			want: `{"schedule":{"days":[],"frequency":"every_day"},"title":"Standup?"}`,
		},
		{
			name: "create frequency only",
			call: func(svc *CheckinsService) error {
				_, err := svc.CreateQuestion(context.Background(), 777, &CreateQuestionRequest{
					Title:    "Standup?",
					Schedule: &QuestionSchedule{Frequency: "every_day"},
				})
				return err
			},
			want: `{"schedule":{"frequency":"every_day"},"title":"Standup?"}`,
		},
		{
			name: "update title only",
			call: func(svc *CheckinsService) error {
				_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
					Title: "New question title",
				})
				return err
			},
			want: `{"title":"New question title"}`,
		},
		{
			name: "update paused only",
			call: func(svc *CheckinsService) error {
				_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
					Paused: boolp(true),
				})
				return err
			},
			want: `{"paused":true}`,
		},
		{
			name: "update partial schedule",
			call: func(svc *CheckinsService) error {
				_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
					Schedule: &QuestionSchedule{EndDate: "2025-06-30"},
				})
				return err
			},
			want: `{"schedule":{"end_date":"2025-06-30"}}`,
		},
		{
			name: "update empty schedule struct omitted",
			call: func(svc *CheckinsService) error {
				_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
					Title:    "New title",
					Schedule: &QuestionSchedule{},
				})
				return err
			},
			want: `{"title":"New title"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := loadCheckinsFixture(t, "question.json")
			var gotBody string
			svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "POST" {
					w.WriteHeader(201)
				} else {
					w.WriteHeader(200)
				}
				w.Write(fixture)
			})

			if err := tc.call(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotBody != tc.want {
				t.Errorf("request body = %s, want %s", gotBody, tc.want)
			}
		})
	}
}

// TestCheckinsService_QuestionScheduleRejectsOutOfRangeInts: the generated
// QuestionSchedule types its integers as int32, so a schedule int that does
// not fit must fail fast as a usage error with nothing sent on the wire —
// never wrap silently (gosec G115: 4294967297 would otherwise reach the
// server as 1). The map implementation sent the caller's int verbatim and
// let the server reject it; rejecting client-side is a deliberate behavior
// change, trading a round-trip to a server 4xx for an immediate usage error.
func TestCheckinsService_QuestionScheduleRejectsOutOfRangeInts(t *testing.T) {
	if strconv.IntSize == 32 {
		t.Skip("out-of-int32-range ints are not representable on a 32-bit platform")
	}
	// Materialized as an int64 variable first: written as one constant
	// expression, int(...) of 2^31+1 overflows int at COMPILE time on a
	// 32-bit platform — before the skip above can run. Narrowing a runtime
	// value compiles everywhere; the skip keeps 32-bit builds honest.
	big64 := int64(math.MaxInt32) + 2 // 2^31+1; wraps to a small value if narrowed blindly
	big := int(big64)

	fixture := loadCheckinsFixture(t, "question.json")
	var requestCount int
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(fixture)
	})

	for name, call := range map[string]func() error{
		"create days": func() error {
			_, err := svc.CreateQuestion(context.Background(), 777, &CreateQuestionRequest{
				Title:    "Standup?",
				Schedule: &QuestionSchedule{Frequency: "every_day", Days: []int{big}},
			})
			return err
		},
		"update hour": func() error {
			_, err := svc.UpdateQuestion(context.Background(), 12345, &UpdateQuestionRequest{
				Schedule: &QuestionSchedule{Hour: &big},
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			apiErr, ok := err.(*Error)
			if !ok || apiErr.Code != CodeUsage {
				t.Fatalf("expected usage error, got %v", err)
			}
			if requestCount != 0 {
				t.Fatalf("expected 0 requests (nothing on the wire), got %d", requestCount)
			}
		})
	}
}

func TestCheckinsService_UpdateAnswerPreservesGroupOn(t *testing.T) {
	answerFixture := loadCheckinsFixture(t, "answer.json")
	var receivedBody map[string]any
	var requestCount int

	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			// First request: GET to fetch existing answer for its group_on
			w.WriteHeader(200)
			w.Write(answerFixture)
		case http.MethodPut:
			// Second request: PUT with content and preserved group_on
			receivedBody = decodeRequestBody(t, r)
			w.WriteHeader(204)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})

	err := svc.UpdateAnswer(context.Background(), 1069479450, &UpdateAnswerRequest{
		Content: "<div>Updated content.</div>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 requests (GET + PUT), got %d", requestCount)
	}

	if receivedBody["content"] != "<div>Updated content.</div>" {
		t.Errorf("unexpected content: %v", receivedBody["content"])
	}

	// The existing answer fixture has group_on "2022-10-28" — it must be carried forward
	if receivedBody["group_on"] != "2022-10-28" {
		t.Errorf("expected group_on '2022-10-28' preserved from existing answer, got %v", receivedBody["group_on"])
	}
}

func TestCheckinsService_UpdateAnswerExplicitGroupOn(t *testing.T) {
	var receivedBody map[string]any
	var requestCount int

	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodPut:
			receivedBody = decodeRequestBody(t, r)
			w.WriteHeader(204)
		default:
			t.Fatalf("unexpected method: %s (should skip GET when GroupOn is provided)", r.Method)
		}
	})

	// When GroupOn is explicitly provided, no GET should be needed
	err := svc.UpdateAnswer(context.Background(), 1069479450, &UpdateAnswerRequest{
		Content: "<div>Updated content.</div>",
		GroupOn: "2025-03-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount != 1 {
		t.Fatalf("expected 1 request (PUT only), got %d", requestCount)
	}

	if receivedBody["content"] != "<div>Updated content.</div>" {
		t.Errorf("unexpected content: %v", receivedBody["content"])
	}
	if receivedBody["group_on"] != "2025-03-01" {
		t.Errorf("expected group_on '2025-03-01', got %v", receivedBody["group_on"])
	}
}

func TestCheckinsService_UpdateAnswerRejectsInvalidExplicitGroupOn(t *testing.T) {
	var requestCount int

	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	err := svc.UpdateAnswer(context.Background(), 1069479450, &UpdateAnswerRequest{
		Content: "<div>Updated content.</div>",
		GroupOn: "2025/03/01",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != CodeUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("expected 0 requests, got %d", requestCount)
	}
	if apiErr.Message != "group_on must be in YYYY-MM-DD format" {
		t.Fatalf("unexpected error message: %q", apiErr.Message)
	}
}

func TestCheckinsService_UpdateAnswerRejectsMissingResolvedGroupOn(t *testing.T) {
	var requestCount int

	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(200)
			w.Write([]byte(`{"id":1069479450,"content":"Existing answer","group_on":null}`))
		case http.MethodPut:
			t.Fatal("unexpected PUT request when resolved group_on is empty")
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})

	err := svc.UpdateAnswer(context.Background(), 1069479450, &UpdateAnswerRequest{
		Content: "<div>Updated content.</div>",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != CodeUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 request (GET only), got %d", requestCount)
	}
	if apiErr.Message != "group_on is required" {
		t.Fatalf("unexpected error message: %q", apiErr.Message)
	}
}

func TestCheckinsService_ListAnswersByPerson(t *testing.T) {
	fixture := loadCheckinsFixture(t, "answers_by_person.json")

	var requestedMethod, requestedPath string
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.ListAnswersByPerson(context.Background(), 1069479410, 1049715914, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", requestedMethod)
	}
	if requestedPath != "/99999/questions/1069479410/answers/by/1049715914" {
		t.Errorf("expected path /99999/questions/1069479410/answers/by/1049715914, got %q", requestedPath)
	}

	if len(result.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(result.Answers))
	}

	a := result.Answers[0]
	if a.ID != 1069479450 {
		t.Errorf("expected ID 1069479450, got %d", a.ID)
	}
	if a.Creator == nil || a.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %v", a.Creator)
	}
	if a.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", a.Creator.Name)
	}
}

func TestCheckinsService_ListAnswersByPerson_Pagination(t *testing.T) {
	page1Body := `[{"id":1,"creator":{"id":1049715914,"name":"A"}},{"id":2,"creator":{"id":1049715914,"name":"B"}}]`
	page2Body := `[{"id":3,"creator":{"id":1049715914,"name":"C"}},{"id":4,"creator":{"id":1049715914,"name":"D"}}]`

	cases := []struct {
		name          string
		emitNextLink  bool
		opts          *AnswerListOptions
		wantAnswers   int
		wantRequests  int
		wantTruncated bool
	}{
		{
			name:          "collects across pages when no limit",
			emitNextLink:  true,
			opts:          nil,
			wantAnswers:   4,
			wantRequests:  2,
			wantTruncated: false,
		},
		{
			// A pinned page is one request, and the rel="next" Link this
			// call deliberately does not follow is what makes the result
			// truncated (SPEC §8's ListMeta).
			name:          "Page option returns first page and skips Link follow",
			emitNextLink:  true,
			opts:          &AnswerListOptions{Page: 1},
			wantAnswers:   2,
			wantRequests:  1,
			wantTruncated: true,
		},
		{
			name:          "Limit smaller than first page truncates without follow",
			emitNextLink:  true,
			opts:          &AnswerListOptions{Limit: 1},
			wantAnswers:   1,
			wantRequests:  1,
			wantTruncated: true,
		},
		{
			name:          "Limit straddling page boundary trims second page",
			emitNextLink:  true,
			opts:          &AnswerListOptions{Limit: 3},
			wantAnswers:   3,
			wantRequests:  2,
			wantTruncated: true,
		},
		{
			name:          "Limit equal to total across pages is not truncated",
			emitNextLink:  true,
			opts:          &AnswerListOptions{Limit: 4},
			wantAnswers:   4,
			wantRequests:  2,
			wantTruncated: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestCount int
			svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				if requestCount == 1 {
					if tc.emitNextLink {
						w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/questions/1069479410/answers/by/1049715914?page=2>; rel="next"`, r.Host))
					}
					w.WriteHeader(200)
					w.Write([]byte(page1Body))
					return
				}
				w.WriteHeader(200)
				w.Write([]byte(page2Body))
			})

			result, err := svc.ListAnswersByPerson(context.Background(), 1069479410, 1049715914, tc.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Answers) != tc.wantAnswers {
				t.Errorf("expected %d answers, got %d", tc.wantAnswers, len(result.Answers))
			}
			if requestCount != tc.wantRequests {
				t.Errorf("expected %d HTTP requests, got %d", tc.wantRequests, requestCount)
			}
			if result.Meta.Truncated != tc.wantTruncated {
				t.Errorf("expected Truncated=%v, got %v", tc.wantTruncated, result.Meta.Truncated)
			}
		})
	}
}

// TestCheckinsService_CreateQuestionVisibleToClients verifies the tri-state
// visible_to_clients flag reaches the wire correctly on create: nil omits the
// key, true is sent verbatim, and an explicit false is sent (not dropped).
//
// CreateQuestion builds its body as a map (not the generated struct), so this
// also pins the flag as a top-level sibling of title/schedule — mis-nesting it
// inside the schedule wrapper would be a silent server-side no-op.
func TestCheckinsService_CreateQuestionVisibleToClients(t *testing.T) {
	fixture := loadCheckinsFixture(t, "question.json")
	tru, fls := true, false
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{"nil omits the field", nil, false, false},
		{"true is sent", &tru, true, true},
		{"explicit false is sent, not dropped", &fls, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody map[string]any
			svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
				receivedBody = decodeRequestBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				w.Write(fixture)
			})

			_, err := svc.CreateQuestion(context.Background(), 12345, &CreateQuestionRequest{
				Title:            "How are you?",
				Schedule:         &QuestionSchedule{Frequency: "every_day", Days: []int{1, 2, 3, 4, 5}},
				VisibleToClients: tc.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// The flag must be a top-level key, never nested in the schedule wrapper.
			if sched, ok := receivedBody["schedule"].(map[string]any); ok {
				if _, nested := sched["visible_to_clients"]; nested {
					t.Error("visible_to_clients must be top-level, not nested inside schedule")
				}
			}

			val, ok := receivedBody["visible_to_clients"]
			if ok != tc.present {
				t.Fatalf("visible_to_clients present=%v, want %v (body=%v)", ok, tc.present, receivedBody)
			}
			if tc.present && val != tc.want {
				t.Errorf("visible_to_clients=%v, want %v", val, tc.want)
			}
		})
	}
}

func TestCheckinsService_PauseQuestion(t *testing.T) {
	var requestedMethod, requestedPath string
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"paused":true}`))
	})

	if err := svc.PauseQuestion(context.Background(), 1069479410); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", requestedMethod)
	}
	if requestedPath != "/99999/questions/1069479410/pause.json" {
		t.Errorf("expected path /99999/questions/1069479410/pause.json, got %q", requestedPath)
	}
}

func TestCheckinsService_ResumeQuestion(t *testing.T) {
	var requestedMethod, requestedPath string
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"paused":false}`))
	})

	if err := svc.ResumeQuestion(context.Background(), 1069479410); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", requestedMethod)
	}
	if requestedPath != "/99999/questions/1069479410/pause.json" {
		t.Errorf("expected path /99999/questions/1069479410/pause.json, got %q", requestedPath)
	}
}

func TestCheckinsService_PauseQuestionError(t *testing.T) {
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	})

	err := svc.PauseQuestion(context.Background(), 1069479410)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != CodeNotFound {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// TestCheckinsService_UpdateQuestionNotificationSettings verifies the wire
// shape (PUT to notification_settings.json) and the tri-state request fields:
// nil omits a key, and an explicit false is sent (not dropped).
func TestCheckinsService_UpdateQuestionNotificationSettings(t *testing.T) {
	fls := false

	var requestedMethod, requestedPath string
	var receivedBody map[string]any
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		receivedBody = decodeRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"responding":true,"subscribed":false}`))
	})

	settings, err := svc.UpdateQuestionNotificationSettings(context.Background(), 1069479410, &UpdateQuestionNotificationSettingsRequest{
		NotifyOnAnswer: &fls,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", requestedMethod)
	}
	if requestedPath != "/99999/questions/1069479410/notification_settings.json" {
		t.Errorf("expected path /99999/questions/1069479410/notification_settings.json, got %q", requestedPath)
	}

	if got, ok := receivedBody["notify_on_answer"]; !ok || got != false {
		t.Errorf("expected notify_on_answer false to reach the wire, got %v (present=%v)", got, ok)
	}
	if _, ok := receivedBody["digest_include_unanswered"]; ok {
		t.Errorf("expected digest_include_unanswered to be omitted when nil, but it was present: %v", receivedBody["digest_include_unanswered"])
	}

	if !settings.Responding {
		t.Error("expected Responding true")
	}
	if settings.Subscribed {
		t.Error("expected Subscribed false")
	}
}

func TestCheckinsService_UpdateQuestionNotificationSettingsRequiresRequest(t *testing.T) {
	var requestCount int
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := svc.UpdateQuestionNotificationSettings(context.Background(), 1069479410, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != CodeUsage {
		t.Fatalf("expected usage error, got %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("expected 0 requests, got %d", requestCount)
	}
}

func TestCheckinsService_ListAnswerers(t *testing.T) {
	var requestedMethod, requestedPath string
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "2")
		w.WriteHeader(200)
		w.Write([]byte(`[
			{"id":1049715914,"name":"Victor Cooper","email_address":"victor@honchodesign.com"},
			{"id":1049715915,"name":"Annie Bryan","email_address":"annie@honchodesign.com"}
		]`))
	})

	result, err := svc.ListAnswerers(context.Background(), 1069479410, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", requestedMethod)
	}
	if requestedPath != "/99999/questions/1069479410/answers/by.json" {
		t.Errorf("expected path /99999/questions/1069479410/answers/by.json, got %q", requestedPath)
	}

	if len(result.People) != 2 {
		t.Fatalf("expected 2 people, got %d", len(result.People))
	}
	if result.People[0].ID != 1049715914 || result.People[0].Name != "Victor Cooper" {
		t.Errorf("unexpected first person: %+v", result.People[0])
	}
	if result.People[1].EmailAddress != "annie@honchodesign.com" {
		t.Errorf("unexpected second person email: %q", result.People[1].EmailAddress)
	}
	if result.Meta.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", result.Meta.TotalCount)
	}
}

func TestCheckinsService_ListAnswerers_Pagination(t *testing.T) {
	page1Body := `[{"id":1,"name":"A"},{"id":2,"name":"B"}]`
	page2Body := `[{"id":3,"name":"C"},{"id":4,"name":"D"}]`

	cases := []struct {
		name          string
		opts          *PeopleListOptions
		wantPeople    int
		wantRequests  int
		wantTruncated bool
	}{
		{"collects across pages when no limit", nil, 4, 2, false},
		{"Page option returns first page and skips Link follow", &PeopleListOptions{Page: 1}, 2, 1, true},
		{"Limit smaller than first page truncates without follow", &PeopleListOptions{Limit: 1}, 1, 1, true},
		{"Limit straddling page boundary trims second page", &PeopleListOptions{Limit: 3}, 3, 2, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestCount int
			svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				if requestCount == 1 {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/questions/1069479410/answers/by.json?page=2>; rel="next"`, r.Host))
					w.WriteHeader(200)
					w.Write([]byte(page1Body))
					return
				}
				w.WriteHeader(200)
				w.Write([]byte(page2Body))
			})

			result, err := svc.ListAnswerers(context.Background(), 1069479410, tc.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.People) != tc.wantPeople {
				t.Errorf("expected %d people, got %d", tc.wantPeople, len(result.People))
			}
			if requestCount != tc.wantRequests {
				t.Errorf("expected %d HTTP requests, got %d", tc.wantRequests, requestCount)
			}
			if result.Meta.Truncated != tc.wantTruncated {
				t.Errorf("expected Truncated=%v, got %v", tc.wantTruncated, result.Meta.Truncated)
			}
		})
	}
}

func TestCheckinsService_ListQuestionReminders(t *testing.T) {
	var requestedMethod, requestedPath string
	svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedMethod = r.Method
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[
			{
				"group_on": "2022-10-28",
				"remind_at": "2022-10-28T09:00:00.000Z",
				"reminder_id": 123,
				"question": {"id": 1069479410, "title": "What did you work on today?", "type": "Question", "paused": false}
			},
			{
				"remind_at": "2022-10-29T09:00:00.000Z",
				"question": {"id": 1069479411, "title": "Any blockers?", "type": "Question", "paused": false}
			}
		]`))
	})

	result, err := svc.ListQuestionReminders(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", requestedMethod)
	}
	if requestedPath != "/99999/my/question_reminders.json" {
		t.Errorf("expected path /99999/my/question_reminders.json, got %q", requestedPath)
	}

	if len(result.Reminders) != 2 {
		t.Fatalf("expected 2 reminders, got %d", len(result.Reminders))
	}

	r1 := result.Reminders[0]
	if r1.GroupOn != "2022-10-28" {
		t.Errorf("expected GroupOn '2022-10-28', got %q", r1.GroupOn)
	}
	if r1.RemindAt == nil {
		t.Error("expected RemindAt to be non-nil")
	} else if r1.RemindAt.IsZero() {
		t.Error("expected RemindAt to be non-zero")
	}
	if r1.ReminderID == nil || *r1.ReminderID != 123 {
		t.Errorf("expected ReminderID 123, got %v", r1.ReminderID)
	}
	if r1.Question.ID != 1069479410 {
		t.Errorf("expected Question.ID 1069479410, got %d", r1.Question.ID)
	}
	if r1.Question.Title != "What did you work on today?" {
		t.Errorf("unexpected Question.Title: %q", r1.Question.Title)
	}

	r2 := result.Reminders[1]
	if r2.GroupOn != "" {
		t.Errorf("expected empty GroupOn when absent, got %q", r2.GroupOn)
	}
	if r2.ReminderID != nil {
		t.Errorf("expected nil ReminderID when absent, got %v", r2.ReminderID)
	}
	if r2.Question.ID != 1069479411 {
		t.Errorf("expected Question.ID 1069479411, got %d", r2.Question.ID)
	}
}

func TestCheckinsService_ListQuestionReminders_Pagination(t *testing.T) {
	page1Body := `[{"remind_at":"2022-10-28T09:00:00.000Z","question":{"id":1}},{"remind_at":"2022-10-28T09:00:00.000Z","question":{"id":2}}]`
	page2Body := `[{"remind_at":"2022-10-28T09:00:00.000Z","question":{"id":3}},{"remind_at":"2022-10-28T09:00:00.000Z","question":{"id":4}}]`

	cases := []struct {
		name          string
		opts          *QuestionReminderListOptions
		wantReminders int
		wantRequests  int
		wantTruncated bool
	}{
		{"collects across pages when no limit", nil, 4, 2, false},
		{"Page option returns first page and skips Link follow", &QuestionReminderListOptions{Page: 1}, 2, 1, true},
		{"Limit smaller than first page truncates without follow", &QuestionReminderListOptions{Limit: 1}, 1, 1, true},
		{"Limit straddling page boundary trims second page", &QuestionReminderListOptions{Limit: 3}, 3, 2, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestCount int
			svc := testCheckinsServer(t, func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				if requestCount == 1 {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s/99999/my/question_reminders.json?page=2>; rel="next"`, r.Host))
					w.WriteHeader(200)
					w.Write([]byte(page1Body))
					return
				}
				w.WriteHeader(200)
				w.Write([]byte(page2Body))
			})

			result, err := svc.ListQuestionReminders(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Reminders) != tc.wantReminders {
				t.Errorf("expected %d reminders, got %d", tc.wantReminders, len(result.Reminders))
			}
			if requestCount != tc.wantRequests {
				t.Errorf("expected %d HTTP requests, got %d", tc.wantRequests, requestCount)
			}
			if result.Meta.Truncated != tc.wantTruncated {
				t.Errorf("expected Truncated=%v, got %v", tc.wantTruncated, result.Meta.Truncated)
			}
		})
	}
}
