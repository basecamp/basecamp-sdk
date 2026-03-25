package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// DefaultCampfireLineLimit is the default number of campfire lines to return when no limit is specified.
const DefaultCampfireLineLimit = 100

// DefaultCampfireUploadLimit is the default number of campfire uploads to return when no limit is specified.
const DefaultCampfireUploadLimit = 100

// CampfireListOptions specifies options for listing campfires.
type CampfireListOptions struct {
	// Limit is the maximum number of campfires to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// CampfireLineListOptions specifies options for listing campfire lines.
type CampfireLineListOptions struct {
	// Sort field: "created_at" or "updated_at".
	Sort string

	// Direction: "asc" (oldest first, default) or "desc" (newest first).
	Direction string

	// Limit is the maximum number of lines to return.
	// If 0, uses DefaultCampfireLineLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// CampfireUploadListOptions specifies options for listing campfire uploads.
type CampfireUploadListOptions struct {
	// Sort field: "created_at" or "updated_at".
	Sort string

	// Direction: "asc" (oldest first, default) or "desc" (newest first).
	Direction string

	// Limit is the maximum number of uploads to return.
	// If 0, uses DefaultCampfireUploadLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// Campfire represents a Basecamp Campfire (real-time chat room).
type Campfire struct {
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
	LinesURL         string    `json:"lines_url"`
	FilesURL         string    `json:"files_url,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	BoostsCount      int       `json:"boosts_count,omitempty"`
}

// CampfireLine represents a message in a Campfire chat.
type CampfireLine struct {
	ID               int64                    `json:"id"`
	Status           string                   `json:"status"`
	VisibleToClients bool                     `json:"visible_to_clients"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	Title            string                   `json:"title"`
	InheritsStatus   bool                     `json:"inherits_status"`
	Type             string                   `json:"type"`
	URL              string                   `json:"url"`
	AppURL           string                   `json:"app_url"`
	Content          string                   `json:"content,omitempty"`
	Attachments      []CampfireLineAttachment `json:"attachments,omitempty"`
	Parent           *Parent                  `json:"parent,omitempty"`
	Bucket           *Bucket                  `json:"bucket,omitempty"`
	Creator          *Person                  `json:"creator,omitempty"`
	BoostsCount      int                      `json:"boosts_count,omitempty"`
}

// CampfireLineAttachment represents a file attached to an upload line.
type CampfireLineAttachment struct {
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	ByteSize    int64  `json:"byte_size,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// Line content type constants for campfire messages.
const (
	// LineContentTypePlain sends the line as plain text (the default when omitted).
	LineContentTypePlain = "text/plain"
	// LineContentTypeHTML sends the line as rich HTML content.
	LineContentTypeHTML = "text/html"
)

// CreateCampfireLineRequest specifies the parameters for creating a campfire line.
type CreateCampfireLineRequest struct {
	// Content is the message body (required).
	Content string `json:"content"`
	// ContentType is "text/plain" or "text/html". If empty, the API defaults to plain text.
	ContentType string `json:"content_type,omitempty"`
}

// CreateLineOptions specifies optional parameters for creating a campfire line.
type CreateLineOptions struct {
	// ContentType is "text/plain" or "text/html". If empty, the API defaults to plain text.
	ContentType string
}

// Chatbot represents a Basecamp chatbot integration.
type Chatbot struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ServiceName string    `json:"service_name"`
	CommandURL  string    `json:"command_url,omitempty"`
	URL         string    `json:"url"`
	AppURL      string    `json:"app_url"`
	LinesURL    string    `json:"lines_url"`
}

// CreateChatbotRequest specifies the parameters for creating a chatbot.
type CreateChatbotRequest struct {
	// ServiceName is the chatbot name used to invoke queries and commands (required).
	// No spaces, emoji or non-word characters are allowed.
	ServiceName string `json:"service_name"`
	// CommandURL is the HTTPS URL that Basecamp should call when the bot is addressed (optional).
	CommandURL string `json:"command_url,omitempty"`
}

// UpdateChatbotRequest specifies the parameters for updating a chatbot.
type UpdateChatbotRequest struct {
	// ServiceName is the chatbot name used to invoke queries and commands (required).
	// No spaces, emoji or non-word characters are allowed.
	ServiceName string `json:"service_name"`
	// CommandURL is the HTTPS URL that Basecamp should call when the bot is addressed (optional).
	CommandURL string `json:"command_url,omitempty"`
}

// CampfireListResult contains the results from listing campfires.
type CampfireListResult struct {
	// Campfires is the list of campfires returned.
	Campfires []Campfire
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// CampfireLineListResult contains the results from listing campfire lines.
type CampfireLineListResult struct {
	// Lines is the list of campfire lines returned.
	Lines []CampfireLine
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// CampfiresService handles campfire operations.
type CampfiresService struct {
	client *AccountClient
}

// NewCampfiresService creates a new CampfiresService.
func NewCampfiresService(client *AccountClient) *CampfiresService {
	return &CampfiresService{client: client}
}

// List returns all campfires across the account.
//
// Pagination options:
//   - Limit: maximum number of campfires to return (0 = all, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned CampfireListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CampfiresService) List(ctx context.Context, opts *CampfireListOptions) (result *CampfireListResult, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "List",
		ResourceType: "campfire", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListCampfiresWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var campfires []Campfire
	if resp.JSON200 != nil {
		for _, gc := range *resp.JSON200 {
			campfires = append(campfires, campfireFromGenerated(gc))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &CampfireListResult{Campfires: campfires, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (no limit)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(campfires) >= limit {
		return &CampfireListResult{Campfires: campfires[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(campfires), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(campfires), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gc generated.Campfire
		if err := json.Unmarshal(raw, &gc); err != nil {
			return nil, fmt.Errorf("failed to parse campfire: %w", err)
		}
		campfires = append(campfires, campfireFromGenerated(gc))
	}

	return &CampfireListResult{Campfires: campfires, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a campfire by ID.
func (s *CampfiresService) Get(ctx context.Context, campfireID int64) (result *Campfire, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "Get",
		ResourceType: "campfire", IsMutation: false,
		ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCampfireWithResponse(ctx, s.client.accountID, campfireID)
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

	campfire := campfireFromGenerated(*resp.JSON200)
	return &campfire, nil
}

// ListLines returns all lines (messages) in a campfire.
//
// By default, returns up to 100 lines. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of lines to return (0 = 100, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned CampfireLineListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CampfiresService) ListLines(ctx context.Context, campfireID int64, opts *CampfireLineListOptions) (result *CampfireLineListResult, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "ListLines",
		ResourceType: "campfire_line", IsMutation: false,
		ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListCampfireLinesParams
	if opts != nil && (opts.Sort != "" || opts.Direction != "") {
		params = &generated.ListCampfireLinesParams{
			Sort:      opts.Sort,
			Direction: opts.Direction,
		}
	}

	resp, err := s.client.parent.gen.ListCampfireLinesWithResponse(ctx, s.client.accountID, campfireID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var lines []CampfireLine
	if resp.JSON200 != nil {
		for _, gl := range *resp.JSON200 {
			lines = append(lines, campfireLineFromGenerated(gl))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &CampfireLineListResult{Lines: lines, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultCampfireLineLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(lines) >= limit {
		return &CampfireLineListResult{Lines: lines[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(lines), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(lines), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gl generated.CampfireLine
		if err := json.Unmarshal(raw, &gl); err != nil {
			return nil, fmt.Errorf("failed to parse campfire line: %w", err)
		}
		lines = append(lines, campfireLineFromGenerated(gl))
	}

	return &CampfireLineListResult{Lines: lines, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetLine returns a single line (message) from a campfire.
func (s *CampfiresService) GetLine(ctx context.Context, campfireID, lineID int64) (result *CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "GetLine",
		ResourceType: "campfire_line", IsMutation: false,
		ResourceID: lineID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCampfireLineWithResponse(ctx, s.client.accountID, campfireID, lineID)
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

	line := campfireLineFromGenerated(*resp.JSON200)
	return &line, nil
}

// CreateLine creates a new line (message) in a campfire.
// opts is optional; pass a CreateLineOptions to set content_type (text/html or text/plain).
// Returns the created line.
func (s *CampfiresService) CreateLine(ctx context.Context, campfireID int64, content string, opts ...*CreateLineOptions) (result *CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "CreateLine",
		ResourceType: "campfire_line", IsMutation: true,
		ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if len(opts) > 1 {
		err = ErrUsage("CreateLine accepts at most one CreateLineOptions argument")
		return nil, err
	}

	if content == "" {
		err = ErrUsage("campfire line content is required")
		return nil, err
	}

	body := generated.CreateCampfireLineJSONRequestBody{
		Content: content,
	}
	if len(opts) > 0 && opts[0] != nil && opts[0].ContentType != "" {
		switch opts[0].ContentType {
		case LineContentTypePlain, LineContentTypeHTML:
			body.ContentType = opts[0].ContentType
		default:
			err = ErrUsage("content_type must be \"text/plain\" or \"text/html\"")
			return nil, err
		}
	}

	resp, err := s.client.parent.gen.CreateCampfireLineWithResponse(ctx, s.client.accountID, campfireID, body)
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

	line := campfireLineFromGenerated(*resp.JSON201)
	return &line, nil
}

// DeleteLine deletes a line (message) from a campfire.
func (s *CampfiresService) DeleteLine(ctx context.Context, campfireID, lineID int64) (err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "DeleteLine",
		ResourceType: "campfire_line", IsMutation: true,
		ResourceID: lineID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteCampfireLineWithResponse(ctx, s.client.accountID, campfireID, lineID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// ListUploads returns all uploaded files in a campfire.
//
// By default, returns up to 100 uploads. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of uploads to return (0 = 100, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned CampfireLineListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CampfiresService) ListUploads(ctx context.Context, campfireID int64, opts *CampfireUploadListOptions) (result *CampfireLineListResult, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "ListUploads",
		ResourceType: "campfire_line", IsMutation: false,
		ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var uploadParams *generated.ListCampfireUploadsParams
	if opts != nil && (opts.Sort != "" || opts.Direction != "") {
		uploadParams = &generated.ListCampfireUploadsParams{
			Sort:      opts.Sort,
			Direction: opts.Direction,
		}
	}

	resp, err := s.client.parent.gen.ListCampfireUploadsWithResponse(ctx, s.client.accountID, campfireID, uploadParams)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var lines []CampfireLine
	if resp.JSON200 != nil {
		for _, gl := range *resp.JSON200 {
			lines = append(lines, campfireLineFromGenerated(gl))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &CampfireLineListResult{Lines: lines, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultCampfireUploadLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(lines) >= limit {
		return &CampfireLineListResult{Lines: lines[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(lines), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(lines), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gl generated.CampfireLine
		if err := json.Unmarshal(raw, &gl); err != nil {
			return nil, fmt.Errorf("failed to parse campfire upload: %w", err)
		}
		lines = append(lines, campfireLineFromGenerated(gl))
	}

	return &CampfireLineListResult{Lines: lines, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// CreateUpload uploads a file to a campfire.
// filename is the name of the file, contentType is the MIME type (e.g., "image/png"),
// and data is the raw file content. Returns the created upload line.
func (s *CampfiresService) CreateUpload(ctx context.Context, campfireID int64, filename, contentType string, data io.Reader) (result *CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "CreateUpload",
		ResourceType: "campfire_line", IsMutation: true,
		ResourceID: campfireID,
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

	if data == nil {
		err = ErrUsage("file data is required")
		return nil, err
	}

	body, err := io.ReadAll(data)
	if err != nil {
		err = fmt.Errorf("failed to read file data: %w", err)
		return nil, err
	}

	if len(body) == 0 {
		err = ErrUsage("file data is required")
		return nil, err
	}

	params := &generated.CreateCampfireUploadParams{
		Name: filename,
	}

	resp, err := s.client.parent.gen.CreateCampfireUploadWithBodyWithResponse(ctx, s.client.accountID, campfireID, params, contentType, bytes.NewReader(body))
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

	line := campfireLineFromGenerated(*resp.JSON201)
	return &line, nil
}

// ChatbotListOptions specifies options for listing chatbots.
type ChatbotListOptions struct {
	// Limit is the maximum number of chatbots to return.
	// If 0 (default), returns all chatbots.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// ChatbotListResult contains the results from listing chatbots.
type ChatbotListResult struct {
	// Chatbots is the list of chatbots returned.
	Chatbots []Chatbot
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ListChatbots returns all chatbots for a campfire.
// Note: Chatbots are account-wide but with basecamp-specific callback URLs.
//
// Pagination options:
//   - Limit: maximum number of chatbots to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned ChatbotListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CampfiresService) ListChatbots(ctx context.Context, campfireID int64, opts *ChatbotListOptions) (result *ChatbotListResult, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "ListChatbots",
		ResourceType: "chatbot", IsMutation: false,
		ResourceID: campfireID,
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
	resp, err := s.client.parent.gen.ListChatbotsWithResponse(ctx, s.client.accountID, campfireID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var chatbots []Chatbot
	if resp.JSON200 != nil {
		for _, gc := range *resp.JSON200 {
			chatbots = append(chatbots, chatbotFromGenerated(gc))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ChatbotListResult{Chatbots: chatbots, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for chatbots)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(chatbots) >= limit {
		return &ChatbotListResult{Chatbots: chatbots[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(chatbots), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(chatbots), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gc generated.Chatbot
		if err := json.Unmarshal(raw, &gc); err != nil {
			return nil, fmt.Errorf("failed to parse chatbot: %w", err)
		}
		chatbots = append(chatbots, chatbotFromGenerated(gc))
	}

	return &ChatbotListResult{Chatbots: chatbots, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetChatbot returns a chatbot by ID.
func (s *CampfiresService) GetChatbot(ctx context.Context, campfireID, chatbotID int64) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "GetChatbot",
		ResourceType: "chatbot", IsMutation: false,
		ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetChatbotWithResponse(ctx, s.client.accountID, campfireID, chatbotID)
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

	chatbot := chatbotFromGenerated(*resp.JSON200)
	return &chatbot, nil
}

// CreateChatbot creates a new chatbot for a campfire.
// Note: Chatbots are account-wide and can only be managed by administrators.
// Returns the created chatbot with its lines_url for posting.
func (s *CampfiresService) CreateChatbot(ctx context.Context, campfireID int64, req *CreateChatbotRequest) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "CreateChatbot",
		ResourceType: "chatbot", IsMutation: true,
		ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.ServiceName == "" {
		err = ErrUsage("chatbot service_name is required")
		return nil, err
	}

	body := generated.CreateChatbotJSONRequestBody{
		ServiceName: req.ServiceName,
	}
	if req.CommandURL != "" {
		body.CommandUrl = req.CommandURL
	}

	resp, err := s.client.parent.gen.CreateChatbotWithResponse(ctx, s.client.accountID, campfireID, body)
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

	chatbot := chatbotFromGenerated(*resp.JSON201)
	return &chatbot, nil
}

// UpdateChatbot updates an existing chatbot.
// Note: Updates to chatbots are account-wide.
// Returns the updated chatbot.
func (s *CampfiresService) UpdateChatbot(ctx context.Context, campfireID, chatbotID int64, req *UpdateChatbotRequest) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "UpdateChatbot",
		ResourceType: "chatbot", IsMutation: true,
		ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.ServiceName == "" {
		err = ErrUsage("chatbot service_name is required")
		return nil, err
	}

	body := generated.UpdateChatbotJSONRequestBody{
		ServiceName: req.ServiceName,
	}
	if req.CommandURL != "" {
		body.CommandUrl = req.CommandURL
	}

	resp, err := s.client.parent.gen.UpdateChatbotWithResponse(ctx, s.client.accountID, campfireID, chatbotID, body)
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

	chatbot := chatbotFromGenerated(*resp.JSON200)
	return &chatbot, nil
}

// DeleteChatbot deletes a chatbot.
// Note: Deleting a chatbot removes it from the entire account.
func (s *CampfiresService) DeleteChatbot(ctx context.Context, campfireID, chatbotID int64) (err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "DeleteChatbot",
		ResourceType: "chatbot", IsMutation: true,
		ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteChatbotWithResponse(ctx, s.client.accountID, campfireID, chatbotID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// campfireFromGenerated converts a generated Campfire to our clean Campfire type.
func campfireFromGenerated(gc generated.Campfire) Campfire {
	c := Campfire{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		LinesURL:         gc.LinesUrl,
		FilesURL:         gc.FilesUrl,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	c.ID = gc.Id

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           int64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	return c
}

// campfireLineFromGenerated converts a generated CampfireLine to our clean CampfireLine type.
func campfireLineFromGenerated(gl generated.CampfireLine) CampfireLine {
	l := CampfireLine{
		Status:           gl.Status,
		VisibleToClients: gl.VisibleToClients,
		Title:            gl.Title,
		InheritsStatus:   gl.InheritsStatus,
		Type:             gl.Type,
		URL:              gl.Url,
		AppURL:           gl.AppUrl,
		Content:          gl.Content,
		CreatedAt:        gl.CreatedAt,
		UpdatedAt:        gl.UpdatedAt,
		BoostsCount:      int(gl.BoostsCount),
	}

	l.ID = gl.Id

	if len(gl.Attachments) > 0 {
		l.Attachments = make([]CampfireLineAttachment, len(gl.Attachments))
		for i, ga := range gl.Attachments {
			l.Attachments[i] = CampfireLineAttachment{
				Title:       ga.Title,
				URL:         ga.Url,
				Filename:    ga.Filename,
				ContentType: ga.ContentType,
				ByteSize:    ga.ByteSize,
				DownloadURL: ga.DownloadUrl,
			}
		}
	}

	if gl.Parent.Id != 0 || gl.Parent.Title != "" {
		l.Parent = &Parent{
			ID:     gl.Parent.Id,
			Title:  gl.Parent.Title,
			Type:   gl.Parent.Type,
			URL:    gl.Parent.Url,
			AppURL: gl.Parent.AppUrl,
		}
	}

	if gl.Bucket.Id != 0 || gl.Bucket.Name != "" {
		l.Bucket = &Bucket{
			ID:   gl.Bucket.Id,
			Name: gl.Bucket.Name,
			Type: gl.Bucket.Type,
		}
	}

	if gl.Creator.Id != 0 || gl.Creator.Name != "" {
		l.Creator = &Person{
			ID:           int64(gl.Creator.Id),
			Name:         gl.Creator.Name,
			EmailAddress: gl.Creator.EmailAddress,
			AvatarURL:    gl.Creator.AvatarUrl,
			Admin:        gl.Creator.Admin,
			Owner:        gl.Creator.Owner,
		}
	}

	return l
}

// chatbotFromGenerated converts a generated Chatbot to our clean Chatbot type.
func chatbotFromGenerated(gc generated.Chatbot) Chatbot {
	c := Chatbot{
		ServiceName: gc.ServiceName,
		CommandURL:  gc.CommandUrl,
		URL:         gc.Url,
		AppURL:      gc.AppUrl,
		LinesURL:    gc.LinesUrl,
		CreatedAt:   gc.CreatedAt,
		UpdatedAt:   gc.UpdatedAt,
	}

	c.ID = gc.Id

	return c
}
