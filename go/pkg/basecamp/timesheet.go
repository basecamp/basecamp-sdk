package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
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
	client *Client
}

// NewTimesheetService creates a new TimesheetService.
func NewTimesheetService(client *Client) *TimesheetService {
	return &TimesheetService{client: client}
}

// buildQueryParams builds query parameters from options.
func (s *TimesheetService) buildQueryParams(opts *TimesheetReportOptions) string {
	if opts == nil {
		return ""
	}

	params := url.Values{}
	if opts.From != "" {
		params.Set("from", opts.From)
	}
	if opts.To != "" {
		params.Set("to", opts.To)
	}
	if opts.PersonID != 0 {
		params.Set("person_id", fmt.Sprintf("%d", opts.PersonID))
	}

	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
}

// Report returns the account-wide timesheet report.
// This includes time entries across all projects in the account.
func (s *TimesheetService) Report(ctx context.Context, opts *TimesheetReportOptions) ([]TimesheetEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := "/reports/timesheet.json" + s.buildQueryParams(opts)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	entries := make([]TimesheetEntry, 0, len(results))
	for _, raw := range results {
		var e TimesheetEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("failed to parse timesheet entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// ProjectReport returns the timesheet report for a specific project.
// projectID is the project (bucket) ID.
func (s *TimesheetService) ProjectReport(ctx context.Context, projectID int64, opts *TimesheetReportOptions) ([]TimesheetEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/timesheet.json", projectID) + s.buildQueryParams(opts)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	entries := make([]TimesheetEntry, 0, len(results))
	for _, raw := range results {
		var e TimesheetEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("failed to parse timesheet entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// RecordingReport returns the timesheet report for a specific recording within a project.
// projectID is the project (bucket) ID, recordingID is the recording ID (e.g., a todo).
func (s *TimesheetService) RecordingReport(ctx context.Context, projectID, recordingID int64, opts *TimesheetReportOptions) ([]TimesheetEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/timesheet.json", projectID, recordingID) + s.buildQueryParams(opts)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	entries := make([]TimesheetEntry, 0, len(results))
	for _, raw := range results {
		var e TimesheetEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("failed to parse timesheet entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}
