//! SPEC §3: the client, and the account-scoped client every service hangs off.

use std::fmt;
use std::sync::Arc;
use std::time::{Duration, Instant};

use bytes::Bytes;
use serde::de::DeserializeOwned;
use url::Url;

use crate::auth::{AuthStrategy, BearerAuth, StaticTokenProvider, TokenProvider};
use crate::config::Config;
use crate::error::{Error, ErrorCode, parse_retry_after_header};
use crate::hooks::{Hooks, NoopHooks, OperationResult, RequestInfo, RequestResult};
use crate::http::header::{ACCEPT, CONTENT_TYPE, USER_AGENT};
use crate::http::{
    Body, HeaderMap, HeaderValue, HttpClient, Request, Response as HttpResponse, StatusCode,
};
use crate::operation::Operation;
use crate::pagination::Page;
use crate::retry::{backoff_with_jitter, effective_attempts};
use crate::route::{Representation, Route};
use crate::security::{is_same_origin, require_secure_endpoint};
use crate::version::default_user_agent;

/// A Basecamp client: one identity, one API origin. Derive an [`AccountClient`] with
/// [`Client::for_account`] to reach the services.
///
/// Clients are cheap to clone and share their connection pool, credentials and hooks.
#[derive(Clone)]
pub struct Client {
    shared: Arc<Shared>,
}

/// The client scoped to one account: where every service lives.
#[derive(Clone)]
pub struct AccountClient {
    shared: Arc<Shared>,
    account_id: String,
}

pub(crate) struct Shared {
    pub(crate) config: Config,
    pub(crate) base_url: Url,
    pub(crate) http: Arc<dyn HttpClient>,
    pub(crate) auth: Arc<dyn AuthStrategy>,
    pub(crate) user_agent: String,
    pub(crate) hooks: Arc<dyn Hooks>,
}

/// What came back from Basecamp, before it is decoded.
#[derive(Debug, Clone)]
#[non_exhaustive]
pub struct Response {
    /// The status.
    pub status: StatusCode,
    /// The headers.
    pub headers: HeaderMap,
    /// The body, read whole.
    pub body: Bytes,
    /// The URL the answer came from.
    pub url: Url,
}

impl Response {
    /// The body decoded as JSON. A body that does not decode as `T` is a statusless,
    /// non-retryable `api_error` (SPEC §6) that keeps the request id.
    pub fn json<T: DeserializeOwned>(&self) -> Result<T, Error> {
        serde_json::from_slice(&self.body).map_err(|error| {
            let mut mapped = Error::malformed_response(error.to_string());
            if let Some(request_id) = self.header("x-request-id") {
                mapped = mapped.with_request_id(request_id);
            }
            mapped
        })
    }

    /// One header, when present and readable as text.
    pub fn header(&self, name: &str) -> Option<&str> {
        self.headers.get(name).and_then(|value| value.to_str().ok())
    }
}

/// Builds a [`Client`].
pub struct ClientBuilder {
    config: Config,
    auth: Option<Arc<dyn AuthStrategy>>,
    http: Option<Arc<dyn HttpClient>>,
    user_agent: String,
    hooks: Arc<dyn Hooks>,
    auth_given: u8,
}

impl ClientBuilder {
    /// A builder over a configuration.
    pub fn new(config: Config) -> ClientBuilder {
        ClientBuilder {
            config,
            auth: None,
            http: None,
            user_agent: default_user_agent(),
            hooks: Arc::new(NoopHooks),
            auth_given: 0,
        }
    }

    /// A fixed bearer token — a personal access token, say.
    pub fn access_token(self, token: impl Into<String>) -> ClientBuilder {
        self.token_provider(StaticTokenProvider::new(token.into()))
    }

    /// Bearer authentication over a [`TokenProvider`], refreshed on 401 where the provider
    /// supports it.
    pub fn token_provider(self, provider: impl TokenProvider + 'static) -> ClientBuilder {
        self.auth_strategy(BearerAuth::new(provider))
    }

