//! Recordings: the generated wire methods plus
//! [`RecordingsService::summarize`](RecordingsService::summarize), the SPEC §18 composite
//! that resolves an event pointer into a compact projection of the recording it names.
//!
//! [`RecordingSummary`] exists for consumers that must decide something about a recording
//! without paying for its full payload: an agent connector's admission step, an MCP tool
//! answering "what is this?".
//!
//! The SDK has no untyped recording read (BC3 has no such route), so the type is the
//! routing key: `comment.created` reads a comment, `card.created` reads a card, and so on —
//! one typed read per type. Chat lines are the exception, because their read needs the
//! Campfire id and the pointer does not carry it; `summarize` discovers the Campfire first
//! (see [`campfire_index`](crate::services::campfire_index)).
//!
//! This is hand-written composition over the generated services. It makes no wire request
//! of its own, and it mints no operation identity: hooks see the constituent reads under
//! their own names (SPEC §18 rule 3).

pub use crate::generated::services::recordings::{ListRecordingsParams, RecordingsService};

use std::fmt;

use serde::{Deserialize, Serialize};

use crate::client::AccountClient;
use crate::error::{Error, ErrorCode};
use crate::generated::types::{
    CampfireLine, Person, RecordingBucket, RecordingParent, TodoBucket, TodoParent,
};
use crate::mentions::mentioned_person_ids;
use crate::services::campfire_index::{MAX_CAMPFIRE_CANDIDATES, SourceRead, is_listing_overflow};
use crate::types::DateTime;

/// A pointer at one recording, the way an account event feed row carries it.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RecordingRef {
    /// The project the recording lives in. Required: it scopes the Campfire discovery for
    /// chat lines, and the read is checked against it so a pointer from one project can
    /// never resolve to a recording in another.
    pub bucket_id: i64,

    /// The recording's id.
    pub recording_id: i64,

    /// The account event feed type that named the recording — `comment.created`,
    /// `card.assignment_changed`, `chat.line.created`. The segment before the action names
    /// the recording type. Used when `recording_type` is absent.
    pub event_type: Option<String>,

    /// The recording's own type as BC3 spells it — `Comment`, `Kanban::Card`,
    /// `Chat::Lines::Text`. When set it takes precedence over `event_type`, being the more
    /// exact of the two.
    pub recording_type: Option<String>,
}

impl RecordingRef {
    /// A pointer from an event feed type — `comment.created` and friends.
    pub fn from_event(bucket_id: i64, recording_id: i64, event_type: impl Into<String>) -> Self {
        RecordingRef {
            bucket_id,
            recording_id,
            event_type: Some(event_type.into()),
            recording_type: None,
        }
    }

    /// A pointer from a BC3 recording type — `Comment`, `Kanban::Card` and friends.
    pub fn from_recording_type(
        bucket_id: i64,
        recording_id: i64,
        recording_type: impl Into<String>,
    ) -> Self {
        RecordingRef {
            bucket_id,
            recording_id,
            event_type: None,
            recording_type: Some(recording_type.into()),
        }
    }

    /// The routing key the pointer is read by: the recording type when it carries one, the
    /// event type otherwise.
    fn key(&self) -> &str {
        let recording_type = self.recording_type.as_deref().unwrap_or_default().trim();
        if recording_type.is_empty() {
            self.event_type.as_deref().unwrap_or_default().trim()
        } else {
            recording_type
        }
    }
}

/// The projection [`RecordingsService::summarize`] returns. Fields a type does not have are
/// empty: a comment has no assignees, a vault no content.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[non_exhaustive]
pub struct RecordingSummary {
    /// The recording's id.
    pub id: i64,
    /// `active`, `archived`, `trashed`.
    pub status: String,
    /// The recording type as BC3 spells it (`Comment`, `Kanban::Card`).
    pub r#type: String,
    /// The recording's title.
    pub title: String,
    /// Where a person reads it in Basecamp.
    pub app_url: String,
    /// The recording this one hangs off — the commented recording for a comment, the
    /// Campfire for a chat line, the column for a card.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parent: Option<RecordingParent>,
    /// The project the recording lives in.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bucket: Option<RecordingBucket>,
    /// Who created it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub creator: Option<Person>,
    /// Set for the assignable types (to-dos, cards, card steps).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub assignees: Vec<Person>,
    /// The people [`content`](RecordingSummary::content) mentions, per
    /// [`mentioned_person_ids`]. Always serialized, so a JSON consumer reads `[]` rather
    /// than a missing key.
    pub mentioned_person_ids: Vec<i64>,
    /// The recording's rich text, in full: the comment body, the message body, a to-do's
    /// description, a card's content, the chat line.
    pub content: String,
    /// When it last changed.
    pub updated_at: DateTime,
    /// The Campfire a chat line was found under — the reply destination for a chat trigger.
    /// Absent for every other type.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub campfire_id: Option<i64>,
}

/// A chat line that was found under none of the Campfires the caller can currently see.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct UnresolvedRecording {
    /// The bucket the pointer named.
    pub bucket_id: i64,
    /// The line the pointer named.
    pub recording_id: i64,
    /// The candidates tried, in order; empty when the bucket has no visible Campfire at all.
    pub campfire_ids: Vec<i64>,
    /// Whether the cached discovery sources were re-read before concluding. `false` when
    /// every source had been read within the last
    /// [`CAMPFIRE_INDEX_MIN_REFRESH`](crate::services::campfire_index::CAMPFIRE_INDEX_MIN_REFRESH),
    /// so a Campfire created in that window was not seen: the conclusion stands on data up
    /// to that old, and a retry after the floor sees the current sources.
    pub refreshed: bool,
    /// Candidates from the cache that the refreshed sources no longer list — Campfires the
    /// caller could see when the cache filled and cannot now. Set only when `refreshed`.
    pub stale_campfire_ids: Vec<i64>,
}

/// Why [`RecordingsService::summarize`] could not answer, when the reason is the
/// composite's own rather than a read's.
///
/// It is chained as the cause of the [`Error`] the call returns, so a consumer matches on
/// the identity rather than parsing a message: `RecordingSummaryError::of(&error)`. Every
/// other failure — a 403 on a candidate, a 404 on the typed read, a transport error — is
/// returned as that read's own [`Error`], with no cause of this kind.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum RecordingSummaryError {
    /// The event type names no recording type — `boost.created`, whose recording is the
    /// boost's target and whose type the feed row does not carry. A consumer resolves those
    /// from its own record of what it posted, not through `summarize`.
    NoRecordingType {
        /// The routing key that was refused.
        key: String,
    },

    /// Neither the event type nor the recording type names a type in the routing table.
    UnknownRecordingType {
        /// The routing key that was refused.
        key: String,
    },

    /// A chat line was found under none of the Campfires the caller can currently see.
    ///
    /// It is distinct from a failed read (any non-404 answer is returned as itself) and
    /// from discovery that could not finish ([`Self::CampfireDiscoveryIncomplete`]): every
    /// candidate answered 404. It is *not* distinct from lost visibility — BC3 answers 404
    /// for a Campfire the caller may not see, too — so a consumer marks the record blocked
    /// and retries on its own schedule; see
    /// [`UnresolvedRecording::stale_campfire_ids`].
    Unresolved(UnresolvedRecording),

    /// Discovery could not be carried to a conclusion — the Campfire listing overflowed
    /// [`MAX_CAMPFIRE_LISTING`](crate::services::campfire_index::MAX_CAMPFIRE_LISTING), or
    /// a bucket has more visible Campfires than
    /// [`MAX_CAMPFIRE_CANDIDATES`]. Distinct from [`Self::Unresolved`]: candidates were
    /// left unsearched, so nothing can be reported absent.
    CampfireDiscoveryIncomplete {
        /// The bucket the pointer named.
        bucket_id: i64,
        /// The line the pointer named.
        recording_id: i64,
        /// Why discovery stopped short.
        reason: String,
    },

    /// The recording the read returned lives in a different bucket from the one the pointer
    /// named.
    BucketMismatch {
        /// The bucket the pointer named.
        bucket_id: i64,
        /// The recording the pointer named.
        recording_id: i64,
        /// The bucket the read returned.
        found_in: i64,
    },
}

