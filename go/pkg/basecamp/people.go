package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// UpdateProjectAccessRequest specifies the parameters for updating project access.
type UpdateProjectAccessRequest struct {
	// Grant is a list of person IDs to grant access to the project.
	Grant []int64 `json:"grant,omitempty"`
	// Revoke is a list of person IDs to revoke access from the project.
	Revoke []int64 `json:"revoke,omitempty"`
	// Create is a list of new people to create and grant access.
	Create []CreatePersonRequest `json:"create,omitempty"`
}

// CreatePersonRequest specifies the parameters for creating a new person.
type CreatePersonRequest struct {
	// Name is the person's full name (required).
	Name string `json:"name"`
	// EmailAddress is the person's email address (required).
	EmailAddress string `json:"email_address"`
	// Title is the person's job title (optional).
	Title string `json:"title,omitempty"`
	// CompanyName is the person's company name (optional).
	CompanyName string `json:"company_name,omitempty"`
}

// UpdateProjectAccessResponse is the response from updating project access.
type UpdateProjectAccessResponse struct {
	// Granted is the list of people who were granted access.
	Granted []Person `json:"granted"`
	// Revoked is the list of people whose access was revoked.
	Revoked []Person `json:"revoked"`
}

// PeopleListOptions specifies options for listing people.
type PeopleListOptions struct {
	// Limit is the maximum number of people to return.
	// If 0 (default), returns all people.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// PeopleListResult contains the results from listing people.
type PeopleListResult struct {
	// People is the list of people returned.
	People []Person
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// PeopleService handles people operations.
type PeopleService struct {
	client *AccountClient
}

// NewPeopleService creates a new PeopleService.
func NewPeopleService(client *AccountClient) *PeopleService {
	return &PeopleService{client: client}
}

// List returns all people visible to the current user in the account.
//
// Pagination options:
//   - Limit: maximum number of people to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned PeopleListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *PeopleService) List(ctx context.Context, opts *PeopleListOptions) (result *PeopleListResult, err error) {
	op := OperationInfo{
		Service: "People", Operation: "List",
		ResourceType: "person", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListPeopleWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var people []Person
	if resp.JSON200 != nil {
		for _, gp := range *resp.JSON200 {
			people = append(people, personFromGenerated(gp))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for people)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(people), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gp generated.Person
		if err := json.Unmarshal(raw, &gp); err != nil {
			return nil, fmt.Errorf("failed to parse person: %w", err)
		}
		people = append(people, personFromGenerated(gp))
	}

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a person by ID.
func (s *PeopleService) Get(ctx context.Context, personID int64) (result *Person, err error) {
	op := OperationInfo{
		Service: "People", Operation: "Get",
		ResourceType: "person", IsMutation: false,
		ResourceID: personID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetPersonWithResponse(ctx, s.client.accountID, personID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	person := personFromGenerated(*resp.JSON200)
	return &person, nil
}

// Me returns the current authenticated user's profile.
func (s *PeopleService) Me(ctx context.Context) (result *Person, err error) {
	op := OperationInfo{
		Service: "People", Operation: "Me",
		ResourceType: "person", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMyProfileWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	person := personFromGenerated(*resp.JSON200)
	return &person, nil
}

// ListProjectPeople returns all active people on a project.
//
// Pagination options:
//   - Limit: maximum number of people to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned PeopleListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *PeopleService) ListProjectPeople(ctx context.Context, projectID int64, opts *PeopleListOptions) (result *PeopleListResult, err error) {
	op := OperationInfo{
		Service: "People", Operation: "ListProjectPeople",
		ResourceType: "person", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListProjectPeopleWithResponse(ctx, s.client.accountID, projectID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var people []Person
	if resp.JSON200 != nil {
		for _, gp := range *resp.JSON200 {
			people = append(people, personFromGenerated(gp))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for people)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(people), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gp generated.Person
		if err := json.Unmarshal(raw, &gp); err != nil {
			return nil, fmt.Errorf("failed to parse person: %w", err)
		}
		people = append(people, personFromGenerated(gp))
	}

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Pingable returns all account users who can be pinged.
// Note: This endpoint is not paginated in the Basecamp API.
func (s *PeopleService) Pingable(ctx context.Context) (result []Person, err error) {
	op := OperationInfo{
		Service: "People", Operation: "Pingable",
		ResourceType: "person", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListPingablePeopleWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	people := make([]Person, 0, len(*resp.JSON200))
	for _, gp := range *resp.JSON200 {
		people = append(people, personFromGenerated(gp))
	}

	return people, nil
}

// UpdateProjectAccess grants or revokes project access for people.
// Returns the list of people who were granted and revoked access.
func (s *PeopleService) UpdateProjectAccess(ctx context.Context, projectID int64, req *UpdateProjectAccessRequest) (result *UpdateProjectAccessResponse, err error) {
	op := OperationInfo{
		Service: "People", Operation: "UpdateProjectAccess",
		ResourceType: "person", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || (len(req.Grant) == 0 && len(req.Revoke) == 0 && len(req.Create) == 0) {
		err = ErrUsage("at least one of grant, revoke, or create must be specified")
		return nil, err
	}

	body := generated.UpdateProjectAccessJSONRequestBody{
		Grant:  req.Grant,
		Revoke: req.Revoke,
	}
	if len(req.Create) > 0 {
		body.Create = make([]generated.CreatePersonRequest, 0, len(req.Create))
		for _, cp := range req.Create {
			body.Create = append(body.Create, generated.CreatePersonRequest{
				Name:         cp.Name,
				EmailAddress: cp.EmailAddress,
				Title:        cp.Title,
				CompanyName:  cp.CompanyName,
			})
		}
	}

	resp, err := s.client.parent.gen.UpdateProjectAccessWithResponse(ctx, s.client.accountID, projectID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	// Convert the response
	accessResult := &UpdateProjectAccessResponse{
		Granted: make([]Person, 0, len(resp.JSON200.Granted)),
		Revoked: make([]Person, 0, len(resp.JSON200.Revoked)),
	}
	for _, gp := range resp.JSON200.Granted {
		accessResult.Granted = append(accessResult.Granted, personFromGenerated(gp))
	}
	for _, gp := range resp.JSON200.Revoked {
		accessResult.Revoked = append(accessResult.Revoked, personFromGenerated(gp))
	}

	return accessResult, nil
}

// personFromGenerated converts a generated Person to our clean Person type.
func personFromGenerated(gp generated.Person) Person {
	p := Person{
		AttachableSGID:    gp.AttachableSgid,
		Name:              gp.Name,
		EmailAddress:      gp.EmailAddress,
		PersonableType:    gp.PersonableType,
		Title:             gp.Title,
		Bio:               gp.Bio,
		Location:          gp.Location,
		Admin:             gp.Admin,
		Owner:             gp.Owner,
		Client:            gp.Client,
		Employee:          gp.Employee,
		TimeZone:          gp.TimeZone,
		AvatarURL:         gp.AvatarUrl,
		CanManageProjects: gp.CanManageProjects,
		CanManagePeople:   gp.CanManagePeople,
	}

	if derefInt64(gp.Id) != 0 {
		p.ID = derefInt64(gp.Id)
	}

	// Convert timestamps to strings (the SDK Person type uses strings for these)
	if !gp.CreatedAt.IsZero() {
		p.CreatedAt = gp.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !gp.UpdatedAt.IsZero() {
		p.UpdatedAt = gp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Convert company
	if derefInt64(gp.Company.Id) != 0 || gp.Company.Name != "" {
		p.Company = &PersonCompany{
			ID:   derefInt64(gp.Company.Id),
			Name: gp.Company.Name,
		}
	}

	return p
}
