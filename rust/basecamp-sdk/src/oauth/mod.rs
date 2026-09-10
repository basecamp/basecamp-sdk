//! SPEC §16: OAuth utilities — PKCE, resource-first discovery, the authorization-code
//! exchange and refresh, and the RFC 8628 device grant.
//!
//! Everything here goes out through one [`OAuthClient`], which sends on any
//! [`HttpClient`] and holds the transport policy every credential-bearing request shares:
//! a bounded request timeout (30 s by default, never more than an hour), a bounded body
//! read, and no redirect ever followed. With the `reqwest` feature, [`OAuthClient::shipped`]
//! builds one over the client the SDK ships.
//!
//! The secrets the flows create or receive — the PKCE verifier, an authorization code, the
//! tokens, a device code — travel as [`SensitiveString`]s, so a `{:?}` of any value here
//! prints `[REDACTED]` in their place. An OAuth endpoint's failure is reported as the RFC
//! 6749 `error` and a truncated `error_description`, and nothing else of its body (SPEC §9).
//!
//! Every failure is the crate's one [`Error`](crate::Error). Where a flow ends for a reason a caller
//! branches on — a user declining a device code, a resource advertising two issuers — the
//! reason rides the error's source chain as a [`DeviceFlowError`] or [`SelectionError`],
//! and the error's [`ErrorCode`](crate::ErrorCode) is derived from it.

use std::fmt;
use std::sync::Arc;
use std::time::Duration;

use crate::http::HttpClient;
use crate::types::SensitiveString;

mod device;
mod discovery;
mod pkce;
mod provider;
#[cfg(test)]
mod testing;
mod token;
mod transport;

pub use device::{
    Clock, DEVICE_CODE_GRANT_TYPE, DeviceAuthorization, DeviceFlowError, DeviceFlowReason,
    MAX_DEVICE_SECONDS, MonotonicClock,
};
pub use discovery::{
    DiscoveryOutcome, FallbackReason, LAUNCHPAD_ISSUER, ProtectedResourceMetadata, SelectionError,
    SelectionFailure, ServerMetadata, require_origin_root,
};
pub use pkce::{Pkce, generate_pkce, generate_state};
pub use provider::RefreshingTokenProvider;
pub use token::{
    ExchangeRequest, MAX_TOKEN_LIFETIME_SECONDS, RefreshRequest, Token, authorization_url,
};
pub use transport::{DEFAULT_REQUEST_TIMEOUT, MAX_REQUEST_TIMEOUT};

/// Talks to an OAuth 2.0 authorization server: discovery, the authorization-code exchange
/// and refresh, and the device grant.
///
/// One client serves any number of issuers; nothing about an issuer is held here. What is
/// held is the transport policy: the [`HttpClient`] the requests go out on, the per-request
/// timeout of [`OAuthClient::with_request_timeout`], and the Launchpad issuer the
/// resource-first selection heuristic excludes.
#[derive(Clone)]
pub struct OAuthClient {
    http: Arc<dyn HttpClient>,
    request_timeout: Duration,
    launchpad_issuer: String,
}

impl OAuthClient {
    /// A client sending on `http`, with the default request timeout.
    ///
    /// The client is the caller's, and keeps its own transport settings; what this layer
    /// adds on top and never removes is the redirect refusal, the body cap and the
    /// wall-clock bound on each request.
    pub fn new(http: impl HttpClient + 'static) -> OAuthClient {
        OAuthClient {
            http: Arc::new(http),
            request_timeout: DEFAULT_REQUEST_TIMEOUT,
            launchpad_issuer: LAUNCHPAD_ISSUER.to_string(),
        }
    }

    /// The same client with another bound on each request — every discovery fetch, token
    /// POST and device-flow POST it makes. A zero timeout, or one past
    /// [`MAX_REQUEST_TIMEOUT`], is replaced with [`DEFAULT_REQUEST_TIMEOUT`] rather than
    /// refused: an invalid value must not leave a credential POST unbounded.
    pub fn with_request_timeout(mut self, timeout: Duration) -> OAuthClient {
        self.request_timeout = transport::normalize_timeout(timeout);
        self
    }

    /// The same client with another origin standing in for Launchpad: the issuer the
    /// selection heuristic of [`OAuthClient::discover_from_resource`] identifies BC5 by
    /// excluding, and the one [`OAuthClient::discover_launchpad`] reads. A staging
    /// Launchpad, or a test double.
    pub fn with_launchpad_issuer(mut self, issuer: impl Into<String>) -> OAuthClient {
        self.launchpad_issuer = issuer.into();
        self
    }

    /// The bound on each request.
    pub fn request_timeout(&self) -> Duration {
        self.request_timeout
    }

    /// The origin standing in for Launchpad.
    pub fn launchpad_issuer(&self) -> &str {
        &self.launchpad_issuer
    }

    pub(crate) fn http(&self) -> &dyn HttpClient {
        self.http.as_ref()
    }
}

impl fmt::Debug for OAuthClient {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("OAuthClient")
            .field("request_timeout", &self.request_timeout)
            .field("launchpad_issuer", &self.launchpad_issuer)
            .finish_non_exhaustive()
    }
}

#[cfg(feature = "reqwest")]
#[cfg_attr(docsrs, doc(cfg(feature = "reqwest")))]
impl OAuthClient {
    /// A client over the shipped [`ReqwestClient`](crate::http::ReqwestClient), whose own
    /// timeout is set to the ceiling so that the bound in force is this layer's: the device
    /// poll backs off from a request this layer timed out, and would end the flow on one
    /// the transport gave up on first.
    pub fn shipped() -> Result<OAuthClient, crate::Error> {
        let http = crate::http::ReqwestClient::with_timeout(MAX_REQUEST_TIMEOUT)?;
        Ok(OAuthClient::new(http))
    }
}

/// Whether a secret has a value, for the required-field checks that never print it.
fn is_blank(secret: &SensitiveString) -> bool {
    secret.is_empty()
}
