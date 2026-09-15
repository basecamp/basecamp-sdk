//! The scalar types the model's shapes are built from.

use std::fmt;
use std::str::FromStr;

use chrono::{NaiveDate, Utc};
use serde::{Deserialize, Deserializer, Serialize, Serializer};

use crate::error::Error;

/// An instant Basecamp reports, ISO 8601 with its offset, held in UTC.
pub type DateTime = chrono::DateTime<Utc>;

/// A calendar date without a time zone, as Basecamp writes `due_on` and `starts_on`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Date(pub NaiveDate);

impl Date {
    /// A date from its parts, or `None` when they do not make one.
    pub fn new(year: i32, month: u32, day: u32) -> Option<Date> {
        NaiveDate::from_ymd_opt(year, month, day).map(Date)
    }

    /// Reads `YYYY-MM-DD`.
    pub fn parse(source: &str) -> Result<Date, Error> {
        source
            .parse()
            .map_err(|error| Error::usage(format!("invalid date {source:?}: {error}")))
    }
}

impl From<NaiveDate> for Date {
    fn from(date: NaiveDate) -> Date {
        Date(date)
    }
}

impl From<Date> for NaiveDate {
    fn from(date: Date) -> NaiveDate {
        date.0
    }
}

impl fmt::Display for Date {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0.format("%Y-%m-%d"))
    }
}

impl FromStr for Date {
    type Err = chrono::ParseError;

    fn from_str(source: &str) -> Result<Date, chrono::ParseError> {
        NaiveDate::parse_from_str(source, "%Y-%m-%d").map(Date)
    }
}

impl Serialize for Date {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.to_string())
    }
}

impl<'de> Deserialize<'de> for Date {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Date, D::Error> {
        let text = String::deserialize(deserializer)?;
        text.parse().map_err(serde::de::Error::custom)
    }
}

/// A moment Basecamp writes either as a bare date (`2016-06-01`, an all-day entry) or as a
/// full timestamp, and which the SDK round-trips verbatim rather than re-rendering.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct FlexibleTime(pub String);

impl FlexibleTime {
    /// The value as the API wrote it.
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// The value read as a date, when it is one.
    pub fn date(&self) -> Option<Date> {
        self.0.parse().ok()
    }

    /// The value read as an instant, when it is one.
    pub fn datetime(&self) -> Option<DateTime> {
        chrono::DateTime::parse_from_rfc3339(&self.0)
            .ok()
            .map(|moment| moment.with_timezone(&Utc))
    }
}

impl From<&str> for FlexibleTime {
    fn from(value: &str) -> FlexibleTime {
        FlexibleTime(value.to_string())
    }
}

impl From<String> for FlexibleTime {
    fn from(value: String) -> FlexibleTime {
        FlexibleTime(value)
    }
}

impl fmt::Display for FlexibleTime {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

/// Reads a nullable pixel dimension the API may spell as a float (`1024.0`), per SPEC §10.
pub(crate) mod flex_int {
    use serde::{Deserialize, Deserializer};

    /// `null` is `None`; an integer or an integral float is the integer.
    pub(crate) fn deserialize<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Option<i32>, D::Error> {
        match Option::<serde_json::Value>::deserialize(deserializer)? {
            None | Some(serde_json::Value::Null) => Ok(None),
            Some(serde_json::Value::Number(number)) => {
                if let Some(value) = number.as_i64() {
                    i32::try_from(value)
                        .map(Some)
                        .map_err(|_| serde::de::Error::custom("dimension out of range"))
                } else if let Some(value) = number.as_f64() {
                    if value.fract() == 0.0
                        && (f64::from(i32::MIN)..=f64::from(i32::MAX)).contains(&value)
                    {
                        #[allow(clippy::cast_possible_truncation)]
                        Ok(Some(value as i32))
                    } else {
                        Err(serde::de::Error::custom("dimension is not integral"))
                    }
                } else {
                    Err(serde::de::Error::custom("dimension is not a number"))
                }
            }
            Some(other) => Err(serde::de::Error::custom(format!(
                "dimension is not a number: {other}"
            ))),
        }
    }
}

/// Reads a 64-bit id the API may spell as a string. A string that is not a number — the
/// `"basecamp"` system actor's id — reads as `0`, as Go's `FlexibleInt64` and Kotlin's
/// `FlexibleLongSerializer` read it; a numeric string past 64 bits is still an error.
///
/// "Is a number" is Go's `strconv.ParseInt(s, 10, 64)` and nothing looser — see
/// `is_go_decimal` below for the grammar and what it deliberately refuses.
pub(crate) mod flexible_i64 {
    use serde::{Deserialize, Deserializer};

