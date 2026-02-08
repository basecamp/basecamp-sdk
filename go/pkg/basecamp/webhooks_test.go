package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func webhooksFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "webhooks")
}

func loadWebhooksFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(webhooksFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestWebhook_UnmarshalList(t *testing.T) {
	data := loadWebhooksFixture(t, "list.json")

	var webhooks []Webhook
	if err := json.Unmarshal(data, &webhooks); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(webhooks))
	}

	// Verify first webhook
	wh1 := webhooks[0]
	if wh1.ID != 9007199254741433 {
		t.Errorf("expected ID 9007199254741433, got %d", wh1.ID)
	}
	if wh1.PayloadURL != "https://example.com/webhooks/basecamp" {
		t.Errorf("expected payload_url 'https://example.com/webhooks/basecamp', got %q", wh1.PayloadURL)
	}
	if !wh1.Active {
		t.Error("expected active to be true")
	}
	if len(wh1.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(wh1.Types))
	}
	if wh1.Types[0] != "Todo" {
		t.Errorf("expected first type 'Todo', got %q", wh1.Types[0])
	}
	if wh1.Types[1] != "Todolist" {
		t.Errorf("expected second type 'Todolist', got %q", wh1.Types[1])
	}

	// Verify second webhook is inactive
	wh2 := webhooks[1]
	if wh2.ID != 9007199254741434 {
		t.Errorf("expected ID 9007199254741434, got %d", wh2.ID)
	}
	if wh2.Active {
		t.Error("expected active to be false")
	}
	if len(wh2.Types) != 1 || wh2.Types[0] != "Comment" {
		t.Errorf("expected types ['Comment'], got %v", wh2.Types)
	}
}

func TestWebhook_UnmarshalGet(t *testing.T) {
	data := loadWebhooksFixture(t, "get.json")

	var webhook Webhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if webhook.ID != 9007199254741433 {
		t.Errorf("expected ID 9007199254741433, got %d", webhook.ID)
	}
	if webhook.PayloadURL != "https://example.com/webhooks/basecamp" {
		t.Errorf("expected payload_url 'https://example.com/webhooks/basecamp', got %q", webhook.PayloadURL)
	}
	if !webhook.Active {
		t.Error("expected active to be true")
	}
	if len(webhook.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(webhook.Types))
	}
	if webhook.URL == "" {
		t.Error("expected non-empty URL")
	}
	if webhook.AppURL == "" {
		t.Error("expected non-empty AppURL")
	}
}

func TestCreateWebhookRequest_Marshal(t *testing.T) {
	data := loadWebhooksFixture(t, "create-request.json")

	var req CreateWebhookRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.PayloadURL != "https://example.com/webhooks/new" {
		t.Errorf("expected payload_url 'https://example.com/webhooks/new', got %q", req.PayloadURL)
	}
	if len(req.Types) != 3 {
		t.Errorf("expected 3 types, got %d", len(req.Types))
	}
	if req.Types[0] != "Todo" || req.Types[1] != "Comment" || req.Types[2] != "Message" {
		t.Errorf("unexpected types: %v", req.Types)
	}
	if req.Active == nil || !*req.Active {
		t.Error("expected active to be true")
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateWebhookRequest: %v", err)
	}

	var roundtrip CreateWebhookRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.PayloadURL != req.PayloadURL {
		t.Error("round-trip payload_url mismatch")
	}
	if len(roundtrip.Types) != len(req.Types) {
		t.Error("round-trip types mismatch")
	}
}

func TestUpdateWebhookRequest_Marshal(t *testing.T) {
	data := loadWebhooksFixture(t, "update-request.json")

	var req UpdateWebhookRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.PayloadURL != "https://example.com/webhooks/updated" {
		t.Errorf("expected payload_url 'https://example.com/webhooks/updated', got %q", req.PayloadURL)
	}
	if len(req.Types) != 1 || req.Types[0] != "Todo" {
		t.Errorf("expected types ['Todo'], got %v", req.Types)
	}
	if req.Active == nil || *req.Active {
		t.Error("expected active to be false")
	}
}

func TestWebhook_UnmarshalGetWithRecentDeliveries(t *testing.T) {
	data := loadWebhooksFixture(t, "get.json")

	var webhook Webhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if len(webhook.RecentDeliveries) != 1 {
		t.Fatalf("expected 1 recent delivery, got %d", len(webhook.RecentDeliveries))
	}

	delivery := webhook.RecentDeliveries[0]
	if delivery.ID != 1230 {
		t.Errorf("expected delivery ID 1230, got %d", delivery.ID)
	}
	if delivery.CreatedAt.IsZero() {
		t.Error("expected non-zero delivery CreatedAt")
	}

	// Request
	if delivery.Request.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected request Content-Type 'application/json', got %q", delivery.Request.Headers["Content-Type"])
	}
	if delivery.Request.Headers["User-Agent"] != "Basecamp3 Webhook" {
		t.Errorf("expected request User-Agent 'Basecamp3 Webhook', got %q", delivery.Request.Headers["User-Agent"])
	}

	// Request body is a WebhookEvent
	body := delivery.Request.Body
	if body.ID != 9007199254741001 {
		t.Errorf("expected body event ID 9007199254741001, got %d", body.ID)
	}
	if body.Kind != "todo_created" {
		t.Errorf("expected body event kind 'todo_created', got %q", body.Kind)
	}
	if body.Recording.Type != "Todo" {
		t.Errorf("expected body recording type 'Todo', got %q", body.Recording.Type)
	}
	if body.Recording.Title != "Ship the feature" {
		t.Errorf("expected body recording title 'Ship the feature', got %q", body.Recording.Title)
	}
	if body.Creator.Name != "Annie Bryan" {
		t.Errorf("expected body creator name 'Annie Bryan', got %q", body.Creator.Name)
	}

	// Response
	if delivery.Response.Code != 200 {
		t.Errorf("expected response code 200, got %d", delivery.Response.Code)
	}
	if delivery.Response.Message != "OK" {
		t.Errorf("expected response message 'OK', got %q", delivery.Response.Message)
	}
	if delivery.Response.Headers["Content-Type"] != "text/html" {
		t.Errorf("expected response Content-Type 'text/html', got %q", delivery.Response.Headers["Content-Type"])
	}
}

func TestWebhook_TimestampParsing(t *testing.T) {
	data := loadWebhooksFixture(t, "get.json")

	var webhook Webhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if webhook.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if webhook.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if webhook.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", webhook.CreatedAt.Year())
	}
}
