package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	client *AccountClient
}

// NewCommentsService creates a new CommentsService.
func NewCommentsService(client *AccountClient) *CommentsService {
	return &CommentsService{client: client}
}

// List returns all comments on a recording.
// bucketID is the project ID, recordingID is the ID of the recording (todo, message, etc.).
func (s *CommentsService) List(ctx context.Context, bucketID, recordingID int64) (result []Comment, err error) {
	op := OperationInfo{
		Service: "Comments", Operation: "List",
		ResourceType: "comment", IsMutation: false,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListCommentsWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	// Convert generated types to our clean types
	comments := make([]Comment, 0, len(*resp.JSON200))
	for _, gc := range *resp.JSON200 {
		comments = append(comments, commentFromGenerated(gc))
	}
	return comments, nil
}

// Get returns a comment by ID.
// bucketID is the project ID, commentID is the comment ID.
func (s *CommentsService) Get(ctx context.Context, bucketID, commentID int64) (result *Comment, err error) {
	op := OperationInfo{
		Service: "Comments", Operation: "Get",
		ResourceType: "comment", IsMutation: false,
		BucketID: bucketID, ResourceID: commentID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCommentWithResponse(ctx, s.client.accountID, bucketID, commentID)
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

	comment := commentFromGenerated(*resp.JSON200)
	return &comment, nil
}

// Create creates a new comment on a recording.
// bucketID is the project ID, recordingID is the ID of the recording to comment on.
// Returns the created comment.
func (s *CommentsService) Create(ctx context.Context, bucketID, recordingID int64, req *CreateCommentRequest) (result *Comment, err error) {
	op := OperationInfo{
		Service: "Comments", Operation: "Create",
		ResourceType: "comment", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
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
		err = ErrUsage("comment content is required")
		return nil, err
	}

	body := generated.CreateCommentJSONRequestBody{
		Content: req.Content,
	}

	resp, err := s.client.parent.gen.CreateCommentWithResponse(ctx, s.client.accountID, bucketID, recordingID, body)
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

	comment := commentFromGenerated(resp.JSON200.Comment)
	return &comment, nil
}

// Update updates an existing comment.
// bucketID is the project ID, commentID is the comment ID.
// Returns the updated comment.
func (s *CommentsService) Update(ctx context.Context, bucketID, commentID int64, req *UpdateCommentRequest) (result *Comment, err error) {
	op := OperationInfo{
		Service: "Comments", Operation: "Update",
		ResourceType: "comment", IsMutation: true,
		BucketID: bucketID, ResourceID: commentID,
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
		err = ErrUsage("comment content is required")
		return nil, err
	}

	body := generated.UpdateCommentJSONRequestBody{
		Content: req.Content,
	}

	resp, err := s.client.parent.gen.UpdateCommentWithResponse(ctx, s.client.accountID, bucketID, commentID, body)
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

	comment := commentFromGenerated(resp.JSON200.Comment)
	return &comment, nil
}

// Trash moves a comment to the trash.
// bucketID is the project ID, commentID is the comment ID.
// Trashed comments can be recovered from the trash.
func (s *CommentsService) Trash(ctx context.Context, bucketID, commentID int64) (err error) {
	op := OperationInfo{
		Service: "Comments", Operation: "Trash",
		ResourceType: "comment", IsMutation: true,
		BucketID: bucketID, ResourceID: commentID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, bucketID, commentID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Note: Permanent deletion of comments is not supported by the Basecamp API.
// Use Trash() to move comments to trash (recoverable via the web UI).

// commentFromGenerated converts a generated Comment to our clean Comment type.
func commentFromGenerated(gc generated.Comment) Comment {
	c := Comment{
		Status:    gc.Status,
		Content:   gc.Content,
		Type:      gc.Type,
		URL:       gc.Url,
		AppURL:    gc.AppUrl,
		CreatedAt: gc.CreatedAt,
		UpdatedAt: gc.UpdatedAt,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
	}

	// Convert nested types
	if gc.Parent.Id != nil || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     derefInt64(gc.Parent.Id),
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != nil || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   derefInt64(gc.Bucket.Id),
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != nil || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           derefInt64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	return c
}
