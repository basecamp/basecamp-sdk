package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
	if q1.Schedule.Hour != 17 {
		t.Errorf("expected Schedule.Hour 17, got %d", q1.Schedule.Hour)
	}
	if q1.Schedule.Minute != 0 {
		t.Errorf("expected Schedule.Minute 0, got %d", q1.Schedule.Minute)
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
			Hour:      17,
			Minute:    0,
		},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateQuestionRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "What are you working on?" {
		t.Errorf("unexpected title: %v", data["title"])
	}

	schedule, ok := data["schedule"].(map[string]interface{})
	if !ok {
		t.Fatal("expected schedule to be a map")
	}
	if schedule["frequency"] != "every_day" {
		t.Errorf("unexpected frequency: %v", schedule["frequency"])
	}
	if schedule["hour"] != float64(17) {
		t.Errorf("unexpected hour: %v", schedule["hour"])
	}
	if schedule["minute"] != float64(0) {
		t.Errorf("unexpected minute: %v", schedule["minute"])
	}

	days, ok := schedule["days"].([]interface{})
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
			Hour:      16,
			Minute:    30,
		},
		Paused: &paused,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateQuestionRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Updated question text" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["paused"] != true {
		t.Errorf("unexpected paused: %v", data["paused"])
	}

	schedule, ok := data["schedule"].(map[string]interface{})
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

	var data map[string]interface{}
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

	var data map[string]interface{}
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

	var data map[string]interface{}
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

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "<div>Updated: Today I finished the API documentation.</div>" {
		t.Errorf("unexpected content: %v", data["content"])
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

func TestCreateAnswerRequestWrapper_Marshal(t *testing.T) {
	// Test that the wrapper correctly wraps the request for the API
	// API expects: {"question_answer": {"content": "...", "group_on": "..."}}
	req := &CreateAnswerRequest{
		Content: "<div>Today I worked on the API documentation.</div>",
		GroupOn: "2024-01-22",
	}
	wrapper := createAnswerRequestWrapper{QuestionAnswer: req}

	out, err := json.Marshal(wrapper)
	if err != nil {
		t.Fatalf("failed to marshal wrapper: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Verify the structure is wrapped in "question_answer" key
	questionAnswer, ok := data["question_answer"].(map[string]interface{})
	if !ok {
		t.Fatal("expected question_answer to be a map")
	}
	if questionAnswer["content"] != "<div>Today I worked on the API documentation.</div>" {
		t.Errorf("unexpected content: %v", questionAnswer["content"])
	}
	if questionAnswer["group_on"] != "2024-01-22" {
		t.Errorf("unexpected group_on: %v", questionAnswer["group_on"])
	}
}

func TestUpdateAnswerRequestWrapper_Marshal(t *testing.T) {
	// Test that the wrapper correctly wraps the request for the API
	// API expects: {"question_answer": {"content": "..."}}
	req := &UpdateAnswerRequest{
		Content: "<div>My updated answer.</div>",
	}
	wrapper := updateAnswerRequestWrapper{QuestionAnswer: req}

	out, err := json.Marshal(wrapper)
	if err != nil {
		t.Fatalf("failed to marshal wrapper: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Verify the structure is wrapped in "question_answer" key
	questionAnswer, ok := data["question_answer"].(map[string]interface{})
	if !ok {
		t.Fatal("expected question_answer to be a map")
	}
	if questionAnswer["content"] != "<div>My updated answer.</div>" {
		t.Errorf("unexpected content: %v", questionAnswer["content"])
	}
}
