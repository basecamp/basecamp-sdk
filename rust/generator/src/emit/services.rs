use std::fmt::Write;

use crate::emit::{HEADER, doc_comment, string_literal};
use crate::model::{Body, Model, Operation, ParamKind, Response, Service};
use crate::naming::{constant_name, field_ident};

pub fn render_mod(model: &Model) -> String {
    let mut out = String::from(HEADER);
    out.push_str("//! One module per service, each a handle on an [`AccountClient`](crate::client::AccountClient).\n\n");
    for service in &model.services {
        writeln!(out, "pub mod {};", service.module).unwrap();
    }
    out
}

pub fn render_service(service: &Service) -> String {
    let name = &service.struct_name;
    let mut out = String::from(HEADER);
    writeln!(out, "//! {} operations.\n", service.name).unwrap();
    out.push_str(
        "#![allow(clippy::too_many_arguments, clippy::wildcard_imports, clippy::doc_markdown)]\n\n",
    );
    if service
        .operations
        .iter()
        .any(|operation| matches!(operation.body, Body::Octet | Body::Multipart { .. }))
    {
        out.push_str("use bytes::Bytes;\n\n");
    }
    out.push_str("use crate::client::AccountClient;\n");
    out.push_str("use crate::error::Error;\n");
    out.push_str("use crate::generated::routes;\n");
    if service.operations.iter().any(uses_types) {
        out.push_str("#[allow(unused_imports)]\nuse crate::generated::types::*;\n");
    }
    if service
        .operations
        .iter()
        .any(|operation| operation.pagination.is_some())
    {
        out.push_str("use crate::pagination::Page;\n");
    }
    out.push('\n');

    for operation in &service.operations {
        render_params(&mut out, operation);
    }

    writeln!(
        out,
        "/// `{}` operations, sent through one [`AccountClient`].",
        service.name
    )
    .unwrap();
    out.push_str("#[derive(Debug, Clone, Copy)]\n");
    writeln!(out, "pub struct {name}<'a> {{").unwrap();
    out.push_str("    client: &'a AccountClient,\n}\n\n");
    writeln!(out, "impl<'a> {name}<'a> {{").unwrap();
    out.push_str(
        "    pub(crate) fn new(client: &'a AccountClient) -> Self {\n        Self { client }\n    }\n\n",
    );
    out.push_str("    /// The client this service sends through.\n");
    out.push_str("    pub fn client(&self) -> &'a AccountClient {\n        self.client\n    }\n\n");
    for operation in &service.operations {
        render_method(&mut out, operation);
    }
    out.push_str("}\n");
    out
}

fn render_params(out: &mut String, operation: &Operation) {
    let optional: Vec<_> = operation
        .query_params
        .iter()
        .filter(|param| !param.required)
        .collect();
    if optional.is_empty() {
        return;
    }
    writeln!(out, "/// Optional query parameters for `{}`.", operation.id).unwrap();
    out.push_str("#[derive(Debug, Clone, Default, PartialEq)]\n");
    writeln!(out, "pub struct {}Params {{", operation.id).unwrap();
    for param in optional {
        match &param.description {
            Some(description) => out.push_str(&doc_comment(Some(description), "    ")),
            None => writeln!(out, "    /// `{}`.", param.wire_name).unwrap(),
        }
        if let Some(note) = &param.deprecated {
            writeln!(out, "    #[deprecated(note = {})]", string_literal(note)).unwrap();
        }
        writeln!(
            out,
            "    pub {}: Option<{}>,",
            field_ident(&param.wire_name),
            owned_type(param.kind)
        )
        .unwrap();
    }
    out.push_str("}\n\n");
}

