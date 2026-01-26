package basecamp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	// Build URL with query parameter for filename (URL-encoded)
	path := fmt.Sprintf("/attachments.json?name=%s", url.QueryEscape(filename))
	url := s.client.buildURL(path)

	// Get access token
	token, err := s.client.tokenProvider.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Create request with raw binary body
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", s.client.userAgent)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))

	s.client.logger.Debug("http request", "method", "POST", "url", url)

	// Execute request
	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	s.client.logger.Debug("http response", "status", resp.StatusCode)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusCreated {
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("attachment upload failed: %s", string(respBody)))
	}

	// Parse response
	apiResp := &Response{
		Data:       respBody,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}

	var attachment AttachmentResponse
	if err := apiResp.UnmarshalData(&attachment); err != nil {
		return nil, fmt.Errorf("failed to parse attachment response: %w", err)
	}

	return &attachment, nil
}
