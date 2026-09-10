//! Signing in from a terminal with the RFC 8628 device grant, then calling the API with a
//! token that refreshes itself.
//!
//! ```sh
//! BASECAMP_ACCOUNT=999 cargo run --example device_login
//! ```

use basecamp_sdk::oauth::{DiscoveryOutcome, MonotonicClock, OAuthClient, RefreshingTokenProvider};
use basecamp_sdk::{Client, Config};

const CLIENT_ID: &str = "basecamp-cli";

#[tokio::main]
async fn main() -> Result<(), basecamp_sdk::Error> {
    let account_id = std::env::var("BASECAMP_ACCOUNT").expect("BASECAMP_ACCOUNT");
    let oauth = OAuthClient::shipped()?;

    // Resource-first discovery: ask the API host which authorization server it trusts.
    let metadata = match oauth
        .discover_from_resource("https://3.basecampapi.com", None)
        .await?
    {
        DiscoveryOutcome::Selected(metadata) => metadata,
        DiscoveryOutcome::Fallback(reason) => {
            eprintln!("no authorization server advertised ({reason:?}); falling back to Launchpad");
            oauth.discover_launchpad().await?
        }
    };

    let token = oauth
        .perform_device_login(
            &metadata,
            CLIENT_ID,
            Some("read"),
            |authorization| {
                println!(
                    "Open {} and enter the code {}",
                    authorization.verification_uri, authorization.user_code
                );
            },
            &MonotonicClock,
            std::env::var("BASECAMP_LOGIN_HINT").ok().as_deref(),
        )
        .await?;

    let provider =
        RefreshingTokenProvider::new(oauth, metadata.token_endpoint.clone(), CLIENT_ID, token)
            .on_refresh(|rotated| {
                println!("token rotated; expires at {:?}", rotated.expires_at);
            });
    let account = Client::builder(Config::default())
        .token_provider(provider)
        .build()?
        .for_account(account_id);

    let me = account.people().me().await?;
    println!("Signed in as {}", me.name.expose());
    Ok(())
}
