//! SPEC §6: the one error every call can answer with.

use std::collections::BTreeMap;
use std::fmt;

use chrono::{DateTime, Utc};
use serde_json::Value;

use crate::generated::types::TemplateLibraryConfirmationPerson;
use crate::http::{HeaderMap, StatusCode};

/// The most of a server's message an error carries; longer ones end in `...`.
pub const MAX_ERROR_MESSAGE_LENGTH: usize = 500;
/// The most of a failure's body an error keeps.
pub const MAX_ERROR_BODY_BYTES: usize = 1_048_576;
/// The ceiling both `Retry-After` forms saturate at, in seconds (SPEC §6 / Appendix A).
pub const MAX_RETRY_AFTER_SECONDS: u32 = 2_147_483_647;

/// Machine-readable error categories, shared with every Basecamp SDK.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[non_exhaustive]
pub enum ErrorCode {
    /// Client misconfiguration: bad arguments, a bad URL.
    Usage,
    /// 404.
    NotFound,
    /// 401, or no credentials to send.
    AuthRequired,
    /// 403.
    Forbidden,
    /// 429.
    RateLimit,
    /// Connection failure, timeout, DNS.
    Network,
    /// A server-side error, or a malformed 2xx body.
    ApiError,
    /// A name matched more than one record.
    Ambiguous,
    /// 400 or 422.
    Validation,
    /// 507: an account limit blocks the request.
    LimitExceeded,
}

impl ErrorCode {
    /// The code as every SDK spells it: `auth_required`, `rate_limit`.
    pub fn as_str(&self) -> &'static str {
        match self {
            ErrorCode::Usage => "usage",
            ErrorCode::NotFound => "not_found",
            ErrorCode::AuthRequired => "auth_required",
            ErrorCode::Forbidden => "forbidden",
            ErrorCode::RateLimit => "rate_limit",
            ErrorCode::Network => "network",
            ErrorCode::ApiError => "api_error",
            ErrorCode::Ambiguous => "ambiguous",
            ErrorCode::Validation => "validation",
            ErrorCode::LimitExceeded => "limit_exceeded",
        }
    }

    /// The process exit status a command should end with.
    pub fn exit_code(&self) -> i32 {
        match self {
            ErrorCode::Usage => 1,
            ErrorCode::NotFound => 2,
            ErrorCode::AuthRequired => 3,
            ErrorCode::Forbidden => 4,
            ErrorCode::RateLimit => 5,
            ErrorCode::Network => 6,
            ErrorCode::ApiError => 7,
            ErrorCode::Ambiguous => 8,
            ErrorCode::Validation => 9,
            ErrorCode::LimitExceeded => 10,
        }
    }
}

impl fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The error every SDK call can answer with: SPEC §6's record, read through accessors.
///
/// It is one pointer wide — the record lives behind a box — so a `Result<T, Error>` costs a
/// caller nothing on the success path.
#[derive(thiserror::Error)]
#[error("{}", self.render())]
pub struct Error {
    inner: Box<Inner>,
    #[source]
    source: Option<Box<dyn std::error::Error + Send + Sync>>,
}

struct Inner {
    code: ErrorCode,
    message: String,
    hint: Option<String>,
    http_status: Option<u16>,
    retryable: bool,
    retry_after: Option<u32>,
    request_id: Option<String>,
    field_errors: Option<BTreeMap<String, Vec<String>>>,
    confirmation_people: Option<Vec<TemplateLibraryConfirmationPerson>>,
    response_too_large: bool,
    deadline_exceeded: bool,
    body: Option<Box<[u8]>>,
}

impl Error {
    /// An error with a code and a message.
    pub fn new(code: ErrorCode, message: impl Into<String>) -> Error {
        Error {
            inner: Box::new(Inner {
                code,
                message: message.into(),
                hint: None,
                http_status: None,
                retryable: false,
                retry_after: None,
                request_id: None,
                field_errors: None,
                confirmation_people: None,
                response_too_large: false,
                deadline_exceeded: false,
                body: None,
            }),
            source: None,
        }
    }

