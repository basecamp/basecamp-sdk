//! A [`TokenProvider`] that refreshes its OAuth token: what the client's 401 replay
//! (SPEC §4) runs when the credentials came from an OAuth grant.

use std::fmt;
use std::sync::Arc;

use async_trait::async_trait;
use tokio::sync::Mutex;

use super::OAuthClient;
use super::token::{RefreshRequest, Token};
use crate::auth::TokenProvider;
use crate::error::{Error, ErrorCode};
use crate::types::SensitiveString;

type RefreshHook = Arc<dyn Fn(&Token) + Send + Sync>;

/// A [`TokenProvider`] over an OAuth [`Token`] that refreshes it through an
/// [`OAuthClient`] when the client meets a 401.
///
/// Each refresh echoes the stored token's `resource` — a BC5 multi-account refresh token is
/// refused without it — and carries the stored `refresh_token` and `resource` forward when
/// the response omits them, so a rotated credential is never poorer than the one it
/// replaced. An [`on_refresh`](RefreshingTokenProvider::on_refresh) hook sees every
/// rotated token, which is how a CLI persists it.
pub struct RefreshingTokenProvider {
    oauth: OAuthClient,
    token_endpoint: String,
    client_id: String,
    client_secret: Option<SensitiveString>,
    token: Mutex<Token>,
    on_refresh: Option<RefreshHook>,
}

impl RefreshingTokenProvider {
    /// A provider over `token`, refreshing at `token_endpoint` as `client_id`.
    pub fn new(
        oauth: OAuthClient,
        token_endpoint: impl Into<String>,
        client_id: impl Into<String>,
        token: Token,
    ) -> RefreshingTokenProvider {
        RefreshingTokenProvider {
            oauth,
            token_endpoint: token_endpoint.into(),
            client_id: client_id.into(),
            client_secret: None,
            token: Mutex::new(token),
            on_refresh: None,
        }
    }

    /// The same provider refreshing as a confidential client.
    pub fn with_client_secret(
        mut self,
        client_secret: impl Into<SensitiveString>,
    ) -> RefreshingTokenProvider {
        self.client_secret = Some(client_secret.into());
        self
    }

    /// The same provider calling `hook` with each rotated token, after it is in use.
    pub fn on_refresh(
        mut self,
        hook: impl Fn(&Token) + Send + Sync + 'static,
    ) -> RefreshingTokenProvider {
        self.on_refresh = Some(Arc::new(hook));
        self
    }

    /// A copy of the token in use.
    pub async fn token(&self) -> Token {
        self.token.lock().await.clone()
    }
}

#[async_trait]
impl TokenProvider for RefreshingTokenProvider {
    async fn access_token(&self) -> Result<String, Error> {
        let token = self.token.lock().await;
        if token.access_token.is_empty() {
            Err(Error::new(
                ErrorCode::AuthRequired,
                "no access token configured",
            ))
        } else {
            Ok(token.access_token.expose().to_string())
        }
    }

    fn refreshable(&self) -> bool {
        true
    }

    async fn refresh(&self) -> Result<bool, Error> {
        let mut stored = self.token.lock().await;
        let Some(refresh_token) = stored.refresh_token.clone() else {
            return Err(Error::new(
                ErrorCode::AuthRequired,
                "the token cannot be refreshed: no refresh token was issued",
            ));
        };
        let request = RefreshRequest {
            token_endpoint: self.token_endpoint.clone(),
            refresh_token,
            client_id: self.client_id.clone(),
            client_secret: self.client_secret.clone(),
            resource: stored.resource.clone(),
        };
        let mut rotated = self.oauth.refresh_token(&request).await?;
        if rotated.refresh_token.is_none() {
            rotated.refresh_token = stored.refresh_token.take();
        }
        if rotated.resource.is_none() {
            rotated.resource = stored.resource.take();
        }
        *stored = rotated;
        if let Some(hook) = &self.on_refresh {
            hook(&stored);
        }
        Ok(true)
    }
}

impl fmt::Debug for RefreshingTokenProvider {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("RefreshingTokenProvider")
            .field("token_endpoint", &self.token_endpoint)
            .field("client_id", &self.client_id)
            .field("client_secret", &self.client_secret)
            .finish_non_exhaustive()
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use serde_json::json;