    /// Any [`AuthStrategy`]. Exactly one of this, [`ClientBuilder::access_token`] or
    /// [`ClientBuilder::token_provider`] must be given.
    pub fn auth_strategy(mut self, strategy: impl AuthStrategy + 'static) -> ClientBuilder {
        self.auth_given += 1;
        self.auth = Some(Arc::new(strategy));
        self
    }

    /// Replaces the HTTP client every request goes out on. It must not follow redirects;
    /// see [`HttpClient`]. The configured timeout then has no effect — a timeout belongs to
    /// the client that can enforce it.
    pub fn http_client(mut self, http: impl HttpClient + 'static) -> ClientBuilder {
        self.http = Some(Arc::new(http));
        self
    }

    /// Another `User-Agent`.
    pub fn user_agent(mut self, user_agent: impl Into<String>) -> ClientBuilder {
        self.user_agent = user_agent.into();
        self
    }

    /// Reports every operation and every request the client makes. Several sets of hooks
    /// go on as one with [`crate::hooks::ChainHooks`].
    pub fn hooks(mut self, hooks: impl Hooks + 'static) -> ClientBuilder {
        self.hooks = Arc::new(hooks);
        self
    }

    /// The whole-operation deadline; see [`Config::operation_deadline`].
    pub fn operation_deadline(mut self, deadline: Duration) -> ClientBuilder {
        self.config.operation_deadline = Some(deadline);
        self
    }

    /// Validates the configuration (SPEC §2) and builds the client.
    pub fn build(self) -> Result<Client, Error> {
        if self.auth_given > 1 {
            return Err(Error::usage(
                "Provide either auth or access_token, not both",
            ));
        }
        let auth = self
            .auth
            .ok_or_else(|| Error::usage("Either auth or access_token is required"))?;
        let base_url = parse_base_url(&self.config.base_url)?;
        self.config.validate()?;
        let http = match self.http {
            Some(http) => http,
            None => shipped_http_client(self.config.timeout)?,
        };
        Ok(Client {
            shared: Arc::new(Shared {
                config: self.config,
                base_url,
                http,
                auth,
                user_agent: self.user_agent,
                hooks: self.hooks,
            }),
        })
    }
}

impl Client {
    /// A builder over a configuration.
    pub fn builder(config: Config) -> ClientBuilder {
        ClientBuilder::new(config)
    }

    /// The configuration the client was built with.
    pub fn config(&self) -> &Config {
        &self.shared.config
    }

    /// The API origin.
    pub fn base_url(&self) -> &Url {
        &self.shared.base_url
    }

    /// The client scoped to one account. Every account-scoped path is prefixed with the id.
    pub fn for_account(&self, account_id: impl Into<String>) -> AccountClient {
        AccountClient {
            shared: self.shared.clone(),
            account_id: account_id.into(),
        }
    }
}

impl fmt::Debug for Client {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Client")
            .field("base_url", &self.shared.base_url)
            .finish_non_exhaustive()
    }
}

impl fmt::Debug for AccountClient {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("AccountClient")
            .field("base_url", &self.shared.base_url)
            .field("account_id", &self.account_id)
            .finish_non_exhaustive()
    }
}

impl AccountClient {
    /// The account this client is scoped to.
    pub fn account_id(&self) -> &str {
        &self.account_id
    }

    /// The configuration the client was built with.
    pub fn config(&self) -> &Config {
        &self.shared.config
    }

    /// The API origin.
    pub fn base_url(&self) -> &Url {
        &self.shared.base_url
    }

    /// How many pages an auto-paginating read follows.
    pub fn max_pages(&self) -> usize {
        self.shared.config.max_pages
    }

    pub(crate) fn shared(&self) -> &Arc<Shared> {
        &self.shared
    }

    /// The account-independent client this was derived from.
    pub fn client(&self) -> Client {
        Client {
            shared: self.shared.clone(),
        }
    }