    /// A client-side misuse.
    pub fn usage(message: impl Into<String>) -> Error {
        Error::new(ErrorCode::Usage, message)
    }

    /// A transport failure: no connection, a timeout, a broken stream. Retryable.
    pub fn network(source: impl std::error::Error + Send + Sync + 'static) -> Error {
        Error::new(ErrorCode::Network, "Network error")
            .with_hint(source.to_string())
            .retryable(true)
            .with_source(source)
    }

    /// A transport failure whose cause is not chained, for a request whose URL carries a
    /// credential (SPEC §9): only the origin is named.
    pub fn network_at(origin: &str) -> Error {
        Error::new(
            ErrorCode::Network,
            format!("Network error contacting {origin}"),
        )
        .retryable(true)
    }

    /// A successful status whose body could not be read as the type the operation
    /// declares: statusless, non-retryable `api_error` (SPEC §6).
    pub fn malformed_response(detail: impl Into<String>) -> Error {
        Error::new(
            ErrorCode::ApiError,
            truncate(&format!("malformed response: {}", detail.into())),
        )
    }

    /// A body the client refused to read past its cap.
    pub fn response_too_large(limit: usize) -> Error {
        let mut error = Error::new(
            ErrorCode::ApiError,
            format!("response body exceeds {limit} bytes"),
        );
        error.inner.response_too_large = true;
        error
    }

    /// The operation deadline passed before the call finished.
    pub fn deadline_exceeded(after: std::time::Duration) -> Error {
        let mut error = Error::new(
            ErrorCode::Network,
            format!("operation deadline of {after:?} exceeded"),
        );
        error.inner.deadline_exceeded = true;
        error
    }

    /// Maps a non-2xx response onto SPEC §6's status table, parses the body per its Error
    /// Body Parsing and field-keyed algorithms, and reads `Retry-After` and `X-Request-Id`.
    pub fn from_response(status: StatusCode, headers: &HeaderMap, body: &[u8]) -> Error {
        let status_code = status.as_u16();
        let (code, retryable) = match status_code {
            401 => (ErrorCode::AuthRequired, false),
            403 => (ErrorCode::Forbidden, false),
            404 => (ErrorCode::NotFound, false),
            429 => (ErrorCode::RateLimit, true),
            400 | 422 => (ErrorCode::Validation, false),
            500 | 502 | 503 | 504 => (ErrorCode::ApiError, true),
            507 => (ErrorCode::LimitExceeded, false),
            code if code >= 500 => (ErrorCode::ApiError, true),
            _ => (ErrorCode::ApiError, false),
        };
        let parsed = serde_json::from_slice::<Value>(body).ok();
        let mut message = parsed
            .as_ref()
            .and_then(|value| value.get("error").and_then(Value::as_str))
            .or_else(|| {
                parsed
                    .as_ref()
                    .and_then(|value| value.get("message").and_then(Value::as_str))
            })
            .map_or_else(
                || format!("Request failed (HTTP {status_code})"),
                str::to_string,
            );
        let hint = parsed
            .as_ref()
            .and_then(|value| value.get("error_description").and_then(Value::as_str))
            .map(truncate);

        let field_errors = match (status_code, &parsed) {
            (400 | 422, Some(value)) => field_errors_of(value),
            _ => None,
        };
        if let Some(fields) = &field_errors {
            let flattened = flatten(fields);
            message = if parsed
                .as_ref()
                .is_some_and(|value| value.get("error").is_some() || value.get("message").is_some())
                && !message.starts_with("Request failed (HTTP")
            {
                format!("{message} ({flattened})")
            } else {
                flattened
            };
        }
        let confirmation_people = match (status_code, &parsed) {
            (422, Some(value)) => confirmation_people_of(value),
            _ => None,
        };

        let mut error = Error::new(code, truncate(&message))
            .retryable(retryable)
            .with_status(status_code);
        error.inner.hint = hint;
        error.inner.retry_after = parse_retry_after_header(headers, Utc::now());
        error.inner.request_id = headers
            .get("x-request-id")
            .and_then(|value| value.to_str().ok())
            .map(str::to_string);
        error.inner.field_errors = field_errors;
        error.inner.confirmation_people = confirmation_people;
        if !body.is_empty() {
            let kept = body.len().min(MAX_ERROR_BODY_BYTES);
            error.inner.body = Some(Box::from(&body[..kept]));
        }
        error
    }

