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
pub mod flex_int {
    use serde::{Deserialize, Deserializer};

    /// `null` is `None`; an integer or an integral float is the integer.
    pub fn deserialize<'de, D: Deserializer<'de>>(
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

/// Reads a 64-bit id the API may spell as a string.
pub mod flexible_i64 {
    use serde::{Deserialize, Deserializer};

    #[derive(Deserialize)]
    #[serde(untagged)]
    enum Flexible {
        Number(i64),
        Text(String),
    }

    /// An integer, or a string holding one.
    pub fn deserialize<'de, D: Deserializer<'de>>(deserializer: D) -> Result<i64, D::Error> {
        match Flexible::deserialize(deserializer)? {
            Flexible::Number(value) => Ok(value),
            Flexible::Text(text) => text.parse().map_err(serde::de::Error::custom),
        }
    }

    /// [`deserialize`], with `null` as `None`.
    pub fn deserialize_optional<'de, D: Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Option<i64>, D::Error> {
        match Option::<Flexible>::deserialize(deserializer)? {
            None => Ok(None),
            Some(Flexible::Number(value)) => Ok(Some(value)),
            Some(Flexible::Text(text)) => text.parse().map(Some).map_err(serde::de::Error::custom),
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
