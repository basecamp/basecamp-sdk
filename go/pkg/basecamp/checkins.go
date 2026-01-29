package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// QuestionListOptions specifies options for listing questions.
type QuestionListOptions struct {
	// Limit is the maximum number of questions to return.
	// If 0 (default), returns all questions. Use a positive value to cap results.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// AnswerListOptions specifies options for listing answers.
type AnswerListOptions struct {
	// Limit is the maximum number of answers to return.
	// If 0 (default), returns all answers. Use a positive value to cap results.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// Questionnaire represents a Basecamp automatic check-in questionnaire.
type Questionnaire struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	QuestionsURL     string    `json:"questions_url"`
	QuestionsCount   int       `json:"questions_count"`
	Name             string    `json:"name"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
}

// QuestionSchedule represents the schedule configuration for a question.
type QuestionSchedule struct {
	Frequency     string `json:"frequency"`
	Days          []int  `json:"days"`
	Hour          int    `json:"hour"`
	Minute        int    `json:"minute"`
	WeekInstance  *int   `json:"week_instance,omitempty"`
	WeekInterval  *int   `json:"week_interval,omitempty"`
	MonthInterval *int   `json:"month_interval,omitempty"`
	StartDate     string `json:"start_date,omitempty"`
	EndDate       string `json:"end_date,omitempty"`
}

// Question represents a Basecamp automatic check-in question.
type Question struct {
	ID               int64             `json:"id"`
	Status           string            `json:"status"`
	VisibleToClients bool              `json:"visible_to_clients"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Title            string            `json:"title"`
	InheritsStatus   bool              `json:"inherits_status"`
	Type             string            `json:"type"`
	URL              string            `json:"url"`
	AppURL           string            `json:"app_url"`
	BookmarkURL      string            `json:"bookmark_url"`
	SubscriptionURL  string            `json:"subscription_url"`
	Parent           *Parent           `json:"parent,omitempty"`
	Bucket           *Bucket           `json:"bucket,omitempty"`
	Creator          *Person           `json:"creator,omitempty"`
	Paused           bool              `json:"paused"`
	Schedule         *QuestionSchedule `json:"schedule,omitempty"`
	AnswersCount     int               `json:"answers_count"`
	AnswersURL       string            `json:"answers_url"`
}

// QuestionAnswer represents an answer to a Basecamp check-in question.
type QuestionAnswer struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommentsCount    int       `json:"comments_count"`
	CommentsURL      string    `json:"comments_url"`
	Content          string    `json:"content"`
	GroupOn          string    `json:"group_on"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
}

// CreateQuestionRequest specifies the parameters for creating a question.
type CreateQuestionRequest struct {
	// Title is the question text (required).
	Title string `json:"title"`
	// Schedule is the question schedule configuration (required).
	Schedule *QuestionSchedule `json:"schedule"`
}

// UpdateQuestionRequest specifies the parameters for updating a question.
type UpdateQuestionRequest struct {
	// Title is the question text.
	Title string `json:"title,omitempty"`
	// Schedule is the question schedule configuration.
	Schedule *QuestionSchedule `json:"schedule,omitempty"`
	// Paused indicates whether the question is paused.
	Paused *bool `json:"paused,omitempty"`
}

// CreateAnswerRequest specifies the parameters for creating an answer.
type CreateAnswerRequest struct {
	// Content is the answer content in HTML (required).
	Content string `json:"content"`
	// GroupOn is the date to group the answer with (optional, ISO 8601 format).
	GroupOn string `json:"group_on,omitempty"`
}

// createAnswerRequestWrapper wraps the create request for the API.
// The Basecamp API expects: {"question_answer": {"content": "...", "group_on": "..."}}
type createAnswerRequestWrapper struct {
	QuestionAnswer *CreateAnswerRequest `json:"question_answer"`
}

// UpdateAnswerRequest specifies the parameters for updating an answer.
type UpdateAnswerRequest struct {
	// Content is the updated answer content in HTML (required).
	Content string `json:"content"`
}

// updateAnswerRequestWrapper wraps the update request for the API.
// The Basecamp API expects: {"question_answer": {"content": "..."}}
type updateAnswerRequestWrapper struct {
	QuestionAnswer *UpdateAnswerRequest `json:"question_answer"`
}

// CheckinsService handles automatic check-in operations.
type CheckinsService struct {
	client *AccountClient
}

// NewCheckinsService creates a new CheckinsService.
func NewCheckinsService(client *AccountClient) *CheckinsService {
	return &CheckinsService{client: client}
}

// GetQuestionnaire returns a questionnaire by ID.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
func (s *CheckinsService) GetQuestionnaire(ctx context.Context, bucketID, questionnaireID int64) (result *Questionnaire, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetQuestionnaire",
		ResourceType: "questionnaire", IsMutation: false,
		BucketID: bucketID, ResourceID: questionnaireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetQuestionnaireWithResponse(ctx, s.client.accountID, bucketID, questionnaireID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	questionnaire := questionnaireFromGenerated(*resp.JSON200)
	return &questionnaire, nil
}

// ListQuestions returns all questions in a questionnaire.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
//
// By default, returns all questions (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of questions to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
func (s *CheckinsService) ListQuestions(ctx context.Context, bucketID, questionnaireID int64, opts *QuestionListOptions) (result []Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListQuestions",
		ResourceType: "question", IsMutation: false,
		BucketID: bucketID, ResourceID: questionnaireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Handle single page fetch
	if opts != nil && opts.Page > 0 {
		resp, err := s.client.parent.gen.ListQuestionsWithResponse(ctx, s.client.accountID, bucketID, questionnaireID)
		if err != nil {
			return nil, err
		}
		if err = checkResponse(resp.HTTPResponse); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil
		}
		questions := make([]Question, 0, len(*resp.JSON200))
		for _, gq := range *resp.JSON200 {
			questions = append(questions, questionFromGenerated(gq))
		}
		return questions, nil
	}

	// Determine limit: 0 = all (default for questions), >0 = specific limit
	limit := 0 // default to all for questions
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	path := fmt.Sprintf("/buckets/%d/questionnaires/%d/questions.json", bucketID, questionnaireID)
	rawResults, err := s.client.GetAllWithLimit(ctx, path, limit)
	if err != nil {
		return nil, err
	}

	questions := make([]Question, 0, len(rawResults))
	for _, raw := range rawResults {
		var gq generated.Question
		if err := json.Unmarshal(raw, &gq); err != nil {
			return nil, fmt.Errorf("failed to parse question: %w", err)
		}
		questions = append(questions, questionFromGenerated(gq))
	}

	return questions, nil
}

// GetQuestion returns a question by ID.
// bucketID is the project ID, questionID is the question ID.
func (s *CheckinsService) GetQuestion(ctx context.Context, bucketID, questionID int64) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetQuestion",
		ResourceType: "question", IsMutation: false,
		BucketID: bucketID, ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetQuestionWithResponse(ctx, s.client.accountID, bucketID, questionID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	question := questionFromGenerated(*resp.JSON200)
	return &question, nil
}

// CreateQuestion creates a new question in a questionnaire.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
// Returns the created question.
func (s *CheckinsService) CreateQuestion(ctx context.Context, bucketID, questionnaireID int64, req *CreateQuestionRequest) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "CreateQuestion",
		ResourceType: "question", IsMutation: true,
		BucketID: bucketID, ResourceID: questionnaireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("question title is required")
		return nil, err
	}
	if req.Schedule == nil {
		err = ErrUsage("question schedule is required")
		return nil, err
	}

	body := generated.CreateQuestionJSONRequestBody{
		Title:    req.Title,
		Schedule: questionScheduleToGenerated(req.Schedule),
	}

	resp, err := s.client.parent.gen.CreateQuestionWithResponse(ctx, s.client.accountID, bucketID, questionnaireID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	question := questionFromGenerated(resp.JSON200.Question)
	return &question, nil
}

// UpdateQuestion updates an existing question.
// bucketID is the project ID, questionID is the question ID.
// Returns the updated question.
func (s *CheckinsService) UpdateQuestion(ctx context.Context, bucketID, questionID int64, req *UpdateQuestionRequest) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "UpdateQuestion",
		ResourceType: "question", IsMutation: true,
		BucketID: bucketID, ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateQuestionJSONRequestBody{}
	if req.Title != "" {
		body.Title = req.Title
	}
	if req.Schedule != nil {
		body.Schedule = questionScheduleToGenerated(req.Schedule)
	}
	if req.Paused != nil {
		body.Paused = *req.Paused
	}

	resp, err := s.client.parent.gen.UpdateQuestionWithResponse(ctx, s.client.accountID, bucketID, questionID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	question := questionFromGenerated(resp.JSON200.Question)
	return &question, nil
}

// ListAnswers returns all answers for a question.
// bucketID is the project ID, questionID is the question ID.
//
// By default, returns all answers (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of answers to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
func (s *CheckinsService) ListAnswers(ctx context.Context, bucketID, questionID int64, opts *AnswerListOptions) (result []QuestionAnswer, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListAnswers",
		ResourceType: "answer", IsMutation: false,
		BucketID: bucketID, ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Handle single page fetch
	if opts != nil && opts.Page > 0 {
		resp, err := s.client.parent.gen.ListAnswersWithResponse(ctx, s.client.accountID, bucketID, questionID)
		if err != nil {
			return nil, err
		}
		if err = checkResponse(resp.HTTPResponse); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil
		}
		answers := make([]QuestionAnswer, 0, len(*resp.JSON200))
		for _, ga := range *resp.JSON200 {
			answers = append(answers, questionAnswerFromGenerated(ga))
		}
		return answers, nil
	}

	// Determine limit: 0 = all (default for answers), >0 = specific limit
	limit := 0 // default to all for answers
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	path := fmt.Sprintf("/buckets/%d/questions/%d/answers.json", bucketID, questionID)
	rawResults, err := s.client.GetAllWithLimit(ctx, path, limit)
	if err != nil {
		return nil, err
	}

	answers := make([]QuestionAnswer, 0, len(rawResults))
	for _, raw := range rawResults {
		var ga generated.QuestionAnswer
		if err := json.Unmarshal(raw, &ga); err != nil {
			return nil, fmt.Errorf("failed to parse answer: %w", err)
		}
		answers = append(answers, questionAnswerFromGenerated(ga))
	}

	return answers, nil
}

// GetAnswer returns a question answer by ID.
// bucketID is the project ID, answerID is the answer ID.
func (s *CheckinsService) GetAnswer(ctx context.Context, bucketID, answerID int64) (result *QuestionAnswer, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetAnswer",
		ResourceType: "answer", IsMutation: false,
		BucketID: bucketID, ResourceID: answerID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetAnswerWithResponse(ctx, s.client.accountID, bucketID, answerID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	answer := questionAnswerFromGenerated(*resp.JSON200)
	return &answer, nil
}

// CreateAnswer creates a new answer for a question.
// bucketID is the project ID, questionID is the question ID.
// Returns the created answer.
func (s *CheckinsService) CreateAnswer(ctx context.Context, bucketID, questionID int64, req *CreateAnswerRequest) (result *QuestionAnswer, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "CreateAnswer",
		ResourceType: "answer", IsMutation: true,
		BucketID: bucketID, ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Content == "" {
		err = ErrUsage("answer content is required")
		return nil, err
	}

	body := generated.CreateAnswerJSONRequestBody{
		Content: req.Content,
	}
	if req.GroupOn != "" {
		if d, parseErr := types.ParseDate(req.GroupOn); parseErr == nil {
			body.GroupOn = d
		}
	}

	resp, err := s.client.parent.gen.CreateAnswerWithResponse(ctx, s.client.accountID, bucketID, questionID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	answer := questionAnswerFromGenerated(resp.JSON200.Answer)
	return &answer, nil
}

// UpdateAnswer updates an existing question answer.
// bucketID is the project ID, answerID is the answer ID.
// Returns nil on success (204 No Content).
func (s *CheckinsService) UpdateAnswer(ctx context.Context, bucketID, answerID int64, req *UpdateAnswerRequest) (err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "UpdateAnswer",
		ResourceType: "answer", IsMutation: true,
		BucketID: bucketID, ResourceID: answerID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Content == "" {
		err = ErrUsage("answer content is required")
		return err
	}

	body := generated.UpdateAnswerJSONRequestBody{
		Content: req.Content,
	}

	resp, err := s.client.parent.gen.UpdateAnswerWithResponse(ctx, s.client.accountID, bucketID, answerID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// questionnaireFromGenerated converts a generated Questionnaire to our clean type.
func questionnaireFromGenerated(gq generated.Questionnaire) Questionnaire {
	q := Questionnaire{
		Status:           gq.Status,
		VisibleToClients: gq.VisibleToClients,
		CreatedAt:        gq.CreatedAt,
		UpdatedAt:        gq.UpdatedAt,
		Title:            gq.Title,
		InheritsStatus:   gq.InheritsStatus,
		Type:             gq.Type,
		URL:              gq.Url,
		AppURL:           gq.AppUrl,
		BookmarkURL:      gq.BookmarkUrl,
		QuestionsURL:     gq.QuestionsUrl,
		QuestionsCount:   int(gq.QuestionsCount),
		Name:             gq.Name,
	}

	if gq.Id != nil {
		q.ID = *gq.Id
	}

	if gq.Bucket.Id != nil || gq.Bucket.Name != "" {
		q.Bucket = &Bucket{
			ID:   derefInt64(gq.Bucket.Id),
			Name: gq.Bucket.Name,
			Type: gq.Bucket.Type,
		}
	}

	if gq.Creator.Id != nil || gq.Creator.Name != "" {
		q.Creator = &Person{
			ID:           derefInt64(gq.Creator.Id),
			Name:         gq.Creator.Name,
			EmailAddress: gq.Creator.EmailAddress,
			AvatarURL:    gq.Creator.AvatarUrl,
			Admin:        gq.Creator.Admin,
			Owner:        gq.Creator.Owner,
		}
	}

	return q
}

// questionFromGenerated converts a generated Question to our clean type.
func questionFromGenerated(gq generated.Question) Question {
	q := Question{
		Status:           gq.Status,
		VisibleToClients: gq.VisibleToClients,
		CreatedAt:        gq.CreatedAt,
		UpdatedAt:        gq.UpdatedAt,
		Title:            gq.Title,
		InheritsStatus:   gq.InheritsStatus,
		Type:             gq.Type,
		URL:              gq.Url,
		AppURL:           gq.AppUrl,
		BookmarkURL:      gq.BookmarkUrl,
		SubscriptionURL:  gq.SubscriptionUrl,
		Paused:           gq.Paused,
		AnswersCount:     int(gq.AnswersCount),
		AnswersURL:       gq.AnswersUrl,
	}

	if gq.Id != nil {
		q.ID = *gq.Id
	}

	if gq.Schedule.Frequency != "" {
		days := make([]int, len(gq.Schedule.Days))
		for i, d := range gq.Schedule.Days {
			days[i] = int(d)
		}
		q.Schedule = &QuestionSchedule{
			Frequency: gq.Schedule.Frequency,
			Days:      days,
			Hour:      int(gq.Schedule.Hour),
			Minute:    int(gq.Schedule.Minute),
			StartDate: gq.Schedule.StartDate,
			EndDate:   gq.Schedule.EndDate,
		}
		if gq.Schedule.WeekInstance != 0 {
			wi := int(gq.Schedule.WeekInstance)
			q.Schedule.WeekInstance = &wi
		}
		if gq.Schedule.WeekInterval != 0 {
			wi := int(gq.Schedule.WeekInterval)
			q.Schedule.WeekInterval = &wi
		}
		if gq.Schedule.MonthInterval != 0 {
			mi := int(gq.Schedule.MonthInterval)
			q.Schedule.MonthInterval = &mi
		}
	}

	if gq.Parent.Id != nil || gq.Parent.Title != "" {
		q.Parent = &Parent{
			ID:     derefInt64(gq.Parent.Id),
			Title:  gq.Parent.Title,
			Type:   gq.Parent.Type,
			URL:    gq.Parent.Url,
			AppURL: gq.Parent.AppUrl,
		}
	}

	if gq.Bucket.Id != nil || gq.Bucket.Name != "" {
		q.Bucket = &Bucket{
			ID:   derefInt64(gq.Bucket.Id),
			Name: gq.Bucket.Name,
			Type: gq.Bucket.Type,
		}
	}

	if gq.Creator.Id != nil || gq.Creator.Name != "" {
		q.Creator = &Person{
			ID:           derefInt64(gq.Creator.Id),
			Name:         gq.Creator.Name,
			EmailAddress: gq.Creator.EmailAddress,
			AvatarURL:    gq.Creator.AvatarUrl,
			Admin:        gq.Creator.Admin,
			Owner:        gq.Creator.Owner,
		}
	}

	return q
}

// questionAnswerFromGenerated converts a generated QuestionAnswer to our clean type.
func questionAnswerFromGenerated(ga generated.QuestionAnswer) QuestionAnswer {
	a := QuestionAnswer{
		Status:           ga.Status,
		VisibleToClients: ga.VisibleToClients,
		CreatedAt:        ga.CreatedAt,
		UpdatedAt:        ga.UpdatedAt,
		Title:            ga.Title,
		InheritsStatus:   ga.InheritsStatus,
		Type:             ga.Type,
		URL:              ga.Url,
		AppURL:           ga.AppUrl,
		BookmarkURL:      ga.BookmarkUrl,
		SubscriptionURL:  ga.SubscriptionUrl,
		CommentsCount:    int(ga.CommentsCount),
		CommentsURL:      ga.CommentsUrl,
		Content:          ga.Content,
	}

	if ga.Id != nil {
		a.ID = *ga.Id
	}

	// Convert date fields to strings
	if !ga.GroupOn.IsZero() {
		a.GroupOn = ga.GroupOn.String()
	}

	if ga.Parent.Id != nil || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     derefInt64(ga.Parent.Id),
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}

	if ga.Bucket.Id != nil || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   derefInt64(ga.Bucket.Id),
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if ga.Creator.Id != nil || ga.Creator.Name != "" {
		a.Creator = &Person{
			ID:           derefInt64(ga.Creator.Id),
			Name:         ga.Creator.Name,
			EmailAddress: ga.Creator.EmailAddress,
			AvatarURL:    ga.Creator.AvatarUrl,
			Admin:        ga.Creator.Admin,
			Owner:        ga.Creator.Owner,
		}
	}

	return a
}

// questionScheduleToGenerated converts our QuestionSchedule to the generated type.
func questionScheduleToGenerated(s *QuestionSchedule) generated.QuestionSchedule {
	days := make([]int32, len(s.Days))
	for i, d := range s.Days {
		days[i] = int32(d)
	}

	gs := generated.QuestionSchedule{
		Frequency: s.Frequency,
		Days:      days,
		Hour:      int32(s.Hour),
		Minute:    int32(s.Minute),
		StartDate: s.StartDate,
		EndDate:   s.EndDate,
	}

	if s.WeekInstance != nil {
		gs.WeekInstance = int32(*s.WeekInstance)
	}
	if s.WeekInterval != nil {
		gs.WeekInterval = int32(*s.WeekInterval)
	}
	if s.MonthInterval != nil {
		gs.MonthInterval = int32(*s.MonthInterval)
	}

	return gs
}