    /// The same error with a hint.
    pub fn with_hint(mut self, hint: impl Into<String>) -> Error {
        self.inner.hint = Some(hint.into());
        self
    }

    /// The same error carrying an HTTP status.
    pub fn with_status(mut self, status: u16) -> Error {
        self.inner.http_status = Some(status);
        self
    }

    /// The same error carrying a request id.
    pub fn with_request_id(mut self, request_id: impl Into<String>) -> Error {
        self.inner.request_id = Some(request_id.into());
        self
    }

    /// The same error with a cause.
    pub fn with_source(mut self, source: impl std::error::Error + Send + Sync + 'static) -> Error {
        self.source = Some(Box::new(source));
        self
    }

    /// The same error, retryable or not.
    pub fn retryable(mut self, retryable: bool) -> Error {
        self.inner.retryable = retryable;
        self
    }

    /// The categorical code.
    pub fn code(&self) -> ErrorCode {
        self.inner.code
    }

    /// The human-readable description, at most [`MAX_ERROR_MESSAGE_LENGTH`] characters.
    pub fn message(&self) -> &str {
        &self.inner.message
    }

    /// Resolution guidance, when there is any.
    pub fn hint(&self) -> Option<&str> {
        self.inner.hint.as_deref()
    }

    /// The HTTP status that caused the error, when one did.
    pub fn http_status(&self) -> Option<u16> {
        self.inner.http_status
    }

    /// Whether the operation can be retried.
    pub fn is_retryable(&self) -> bool {
        self.inner.retryable
    }

    /// The seconds `Retry-After` named, parsed per SPEC §6, at whatever status carried it.
    pub fn retry_after(&self) -> Option<u32> {
        self.inner.retry_after
    }

    /// `X-Request-Id`, when the response carried one.
    pub fn request_id(&self) -> Option<&str> {
        self.inner.request_id.as_deref()
    }

    /// The structured 400/422 field messages, raw and untruncated. Row-keyed bodies land
    /// here keyed by email address, then `index`, then position.
    pub fn field_errors(&self) -> Option<&BTreeMap<String, Vec<String>>> {
        self.inner.field_errors.as_ref()
    }

    /// The people a template-library copy needs confirmed, when a 422 named them.
    pub fn confirmation_people(&self) -> Option<&[TemplateLibraryConfirmationPerson]> {
        self.inner.confirmation_people.as_deref()
    }

    /// The process exit status a command should end with.
    pub fn exit_code(&self) -> i32 {
        self.inner.code.exit_code()
    }

    /// The failure body, up to [`MAX_ERROR_BODY_BYTES`], when the server sent one.
    pub fn body(&self) -> Option<&[u8]> {
        self.inner.body.as_deref()
    }

    /// The answer was longer than the client will hold.
    pub fn is_response_too_large(&self) -> bool {
        self.inner.response_too_large
    }

    /// The operation deadline passed.
    pub fn is_deadline_exceeded(&self) -> bool {
        self.inner.deadline_exceeded
    }

    fn render(&self) -> String {
        match &self.inner.hint {
            Some(hint) => format!("{}: {hint}", self.inner.message),
            None => self.inner.message.clone(),
        }
    }
}

