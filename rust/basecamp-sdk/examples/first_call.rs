//! The first call: a static token, one account, one list.
//!
//! ```sh
//! BASECAMP_TOKEN=... BASECAMP_ACCOUNT_ID=... cargo run --example first_call
//! ```

use basecamp_sdk::services::projects::ListProjectsParams;
use basecamp_sdk::{Client, Config};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder(Config::default())
        .access_token(std::env::var("BASECAMP_TOKEN")?)
        .user_agent("basecamp-sdk example (you@example.com)")
        .build()?;
    let account = client.for_account(std::env::var("BASECAMP_ACCOUNT_ID")?);

    let page = account
        .projects()
        .list(&ListProjectsParams::default())
        .await?;
    for project in page.iter() {
        println!("{}: {}", project.id, project.name);
    }
    Ok(())
}