impl fmt::Display for RecordingSummaryError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            RecordingSummaryError::NoRecordingType { key } => {
                write!(f, "event type names no recording type: {key:?}")
            }
            RecordingSummaryError::UnknownRecordingType { key } => {
                write!(f, "no typed read for recording type: {key:?}")
            }
            RecordingSummaryError::Unresolved(unresolved) => write!(
                f,
                "chat line found under no visible campfire: line {} in bucket {} (tried {} campfires)",
                unresolved.recording_id,
                unresolved.bucket_id,
                unresolved.campfire_ids.len()
            ),
            RecordingSummaryError::CampfireDiscoveryIncomplete {
                bucket_id,
                recording_id,
                reason,
            } => write!(
                f,
                "campfire discovery incomplete: line {recording_id} in bucket {bucket_id}: {reason}"
            ),
            RecordingSummaryError::BucketMismatch {
                bucket_id,
                recording_id,
                found_in,
            } => write!(
                f,
                "recording is not in the requested bucket: recording {recording_id} is in bucket {found_in}, not {bucket_id}"
            ),
        }
    }
}

impl std::error::Error for RecordingSummaryError {}

impl RecordingSummaryError {
    /// The composite's own reason for an [`Error`], when it has one. A read that failed on
    /// its own terms — a 403, a 404, a transport error — answers `None`, and is classified
    /// by [`Error::code`] like any other SDK failure.
    ///
    /// The whole cause chain is searched, not just its first link, so the verdict survives
    /// being re-wrapped on its way out — which is what Go's `errors.Is` gives the reference
    /// implementation, and what makes this a contract a consumer can rely on rather than one
    /// a single [`Error::with_source`] elsewhere would quietly break.
    pub fn of(error: &Error) -> Option<&RecordingSummaryError> {
        error.find_source::<RecordingSummaryError>()
    }

    /// The taxonomy member this reason is reported under.
    ///
    /// Routing refusals are `usage`: the pointer names no read, and nothing was sent.
    /// "Unresolved" and a bucket mismatch are `not_found`: the recording the pointer names
    /// is not where it was looked for.
    ///
    /// Incomplete discovery is `usage`, settled across every port on [card 40] after the
    /// merged ports shipped two different answers — this one said `api_error`, Kotlin said
    /// `usage`, and a caller got exit 7 from one SDK and exit 1 from another for the same
    /// condition. `usage` is the one coarse code no HTTP response can produce:
    /// [`ErrorCode::from_status`] yields `auth_required`, `forbidden`, `not_found`,
    /// `rate_limit`, `validation`, `limit_exceeded` and `api_error`, never this one, so a
    /// verdict the composite reached on its own can never be read back as a constituent
    /// read's own answer. It is explicitly not `not_found`, because nothing left unsearched
    /// may be reported absent. Retryability is a separate field and is unchanged: the
    /// [`Error`] built from this carries `retryable = false`, because both reasons are
    /// deterministic for the same account state and a retry loop would re-run the identical
    /// search forever.
    ///
    /// [card 40]: https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308122086
    fn code(&self) -> ErrorCode {
        match self {
            RecordingSummaryError::NoRecordingType { .. }
            | RecordingSummaryError::UnknownRecordingType { .. }
            | RecordingSummaryError::CampfireDiscoveryIncomplete { .. } => ErrorCode::Usage,
            RecordingSummaryError::Unresolved(_) | RecordingSummaryError::BucketMismatch { .. } => {
                ErrorCode::NotFound
            }
        }
    }
}

impl From<RecordingSummaryError> for Error {
    fn from(reason: RecordingSummaryError) -> Error {
        Error::new(reason.code(), reason.to_string()).with_source(reason)
    }
}

/// Which typed read serves a [`RecordingRef`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Kind {
    Comment,
    Message,
    Todo,
    Card,
    ChatLine,
    Document,
    Upload,
    ScheduleEntry,
    Question,
    QuestionAnswer,
    Todolist,
    Vault,
    Forward,
    ClientApproval,
    ClientCorrespondence,
    GoogleDocument,
    CloudFile,
    CardStep,
    Questionnaire,
    Schedule,
    Todoset,
    MessageBoard,
    CardTable,
    CardColumn,
    Inbox,
    Campfire,
}

/// The subject of an account event feed type — everything before its final `.` — and the
/// read it names. This is the feed's catalog (bc3 `Event::EventType`) minus `boost`, which
/// names no recording type and is refused explicitly rather than left to fall through as
/// unknown.
const EVENT_SUBJECTS: &[(&str, Kind)] = &[
    ("comment", Kind::Comment),
    ("message", Kind::Message),
    ("todo", Kind::Todo),
    ("card", Kind::Card),
    ("chat.line", Kind::ChatLine),
];

/// BC3's recording type strings and the read each names. It is the routing contract, and it
/// is a DELIBERATE set, not an exhaustive one: the recording types the account event feed's
/// trigger matrix names (comment, message, to-do, card, chat line), plus the content and
/// tool recordings a consumer reasoning about those is likely to hold an id for. Chat lines
/// are matched by prefix (`Chat::Lines::Text`, `::RichText`, `::Code`, `::Upload`,
/// `::Integration` all read through the same route); everything else exactly.
///
/// A type outside this set is [`RecordingSummaryError::UnknownRecordingType`] by design,
/// whether or not the SDK has an id-only read for it — `Timesheet::Entry` and
/// `Gauge::Needle` do, and are not routed; `Client::Reply` and `Forward::Reply` cannot be,
/// since their reads need a parent id the pointer does not carry. Widening the set is a
/// product decision, not a gap: add the type here, its projection in `read_summary`, a
/// routing row in this module's tests, and a case in
/// `conformance/tests/recording_summary.json`, the fixture every SDK implements.
const RECORDING_TYPES: &[(&str, Kind)] = &[
    ("Chat::Transcript", Kind::Campfire),
    ("Client::Approval", Kind::ClientApproval),
    ("Client::Correspondence", Kind::ClientCorrespondence),
    ("CloudFile", Kind::CloudFile),
    ("Comment", Kind::Comment),
    ("Document", Kind::Document),
    ("GoogleDocument", Kind::GoogleDocument),
    ("Inbox", Kind::Inbox),
    ("Inbox::Forward", Kind::Forward),
    ("Kanban::Board", Kind::CardTable),
    ("Kanban::Card", Kind::Card),
    ("Kanban::Column", Kind::CardColumn),
    ("Kanban::Step", Kind::CardStep),
    ("Message", Kind::Message),
    ("Message::Board", Kind::MessageBoard),
    ("Question", Kind::Question),
    ("Question::Answer", Kind::QuestionAnswer),
    ("Questionnaire", Kind::Questionnaire),
    ("Schedule", Kind::Schedule),
    ("Schedule::Entry", Kind::ScheduleEntry),
    ("Todo", Kind::Todo),
    ("Todolist", Kind::Todolist),
    ("Todoset", Kind::Todoset),
    ("Upload", Kind::Upload),
    ("Vault", Kind::Vault),
];

/// Every `Chat::Lines::*` subtype reads through the one Campfire-line route.
const CHAT_LINE_TYPE_PREFIX: &str = "Chat::Lines::";

