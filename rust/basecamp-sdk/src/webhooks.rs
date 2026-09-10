//! SPEC §15: verifying a webhook delivery's signature.

use hmac::{Hmac, KeyInit, Mac};
use sha2::Sha256;

/// Whether `signature` is the hex HMAC-SHA256 of `payload` under `secret`. An empty
/// signature or secret never verifies; comparison is constant-time.
pub fn verify_signature(payload: &[u8], signature: &str, secret: &str) -> bool {
    if signature.is_empty() || secret.is_empty() {
        return false;
    }
    let Ok(mut mac) = Hmac::<Sha256>::new_from_slice(secret.as_bytes()) else {
        return false;
    };
    mac.update(payload);
    let Some(expected) = decode_hex(signature.trim_start_matches("sha256=")) else {
        return false;
    };
    mac.verify_slice(&expected).is_ok()
}

fn decode_hex(text: &str) -> Option<Vec<u8>> {
    if !text.len().is_multiple_of(2) {
        return None;
    }
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).ok())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn verifies_a_known_digest() {
        let mut mac = Hmac::<Sha256>::new_from_slice(b"secret").unwrap();
        mac.update(b"payload");
        let mut digest = String::new();
        for byte in mac.finalize().into_bytes() {
            use std::fmt::Write;
            write!(digest, "{byte:02x}").unwrap();
        }
        assert!(verify_signature(b"payload", &digest, "secret"));
        assert!(!verify_signature(b"payload", &digest, "other"));
        assert!(!verify_signature(b"other", &digest, "secret"));
        assert!(!verify_signature(b"payload", "", "secret"));
        assert!(!verify_signature(b"payload", &digest, ""));
    }
}