    #[derive(Deserialize)]
    #[serde(untagged)]
    enum Flexible {
        Number(i64),
        Text(String),
    }

    /// An integer, a string holding one, or `0` for a non-numeric sentinel.
    pub(crate) fn deserialize<'de, D: Deserializer<'de>>(deserializer: D) -> Result<i64, D::Error> {
        match Flexible::deserialize(deserializer)? {
            Flexible::Number(value) => Ok(value),
            Flexible::Text(text) => from_text(&text).map_err(serde::de::Error::custom),
        }
    }

    /// The string path of Go's `FlexibleInt64`: `strconv.ParseInt(s, 10, 64)`, whose
    /// syntax errors collapse to `0` and whose range errors are raised.
    fn from_text(text: &str) -> Result<i64, String> {
        if !is_go_decimal(text) {
            return Ok(0);
        }
        text.parse()
            .map_err(|_| format!("integer id {text:?} does not fit 64 bits"))
    }

    /// `strconv.ParseInt(s, 10, 64)`'s grammar: one optional ASCII sign, then one or more
    /// ASCII digits, and nothing else. Go trims no whitespace, so `" 7"` is a syntax error
    /// there and reads as `0`; it accepts a leading `+`; it takes `_` as a digit separator
    /// only for base 0, never for base 10; and it reads ASCII digits alone, so a fullwidth
    /// `７` is not a digit. Everything this admits, `i64::from_str` also admits, so the
    /// parse that follows can only fail on range — the one error Go raises.
    fn is_go_decimal(text: &str) -> bool {
        let digits = text.strip_prefix(['+', '-']).unwrap_or(text);
        !digits.is_empty() && digits.bytes().all(|b| b.is_ascii_digit())
    }

    /// [`deserialize`], with `null` as `None`. The generator emits it for an optional
    /// flexible id; the model has none today.
    #[allow(dead_code)]
    pub(crate) fn deserialize_optional<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Option<i64>, D::Error> {
        match Option::<Flexible>::deserialize(deserializer)? {
            None => Ok(None),
            Some(Flexible::Number(value)) => Ok(Some(value)),
            Some(Flexible::Text(text)) => {
                from_text(&text).map(Some).map_err(serde::de::Error::custom)
            }
        }
    }
}

/// A string that must not end up in logs — a person's name or email address. It prints as
/// `[REDACTED]`; call [`SensitiveString::expose`] to read it.
#[derive(Clone, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SensitiveString(String);

impl SensitiveString {
    /// Wraps a value.
    pub fn new(value: impl Into<String>) -> SensitiveString {
        SensitiveString(value.into())
    }

    /// The value itself.
    pub fn expose(&self) -> &str {
        &self.0
    }

    /// The value itself, owned.
    pub fn into_inner(self) -> String {
        self.0
    }

    /// Whether there is anything to redact.
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl From<String> for SensitiveString {
    fn from(value: String) -> SensitiveString {
        SensitiveString(value)
    }
}

impl From<&str> for SensitiveString {
    fn from(value: &str) -> SensitiveString {
        SensitiveString(value.to_string())
    }
}

impl fmt::Debug for SensitiveString {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.0.is_empty() {
            f.write_str("\"\"")
        } else {
            f.write_str("[REDACTED]")
        }
    }
}

impl fmt::Display for SensitiveString {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.0.is_empty() {
            Ok(())
        } else {
            f.write_str("[REDACTED]")
        }
    }
}

/// A download URL that needs the SDK's credentials to answer (`x-basecamp-auth-routable-url`):
/// it is fetched through [`AccountClient::download_url`](crate::AccountClient::download_url),
/// the two-hop flow of SPEC §14, never with a bare GET.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct AuthRoutableUrl(pub String);

