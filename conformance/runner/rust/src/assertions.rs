//! The 22 assertion types of `conformance/schema.json`, with the Go runner's semantics:
//! per-request `index` (negative from the end; past-end fails, never vacuous), an
//! `errorRaised` that is code-agnostic, and a `requestCount` that the `link-header` tag
//! suppresses without shedding the rest of the case.

use std::time::Duration;

use basecamp_sdk::Error;
use basecamp_sdk::http::HeaderMap;
use serde_json::Value;

use crate::fixtures::{Assertion, TestCase};
use crate::operations::Outcome;
use crate::transport::Recorded;

pub struct Run<'a> {
    pub case: &'a TestCase,
    pub outcome: &'a Result<Outcome, Error>,
    pub recorded: &'a Recorded,
}

pub fn check_all(run: &Run) -> Result<(), String> {
    // Implicit method invariant: the transport answers any verb, so a wrong-verb request
    // would consume a queued response silently. When the fixture declares a method and
    // carries no explicit requestMethod assertion, the first request must use it.
    let has_method_assertion = run
        .case
        .assertions
        .iter()
        .any(|a| a.kind == "requestMethod");
    if !run.case.method.is_empty()
        && !has_method_assertion
        && let Some(first) = run.recorded.methods.first()
        && !first.eq_ignore_ascii_case(&run.case.method)
    {
        return Err(format!(
            "Expected first request method {:?}, got {first:?}",
            run.case.method.to_uppercase()
        ));
    }
    for assertion in &run.case.assertions {
        check(run, assertion)?;
    }
    Ok(())
}

pub const LINK_HEADER_TAG: &str = "link-header";

/// Whether a `requestCount` applies: the SDK auto-paginates, so a fixture that counts
/// first-page requests only is inapplicable — but ONLY its count is (#573).
pub fn request_count_applies(tags: &[String]) -> bool {
    !tags.iter().any(|tag| tag == LINK_HEADER_TAG)
}

pub fn check_request_count(actual: usize, expected: i64) -> Option<String> {
    (i64::try_from(actual).ok() != Some(expected))
        .then(|| format!("Expected {expected} requests, got {actual}"))
}

/// The inverse of `noError`, deliberately code-agnostic. Kept apart so its failing branch
/// is unit-testable: no committed fixture can reach it.
pub fn error_raised_failure(dispatch_failed: bool) -> Option<String> {
    (!dispatch_failed).then(|| "Expected the call to fail, but it succeeded".to_string())
}

/// The `delayBetweenRequests` contract. With an `index`, exactly that gap must exist and
/// hold; without one, at least one gap must exist and every gap must hold. Fewer requests
/// than the assertion needs is a failure, never a vacuous pass (#563).
pub fn check_delay_gaps(
    times: &[std::time::Instant],
    min: Duration,
    index: Option<i64>,
) -> Option<String> {
    let gaps = times.len().saturating_sub(1);
    if let Some(gap) = index {
        if gap < 0 {
            return Some(format!(
                "delayBetweenRequests gap index must be non-negative, got {gap}"
            ));
        }
        let gap = usize::try_from(gap).unwrap_or(usize::MAX);
        if gap >= gaps {
            return Some(format!(
                "Expected a delay at gap {gap}, but only {} request(s) were made",
                times.len()
            ));
        }
        let delay = times[gap + 1].duration_since(times[gap]);
        return (delay < min)
            .then(|| format!("Expected delay >= {min:?} at gap {gap}, got {delay:?}"));
    }
    if gaps < 1 {
        return Some(format!(
            "Expected a delay between requests, but only {} request(s) were made",
            times.len()
        ));
    }
    for i in 0..gaps {
        let delay = times[i + 1].duration_since(times[i]);
        if delay < min {
            return Some(format!(
                "Expected delay >= {min:?} at gap {i}, got {delay:?}"
            ));
        }
    }
    None
}

