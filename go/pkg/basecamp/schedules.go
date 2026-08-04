package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// ScheduleEntryListOptions specifies options for listing schedule entries.
type ScheduleEntryListOptions struct {
	// Limit is the maximum number of entries to return.
	// If 0 (default), returns all entries. Use a positive value to cap results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int

	// Status filters entries by status: "active", "archived", or "trashed".
	// If empty, returns active entries (API default).
	Status string
}

// Schedule represents a Basecamp schedule (calendar) within a project.
type Schedule struct {
	ID                    int64     `json:"id"`
	Status                string    `json:"status"`
	VisibleToClients      bool      `json:"visible_to_clients"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	Title                 string    `json:"title"`
	InheritsStatus        bool      `json:"inherits_status"`
	Type                  string    `json:"type"`
	URL                   string    `json:"url"`
	AppURL                string    `json:"app_url"`
	BookmarkURL           string    `json:"bookmark_url"`
	Position              int       `json:"position"`
	Bucket                *Bucket   `json:"bucket,omitempty"`
	Creator               *Person   `json:"creator,omitempty"`
	IncludeDueAssignments bool      `json:"include_due_assignments"`
	EntriesCount          int       `json:"entries_count"`
	EntriesURL            string    `json:"entries_url"`
}

// ScheduleEntry represents an event on a Basecamp schedule.
type ScheduleEntry struct {
	ID               int64              `json:"id"`
	Status           string             `json:"status"`
	VisibleToClients bool               `json:"visible_to_clients"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Title            string             `json:"title"`
	Summary          string             `json:"summary"`
	InheritsStatus   bool               `json:"inherits_status"`
	Type             string             `json:"type"`
	URL              string             `json:"url"`
	AppURL           string             `json:"app_url"`
	BookmarkURL      string             `json:"bookmark_url"`
	BoostsCount      int                `json:"boosts_count,omitempty"`
	BoostsURL        string             `json:"boosts_url,omitempty"`
	SubscriptionURL  string             `json:"subscription_url"`
	CommentsURL      string             `json:"comments_url"`
	CommentsCount    int                `json:"comments_count"`
	StartsAt         types.FlexibleTime `json:"starts_at"`
	EndsAt           types.FlexibleTime `json:"ends_at"`
	AllDay           bool               `json:"all_day"`
	Description      string             `json:"description"`
	// DescriptionAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Description. @required — the API always sends this
	// array (empty when the description has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	// JoinURL is the entry's join link — a video-call URL or similar — or ""
	// when it has none.
	//
	// Spelled join_url on the way in and url on the way out: URL above is the
	// entry's own Basecamp API URL, written by a partial that renders before
	// this one, so BC3 emits the join link under a non-colliding key. Write it
	// back through ReplaceScheduleEntryRequest.URL, never through this field's
	// namesake. Echoing URL into the request would store the API URL as the
	// join link.
	//
	// Absent from the reduced calendar partial GetUpcomingSchedule renders, so
	// "" means either "no join link" or "this shape does not carry one".
	JoinURL string `json:"join_url"`
	// Highlighted reports whether the entry is highlighted on the schedule.
	// Absent from the reduced calendar partial, like JoinURL.
	Highlighted  bool     `json:"highlighted"`
	Parent       *Parent  `json:"parent,omitempty"`
	Bucket       *Bucket  `json:"bucket,omitempty"`
	Creator      *Person  `json:"creator,omitempty"`
	Participants []Person `json:"participants,omitempty"`
}

// CreateScheduleEntryRequest specifies the parameters for creating a schedule entry.
//
// BREAKING CHANGE: AllDay changed from bool to *bool so that
// "not provided" (nil) is distinguishable from "set to false". Use
// a bool variable and take its address (&v) to set explicitly.
type CreateScheduleEntryRequest struct {
	// Summary is the event title (required).
	Summary string `json:"summary"`
	// StartsAt is the event start time (required, ISO 8601 format).
	StartsAt string `json:"starts_at"`
	// EndsAt is the event end time (required, ISO 8601 format).
	EndsAt string `json:"ends_at"`
	// Description is the event details in HTML (optional).
	Description string `json:"description,omitempty"`
	// ParticipantIDs is a list of people IDs to assign (optional).
	ParticipantIDs []int64 `json:"participant_ids,omitempty"`
	// AllDay indicates if this is an all-day event (optional).
	// Use a pointer to distinguish "not set" from "set to false".
	AllDay *bool `json:"all_day,omitempty"`
	// Notify triggers participant notifications when true (optional).
	Notify bool `json:"notify,omitempty"`
	// Subscriptions controls who gets notified and subscribed.
	// nil: field omitted (server default). &[]int64{}: subscribe nobody. &[]int64{1,2}: those people.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
	// URL is the entry's join link — a video-call URL or similar, up to 2500
	// characters, validated as a URL when present. A scheme-less value is
	// normalized to https://.
	//
	// Spell it URL here and read it back as JoinURL: the response's url is the
	// entry's own Basecamp API URL, written by a partial that renders before this
	// field, so BC3 emits the join link under a non-colliding key. Sending
	// join_url on write is silently dropped by strong parameters and the create
	// succeeds with no join link.
	//
	// nil omits the key. There is no carve-out on create — BC3 builds a fresh
	// record — so the pointer only distinguishes unset from an explicit empty
	// string.
	URL *string `json:"url,omitempty"`
	// Highlighted highlights the entry on the schedule. Defaults to false.
	//
	// Do not send an explicit null: schedule_entries.highlighted is NOT NULL, so
	// BC3 raises rather than falling back to the default. nil omits the key,
	// which is what a Go caller gets by leaving this unset.
	Highlighted *bool `json:"highlighted,omitempty"`
	// Status is the publication state at creation: "active" (the API default) or
	// "drafted".
	//
	// A top-level parameter rather than one of the entry's attributes: status is
	// a Recording column, so wrap_parameters leaves it outside the schedule_entry
	// envelope. On create BC3 accepts "drafted", "active", "archived" or
	// "trashed" and answers 400 — not 422 — for anything else.
	Status *string `json:"status,omitempty"`
	// VisibleToClients sets client visibility at create time (optional, tri-state).
	// nil omits the field so the server applies its own default visibility rule; a
	// non-nil value is sent verbatim, and an explicit false reaches the wire (the
	// pointer distinguishes unset from false).
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}

