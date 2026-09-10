//! SPEC §16 "PKCE S256" and "State Generation".

use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use sha2::{Digest, Sha256};

use crate::types::SensitiveString;

/// A PKCE code verifier and the S256 challenge derived from it. The verifier is the
/// secret half — whoever holds it can redeem the authorization code — so it prints as
/// `[REDACTED]`; the challenge goes out in the authorization URL and is public.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Pkce {
    /// The `code_verifier` sent with the token exchange.
    pub verifier: SensitiveString,
    /// The `code_challenge` sent with the authorization request.
    pub challenge: String,
}

/// Draws 32 random bytes from the system CSPRNG as the verifier (base64url, no padding)
/// and derives its SHA-256 challenge the same way.
pub fn generate_pkce() -> Pkce {
    let verifier = URL_SAFE_NO_PAD.encode(rand::random::<[u8; 32]>());
    let challenge = URL_SAFE_NO_PAD.encode(Sha256::digest(verifier.as_bytes()));
    Pkce {
        verifier: SensitiveString::new(verifier),
        challenge,
    }
}

/// Draws 16 random bytes as a `state` parameter (base64url, no padding). Store it before
/// redirecting and compare it on the callback.
pub fn generate_state() -> String {
    URL_SAFE_NO_PAD.encode(rand::random::<[u8; 16]>())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_challenge_is_the_digest_of_the_verifier() {
        let pkce = generate_pkce();
        let decoded = URL_SAFE_NO_PAD.decode(pkce.verifier.expose()).unwrap();
        assert_eq!(decoded.len(), 32);
        assert_eq!(pkce.verifier.expose().len(), 43);
        assert_eq!(
            pkce.challenge,
            URL_SAFE_NO_PAD.encode(Sha256::digest(pkce.verifier.expose().as_bytes()))
        );
        assert_ne!(pkce.verifier, generate_pkce().verifier);
        assert_eq!(format!("{:?}", pkce.verifier), "[REDACTED]");
    }

    #[test]
    fn state_is_sixteen_random_bytes() {
        let state = generate_state();
        assert_eq!(URL_SAFE_NO_PAD.decode(&state).unwrap().len(), 16);
        assert_eq!(state.len(), 22);
        assert_ne!(state, generate_state());
    }
}