/// The recording types [`RecordingsService::summarize`] routes by
/// [`RecordingRef::recording_type`], sorted, with the `Chat::Lines` subtypes represented by
/// their shared prefix (`Chat::Lines::*`). The set is deliberate rather than exhaustive —
/// see the routing table — and any other type is
/// [`RecordingSummaryError::UnknownRecordingType`] by design.
pub fn summarizable_recording_types() -> Vec<String> {
    let mut types: Vec<String> = RECORDING_TYPES
        .iter()
        .map(|(name, _)| (*name).to_string())
        .chain(std::iter::once(format!("{CHAT_LINE_TYPE_PREFIX}*")))
        .collect();
    types.sort();
    types
}

/// The account event feed subjects [`RecordingsService::summarize`] routes by
/// [`RecordingRef::event_type`] — an event type is `<subject>.<action>`, and any action on a
/// listed subject routes to that subject's read — sorted. `boost` is absent on purpose:
/// [`RecordingSummaryError::NoRecordingType`].
pub fn summarizable_event_types() -> Vec<String> {
    let mut subjects: Vec<String> = EVENT_SUBJECTS
        .iter()
        .map(|(subject, _)| format!("{subject}.*"))
        .collect();
    subjects.sort();
    subjects
}

/// Whether a chat line subtype carries rich text — the two that declare
/// `rich_text_attribute :content` in BC3, and so the only two whose content can hold a
/// mention. A `Text` line's content is HTML-escaped on the way out
/// (`content_helper.rb`, `format_chat_line_with`), a `Code` line's is served verbatim — a
/// snippet that happens to contain a `bc-attachment` tag — and an `Upload` line has no
/// content.
fn chat_line_is_rich_text(line_type: &str) -> bool {
    matches!(
        line_type,
        "Chat::Lines::RichText" | "Chat::Lines::Integration"
    )
}

/// Picks the read for a pointer. The recording type wins when set.
fn route(reference: &RecordingRef) -> Result<Kind, RecordingSummaryError> {
    let unknown = || RecordingSummaryError::UnknownRecordingType {
        key: reference.key().to_string(),
    };
    let recording_type = reference
        .recording_type
        .as_deref()
        .unwrap_or_default()
        .trim();
    if !recording_type.is_empty() {
        if recording_type.starts_with(CHAT_LINE_TYPE_PREFIX) {
            return Ok(Kind::ChatLine);
        }
        return lookup(RECORDING_TYPES, recording_type).ok_or_else(unknown);
    }
    let event_type = reference.event_type.as_deref().unwrap_or_default().trim();
    // A feed type is "<subject>.<action>"; the subject names the recording type. A string
    // with no action is not a feed type and is not routed.
    let Some(index) = event_type.rfind('.') else {
        return Err(unknown());
    };
    if index == 0 || index == event_type.len() - 1 {
        return Err(unknown());
    }
    let subject = &event_type[..index];
    if subject == "boost" {
        return Err(RecordingSummaryError::NoRecordingType {
            key: event_type.to_string(),
        });
    }
    lookup(EVENT_SUBJECTS, subject).ok_or_else(unknown)
}

fn lookup(table: &[(&str, Kind)], key: &str) -> Option<Kind> {
    table
        .iter()
        .find(|(name, _)| *name == key)
        .map(|(_, kind)| *kind)
}

impl RecordingsService<'_> {
    /// Resolves a recording pointer into a [`RecordingSummary`] through the typed read its
    /// type names. See [`RecordingRef`] for the routing inputs and the module docs for the
    /// design.
    ///
    /// # Errors
    ///
    /// A routing failure before any request; the read's own [`Error`] otherwise — a 404 is
    /// `not_found`, as from the typed read itself; for chat lines,
    /// [`RecordingSummaryError::Unresolved`] when every visible Campfire answered 404, which
    /// is distinct from a read that failed (any non-404 from a candidate is returned as that
    /// error, and the loop stops there) and from
    /// [`RecordingSummaryError::CampfireDiscoveryIncomplete`] (candidates were left
    /// unsearched); [`RecordingSummaryError::BucketMismatch`] when the read returned a
    /// recording from another bucket. Match the composite's own reasons with
    /// [`RecordingSummaryError::of`].
    pub async fn summarize(&self, reference: &RecordingRef) -> Result<RecordingSummary, Error> {
        if reference.bucket_id <= 0 || reference.recording_id <= 0 {
            return Err(Error::usage("bucket id and recording id are required"));
        }
        let kind = route(reference)?;
        let summary = read_summary(self.client(), reference, kind).await?;
        if let Some(bucket) = &summary.bucket
            && bucket.id != 0
            && bucket.id != reference.bucket_id
        {
            return Err(RecordingSummaryError::BucketMismatch {
                bucket_id: reference.bucket_id,
                recording_id: reference.recording_id,
                found_in: bucket.id,
            }
            .into());
        }
        Ok(summary)
    }
}

/// The recording fields every read shares, plus what its own shape adds, before the
/// mentions are read off the content.
struct Projected {
    id: i64,
    status: String,
    r#type: String,
    title: String,
    app_url: String,
    updated_at: DateTime,
    parent: Option<RecordingParent>,
    bucket: Option<RecordingBucket>,
    creator: Option<Person>,
    assignees: Vec<Person>,
    content: String,
}

impl Projected {
    fn new(
        id: i64,
        status: String,
        r#type: String,
        title: String,
        app_url: String,
        updated_at: DateTime,
    ) -> Projected {
        Projected {
            id,
            status,
            r#type,
            title,
            app_url,
            updated_at,
            parent: None,
            bucket: None,
            creator: None,
            assignees: Vec::new(),
            content: String::new(),
        }
    }

    fn parent(mut self, parent: RecordingParent) -> Projected {
        self.parent = Some(parent);
        self
    }

    fn maybe_parent(mut self, parent: Option<RecordingParent>) -> Projected {
        self.parent = parent;
        self
    }

    fn todo_parent(self, parent: TodoParent) -> Projected {
        self.parent(RecordingParent {
            id: parent.id,
            title: parent.title,
            r#type: parent.r#type,
            url: parent.url,
            app_url: parent.app_url,
            bucket: None,
        })
    }

    fn bucket(mut self, bucket: RecordingBucket) -> Projected {
        self.bucket = Some(bucket);
        self
    }

    fn todo_bucket(self, bucket: TodoBucket) -> Projected {
        self.bucket(RecordingBucket {
            id: bucket.id,
            name: bucket.name,
            r#type: bucket.r#type,
        })
    }

    fn creator(mut self, creator: Person) -> Projected {
        self.creator = Some(creator);
        self
    }

    fn assignees(mut self, assignees: Option<Vec<Person>>) -> Projected {
        self.assignees = assignees.unwrap_or_default();
        self
    }

    fn content(mut self, content: impl Into<String>) -> Projected {
        self.content = content.into();
        self
    }
}

impl From<Projected> for RecordingSummary {
    fn from(projected: Projected) -> RecordingSummary {
        RecordingSummary {
            id: projected.id,
            status: projected.status,
            r#type: projected.r#type,
            title: projected.title,
            app_url: projected.app_url,
            parent: projected.parent,
            bucket: projected.bucket,
            creator: projected.creator,
            assignees: projected.assignees,
            mentioned_person_ids: mentioned_person_ids(&projected.content),
            content: projected.content,
            updated_at: projected.updated_at,
            campfire_id: None,
        }
    }
}

/// The first of the candidates that is not empty, as a `String`.
fn first_non_empty(primary: String, fallback: Option<String>) -> String {
    if primary.is_empty() {
        fallback.unwrap_or_default()
    } else {
        primary
    }
}

