package basecamp

import (
	"context"
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
	Parent         *Parent   `json:"parent,omitempty"`
	Bucket         *Bucket   `json:"bucket,omitempty"`
	BillableStatus string    `json:"billable_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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

	entries := make([]TimesheetEntry, 0, len(resp.JSON200.Entries))
	for _, ge := range resp.JSON200.Entries {
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
		BucketID: projectID,
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

	entries := make([]TimesheetEntry, 0, len(resp.JSON200.Entries))
	for _, ge := range resp.JSON200.Entries {
		entries = append(entries, timesheetEntryFromGenerated(ge))
	}

	return entries, nil
}

// RecordingReport returns the timesheet report for a specific recording within a project.
// projectID is the project (bucket) ID, recordingID is the recording ID (e.g., a todo).
func (s *TimesheetService) RecordingReport(ctx context.Context, projectID, recordingID int64, opts *TimesheetReportOptions) (result []TimesheetEntry, err error) {
	op := OperationInfo{
		Service: "Timesheet", Operation: "RecordingReport",
		ResourceType: "timesheet_entry", IsMutation: false,
		BucketID: projectID, ResourceID: recordingID,
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

	resp, err := s.client.parent.gen.GetRecordingTimesheetWithResponse(ctx, s.client.accountID, projectID, recordingID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	entries := make([]TimesheetEntry, 0, len(resp.JSON200.Entries))
	for _, ge := range resp.JSON200.Entries {
		entries = append(entries, timesheetEntryFromGenerated(ge))
	}

	return entries, nil
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

	if ge.Id != nil {
		e.ID = *ge.Id
	}

	if ge.Creator.Id != nil || ge.Creator.Name != "" {
		e.Creator = &Person{
			ID:           derefInt64(ge.Creator.Id),
			Name:         ge.Creator.Name,
			EmailAddress: ge.Creator.EmailAddress,
			AvatarURL:    ge.Creator.AvatarUrl,
			Admin:        ge.Creator.Admin,
			Owner:        ge.Creator.Owner,
		}
	}

	if ge.Parent.Id != nil || ge.Parent.Title != "" {
		e.Parent = &Parent{
			ID:     derefInt64(ge.Parent.Id),
			Title:  ge.Parent.Title,
			Type:   ge.Parent.Type,
			URL:    ge.Parent.Url,
			AppURL: ge.Parent.AppUrl,
		}
	}

	if ge.Bucket.Id != nil || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   derefInt64(ge.Bucket.Id),
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	return e
}
