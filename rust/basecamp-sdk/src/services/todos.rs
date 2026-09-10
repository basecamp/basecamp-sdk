//! Todos: the generated wire methods plus the merge-safe `update` and read-modify-write
//! `edit` of SPEC §5 "Merge-Safe Write Surface (Todos)".
//!
//! `PUT /todos/{todoId}` is a full replace: BC3 clears every writable field the body omits.
//! The composites GET the current todo, change what the caller asked for, and PUT the
//! whole representation back through [`TodosService::replace`], so hooks observe `GetTodo`
//! then `ReplaceTodo` under their own identities.
//!
//! Neither composite is atomic. There is no conditional-update signal on the endpoint, so
//! a concurrent write between the GET and the PUT is overwritten — last write wins for the
//! whole representation, with a window of one round-trip. Use `replace` to overwrite
//! deliberately.

pub use crate::generated::services::todos::{ListTodosParams, TodosService};

use crate::error::Error;
use crate::generated::types::{ReplaceTodoRequestContent, Todo};
use crate::services::DateChange;
use crate::types::Date;

/// The fields a merge-safe [`TodosService::update`] may set. A `None` member is left
/// untouched on the todo, guaranteed; a `Some` member is written, so `Some(String::new())`
/// and `Some(vec![])` clear.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct UpdateTodoRequest {
    /// Text content. The server rejects an empty one.
    pub content: Option<String>,
    /// Rich text description (HTML).
    pub description: Option<String>,
    /// The complete list of assigned person ids.
    pub assignee_ids: Option<Vec<i64>>,
    /// The complete list of person ids notified on completion.
    pub completion_subscriber_ids: Option<Vec<i64>>,
    /// The due date.
    pub due_on: Option<DateChange>,
    /// The start date.
    pub starts_on: Option<DateChange>,
    /// Whether to notify assignees about this write. A send directive, not todo state.
    pub notify: Option<bool>,
}

/// A todo's full writable state, handed to the [`TodosService::edit`] closure. The whole
/// value is PUT back, so clearing a field means setting it empty — `""`, `vec![]` or
/// `None` — there is no third state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TodoFields {
    /// Text content. The server rejects an empty one.
    pub content: String,
    /// Rich text description (HTML). Set `""` to clear.
    pub description: String,
    /// The complete list of assigned person ids. Set `vec![]` to clear.
    pub assignee_ids: Vec<i64>,
    /// The complete list of person ids notified on completion. Set `vec![]` to clear.
    pub completion_subscriber_ids: Vec<i64>,
    /// The due date. Set `None` to clear.
    pub due_on: Option<Date>,
    /// The start date. Set `None` to clear.
    pub starts_on: Option<Date>,
    /// Whether to notify assignees about this write. A send directive, never read from
    /// the todo and sent only when `true`.
    pub notify: bool,
}

impl TodoFields {
    fn from_todo(todo: &Todo) -> TodoFields {
        TodoFields {
            content: todo.content.clone(),
            description: todo.description.clone().unwrap_or_default(),
            assignee_ids: ids_of(todo.assignees.as_deref()),
            completion_subscriber_ids: ids_of(todo.completion_subscribers.as_deref()),
            due_on: todo.due_on,
            starts_on: todo.starts_on,
            notify: false,
        }
    }

    fn apply(&mut self, request: &UpdateTodoRequest) {
        if let Some(content) = &request.content {
            self.content.clone_from(content);
        }
        if let Some(description) = &request.description {
            self.description.clone_from(description);
        }
        if let Some(assignee_ids) = &request.assignee_ids {
            self.assignee_ids.clone_from(assignee_ids);
        }
        if let Some(completion_subscriber_ids) = &request.completion_subscriber_ids {
            self.completion_subscriber_ids
                .clone_from(completion_subscriber_ids);
        }
        if let Some(due_on) = request.due_on {
            self.due_on = date_of(due_on);
        }
        if let Some(starts_on) = request.starts_on {
            self.starts_on = date_of(starts_on);
        }
        if let Some(notify) = request.notify {
            self.notify = notify;
        }
    }

    /// The full-state body: content, description and both id lists always, empties
    /// included so clears survive; dates only when set, because the server clears an
    /// omitted date; `notify` only when true.
    fn into_body(self) -> ReplaceTodoRequestContent {
        ReplaceTodoRequestContent {
            content: self.content,
            description: Some(self.description),
            assignee_ids: Some(self.assignee_ids),
            completion_subscriber_ids: Some(self.completion_subscriber_ids),
            notify: self.notify.then_some(true),
            due_on: self.due_on,
            starts_on: self.starts_on,
        }
    }
}

fn ids_of(people: Option<&[crate::generated::types::Person]>) -> Vec<i64> {
    people
        .unwrap_or_default()
        .iter()
        .map(|person| person.id)
        .collect()
}

fn date_of(change: DateChange) -> Option<Date> {
    match change {
        DateChange::Clear => None,
        DateChange::On(date) => Some(date),
    }
}

impl TodosService<'_> {
    /// Sets the given fields on a todo and preserves everything else: GETs the current
    /// todo, overlays the `Some` members of `request`, and PUTs the full representation
    /// back through [`TodosService::replace`].
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn update(&self, todo_id: i64, request: &UpdateTodoRequest) -> Result<Todo, Error> {
        let mut fields = TodoFields::from_todo(&self.get(todo_id).await?);
        fields.apply(request);
        self.replace(todo_id, &fields.into_body()).await
    }

    /// Applies a read-modify-write closure to a todo: GETs the current todo, hands the
    /// closure its full writable state, and PUTs the whole thing back through
    /// [`TodosService::replace`]. An error from the closure aborts before anything is
    /// written.
    ///
    /// ```no_run
    /// # async fn example(account: basecamp_sdk::AccountClient) -> Result<(), basecamp_sdk::Error> {
    /// account
    ///     .todos()
    ///     .edit(456, |todo| {
    ///         todo.content = format!("🚨 {}", todo.content);
    ///         todo.due_on = None;
    ///         Ok(())
    ///     })
    ///     .await?;
    /// # Ok(())
    /// # }
    /// ```
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn edit<F>(&self, todo_id: i64, mutate: F) -> Result<Todo, Error>
    where
        F: FnOnce(&mut TodoFields) -> Result<(), Error>,
    {
        let mut fields = TodoFields::from_todo(&self.get(todo_id).await?);
        mutate(&mut fields)?;
        self.replace(todo_id, &fields.into_body()).await
    }
}
