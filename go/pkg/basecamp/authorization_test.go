package basecamp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizationService_GetInfo(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		statusCode int
		opts       *GetInfoOptions
		wantErr    bool
		wantCount  int // expected number of accounts after filtering
	}{
		{
			name: "successful response",
			response: map[string]interface{}{
				"identity": map[string]interface{}{
					"id":            123,
					"first_name":    "Test",
					"last_name":     "User",
					"email_address": "test@example.com",
				},
				"accounts": []map[string]interface{}{
					{"id": 1, "name": "Account 1", "product": "bc3"},
					{"id": 2, "name": "Account 2", "product": "hey"},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			wantCount:  2,
		},
		{
			name: "filter by product",
			response: map[string]interface{}{
				"identity": map[string]interface{}{
					"id":         123,
					"first_name": "Test",
				},
				"accounts": []map[string]interface{}{
					{"id": 1, "name": "Basecamp Account", "product": "bc3"},
					{"id": 2, "name": "HEY Account", "product": "hey"},
					{"id": 3, "name": "Another BC", "product": "bc3"},
				},
			},
			statusCode: http.StatusOK,
			opts:       &GetInfoOptions{FilterProduct: "bc3"},
			wantErr:    false,
			wantCount:  2, // Only bc3 accounts
		},
		{
			name:       "unauthorized",
			response:   map[string]interface{}{"error": "invalid token"},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "server error",
			response:   "Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authorization header
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("unexpected Authorization header: %s", auth)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else if s, ok := tt.response.(string); ok {
					_, _ = w.Write([]byte(s))
				} else {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.BaseURL = server.URL
			token := &StaticTokenProvider{Token: "test-token"}
			client := NewClient(cfg, token, WithHTTPClient(server.Client()))

			opts := tt.opts
			if opts == nil {
				opts = &GetInfoOptions{}
			}
			opts.Endpoint = server.URL + "/authorization.json"

			info, err := client.Authorization().GetInfo(t.Context(), opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if info == nil {
					t.Error("GetInfo() returned nil info")
					return
				}
				if len(info.Accounts) != tt.wantCount {
					t.Errorf("GetInfo() returned %d accounts, want %d", len(info.Accounts), tt.wantCount)
				}
			}
		})
	}
}

func TestAuthorizationInfo_Unmarshal(t *testing.T) {
	jsonData := `{
		"expires_at": "2024-01-15T10:30:00Z",
		"identity": {
			"id": 12345,
			"first_name": "John",
			"last_name": "Doe",
			"email_address": "john@example.com"
		},
		"accounts": [
			{
				"id": 1001,
				"name": "My Company",
				"product": "bc3",
				"href": "https://3.basecampapi.com/1001",
				"app_href": "https://3.basecamp.com/1001",
				"hidden": false,
				"expired": false,
				"featured": true
			},
			{
				"id": 1002,
				"name": "Side Project",
				"product": "bc3",
				"href": "https://3.basecampapi.com/1002",
				"app_href": "https://3.basecamp.com/1002"
			}
		]
	}`

	var info AuthorizationInfo
	if err := json.Unmarshal([]byte(jsonData), &info); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if info.Identity.ID != 12345 {
		t.Errorf("Identity.ID = %d, want 12345", info.Identity.ID)
	}
	if info.Identity.FirstName != "John" {
		t.Errorf("Identity.FirstName = %q, want %q", info.Identity.FirstName, "John")
	}
	if info.Identity.EmailAddress != "john@example.com" {
		t.Errorf("Identity.EmailAddress = %q, want %q", info.Identity.EmailAddress, "john@example.com")
	}
	if len(info.Accounts) != 2 {
		t.Errorf("len(Accounts) = %d, want 2", len(info.Accounts))
	}
	if info.Accounts[0].Name != "My Company" {
		t.Errorf("Accounts[0].Name = %q, want %q", info.Accounts[0].Name, "My Company")
	}
	if !info.Accounts[0].Featured {
		t.Error("Accounts[0].Featured = false, want true")
	}
}
