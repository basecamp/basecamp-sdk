//! Schedules: the generated wire methods plus the merge-safe `update_entry` and
//! read-modify-write `edit_entry` of SPEC §5 "Merge-Safe Write Surface (Schedule
//! Entries)".
//!
//! `PUT /schedule_entries/{entryId}` is a full replace with declared carve-outs. BC3
//! rebuilds the recordable from the permitted params, so a body that omits `description`
//! erases it, one that omits `summary` leaves the entry reading back as "Untitled", and
//! one that omits `all_day` turns an all-day entry into a midnight-to-midnight timed one.
//! Three writable fields are exempt: `participant_ids`, `url` and `highlighted` are
//! preserved server-side when the body does not name them.
//!
//! The composites therefore split the writable set in two. The five replaced fields —
//! `summary`, `description`, `all_day`, `starts_at`, `ends_at` — are read back and
//! resent whether or not the caller touched them. The three carve-outs, and the `notify`
//! directive, reach the wire only when the caller addressed them, and then apply
//! normally: `vec![]` clears the participants, `""` clears the join link, `false` removes
//! the highlight. Resending an unaddressed carve-out would be redundant at best and wrong
//! if the read raced a change — and the response spells the join link `join_url`, because
//! the entry's own `url` is its Basecamp API URL.
//!
//! Both composites go through [`SchedulesService::get_entry`] and
//! [`SchedulesService::replace_entry`], so hooks observe `GetScheduleEntry` then
//! `ReplaceScheduleEntry` under their own identities. Recurring entries are unreachable
//! here: BC3 redirects both reads and writes of one to its occurrence.
//!
//! Neither composite is atomic: a concurrent write between the GET and the PUT is
//! overwritten, last write wins, with a window of one round-trip. Use `replace_entry` to
//! overwrite deliberately.

pub use crate::generated::services::schedules::{ListScheduleEntriesParams, SchedulesService};

use crate::error::Error;
use crate::generated::types::{ReplaceScheduleEntryRequestContent, ScheduleEntry};
use crate::types::FlexibleTime;

/// The fields a merge-safe [`SchedulesService::update_entry`] may set. A `None` member is
/// not addressed: for the five replaced fields the read-back value is resent, and for
/// the carve-outs the key stays off the wire so BC3 keeps what it holds. A `Some` member
/// is written, so `Some(String::new())`, `Some(vec![])` and `Some(false)` are clears.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct UpdateScheduleEntryRequest {
    /// Plain-text summary. Cleared, the entry reads back as "Untitled".
    pub summary: Option<String>,
    /// Rich text description (HTML).
    pub description: Option<String>,
    /// Whether the entry spans whole days.
    pub all_day: Option<bool>,
    /// The start, a bare date for an all-day entry or a full timestamp otherwise, sent
    /// verbatim.
    pub starts_at: Option<FlexibleTime>,
    /// The end. See `starts_at` for the date-versus-timestamp rule.
    pub ends_at: Option<FlexibleTime>,
    /// Replaces the participants; `Some(vec![])` clears them.
    pub participant_ids: Option<Vec<i64>>,
    /// The join link; `Some(String::new())` clears it.
    pub url: Option<String>,
    /// Whether the entry is highlighted; `Some(false)` removes the highlight.
    pub highlighted: Option<bool>,
    /// Whether to notify participants. A directive, not entry state.
    pub notify: Option<bool>,
}

