//! SPEC §8: `Link`-header pagination.

use std::ops::Deref;

use futures_util::Stream;
use serde::de::DeserializeOwned;
use url::Url;

use crate::client::{AccountClient, Response};
use crate::error::Error;
use crate::route::Route;

/// One page of a paginated read, with the cursor Basecamp handed out for the next one.
///
/// The page derefs to its value, so `page.iter()` on a `Page<Vec<T>>` reads the same as it
/// would on the list itself.
#[derive(Debug, Clone)]
pub struct Page<T> {
    value: T,
    next: Option<Result<Url, String>>,
    total_count: Option<u64>,
    route: &'static Route,
    origin: Url,
}

impl<T> Page<T> {
    pub(crate) fn new(value: T, response: &Response, route: &'static Route) -> Page<T> {
        let next = response
            .header("link")
            .and_then(parse_next_link)
            .map(|target| response.url.join(&target).map_err(|_| target));
        let total_count = response
            .header("x-total-count")
            .and_then(|value| value.trim().parse().ok());
        Page {
            value,
            next,
            total_count,
            route,
            origin: response.url.clone(),
        }
    }

    pub(crate) fn route(&self) -> &'static Route {
        self.route
    }

    pub(crate) fn origin(&self) -> &Url {
        &self.origin
    }

    /// The page's contents, owned.
    pub fn into_inner(self) -> T {
        self.value
    }

    /// The page's contents.
    pub fn value(&self) -> &T {
        &self.value
    }

    /// The URL of the page after this one, as the `Link` header named it, when it named
    /// one that is a URL.
    pub fn next_url(&self) -> Option<&Url> {
        self.next.as_ref().and_then(|next| next.as_ref().ok())
    }

    /// Whether Basecamp named a page after this one, whether or not its target resolves.
    pub fn has_next(&self) -> bool {
        self.next.is_some()
    }

    /// The next target as a URL, or a usage error when Basecamp named one that is not.
    pub(crate) fn next_target(&self) -> Option<Result<Url, Error>> {
        self.next.as_ref().map(|next| match next {
            Ok(url) => Ok(url.clone()),
            Err(_) => Err(Error::usage(
                "pagination Link header names a target that is not a URL",
            )),
        })
    }

    /// The `X-Total-Count` header, when the read carried one.
    pub fn total_count(&self) -> Option<u64> {
        self.total_count
    }

    /// The same page with its value transformed.
    pub fn map<U>(self, f: impl FnOnce(T) -> U) -> Page<U> {
        Page {
            value: f(self.value),
            next: self.next,
            total_count: self.total_count,
            route: self.route,
            origin: self.origin,
        }
    }
}

impl<T> Deref for Page<T> {
    type Target = T;

    fn deref(&self) -> &T {
        &self.value
    }
}

/// SPEC §8's `ListResult`: every item an auto-paginating read collected, and what it
/// learned about the collection on the way.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct ListResult<T> {
    /// The items, in page order.
    pub items: Vec<T>,
    /// What the read learned about the collection.
    pub meta: ListMeta,
}

/// What an auto-paginating read learned about the collection.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
#[non_exhaustive]
pub struct ListMeta {
    /// `X-Total-Count`, or 0 when the server sent none.
    pub total_count: u64,
    /// Items beyond those returned were available: a next link was left unfollowed —
    /// because `max_items` was reached or the client's page cap was hit — or excess items
    /// were dropped.
    pub truncated: bool,
    /// The page the read stopped before, when it stopped early.
    pub next_url: Option<Url>,
}

impl AccountClient {
    /// Reads the page after the given one, or `None` when Basecamp named no next page. The
    /// read is the same operation as the first page — same hooks identity, same retry
    /// policy — and a `Link` header pointing off the origin the walk started on, or
    /// downgrading it to plain HTTP, is refused rather than followed.
    pub async fn next_page<T: DeserializeOwned>(
        &self,
        page: &Page<T>,
    ) -> Result<Option<Page<T>>, Error> {
        match page.next_target() {
            None => Ok(None),
            Some(next) => {
                let operation = self.follow_up(page.route(), page.origin(), &next?)?;
                self.send_page(operation).await.map(Some)
            }
        }
    }

