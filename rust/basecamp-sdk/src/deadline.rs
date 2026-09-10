//! SPEC §2's `operation_deadline`: one bound over the whole of a call — credentials,
//! every attempt, every wait, the body read — applied at each await rather than around
//! the call, so a request the deadline cuts short still reports its end to the hooks.

use std::time::Duration;

use tokio::time::Instant;

use crate::error::Error;

#[derive(Debug, Clone, Copy)]
pub(crate) struct Deadline {
    configured: Option<Duration>,
    at: Option<Instant>,
}

impl Deadline {
    /// The configured bound, counted from now; `None` bounds nothing.
    pub(crate) fn starting_now(configured: Option<Duration>) -> Deadline {
        // A bound the clock cannot represent is no bound.
        Deadline {
            configured,
            at: configured.and_then(|after| Instant::now().checked_add(after)),
        }
    }

    /// Runs `work`, unless the deadline passes first.
    pub(crate) async fn bound<T>(
        &self,
        work: impl Future<Output = Result<T, Error>>,
    ) -> Result<T, Error> {
        match self.at {
            None => work.await,
            Some(at) => match tokio::time::timeout_at(at, work).await {
                Ok(outcome) => outcome,
                Err(_) => Err(self.exceeded()),
            },
        }
    }

    /// Waits `delay` before a resend, unless the deadline would pass first: a resend that
    /// could not go out in time is not begun, and the deadline is exceeded now rather than
    /// after the wait — and a wait that was admitted still ends at the deadline.
    pub(crate) async fn wait(&self, delay: Duration) -> Result<(), Error> {
        match self.at {
            Some(at) if at.saturating_duration_since(Instant::now()) < delay => {
                Err(self.exceeded())
            }
            _ => {
                self.bound(async {
                    tokio::time::sleep(delay).await;
                    Ok(())
                })
                .await
            }
        }
    }

    fn exceeded(&self) -> Error {
        Error::deadline_exceeded(self.configured.unwrap_or_default())
    }
}
