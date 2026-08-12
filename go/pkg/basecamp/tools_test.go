package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func toolsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "tools")
}

func loadToolsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(toolsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

// The dock-tool projection is the bare recordings/recording partial:
// api/docks/tools/show.json.jbuilder renders it and adds nothing, so a tool
// response carries no `name` and no `enabled` key at all (#650).
func TestTool_UnmarshalGet(t *testing.T) {
	data := loadToolsFixture(t, "get.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if tool.ID != 1069479832 {
		t.Errorf("expected ID 1069479832, got %d", tool.ID)
	}
	if tool.Status != "active" {
		t.Errorf("expected status 'active', got %q", tool.Status)
	}
	if tool.Title != "Chat" {
		t.Errorf("expected title 'Chat', got %q", tool.Title)
	}
	if tool.Type != "Chat::Transcript" {
		t.Errorf("expected type 'Chat::Transcript', got %q", tool.Type)
	}
	if tool.VisibleToClients {
		t.Error("expected VisibleToClients to be false")
	}
	if !tool.InheritsStatus {
		t.Error("expected InheritsStatus to be true")
	}
	if tool.BookmarkURL == "" {
		t.Error("expected BookmarkURL to be populated")
	}
	// Chat::Transcript overrides Recordable#subscribable?, so the partial's
	// `if recording.subscribable?` fires and subscription_url is emitted.
	if tool.SubscriptionURL == "" {
		t.Error("expected SubscriptionURL to be populated for a Chat::Transcript")
	}
	if tool.Position == nil || *tool.Position != 5 {
		t.Errorf("expected position 5, got %v", tool.Position)
	}
	if tool.URL != "https://3.basecampapi.com/195539477/buckets/2085958505/chats/1069479832.json" {
		t.Errorf("unexpected URL: %q", tool.URL)
	}
	if tool.AppURL != "https://3.basecamp.com/195539477/buckets/2085958505/chats/1069479832" {
		t.Errorf("unexpected AppURL: %q", tool.AppURL)
	}

	// The two keys #650 relaxed: never emitted, so never non-nil.
	if tool.Name != nil {
		t.Errorf("expected Name to be nil, got %q", *tool.Name)
	}
	if tool.Enabled != nil {
		t.Errorf("expected Enabled to be nil, got %v", *tool.Enabled)
	}
	// A docked tool is docked, so the partial's `if !recording.docked?` is false.
	if tool.Parent != nil {
		t.Errorf("expected Parent to be nil for a docked tool, got %+v", tool.Parent)
	}

	// Verify timestamps are parsed
	if tool.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if tool.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify bucket
	if tool.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if tool.Bucket.ID != 2085958505 {
		t.Errorf("expected Bucket.ID 2085958505, got %d", tool.Bucket.ID)
	}
	if tool.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", tool.Bucket.Name)
	}
	if tool.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", tool.Bucket.Type)
	}

	// Verify creator
	if tool.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if tool.Creator.ID != 1049715913 {
		t.Errorf("expected Creator.ID 1049715913, got %d", tool.Creator.ID)
	}
	if tool.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", tool.Creator.Name)
	}
}