/// Performs the one typed read a kind names and projects it.
///
/// One arm per routed type, spelled out: the projection differs per shape — which member
/// carries the rich text, which carries the human title — and a table could not say that.
#[allow(clippy::too_many_lines)]
async fn read_summary(
    account: &AccountClient,
    reference: &RecordingRef,
    kind: Kind,
) -> Result<RecordingSummary, Error> {
    let id = reference.recording_id;
    let projected = match kind {
        Kind::Comment => {
            let comment = account.comments().get(id).await?;
            Projected::new(
                comment.id,
                comment.status,
                comment.r#type,
                comment.title,
                comment.app_url,
                comment.updated_at,
            )
            .parent(comment.parent)
            .todo_bucket(comment.bucket)
            .creator(comment.creator)
            .content(comment.content)
        }
        Kind::Message => {
            let message = account.messages().get(id).await?;
            Projected::new(
                message.id,
                message.status,
                message.r#type,
                first_non_empty(message.title, Some(message.subject)),
                message.app_url,
                message.updated_at,
            )
            .parent(message.parent)
            .todo_bucket(message.bucket)
            .creator(message.creator)
            .content(message.content)
        }
        Kind::Todo => {
            // A to-do's `content` is its plain title; the rich text — where mentions live —
            // is the description.
            let todo = account.todos().get(id).await?;
            Projected::new(
                todo.id,
                todo.status,
                todo.r#type,
                first_non_empty(todo.title, Some(todo.content)),
                todo.app_url,
                todo.updated_at,
            )
            .todo_parent(todo.parent)
            .todo_bucket(todo.bucket)
            .creator(todo.creator)
            .assignees(todo.assignees)
            .content(todo.description.unwrap_or_default())
        }
        Kind::Card => {
            let card = account.cards().get(id).await?;
            Projected::new(
                card.id,
                card.status,
                card.r#type,
                card.title,
                card.app_url,
                card.updated_at,
            )
            .parent(card.parent)
            .todo_bucket(card.bucket)
            .creator(card.creator)
            .assignees(card.assignees)
            .content(first_non_empty(
                card.content.unwrap_or_default(),
                card.description,
            ))
        }
        Kind::ChatLine => return chat_line_summary(account, reference).await,
        Kind::Document => {
            let document = account.documents().get(id).await?;
            Projected::new(
                document.id,
                document.status,
                document.r#type,
                document.title,
                document.app_url,
                document.updated_at,
            )
            .parent(document.parent)
            .todo_bucket(document.bucket)
            .creator(document.creator)
            .content(document.content.unwrap_or_default())
        }
        Kind::Upload => {
            let upload = account.uploads().get(id).await?;
            Projected::new(
                upload.id,
                upload.status,
                upload.r#type,
                first_non_empty(upload.title, upload.filename),
                upload.app_url,
                upload.updated_at,
            )
            .parent(upload.parent)
            .todo_bucket(upload.bucket)
            .creator(upload.creator)
            .content(upload.description.unwrap_or_default())
        }
        Kind::ScheduleEntry => {
            let entry = account.schedules().get_entry(id).await?;
            Projected::new(
                entry.id,
                entry.status,
                entry.r#type,
                first_non_empty(entry.title, Some(entry.summary)),
                entry.app_url,
                entry.updated_at,
            )
            .parent(entry.parent)
            .todo_bucket(entry.bucket)
            .creator(entry.creator)
            .content(entry.description.unwrap_or_default())
        }
        Kind::Question => {
            let question = account.checkins().get_question(id).await?;
            Projected::new(
                question.id,
                question.status,
                question.r#type,
                question.title,
                question.app_url,
                question.updated_at,
            )
            .parent(question.parent)
            .bucket(question.bucket)
            .creator(question.creator)
        }
        Kind::QuestionAnswer => {
            let answer = account.checkins().get_answer(id).await?;
            Projected::new(
                answer.id,
                answer.status,
                answer.r#type,
                answer.title,
                answer.app_url,
                answer.updated_at,
            )
            .parent(answer.parent)
            .bucket(answer.bucket)
            .creator(answer.creator)
            .content(answer.content)
        }
        Kind::Todolist => {
            let todolist = account.todolists().get(id).await?;
            Projected::new(
                todolist.id,
                todolist.status,
                todolist.r#type,
                first_non_empty(todolist.title, Some(todolist.name)),
                todolist.app_url,
                todolist.updated_at,
            )
            .todo_parent(todolist.parent)
            .todo_bucket(todolist.bucket)
            .creator(todolist.creator)
            .content(todolist.description)
        }
        Kind::Vault => {
            let vault = account.vaults().get(id).await?;
            Projected::new(
                vault.id,
                vault.status,
                vault.r#type,
                vault.title,
                vault.app_url,
                vault.updated_at,
            )
            .maybe_parent(vault.parent)
            .todo_bucket(vault.bucket)
            .creator(vault.creator)
        }
        Kind::Forward => {
            let forward = account.forwards().get(id).await?;
            Projected::new(
                forward.id,
                forward.status,
                forward.r#type,
                first_non_empty(forward.title, Some(forward.subject)),
                forward.app_url,
                forward.updated_at,
            )
            .parent(forward.parent)
            .todo_bucket(forward.bucket)
            .creator(forward.creator)
            .content(forward.content.unwrap_or_default())
        }
        Kind::ClientApproval => {
            let approval = account.client_approvals().get(id).await?;
            Projected::new(
                approval.id,
                approval.status,
                approval.r#type,
                first_non_empty(approval.title, approval.subject),
                approval.app_url,
                approval.updated_at,
            )
            .parent(approval.parent)
            .bucket(approval.bucket)
            .creator(approval.creator)
            .content(approval.content.unwrap_or_default())
        }
        Kind::ClientCorrespondence => {
            let correspondence = account.client_correspondences().get(id).await?;
            Projected::new(
                correspondence.id,
                correspondence.status,
                correspondence.r#type,
                first_non_empty(correspondence.title, Some(correspondence.subject)),
                correspondence.app_url,
                correspondence.updated_at,
            )
            .parent(correspondence.parent)
            .bucket(correspondence.bucket)
            .creator(correspondence.creator)
            .content(correspondence.content.unwrap_or_default())
        }
        Kind::GoogleDocument => {
            let document = account.google_documents().google_document(id).await?;
            Projected::new(
                document.id,
                document.status,
                document.r#type,
                document.title,
                document.app_url,
                document.updated_at,
            )
            .parent(document.parent)
            .todo_bucket(document.bucket)
            .creator(document.creator)
            .content(document.description.unwrap_or_default())
        }
        Kind::CloudFile => {
            let file = account.cloud_files().cloud_file(id).await?;
            Projected::new(
                file.id,
                file.status,
                file.r#type,
                file.title,
                file.app_url,
                file.updated_at,
            )
            .parent(file.parent)
            .todo_bucket(file.bucket)
            .creator(file.creator)
            .content(file.description.unwrap_or_default())
        }
        Kind::CardStep => {
            let step = account.card_steps().get(id).await?;
            Projected::new(
                step.id,
                step.status,
                step.r#type,
                step.title,
                step.app_url,
                step.updated_at,
            )
            .parent(step.parent)
            .todo_bucket(step.bucket)
            .creator(step.creator)
            .assignees(step.assignees)
        }
        Kind::Questionnaire => {
            let questionnaire = account.checkins().get_questionnaire(id).await?;
            Projected::new(
                questionnaire.id,
                questionnaire.status,
                questionnaire.r#type,
                first_non_empty(questionnaire.title, Some(questionnaire.name)),
                questionnaire.app_url,
                questionnaire.updated_at,
            )
            .bucket(questionnaire.bucket)
            .creator(questionnaire.creator)
        }
        Kind::Schedule => {
            let schedule = account.schedules().get(id).await?;
            Projected::new(
                schedule.id,
                schedule.status,
                schedule.r#type,
                schedule.title,
                schedule.app_url,
                schedule.updated_at,
            )
            .todo_bucket(schedule.bucket)
            .creator(schedule.creator)
        }
        Kind::Todoset => {
            let todoset = account.todosets().get(id).await?;
            Projected::new(
                todoset.id,
                todoset.status,
                todoset.r#type,
                first_non_empty(todoset.title, Some(todoset.name)),
                todoset.app_url,
                todoset.updated_at,
            )
            .todo_bucket(todoset.bucket)
            .creator(todoset.creator)
        }
        Kind::MessageBoard => {
            let board = account.message_boards().get(id).await?;
            Projected::new(
                board.id,
                board.status,
                board.r#type,
                board.title,
                board.app_url,
                board.updated_at,
            )
            .todo_bucket(board.bucket)
            .creator(board.creator)
        }
        Kind::CardTable => {
            let table = account.card_tables().get(id).await?;
            Projected::new(
                table.id,
                table.status,
                table.r#type,
                table.title,
                table.app_url,
                table.updated_at,
            )
            .todo_bucket(table.bucket)
            .creator(table.creator)
        }
        Kind::CardColumn => {
            let column = account.card_columns().get(id).await?;
            Projected::new(
                column.id,
                column.status,
                column.r#type,
                column.title,
                column.app_url,
                column.updated_at,
            )
            .parent(column.parent)
            .todo_bucket(column.bucket)
            .creator(column.creator)
            .content(column.description.unwrap_or_default())
        }
        Kind::Inbox => {
            let inbox = account.forwards().get_inbox(id).await?;
            Projected::new(
                inbox.id,
                inbox.status,
                inbox.r#type,
                inbox.title,
                inbox.app_url,
                inbox.updated_at,
            )
            .todo_bucket(inbox.bucket)
            .creator(inbox.creator)
        }
        Kind::Campfire => {
            let campfire = account.campfires().get(id).await?;
            Projected::new(
                campfire.id,
                campfire.status,
                campfire.r#type,
                campfire.title,
                campfire.app_url,
                campfire.updated_at,
            )
            .todo_bucket(campfire.bucket)
            .creator(campfire.creator)
        }
    };
    Ok(projected.into())
}

