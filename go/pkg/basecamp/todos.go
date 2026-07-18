package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// DefaultTodoLimit is the default number of todos to return when no limit is specified.
const DefaultTodoLimit = 100

// Todo represents a Basecamp todo item.
type Todo struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	VisibleTo   []int64    `json:"visible_to"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Title       string     `json:"title"`
	InheritsVis bool       `json:"inherits_status"`
	Type        string     `json:"type"`
	URL         string     `json:"url"`
	AppURL      string     `json:"app_url"`
	BookmarkURL string     `json:"bookmark_url"`
	Parent      *Parent    `json:"parent,omitempty"`
	Bucket      *Bucket    `json:"bucket,omitempty"`
	Creator     *Person    `json:"creator,omitempty"`
	Content     string     `json:"content"`
	Description string     `json:"description"`
	StartsOn    string     `json:"starts_on,omitempty"`
	DueOn       string     `json:"due_on,omitempty"`
	Completed   bool       `json:"completed"`
	BoostsCount int        `json:"boosts_count,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Completer   *Person    `json:"completer,omitempty"`
	Assignees   []Person   `json:"assignees,omitempty"`
	// CompletionSubscribers distinguishes present-but-empty from absent:
	// a non-nil zero-length slice means the server sent an empty list,
	// nil means the property was absent from the response. Deliberately
	// not omitempty so re-encoding keeps the field visible either way
	// (nil marshals as null, empty as []) instead of dropping it.
	CompletionSubscribers []Person `json:"completion_subscribers"`
	CommentsCount         int      `json:"comments_count,omitempty"`
	Position              int      `json:"position"`
}

// Person represents a Basecamp user or system actor.
// For system actors (personable_type "LocalPerson"), ID is 0. SystemLabel
// holds the original non-numeric identifier (e.g. "basecamp") when the
// response is processed through normalizeJSON (currently notifications).
// Endpoints that decode through the generated parser lose the label —
// use PersonableType == "LocalPerson" as the authoritative discriminator.
type Person struct {
	ID                int64          `json:"id"`
	SystemLabel       string         `json:"system_label,omitempty"`
	AttachableSGID    string         `json:"attachable_sgid,omitempty"`
	Name              string         `json:"name"`
	EmailAddress      string         `json:"email_address,omitempty"`
	PersonableType    string         `json:"personable_type,omitempty"`
	Title             string         `json:"title,omitempty"`
	Bio               string         `json:"bio,omitempty"`
	Location          string         `json:"location,omitempty"`
	CreatedAt         string         `json:"created_at,omitempty"`
	UpdatedAt         string         `json:"updated_at,omitempty"`
	Admin             bool           `json:"admin,omitempty"`
	Owner             bool           `json:"owner,omitempty"`
	Client            bool           `json:"client,omitempty"`
	Employee          bool           `json:"employee,omitempty"`
	TimeZone          string         `json:"time_zone,omitempty"`
	AvatarURL         string         `json:"avatar_url,omitempty"`
	CanPing           bool           `json:"can_ping,omitempty"`
	Company           *PersonCompany `json:"company,omitempty"`
	CanManageProjects bool           `json:"can_manage_projects,omitempty"`
	CanManagePeople   bool           `json:"can_manage_people,omitempty"`
}

// PersonCompany represents a company associated with a person.
type PersonCompany struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Parent represents the parent object of a todo.
type Parent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	AppURL string `json:"app_url"`
}

// Bucket represents the project (bucket) containing a todo.
type Bucket struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TodoListOptions specifies options for listing todos.
type TodoListOptions struct {
	// Status filters by recording lifecycle: "archived" or "trashed".
	// Omit for the API default — incomplete todos with status inherited
	// from the parent list. Unsupported values are rejected at the wrapper
	// boundary (the BC3 server silently coerces them to nil, so we fail
	// fast to surface bugs instead of returning unexpectedly-default
	// results).
	Status string

	// Completed, when true, returns only completed todos.
	// May be combined with Status (e.g. Status="archived", Completed=true
	// to list archived completed todos).
	Completed bool

	// Limit is the maximum number of todos to return.
	// If 0, uses DefaultTodoLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// TodoListResult contains the results from listing todos.
type TodoListResult struct {
	// Todos is the list of todos returned.
	Todos []Todo
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// CreateTodoRequest specifies the parameters for creating a todo.
type CreateTodoRequest struct {
	// Content is the todo text (required).
	Content string `json:"content"`
	// Description is an optional extended description (can include HTML).
	Description string `json:"description,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this todo to.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
	// CompletionSubscriberIDs is a list of person IDs to notify on completion.
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids,omitempty"`
	// Notify when true, will notify assignees.
	Notify bool `json:"notify,omitempty"`
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD).
	DueOn string `json:"due_on,omitempty"`
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD).
	StartsOn string `json:"starts_on,omitempty"`
}

