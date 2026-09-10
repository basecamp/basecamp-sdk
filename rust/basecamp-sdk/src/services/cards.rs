//! Cards: the generated wire methods plus the merge-safe `update` of SPEC §5 "Merge-Safe
//! Write Surface (Cards)".
//!
//! `PUT /card_tables/cards/{cardId}` merges: BC3 builds the update from the JSON params
//! as they arrive, so a field the body omits is left alone and only a field that is
//! present changes. That makes a sparse PUT safe, and [`CardsService::update`] sends one
//! carrying exactly what the caller addressed — no read before the write, nothing echoed
//! back. Assignees especially: BC3 screens incoming ids through the board's reachable
//! people, so resending a list read from the card would silently unassign anyone who has
//! since lost access.
//!
//! Clearing the due date is something to say, not to leave unsaid. An absent `due_on`
//! means "leave unchanged", so the clear is spelled `"due_on": ""`, which BC3 casts to
//! nil; an explicit `null` is never sent (SPEC §18 body compaction).

pub use crate::generated::services::cards::{CardsService, ListCardsParams};

use crate::error::Error;
use crate::generated::routes;
use crate::generated::types::{Card, UpdateCardRequestContent};
use crate::services::DateChange;

/// The fields a merge-safe [`CardsService::update`] may set. A `None` member stays off
/// the wire and the server leaves it unchanged; a `Some` member is sent as given, so
/// `Some(String::new())` and `Some(vec![])` clear.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct UpdateCardRequest {
    /// The title.
    pub title: Option<String>,
    /// Rich text content (HTML).
    pub content: Option<String>,
    /// The due date: `None` leaves it alone, `Some(DateChange::Clear)` sends `""` to
    /// remove it, `Some(DateChange::On(date))` sets it.
    pub due_on: Option<DateChange>,
    /// The complete list of assigned person ids.
    pub assignee_ids: Option<Vec<i64>>,
}

impl UpdateCardRequest {
    fn body(&self) -> UpdateCardRequestContent {
        UpdateCardRequestContent {
            title: self.title.clone(),
            content: self.content.clone(),
            due_on: match self.due_on {
                Some(DateChange::On(date)) => Some(date),
                Some(DateChange::Clear) | None => None,
            },
            assignee_ids: self.assignee_ids.clone(),
        }
    }
}

impl CardsService<'_> {
    /// Updates a card, touching only the fields the caller named, in one PUT with no
    /// preceding read.
    ///
    /// A due-date clear is the one body the generated request type cannot spell — its
    /// `due_on` is a typed date, and `""` is not one — so that case takes the SPEC §18
    /// carve-out: the generated request is serialised, `"due_on": ""` is set on it, and
    /// the body goes out on the generated `UpdateCard` route, where hooks and retry still
    /// see the operation under its own identity. Every other request goes through
    /// [`CardsService::update_verbatim`] unchanged.
    pub async fn update(&self, card_id: i64, request: &UpdateCardRequest) -> Result<Card, Error> {
        let body = request.body();
        match request.due_on {
            Some(DateChange::Clear) => {
                let mut value = serde_json::to_value(&body).map_err(|error| {
                    Error::usage(format!("request body could not be serialized: {error}"))
                })?;
                if let serde_json::Value::Object(members) = &mut value {
                    members.insert(
                        "due_on".to_string(),
                        serde_json::Value::String(String::new()),
                    );
                }
                let mut operation = self.client().operation(&routes::UPDATE_CARD, &[&card_id]);
                operation.json(&value)?;
                self.client().send(operation).await
            }
            _ => self.update_verbatim(card_id, &body).await,
        }
    }
}
