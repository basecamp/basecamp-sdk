package basecamp

import (
	"context"
	"fmt"
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

	path := fmt.Sprintf("/buckets/%d/recordings/%d/subscription.json", bucketID, recordingID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var subscription Subscription
	if err := resp.UnmarshalData(&subscription); err != nil {
		return nil, fmt.Errorf("failed to parse subscription: %w", err)
	}

	return &subscription, nil
}

// Subscribe subscribes the current user to the recording.
// bucketID is the project ID, recordingID is the ID of the recording.
// Returns the updated subscription information.
func (s *SubscriptionsService) Subscribe(ctx context.Context, bucketID, recordingID int64) (*Subscription, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/subscription.json", bucketID, recordingID)
	resp, err := s.client.Post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var subscription Subscription
	if err := resp.UnmarshalData(&subscription); err != nil {
		return nil, fmt.Errorf("failed to parse subscription: %w", err)
	}

	return &subscription, nil
}

// Unsubscribe unsubscribes the current user from the recording.
// bucketID is the project ID, recordingID is the ID of the recording.
// Returns nil on success (204 No Content).
func (s *SubscriptionsService) Unsubscribe(ctx context.Context, bucketID, recordingID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/subscription.json", bucketID, recordingID)
	_, err := s.client.Delete(ctx, path)
	return err
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

	path := fmt.Sprintf("/buckets/%d/recordings/%d/subscription.json", bucketID, recordingID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var subscription Subscription
	if err := resp.UnmarshalData(&subscription); err != nil {
		return nil, fmt.Errorf("failed to parse subscription: %w", err)
	}

	return &subscription, nil
}
