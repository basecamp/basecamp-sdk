package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
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

// PeopleService handles people operations.
type PeopleService struct {
	client *Client
}

// NewPeopleService creates a new PeopleService.
func NewPeopleService(client *Client) *PeopleService {
	return &PeopleService{client: client}
}

// List returns all people visible to the current user in the account.
func (s *PeopleService) List(ctx context.Context) ([]Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	results, err := s.client.GetAll(ctx, "/people.json")
	if err != nil {
		return nil, err
	}

	people := make([]Person, 0, len(results))
	for _, raw := range results {
		var p Person
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("failed to parse person: %w", err)
		}
		people = append(people, p)
	}

	return people, nil
}

// Get returns a person by ID.
func (s *PeopleService) Get(ctx context.Context, personID int64) (*Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/people/%d.json", personID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var person Person
	if err := resp.UnmarshalData(&person); err != nil {
		return nil, fmt.Errorf("failed to parse person: %w", err)
	}

	return &person, nil
}

// Me returns the current authenticated user's profile.
func (s *PeopleService) Me(ctx context.Context) (*Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, "/my/profile.json")
	if err != nil {
		return nil, err
	}

	var person Person
	if err := resp.UnmarshalData(&person); err != nil {
		return nil, fmt.Errorf("failed to parse person: %w", err)
	}

	return &person, nil
}

// ListProjectPeople returns all active people on a project.
// bucketID is the project ID.
func (s *PeopleService) ListProjectPeople(ctx context.Context, bucketID int64) ([]Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/projects/%d/people.json", bucketID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	people := make([]Person, 0, len(results))
	for _, raw := range results {
		var p Person
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("failed to parse person: %w", err)
		}
		people = append(people, p)
	}

	return people, nil
}

// Pingable returns all account users who can be pinged.
// Note: This endpoint is not paginated in the Basecamp API.
func (s *PeopleService) Pingable(ctx context.Context) ([]Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, "/circles/people.json")
	if err != nil {
		return nil, err
	}

	var people []Person
	if err := resp.UnmarshalData(&people); err != nil {
		return nil, fmt.Errorf("failed to parse people: %w", err)
	}

	return people, nil
}

// UpdateProjectAccess grants or revokes project access for people.
// bucketID is the project ID.
// Returns the list of people who were granted and revoked access.
func (s *PeopleService) UpdateProjectAccess(ctx context.Context, bucketID int64, req *UpdateProjectAccessRequest) (*UpdateProjectAccessResponse, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || (len(req.Grant) == 0 && len(req.Revoke) == 0 && len(req.Create) == 0) {
		return nil, ErrUsage("at least one of grant, revoke, or create must be specified")
	}

	path := fmt.Sprintf("/projects/%d/people/users.json", bucketID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var result UpdateProjectAccessResponse
	if err := resp.UnmarshalData(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
