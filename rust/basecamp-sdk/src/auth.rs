//! SPEC §4: how credentials get onto a request, and how a refreshed token replaces a
//! rejected one.

use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use async_trait::async_trait;
use bytes::Bytes;
use tokio::sync::Mutex;

use crate::error::Error;
use crate::http::header::AUTHORIZATION;
use crate::http::{HeaderValue, Request};
use crate::types::SensitiveString;

/// Supplies the access token each request goes out with.
#[async_trait]
pub trait TokenProvider: Send + Sync {
    /// The token to send now.
    async fn access_token(&self) -> Result<String, Error>;

    /// Whether [`TokenProvider::refresh`] can do anything. A provider that answers `false`
    /// is never asked, and a 401 surfaces at once.
    fn refreshable(&self) -> bool {
        false
    }

    /// Asked once when a request is answered with 401. Answer `true` when the next
    /// [`TokenProvider::access_token`] hands out renewed credentials, and the request is
    /// sent again. Concurrent 401s are coalesced by the client: only one refresh runs at a
    /// time, and a request that arrives while one is in flight waits for its outcome.
    async fn refresh(&self) -> Result<bool, Error> {
        Ok(false)
    }
}

/// A fixed token — a personal access token, or one read from the environment. It prints as
/// `[REDACTED]`, so a `{:?}` of the provider cannot put the token in a log.
#[derive(Debug, Clone)]
pub struct StaticTokenProvider {
    token: SensitiveString,
}

impl StaticTokenProvider {
    /// A provider over one token.
    pub fn new(token: impl Into<SensitiveString>) -> StaticTokenProvider {
        StaticTokenProvider {
            token: token.into(),
        }
    }
}

#[async_trait]
impl TokenProvider for StaticTokenProvider {
    async fn access_token(&self) -> Result<String, Error> {
        if self.token.is_empty() {
            Err(Error::new(
                crate::ErrorCode::AuthRequired,
                "no access token configured",
            ))
        } else {
            Ok(self.token.expose().to_string())
        }
    }
}

/// Puts credentials on a request. The default, [`BearerAuth`], sets an `Authorization`
/// header from a [`TokenProvider`]; anything else can plug in here.
#[async_trait]
pub trait AuthStrategy: Send + Sync {
    /// Applies credentials to the request.
    async fn authenticate(&self, request: &mut Request<Bytes>) -> Result<(), Error>;

    /// Whether a 401 is worth a [`AuthStrategy::refresh`].
    fn refreshable(&self) -> bool {
        false
    }

    /// A counter that moves every time the credentials change. The client reads it before
    /// authenticating a request and hands it back to [`AuthStrategy::refresh`], which is how
    /// concurrent 401s coalesce into one refresh.
    fn generation(&self) -> u64 {
        0
    }

    /// Asked at most once per request when it is answered with 401, and only while the
    /// attempt budget has another attempt left. `seen` is the generation the rejected
    /// request was authenticated under. Answers `true` when the request should be replayed
    /// with fresh credentials.
    async fn refresh(&self, seen: u64) -> Result<bool, Error> {
        let _ = seen;
        Ok(false)
    }
}

/// `Authorization: Bearer {token}` from a [`TokenProvider`], with concurrent refreshes
/// coalesced: whichever request meets the 401 first runs the refresh, and every request that
/// meets one while it runs waits and shares its answer rather than refreshing again.
pub struct BearerAuth<P: TokenProvider> {
    provider: P,
    generation: AtomicU64,
    refreshing: Mutex<()>,
}

impl<P: TokenProvider> BearerAuth<P> {
    /// Bearer authentication over a provider.
    pub fn new(provider: P) -> BearerAuth<P> {
        BearerAuth {
            provider,
            generation: AtomicU64::new(0),
            refreshing: Mutex::new(()),
        }
    }

    /// The provider the tokens come from.
    pub fn provider(&self) -> &P {
        &self.provider
    }
}

impl<P: TokenProvider + std::fmt::Debug> std::fmt::Debug for BearerAuth<P> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("BearerAuth")
            .field("provider", &self.provider)
            .finish_non_exhaustive()
    }
}

#[async_trait]
impl<P: TokenProvider> AuthStrategy for BearerAuth<P> {
    async fn authenticate(&self, request: &mut Request<Bytes>) -> Result<(), Error> {
        let token = self.provider.access_token().await?;
        let value = HeaderValue::from_str(&format!("Bearer {token}")).map_err(|_| {
            Error::new(
                crate::ErrorCode::AuthRequired,
                "access token is not a valid header value",
            )
        })?;
        request.headers_mut().insert(AUTHORIZATION, value);
        Ok(())
    }

    fn refreshable(&self) -> bool {
        self.provider.refreshable()
    }

    fn generation(&self) -> u64 {
        self.generation.load(Ordering::Acquire)
    }

    async fn refresh(&self, seen: u64) -> Result<bool, Error> {
        // One refresh at a time. A request that was authenticated before the refresh that
        // just finished sees the generation move and replays without refreshing again.
        let _running = self.refreshing.lock().await;
        if self.generation.load(Ordering::Acquire) != seen {
            return Ok(true);
        }
        let refreshed = self.provider.refresh().await?;
        if refreshed {
            self.generation.fetch_add(1, Ordering::AcqRel);
        }
        Ok(refreshed)
    }
}

#[async_trait]
impl<S: AuthStrategy + ?Sized> AuthStrategy for Arc<S> {
    async fn authenticate(&self, request: &mut Request<Bytes>) -> Result<(), Error> {
        (**self).authenticate(request).await
    }

    fn refreshable(&self) -> bool {
        (**self).refreshable()
    }

    fn generation(&self) -> u64 {
        (**self).generation()
    }

    async fn refresh(&self, seen: u64) -> Result<bool, Error> {
        (**self).refresh(seen).await
    }
}

#[async_trait]
impl<P: TokenProvider + ?Sized> TokenProvider for Arc<P> {
    async fn access_token(&self) -> Result<String, Error> {
        (**self).access_token().await
    }

    fn refreshable(&self) -> bool {
        (**self).refreshable()
    }

    async fn refresh(&self) -> Result<bool, Error> {
        (**self).refresh().await
    }
}