    use super::super::testing::{Script, Scripted, form_pairs};
    use super::*;
    use crate::auth::{AuthStrategy, BearerAuth};

    fn stored() -> Token {
        Token {
            access_token: "access-1".into(),
            refresh_token: Some("refresh-1".into()),
            token_type: "Bearer".to_string(),
            expires_in: Some(3600),
            scope: Some("read".to_string()),
            resource: Some("urn:bc:account:42".to_string()),
            expires_at: None,
        }
    }

    fn provider(script: &Arc<Script>) -> RefreshingTokenProvider {
        RefreshingTokenProvider::new(
            OAuthClient::new(Arc::clone(script)),
            "https://as.example/oauth/token",
            "client-1",
            stored(),
        )
    }

    #[tokio::test]
    async fn refresh_echoes_the_resource_and_carries_omitted_fields_forward() {
        let script = Script::new([Scripted::json(
            200,
            json!({"access_token": "access-2", "expires_in": 60}),
        )]);
        let seen = Arc::new(std::sync::Mutex::new(Vec::new()));
        let hook_seen = Arc::clone(&seen);
        let provider = provider(&script)
            .with_client_secret("secret-1")
            .on_refresh(move |token| hook_seen.lock().unwrap().push(token.clone()));

        assert!(provider.refreshable());
        assert_eq!(provider.access_token().await.unwrap(), "access-1");
        assert!(provider.refresh().await.unwrap());
        assert_eq!(provider.access_token().await.unwrap(), "access-2");

        let pairs = form_pairs(&script.requests()[0].body);
        assert_eq!(
            pairs,
            vec![
                ("grant_type".to_string(), "refresh_token".to_string()),
                ("refresh_token".to_string(), "refresh-1".to_string()),
                ("client_id".to_string(), "client-1".to_string()),
                ("client_secret".to_string(), "secret-1".to_string()),
                ("resource".to_string(), "urn:bc:account:42".to_string()),
            ]
        );
        let token = provider.token().await;
        assert_eq!(token.refresh_token.as_ref().unwrap().expose(), "refresh-1");
        assert_eq!(token.resource.as_deref(), Some("urn:bc:account:42"));
        assert_eq!(token.expires_in, Some(60));
        assert!(token.expires_at.is_some());
        assert_eq!(seen.lock().unwrap().as_slice(), &[token]);
        assert!(!format!("{provider:?}").contains("secret-1"));
    }

    #[tokio::test]
    async fn a_rotated_refresh_token_and_resource_replace_the_stored_ones() {
        let script = Script::new([Scripted::json(
            200,
            json!({
                "access_token": "access-2",
                "refresh_token": "refresh-2",
                "resource": "urn:bc:account:7"
            }),
        )]);
        let provider = provider(&script);
        provider.refresh().await.unwrap();
        let token = provider.token().await;
        assert_eq!(token.refresh_token.as_ref().unwrap().expose(), "refresh-2");
        assert_eq!(token.resource.as_deref(), Some("urn:bc:account:7"));
    }

    #[tokio::test]
    async fn a_token_without_a_refresh_token_cannot_refresh() {
        let script = Script::new([]);
        let provider = RefreshingTokenProvider::new(
            OAuthClient::new(Arc::clone(&script)),
            "https://as.example/oauth/token",
            "client-1",
            Token {
                refresh_token: None,
                ..stored()
            },
        );
        let error = provider.refresh().await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::AuthRequired);
        assert!(script.requests().is_empty());
    }

    #[tokio::test]
    async fn a_refused_refresh_keeps_the_stored_token() {
        let script = Script::new([Scripted::json(400, json!({"error": "invalid_grant"}))]);
        let provider = provider(&script);
        let error = provider.refresh().await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::AuthRequired);
        assert_eq!(provider.token().await, stored());
    }

    #[tokio::test]
    async fn the_client_refresh_path_moves_the_generation() {
        let script = Script::new([Scripted::json(200, json!({"access_token": "access-2"}))]);
        let auth = BearerAuth::new(provider(&script));
        assert!(auth.refreshable());
        assert!(auth.refresh(0).await.unwrap());
        assert_eq!(auth.generation(), 1);
        assert_eq!(auth.provider().access_token().await.unwrap(), "access-2");
    }
}
