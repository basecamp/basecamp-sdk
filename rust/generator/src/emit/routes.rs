use std::fmt::Write;

use crate::emit::{HEADER, string_literal};
use crate::model::{Body, Model, Operation, ParamKind, Response};
use crate::naming::constant_name;

pub fn render(model: &Model) -> String {
    let mut out = String::from(HEADER);
    out.push_str("//! Every modelled route as public data: method, path, parameters and the behaviour the\n//! model attaches to it.\n\n");
    out.push_str("use crate::generated::metadata;\n");
    out.push_str("use crate::http::Method;\n");
    out.push_str(
        "use crate::route::{BodyKind, Pagination, ParamKind, Representation, Route, RouteParam, WriteSemantics};\n\n",
    );

    let mut operations: Vec<&Operation> = model.operations().collect();
    operations.sort_by(|a, b| a.id.cmp(&b.id));

    for operation in &operations {
        render_route(&mut out, operation);
    }

    out.push_str("/// Every route the SDK knows, one per operation, in `operationId` order.\n");
    out.push_str("pub static ROUTES: &[&Route] = &[\n");
    for operation in &operations {
        writeln!(out, "    &{},", constant_name(&operation.id)).unwrap();
    }
    out.push_str("];\n");
    out
}

fn render_route(out: &mut String, operation: &Operation) {
    let pattern = operation.path.trim_end_matches(".json");
    writeln!(out, "/// `{} {}`.", operation.http_method, operation.path).unwrap();
    writeln!(
        out,
        "pub static {}: Route = Route {{",
        constant_name(&operation.id)
    )
    .unwrap();
    writeln!(out, "    id: \"{}\",", operation.id).unwrap();
    writeln!(out, "    service: \"{}\",", operation.service).unwrap();
    writeln!(out, "    method: Method::{},", operation.http_method).unwrap();
    writeln!(out, "    path: \"{}\",", operation.path).unwrap();
    writeln!(out, "    pattern: \"{pattern}\",").unwrap();
    writeln!(out, "    resource_type: \"{}\",", operation.resource_type).unwrap();
    out.push_str("    params: &[\n");
    for param in &operation.path_params {
        writeln!(
            out,
            "        RouteParam {{ name: \"{}\", kind: ParamKind::{} }},",
            param.wire_name,
            param_kind(param.kind)
        )
        .unwrap();
    }
    out.push_str("    ],\n");
    let body = match &operation.body {
        Body::None => "BodyKind::None".to_string(),
        Body::Json(_) => "BodyKind::Json".to_string(),
        Body::Octet => "BodyKind::Octet".to_string(),
        Body::Multipart { field } => {
            format!("BodyKind::Multipart {{ field: {} }}", string_literal(field))
        }
    };
    writeln!(out, "    body: {body},").unwrap();
    let response = match &operation.response {
        Response::Empty => "Representation::Empty",
        Response::Json(_) => "Representation::Json",
    };
    writeln!(out, "    response: {response},").unwrap();
    let pagination = match &operation.pagination {
        None => "Pagination::None".to_string(),
        Some(pagination) => format!(
            "Pagination::Link {{ key: {}, total_count_header: {} }}",
            option_literal(pagination.key.as_deref()),
            option_literal(pagination.total_count_header.as_deref())
        ),
    };
    writeln!(out, "    pagination: {pagination},").unwrap();
    let write = match &operation.write {
        None => "None".to_string(),
        Some(write) => format!(
            "Some(&WriteSemantics {{ clears_omitted: {}, preserved_on_omission: &{:?} }})",
            write.clears_omitted, write.preserved_on_omission
        ),
    };
    writeln!(out, "    write: {write},").unwrap();
    writeln!(out, "    deprecated: {},", operation.deprecated.is_some()).unwrap();
    writeln!(
        out,
        "    metadata: &metadata::{},",
        constant_name(&operation.id)
    )
    .unwrap();
    out.push_str("};\n\n");
}

fn option_literal(value: Option<&str>) -> String {
    match value {
        Some(value) => format!("Some({})", string_literal(value)),
        None => "None".to_string(),
    }
}

pub fn param_kind(kind: ParamKind) -> &'static str {
    match kind {
        ParamKind::String => "String",
        ParamKind::Bool => "Bool",
        ParamKind::Int32 => "Int32",
        ParamKind::Int64 => "Int64",
        ParamKind::StringList => "StringList",
        ParamKind::Int64List => "Int64List",
    }
}
