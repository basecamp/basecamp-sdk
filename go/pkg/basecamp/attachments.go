package basecamp

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// AttachmentResponse represents the response from creating an attachment.
type AttachmentResponse struct {
	AttachableSGID string `json:"attachable_sgid"`
}

// AttachmentsService handles attachment upload operations.
type AttachmentsService struct {
	client *Client
}

// NewAttachmentsService creates a new AttachmentsService.
func NewAttachmentsService(client *Client) *AttachmentsService {
	return &AttachmentsService{client: client}
}

// Create uploads a file and returns an attachable_sgid for embedding in rich text.
// filename is the name of the file, contentType is the MIME type (e.g., "image/png"),
// and data is the raw file content.
func (s *AttachmentsService) Create(ctx context.Context, filename, contentType string, data io.Reader) (*AttachmentResponse, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if filename == "" {
		return nil, ErrUsage("filename is required")
	}
	if contentType == "" {
		return nil, ErrUsage("content type is required")
	}

	// Read all data to get content length
	body, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if len(body) == 0 {
		return nil, ErrUsage("file data is required")
	}

	params := &generated.CreateAttachmentParams{
		Name: filename,
	}

	resp, err := s.client.gen.CreateAttachmentWithBodyWithResponse(ctx, params, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	return &AttachmentResponse{
		AttachableSGID: resp.JSON200.AttachableSgid,
	}, nil
}
