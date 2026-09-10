//! A request the client has not sent yet.

use std::fmt::Display;

use bytes::{Bytes, BytesMut};
use serde::Serialize;
use url::Url;

use crate::error::Error;
use crate::hooks::OperationInfo;
use crate::http::Method;
use crate::route::Route;

/// A request the client has not sent yet. Generated service methods build one from a
/// [`Route`]; reach for it directly only to add a query parameter they do not expose.
#[derive(Debug, Clone)]
pub struct Operation {
    pub(crate) route: &'static Route,
    pub(crate) info: OperationInfo,
    pub(crate) method: Method,
    pub(crate) path: String,
    /// An absolute URL to send to instead of the path: a follow-on page.
    pub(crate) url: Option<Url>,
    pub(crate) query: Vec<(String, String)>,
    pub(crate) body: Option<Body>,
}

#[derive(Debug, Clone)]
pub(crate) struct Body {
    pub(crate) content_type: String,
    pub(crate) bytes: Bytes,
}

impl Operation {
    /// What the hooks are told about a route, before any path parameter is known.
    pub(crate) fn info_for(route: &'static Route) -> OperationInfo {
        OperationInfo {
            service: route.service,
            operation: route.id,
            resource_type: route.resource_type,
            is_mutation: route.method != Method::GET,
            project_id: None,
            resource_id: None,
        }
    }

    pub(crate) fn for_route(route: &'static Route, params: &[&dyn Display]) -> Operation {
        let path = route.fill(params);
        let (project_id, resource_id) = ids_of(route, params);
        Operation {
            route,
            info: OperationInfo {
                project_id,
                resource_id,
                ..Operation::info_for(route)
            },
            method: route.method.clone(),
            path,
            url: None,
            query: Vec::new(),
            body: None,
        }
    }

    pub(crate) fn at(route: &'static Route, info: OperationInfo, url: Url) -> Operation {
        Operation {
            route,
            info,
            method: Method::GET,
            path: url.path().to_string(),
            url: Some(url),
            query: Vec::new(),
            body: None,
        }
    }

    /// The route this sends.
    pub fn route(&self) -> &'static Route {
        self.route
    }

    /// The `operationId`.
    pub fn id(&self) -> &'static str {
        self.route.id
    }

    /// What the hooks are told about this operation.
    pub fn info(&self) -> &OperationInfo {
        &self.info
    }

    /// The HTTP method.
    pub fn method(&self) -> &Method {
        &self.method
    }

    /// The path, parameters filled in, without the account prefix.
    pub fn path(&self) -> &str {
        &self.path
    }

    /// Adds a query parameter.
    pub fn query(&mut self, name: &str, value: impl Display) -> &mut Operation {
        self.query.push((name.to_string(), value.to_string()));
        self
    }

    /// Adds a query parameter when there is a value for it.
    pub fn query_optional<T: Display>(&mut self, name: &str, value: Option<&T>) -> &mut Operation {
        if let Some(value) = value {
            self.query(name, value);
        }
        self
    }

    /// Adds a repeated query parameter, one pair per element.
    pub fn query_list<T: Display>(&mut self, name: &str, values: &[T]) -> &mut Operation {
        for value in values {
            self.query(name, value);
        }
        self
    }

    /// Adds a repeated query parameter when there is a list for it.
    pub fn query_list_optional<T: Display>(
        &mut self,
        name: &str,
        values: Option<&[T]>,
    ) -> &mut Operation {
        if let Some(values) = values {
            self.query_list(name, values);
        }
        self
    }

    /// A JSON body. Members that are `null` are dropped before the wire (SPEC §18 body
    /// compaction); an empty string is sent as written.
    pub fn json<T: Serialize + ?Sized>(&mut self, body: &T) -> Result<&mut Operation, Error> {
        let value = serde_json::to_value(body).map_err(|error| {
            Error::usage(format!("request body could not be serialized: {error}"))
        })?;
        let compacted = compact(value);
        let bytes = serde_json::to_vec(&compacted).map_err(|error| {
            Error::usage(format!("request body could not be serialized: {error}"))
        })?;
        self.bytes("application/json", Bytes::from(bytes));
        Ok(self)
    }

    /// A body the caller encoded, with its content type.
    pub fn bytes(&mut self, content_type: &str, bytes: Bytes) -> &mut Operation {
        self.body = Some(Body {
            content_type: content_type.to_string(),
            bytes,
        });
        self
    }

    /// One file under a multipart form field.
    pub fn multipart(
        &mut self,
        field: &str,
        filename: &str,
        content_type: &str,
        bytes: &[u8],
    ) -> &mut Operation {
        let boundary = format!("basecamp-sdk-{:032x}", rand::random::<u128>());
        let mut body = BytesMut::new();
        body.extend_from_slice(format!("--{boundary}\r\n").as_bytes());
        body.extend_from_slice(
            format!(
                "Content-Disposition: form-data; name=\"{}\"; filename=\"{}\"\r\n",
                escape_quotes(field),
                escape_quotes(filename)
            )
            .as_bytes(),
        );
        body.extend_from_slice(format!("Content-Type: {content_type}\r\n\r\n").as_bytes());
        body.extend_from_slice(bytes);
        body.extend_from_slice(format!("\r\n--{boundary}--\r\n").as_bytes());
        self.body = Some(Body {
            content_type: format!("multipart/form-data; boundary={boundary}"),
            bytes: body.freeze(),
        });
        self
    }

    /// The body as it will go out, if one has been set.
    pub fn body_bytes(&self) -> Option<&Bytes> {
        self.body.as_ref().map(|body| &body.bytes)
    }

    /// Whether the query selects exactly one page: SPEC §8's positive `page`, whose
    /// answer is never followed.
    pub(crate) fn pins_a_page(&self) -> bool {
        self.query
            .iter()
            .any(|(name, value)| name == "page" && value.parse::<i64>().is_ok_and(|page| page > 0))
    }
}

