//! Walking a paginated read: a page at a time, or all at once under a cap.

use basecamp_sdk::services::todos::ListTodosParams;
use basecamp_sdk::{Client, Config};

#[tokio::main]
async fn main() -> Result<(), basecamp_sdk::Error> {
    let token = std::env::var("BASECAMP_TOKEN").expect("BASECAMP_TOKEN");
    let account_id = std::env::var("BASECAMP_ACCOUNT").expect("BASECAMP_ACCOUNT");
    let todolist_id: i64 = std::env::var("BASECAMP_TODOLIST")
        .expect("BASECAMP_TODOLIST")
        .parse()
        .expect("a numeric todolist id");

    let account = Client::builder(Config::default())
        .access_token(token)
        .build()?
        .for_account(account_id);

    // A page at a time: every follow-on read is the same operation, with the same retry
    // policy, and a Link header pointing off the API origin is refused.
    let params = ListTodosParams {
        completed: Some(false),
        ..Default::default()
    };
    let mut page = account.todos().list(todolist_id, &params).await?;
    loop {
        for todo in page.iter() {
            println!("- {}", todo.title);
        }
        match account.next_page(&page).await? {
            Some(next) => page = next,
            None => break,
        }
    }

    // Or everything at once, capped: `truncated` says whether the cap or the client's
    // page limit stopped the walk before the end.
    let first = account.todos().list(todolist_id, &params).await?;
    let all = account.collect_all(first, Some(500)).await?;
    println!(
        "{} of {} to-dos{}",
        all.items.len(),
        all.meta.total_count,
        if all.meta.truncated {
            " (truncated)"
        } else {
            ""
        }
    );
    Ok(())
}
