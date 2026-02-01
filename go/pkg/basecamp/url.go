package basecamp

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed url-routes.json
var routeTableJSON []byte

// MatchSource indicates how a URL was recognized.
type MatchSource int

const (
	// MatchedAPI means the URL matched a route in the OpenAPI-derived route table.
	// Operation, Resource, and Params are populated.
	MatchedAPI MatchSource = iota + 1

	// MatchedStructural means the URL matched Basecamp's web URL conventions
	// but has no corresponding API route. Only IDs and PathType are populated.
	MatchedStructural
)

// Match holds the components extracted from a Basecamp URL.
type Match struct {
	// Source indicates how the URL was matched: API route table or structural convention.
	Source MatchSource

	// Operation is the matched API operation name (e.g., "GetTodo", "CreateMessage").
	// Empty for structural matches.
	Operation string

	// Operations lists all API operations for the matched pattern, keyed by HTTP method.
	// Nil for structural matches.
	Operations map[string]string

	// Resource is the API resource group (e.g., "Todos", "Messages").
	// Empty for structural matches.
	Resource string

	// PathType is the resource type segment from the URL path (e.g., "todos",
	// "messages", "cards", "columns"). Always populated when a bucket resource
	// is matched, whether by API route or structural fallback.
	PathType string

	// AccountID is the Basecamp account ID from the URL path.
	AccountID string

	// ProjectID is the bucket/project ID. Present for most resource URLs.
	ProjectID string

	// Params contains all named path parameters extracted from the URL.
	// Keys match the route table parameter names (e.g., "todoId", "messageId").
	// Nil for structural matches.
	Params map[string]string

	// CommentID is the comment ID extracted from a #__recording_{id} fragment.
	CommentID string

	// resourceID is the last non-account, non-project parameter value.
	resourceID string
}

// ResourceID returns the "primary" resource ID from the match — the last
// path parameter that isn't accountId or projectId. Returns empty string
// if no such parameter exists (e.g., for project or list URLs).
func (m *Match) ResourceID() string {
	if m == nil {
		return ""
	}
	return m.resourceID
}

// routeEntry is a compiled route from the route table.
type routeEntry struct {
	pattern    string
	resource   string
	operations map[string]string // HTTP method -> operation name
	regex      *regexp.Regexp
	params     []string // parameter names in order
}

// Router matches Basecamp URLs against the OpenAPI-derived route table,
// with structural fallbacks for web-only URLs.
type Router struct {
	routes []routeEntry
}

var (
	defaultRouter     *Router
	defaultRouterOnce sync.Once
)

// DefaultRouter returns a shared Router instance using the embedded route table.
func DefaultRouter() *Router {
	defaultRouterOnce.Do(func() {
		r, err := NewRouter(routeTableJSON)
		if err != nil {
			panic("basecamp: failed to load embedded route table: " + err.Error())
		}
		defaultRouter = r
	})
	return defaultRouter
}

// routeTable is the JSON schema for url-routes.json.
type routeTable struct {
	Routes []routeJSON `json:"routes"`
}

type routeJSON struct {
	Pattern    string            `json:"pattern"`
	Resource   string            `json:"resource"`
	Operations map[string]string `json:"operations"`
}

// NewRouter creates a Router from a JSON route table.
func NewRouter(tableJSON []byte) (*Router, error) {
	var table routeTable
	if err := json.Unmarshal(tableJSON, &table); err != nil {
		return nil, err
	}

	r := &Router{routes: make([]routeEntry, 0, len(table.Routes))}
	for _, entry := range table.Routes {
		compiled, params := compilePattern(entry.Pattern)
		r.routes = append(r.routes, routeEntry{
			pattern:    entry.Pattern,
			resource:   entry.Resource,
			operations: entry.Operations,
			regex:      compiled,
			params:     params,
		})
	}

	// Sort routes by specificity: more segments first, then alphabetically.
	sortRoutes(r.routes)

	return r, nil
}

// paramPattern matches {paramName} in route patterns.
var paramPattern = regexp.MustCompile(`\{([^}]+)\}`)

