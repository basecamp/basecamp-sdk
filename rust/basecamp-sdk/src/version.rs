//! The SDK's own version and the API contract it was generated against.

pub use crate::generated::API_VERSION;

/// The SDK's own version, from `Cargo.toml`.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// The `User-Agent` every request goes out with: `basecamp-sdk-rust/{VERSION} (api:{API_VERSION})`.
pub fn default_user_agent() -> String {
    format!("basecamp-sdk-rust/{VERSION} (api:{API_VERSION})")
}