// PUT /{accountId}/schedule_entries/{entryId} is a FULL REPLACE.
// Schedules::EntriesController#update rebuilds the recordable from the
// submitted params, so a writable field the body omits is cleared — with three
// exceptions BC3 seeds from the existing record when the request does not
// address them:
//
//	PRESERVED_ON_OMISSION = %i[ url highlighted ]   # plus participant_ids, guarded since bc3#12425
//
// Hence the three-method write surface below. ReplaceEntry is the verbatim,
// destructive PUT; UpdateEntry overlays onto the current state; EditEntry hands
// the caller that state. The split between "full state" and "addressed-only" is
// the whole of the serialization contract:
//
//   - FULL STATE — summary, starts_at, ends_at, description, all_day. Always
//     sent by UpdateEntry and EditEntry, empties included: "" is how a clear is
//     expressed on a full-replace endpoint, never JSON null (SPEC §18) and never
//     omission, which would hand the clear back to the server's own rebuild and
//     read as an accident rather than an intent.
//
//   - ADDRESSED-ONLY — participant_ids, url, highlighted, notify. Sent only
//     when the caller addressed them, and NEVER seeded from the read-back. BC3
//     preserves the first three server-side, so resending is redundant at best
//     and wrong if the read raced a concurrent change; and the response spells
//     the join link join_url, so echoing the response's url would write the
//     entry's own API URL into the join link. notify is addressed-only because
//     it is a directive, not state: sending it makes BC3 recompute a drafted
//     entry's subscriber list.
//
// This route serves non-recurring entries only: ensure_non_recurring_event
// 302-redirects both show and update for a recurring entry, and the SDK does
// not follow a redirect on a PUT.

// UpdateScheduleEntryRequest specifies the fields to set on a schedule entry,
// preserving everything the caller leaves unset.
//
// Every field is a presence-bearing pointer, which is a deliberate departure
// from UpdateDocumentRequest and UpdateTodolistRequest — those detect "set" by
// zero value, so "" reads as unaddressed and a clear is unreachable through the
// composite. That compromise is not available here:
//
//   - The three carve-outs are addressed BY their zero values. participant_ids
//     [] removes everyone, url "" drops the join link and highlighted false
//     stops highlighting. A zero-value guard would silently drop all three,
//     handing each clear back to BC3's preserve-on-omission.
//   - description "" is a real clear, and all_day false really does convert an
//     all-day entry into a timed one. Both are legitimate, distinct writes; a
//     zero-value guard makes each unreachable.
//
// Once most of the struct must be presence-bearing, a mixed shape where Summary
// string means "unset when empty" but Description *string means "unset when
// nil" is a footgun. One rule for the whole type: nil is unaddressed, non-nil is
// sent.
//
// BREAKING (v0.13.0): Summary, StartsAt, EndsAt, Description and Notify changed
// from value types to pointers, and this request now drives a read-then-write
// composite rather than a single sparse PUT. The verbatim single PUT moved to
// ReplaceEntry/ReplaceScheduleEntryRequest.
type UpdateScheduleEntryRequest struct {
	// Summary is the event title. Nil leaves the fetched summary in place.
	Summary *string `json:"summary,omitempty"`
	// StartsAt is the event start. Nil leaves the fetched value in place.
	//
	// Sent as an opaque string, not parsed and re-rendered: BC3 renders a bare
	// date ("2026-06-01") for an all-day entry and a full timestamp otherwise,
	// and both must round-trip verbatim. See ScheduleEntryFields.StartsAt.
	StartsAt *string `json:"starts_at,omitempty"`
	// EndsAt is the event end. Nil leaves the fetched value in place. Same
	// date-or-timestamp rule as StartsAt.
	EndsAt *string `json:"ends_at,omitempty"`
	// Description is the event details in HTML. Nil leaves the fetched
	// description in place; a pointer to "" clears it.
	Description *string `json:"description,omitempty"`
	// AllDay reports whether the entry occupies whole days. Nil leaves the
	// fetched value in place; a pointer to false converts an all-day entry into
	// a timed one.
	AllDay *bool `json:"all_day,omitempty"`
	// ParticipantIDs replaces the entry's participants. Nil leaves them alone
	// (BC3 preserves them, and the composite never resends them); a pointer to
	// an empty slice removes everyone.
	ParticipantIDs *[]int64 `json:"participant_ids,omitempty"`
	// Notify makes BC3 recompute the entry's subscribers and notify them. Nil
	// omits the directive entirely.
	Notify *bool `json:"notify,omitempty"`
	// URL is the entry's join link — read back as ScheduleEntry.JoinURL, never
	// as ScheduleEntry.URL. Nil leaves it alone; a pointer to "" clears it.
	URL *string `json:"url,omitempty"`
	// Highlighted controls whether the entry is highlighted on the schedule.
	// Nil leaves it alone; a pointer to false removes the highlight.
	Highlighted *bool `json:"highlighted,omitempty"`
}

