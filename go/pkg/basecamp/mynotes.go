package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// MyNote is the authenticated user's per-person notebook note (wire type
// Notebook::Note). Before the first write, ID/CreatedAt/UpdatedAt are nil and
// Content is empty — the record is created on first update.
type MyNote struct {
	ID        *int64     `json:"id"`
	Type      string     `json:"type"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	Content   string     `json:"content"`
	// ContentAttachments carries any bc-attachment references embedded in the
	// rich-text content.
	ContentAttachments []RichTextAttachment `json:"content_attachments"`
	URL                string               `json:"url"`
	AppURL             string               `json:"app_url"`
}

// MyNotesService reads and writes the current user's scratchpad note.
type MyNotesService struct {
	client *AccountClient
}

// NewMyNotesService creates a new MyNotesService.
func NewMyNotesService(client *AccountClient) *MyNotesService {
	return &MyNotesService{client: client}
}

// Get returns the authenticated user's note. Pre-first-write, ID and the
// timestamps are nil and Content is empty.
func (s *MyNotesService) Get(ctx context.Context) (result *MyNote, err error) {
	op := OperationInfo{
		Service: "MyNotes", Operation: "Get",
		ResourceType: "my_note", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMyNoteWithResponse(ctx, s.client.accountID)
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

	note := myNoteFromGenerated(*resp.JSON200)
	return &note, nil
}

// Update replaces the note's content (recording a new server-side revision);
// the first update creates the underlying notebook. Returns the updated note.
func (s *MyNotesService) Update(ctx context.Context, content string) (result *MyNote, err error) {
	op := OperationInfo{
		Service: "MyNotes", Operation: "Update",
		ResourceType: "my_note", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.UpdateMyNoteJSONRequestBody{
		Note: generated.MyNoteAttributes{Content: content},
	}
	resp, err := s.client.parent.gen.UpdateMyNoteWithResponse(ctx, s.client.accountID, body)
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

	note := myNoteFromGenerated(*resp.JSON200)
	return &note, nil
}

func myNoteFromGenerated(gn generated.MyNote) MyNote {
	return MyNote{
		ID:                 gn.Id,
		Type:               gn.Type,
		CreatedAt:          gn.CreatedAt,
		UpdatedAt:          gn.UpdatedAt,
		Content:            gn.Content,
		ContentAttachments: richTextAttachmentsFromGenerated(gn.ContentAttachments),
		URL:                gn.Url,
		AppURL:             gn.AppUrl,
	}
}
