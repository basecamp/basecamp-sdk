package basecamp

import (
	"testing"
)

func TestForAccount_Validation(t *testing.T) {
	cfg := DefaultConfig()
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	tests := []struct {
		name      string
		accountID string
		wantPanic string
	}{
		{
			name:      "empty account ID",
			accountID: "",
			wantPanic: "basecamp: ForAccount requires non-empty account ID",
		},
		{
			name:      "non-numeric account ID",
			accountID: "abc",
			wantPanic: "basecamp: ForAccount requires numeric account ID, got: abc",
		},
		{
			name:      "account ID with letters",
			accountID: "123abc",
			wantPanic: "basecamp: ForAccount requires numeric account ID, got: 123abc",
		},
		{
			name:      "account ID with special chars",
			accountID: "123-456",
			wantPanic: "basecamp: ForAccount requires numeric account ID, got: 123-456",
		},
		{
			name:      "valid numeric account ID",
			accountID: "12345",
			wantPanic: "", // no panic expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic == "" {
					if r != nil {
						t.Errorf("ForAccount(%q) panicked unexpectedly: %v", tt.accountID, r)
					}
				} else {
					if r == nil {
						t.Errorf("ForAccount(%q) did not panic, expected: %s", tt.accountID, tt.wantPanic)
					} else if r != tt.wantPanic {
						t.Errorf("ForAccount(%q) panic = %v, want %s", tt.accountID, r, tt.wantPanic)
					}
				}
			}()

			ac := client.ForAccount(tt.accountID)
			if tt.wantPanic == "" && ac.AccountID() != tt.accountID {
				t.Errorf("AccountID() = %q, want %q", ac.AccountID(), tt.accountID)
			}
		})
	}
}

func TestAccountPath_DoublePrefixGuard(t *testing.T) {
	cfg := DefaultConfig()
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	ac := client.ForAccount("12345")

	tests := []struct {
		name string
		path string
		want string
	}{
		// Normal paths - should be prefixed
		{
			name: "simple path",
			path: "/projects.json",
			want: "/12345/projects.json",
		},
		{
			name: "path without leading slash",
			path: "projects.json",
			want: "/12345/projects.json",
		},
		{
			name: "nested path",
			path: "/buckets/123/todolists.json",
			want: "/12345/buckets/123/todolists.json",
		},

		// Already prefixed - should NOT double-prefix
		{
			name: "already prefixed with trailing path",
			path: "/12345/projects.json",
			want: "/12345/projects.json",
		},
		{
			name: "already prefixed exact",
			path: "/12345",
			want: "/12345",
		},
		{
			name: "already prefixed with query string",
			path: "/12345?foo=bar",
			want: "/12345?foo=bar",
		},
		{
			name: "already prefixed path with query",
			path: "/12345/projects.json?status=active",
			want: "/12345/projects.json?status=active",
		},

		// Similar but different account ID - should be prefixed
		{
			name: "different account ID",
			path: "/99999/projects.json",
			want: "/12345/99999/projects.json",
		},
		{
			name: "partial match not same account",
			path: "/123456/projects.json", // 123456 != 12345
			want: "/12345/123456/projects.json",
		},

		// Absolute URLs - should be unchanged
		{
			name: "absolute http URL",
			path: "http://example.com/12345/projects.json",
			want: "http://example.com/12345/projects.json",
		},
		{
			name: "absolute https URL",
			path: "https://3.basecampapi.com/12345/projects.json",
			want: "https://3.basecampapi.com/12345/projects.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ac.accountPath(tt.path)
			if got != tt.want {
				t.Errorf("accountPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestForAccount_ReturnsLightweightClient(t *testing.T) {
	cfg := DefaultConfig()
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	// Create multiple AccountClients - they should share the same parent
	ac1 := client.ForAccount("11111")
	ac2 := client.ForAccount("22222")

	// Verify they have different account IDs
	if ac1.AccountID() == ac2.AccountID() {
		t.Errorf("AccountClients should have different account IDs")
	}

	// Verify they share the same parent
	if ac1.parent != ac2.parent {
		t.Errorf("AccountClients should share the same parent Client")
	}

	// Verify the parent's generated client is shared (initialized via sync.Once)
	if ac1.parent.gen != ac2.parent.gen {
		t.Errorf("AccountClients should share the same generated client")
	}
}