    /// Reads every page after the first, up to `max_items` items and the client's page cap,
    /// and hands back everything as one list. Stopping early is reported in
    /// [`ListMeta::truncated`], never silently.
    pub async fn collect_all<T: DeserializeOwned>(
        &self,
        first: Page<Vec<T>>,
        max_items: Option<usize>,
    ) -> Result<ListResult<T>, Error> {
        let total_count = first.total_count().unwrap_or(0);
        let max_pages = self.max_pages();
        let mut page = first;
        let mut pages = 1;
        let mut items = Vec::new();
        let route = page.route();
        let origin = page.origin().clone();
        loop {
            let next_url = page.next_target().transpose()?;
            items.extend(page.into_inner());
            if let Some(cap) = max_items
                && items.len() >= cap
            {
                let truncated = next_url.is_some() || items.len() > cap;
                items.truncate(cap);
                return Ok(ListResult {
                    items,
                    meta: ListMeta {
                        total_count,
                        truncated,
                        next_url,
                    },
                });
            }
            let Some(next) = next_url else {
                return Ok(ListResult {
                    items,
                    meta: ListMeta {
                        total_count,
                        truncated: false,
                        next_url: None,
                    },
                });
            };
            if pages >= max_pages {
                return Ok(ListResult {
                    items,
                    meta: ListMeta {
                        total_count,
                        truncated: true,
                        next_url: Some(next),
                    },
                });
            }
            let operation = self.follow_up(route, &origin, &next)?;
            page = self.send_page(operation).await?;
            pages += 1;
        }
    }
}

impl AccountClient {
    /// Every page from `first` onward as a lazy stream, one request per page as it is
    /// polled, up to the client's page cap. An error ends the stream; the pages before it
    /// have already been yielded. Cancellation-safe: dropping the stream between pages
    /// sends nothing more.
    pub fn pages<T: DeserializeOwned + Send + 'static>(
        &self,
        first: Page<T>,
    ) -> impl Stream<Item = Result<Page<T>, Error>> + Send + '_ {
        let max_pages = self.max_pages();
        let route = first.route();
        let origin = first.origin().clone();
        // The page in hand is yielded before its successor is asked for, so taking one
        // page costs one request and a failing follow-up comes after the pages before it.
        futures_util::stream::try_unfold((Some(Ok(first)), 0usize), move |(pending, yielded)| {
            let origin = origin.clone();
            async move {
                let page = match pending {
                    None => return Ok(None),
                    Some(Ok(page)) => page,
                    Some(Err(next)) => {
                        let operation = self.follow_up(route, &origin, &next)?;
                        self.send_page(operation).await?
                    }
                };
                let following = if yielded + 1 < max_pages {
                    page.next_target().transpose()?.map(Err)
                } else {
                    None
                };
                Ok(Some((page, (following, yielded + 1))))
            }
        })
    }

    /// Every item from `first` onward as a lazy stream, page by page.
    pub fn items<T: DeserializeOwned + Send + 'static>(
        &self,
        first: Page<Vec<T>>,
    ) -> impl Stream<Item = Result<T, Error>> + Send + '_ {
        use futures_util::TryStreamExt;
        self.pages(first)
            .map_ok(|page| futures_util::stream::iter(page.into_inner().into_iter().map(Ok)))
            .try_flatten()
    }
}

/// SPEC §8's `parseNextLink`: split on commas, take the first `rel="next"` part with an
/// extractable target, and never let a `rel="next"` part without one suppress a later one.
pub fn parse_next_link(header: &str) -> Option<String> {
    if header.is_empty() {
        return None;
    }
    header
        .split(',')
        .map(str::trim)
        .filter(|part| part.contains("rel=\"next\""))
        .find_map(extract_angle_bracketed)
}

/// SPEC §8's `extractAngleBracketed`: leftmost `<…>` with something inside, skipping an
/// empty `<>`, searching for `>` only after the `<`, in bytes, so it is linear.
fn extract_angle_bracketed(part: &str) -> Option<String> {
    let bytes = part.as_bytes();
    let mut cursor = 0;
    loop {
        let start = cursor + bytes[cursor..].iter().position(|b| *b == b'<')?;
        let end = start + 1 + bytes[start + 1..].iter().position(|b| *b == b'>')?;
        if end > start + 1 {
            return Some(part[start + 1..end].to_string());
        }
        cursor = start + 1;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_the_next_link_among_others() {
        let header = r#"<https://3.basecampapi.com/999/projects.json?page=1>; rel="prev", <https://3.basecampapi.com/999/projects.json?page=3>; rel="next""#;
        assert_eq!(
            parse_next_link(header).as_deref(),
            Some("https://3.basecampapi.com/999/projects.json?page=3")
        );
        assert_eq!(parse_next_link(r#"</x?page=2>; rel="last""#), None);
        assert_eq!(parse_next_link(""), None);
    }

    #[test]
    fn an_unextractable_next_part_does_not_hide_a_later_one() {
        let header = r#"rel="next", </x?page=2>; rel="next""#;
        assert_eq!(parse_next_link(header).as_deref(), Some("/x?page=2"));
        assert_eq!(extract_angle_bracketed("<> <a>").as_deref(), Some("a"));
        assert_eq!(extract_angle_bracketed("> <a>").as_deref(), Some("a"));
        assert_eq!(extract_angle_bracketed("<<a>").as_deref(), Some("<a"));
        assert_eq!(extract_angle_bracketed("<a"), None);
        assert_eq!(extract_angle_bracketed("<é>").as_deref(), Some("é"));
    }
}