fn escape_quotes(value: &str) -> String {
    value.replace('"', "%22").replace(['\r', '\n'], "")
}

/// Drops every `null` member of every object, recursively. Arrays keep their nulls: a
/// position in a list is not a field a server merges over.
pub(crate) fn compact(value: serde_json::Value) -> serde_json::Value {
    match value {
        serde_json::Value::Object(members) => serde_json::Value::Object(
            members
                .into_iter()
                .filter(|(_, member)| !member.is_null())
                .map(|(name, member)| (name, compact(member)))
                .collect(),
        ),
        serde_json::Value::Array(items) => {
            serde_json::Value::Array(items.into_iter().map(compact).collect())
        }
        other => other,
    }
}

/// The project the path is scoped to and the record it names: the `bucketId` or
/// `projectId` parameter, and the last numeric parameter when it is not that one. A path
/// naming only the project names no record, as the other SDKs' hook metadata has it.
fn ids_of(route: &Route, params: &[&dyn Display]) -> (Option<i64>, Option<i64>) {
    let mut project_id = None;
    let mut resource_id = None;
    for (param, value) in route.params.iter().zip(params) {
        let Ok(id) = value.to_string().parse::<i64>() else {
            continue;
        };
        if param.name == "bucketId" || param.name == "projectId" {
            project_id = Some(id);
        }
        resource_id = Some(id);
    }
    if route.params.len() == 1 && project_id.is_some() {
        resource_id = None;
    }
    (project_id, resource_id)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    #[test]
    fn ids_name_the_project_and_the_record_the_path_names() {
        use crate::generated::routes;
        assert_eq!(ids_of(&routes::GET_PROJECT, &[&12]), (Some(12), None));
        assert_eq!(
            ids_of(&routes::LIST_PROJECT_PEOPLE, &[&12]),
            (Some(12), None)
        );
        assert_eq!(ids_of(&routes::GET_TODO, &[&34]), (None, Some(34)));
        assert_eq!(
            ids_of(&routes::CREATE_CLOUD_FILE, &[&12, &56]),
            (Some(12), Some(56))
        );
    }

    #[test]
    fn compaction_strips_nulls_but_not_empty_strings() {
        let compacted =
            compact(json!({"a": null, "b": "", "c": {"d": null, "e": 1}, "f": [null, 1]}));
        assert_eq!(compacted, json!({"b": "", "c": {"e": 1}, "f": [null, 1]}));
    }
}