impl fmt::Debug for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Error")
            .field("code", &self.inner.code)
            .field("message", &self.inner.message)
            .field("hint", &self.inner.hint)
            .field("http_status", &self.inner.http_status)
            .field("retryable", &self.inner.retryable)
            .field("retry_after", &self.inner.retry_after)
            .field("request_id", &self.inner.request_id)
            .field("field_errors", &self.inner.field_errors)
            .finish_non_exhaustive()
    }
}

/// Cuts a message to [`MAX_ERROR_MESSAGE_LENGTH`] characters, ending it in `...` when it was
/// longer. The unit is the character, a language adaptation SPEC §9 allows.
pub fn truncate(message: &str) -> String {
    if message.chars().count() <= MAX_ERROR_MESSAGE_LENGTH {
        message.to_string()
    } else {
        let kept: String = message.chars().take(MAX_ERROR_MESSAGE_LENGTH - 3).collect();
        format!("{kept}...")
    }
}

/// SPEC §6's field-keyed algorithm, steps 1, 1b and 2.
fn field_errors_of(body: &Value) -> Option<BTreeMap<String, Vec<String>>> {
    let object = body.as_object()?;
    match object.get("errors") {
        Some(Value::Object(fields)) => {
            let mut map = BTreeMap::new();
            for (field, messages) in fields {
                if let Some(messages) = messages.as_array() {
                    let kept: Vec<String> = messages
                        .iter()
                        .filter_map(Value::as_str)
                        .map(str::to_string)
                        .collect();
                    if !kept.is_empty() {
                        map.insert(field.clone(), kept);
                    }
                }
            }
            (!map.is_empty()).then_some(map)
        }
        Some(Value::Array(rows)) => row_keyed(rows),
        Some(_) => None,
        None => {
            if object.is_empty() {
                return None;
            }
            let mut map = BTreeMap::new();
            for (field, messages) in object {
                let messages = messages.as_array()?;
                if messages.is_empty() {
                    return None;
                }
                let mut kept = Vec::new();
                for message in messages {
                    match message.as_str() {
                        Some(text) if !text.is_empty() => kept.push(text.to_string()),
                        _ => return None,
                    }
                }
                map.insert(field.clone(), kept);
            }
            Some(map)
        }
    }
}

/// Step 1b: every element an object whose `messages` holds at least one non-empty string,
/// keyed by `email_address`, then `index`, then position.
fn row_keyed(rows: &[Value]) -> Option<BTreeMap<String, Vec<String>>> {
    if rows.is_empty() {
        return None;
    }
    let mut map: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for (position, row) in rows.iter().enumerate() {
        let object = row.as_object()?;
        let messages: Vec<String> = object
            .get("messages")?
            .as_array()?
            .iter()
            .filter_map(Value::as_str)
            .filter(|text| !text.is_empty())
            .map(str::to_string)
            .collect();
        if messages.is_empty() {
            return None;
        }
        let key = match object.get("email_address").and_then(Value::as_str) {
            Some(address) if !address.is_empty() => address.to_string(),
            _ => match object.get("index").and_then(Value::as_i64) {
                Some(index) => index.to_string(),
                None => position.to_string(),
            },
        };
        map.entry(key).or_default().extend(messages);
    }
    Some(map)
}

/// `field: m1; m2, other: m3`, fields sorted.
fn flatten(fields: &BTreeMap<String, Vec<String>>) -> String {
    fields
        .iter()
        .map(|(field, messages)| format!("{field}: {}", messages.join("; ")))
        .collect::<Vec<_>>()
        .join(", ")
}

fn confirmation_people_of(body: &Value) -> Option<Vec<TemplateLibraryConfirmationPerson>> {
    let people = body.get("people")?.as_array()?;
    if people.is_empty() {
        return None;
    }
    let mut result = Vec::new();
    for person in people {
        let object = person.as_object()?;
        let id = object.get("id")?.as_i64()?;
        let name = object.get("name")?.as_str()?;
        let avatar_url = object.get("avatar_url")?.as_str()?;
        if id <= 0 || name.is_empty() || avatar_url.is_empty() {
            return None;
        }
        result.push(serde_json::from_value(person.clone()).ok()?);
    }
    Some(result)
}

