package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TimesheetEntry represents a single time entry in a Basecamp timesheet report.
type TimesheetEntry struct {
	ID             int64     `json:"id"`
	Date           string    `json:"date"`
	Hours          string    `json:"hours"`
	Description    string    `json:"description,omitempty"`
	Creator        *Person   `json:"creator,omitempty"`
	Person         *Person   `json:"person,omitempty"`
	Parent         *Parent   `json:"parent,omitempty"`
	Bucket         *Bucket   `json:"bucket,omitempty"`
	BillableStatus string    `json:"billable_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateTimesheetEntryRequest specifies the parameters for creating a timesheet entry.
type CreateTimesheetEntryRequest struct {
	Date        string `json:"date"`
	Hours       string `json:"hours"`
	Description string `json:"description,omitempty"`
	PersonID    int64  `json:"person_id,omitempty"`
}

// UpdateTimesheetEntryRequest specifies the parameters for updating a timesheet entry.
type UpdateTimesheetEntryRequest struct {
	Date        string `json:"date,omitempty"`
	Hours       string `json:"hours,omitempty"`
	Description string `json:"description,omitempty"`
	PersonID    int64  `json:"person_id,omitempty"`
}

// TimesheetReportOptions specifies options for timesheet reports.
type TimesheetReportOptions struct {
	// From filters entries on or after this date (ISO 8601 format, e.g., "2024-01-01").
	From string
	// To filters entries on or before this date (ISO 8601 format, e.g., "2024-01-31").
	To string
	// PersonID filters entries by a specific person.
	PersonID int64
}

// TimesheetService handles timesheet report operations.
type TimesheetService struct {
	client *AccountClient
}

// NewTimesheetService creates a new TimesheetService.
func NewTimesheetService(client *AccountClient) *TimesheetService {
	return &TimesheetService{client: client}
}

// buildTimesheetParams builds query parameters for the generated client.
// Returns nil if no filters are specified to avoid serializing zero values.
func (s *TimesheetService) buildTimesheetParams(opts *TimesheetReportOptions) *generated.GetTimesheetReportParams {
	if opts == nil {
		return nil
	}

	// Only create params if there are actual filter values
	if opts.From == "" && opts.To == "" && opts.PersonID == 0 {
		return nil
	}

	return &generated.GetTimesheetReportParams{
		From:     opts.From,
		To:       opts.To,
		PersonId: opts.PersonID,
	}
}

// Report returns the account-wide timesheet report.
// This includes time entries across all projects in the account.
func (s *TimesheetService) Report(ctx context.Context, opts *TimesheetReportOptions) (result []TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "Report",
		ResourceType: "timesheet_entry", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	params := s.buildTimesheetParams(opts)

	resp, err := s.client.parent.gen.GetTimesheetReportWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	entries := make([]TimesheetEntry, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
		entries = append(entries, timesheetEntryFromGenerated(ge))
	}

	return entries, nil
}

// ProjectReport returns the timesheet report for a specific project.
// projectID is the project (bucket) ID.
func (s *TimesheetService) ProjectReport(ctx context.Context, projectID int64, opts *TimesheetReportOptions) (result []TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "ProjectReport",
		ResourceType: "timesheet_entry", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetProjectTimesheetParams
	if opts != nil {
		params = &generated.GetProjectTimesheetParams{
			From:     opts.From,
			To:       opts.To,
			PersonId: opts.PersonID,
		}
	}

	resp, err := s.client.parent.gen.GetProjectTimesheetWithResponse(ctx, s.client.accountID, projectID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	entries := make([]TimesheetEntry, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
		entries = append(entries, timesheetEntryFromGenerated(ge))
	}

	return entries, nil
}

// RecordingReport returns the timesheet report for a specific recording.
// recordingID is the recording ID (e.g., a todo).
func (s *TimesheetService) RecordingReport(ctx context.Context, recordingID int64, opts *TimesheetReportOptions) (result []TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "RecordingReport",
		ResourceType: "timesheet_entry", IsMutation: false,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetRecordingTimesheetParams
	if opts != nil {
		params = &generated.GetRecordingTimesheetParams{
			From:     opts.From,
			To:       opts.To,
			PersonId: opts.PersonID,
		}
	}

	resp, err := s.client.parent.gen.GetRecordingTimesheetWithResponse(ctx, s.client.accountID, recordingID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	entries := make([]TimesheetEntry, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
		entries = append(entries, timesheetEntryFromGenerated(ge))
	}

	return entries, nil
}

// Get returns a single timesheet entry.
func (s *TimesheetService) Get(ctx context.Context, entryID int64) (result *TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "Get",
		ResourceType: "timesheet_entry", IsMutation: false,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTimesheetEntryWithResponse(ctx, s.client.accountID, entryID)
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

	entry := timesheetEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// Create creates a timesheet entry on a recording.
func (s *TimesheetService) Create(ctx context.Context, recordingID int64, req *CreateTimesheetEntryRequest) (result *TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "Create",
		ResourceType: "timesheet_entry", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req.Date == "" {
		err = ErrUsage("timesheet entry date is required")
		return nil, err
	}
	if req.Hours == "" {
		err = ErrUsage("timesheet entry hours is required")
		return nil, err
	}

	body := generated.CreateTimesheetEntryJSONRequestBody{
		Date:        req.Date,
		Hours:       req.Hours,
		Description: req.Description,
	}
	if req.PersonID != 0 {
		body.PersonId = &req.PersonID
	}

	resp, err := s.client.parent.gen.CreateTimesheetEntryWithResponse(ctx, s.client.accountID, recordingID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	entry := timesheetEntryFromGenerated(*resp.JSON201)
	return &entry, nil
}

// Update updates an existing timesheet entry.
func (s *TimesheetService) Update(ctx context.Context, entryID int64, req *UpdateTimesheetEntryRequest) (result *TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "Update",
		ResourceType: "timesheet_entry", IsMutation: true,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.UpdateTimesheetEntryJSONRequestBody{
		Date:        req.Date,
		Hours:       req.Hours,
		Description: req.Description,
	}
	if req.PersonID != 0 {
		body.PersonId = &req.PersonID
	}

	resp, err := s.client.parent.gen.UpdateTimesheetEntryWithResponse(ctx, s.client.accountID, entryID, body)
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

	entry := timesheetEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// Trash moves a timesheet entry to the trash.
func (s *TimesheetService) Trash(ctx context.Context, entryID int64) (err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "Trash",
		ResourceType: "timesheet_entry", IsMutation: true,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, entryID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// timesheetEntryFromGenerated converts a generated TimesheetEntry to our clean type.
func timesheetEntryFromGenerated(ge generated.TimesheetEntry) TimesheetEntry {
	e := TimesheetEntry{
		Date:        ge.Date,
		Hours:       ge.Hours,
		Description: ge.Description,
		CreatedAt:   ge.CreatedAt,
		UpdatedAt:   ge.UpdatedAt,
	}

	if derefInt64(ge.Id) != 0 {
		e.ID = derefInt64(ge.Id)
	}

	if derefInt64(ge.Creator.Id) != 0 || ge.Creator.Name != "" {
		e.Creator = &Person{
			ID:           derefInt64(ge.Creator.Id),
			Name:         ge.Creator.Name,
			EmailAddress: ge.Creator.EmailAddress,
			AvatarURL:    ge.Creator.AvatarUrl,
			Admin:        ge.Creator.Admin,
			Owner:        ge.Creator.Owner,
		}
	}

	if derefInt64(ge.Person.Id) != 0 || ge.Person.Name != "" {
		e.Person = &Person{
			ID:           derefInt64(ge.Person.Id),
			Name:         ge.Person.Name,
			EmailAddress: ge.Person.EmailAddress,
			AvatarURL:    ge.Person.AvatarUrl,
			Admin:        ge.Person.Admin,
			Owner:        ge.Person.Owner,
		}
	}

	if derefInt64(ge.Parent.Id) != 0 || ge.Parent.Title != "" {
		e.Parent = &Parent{
			ID:     derefInt64(ge.Parent.Id),
			Title:  ge.Parent.Title,
			Type:   ge.Parent.Type,
			URL:    ge.Parent.Url,
			AppURL: ge.Parent.AppUrl,
		}
	}

	if derefInt64(ge.Bucket.Id) != 0 || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   derefInt64(ge.Bucket.Id),
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	return e
}
