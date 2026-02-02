package testing

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// MatchAny matches any request.
func MatchAny(*http.Request) bool {
	return true
}

// MatchPath returns a matcher that matches requests to the given path.
// The path should include a leading slash (e.g., "/projects.json").
// The path is matched against req.URL.EscapedPath().
func MatchPath(path string) Matcher {
	return func(req *http.Request) bool {
		return req.URL.EscapedPath() == path
	}
}

// MatchMethod returns a matcher that matches requests with the given HTTP method.
func MatchMethod(method string) Matcher {
	return func(req *http.Request) bool {
		return strings.EqualFold(req.Method, method)
	}
}

// REST returns a matcher that matches requests with the given HTTP method and path.
// The path should NOT include a leading slash; one will be added automatically.
// For example: REST("GET", "projects.json") matches GET /projects.json
func REST(method, path string) Matcher {
	return func(req *http.Request) bool {
		if !strings.EqualFold(req.Method, method) {
			return false
		}
		return req.URL.EscapedPath() == "/"+path
	}
}

// MatchPathPrefix returns a matcher that matches requests whose path starts with the given prefix.
func MatchPathPrefix(prefix string) Matcher {
	return func(req *http.Request) bool {
		return strings.HasPrefix(req.URL.EscapedPath(), prefix)
	}
}

// MatchPathPattern returns a matcher that matches requests whose path matches the given regex.
func MatchPathPattern(pattern string) Matcher {
	re := regexp.MustCompile(pattern)
	return func(req *http.Request) bool {
		return re.MatchString(req.URL.EscapedPath())
	}
}

// MatchHost returns a matcher that matches requests to the given host.
func MatchHost(host string) Matcher {
	return func(req *http.Request) bool {
		return strings.EqualFold(req.Host, host)
	}
}

// MatchQuery returns a matcher that matches requests with the given query parameters.
// Only the specified parameters are checked; additional query parameters are allowed.
func MatchQuery(method, path string, query url.Values) Matcher {
	return func(req *http.Request) bool {
		if !REST(method, path)(req) {
			return false
		}

		actualQuery := req.URL.Query()
		for param := range query {
			if actualQuery.Get(param) != query.Get(param) {
				return false
			}
		}
		return true
	}
}

// MatchHeader returns a matcher that matches requests with the given header value.
func MatchHeader(header, value string) Matcher {
	return func(req *http.Request) bool {
		return req.Header.Get(header) == value
	}
}

// And combines multiple matchers with AND logic.
// All matchers must match for the combined matcher to match.
func And(matchers ...Matcher) Matcher {
	return func(req *http.Request) bool {
		for _, m := range matchers {
			if !m(req) {
				return false
			}
		}
		return true
	}
}

// Or combines multiple matchers with OR logic.
// Any matcher matching will cause the combined matcher to match.
func Or(matchers ...Matcher) Matcher {
	return func(req *http.Request) bool {
		for _, m := range matchers {
			if m(req) {
				return true
			}
		}
		return false
	}
}

// WithHost decorates a matcher to also check the request host.
func WithHost(matcher Matcher, host string) Matcher {
	return And(matcher, MatchHost(host))
}

// readBody reads the request body and restores it for subsequent reads.
func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	bodyCopy := &bytes.Buffer{}
	r := io.TeeReader(req.Body, bodyCopy)
	req.Body = io.NopCloser(bodyCopy)
	return io.ReadAll(r)
}

// decodeJSONBody reads and decodes the JSON request body.
func decodeJSONBody(req *http.Request, dest interface{}) error {
	b, err := readBody(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// MatchJSONBody returns a matcher that matches requests with JSON body content.
// The callback receives the decoded JSON and returns true if it matches.
func MatchJSONBody(method, path string, cb func(body map[string]interface{}) bool) Matcher {
	return func(req *http.Request) bool {
		if !REST(method, path)(req) {
			return false
		}

		var bodyData map[string]interface{}
		if err := decodeJSONBody(req, &bodyData); err != nil {
			return false
		}

		return cb(bodyData)
	}
}