/// A schedule entry's writable state, handed to the [`SchedulesService::edit_entry`]
/// closure.
///
/// The five replaced fields are plain members: seeded from the read-back and resent
/// whether or not the closure touches them, so clearing one means setting it `""`. The
/// carve-outs are seeded for reading — [`url`](Self::url) from the response's `join_url`,
/// [`participant_ids`](Self::participant_ids) projected from its `participants`,
/// [`highlighted`](Self::highlighted) as returned — but reach the wire only when the
/// matching setter is invoked. Dirty tracking is by setter invocation, not value
/// comparison: assigning the value the read returned is still an address and is sent.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScheduleEntryFields {
    /// Plain-text summary. Set `""` to clear; the entry then reads back as "Untitled".
    pub summary: String,
    /// Rich text description (HTML). Set `""` to clear.
    pub description: String,
    /// Whether the entry spans whole days.
    pub all_day: bool,
    /// The start, round-tripped verbatim: a bare date for an all-day entry, a full
    /// timestamp otherwise.
    pub starts_at: FlexibleTime,
    /// The end, round-tripped verbatim.
    pub ends_at: FlexibleTime,
    participant_ids: Option<Vec<i64>>,
    url: Option<String>,
    highlighted: Option<bool>,
    notify: Option<bool>,
    addressed: Addressed,
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
struct Addressed {
    participant_ids: bool,
    url: bool,
    highlighted: bool,
    notify: bool,
}

impl ScheduleEntryFields {
    /// The participants' ids as the read-back projected them, or `None` when the
    /// response carried no `participants`.
    pub fn participant_ids(&self) -> Option<&[i64]> {
        self.participant_ids.as_deref()
    }

    /// The join link as the read-back's `join_url`, or `None` when the entry has none.
    pub fn url(&self) -> Option<&str> {
        self.url.as_deref()
    }

    /// Whether the entry is highlighted, or `None` when the response did not say.
    pub fn highlighted(&self) -> Option<bool> {
        self.highlighted
    }

    /// Whether participants will be notified, once [`set_notify`](Self::set_notify) has
    /// been called.
    pub fn notify(&self) -> Option<bool> {
        self.notify
    }

    /// Replaces the participants. `vec![]` removes everyone.
    pub fn set_participant_ids(&mut self, participant_ids: Vec<i64>) {
        self.participant_ids = Some(participant_ids);
        self.addressed.participant_ids = true;
    }

    /// Sets the join link. `""` removes it.
    pub fn set_url(&mut self, url: impl Into<String>) {
        self.url = Some(url.into());
        self.addressed.url = true;
    }

    /// Sets the highlight. `false` removes it.
    pub fn set_highlighted(&mut self, highlighted: bool) {
        self.highlighted = Some(highlighted);
        self.addressed.highlighted = true;
    }

    /// Asks the server to notify participants about this write.
    pub fn set_notify(&mut self, notify: bool) {
        self.notify = Some(notify);
        self.addressed.notify = true;
    }

    /// `summary`, `starts_at` and `ends_at` are required on the response and BC3 never
    /// renders them blank — `Schedule::Entry#summary` falls back to "Untitled" and the
    /// bounds are NOT NULL columns — so a blank one in a 2xx body is malformed rather than
    /// empty: resending it would blank the real value on a call that touched something
    /// else. `all_day` is taken as decoded, `false` included.
    fn from_entry(entry: &ScheduleEntry) -> Result<ScheduleEntryFields, Error> {
        require_rendered(&entry.summary, "summary")?;
        require_rendered(entry.starts_at.as_str(), "starts_at")?;
        require_rendered(entry.ends_at.as_str(), "ends_at")?;
        Ok(ScheduleEntryFields {
            summary: entry.summary.clone(),
            description: entry.description.clone().unwrap_or_default(),
            all_day: entry.all_day,
            starts_at: entry.starts_at.clone(),
            ends_at: entry.ends_at.clone(),
            participant_ids: entry
                .participants
                .as_ref()
                .map(|people| people.iter().map(|person| person.id).collect()),
            url: entry.join_url.clone(),
            highlighted: entry.highlighted,
            notify: None,
            addressed: Addressed::default(),
        })
    }

