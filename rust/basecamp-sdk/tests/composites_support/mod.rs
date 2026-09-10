//! A runner for the SPEC §18 composite fixtures under `conformance/tests/`: each case's
//! mock responses are served in order, the typed SDK call runs against them, and every
//! assertion the fixture states is checked as written. The fixture is the contract; the
//! test file supplies only the call.

#![allow(dead_code, unreachable_pub)]

use std::future::Future;
use std::path::Path;

use basecamp_sdk::{AccountClient, Client, Config, Error};
use serde_json::Value;
use wiremock::matchers::any;
use wiremock::{Mock, MockServer, Request, ResponseTemplate};

/// Runs one named case of a fixture file and hands back what the call answered, after
/// the fixture's assertions have all held.
pub async fn run<F, Fut, T>(fixture: &str, name: &str, call: F) -> Result<T, Error>
where
    F: FnOnce(AccountClient) -> Fut,
    Fut: Future<Output = Result<T, Error>>,
{
    let case = load(fixture, name);
    let server = MockServer::start().await;
    let mocks = case["mockResponses"]
        .as_array()
        .expect("mockResponses is a list");
    for (index, mock) in mocks.iter().enumerate() {
        Mock::given(any())
            .respond_with(template(mock))
            .up_to_n_times(1)
            .with_priority(u8::try_from(index + 1).expect("fewer than 255 mocks"))
            .mount(&server)
            .await;
    }
    let outcome = call(account(&server)).await;
    let requests = server.received_requests().await.unwrap_or_default();
    let assertions = case["assertions"].as_array().expect("assertions is a list");
    for assertion in assertions {
        check(assertion, &requests, outcome.as_ref().err());
    }
    outcome
}

fn load(fixture: &str, name: &str) -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../conformance/tests")
        .join(format!("{fixture}.json"));
    let text = std::fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    let cases: Vec<Value> = serde_json::from_str(&text).expect("fixture is a JSON list");
    cases
        .into_iter()
        .find(|case| case["name"] == name)
        .unwrap_or_else(|| panic!("no case named {name:?} in {fixture}.json"))
}

fn account(server: &MockServer) -> AccountClient {
    Client::builder(Config::default().with_base_url(server.uri()))
        .access_token("test-token")
        .build()
        .expect("client")
        .for_account("999")
}

fn template(mock: &Value) -> ResponseTemplate {
    let status = u16::try_from(mock["status"].as_u64().expect("status")).expect("status fits");
    let headers = mock["headers"].as_object();
    let declared_type = headers
        .into_iter()
        .flatten()
        .find(|(name, _)| name.eq_ignore_ascii_case("content-type"))
        .and_then(|(_, value)| value.as_str());
    let mut template = match &mock["body"] {
        Value::Null => ResponseTemplate::new(status),
        Value::String(text) => ResponseTemplate::new(status).set_body_raw(
            text.as_bytes().to_vec(),
            declared_type.unwrap_or("text/plain"),
        ),
        body => ResponseTemplate::new(status).set_body_raw(
            serde_json::to_vec(body).expect("body serializes"),
            declared_type.unwrap_or("application/json"),
        ),
    };
    for (name, value) in headers.into_iter().flatten() {
        if !name.eq_ignore_ascii_case("content-type") {
            template = template.insert_header(name.as_str(), value.as_str().expect("header text"));
        }
    }
    template
}

fn check(assertion: &Value, requests: &[Request], error: Option<&Error>) {
    let kind = assertion["type"].as_str().expect("assertion type");
    match kind {
        "requestCount" => assert_eq!(
            requests.len() as u64,
            assertion["expected"].as_u64().expect("count"),
            "request count"
        ),
        "requestMethod" => assert_eq!(
            request_at(requests, assertion).method.as_str(),
            assertion["expected"].as_str().expect("method"),
            "request method"
        ),
        "requestPath" => assert_eq!(
            request_at(requests, assertion).url.path(),
            assertion["expected"].as_str().expect("path"),
            "request path"
        ),
        "requestBody" => {
            let path = assertion["path"].as_str().expect("body path");
            assert_eq!(
                member(&body_of(request_at(requests, assertion)), path),
                Some(&assertion["expected"]),
                "request body member {path}"
            );
        }
        "requestBodyAbsent" => {
            let path = assertion["path"].as_str().expect("body path");
            assert_eq!(
                member(&body_of(request_at(requests, assertion)), path),
                None,
                "request body member {path} should be absent"
            );
        }
        "headerPresent" | "headerAbsent" => {
            let name = assertion["path"].as_str().expect("header name");
            let present = request_at(requests, assertion).headers.get(name).is_some();
            assert_eq!(present, kind == "headerPresent", "header {name}");
        }
        "noError" => {
            if let Some(error) = error {
                panic!("expected no error, got {error}");
            }
        }
        "errorRaised" => assert!(error.is_some(), "expected an error"),
        "errorMessage" => {
            let expected = assertion["expected"].as_str().expect("message");
            let message = error.map(ToString::to_string).expect("expected an error");
            assert!(
                message.contains(expected),
                "error {message:?} should mention {expected:?}"
            );
        }
        other => panic!("unsupported assertion type {other:?}"),
    }
}

fn request_at<'a>(requests: &'a [Request], assertion: &Value) -> &'a Request {
    let index = assertion["index"].as_i64().unwrap_or(0);
    let position = if index < 0 {
        requests
            .len()
            .checked_sub(usize::try_from(-index).expect("index fits"))
            .expect("negative index within range")
    } else {
        usize::try_from(index).expect("index fits")
    };
    requests
        .get(position)
        .unwrap_or_else(|| panic!("no request at index {index} of {}", requests.len()))
}

fn body_of(request: &Request) -> Value {
    serde_json::from_slice(&request.body).expect("request body is JSON")
}

fn member<'a>(body: &'a Value, path: &str) -> Option<&'a Value> {
    path.split('.').try_fold(body, |value, key| value.get(key))
}