// UpdateTodoRequest specifies the fields to set when updating a todo.
// Zero-value fields are left untouched (see TodosService.Update).
type UpdateTodoRequest struct {
	// Content is the todo text.
	Content string `json:"content,omitempty"`
	// Description is an optional extended description (can include HTML).
	Description string `json:"description,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this todo to.
	// A non-nil empty slice clears assignees; nil leaves them untouched.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
	// CompletionSubscriberIDs is a list of person IDs to notify on completion.
	// A non-nil empty slice clears subscribers; nil leaves them untouched.
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids,omitempty"`
	// Notify when true, will notify assignees.
	Notify bool `json:"notify,omitempty"`
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD).
	DueOn string `json:"due_on,omitempty"`
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD).
	StartsOn string `json:"starts_on,omitempty"`
}

// ReplaceTodoRequest specifies the new complete representation of a todo
// for TodosService.Replace. Omitted fields are cleared server-side.
type ReplaceTodoRequest struct {
	// Content is the todo text (required).
	Content string `json:"content"`
	// Description is an optional extended description (can include HTML).
	Description string `json:"description,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this todo to.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
	// CompletionSubscriberIDs is a list of person IDs to notify on completion.
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids,omitempty"`
	// Notify when true, will notify assignees.
	Notify bool `json:"notify,omitempty"`
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD).
	DueOn string `json:"due_on,omitempty"`
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD).
	StartsOn string `json:"starts_on,omitempty"`
}

// TodoFields holds a todo's full writable state for TodosService.Update
// and TodosService.Edit. The whole struct is PUT back to the server, so
// clearing a field means setting it empty ("" for strings and dates, an
// empty slice for ID lists) — there is no third state.
type TodoFields struct {
	// Content is the todo text (required; the server rejects an empty one).
	Content string
	// Description is an extended description (can include HTML). "" clears it.
	Description string
	// AssigneeIDs is the complete list of assigned person IDs. Empty clears.
	AssigneeIDs []int64
	// CompletionSubscriberIDs is the complete list of person IDs notified on
	// completion. Empty clears.
	CompletionSubscriberIDs []int64
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD). "" clears it.
	DueOn string
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD). "" clears it.
	StartsOn string
	// Notify is a send directive, not todo state: it is never populated
	// from the current todo and is sent only when true, asking the server
	// to notify assignees about this write.
	Notify bool
}

// fieldsFromTodo derives the full writable state from a fetched todo.
func fieldsFromTodo(t *Todo) *TodoFields {
	f := &TodoFields{
		Content:                 t.Content,
		Description:             t.Description,
		AssigneeIDs:             make([]int64, 0, len(t.Assignees)),
		CompletionSubscriberIDs: make([]int64, 0, len(t.CompletionSubscribers)),
		DueOn:                   t.DueOn,
		StartsOn:                t.StartsOn,
	}
	for _, p := range t.Assignees {
		f.AssigneeIDs = append(f.AssigneeIDs, p.ID)
	}
	for _, p := range t.CompletionSubscribers {
		f.CompletionSubscriberIDs = append(f.CompletionSubscriberIDs, p.ID)
	}
	return f
}

// fullBody serializes the complete writable state for the replace
// transport: content, description, and both ID lists are always sent
// (empties included, so clears survive the PUT); dates are sent only when
// non-empty (the server clears an omitted date, and "" is a format error);
// notify is sent only when true.
func (f *TodoFields) fullBody() (map[string]any, error) {
	if f.Content == "" {
		return nil, ErrUsage("todo content is required")
	}
	assigneeIDs := f.AssigneeIDs
	if assigneeIDs == nil {
		assigneeIDs = []int64{}
	}
	subscriberIDs := f.CompletionSubscriberIDs
	if subscriberIDs == nil {
		subscriberIDs = []int64{}
	}
	body := map[string]any{
		"content":                   f.Content,
		"description":               f.Description,
		"assignee_ids":              assigneeIDs,
		"completion_subscriber_ids": subscriberIDs,
	}
	if f.DueOn != "" {
		if _, parseErr := types.ParseDate(f.DueOn); parseErr != nil {
			return nil, ErrUsage("todo due_on must be in YYYY-MM-DD format")
		}
		body["due_on"] = f.DueOn
	}
	if f.StartsOn != "" {
		if _, parseErr := types.ParseDate(f.StartsOn); parseErr != nil {
			return nil, ErrUsage("todo starts_on must be in YYYY-MM-DD format")
		}
		body["starts_on"] = f.StartsOn
	}
	if f.Notify {
		body["notify"] = true
	}
	return body, nil
}

