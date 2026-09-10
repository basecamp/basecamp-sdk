//! One API operation as data: its method, its path template and the behaviour the model
//! attaches to it. Every route the SDK knows lives in [`crate::routes`].

use std::fmt::Display;

use percent_encoding::{AsciiSet, NON_ALPHANUMERIC, utf8_percent_encode};

use crate::http::Method;

/// One modelled operation.
#[derive(Debug)]
#[non_exhaustive]
pub struct Route {
    /// The `operationId`.
    pub id: &'static str,
    /// The service that sends it, as every Basecamp SDK names it: `Todos`, `CardTables`.
    pub service: &'static str,
    /// The HTTP method.
    pub method: Method,
    /// The path as Basecamp serves it, `{param}` placeholders included and the account
    /// prefix left off.
    pub path: &'static str,
    /// The path without a `.json` suffix, for recognizing pasted URLs.
    pub pattern: &'static str,
    /// The kind of record the route acts on, snake-cased: `todo`, `card_column`.
    pub resource_type: &'static str,
    /// The path parameters, in order.
    pub params: &'static [RouteParam],
    /// What the request carries.
    pub body: BodyKind,
    /// What a successful answer carries.
    pub response: Representation,
    /// How the answer paginates.
    pub pagination: Pagination,
    /// The SPEC §18 write semantics, on the replace-style operations that declare them.
    pub write: Option<&'static WriteSemantics>,
    /// The operation is deprecated upstream.
    pub deprecated: bool,
    /// Idempotency, read-onlyness and the retry tuple from `behavior-model.json`.
    pub metadata: &'static OperationMetadata,
}

/// One path parameter.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub struct RouteParam {
    /// The name in the path template.
    pub name: &'static str,
    /// Its type.
    pub kind: ParamKind,
}

/// The type of a path or query parameter.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum ParamKind {
    /// A string.
    String,
    /// A boolean.
    Bool,
    /// A 32-bit integer.
    Int32,
    /// A 64-bit integer.
    Int64,
    /// A repeated string (`name[]=a&name[]=b`).
    StringList,
    /// A repeated 64-bit integer.
    Int64List,
}

/// What a request carries.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum BodyKind {
    /// Nothing.
    None,
    /// A JSON document.
    Json,
    /// Raw bytes, with the caller's content type.
    Octet,
    /// One file under a multipart form field.
    Multipart {
        /// The form field the file goes under.
        field: &'static str,
    },
}

/// What a successful answer carries.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum Representation {
    /// No body (204, or a 200 with nothing to read).
    Empty,
    /// A JSON document.
    Json,
}

/// How an answer paginates.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum Pagination {
    /// A single answer.
    None,
    /// `Link: <…>; rel="next"` headers, SPEC §8.
    Link {
        /// The response member holding the paginated array, when the body is an envelope
        /// rather than a bare array.
        key: Option<&'static str>,
        /// The header carrying the collection's total size.
        total_count_header: Option<&'static str>,
    },
}

/// SPEC §18 write semantics for a replace-style operation.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub struct WriteSemantics {
    /// A field the body omits is cleared by the server.
    pub clears_omitted: bool,
    /// The fields the server preserves on omission anyway — the declared carve-outs.
    pub preserved_on_omission: &'static [&'static str],
}

/// Per-operation behaviour from `behavior-model.json`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub struct OperationMetadata {
    /// The `operationId`.
    pub operation: &'static str,
    /// The naturally-idempotent flag: SPEC §7 Gate 2 for a POST.
    pub idempotent: bool,
    /// The operation only reads.
    pub readonly: bool,
    /// The retry tuple SPEC §7 Gate 3 consumes.
    pub retry: RetryConfig,
}

/// The retry tuple of one operation.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub struct RetryConfig {
    /// Total attempts, the initial request included.
    pub max_attempts: u32,
    /// The base of the backoff curve.
    pub base_delay_ms: u64,
    /// The curve's shape.
    pub backoff: Backoff,
    /// The statuses that are retried. Exhaustive: anything else is surfaced at once.
    pub retry_on: &'static [u16],
}

/// The shape of a backoff curve.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum Backoff {
    /// `base × 2^i`.
    Exponential,
    /// `base × (i + 1)`.
    Linear,
    /// `base`.
    Constant,
}

const PATH_SEGMENT: &AsciiSet = &NON_ALPHANUMERIC
    .remove(b'-')
    .remove(b'_')
    .remove(b'.')
    .remove(b'~');

impl Route {
    /// Whether SPEC §7's first two gates admit a retry: the method is naturally idempotent,
    /// or the model flags the operation so.
    pub fn retry_eligible(&self) -> bool {
        matches!(
            self.method,
            Method::GET | Method::HEAD | Method::PUT | Method::DELETE
        ) || self.metadata.idempotent
    }

    /// Substitutes the path parameters, in order, percent-encoding each value.
    ///
    /// # Panics
    ///
    /// When `values` is not exactly as long as [`Route::params`]. A short list would leave
    /// a `{param}` in the path and send it as written, which is worse than stopping; every
    /// generated caller passes the right count, so reaching this means the call was built
    /// by hand and built wrong.
    pub fn fill(&self, values: &[&dyn Display]) -> String {
        assert_eq!(
            values.len(),
            self.params.len(),
            "{} takes {} path parameters",
            self.id,
            self.params.len()
        );
        let mut path = self.path.to_string();
        for (param, value) in self.params.iter().zip(values) {
            let encoded = utf8_percent_encode(&value.to_string(), PATH_SEGMENT).to_string();
            path = path.replace(&format!("{{{}}}", param.name), &encoded);
        }
        path
    }

    /// Matches a path against the route's pattern and answers the captured parameters.
    pub fn recognize(&self, path: &str) -> Option<Vec<(&'static str, String)>> {
        let pattern_segments: Vec<&str> = self.pattern.split('/').collect();
        let path_segments: Vec<&str> = path.split('/').collect();
        if pattern_segments.len() != path_segments.len() {
            return None;
        }
        let mut params = Vec::new();
        for (pattern, actual) in pattern_segments.iter().zip(&path_segments) {
            if let Some(name) = pattern
                .strip_prefix('{')
                .and_then(|rest| rest.strip_suffix('}'))
            {
                if actual.is_empty() {
                    return None;
                }
                let param = self.params.iter().find(|param| param.name == name)?;
                params.push((param.name, (*actual).to_string()));
            } else if pattern != actual {
                return None;
            }
        }
        Some(params)
    }
}
