//! Recognizes Basecamp paths and URLs, as pasted from the web app or the API, and names the
//! operations they refer to. Works offline, from the route table alone.

use std::collections::BTreeMap;
use std::sync::OnceLock;

use url::Url;

use crate::generated::routes::ROUTES;
use crate::http::Method;
use crate::route::Route;

/// A recognizer over a set of routes.
pub struct Router {
    patterns: Vec<Pattern>,
}

struct Pattern {
    pattern: &'static str,
    routes: Vec<&'static Route>,
}

/// What a recognized path refers to.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct Match {
    /// The pattern that matched.
    pub pattern: &'static str,
    /// The operations served on this path, by method.
    pub operations: BTreeMap<String, &'static str>,
    /// The path parameters in the order they appear.
    pub params: Vec<(&'static str, String)>,
}

impl Match {
    /// The read on this path, or the alphabetically first operation when there is none.
    pub fn operation(&self) -> Option<&'static str> {
        self.operations
            .get(Method::GET.as_str())
            .or_else(|| self.operations.values().min())
            .copied()
    }

    /// The last path parameter: the id of the record the path names, if any.
    pub fn resource_id(&self) -> Option<&str> {
        self.params.last().map(|(_, value)| value.as_str())
    }
}

impl Router {
    /// A router over every modelled route.
    pub fn new() -> Router {
        Router::over(ROUTES)
    }

    /// A router over a chosen set of routes.
    pub fn over(routes: &[&'static Route]) -> Router {
        let mut by_pattern: BTreeMap<&'static str, Vec<&'static Route>> = BTreeMap::new();
        for route in routes {
            by_pattern.entry(route.pattern).or_default().push(route);
        }
        let mut patterns: Vec<Pattern> = by_pattern
            .into_iter()
            .map(|(pattern, routes)| Pattern { pattern, routes })
            .collect();
        patterns.sort_by(|a, b| {
            let depth = |pattern: &str| pattern.matches('/').count();
            depth(b.pattern)
                .cmp(&depth(a.pattern))
                .then_with(|| a.pattern.cmp(b.pattern))
        });
        Router { patterns }
    }

    /// Recognizes a path such as `/projects/456` or a full URL such as
    /// `https://3.basecampapi.com/999/projects/456.json?x=1`. A leading numeric account
    /// segment, trailing slashes, a `.json` suffix, the query and the fragment are ignored.
    pub fn recognize(&self, path_or_url: &str) -> Option<Match> {
        let path = match Url::parse(path_or_url) {
            Ok(url) if !url.cannot_be_a_base() => url.path().to_string(),
            _ => path_or_url
                .split(['?', '#'])
                .next()
                .unwrap_or_default()
                .to_string(),
        };
        let path = path.trim_end_matches('/');
        let path = path.strip_suffix(".json").unwrap_or(path);
        let unscoped = strip_account(path);
        self.patterns.iter().find_map(|pattern| {
            pattern
                .recognize(path)
                .or_else(|| pattern.recognize(unscoped))
        })
    }
}

fn strip_account(path: &str) -> &str {
    let rest = path.strip_prefix('/').unwrap_or(path);
    match rest.split_once('/') {
        Some((first, remainder))
            if !first.is_empty() && first.bytes().all(|b| b.is_ascii_digit()) =>
        {
            &path[path.len() - remainder.len() - 1..]
        }
        _ => path,
    }
}

impl Default for Router {
    fn default() -> Router {
        Router::new()
    }
}

impl Pattern {
    fn recognize(&self, path: &str) -> Option<Match> {
        let params = self.routes[0].recognize(path)?;
        let operations = self
            .routes
            .iter()
            .map(|route| (route.method.to_string(), route.id))
            .collect();
        Some(Match {
            pattern: self.pattern,
            operations,
            params,
        })
    }
}

/// The process-wide router.
pub fn router() -> &'static Router {
    static ROUTER: OnceLock<Router> = OnceLock::new();
    ROUTER.get_or_init(Router::new)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn recognizes_a_pasted_project_url() {
        let matched = router()
            .recognize("https://3.basecampapi.com/999/projects/456.json?x=1")
            .unwrap();
        assert_eq!(matched.operation(), Some("GetProject"));
        assert_eq!(matched.resource_id(), Some("456"));
        assert_eq!(matched.operations["DELETE"], "TrashProject");
    }

    #[test]
    fn recognizes_bare_paths_and_refuses_unknown_ones() {
        assert_eq!(
            router().recognize("/projects.json").unwrap().operation(),
            Some("ListProjects")
        );
        assert!(router().recognize("/nothing/here").is_none());
        assert!(router().recognize("/projects//todolists").is_none());
    }
}