/// Normalizes a request index against `n` captured requests: negative counts from the end.
pub fn resolve_index(index: i64, n: usize) -> Option<usize> {
    let n = i64::try_from(n).ok()?;
    let resolved = if index < 0 { index + n } else { index };
    (0..n)
        .contains(&resolved)
        .then(|| usize::try_from(resolved).ok())
        .flatten()
}

/// A fixture `min` in milliseconds as a duration: fractional and negative values are
/// schema-legal, so the floor is taken and a negative floor is no floor.
fn min_millis(min: f64) -> u64 {
    if min.is_finite() && min > 0.0 {
        // f64 -> u64 is saturating in Rust; a value past u64::MAX is not a real bound.
        #[allow(clippy::cast_possible_truncation, clippy::cast_sign_loss)]
        {
            min.floor() as u64
        }
    } else {
        0
    }
}

fn index_of(assertion: &Assertion) -> i64 {
    assertion.index.unwrap_or(0)
}

fn check(run: &Run, assertion: &Assertion) -> Result<(), String> {
    let recorded = run.recorded;
    let sdk_error = run.outcome.as_ref().err();
    match assertion.kind.as_str() {
        "requestCount" => {
            if !request_count_applies(&run.case.tags) {
                return Ok(());
            }
            check_request_count(recorded.count(), expected_int(assertion)?).map_or(Ok(()), Err)
        }
        "delayBetweenRequests" => check_delay_gaps(
            &recorded.times,
            Duration::from_millis(min_millis(assertion.min)),
            assertion.index,
        )
        .map_or(Ok(()), Err),
        "noError" => match sdk_error {
            None => Ok(()),
            Some(error) => Err(format!("Expected no error, got: {error}")),
        },
        "errorRaised" => error_raised_failure(sdk_error.is_some()).map_or(Ok(()), Err),
        "errorType" | "errorCode" => {
            let expected = expected_string(assertion)?;
            let Some(error) = sdk_error else {
                return Err(format!(
                    "Expected error {} {expected:?}, but got no error",
                    assertion.kind
                ));
            };
            let actual = error.code().as_str();
            if actual == expected {
                Ok(())
            } else {
                Err(format!(
                    "Expected error {} {expected:?}, got {actual:?} ({error})",
                    assertion.kind
                ))
            }
        }
        "statusCode" | "responseStatus" => {
            let expected = expected_int(assertion)?;
            match sdk_error {
                Some(error) => {
                    let actual = i64::from(error.http_status().unwrap_or(0));
                    if actual == expected {
                        Ok(())
                    } else {
                        Err(format!("Expected status code {expected}, got {actual}"))
                    }
                }
                None if expected >= 400 => Err(format!(
                    "Expected error with status {expected}, but operation succeeded"
                )),
                None => Ok(()),
            }
        }
        "responseBody" => {
            let path = &assertion.path;
            let body = match run.outcome {
                Err(error) => return Err(format!("Expected responseBody.{path}, got: {error}")),
                Ok(outcome) => outcome.body().ok_or_else(|| {
                    format!("Expected responseBody.{path}, but no result returned")
                })?,
            };
            let actual = lookup(body, path)
                .ok_or_else(|| format!("Expected responseBody.{path}, but field not present"))?;
            compare_values(&format!("responseBody.{path}"), &assertion.expected, actual)
        }
        "requestPath" => {
            let expected = expected_string(assertion)?;
            let (i, index) = pick(assertion, recorded.paths.len(), || {
                format!("Expected request path {expected:?}")
            })?;
            let actual = &recorded.paths[i];
            if actual == expected {
                Ok(())
            } else {
                Err(format!(
                    "Expected request path {expected:?} on request index {index}, got {actual:?}"
                ))
            }
        }
        "requestMethod" => {
            let expected = expected_string(assertion)?;
            let (i, index) = pick(assertion, recorded.methods.len(), || {
                format!("Expected request method {expected:?}")
            })?;
            let actual = &recorded.methods[i];
            if actual == expected {
                Ok(())
            } else {
                Err(format!(
                    "Expected request method {expected:?} on request index {index}, got {actual:?}"
                ))
            }
        }
        "requestBody" => {
            // Path names one key; empty, `expected` is the WHOLE body, compared
            // exactly, so a key the SDK added fails rather than slipping past.
            let path = &assertion.path;
            let what = if path.is_empty() {
                "request body".to_string()
            } else {
                format!("request body field {path:?}")
            };
            let (i, index) = pick(assertion, recorded.bodies.len(), || {
                format!("Expected {what}")
            })?;
            let Some(body) = &recorded.bodies[i] else {
                return Err(format!(
                    "Expected {what} on request index {index}, but request had no JSON body"
                ));
            };
            if path.is_empty() {
                return if assertion.expected == *body {
                    Ok(())
                } else {
                    Err(format!(
                        "Expected request body on request index {index} to equal {} exactly, got {body}",
                        assertion.expected
                    ))
                };
            }
            let actual = lookup(body, path).ok_or_else(|| {
                format!("Expected request body field {path:?} on request index {index}, but it was absent")
            })?;
            // Canonical JSON equality, as the Go runner's jsonEqual: a body that sent
            // "42" where the fixture says 42 is a wrong body, not a spelling.
            if assertion.expected == *actual {
                Ok(())
            } else {
                Err(format!(
                    "Expected request body {path} = {} on request index {index}, got {}",
                    assertion.expected, actual
                ))
            }
        }
        "requestBodyAbsent" => {
            let path = &assertion.path;
            let (i, index) = pick(assertion, recorded.bodies.len(), || {
                format!("Expected request body field {path:?} absent")
            })?;
            match recorded.bodies[i]
                .as_ref()
                .and_then(|body| lookup(body, path))
            {
                None => Ok(()),
                Some(actual) => Err(format!(
                    "Expected request body field {path:?} absent on request index {index}, got {actual}"
                )),
            }
        }
        "errorMessage" => {
            let expected = expected_string(assertion)?;
            let Some(error) = sdk_error else {
                return Err(format!(
                    "Expected error message containing {expected:?}, but got no error"
                ));
            };
            let message = error.to_string();
            if message.contains(expected) {
                Ok(())
            } else {
                Err(format!(
                    "Expected error message containing {expected:?}, got {message:?}"
                ))
            }
        }
        "errorField" => {
            let path = &assertion.path;
            let Some(error) = sdk_error else {
                return Err(format!("Expected error field {path}, but got no error"));
            };
            let actual = match path.as_str() {
                "httpStatus" => Value::from(error.http_status().unwrap_or(0)),
                "retryable" => Value::from(error.is_retryable()),
                "code" => Value::from(error.code().as_str()),
                "message" => Value::from(error.message()),
                "requestId" => Value::from(error.request_id().unwrap_or_default()),
                // Absent is JSON null, as Go's *int and Python's None render it; the
                // fixture pins a value only where a Retry-After was served.
                "retryAfter" => error.retry_after().map_or(Value::Null, Value::from),
                "confirmationPeople.0.id" => {
                    let Some(first) = error.confirmation_people().and_then(<[_]>::first) else {
                        return Err("Expected error.confirmationPeople.0.id, but confirmation people were absent".to_string());
                    };
                    Value::from(first.id)
                }
                other => return Err(format!("Unknown error field: {other}")),
            };
            compare_values(&format!("error.{path}"), &assertion.expected, &actual)
        }
        "headerInjected" => {
            let name = &assertion.path;
            let expected = expected_string(assertion)?;
            let (i, index) = pick(assertion, recorded.headers.len(), || {
                format!("Expected header {name}={expected:?}")
            })?;
            let actual = header_value(&recorded.headers[i], name);
            if actual == expected {
                Ok(())
            } else {
                Err(format!(
                    "Expected header {name}={expected:?} on request index {index}, got {actual:?}"
                ))
            }
        }
        "headerPresent" => {
            let name = &assertion.path;
            let (i, index) = pick(assertion, recorded.headers.len(), || {
                format!("Expected header {name}")
            })?;
            if header_value(&recorded.headers[i], name).is_empty() {
                Err(format!(
                    "Expected header {name} on request index {index}, but it was empty or missing"
                ))
            } else {
                Ok(())
            }
        }
        "headerAbsent" => {
            let name = &assertion.path;
            let (i, index) = pick(assertion, recorded.headers.len(), || {
                format!("Expected header {name} absent")
            })?;
            // Present-with-empty-value must fail an absence assertion too, so this counts
            // entries rather than reading a value.
            let values: Vec<_> = recorded.headers[i].get_all(name).iter().collect();
            if values.is_empty() {
                Ok(())
            } else {
                Err(format!(
                    "Expected header {name} absent on request index {index}, got {values:?}"
                ))
            }
        }
        "headerValue" => {
            // Checks the fixture's own first mock response, as the Go runner does; SDK-parsed
            // values are what responseMeta is for.
            let name = &assertion.path;
            let expected = expected_string(assertion)?;
            let Some(first) = run.case.mock_responses.first() else {
                return Err(format!(
                    "Expected response header {name}={expected:?}, but no mock responses defined"
                ));
            };
            let actual = first
                .headers
                .iter()
                .find(|(header, _)| header.eq_ignore_ascii_case(name))
                .map(|(_, value)| value.as_str())
                .unwrap_or_default();
            if actual == expected {
                Ok(())
            } else {
                Err(format!(
                    "Expected response header {name}={expected:?}, got {actual:?}"
                ))
            }
        }
        "responseMeta" => {
            let path = &assertion.path;
            let meta = match run.outcome {
                Ok(outcome) => outcome.meta(),
                Err(error) => return Err(format!("Expected response meta {path}, got: {error}")),
            };
            let Some(meta) = meta else {
                return Err(format!(
                    "Expected response meta {path}, but no metadata returned"
                ));
            };
            let actual = meta.get(path).ok_or_else(|| {
                format!("Expected response meta {path}, but field not present in metadata")
            })?;
            compare_values(&format!("meta.{path}"), &assertion.expected, actual)
        }
        "requestScheme" => {
            let expected = expected_string(assertion)?;
            if expected == "https" && sdk_error.is_none() {
                Err("Expected HTTPS enforcement error, but request succeeded over HTTP".to_string())
            } else {
                Ok(())
            }
        }
        "urlOrigin" => {
            let expected = expected_string(assertion)?;
            if expected != "rejected" {
                Ok(())
            } else if recorded.count() > 1 {
                Err(format!(
                    "Expected cross-origin URL rejection (1 request), but {} requests were made",
                    recorded.count()
                ))
            } else if sdk_error.is_none() {
                Err("Expected cross-origin URL rejection, but the operation succeeded".to_string())
            } else {
                Ok(())
            }
        }
        kind => Err(format!("Unknown assertion type: {kind}")),
    }
}