    /// The carve-outs go through the same setters a closure would call, so "the caller
    /// addressed this" has one implementation.
    fn apply(&mut self, request: &UpdateScheduleEntryRequest) {
        if let Some(summary) = &request.summary {
            self.summary.clone_from(summary);
        }
        if let Some(description) = &request.description {
            self.description.clone_from(description);
        }
        if let Some(all_day) = request.all_day {
            self.all_day = all_day;
        }
        if let Some(starts_at) = &request.starts_at {
            self.starts_at.clone_from(starts_at);
        }
        if let Some(ends_at) = &request.ends_at {
            self.ends_at.clone_from(ends_at);
        }
        if let Some(participant_ids) = &request.participant_ids {
            self.set_participant_ids(participant_ids.clone());
        }
        if let Some(url) = &request.url {
            self.set_url(url.clone());
        }
        if let Some(highlighted) = request.highlighted {
            self.set_highlighted(highlighted);
        }
        if let Some(notify) = request.notify {
            self.set_notify(notify);
        }
    }

    /// The five replaced fields always, empties included: `""` is how a clear is spelled
    /// on a full-replace endpoint, never `null` (SPEC §18) and never omission. A carve-out
    /// is passed only when addressed; unaddressed, it is `None` and body compaction leaves
    /// no key at all.
    fn into_body(self) -> ReplaceScheduleEntryRequestContent {
        let addressed = self.addressed;
        ReplaceScheduleEntryRequestContent {
            summary: Some(self.summary),
            starts_at: self.starts_at.0,
            ends_at: self.ends_at.0,
            description: Some(self.description),
            participant_ids: addressed
                .participant_ids
                .then(|| self.participant_ids.unwrap_or_default()),
            all_day: Some(self.all_day),
            notify: addressed.notify.then_some(self.notify).flatten(),
            url: addressed.url.then(|| self.url.unwrap_or_default()),
            highlighted: addressed.highlighted.then_some(self.highlighted).flatten(),
        }
    }
}

fn require_rendered(value: &str, key: &str) -> Result<(), Error> {
    if value.trim().is_empty() {
        Err(Error::malformed_response(format!(
            "GetScheduleEntry returned a schedule entry with a blank \"{key}\", which the API \
             never renders"
        ))
        .with_hint(MALFORMED_HINT))
    } else {
        Ok(())
    }
}

const MALFORMED_HINT: &str = "The merge-safe update_entry and edit_entry resend this \
                              record's fields verbatim, so a malformed response cannot be \
                              written back safely. Use replace_entry to write the record \
                              deliberately.";

impl SchedulesService<'_> {
    /// Sets the given fields on a schedule entry and preserves the rest: GETs the current
    /// entry, resends its five replaced fields with the `Some` members of `request`
    /// overlaid, adds any carve-out the request addressed, and PUTs through
    /// [`SchedulesService::replace_entry`].
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn update_entry(
        &self,
        entry_id: i64,
        request: &UpdateScheduleEntryRequest,
    ) -> Result<ScheduleEntry, Error> {
        let mut fields = ScheduleEntryFields::from_entry(&self.get_entry(entry_id).await?)?;
        fields.apply(request);
        self.replace_entry(entry_id, &fields.into_body()).await
    }

    /// Applies a read-modify-write closure to a schedule entry: GETs the current entry,
    /// hands the closure its writable state, and PUTs it back through
    /// [`SchedulesService::replace_entry`]. An error from the closure aborts before
    /// anything is written.
    ///
    /// ```no_run
    /// # async fn example(account: basecamp_sdk::AccountClient) -> Result<(), basecamp_sdk::Error> {
    /// account
    ///     .schedules()
    ///     .edit_entry(1069479523, |entry| {
    ///         entry.summary = format!("🚨 {}", entry.summary);
    ///         if entry.url().is_some_and(|url| url.starts_with("https://meet.example.com/")) {
    ///             entry.set_url("");
    ///         }
    ///         Ok(())
    ///     })
    ///     .await?;
    /// # Ok(())
    /// # }
    /// ```
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn edit_entry<F>(&self, entry_id: i64, mutate: F) -> Result<ScheduleEntry, Error>
    where
        F: FnOnce(&mut ScheduleEntryFields) -> Result<(), Error>,
    {
        let mut fields = ScheduleEntryFields::from_entry(&self.get_entry(entry_id).await?)?;
        mutate(&mut fields)?;
        self.replace_entry(entry_id, &fields.into_body()).await
    }
}