// TodosService handles todo operations.
type TodosService struct {
	client *AccountClient
}

// NewTodosService creates a new TodosService.
func NewTodosService(client *AccountClient) *TodosService {
	return &TodosService{client: client}
}

// List returns todos in a todolist.
//
// By default, returns up to 100 todos. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of todos to return (0 = 100, -1 = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned TodoListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TodosService) List(ctx context.Context, todolistID int64, opts *TodoListOptions) (result *TodoListResult, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "List",
		ResourceType: "todo", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if opts != nil && opts.Status != "" && opts.Status != "archived" && opts.Status != "trashed" {
		err = ErrUsage(fmt.Sprintf("todo list status must be empty, %q, or %q (got %q)", "archived", "trashed", opts.Status))
		return nil, err
	}

	// Build params for generated client. Status and Completed are orthogonal
	// upstream: Status filters by recording lifecycle (archived/trashed),
	// Completed=true narrows to completed todos, and they may be combined.
	var params *generated.ListTodosParams
	if opts != nil && (opts.Status != "" || opts.Completed) {
		params = &generated.ListTodosParams{Status: opts.Status, Completed: opts.Completed}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListTodosWithResponse(ctx, s.client.accountID, todolistID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var todos []Todo
	if resp.JSON200 != nil {
		for _, gt := range *resp.JSON200 {
			todos = append(todos, todoFromGenerated(gt))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &TodoListResult{Todos: todos, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultTodoLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(todos) >= limit {
		return &TodoListResult{Todos: todos[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(todos), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(todos), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gt generated.Todo
		if err := json.Unmarshal(raw, &gt); err != nil {
			return nil, fmt.Errorf("failed to parse todo: %w", err)
		}
		todos = append(todos, todoFromGenerated(gt))
	}

	return &TodoListResult{Todos: todos, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a todo by ID.
func (s *TodosService) Get(ctx context.Context, todoID int64) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Get",
		ResourceType: "todo", IsMutation: false,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTodoWithResponse(ctx, s.client.accountID, todoID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	todo := todoFromGenerated(*resp.JSON200)
	return &todo, nil
}

// Create creates a new todo in a todolist.
// Returns the created todo.
func (s *TodosService) Create(ctx context.Context, todolistID int64, req *CreateTodoRequest) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Create",
		ResourceType: "todo", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req.Content == "" {
		err = ErrUsage("todo content is required")
		return nil, err
	}

	body := generated.CreateTodoJSONRequestBody{
		Content:                 req.Content,
		Description:             req.Description,
		AssigneeIds:             req.AssigneeIDs,
		CompletionSubscriberIds: req.CompletionSubscriberIDs,
		Notify:                  &req.Notify,
	}
	// Parse date strings to types.Date for the generated client
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("todo due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}
	if req.StartsOn != "" {
		d, parseErr := types.ParseDate(req.StartsOn)
		if parseErr != nil {
			err = ErrUsage("todo starts_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.StartsOn = d
	}

	resp, err := s.client.parent.gen.CreateTodoWithResponse(ctx, s.client.accountID, todolistID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	todo := todoFromGenerated(*resp.JSON201)
	return &todo, nil
}

// Update sets the given fields on a todo and preserves everything else:
// it GETs the current todo, overlays the explicitly-set request fields,
// and PUTs the full representation back. A zero-value field is untouched,
// guaranteed. A non-nil empty ID slice is an explicit set (clears).
// Strings and dates cannot be cleared through Update — use Edit or
// Replace to clear.
//
// Hooks observe the two wire operations (Todos.Get then Todos.Replace),
// not a synthetic composite.
//
// Update is read-modify-write, not atomic: there is no conditional-update
// signal on this endpoint, so a concurrent write between the GET and PUT
// is overwritten — last write wins for the whole representation. The
// window is one round-trip. Use Replace to overwrite deliberately.
// Returns the updated todo.
func (s *TodosService) Update(ctx context.Context, todoID int64, req *UpdateTodoRequest) (*Todo, error) {
	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	current, err := s.Get(ctx, todoID)
	if err != nil {
		return nil, err
	}

	fields := fieldsFromTodo(current)
	if req.Content != "" {
		fields.Content = req.Content
	}
	if req.Description != "" {
		fields.Description = req.Description
	}
	if req.AssigneeIDs != nil {
		fields.AssigneeIDs = req.AssigneeIDs
	}
	if req.CompletionSubscriberIDs != nil {
		fields.CompletionSubscriberIDs = req.CompletionSubscriberIDs
	}
	if req.DueOn != "" {
		fields.DueOn = req.DueOn
	}
	if req.StartsOn != "" {
		fields.StartsOn = req.StartsOn
	}
	if req.Notify {
		fields.Notify = true
	}

	return s.replaceTodo(ctx, todoID, fields.fullBody)
}

// Edit applies a read-modify-write closure to a todo: it GETs the current
// todo, hands fn the full writable representation, and PUTs the whole
// thing back. Clearing a field means setting it empty ("", or an empty
// slice) — an untouched field keeps its current value. If fn returns an
// error, the edit aborts and nothing is written.
//
// Hooks observe the two wire operations (Todos.Get then Todos.Replace),
// not a synthetic composite.
//
// Edit is read-modify-write, not atomic: there is no conditional-update
// signal on this endpoint, so a concurrent write between the GET and PUT
// is overwritten — last write wins for the whole representation. The
// window is one round-trip. Use Replace to overwrite deliberately.
// Returns the updated todo.
func (s *TodosService) Edit(ctx context.Context, todoID int64, fn func(*TodoFields) error) (*Todo, error) {
	if fn == nil {
		return nil, ErrUsage("edit function is required")
	}

	current, err := s.Get(ctx, todoID)
	if err != nil {
		return nil, err
	}

	fields := fieldsFromTodo(current)
	if err := fn(fields); err != nil {
		return nil, err
	}

	return s.replaceTodo(ctx, todoID, fields.fullBody)
}

// Replace sends the request verbatim as the todo's new complete
// representation — the server's native PUT semantics. No GET is issued,
// and any field omitted from the request is cleared server-side (empty or
// missing assignee_ids clears assignees, a missing description clears it,
// and so on). Content is required. Use Update or Edit to preserve
// unspecified fields.
// Returns the updated todo.
func (s *TodosService) Replace(ctx context.Context, todoID int64, req *ReplaceTodoRequest) (*Todo, error) {
	return s.replaceTodo(ctx, todoID, func() (map[string]any, error) {
		if req == nil {
			return nil, ErrUsage("replace request is required")
		}
		if req.Content == "" {
			return nil, ErrUsage("todo content is required")
		}
		body := map[string]any{"content": req.Content}
		if req.Description != "" {
			body["description"] = req.Description
		}
		if req.AssigneeIDs != nil {
			body["assignee_ids"] = req.AssigneeIDs
		}
		if req.CompletionSubscriberIDs != nil {
			body["completion_subscriber_ids"] = req.CompletionSubscriberIDs
		}
		if req.Notify {
			body["notify"] = true
		}
		if req.DueOn != "" {
			if _, parseErr := types.ParseDate(req.DueOn); parseErr != nil {
				return nil, ErrUsage("todo due_on must be in YYYY-MM-DD format")
			}
			body["due_on"] = req.DueOn
		}
		if req.StartsOn != "" {
			if _, parseErr := types.ParseDate(req.StartsOn); parseErr != nil {
				return nil, ErrUsage("todo starts_on must be in YYYY-MM-DD format")
			}
			body["starts_on"] = req.StartsOn
		}
		return body, nil
	})
}

// replaceTodo is the single transport for the ReplaceTodo wire operation,
// shared by Replace, Update, and Edit. It owns the Todos.Replace hook
// envelope and the one generated-client call site; buildBody runs inside
// the envelope so usage errors are observable to hooks.
func (s *TodosService) replaceTodo(ctx context.Context, todoID int64, buildBody func() (map[string]any, error)) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Replace",
		ResourceType: "todo", IsMutation: true,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body, err := buildBody()
	if err != nil {
		return nil, err
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.ReplaceTodoWithBodyWithResponse(ctx, s.client.accountID, todoID, "application/json", bodyReader)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	todo := todoFromGenerated(*resp.JSON200)
	return &todo, nil
}

// Trash moves a todo to the trash.
// Trashed todos can be recovered from the trash.
func (s *TodosService) Trash(ctx context.Context, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Trash",
		ResourceType: "todo", IsMutation: true,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashTodoWithResponse(ctx, s.client.accountID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Complete marks a todo as completed.
func (s *TodosService) Complete(ctx context.Context, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Complete",
		ResourceType: "todo", IsMutation: true,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.CompleteTodoWithResponse(ctx, s.client.accountID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Uncomplete marks a completed todo as incomplete (reopens it).
func (s *TodosService) Uncomplete(ctx context.Context, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Uncomplete",
		ResourceType: "todo", IsMutation: true,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UncompleteTodoWithResponse(ctx, s.client.accountID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Reposition changes the position of a todo within its todolist.
// position is 1-based (1 = first position).
// parentID, if non-nil, moves the todo to a different todolist within the same project.
func (s *TodosService) Reposition(ctx context.Context, todoID int64, position int, parentID *int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Reposition",
		ResourceType: "todo", IsMutation: true,
		ResourceID: todoID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}

	body := generated.RepositionTodoJSONRequestBody{
		Position: int32(position), // #nosec G115 -- position is validated and bounded by API
		ParentId: parentID,
	}
	resp, err := s.client.parent.gen.RepositionTodoWithResponse(ctx, s.client.accountID, todoID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// todoFromGenerated converts a generated Todo to our clean Todo type.
func todoFromGenerated(gt generated.Todo) Todo {
	t := Todo{
		Status:        gt.Status,
		Title:         gt.Title,
		Type:          gt.Type,
		URL:           gt.Url,
		AppURL:        gt.AppUrl,
		BookmarkURL:   gt.BookmarkUrl,
		Content:       gt.Content,
		Description:   gt.Description,
		Completed:     gt.Completed,
		Position:      int(gt.Position),
		CreatedAt:     gt.CreatedAt,
		UpdatedAt:     gt.UpdatedAt,
		InheritsVis:   gt.InheritsStatus,
		BoostsCount:   int(gt.BoostsCount),
		CommentsCount: int(gt.CommentsCount),
	}

	if gt.Id != 0 {
		t.ID = gt.Id
	}

	// Convert date fields to strings
	if !gt.StartsOn.IsZero() {
		t.StartsOn = gt.StartsOn.String()
	}
	if !gt.DueOn.IsZero() {
		t.DueOn = gt.DueOn.String()
	}

	// Convert nested types
	if gt.Parent.Id != 0 || gt.Parent.Title != "" {
		t.Parent = &Parent{
			ID:     gt.Parent.Id,
			Title:  gt.Parent.Title,
			Type:   gt.Parent.Type,
			URL:    gt.Parent.Url,
			AppURL: gt.Parent.AppUrl,
		}
	}

	if gt.Bucket.Id != 0 || gt.Bucket.Name != "" {
		t.Bucket = &Bucket{
			ID:   gt.Bucket.Id,
			Name: gt.Bucket.Name,
			Type: gt.Bucket.Type,
		}
	}

	if gt.Creator.Id != 0 || gt.Creator.Name != "" {
		t.Creator = &Person{
			ID:           int64(gt.Creator.Id),
			Name:         gt.Creator.Name,
			EmailAddress: gt.Creator.EmailAddress,
			AvatarURL:    gt.Creator.AvatarUrl,
			Admin:        gt.Creator.Admin,
			Owner:        gt.Creator.Owner,
		}
	}

	// Convert assignees
	if len(gt.Assignees) > 0 {
		t.Assignees = make([]Person, 0, len(gt.Assignees))
		for _, ga := range gt.Assignees {
			t.Assignees = append(t.Assignees, personFromGenerated(ga))
		}
	}

	// Convert completion subscribers, preserving the nil-vs-empty
	// distinction: a server-sent [] becomes a non-nil zero-length slice,
	// an absent property stays nil.
	if gt.CompletionSubscribers != nil {
		t.CompletionSubscribers = make([]Person, 0, len(gt.CompletionSubscribers))
		for _, gs := range gt.CompletionSubscribers {
			t.CompletionSubscribers = append(t.CompletionSubscribers, personFromGenerated(gs))
		}
	}

	return t
}
