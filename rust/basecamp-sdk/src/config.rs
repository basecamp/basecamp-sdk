//! SPEC §2: what a client needs to know before it can talk to Basecamp.

use std::env;
use std::time::Duration;

use crate::error::Error;

/// Where the API lives when nothing says otherwise.
pub const DEFAULT_BASE_URL: &str = "https://3.basecampapi.com";
/// How long the shipped HTTP client gives one attempt to answer.
pub const DEFAULT_TIMEOUT: Duration = Duration::from_secs(30);
/// The client-wide attempt cap: total attempts, the initial request included. Each
/// operation's own `max_attempts` is a ceiling under it.
pub const DEFAULT_MAX_RETRIES: u32 = 3;
/// The most jitter added to a locally computed backoff.
pub const DEFAULT_MAX_JITTER: Duration = Duration::from_millis(100);
/// How many pages an auto-paginating read follows before it stops and says so.
pub const DEFAULT_MAX_PAGES: usize = 10_000;
/// The most of a successful body the client will hold in memory.
pub const MAX_RESPONSE_BODY_BYTES: usize = 52_428_800;

/// The client configuration. Build one literally, or from the environment with
/// [`Config::from_env`], and hand it to [`crate::Client::builder`].
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct Config {
    /// The API origin, without a trailing slash.
    pub base_url: String,
    /// How long the shipped HTTP client gives one attempt.
    pub timeout: Duration,
    /// Total attempts per request, the initial one included; `0` is one attempt and no
    /// retries. Each operation's own ceiling applies under it. The backoff curve itself is
    /// each operation's own (`base_delay_ms` and `backoff` from the model), not a client
    /// setting.
    pub max_retries: u32,
    /// The most jitter added to a locally computed backoff.
    pub max_jitter: Duration,
    /// How many pages an auto-paginating read follows.
    pub max_pages: usize,
    /// The most of a successful body the client will hold in memory.
    pub max_response_body_bytes: usize,
    /// A bound on the whole of one operation — every attempt, wait and body read — as
    /// distinct from the per-attempt transport timeout. `None` leaves it unbounded.
    pub operation_deadline: Option<Duration>,
}

impl Default for Config {
    fn default() -> Config {
        Config {
            base_url: DEFAULT_BASE_URL.to_string(),
            timeout: DEFAULT_TIMEOUT,
            max_retries: DEFAULT_MAX_RETRIES,
            max_jitter: DEFAULT_MAX_JITTER,
            max_pages: DEFAULT_MAX_PAGES,
            max_response_body_bytes: MAX_RESPONSE_BODY_BYTES,
            operation_deadline: None,
        }
    }
}

impl Config {
    /// The defaults with `BASECAMP_BASE_URL`, `BASECAMP_TIMEOUT` (seconds) and
    /// `BASECAMP_MAX_RETRIES` laid over them. An empty variable counts as unset; one that
    /// does not parse is a usage error.
    pub fn from_env() -> Result<Config, Error> {
        let mut config = Config::default();
        if let Some(value) = env_value("BASECAMP_BASE_URL") {
            config.base_url = value.trim_end_matches('/').to_string();
        }
        if let Some(value) = env_value("BASECAMP_TIMEOUT") {
            let seconds: u64 = value.parse().map_err(|_| {
                Error::usage(format!(
                    "BASECAMP_TIMEOUT must be an integer number of seconds, not {value:?}"
                ))
            })?;
            config.timeout = Duration::from_secs(seconds);
        }
        if let Some(value) = env_value("BASECAMP_MAX_RETRIES") {
            config.max_retries = value.parse().map_err(|_| {
                Error::usage(format!(
                    "BASECAMP_MAX_RETRIES must be a non-negative integer, not {value:?}"
                ))
            })?;
        }
        Ok(config)
    }

    /// The same configuration at another origin.
    pub fn with_base_url(mut self, base_url: impl Into<String>) -> Config {
        self.base_url = base_url.into();
        self
    }

    /// The same configuration with another attempt cap.
    pub fn with_max_retries(mut self, max_retries: u32) -> Config {
        self.max_retries = max_retries;
        self
    }

    /// The same configuration with another per-attempt timeout.
    pub fn with_timeout(mut self, timeout: Duration) -> Config {
        self.timeout = timeout;
        self
    }

    /// SPEC §2's validation algorithm.
    pub fn validate(&self) -> Result<(), Error> {
        if self.timeout.is_zero() {
            return Err(Error::usage("timeout must be greater than zero"));
        }
        if self.max_pages == 0 {
            return Err(Error::usage("max_pages must be greater than zero"));
        }
        if self.max_response_body_bytes == 0 {
            return Err(Error::usage(
                "max_response_body_bytes must be greater than zero",
            ));
        }
        if self
            .operation_deadline
            .is_some_and(|deadline| deadline.is_zero())
        {
            return Err(Error::usage("operation_deadline must be greater than zero"));
        }
        Ok(())
    }
}

fn env_value(name: &str) -> Option<String> {
    env::var(name).ok().filter(|value| !value.is_empty())
}
