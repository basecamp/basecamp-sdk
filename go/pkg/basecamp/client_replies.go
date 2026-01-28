package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// ClientReply represents a reply to a client correspondence or approval.
type ClientReply struct {
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
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
}

// ClientRepliesService handles client reply operations.
type ClientRepliesService struct {
	client *AccountClient
}

// NewClientRepliesService creates a new ClientRepliesService.
func NewClientRepliesService(client *AccountClient) *ClientRepliesService {
	return &ClientRepliesService{client: client}
}

// List returns all replies for a client recording (correspondence or approval).
// bucketID is the project ID, recordingID is the parent correspondence/approval ID.
func (s *ClientRepliesService) List(ctx context.Context, bucketID, recordingID int64) (result []ClientReply, err error) {
	op := OperationInfo{
		Service: "ClientReplies", Operation: "List",
		ResourceType: "client_reply", IsMutation: false,
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

	resp, err := s.client.gen.ListClientRepliesWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	replies := make([]ClientReply, 0, len(*resp.JSON200))
	for _, gr := range *resp.JSON200 {
		replies = append(replies, clientReplyFromGenerated(gr))
	}

	return replies, nil
}

// Get returns a specific client reply.
// bucketID is the project ID, recordingID is the parent correspondence/approval ID,
// replyID is the client reply ID.
func (s *ClientRepliesService) Get(ctx context.Context, bucketID, recordingID, replyID int64) (result *ClientReply, err error) {
	op := OperationInfo{
		Service: "ClientReplies", Operation: "Get",
		ResourceType: "client_reply", IsMutation: false,
		BucketID: bucketID, ResourceID: replyID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetClientReplyWithResponse(ctx, s.client.accountID, bucketID, recordingID, replyID)
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

	reply := clientReplyFromGenerated(resp.JSON200.Reply)
	return &reply, nil
}

// clientReplyFromGenerated converts a generated ClientReply to our clean type.
func clientReplyFromGenerated(gr generated.ClientReply) ClientReply {
	r := ClientReply{
		Status:           gr.Status,
		VisibleToClients: gr.VisibleToClients,
		CreatedAt:        gr.CreatedAt,
		UpdatedAt:        gr.UpdatedAt,
		Title:            gr.Title,
		InheritsStatus:   gr.InheritsStatus,
		Type:             gr.Type,
		URL:              gr.Url,
		AppURL:           gr.AppUrl,
		BookmarkURL:      gr.BookmarkUrl,
		Content:          gr.Content,
	}

	if gr.Id != nil {
		r.ID = *gr.Id
	}

	if gr.Parent.Id != nil || gr.Parent.Title != "" {
		r.Parent = &Parent{
			ID:     derefInt64(gr.Parent.Id),
			Title:  gr.Parent.Title,
			Type:   gr.Parent.Type,
			URL:    gr.Parent.Url,
			AppURL: gr.Parent.AppUrl,
		}
	}

	if gr.Bucket.Id != nil || gr.Bucket.Name != "" {
		r.Bucket = &Bucket{
			ID:   derefInt64(gr.Bucket.Id),
			Name: gr.Bucket.Name,
			Type: gr.Bucket.Type,
		}
	}

	if gr.Creator.Id != nil || gr.Creator.Name != "" {
		r.Creator = &Person{
			ID:           derefInt64(gr.Creator.Id),
			Name:         gr.Creator.Name,
			EmailAddress: gr.Creator.EmailAddress,
			AvatarURL:    gr.Creator.AvatarUrl,
			Admin:        gr.Creator.Admin,
			Owner:        gr.Creator.Owner,
		}
	}

	return r
}
