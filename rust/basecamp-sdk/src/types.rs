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
/// "Is a number" is Go's `strconv.ParseInt(s, 10, 64)` and nothing looser — see `parse_int`
/// below for the grammar, for which refusal Go turns into `0` and which it raises, and for
/// why the two cannot be told apart without walking the string the way Go walks it.
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

    /// The string path of `FlexibleInt64` (`go/pkg/types/flexible_int64.go:34`): the two
    /// error kinds `strconv.ParseInt` distinguishes, and what Go does with each — a syntax
    /// error becomes `0` (`:46`), a range error is raised (`:43`).
    fn from_text(text: &str) -> Result<i64, String> {
        match parse_int(text) {
            Ok(value) => Ok(value),
            Err(Refusal::Syntax) => Ok(0),
            Err(Refusal::Range) => Err(format!("integer id {text:?} does not fit 64 bits")),
        }
    }

    /// Which way `strconv.ParseInt` refused. Keeping them apart is the whole point: Go
    /// answers `0` to one and an error to the other, and no single "is this a number?"
    /// predicate can tell them apart, because *which refusal comes first* depends on
    /// where in the string each disqualifying byte sits.
    enum Refusal {
        Syntax,
        Range,
    }

    /// `strconv.ParseInt(s, 10, 64)`, scan order included.
    ///
    /// It takes one optional ASCII sign, then one or more ASCII digits, and nothing else:
    /// no whitespace (Go trims none, so `" 7"` is a syntax error and reads `0`), a leading
    /// `+` accepted, `_` never a separator at base 10 (only base 0 allows it), and ASCII
    /// digits alone, so a fullwidth `７` is not a digit.
    ///
    /// The subtlety worth the hand-rolled loop: `ParseUint` checks the magnitude *inside*
    /// the scan and returns `ErrRange` the moment the accumulator would overflow `u64`,
    /// before it ever looks at the rest of the string. So the first disqualifying byte
    /// wins, and `"18446744073709551616x"` is a **range** error — Go raises it — while
    /// `"18446744073709551615x"` is a **syntax** error and reads `0`. Testing the whole
    /// string for well-formedness first gets that pair backwards.
    ///
    /// Note that this is *not* the rule the global-id parser applies to the person id in
    /// `gid://bc3/Person/<id>`. That one walks the bytes and refuses anything outside
    /// `0..=9` *before* it parses (`go/pkg/basecamp/mentions.go:252-256`), so it rejects a
    /// leading `+` that this one accepts. Both live in this crate: `parse_global_id` in
    /// [`crate::mentions`] carries that digit walk, and it is *correct* to, because the
    /// reference has that shape at that site. The same shape was wrong here only because
    /// the Go line governing this site has no pre-walk. Two sites, two rules, deliberately:
    /// do not hoist either into the other, in either direction.
    fn parse_int(text: &str) -> Result<i64, Refusal> {
        let (negative, digits) = match text.as_bytes() {
            [] => return Err(Refusal::Syntax),
            [b'+', rest @ ..] => (false, rest),
            [b'-', rest @ ..] => (true, rest),
            whole => (false, whole),
        };
        // Unobservable through this consumer — an empty digit run accumulates 0 and a
        // syntax refusal reads 0, so no test here can pin it — but it is Go's behaviour
        // and it matters the moment `parse_int` is read by anything where the two differ.
        if digits.is_empty() {
            return Err(Refusal::Syntax);
        }

        // `ParseUint`'s loop, byte for byte.
        let mut magnitude: u64 = 0;
        for byte in digits {
            if !byte.is_ascii_digit() {
                return Err(Refusal::Syntax);
            }
            magnitude = magnitude
                .checked_mul(10)
                .and_then(|shifted| shifted.checked_add(u64::from(byte - b'0')))
                .ok_or(Refusal::Range)?;
        }

        // `ParseInt`'s own bound, applied to what `ParseUint` returned.
        if negative {
            if magnitude > i64::MIN.unsigned_abs() {
                return Err(Refusal::Range);
            }
            Ok(i64::try_from(magnitude).map_or(i64::MIN, |value| -value))
        } else {
            i64::try_from(magnitude).map_err(|_| Refusal::Range)
        }
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

/// Reads a list of people the way the reference does. Go decodes `[]generated.Person`, and
/// `encoding/json` leaves a `null` element as the zero `Person` — id `0`, empty name — rather
/// than failing the read, so a merge-safe write that reads the list back sends `0` for it. The
/// generator emits this for every member whose element type is a person (a struct with a
/// required flexible id); a plain-`int64` person type in Go keeps the strict list.
///
/// Only the element is lenient. A `null` *list* is `None` as before, an element that is not an
/// object still fails, and so does an element whose id is an explicit `null`.
pub(crate) mod person_list {
    use serde::{Deserialize, Deserializer};

    /// A required list of people.
    #[allow(dead_code)]
    pub(crate) fn deserialize<'de, D, T>(deserializer: D) -> Result<Vec<T>, D::Error>
    where
        D: Deserializer<'de>,
        T: Deserialize<'de> + Default,
    {
        Ok(zero_nulls(Vec::<Option<T>>::deserialize(deserializer)?))
    }

    /// An optional (or nullable) list of people: `null` is `None`.
    pub(crate) fn deserialize_optional<'de, D, T>(
        deserializer: D,
    ) -> Result<Option<Vec<T>>, D::Error>
    where
        D: Deserializer<'de>,
        T: Deserialize<'de> + Default,
    {
        Ok(Option::<Vec<Option<T>>>::deserialize(deserializer)?.map(zero_nulls))
    }

    fn zero_nulls<T: Default>(elements: Vec<Option<T>>) -> Vec<T> {
        elements
            .into_iter()
            .map(Option::unwrap_or_default)
            .collect()
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

    /// The shape the generator emits for an *optional* flexible id, attribute for
    /// attribute. The model has no such field today, so this frame is the only thing
    /// holding `deserialize_optional` to the same rule as its required sibling.
    #[derive(Deserialize)]
    struct OptionallyIdentified {
        #[serde(default, deserialize_with = "flexible_i64::deserialize_optional")]
        id: Option<i64>,
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
    // verdict: `Some(n)` accepted with value n, `None` rejected.
    //
    // The probe was proved live before it was trusted, on both of the paths it measures:
    // a mutation of the string path in `flexible_int64.go` moved 33 rows, a mutation of
    // the number path moved 6, and reverting each restored every one byte for byte.
    // `generated::types::Person::id` in this crate is the same field through the
    // deserializer under test.
    //
    // The rule those rows turned out to encode, for a reader who wants it in a sentence:
    // the string path is `strconv.ParseInt(s, 10, 64)`, whose syntax errors read as `0`
    // and whose range errors are raised, deciding between the two by whichever
    // disqualifying byte the left-to-right scan reaches first; the number path is
    // `json.Number.Int64()`, the same parse over the literal's own text. The rows are the
    // evidence; this sentence is only a summary of them.
    // -----------------------------------------------------------------------------

    /// Runs one slice of the corpus through the deserializer under test. `None` means the
    /// decode must fail; `Some(n)` that it must succeed with exactly `n`. Reports every
    /// divergent row at once, so a regression names all of its casualties.
    ///
    /// Every row is read through *both* frames the generator can emit — the required id
    /// and the optional one. Go's own optional frame, a `*types.FlexibleInt64` field,
    /// returns the required frame's verdict on every row but one: a literal `null`, which
    /// it takes as absent. So the two frames share one set of expectations, with `null` as
    /// the single exception, and the optional arm cannot drift away from the rule while
    /// the required arm still follows it.
    fn check_against_go(rows: &[(&str, Option<i64>)]) {
        let divergences: Vec<String> = rows
            .iter()
            .flat_map(|(raw, expected)| {
                let body = format!("{{\"id\": {raw}}}");
                let required = serde_json::from_str::<Identified>(&body)
                    .ok()
                    .map(|identified| identified.id);
                let optional = serde_json::from_str::<OptionallyIdentified>(&body)
                    .ok()
                    .map(|identified| identified.id);
                // `null` is the one row where the optional frame legitimately parts
                // company with the required one: absent, not a zero and not an error.
                let optional_expected = if *raw == "null" {
                    Some(None)
                } else {
                    expected.map(Some)
                };
                [
                    (required != *expected).then(|| {
                        format!("  id: {raw} — Go reads {expected:?}, this crate {required:?}")
                    }),
                    (optional != optional_expected).then(|| {
                        format!(
                            "  id: {raw} (optional field) — Go reads {optional_expected:?}, \
                             this crate {optional:?}"
                        )
                    }),
                ]
            })
            .flatten()
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
            // TEN, not eight. Base is never detected from the literal: Ruby's `Integer()`
            // read this one as 8, which is not a refusal against an acceptance but two
            // different PEOPLE from one wire value (card 35).
            ("\"010\"", Some(10)),  // string '010'
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

    #[test]
    fn go_parity_which_refusal_comes_first() {
        // `ParseUint` checks the magnitude inside the scan, so an overflowing digit
        // ends the parse before a later non-digit is seen: the first disqualifying byte
        // wins. That makes the boundary u64::MAX, not i64::MAX, and it is why junk after
        // an oversized prefix is RAISED while the same junk after a merely large one
        // reads 0. Junk before the digits is a syntax error wherever it sits.
        check_against_go(&[
            ("\"18446744073709551615x\"", Some(0)), // string '18446744073709551615x'
            ("\"9223372036854775808x\"", Some(0)),  // string '9223372036854775808x'
            ("\"9223372036854775807x\"", Some(0)),  // string '9223372036854775807x'
            ("\"-9223372036854775809x\"", Some(0)), // string '-9223372036854775809x'
            ("\"000018446744073709551615x\"", Some(0)), // string '000018446744073709551615x'
            ("\"18446744073709551616x\"", None),    // string '18446744073709551616x'
            ("\"18446744073709551616 \"", None),    // string '18446744073709551616 '
            ("\"18446744073709551616.0\"", None),   // string '18446744073709551616.0'
            ("\"18446744073709551616\\n\"", None),  // string '18446744073709551616\n'
            ("\"18446744073709551616\\t\"", None),  // string '18446744073709551616\t'
            ("\"18446744073709551616\\u0000\"", None), // string '18446744073709551616\x00'
            ("\"18446744073709551616\\u00a0\"", None), // string '18446744073709551616\xa0'
            ("\"18446744073709551616_0\"", None),   // string '18446744073709551616_0'
            ("\"18446744073709551616abc\"", None),  // string '18446744073709551616abc'
            ("\"-18446744073709551616x\"", None),   // string '-18446744073709551616x'
            ("\"+18446744073709551616x\"", None),   // string '+18446744073709551616x'
            ("\"000018446744073709551616x\"", None), // string '000018446744073709551616x'
            ("\"184467440737095516161234x\"", None), // string '184467440737095516161234x'
            ("\"99999999999999999999_\"", None),    // string '99999999999999999999_'
            ("\"1844674407370955161612345678901234567890x\"", None), // string '1844674407370955161612345678901234567890x'
            ("\"x18446744073709551616\"", Some(0)), // string 'x18446744073709551616'
            ("\" 18446744073709551616\"", Some(0)), // string ' 18446744073709551616'
            ("\"+x18446744073709551616\"", Some(0)), // string '+x18446744073709551616'
            ("\"-18446744073709551616\"", None),    // string '-18446744073709551616'
            ("\"+18446744073709551616\"", None),    // string '+18446744073709551616'
            (
                "\"9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999z\"",
                None,
            ), // 100 nines then a letter
            (
                "\"9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999\"",
                None,
            ), // 100 nines
            (
                "\"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007\"",
                Some(7),
            ), // 300 leading zeros then 7
            (
                "\"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007z\"",
                Some(0),
            ), // 300 leading zeros then 7 then junk
        ]);
    }

    #[test]
    fn go_parity_the_i64_min_boundary_in_several_spellings() {
        // The two subtlest lines in `parse_int` — the negative-magnitude comparison
        // and the fallback that turns 2^63 into i64::MIN — were each pinned by exactly one
        // row, so an edit to that row could have hidden a mutation. These are the same two
        // magnitudes spelled several ways.
        check_against_go(&[
            ("\"-09223372036854775808\"", Some(i64::MIN)), // string '-09223372036854775808'
            ("\"-009223372036854775808\"", Some(i64::MIN)), // string '-009223372036854775808'
            (
                "\"-00000000000000000000009223372036854775808\"",
                Some(i64::MIN),
            ), // string '-00000000000000000000009223372036854775808'
            ("\"+09223372036854775807\"", Some(i64::MAX)), // string '+09223372036854775807'
            ("\"+009223372036854775807\"", Some(i64::MAX)), // string '+009223372036854775807'
            (
                "\"-09223372036854775807\"",
                Some(-9_223_372_036_854_775_807),
            ), // string '-09223372036854775807'
            ("\"-009223372036854775809\"", None),          // string '-009223372036854775809'
            ("\"+09223372036854775808\"", None),           // string '+09223372036854775808'
            ("\"+009223372036854775808\"", None),          // string '+009223372036854775808'
            ("\"-0000000000000000000000000000000000000001\"", Some(-1)), // string '-0000000000000000000000000000000000000001'
            ("\"+0000000000000000000000000000000000000001\"", Some(1)), // string '+0000000000000000000000000000000000000001'
        ]);
    }

    /// A bare JSON `-0` is the one corpus row this crate still refuses where Go accepts
    /// it. Go parses the literal's own text — `ParseInt("-0", 10, 64)` is 0 — while
    /// `serde_json` has already turned that literal into the float `-0.0` by the time the
    /// deserializer sees it, and the text is gone. Reading the float back as 0 would also
    /// accept `-0.0`, `-0e0`, `-0.000` and `-0.0e0`, which Go rejects (rows above), and a
    /// fallback keyed on the value rather than the sign would take `0.0` and `0e0` too:
    /// six new divergences in the accepting direction to close one in the refusing
    /// direction. So the divergence stays, pinned here so it cannot change unnoticed.
    #[test]
    fn a_bare_negative_zero_is_a_known_residual_divergence() {
        // Go accepts this and reads 0; this crate rejects it.
        assert!(serde_json::from_str::<Identified>(r#"{"id": -0}"#).is_err());
    }

    /// The other residual, and it is not this function's to fix: a lone surrogate escape
    /// inside the id string. Go's `encoding/json` folds it to U+FFFD, so `ParseInt` sees a
    /// syntax error and the id reads 0; `serde_json` refuses the *document*, so the
    /// deserializer is never reached. The proof that the layer is the parser and not the
    /// rule is that a plain `String` field fails on the same input. It is the refusing
    /// direction, and closing it would mean changing JSON parsers.
    #[test]
    fn a_lone_surrogate_is_refused_by_the_json_parser_not_by_this_rule() {
        assert!(serde_json::from_str::<String>(r#""\ud800""#).is_err());
        assert!(serde_json::from_str::<Identified>(r#"{"id": "\ud800"}"#).is_err());
    }

    /// An optional flexible id that is absent reads as absent, not as `0` — the same as
    /// Go's `*types.FlexibleInt64`, which stays nil for a missing key and for `null`.
    #[test]
    fn an_absent_optional_flexible_id_is_none() {
        assert_eq!(
            serde_json::from_str::<OptionallyIdentified>("{}")
                .ok()
                .map(|identified| identified.id),
            Some(None)
        );
        assert_eq!(
            serde_json::from_str::<OptionallyIdentified>(r#"{"id": null}"#)
                .ok()
                .map(|identified| identified.id),
            Some(None)
        );
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
            // The pair that only a left-to-right scan gets right, on the shipped field.
            ("\"18446744073709551615x\"", Some(0)),
            ("\"18446744073709551616x\"", None),
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

    /// The shape the generator emits for a *required* list of people. The model has none
    /// today (every person list is optional), so this frame is what holds
    /// `person_list::deserialize` to the rule its optional sibling is tested against.
    #[derive(Deserialize, Default, Debug, PartialEq)]
    struct Someone {
        #[serde(default, deserialize_with = "flexible_i64::deserialize")]
        id: i64,
    }

    #[derive(Deserialize)]
    struct RequiredPeople {
        #[serde(deserialize_with = "person_list::deserialize")]
        people: Vec<Someone>,
    }

    #[test]
    fn a_required_person_list_reads_a_null_element_as_the_zero_person() {
        let read = |body: &str| serde_json::from_str::<RequiredPeople>(body).map(|r| r.people);
        assert_eq!(
            read(r#"{"people": [null, {"id": "7"}, {}]}"#).unwrap(),
            vec![Someone { id: 0 }, Someone { id: 7 }, Someone { id: 0 }]
        );
        assert!(read(r#"{"people": null}"#).is_err());
        assert!(read("{}").is_err());
        assert!(read(r#"{"people": [5]}"#).is_err());
        assert!(read(r#"{"people": [{"id": null}]}"#).is_err());
    }
}