/// SPEC §6's `Retry-After` table: `1*DIGIT` (no sign, no fraction) saturating at
/// [`MAX_RETRY_AFTER_SECONDS`], with `0` falling through; or an HTTP-date, rounded up to
/// whole seconds from `now` and saturating at the same ceiling, with a past date falling
/// through. Anything else is no value.
pub fn parse_retry_after(value: &str, now: DateTime<Utc>) -> Option<u32> {
    let value = value.trim_matches([' ', '\t']);
    if value.is_empty() {
        return None;
    }
    if value.bytes().all(|b| b.is_ascii_digit()) {
        let seconds = match value.parse::<u64>() {
            Ok(seconds) => seconds.min(u64::from(MAX_RETRY_AFTER_SECONDS)),
            Err(_) => u64::from(MAX_RETRY_AFTER_SECONDS),
        };
        return (seconds > 0).then(|| u32::try_from(seconds).unwrap_or(MAX_RETRY_AFTER_SECONDS));
    }
    let until = parse_http_date(value)?;
    let left = until.signed_duration_since(now);
    let millis = left.num_milliseconds();
    if millis <= 0 {
        return None;
    }
    let seconds = (millis + 999) / 1000;
    Some(
        u32::try_from(seconds)
            .unwrap_or(MAX_RETRY_AFTER_SECONDS)
            .min(MAX_RETRY_AFTER_SECONDS),
    )
}

/// IMF-fixdate, then the obsolete RFC 850 and asctime forms.
fn parse_http_date(value: &str) -> Option<DateTime<Utc>> {
    if let Ok(date) = DateTime::parse_from_rfc2822(value) {
        return Some(date.with_timezone(&Utc));
    }
    let collapsed: String = value.split_whitespace().collect::<Vec<_>>().join(" ");
    for format in ["%A, %d-%b-%y %H:%M:%S GMT", "%a %b %d %H:%M:%S %Y"] {
        if let Ok(naive) = chrono::NaiveDateTime::parse_from_str(&collapsed, format) {
            return Some(naive.and_utc());
        }
    }
    None
}