func TestToolsServiceCreatePostsToBucketDock(t *testing.T) {
	const (
		accountID = "5245563"
		bucketID  = int64(33861629)
		toolType  = "Message::Board"
		title     = "Intervention Log / Journal"
	)

	expectedPath := fmt.Sprintf("/%s/buckets/%d/dock/tools.json", accountID, bucketID)

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if r.Method != http.MethodPost || r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}

		body := decodeRequestBody(t, r)
		if got := body["tool_type"]; got != toolType {
			t.Fatalf("tool_type = %v, want %q", got, toolType)
		}
		if got := body["title"]; got != title {
			t.Fatalf("title = %v, want %q", got, title)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(loadToolsFixture(t, "create.json"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	var capturedOp OperationInfo
	hooks := &testHooks{
		onOperationStart: func(ctx context.Context, op OperationInfo) context.Context {
			capturedOp = op
			return ctx
		},
	}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHooks(hooks))

	_, err := client.ForAccount(accountID).Tools().Create(
		context.Background(),
		bucketID,
		toolType,
		&CreateToolOptions{Title: title},
	)
	if err != nil {
		t.Fatalf("Create() error = %v; request path = %s; want bucket %d dock tools endpoint", err, capturedPath, bucketID)
	}
	if capturedOp.ProjectID != bucketID {
		t.Fatalf("Create() operation ProjectID = %d, want destination bucket %d", capturedOp.ProjectID, bucketID)
	}
	if capturedOp.ResourceID != 0 {
		t.Fatalf("Create() operation ResourceID = %d, want 0 (bucket is project scope, not a resource)", capturedOp.ResourceID)
	}
}

func TestToolsServiceCreateOmitsTitleWhenNotProvided(t *testing.T) {
	const (
		accountID = "5245563"
		bucketID  = int64(33861629)
		toolType  = "Message::Board"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequestBody(t, r)
		if got := body["tool_type"]; got != toolType {
			t.Fatalf("tool_type = %v, want %q", got, toolType)
		}
		if title, present := body["title"]; present {
			t.Fatalf("title = %v, want omitted from request body", title)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(loadToolsFixture(t, "create.json"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	if _, err := client.ForAccount(accountID).Tools().Create(context.Background(), bucketID, toolType, nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

// TestToolsServiceCreateVisibleToClients verifies the tri-state
// visible_to_clients flag reaches the wire correctly on create: nil omits the
// key, true is sent verbatim, and an explicit false is sent (not dropped).
func TestToolsServiceCreateVisibleToClients(t *testing.T) {
	const (
		accountID = "5245563"
		bucketID  = int64(33861629)
		toolType  = "Chat::Transcript"
	)
	tru, fls := true, false
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{"nil omits the field", nil, false, false},
		{"true is sent", &tru, true, true},
		{"explicit false is sent, not dropped", &fls, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedBody = decodeRequestBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(loadToolsFixture(t, "create.json"))
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.BaseURL = server.URL
			client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

			_, err := client.ForAccount(accountID).Tools().Create(
				context.Background(),
				bucketID,
				toolType,
				&CreateToolOptions{VisibleToClients: tc.value},
			)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			val, ok := receivedBody["visible_to_clients"]
			if ok != tc.present {
				t.Fatalf("visible_to_clients present=%v, want %v (body=%v)", ok, tc.present, receivedBody)
			}
			if tc.present && val != tc.want {
				t.Errorf("visible_to_clients=%v, want %v", val, tc.want)
			}
		})
	}
}

func TestToolsServiceCreateEmptyToolType(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	var startedOp, endedOp OperationInfo
	var startObserved, endObserved bool
	var endErr error
	hooks := &testHooks{
		onOperationStart: func(ctx context.Context, op OperationInfo) context.Context {
			startObserved = true
			startedOp = op
			return ctx
		},
		onOperationEnd: func(ctx context.Context, op OperationInfo, err error, d time.Duration) {
			endObserved = true
			endedOp = op
			endErr = err
		},
	}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHooks(hooks))

	_, err := client.ForAccount("5245563").Tools().Create(context.Background(), 33861629, "", nil)
	if err == nil {
		t.Fatal("expected error for empty tool type")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Fatalf("expected usage error, got: %v", err)
	}
	if apiErr.Message != "tool type is required" {
		t.Errorf("expected message %q, got %q", "tool type is required", apiErr.Message)
	}
	if requestCount != 0 {
		t.Errorf("expected 0 HTTP requests for client-side validation failure, got %d", requestCount)
	}
	if !startObserved || !endObserved {
		t.Fatalf("expected operation hooks to fire: start=%v end=%v", startObserved, endObserved)
	}
	if startedOp.Operation != "Create" || endedOp.Operation != "Create" {
		t.Errorf("expected hooks to observe Create operation, got start=%q end=%q", startedOp.Operation, endedOp.Operation)
	}
	if !errors.Is(endErr, err) {
		t.Errorf("expected OnOperationEnd to observe the usage error, got: %v", endErr)
	}
}

func TestTool_UnmarshalCreate(t *testing.T) {
	data := loadToolsFixture(t, "create.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal create.json: %v", err)
	}

	if tool.ID != 1069479911 {
		t.Errorf("expected ID 1069479911, got %d", tool.ID)
	}
	if tool.Title != "Q&A Chat" {
		t.Errorf("expected title 'Q&A Chat', got %q", tool.Title)
	}
	if tool.Type != "Chat::Transcript" {
		t.Errorf("expected type 'Chat::Transcript', got %q", tool.Type)
	}
	// The 201 body is the same projection as GET, so it too omits both keys.
	if tool.Name != nil {
		t.Errorf("expected Name to be nil, got %q", *tool.Name)
	}
	if tool.Enabled != nil {
		t.Errorf("expected Enabled to be nil, got %v", *tool.Enabled)
	}
	// Chat::Transcript is one of the two types that honor create-time
	// visible_to_clients, and this tool was created with it true.
	if !tool.VisibleToClients {
		t.Error("expected VisibleToClients to be true")
	}
	if tool.Position == nil || *tool.Position != 6 {
		t.Errorf("expected position 6, got %v", tool.Position)
	}
}

func TestTool_UnmarshalUpdate(t *testing.T) {
	data := loadToolsFixture(t, "update.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal update.json: %v", err)
	}

	if tool.ID != 1069479832 {
		t.Errorf("expected ID 1069479832, got %d", tool.ID)
	}
	if tool.Title != "Team Chat" {
		t.Errorf("expected title 'Team Chat', got %q", tool.Title)
	}
	if tool.Name != nil {
		t.Errorf("expected Name to be nil, got %q", *tool.Name)
	}
}

// A disabled tool is removed from the dock, not deleted: `recording.positioned?`
// is false so `position` is absent entirely. Absence of Position — never an
// `enabled` key — is the disabled signal. This one is also a Vault, which does
// not override Recordable#subscribable? (default false), so subscription_url is
// absent too.
func TestTool_UnmarshalDisabled(t *testing.T) {
	data := loadToolsFixture(t, "disabled.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal disabled.json: %v", err)
	}

	if tool.ID != 1069479343 {
		t.Errorf("expected ID 1069479343, got %d", tool.ID)
	}
	if tool.Title != "Docs & Files" {
		t.Errorf("expected title 'Docs & Files', got %q", tool.Title)
	}
	if tool.Type != "Vault" {
		t.Errorf("expected type 'Vault', got %q", tool.Type)
	}
	if tool.Name != nil {
		t.Errorf("expected Name to be nil, got %q", *tool.Name)
	}
	if tool.Enabled != nil {
		t.Errorf("expected Enabled to be nil even for a disabled tool, got %v", *tool.Enabled)
	}
	if tool.Position != nil {
		t.Errorf("expected position to be nil, got %v", tool.Position)
	}
	if tool.SubscriptionURL != "" {
		t.Errorf("expected SubscriptionURL to be empty for a Vault, got %q", tool.SubscriptionURL)
	}
}

// `parent` is emitted only when `!recording.docked?`. The dock-tool lookup
// scopes by recordable TYPE (Recordable::CORE_GROUPS["dock_tools"] includes
// Vault) rather than by dock membership, so a vault nested inside another vault
// resolves through GET /dock/tools/:id and does carry a parent.
func TestTool_UnmarshalNestedVaultCarriesParent(t *testing.T) {
	data := loadToolsFixture(t, "nested_vault.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal nested_vault.json: %v", err)
	}

	if tool.ID != 1069479562 {
		t.Errorf("expected ID 1069479562, got %d", tool.ID)
	}
	if tool.Parent == nil {
		t.Fatal("expected Parent to be non-nil for a nested vault")
	}
	if tool.Parent.ID != 1069479343 {
		t.Errorf("expected Parent.ID 1069479343, got %d", tool.Parent.ID)
	}
	if tool.Parent.Title != "Docs & Files" {
		t.Errorf("expected Parent.Title 'Docs & Files', got %q", tool.Parent.Title)
	}
	if tool.Parent.Type != "Vault" {
		t.Errorf("expected Parent.Type 'Vault', got %q", tool.Parent.Type)
	}
}