// compilePattern converts a route pattern like "/{accountId}/buckets/{projectId}/todos/{todoId}"
// into a regexp and extracts the parameter names.
func compilePattern(pattern string) (*regexp.Regexp, []string) {
	var params []string
	regexStr := "^"

	remaining := pattern
	for remaining != "" {
		loc := paramPattern.FindStringIndex(remaining)
		if loc == nil {
			regexStr += regexp.QuoteMeta(remaining)
			break
		}
		regexStr += regexp.QuoteMeta(remaining[:loc[0]])
		match := paramPattern.FindStringSubmatch(remaining[loc[0]:])
		params = append(params, match[1])
		regexStr += `([^/]+)`
		remaining = remaining[loc[0]+len(match[0]):]
	}
	regexStr += `$`

	return regexp.MustCompile(regexStr), params
}

// sortRoutes sorts routes by descending segment count, then alphabetically by pattern.
func sortRoutes(routes []routeEntry) {
	sort.Slice(routes, func(i, j int) bool {
		si := strings.Count(routes[i].pattern, "/")
		sj := strings.Count(routes[j].pattern, "/")
		if si != sj {
			return si > sj // More segments first
		}
		return routes[i].pattern < routes[j].pattern
	})
}

var (
	reCommentFrag = regexp.MustCompile(`__recording_(\d+)`)
	reDigitsOnly  = regexp.MustCompile(`^\d+$`)
)

// Structural fallback patterns for web-only URLs not in the API route table.
// These match the Basecamp URL conventions that the Rails app uses.
var (
	// card_tables sub-resources: /{account}/buckets/{project}/card_tables/{sub}/{id}
	reCardTableSub = regexp.MustCompile(`^/(\d+)/buckets/(\d+)/card_tables/([^/]+)/(\d+)`)
	// Generic recording: /{account}/buckets/{project}/{type}/{id}
	reRecording = regexp.MustCompile(`^/(\d+)/buckets/(\d+)/([^/]+)/(\d+)`)
	// Type list: /{account}/buckets/{project}/{type}
	reTypeList = regexp.MustCompile(`^/(\d+)/buckets/(\d+)/([^/]+)/?$`)
	// Project: /{account}/projects/{project}
	reProject = regexp.MustCompile(`^/(\d+)/projects/(\d+)(?:/|$)`)
	// Account only: /{account}
	reAccount = regexp.MustCompile(`^/(\d+)(?:/|$)`)
)

// Match parses a Basecamp URL and returns the matched route and extracted parameters.
// Returns nil if the URL does not look like a Basecamp URL.
//
// First tries the spec-derived API route table for a rich match (Source=MatchedAPI).
// Falls back to structural pattern matching for web-only URLs (Source=MatchedStructural).
func (r *Router) Match(rawURL string) *Match {
	path, fragment, ok := preprocessURL(rawURL)
	if !ok {
		return nil
	}

	// Try spec-derived API routes first.
	if m := r.matchAPIRoute(path, fragment); m != nil {
		m.PathType = derivePathType(path)
		return m
	}

	// Structural fallback for web-only URLs.
	return matchStructural(path, fragment)
}

// MatchAPI matches only against the spec-derived API route table.
// Returns nil if the URL doesn't correspond to a known API route.
// Use this when you need to resolve a URL to a specific API operation.
func (r *Router) MatchAPI(rawURL string) *Match {
	path, fragment, ok := preprocessURL(rawURL)
	if !ok {
		return nil
	}

	m := r.matchAPIRoute(path, fragment)
	if m != nil {
		m.PathType = derivePathType(path)
	}
	return m
}

// MatchStructural matches only against Basecamp's web URL conventions.
// Returns nil if the URL doesn't look like a Basecamp URL.
// Use this when you know you have a web URL and only need ID extraction.
func (r *Router) MatchStructural(rawURL string) *Match {
	path, fragment, ok := preprocessURL(rawURL)
	if !ok {
		return nil
	}
	return matchStructural(path, fragment)
}