/// The `Retry-After` header, parsed.
pub(crate) fn parse_retry_after_header(headers: &HeaderMap, now: DateTime<Utc>) -> Option<u32> {
    headers
        .get("retry-after")
        .and_then(|value| value.to_str().ok())
        .and_then(|value| parse_retry_after(value, now))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::http::HeaderValue;

    fn at(seconds: i64) -> DateTime<Utc> {
        DateTime::from_timestamp(1_700_000_000 + seconds, 0).unwrap()
    }

    fn headers(pairs: &[(&str, &str)]) -> HeaderMap {
        let mut headers = HeaderMap::new();
        for (name, value) in pairs {
            headers.insert(
                crate::http::HeaderName::from_bytes(name.as_bytes()).unwrap(),
                HeaderValue::from_str(value).unwrap(),
            );
        }
        headers
    }

    #[test]
    fn retry_after_digits_saturate_and_zero_falls_through() {
        assert_eq!(parse_retry_after("5", at(0)), Some(5));
        assert_eq!(parse_retry_after(" 5 ", at(0)), Some(5));
        assert_eq!(parse_retry_after("0", at(0)), None);
        assert_eq!(parse_retry_after("+5", at(0)), None);
        assert_eq!(parse_retry_after("-5", at(0)), None);
        assert_eq!(parse_retry_after("2.5", at(0)), None);
        assert_eq!(parse_retry_after("5s", at(0)), None);
        assert_eq!(parse_retry_after("2147483647", at(0)), Some(2_147_483_647));
        assert_eq!(parse_retry_after("2147483648", at(0)), Some(2_147_483_647));
        assert_eq!(
            parse_retry_after("99999999999999999999999999", at(0)),
            Some(2_147_483_647)
        );
        assert_eq!(parse_retry_after("", at(0)), None);
    }

    #[test]
    fn retry_after_dates_round_up_and_past_dates_fall_through() {
        let now = DateTime::parse_from_rfc3339("2021-06-09T10:18:14.250Z")
            .unwrap()
            .with_timezone(&Utc);
        assert_eq!(
            parse_retry_after("Wed, 09 Jun 2021 10:18:16 GMT", now),
            Some(2)
        );
        assert_eq!(
            parse_retry_after("Wed, 09 Jun 2021 10:18:15 GMT", now),
            Some(1)
        );
        assert_eq!(
            parse_retry_after("Wed, 09 Jun 2021 10:18:14 GMT", now),
            None
        );
        assert_eq!(
            parse_retry_after("Wed, 09 Jun 2021 09:00:00 GMT", now),
            None
        );
        assert_eq!(
            parse_retry_after("Wednesday, 09-Jun-21 10:18:16 GMT", now),
            Some(2)
        );
        assert_eq!(parse_retry_after("Wed Jun  9 10:18:16 2021", now), Some(2));
        assert_eq!(
            parse_retry_after("Tue, 31 Dec 2999 00:00:00 GMT", now),
            Some(2_147_483_647)
        );
        assert_eq!(parse_retry_after("not a date", now), None);
    }

    #[test]
    fn statuses_map_to_codes_in_order() {
        let cases = [
            (401, ErrorCode::AuthRequired, false),
            (403, ErrorCode::Forbidden, false),
            (404, ErrorCode::NotFound, false),
            (429, ErrorCode::RateLimit, true),
            (400, ErrorCode::Validation, false),
            (422, ErrorCode::Validation, false),
            (500, ErrorCode::ApiError, true),
            (502, ErrorCode::ApiError, true),
            (503, ErrorCode::ApiError, true),
            (504, ErrorCode::ApiError, true),
            (507, ErrorCode::LimitExceeded, false),
            (599, ErrorCode::ApiError, true),
            (418, ErrorCode::ApiError, false),
        ];
        for (status, code, retryable) in cases {
            let error = Error::from_response(
                StatusCode::from_u16(status).unwrap(),
                &headers(&[("x-request-id", "req-1")]),
                b"",
            );
            assert_eq!(error.code(), code, "{status}");
            assert_eq!(error.is_retryable(), retryable, "{status}");
            assert_eq!(error.http_status(), Some(status));
            assert_eq!(error.request_id(), Some("req-1"));
            assert_eq!(error.message(), format!("Request failed (HTTP {status})"));
        }
    }

    #[test]
    fn retry_after_is_read_at_every_status() {
        let error = Error::from_response(
            StatusCode::SERVICE_UNAVAILABLE,
            &headers(&[("retry-after", "7")]),
            b"{}",
        );
        assert_eq!(error.retry_after(), Some(7));
        let error = Error::from_response(
            StatusCode::NOT_FOUND,
            &headers(&[("retry-after", "3")]),
            b"",
        );
        assert_eq!(error.retry_after(), Some(3));
    }

    #[test]
    fn bodies_supply_message_and_hint() {
        let error = Error::from_response(
            StatusCode::UNAUTHORIZED,
            &headers(&[]),
            br#"{"error": "Unauthorized", "error_description": "token expired"}"#,
        );
        assert_eq!(error.message(), "Unauthorized");
        assert_eq!(error.hint(), Some("token expired"));
        let error = Error::from_response(
            StatusCode::NOT_FOUND,
            &headers(&[]),
            br#"{"message": "Not found"}"#,
        );
        assert_eq!(error.message(), "Not found");
        let error = Error::from_response(StatusCode::NOT_FOUND, &headers(&[]), b"<html>");
        assert_eq!(error.message(), "Request failed (HTTP 404)");
        let long = format!(r#"{{"error": "{}"}}"#, "x".repeat(600));
        let error = Error::from_response(StatusCode::BAD_REQUEST, &headers(&[]), long.as_bytes());
        assert_eq!(error.message().chars().count(), 500);
        assert!(error.message().ends_with("..."));
    }

    #[test]
    fn field_keyed_bodies_are_flattened_and_exposed() {
        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"error": "Validation failed", "errors": {"color": ["is not a valid color"], "name": ["can't be blank", "is too short"]}}"#,
        );
        assert_eq!(error.code(), ErrorCode::Validation);
        assert_eq!(
            error.message(),
            "Validation failed (color: is not a valid color, name: can't be blank; is too short)"
        );
        assert_eq!(
            error.field_errors().unwrap()["name"],
            vec!["can't be blank", "is too short"]
        );

        let error = Error::from_response(
            StatusCode::BAD_REQUEST,
            &headers(&[]),
            br#"{"payload_url": ["is not a valid URL"]}"#,
        );
        assert_eq!(error.message(), "payload_url: is not a valid URL");
        assert_eq!(error.field_errors().unwrap().len(), 1);

        let error = Error::from_response(
            StatusCode::BAD_REQUEST,
            &headers(&[]),
            br#"{"error": "Webhook is invalid", "payload_url": ["is invalid"]}"#,
        );
        assert_eq!(error.message(), "Webhook is invalid");
        assert!(error.field_errors().is_none());

        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"error": {}, "errors": {"base": ["Can't be undocked"], "__proto__": ["x"]}}"#,
        );
        assert_eq!(error.message(), "__proto__: x, base: Can't be undocked");

        let error = Error::from_response(
            StatusCode::NOT_FOUND,
            &headers(&[]),
            br#"{"errors": {"x": ["y"]}}"#,
        );
        assert!(error.field_errors().is_none());
    }

    #[test]
    fn row_keyed_bodies_land_on_addresses_indexes_then_positions() {
        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"errors": [{"email_address": "not-an-address", "messages": ["Email address must be valid"]}, {"email_address": null, "messages": ["Email address can't be blank"]}, {"index": 7, "messages": ["Name can't be blank"]}, {"email_address": "not-an-address", "messages": ["Already invited"]}]}"#,
        );
        let fields = error.field_errors().unwrap();
        assert_eq!(
            fields["not-an-address"],
            vec!["Email address must be valid", "Already invited"]
        );
        assert_eq!(fields["1"], vec!["Email address can't be blank"]);
        assert_eq!(fields["7"], vec!["Name can't be blank"]);
        assert_eq!(
            error.message(),
            "1: Email address can't be blank, 7: Name can't be blank, not-an-address: Email address must be valid; Already invited"
        );

        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"errors": [{"email_address": "a", "messages": ["x"]}, {"email_address": "b"}]}"#,
        );
        assert!(error.field_errors().is_none());
        assert_eq!(error.message(), "Request failed (HTTP 422)");
    }

    #[test]
    fn confirmation_people_are_projected_only_when_every_entry_conforms() {
        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"error": "Confirmation required", "people": [{"id": 1, "name": "Ann", "avatar_url": "https://x/a.png"}]}"#,
        );
        assert_eq!(error.code(), ErrorCode::Validation);
        assert_eq!(error.confirmation_people().unwrap().len(), 1);
        assert_eq!(error.confirmation_people().unwrap()[0].id, 1);

        let error = Error::from_response(
            StatusCode::UNPROCESSABLE_ENTITY,
            &headers(&[]),
            br#"{"error": "x", "people": [{"id": 0, "name": "Ann", "avatar_url": "u"}]}"#,
        );
        assert!(error.confirmation_people().is_none());
    }

    #[test]
    fn errors_are_send_and_sync() {
        fn assert_send_sync<T: Send + Sync>() {}
        assert_send_sync::<Error>();
    }
}
