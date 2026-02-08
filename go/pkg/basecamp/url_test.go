package basecamp

import (
	"testing"
)

func TestRouterMatch(t *testing.T) {
	r := DefaultRouter()

	tests := []struct {
		name        string
		input       string
		wantNil     bool
		wantSource  MatchSource
		wantOp      string
		wantAccount string
		wantProject string
		wantRes     string
		wantComment string
		wantPath    string
	}{
		{
			name:        "todo URL",
			input:       "https://3.basecamp.com/123/buckets/456/todos/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "todos",
		},
		{
			name:        "message URL",
			input:       "https://3.basecamp.com/123/buckets/456/messages/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetMessage",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "messages",
		},
		{
			name:        "URL with comment fragment",
			input:       "https://3.basecamp.com/123/buckets/456/todos/789#__recording_999",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantComment: "999",
			wantPath:    "todos",
		},
		{
			name:        "card URL",
			input:       "https://3.basecamp.com/123/buckets/456/card_tables/cards/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetCard",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "cards",
		},
		{
			name:        "card column URL",
			input:       "https://3.basecamp.com/123/buckets/456/card_tables/columns/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetCardColumn",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "columns",
		},
		{
			name:        "project URL",
			input:       "https://3.basecamp.com/123/projects/456",
			wantSource:  MatchedAPI,
			wantOp:      "GetProject",
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "project",
		},
		{
			name:        "todolists URL",
			input:       "https://3.basecamp.com/123/buckets/456/todosets/777/todolists",
			wantSource:  MatchedAPI,
			wantOp:      "ListTodolists",
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "todolists", // proximate resource, not parent container
		},
		{
			name:        "localhost URL with buckets",
			input:       "http://localhost:3000/123/buckets/456/todos/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "todos",
		},
		{
			name:    "plain ID",
			input:   "789",
			wantNil: true,
		},
		{
			name:    "project name",
			input:   "my-project",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:        "comment URL",
			input:       "https://3.basecamp.com/123/buckets/456/comments/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetComment",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "comments",
		},
		{
			name:        "upload URL",
			input:       "https://3.basecamp.com/123/buckets/456/uploads/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetUpload",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "uploads",
		},
		{
			name:        "schedule entry URL",
			input:       "https://3.basecamp.com/123/buckets/456/schedule_entries/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetScheduleEntry",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "schedule_entries",
		},
		{
			name:        "numeric fragment",
			input:       "https://3.basecamp.com/123/buckets/456/messages/111#999",
			wantSource:  MatchedAPI,
			wantOp:      "GetMessage",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "111",
			wantComment: "999",
			wantPath:    "messages",
		},
		// Structural fallback tests — web-only URLs not in API spec
		{
			name:        "card_tables/lists alias (web-only)",
			input:       "https://3.basecamp.com/123/buckets/456/card_tables/lists/789",
			wantSource:  MatchedStructural,
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "columns", // normalized from "lists"
		},
		{
			name:        "generic type list URL (structural)",
			input:       "https://3.basecamp.com/123/buckets/456/sometype",
			wantSource:  MatchedStructural,
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "sometype",
		},
		{
			name:        "unknown recording type (structural)",
			input:       "https://3.basecamp.com/123/buckets/456/unknowntype/789",
			wantSource:  MatchedStructural,
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "unknowntype",
		},
		{
			name:        "localhost project URL",
			input:       "http://localhost:3000/123/projects/456",
			wantSource:  MatchedAPI,
			wantOp:      "GetProject",
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "project",
		},
		{
			name:        "staging URL",
			input:       "https://staging.example.com/123/buckets/456/todos/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "todos",
		},
		// Card table container and sub-resource PathType tests
		{
			name:        "card_table container URL",
			input:       "https://3.basecamp.com/123/buckets/456/card_tables/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetCardTable",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "card_tables", // proximate resource is the card table itself
		},
		{
			name:        "card_table columns list URL",
			input:       "https://3.basecamp.com/123/buckets/456/card_tables/789/columns",
			wantSource:  MatchedAPI,
			wantOp:      "CreateCardColumn", // API only has POST on this endpoint, no GET
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "columns", // proximate resource is the columns endpoint
		},
		// Scheme-less and alternative URL formats
		{
			name:        "scheme-less basecamp URL",
			input:       "3.basecamp.com/123/buckets/456/todos/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "todos",
		},
		{
			name:        "protocol-relative URL",
			input:       "//3.basecamp.com/123/buckets/456/todos/789",
			wantSource:  MatchedAPI,
			wantOp:      "GetTodo",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "789",
			wantPath:    "todos",
		},
		// Account-level routes on non-basecamp hosts
		{
			name:        "localhost people URL",
			input:       "http://localhost:3000/123/people/456",
			wantSource:  MatchedAPI,
			wantOp:      "GetPerson",
			wantAccount: "123",
			wantRes:     "456",
			wantPath:    "people",
		},
		// API URLs with .json suffix
		{
			name:        "API URL with .json suffix (boosts list)",
			input:       "https://3.basecampapi.com/123/recordings/200/boosts.json",
			wantSource:  MatchedAPI,
			wantOp:      "ListRecordingBoosts",
			wantAccount: "123",
			wantRes:     "200",
			wantPath:    "boosts",
		},
		{
			name:        "API URL with .json suffix and bucket (flattened)",
			input:       "https://3.basecampapi.com/123/buckets/456/recordings/200/boosts.json",
			wantSource:  MatchedAPI,
			wantOp:      "ListRecordingBoosts",
			wantAccount: "123",
			wantProject: "456",
			wantRes:     "200",
			wantPath:    "boosts",
		},
		{
			name:        "API URL with .json suffix (campfire lines)",
			input:       "https://3.basecampapi.com/123/buckets/456/chats/200/lines.json",
			wantSource:  MatchedAPI,
			wantOp:      "ListCampfireLines",
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "lines",
		},
		{
			name:        "API URL without .json still works",
			input:       "https://3.basecampapi.com/123/boosts/500",
			wantSource:  MatchedAPI,
			wantOp:      "GetBoost",
			wantAccount: "123",
			wantRes:     "500",
		},
		{
			name:        "API URL project sub-resource .json",
			input:       "https://3.basecampapi.com/123/projects/456/timeline.json",
			wantSource:  MatchedAPI,
			wantOp:      "GetProjectTimeline",
			wantAccount: "123",
			wantProject: "456",
			wantPath:    "timeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := r.Match(tt.input)
			if tt.wantNil {
				if m != nil {
					t.Errorf("Match(%q) = %+v, want nil", tt.input, m)
				}
				return
			}
			if m == nil {
				t.Fatalf("Match(%q) = nil, want non-nil", tt.input)
			}
			if tt.wantSource != 0 && m.Source != tt.wantSource {
				t.Errorf("Source = %v, want %v", m.Source, tt.wantSource)
			}
			if tt.wantOp != "" && m.Operation != tt.wantOp {
				t.Errorf("Operation = %q, want %q", m.Operation, tt.wantOp)
			}
			if m.AccountID != tt.wantAccount {
				t.Errorf("AccountID = %q, want %q", m.AccountID, tt.wantAccount)
			}
			if m.ProjectID != tt.wantProject {
				t.Errorf("ProjectID = %q, want %q", m.ProjectID, tt.wantProject)
			}
			if tt.wantRes != "" && m.ResourceID() != tt.wantRes {
				t.Errorf("ResourceID() = %q, want %q", m.ResourceID(), tt.wantRes)
			}
			if m.CommentID != tt.wantComment {
				t.Errorf("CommentID = %q, want %q", m.CommentID, tt.wantComment)
			}
			if tt.wantPath != "" && m.PathType != tt.wantPath {
				t.Errorf("PathType = %q, want %q", m.PathType, tt.wantPath)
			}
		})
	}
}