// preprocessURL validates and parses a raw URL into path and fragment components.
// Returns (path, fragment, ok). Returns ok=false if the URL doesn't look like a Basecamp URL.
func preprocessURL(rawURL string) (path, fragment string, ok bool) {
	if !looksLikeBasecampURL(rawURL) {
		return "", "", false
	}

	urlPart := rawURL
	if idx := strings.Index(rawURL, "#"); idx != -1 {
		fragment = rawURL[idx+1:]
		urlPart = rawURL[:idx]
	}

	path = extractPath(urlPart)
	return path, fragment, true
}

// matchAPIRoute tries to match the path against the spec-derived route table.
func (r *Router) matchAPIRoute(path, fragment string) *Match {
	for i := range r.routes {
		rt := &r.routes[i]
		matches := rt.regex.FindStringSubmatch(path)
		if matches == nil {
			continue
		}

		m := &Match{
			Source:     MatchedAPI,
			Operations: rt.operations,
			Resource:   rt.resource,
			Params:     make(map[string]string, len(rt.params)),
		}

		// Pick the default operation: prefer GET, then first alphabetically.
		if op, ok := rt.operations["GET"]; ok {
			m.Operation = op
		} else {
			for _, op := range rt.operations {
				if m.Operation == "" || op < m.Operation {
					m.Operation = op
				}
			}
		}

		for j, paramName := range rt.params {
			val := matches[j+1]
			m.Params[paramName] = val
			switch paramName {
			case "accountId":
				m.AccountID = val
			case "projectId":
				m.ProjectID = val
			default:
				m.resourceID = val
			}
		}

		parseFragment(m, fragment)
		return m
	}
	return nil
}

// matchStructural matches Basecamp web URL conventions that may not exist
// in the API route table. Returns nil if no structural pattern matches.
func matchStructural(path, fragment string) *Match {
	// Card table sub-resources: card_tables/{sub}/{id}
	if matches := reCardTableSub.FindStringSubmatch(path); matches != nil {
		m := &Match{
			Source:     MatchedStructural,
			AccountID:  matches[1],
			ProjectID:  matches[2],
			PathType:   matches[3], // "cards", "columns", "lists", "steps"
			resourceID: matches[4],
		}
		// Normalize "lists" alias to "columns"
		if m.PathType == "lists" {
			m.PathType = "columns"
		}
		parseFragment(m, fragment)
		return m
	}

	// Generic recording: /{account}/buckets/{project}/{type}/{id}
	if matches := reRecording.FindStringSubmatch(path); matches != nil {
		m := &Match{
			Source:     MatchedStructural,
			AccountID:  matches[1],
			ProjectID:  matches[2],
			PathType:   matches[3],
			resourceID: matches[4],
		}
		parseFragment(m, fragment)
		return m
	}

	// Type list: /{account}/buckets/{project}/{type}
	if matches := reTypeList.FindStringSubmatch(path); matches != nil {
		m := &Match{
			Source:    MatchedStructural,
			AccountID: matches[1],
			ProjectID: matches[2],
			PathType:  matches[3],
		}
		parseFragment(m, fragment)
		return m
	}

	// Project: /{account}/projects/{project}
	if matches := reProject.FindStringSubmatch(path); matches != nil {
		m := &Match{
			Source:    MatchedStructural,
			AccountID: matches[1],
			ProjectID: matches[2],
			PathType:  "project",
		}
		parseFragment(m, fragment)
		return m
	}

	// Account only: /{account}
	if matches := reAccount.FindStringSubmatch(path); matches != nil {
		m := &Match{
			Source:    MatchedStructural,
			AccountID: matches[1],
		}
		parseFragment(m, fragment)
		return m
	}

	return nil
}

