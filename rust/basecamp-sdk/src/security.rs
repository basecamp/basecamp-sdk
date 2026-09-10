//! SPEC §9: HTTPS enforcement, origin comparison and redaction.

use url::Url;

use crate::error::Error;
use crate::http::{HeaderMap, HeaderValue};

/// The headers that are replaced with `[REDACTED]` before anything is logged or formatted.
pub const SENSITIVE_HEADERS: &[&str] = &["authorization", "cookie", "set-cookie", "x-csrf-token"];

/// Refuses a URL that would carry credentials over plain HTTP, unless it is on this machine.
pub fn require_secure_endpoint(url: &Url) -> Result<(), Error> {
    if url.scheme() == "https" || (url.scheme() == "http" && is_localhost(url)) {
        Ok(())
    } else {
        Err(Error::usage(format!(
            "{} must use HTTPS or be a localhost URL",
            redact_url(url)
        )))
    }
}

/// `localhost`, `127.0.0.1`, `::1` (bracketed or bare) and any `*.localhost` subdomain.
pub fn is_localhost(url: &Url) -> bool {
    match url.host_str() {
        Some(host) => {
            let host = host
                .trim_start_matches('[')
                .trim_end_matches(']')
                .to_ascii_lowercase();
            host == "localhost"
                || host == "127.0.0.1"
                || host == "::1"
                || host.ends_with(".localhost")
        }
        None => false,
    }
}

/// SPEC §8's `isSameOrigin`: scheme and host compared case-insensitively, with a default
/// port stripped so `https://x` and `https://x:443` are one origin.
pub fn is_same_origin(a: &Url, b: &Url) -> bool {
    a.scheme().eq_ignore_ascii_case(b.scheme())
        && a.host_str()
            .unwrap_or_default()
            .eq_ignore_ascii_case(b.host_str().unwrap_or_default())
        && a.port_or_known_default() == b.port_or_known_default()
}

/// A copy of the headers with credentials replaced, for logging.
pub fn redact_headers(headers: &HeaderMap) -> HeaderMap {
    let mut redacted = headers.clone();
    for name in SENSITIVE_HEADERS {
        if redacted.contains_key(*name) {
            redacted.insert(*name, HeaderValue::from_static("[REDACTED]"));
        }
    }
    redacted
}

/// A URL projected to its origin and path — no userinfo, query or fragment — which is all
/// an error or a hook needs from a request that may carry a signature in its query.
pub fn redact_url(url: &Url) -> String {
    match (url.host_str(), url.scheme()) {
        (Some(host), scheme) if !scheme.is_empty() => {
            let port = url
                .port()
                .map(|port| format!(":{port}"))
                .unwrap_or_default();
            format!("{scheme}://{host}{port}{}", url.path())
        }
        _ => "unparsable".to_string(),
    }
}

/// The origin alone — `https://host:port` — or the fixed token `unparsable`.
pub fn origin_of(raw: &str) -> String {
    match Url::parse(raw) {
        Ok(url) if url.host_str().is_some() => {
            let port = url
                .port()
                .map(|port| format!(":{port}"))
                .unwrap_or_default();
            format!(
                "{}://{}{port}",
                url.scheme(),
                url.host_str().unwrap_or_default()
            )
        }
        _ => "unparsable".to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn localhost_may_use_plain_http() {
        assert!(require_secure_endpoint(&Url::parse("http://localhost:3000").unwrap()).is_ok());
        assert!(require_secure_endpoint(&Url::parse("http://127.0.0.1:8080/x").unwrap()).is_ok());
        assert!(require_secure_endpoint(&Url::parse("http://[::1]:3000").unwrap()).is_ok());
        assert!(require_secure_endpoint(&Url::parse("http://app.localhost").unwrap()).is_ok());
        assert!(require_secure_endpoint(&Url::parse("https://3.basecampapi.com").unwrap()).is_ok());
        assert!(require_secure_endpoint(&Url::parse("http://evil.example.com").unwrap()).is_err());
    }

    #[test]
    fn same_origin_ignores_default_ports_and_case() {
        let a = Url::parse("https://3.Basecampapi.com/999/projects.json").unwrap();
        assert!(is_same_origin(
            &a,
            &Url::parse("HTTPS://3.basecampapi.com:443/other").unwrap()
        ));
        assert!(!is_same_origin(
            &a,
            &Url::parse("http://3.basecampapi.com/other").unwrap()
        ));
        assert!(!is_same_origin(
            &a,
            &Url::parse("https://evil.example.com/x").unwrap()
        ));
        assert!(!is_same_origin(
            &a,
            &Url::parse("https://3.basecampapi.com:8443/x").unwrap()
        ));
    }

    #[test]
    fn credentials_are_redacted() {
        let mut headers = HeaderMap::new();
        headers.insert("Authorization", HeaderValue::from_static("Bearer secret"));
        headers.insert("Accept", HeaderValue::from_static("application/json"));
        let redacted = redact_headers(&headers);
        assert_eq!(redacted["authorization"], "[REDACTED]");
        assert_eq!(redacted["accept"], "application/json");
    }

    #[test]
    fn urls_are_projected_to_origin_and_path() {
        let url = Url::parse("https://storage.example.com/blobs/1?signature=secret#x").unwrap();
        assert_eq!(redact_url(&url), "https://storage.example.com/blobs/1");
        assert_eq!(
            origin_of("https://storage.example.com:8443/blobs/1?s=1"),
            "https://storage.example.com:8443"
        );
        assert_eq!(origin_of("not a url"), "unparsable");
    }
}