impl AuthRoutableUrl {
    /// The URL as the API wrote it.
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for AuthRoutableUrl {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl From<&str> for AuthRoutableUrl {
    fn from(value: &str) -> AuthRoutableUrl {
        AuthRoutableUrl(value.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Deserialize)]
    struct Dimensions {
        #[serde(default, deserialize_with = "flex_int::deserialize")]
        width: Option<i32>,
    }

    #[derive(Deserialize)]
    struct Identified {
        #[serde(deserialize_with = "flexible_i64::deserialize")]
        id: i64,
    }

    #[test]
    fn dates_round_trip_through_json() {
        let date: Date = serde_json::from_str("\"2026-03-04\"").unwrap();
        assert_eq!(date, Date::new(2026, 3, 4).unwrap());
        assert_eq!(serde_json::to_string(&date).unwrap(), "\"2026-03-04\"");
    }

    #[test]
    fn dimensions_read_null_integers_and_integral_floats() {
        assert_eq!(
            serde_json::from_str::<Dimensions>(r#"{"width": null}"#)
                .unwrap()
                .width,
            None
        );
        assert_eq!(
            serde_json::from_str::<Dimensions>("{}").unwrap().width,
            None
        );
        assert_eq!(
            serde_json::from_str::<Dimensions>(r#"{"width": 1024}"#)
                .unwrap()
                .width,
            Some(1024)
        );
        assert_eq!(
            serde_json::from_str::<Dimensions>(r#"{"width": 1024.0}"#)
                .unwrap()
                .width,
            Some(1024)
        );
        assert!(serde_json::from_str::<Dimensions>(r#"{"width": 2.5}"#).is_err());
        assert_eq!(
            serde_json::from_str::<Dimensions>(r#"{"width": 2147483647.0}"#)
                .unwrap()
                .width,
            Some(i32::MAX)
        );
        assert!(serde_json::from_str::<Dimensions>(r#"{"width": 2147483648.0}"#).is_err());
    }

    #[test]
    fn ids_keep_sixty_four_bits_and_read_strings() {
        assert_eq!(
            serde_json::from_str::<Identified>(r#"{"id": 9007199254740993}"#)
                .unwrap()
                .id,
            9_007_199_254_740_993
        );
        assert_eq!(
            serde_json::from_str::<Identified>(r#"{"id": "42"}"#)
                .unwrap()
                .id,
            42
        );
        assert_eq!(
            serde_json::from_str::<Identified>(r#"{"id": "basecamp"}"#)
                .unwrap()
                .id,
            0
        );
        assert!(serde_json::from_str::<Identified>(r#"{"id": "99999999999999999999"}"#).is_err());
        assert_eq!(
            serde_json::from_str::<Identified>(r#"{"id": "basecamp"}"#)
                .unwrap()
                .id,
            0
        );
        assert!(serde_json::from_str::<Identified>(r#"{"id": "99999999999999999999"}"#).is_err());
    }

    // -----------------------------------------------------------------------------
    // Parity with Go's `types.FlexibleInt64` (`go/pkg/types/flexible_int64.go`).
    //
    // Every expectation below was recorded from the Go reference itself — not from this
    // implementation, and not from `strconv.ParseInt`'s documentation. A linked probe
    // unmarshalled `{"id": <case>, "name": "x"}` into `go/pkg/generated.Person`, whose
    // `Id` is `types.FlexibleInt64`, over the whole corpus, and each row pins that run's
    // verdict: `Some(n)` accepted with value n, `None` rejected. The probe was proved
    // live first — a mutation of `flexible_int64.go` moved 15 of its rows, and reverting
    // it restored them. `generated::types::Person::id` in this crate is the same field
    // through the deserializer under test.
    //
    // The rule those rows turned out to encode, for a reader who wants it in a sentence:
    // the string path is `strconv.ParseInt(s, 10, 64)`, whose syntax errors read as `0`
    // and whose range errors are raised; the number path is `json.Number.Int64()`, the
    // same parse over the literal's own text. The rows are the evidence; this sentence
    // is only a summary of them.
    // -----------------------------------------------------------------------------

    /// Runs one slice of the corpus through the deserializer under test. `None` means the
    /// decode must fail; `Some(n)` that it must succeed with exactly `n`. Reports every
    /// divergent row at once, so a regression names all of its casualties.
    fn check_against_go(rows: &[(&str, Option<i64>)]) {
        let divergences: Vec<String> = rows
            .iter()
            .filter_map(|(raw, expected)| {
                let read = serde_json::from_str::<Identified>(&format!("{{\"id\": {raw}}}"))
                    .ok()
                    .map(|identified| identified.id);
                (read != *expected)
                    .then(|| format!("  id: {raw} — Go reads {expected:?}, this crate {read:?}"))
            })
            .collect();
        assert!(
            divergences.is_empty(),
            "{} of {} rows diverge from Go:\n{}",
            divergences.len(),
            rows.len(),
            divergences.join("\n")
        );
    }

    #[test]
    fn go_parity_bare_json_numbers() {
        // The JSON-number path: Go reads the literal with `json.Number.Int64()`,
        // which is `ParseInt` over the literal text, so `7.0` and `1e2` are errors, not 7 and 100.
        check_against_go(&[
            ("7", Some(7)),                           // number 7
            ("-7", Some(-7)),                         // number -7
            ("0", Some(0)),                           // number 0
            ("7.0", None),                            // number 7.0
            ("-7.0", None),                           // number -7.0
            ("7.5", None),                            // number 7.5
            ("0.7", None),                            // number 0.7
            ("1e2", None),                            // number 1e2
            ("1E2", None),                            // number 1E2
            ("1e+2", None),                           // number 1e+2
            ("1e-2", None),                           // number 1e-2
            ("7e0", None),                            // number 7e0
            ("1e400", None),                          // number 1e400
            ("-1e400", None),                         // number -1e400
            ("9223372036854775807", Some(i64::MAX)),  // number 9223372036854775807
            ("9223372036854775808", None),            // number 9223372036854775808
            ("-9223372036854775808", Some(i64::MIN)), // number -9223372036854775808
            ("-9223372036854775809", None),           // number -9223372036854775809
            ("18446744073709551615", None),           // number 18446744073709551615
            ("18446744073709551616", None),           // number 18446744073709551616
            ("12345678901234567890", None),           // number 12345678901234567890
            ("-12345678901234567890", None),          // number -12345678901234567890
            ("9223372036854775807.0", None),          // number 9223372036854775807.0
            ("0e0", None),                            // number 0e0
            ("-0.0", None),                           // number -0.0
            ("-0e0", None),                           // number -0e0
            ("-0.000", None),                         // number -0.000
            ("0.0", None),                            // number 0.0
            ("-0.0e0", None),                         // number -0.0e0
        ]);
    }

    #[test]
    fn go_parity_non_string_json_values() {
        // Neither side coerces a non-scalar or a boolean.
        check_against_go(&[
            ("true", None),      // literal true
            ("false", None),     // literal false
            ("null", None),      // literal null
            ("[]", None),        // empty array
            ("[7]", None),       // array [7]
            ("[\"7\"]", None),   // array ["7"]
            ("{}", None),        // empty object
            ("{\"a\":1}", None), // object {"a":1}
        ]);
    }

    #[test]
    fn go_parity_numeric_strings_and_leading_zeros() {
        check_against_go(&[
            ("\"7\"", Some(7)),                           // string '7'
            ("\"-7\"", Some(-7)),                         // string '-7'
            ("\"+7\"", Some(7)),                          // string '+7'
            ("\"0\"", Some(0)),                           // string '0'
            ("\"-0\"", Some(0)),                          // string '-0'
            ("\"+0\"", Some(0)),                          // string '+0'
            ("\"00\"", Some(0)),                          // string '00'
            ("\"007\"", Some(7)),                         // string '007'
            ("\"-007\"", Some(-7)),                       // string '-007'
            ("\"+007\"", Some(7)),                        // string '+007'
            ("\"000000000000000000000000007\"", Some(7)), // string '000000000000000000000000007'
            ("\"1234567890\"", Some(1_234_567_890)),      // string '1234567890'
        ]);
    }

    #[test]
    fn go_parity_ascii_whitespace_is_never_trimmed() {
        // `ParseInt` trims nothing, so any surrounding byte is a syntax error and reads 0.
        check_against_go(&[
            ("\" 7\"", Some(0)),              // leading space
            ("\"7 \"", Some(0)),              // trailing space
            ("\" 7 \"", Some(0)),             // surrounding space
            ("\"\\t7\"", Some(0)),            // leading tab
            ("\"7\\t\"", Some(0)),            // trailing tab
            ("\"\\t7\\t\"", Some(0)),         // surrounding tab
            ("\"\\n7\"", Some(0)),            // leading LF
            ("\"7\\n\"", Some(0)),            // trailing LF
            ("\"\\n7\\n\"", Some(0)),         // surrounding LF
            ("\"\\r7\"", Some(0)),            // leading CR
            ("\"7\\r\"", Some(0)),            // trailing CR
            ("\"\\r7\\r\"", Some(0)),         // surrounding CR
            ("\"\\u000b7\"", Some(0)),        // leading VT
            ("\"7\\u000b\"", Some(0)),        // trailing VT
            ("\"\\u000b7\\u000b\"", Some(0)), // surrounding VT
            ("\"\\f7\"", Some(0)),            // leading FF
            ("\"7\\f\"", Some(0)),            // trailing FF
            ("\"\\f7\\f\"", Some(0)),         // surrounding FF
            ("\"7 7\"", Some(0)),             // interior space
            ("\" -7\"", Some(0)),             // space before sign
            ("\"- 7\"", Some(0)),             // space after sign
            ("\"+ 7\"", Some(0)),             // space after plus
            ("\" \"", Some(0)),               // whitespace only space
            ("\"\\t\"", Some(0)),             // whitespace only tab
            ("\"\\n\"", Some(0)),             // whitespace only LF
        ]);
    }

    #[test]
    fn go_parity_unicode_whitespace_is_never_trimmed() {
        // Nor does it know Unicode whitespace — `str::trim` would have eaten these.
        check_against_go(&[
            ("\"\\u00a07\"", Some(0)), // leading NBSP
            ("\"7\\u00a0\"", Some(0)), // trailing NBSP
            ("\"\\u20077\"", Some(0)), // leading FIGURE SPACE
            ("\"7\\u2007\"", Some(0)), // trailing FIGURE SPACE
            ("\"\\u30007\"", Some(0)), // leading IDEOGRAPHIC SPACE
            ("\"7\\u3000\"", Some(0)), // trailing IDEOGRAPHIC SPACE
            ("\"\\ufeff7\"", Some(0)), // leading ZWNBSP/BOM
            ("\"7\\ufeff\"", Some(0)), // trailing ZWNBSP/BOM
            ("\"\\u20287\"", Some(0)), // leading LINE SEPARATOR
            ("\"7\\u2028\"", Some(0)), // trailing LINE SEPARATOR
            ("\"\\u00857\"", Some(0)), // leading NEL
            ("\"7\\u0085\"", Some(0)), // trailing NEL
            ("\"\\u20097\"", Some(0)), // leading THIN SPACE
            ("\"7\\u2009\"", Some(0)), // trailing THIN SPACE
            ("\"\\u200b7\"", Some(0)), // leading ZERO WIDTH SPACE
            ("\"7\\u200b\"", Some(0)), // trailing ZERO WIDTH SPACE
        ]);
    }

    #[test]
    fn go_parity_signs() {
        // `ParseInt` takes exactly one leading `+` or `-`.
        check_against_go(&[
            ("\"+\"", Some(0)),        // string '+'
            ("\"-\"", Some(0)),        // string '-'
            ("\"++7\"", Some(0)),      // string '++7'
            ("\"--7\"", Some(0)),      // string '--7'
            ("\"+-7\"", Some(0)),      // string '+-7'
            ("\"-+7\"", Some(0)),      // string '-+7'
            ("\"7+\"", Some(0)),       // string '7+'
            ("\"7-\"", Some(0)),       // string '7-'
            ("\"-7-\"", Some(0)),      // string '-7-'
            ("\"+7+\"", Some(0)),      // string '+7+'
            ("\"-+\"", Some(0)),       // string '-+'
            ("\"++\"", Some(0)),       // string '++'
            ("\"7-7\"", Some(0)),      // string '7-7'
            ("\"\\u00b17\"", Some(0)), // string '±7'
        ]);
    }

    #[test]
    fn go_parity_empty_string() {
        check_against_go(&[
            ("\"\"", Some(0)), // empty string
        ]);
    }

    #[test]
    fn go_parity_underscores_are_not_digit_separators() {
        // `_` is a digit separator only for base 0; at base 10 it is a syntax error.
        check_against_go(&[
            ("\"1_0\"", Some(0)),   // string '1_0'
            ("\"_7\"", Some(0)),    // string '_7'
            ("\"7_\"", Some(0)),    // string '7_'
            ("\"1_000\"", Some(0)), // string '1_000'
            ("\"+1_0\"", Some(0)),  // string '+1_0'
            ("\"-1_0\"", Some(0)),  // string '-1_0'
            ("\"__7\"", Some(0)),   // string '__7'
        ]);
    }

    #[test]
    fn go_parity_sixty_four_bit_bounds() {
        // A well-formed number past the bound is the one string error Go raises;
        // a malformed one of any size is still just 0.
        check_against_go(&[
            ("\"9223372036854775807\"", Some(i64::MAX)), // string '9223372036854775807'
            ("\"9223372036854775808\"", None),           // string '9223372036854775808'
            ("\"9223372036854775806\"", Some(9_223_372_036_854_775_806)), // string '9223372036854775806'
            ("\"+9223372036854775807\"", Some(i64::MAX)), // string '+9223372036854775807'
            ("\"+9223372036854775808\"", None),           // string '+9223372036854775808'
            ("\"-9223372036854775808\"", Some(i64::MIN)), // string '-9223372036854775808'
            ("\"-9223372036854775809\"", None),           // string '-9223372036854775809'
            ("\"-9223372036854775807\"", Some(-9_223_372_036_854_775_807)), // string '-9223372036854775807'
            ("\"0009223372036854775807\"", Some(i64::MAX)), // string '0009223372036854775807'
            ("\"0009223372036854775808\"", None),           // string '0009223372036854775808'
            ("\"18446744073709551615\"", None),             // string '18446744073709551615'
            ("\"18446744073709551616\"", None),             // string '18446744073709551616'
            ("\"99999999999999999999999999999999\"", None), // string '99999999999999999999999999999999'
            ("\"-99999999999999999999999999999999\"", None), // string '-99999999999999999999999999999999'
            ("\"9223372036854775808 \"", Some(0)),           // string '9223372036854775808 '
            ("\" 9223372036854775808\"", Some(0)),           // string ' 9223372036854775808'
            ("\"-0000000000000000000009223372036854775809\"", None), // string '-0000000000000000000009223372036854775809'
        ]);
    }

    #[test]
    fn go_parity_base_prefixes_are_not_recognized() {
        // Base 10 is fixed, so `0x`/`0o`/`0b` are ordinary syntax errors.
        check_against_go(&[
            ("\"0x10\"", Some(0)),  // string '0x10'
            ("\"0X10\"", Some(0)),  // string '0X10'
            ("\"0o17\"", Some(0)),  // string '0o17'
            ("\"0O17\"", Some(0)),  // string '0O17'
            ("\"0b101\"", Some(0)), // string '0b101'
            ("\"0B101\"", Some(0)), // string '0B101'
            ("\"0xg\"", Some(0)),   // string '0xg'
            ("\"#7\"", Some(0)),    // string '#7'
            ("\"07\"", Some(7)),    // string '07'
            ("\"0x7\"", Some(0)),   // string '0x7'
            ("\"-0x10\"", Some(0)), // string '-0x10'
            ("\"8#7\"", Some(0)),   // string '8#7'
            ("\"1e2\"", Some(0)),   // string '1e2'
            ("\"7e2\"", Some(0)),   // string '7e2'
            ("\"0d7\"", Some(0)),   // string '0d7'
        ]);
    }

    #[test]
    fn go_parity_non_ascii_digits_are_not_digits() {
        // Go's digit test is ASCII-only, and so is `i64::from_str` — `char::is_numeric` is not.
        check_against_go(&[
            ("\"\\uff17\"", Some(0)),        // string '７'
            ("\"\\uff17\\uff17\"", Some(0)), // string '７７'
            ("\"\\u0667\"", Some(0)),        // string '٧'
            ("\"\\u096d\"", Some(0)),        // string '७'
            ("\"7\\u0667\"", Some(0)),       // string '7٧'
            ("\"\\u06677\"", Some(0)),       // string '٧7'
            ("\"\\u2467\"", Some(0)),        // string '⑧'
            ("\"\\u2166\"", Some(0)),        // string 'Ⅶ'
            ("\"\\u00bd\"", Some(0)),        // string '½'
            ("\"\\u2077\"", Some(0)),        // string '⁷'
            ("\"\\u00b2\"", Some(0)),        // string '²'
            ("\"\\u0660\"", Some(0)),        // string '٠'
        ]);
    }

    #[test]
    fn go_parity_non_numeric_sentinels() {
        // "basecamp" is the one the API actually sends.
        check_against_go(&[
            ("\"basecamp\"", Some(0)),  // string 'basecamp'
            ("\"abc\"", Some(0)),       // string 'abc'
            ("\"7abc\"", Some(0)),      // string '7abc'
            ("\"abc7\"", Some(0)),      // string 'abc7'
            ("\"7.0\"", Some(0)),       // string '7.0'
            ("\"-7.0\"", Some(0)),      // string '-7.0'
            ("\"7,0\"", Some(0)),       // string '7,0'
            ("\"7.\"", Some(0)),        // string '7.'
            ("\".7\"", Some(0)),        // string '.7'
            ("\"NaN\"", Some(0)),       // string 'NaN'
            ("\"nan\"", Some(0)),       // string 'nan'
            ("\"Inf\"", Some(0)),       // string 'Inf'
            ("\"+Inf\"", Some(0)),      // string '+Inf'
            ("\"-Inf\"", Some(0)),      // string '-Inf'
            ("\"infinity\"", Some(0)),  // string 'infinity'
            ("\"null\"", Some(0)),      // string 'null'
            ("\"true\"", Some(0)),      // string 'true'
            ("\"false\"", Some(0)),     // string 'false'
            ("\"[7]\"", Some(0)),       // string '[7]'
            ("\"{}\"", Some(0)),        // string '{}'
            ("\"7\\u00007\"", Some(0)), // string '7\x007'
            ("\"\\u00007\"", Some(0)),  // string '\x007'
            ("\"7\\u0000\"", Some(0)),  // string '7\x00'
            ("\"7 7 7\"", Some(0)),     // string '7 7 7'
            ("\"'7'\"", Some(0)),       // string "'7'"
            ("\"\\\"7\\\"\"", Some(0)), // string '"7"'
            ("\"seven\"", Some(0)),     // string 'seven'
        ]);
    }

    /// A bare JSON `-0` is the one corpus row this crate still refuses where Go accepts
    /// it. Go parses the literal's own text — `ParseInt("-0", 10, 64)` is 0 — while
    /// `serde_json` has already turned that literal into the float `-0.0` by the time the
    /// deserializer sees it, and the text is gone. Reading the float back as 0 would also
    /// accept `-0.0`, `-0e0` and `-0.000`, which Go rejects (rows above): three new
    /// divergences in the accepting direction to close one in the refusing direction. So
    /// the divergence stays, pinned here so it cannot change unnoticed.
    #[test]
    fn a_bare_negative_zero_is_a_known_residual_divergence() {
        // Go accepts this and reads 0; this crate rejects it.
        assert!(serde_json::from_str::<Identified>(r#"{"id": -0}"#).is_err());
    }

    /// The corpus runs through `Identified`. The field that actually ships is
    /// `generated::types::Person::id`, annotated with the same `deserialize_with`; these
    /// rows hold the two frames together.
    #[test]
    fn the_shipped_person_id_reads_what_the_corpus_pins() {
        for (raw, expected) in [
            ("7", Some(7_i64)),
            ("\" 7\"", Some(0)),
            ("\"+7\"", Some(7)),
            ("\"basecamp\"", Some(0)),
            ("\"9223372036854775808\"", None),
        ] {
            let read = serde_json::from_str::<crate::generated::types::Person>(&format!(
                "{{\"id\": {raw}, \"name\": \"x\"}}"
            ))
            .ok()
            .map(|person| person.id);
            assert_eq!(read, expected, "id: {raw}");
        }
    }

    #[test]
    fn flexible_times_read_both_shapes() {
        assert_eq!(
            FlexibleTime::from("2016-06-01").date(),
            Date::new(2016, 6, 1)
        );
        assert!(FlexibleTime::from("2016-06-01").datetime().is_none());
        assert!(
            FlexibleTime::from("2016-06-01T10:00:00Z")
                .datetime()
                .is_some()
        );
    }

    #[test]
    fn sensitive_strings_hide_their_value() {
        let secret = SensitiveString::new("jane@example.com");
        assert_eq!(format!("{secret:?}"), "[REDACTED]");
        assert_eq!(secret.to_string(), "[REDACTED]");
        assert_eq!(secret.expose(), "jane@example.com");
        assert_eq!(
            serde_json::to_string(&secret).unwrap(),
            "\"jane@example.com\""
        );
    }
}