/// Discovers the Campfire a chat line lives in, reads the line under it, and projects it.
async fn chat_line_summary(
    account: &AccountClient,
    reference: &RecordingRef,
) -> Result<RecordingSummary, Error> {
    let (line, campfire_id) =
        resolve_chat_line(account, reference.bucket_id, reference.recording_id).await?;
    let rich_text = chat_line_is_rich_text(&line.r#type);
    let mut summary: RecordingSummary = Projected::new(
        line.id,
        line.status,
        line.r#type,
        line.title,
        line.app_url,
        line.updated_at,
    )
    .parent(line.parent)
    .todo_bucket(line.bucket)
    .creator(line.creator)
    .content(line.content.unwrap_or_default())
    .into();
    if !rich_text {
        // A plain-text or code line's content is text BC3 never read as markup, so a
        // literal "<bc-attachment>" in it mentions nobody.
        summary.mentioned_person_ids.clear();
    }
    summary.campfire_id = Some(campfire_id);
    Ok(summary)
}

/// One `summarize` call's discovery state: which candidates have answered "not here", how
/// many more this call may try, and whether any were left untried for want of budget.
struct ChatLineSearch<'a> {
    account: &'a AccountClient,
    line_id: i64,
    tried: Vec<i64>,
    budget: usize,
    skipped: bool,
}

impl ChatLineSearch<'_> {
    /// Reads the line under each candidate not yet tried. Answers the line and its Campfire
    /// on a hit; on a miss it answers `None` with no error and records the candidates in
    /// `tried`. Any answer but 404 is returned as is.
    async fn try_candidates(
        &mut self,
        candidates: &[i64],
    ) -> Result<Option<(CampfireLine, i64)>, Error> {
        for campfire_id in candidates {
            if self.tried.contains(campfire_id) {
                continue;
            }
            if self.budget == 0 {
                self.skipped = true;
                return Ok(None);
            }
            self.budget -= 1;
            match self
                .account
                .campfires()
                .get_line(*campfire_id, self.line_id)
                .await
            {
                Ok(line) => return Ok(Some((line, *campfire_id))),
                Err(error) if error.code() == ErrorCode::NotFound => self.tried.push(*campfire_id),
                Err(error) => return Err(error),
            }
        }
        Ok(None)
    }
}

/// Finds the Campfire a line lives in and reads it.
///
/// Two failure shapes are kept apart on purpose. A candidate that answers anything but 404 —
/// 401, 403, 5xx, a transport error — stops the loop and is returned as that error: the read
/// failed, and trying the next Campfire would only hide it. A 404 means "not here", so the
/// loop moves on. Only when every candidate said "not here" is the line
/// [`RecordingSummaryError::Unresolved`] — and before concluding that, the cached sources
/// are refreshed (subject to a floor) so a Campfire created after the cache filled is tried
/// too. Discovery that could not be completed — a listing cut off at its cap, a bucket with
/// more candidates than the budget — is
/// [`RecordingSummaryError::CampfireDiscoveryIncomplete`], never "unresolved": nothing
/// unsearched is ever reported absent.
///
/// What HTTP cannot tell apart: BC3 answers 404 both for a line that is not in a Campfire
/// and for a Campfire the caller may no longer see. "Unresolved" therefore means "under no
/// Campfire the caller can currently see", and the error reports the cached candidates that
/// the refreshed sources no longer list
/// ([`UnresolvedRecording::stale_campfire_ids`]) so a consumer can see when visibility, not
/// existence, is what changed.
async fn resolve_chat_line(
    account: &AccountClient,
    bucket_id: i64,
    line_id: i64,
) -> Result<(CampfireLine, i64), Error> {
    let index = account.campfire_index();
    let mut search = ChatLineSearch {
        account,
        line_id,
        tried: Vec::new(),
        budget: MAX_CAMPFIRE_CANDIDATES,
        skipped: false,
    };
    let incomplete = |reason: String| -> Error {
        RecordingSummaryError::CampfireDiscoveryIncomplete {
            bucket_id,
            recording_id: line_id,
            reason,
        }
        .into()
    };
    let over_budget =
        || format!("more than {MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket");

    // Pass 1: what the sources already hold — the dock (read if it must be), then the
    // listing only if it is cached. A listing fetch is the expensive, slow request, and it
    // is not made until the dock — including its refresh — has had its say, so a listing
    // that is down, over its cap, or stalled on a deadline never stands between a project's
    // line and the one project read that finds it.
    let mut dock = index.dock_campfires(account, bucket_id, false).await?;
    if let Some(found) = search.try_candidates(&dock.ids).await? {
        return Ok(found);
    }
    let listed = index.cached_listed_campfires(account.account_id(), bucket_id);
    if let Some(listed) = &listed
        && let Some(found) = search.try_candidates(&listed.ids).await?
    {
        return Ok(found);
    }

    // Pass 2: re-read the dock if it was served from cache (the floor may decline), then
    // fetch or refresh the listing. Whatever comes back is the current snapshot of that
    // source, whoever loaded it — another caller may have populated or refreshed it in the
    // meantime — so it always replaces the pass-1 one; "refreshed" is whether a source the
    // conclusion had consulted is now newer than when it was consulted.
    //
    // A spent budget stops both, and the two sources part company on WHY. A source already
    // consulted cannot hand this call a candidate it may try, so its refresh is skipped and
    // the conclusion stands on what was seen (`refreshed` stays false) — paying for that
    // request would buy nothing, and a failure on it would replace a settled `incomplete`
    // with a transient error a consumer retries forever. A source NEVER consulted is a
    // different verdict: candidates may exist there unsearched, so running out of budget
    // before reaching it makes the answer incomplete rather than unresolved.
    //
    // `skipped` alone cannot express that, which is what the earlier shape got wrong. It is
    // set only when a candidate is OBSERVED and cannot be tried, so a source holding
    // exactly `MAX_CAMPFIRE_CANDIDATES` candidates that all answer 404 leaves the budget at
    // zero with `skipped` still false. And concluding `incomplete` on a spent budget alone
    // is equally wrong in the other direction: a bucket whose candidates were all searched
    // is not incomplete. The budget gates the re-reads; `skipped` still decides the final
    // verdict.
    let mut refreshed = false;
    if search.budget > 0 && dock.cached {
        let again = index.dock_campfires(account, bucket_id, true).await?;
        if newer_than(&again, &dock) {
            refreshed = true;
        }
        dock = again;
        if let Some(found) = search.try_candidates(&dock.ids).await? {
            return Ok(found);
        }
    }
    // Seeded from what pass 1 consulted, not empty: on the budget-spent path the listing is
    // not re-read, and the stale filter below has to compare against the snapshot the
    // conclusion actually used. Starting empty reports every listing-only candidate as one
    // the caller has LOST VISIBILITY of, when nothing changed.
    let mut listed_ids = listed
        .as_ref()
        .map(|listed| listed.ids.clone())
        .unwrap_or_default();
    if search.budget == 0 {
        if listed.is_none() {
            return Err(incomplete(format!(
                "the candidate budget of {MAX_CAMPFIRE_CANDIDATES} was spent before the account listing was consulted"
            )));
        }
    } else {
        let again = index
            .listed_campfires(account, bucket_id, listed.is_some())
            .await
            .map_err(|error| {
                if is_listing_overflow(&error) {
                    incomplete(error.to_string())
                } else {
                    error
                }
            })?;
        if let Some(previous) = &listed
            && newer_than(&again, previous)
        {
            refreshed = true;
        }
        listed_ids = again.ids;
        if let Some(found) = search.try_candidates(&listed_ids).await? {
            return Ok(found);
        }
    }
    if search.skipped {
        return Err(incomplete(over_budget()));
    }

    let mut unresolved = UnresolvedRecording {
        bucket_id,
        recording_id: line_id,
        campfire_ids: search.tried.clone(),
        refreshed,
        stale_campfire_ids: Vec::new(),
    };
    if refreshed {
        unresolved.stale_campfire_ids = search
            .tried
            .iter()
            .copied()
            .filter(|id| !dock.ids.contains(id) && !listed_ids.contains(id))
            .collect();
    }
    Err(RecordingSummaryError::Unresolved(unresolved).into())
}

