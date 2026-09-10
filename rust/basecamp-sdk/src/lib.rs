#![doc = include_str!("../README.md")]
#![cfg_attr(docsrs, feature(doc_cfg))]
//!
//! # Request construction policy
//!
//! Request bodies and query-parameter structs are plain `pub` structs with `Default` and no
//! `#[non_exhaustive]`, so they are built literally — `CreateTodoRequestContent { content,
//! ..Default::default() }` — from any crate. A field the model adds to one of them is
//! therefore a source break, accepted as a `0.MINOR` release under the pre-1.0 policy.
//! Response models and open enumerations (`ErrorCode`, the model's own string enums) are
//! `#[non_exhaustive]`: a field or variant the API grows is not a break. Types with private
//! fields ([`Error`], [`Client`]) need neither.
//!
//! # Public traits
//!
//! [`HttpClient`], [`AuthStrategy`], [`TokenProvider`] and [`Hooks`] are `Send + Sync` and
//! object-safe; the SDK holds each as an `Arc<dyn Trait>`. A method added to one of them
//! gets a default body, so implementing them stays source-compatible across `0.x.PATCH`
//! releases.

pub mod auth;
pub mod client;
pub mod config;
mod deadline;
pub mod download;
pub mod error;
#[doc(hidden)]
pub mod generated;
pub mod hooks;
pub mod http;
#[cfg(feature = "oauth")]
#[cfg_attr(docsrs, doc(cfg(feature = "oauth")))]
pub mod oauth;
pub mod operation;
pub mod pagination;
pub mod retry;
pub mod route;
pub mod security;
pub mod services;
pub mod types;
pub mod url;
pub mod version;
pub mod webhooks;

pub use auth::{AuthStrategy, BearerAuth, StaticTokenProvider, TokenProvider};
pub use client::{AccountClient, Client, ClientBuilder, Response};
pub use config::Config;
pub use download::DownloadResult;
pub use error::{Error, ErrorCode};
pub use generated::OPERATION_COUNT;
pub use hooks::Hooks;
pub use http::HttpClient;
#[cfg(feature = "oauth")]
#[cfg_attr(docsrs, doc(cfg(feature = "oauth")))]
pub use oauth::{
    DeviceAuthorization, DeviceFlowError, DeviceFlowReason, DiscoveryOutcome, ExchangeRequest,
    FallbackReason, OAuthClient, Pkce, ProtectedResourceMetadata, RefreshRequest,
    RefreshingTokenProvider, SelectionError, SelectionFailure, ServerMetadata, Token,
};
pub use operation::Operation;
pub use pagination::{ListMeta, ListResult, Page};
pub use types::{AuthRoutableUrl, Date, DateTime, FlexibleTime, SensitiveString};
pub use version::{API_VERSION, VERSION};

/// The request and response types the Basecamp API speaks, generated from the model.
pub mod models {
    pub use crate::generated::types::*;
}

/// Every modelled route as public data, generated from the model.
pub mod routes {
    pub use crate::generated::routes::*;
    pub use crate::route::{
        Backoff, BodyKind, OperationMetadata, Pagination, ParamKind, Representation, RetryConfig,
        Route, RouteParam, WriteSemantics,
    };
}

/// Per-operation behaviour from `behavior-model.json`, generated from the model.
pub mod metadata {
    pub use crate::generated::metadata::*;
}