// ReplaceScheduleEntryRequest specifies a schedule entry's complete new
// representation for SchedulesService.ReplaceEntry.
//
// This is the verbatim request: one PUT, no read-before-write. Every writable
// field it omits is omitted from the body, and BC3 clears it — except the three
// it preserves on omission (participant_ids, url, highlighted), which is
// exactly why those must be presence-bearing here rather than defaulted. An
// absent url and an explicit "" are different requests: one preserves the join
// link, the other drops it.
//
// StartsAt and EndsAt are @required and are not on the preserve list, and
// schedule_entries.starts_at/ends_at are NOT NULL, so a body omitting either
// cannot succeed. ReplaceEntry refuses that locally rather than spending a
// round-trip to learn it.
type ReplaceScheduleEntryRequest struct {
	// Summary is the event title. Nil omits it and the server clears it; the
	// entry then reads back as "Untitled" (Schedule::Entry#summary falls back
	// when blank).
	Summary *string `json:"summary,omitempty"`
	// StartsAt is the event start (required). Send a bare date
	// ("2026-06-01") for an all-day entry and a timestamp otherwise; the value
	// is passed through opaquely, never parsed and re-rendered.
	StartsAt *string `json:"starts_at,omitempty"`
	// EndsAt is the event end (required). Same date-or-timestamp rule.
	EndsAt *string `json:"ends_at,omitempty"`
	// Description is the event details in HTML. Nil omits it and the server
	// clears it; a pointer to "" clears it explicitly.
	Description *string `json:"description,omitempty"`
	// AllDay reports whether the entry occupies whole days. Nil omits it, and
	// because the column is NOT NULL DEFAULT false the server RESETS it —
	// silently converting an all-day entry into a midnight-to-midnight timed
	// one. UpdateEntry and EditEntry resend it for exactly this reason.
	AllDay *bool `json:"all_day,omitempty"`
	// ParticipantIDs replaces the entry's participants. Nil omits the key and
	// BC3 preserves the current participants; a pointer to an empty slice
	// removes everyone.
	ParticipantIDs *[]int64 `json:"participant_ids,omitempty"`
	// Notify makes BC3 recompute the entry's subscribers and notify them.
	Notify *bool `json:"notify,omitempty"`
	// URL is the entry's join link. Nil omits the key and BC3 preserves the
	// current link; a pointer to "" clears it.
	URL *string `json:"url,omitempty"`
	// Highlighted controls the schedule highlight. Nil omits the key and BC3
	// preserves the current state; a pointer to false removes the highlight.
	//
	// An explicit JSON null would be worse than omission here and the SDK never
	// sends one: the column rejects NULL, so BC3 raises rather than defaulting.
	Highlighted *bool `json:"highlighted,omitempty"`
}

// body renders the verbatim request, sending exactly the members the caller set.
func (r *ReplaceScheduleEntryRequest) body() (map[string]any, error) {
	if r.StartsAt == nil || r.EndsAt == nil {
		return nil, ErrUsage("replace request must set starts_at and ends_at; the server rebuilds the entry from the body and neither column accepts a missing value")
	}

	body := map[string]any{
		"starts_at": *r.StartsAt,
		"ends_at":   *r.EndsAt,
	}
	if r.Summary != nil {
		body["summary"] = *r.Summary
	}
	if r.Description != nil {
		body["description"] = *r.Description
	}
	if r.AllDay != nil {
		body["all_day"] = *r.AllDay
	}
	if r.ParticipantIDs != nil {
		body["participant_ids"] = nonNilIDs(*r.ParticipantIDs)
	}
	if r.Notify != nil {
		body["notify"] = *r.Notify
	}
	if r.URL != nil {
		body["url"] = *r.URL
	}
	if r.Highlighted != nil {
		body["highlighted"] = *r.Highlighted
	}
	return body, nil
}

// ScheduleEntryFields is a schedule entry's full writable state, handed to the
// EditEntry callback. Everything in it is PUT back to the server, so clearing a
// full-state field means setting it empty ("") — there is no third state.
//
// The four addressed-only fields are behind setters instead, because on this
// endpoint the difference between "left alone" and "cleared" is not recoverable
// from the value. Assignment is what puts a carve-out on the wire:
// SetURL(f.URL()) is a WRITE, and it is sent even though nothing changed. A
// value comparison against the read-back would decide otherwise and hand the
// write back to BC3's preserve-on-omission — the conformance case
// edit-touched-carve-outs exists to fail exactly that implementation. The
// getters return the read-back values so a caller can inspect before deciding;
// only the setter marks dirty.
type ScheduleEntryFields struct {
	// Summary is the event title. "" reads back as "Untitled".
	Summary string
	// StartsAt is the event start, verbatim off the wire.
	//
	// A string, not a time. BC3 renders starts_at_date_or_time, which is
	// starts_at.to_date unless the entry is timed, so an all-day entry reads
	// back as "2026-06-01" and a timed one as a full timestamp. Round-tripping
	// through a time type would rewrite the all-day form into a midnight
	// timestamp and shift the entry's bounds by the account's UTC offset, so
	// the composites carry the raw response string through untouched.
	StartsAt string
	// EndsAt is the event end, verbatim off the wire. Same rule as StartsAt.
	EndsAt string
	// Description is the event details in HTML. "" clears it.
	Description string
	// AllDay reports whether the entry occupies whole days. Always resent,
	// because omitting it resets the NOT NULL DEFAULT false column.
	AllDay bool

	url               string
	urlSet            bool
	highlighted       bool
	highlightedSet    bool
	participantIDs    []int64
	participantIDsSet bool
	notify            bool
	notifySet         bool
}

