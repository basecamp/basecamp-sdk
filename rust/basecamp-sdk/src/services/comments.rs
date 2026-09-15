//! Comments: the generated wire methods plus the SPEC §18 mention composites —
//! [`CommentsService::expand_mentions`] and [`CommentsService::create_with_mentions`].
//!
//! [`CommentsService::expand_mentions`] READS only: it resolves each requested id to its
//! `attachable_sgid` through the generated people read, then has [`with_mentions`] render
//! the markup. It posts nothing, and is public so a caller can build content for a Campfire
//! line or any other rich-text field rather than only a comment.
//! [`CommentsService::create_with_mentions`] is that followed by the generated comment
//! create.
//!
//! Nothing here touches the wire on its own, and hooks see `GetPerson` and `CreateComment`
//! under their own names (SPEC §18 rule 3).

pub use crate::generated::services::comments::{CommentsService, ListCommentsParams};

use std::collections::HashSet;

use crate::error::Error;
use crate::generated::types::{Comment, CreateCommentRequestContent, Person};
use crate::mentions::with_mentions;

impl CommentsService<'_> {
    /// Content that mentions each of the given people, for posting as a comment — or, since
    /// the markup is the same, as a rich-text Campfire line.
    ///
    /// Every requested id is read through `people().get` for its `attachable_sgid` — one
    /// read per distinct id, always: an sgid already in the content is unsigned and cannot
    /// prove the person is mentioned, so it never stands in for the read — and the mentions
    /// are placed as [`with_mentions`] places them, which adds nothing for a person whose
    /// exact `attachable_sgid` the content already carries. A person read that fails — an id
    /// that is not a person in this account, a 403 — fails the expansion; nothing is posted
    /// on a partial mention list.
    ///
    /// The rendered mentions round-trip:
    /// [`mentioned_person_ids`](crate::mentions::mentioned_person_ids) on the returned
    /// content reports every id passed here, and
    /// [`RecordingsService::summarize`](crate::services::recordings::RecordingsService::summarize)
    /// reports them on the comment once posted.
    pub async fn expand_mentions(
        &self,
        content: &str,
        person_ids: &[i64],
    ) -> Result<String, Error> {
        if person_ids.is_empty() {
            return Ok(content.to_string());
        }
        let mut people: Vec<Person> = Vec::with_capacity(person_ids.len());
        let mut seen = HashSet::new();
        for id in person_ids {
            if *id <= 0 {
                return Err(Error::usage(format!("invalid mention person id {id}")));
            }
            if !seen.insert(*id) {
                continue;
            }
            // Context only: the generated read's error travels whole — status, hint,
            // request id, the timeout and deadline flags — so a caller still classifies it
            // through the ordinary accessors.
            let person = self.client().people().get(*id).await.map_err(|error| {
                error.with_context(format!("resolving mention for person {id}"))
            })?;
            people.push(person);
        }
        with_mentions(content, &people)
    }

    /// Creates a comment on a recording whose content mentions the given people:
    /// [`expand_mentions`](CommentsService::expand_mentions), then the generated create. The
    /// mention reads happen before the write, so a failed lookup posts nothing.
    pub async fn create_with_mentions(
        &self,
        recording_id: i64,
        content: &str,
        person_ids: &[i64],
    ) -> Result<Comment, Error> {
        if content.is_empty() {
            return Err(Error::usage("comment content is required"));
        }
        let expanded = self.expand_mentions(content, person_ids).await?;
        self.create(
            recording_id,
            &CreateCommentRequestContent { content: expanded },
        )
        .await
    }
}
