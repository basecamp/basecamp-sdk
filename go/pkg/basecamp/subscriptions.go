package basecamp

import (
	"context"
	"fmt"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Subscription represents the subscription state for a recording.
type Subscription struct {
	Subscribed  bool     `json:"subscribed"`
	Count       int      `json:"count"`
	URL         string   `json:"url"`
	Subscribers []Person `json:"subscribers"`
}

// UpdateSubscriptionRequest specifies the parameters for updating subscriptions.
type UpdateSubscriptionRequest struct {
	// Subscriptions is a list of person IDs to subscribe to the recording.
	Subscriptions []int64 `json:"subscriptions,omitempty"`
	// Unsubscriptions is a list of person IDs to unsubscribe from the recording.
	Unsubscriptions []int64 `json:"unsubscriptions,omitempty"`
}

// SubscriptionsService handles subscription operations on recordings.
type SubscriptionsService struct {
	client *Client
}

// NewSubscriptionsService creates a new SubscriptionsService.
func NewSubscriptionsService(client *Client) *SubscriptionsService {
	return &SubscriptionsService{client: client}
}

// Get returns the subscription information for a recording.
// bucketID is the project ID, recordingID is the ID of the recording.
func (s *SubscriptionsService) Get(ctx context.Context, bucketID, recordingID int64) (*Subscription, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetSubscriptionWithResponse(ctx, bucketID, recordingID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	subscription := subscriptionFromGenerated(resp.JSON200.Subscription)
	return &subscription, nil
}

// Subscribe subscribes the current user to the recording.
// bucketID is the project ID, recordingID is the ID of the recording.
// Returns the updated subscription information.
func (s *SubscriptionsService) Subscribe(ctx context.Context, bucketID, recordingID int64) (*Subscription, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.SubscribeWithResponse(ctx, bucketID, recordingID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	subscription := subscriptionFromGenerated(resp.JSON200.Subscription)
	return &subscription, nil
}

// Unsubscribe unsubscribes the current user from the recording.
// bucketID is the project ID, recordingID is the ID of the recording.
// Returns nil on success (204 No Content).
func (s *SubscriptionsService) Unsubscribe(ctx context.Context, bucketID, recordingID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.UnsubscribeWithResponse(ctx, bucketID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Update batch modifies subscriptions by adding or removing specific users.
// bucketID is the project ID, recordingID is the ID of the recording.
// Returns the updated subscription information.
func (s *SubscriptionsService) Update(ctx context.Context, bucketID, recordingID int64, req *UpdateSubscriptionRequest) (*Subscription, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || (len(req.Subscriptions) == 0 && len(req.Unsubscriptions) == 0) {
		return nil, ErrUsage("at least one of subscriptions or unsubscriptions must be specified")
	}

	body := generated.UpdateSubscriptionJSONRequestBody{
		Subscriptions:   req.Subscriptions,
		Unsubscriptions: req.Unsubscriptions,
	}

	resp, err := s.client.gen.UpdateSubscriptionWithResponse(ctx, bucketID, recordingID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	subscription := subscriptionFromGenerated(resp.JSON200.Subscription)
	return &subscription, nil
}

// subscriptionFromGenerated converts a generated Subscription to our clean type.
func subscriptionFromGenerated(gs generated.Subscription) Subscription {
	s := Subscription{
		Subscribed: gs.Subscribed,
		Count:      int(gs.Count),
		URL:        gs.Url,
	}

	if len(gs.Subscribers) > 0 {
		s.Subscribers = make([]Person, 0, len(gs.Subscribers))
		for _, gp := range gs.Subscribers {
			p := Person{
				Name:         gp.Name,
				EmailAddress: gp.EmailAddress,
				AvatarURL:    gp.AvatarUrl,
				Admin:        gp.Admin,
				Owner:        gp.Owner,
			}
			if gp.Id != nil {
				p.ID = *gp.Id
			}
			s.Subscribers = append(s.Subscribers, p)
		}
	}

	return s
}