/// Whether a re-read of a source answered something newer than the snapshot the conclusion
/// had already consulted — a fresh load, or a snapshot another caller published in the
/// meantime.
fn newer_than(again: &SourceRead, previous: &SourceRead) -> bool {
    !again.cached || again.fetched > previous.fetched
}

#[cfg(test)]
mod tests {
    use super::*;

    fn kind_of(reference: &RecordingRef) -> Result<Kind, RecordingSummaryError> {
        route(reference)
    }

    #[test]
    fn a_recording_type_routes_and_wins_over_the_event_type() {
        let reference = RecordingRef {
            bucket_id: 1,
            recording_id: 2,
            event_type: Some("comment.created".to_string()),
            recording_type: Some("Kanban::Card".to_string()),
        };
        assert_eq!(kind_of(&reference), Ok(Kind::Card));
    }

    #[test]
    fn every_chat_line_subtype_reads_through_the_one_route() {
        for subtype in [
            "Chat::Lines::Text",
            "Chat::Lines::RichText",
            "Chat::Lines::Code",
            "Chat::Lines::Upload",
            "Chat::Lines::Integration",
        ] {
            assert_eq!(
                kind_of(&RecordingRef::from_recording_type(1, 2, subtype)),
                Ok(Kind::ChatLine),
                "{subtype}"
            );
        }
    }

    #[test]
    fn every_feed_subject_routes_whatever_the_action_is() {
        for event in [
            "comment.created",
            "message.updated",
            "todo.assignment_changed",
            "card.moved",
            "chat.line.created",
        ] {
            assert!(
                kind_of(&RecordingRef::from_event(1, 2, event)).is_ok(),
                "{event}"
            );
        }
    }

    #[test]
    fn boost_is_refused_as_naming_no_recording_type() {
        let refused = kind_of(&RecordingRef::from_event(1, 2, "boost.created"));
        assert!(matches!(
            refused,
            Err(RecordingSummaryError::NoRecordingType { .. })
        ));
        let error: Error = refused.unwrap_err().into();
        assert_eq!(error.code(), ErrorCode::Usage);
        assert!(error.to_string().contains("names no recording type"));
        assert!(matches!(
            RecordingSummaryError::of(&error),
            Some(RecordingSummaryError::NoRecordingType { .. })
        ));
    }

    #[test]
    fn an_unrouted_type_or_a_malformed_event_type_is_unknown() {
        for reference in [
            RecordingRef::from_recording_type(1, 2, "Timesheet::Entry"),
            RecordingRef::from_recording_type(1, 2, "Client::Reply"),
            RecordingRef::from_event(1, 2, "comment"),
            RecordingRef::from_event(1, 2, "comment."),
            RecordingRef::from_event(1, 2, ".created"),
            RecordingRef::from_event(1, 2, "unheard_of.created"),
            RecordingRef::default(),
        ] {
            assert!(
                matches!(
                    kind_of(&reference),
                    Err(RecordingSummaryError::UnknownRecordingType { .. })
                ),
                "{reference:?}"
            );
        }
    }

    #[test]
    fn the_documented_sets_are_the_routed_ones() {
        let types = summarizable_recording_types();
        assert_eq!(types.len(), RECORDING_TYPES.len() + 1);
        assert!(types.contains(&"Chat::Lines::*".to_string()));
        assert!(types.windows(2).all(|pair| pair[0] <= pair[1]), "sorted");
        for (name, _) in RECORDING_TYPES {
            assert!(types.contains(&(*name).to_string()), "{name}");
            assert!(
                kind_of(&RecordingRef::from_recording_type(1, 2, *name)).is_ok(),
                "{name}"
            );
        }
        let events = summarizable_event_types();
        assert_eq!(events.len(), EVENT_SUBJECTS.len());
        assert!(!events.iter().any(|subject| subject.starts_with("boost")));
        assert!(events.windows(2).all(|pair| pair[0] <= pair[1]), "sorted");
    }

    #[test]
    fn only_the_two_rich_text_line_subtypes_can_carry_a_mention() {
        assert!(chat_line_is_rich_text("Chat::Lines::RichText"));
        assert!(chat_line_is_rich_text("Chat::Lines::Integration"));
        assert!(!chat_line_is_rich_text("Chat::Lines::Text"));
        assert!(!chat_line_is_rich_text("Chat::Lines::Code"));
        assert!(!chat_line_is_rich_text("Chat::Lines::Upload"));
    }