    /// Starts a request for one of the modelled routes. Generated service methods call
    /// this; reach for it directly only to add a query parameter they do not expose.
    pub fn operation(&self, route: &'static Route, params: &[&dyn fmt::Display]) -> Operation {
        Operation::for_route(route, params)
    }

    /// Sends an operation and decodes its JSON body.
    pub async fn send<T: DeserializeOwned>(&self, operation: Operation) -> Result<T, Error> {
        self.execute(operation).await?.json()
    }

    /// Sends an operation whose answer carries no body worth reading.
    pub async fn send_unit(&self, operation: Operation) -> Result<(), Error> {
        self.execute(operation).await.map(|_| ())
    }

    /// Sends a paginated read and keeps the cursor Basecamp answered with.
    pub async fn send_page<T: DeserializeOwned>(
        &self,
        operation: Operation,
    ) -> Result<Page<T>, Error> {
        let route = operation.route;
        let response = self.execute(operation).await?;
        Ok(Page::new(response.json()?, &response, route))
    }

    /// The follow-on read for a `Link` target: the same route and identity, the target
    /// checked against the origin the walk started on (SPEC §8).
    #[allow(clippy::unused_self)]
    pub(crate) fn follow_up(
        &self,
        route: &'static Route,
        origin: &Url,
        next: &Url,
    ) -> Result<Operation, Error> {
        if !is_same_origin(next, origin) {
            return Err(Error::usage(format!(
                "pagination Link header points to a different origin: {}",
                crate::security::redact_url(next)
            )));
        }
        let mut info = Operation::for_route(route, &[]).info;
        info.is_mutation = false;
        Ok(Operation::at(route, info, next.clone()))
    }

    /// Sends an operation: applies credentials and account scope, retries transient
    /// failures under SPEC §7's three gates, resends once after a refreshed 401, and maps
    /// every non-2xx status onto an [`Error`].
    pub async fn execute(&self, operation: Operation) -> Result<Response, Error> {
        let started = Instant::now();
        let hooks = self.shared.hooks.clone();
        crate::hooks::guarded(|| hooks.on_operation_start(&operation.info));
        let work = self.dispatch(&operation);
        let outcome = match self.shared.config.operation_deadline {
            None => work.await,
            Some(deadline) => match tokio::time::timeout(deadline, work).await {
                Ok(outcome) => outcome,
                Err(_) => Err(Error::deadline_exceeded(deadline)),
            },
        };
        let result = OperationResult {
            error: outcome.as_ref().err(),
            duration: started.elapsed(),
        };
        crate::hooks::guarded(|| hooks.on_operation_end(&operation.info, &result));
        outcome
    }

    #[cfg(feature = "tracing")]
    async fn dispatch(&self, operation: &Operation) -> Result<Response, Error> {
        use tracing::Instrument;
        let span = tracing::info_span!(
            "basecamp.operation",
            operation = operation.route.id,
            service = operation.route.service,
            http.status = tracing::field::Empty,
            request_id = tracing::field::Empty,
        );
        self.attempts(operation).instrument(span).await
    }

    #[cfg(not(feature = "tracing"))]
    async fn dispatch(&self, operation: &Operation) -> Result<Response, Error> {
        self.attempts(operation).await
    }

