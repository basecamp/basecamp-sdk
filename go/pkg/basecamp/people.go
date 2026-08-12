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

// FirstWeekDay represents the first day of the week.
// Use the exported constants (FirstWeekDaySunday, FirstWeekDayMonday, etc.).
type FirstWeekDay string

// FirstWeekDay constants for all seven days.
const (
	FirstWeekDaySunday    FirstWeekDay = "Sunday"
	FirstWeekDayMonday    FirstWeekDay = "Monday"
	FirstWeekDayTuesday   FirstWeekDay = "Tuesday"
	FirstWeekDayWednesday FirstWeekDay = "Wednesday"
	FirstWeekDayThursday  FirstWeekDay = "Thursday"
	FirstWeekDayFriday    FirstWeekDay = "Friday"
	FirstWeekDaySaturday  FirstWeekDay = "Saturday"
)

// UpdateMyProfileRequest specifies the parameters for updating the current user's profile.
// Use pointer fields (*string) to distinguish "not provided" (nil) from "clear field" ("").
type UpdateMyProfileRequest struct {
	// Name is the person's display name.
	Name *string `json:"name,omitempty"`
	// EmailAddress is the person's email address.
	EmailAddress *string `json:"email_address,omitempty"`
	// Title is the person's job title.
	Title *string `json:"title,omitempty"`
	// Bio is a short biographical text.
	Bio *string `json:"bio,omitempty"`
	// Location is the person's location.
	Location *string `json:"location,omitempty"`
	// TimeZoneName is a Rails time zone name (e.g. "America/Chicago").
	TimeZoneName *string `json:"time_zone_name,omitempty"`
	// FirstWeekDay is the first day of the week (e.g. FirstWeekDaySunday).
	FirstWeekDay *FirstWeekDay `json:"first_week_day,omitempty"`
	// TimeFormat is the time display format (e.g. "twelve_hour", "twenty_four_hour").
	TimeFormat *string `json:"time_format,omitempty"`
}

// PeopleListOptions specifies options for listing people.
type PeopleListOptions struct {
	// Limit is the maximum number of people to return.
	// If 0 (default), returns all people.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	// Ignored by endpoints that are not paginated server-side (Pingable).
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
//   - Page: if positive, fetches only that page and disables auto-pagination
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
	var params *generated.ListPeopleParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListPeopleParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListPeopleWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		keep, truncated := pageCap(len(people), opts.Limit, resp.HTTPResponse)
		return &PeopleListResult{People: people[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for people)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(people), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(people), limit)
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

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	person := personFromGenerated(*resp.JSON200)
	return &person, nil
}

