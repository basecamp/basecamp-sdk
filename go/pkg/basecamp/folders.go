package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Folder groups projects on one person's home screen. Folders are per-user:
// filing a project away for yourself changes nothing for anyone else.
//
// The wire type is "Stack", not "Folder" — the product was renamed, the payload
// was not. Anything matching on Type must match "Stack".
//
// This is the shape List returns. Get, Create and Update return
// FolderWithProjects, which adds the expanded projects.
type Folder struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Type is always "Stack".
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// BucketIDs are the ids of the projects filed into this folder — the same
	// ids Create takes as ProjectIDs and FolderWithProjects expands.
	BucketIDs       []int64 `json:"bucket_ids"`
	IsEmojiOnlyName bool    `json:"is_emoji_only_name"`
	StarURL         string  `json:"star_url"`
	// GaugesURL is nil when none of the folder's projects is gauged. The key is
	// always present on the wire; the pointer models the null, not absence.
	GaugesURL *string `json:"gauges_url"`
	// Color is the viewer's colour customization, nil when unset.
	Color *string `json:"color"`
	// ImageURL is the viewer's folder image, nil when unset. Read-only: there is
	// no image create or update in v1.
	ImageURL *string `json:"image_url"`
	URL      string  `json:"url"`
}

// FolderWithProjects is one folder plus the projects grouped inside it, as Get,
// Create and Update return it. Projects is always present and is empty for an
// empty folder.
//
// The base fields are repeated rather than embedded: an embedded Folder would
// promote them correctly at runtime but hide them from the wrapper-drift guard,
// which walks declared fields and would report every one as missing.
type FolderWithProjects struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Type is always "Stack".
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// BucketIDs are the ids of the projects filed into this folder — the same
	// set Projects expands.
	BucketIDs       []int64 `json:"bucket_ids"`
	IsEmojiOnlyName bool    `json:"is_emoji_only_name"`
	StarURL         string  `json:"star_url"`
	// GaugesURL is nil when none of the folder's projects is gauged.
	GaugesURL *string `json:"gauges_url"`
	// Color is the viewer's colour customization, nil when unset.
	Color *string `json:"color"`
	// ImageURL is the viewer's folder image, nil when unset. Read-only.
	ImageURL *string `json:"image_url"`
	URL      string  `json:"url"`
	// Projects are the projects filed into this folder, expanded. Always
	// present; empty for an empty folder.
	Projects []Project `json:"projects"`
}

// CreateFolderRequest specifies the parameters for creating a folder.
type CreateFolderRequest struct {
	// Name is the folder's name. Blank or omitted defaults to "New folder".
	Name string `json:"name,omitempty"`
	// ProjectIDs are the projects to file into the folder. Every id is
	// preflighted: if any is archived, trashed, or an invitation-only project
	// the user is not on, the whole request fails with 404 and nothing is
	// created. Filing an all-access project the user has not joined grants them
	// access to it. Omit for an empty folder.
	ProjectIDs []int64 `json:"project_ids,omitempty"`
}

// FoldersService handles the current user's home-screen folders.
type FoldersService struct {
	client *AccountClient
}

// NewFoldersService creates a new FoldersService.
func NewFoldersService(client *AccountClient) *FoldersService {
	return &FoldersService{client: client}
}

// List returns the current user's folders in home-screen order. The response is
// a bare array with no pagination, and its items carry no expanded projects —
// use Get for that.
func (s *FoldersService) List(ctx context.Context) (result []Folder, err error) {
	op := OperationInfo{
		Service: "Folders", Operation: "List",
		ResourceType: "folder", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListFoldersWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var folders []Folder
	if resp.JSON200 != nil {
		for _, gf := range *resp.JSON200 {
			folders = append(folders, folderFromGenerated(gf))
		}
	}
	return folders, nil
}

// Get returns one folder with the projects grouped inside it expanded.
func (s *FoldersService) Get(ctx context.Context, folderID int64) (result *FolderWithProjects, err error) {
	op := OperationInfo{
		Service: "Folders", Operation: "Get",
		ResourceType: "folder", IsMutation: false,
		ResourceID: folderID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetFolderWithResponse(ctx, s.client.accountID, folderID)
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

	folder := folderWithProjectsFromGenerated(*resp.JSON200)
	return &folder, nil
}

// Create files the given projects into a new folder at the top of the home
// screen. Not idempotent: each call creates another folder.
func (s *FoldersService) Create(ctx context.Context, req CreateFolderRequest) (result *FolderWithProjects, err error) {
	op := OperationInfo{
		Service: "Folders", Operation: "Create",
		ResourceType: "folder", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.CreateFolderJSONRequestBody{}
	if req.Name != "" {
		body.Name = &req.Name
	}
	if req.ProjectIDs != nil {
		ids := req.ProjectIDs
		body.ProjectIds = &ids
	}

	resp, err := s.client.parent.gen.CreateFolderWithResponse(ctx, s.client.accountID, body)
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

	folder := folderWithProjectsFromGenerated(*resp.JSON201)
	return &folder, nil
}

// Update renames a folder. Name is the only writable attribute; a folder's
// projects, ordering and image are managed elsewhere. A blank name is rejected
// with 422 — unlike Create, Update does not fall back to a default.
func (s *FoldersService) Update(ctx context.Context, folderID int64, name string) (result *FolderWithProjects, err error) {
	op := OperationInfo{
		Service: "Folders", Operation: "Update",
		ResourceType: "folder", IsMutation: true,
		ResourceID: folderID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UpdateFolderWithResponse(ctx, s.client.accountID, folderID,
		generated.UpdateFolderJSONRequestBody{Name: name})
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

	folder := folderWithProjectsFromGenerated(*resp.JSON200)
	return &folder, nil
}

// Delete removes the folder and unpins its projects from the home screen. The
// projects themselves are not deleted, and they are not moved back out onto the
// home screen; they stop appearing there until pinned again.
func (s *FoldersService) Delete(ctx context.Context, folderID int64) (err error) {
	op := OperationInfo{
		Service: "Folders", Operation: "Delete",
		ResourceType: "folder", IsMutation: true,
		ResourceID: folderID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteFolderWithResponse(ctx, s.client.accountID, folderID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

func folderFromGenerated(gf generated.Folder) Folder {
	return Folder{
		ID:              gf.Id,
		Name:            gf.Name,
		Type:            gf.Type,
		CreatedAt:       gf.CreatedAt,
		UpdatedAt:       gf.UpdatedAt,
		BucketIDs:       gf.BucketIds,
		IsEmojiOnlyName: gf.IsEmojiOnlyName,
		StarURL:         gf.StarUrl,
		GaugesURL:       gf.GaugesUrl,
		Color:           gf.Color,
		ImageURL:        gf.ImageUrl,
		URL:             gf.Url,
	}
}

func folderWithProjectsFromGenerated(gf generated.FolderWithProjects) FolderWithProjects {
	folder := FolderWithProjects{
		ID:              gf.Id,
		Name:            gf.Name,
		Type:            gf.Type,
		CreatedAt:       gf.CreatedAt,
		UpdatedAt:       gf.UpdatedAt,
		BucketIDs:       gf.BucketIds,
		IsEmojiOnlyName: gf.IsEmojiOnlyName,
		StarURL:         gf.StarUrl,
		GaugesURL:       gf.GaugesUrl,
		Color:           gf.Color,
		ImageURL:        gf.ImageUrl,
		URL:             gf.Url,
		Projects:        make([]Project, 0, len(gf.Projects)),
	}
	for _, gp := range gf.Projects {
		folder.Projects = append(folder.Projects, projectFromGenerated(gp))
	}
	return folder
}