    /// SPEC §7's loop, written out in the order the specification states it.
    #[allow(clippy::too_many_lines)]
    async fn attempts(&self, operation: &Operation) -> Result<Response, Error> {
        let route = operation.route;
        let retry = &route.metadata.retry;
        let eligible = route.retry_eligible();
        let attempts = if eligible {
            effective_attempts(self.shared.config.max_retries, retry.max_attempts)
        } else {
            1
        };
        let url = self.url_for(operation)?;
        let hooks = &self.shared.hooks;
        let mut refreshed = false;
        let mut attempt: u32 = 1;
        let mut retry_index: u32 = 0;

        loop {
            let generation = self.shared.auth.generation();
            let request = self.prepare(operation, &url).await?;
            let info = RequestInfo {
                method: operation.method.clone(),
                url: url.clone(),
                attempt,
            };
            crate::hooks::guarded(|| hooks.on_request_start(&info));
            let sent_at = Instant::now();
            let sent = self.shared.http.send(request).await;
            let duration = sent_at.elapsed();

            match sent {
                Err(error) => {
                    crate::hooks::guarded(|| {
                        hooks.on_request_end(
                            &info,
                            &RequestResult {
                                status: None,
                                duration,
                                error: Some(&error),
                                retry_after: None,
                            },
                        );
                    });
                    if eligible && attempt < attempts {
                        let delay =
                            backoff_with_jitter(retry, retry_index, self.shared.config.max_jitter);
                        crate::hooks::guarded(|| hooks.on_retry(&info, attempt + 1, &error, delay));
                        tokio::time::sleep(delay).await;
                        retry_index += 1;
                        attempt += 1;
                    } else {
                        return Err(error);
                    }
                }
                Ok(response) => {
                    let status = response.status();
                    let retry_after =
                        parse_retry_after_header(response.headers(), chrono::Utc::now());
                    #[cfg(feature = "tracing")]
                    {
                        let span = tracing::Span::current();
                        span.record("http.status", status.as_u16());
                        if let Some(id) = response
                            .headers()
                            .get("x-request-id")
                            .and_then(|v| v.to_str().ok())
                        {
                            span.record("request_id", id);
                        }
                    }
                    if status == StatusCode::UNAUTHORIZED
                        && !refreshed
                        && attempt < attempts
                        && self.shared.auth.refreshable()
                    {
                        let cause = Error::from_response(status, response.headers(), &[]);
                        crate::hooks::guarded(|| {
                            hooks.on_request_end(
                                &info,
                                &RequestResult {
                                    status: Some(status),
                                    duration,
                                    error: Some(&cause),
                                    retry_after,
                                },
                            );
                        });
                        refreshed = true;
                        if self.shared.auth.refresh(generation).await? {
                            crate::hooks::guarded(|| {
                                hooks.on_retry(&info, attempt + 1, &cause, Duration::ZERO);
                            });
                            attempt += 1;
                            continue;
                        }
                        return Err(self.finish_failure(operation, response).await);
                    }
                    if eligible && attempt < attempts && retry.retry_on.contains(&status.as_u16()) {
                        let cause = Error::from_response(status, response.headers(), &[]);
                        crate::hooks::guarded(|| {
                            hooks.on_request_end(
                                &info,
                                &RequestResult {
                                    status: Some(status),
                                    duration,
                                    error: Some(&cause),
                                    retry_after,
                                },
                            );
                        });
                        let delay = match retry_after {
                            Some(seconds) => Duration::from_secs(u64::from(seconds)),
                            None => backoff_with_jitter(
                                retry,
                                retry_index,
                                self.shared.config.max_jitter,
                            ),
                        };
                        crate::hooks::guarded(|| hooks.on_retry(&info, attempt + 1, &cause, delay));
                        tokio::time::sleep(delay).await;
                        retry_index += 1;
                        attempt += 1;
                        continue;
                    }
                    let finished = self.finish(operation, url.clone(), response).await;
                    crate::hooks::guarded(|| {
                        hooks.on_request_end(
                            &info,
                            &RequestResult {
                                status: Some(status),
                                duration,
                                error: finished.as_ref().err(),
                                retry_after,
                            },
                        );
                    });
                    return finished;
                }
            }
        }
    }

