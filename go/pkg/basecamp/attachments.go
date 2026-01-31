package basecamp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// AttachmentResponse represents the response from creating an attachment.
type AttachmentResponse struct {
	AttachableSGID string `json:"attachable_sgid"`
}

// AttachmentsService handles attachment upload operations.
type AttachmentsService struct {
	client *AccountClient
}

// NewAttachmentsService creates a new AttachmentsService.
func NewAttachmentsService(client *AccountClient) *AttachmentsService {
	return &AttachmentsService{client: client}
}

// Create uploads a file and returns an attachable_sgid for embedding in rich text.
// filename is the name of the file, contentType is the MIME type (e.g., "image/png"),
// and data is the raw file content.
func (s *AttachmentsService) Create(ctx context.Context, filename, contentType string, data io.Reader) (result *AttachmentResponse, err error) {
	op := OperationInfo{
		Service: "Attachments", Operation: "Create",
		ResourceType: "attachment", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if filename == "" {
		err = ErrUsage("filename is required")
		return nil, err
	}
	if contentType == "" {
		err = ErrUsage("content type is required")
		return nil, err
	}

	// Read all data to get content length
	body, err := io.ReadAll(data)
	if err != nil {
		err = fmt.Errorf("failed to read file data: %w", err)
		return nil, err
	}

	if len(body) == 0 {
		err = ErrUsage("file data is required")
		return nil, err
	}

	params := &generated.CreateAttachmentParams{
		Name: filename,
	}

	resp, err := s.client.parent.gen.CreateAttachmentWithBodyWithResponse(ctx, s.client.accountID, params, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	return &AttachmentResponse{
		AttachableSGID: resp.JSON201.AttachableSgid,
	}, nil
}
