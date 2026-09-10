//! Builds the client a case asks for and dispatches its operation — one explicit arm per
//! operation the fixtures name, so an operation this runner does not know is refused
//! loudly rather than dispatched by reflection.
//!
//! Where a fixture reads `responseBody`, the arm answers a FLAT summary of the decoded
//! model (the Go runner's shape): a `responseBody` path resolves as a top-level key in
//! every runner, so nested paths and array indexes are not portable. The summaries are
//! built from the decoded models rather than the wire body, which is where a decode that
//! dropped a member would show.

use std::collections::BTreeMap;
use std::time::Duration;

use basecamp_sdk::models::*;
use basecamp_sdk::pagination::PageItems;
use basecamp_sdk::services::cards::UpdateCardRequest;
use basecamp_sdk::services::documents::UpdateDocumentRequest;
use basecamp_sdk::services::schedules::UpdateScheduleEntryRequest;
use basecamp_sdk::services::todolists::UpdateTodolistRequest;
use basecamp_sdk::services::todos::UpdateTodoRequest;
use basecamp_sdk::services::{
    DateChange, bookmarks, drafts, projects, reports, search, timeline, timesheets,
    todolist_groups, todos,
};
use basecamp_sdk::{AccountClient, Client, Config, Date, Error, FlexibleTime, ListResult, Page};
use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::{Value, json};

use crate::fixtures::{
    Params, TestCase, int64_param, optional_bool_param, optional_int64_list_param,
    optional_string_list_param, optional_string_param, string_param,
};
use crate::transport::ScriptedTransport;

const CONFORMANCE_TOKEN: &str = "conformance-test-token";
/// Default account for conformance cases.
const ACCOUNT_ID: &str = "999";

/// The date window every `GetUpcomingSchedule` case is dispatched with. Fixed here rather
/// than read from the case because no mock runner consumes queryParams and no assertion
/// type can pin a query string.
const UPCOMING_WINDOW_START: &str = "2026-06-01";
const UPCOMING_WINDOW_END: &str = "2026-06-30";

/// The query every Search case is dispatched with, fixed for the same reason. It is
/// required (an empty q is a client-side usage error), and the mock answers its queued
/// body regardless of what is asked for.
const SEARCH_QUERY: &str = "Leto";

/// What an operation answered, in a shape the assertions can read.
///
/// `List` carries the pagination metadata a list result reports.
pub enum Outcome {
    Unit,
    Json(Value),
    List {
        value: Value,
        meta: BTreeMap<String, Value>,
    },
}

impl Outcome {
    pub fn body(&self) -> Option<&Value> {
        match self {
            Outcome::Unit => None,
            Outcome::Json(value) | Outcome::List { value, .. } => Some(value),
        }
    }

    pub fn meta(&self) -> Option<&BTreeMap<String, Value>> {
        match self {
            Outcome::List { meta, .. } => Some(meta),
            _ => None,
        }
    }
}

/// A failure of the harness itself — an operation this runner has no arm for, a fixture
/// value it cannot express — as distinct from what the SDK answered. It is reported as a
/// failed case, never as an SDK error an `errorRaised` or `errorCode` assertion could
/// accept.
#[derive(Debug)]
pub struct Harness(pub String);

pub async fn execute_case(
    case: &TestCase,
    transport: ScriptedTransport,
) -> Result<Result<Outcome, Error>, Harness> {
    let account = match client_for(case, transport) {
        Ok(account) => account,
        Err(error) => return Ok(Err(error)),
    };
    // Harness failures travel as usage errors tagged `harness:` so the arms can use `?`
    // uniformly; they are separated from SDK errors here, before any assertion runs.
    match dispatch(&account, case).await {
        Err(error)
            if error.code() == basecamp_sdk::ErrorCode::Usage
                && error.message().starts_with("harness:") =>
        {
            Err(Harness(error.message().to_string()))
        }
        other => Ok(other),
    }
}

fn client_for(case: &TestCase, transport: ScriptedTransport) -> Result<AccountClient, Error> {
    let overrides = &case.config_overrides;
    let mut config = Config::default()
        .with_timeout(Duration::from_secs(10))
        .with_max_retries(overrides.max_retries.unwrap_or(3));
    if let Some(base_url) = &overrides.base_url {
        config = config.with_base_url(base_url);
    }
    if let Some(max_pages) = overrides.max_pages {
        config.max_pages = usize::try_from(max_pages).unwrap_or(usize::MAX);
    }
    let client = Client::builder(config)
        .access_token(CONFORMANCE_TOKEN)
        .http_client(transport)
        .build()?;
    Ok(client.for_account(ACCOUNT_ID))
}

/// The `max_items` cap a case asks for, threaded to `collect_all`.
fn max_items(case: &TestCase) -> Option<usize> {
    case.config_overrides
        .max_items
        .map(|n| usize::try_from(n).unwrap_or(usize::MAX))
}

/// The pinned page a case asks for, as the generated params spell it. SPEC §8 pins on a
/// POSITIVE page only; zero is not a selection.
fn page_param(case: &TestCase) -> Option<i32> {
    case.config_overrides
        .page
        .filter(|page| *page > 0)
        .and_then(|page| i32::try_from(page).ok())
}

#[allow(clippy::unnecessary_wraps)] // every arm is a `Result`, and `json(x.await?)` reads as one
fn json<T: Serialize>(value: T) -> Result<Outcome, Error> {
    Ok(decoded(value))
}

fn decoded<T: Serialize>(value: T) -> Outcome {
    Outcome::Json(serde_json::to_value(value).unwrap_or(Value::Null))
}