    /// SPEC §3's `buildURL`: an absolute same-origin URL as given; else the base URL, the
    /// account id and the path.
    pub(crate) fn url_for(&self, operation: &Operation) -> Result<Url, Error> {
        let mut url = if let Some(url) = &operation.url {
            if !is_same_origin(url, &self.shared.base_url) {
                return Err(Error::usage("absolute URL must be same-origin as base_url"));
            }
            url.clone()
        } else {
            let path = operation.path.trim_start_matches('/');
            let mut url = self.shared.base_url.clone();
            url.set_path(&format!("/{}/{path}", self.account_id));
            url
        };
        if !operation.query.is_empty() {
            url.query_pairs_mut().extend_pairs(&operation.query);
        }
        Ok(url)
    }

    async fn prepare(&self, operation: &Operation, url: &Url) -> Result<Request<Bytes>, Error> {
        let mut request = Request::builder()
            .method(operation.method.clone())
            .uri(url.as_str())
            .body(Bytes::new())
            .map_err(|error| Error::usage(format!("request could not be built: {error}")))?;
        let headers = request.headers_mut();
        headers.insert(USER_AGENT, header_value(&self.shared.user_agent)?);
        headers.insert(ACCEPT, HeaderValue::from_static("application/json"));
        if let Some(body) = &operation.body {
            headers.insert(CONTENT_TYPE, header_value(&body.content_type)?);
            *request.body_mut() = body.bytes.clone();
        }
        self.shared.auth.authenticate(&mut request).await?;
        Ok(request)
    }

    async fn finish(
        &self,
        operation: &Operation,
        url: Url,
        response: HttpResponse<Body>,
    ) -> Result<Response, Error> {
        let status = response.status();
        if !status.is_success() {
            return Err(self.finish_failure(operation, response).await);
        }
        let headers = response.headers().clone();
        let limit = self.shared.config.max_response_body_bytes;
        let body = response
            .into_body()
            .collect(limit, || Error::response_too_large(limit))
            .await?;
        if operation.route.response == Representation::Json
            && body.is_empty()
            && status != StatusCode::NO_CONTENT
        {
            return Err(Error::malformed_response("empty body").with_status(status.as_u16()));
        }
        Ok(Response {
            status,
            headers,
            body,
            url,
        })
    }

    async fn finish_failure(&self, _operation: &Operation, response: HttpResponse<Body>) -> Error {
        let status = response.status();
        let headers = response.headers().clone();
        let body = response
            .into_body()
            .collect(crate::error::MAX_ERROR_BODY_BYTES, || {
                Error::response_too_large(crate::error::MAX_ERROR_BODY_BYTES)
            })
            .await
            .unwrap_or_default();
        Error::from_response(status, &headers, &body)
    }
}

#[cfg(feature = "reqwest")]
fn shipped_http_client(timeout: Duration) -> Result<Arc<dyn HttpClient>, Error> {
    Ok(Arc::new(crate::http::ReqwestClient::with_timeout(timeout)?))
}

#[cfg(not(feature = "reqwest"))]
fn shipped_http_client(_timeout: Duration) -> Result<Arc<dyn HttpClient>, Error> {
    Err(Error::usage(
        "no HTTP client: supply one with ClientBuilder::http_client, or enable the reqwest feature",
    ))
}

fn parse_base_url(base_url: &str) -> Result<Url, Error> {
    let mut url = Url::parse(base_url.trim_end_matches('/'))
        .map_err(|error| Error::usage(format!("base URL {base_url}: {error}")))?;
    if url.cannot_be_a_base() || url.host_str().is_none() {
        return Err(Error::usage(format!("base URL {base_url} has no host")));
    }
    require_secure_endpoint(&url).map_err(|_| Error::usage("base URL must use HTTPS"))?;
    url.set_query(None);
    url.set_fragment(None);
    Ok(url)
}

pub(crate) fn header_value(value: &str) -> Result<HeaderValue, Error> {
    HeaderValue::from_str(value)
        .map_err(|_| Error::usage(format!("{value:?} is not a valid header value")))
}

impl From<ErrorCode> for Error {
    fn from(code: ErrorCode) -> Error {
        Error::new(code, code.as_str())
    }
}
