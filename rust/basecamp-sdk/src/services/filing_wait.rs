//! Move or copy a recording and wait for Basecamp to finish it: the SPEC §18 composite
//! described in Appendix F "Filing Waits", ported from `go/pkg/basecamp/filing_wait.go`.
//!
//! [`RecordingsService::move_and_wait`](crate::services::recordings::RecordingsService::move_and_wait)
//! and `copy_and_wait` send the start once and never again, because each copy request makes
//! another copy, then poll `GetFiling` with backoff until the filing completes or fails, or the
//! wait ends. Every request is a generated operation under its own hook identity.

use std::fmt;
use std::future::Future;
use std::time::Duration;

use tokio::time::Instant;

use crate::error::{Error, ErrorCode};
use crate::generated::types::Filing;

/// The kind of container a move or copy files a recording into. Every kind but
/// [`FilingContainer::Todolist`] is a tool on a project's dock, and [`FilingContainer::dock_name`]
/// is its name there: the destination project's container is its enabled dock item of that
/// name. A to-do list is not on the dock; list the destination project's to-do lists under its
/// todoset.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[non_exhaustive]
pub enum FilingContainer {
    /// `kanban_board`
    CardTable,
    /// `message_board`
    MessageBoard,
    /// `vault`
    Vault,
    /// `schedule`
    Schedule,
    /// `questionnaire`
    Questionnaire,
    /// `inbox`
    Inbox,
    /// `todoset`
    Todoset,
    /// `todolist`
    Todolist,
}

impl FilingContainer {
    /// The kind of container a move or copy files a recording into, from its recordable type
    /// and its parent's type as the recording's own read reports them (`""` when it has no
    /// parent). A card lands in Triage, or in Done if it is completed. `None` for everything
    /// else: a recording that moves only with its parent, such as a comment or the columns
    /// every card table has, and a dock tool, such as a docked Vault, which is filed by
    /// `destination_project_id`.
    pub fn of(recording_type: &str, parent_type: &str) -> Option<FilingContainer> {
        match (recording_type, parent_type) {
            ("Todolist", "Todolist") | ("Todo", _) => Some(FilingContainer::Todolist),
            ("Vault", parent) if parent != "Vault" => None,
            ("Kanban::Card" | "Kanban::Column", _) => Some(FilingContainer::CardTable),
            ("Message", _) => Some(FilingContainer::MessageBoard),
            ("Document" | "Upload" | "CloudFile" | "GoogleDocument" | "Vault", _) => {
                Some(FilingContainer::Vault)
            }
            ("Schedule::Entry", _) => Some(FilingContainer::Schedule),
            ("Question", _) => Some(FilingContainer::Questionnaire),
            ("Inbox::Forward", _) => Some(FilingContainer::Inbox),
            ("Todolist", _) => Some(FilingContainer::Todoset),
            _ => None,
        }
    }

    /// The container's dock name, or `todolist` for a to-do list.
    pub fn dock_name(self) -> &'static str {
        match self {
            FilingContainer::CardTable => "kanban_board",
            FilingContainer::MessageBoard => "message_board",
            FilingContainer::Vault => "vault",
            FilingContainer::Schedule => "schedule",
            FilingContainer::Questionnaire => "questionnaire",
            FilingContainer::Inbox => "inbox",
            FilingContainer::Todoset => "todoset",
            FilingContainer::Todolist => "todolist",
        }
    }

    /// Whether a destination of this kind can take `parent_id`, a folder inside it.
    pub fn takes_folders(self) -> bool {
        self == FilingContainer::Vault
    }
}

/// Bounds how long `move_and_wait` and `copy_and_wait` wait.
#[derive(Debug, Clone, Default)]
#[non_exhaustive]
pub struct FilingWaitOptions {
    /// Ends the wait this long after the filing starts. A minute when `None`.
    pub timeout: Option<Duration>,
}

impl FilingWaitOptions {
    /// Options that end the wait `timeout` after the filing starts.
    pub fn with_timeout(timeout: Duration) -> FilingWaitOptions {
        FilingWaitOptions {
            timeout: Some(timeout),
        }
    }
}

