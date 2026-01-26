package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func subscriptionsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "subscriptions")
}

func loadSubscriptionsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(subscriptionsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestSubscription_UnmarshalGet(t *testing.T) {
	data := loadSubscriptionsFixture(t, "get.json")

	var subscription Subscription
	if err := json.Unmarshal(data, &subscription); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if !subscription.Subscribed {
		t.Error("expected Subscribed to be true")
	}
	if subscription.Count != 3 {
		t.Errorf("expected Count 3, got %d", subscription.Count)
	}
	if subscription.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/recordings/1069479351/subscription.json" {
		t.Errorf("unexpected URL: %q", subscription.URL)
	}
	if len(subscription.Subscribers) != 3 {
		t.Errorf("expected 3 subscribers, got %d", len(subscription.Subscribers))
	}

	// Verify first subscriber
	s1 := subscription.Subscribers[0]
	if s1.ID != 1049715915 {
		t.Errorf("expected ID 1049715915, got %d", s1.ID)
	}
	if s1.Name != "Victor Cooper" {
		t.Errorf("expected Name 'Victor Cooper', got %q", s1.Name)
	}
	if s1.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected EmailAddress 'victor@honchodesign.com', got %q", s1.EmailAddress)
	}
	if s1.Title != "Chief Strategist" {
		t.Errorf("expected Title 'Chief Strategist', got %q", s1.Title)
	}
	if !s1.Admin {
		t.Error("expected Admin to be true")
	}
	if !s1.Owner {
		t.Error("expected Owner to be true")
	}
	if !s1.Employee {
		t.Error("expected Employee to be true")
	}
	if s1.TimeZone != "America/Chicago" {
		t.Errorf("expected TimeZone 'America/Chicago', got %q", s1.TimeZone)
	}

	// Verify company
	if s1.Company == nil {
		t.Fatal("expected Company to be non-nil")
	}
	if s1.Company.ID != 1033447817 {
		t.Errorf("expected Company.ID 1033447817, got %d", s1.Company.ID)
	}
	if s1.Company.Name != "Honcho Design" {
		t.Errorf("expected Company.Name 'Honcho Design', got %q", s1.Company.Name)
	}

	// Verify second subscriber
	s2 := subscription.Subscribers[1]
	if s2.ID != 1049715923 {
		t.Errorf("expected ID 1049715923, got %d", s2.ID)
	}
	if s2.Name != "Andrew Wong" {
		t.Errorf("expected Name 'Andrew Wong', got %q", s2.Name)
	}

	// Verify third subscriber
	s3 := subscription.Subscribers[2]
	if s3.ID != 1049715916 {
		t.Errorf("expected ID 1049715916, got %d", s3.ID)
	}
	if s3.Name != "Annie Bryan" {
		t.Errorf("expected Name 'Annie Bryan', got %q", s3.Name)
	}
}

func TestSubscription_UnmarshalSubscribe(t *testing.T) {
	data := loadSubscriptionsFixture(t, "subscribe.json")

	var subscription Subscription
	if err := json.Unmarshal(data, &subscription); err != nil {
		t.Fatalf("failed to unmarshal subscribe.json: %v", err)
	}

	if !subscription.Subscribed {
		t.Error("expected Subscribed to be true")
	}
	if subscription.Count != 4 {
		t.Errorf("expected Count 4, got %d", subscription.Count)
	}
	if len(subscription.Subscribers) != 4 {
		t.Errorf("expected 4 subscribers, got %d", len(subscription.Subscribers))
	}

	// Verify the new subscriber (current user) was added
	s4 := subscription.Subscribers[3]
	if s4.ID != 1049715917 {
		t.Errorf("expected ID 1049715917, got %d", s4.ID)
	}
	if s4.Name != "Current User" {
		t.Errorf("expected Name 'Current User', got %q", s4.Name)
	}
}

func TestSubscription_UnmarshalUpdate(t *testing.T) {
	data := loadSubscriptionsFixture(t, "update.json")

	var subscription Subscription
	if err := json.Unmarshal(data, &subscription); err != nil {
		t.Fatalf("failed to unmarshal update.json: %v", err)
	}

	if !subscription.Subscribed {
		t.Error("expected Subscribed to be true")
	}
	if subscription.Count != 2 {
		t.Errorf("expected Count 2, got %d", subscription.Count)
	}
	if len(subscription.Subscribers) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(subscription.Subscribers))
	}

	// Verify Victor is still subscribed
	if subscription.Subscribers[0].ID != 1049715915 {
		t.Errorf("expected first subscriber ID 1049715915, got %d", subscription.Subscribers[0].ID)
	}

	// Verify Annie is still subscribed
	if subscription.Subscribers[1].ID != 1049715916 {
		t.Errorf("expected second subscriber ID 1049715916, got %d", subscription.Subscribers[1].ID)
	}
}

func TestUpdateSubscriptionRequest_Marshal(t *testing.T) {
	req := UpdateSubscriptionRequest{
		Subscriptions:   []int64{1049715916},
		Unsubscriptions: []int64{1049715923},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateSubscriptionRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	subs, ok := data["subscriptions"].([]interface{})
	if !ok {
		t.Fatal("subscriptions should be an array")
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}
	if int64(subs[0].(float64)) != 1049715916 {
		t.Errorf("expected subscription ID 1049715916, got %v", subs[0])
	}

	unsubs, ok := data["unsubscriptions"].([]interface{})
	if !ok {
		t.Fatal("unsubscriptions should be an array")
	}
	if len(unsubs) != 1 {
		t.Errorf("expected 1 unsubscription, got %d", len(unsubs))
	}
	if int64(unsubs[0].(float64)) != 1049715923 {
		t.Errorf("expected unsubscription ID 1049715923, got %v", unsubs[0])
	}

	// Round-trip test
	var roundtrip UpdateSubscriptionRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if len(roundtrip.Subscriptions) != 1 || roundtrip.Subscriptions[0] != 1049715916 {
		t.Errorf("expected Subscriptions [1049715916], got %v", roundtrip.Subscriptions)
	}
	if len(roundtrip.Unsubscriptions) != 1 || roundtrip.Unsubscriptions[0] != 1049715923 {
		t.Errorf("expected Unsubscriptions [1049715923], got %v", roundtrip.Unsubscriptions)
	}
}

func TestUpdateSubscriptionRequest_MarshalOmitsEmpty(t *testing.T) {
	// Test that empty arrays are omitted
	req := UpdateSubscriptionRequest{
		Subscriptions: []int64{1049715916},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateSubscriptionRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, exists := data["unsubscriptions"]; exists {
		t.Error("unsubscriptions should be omitted when empty")
	}
}