fn unit<T>(result: Result<T, Error>) -> Result<Outcome, Error> {
    result.map(|_| Outcome::Unit)
}

fn list_meta<T>(result: &ListResult<T>) -> BTreeMap<String, Value> {
    BTreeMap::from([
        ("totalCount".to_string(), json!(result.meta.total_count)),
        ("truncated".to_string(), json!(result.meta.truncated)),
    ])
}

/// Walks every page of a list read and answers its metadata, with the items as the body.
async fn list<P>(
    account: &AccountClient,
    case: &TestCase,
    first: Result<Page<P>, Error>,
) -> Result<Outcome, Error>
where
    P: PageItems + DeserializeOwned,
    P::Item: Serialize,
{
    list_with(account, case, first, |items| {
        Ok(serde_json::to_value(items).unwrap_or(Value::Null))
    })
    .await
}

/// Like [`list`], but the body is a summary computed from the decoded items. A case that
/// pins a `page` (SPEC §8) gets the SDK's own answer: `collect_all` on a pinned page
/// follows nothing and reports the cursor it did not follow as `truncated`.
async fn list_with<P: PageItems + DeserializeOwned>(
    account: &AccountClient,
    case: &TestCase,
    first: Result<Page<P>, Error>,
    summarize: impl FnOnce(&[P::Item]) -> Result<Value, Error>,
) -> Result<Outcome, Error> {
    let result = account.collect_all(first?, max_items(case)).await?;
    Ok(Outcome::List {
        meta: list_meta(&result),
        value: summarize(&result.items)?,
    })
}

fn harness(message: impl std::fmt::Display) -> Error {
    Error::usage(format!("harness: {message}"))
}

