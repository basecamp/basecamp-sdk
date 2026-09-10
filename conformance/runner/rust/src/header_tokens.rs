//! The one token a fixture header value may carry, `{{httpdate+Ns}}` (SPEC §19,
//! `conformance/schema.json`), resolved at the moment the response is served.
//!
//! It resolves to the IMF-fixdate of `floor(now) + N + 1` seconds: the first whole second
//! strictly more than N seconds after the second the response is served in. A compliant
//! SPEC §6 parser sees a remainder in `(N - latency, N + 1]` and, rounding up, computes at
//! least N whole seconds, so the fixture pairs it with a `delayBetweenRequests` floor of
//! N × 1000 ms. It exists because a static fixture has no clock: a literal past date pins
//! only the fall-through, and a far-future one is differently behaved per host.
//!
//! N is one to nine digits, so the arithmetic is exact everywhere and every runner's date
//! formatter stays in range; a longer N is an unrecognised token.
//!
//! An unrecognised `{{…}}` is an error rather than a literal: a typo'd token served
//! verbatim would be an unparseable header, which the SDK answers with its ordinary
//! backoff — the exact outcome the case exists to distinguish from. Every other value
//! passes through untouched.

use std::time::{Duration, SystemTime, UNIX_EPOCH};

pub fn resolve_header_value(value: &str, now: SystemTime) -> Result<String, String> {
    let Some(inner) = value
        .strip_prefix("{{")
        .and_then(|rest| rest.strip_suffix("}}"))
    else {
        return Ok(value.to_string());
    };
    let unrecognised = || {
        format!(
            "unrecognised header token {value:?}: only {{{{httpdate+Ns}}}} is defined (conformance/schema.json)"
        )
    };
    let digits = inner
        .strip_prefix("httpdate+")
        .and_then(|rest| rest.strip_suffix('s'))
        .ok_or_else(unrecognised)?;
    if digits.is_empty() || digits.len() > 9 || !digits.bytes().all(|b| b.is_ascii_digit()) {
        return Err(unrecognised());
    }
    let n: u64 = digits.parse().map_err(|_| unrecognised())?;
    let floor_now = now
        .duration_since(UNIX_EPOCH)
        .map_err(|error| format!("clock before the epoch: {error}"))?
        .as_secs();
    let at = UNIX_EPOCH + Duration::from_secs(floor_now + n + 1);
    Ok(httpdate::fmt_http_date(at))
}

#[cfg(test)]
mod tests {
    use super::*;

    // A quarter-second into 10:18:14 UTC, so the floor and the round-up land on different
    // seconds and a resolver that rounded would show it.
    fn token_now() -> SystemTime {
        UNIX_EPOCH + Duration::from_millis(1_623_233_894_250)
    }

    #[test]
    fn plain_values_pass_through() {
        for value in [
            "",
            "2",
            "Wed, 09 Jun 2021 10:18:14 GMT",
            "application/json",
            "{not a token}",
        ] {
            assert_eq!(
                resolve_header_value(value, token_now()).as_deref(),
                Ok(value)
            );
        }
    }

    #[test]
    fn httpdate_resolves_to_the_whole_second_past_n() {
        let cases = [
            ("{{httpdate+2s}}", "Wed, 09 Jun 2021 10:18:17 GMT"),
            ("{{httpdate+0s}}", "Wed, 09 Jun 2021 10:18:15 GMT"),
            ("{{httpdate+10s}}", "Wed, 09 Jun 2021 10:18:25 GMT"),
        ];
        for (token, want) in cases {
            assert_eq!(
                resolve_header_value(token, token_now()).as_deref(),
                Ok(want),
                "{token}"
            );
        }
    }

    #[test]
    fn unknown_tokens_are_errors_not_literals() {
        for value in [
            "{{httpdate}}",
            "{{httpdate+2}}",
            "{{httpdate-2s}}",
            "{{now}}",
            "{{}}",
            "{{httpdate+1000000000s}}",
        ] {
            let error = resolve_header_value(value, token_now())
                .expect_err("an unknown token must never be served literally");
            assert!(
                error.contains(value),
                "error for {value:?} does not name the token: {error}"
            );
        }
    }
}
