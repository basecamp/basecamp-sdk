//! Walking every page of a list, with the SDK following `Link: rel="next"` on the same
//! origin and stopping at the configured page cap.
//!
//! ```sh
//! BASECAMP_TOKEN=... BASECAMP_ACCOUNT_ID=... cargo run --example pagination
//! ```

use basecamp_sdk::services::projects::ListProjectsParams;
use basecamp_sdk::{Client, Config};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder(Config::default())
        .access_token(std::env::var("BASECAMP_TOKEN")?)
        .build()?;
    let account = client.for_account(std::env::var("BASECAMP_ACCOUNT_ID")?);

    // One page at a time: `next_page` follows the cursor the last response carried.
    let mut page = account
        .projects()
        .list(&ListProjectsParams::default())
        .await?;
    let mut seen = page.len();
    while let Some(next) = account.next_page(&page).await? {
        seen += next.len();
        page = next;
    }
    println!("{seen} projects across every page");

    // Or all at once, capped at 50 items; `meta.truncated` says whether the cap bit.
    let first = account
        .projects()
        .list(&ListProjectsParams::default())
        .await?;
    let all = account.collect_all(first, Some(50)).await?;
    println!(
        "{} of {} projects (truncated: {})",
        all.items.len(),
        all.meta.total_count,
        all.meta.truncated
    );
    Ok(())
}