fn render_method(out: &mut String, operation: &Operation) {
    let mut arguments = vec!["&self".to_string()];
    for param in &operation.path_params {
        arguments.push(format!(
            "{}: {}",
            field_ident(&param.wire_name),
            borrowed_type(param.kind)
        ));
    }
    for param in operation.query_params.iter().filter(|param| param.required) {
        arguments.push(format!(
            "{}: {}",
            field_ident(&param.wire_name),
            borrowed_type(param.kind)
        ));
    }
    let has_params = operation.query_params.iter().any(|param| !param.required);
    if has_params {
        arguments.push(format!("params: &{}Params", operation.id));
    }
    match &operation.body {
        Body::None => {}
        Body::Json(body) => arguments.push(format!("body: &{body}")),
        Body::Octet => {
            arguments.push("content_type: &str".to_string());
            arguments.push("body: Bytes".to_string());
        }
        Body::Multipart { .. } => {
            arguments.push("filename: &str".to_string());
            arguments.push("content_type: &str".to_string());
            arguments.push("body: Bytes".to_string());
        }
    }

    out.push_str(&doc_comment(operation.description.as_deref(), "    "));
    if operation.description.is_some() {
        out.push_str("    ///\n");
    }
    let eligible = operation.idempotent
        || matches!(
            operation.http_method.as_str(),
            "GET" | "HEAD" | "PUT" | "DELETE"
        );
    if eligible {
        writeln!(
            out,
            "    /// `{} {}` — idempotent; retries up to {} attempt(s) on {}.",
            operation.http_method,
            operation.path,
            operation.retry.max,
            operation
                .retry
                .retry_on
                .iter()
                .map(ToString::to_string)
                .collect::<Vec<_>>()
                .join(", ")
        )
        .unwrap();
    } else {
        writeln!(
            out,
            "    /// `{} {}` — not idempotent, sent exactly once.",
            operation.http_method, operation.path
        )
        .unwrap();
    }
    if let Some(note) = &operation.deprecated {
        writeln!(out, "    #[deprecated(note = {})]", string_literal(note)).unwrap();
    }
    if operation
        .query_params
        .iter()
        .any(|param| param.deprecated.is_some())
    {
        out.push_str("    #[allow(deprecated)]\n");
    }
    writeln!(
        out,
        "    pub async fn {}({}) -> Result<{}, Error> {{",
        operation.method_name,
        arguments.join(", "),
        return_type(operation)
    )
    .unwrap();

    let path_arguments: Vec<String> = operation
        .path_params
        .iter()
        .map(|param| format!("&{}", field_ident(&param.wire_name)))
        .collect();
    let binding = if operation.query_params.is_empty() && matches!(operation.body, Body::None) {
        "let"
    } else {
        "let mut"
    };
    writeln!(
        out,
        "        {binding} operation = self.client.operation(&routes::{}, &[{}]);",
        constant_name(&operation.id),
        path_arguments.join(", ")
    )
    .unwrap();
    for param in &operation.query_params {
        let ident = field_ident(&param.wire_name);
        let wire = string_literal(&param.wire_name);
        match (param.required, param.kind) {
            (true, ParamKind::StringList | ParamKind::Int64List) => {
                writeln!(out, "        operation.query_list({wire}, {ident});").unwrap();
            }
            (true, _) => writeln!(out, "        operation.query({wire}, {ident});").unwrap(),
            (false, ParamKind::StringList | ParamKind::Int64List) => writeln!(
                out,
                "        operation.query_list_optional({wire}, params.{ident}.as_deref());"
            )
            .unwrap(),
            (false, _) => writeln!(
                out,
                "        operation.query_optional({wire}, params.{ident}.as_ref());"
            )
            .unwrap(),
        }
    }
    match &operation.body {
        Body::None => {}
        Body::Json(_) => out.push_str("        operation.json(body)?;\n"),
        Body::Octet => out.push_str("        operation.bytes(content_type, body);\n"),
        Body::Multipart { field } => writeln!(
            out,
            "        operation.multipart({}, filename, content_type, &body);",
            string_literal(field)
        )
        .unwrap(),
    }
    writeln!(
        out,
        "        self.client.{}(operation).await",
        send_method(operation)
    )
    .unwrap();
    out.push_str("    }\n\n");
}

fn uses_types(operation: &Operation) -> bool {
    matches!(operation.body, Body::Json(_)) || matches!(operation.response, Response::Json(_))
}

fn return_type(operation: &Operation) -> String {
    match (&operation.response, operation.pagination.is_some()) {
        (Response::Empty, _) => "()".into(),
        (Response::Json(name), true) => format!("Page<{name}>"),
        (Response::Json(name), false) => name.clone(),
    }
}

fn send_method(operation: &Operation) -> &'static str {
    match (&operation.response, operation.pagination.is_some()) {
        (Response::Empty, _) => "send_unit",
        (Response::Json(_), true) => "send_page",
        (Response::Json(_), false) => "send",
    }
}

fn owned_type(kind: ParamKind) -> &'static str {
    match kind {
        ParamKind::String => "String",
        ParamKind::Bool => "bool",
        ParamKind::Int32 => "i32",
        ParamKind::Int64 => "i64",
        ParamKind::StringList => "Vec<String>",
        ParamKind::Int64List => "Vec<i64>",
    }
}

fn borrowed_type(kind: ParamKind) -> &'static str {
    match kind {
        ParamKind::String => "&str",
        ParamKind::Bool => "bool",
        ParamKind::Int32 => "i32",
        ParamKind::Int64 => "i64",
        ParamKind::StringList => "&[String]",
        ParamKind::Int64List => "&[i64]",
    }
}
