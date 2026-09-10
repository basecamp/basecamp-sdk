//! SPEC §8: `Link`-header pagination.

use std::ops::Deref;

use futures_util::Stream;
use serde::de::DeserializeOwned;
use url::Url;

use crate::client::{AccountClient, Response};
use crate::error::Error;
use crate::hooks::OperationInfo;
use crate::operation::Operation;
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
    /// What the hooks were told about the read, path parameters included, so a follow-on
    /// page is reported as the same operation on the same record.
    info: OperationInfo,
    origin: Url,
    /// The request asked for exactly this page (SPEC §8's positive `page`): the cursor is
    /// reported, never followed.
    pinned: bool,
}

impl<T> Page<T> {
    pub(crate) fn new(
        value: T,
        response: &Response,
        route: &'static Route,
        info: OperationInfo,
        pinned: bool,
    ) -> Page<T> {
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
            info,
            origin: response.url.clone(),
            pinned,
        }
    }

    pub(crate) fn route(&self) -> &'static Route {
        self.route
    }

    pub(crate) fn info(&self) -> &OperationInfo {
        &self.info
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

    /// Whether the request selected exactly this page. A pinned page's cursor says more
    /// exists — [`Page::has_next`], [`ListMeta::truncated`] — and is never followed.
    pub fn is_pinned(&self) -> bool {
        self.pinned
    }

    /// The next target to follow, as a URL, or a usage error when Basecamp named one that
    /// is not; none for a pinned page.
    pub(crate) fn next_target(&self) -> Option<Result<Url, Error>> {
        if self.pinned {
            return None;
        }
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
}

/// What one page of a paginated read holds: the array itself for the reads that answer a
/// bare array, or the member of an envelope that the model names as the collection (the
/// generated shape of such a read implements this). [`AccountClient::collect_all`] and
/// [`AccountClient::items`] gather over it; the envelope's other members are the same on
/// every page, so read them off the first.
pub trait PageItems {
    /// The element type.
    type Item;

    /// The page's items, in order.
    fn into_items(self) -> Vec<Self::Item>;
}

impl<T> PageItems for Vec<T> {
    type Item = T;

    fn into_items(self) -> Vec<T> {
        self
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
    /// Reads the page after the given one, or `None` when Basecamp named no next page or
    /// the page was pinned. The read is the same operation as the first page — same hooks
    /// identity, same retry policy — and a `Link` header pointing off the origin the walk
    /// started on, or downgrading it to plain HTTP, is refused rather than followed.
    pub fn next_page<T: DeserializeOwned>(
        &self,
        page: &Page<T>,
    ) -> impl Future<Output = Result<Option<Page<T>>, Error>> + Send + '_ {
        // The follow-up is built here, not in the future, so the future never holds
        // `&Page<T>` across an await and is `Send` whatever `T` is.
        let operation = page.next_target().map(|next| self.follow_up(page, &next?));
        async move {
            match operation {
                None => Ok(None),
                Some(operation) => self.send_page(operation?).await.map(Some),
            }
        }
    }

    /// Reads every page after the first, up to `max_items` items and the client's page cap,
    /// and hands back everything as one list. Stopping early is reported in
    /// [`ListMeta::truncated`], never silently. A pinned page is the whole answer: nothing
    /// is followed, and `truncated` says whether the page named a successor.
    pub async fn collect_all<P: PageItems + DeserializeOwned>(
        &self,
        first: Page<P>,
        max_items: Option<usize>,
    ) -> Result<ListResult<P::Item>, Error> {
        let total_count = first.total_count().unwrap_or(0);
        let max_pages = self.max_pages();
        let mut page = first;
        let mut pages = 1;
        let mut items = Vec::new();
        loop {
            let next_url = page.next_target().transpose()?;
            let operation = match &next_url {
                Some(next) => Some(self.follow_up(&page, next)?),
                None => None,
            };
            let more = page.has_next();
            let named = page.next_url().cloned();
            items.extend(page.into_inner().into_items());
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
                        truncated: more,
                        next_url: named,
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
            page = match operation {
                Some(operation) => self.send_page(operation).await?,
                None => unreachable!("a next target always has a follow-up"),
            };
            pages += 1;
        }
    }
}

/// The pages of a walk: the one in hand, the follow-up that fetches the next one, or the
/// refusal a bad cursor earned — held back until the page before it has been yielded.
enum Pending<T> {
    Page(Page<T>),
    Fetch(Operation),
    Refused(Error),
}

impl AccountClient {
    /// Every page from `first` onward as a lazy stream, one request per page as it is
    /// polled, up to the client's page cap. An error ends the stream; the pages before it
    /// have already been yielded. Cancellation-safe: dropping the stream between pages
    /// sends nothing more. The cap ends the stream after its last page, whose
    /// [`Page::has_next`] still says whether more was available.
    pub fn pages<T: DeserializeOwned + Send + 'static>(
        &self,
        first: Page<T>,
    ) -> impl Stream<Item = Result<Page<T>, Error>> + Send + '_ {
        let max_pages = self.max_pages();
        // The page in hand is yielded before its successor is asked for, so taking one
        // page costs one request and a failing follow-up comes after the pages before it.
        futures_util::stream::try_unfold(
            (Some(Pending::Page(first)), 0usize),
            move |(pending, yielded)| async move {
                let page = match pending {
                    None => return Ok(None),
                    Some(Pending::Page(page)) => page,
                    Some(Pending::Fetch(operation)) => self.send_page(operation).await?,
                    Some(Pending::Refused(error)) => return Err(error),
                };
                let following = if yielded + 1 < max_pages {
                    self.pending_follow_up(&page)
                } else {
                    None
                };
                Ok(Some((page, (following, yielded + 1))))
            },
        )
    }

    /// Every item from `first` onward as a lazy stream, page by page. A page cap that
    /// leaves a next page unread ends the stream with a `usage` error after the items it
    /// did read, so a capped collection is never mistaken for a complete one (SPEC §8).
    pub fn items<P>(&self, first: Page<P>) -> impl Stream<Item = Result<P::Item, Error>> + Send + '_
    where
        P: PageItems + DeserializeOwned + Send + 'static,
        P::Item: Send + 'static,
    {
        use futures_util::TryStreamExt;
        let max_pages = self.max_pages();
        futures_util::stream::try_unfold(
            (Some(Pending::Page(first)), 0usize),
            move |(pending, yielded)| async move {
                let page = match pending {
                    None => return Ok::<_, Error>(None),
                    Some(Pending::Page(page)) => page,
                    Some(Pending::Fetch(operation)) => self.send_page(operation).await?,
                    Some(Pending::Refused(error)) => return Err(error),
                };
                let capped = yielded + 1 >= max_pages;
                let following = if capped {
                    None
                } else {
                    self.pending_follow_up(&page)
                };
                let truncated = capped && page.has_next();
                let mut items: Vec<Result<P::Item, Error>> =
                    page.into_inner().into_items().into_iter().map(Ok).collect();
                if truncated {
                    items.push(Err(Error::usage(format!(
                        "collection truncated at the page cap (max_pages = {max_pages}) with a next page available"
                    ))));
                }
                Ok(Some((
                    futures_util::stream::iter(items),
                    (following, yielded + 1),
                )))
            },
        )
        .try_flatten()
    }

    fn pending_follow_up<T>(&self, page: &Page<T>) -> Option<Pending<T>> {
        let next = page.next_target()?;
        Some(match next.and_then(|next| self.follow_up(page, &next)) {
            Ok(operation) => Pending::Fetch(operation),
            Err(error) => Pending::Refused(error),
        })
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
