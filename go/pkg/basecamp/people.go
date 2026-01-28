package basecamp

import (
	"context"
	"fmt"

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

	resp, err := s.client.gen.ListPeopleWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	people := make([]Person, 0, len(resp.JSON200.People))
	for _, gp := range resp.JSON200.People {
		people = append(people, personFromGenerated(gp))
	}

	return people, nil
}

// Get returns a person by ID.
func (s *PeopleService) Get(ctx context.Context, personID int64) (*Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetPersonWithResponse(ctx, personID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	person := personFromGenerated(resp.JSON200.Person)
	return &person, nil
}

// Me returns the current authenticated user's profile.
func (s *PeopleService) Me(ctx context.Context) (*Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetMyProfileWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	person := personFromGenerated(resp.JSON200.Person)
	return &person, nil
}

// ListProjectPeople returns all active people on a project.
// bucketID is the project ID.
func (s *PeopleService) ListProjectPeople(ctx context.Context, bucketID int64) ([]Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListProjectPeopleWithResponse(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	people := make([]Person, 0, len(resp.JSON200.People))
	for _, gp := range resp.JSON200.People {
		people = append(people, personFromGenerated(gp))
	}

	return people, nil
}

// Pingable returns all account users who can be pinged.
// Note: This endpoint is not paginated in the Basecamp API.
func (s *PeopleService) Pingable(ctx context.Context) ([]Person, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListPingablePeopleWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	people := make([]Person, 0, len(resp.JSON200.People))
	for _, gp := range resp.JSON200.People {
		people = append(people, personFromGenerated(gp))
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

	resp, err := s.client.gen.UpdateProjectAccessWithResponse(ctx, bucketID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	// Convert the response
	result := &UpdateProjectAccessResponse{
		Granted: make([]Person, 0, len(resp.JSON200.Result.Granted)),
		Revoked: make([]Person, 0, len(resp.JSON200.Result.Revoked)),
	}
	for _, gp := range resp.JSON200.Result.Granted {
		result.Granted = append(result.Granted, personFromGenerated(gp))
	}
	for _, gp := range resp.JSON200.Result.Revoked {
		result.Revoked = append(result.Revoked, personFromGenerated(gp))
	}

	return result, nil
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

	if gp.Id != nil {
		p.ID = *gp.Id
	}

	// Convert timestamps to strings (the SDK Person type uses strings for these)
	if !gp.CreatedAt.IsZero() {
		p.CreatedAt = gp.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !gp.UpdatedAt.IsZero() {
		p.UpdatedAt = gp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Convert company
	if gp.Company.Id != nil || gp.Company.Name != "" {
		p.Company = &PersonCompany{
			ID:   derefInt64(gp.Company.Id),
			Name: gp.Company.Name,
		}
	}

	return p
}