// URL returns the entry's join link as the GET reported it (join_url on the
// wire). Reading it does not put it on the wire; SetURL does.
func (f *ScheduleEntryFields) URL() string { return f.url }

// SetURL sets the entry's join link and marks it for the wire. "" clears the
// link. Assigning the value URL already returned still sends it.
func (f *ScheduleEntryFields) SetURL(url string) {
	f.url, f.urlSet = url, true
}

// Highlighted returns the entry's highlight state as the GET reported it.
// Reading it does not put it on the wire; SetHighlighted does.
func (f *ScheduleEntryFields) Highlighted() bool { return f.highlighted }

// SetHighlighted sets the entry's highlight state and marks it for the wire.
// false removes the highlight. Assigning the value Highlighted already returned
// still sends it.
func (f *ScheduleEntryFields) SetHighlighted(highlighted bool) {
	f.highlighted, f.highlightedSet = highlighted, true
}

// ParticipantIDs returns the participant IDs the GET reported, as a copy.
// Mutating the returned slice changes nothing; SetParticipantIDs does.
func (f *ScheduleEntryFields) ParticipantIDs() []int64 { return slices.Clone(f.participantIDs) }

// SetParticipantIDs replaces the entry's participants and marks them for the
// wire. An empty (or nil) slice removes everyone — it is sent as [], not
// omitted and not null.
func (f *ScheduleEntryFields) SetParticipantIDs(ids []int64) {
	f.participantIDs, f.participantIDsSet = slices.Clone(ids), true
}

// Notify reports whether this edit will ask BC3 to notify participants. It is a
// directive rather than state, so it has no read-back value and starts false.
func (f *ScheduleEntryFields) Notify() bool { return f.notify }

// SetNotify marks the notify directive for the wire. Sending it makes BC3
// recompute a drafted entry's subscriber list.
func (f *ScheduleEntryFields) SetNotify(notify bool) {
	f.notify, f.notifySet = notify, true
}

// fullBody serializes the writable state for the replace transport: the five
// full-state fields always, empties included, plus whichever carve-outs the
// caller addressed.
func (f *ScheduleEntryFields) fullBody() (map[string]any, error) {
	if f.StartsAt == "" || f.EndsAt == "" {
		return nil, ErrUsage("schedule entry starts_at and ends_at are required; the server rebuilds the entry from the body and neither column accepts a missing value")
	}

	body := map[string]any{
		"summary":     f.Summary,
		"starts_at":   f.StartsAt,
		"ends_at":     f.EndsAt,
		"description": f.Description,
		"all_day":     f.AllDay,
	}
	if f.participantIDsSet {
		body["participant_ids"] = nonNilIDs(f.participantIDs)
	}
	if f.urlSet {
		body["url"] = f.url
	}
	if f.highlightedSet {
		body["highlighted"] = f.highlighted
	}
	if f.notifySet {
		body["notify"] = f.notify
	}
	return body, nil
}

// nonNilIDs makes an explicitly-empty participant list serialize as [] rather
// than null. A nil slice marshals to JSON null, and null is not the clear:
// participant_ids is addressed-only, so BC3 reads a null the same way it reads
// an omission for every other purpose, while [] is the documented "remove
// everyone".
func nonNilIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

// scheduleEntryDecodeError renders a response-decoder failure in the SPEC §6
// shape, the way documentDecodeError does for documents.
//
// Go's json.Unmarshal is the typed guard the dynamic SDKs write by hand, and it
// rejects a wrong-typed field before a composite ever sees it — but it reports
// that as a raw decoder error, which callers switching on *Error would miss and
// which carries no hint.
//
// There is no classification here, deliberately: decoder errors are not
// enumerable (created_at is time.Time, whose UnmarshalJSON returns
// *time.ParseError; description_attachments carries *types.FlexInt dimensions
// rejected with a plain fmt.Errorf that is no named type at all), and neither
// are the errors that precede a response. So getEntryWithBody splits the
// request from the decode and calls this on the decode step only, where the
// origin is known by construction rather than guessed.
func scheduleEntryDecodeError(err error) error {
	return &Error{
		Code:    CodeAPI,
		Message: truncate(fmt.Sprintf("GetScheduleEntry returned a body that does not decode as a schedule entry: %v", err)),
		Hint: "The merge-safe UpdateEntry/EditEntry resend this record's fields verbatim, so a malformed " +
			"response cannot be written back safely. Use ReplaceEntry to write the record deliberately.",
		Retryable: false,
	}
}

// scheduleEntryMalformed is the statusless api_error for a 2xx read the
// composites cannot safely write back. Statusless by SPEC §6: the transport
// succeeded, so no HTTP status describes this, and re-requesting cannot repair a
// malformed body.
func scheduleEntryMalformed(what string) error {
	return &Error{
		Code:    CodeAPI,
		Message: truncate(fmt.Sprintf("GetScheduleEntry returned an entry %s", what)),
		Hint: "The merge-safe UpdateEntry/EditEntry resend this field verbatim, so a missing value " +
			"would overwrite the current one. Use ReplaceEntry to write the record deliberately.",
		Retryable: false,
	}
}

