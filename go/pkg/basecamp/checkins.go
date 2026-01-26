package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Questionnaire represents a Basecamp automatic check-in questionnaire.
type Questionnaire struct {
	ID              int64     `json:"id"`
	Status          string    `json:"status"`
	VisibleToClients bool     `json:"visible_to_clients"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Title           string    `json:"title"`
	InheritsStatus  bool      `json:"inherits_status"`
	Type            string    `json:"type"`
	URL             string    `json:"url"`
	AppURL          string    `json:"app_url"`
	BookmarkURL     string    `json:"bookmark_url"`
	QuestionsURL    string    `json:"questions_url"`
	QuestionsCount  int       `json:"questions_count"`
	Name            string    `json:"name"`
	Bucket          *Bucket   `json:"bucket,omitempty"`
	Creator         *Person   `json:"creator,omitempty"`
}

// QuestionSchedule represents the schedule configuration for a question.
type QuestionSchedule struct {
	Frequency string `json:"frequency"`
	Days      []int  `json:"days"`
	Hour      int    `json:"hour"`
	Minute    int    `json:"minute"`
	StartDate string `json:"start_date,omitempty"`
}

// Question represents a Basecamp automatic check-in question.
type Question struct {
	ID              int64             `json:"id"`
	Status          string            `json:"status"`
	VisibleToClients bool             `json:"visible_to_clients"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Title           string            `json:"title"`
	InheritsStatus  bool              `json:"inherits_status"`
	Type            string            `json:"type"`
	URL             string            `json:"url"`
	AppURL          string            `json:"app_url"`
	BookmarkURL     string            `json:"bookmark_url"`
	SubscriptionURL string            `json:"subscription_url"`
	Parent          *Parent           `json:"parent,omitempty"`
	Bucket          *Bucket           `json:"bucket,omitempty"`
	Creator         *Person           `json:"creator,omitempty"`
	Paused          bool              `json:"paused"`
	Schedule        *QuestionSchedule `json:"schedule,omitempty"`
	AnswersCount    int               `json:"answers_count"`
	AnswersURL      string            `json:"answers_url"`
}

// QuestionAnswer represents an answer to a Basecamp check-in question.
type QuestionAnswer struct {
	ID              int64     `json:"id"`
	Status          string    `json:"status"`
	VisibleToClients bool     `json:"visible_to_clients"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Title           string    `json:"title"`
	InheritsStatus  bool      `json:"inherits_status"`
	Type            string    `json:"type"`
	URL             string    `json:"url"`
	AppURL          string    `json:"app_url"`
	BookmarkURL     string    `json:"bookmark_url"`
	SubscriptionURL string    `json:"subscription_url"`
	CommentsCount   int       `json:"comments_count"`
	CommentsURL     string    `json:"comments_url"`
	Content         string    `json:"content"`
	GroupOn         string    `json:"group_on"`
	Parent          *Parent   `json:"parent,omitempty"`
	Bucket          *Bucket   `json:"bucket,omitempty"`
	Creator         *Person   `json:"creator,omitempty"`
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

// CheckinsService handles automatic check-in operations.
type CheckinsService struct {
	client *Client
}

// NewCheckinsService creates a new CheckinsService.
func NewCheckinsService(client *Client) *CheckinsService {
	return &CheckinsService{client: client}
}

// GetQuestionnaire returns a questionnaire by ID.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
func (s *CheckinsService) GetQuestionnaire(ctx context.Context, bucketID, questionnaireID int64) (*Questionnaire, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/questionnaires/%d.json", bucketID, questionnaireID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var questionnaire Questionnaire
	if err := resp.UnmarshalData(&questionnaire); err != nil {
		return nil, fmt.Errorf("failed to parse questionnaire: %w", err)
	}

	return &questionnaire, nil
}

// ListQuestions returns all questions in a questionnaire.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
func (s *CheckinsService) ListQuestions(ctx context.Context, bucketID, questionnaireID int64) ([]Question, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/questionnaires/%d/questions.json", bucketID, questionnaireID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	questions := make([]Question, 0, len(results))
	for _, raw := range results {
		var q Question
		if err := json.Unmarshal(raw, &q); err != nil {
			return nil, fmt.Errorf("failed to parse question: %w", err)
		}
		questions = append(questions, q)
	}

	return questions, nil
}

// GetQuestion returns a question by ID.
// bucketID is the project ID, questionID is the question ID.
func (s *CheckinsService) GetQuestion(ctx context.Context, bucketID, questionID int64) (*Question, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/questions/%d.json", bucketID, questionID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var question Question
	if err := resp.UnmarshalData(&question); err != nil {
		return nil, fmt.Errorf("failed to parse question: %w", err)
	}

	return &question, nil
}

// CreateQuestion creates a new question in a questionnaire.
// bucketID is the project ID, questionnaireID is the questionnaire ID.
// Returns the created question.
func (s *CheckinsService) CreateQuestion(ctx context.Context, bucketID, questionnaireID int64, req *CreateQuestionRequest) (*Question, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("question title is required")
	}
	if req.Schedule == nil {
		return nil, ErrUsage("question schedule is required")
	}

	path := fmt.Sprintf("/buckets/%d/questionnaires/%d/questions.json", bucketID, questionnaireID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var question Question
	if err := resp.UnmarshalData(&question); err != nil {
		return nil, fmt.Errorf("failed to parse question: %w", err)
	}

	return &question, nil
}

// UpdateQuestion updates an existing question.
// bucketID is the project ID, questionID is the question ID.
// Returns the updated question.
func (s *CheckinsService) UpdateQuestion(ctx context.Context, bucketID, questionID int64, req *UpdateQuestionRequest) (*Question, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/questions/%d.json", bucketID, questionID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var question Question
	if err := resp.UnmarshalData(&question); err != nil {
		return nil, fmt.Errorf("failed to parse question: %w", err)
	}

	return &question, nil
}

// ListAnswers returns all answers for a question.
// bucketID is the project ID, questionID is the question ID.
func (s *CheckinsService) ListAnswers(ctx context.Context, bucketID, questionID int64) ([]QuestionAnswer, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/questions/%d/answers.json", bucketID, questionID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	answers := make([]QuestionAnswer, 0, len(results))
	for _, raw := range results {
		var a QuestionAnswer
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("failed to parse question answer: %w", err)
		}
		answers = append(answers, a)
	}

	return answers, nil
}

// GetAnswer returns a question answer by ID.
// bucketID is the project ID, answerID is the answer ID.
func (s *CheckinsService) GetAnswer(ctx context.Context, bucketID, answerID int64) (*QuestionAnswer, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/question_answers/%d.json", bucketID, answerID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var answer QuestionAnswer
	if err := resp.UnmarshalData(&answer); err != nil {
		return nil, fmt.Errorf("failed to parse question answer: %w", err)
	}

	return &answer, nil
}

// CreateAnswer creates a new answer for a question.
// bucketID is the project ID, questionID is the question ID.
// Returns the created answer.
func (s *CheckinsService) CreateAnswer(ctx context.Context, bucketID, questionID int64, req *CreateAnswerRequest) (*QuestionAnswer, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Content == "" {
		return nil, ErrUsage("answer content is required")
	}

	path := fmt.Sprintf("/buckets/%d/questions/%d/answers.json", bucketID, questionID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var answer QuestionAnswer
	if err := resp.UnmarshalData(&answer); err != nil {
		return nil, fmt.Errorf("failed to parse question answer: %w", err)
	}

	return &answer, nil
}