async fn dispatch(account: &AccountClient, case: &TestCase) -> Result<Outcome, Error> {
    let path = &case.path_params;
    let body = &case.request_body;
    let id = |key: &str| int64_param(path, key);

    match case.operation.as_str() {
        // --- projects / templates ------------------------------------------------------
        "ListProjects" => {
            let params = projects::ListProjectsParams {
                page: page_param(case),
                ..Default::default()
            };
            list_with(
                account,
                case,
                account.projects().list(&params).await,
                |projects| Ok(summarize_projects(projects)),
            )
            .await
        }
        "GetProject" => json(account.projects().get(id("projectId")).await?),
        "ListRecentProjects" => {
            let projects = account.projects().list_recent_projects().await?;
            Ok(Outcome::Json(summarize_projects(&projects)))
        }
        "RecordProjectVisit" => unit(
            account
                .projects()
                .record_project_visit(id("projectId"))
                .await,
        ),
        "CreateProject" => {
            let mut name = string_param(body, "name");
            if name.is_empty() {
                name = "Conformance Test".to_string();
            }
            let request = CreateProjectRequestContent {
                name,
                ..Default::default()
            };
            unit(account.projects().create(&request).await)
        }
        "UpdateProject" => {
            let mut name = string_param(body, "name");
            if name.is_empty() {
                name = "Conformance Test".to_string();
            }
            let request = UpdateProjectRequestContent {
                name,
                ..Default::default()
            };
            unit(account.projects().update(id("projectId"), &request).await)
        }
        "TrashProject" => unit(account.projects().trash(id("projectId")).await),
        "GetTemplateLibrary" => {
            let library = account.templates().get_library().await?;
            Ok(Outcome::Json(summarize_template_library(&library)))
        }
        "CreateTemplateLibraryCopy" => {
            let request = CreateTemplateLibraryCopyRequestContent {
                template_recording_id: exact_int64(body, "template_recording_id")?,
                destination_parent_id: exact_int64(body, "destination_parent_id")?,
                adding_people_confirmed: optional_bool_param(body, "adding_people_confirmed"),
            };
            let copy = account.templates().create_library_copy(&request).await?;
            Ok(Outcome::Json(summarize_template_library_copy(&copy)))
        }
        "CreateProjectFromTemplate" => {
            let request = CreateProjectFromTemplateRequestContent {
                project: ProjectConstructionAttributes {
                    name: string_param(body, "name"),
                    description: optional_string_param(body, "description"),
                    start_date: optional_string_param(body, "start_date"),
                    ..Default::default()
                },
            };
            let construction = account
                .templates()
                .create_project(exact_int64(path, "templateId")?, &request)
                .await?;
            Ok(Outcome::Json(json!({
                "id": construction.id,
                "status": construction.status,
            })))
        }
        "GetTemplateLibraryCopy" => {
            let copy = account
                .templates()
                .get_library_copy(exact_int64(path, "copyId")?)
                .await?;
            Ok(Outcome::Json(summarize_template_library_copy(&copy)))
        }

        // --- todos / todolists ---------------------------------------------------------
        "ListTodos" => {
            let params = todos::ListTodosParams::default();
            list(
                account,
                case,
                account.todos().list(id("todolistId"), &params).await,
            )
            .await
        }
        "GetTodo" => unit(account.todos().get(id("todoId")).await),
        "CreateTodo" => {
            let mut content = string_param(body, "content");
            if content.is_empty() {
                content = "Conformance Test".to_string();
            }
            let request = CreateTodoRequestContent {
                content,
                ..Default::default()
            };
            unit(account.todos().create(id("todolistId"), &request).await)
        }
        "CreateTodosetTodo" => {
            let mut content = string_param(body, "content");
            if content.is_empty() {
                content = "Conformance Test".to_string();
            }
            let request = CreateTodosetTodoRequestContent {
                content,
                ..Default::default()
            };
            unit(
                account
                    .todos()
                    .create_todoset_todo(id("bucketId"), id("todosetId"), &request)
                    .await,
            )
        }
        "CompleteTodo" => unit(account.todos().complete(id("todoId")).await),
        "ReplaceTodo" => {
            let request = ReplaceTodoRequestContent {
                content: string_param(body, "content"),
                description: optional_string_param(body, "description"),
                assignee_ids: optional_int64_list_param(body, "assignee_ids"),
                completion_subscriber_ids: optional_int64_list_param(
                    body,
                    "completion_subscriber_ids",
                ),
                due_on: date_param(body, "due_on")?,
                starts_on: date_param(body, "starts_on")?,
                notify: optional_bool_param(body, "notify"),
                ..Default::default()
            };
            unit(account.todos().replace(id("todoId"), &request).await)
        }
        "GetTodolistOrGroup" => json(account.todolists().get(id("id")).await?),
        "ReplaceTodolist" => {
            let request = UpdateTodolistOrGroupRequestContent {
                name: string_param(body, "name"),
                description: optional_string_param(body, "description"),
                ..Default::default()
            };
            unit(account.todolists().replace(id("id"), &request).await)
        }
        "ListTodolistGroups" => {
            let params = todolist_groups::ListTodolistGroupsParams {
                page: page_param(case),
            };
            list_with(
                account,
                case,
                account.todolist_groups().list(id("todolistId"), &params).await,
                |groups| {
                    // Not a pass on an empty list: responseBody assertions read the first
                    // decoded element, so nothing would be compared.
                    groups.first().map_or_else(
                        || {
                            Err(harness(
                                "ListTodolistGroups decoded 0 groups; responseBody assertions read the first element, so there is nothing to assert against",
                            ))
                        },
                        |first| Ok(serde_json::to_value(first).unwrap_or(Value::Null)),
                    )
                },
            )
            .await
        }
        "RepositionTodolistGroup" => {
            let request = RepositionTodolistGroupRequestContent {
                position: i32::try_from(int64_param(body, "position")).unwrap_or_default(),
            };
            unit(
                account
                    .todolist_groups()
                    .reposition(id("groupId"), &request)
                    .await,
            )
        }
        // --- SPEC section 18 composites (hand-written, over generated get + replace) -----
        "UpdateTodo" => {
            let request = UpdateTodoRequest {
                content: optional_string_param(body, "content"),
                description: optional_string_param(body, "description"),
                assignee_ids: optional_int64_list_param(body, "assignee_ids"),
                completion_subscriber_ids: optional_int64_list_param(
                    body,
                    "completion_subscriber_ids",
                ),
                due_on: date_change_param(body, "due_on")?,
                starts_on: date_change_param(body, "starts_on")?,
                notify: optional_bool_param(body, "notify"),
            };
            unit(account.todos().update(id("todoId"), &request).await)
        }
        "EditTodo" => {
            // The edit closure assigns each fixture key onto the same-named member; absence
            // stays absence, so an untouched field keeps its fetched value.
            let content = optional_string_param(body, "content");
            let description = optional_string_param(body, "description");
            let assignee_ids = optional_int64_list_param(body, "assignee_ids");
            let completion_subscriber_ids =
                optional_int64_list_param(body, "completion_subscriber_ids");
            let due_on = date_change_param(body, "due_on")?;
            let starts_on = date_change_param(body, "starts_on")?;
            let notify = optional_bool_param(body, "notify");
            unit(
                account
                    .todos()
                    .edit(id("todoId"), |fields| {
                        if let Some(content) = content {
                            fields.content = content;
                        }
                        if let Some(description) = description {
                            fields.description = description;
                        }
                        if let Some(ids) = assignee_ids {
                            fields.assignee_ids = ids;
                        }
                        if let Some(ids) = completion_subscriber_ids {
                            fields.completion_subscriber_ids = ids;
                        }
                        if let Some(change) = due_on {
                            fields.due_on = match change {
                                DateChange::On(date) => Some(date),
                                DateChange::Clear => None,
                            };
                        }
                        if let Some(change) = starts_on {
                            fields.starts_on = match change {
                                DateChange::On(date) => Some(date),
                                DateChange::Clear => None,
                            };
                        }
                        if let Some(notify) = notify {
                            fields.notify = notify;
                        }
                        Ok(())
                    })
                    .await,
            )
        }
        "UpdateTodolist" => {
            let request = UpdateTodolistRequest {
                name: optional_string_param(body, "name"),
                description: optional_string_param(body, "description"),
            };
            unit(account.todolists().update(id("id"), &request).await)
        }
        "EditTodolist" => {
            let name = optional_string_param(body, "name");
            let description = optional_string_param(body, "description");
            unit(
                account
                    .todolists()
                    .edit(id("id"), |fields| {
                        if let Some(name) = name {
                            fields.name = name;
                        }
                        if let Some(description) = description {
                            fields.description = description;
                        }
                        Ok(())
                    })
                    .await,
            )
        }
        "UpdateDocument" => {
            let request = UpdateDocumentRequest {
                title: optional_string_param(body, "title"),
                content: optional_string_param(body, "content"),
            };
            unit(account.documents().update(id("documentId"), &request).await)
        }
        "EditDocument" => {
            let title = optional_string_param(body, "title");
            let content = optional_string_param(body, "content");
            unit(
                account
                    .documents()
                    .edit(id("documentId"), |fields| {
                        if let Some(title) = title {
                            fields.title = title;
                        }
                        if let Some(content) = content {
                            fields.content = content;
                        }
                        Ok(())
                    })
                    .await,
            )
        }
        "UpdateScheduleEntry" => {
            let request = UpdateScheduleEntryRequest {
                summary: optional_string_param(body, "summary"),
                description: optional_string_param(body, "description"),
                all_day: optional_bool_param(body, "all_day"),
                starts_at: optional_string_param(body, "starts_at").map(FlexibleTime::from),
                ends_at: optional_string_param(body, "ends_at").map(FlexibleTime::from),
                participant_ids: optional_int64_list_param(body, "participant_ids"),
                url: optional_string_param(body, "url"),
                highlighted: optional_bool_param(body, "highlighted"),
                notify: optional_bool_param(body, "notify"),
            };
            unit(
                account
                    .schedules()
                    .update_entry(id("entryId"), &request)
                    .await,
            )
        }
        "EditScheduleEntry" => {
            // The carve-outs go through setters because assignment, not value, is what
            // marks them addressed: assigning exactly what the GET returned still sends them.
            let summary = optional_string_param(body, "summary");
            let description = optional_string_param(body, "description");
            let all_day = optional_bool_param(body, "all_day");
            let starts_at = optional_string_param(body, "starts_at");
            let ends_at = optional_string_param(body, "ends_at");
            let participant_ids = optional_int64_list_param(body, "participant_ids");
            let url = optional_string_param(body, "url");
            let highlighted = optional_bool_param(body, "highlighted");
            let notify = optional_bool_param(body, "notify");
            unit(
                account
                    .schedules()
                    .edit_entry(id("entryId"), |fields| {
                        if let Some(summary) = summary {
                            fields.summary = summary;
                        }
                        if let Some(description) = description {
                            fields.description = description;
                        }
                        if let Some(all_day) = all_day {
                            fields.all_day = all_day;
                        }
                        if let Some(starts_at) = starts_at {
                            fields.starts_at = FlexibleTime::from(starts_at);
                        }
                        if let Some(ends_at) = ends_at {
                            fields.ends_at = FlexibleTime::from(ends_at);
                        }
                        if let Some(ids) = participant_ids {
                            fields.set_participant_ids(ids);
                        }
                        if let Some(url) = url {
                            fields.set_url(url);
                        }
                        if let Some(highlighted) = highlighted {
                            fields.set_highlighted(highlighted);
                        }
                        if let Some(notify) = notify {
                            fields.set_notify(notify);
                        }
                        Ok(())
                    })
                    .await,
            )
        }
        "UpdateCard" => {
            let request = UpdateCardRequest {
                title: optional_string_param(body, "title"),
                content: optional_string_param(body, "content"),
                due_on: date_change_param(body, "due_on")?,
                assignee_ids: optional_int64_list_param(body, "assignee_ids"),
            };
            unit(account.cards().update(id("cardId"), &request).await)
        }
        "UploadsDownload" => unit(account.uploads().download(id("uploadId")).await),
        "DownloadURL" => {
            // An absolute URL the SDK accepts: it rewrites scheme and host to the configured
            // origin (SPEC §14), so only the case's path matters.
            let raw = format!("https://storage.3.basecamp.com{}", case.path);
            unit(account.download_url(&raw).await)
        }

        // --- documents / schedules / cards ---------------------------------------------
        "ReplaceDocument" => {
            let request = ReplaceDocumentRequestContent {
                title: optional_string_param(body, "title"),
                content: optional_string_param(body, "content"),
            };
            unit(
                account
                    .documents()
                    .replace(id("documentId"), &request)
                    .await,
            )
        }
        "CreateScheduleEntry" => {
            let request = CreateScheduleEntryRequestContent {
                summary: string_param(body, "summary"),
                starts_at: string_param(body, "starts_at"),
                ends_at: string_param(body, "ends_at"),
                description: optional_string_param(body, "description"),
                all_day: optional_bool_param(body, "all_day"),
                participant_ids: optional_int64_list_param(body, "participant_ids"),
                notify: optional_bool_param(body, "notify"),
                url: optional_string_param(body, "url"),
                highlighted: optional_bool_param(body, "highlighted"),
                status: optional_string_param(body, "status"),
                ..Default::default()
            };
            unit(
                account
                    .schedules()
                    .create_entry(id("scheduleId"), &request)
                    .await,
            )
        }
        "ReplaceScheduleEntry" => {
            // The raw wire method: one verbatim PUT, no read-before-write. Presence-bearing,
            // so only the keys the fixture carries reach the wire.
            let request = ReplaceScheduleEntryRequestContent {
                summary: optional_string_param(body, "summary"),
                starts_at: string_param(body, "starts_at"),
                ends_at: string_param(body, "ends_at"),
                description: optional_string_param(body, "description"),
                all_day: optional_bool_param(body, "all_day"),
                participant_ids: optional_int64_list_param(body, "participant_ids"),
                notify: optional_bool_param(body, "notify"),
                url: optional_string_param(body, "url"),
                highlighted: optional_bool_param(body, "highlighted"),
                ..Default::default()
            };
            unit(
                account
                    .schedules()
                    .replace_entry(id("entryId"), &request)
                    .await,
            )
        }
        "UpdateCardVerbatim" => {
            let request = UpdateCardRequestContent {
                title: optional_string_param(body, "title"),
                content: optional_string_param(body, "content"),
                due_on: date_param(body, "due_on")?,
                assignee_ids: optional_int64_list_param(body, "assignee_ids"),
                ..Default::default()
            };
            unit(
                account
                    .cards()
                    .update_verbatim(id("cardId"), &request)
                    .await,
            )
        }

        // --- my things -----------------------------------------------------------------
        "Subscribe" => unit(account.subscriptions().subscribe(id("recordingId")).await),
        "ListMyBookmarks" => {
            let params = bookmarks::ListMyBookmarksParams::default();
            list(
                account,
                case,
                account.bookmarks().list_my_bookmarks(&params).await,
            )
            .await
        }
        "ListMyDrafts" => {
            let params = drafts::ListMyDraftsParams::default();
            list(
                account,
                case,
                account.drafts().list_my_drafts(&params).await,
            )
            .await
        }
        "GetMyNote" => unit(account.my_notes().get_my_note().await),
        "UpdateMyNote" => {
            let content = body
                .get("note")
                .and_then(Value::as_object)
                .map(|note| string_param(note, "content"))
                .unwrap_or_default();
            let request = UpdateMyNoteRequestContent {
                note: MyNoteAttributes { content },
            };
            unit(account.my_notes().update_my_note(&request).await)
        }
        "PrioritizeAssignment" => {
            let request = PrioritizeAssignmentRequestContent {
                id: int64_param(body, "id"),
            };
            unit(
                account
                    .my_assignments()
                    .prioritize_assignment(&request)
                    .await,
            )
        }
        "DeprioritizeAssignment" => unit(
            account
                .my_assignments()
                .deprioritize_assignment(id("recordingId"))
                .await,
        ),
        "ReorderUpNext" => {
            let request = ReorderUpNextRequestContent {
                source_id: int64_param(body, "source_id"),
                position: i32::try_from(int64_param(body, "position")).unwrap_or_default(),
            };
            unit(account.my_assignments().reorder_up_next(&request).await)
        }
        "GetCalendar" => unit(account.calendars().get_calendar(id("calendarId")).await),
        "UpdateCalendar" => {
            let color = body
                .get("calendar")
                .and_then(Value::as_object)
                .map(|calendar| string_param(calendar, "color"))
                .unwrap_or_default();
            let request = UpdateCalendarRequestContent {
                calendar: CalendarAttributes { color },
            };
            unit(
                account
                    .calendars()
                    .update_calendar(id("calendarId"), &request)
                    .await,
            )
        }

        // --- people / clients ----------------------------------------------------------
        "UpdateProjectClientAccess" => {
            let create = body.get("create").and_then(Value::as_array).map(|rows| {
                rows.iter()
                    .map(|row| {
                        let row = row.as_object().cloned().unwrap_or_default();
                        CreateClientRequest {
                            email_address: string_param(&row, "email_address"),
                            name: optional_string_param(&row, "name"),
                            title: optional_string_param(&row, "title"),
                            company_name: optional_string_param(&row, "company_name"),
                        }
                    })
                    .collect()
            });
            let request = UpdateProjectClientAccessRequestContent {
                grant: optional_int64_list_param(body, "grant"),
                revoke: optional_int64_list_param(body, "revoke"),
                create,
            };
            json(
                account
                    .people()
                    .update_project_client_access(id("projectId"), &request)
                    .await?,
            )
        }
        "EnableProjectClients" => json(
            account
                .people()
                .enable_project_clients(id("projectId"))
                .await?,
        ),
        "DisableProjectClients" => json(
            account
                .people()
                .disable_project_clients(id("projectId"))
                .await?,
        ),

        // --- bookmarks / bubble ups / spotlights ---------------------------------------
        "GetBookmark" => unit(account.bookmarks().get_bookmark(id("recordingId")).await),
        "CreateBookmark" => unit(account.bookmarks().create_bookmark(id("recordingId")).await),
        "DeleteBookmark" => unit(account.bookmarks().delete_bookmark(id("recordingId")).await),
        "CreateBubbleUp" => {
            let request = CreateBubbleUpRequestContent {
                at: optional_string_param(body, "at"),
            };
            unit(
                account
                    .bubble_ups()
                    .create_bubble_up(id("recordingId"), &request)
                    .await,
            )
        }
        "DeleteBubbleUp" => unit(
            account
                .bubble_ups()
                .delete_bubble_up(id("recordingId"))
                .await,
        ),
        "SpotlightRecording" => unit(account.recordings().spotlight(id("recordingId")).await),
        "UnspotlightRecording" => unit(account.recordings().unspotlight(id("recordingId")).await),

        // --- folders -------------------------------------------------------------------
        "ListFolders" => unit(account.folders().list_folders().await),
        "GetFolder" => unit(account.folders().get_folder(id("folderId")).await),
        "CreateFolder" => {
            let request = CreateFolderRequestContent {
                name: optional_string_param(body, "name"),
                project_ids: optional_int64_list_param(body, "project_ids"),
                ..Default::default()
            };
            unit(account.folders().create_folder(&request).await)
        }
        "UpdateFolder" => {
            let request = UpdateFolderRequestContent {
                name: string_param(body, "name"),
            };
            unit(
                account
                    .folders()
                    .update_folder(id("folderId"), &request)
                    .await,
            )
        }
        "DeleteFolder" => unit(account.folders().delete_folder(id("folderId")).await),

        // --- timesheets / reports / timeline -------------------------------------------
        "GetTimesheetEntry" => unit(account.timesheets().get(id("entryId")).await),
        "DestroyTimesheetEntry" => unit(account.timesheets().destroy(id("entryId")).await),
        "UpdateTimesheetEntry" => {
            let non_empty = |key: &str| optional_string_param(body, key).filter(|v| !v.is_empty());
            let request = UpdateTimesheetEntryRequestContent {
                date: non_empty("date"),
                hours: non_empty("hours"),
                description: non_empty("description"),
                ..Default::default()
            };
            unit(account.timesheets().update(id("entryId"), &request).await)
        }
        "GetProjectTimesheet" => {
            let params = timesheets::GetProjectTimesheetParams::default();
            unit(
                account
                    .timesheets()
                    .for_project(id("projectId"), &params)
                    .await,
            )
        }
        "GetProjectTimeline" => {
            let params = timeline::GetProjectTimelineParams::default();
            list(
                account,
                case,
                account
                    .timeline()
                    .project_timeline(id("projectId"), &params)
                    .await,
            )
            .await
        }
        "GetProgressReport" => {
            let params = reports::GetProgressReportParams::default();
            list(account, case, account.reports().progress(&params).await).await
        }
        "GetPersonProgress" => {
            let params = reports::GetPersonProgressParams {
                page: page_param(case),
            };
            list(
                account,
                case,
                account
                    .reports()
                    .person_progress(id("personId"), &params)
                    .await,
            )
            .await
        }
        "GetUpcomingSchedule" => {
            let result = account
                .reports()
                .upcoming(UPCOMING_WINDOW_START, UPCOMING_WINDOW_END)
                .await?;
            Ok(Outcome::Json(summarize_upcoming(&result)))
        }

        // --- webhooks / tools ----------------------------------------------------------
        "ListWebhooks" => list(account, case, account.webhooks().list(id("bucketId")).await).await,
        "CreateWebhook" => {
            let request = CreateWebhookRequestContent {
                payload_url: string_param(body, "payload_url"),
                types: optional_string_list_param(body, "types").unwrap_or_default(),
                ..Default::default()
            };
            unit(account.webhooks().create(id("bucketId"), &request).await)
        }
        "GetTool" => unit(account.tools().get(id("toolId")).await),
        "CreateTool" => {
            let request = CreateToolRequestContent {
                tool_type: string_param(body, "tool_type"),
                title: optional_string_param(body, "title").filter(|t| !t.is_empty()),
                ..Default::default()
            };
            unit(account.tools().create(id("bucketId"), &request).await)
        }
        "EnableTool" => unit(account.tools().enable(id("toolId")).await),

        // --- uploads -------------------------------------------------------------------
        "CreateUploadVersion" => {
            // Presence-bearing: a key the fixture omits never reaches the wire, so an
            // unaddressed description carries forward while "" is sent and clears.
            let request = CreateUploadVersionRequestContent {
                attachable_sgid: string_param(body, "attachable_sgid"),
                base_name: optional_string_param(body, "base_name"),
                description: optional_string_param(body, "description"),
                notify: optional_string_param(body, "notify"),
                subscriptions: optional_int64_list_param(body, "subscriptions"),
                ..Default::default()
            };
            unit(
                account
                    .uploads()
                    .create_version(id("uploadId"), &request)
                    .await,
            )
        }
        "UpdateUpload" => {
            let request = UpdateUploadRequestContent {
                description: optional_string_param(body, "description"),
                base_name: optional_string_param(body, "base_name"),
            };
            unit(account.uploads().update(id("uploadId"), &request).await)
        }
        "ListUploadVersions" => {
            list_with(
                account,
                case,
                account.uploads().list_versions(id("uploadId")).await,
                |versions| Ok(summarize_upload_versions(versions)),
            )
            .await
        }

        // --- search / everything -------------------------------------------------------
        "Search" => {
            list_with(
                account,
                case,
                account
                    .search()
                    .search(SEARCH_QUERY, &search::SearchParams::default())
                    .await,
                |results| Ok(summarize_search(results)),
            )
            .await
        }
        "GetEverythingMessages" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_messages(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingComments" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_comments(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingCheckins" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_checkins(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingForwards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_forwards(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingFiles" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_files(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingOverdueTodos" => unit(
            account
                .everything()
                .everything_overdue_todos(&Default::default())
                .await,
        ),
        "GetEverythingOverdueCards" => unit(
            account
                .everything()
                .everything_overdue_cards(&Default::default())
                .await,
        ),
        "GetEverythingOpenTodos" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_open_todos(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingCompletedTodos" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_completed_todos(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingUnassignedTodos" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_unassigned_todos(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingNoDueDateTodos" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_no_due_date_todos(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingOpenCards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_open_cards(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingCompletedCards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_completed_cards(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingUnassignedCards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_unassigned_cards(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingNoDueDateCards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_no_due_date_cards(&Default::default())
                    .await,
            )
            .await
        }
        "GetEverythingNotNowCards" => {
            list(
                account,
                case,
                account
                    .everything()
                    .everything_not_now_cards(&Default::default())
                    .await,
            )
            .await
        }

        // --- forwards / campfires / clients (#588 bucket-scoped spellings) -------------
        "ListForwards" => {
            list(
                account,
                case,
                account
                    .forwards()
                    .list(id("inboxId"), &Default::default())
                    .await,
            )
            .await
        }
        "ListChatbots" => {
            list(
                account,
                case,
                account
                    .campfires()
                    .list_chatbots(id("bucketId"), id("campfireId"))
                    .await,
            )
            .await
        }
        "GetChatbot" => unit(
            account
                .campfires()
                .get_chatbot(id("bucketId"), id("campfireId"), id("chatbotId"))
                .await,
        ),
        "CreateChatbot" => {
            let request = CreateChatbotRequestContent {
                service_name: string_param(body, "service_name"),
                command_url: optional_string_param(body, "command_url"),
            };
            unit(
                account
                    .campfires()
                    .create_chatbot(id("bucketId"), id("campfireId"), &request)
                    .await,
            )
        }
        "UpdateChatbot" => {
            let request = UpdateChatbotRequestContent {
                service_name: string_param(body, "service_name"),
                command_url: optional_string_param(body, "command_url"),
            };
            unit(
                account
                    .campfires()
                    .update_chatbot(id("bucketId"), id("campfireId"), id("chatbotId"), &request)
                    .await,
            )
        }
        "DeleteChatbot" => unit(
            account
                .campfires()
                .delete_chatbot(id("bucketId"), id("campfireId"), id("chatbotId"))
                .await,
        ),
        "ListClientApprovals" => {
            list(
                account,
                case,
                account
                    .client_approvals()
                    .list(id("bucketId"), &Default::default())
                    .await,
            )
            .await
        }
        "ListClientCorrespondences" => {
            list(
                account,
                case,
                account
                    .client_correspondences()
                    .list(id("bucketId"), &Default::default())
                    .await,
            )
            .await
        }
        "ListClientReplies" => {
            list(
                account,
                case,
                account
                    .client_replies()
                    .list(id("bucketId"), id("recordingId"), &Default::default())
                    .await,
            )
            .await
        }
        "GetClientReply" => unit(
            account
                .client_replies()
                .get(id("bucketId"), id("recordingId"), id("replyId"))
                .await,
        ),

        other => Err(harness(format!("unknown operation: {other}"))),
    }
}

/// A date the fixture carries, as the typed request spells it. A raw replace has no way to
/// spell the `""` clear — that is the composites' carve-out (SPEC §18) — so an empty
/// string here is a fixture asking for something this arm cannot send.
fn date_param(params: &Params, key: &str) -> Result<Option<Date>, Error> {
    optional_string_param(params, key)
        .map(|text| {
            Date::parse(&text)
                .map_err(|error| harness(format!("{key}={text:?} is not a date ({error})")))
        })
        .transpose()
}

/// A date the fixture carries for a merge-safe composite: `""` is the clear, anything else
/// a date the request sets.
fn date_change_param(params: &Params, key: &str) -> Result<Option<DateChange>, Error> {
    optional_string_param(params, key)
        .map(|text| {
            if text.is_empty() {
                Ok(DateChange::Clear)
            } else {
                Date::parse(&text).map(DateChange::On)
            }
        })
        .transpose()
}
/// A required integer read without rounding: a fixture id past 2^53 must survive.
fn exact_int64(params: &Params, key: &str) -> Result<i64, Error> {
    match params.get(key) {
        None => Err(harness(format!("{key} is required"))),
        Some(value) => value
            .as_i64()
            .ok_or_else(|| harness(format!("{key} must be an integer"))),
    }
}

// --- summaries ---------------------------------------------------------------------------

fn summarize_projects(projects: &[Project]) -> Value {
    json!({
        "project_count": projects.len(),
        "first_project_id": projects.first().map_or(0, |p| p.id),
        "last_project_id": projects.last().map_or(0, |p| p.id),
    })
}

fn summarize_template_library(library: &TemplateLibrary) -> Value {
    let mut summary = json!({
        "bucket_id": library.bucket.id,
        "todoset_id": library.todoset.id,
    });
    if let Some(first) = library.todolists.first() {
        summary["first_todolist_id"] = json!(first.id);
    }
    summary
}

fn summarize_template_library_copy(copy: &TemplateLibraryCopy) -> Value {
    let mut summary = json!({ "id": copy.id, "status": copy.status });
    if let Some(todolist) = &copy.destination_todolist {
        summary["destination_todolist_id"] = json!(todolist.id);
    }
    summary
}

fn summarize_upcoming(result: &GetUpcomingScheduleResponseContent) -> Value {
    let mut summary = json!({
        "schedule_entries_count": result.schedule_entries.len(),
        "recurring_occurrences_count": result.recurring_schedule_entry_occurrences.len(),
        "assignables_count": result.assignables.len(),
    });
    if let Some(entry) = result.schedule_entries.first() {
        summary["entry_summary"] = json!(entry.summary);
        summary["entry_recurring"] = json!(entry.recurring);
        summary["entry_bucket_name"] = json!(entry.bucket.name);
    }
    if let Some(occurrence) = result.recurring_schedule_entry_occurrences.first() {
        summary["occurrence_recurring"] = json!(occurrence.recurring);
        summary["occurrence_all_day"] = json!(occurrence.all_day);
        summary["occurrence_starts_at"] = json!(
            occurrence
                .starts_at
                .date()
                .map(|d| d.to_string())
                .unwrap_or_default()
        );
    }
    if let Some(assignable) = result.assignables.first() {
        summary["assignable_content"] = json!(assignable.content);
        summary["assignable_type"] = json!(assignable.r#type);
        summary["assignable_parent_title"] = json!(assignable.parent.title);
        summary["assignable_completion_url"] = json!(assignable.completion_url);
    }
    summary
}

fn summarize_upload_versions(versions: &[UploadVersion]) -> Value {
    let current_count = versions
        .iter()
        .filter(|v| v.upload.as_ref().is_some_and(|u| u.current))
        .count();
    let mut summary = json!({
        "versions_count": versions.len(),
        "current_count": current_count,
    });
    if let (Some(first), Some(last)) = (versions.first(), versions.last()) {
        summary["first_action"] = json!(first.action);
        if let Some(upload) = &first.upload {
            summary["first_filename"] = json!(upload.filename);
            summary["first_content_type"] = json!(upload.content_type);
            summary["first_byte_size"] = json!(upload.byte_size);
            summary["first_current"] = json!(upload.current);
        }
        summary["last_action"] = json!(last.action);
        summary["last_has_upload"] = json!(last.upload.is_some());
    }
    summary
}

/// One group per branch of BC3's polymorphic search projection; see the Go runner's
/// `summarizeSearch` for why each hit is selected by predicate and reported as booleans.
fn summarize_search(results: &[SearchResult]) -> Value {
    let find = |pred: &dyn Fn(&SearchResult) -> bool| results.iter().find(|r| pred(r));
    let non_empty = |s: &Option<String>| s.as_deref().is_some_and(|s| !s.is_empty());
    let text = |s: &Option<String>| s.clone().unwrap_or_default();
    let int = |n: Option<i32>| n.unwrap_or_default();

    let mut summary = json!({
        "result_count": results.len(),
        "bubble_up_url_count": results.iter().filter(|r| non_empty(&r.bubble_up_url)).count(),
        "generic_type": "",
        "attachment_has_content": false,
        "attachment_has_description": false,
        "attachment_filename": "",
        "attachment_content_type": "",
        "attachment_byte_size": 0,
        "attachment_previewable": false,
        "attachment_width": 0,
        "attachment_height": 0,
        "upload_line_type": "",
        "upload_boosts_count": 0,
        "upload_attachment_filename": "",
        "upload_attachment_has_title": false,
        "upload_attachment_has_id": false,
        "upload_attachment_has_sgid": false,
        "needle_type": "",
        "needle_color": "",
        "needle_position": 0,
        "needle_comments_count": 0,
        "needle_comment_count": 0,
        "needle_boosts_count": 0,
        "needle_attachment_has_id": false,
        "needle_attachment_has_sgid": false,
        "needle_attachment_width": 0,
        "kanban_type": "",
        "kanban_position": 0,
        "kanban_cards_count": 0,
        "kanban_comment_count": 0,
        "kanban_subscriber_count": 0,
        "kanban_has_color": false,
        "kanban_has_on_hold": false,
        "kanban_on_hold_cards_count": 0,
    });
    for key in ["id", "title", "type", "url", "app_url"] {
        summary[format!("generic_has_{key}")] = json!(false);
        summary[format!("attachment_has_{key}")] = json!(false);
    }

    if let Some(g) = find(&|r| non_empty(&r.r#type)) {
        summary["generic_type"] = json!(text(&g.r#type));
        summary["generic_has_id"] = json!(g.id.is_some_and(|id| id != 0));
        summary["generic_has_title"] = json!(non_empty(&g.title));
        summary["generic_has_type"] = json!(true);
        summary["generic_has_url"] = json!(non_empty(&g.url));
        summary["generic_has_app_url"] = json!(non_empty(&g.app_url));
    }
    if let Some(a) = find(&|r| !non_empty(&r.r#type)) {
        summary["attachment_has_id"] = json!(a.id.is_some_and(|id| id != 0));
        summary["attachment_has_title"] = json!(non_empty(&a.title));
        summary["attachment_has_type"] = json!(false);
        summary["attachment_has_url"] = json!(non_empty(&a.url));
        summary["attachment_has_app_url"] = json!(non_empty(&a.app_url));
        summary["attachment_has_content"] = json!(a.content.is_some());
        summary["attachment_has_description"] = json!(a.description.is_some());
        summary["attachment_filename"] = json!(text(&a.filename));
        summary["attachment_content_type"] = json!(text(&a.content_type));
        summary["attachment_byte_size"] = json!(a.byte_size.unwrap_or_default());
        summary["attachment_previewable"] = json!(a.previewable.unwrap_or_default());
        summary["attachment_width"] = json!(int(a.width));
        summary["attachment_height"] = json!(int(a.height));
    }
    if let Some(u) = find(&|r| r.r#type.as_deref() == Some("Chat::Lines::Upload")) {
        summary["upload_line_type"] = json!(text(&u.r#type));
        summary["upload_boosts_count"] = json!(int(u.boosts_count));
        if let Some(att) = u.attachments.as_ref().and_then(|a| a.first()) {
            summary["upload_attachment_filename"] = json!(att.filename);
            summary["upload_attachment_has_title"] = json!(non_empty(&att.title));
            summary["upload_attachment_has_id"] = json!(att.id.is_some_and(|id| id != 0));
            summary["upload_attachment_has_sgid"] = json!(non_empty(&att.sgid));
        }
    }
    if let Some(n) = find(&|r| r.r#type.as_deref() == Some("Gauge::Needle")) {
        summary["needle_type"] = json!(text(&n.r#type));
        summary["needle_color"] = json!(text(&n.color));
        summary["needle_position"] = json!(int(n.position));
        summary["needle_comments_count"] = json!(int(n.comments_count));
        summary["needle_comment_count"] = json!(int(n.comment_count));
        summary["needle_boosts_count"] = json!(int(n.boosts_count));
        if let Some(att) = n.attachments.as_ref().and_then(|a| a.first()) {
            summary["needle_attachment_has_id"] = json!(att.id.is_some_and(|id| id != 0));
            summary["needle_attachment_has_sgid"] = json!(non_empty(&att.sgid));
            summary["needle_attachment_width"] = json!(int(att.width));
        }
    }
    if let Some(k) = find(&|r| r.r#type.as_deref() == Some("Kanban::Column")) {
        summary["kanban_type"] = json!(text(&k.r#type));
        summary["kanban_position"] = json!(int(k.position));
        summary["kanban_cards_count"] = json!(int(k.cards_count));
        summary["kanban_comment_count"] = json!(int(k.comment_count));
        summary["kanban_subscriber_count"] = json!(k.subscribers.as_ref().map_or(0, Vec::len));
        summary["kanban_has_color"] = json!(non_empty(&k.color));
        summary["kanban_has_on_hold"] = json!(k.on_hold.is_some());
        summary["kanban_on_hold_cards_count"] =
            json!(k.on_hold.as_ref().map_or(0, |h| h.cards_count));
    }
    summary
}
