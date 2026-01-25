package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Comment represents a Basecamp comment on a recording.
type Comment struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	Parent    *Parent   `json:"parent,omitempty"`
	Bucket    *Bucket   `json:"bucket,omitempty"`
	Creator   *Person   `json:"creator,omitempty"`
}

// CreateCommentRequest specifies the parameters for creating a comment.
type CreateCommentRequest struct {
	// Content is the comment text in HTML (required).
	Content string `json:"content"`
}

// UpdateCommentRequest specifies the parameters for updating a comment.
type UpdateCommentRequest struct {
	// Content is the comment text in HTML (required).
	Content string `json:"content"`
}

// CommentsService handles comment operations.
type CommentsService struct {
	client *Client
}

// NewCommentsService creates a new CommentsService.
func NewCommentsService(client *Client) *CommentsService {
	return &CommentsService{client: client}
}

// List returns all comments on a recording.
// bucketID is the project ID, recordingID is the ID of the recording (todo, message, etc.).
func (s *CommentsService) List(ctx context.Context, bucketID, recordingID int64) ([]Comment, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/comments.json", bucketID, recordingID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	comments := make([]Comment, 0, len(results))
	for _, raw := range results {
		var c Comment
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("failed to parse comment: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, nil
}

// Get returns a comment by ID.
// bucketID is the project ID, commentID is the comment ID.
func (s *CommentsService) Get(ctx context.Context, bucketID, commentID int64) (*Comment, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/comments/%d.json", bucketID, commentID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := resp.UnmarshalData(&comment); err != nil {
		return nil, fmt.Errorf("failed to parse comment: %w", err)
	}

	return &comment, nil
}

// Create creates a new comment on a recording.
// bucketID is the project ID, recordingID is the ID of the recording to comment on.
// Returns the created comment.
func (s *CommentsService) Create(ctx context.Context, bucketID, recordingID int64, req *CreateCommentRequest) (*Comment, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Content == "" {
		return nil, ErrUsage("comment content is required")
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/comments.json", bucketID, recordingID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := resp.UnmarshalData(&comment); err != nil {
		return nil, fmt.Errorf("failed to parse comment: %w", err)
	}

	return &comment, nil
}

// Update updates an existing comment.
// bucketID is the project ID, commentID is the comment ID.
// Returns the updated comment.
func (s *CommentsService) Update(ctx context.Context, bucketID, commentID int64, req *UpdateCommentRequest) (*Comment, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Content == "" {
		return nil, ErrUsage("comment content is required")
	}

	path := fmt.Sprintf("/buckets/%d/comments/%d.json", bucketID, commentID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := resp.UnmarshalData(&comment); err != nil {
		return nil, fmt.Errorf("failed to parse comment: %w", err)
	}

	return &comment, nil
}
