package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Calendar is a per-account calendar (wire type Calendar), a top-level BC5
// bucketable keyed by its own bucket id, distinct from a project. It exposes
// display metadata and a link to its underlying schedule resource.
type Calendar struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	// Color is one of: white, red, orange, yellow, green, blue, aqua, purple,
	// gray, pink, brown.
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	// ScheduleURL is the API URL of the calendar's underlying schedule.
	ScheduleURL string `json:"schedule_url"`
}

// CalendarsService reads and updates per-account calendars (show + update
// only — the shipped BC5 scope).
type CalendarsService struct {
	client *AccountClient
}

// NewCalendarsService creates a new CalendarsService.
func NewCalendarsService(client *AccountClient) *CalendarsService {
	return &CalendarsService{client: client}
}

// Get returns the calendar with the given bucket id.
func (s *CalendarsService) Get(ctx context.Context, calendarID int64) (result *Calendar, err error) {
	op := OperationInfo{
		Service: "Calendars", Operation: "Get",
		ResourceType: "calendar", IsMutation: false,
		ResourceID: calendarID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCalendarWithResponse(ctx, s.client.accountID, calendarID)
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

	calendar := calendarFromGenerated(*resp.JSON200)
	return &calendar, nil
}

// Update sets the calendar's display color. An unknown color returns a
// validation error (422 with a field-keyed errors payload).
func (s *CalendarsService) Update(ctx context.Context, calendarID int64, color string) (result *Calendar, err error) {
	op := OperationInfo{
		Service: "Calendars", Operation: "Update",
		ResourceType: "calendar", IsMutation: true,
		ResourceID: calendarID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if color == "" {
		err = ErrUsage("calendar color is required")
		return nil, err
	}

	body := generated.UpdateCalendarJSONRequestBody{
		Calendar: generated.CalendarAttributes{Color: color},
	}
	resp, err := s.client.parent.gen.UpdateCalendarWithResponse(ctx, s.client.accountID, calendarID, body)
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

	calendar := calendarFromGenerated(*resp.JSON200)
	return &calendar, nil
}

func calendarFromGenerated(gc generated.Calendar) Calendar {
	return Calendar{
		ID:          gc.Id,
		Type:        gc.Type,
		Name:        gc.Name,
		Color:       gc.Color,
		CreatedAt:   gc.CreatedAt,
		UpdatedAt:   gc.UpdatedAt,
		URL:         gc.Url,
		AppURL:      gc.AppUrl,
		ScheduleURL: gc.ScheduleUrl,
	}
}
