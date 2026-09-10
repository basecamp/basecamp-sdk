//! The first call: list the projects of one account.
//!
//! ```sh
//! BASECAMP_TOKEN=… BASECAMP_ACCOUNT=999 cargo run --example first_call
//! ```

use basecamp_sdk::{Client, Config};

#[tokio::main]
async fn main() -> Result<(), basecamp_sdk::Error> {
    let token = std::env::var("BASECAMP_TOKEN").expect("BASECAMP_TOKEN");
    let account_id = std::env::var("BASECAMP_ACCOUNT").expect("BASECAMP_ACCOUNT");

    let client = Client::builder(Config::default())
        .access_token(token)
        .build()?;
    let account = client.for_account(account_id);

    let projects = account.projects().list(&Default::default()).await?;
    for project in projects.iter() {
        println!("{:>12}  {}", project.id, project.name);
    }
    if projects.has_next() {
        println!(
            "… and more: {} in total",
            projects.total_count().unwrap_or(0)
        );
    }
    Ok(())
}