// scheduleEntryFieldsFrom derives an entry's full writable state from a GET.
//
// body is the raw payload the entry decoded from, and it is a parameter rather
// than an afterthought because two of the guards below are unimplementable from
// the struct alone. generated.ScheduleEntry declares all_day as a plain bool and
// starts_at/ends_at as types.FlexibleTime, all three now @required, so an absent
// key decodes to the zero value and is indistinguishable at that layer from a
// real false or a real timestamp — Swift's Codable and kotlinx.serialization
// reject a missing required member during decoding, and encoding/json does not.
// The distinction survives only in the response bytes, which the GET already has
// in hand (GetScheduleEntryResponse.Body) and threads through here. This second
// decode is not a redundant one: it reads presence, which the first decode
// discarded — the same shape todolists.go's requireDescription uses.
//
// What each guard protects:
//
//   - summary: @required and never blank (Schedule::Entry#summary is
//     super.presence || "Untitled"), so blank on a 2xx read is malformed, not an
//     empty summary — and writing it back would blank the real one.
//   - all_day: NOT NULL DEFAULT false. Defaulting a missing value to false would
//     convert an all-day event into a midnight-to-midnight timed one.
//   - starts_at, ends_at: NOT NULL, emitted by every partial. The raw string is
//     taken verbatim rather than re-rendered from the decoded time, so an
//     all-day entry's bare date survives the round trip.
//   - description: optional and nullable, so absent or null is genuinely empty.
//     A wrong-TYPED description never reaches here — the decoder rejected it and
//     getEntryWithBody turned that into scheduleEntryDecodeError.
func scheduleEntryFieldsFrom(e *ScheduleEntry, body []byte) (*ScheduleEntryFields, error) {
	if strings.TrimSpace(e.Summary) == "" {
		return nil, scheduleEntryMalformed(`with no "summary", but the field is required`)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, scheduleEntryMalformed("in a body that is not a JSON object")
	}
	if err := requireEntryKey(raw, "all_day"); err != nil {
		return nil, err
	}
	startsAt, err := requiredEntryString(raw, "starts_at")
	if err != nil {
		return nil, err
	}
	endsAt, err := requiredEntryString(raw, "ends_at")
	if err != nil {
		return nil, err
	}

	fields := &ScheduleEntryFields{
		Summary:     e.Summary,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		Description: e.Description,
		AllDay:      e.AllDay,
	}
	// Seeded for the getters only. None of these is marked set, so none reaches
	// the wire unless the callback assigns it.
	fields.url = e.JoinURL
	fields.highlighted = e.Highlighted
	fields.participantIDs = make([]int64, 0, len(e.Participants))
	for _, p := range e.Participants {
		fields.participantIDs = append(fields.participantIDs, p.ID)
	}
	return fields, nil
}

// requireEntryKey refuses a response whose @required key is absent or null.
func requireEntryKey(raw map[string]json.RawMessage, key string) error {
	value, ok := raw[key]
	if !ok {
		return scheduleEntryMalformed(fmt.Sprintf("with no %q, but the field is required", key))
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return scheduleEntryMalformed(fmt.Sprintf("with a null %q, but the field is required and never null", key))
	}
	return nil
}

// requiredEntryString is requireEntryKey plus the verbatim string value, for the
// two timestamps the composites round-trip as opaque text.
func requiredEntryString(raw map[string]json.RawMessage, key string) (string, error) {
	if err := requireEntryKey(raw, key); err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal(raw[key], &value); err != nil {
		return "", scheduleEntryMalformed(fmt.Sprintf("whose %q is not a string", key))
	}
	if value == "" {
		return "", scheduleEntryMalformed(fmt.Sprintf("with an empty %q, but the field is required", key))
	}
	return value, nil
}

// UpdateScheduleSettingsRequest specifies the parameters for updating schedule settings.
type UpdateScheduleSettingsRequest struct {
	// IncludeDueAssignments controls whether to-do due dates appear on the schedule.
	IncludeDueAssignments bool `json:"include_due_assignments"`
}

