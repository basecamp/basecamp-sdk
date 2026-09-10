# basecamp-sdk

The Rust client for the [Basecamp API](https://github.com/basecamp/bc3-api), generated from
the Smithy model in this repository's `spec/`: every wire operation, every request and
response shape, and every operation's retry, idempotency and pagination behaviour come from
`openapi.json` and `behavior-model.json`. The hand-written part is the plumbing they share —
authentication, retries, pagination, the two-hop download, OAuth — and the SPEC §18
merge-safe composites.

```toml
[dependencies]
basecamp-sdk = "0.18"
tokio = { version = "1", features = ["full"] }
```

## First call

```rust,no_run
use basecamp_sdk::{Client, Config};

# async fn run() -> Result<(), basecamp_sdk::Error> {
let client = Client::builder(Config::default())
    .access_token(std::env::var("BASECAMP_TOKEN").expect("BASECAMP_TOKEN"))
    .build()?;
let account = client.for_account("999");

for project in account.projects().list(&Default::default()).await?.iter() {
    println!("{} ({})", project.name, project.id);
}
# Ok(())
# }
```

Services hang off an [`AccountClient`](crate::AccountClient): `account.projects()`,
`account.todos()`, `account.cards()` and so on, one per Basecamp service. Every method maps
to one operation of the model; the routes themselves are public data under
[`routes`](crate::routes).

## Pagination

A paginated read answers a [`Page`](crate::Page): the first page's items plus the cursor
Basecamp handed out. Follow it a page at a time with
[`AccountClient::next_page`](crate::AccountClient::next_page), or collect the whole
collection with [`AccountClient::collect_all`](crate::AccountClient::collect_all), which
stops at the client's page cap and says so in [`ListMeta::truncated`](crate::ListMeta).
A `Link` header pointing off the API origin is refused, never followed.

## Errors

Every call answers a [`Result<T, Error>`](crate::Error). The error carries the SPEC §6
record: a [`code`](crate::Error::code), the message and hint, the HTTP status, whether the
call is retryable and the `Retry-After` the server named, the request id, and — on a 400/422 —
the structured [`field_errors`](crate::Error::field_errors).

## Retries

Retries follow SPEC §7's three gates: the HTTP method's own idempotency, the model's
`idempotent` flag for a POST, and the operation's declared `retry_on` statuses. Attempts are
`min(client cap, operation max)`; a `Retry-After` header is honoured at every retried status,
and the local backoff is exponential with a 30 s ceiling and 100 ms of jitter. A 401 is
replayed once after the token provider refreshes, spending an attempt.

## Bring your own HTTP client

The SDK sends on one [`HttpClient`](crate::HttpClient). The `reqwest` feature (on by default)
ships one over rustls; with `--no-default-features` an application supplies its own.

## Features

| Feature | Default | What it adds |
|---|---|---|
| `reqwest` | yes | The shipped [`HttpClient`](crate::HttpClient) |
| `rustls-tls` | yes | rustls for the shipped client |
| `native-tls` | no | The platform TLS stack for the shipped client (additive) |
| `oauth` | yes | [`oauth`](crate::oauth): PKCE, device grant, token exchange and refresh |
| `tracing` | yes | One `tracing` span per operation |
| `event-feed` | no | Reserved for the SPEC §23 connector |

## Environment variables

Only [`Config::from_env`](crate::Config::from_env) reads the environment; a `Config` built
literally reads no variable at all.

| Variable | Meaning | Only when |
|---|---|---|
| `BASECAMP_BASE_URL` | The API origin (trailing `/` stripped) | `Config::from_env()` |
| `BASECAMP_TIMEOUT` | Per-attempt timeout, in seconds | `Config::from_env()` |
| `BASECAMP_MAX_RETRIES` | Total attempts per request, the first included | `Config::from_env()` |

`BASECAMP_TOKEN` and `BASECAMP_ACCOUNT_ID` appear in the examples only because the caller
reads them; the SDK never looks either up.

## Examples

`examples/` holds one compiled program per README section — `first_call`, `pagination`,
`errors` and `custom_http_client` — and `rs-test` builds them all.

## Versioning

The crate follows the repository's `vX.Y.Z` tags. Pre-1.0, a breaking change lands in a
`0.MINOR` release and an additive one in a `0.x.PATCH`. The minimum supported Rust version
is 1.88 and is enforced in CI; it moves only in a `0.MINOR` release, never past the
second-newest stable.