fn pick(
    assertion: &Assertion,
    n: usize,
    what: impl Fn() -> String,
) -> Result<(usize, i64), String> {
    let index = index_of(assertion);
    resolve_index(index, n).map(|i| (i, index)).ok_or_else(|| {
        format!(
            "{} on request index {index}, but only {n} requests were recorded",
            what()
        )
    })
}

fn header_value<'a>(headers: &'a HeaderMap, name: &str) -> &'a str {
    headers
        .get(name)
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default()
}

fn expected_int(assertion: &Assertion) -> Result<i64, String> {
    match &assertion.expected {
        Value::Number(n) => n
            .as_i64()
            .or_else(|| n.as_f64().and_then(integral))
            .ok_or_else(|| {
                format!(
                    "{}: expected an integer, got {}",
                    assertion.kind, assertion.expected
                )
            }),
        other => Err(format!(
            "{}: expected an integer, got {other}",
            assertion.kind
        )),
    }
}

/// An integral float as the integer it spells; anything fractional or out of range is not
/// an integer the fixture meant.
fn integral(f: f64) -> Option<i64> {
    if f.is_finite() && f.fract() == 0.0 && f.abs() < 9_007_199_254_740_992.0 {
        #[allow(clippy::cast_possible_truncation)]
        Some(f as i64)
    } else {
        None
    }
}

