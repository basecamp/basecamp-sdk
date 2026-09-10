//! Todolists: the generated wire methods plus the merge-safe `update` and
//! read-modify-write `edit` of SPEC §5 "Merge-Safe Write Surface (Todolists)".
//!
//! `PUT /todolists/{id}` is a full replace: BC3 builds a brand-new `Todolist` from the
//! permitted params and swaps it in, so a body that omits `description` erases it. The
//! composites GET the current list, change what the caller asked for, and PUT
//! `{name, description}` back through [`TodolistsService::replace`], so hooks observe
//! `GetTodolistOrGroup` then `UpdateTodolistOrGroup` under their own identities.
//!
//! The route is polymorphic and the composites are variant-agnostic: a group is a
//! `Todolist` whose parent is another list, rendered through the same partial, and its
//! `{name, description}` are preserved exactly as a list's — nothing here branches on
//! `groups_url` or `group_position_url`.
//!
//! Neither composite is atomic: a concurrent write between the GET and the PUT is
//! overwritten, last write wins, with a window of one round-trip. Use `replace` to
//! overwrite deliberately.

pub use crate::generated::services::todolists::{ListTodolistsParams, TodolistsService};

use crate::error::Error;
use crate::generated::types::{Todolist, UpdateTodolistOrGroupRequestContent};

/// The fields a merge-safe [`TodolistsService::update`] may set. A `None` member is left
/// untouched, guaranteed; `Some(String::new())` clears.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct UpdateTodolistRequest {
    /// The name. Presence-validated server-side, so an empty one is a 422.
    pub name: Option<String>,
    /// Rich text description (HTML).
    pub description: Option<String>,
}

/// A todolist's full writable state, handed to the [`TodolistsService::edit`] closure.
/// The whole value is PUT back, so clearing the description means setting it `""`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TodolistFields {
    /// The name. Presence-validated server-side, so an empty one is a 422 rather than a
    /// clear.
    pub name: String,
    /// Rich text description (HTML). Set `""` to clear.
    pub description: String,
}

impl TodolistFields {
    /// `name` is presence-validated server-side, so a 2xx body without one is malformed
    /// rather than empty: resending it would blank a real name on a call that only
    /// touched the description.
    fn from_todolist(todolist: &Todolist) -> Result<TodolistFields, Error> {
        if todolist.name.trim().is_empty() {
            return Err(Error::malformed_response(
                "GetTodolistOrGroup returned a todolist with a blank \"name\", which the API \
                 never renders",
            )
            .with_hint(MALFORMED_HINT));
        }
        Ok(TodolistFields {
            name: todolist.name.clone(),
            description: todolist.description.clone(),
        })
    }

    fn apply(&mut self, request: &UpdateTodolistRequest) {
        if let Some(name) = &request.name {
            self.name.clone_from(name);
        }
        if let Some(description) = &request.description {
            self.description.clone_from(description);
        }
    }

    /// Both fields always, the description included when empty: `""` is how a clear is
    /// spelled on a full-replace endpoint, never `null` (SPEC §18) and never omission.
    fn into_body(self) -> UpdateTodolistOrGroupRequestContent {
        UpdateTodolistOrGroupRequestContent {
            name: self.name,
            description: Some(self.description),
        }
    }
}

const MALFORMED_HINT: &str = "The merge-safe update and edit resend this record's fields \
                              verbatim, so a malformed response cannot be written back \
                              safely. Use replace to write the record deliberately.";

impl TodolistsService<'_> {
    /// Sets the given fields on a todolist or group and preserves the rest: GETs the
    /// current record, overlays the `Some` members of `request`, and PUTs
    /// `{name, description}` back through [`TodolistsService::replace`].
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn update(
        &self,
        id: i64,
        request: &UpdateTodolistRequest,
    ) -> Result<Todolist, Error> {
        let mut fields = TodolistFields::from_todolist(&self.get(id).await?)?;
        fields.apply(request);
        self.replace(id, &fields.into_body()).await
    }

    /// Applies a read-modify-write closure to a todolist or group: GETs the current
    /// record, hands the closure its writable state, and PUTs it back through
    /// [`TodolistsService::replace`]. An error from the closure aborts before anything is
    /// written.
    ///
    /// ```no_run
    /// # async fn example(account: basecamp_sdk::AccountClient) -> Result<(), basecamp_sdk::Error> {
    /// account
    ///     .todolists()
    ///     .edit(2, |list| {
    ///         list.description = String::new();
    ///         Ok(())
    ///     })
    ///     .await?;
    /// # Ok(())
    /// # }
    /// ```
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn edit<F>(&self, id: i64, mutate: F) -> Result<Todolist, Error>
    where
        F: FnOnce(&mut TodolistFields) -> Result<(), Error>,
    {
        let mut fields = TodolistFields::from_todolist(&self.get(id).await?)?;
        mutate(&mut fields)?;
        self.replace(id, &fields.into_body()).await
    }
}