// ScheduleEntryListResult contains the results from listing schedule entries.
type ScheduleEntryListResult struct {
	// Entries is the list of schedule entries returned.
	Entries []ScheduleEntry
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SchedulesService handles schedule operations.
type SchedulesService struct {
	client *AccountClient
}

// NewSchedulesService creates a new SchedulesService.
func NewSchedulesService(client *AccountClient) *SchedulesService {
	return &SchedulesService{client: client}
}

// Get returns a schedule by ID.
func (s *SchedulesService) Get(ctx context.Context, scheduleID int64) (result *Schedule, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "Get",
		ResourceType: "schedule", IsMutation: false,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetScheduleWithResponse(ctx, s.client.accountID, scheduleID)
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

	schedule := scheduleFromGenerated(*resp.JSON200)
	return &schedule, nil
}

// ListEntries returns all entries on a schedule.
//
// By default, returns all entries (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of entries to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned ScheduleEntryListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *SchedulesService) ListEntries(ctx context.Context, scheduleID int64, opts *ScheduleEntryListOptions) (result *ScheduleEntryListResult, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "ListEntries",
		ResourceType: "schedule_entry", IsMutation: false,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Build params for generated client
	var params *generated.ListScheduleEntriesParams
	if opts != nil && (opts.Status != "" || opts.Page > 0) {
		params = &generated.ListScheduleEntriesParams{
			// omitzero, not &opts.Status: the guard above now also fires for a
			// page-only call, and a non-nil pointer to "" is sent as an empty
			// status= rather than omitted — which would displace the server's
			// active-entries default. Same shape as every other wrapper here.
			Status: omitzero(opts.Status),
		}
		if opts.Page > 0 {
			var page *int32
			if page, err = pageParam(opts.Page); err != nil {
				return nil, err
			}
			params.Page = page
		}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListScheduleEntriesWithResponse(ctx, s.client.accountID, scheduleID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var entries []ScheduleEntry
	if resp.JSON200 != nil {
		for _, ge := range *resp.JSON200 {
			entries = append(entries, scheduleEntryFromGenerated(ge))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(entries), opts.Limit, resp.HTTPResponse)
		return &ScheduleEntryListResult{Entries: entries[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for entries), >0 = specific limit
	limit := 0 // default to all for entries
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(entries) >= limit {
		return &ScheduleEntryListResult{Entries: entries[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(entries), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(entries), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var ge generated.ScheduleEntry
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
		}
		entries = append(entries, scheduleEntryFromGenerated(ge))
	}

	return &ScheduleEntryListResult{Entries: entries, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetEntry returns a schedule entry by ID.
//
// A recurring entry answers with a 302 rather than an entry:
// ensure_non_recurring_event redirects both show and update, and this route
// therefore serves non-recurring entries only.
func (s *SchedulesService) GetEntry(ctx context.Context, entryID int64) (*ScheduleEntry, error) {
	entry, _, err := s.getEntryWithBody(ctx, entryID)
	return entry, err
}

// getEntryWithBody is GetEntry, plus the raw response payload the entry decoded
// from.
//
// UpdateEntry and EditEntry need both: the decoded struct for the values and the
// bytes for what the struct cannot express. all_day, starts_at and ends_at are
// @required on the response and non-pointer on the generated model, so an absent
// key decodes to the zero value and no longer distinguishable from a real one —
// see scheduleEntryFieldsFrom. GetEntry itself drops the bytes, keeping the
// public read surface unchanged; nothing else about the request differs, so
// hooks observe one Schedules.GetEntry either way.
//
// The request and the decode are split rather than calling
// GetScheduleEntryWithResponse, so the two error origins never mix. The
// merge-safe composites read this body and write every field of it back, so a
// malformed one has to arrive as the documented statusless api_error
// (scheduleEntryDecodeError) — but everything BEFORE the response is a different
// failure with its own meaning, and no inspection of the returned error can
// reliably tell them apart. GetScheduleEntry covers the gate's successors: the
// per-request auth editor (a token provider or custom AuthStrategy may return
// ANY error), the transport, and context cancellation. Those return verbatim, so
// errors.Is keeps working; only ParseGetScheduleEntryResponse's failure is a
// decode failure. Same split as DocumentsService.Get.
func (s *SchedulesService) getEntryWithBody(ctx context.Context, entryID int64) (result *ScheduleEntry, body []byte, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "GetEntry",
		ResourceType: "schedule_entry", IsMutation: false,
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

	//nolint:bodyclose // ParseGetScheduleEntryResponse below closes the body (it
	// defers rsp.Body.Close()), and it is called unconditionally on the next line.
	httpResp, err := s.client.parent.gen.GetScheduleEntry(ctx, s.client.accountID, entryID)
	if err != nil {
		return nil, nil, err
	}
	resp, decodeErr := generated.ParseGetScheduleEntryResponse(httpResp)
	if decodeErr != nil {
		err = scheduleEntryDecodeError(decodeErr)
		return nil, nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil {
		err = scheduleEntryDecodeError(fmt.Errorf("the response carried no schedule entry object"))
		return nil, nil, err
	}

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, resp.Body, nil
}

// CreateEntry creates a new entry on a schedule.
// Returns the created schedule entry.
func (s *SchedulesService) CreateEntry(ctx context.Context, scheduleID int64, req *CreateScheduleEntryRequest) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "CreateEntry",
		ResourceType: "schedule_entry", IsMutation: true,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Summary == "" {
		err = ErrUsage("schedule entry summary is required")
		return nil, err
	}
	if req.StartsAt == "" {
		err = ErrUsage("schedule entry starts_at is required")
		return nil, err
	}
	if req.EndsAt == "" {
		err = ErrUsage("schedule entry ends_at is required")
		return nil, err
	}

	startsAt, parseErr := time.Parse(time.RFC3339, req.StartsAt)
	if parseErr != nil {
		err = ErrUsage("schedule entry starts_at must be in RFC3339 format (e.g., 2024-01-15T09:00:00Z)")
		return nil, err
	}
	endsAt, parseErr := time.Parse(time.RFC3339, req.EndsAt)
	if parseErr != nil {
		err = ErrUsage("schedule entry ends_at must be in RFC3339 format (e.g., 2024-01-15T17:00:00Z)")
		return nil, err
	}

	body := generated.CreateScheduleEntryJSONRequestBody{
		Summary:     req.Summary,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		Description: omitzero(req.Description),
		AllDay:      req.AllDay,
		// The join link is `url` on the way in and reads back as `join_url`.
		// Threading the response's JoinURL into this field is the correct
		// round-trip; threading its URL would write the entry's API URL into the
		// join link.
		Url:              req.URL,
		Highlighted:      req.Highlighted,
		Status:           req.Status,
		Subscriptions:    req.Subscriptions,
		VisibleToClients: req.VisibleToClients,
	}
	// nil means "not addressed" (omitted); a non-nil empty slice is an explicit
	// empty participant list and must reach the wire — matching UpdateEntry.
	if req.ParticipantIDs != nil {
		body.ParticipantIds = &req.ParticipantIDs
	}
	if req.Notify {
		body.Notify = &req.Notify
	}

	resp, err := s.client.parent.gen.CreateScheduleEntryWithResponse(ctx, s.client.accountID, scheduleID, body)
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

	entry := scheduleEntryFromGenerated(*resp.JSON201)
	return &entry, nil
}

// UpdateEntry sets the given fields on a schedule entry and preserves
// everything else: GETs the current entry, overlays the addressed request
// fields, and PUTs the full representation back.
//
// A nil field is untouched, guaranteed — and unlike DocumentsService.Update, a
// non-nil empty one is a CLEAR that reaches the wire. The five full-state fields
// (summary, starts_at, ends_at, description, all_day) are always sent, seeded
// from the read; the four addressed-only ones (participant_ids, url,
// highlighted, notify) are sent only when the caller set them and are never
// seeded from the read.
//
// Composes the public GetEntry and ReplaceEntry paths, so hooks observe both
// wire operations (Schedules.GetEntry then Schedules.ReplaceEntry) rather than a
// synthetic composite.
//
// Not atomic: there is no conditional-update signal on this endpoint, so a
// concurrent write between the GET and PUT is overwritten — last write wins for
// the whole representation, with a window of one round-trip. Use ReplaceEntry to
// overwrite deliberately.
//
// Non-recurring entries only. ensure_non_recurring_event 302-redirects both show
// and update for a recurring entry, so one surfaces here as an unexpected body
// on the GET rather than as an update.
func (s *SchedulesService) UpdateEntry(ctx context.Context, entryID int64, req *UpdateScheduleEntryRequest) (*ScheduleEntry, error) {
	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	current, body, err := s.getEntryWithBody(ctx, entryID)
	if err != nil {
		return nil, err
	}

	fields, err := scheduleEntryFieldsFrom(current, body)
	if err != nil {
		return nil, err
	}
	if req.Summary != nil {
		fields.Summary = *req.Summary
	}
	if req.StartsAt != nil {
		fields.StartsAt = *req.StartsAt
	}
	if req.EndsAt != nil {
		fields.EndsAt = *req.EndsAt
	}
	if req.Description != nil {
		fields.Description = *req.Description
	}
	if req.AllDay != nil {
		fields.AllDay = *req.AllDay
	}
	if req.ParticipantIDs != nil {
		fields.SetParticipantIDs(*req.ParticipantIDs)
	}
	if req.URL != nil {
		fields.SetURL(*req.URL)
	}
	if req.Highlighted != nil {
		fields.SetHighlighted(*req.Highlighted)
	}
	if req.Notify != nil {
		fields.SetNotify(*req.Notify)
	}

	return s.replaceScheduleEntry(ctx, entryID, fields.fullBody)
}

// EditEntry applies a read-modify-write callback to a schedule entry: GETs the
// current entry, hands the callback its full writable state, and PUTs the whole
// thing back. If the callback returns an error, the edit aborts and nothing is
// written.
//
// The five full-state fields are plain struct members: an untouched one keeps
// its fetched value and is resent either way, and clearing one means setting it
// empty (""). The four addressed-only fields are behind setters, and only an
// actual assignment puts one on the wire — SetURL(f.URL()) is a write, and it is
// sent. See ScheduleEntryFields.
//
// Not atomic — see UpdateEntry for the GET→PUT race, and for the recurring-entry
// redirect.
func (s *SchedulesService) EditEntry(ctx context.Context, entryID int64, fn func(*ScheduleEntryFields) error) (*ScheduleEntry, error) {
	if fn == nil {
		return nil, ErrUsage("edit callback is required")
	}

	current, body, err := s.getEntryWithBody(ctx, entryID)
	if err != nil {
		return nil, err
	}

	fields, err := scheduleEntryFieldsFrom(current, body)
	if err != nil {
		return nil, err
	}
	if err := fn(fields); err != nil {
		return nil, err
	}

	return s.replaceScheduleEntry(ctx, entryID, fields.fullBody)
}

// ReplaceEntry overwrites a schedule entry with the given representation
// verbatim: one PUT, no read-before-write. Every writable field the request
// omits is omitted from the body, and the server clears it — except
// participant_ids, url and highlighted, which BC3 preserves on omission.
//
// Sharp by construction. Use UpdateEntry or EditEntry to preserve the fields the
// call does not name.
//
// Renamed from UpdateEntry in v0.13.0, following the ReplaceScheduleEntry wire
// rename, and with no deprecated alias — the same break ReplaceTodo and
// ReplaceDocument took. The name that says "this clears what it omits" is the
// one the destructive method should carry.
func (s *SchedulesService) ReplaceEntry(ctx context.Context, entryID int64, req *ReplaceScheduleEntryRequest) (*ScheduleEntry, error) {
	return s.replaceScheduleEntry(ctx, entryID, func() (map[string]any, error) {
		if req == nil {
			return nil, ErrUsage("replace request is required")
		}
		return req.body()
	})
}

// replaceScheduleEntry is the single transport behind UpdateEntry, EditEntry and
// ReplaceEntry. It owns the hook envelope and the one *WithBody call site.
//
// The body is a hand-marshaled map rather than the generated request struct:
// generated.ReplaceScheduleEntryJSONRequestBody uses omitempty on the optional
// members, which cannot express the always-send-empty semantics a full-replace
// composite needs (an empty description is a clear, and it has to reach the
// wire), and it types starts_at/ends_at as time.Time, which cannot express the
// bare date BC3 renders for an all-day entry at all. SPEC §18 rule 1 sanctions
// exactly this carve-out — the generated wrapper still owns path, verb, content
// type and response decoding, and the operation identity still reaches hooks and
// retry.
func (s *SchedulesService) replaceScheduleEntry(ctx context.Context, entryID int64, buildBody func() (map[string]any, error)) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "ReplaceEntry",
		ResourceType: "schedule_entry", IsMutation: true,
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

	// Built inside the envelope so a usage error is observable to hooks.
	body, err := buildBody()
	if err != nil {
		return nil, err
	}
	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.parent.gen.ReplaceScheduleEntryWithBodyWithResponse(
		ctx, s.client.accountID, entryID, "application/json", bodyReader)
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

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// GetEntryOccurrence returns a specific occurrence of a recurring schedule entry.
func (s *SchedulesService) GetEntryOccurrence(ctx context.Context, entryID int64, date string) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "GetEntryOccurrence",
		ResourceType: "schedule_entry", IsMutation: false,
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

	if date == "" {
		err = ErrUsage("occurrence date is required")
		return nil, err
	}

	resp, err := s.client.parent.gen.GetScheduleEntryOccurrenceWithResponse(ctx, s.client.accountID, entryID, date)
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

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// UpdateSettings updates the settings for a schedule.
// Returns the updated schedule.
func (s *SchedulesService) UpdateSettings(ctx context.Context, scheduleID int64, req *UpdateScheduleSettingsRequest) (result *Schedule, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "UpdateSettings",
		ResourceType: "schedule", IsMutation: true,
		ResourceID: scheduleID,
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
		err = ErrUsage("update settings request is required")
		return nil, err
	}

	body := generated.UpdateScheduleSettingsJSONRequestBody{
		IncludeDueAssignments: req.IncludeDueAssignments,
	}

	resp, err := s.client.parent.gen.UpdateScheduleSettingsWithResponse(ctx, s.client.accountID, scheduleID, body)
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

	schedule := scheduleFromGenerated(*resp.JSON200)
	return &schedule, nil
}

// TrashEntry moves a schedule entry to the trash.
// Trashed entries can be recovered from the trash.
func (s *SchedulesService) TrashEntry(ctx context.Context, entryID int64) (err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "TrashEntry",
		ResourceType: "schedule_entry", IsMutation: true,
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
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Note: Permanent deletion of schedule entries is not supported by the Basecamp API.
// Use TrashEntry() to move entries to trash (recoverable via the web UI).

// scheduleFromGenerated converts a generated Schedule to our clean type.
func scheduleFromGenerated(gs generated.Schedule) Schedule {
	s := Schedule{
		Status:                gs.Status,
		VisibleToClients:      gs.VisibleToClients,
		CreatedAt:             gs.CreatedAt,
		UpdatedAt:             gs.UpdatedAt,
		Title:                 gs.Title,
		InheritsStatus:        gs.InheritsStatus,
		Type:                  gs.Type,
		URL:                   gs.Url,
		AppURL:                gs.AppUrl,
		BookmarkURL:           deref(gs.BookmarkUrl),
		Position:              int(deref(gs.Position)),
		IncludeDueAssignments: deref(gs.IncludeDueAssignments),
		EntriesCount:          int(deref(gs.EntriesCount)),
		EntriesURL:            deref(gs.EntriesUrl),
	}

	if gs.Id != 0 {
		s.ID = gs.Id
	}

	if gs.Bucket.Id != 0 || gs.Bucket.Name != "" {
		s.Bucket = &Bucket{
			ID:   gs.Bucket.Id,
			Name: gs.Bucket.Name,
			Type: gs.Bucket.Type,
		}
	}

	if gs.Creator.Id != 0 || gs.Creator.Name != "" {
		creator := personFromGenerated(gs.Creator)
		s.Creator = &creator
	}

	return s
}

// scheduleEntryFromGenerated converts a generated ScheduleEntry to our clean type.
func scheduleEntryFromGenerated(ge generated.ScheduleEntry) ScheduleEntry {
	e := ScheduleEntry{
		Status:           ge.Status,
		VisibleToClients: ge.VisibleToClients,
		CreatedAt:        ge.CreatedAt,
		UpdatedAt:        ge.UpdatedAt,
		Title:            ge.Title,
		Summary:          ge.Summary,
		InheritsStatus:   ge.InheritsStatus,
		Type:             ge.Type,
		URL:              ge.Url,
		AppURL:           ge.AppUrl,
		BookmarkURL:      deref(ge.BookmarkUrl),
		BoostsCount:      int(deref(ge.BoostsCount)),
		BoostsURL:        deref(ge.BoostsUrl),
		SubscriptionURL:  deref(ge.SubscriptionUrl),
		CommentsURL:      deref(ge.CommentsUrl),
		CommentsCount:    int(deref(ge.CommentsCount)),
		// @required since bc3#12502, so the generated model carries these three
		// as values rather than pointers — no deref, and a missing key lands as
		// the zero value. The composites refuse that shape rather than write it
		// back; see scheduleEntryFieldsFrom.
		StartsAt:    ge.StartsAt,
		EndsAt:      ge.EndsAt,
		AllDay:      ge.AllDay,
		Description: deref(ge.Description),
		// Optional because GetUpcomingSchedule renders a reduced partial that
		// omits both. Never read by the composites — they are addressed-only.
		JoinURL:     deref(ge.JoinUrl),
		Highlighted: deref(ge.Highlighted),
	}

	if ge.Id != 0 {
		e.ID = ge.Id
	}

	if ge.Parent.Id != 0 || ge.Parent.Title != "" {
		e.Parent = &Parent{
			ID:     ge.Parent.Id,
			Title:  ge.Parent.Title,
			Type:   ge.Parent.Type,
			URL:    ge.Parent.Url,
			AppURL: ge.Parent.AppUrl,
		}
	}

	if ge.Bucket.Id != 0 || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   ge.Bucket.Id,
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	if ge.Creator.Id != 0 || ge.Creator.Name != "" {
		creator := personFromGenerated(ge.Creator)
		e.Creator = &creator
	}

	// Convert participants
	if len(ge.Participants) > 0 {
		e.Participants = make([]Person, 0, len(ge.Participants))
		for _, gp := range ge.Participants {
			e.Participants = append(e.Participants, personFromGenerated(gp))
		}
	}

	e.DescriptionAttachments = richTextAttachmentsFromGenerated(ge.DescriptionAttachments)

	return e
}
