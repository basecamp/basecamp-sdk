package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

type Embed struct {
	URL         string `json:"url"`
	Status      string `json:"status"`
	SGID        string `json:"sgid"`
	Type        string `json:"type,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	Content     string `json:"content,omitempty"`
}

type EmbedsService struct {
	client *AccountClient
}

func NewEmbedsService(client *AccountClient) *EmbedsService {
	return &EmbedsService{client: client}
}

func (s *EmbedsService) Create(ctx context.Context, url string) (result *Embed, err error) {
	op := OperationInfo{
		Service: "Embeds", Operation: "Create",
		ResourceType: "embed", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if url == "" {
		err = ErrUsage("embed url is required")
		return nil, err
	}

	resp, err := s.client.parent.gen.CreateEmbedWithResponse(ctx, s.client.accountID, generated.CreateEmbedJSONRequestBody{Url: url})
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return embedFromBody(resp.Body)
}

func (s *EmbedsService) Get(ctx context.Context, sgid string) (result *Embed, err error) {
	op := OperationInfo{
		Service: "Embeds", Operation: "Get",
		ResourceType: "embed", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if sgid == "" {
		err = ErrUsage("embed sgid is required")
		return nil, err
	}

	resp, err := s.client.parent.gen.GetEmbedWithResponse(ctx, s.client.accountID, sgid)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return embedFromBody(resp.Body)
}

func embedFromBody(body []byte) (*Embed, error) {
	var ge generated.Embed
	if err := json.Unmarshal(body, &ge); err != nil {
		return nil, fmt.Errorf("failed to parse embed: %w", err)
	}
	return &Embed{
		URL:         ge.Url,
		Status:      ge.Status,
		SGID:        ge.Sgid,
		Type:        deref(ge.Type),
		ContentType: deref(ge.ContentType),
		ImageURL:    deref(ge.ImageUrl),
		Content:     deref(ge.Content),
	}, nil
}
