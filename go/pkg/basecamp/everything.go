package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// EverythingService exposes the account-wide "everything" aggregate listings:
// recency-ordered, paginated roots (messages, comments, checkins, forwards,
// files) and the unpaginated oldest-first overdue todo/card lists. Each item
// embeds its bucket for project context. See
// spec/api-gaps/everything-aggregates.md.
type EverythingService struct {
	client *AccountClient
}

// NewEverythingService creates a new EverythingService.
func NewEverythingService(client *AccountClient) *EverythingService {
	return &EverythingService{client: client}
}

// RecordingsPage is a page-followed list of recordings with pagination metadata.
type RecordingsPage struct {
	Recordings []Recording
	Meta       ListMeta
}

// EverythingFilesPage is a page-followed list of files with pagination metadata.
type EverythingFilesPage struct {
	Files []EverythingFile
	Meta  ListMeta
}

// EverythingFilesOptions specifies optional filters for the files feed.
type EverythingFilesOptions struct {
	// Kind filters by file kind: "all" (default), "images", "pdfs",
	// "documents", or "videos".
	Kind string
	// PeopleIDs restricts the list to files created by the given people.
	PeopleIDs []int64
}

// EverythingFile is a single item in the /files.json feed: an optional-field
// superset over three wire variants — a full Upload recording, a Basecamp
// Document recording, and a rich-text attachment wrapped in a recording envelope
// (distinguished by AttachableSGID and blob metadata). Only the fields of the
// variant an instance represents are populated.
type EverythingFile struct {
	// Every field is optional and pointer-backed: the superset populates only the
	// fields of the variant an instance represents, so an absent field must stay
	// nil and re-marshal as omitted rather than as a fabricated sentinel. Per
	// SPEC.md §10, an empty string is NOT an acceptable stand-in for absence, so
	// the optional strings are *string too (a Document with no filename must be
	// distinguishable from an upload with an explicit empty filename).
	ID               *int64     `json:"id,omitempty"`
	Status           *string    `json:"status,omitempty"`
	VisibleToClients *bool      `json:"visible_to_clients,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	Title            *string    `json:"title,omitempty"`
	InheritsStatus   *bool      `json:"inherits_status,omitempty"`
	// Type is "Upload", "Document", or "Attachment".
	Type            *string `json:"type,omitempty"`
	URL             *string `json:"url,omitempty"`
	AppURL          *string `json:"app_url,omitempty"`
	BookmarkURL     *string `json:"bookmark_url,omitempty"`
	SubscriptionURL *string `json:"subscription_url,omitempty"`
	CommentsCount   *int32  `json:"comments_count,omitempty"`
	CommentsURL     *string `json:"comments_url,omitempty"`
	BoostsCount     *int32  `json:"boosts_count,omitempty"`
	BoostsURL       *string `json:"boosts_url,omitempty"`
	Position        *int32  `json:"position,omitempty"`
	Parent          *Parent `json:"parent,omitempty"`
	Bucket          *Bucket `json:"bucket,omitempty"`
	Creator         *Person `json:"creator,omitempty"`
	// AttachableSGID is present on the rich-text attachment variant only.
	AttachableSGID *string `json:"attachable_sgid,omitempty"`
	// Blob/file metadata (uploads and attachments).
	ContentType    *string `json:"content_type,omitempty"`
	ByteSize       *int64  `json:"byte_size,omitempty"`
	Filename       *string `json:"filename,omitempty"`
	DownloadURL    *string `json:"download_url,omitempty"`
	AppDownloadURL *string `json:"app_download_url,omitempty"`
	// Width and Height are null for non-image blobs and may be float-spelled
	// (1024.0) on the wire; narrowed to *int32 here (nil when absent/null).
	Width       *int32  `json:"width,omitempty"`
	Height      *int32  `json:"height,omitempty"`
	Description *string `json:"description,omitempty"`
	// DescriptionAttachments carries the rich-text companion array for the
	// upload/document Description (absent on the attachment variant).
	DescriptionAttachments *[]RichTextAttachment `json:"description_attachments,omitempty"`
	// Content and ContentAttachments carry the Document variant's rich-text body
	// (uploads and attachments omit them).
	Content            *string               `json:"content,omitempty"`
	ContentAttachments *[]RichTextAttachment `json:"content_attachments,omitempty"`
}

// UnmarshalJSON routes decoding through the generated EverythingFile so the
// public struct handles the float-encoded integers (1024.0) and null dimensions
// the BC3 API emits for width/height. Mirrors TimelineAttachment.UnmarshalJSON.
func (f *EverythingFile) UnmarshalJSON(data []byte) error {
	var gf generated.EverythingFile
	if err := json.Unmarshal(data, &gf); err != nil {
		return err
	}
	*f = everythingFileFromGenerated(gf)
	return nil
}

// everythingFileFromGenerated converts a generated EverythingFile (the
// optional-field superset) to the clean public type. Width and Height are
// optional/nullable *types.FlexInt in the generated type; a nil pointer leaves
// the public *int32 nil, and a present value is narrowed to int32.
func everythingFileFromGenerated(gf generated.EverythingFile) EverythingFile {
	f := EverythingFile{
		ID:               gf.Id,
		Status:           gf.Status,
		VisibleToClients: gf.VisibleToClients,
		CreatedAt:        gf.CreatedAt,
		UpdatedAt:        gf.UpdatedAt,
		Title:            gf.Title,
		InheritsStatus:   gf.InheritsStatus,
		Type:             gf.Type,
		URL:              gf.Url,
		AppURL:           gf.AppUrl,
		BookmarkURL:      gf.BookmarkUrl,
		SubscriptionURL:  gf.SubscriptionUrl,
		CommentsCount:    gf.CommentsCount,
		CommentsURL:      gf.CommentsUrl,
		BoostsCount:      gf.BoostsCount,
		BoostsURL:        gf.BoostsUrl,
		Position:         gf.Position,
		AttachableSGID:   gf.AttachableSgid,
		ContentType:      gf.ContentType,
		ByteSize:         gf.ByteSize,
		Filename:         gf.Filename,
		DownloadURL:      gf.DownloadUrl,
		AppDownloadURL:   gf.AppDownloadUrl,
		Description:      gf.Description,
		Content:          gf.Content,
	}
	if gf.Width != nil {
		w := int32(*gf.Width)
		f.Width = &w
	}
	if gf.Height != nil {
		h := int32(*gf.Height)
		f.Height = &h
	}
	if gf.Parent != nil {
		f.Parent = &Parent{ID: gf.Parent.Id, Title: gf.Parent.Title, Type: gf.Parent.Type, URL: gf.Parent.Url, AppURL: gf.Parent.AppUrl}
	}
	if gf.Bucket != nil {
		f.Bucket = &Bucket{ID: gf.Bucket.Id, Name: gf.Bucket.Name, Type: gf.Bucket.Type}
	}
	if gf.Creator != nil {
		creator := personFromGenerated(*gf.Creator)
		f.Creator = &creator
	}
	f.DescriptionAttachments = richTextAttachmentsPtrFromGenerated(gf.DescriptionAttachments)
	f.ContentAttachments = richTextAttachmentsPtrFromGenerated(gf.ContentAttachments)
	return f
}

// Messages returns every message across all accessible projects, newest-first.
// Pass a positive page to return only that page; page 0 follows the Link header
// across all pages.
func (s *EverythingService) Messages(ctx context.Context, page int32) (result *RecordingsPage, err error) {
	op := OperationInfo{Service: "Everything", Operation: "Messages", ResourceType: "recording", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingMessagesParams
	if page > 0 {
		params = &generated.GetEverythingMessagesParams{Page: &page}
	}
	resp, err := s.client.parent.gen.GetEverythingMessagesWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishRecordingsPage(ctx, resp.HTTPResponse, resp.Body, resp.JSON200, page)
}

// Comments returns every comment across all accessible projects, newest-first.
func (s *EverythingService) Comments(ctx context.Context, page int32) (result *RecordingsPage, err error) {
	op := OperationInfo{Service: "Everything", Operation: "Comments", ResourceType: "recording", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingCommentsParams
	if page > 0 {
		params = &generated.GetEverythingCommentsParams{Page: &page}
	}
	resp, err := s.client.parent.gen.GetEverythingCommentsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishRecordingsPage(ctx, resp.HTTPResponse, resp.Body, resp.JSON200, page)
}

// Checkins returns every automatic check-in answer across all accessible
// projects, newest-first.
func (s *EverythingService) Checkins(ctx context.Context, page int32) (result *RecordingsPage, err error) {
	op := OperationInfo{Service: "Everything", Operation: "Checkins", ResourceType: "recording", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingCheckinsParams
	if page > 0 {
		params = &generated.GetEverythingCheckinsParams{Page: &page}
	}
	resp, err := s.client.parent.gen.GetEverythingCheckinsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishRecordingsPage(ctx, resp.HTTPResponse, resp.Body, resp.JSON200, page)
}

// Forwards returns every inbox forward across all accessible projects,
// newest-first.
func (s *EverythingService) Forwards(ctx context.Context, page int32) (result *RecordingsPage, err error) {
	op := OperationInfo{Service: "Everything", Operation: "Forwards", ResourceType: "recording", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingForwardsParams
	if page > 0 {
		params = &generated.GetEverythingForwardsParams{Page: &page}
	}
	resp, err := s.client.parent.gen.GetEverythingForwardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishRecordingsPage(ctx, resp.HTTPResponse, resp.Body, resp.JSON200, page)
}

// finishRecordingsPage decodes the first page of a []Recording aggregate root
// and follows the Link header (unless a positive page was requested).
func (s *EverythingService) finishRecordingsPage(ctx context.Context, httpResp *http.Response, body []byte, json200 *[]generated.Recording, page int32) (*RecordingsPage, error) {
	if err := checkResponse(httpResp, body); err != nil {
		return nil, err
	}
	var recordings []Recording
	if json200 != nil {
		for _, gr := range *json200 {
			recordings = append(recordings, recordingFromGenerated(gr))
		}
	}
	totalCount := parseTotalCount(httpResp)
	if page > 0 {
		return &RecordingsPage{Recordings: recordings, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(httpResp)}}, nil
	}
	rawMore, truncated, err := s.client.parent.followPagination(ctx, httpResp, len(recordings), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var gr generated.Recording
		if err := json.Unmarshal(raw, &gr); err != nil {
			return nil, fmt.Errorf("failed to parse recording: %w", err)
		}
		recordings = append(recordings, recordingFromGenerated(gr))
	}
	return &RecordingsPage{Recordings: recordings, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Files returns every file recording across all accessible projects,
// newest-first. The feed is heterogeneous (uploads, documents, attachments);
// each element is an optional-field superset. Optional filters narrow by kind
// and creator.
func (s *EverythingService) Files(ctx context.Context, page int32, opts *EverythingFilesOptions) (result *EverythingFilesPage, err error) {
	op := OperationInfo{Service: "Everything", Operation: "Files", ResourceType: "file", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)

	var params *generated.GetEverythingFilesParams
	if opts != nil || page > 0 {
		params = &generated.GetEverythingFilesParams{}
		if page > 0 {
			params.Page = &page
		}
		if opts != nil {
			if opts.Kind != "" {
				params.Kind = &opts.Kind
			}
			if len(opts.PeopleIDs) > 0 {
				ids := append([]int64(nil), opts.PeopleIDs...)
				params.PeopleIds = &ids
			}
		}
	}

	resp, err := s.client.parent.gen.GetEverythingFilesWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	files, err := decodeEverythingFiles(resp.Body)
	if err != nil {
		return nil, err
	}
	totalCount := parseTotalCount(resp.HTTPResponse)
	if page > 0 {
		return &EverythingFilesPage{Files: files, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(files), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var gf generated.EverythingFile
		if err := json.Unmarshal(raw, &gf); err != nil {
			return nil, fmt.Errorf("failed to parse file: %w", err)
		}
		files = append(files, everythingFileFromGenerated(gf))
	}
	return &EverythingFilesPage{Files: files, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// decodeEverythingFiles decodes a files page (a bare JSON array) into clean
// EverythingFile values via the superset's UnmarshalJSON (which routes through
// the generated FlexInt-aware type).
func decodeEverythingFiles(body []byte) ([]EverythingFile, error) {
	var files []EverythingFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("failed to parse files: %w", err)
	}
	return files, nil
}

// OverdueTodos returns every overdue to-do across all accessible projects — a
// complete, oldest-due-date-first array (unpaginated, no Link-following).
func (s *EverythingService) OverdueTodos(ctx context.Context, filters *EverythingTaskFilters) (result []Todo, err error) {
	op := OperationInfo{Service: "Everything", Operation: "OverdueTodos", ResourceType: "todo", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)

	params := (*generated.GetEverythingOverdueTodosParams)(nil)
	if !filters.empty() {
		params = &generated.GetEverythingOverdueTodosParams{Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	resp, err := s.client.parent.gen.GetEverythingOverdueTodosWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	var todos []Todo
	if resp.JSON200 != nil {
		for _, gt := range *resp.JSON200 {
			todos = append(todos, todoFromGenerated(gt))
		}
	}
	return todos, nil
}

// OverdueCards returns every overdue card across all accessible projects — a
// complete, oldest-due-date-first array (unpaginated, no Link-following).
func (s *EverythingService) OverdueCards(ctx context.Context, filters *EverythingTaskFilters) (result []Card, err error) {
	op := OperationInfo{Service: "Everything", Operation: "OverdueCards", ResourceType: "card", IsMutation: false}
	ctx, done, err := s.begin(ctx, op)
	if err != nil {
		return nil, err
	}
	defer done(&err)

	params := (*generated.GetEverythingOverdueCardsParams)(nil)
	if !filters.empty() {
		params = &generated.GetEverythingOverdueCardsParams{Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	resp, err := s.client.parent.gen.GetEverythingOverdueCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	var cards []Card
	if resp.JSON200 != nil {
		for _, gc := range *resp.JSON200 {
			cards = append(cards, cardFromGenerated(gc))
		}
	}
	return cards, nil
}

// EverythingTaskFilters narrows the everything to-do/card listings (BC3 #12442).
// Both filters apply to every method in the family and compose.
type EverythingTaskFilters struct {
	// AssigneeIDs restricts to tasks assigned to at least one of these people.
	// Assignees on nested steps are not considered.
	AssigneeIDs []int64
	// Due filters by due date: "with", "without", or "overdue".
	Due string
}

func (f *EverythingTaskFilters) empty() bool {
	return f == nil || (len(f.AssigneeIDs) == 0 && f.Due == "")
}

func (f *EverythingTaskFilters) due() string {
	if f == nil {
		return ""
	}
	return f.Due
}

func (f *EverythingTaskFilters) assigneeIDs() *[]int64 {
	if f == nil || len(f.AssigneeIDs) == 0 {
		return nil
	}
	ids := append([]int64(nil), f.AssigneeIDs...)
	return &ids
}

// ---- bucket-grouped todo/card filter family ----

// BucketTodosGroup is one project's slice of a filtered to-do listing: the
// parent project and the matching to-dos (each carrying its steps).
type BucketTodosGroup struct {
	Bucket Bucket `json:"bucket"`
	Todos  []Todo `json:"todos"`
}

// BucketCardsGroup is one project's slice of a filtered card listing.
type BucketCardsGroup struct {
	Bucket Bucket `json:"bucket"`
	Cards  []Card `json:"cards"`
}

// BucketTodosGroupsPage is a page-followed list of to-do bucket groups.
type BucketTodosGroupsPage struct {
	Groups []BucketTodosGroup
	Meta   ListMeta
}

// BucketCardsGroupsPage is a page-followed list of card bucket groups.
type BucketCardsGroupsPage struct {
	Groups []BucketCardsGroup
	Meta   ListMeta
}

func bucketTodosGroupFromGenerated(g generated.BucketTodosGroup) BucketTodosGroup {
	grp := BucketTodosGroup{
		Bucket: Bucket{ID: g.Bucket.Id, Name: g.Bucket.Name, Type: g.Bucket.Type},
		Todos:  make([]Todo, 0, len(g.Todos)),
	}
	for _, gt := range g.Todos {
		grp.Todos = append(grp.Todos, todoFromGenerated(gt))
	}
	return grp
}

func bucketCardsGroupFromGenerated(g generated.BucketCardsGroup) BucketCardsGroup {
	grp := BucketCardsGroup{
		Bucket: Bucket{ID: g.Bucket.Id, Name: g.Bucket.Name, Type: g.Bucket.Type},
		Cards:  make([]Card, 0, len(g.Cards)),
	}
	for _, gc := range g.Cards {
		grp.Cards = append(grp.Cards, cardFromGenerated(gc))
	}
	return grp
}

// OpenTodos returns active, incomplete to-dos across all accessible projects,
// grouped by project (paginated). Pass a positive page to return only that page;
// page 0 follows the Link header across all pages.
func (s *EverythingService) OpenTodos(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketTodosGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "OpenTodos", ResourceType: "todo"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingOpenTodosParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingOpenTodosParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingOpenTodosWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishTodoGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// CompletedTodos returns completed to-dos, grouped by project (paginated).
func (s *EverythingService) CompletedTodos(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketTodosGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "CompletedTodos", ResourceType: "todo"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingCompletedTodosParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingCompletedTodosParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingCompletedTodosWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishTodoGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// UnassignedTodos returns open, unassigned to-dos, grouped by project (paginated).
func (s *EverythingService) UnassignedTodos(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketTodosGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "UnassignedTodos", ResourceType: "todo"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingUnassignedTodosParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingUnassignedTodosParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingUnassignedTodosWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishTodoGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// NoDueDateTodos returns open to-dos with no due date, grouped by project (paginated).
func (s *EverythingService) NoDueDateTodos(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketTodosGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "NoDueDateTodos", ResourceType: "todo"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingNoDueDateTodosParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingNoDueDateTodosParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingNoDueDateTodosWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishTodoGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// OpenCards returns incomplete cards in active columns, grouped by project (paginated).
func (s *EverythingService) OpenCards(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketCardsGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "OpenCards", ResourceType: "card"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingOpenCardsParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingOpenCardsParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingOpenCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishCardGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// CompletedCards returns completed cards, grouped by project (paginated).
func (s *EverythingService) CompletedCards(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketCardsGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "CompletedCards", ResourceType: "card"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingCompletedCardsParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingCompletedCardsParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingCompletedCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishCardGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// UnassignedCards returns open, unassigned cards, grouped by project (paginated).
func (s *EverythingService) UnassignedCards(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketCardsGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "UnassignedCards", ResourceType: "card"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingUnassignedCardsParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingUnassignedCardsParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingUnassignedCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishCardGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// NoDueDateCards returns open cards with no due date, grouped by project (paginated).
func (s *EverythingService) NoDueDateCards(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketCardsGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "NoDueDateCards", ResourceType: "card"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingNoDueDateCardsParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingNoDueDateCardsParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingNoDueDateCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishCardGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

// NotNowCards returns cards parked in a project's "Not now" column, grouped by
// project (paginated).
func (s *EverythingService) NotNowCards(ctx context.Context, page int32, filters *EverythingTaskFilters) (result *BucketCardsGroupsPage, err error) {
	ctx, done, err := s.begin(ctx, OperationInfo{Service: "Everything", Operation: "NotNowCards", ResourceType: "card"})
	if err != nil {
		return nil, err
	}
	defer done(&err)
	var params *generated.GetEverythingNotNowCardsParams
	if page > 0 || !filters.empty() {
		params = &generated.GetEverythingNotNowCardsParams{Page: omitzero(page), Due: omitzero(filters.due()), AssigneeIds: filters.assigneeIDs()}
	}
	r, err := s.client.parent.gen.GetEverythingNotNowCardsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	return s.finishCardGroupsPage(ctx, r.HTTPResponse, r.Body, r.JSON200, page)
}

func (s *EverythingService) finishTodoGroupsPage(ctx context.Context, httpResp *http.Response, body []byte, json200 *[]generated.BucketTodosGroup, page int32) (*BucketTodosGroupsPage, error) {
	if err := checkResponse(httpResp, body); err != nil {
		return nil, err
	}
	var groups []BucketTodosGroup
	if json200 != nil {
		for _, g := range *json200 {
			groups = append(groups, bucketTodosGroupFromGenerated(g))
		}
	}
	totalCount := parseTotalCount(httpResp)
	if page > 0 {
		return &BucketTodosGroupsPage{Groups: groups, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(httpResp)}}, nil
	}
	rawMore, truncated, err := s.client.parent.followPagination(ctx, httpResp, len(groups), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var g generated.BucketTodosGroup
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("failed to parse bucket todo group: %w", err)
		}
		groups = append(groups, bucketTodosGroupFromGenerated(g))
	}
	return &BucketTodosGroupsPage{Groups: groups, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

func (s *EverythingService) finishCardGroupsPage(ctx context.Context, httpResp *http.Response, body []byte, json200 *[]generated.BucketCardsGroup, page int32) (*BucketCardsGroupsPage, error) {
	if err := checkResponse(httpResp, body); err != nil {
		return nil, err
	}
	var groups []BucketCardsGroup
	if json200 != nil {
		for _, g := range *json200 {
			groups = append(groups, bucketCardsGroupFromGenerated(g))
		}
	}
	totalCount := parseTotalCount(httpResp)
	if page > 0 {
		return &BucketCardsGroupsPage{Groups: groups, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(httpResp)}}, nil
	}
	rawMore, truncated, err := s.client.parent.followPagination(ctx, httpResp, len(groups), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var g generated.BucketCardsGroup
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("failed to parse bucket card group: %w", err)
		}
		groups = append(groups, bucketCardsGroupFromGenerated(g))
	}
	return &BucketCardsGroupsPage{Groups: groups, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// begin runs the gating + start/end hook lifecycle shared by the everything
// methods and returns the (possibly gated) context plus a deferred finisher.
func (s *EverythingService) begin(ctx context.Context, op OperationInfo) (context.Context, func(*error), error) {
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		var err error
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return ctx, func(*error) {}, err
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	return ctx, func(errp *error) {
		var e error
		if errp != nil {
			e = *errp
		}
		s.client.parent.hooks.OnOperationEnd(ctx, op, e, time.Since(start))
	}, nil
}
