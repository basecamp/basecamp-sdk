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

	// Page, if positive, fetches only that page and disables auto-pagination.
	// Use 0 to paginate through all results up to Limit.
	Page int
}

// AnswerListOptions specifies options for listing answers.
type AnswerListOptions struct {
	// Limit is the maximum number of answers to return.
	// If 0 (default), returns all answers. Use a positive value to cap results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	// Use 0 to paginate through all results up to Limit.
	Page int
}

// QuestionReminderListOptions specifies options for listing question reminders.
type QuestionReminderListOptions struct {
	// Limit is the maximum number of reminders to return.
	// If 0 (default), returns all reminders. Use a positive value to cap results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	// Use 0 to paginate through all results up to Limit.
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
//
// BREAKING CHANGE: Hour and Minute changed from int to *int so that
// "not provided" (nil) is distinguishable from "set to 0" (midnight / top
// of hour).
type QuestionSchedule struct {
	Frequency     string `json:"frequency"`
	Days          []int  `json:"days"`
	Hour          *int   `json:"hour,omitempty"`
	Minute        *int   `json:"minute,omitempty"`
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
	BoostsCount      int       `json:"boosts_count,omitempty"`
	BoostsURL        string    `json:"boosts_url,omitempty"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommentsCount    int       `json:"comments_count"`
	CommentsURL      string    `json:"comments_url"`
	Content          string    `json:"content"`
	// ContentAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Content. @required — the API always sends this
	// array (empty when the content has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	ContentAttachments []RichTextAttachment `json:"content_attachments"`
	GroupOn            string               `json:"group_on"`
	Parent             *Parent              `json:"parent,omitempty"`
	Bucket             *Bucket              `json:"bucket,omitempty"`
	Creator            *Person              `json:"creator,omitempty"`
}

// QuestionReminder represents a pending check-in reminder for the current user.
type QuestionReminder struct {
	GroupOn    string    `json:"group_on,omitempty"`
	Question   Question  `json:"question"`
	RemindAt   time.Time `json:"remind_at"`
	ReminderID *int64    `json:"reminder_id,omitempty"`
}

// QuestionNotificationSettings represents the current user's notification
// settings for a check-in question.
type QuestionNotificationSettings struct {
	Responding bool `json:"responding"`
	Subscribed bool `json:"subscribed"`
}

// CreateQuestionRequest specifies the parameters for creating a question.
type CreateQuestionRequest struct {
	// Title is the question text (required).
	Title string `json:"title"`
	// Schedule is the question schedule configuration (required).
	Schedule *QuestionSchedule `json:"schedule"`
	// VisibleToClients sets client visibility at create time (optional, tri-state).
	// nil omits the field so the server applies its own default visibility rule; a
	// non-nil value is sent verbatim, and an explicit false reaches the wire (the
	// pointer distinguishes unset from false).
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
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

// UpdateAnswerRequest specifies the parameters for updating an answer.
type UpdateAnswerRequest struct {
	// Content is the updated answer content in HTML (required).
	Content string `json:"content"`
	// GroupOn is the date the answer is grouped under (ISO 8601 format).
	// If empty, the existing group_on is preserved automatically.
	GroupOn string `json:"group_on,omitempty"`
}

// UpdateQuestionNotificationSettingsRequest specifies the parameters for
// updating the current user's notification settings for a question.
//
// Both fields are optional and tri-state: nil omits the field so the server
// leaves that setting unchanged; a non-nil value is sent verbatim, and an
// explicit false reaches the wire (the pointer distinguishes unset from false).
type UpdateQuestionNotificationSettingsRequest struct {
	// NotifyOnAnswer controls whether the user is notified when someone answers.
	NotifyOnAnswer *bool `json:"notify_on_answer,omitempty"`
	// DigestIncludeUnanswered controls whether unanswered questions are
	// included in the digest.
	DigestIncludeUnanswered *bool `json:"digest_include_unanswered,omitempty"`
}

// QuestionListResult contains the results from listing questions.
type QuestionListResult struct {
	// Questions is the list of questions returned.
	Questions []Question
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// AnswerListResult contains the results from listing answers.
type AnswerListResult struct {
	// Answers is the list of answers returned.
	Answers []QuestionAnswer
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// QuestionReminderListResult contains the results from listing question reminders.
type QuestionReminderListResult struct {
	// Reminders is the list of question reminders returned.
	Reminders []QuestionReminder
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
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
func (s *CheckinsService) GetQuestionnaire(ctx context.Context, questionnaireID int64) (result *Questionnaire, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetQuestionnaire",
		ResourceType: "questionnaire", IsMutation: false,
		ResourceID: questionnaireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetQuestionnaireWithResponse(ctx, s.client.accountID, questionnaireID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
//
// By default, returns all questions (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of questions to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned QuestionListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CheckinsService) ListQuestions(ctx context.Context, questionnaireID int64, opts *QuestionListOptions) (result *QuestionListResult, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListQuestions",
		ResourceType: "question", IsMutation: false,
		ResourceID: questionnaireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	var params *generated.ListQuestionsParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListQuestionsParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListQuestionsWithResponse(ctx, s.client.accountID, questionnaireID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var questions []Question
	if resp.JSON200 != nil {
		for _, gq := range *resp.JSON200 {
			questions = append(questions, questionFromGenerated(gq))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &QuestionListResult{Questions: questions, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	// Determine limit: 0 = all (default for questions), >0 = specific limit
	limit := 0 // default to all for questions
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(questions) >= limit {
		return &QuestionListResult{Questions: questions[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(questions), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(questions), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gq generated.Question
		if err := json.Unmarshal(raw, &gq); err != nil {
			return nil, fmt.Errorf("failed to parse question: %w", err)
		}
		questions = append(questions, questionFromGenerated(gq))
	}

	return &QuestionListResult{Questions: questions, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetQuestion returns a question by ID.
func (s *CheckinsService) GetQuestion(ctx context.Context, questionID int64) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetQuestion",
		ResourceType: "question", IsMutation: false,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetQuestionWithResponse(ctx, s.client.accountID, questionID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
// Returns the created question.
func (s *CheckinsService) CreateQuestion(ctx context.Context, questionnaireID int64, req *CreateQuestionRequest) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "CreateQuestion",
		ResourceType: "question", IsMutation: true,
		ResourceID: questionnaireID,
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

	body := map[string]any{
		"title":    req.Title,
		"schedule": questionScheduleToMap(req.Schedule),
	}
	if req.VisibleToClients != nil {
		body["visible_to_clients"] = *req.VisibleToClients
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.CreateQuestionWithBodyWithResponse(ctx, s.client.accountID, questionnaireID, "application/json", bodyReader)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	question := questionFromGenerated(*resp.JSON201)
	return &question, nil
}

// UpdateQuestion updates an existing question.
// Returns the updated question.
func (s *CheckinsService) UpdateQuestion(ctx context.Context, questionID int64, req *UpdateQuestionRequest) (result *Question, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "UpdateQuestion",
		ResourceType: "question", IsMutation: true,
		ResourceID: questionID,
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

	body := map[string]any{}
	if req.Title != "" {
		body["title"] = req.Title
	}
	if req.Schedule != nil {
		sm := questionScheduleToMap(req.Schedule)
		if len(sm) > 0 {
			body["schedule"] = sm
		}
	}
	if req.Paused != nil {
		body["paused"] = *req.Paused
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.UpdateQuestionWithBodyWithResponse(ctx, s.client.accountID, questionID, "application/json", bodyReader)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	question := questionFromGenerated(*resp.JSON200)
	return &question, nil
}

// PauseQuestion pauses a check-in question, stopping its reminders.
// Returns nil on success.
func (s *CheckinsService) PauseQuestion(ctx context.Context, questionID int64) (err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "PauseQuestion",
		ResourceType: "question", IsMutation: true,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.PauseQuestionWithResponse(ctx, s.client.accountID, questionID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// ResumeQuestion resumes a paused check-in question, restarting its reminders.
// Returns nil on success.
func (s *CheckinsService) ResumeQuestion(ctx context.Context, questionID int64) (err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ResumeQuestion",
		ResourceType: "question", IsMutation: true,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ResumeQuestionWithResponse(ctx, s.client.accountID, questionID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// UpdateQuestionNotificationSettings updates the current user's notification
// settings for a question.
// Returns the updated settings.
func (s *CheckinsService) UpdateQuestionNotificationSettings(ctx context.Context, questionID int64, req *UpdateQuestionNotificationSettingsRequest) (result *QuestionNotificationSettings, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "UpdateQuestionNotificationSettings",
		ResourceType: "question", IsMutation: true,
		ResourceID: questionID,
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

	body := generated.UpdateQuestionNotificationSettingsJSONRequestBody{
		NotifyOnAnswer:          req.NotifyOnAnswer,
		DigestIncludeUnanswered: req.DigestIncludeUnanswered,
	}

	resp, err := s.client.parent.gen.UpdateQuestionNotificationSettingsWithResponse(ctx, s.client.accountID, questionID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	settings := questionNotificationSettingsFromGenerated(*resp.JSON200)
	return &settings, nil
}

// ListAnswers returns all answers for a question.
//
// By default, returns all answers (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of answers to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination.
//     Use 0 to paginate through all results up to Limit.
//
// The returned AnswerListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CheckinsService) ListAnswers(ctx context.Context, questionID int64, opts *AnswerListOptions) (result *AnswerListResult, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListAnswers",
		ResourceType: "answer", IsMutation: false,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	var params *generated.ListAnswersParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListAnswersParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListAnswersWithResponse(ctx, s.client.accountID, questionID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var answers []QuestionAnswer
	if resp.JSON200 != nil {
		for _, ga := range *resp.JSON200 {
			answers = append(answers, questionAnswerFromGenerated(ga))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &AnswerListResult{Answers: answers, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	// Determine limit: 0 = all (default for answers), >0 = specific limit
	limit := 0 // default to all for answers
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(answers) >= limit {
		return &AnswerListResult{Answers: answers[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(answers), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(answers), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var ga generated.QuestionAnswer
		if err := json.Unmarshal(raw, &ga); err != nil {
			return nil, fmt.Errorf("failed to parse answer: %w", err)
		}
		answers = append(answers, questionAnswerFromGenerated(ga))
	}

	return &AnswerListResult{Answers: answers, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// ListAnswersByPerson returns all answers for a question posted by a specific person.
//
// By default, returns all answers (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of answers to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination.
//     Use 0 to paginate through all results up to Limit.
func (s *CheckinsService) ListAnswersByPerson(ctx context.Context, questionID, personID int64, opts *AnswerListOptions) (result *AnswerListResult, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListAnswersByPerson",
		ResourceType: "answer", IsMutation: false,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetAnswersByPersonParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.GetAnswersByPersonParams{Page: page}
	}

	resp, err := s.client.parent.gen.GetAnswersByPersonWithResponse(ctx, s.client.accountID, questionID, personID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	var answers []QuestionAnswer
	if resp.JSON200 != nil {
		for _, ga := range *resp.JSON200 {
			answers = append(answers, questionAnswerFromGenerated(ga))
		}
	}

	if opts != nil && opts.Page > 0 {
		return &AnswerListResult{Answers: answers, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	limit := 0
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	if limit > 0 && len(answers) >= limit {
		return &AnswerListResult{Answers: answers[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(answers), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(answers), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var ga generated.QuestionAnswer
		if err := json.Unmarshal(raw, &ga); err != nil {
			return nil, fmt.Errorf("failed to parse answer: %w", err)
		}
		answers = append(answers, questionAnswerFromGenerated(ga))
	}

	return &AnswerListResult{Answers: answers, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// ListAnswerers returns all people who have answered a question.
//
// By default, returns all answerers (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of people to return (0 = all)
//   - Page: the page number is ignored (the answerers endpoint is not
//     paginated server-side), but any positive value still disables
//     auto-pagination, returning the single response as-is without applying Limit
//
// The returned PeopleListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CheckinsService) ListAnswerers(ctx context.Context, questionID int64, opts *PeopleListOptions) (result *PeopleListResult, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListAnswerers",
		ResourceType: "person", IsMutation: false,
		ResourceID: questionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListQuestionAnswerersWithResponse(ctx, s.client.accountID, questionID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	var people []Person
	if resp.JSON200 != nil {
		for _, gp := range *resp.JSON200 {
			people = append(people, personFromGenerated(gp))
		}
	}

	if opts != nil && opts.Page > 0 {
		return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	limit := 0
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(people), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(people), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var gp generated.Person
		if err := json.Unmarshal(raw, &gp); err != nil {
			return nil, fmt.Errorf("failed to parse person: %w", err)
		}
		people = append(people, personFromGenerated(gp))
	}

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetAnswer returns a question answer by ID.
func (s *CheckinsService) GetAnswer(ctx context.Context, answerID int64) (result *QuestionAnswer, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "GetAnswer",
		ResourceType: "answer", IsMutation: false,
		ResourceID: answerID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetAnswerWithResponse(ctx, s.client.accountID, answerID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
// Returns the created answer.
func (s *CheckinsService) CreateAnswer(ctx context.Context, questionID int64, req *CreateAnswerRequest) (result *QuestionAnswer, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "CreateAnswer",
		ResourceType: "answer", IsMutation: true,
		ResourceID: questionID,
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
		d, parseErr := types.ParseDate(req.GroupOn)
		if parseErr != nil {
			err = ErrUsage("answer group_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.GroupOn = &d
	}

	resp, err := s.client.parent.gen.CreateAnswerWithResponse(ctx, s.client.accountID, questionID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	answer := questionAnswerFromGenerated(*resp.JSON201)
	return &answer, nil
}

// UpdateAnswer updates an existing question answer.
// Returns nil on success (204 No Content).
func (s *CheckinsService) UpdateAnswer(ctx context.Context, answerID int64, req *UpdateAnswerRequest) (err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "UpdateAnswer",
		ResourceType: "answer", IsMutation: true,
		ResourceID: answerID,
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

	// The BC3 Question::Answer model validates presence of group_on.
	// The controller rebuilds the recordable from params, so we must
	// always include group_on even when only updating content.
	groupOn := req.GroupOn
	if groupOn == "" {
		existing, fetchErr := s.GetAnswer(ctx, answerID)
		if fetchErr != nil {
			return fetchErr
		}
		groupOn = existing.GroupOn
	}
	if groupOn == "" {
		return ErrUsage("group_on is required")
	}
	if _, parseErr := types.ParseDate(groupOn); parseErr != nil {
		return ErrUsage("group_on must be in YYYY-MM-DD format")
	}

	body := map[string]any{
		"content":  req.Content,
		"group_on": groupOn,
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return err
	}
	resp, err := s.client.parent.gen.UpdateAnswerWithBodyWithResponse(ctx, s.client.accountID, answerID, "application/json", bodyReader)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// ListQuestionReminders returns pending check-in reminders for the current user.
//
// Reminders cover questions that are awaiting a response from the
// authenticated user, across all projects in the account.
//
// By default, returns all reminders (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of reminders to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination.
//     Use 0 to paginate through all results up to Limit.
//
// The returned QuestionReminderListResult includes pagination metadata
// (TotalCount from X-Total-Count header) when available.
func (s *CheckinsService) ListQuestionReminders(ctx context.Context, opts *QuestionReminderListOptions) (result *QuestionReminderListResult, err error) {
	op := OperationInfo{
		Service: "Checkins", Operation: "ListQuestionReminders",
		ResourceType: "question_reminder", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetQuestionRemindersParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.GetQuestionRemindersParams{Page: page}
	}

	resp, err := s.client.parent.gen.GetQuestionRemindersWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	var reminders []QuestionReminder
	if resp.JSON200 != nil {
		for _, gr := range *resp.JSON200 {
			reminders = append(reminders, questionReminderFromGenerated(gr))
		}
	}

	if opts != nil && opts.Page > 0 {
		return &QuestionReminderListResult{Reminders: reminders, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	limit := 0
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	if limit > 0 && len(reminders) >= limit {
		return &QuestionReminderListResult{Reminders: reminders[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(reminders), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(reminders), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var gr generated.QuestionReminder
		if err := json.Unmarshal(raw, &gr); err != nil {
			return nil, fmt.Errorf("failed to parse question reminder: %w", err)
		}
		reminders = append(reminders, questionReminderFromGenerated(gr))
	}

	return &QuestionReminderListResult{Reminders: reminders, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
		BookmarkURL:      deref(gq.BookmarkUrl),
		QuestionsURL:     deref(gq.QuestionsUrl),
		QuestionsCount:   int(deref(gq.QuestionsCount)),
		Name:             gq.Name,
	}

	if gq.Id != 0 {
		q.ID = gq.Id
	}

	if gq.Bucket.Id != 0 || gq.Bucket.Name != "" {
		q.Bucket = &Bucket{
			ID:   gq.Bucket.Id,
			Name: gq.Bucket.Name,
			Type: gq.Bucket.Type,
		}
	}

	if gq.Creator.Id != 0 || gq.Creator.Name != "" {
		creator := personFromGenerated(gq.Creator)
		q.Creator = &creator
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
		BookmarkURL:      deref(gq.BookmarkUrl),
		SubscriptionURL:  deref(gq.SubscriptionUrl),
		Paused:           deref(gq.Paused),
		AnswersCount:     int(deref(gq.AnswersCount)),
		AnswersURL:       deref(gq.AnswersUrl),
	}

	if gq.Id != 0 {
		q.ID = gq.Id
	}

	// Presence is the pointer: a schedule present without a frequency still
	// carries days/hour/dates, and dropping the whole wrapper loses them.
	if gq.Schedule != nil {
		var days []int
		if gq.Schedule.Days != nil {
			days = make([]int, len(*gq.Schedule.Days))
			for i, d := range *gq.Schedule.Days {
				days[i] = int(d)
			}
		}
		q.Schedule = &QuestionSchedule{
			Frequency: deref(gq.Schedule.Frequency),
			Days:      days,
			// Hour/Minute are *int on the SDK type precisely so an absent
			// value stays absent — carry the generated pointer's nil through
			// rather than manufacturing a pointer to zero.
			Hour:      intPtrFrom(gq.Schedule.Hour),
			Minute:    intPtrFrom(gq.Schedule.Minute),
			StartDate: deref(gq.Schedule.StartDate),
			EndDate:   deref(gq.Schedule.EndDate),
		}
		q.Schedule.WeekInstance = intPtrFrom(gq.Schedule.WeekInstance)
		q.Schedule.WeekInterval = intPtrFrom(gq.Schedule.WeekInterval)
		q.Schedule.MonthInterval = intPtrFrom(gq.Schedule.MonthInterval)
	}

	if gq.Parent.Id != 0 || gq.Parent.Title != "" {
		q.Parent = &Parent{
			ID:     gq.Parent.Id,
			Title:  gq.Parent.Title,
			Type:   gq.Parent.Type,
			URL:    gq.Parent.Url,
			AppURL: gq.Parent.AppUrl,
		}
	}

	if gq.Bucket.Id != 0 || gq.Bucket.Name != "" {
		q.Bucket = &Bucket{
			ID:   gq.Bucket.Id,
			Name: gq.Bucket.Name,
			Type: gq.Bucket.Type,
		}
	}

	if gq.Creator.Id != 0 || gq.Creator.Name != "" {
		creator := personFromGenerated(gq.Creator)
		q.Creator = &creator
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
		BookmarkURL:      deref(ga.BookmarkUrl),
		BoostsCount:      int(deref(ga.BoostsCount)),
		BoostsURL:        deref(ga.BoostsUrl),
		SubscriptionURL:  deref(ga.SubscriptionUrl),
		CommentsCount:    int(deref(ga.CommentsCount)),
		CommentsURL:      deref(ga.CommentsUrl),
		Content:          ga.Content,
	}

	if ga.Id != 0 {
		a.ID = ga.Id
	}

	// Convert date fields to strings
	if ga.GroupOn != nil && !ga.GroupOn.IsZero() {
		a.GroupOn = ga.GroupOn.String()
	}

	if ga.Parent.Id != 0 || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     ga.Parent.Id,
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}

	if ga.Bucket.Id != 0 || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   ga.Bucket.Id,
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if ga.Creator.Id != 0 || ga.Creator.Name != "" {
		creator := personFromGenerated(ga.Creator)
		a.Creator = &creator
	}

	a.ContentAttachments = richTextAttachmentsFromGenerated(ga.ContentAttachments)

	return a
}

// questionReminderFromGenerated converts a generated QuestionReminder to our clean type.
func questionReminderFromGenerated(gr generated.QuestionReminder) QuestionReminder {
	r := QuestionReminder{
		RemindAt:   deref(gr.RemindAt),
		ReminderID: gr.ReminderId,
	}

	if gr.Question != nil {
		r.Question = questionFromGenerated(*gr.Question)
	}

	// Convert date fields to strings
	if gr.GroupOn != nil && !gr.GroupOn.IsZero() {
		r.GroupOn = gr.GroupOn.String()
	}

	return r
}

// questionNotificationSettingsFromGenerated converts a generated
// UpdateQuestionNotificationSettingsResponseContent to our clean type.
func questionNotificationSettingsFromGenerated(gs generated.UpdateQuestionNotificationSettingsResponseContent) QuestionNotificationSettings {
	return QuestionNotificationSettings{
		Responding: deref(gs.Responding),
		Subscribed: deref(gs.Subscribed),
	}
}

// questionScheduleToMap converts a QuestionSchedule to a map for JSON marshaling.
// Used by CreateQuestion and UpdateQuestion to avoid the generated QuestionSchedule
// struct's zero-value serialization leaking empty fields.
func questionScheduleToMap(s *QuestionSchedule) map[string]any {
	m := map[string]any{}
	if s.Frequency != "" {
		m["frequency"] = s.Frequency
	}
	// nil means "not addressed" (omitted); a non-nil empty slice is an explicit
	// empty day list and must reach the wire.
	if s.Days != nil {
		m["days"] = s.Days
	}
	if s.Hour != nil {
		m["hour"] = *s.Hour
	}
	if s.Minute != nil {
		m["minute"] = *s.Minute
	}
	if s.StartDate != "" {
		m["start_date"] = s.StartDate
	}
	if s.EndDate != "" {
		m["end_date"] = s.EndDate
	}
	if s.WeekInstance != nil {
		m["week_instance"] = *s.WeekInstance
	}
	if s.WeekInterval != nil {
		m["week_interval"] = *s.WeekInterval
	}
	if s.MonthInterval != nil {
		m["month_interval"] = *s.MonthInterval
	}
	return m
}
