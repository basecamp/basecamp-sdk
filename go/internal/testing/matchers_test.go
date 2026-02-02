package testing

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMatchAny(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/anything", nil)
	if !MatchAny(req) {
		t.Error("MatchAny should match any request")
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		reqPath  string
		expected bool
	}{
		{"exact match", "/projects.json", "/projects.json", true},
		{"no match", "/projects.json", "/users.json", false},
		{"path with id", "/projects/123.json", "/projects/123.json", true},
		{"missing leading slash", "/projects.json", "projects.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := MatchPath(tt.path)
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com"+tt.reqPath, nil)

			if matcher(req) != tt.expected {
				t.Errorf("MatchPath(%q) for path %q = %v, expected %v",
					tt.path, tt.reqPath, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchMethod(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		reqMethod string
		expected  bool
	}{
		{"exact match", "GET", "GET", true},
		{"case insensitive", "get", "GET", true},
		{"no match", "POST", "GET", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := MatchMethod(tt.method)
			req, _ := http.NewRequestWithContext(context.Background(), tt.reqMethod, "https://api.example.com/test", nil)

			if matcher(req) != tt.expected {
				t.Errorf("MatchMethod(%q) for method %q = %v, expected %v",
					tt.method, tt.reqMethod, matcher(req), tt.expected)
			}
		})
	}
}

func TestREST(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		path      string
		reqMethod string
		reqPath   string
		expected  bool
	}{
		{"GET match", "GET", "projects.json", "GET", "/projects.json", true},
		{"POST match", "POST", "projects.json", "POST", "/projects.json", true},
		{"method mismatch", "GET", "projects.json", "POST", "/projects.json", false},
		{"path mismatch", "GET", "projects.json", "GET", "/users.json", false},
		{"path with account id", "GET", "12345/projects.json", "GET", "/12345/projects.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := REST(tt.method, tt.path)
			req, _ := http.NewRequestWithContext(context.Background(), tt.reqMethod, "https://api.example.com"+tt.reqPath, nil)

			if matcher(req) != tt.expected {
				t.Errorf("REST(%q, %q) for %s %s = %v, expected %v",
					tt.method, tt.path, tt.reqMethod, tt.reqPath, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchPathPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		reqPath  string
		expected bool
	}{
		{"exact prefix", "/projects", "/projects/123", true},
		{"with json extension", "/projects", "/projects.json", true},
		{"no match", "/users", "/projects.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := MatchPathPrefix(tt.prefix)
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com"+tt.reqPath, nil)

			if matcher(req) != tt.expected {
				t.Errorf("MatchPathPrefix(%q) for path %q = %v, expected %v",
					tt.prefix, tt.reqPath, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		reqPath  string
		expected bool
	}{
		{"simple pattern", `/projects/\d+\.json`, "/projects/123.json", true},
		{"no match", `/projects/\d+\.json`, "/projects/abc.json", false},
		{"any bucket id", `/buckets/\d+/todos`, "/buckets/999/todos", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := MatchPathPattern(tt.pattern)
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com"+tt.reqPath, nil)

			if matcher(req) != tt.expected {
				t.Errorf("MatchPathPattern(%q) for path %q = %v, expected %v",
					tt.pattern, tt.reqPath, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchQuery(t *testing.T) {
	matcher := MatchQuery("GET", "projects.json", url.Values{
		"status": []string{"active"},
	})

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"with matching query", "https://api.example.com/projects.json?status=active", true},
		{"with extra query params", "https://api.example.com/projects.json?status=active&page=1", true},
		{"wrong query value", "https://api.example.com/projects.json?status=archived", false},
		{"missing query param", "https://api.example.com/projects.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", tt.url, nil)

			if matcher(req) != tt.expected {
				t.Errorf("MatchQuery for %s = %v, expected %v", tt.url, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchHeader(t *testing.T) {
	matcher := MatchHeader("Authorization", "Bearer token123")

	tests := []struct {
		name     string
		header   string
		value    string
		expected bool
	}{
		{"matching header", "Authorization", "Bearer token123", true},
		{"wrong value", "Authorization", "Bearer other", false},
		{"missing header", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}

			if matcher(req) != tt.expected {
				t.Errorf("MatchHeader for header=%q, value=%q = %v, expected %v",
					tt.header, tt.value, matcher(req), tt.expected)
			}
		})
	}
}

func TestAnd(t *testing.T) {
	matcher := And(
		MatchMethod("GET"),
		MatchPath("/projects.json"),
	)

	tests := []struct {
		name     string
		method   string
		path     string
		expected bool
	}{
		{"both match", "GET", "/projects.json", true},
		{"method mismatch", "POST", "/projects.json", false},
		{"path mismatch", "GET", "/users.json", false},
		{"both mismatch", "POST", "/users.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), tt.method, "https://api.example.com"+tt.path, nil)

			if matcher(req) != tt.expected {
				t.Errorf("And matcher for %s %s = %v, expected %v",
					tt.method, tt.path, matcher(req), tt.expected)
			}
		})
	}
}

func TestOr(t *testing.T) {
	matcher := Or(
		MatchPath("/projects.json"),
		MatchPath("/users.json"),
	)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"first match", "/projects.json", true},
		{"second match", "/users.json", true},
		{"neither match", "/todos.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com"+tt.path, nil)

			if matcher(req) != tt.expected {
				t.Errorf("Or matcher for path %s = %v, expected %v",
					tt.path, matcher(req), tt.expected)
			}
		})
	}
}

func TestWithHost(t *testing.T) {
	matcher := WithHost(MatchPath("/projects.json"), "api.basecamp.com")

	tests := []struct {
		name     string
		host     string
		path     string
		expected bool
	}{
		{"matching host and path", "api.basecamp.com", "/projects.json", true},
		{"wrong host", "evil.com", "/projects.json", false},
		{"wrong path", "api.basecamp.com", "/users.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://"+tt.host+tt.path, nil)

			if matcher(req) != tt.expected {
				t.Errorf("WithHost matcher for host=%s, path=%s = %v, expected %v",
					tt.host, tt.path, matcher(req), tt.expected)
			}
		})
	}
}

func TestMatchJSONBody(t *testing.T) {
	matcher := MatchJSONBody("POST", "projects.json", func(body map[string]interface{}) bool {
		name, ok := body["name"].(string)
		return ok && name == "New Project"
	})

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected bool
	}{
		{
			name:     "matching body",
			method:   "POST",
			path:     "/projects.json",
			body:     `{"name": "New Project"}`,
			expected: true,
		},
		{
			name:     "wrong name",
			method:   "POST",
			path:     "/projects.json",
			body:     `{"name": "Other Project"}`,
			expected: false,
		},
		{
			name:     "wrong method",
			method:   "GET",
			path:     "/projects.json",
			body:     `{"name": "New Project"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), tt.method, "https://api.example.com"+tt.path, strings.NewReader(tt.body))

			if matcher(req) != tt.expected {
				t.Errorf("MatchJSONBody for %s %s with body %s = %v, expected %v",
					tt.method, tt.path, tt.body, matcher(req), tt.expected)
			}
		})
	}
}