fn expected_string(assertion: &Assertion) -> Result<&str, String> {
    match &assertion.expected {
        Value::String(text) => Ok(text),
        other => Err(format!(
            "{}: expected a string, got {other}",
            assertion.kind
        )),
    }
}

/// Walks a JSON value by a dot-separated path. A literal key is tried first, so a
/// top-level key containing a dot still resolves; then each segment descends, reading
/// integer segments as array indexes.
pub fn lookup<'a>(value: &'a Value, path: &str) -> Option<&'a Value> {
    if let Value::Object(fields) = value
        && let Some(direct) = fields.get(path)
    {
        return Some(direct);
    }
    let mut current = value;
    for segment in path.split('.') {
        current = match current {
            Value::Object(fields) => fields.get(segment)?,
            Value::Array(items) => items.get(segment.parse::<usize>().ok()?)?,
            _ => return None,
        };
    }
    Some(current)
}

/// Compares an expected fixture value with a value read off a decoded model or an error
/// field — integers exactly (large ids included), floats numerically, and a number or
/// bool against its string rendering, as the Go runner's compareValues does for values
/// that arrive through `%v`. Request bodies are NOT compared this way; see requestBody.
pub fn compare_values(label: &str, expected: &Value, actual: &Value) -> Result<(), String> {
    if json_equal(expected, actual) {
        Ok(())
    } else {
        Err(format!("Expected {label} = {expected}, got {actual}"))
    }
}