func TestMatchAPI(t *testing.T) {
	r := DefaultRouter()

	// Known API route should match
	m := r.MatchAPI("https://3.basecamp.com/123/buckets/456/todos/789")
	if m == nil {
		t.Fatal("MatchAPI should match known API route")
	}
	if m.Source != MatchedAPI {
		t.Errorf("Source = %v, want MatchedAPI", m.Source)
	}
	if m.Operation != "GetTodo" {
		t.Errorf("Operation = %q, want GetTodo", m.Operation)
	}

	// API URL with .json suffix should match
	m = r.MatchAPI("https://3.basecampapi.com/123/recordings/200/boosts.json")
	if m == nil {
		t.Fatal("MatchAPI should match .json API URL")
	}
	if m.Operation != "ListRecordingBoosts" {
		t.Errorf("Operation = %q, want ListRecordingBoosts", m.Operation)
	}

	// Web-only URL should NOT match
	m = r.MatchAPI("https://3.basecamp.com/123/buckets/456/sometype")
	if m != nil {
		t.Errorf("MatchAPI should return nil for web-only URL, got %+v", m)
	}
}

func TestMatchStructural(t *testing.T) {
	r := DefaultRouter()

	// Any Basecamp-shaped URL should match structurally
	m := r.MatchStructural("https://3.basecamp.com/123/buckets/456/todos/789")
	if m == nil {
		t.Fatal("MatchStructural should match")
	}
	if m.Source != MatchedStructural {
		t.Errorf("Source = %v, want MatchedStructural", m.Source)
	}
	if m.AccountID != "123" {
		t.Errorf("AccountID = %q, want 123", m.AccountID)
	}
	// Should NOT have Operation (structural only)
	if m.Operation != "" {
		t.Errorf("Operation = %q, want empty for structural match", m.Operation)
	}
}

func TestRouterMatchParams(t *testing.T) {
	r := DefaultRouter()

	m := r.Match("https://3.basecamp.com/123/buckets/456/todos/789")
	if m == nil {
		t.Fatal("expected match")
	}
	if m.Params["accountId"] != "123" {
		t.Errorf("Params[accountId] = %q, want %q", m.Params["accountId"], "123")
	}
	if m.Params["projectId"] != "456" {
		t.Errorf("Params[projectId] = %q, want %q", m.Params["projectId"], "456")
	}
	if m.Params["todoId"] != "789" {
		t.Errorf("Params[todoId] = %q, want %q", m.Params["todoId"], "789")
	}
}

func TestResourceIDNil(t *testing.T) {
	var m *Match
	if m.ResourceID() != "" {
		t.Errorf("nil Match.ResourceID() = %q, want empty", m.ResourceID())
	}
}