/// How far a move or copy got when it did not complete. Match it on an [`Error`] with
/// [`FilingError::of`]. Only [`FilingError::Refused`] is safe to try again.
#[derive(Debug)]
#[non_exhaustive]
pub enum FilingError {
    /// Basecamp refused to start it (a 4xx), or the SDK refused before sending it: nothing
    /// was filed. The refusal is its source, and the outer [`Error`] carries its code.
    Refused(Error),
    /// The start failed without Basecamp refusing it, such as a 5xx or a dropped connection.
    /// It may have been filed, so look before trying again. The failure is not exposed as a
    /// source, so a retryable error inside it cannot read as permission to send it again.
    MayHaveStarted(Error),
    /// Basecamp started it and reported it failed.
    Failed(Filing),
    /// Basecamp started it and had not finished when the wait ended. `filing` is its last
    /// known state; poll `get_filing(filing.id)` rather than starting it again. `cause` is a
    /// `GetFiling` that failed, or `None` when the timeout ended the wait.
    Unsettled {
        /// The filing's last known state.
        filing: Filing,
        /// The `GetFiling` that ended the wait, if one did.
        cause: Option<Error>,
    },
}

impl FilingError {
    /// The filing outcome an [`Error`] carries, when it has one.
    pub fn of(error: &Error) -> Option<&FilingError> {
        error.find_source::<FilingError>()
    }

    fn into_error(self) -> Error {
        let (code, retryable) = match &self {
            FilingError::Refused(cause) => (cause.code(), cause.is_retryable()),
            _ => (ErrorCode::ApiError, false),
        };
        Error::new(code, self.to_string())
            .retryable(retryable)
            .with_source(self)
    }
}

impl fmt::Display for FilingError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            FilingError::Refused(cause) => write!(f, "filing refused: {}", cause.message()),
            FilingError::MayHaveStarted(cause) => {
                write!(f, "filing may have started: {}", cause.message())
            }
            FilingError::Failed(filing) => write!(f, "filing {} failed", filing.id),
            FilingError::Unsettled { filing, .. } => {
                write!(f, "filing {} is still {}", filing.id, filing.status)
            }
        }
    }
}

impl std::error::Error for FilingError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            FilingError::Refused(cause) => Some(cause),
            _ => None,
        }
    }
}

const DEFAULT_WAIT: Duration = Duration::from_secs(60);
const FIRST_POLL: Duration = Duration::from_millis(250);
const MAX_POLL: Duration = Duration::from_secs(2);

fn refused_before_filing(error: &Error) -> bool {
    let status = error.http_status().unwrap_or(0);
    (400..500).contains(&status) || matches!(error.code(), ErrorCode::Usage | ErrorCode::RateLimit)
}

pub(crate) async fn file_and_wait<S, G, F>(
    start: S,
    get_filing: G,
    options: Option<&FilingWaitOptions>,
) -> Result<Filing, Error>
where
    S: Future<Output = Result<Filing, Error>>,
    G: Fn(i64) -> F,
    F: Future<Output = Result<Filing, Error>>,
{
    let mut filing = match start.await {
        Ok(filing) => filing,
        Err(error) if refused_before_filing(&error) => {
            return Err(FilingError::Refused(error).into_error());
        }
        Err(error) => return Err(FilingError::MayHaveStarted(error).into_error()),
    };

    let deadline = Instant::now()
        + options
            .and_then(|o| o.timeout)
            .filter(|t| !t.is_zero())
            .unwrap_or(DEFAULT_WAIT);
    let mut delay = FIRST_POLL;
    loop {
        match filing.status.as_str() {
            "completed" if filing.destination_recording.is_some() => return Ok(filing),
            "failed" => return Err(FilingError::Failed(filing).into_error()),
            _ => {}
        }

        let remaining = deadline.saturating_duration_since(Instant::now());
        if remaining <= delay {
            tokio::time::sleep(remaining).await;
            return Err(FilingError::Unsettled {
                filing,
                cause: None,
            }
            .into_error());
        }
        tokio::time::sleep(delay).await;
        filing = match get_filing(filing.id).await {
            Ok(latest) => latest,
            Err(error) => {
                return Err(FilingError::Unsettled {
                    filing,
                    cause: Some(error),
                }
                .into_error());
            }
        };
        delay = (2 * delay).min(MAX_POLL);
    }
}