// The Unmarshal tests above decode the fixture straight into the wrapper, which
// bypasses toolFromGenerated — the conversion where a dropped field silently
// zeroes. This one goes through the service, so every absorbed key has to
// survive the generated-struct hop.
func TestToolsServiceGetCarriesTheFullProjection(t *testing.T) {
	const (
		accountID = "5245563"
		toolID    = int64(1069479832)
	)

	expectedPath := fmt.Sprintf("/%s/dock/tools/%d", accountID, toolID)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(loadToolsFixture(t, "get.json"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	tool, err := client.ForAccount(accountID).Tools().Get(context.Background(), toolID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tool.Type != "Chat::Transcript" {
		t.Errorf("Type = %q, want \"Chat::Transcript\"", tool.Type)
	}
	if tool.VisibleToClients {
		t.Error("VisibleToClients = true, want false")
	}
	if !tool.InheritsStatus {
		t.Error("InheritsStatus = false, want true")
	}
	if tool.BookmarkURL == "" {
		t.Error("BookmarkURL is empty; the partial emits it unconditionally")
	}
	if tool.SubscriptionURL == "" {
		t.Error("SubscriptionURL is empty; a Chat::Transcript is subscribable")
	}
	if tool.Creator == nil {
		t.Fatal("Creator is nil")
	}
	if tool.Creator.Name != "Victor Cooper" {
		t.Errorf("Creator.Name = %q, want \"Victor Cooper\"", tool.Creator.Name)
	}
	if tool.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("Creator.EmailAddress = %q, want \"victor@honchodesign.com\"", tool.Creator.EmailAddress)
	}
	if tool.Name != nil {
		t.Errorf("Name = %q, want nil — the projection emits no `name` key", *tool.Name)
	}
	if tool.Enabled != nil {
		t.Errorf("Enabled = %v, want nil — the projection emits no `enabled` key", *tool.Enabled)
	}
}

func TestUpdateToolRequest_Marshal(t *testing.T) {
	req := UpdateToolRequest{
		Title: "Project Tasks",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateToolRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Project Tasks" {
		t.Errorf("expected title 'Project Tasks', got %v", data["name"])
	}

	// Round-trip test
	var roundtrip UpdateToolRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
}
