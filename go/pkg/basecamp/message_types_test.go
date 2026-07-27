package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Distinct bucketID/typeID — message types are bucket-scoped, so a future swap of
// the argument order would build a path with the IDs in the wrong slots and fail
// the assertion.
const (
	messageTypesTestBucketID = int64(2085958499)
	messageTypesTestTypeID   = int64(1069479340)
)

// testMessageTypesServer creates an httptest.Server and a MessageTypesService wired to it.
func testMessageTypesServer(t *testing.T, handler http.HandlerFunc) *MessageTypesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.MessageTypes()
}

func messageTypesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "message_types")
}

func loadMessageTypesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(messageTypesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestMessageType_UnmarshalList(t *testing.T) {
	data := loadMessageTypesFixture(t, "list.json")

	var types []MessageType
	if err := json.Unmarshal(data, &types); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(types) != 4 {
		t.Errorf("expected 4 message types, got %d", len(types))
	}

	// Verify first type
	t1 := types[0]
	if t1.ID != 1069479340 {
		t.Errorf("expected ID 1069479340, got %d", t1.ID)
	}
	if t1.Name != "Announcement" {
		t.Errorf("expected name 'Announcement', got %q", t1.Name)
	}
	if t1.Icon != "📢" {
		t.Errorf("expected icon '📢', got %q", t1.Icon)
	}

	// Verify timestamps are parsed
	if t1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if t1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify second type
	t2 := types[1]
	if t2.ID != 1069479341 {
		t.Errorf("expected ID 1069479341, got %d", t2.ID)
	}
	if t2.Name != "FYI" {
		t.Errorf("expected name 'FYI', got %q", t2.Name)
	}

	// Verify third type
	t3 := types[2]
	if t3.ID != 1069479342 {
		t.Errorf("expected ID 1069479342, got %d", t3.ID)
	}
	if t3.Name != "Heartbeat" {
		t.Errorf("expected name 'Heartbeat', got %q", t3.Name)
	}

	// Verify fourth type
	t4 := types[3]
	if t4.ID != 1069479343 {
		t.Errorf("expected ID 1069479343, got %d", t4.ID)
	}
	if t4.Name != "Question" {
		t.Errorf("expected name 'Question', got %q", t4.Name)
	}
}

func TestMessageType_UnmarshalGet(t *testing.T) {
	data := loadMessageTypesFixture(t, "get.json")

	var msgType MessageType
	if err := json.Unmarshal(data, &msgType); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if msgType.ID != 1069479340 {
		t.Errorf("expected ID 1069479340, got %d", msgType.ID)
	}
	if msgType.Name != "Announcement" {
		t.Errorf("expected name 'Announcement', got %q", msgType.Name)
	}
	if msgType.Icon != "📢" {
		t.Errorf("expected icon '📢', got %q", msgType.Icon)
	}

	// Verify timestamps are parsed
	if msgType.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if msgType.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestCreateMessageTypeRequest_Marshal(t *testing.T) {
	req := CreateMessageTypeRequest{
		Name: "Update",
		Icon: "🔄",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMessageTypeRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Update" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["icon"] != "🔄" {
		t.Errorf("unexpected icon: %v", data["icon"])
	}

	// Round-trip test
	var roundtrip CreateMessageTypeRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Icon != req.Icon {
		t.Errorf("expected icon %q, got %q", req.Icon, roundtrip.Icon)
	}
}

func TestUpdateMessageTypeRequest_Marshal(t *testing.T) {
	req := UpdateMessageTypeRequest{
		Name: "Important Update",
		Icon: "⚠️",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageTypeRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Important Update" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["icon"] != "⚠️" {
		t.Errorf("unexpected icon: %v", data["icon"])
	}

	// Round-trip test
	var roundtrip UpdateMessageTypeRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Icon != req.Icon {
		t.Errorf("expected icon %q, got %q", req.Icon, roundtrip.Icon)
	}
}

func TestUpdateMessageTypeRequest_MarshalPartial(t *testing.T) {
	// Test with only name
	req := UpdateMessageTypeRequest{
		Name: "New Name Only",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageTypeRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "New Name Only" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	// Icon should be omitted
	if _, ok := data["icon"]; ok {
		t.Error("expected icon to be omitted")
	}
}

// TestMessageTypesService_Routing pins the bucket-scoped wire paths for all five
// operations. Message types were modeled account-scoped (`/{accountId}/categories`)
// and 404'd against the real API (#368); these assertions fail if the routes ever
// regress to a flat shape or if bucketID/typeID are transposed.
func TestMessageTypesService_Routing(t *testing.T) {
	const (
		wantCollection = "/99999/buckets/2085958499/categories.json"
		wantMember     = "/99999/buckets/2085958499/categories/1069479340"
	)

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		status     int
		body       func(t *testing.T) []byte
		invoke     func(ctx context.Context, svc *MessageTypesService) error
	}{
		{
			name: "List", wantMethod: http.MethodGet, wantPath: wantCollection, status: http.StatusOK,
			body: func(t *testing.T) []byte { return loadMessageTypesFixture(t, "list.json") },
			invoke: func(ctx context.Context, svc *MessageTypesService) error {
				_, err := svc.List(ctx, messageTypesTestBucketID, &MessageTypeListOptions{Page: 1})
				return err
			},
		},
		{
			name: "Create", wantMethod: http.MethodPost, wantPath: wantCollection, status: http.StatusCreated,
			body: func(t *testing.T) []byte { return loadMessageTypesFixture(t, "get.json") },
			invoke: func(ctx context.Context, svc *MessageTypesService) error {
				_, err := svc.Create(ctx, messageTypesTestBucketID, &CreateMessageTypeRequest{Name: "Announcement", Icon: "📢"})
				return err
			},
		},
		{
			name: "Get", wantMethod: http.MethodGet, wantPath: wantMember, status: http.StatusOK,
			body: func(t *testing.T) []byte { return loadMessageTypesFixture(t, "get.json") },
			invoke: func(ctx context.Context, svc *MessageTypesService) error {
				_, err := svc.Get(ctx, messageTypesTestBucketID, messageTypesTestTypeID)
				return err
			},
		},
		{
			name: "Update", wantMethod: http.MethodPut, wantPath: wantMember, status: http.StatusOK,
			body: func(t *testing.T) []byte { return loadMessageTypesFixture(t, "get.json") },
			invoke: func(ctx context.Context, svc *MessageTypesService) error {
				_, err := svc.Update(ctx, messageTypesTestBucketID, messageTypesTestTypeID, &UpdateMessageTypeRequest{Name: "Heads up"})
				return err
			},
		},
		{
			name: "Delete", wantMethod: http.MethodDelete, wantPath: wantMember, status: http.StatusNoContent,
			body: func(*testing.T) []byte { return nil },
			invoke: func(ctx context.Context, svc *MessageTypesService) error {
				return svc.Delete(ctx, messageTypesTestBucketID, messageTypesTestTypeID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			svc := testMessageTypesServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				body := tt.body(t)
				if body != nil {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write(body)
			})

			if err := tt.invoke(context.Background(), svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("expected method %s, got %s", tt.wantMethod, gotMethod)
			}
			if gotPath != tt.wantPath {
				t.Errorf("expected path %s, got %s", tt.wantPath, gotPath)
			}
		})
	}
}