    #[test]
    fn the_composites_own_reasons_carry_their_taxonomy_member() {
        let unresolved: Error = RecordingSummaryError::Unresolved(UnresolvedRecording {
            bucket_id: 1,
            recording_id: 2,
            campfire_ids: vec![3, 4],
            refreshed: true,
            stale_campfire_ids: vec![3],
        })
        .into();
        assert_eq!(unresolved.code(), ErrorCode::NotFound);
        assert!(
            unresolved
                .to_string()
                .contains("found under no visible campfire")
        );

        let incomplete: Error = RecordingSummaryError::CampfireDiscoveryIncomplete {
            bucket_id: 1,
            recording_id: 2,
            reason: "too many".to_string(),
        }
        .into();
        // `usage`, settled on card 40: the one coarse code no HTTP response can
        // produce, so this verdict can never be read back as a constituent read's
        // own answer. Non-retryable, which is the other half of that decision and
        // the half a code change could otherwise carry away with it.
        assert_eq!(incomplete.code(), ErrorCode::Usage);
        assert!(!incomplete.is_retryable());
        // Never not_found: nothing left unsearched may be reported absent.
        assert_ne!(incomplete.code(), unresolved.code());

        let mismatch: Error = RecordingSummaryError::BucketMismatch {
            bucket_id: 1,
            recording_id: 2,
            found_in: 9,
        }
        .into();
        assert_eq!(mismatch.code(), ErrorCode::NotFound);
        assert!(matches!(
            RecordingSummaryError::of(&mismatch),
            Some(RecordingSummaryError::BucketMismatch { found_in: 9, .. })
        ));
    }

    #[test]
    fn a_plain_read_failure_carries_no_composite_reason() {
        let forbidden = Error::new(ErrorCode::Forbidden, "nope").with_status(403);
        assert!(RecordingSummaryError::of(&forbidden).is_none());
    }

    #[test]
    fn a_verdict_survives_being_wrapped_on_its_way_out() {
        let verdict: Error = RecordingSummaryError::BucketMismatch {
            bucket_id: 1,
            recording_id: 2,
            found_in: 9,
        }
        .into();
        let wrapped = Error::new(ErrorCode::ApiError, "while admitting an event")
            .with_source(std::sync::Arc::new(verdict));
        assert!(matches!(
            RecordingSummaryError::of(&wrapped),
            Some(RecordingSummaryError::BucketMismatch { found_in: 9, .. })
        ));
    }

    #[test]
    fn a_title_falls_back_only_when_the_primary_is_empty() {
        assert_eq!(
            first_non_empty("title".to_string(), Some("subject".to_string())),
            "title"
        );
        assert_eq!(
            first_non_empty(String::new(), Some("subject".to_string())),
            "subject"
        );
        assert_eq!(first_non_empty(String::new(), None), "");
    }
}

/// The refresh path, which needs the floor crossed and so needs a clock a test can move.
/// Everything here turns on entries ageing: the rest of the discovery behaviour is driven
/// end-to-end in `tests/composites_recording_summary.rs`, where real time is enough.
///
/// These drive wiremock and so need the shipped transport, exactly as the OAuth unit tests
/// do. The no-default-features lane builds the crate WITHOUT one — that is the point of the
/// lane — so it skips this module rather than failing in it.
#[cfg(all(test, feature = "reqwest"))]
mod refresh_tests {
    use super::*;
    use crate::Config;
    use crate::client::Client;
    use crate::services::campfire_index::{
        CAMPFIRE_INDEX_MIN_REFRESH, MAX_CAMPFIRE_CANDIDATES, TestClock,
    };
    use serde_json::{Value, json};
    use std::sync::Arc;
    use std::time::Duration;
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    const BUCKET: i64 = 1_069_479_338;
    const LINE: i64 = 1_069_479_351;
    const CACHED_A: i64 = 1_069_479_340;
    const CACHED_B: i64 = 1_069_479_345;
    const APPEARED: i64 = 1_069_479_399;
    const OTHER_BUCKET: i64 = 2_085_958_500;

    fn campfire(id: i64, bucket: i64) -> Value {
        json!({
            "id": id,
            "status": "active",
            "created_at": "2022-10-28T15:25:00.000Z",
            "updated_at": "2022-10-28T15:25:00.000Z",
            "title": "Campfire",
            "visible_to_clients": false,
            "inherits_status": true,
            "type": "Chat::Transcript",
            "url": "https://3.basecampapi.com/999/buckets/x/chats/x.json",
            "app_url": "https://3.basecamp.com/999/buckets/x/chats/x",
            "bucket": { "id": bucket, "name": "A project", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" },
        })
    }

    fn project_docking(campfire_ids: &[i64]) -> Value {
        let dock: Vec<Value> = campfire_ids
            .iter()
            .map(|id| {
                json!({ "id": id, "title": "Campfire", "name": "chat", "enabled": true, "url": "u", "app_url": "a" })
            })
            .collect();
        json!({
            "id": BUCKET,
            "status": "active",
            "created_at": "2022-10-28T15:25:00.000Z",
            "updated_at": "2022-10-28T15:25:00.000Z",
            "name": "A project",
            "url": "https://3.basecampapi.com/999/projects/x.json",
            "app_url": "https://3.basecamp.com/999/projects/x",
            "dock": dock,
        })
    }

    async fn mount(server: &MockServer, route: &str, status: u16, body: &Value) {
        Mock::given(method("GET"))
            .and(path(route))
            .respond_with(ResponseTemplate::new(status).set_body_json(body.clone()))
            .mount(server)
            .await;
    }

    /// The dock shows `docked`, the account listing shows `listed`, and every line read
    /// answers 404 — so the search always runs to the end and reports what it tried. The
    /// line routes are mounted one per candidate rather than as a catch-all, which could
    /// shadow the dock or the listing and decide the test by mount order.
    async fn scripted(server: &MockServer, docked: &[i64], listed: &[i64]) {
        server.reset().await;
        mount(
            server,
            &format!("/999/projects/{BUCKET}"),
            200,
            &project_docking(docked),
        )
        .await;
        let listing: Vec<Value> = listed.iter().map(|id| campfire(*id, BUCKET)).collect();
        mount(server, "/999/chats.json", 200, &json!(listing)).await;
        for id in docked.iter().chain(listed) {
            mount(
                server,
                &format!("/999/chats/{id}/lines/{LINE}"),
                404,
                &json!({ "error": "Record not found" }),
            )
            .await;
        }
    }

    async fn paths(server: &MockServer) -> Vec<String> {
        server
            .received_requests()
            .await
            .unwrap_or_default()
            .iter()
            .map(|request| request.url.path().to_string())
            .collect()
    }

    fn account_on(server: &MockServer, clock: &Arc<TestClock>) -> AccountClient {
        Client::builder(
            Config::default()
                .with_base_url(server.uri())
                .with_timeout(Duration::from_secs(86_400)),
        )
        .access_token("test-token")
        .campfire_clock(clock.clock())
        .build()
        .expect("client")
        .for_account("999")
    }

    fn unresolved_from(error: &Error) -> UnresolvedRecording {
        match RecordingSummaryError::of(error) {
            Some(RecordingSummaryError::Unresolved(unresolved)) => unresolved.clone(),
            _ => panic!("expected an unresolved line, got {error}"),
        }
    }

    /// Past the floor a miss re-reads both sources, and the conclusion says so: `refreshed`
    /// is set, and the Campfires the cache had that neither source lists any more are named
    /// as stale. That is the only signal a consumer has that the line went missing because
    /// visibility changed rather than because it never existed — a 404 per candidate cannot
    /// tell the two apart. Mirrors Go's `TestSummarize_ChatLineDiscovery`.
    #[tokio::test]
    async fn a_refreshed_miss_names_the_campfires_the_caller_can_no_longer_see() {
        let clock = TestClock::new();
        let server = MockServer::start().await;
        scripted(&server, &[CACHED_A], &[CACHED_A, CACHED_B]).await;
        let account = account_on(&server, &clock);
        let reference = RecordingRef::from_event(BUCKET, LINE, "chat.line.created");

        // First call: both sources are read during the call, so nothing was refreshed.
        let first = unresolved_from(
            &account
                .recordings()
                .summarize(&reference)
                .await
                .unwrap_err(),
        );
        assert_eq!(first.campfire_ids, vec![CACHED_A, CACHED_B]);
        assert!(!first.refreshed, "sources read this very call");
        assert!(first.stale_campfire_ids.is_empty());

        // Past the floor, with both sources now showing a Campfire that did not exist when
        // the cache filled and no longer showing either that did.
        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH + Duration::from_secs(1));
        scripted(&server, &[APPEARED], &[APPEARED]).await;

        let second = unresolved_from(
            &account
                .recordings()
                .summarize(&reference)
                .await
                .unwrap_err(),
        );
        assert!(
            second.refreshed,
            "the floor was past and both sources answered newer than the cache"
        );
        assert_eq!(
            second.campfire_ids,
            vec![CACHED_A, CACHED_B, APPEARED],
            "the cached candidates were tried first, then the one the refresh found"
        );
        assert_eq!(
            second.stale_campfire_ids,
            vec![CACHED_A, CACHED_B],
            "both cached Campfires are gone from the dock and the listing"
        );
    }