// UpdateMyProfile updates the current authenticated user's profile.
func (s *PeopleService) UpdateMyProfile(ctx context.Context, req *UpdateMyProfileRequest) (err error) {
	op := OperationInfo{
		Service: "People", Operation: "UpdateMyProfile",
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

	if req == nil {
		err = ErrUsage("update request is required")
		return err
	}

	body := generated.UpdateMyProfileJSONRequestBody{
		Name:         req.Name,
		EmailAddress: req.EmailAddress,
		Title:        req.Title,
		Bio:          req.Bio,
		Location:     req.Location,
		TimeZoneName: req.TimeZoneName,
		TimeFormat:   req.TimeFormat,
	}
	if req.FirstWeekDay != nil {
		day := generated.FirstWeekDay(*req.FirstWeekDay)
		body.FirstWeekDay = &day
	}

	resp, err := s.client.parent.gen.UpdateMyProfileWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// ListProjectPeople returns all active people on a project.
//
// Pagination options:
//   - Limit: maximum number of people to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned PeopleListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *PeopleService) ListProjectPeople(ctx context.Context, projectID int64, opts *PeopleListOptions) (result *PeopleListResult, err error) {
	op := OperationInfo{
		Service: "People", Operation: "ListProjectPeople",
		ResourceType: "person", IsMutation: false,
		ProjectID: projectID,
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
	var params *generated.ListProjectPeopleParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListProjectPeopleParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListProjectPeopleWithResponse(ctx, s.client.accountID, projectID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		keep, truncated := pageCap(len(people), opts.Limit, resp.HTTPResponse)
		return &PeopleListResult{People: people[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for people)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(people), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(people), limit)
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

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Pingable returns all account users who can be pinged.
//
// Pagination options:
//   - Limit: maximum number of people to return (0 = all)
//   - Page: the page number is ignored (the pingable-people endpoint is not
//     paginated server-side), but any positive value still disables
//     auto-pagination, returning the single response as-is without applying Limit
//
// The returned PeopleListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *PeopleService) Pingable(ctx context.Context, opts *PeopleListOptions) (result *PeopleListResult, err error) {
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

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListPingablePeopleWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		keep, truncated := pageCap(len(people), opts.Limit, resp.HTTPResponse)
		return &PeopleListResult{People: people[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for people)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(people) >= limit {
		return &PeopleListResult{People: people[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(people), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(people), limit)
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

	return &PeopleListResult{People: people, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// UpdateProjectAccess grants or revokes project access for people.
// Returns the list of people who were granted and revoked access.
func (s *PeopleService) UpdateProjectAccess(ctx context.Context, projectID int64, req *UpdateProjectAccessRequest) (result *UpdateProjectAccessResponse, err error) {
	op := OperationInfo{
		Service: "People", Operation: "UpdateProjectAccess",
		ResourceType: "person", IsMutation: true,
		ProjectID: projectID,
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

	body := generated.UpdateProjectAccessJSONRequestBody{}
	if req.Grant != nil {
		body.Grant = &req.Grant
	}
	if req.Revoke != nil {
		body.Revoke = &req.Revoke
	}
	if req.Create != nil {
		create := make([]generated.CreatePersonRequest, 0, len(req.Create))
		for _, cp := range req.Create {
			create = append(create, generated.CreatePersonRequest{
				Name:         cp.Name,
				EmailAddress: cp.EmailAddress,
				Title:        omitzero(cp.Title),
				CompanyName:  omitzero(cp.CompanyName),
			})
		}
		body.Create = &create
	}

	resp, err := s.client.parent.gen.UpdateProjectAccessWithResponse(ctx, s.client.accountID, projectID, body)
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

// Preferences represents user preferences.
type Preferences struct {
	FirstWeekDay string `json:"first_week_day,omitempty"`
	TimeFormat   string `json:"time_format,omitempty"`
	TimeZoneName string `json:"time_zone_name,omitempty"`
	AppURL       string `json:"app_url,omitempty"`
	URL          string `json:"url,omitempty"`
}

// UpdateMyPreferencesRequest specifies the parameters for updating user preferences.
type UpdateMyPreferencesRequest struct {
	FirstWeekDay string `json:"first_week_day,omitempty"`
	TimeFormat   string `json:"time_format,omitempty"`
	TimeZoneName string `json:"time_zone_name,omitempty"`
}

// OutOfOffice represents out-of-office status for a person.
// When out of office is not enabled, Enabled is false and StartDate,
// EndDate, and BackOnDate are omitted from the response.
type OutOfOffice struct {
	Enabled    bool              `json:"enabled,omitempty"`
	StartDate  string            `json:"start_date,omitempty"`
	EndDate    string            `json:"end_date,omitempty"`
	BackOnDate string            `json:"back_on_date,omitempty"`
	Ongoing    bool              `json:"ongoing,omitempty"`
	Person     OutOfOfficePerson `json:"person,omitempty"`
}

// OutOfOfficePerson represents the person associated with an out-of-office status.
type OutOfOfficePerson struct {
	ID        int64  `json:"id"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// EnableOutOfOfficeRequest specifies the parameters for enabling out-of-office.
type EnableOutOfOfficeRequest struct {
	// StartDate is the start date in ISO 8601 format (YYYY-MM-DD), required.
	StartDate string `json:"start_date"`
	// EndDate is the end date in ISO 8601 format (YYYY-MM-DD), required.
	EndDate string `json:"end_date"`
}

// GetMyPreferences returns the current user's preferences.
func (s *PeopleService) GetMyPreferences(ctx context.Context) (result *Preferences, err error) {
	op := OperationInfo{
		Service: "People", Operation: "GetMyPreferences",
		ResourceType: "preferences", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMyPreferencesWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var prefs Preferences
	if err = json.Unmarshal(resp.Body, &prefs); err != nil {
		return nil, fmt.Errorf("failed to parse preferences: %w", err)
	}

	return &prefs, nil
}

// UpdateMyPreferences updates the current user's preferences.
func (s *PeopleService) UpdateMyPreferences(ctx context.Context, req *UpdateMyPreferencesRequest) (err error) {
	op := OperationInfo{
		Service: "People", Operation: "UpdateMyPreferences",
		ResourceType: "preferences", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update preferences request is required")
		return err
	}

	body := generated.UpdateMyPreferencesJSONRequestBody{
		Person: generated.PreferencesPayload{
			FirstWeekDay: omitzero(req.FirstWeekDay),
			TimeFormat:   omitzero(req.TimeFormat),
			TimeZoneName: omitzero(req.TimeZoneName),
		},
	}

	resp, err := s.client.parent.gen.UpdateMyPreferencesWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// GetOutOfOffice returns the out-of-office status for a person.
func (s *PeopleService) GetOutOfOffice(ctx context.Context, personID int64) (result *OutOfOffice, err error) {
	op := OperationInfo{
		Service: "People", Operation: "GetOutOfOffice",
		ResourceType: "out_of_office", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetOutOfOfficeWithResponse(ctx, s.client.accountID, personID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var ooo OutOfOffice
	if err = json.Unmarshal(resp.Body, &ooo); err != nil {
		return nil, fmt.Errorf("failed to parse out of office: %w", err)
	}

	return &ooo, nil
}

// EnableOutOfOffice enables out-of-office for a person.
func (s *PeopleService) EnableOutOfOffice(ctx context.Context, personID int64, req *EnableOutOfOfficeRequest) (result *OutOfOffice, err error) {
	op := OperationInfo{
		Service: "People", Operation: "EnableOutOfOffice",
		ResourceType: "out_of_office", IsMutation: true,
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

	if req == nil {
		err = ErrUsage("enable out of office request is required")
		return nil, err
	}
	if req.StartDate == "" {
		err = ErrUsage("start_date is required")
		return nil, err
	}
	if req.EndDate == "" {
		err = ErrUsage("end_date is required")
		return nil, err
	}

	body := generated.EnableOutOfOfficeJSONRequestBody{
		OutOfOffice: generated.OutOfOfficePayload{
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
		},
	}

	resp, err := s.client.parent.gen.EnableOutOfOfficeWithResponse(ctx, s.client.accountID, personID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var ooo OutOfOffice
	if err = json.Unmarshal(resp.Body, &ooo); err != nil {
		return nil, fmt.Errorf("failed to parse out of office: %w", err)
	}

	return &ooo, nil
}

// DisableOutOfOffice disables out-of-office for a person.
func (s *PeopleService) DisableOutOfOffice(ctx context.Context, personID int64) (err error) {
	op := OperationInfo{
		Service: "People", Operation: "DisableOutOfOffice",
		ResourceType: "out_of_office", IsMutation: true,
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

	resp, err := s.client.parent.gen.DisableOutOfOfficeWithResponse(ctx, s.client.accountID, personID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// personFromGenerated converts a generated Person to our clean Person type.
func personFromGenerated(gp generated.Person) Person {
	p := Person{
		AttachableSGID:      deref(gp.AttachableSgid),
		Name:                gp.Name,
		EmailAddress:        deref(gp.EmailAddress),
		PersonableType:      deref(gp.PersonableType),
		Title:               deref(gp.Title),
		Bio:                 deref(gp.Bio),
		Tagline:             deref(gp.Tagline), // BC5 forward-compat field
		Location:            deref(gp.Location),
		Admin:               deref(gp.Admin),
		Owner:               deref(gp.Owner),
		Client:              deref(gp.Client),
		Employee:            deref(gp.Employee),
		TimeZone:            deref(gp.TimeZone),
		AvatarURL:           deref(gp.AvatarUrl),
		CanPing:             deref(gp.CanPing),
		CanAccessHillCharts: deref(gp.CanAccessHillCharts),
		CanAccessTimesheet:  deref(gp.CanAccessTimesheet),
		CanManageProjects:   deref(gp.CanManageProjects),
		CanManagePeople:     deref(gp.CanManagePeople),
	}

	if gp.Id != 0 {
		p.ID = int64(gp.Id)
	}

	// Convert timestamps to strings (the SDK Person type uses strings for these)
	if gp.CreatedAt != nil {
		p.CreatedAt = gp.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if gp.UpdatedAt != nil {
		p.UpdatedAt = gp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Convert company
	if gp.Company != nil {
		p.Company = &PersonCompany{
			ID:   gp.Company.Id,
			Name: gp.Company.Name,
		}
	}

	return p
}