// derivePathType extracts the proximate resource type from a URL path.
// Uses Rails resourceful routing semantics: the resource you're accessing,
// which is the last non-numeric path segment.
//
// Examples:
//   - /123/buckets/456/todos/789 → "todos"
//   - /123/buckets/456/todosets/777/todolists → "todolists"
//   - /123/buckets/456/card_tables/789 → "card_tables"
//   - /123/buckets/456/card_tables/789/columns → "columns"
//   - /123/buckets/456/card_tables/cards/789 → "cards"
//   - /123/projects/456 → "project"
//   - /123/people/456 → "people" (account-level route)
func derivePathType(path string) string {
	// Special case for project URLs
	if strings.Contains(path, "/projects/") {
		return "project"
	}

	// Find the resource path: for bucket URLs, skip /buckets/{id}/
	// For account-level URLs, skip /{accountId}/
	var resourcePath string
	if idx := strings.Index(path, "/buckets/"); idx != -1 {
		rest := path[idx+len("/buckets/"):]
		// Skip the bucket ID
		if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
			resourcePath = rest[slashIdx+1:]
		} else {
			return "" // Just /buckets/{id}, no resource path
		}
	} else if len(path) > 1 && path[0] == '/' {
		// Account-level path: /{accountId}/resource/...
		// Skip the account ID segment
		rest := path[1:]
		if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
			firstSeg := rest[:slashIdx]
			if isNumeric(firstSeg) {
				resourcePath = rest[slashIdx+1:]
			}
		}
	}

	if resourcePath == "" {
		return ""
	}

	// Find the proximate resource: last non-numeric segment
	segments := strings.Split(strings.TrimRight(resourcePath, "/"), "/")
	lastType := ""
	for _, seg := range segments {
		if !isNumeric(seg) {
			lastType = seg
		}
	}

	// Normalize aliases
	if lastType == "lists" {
		lastType = "columns"
	}

	return lastType
}

// isNumeric returns true if s contains only ASCII digits.
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// extractPath strips the scheme+host, query string, and trailing slash from a URL.
// Handles full URLs (https://host/path), scheme-less (host/path), and bare paths (/path).
func extractPath(urlPart string) string {
	path := urlPart

	// Strip scheme if present: https://host/path → host/path
	if idx := strings.Index(urlPart, "://"); idx != -1 {
		path = urlPart[idx+3:]
	} else if strings.HasPrefix(urlPart, "//") {
		// Protocol-relative: //host/path → host/path
		path = urlPart[2:]
	}

	// Strip host if present: host/path → /path
	// Look for first slash that starts the path (skip if path already starts with /)
	if len(path) > 0 && path[0] != '/' {
		if slashIdx := strings.Index(path, "/"); slashIdx != -1 {
			path = path[slashIdx:]
		} else {
			return "" // Just a hostname, no path
		}
	}

	// Strip query string
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	return strings.TrimRight(path, "/")
}

func parseFragment(m *Match, fragment string) {
	if fragment == "" {
		return
	}
	if matches := reCommentFrag.FindStringSubmatch(fragment); matches != nil {
		m.CommentID = matches[1]
	} else if reDigitsOnly.MatchString(fragment) {
		m.CommentID = fragment
	}
}

// looksLikeBasecampURL is a cheap heuristic to short-circuit non-Basecamp inputs.
// Accepts:
//   - Any URL containing "basecamp" in the host (production, staging, beta, etc.)
//   - Any URL with Basecamp path conventions (/buckets/ or /projects/)
//   - Any URL with path shape /{digits}/... (account-level routes on localhost/staging)
//   - Scheme-less URLs like "3.basecamp.com/..." or "localhost:3000/123/..."
func looksLikeBasecampURL(input string) bool {
	// Quick accept for any basecamp domain
	if strings.Contains(input, "basecamp") {
		return true
	}

	// Check for Basecamp path conventions
	if strings.Contains(input, "/buckets/") || strings.Contains(input, "/projects/") {
		return true
	}

	// Check for account-level path shape: /{digits}/ or /{digits} at end
	// This catches staging/localhost URLs without /buckets/ or /projects/
	path := extractPath(input)
	if len(path) > 1 && path[0] == '/' {
		// Find end of first segment
		end := strings.Index(path[1:], "/")
		if end == -1 {
			end = len(path) - 1
		}
		firstSeg := path[1 : end+1]
		if isNumeric(firstSeg) {
			return true
		}
	}

	return false
}