pub fn json_equal(expected: &Value, actual: &Value) -> bool {
    match (expected, actual) {
        (Value::Number(a), Value::Number(b)) => {
            if let (Some(a), Some(b)) = (a.as_i64(), b.as_i64()) {
                a == b
            } else if let (Some(a), Some(b)) = (a.as_u64(), b.as_u64()) {
                a == b
            } else {
                a.as_f64() == b.as_f64()
            }
        }
        (Value::Number(a), Value::String(b)) | (Value::String(b), Value::Number(a)) => {
            a.to_string() == *b
        }
        (Value::String(a), Value::Bool(b)) | (Value::Bool(b), Value::String(a)) => {
            a == &b.to_string()
        }
        _ => expected == actual,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    use std::time::Instant;

    #[test]
    fn index_resolves_negative_from_the_end_and_refuses_past_end() {
        assert_eq!(resolve_index(0, 3), Some(0));
        assert_eq!(resolve_index(-1, 3), Some(2));
        assert_eq!(resolve_index(3, 3), None);
        assert_eq!(resolve_index(-4, 3), None);
        assert_eq!(resolve_index(0, 0), None);
    }

    fn times(gaps_ms: &[u64]) -> Vec<Instant> {
        let start = Instant::now();
        let mut out = vec![start];
        let mut at = start;
        for gap in gaps_ms {
            at += Duration::from_millis(*gap);
            out.push(at);
        }
        out
    }

    #[test]
    fn delay_gaps_never_pass_vacuously() {
        let one_request = times(&[]);
        assert!(
            check_delay_gaps(&one_request, Duration::from_millis(10), None)
                .unwrap()
                .contains("only 1 request(s)")
        );
        assert!(
            check_delay_gaps(&one_request, Duration::from_millis(10), Some(0))
                .unwrap()
                .contains("only 1 request(s)")
        );
        assert!(check_delay_gaps(&times(&[50]), Duration::from_millis(10), Some(1)).is_some());
        assert!(check_delay_gaps(&times(&[50]), Duration::from_millis(10), Some(-1)).is_some());
    }

    #[test]
    fn delay_gaps_hold_every_gap_without_an_index_and_one_with() {
        let t = times(&[50, 5]);
        assert!(
            check_delay_gaps(&t, Duration::from_millis(10), None)
                .unwrap()
                .contains("gap 1")
        );
        assert_eq!(
            check_delay_gaps(&t, Duration::from_millis(10), Some(0)),
            None
        );
        assert!(check_delay_gaps(&t, Duration::from_millis(10), Some(1)).is_some());
    }

    #[test]
    fn error_raised_fails_only_on_success() {
        assert_eq!(error_raised_failure(true), None);
        assert!(error_raised_failure(false).is_some());
    }

    #[test]
    fn request_count_is_exact_and_suppressed_only_by_the_link_header_tag() {
        assert_eq!(check_request_count(2, 2), None);
        assert!(check_request_count(3, 2).is_some());
        assert!(request_count_applies(&["pagination".to_string()]));
        assert!(!request_count_applies(&["link-header".to_string()]));
    }

    #[test]
    fn request_bodies_compare_by_canonical_json() {
        let case: TestCase = serde_json::from_value(json!({
            "name": "a", "method": "PUT",
            "assertions": [{"type": "requestBody", "path": "position", "expected": 42}]
        }))
        .unwrap();
        let mut recorded = Recorded::default();
        recorded.methods.push("PUT".into());
        recorded.bodies.push(Some(json!({"position": "42"})));
        let run = Run {
            case: &case,
            outcome: &Ok(Outcome::Unit),
            recorded: &recorded,
        };
        assert!(
            check_all(&run)
                .unwrap_err()
                .contains("Expected request body position = 42")
        );
        recorded.bodies[0] = Some(json!({"position": 42}));
        let run = Run {
            case: &case,
            outcome: &Ok(Outcome::Unit),
            recorded: &recorded,
        };
        assert_eq!(check_all(&run), Ok(()));
    }

    #[test]
    fn a_path_less_request_body_assertion_pins_the_whole_body() {
        let case: TestCase = serde_json::from_value(json!({
            "name": "a", "method": "PUT",
            "assertions": [{"type": "requestBody", "expected": {"summary": "Team Meeting"}}]
        }))
        .unwrap();
        let mut recorded = Recorded::default();
        recorded.methods.push("PUT".into());
        recorded
            .bodies
            .push(Some(json!({"summary": "Team Meeting", "all_day": false})));
        let run = Run {
            case: &case,
            outcome: &Ok(Outcome::Unit),
            recorded: &recorded,
        };
        assert!(
            check_all(&run)
                .unwrap_err()
                .contains("Expected request body on request index 0 to equal")
        );
        recorded.bodies[0] = Some(json!({"summary": "Team Meeting"}));
        let run = Run {
            case: &case,
            outcome: &Ok(Outcome::Unit),
            recorded: &recorded,
        };
        assert_eq!(check_all(&run), Ok(()));
    }

    #[test]
    fn large_integers_compare_exactly() {
        let big: Value = serde_json::from_str("9007199254740993").unwrap();
        let off: Value = serde_json::from_str("9007199254740992").unwrap();
        assert!(json_equal(&big, &big));
        assert!(!json_equal(&big, &off));
        assert!(json_equal(&json!(1), &json!(1.0)));
        assert!(json_equal(&json!("open"), &json!("open")));
    }

    #[test]
    fn lookup_prefers_a_literal_key_then_descends() {
        let value = json!({"a.b": 1, "a": {"b": 2}, "list": [{"id": 7}]});
        assert_eq!(lookup(&value, "a.b"), Some(&json!(1)));
        assert_eq!(lookup(&value, "list.0.id"), Some(&json!(7)));
        assert_eq!(lookup(&value, "missing"), None);
    }
}