    /// Refreshed by the LISTING alone. The account listing is cached per account, not per
    /// bucket, so the first bucket looked at pays for it and every other bucket meets it
    /// already warm — while its own dock is read fresh this call and so is never re-read.
    /// That is the one path where the listing refresh is what makes the conclusion a
    /// refreshed one, and the only reason to test it separately is that on every other path
    /// the dock re-read has already said so.
    ///
    /// Written after mutation testing: with the listing branch's `refreshed = true` removed,
    /// every other test here still passed.
    #[tokio::test]
    async fn a_second_bucket_meets_a_warm_listing_and_is_refreshed_by_it_alone() {
        let clock = TestClock::new();
        let server = MockServer::start().await;
        // Neither bucket is a project, so the dock answers "no dock" and the listing is the
        // only source. It shows one Campfire in each bucket.
        for bucket in [BUCKET, OTHER_BUCKET] {
            mount(
                &server,
                &format!("/999/projects/{bucket}"),
                404,
                &json!({ "error": "Record not found" }),
            )
            .await;
        }
        for id in [CACHED_A, CACHED_B] {
            mount(
                &server,
                &format!("/999/chats/{id}/lines/{LINE}"),
                404,
                &json!({ "error": "Record not found" }),
            )
            .await;
        }
        mount(
            &server,
            "/999/chats.json",
            200,
            &json!([campfire(CACHED_A, BUCKET), campfire(CACHED_B, OTHER_BUCKET)]),
        )
        .await;
        let account = account_on(&server, &clock);

        // The first bucket pays for the listing.
        let first = unresolved_from(
            &account
                .recordings()
                .summarize(&RecordingRef::from_event(BUCKET, LINE, "chat.line.created"))
                .await
                .unwrap_err(),
        );
        assert_eq!(first.campfire_ids, vec![CACHED_A]);
        assert!(!first.refreshed, "the listing was read during that call");

        // Past the floor, and the listing no longer shows the second bucket's Campfire.
        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH + Duration::from_secs(1));
        server.reset().await;
        for bucket in [BUCKET, OTHER_BUCKET] {
            mount(
                &server,
                &format!("/999/projects/{bucket}"),
                404,
                &json!({ "error": "Record not found" }),
            )
            .await;
        }
        mount(
            &server,
            &format!("/999/chats/{CACHED_B}/lines/{LINE}"),
            404,
            &json!({ "error": "Record not found" }),
        )
        .await;
        mount(
            &server,
            "/999/chats.json",
            200,
            &json!([campfire(CACHED_A, BUCKET)]),
        )
        .await;

        let second = unresolved_from(
            &account
                .recordings()
                .summarize(&RecordingRef::from_event(
                    OTHER_BUCKET,
                    LINE,
                    "chat.line.created",
                ))
                .await
                .unwrap_err(),
        );
        assert_eq!(second.bucket_id, OTHER_BUCKET);
        assert_eq!(
            second.campfire_ids,
            vec![CACHED_B],
            "the warm listing supplied the candidate"
        );
        assert!(
            second.refreshed,
            "this bucket's dock was read fresh and never re-read, so only the listing \
             refresh can have said the conclusion rests on newer data"
        );
        assert_eq!(
            second.stale_campfire_ids,
            vec![CACHED_B],
            "the refreshed listing no longer shows it: visibility changed"
        );
    }

    /// The stale list is measured against what the conclusion ACTUALLY consulted, and on
    /// this path the listing is never re-read: the dock refresh spends the last of the
    /// budget, so refreshing the listing could hand this call no candidate it may try and
    /// is skipped. The cached listing's Campfires are therefore still visible as far as
    /// this call knows, and naming them stale would tell a consumer visibility changed when
    /// nothing did. That is the whole reason the listing snapshot is carried forward
    /// instead of starting empty.
    ///
    /// Written after mutation testing showed the other two tests here pass with the
    /// snapshot dropped: they take the branch that overwrites it.
    #[tokio::test]
    async fn a_refresh_that_spends_the_budget_measures_stale_against_the_listing_it_read() {
        let clock = TestClock::new();
        let server = MockServer::start().await;
        // Pass 1 costs two candidates, leaving the rest of the budget for the dock refresh.
        let appeared: Vec<i64> = (0..i64::try_from(MAX_CAMPFIRE_CANDIDATES).unwrap() - 2)
            .map(|index| 9_000_000 + index)
            .collect();
        scripted(&server, &[CACHED_A], &[CACHED_A, CACHED_B]).await;
        let account = account_on(&server, &clock);
        let reference = RecordingRef::from_event(BUCKET, LINE, "chat.line.created");
        account
            .recordings()
            .summarize(&reference)
            .await
            .unwrap_err();

        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH + Duration::from_secs(1));
        // The dock now holds exactly the remaining budget, and the listing still holds what
        // pass 1 saw. Only the dock is re-read; the listing snapshot is the cached one.
        scripted(&server, &appeared, &[CACHED_A, CACHED_B]).await;
        let before = paths(&server).await.len();

        let unresolved = unresolved_from(
            &account
                .recordings()
                .summarize(&reference)
                .await
                .unwrap_err(),
        );
        assert!(
            unresolved.refreshed,
            "the dock re-read answered newer than the cache"
        );
        let mut expected = vec![CACHED_A, CACHED_B];
        expected.extend(&appeared);
        assert_eq!(unresolved.campfire_ids, expected);
        assert!(
            unresolved.stale_campfire_ids.is_empty(),
            "the cached listing still lists both, so neither has been lost from view: {:?}",
            unresolved.stale_campfire_ids
        );
        let after = paths(&server).await;
        assert!(
            !after[before..].iter().any(|path| path == "/999/chats.json"),
            "a listing refresh that could admit no candidate was not paid for"
        );
    }

    /// The stale list is what the conclusion ACTUALLY consulted minus what the refresh
    /// shows, so a candidate the refreshed listing still lists is not stale — even though
    /// the dock (which never had it) does not name it either.
    #[tokio::test]
    async fn a_candidate_the_refresh_still_lists_is_not_stale() {
        let clock = TestClock::new();
        let server = MockServer::start().await;
        scripted(&server, &[CACHED_A], &[CACHED_A, CACHED_B]).await;
        let account = account_on(&server, &clock);
        let reference = RecordingRef::from_event(BUCKET, LINE, "chat.line.created");
        account
            .recordings()
            .summarize(&reference)
            .await
            .unwrap_err();

        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH + Duration::from_secs(1));
        scripted(&server, &[APPEARED], &[CACHED_B]).await;

        let second = unresolved_from(
            &account
                .recordings()
                .summarize(&reference)
                .await
                .unwrap_err(),
        );
        assert!(second.refreshed);
        assert_eq!(
            second.stale_campfire_ids,
            vec![CACHED_A],
            "CACHED_B is still listed, so only the dropped dock Campfire is stale"
        );
    }
}
