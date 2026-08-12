package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// SearchResult represents a single search result from the Basecamp API.
//
// It is a polymorphic projection: most result types render the common
// recording envelope plus their own partial, but BC3 special-cases four
// branches — chat lines, kanban (card table) lists, file attachments and
// gauge needles — each contributing branch-specific fields below. A
// file-attachment hit omits the top-level ID/Title/Type/URL/AppURL envelope
// keys entirely (they stay zero-valued) and carries the file fields
// (Filename through AppDownloadURL) instead.
type SearchResult struct {
	ID               int64  `json:"id"`
	Status           string `json:"status"`
	VisibleToClients bool   `json:"visible_to_clients"`
	// CreatedAt and UpdatedAt are optional on this polymorphic projection —
	// neither carries @required in the spec, and the generated client types
	// both as *time.Time. Pointers here so an absent timestamp stays
	// distinguishable rather than collapsing to 0001-01-01T00:00:00Z; the tags
	// carry omitempty so absence round-trips as an omitted key instead of the
	// fabricated zero time (encoding/json never treats a struct as empty, so a
	// value-typed field would always emit).
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	Title          string     `json:"title"`
	InheritsStatus bool       `json:"inherits_status"`
	Type           string     `json:"type"`
	URL            string     `json:"url"`
	AppURL         string     `json:"app_url"`
	BookmarkURL    string     `json:"bookmark_url"`
	// BubbleUpURL is the URL of the Bubble Up record for this recording. Optional
	// on this polymorphic projection: recordings/_recording.json.jbuilder emits
	// the key only when the caller passes bubbleupable, and todolists/_todolist
	// is the only partial that does — so a Todolist-shaped instance carries it
	// and the other recording types do not.
	BubbleUpURL string  `json:"bubble_up_url,omitempty"`
	Parent      *Parent `json:"parent,omitempty"`
	Bucket      *Bucket `json:"bucket,omitempty"`
	Creator     *Person `json:"creator,omitempty"`
	// Content and Description are always present on the wire and always null:
	// api/searches/show.json.jbuilder renders the recording's own partial and
	// then unconditionally overwrites both with nil to keep the large HTML body
	// out of the search payload. They are modeled as pointers so the guaranteed
	// null round-trips faithfully rather than collapsing to "". Read
	// PlainTextContent and PlainTextDescription instead.
	Content     *string `json:"content"`
	Description *string `json:"description"`
	// PlainTextContent and PlainTextDescription are highlighted, truncated
	// excerpts — NOT plain text despite the name. BC3 converts the rich text
	// with to_plain_text, escapes it with html_escape_once, wraps each query
	// match in <mark class="circled-text"><span></span>…</mark>, and truncates
	// to 300 characters. Treat them as HTML fragments. Optional and
	// non-nullable: a result whose recordable has no such attribute omits the
	// key rather than sending null.
	PlainTextContent     string `json:"plain_text_content,omitempty"`
	PlainTextDescription string `json:"plain_text_description,omitempty"`
	// ContentAttachments and DescriptionAttachments are the rich text companion
	// arrays carried through the polymorphic search projection. A given result
	// is one recording type, so it carries only the array matching its rich text
	// attribute (ContentAttachments for a Comment/Message, DescriptionAttachments
	// for a Todo); a webhook-sourced result carries neither. Optional and
	// non-nullable; modeled as a pointer to a slice with omitempty so all three
	// wire states round-trip faithfully — nil pointer (absent) is omitted, a
	// non-nil pointer to an empty slice re-encodes as [], and a populated one as
	// the list. See Recording for the same contract and RichTextAttachment.
	ContentAttachments     *[]RichTextAttachment `json:"content_attachments,omitempty"`
	DescriptionAttachments *[]RichTextAttachment `json:"description_attachments,omitempty"`
	// Attachments carries the result's file attachments, in either of two wire
	// shapes — see SearchResultAttachment. For a result whose recordable has
	// downloadable rich-text attachments it repeats the companion array above;
	// for a chat upload line it is the bespoke six-key aggregate the line
	// builds inline.
	Attachments []SearchResultAttachment `json:"attachments,omitempty"`
	Subject     string                   `json:"subject,omitempty"`

	// SubscriptionURL is emitted by the recording envelope for any
	// subscribable result — kanban lists and gauge needles among the special
	// branches, plus subscribable generic types (messages, todos, …).
	SubscriptionURL string `json:"subscription_url,omitempty"`
	// Position is shared by two emitters: the envelope's list position for
	// positioned recordings (kanban lists among the special branches), and the
	// gauge-needle branch's own 0–100 gauge position.
	Position int `json:"position,omitempty"`
	// CommentsCount/CommentsURL ride the envelope's commentable flag — gauge
	// needles among the special branches, plus commentable generic types.
	CommentsCount int    `json:"comments_count,omitempty"`
	CommentsURL   string `json:"comments_url,omitempty"`
	// BoostsCount/BoostsURL ride the envelope's boostable flag — chat lines
	// and gauge needles.
	BoostsCount int    `json:"boosts_count,omitempty"`
	BoostsURL   string `json:"boosts_url,omitempty"`

	// Language is the language of a code chat line.
	Language string `json:"language,omitempty"`
	// ImageURL is set on a play-kind chat line whose sound carries an image.
	ImageURL string `json:"image_url,omitempty"`
	// SoundURL is always set on a play-kind chat line.
	SoundURL string `json:"sound_url,omitempty"`

	// Subscribers lists everyone subscribed to a kanban list, as full Person
	// projections.
	Subscribers []Person `json:"subscribers,omitempty"`
	// Color of a kanban list or gauge needle; both branches emit the key
	// unconditionally with a null value when unset, which collapses to "".
	Color        string            `json:"color,omitempty"`
	CardsCount   int               `json:"cards_count,omitempty"`
	CommentCount int               `json:"comment_count,omitempty"`
	CardsURL     string            `json:"cards_url,omitempty"`
	OnHold       *CardColumnOnHold `json:"on_hold,omitempty"`

	// Filename through AppDownloadURL are the file-attachment branch's keys —
	// the one branch that omits the ID/Title/Type/URL/AppURL envelope keys.
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	ByteSize    int64  `json:"byte_size,omitempty"`
	Previewable bool   `json:"previewable,omitempty"`
	// Width and Height are emitted only for previewable files, may be
	// float-spelled (1024.0) on the wire, and are nullable — hence *int32,
	// like RichTextAttachment's dimensions.
	Width          *int32 `json:"width,omitempty"`
	Height         *int32 `json:"height,omitempty"`
	PreviewURL     string `json:"preview_url,omitempty"`
	ThumbnailURL   string `json:"thumbnail_url,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	AppDownloadURL string `json:"app_download_url,omitempty"`
}

// SearchResultAttachment is a file attached to a search result, in either of
// two wire shapes: the rich-text attachment/blob shape (the same emitters
// behind RichTextAttachment) re-emitted under the result's generic
// `attachments` key, or the bespoke six-key aggregate a chat upload line
// builds inline. Only the four fields both variants always emit are
// guaranteed; the rest identify their variant and stay zero-valued on the
// other.
type SearchResultAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	DownloadURL string `json:"download_url"`

	// Rich-text attachment/blob variant.
	ID           int64  `json:"id,omitempty"`
	SGID         string `json:"sgid,omitempty"`
	Previewable  bool   `json:"previewable,omitempty"`
	PreviewURL   string `json:"preview_url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	// Width and Height may be float-spelled (1024.0) on the wire and are null
	// for non-image blobs — hence *int32, like RichTextAttachment's.
	Width  *int32 `json:"width,omitempty"`
	Height *int32 `json:"height,omitempty"`

	// Chat upload-line variant.
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

// UnmarshalJSON decodes a SearchResult from JSON, handling the BC3 API's
// float-encoded integer dimensions (e.g. "width": 1920.0) on a
// file-attachment hit. Delegates through the generated type (which uses
// FlexInt) so the public struct can keep *int32 dimensions while remaining
// directly decodable from the API wire format. Mirrors Upload.UnmarshalJSON.
func (sr *SearchResult) UnmarshalJSON(data []byte) error {
	var gsr generated.SearchResult
	if err := json.Unmarshal(data, &gsr); err != nil {
		return err
	}
	*sr = searchResultFromGenerated(gsr)
	return nil
}

// UnmarshalJSON decodes a SearchResultAttachment from JSON — the rich-text
// variant's dimensions arrive float-spelled (1024.0), exactly like
// RichTextAttachment's. Delegates through the generated type (FlexInt), so
// the elements of a directly-decoded SearchResult.Attachments handle them.
func (a *SearchResultAttachment) UnmarshalJSON(data []byte) error {
	var ga generated.SearchResultAttachment
	if err := json.Unmarshal(data, &ga); err != nil {
		return err
	}
	*a = searchResultAttachmentFromGenerated(ga)
	return nil
}

// SearchMetadata represents the available search filter options returned by
// GET /searches/metadata.json.
type SearchMetadata struct {
	// RecordingSearchTypes are the selectable recording-type filters. Pass a
	// non-nil Key as a SearchOptions.TypeNames value; a nil Key is the default
	// "everything" option.
	RecordingSearchTypes []SearchType `json:"recording_search_types"`
	// FileSearchTypes are the selectable file-type filters. Pass a non-nil Key
	// as SearchOptions.FileType; a nil Key is the default "all files" option.
	FileSearchTypes []SearchType `json:"file_search_types"`
	// DefaultCreatorLabel is the label for the unfiltered creator option.
	DefaultCreatorLabel string `json:"default_creator_label"`
	// DefaultBucketLabel is the label for the unfiltered project option.
	DefaultBucketLabel string `json:"default_bucket_label"`
	// DefaultCircleLabel is the label for the unfiltered ping option.
	DefaultCircleLabel string `json:"default_circle_label"`
	// DefaultFileTypeLabel is the label for the unfiltered file-type option.
	DefaultFileTypeLabel string `json:"default_file_type_label"`
	// DefaultTypeLabel is the label for the unfiltered recording-type option.
	DefaultTypeLabel string `json:"default_type_label"`
}

// SearchType is a selectable search filter option. Key is the value passed back
// as a filter parameter; a nil Key (JSON null on the wire) represents the
// default "everything"/"all files" option. Value is the human-readable label.
type SearchType struct {
	Key   *string `json:"key"`
	Value string  `json:"value"`
}

// SearchListResult contains the results from searching.
type SearchListResult struct {
	// Results is the list of search results returned.
	Results []SearchResult
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SearchOptions specifies optional parameters for search.
type SearchOptions struct {
	// Sort specifies the sort order: "best_match" (default, relevance with a
	// recency boost) or "recency" (strictly newest first).
	Sort string

	// TypeNames restricts results to the given recording types. Use Key values
	// from SearchMetadata.RecordingSearchTypes.
	TypeNames []string

	// BucketIds restricts results to the given project IDs.
	BucketIds []int64

	// CreatorIds restricts results to the given creator person IDs.
	CreatorIds []int64

	// FileType filters attachments by type. Use a Key value from
	// SearchMetadata.FileSearchTypes.
	FileType string

	// ExcludeChat excludes chat results when true.
	ExcludeChat bool

	// Since bounds results to a time range: "last_7_days", "last_30_days",
	// "last_90_days", "last_12_months", or "forever" (the default).
	Since string

	// Type is the deprecated single-recording-type filter. Prefer TypeNames.
	//
	// Deprecated: use TypeNames.
	Type string

	// BucketID is the deprecated single-project filter. Prefer BucketIds.
	//
	// Deprecated: use BucketIds.
	BucketID int64

	// CreatorID is the deprecated single-creator filter. Prefer CreatorIds.
	//
	// Deprecated: use CreatorIds.
	CreatorID int64

	// Limit is the maximum number of results to return.
	// If 0 (default), returns all results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	Page int
}

// SearchService handles search operations.
type SearchService struct {
	client *AccountClient
}

// NewSearchService creates a new SearchService.
func NewSearchService(client *AccountClient) *SearchService {
	return &SearchService{client: client}
}

// Search searches for content across the account.
// The query parameter is the search string.
//
// Pagination options:
//   - Limit: maximum number of results to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned SearchListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *SearchService) Search(ctx context.Context, query string, opts *SearchOptions) (result *SearchListResult, err error) {
	op := OperationInfo{
		Service: "Search", Operation: "Search",
		ResourceType: "search", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if query == "" {
		err = ErrUsage("search query is required")
		return nil, err
	}

	params := &generated.SearchParams{
		Q: query,
	}
	if opts != nil {
		if opts.Page > 0 {
			var page *int32
			if page, err = pageParam(opts.Page); err != nil {
				return nil, err
			}
			params.Page = page
		}
		if opts.Sort != "" {
			params.Sort = &opts.Sort
		}
		// Array filters map onto the generated params, which own the wire
		// encoding (form:"bucket_ids[]" tags → repeated bucket_ids%5B%5D=…
		// pairs); no URL rewriting here. The params are *pointer* slices so the
		// generated client omits them entirely when unset — pass a pointer only
		// for a non-empty slice, else an empty `bucket_ids[]=` would reach Rails
		// and normalize to a bogus [0] filter.
		if len(opts.TypeNames) > 0 {
			params.TypeNames = &opts.TypeNames
		}
		if len(opts.BucketIds) > 0 {
			params.BucketIds = &opts.BucketIds
		}
		if len(opts.CreatorIds) > 0 {
			params.CreatorIds = &opts.CreatorIds
		}
		if opts.FileType != "" {
			params.FileType = &opts.FileType
		}
		params.ExcludeChat = omitzero(opts.ExcludeChat)
		if opts.Since != "" {
			params.Since = &opts.Since
		}
		// Deprecated singular filters (prefer the plural array forms above).
		if opts.Type != "" {
			params.Type = &opts.Type
		}
		if opts.BucketID != 0 {
			params.BucketId = &opts.BucketID
		}
		if opts.CreatorID != 0 {
			params.CreatorId = &opts.CreatorID
		}
	}

	resp, err := s.client.parent.gen.SearchWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var searchResults []SearchResult
	if resp.JSON200 != nil {
		for _, gsr := range *resp.JSON200 {
			searchResults = append(searchResults, searchResultFromGenerated(gsr))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(searchResults), opts.Limit, resp.HTTPResponse)
		return &SearchListResult{Results: searchResults[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for search)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(searchResults) >= limit {
		return &SearchListResult{Results: searchResults[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(searchResults), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(searchResults), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gsr generated.SearchResult
		if err := json.Unmarshal(raw, &gsr); err != nil {
			return nil, fmt.Errorf("failed to parse search result: %w", err)
		}
		searchResults = append(searchResults, searchResultFromGenerated(gsr))
	}

	return &SearchListResult{Results: searchResults, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Metadata returns the available search filter options: the selectable
// recording- and file-search types and the default (unfiltered) labels.
func (s *SearchService) Metadata(ctx context.Context) (result *SearchMetadata, err error) {
	op := OperationInfo{
		Service: "Search", Operation: "Metadata",
		ResourceType: "search", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetSearchMetadataWithResponse(ctx, s.client.accountID)
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

	// Convert metadata
	metadata := &SearchMetadata{
		RecordingSearchTypes: searchTypesFromGenerated(resp.JSON200.RecordingSearchTypes),
		FileSearchTypes:      searchTypesFromGenerated(resp.JSON200.FileSearchTypes),
		DefaultCreatorLabel:  resp.JSON200.DefaultCreatorLabel,
		DefaultBucketLabel:   resp.JSON200.DefaultBucketLabel,
		DefaultCircleLabel:   resp.JSON200.DefaultCircleLabel,
		DefaultFileTypeLabel: resp.JSON200.DefaultFileTypeLabel,
		DefaultTypeLabel:     resp.JSON200.DefaultTypeLabel,
	}

	return metadata, nil
}

// searchTypesFromGenerated converts generated SearchType filter options to the
// clean wrapper type.
func searchTypesFromGenerated(gts []generated.SearchType) []SearchType {
	types := make([]SearchType, 0, len(gts))
	for _, gt := range gts {
		types = append(types, SearchType{Key: gt.Key, Value: gt.Value})
	}
	return types
}

// searchResultFromGenerated converts a generated SearchResult to our clean SearchResult type.
func searchResultFromGenerated(gsr generated.SearchResult) SearchResult {
	sr := SearchResult{
		Status:               deref(gsr.Status),
		VisibleToClients:     deref(gsr.VisibleToClients),
		CreatedAt:            gsr.CreatedAt,
		UpdatedAt:            gsr.UpdatedAt,
		Title:                deref(gsr.Title),
		InheritsStatus:       deref(gsr.InheritsStatus),
		Type:                 deref(gsr.Type),
		URL:                  deref(gsr.Url),
		AppURL:               deref(gsr.AppUrl),
		BookmarkURL:          deref(gsr.BookmarkUrl),
		BubbleUpURL:          deref(gsr.BubbleUpUrl),
		Content:              gsr.Content,
		Description:          gsr.Description,
		PlainTextContent:     deref(gsr.PlainTextContent),
		PlainTextDescription: deref(gsr.PlainTextDescription),
		Subject:              deref(gsr.Subject),
		SubscriptionURL:      deref(gsr.SubscriptionUrl),
		Position:             int(deref(gsr.Position)),
		CommentsCount:        int(deref(gsr.CommentsCount)),
		CommentsURL:          deref(gsr.CommentsUrl),
		BoostsCount:          int(deref(gsr.BoostsCount)),
		BoostsURL:            deref(gsr.BoostsUrl),
		Language:             deref(gsr.Language),
		ImageURL:             deref(gsr.ImageUrl),
		SoundURL:             deref(gsr.SoundUrl),
		Color:                deref(gsr.Color),
		CardsCount:           int(deref(gsr.CardsCount)),
		CommentCount:         int(deref(gsr.CommentCount)),
		CardsURL:             deref(gsr.CardsUrl),
		Filename:             deref(gsr.Filename),
		ContentType:          deref(gsr.ContentType),
		ByteSize:             deref(gsr.ByteSize),
		Previewable:          deref(gsr.Previewable),
		PreviewURL:           deref(gsr.PreviewUrl),
		ThumbnailURL:         deref(gsr.ThumbnailUrl),
		DownloadURL:          deref(gsr.DownloadUrl),
		AppDownloadURL:       deref(gsr.AppDownloadUrl),
	}

	if gsr.Id != nil {
		sr.ID = *gsr.Id
	}

	// Width/Height arrive as optional/nullable *types.FlexInt (the wire may
	// float-spell them); narrow a present value to the public *int32 and leave
	// absent-or-null nil, exactly like richTextAttachmentFromGenerated.
	if gsr.Width != nil {
		w := int32(*gsr.Width)
		sr.Width = &w
	}
	if gsr.Height != nil {
		h := int32(*gsr.Height)
		sr.Height = &h
	}

	if gsr.OnHold != nil {
		sr.OnHold = &CardColumnOnHold{
			ID:             gsr.OnHold.Id,
			Status:         gsr.OnHold.Status,
			InheritsStatus: gsr.OnHold.InheritsStatus,
			Title:          gsr.OnHold.Title,
			CreatedAt:      gsr.OnHold.CreatedAt,
			UpdatedAt:      gsr.OnHold.UpdatedAt,
			CardsCount:     int(gsr.OnHold.CardsCount),
			CardsURL:       gsr.OnHold.CardsUrl,
		}
	}

	if len(gsr.Subscribers) > 0 {
		sr.Subscribers = make([]Person, 0, len(gsr.Subscribers))
		for _, gs := range gsr.Subscribers {
			sr.Subscribers = append(sr.Subscribers, personFromGenerated(gs))
		}
	}

	if len(gsr.Attachments) > 0 {
		sr.Attachments = make([]SearchResultAttachment, 0, len(gsr.Attachments))
		for _, ga := range gsr.Attachments {
			sr.Attachments = append(sr.Attachments, searchResultAttachmentFromGenerated(ga))
		}
	}

	// Convert nested types
	if gsr.Parent != nil {
		sr.Parent = &Parent{
			ID:     gsr.Parent.Id,
			Title:  gsr.Parent.Title,
			Type:   gsr.Parent.Type,
			URL:    gsr.Parent.Url,
			AppURL: gsr.Parent.AppUrl,
		}
	}

	if gsr.Bucket != nil {
		sr.Bucket = &Bucket{
			ID:   gsr.Bucket.Id,
			Name: gsr.Bucket.Name,
			Type: gsr.Bucket.Type,
		}
	}

	if gsr.Creator != nil {
		creator := personFromGenerated(*gsr.Creator)
		sr.Creator = &creator
	}

	sr.ContentAttachments = richTextAttachmentsPtrFromGenerated(gsr.ContentAttachments)
	sr.DescriptionAttachments = richTextAttachmentsPtrFromGenerated(gsr.DescriptionAttachments)

	return sr
}

// searchResultAttachmentFromGenerated converts a generated
// SearchResultAttachment to our clean type. The four both-variant fields are
// @required in the schema and copied directly; the rest deref to their zero
// values when the other variant omits them. Width and Height narrow a present
// *types.FlexInt to *int32, like richTextAttachmentFromGenerated.
func searchResultAttachmentFromGenerated(ga generated.SearchResultAttachment) SearchResultAttachment {
	a := SearchResultAttachment{
		Filename:     ga.Filename,
		ContentType:  ga.ContentType,
		ByteSize:     ga.ByteSize,
		DownloadURL:  ga.DownloadUrl,
		ID:           deref(ga.Id),
		SGID:         deref(ga.Sgid),
		Previewable:  deref(ga.Previewable),
		PreviewURL:   deref(ga.PreviewUrl),
		ThumbnailURL: deref(ga.ThumbnailUrl),
		Title:        deref(ga.Title),
		URL:          deref(ga.Url),
	}
	if ga.Width != nil {
		w := int32(*ga.Width)
		a.Width = &w
	}
	if ga.Height != nil {
		h := int32(*ga.Height)
		a.Height = &h
	}
	return a
}
