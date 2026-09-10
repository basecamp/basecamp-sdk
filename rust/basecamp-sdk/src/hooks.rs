//! SPEC §12: what the SDK tells an application about the calls it makes.

use std::panic::{AssertUnwindSafe, catch_unwind};
use std::sync::Arc;
use std::time::Duration;

use url::Url;

use crate::error::Error;
use crate::http::{Method, StatusCode};

/// The semantic identity of a call: the service and operation as the model names them,
/// the kind of record they touch, and whether they change anything.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct OperationInfo {
    /// The service the call came from: `Todos`, `Projects`.
    pub service: &'static str,
    /// The `operationId`: `ListProjects`, `GetTodo`.
    pub operation: &'static str,
    /// The kind of record the call acts on, snake-cased: `todo`, `project`.
    pub resource_type: &'static str,
    /// The call changes something (any method but GET).
    pub is_mutation: bool,
    /// The project the operation is scoped to, when the path names one.
    pub project_id: Option<i64>,
    /// The record the path names, when it names one.
    pub resource_id: Option<i64>,
}

/// One HTTP request the SDK is about to make, or has just made. `attempt` counts from 1
/// across the whole operation, the resend after a credential refresh included.
#[derive(Debug, Clone)]
#[non_exhaustive]
pub struct RequestInfo {
    /// The HTTP method.
    pub method: Method,
    /// The full request URL. A download's first hop is projected to origin and path.
    pub url: Url,
    /// The 1-based attempt number.
    pub attempt: u32,
}

/// How one HTTP request turned out.
#[derive(Debug)]
#[non_exhaustive]
pub struct RequestResult<'a> {
    /// What Basecamp answered, or `None` when nothing came back at all.
    pub status: Option<StatusCode>,
    /// How long the request took, before the body was read.
    pub duration: Duration,
    /// What this request failed with, whether or not the SDK went on to resend it.
    pub error: Option<&'a Error>,
    /// The seconds `Retry-After` named, when the answer carried one.
    pub retry_after: Option<u32>,
}

/// How a whole operation turned out, retries and all.
#[derive(Debug)]
#[non_exhaustive]
pub struct OperationResult<'a> {
    /// The error the operation ended with, after every retry.
    pub error: Option<&'a Error>,
    /// How long the whole operation took.
    pub duration: Duration,
}

/// The callbacks the SDK makes as it works. Every one does nothing by default, so an
/// implementation says only what it cares about. A hook that panics is caught: hook
/// failures never reach the caller or break the call (SPEC §12's safety invariant).
pub trait Hooks: Send + Sync {
    /// An operation is starting.
    fn on_operation_start(&self, _info: &OperationInfo) {}

    /// An operation ended, after every retry.
    fn on_operation_end(&self, _info: &OperationInfo, _result: &OperationResult<'_>) {}

    /// One HTTP request is about to go out.
    fn on_request_start(&self, _info: &RequestInfo) {}

    /// One HTTP request came back, or failed.
    fn on_request_end(&self, _info: &RequestInfo, _result: &RequestResult<'_>) {}

    /// A resend is about to be made: the attempt that failed in `info`, the one about to
    /// be made as `next_attempt`, what prompted it, and how long the SDK will wait first.
    fn on_retry(&self, _info: &RequestInfo, _next_attempt: u32, _error: &Error, _delay: Duration) {}
}

/// Hooks that do nothing: what a client runs with until it is given others.
#[derive(Debug, Clone, Copy, Default)]
pub struct NoopHooks;

impl Hooks for NoopHooks {}

/// Several [`Hooks`] as one. Starts and retries run in order; ends run in reverse, so a hook
/// that wraps the ones after it closes last, the way nested spans do.
pub struct ChainHooks {
    hooks: Vec<Arc<dyn Hooks>>,
}

impl ChainHooks {
    /// Chains the given hooks.
    pub fn new(hooks: Vec<Arc<dyn Hooks>>) -> ChainHooks {
        ChainHooks { hooks }
    }
}

impl Hooks for ChainHooks {
    fn on_operation_start(&self, info: &OperationInfo) {
        for hook in &self.hooks {
            guarded(|| hook.on_operation_start(info));
        }
    }

    fn on_operation_end(&self, info: &OperationInfo, result: &OperationResult<'_>) {
        for hook in self.hooks.iter().rev() {
            guarded(|| hook.on_operation_end(info, result));
        }
    }

    fn on_request_start(&self, info: &RequestInfo) {
        for hook in &self.hooks {
            guarded(|| hook.on_request_start(info));
        }
    }

    fn on_request_end(&self, info: &RequestInfo, result: &RequestResult<'_>) {
        for hook in self.hooks.iter().rev() {
            guarded(|| hook.on_request_end(info, result));
        }
    }

    fn on_retry(&self, info: &RequestInfo, next_attempt: u32, error: &Error, delay: Duration) {
        for hook in &self.hooks {
            guarded(|| hook.on_retry(info, next_attempt, error, delay));
        }
    }
}

/// Runs a hook and swallows its panic, reporting it on stderr.
pub(crate) fn guarded(call: impl FnOnce()) {
    if catch_unwind(AssertUnwindSafe(call)).is_err() {
        eprintln!("basecamp-sdk: a hook panicked; the operation continues");
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use super::*;

    struct Recorder {
        name: &'static str,
        log: Arc<Mutex<Vec<String>>>,
    }

    impl Hooks for Recorder {
        fn on_operation_start(&self, info: &OperationInfo) {
            self.log
                .lock()
                .unwrap()
                .push(format!("{} start {}", self.name, info.operation));
        }

        fn on_operation_end(&self, info: &OperationInfo, _result: &OperationResult<'_>) {
            self.log
                .lock()
                .unwrap()
                .push(format!("{} end {}", self.name, info.operation));
        }
    }

    struct Panicking;

    impl Hooks for Panicking {
        fn on_operation_start(&self, _info: &OperationInfo) {
            panic!("boom");
        }
    }

    fn info() -> OperationInfo {
        OperationInfo {
            service: "Projects",
            operation: "ListProjects",
            resource_type: "project",
            is_mutation: false,
            project_id: None,
            resource_id: None,
        }
    }

    #[test]
    fn a_chain_runs_forwards_and_unwinds_backwards_and_survives_a_panic() {
        let log = Arc::new(Mutex::new(Vec::new()));
        let chain = ChainHooks::new(vec![
            Arc::new(Recorder {
                name: "first",
                log: log.clone(),
            }),
            Arc::new(Panicking),
            Arc::new(Recorder {
                name: "second",
                log: log.clone(),
            }),
        ]);
        chain.on_operation_start(&info());
        chain.on_operation_end(
            &info(),
            &OperationResult {
                error: None,
                duration: Duration::from_millis(1),
            },
        );
        assert_eq!(
            *log.lock().unwrap(),
            [
                "first start ListProjects",
                "second start ListProjects",
                "second end ListProjects",
                "first end ListProjects"
            ]
        );
    }
}
