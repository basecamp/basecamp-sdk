//! Handling a failed call: the structured `Error` carries the SPEC §6 code, the HTTP
//! status, retryability, the request id and any per-field validation detail.
//!
//! ```sh
//! BASECAMP_TOKEN=... BASECAMP_ACCOUNT_ID=... cargo run --example errors
//! ```

use basecamp_sdk::{Client, Config, ErrorCode};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder(Config::default())
        .access_token(std::env::var("BASECAMP_TOKEN")?)
        .build()?;
    let account = client.for_account(std::env::var("BASECAMP_ACCOUNT_ID")?);

    match account.projects().get(1).await {
        Ok(project) => println!("found {}", project.name),
        Err(error) => {
            match error.code() {
                ErrorCode::NotFound => println!("no such project"),
                ErrorCode::AuthRequired => {
                    println!("token expired or revoked: {}", error.message());
                }
                ErrorCode::Validation => {
                    for (field, messages) in error.field_errors().into_iter().flatten() {
                        println!("{field}: {}", messages.join("; "));
                    }
                }
                _ => println!("{error}"),
            }
            if error.is_retryable() {
                println!("retryable; Retry-After {:?}s", error.retry_after());
            }
            if let Some(request_id) = error.request_id() {
                println!("request id {request_id}");
            }
            std::process::exit(error.exit_code());
        }
    }
    Ok(())
}
