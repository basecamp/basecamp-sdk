//! The shared case definitions under `conformance/tests/`, as this runner reads them.
//! Keys it does not read are ignored; the schema (`conformance/schema.json`) is enforced by
//! `make conformance-fixtures-check`, not here.

use std::collections::BTreeMap;

use serde::Deserialize;
use serde_json::{Map, Value};

pub type Params = Map<String, Value>;

#[derive(Debug, Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct TestCase {
    /// `mock` (the default) or `live`. An `Option` so an absent key stays distinguishable
    /// from `"mode": ""`, which is an unrecognized mode every runner refuses.
    pub mode: Option<String>,
    pub name: String,
    pub description: String,
    pub operation: String,
    pub method: String,
    pub path: String,
    pub path_params: Params,
    pub query_params: Params,
    pub request_body: Params,
    pub mock_responses: Vec<MockResponse>,
    pub assertions: Vec<Assertion>,
    pub tags: Vec<String>,
    pub config_overrides: ConfigOverrides,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct ConfigOverrides {
    pub base_url: Option<String>,
    pub max_pages: Option<u64>,
    pub max_items: Option<u64>,
    /// Pins the list operation to a single page (SPEC §8).
    pub page: Option<u64>,
    /// The client-wide retry cap as a TOTAL attempt count (SPEC §2). An `Option` because
    /// `0` is the value this override exists for — "no retries, exactly one attempt" —
    /// and a plain integer would make it indistinguishable from absent.
    pub max_retries: Option<u32>,
}

#[derive(Debug, Default, Clone, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct MockResponse {
    pub status: u16,
    pub network_error: bool,
    pub headers: BTreeMap<String, String>,
    pub body: Option<Value>,
    pub delay: u64,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct Assertion {
    #[serde(rename = "type")]
    pub kind: String,
    pub expected: Value,
    pub min: f64,
    pub max: f64,
    pub path: String,
    /// Which captured request the assertion reads. Defaults to the first; negative
    /// values count from the end.
    pub index: Option<i64>,
}

impl TestCase {
    pub fn is_mock_mode(&self) -> bool {
        self.mode.as_deref().is_none_or(|mode| mode == "mock")
    }

    pub fn has_tag(&self, tag: &str) -> bool {
        self.tags.iter().any(|t| t == tag)
    }

    /// Whether any queued response carries a `rel="next"` Link, so the SDK will follow it
    /// and the transport should answer the overrun with an empty page rather than a 500.
    pub fn auto_paginates(&self) -> bool {
        self.mock_responses.iter().any(|response| {
            response.headers.iter().any(|(name, value)| {
                name.eq_ignore_ascii_case("link") && value.contains("rel=\"next\"")
            })
        })
    }
}

pub fn int64_param(params: &Params, key: &str) -> i64 {
    params.get(key).and_then(Value::as_i64).unwrap_or_default()
}

pub fn string_param(params: &Params, key: &str) -> String {
    params
        .get(key)
        .and_then(Value::as_str)
        .unwrap_or_default()
        .to_string()
}

/// The value only when the key is PRESENT, so "explicitly set to empty" stays distinct
/// from "not set" — the distinction every merge-safe composite case turns on.
pub fn optional_string_param(params: &Params, key: &str) -> Option<String> {
    params.get(key).map(|value| match value {
        Value::String(text) => text.clone(),
        Value::Null => String::new(),
        other => other.to_string(),
    })
}

pub fn optional_bool_param(params: &Params, key: &str) -> Option<bool> {
    params.get(key).and_then(Value::as_bool)
}

/// Present-but-empty answers `Some(vec![])` (a clear); absent answers `None` (untouched).
pub fn optional_int64_list_param(params: &Params, key: &str) -> Option<Vec<i64>> {
    let values = params.get(key)?.as_array()?;
    Some(values.iter().filter_map(Value::as_i64).collect())
}

pub fn optional_string_list_param(params: &Params, key: &str) -> Option<Vec<String>> {
    let values = params.get(key)?.as_array()?;
    Some(
        values
            .iter()
            .filter_map(Value::as_str)
            .map(str::to_string)
            .collect(),
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn params(value: Value) -> Params {
        value.as_object().cloned().unwrap_or_default()
    }

    #[test]
    fn mode_absent_and_mock_are_mock_but_empty_is_not() {
        let absent: TestCase = serde_json::from_value(json!({"name": "a"})).unwrap();
        let mock: TestCase = serde_json::from_value(json!({"name": "a", "mode": "mock"})).unwrap();
        let empty: TestCase = serde_json::from_value(json!({"name": "a", "mode": ""})).unwrap();
        let live: TestCase = serde_json::from_value(json!({"name": "a", "mode": "live"})).unwrap();
        assert!(absent.is_mock_mode());
        assert!(mock.is_mock_mode());
        assert!(!empty.is_mock_mode());
        assert!(!live.is_mock_mode());
    }

    #[test]
    fn presence_bearing_params_distinguish_empty_from_absent() {
        let body = params(json!({"title": "", "assignee_ids": [], "due_on": null}));
        assert_eq!(optional_string_param(&body, "title"), Some(String::new()));
        assert_eq!(optional_string_param(&body, "content"), None);
        assert_eq!(optional_string_param(&body, "due_on"), Some(String::new()));
        assert_eq!(
            optional_int64_list_param(&body, "assignee_ids"),
            Some(vec![])
        );
        assert_eq!(optional_int64_list_param(&body, "participant_ids"), None);
    }

    #[test]
    fn max_retries_zero_survives_as_zero() {
        let case: TestCase =
            serde_json::from_value(json!({"name": "a", "configOverrides": {"maxRetries": 0}}))
                .unwrap();
        assert_eq!(case.config_overrides.max_retries, Some(0));
        let absent: TestCase = serde_json::from_value(json!({"name": "a"})).unwrap();
        assert_eq!(absent.config_overrides.max_retries, None);
    }

    #[test]
    fn auto_paginates_reads_a_next_link_case_insensitively() {
        let case: TestCase = serde_json::from_value(json!({
            "name": "a",
            "mockResponses": [{"status": 200, "headers": {"LINK": "<http://x/p?page=2>; rel=\"next\""}}]
        }))
        .unwrap();
        assert!(case.auto_paginates());
        let plain: TestCase = serde_json::from_value(json!({
            "name": "a", "mockResponses": [{"status": 200, "headers": {"Link": "<http://x>; rel=\"prev\""}}]
        }))
        .unwrap();
        assert!(!plain.auto_paginates());
    }
}
