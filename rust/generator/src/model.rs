use std::collections::{BTreeMap, BTreeSet, VecDeque};

use serde_json::Value;

use crate::naming::{Naming, module_name, struct_name};

pub(crate) struct Model {
    pub(crate) api_version: String,
    pub(crate) schemas: Vec<Schema>,
    pub(crate) services: Vec<Service>,
}

impl Model {
    /// Every operation, in `operationId` order.
    pub(crate) fn operations(&self) -> impl Iterator<Item = &Operation> {
        self.services.iter().flat_map(|service| &service.operations)
    }
}

pub(crate) struct Schema {
    pub(crate) name: String,
    pub(crate) description: Option<String>,
    /// The `x-deprecated-reason`, or a fixed note, when the whole shape is deprecated.
    pub(crate) deprecated: Option<String>,
    pub(crate) role: Role,
    pub(crate) shape: Shape,
}

/// Whether a shape is something a caller builds (a request body, or anything reachable from
/// one) or something the API answers with. Response shapes are `#[non_exhaustive]` so a
/// field the API grows is not a breaking change; request shapes stay literal-constructible.
#[derive(Clone, Copy, PartialEq, Eq)]
pub(crate) enum Role {
    Request,
    Response,
}

pub(crate) enum Shape {
    Struct(Vec<Field>),
    Alias(FieldType),
    Enum(Vec<String>),
    /// A raw byte payload (`contentEncoding: byte`), sent as an octet-stream body.
    Bytes,
}

pub(crate) struct Field {
    pub(crate) wire_name: String,
    pub(crate) description: Option<String>,
    pub(crate) kind: FieldType,
    pub(crate) required: bool,
    /// The wire may carry an explicit `null`: a `["T", "null"]` union, an `anyOf` with a
    /// null member, or `nullable: true`.
    pub(crate) nullable: bool,
    pub(crate) recursive: bool,
    pub(crate) deprecated: Option<String>,
}

#[derive(Clone, PartialEq, Eq)]
pub(crate) enum FieldType {
    String,
    SensitiveString,
    AuthRoutableUrl,
    DateTime,
    Date,
    FlexibleTime,
    Bool,
    Int32,
    Int64,
    /// A nullable int32 the API may spell as a float (`1024.0`).
    FlexInt,
    /// An int64 the API may spell as a string.
    FlexibleInt64,
    Float,
    Json,
    Named(String),
    List(Box<FieldType>),
    Map(Box<FieldType>),
}

pub(crate) struct Service {
    /// `PascalCase`, as every Basecamp SDK names it: `CardTables`.
    pub(crate) name: String,
    /// The Rust module: `card_tables`.
    pub(crate) module: String,
    /// The Rust struct: `CardTablesService`.
    pub(crate) struct_name: String,
    pub(crate) operations: Vec<Operation>,
}

pub(crate) struct Operation {
    pub(crate) id: String,
    pub(crate) service: String,
    pub(crate) method_name: String,
    pub(crate) description: Option<String>,
    pub(crate) http_method: String,
    /// The path as the API serves it, without the `/{accountId}` prefix.
    pub(crate) path: String,
    pub(crate) resource_type: String,
    pub(crate) path_params: Vec<PathParam>,
    pub(crate) query_params: Vec<QueryParam>,
    pub(crate) body: Body,
    pub(crate) response: Response,
    /// `behavior-model.json`'s `idempotent: true`: the naturally idempotent mutations, the
    /// flag that opens SPEC §7's Gate 2 for a POST.
    pub(crate) idempotent: bool,
    pub(crate) readonly: bool,
    pub(crate) pagination: Option<Pagination>,
    pub(crate) retry: Retry,
    pub(crate) write: Option<WriteSemantics>,
    pub(crate) deprecated: Option<String>,
}

pub(crate) struct PathParam {
    pub(crate) wire_name: String,
    pub(crate) kind: ParamKind,
}

pub(crate) struct QueryParam {
    pub(crate) wire_name: String,
    pub(crate) kind: ParamKind,
    pub(crate) required: bool,
    pub(crate) description: Option<String>,
    pub(crate) deprecated: Option<String>,
}

#[derive(Clone, Copy, PartialEq, Eq)]
pub(crate) enum ParamKind {
    String,
    Bool,
    Int32,
    Int64,
    StringList,
    Int64List,
}

pub(crate) enum Body {
    None,
    Json(String),
    Octet,
    Multipart { field: String },
}

pub(crate) enum Response {
    Empty,
    Json(String),
}

pub(crate) struct Pagination {
    pub(crate) key: Option<String>,
    pub(crate) total_count_header: Option<String>,
}

#[allow(clippy::struct_field_names)] // `retry_on` is the model's own key
pub(crate) struct Retry {
    pub(crate) max: u32,
    pub(crate) base_delay_ms: u64,
    pub(crate) backoff: String,
    pub(crate) retry_on: Vec<u16>,
}

pub(crate) struct WriteSemantics {
    pub(crate) clears_omitted: bool,
    pub(crate) preserved_on_omission: Vec<String>,
}

impl Model {
    pub(crate) fn build(
        openapi: &Value,
        behavior: &Value,
        naming: &Naming,
    ) -> Result<Model, String> {
        let api_version = openapi["info"]["version"]
            .as_str()
            .ok_or("openapi.json has no info.version")?
            .to_string();
        let components = openapi["components"]["schemas"]
            .as_object()
            .ok_or("openapi.json has no components.schemas")?;
        let request_shapes = request_shapes(openapi, components)?;
        let schemas = build_schemas(components, &request_shapes, naming)?;
        let services = build_services(openapi, behavior, naming)?;
        Ok(Model {
            api_version,
            schemas,
            services,
        })
    }
}

/// Every schema a request body references, directly or through another schema.
fn request_shapes(
    openapi: &Value,
    components: &serde_json::Map<String, Value>,
) -> Result<BTreeSet<String>, String> {
    let mut queue = VecDeque::new();
    for (path, item) in openapi["paths"]
        .as_object()
        .ok_or("openapi.json has no paths")?
    {
        for (_, operation) in item.as_object().ok_or(format!("{path} is not an object"))? {
            if let Some(content) = operation["requestBody"]["content"].as_object() {
                for media in content.values() {
                    if let Some(reference) = media["schema"]["$ref"].as_str() {
                        queue.push_back(reference_name(reference));
                    }
                }
            }
        }
    }
    let mut seen = BTreeSet::new();
    while let Some(name) = queue.pop_front() {
        if !seen.insert(name.clone()) {
            continue;
        }
        let schema = components
            .get(&name)
            .ok_or(format!("request body references unknown schema {name}"))?;
        collect_references(schema, &mut queue);
    }
    Ok(seen)
}

fn collect_references(value: &Value, queue: &mut VecDeque<String>) {
    match value {
        Value::Object(object) => {
            for (key, member) in object {
                if key == "$ref" {
                    if let Some(reference) = member.as_str() {
                        queue.push_back(reference_name(reference));
                    }
                } else {
                    collect_references(member, queue);
                }
            }
        }
        Value::Array(items) => {
            for item in items {
                collect_references(item, queue);
            }
        }
        _ => {}
    }
}

fn build_schemas(
    components: &serde_json::Map<String, Value>,
    request_shapes: &BTreeSet<String>,
    naming: &Naming,
) -> Result<Vec<Schema>, String> {
    let mut schemas = Vec::new();
    for (schema_name, schema) in components {
        let name = naming.type_for(schema_name);
        let role = if request_shapes.contains(schema_name) {
            Role::Request
        } else {
            Role::Response
        };
        let shape = if let Some(properties) = schema.get("properties") {
            let required: BTreeSet<&str> = schema["required"]
                .as_array()
                .map(|list| list.iter().filter_map(Value::as_str).collect())
                .unwrap_or_default();
            let properties = properties
                .as_object()
                .ok_or(format!("{name}.properties is not an object"))?;
            let mut fields = Vec::new();
            for (wire_name, property) in properties {
                let kind = field_type(wire_name, property, naming)?;
                fields.push(Field {
                    wire_name: wire_name.clone(),
                    description: description_of(property),
                    recursive: kind.mentions(&name),
                    nullable: is_nullable(property),
                    deprecated: deprecation_of(property),
                    kind,
                    required: required.contains(wire_name.as_str()),
                });
            }
            Shape::Struct(fields)
        } else if let Some(values) = schema["enum"].as_array() {
            Shape::Enum(
                values
                    .iter()
                    .map(|value| {
                        value
                            .as_str()
                            .map(str::to_string)
                            .ok_or(format!("{name}: enum values must be strings"))
                    })
                    .collect::<Result<_, _>>()?,
            )
        } else if schema["contentEncoding"].as_str() == Some("byte") {
            Shape::Bytes
        } else {
            Shape::Alias(field_type(schema_name, schema, naming)?)
        };
        schemas.push(Schema {
            name,
            description: description_of(schema),
            deprecated: deprecation_of(schema),
            role,
            shape,
        });
    }
    Ok(schemas)
}

fn is_nullable(property: &Value) -> bool {
    property["nullable"].as_bool() == Some(true)
        || property["type"]
            .as_array()
            .is_some_and(|types| types.iter().any(|t| t.as_str() == Some("null")))
        || property["anyOf"]
            .as_array()
            .is_some_and(|members| members.iter().any(|m| m["type"].as_str() == Some("null")))
}

fn deprecation_of(value: &Value) -> Option<String> {
    if value["deprecated"].as_bool() == Some(true) {
        Some(
            value["x-deprecated-reason"]
                .as_str()
                .unwrap_or("deprecated")
                .to_string(),
        )
    } else {
        None
    }
}

/// The non-null member of an `OpenAPI` 3.1 `["T", "null"]` union, or the scalar `type`. A
/// union of two non-null types has no Rust shape and is refused rather than guessed at.
fn scalar_type<'a>(name: &str, property: &'a Value) -> Result<Option<&'a str>, String> {
    match &property["type"] {
        Value::String(t) => Ok(Some(t.as_str())),
        Value::Array(types) => {
            let members: Vec<&str> = types
                .iter()
                .filter_map(Value::as_str)
                .filter(|t| *t != "null")
                .collect();
            match members.as_slice() {
                [] => Ok(None),
                [only] => Ok(Some(only)),
                many => Err(format!(
                    "{name}: a union of {} has no Rust type",
                    many.join(" and ")
                )),
            }
        }
        _ => Ok(None),
    }
}

fn field_type(name: &str, property: &Value, naming: &Naming) -> Result<FieldType, String> {
    if let Some(reference) = property.get("$ref").and_then(Value::as_str) {
        return Ok(FieldType::Named(
            naming.type_for(&reference_name(reference)),
        ));
    }
    // `anyOf: [$ref, {type: null}]` — a present-but-nullable reference.
    if let Some(members) = property["anyOf"].as_array() {
        let others: Vec<&Value> = members
            .iter()
            .filter(|member| member["type"].as_str() != Some("null"))
            .collect();
        return match others.as_slice() {
            [only] => field_type(name, only, naming),
            _ => Err(format!(
                "{name}: anyOf with more than one non-null member is unsupported"
            )),
        };
    }
    for composition in ["oneOf", "allOf"] {
        if property.get(composition).is_some() {
            return Err(format!("{name}: {composition} has no Rust shape"));
        }
    }
    let go_type = property["x-go-type"].as_str();
    let format = property["format"].as_str();
    match scalar_type(name, property)? {
        Some("string") => Ok(string_type(property, go_type, format)),
        Some("boolean") => Ok(FieldType::Bool),
        Some("integer") => Ok(match (go_type, format) {
            (Some("types.FlexInt"), _) => FieldType::FlexInt,
            (Some("types.FlexibleInt64"), _) => FieldType::FlexibleInt64,
            (_, Some("int32")) => FieldType::Int32,
            _ => FieldType::Int64,
        }),
        Some("number") => Ok(FieldType::Float),
        Some("array") => {
            if is_nullable(&property["items"]) {
                return Err(format!("{name}: nullable array items have no Rust shape"));
            }
            Ok(FieldType::List(Box::new(field_type(
                name,
                &property["items"],
                naming,
            )?)))
        }
        Some("object") => {
            if let Some(values) = property.get("additionalProperties") {
                Ok(FieldType::Map(Box::new(field_type(name, values, naming)?)))
            } else {
                Ok(FieldType::Json)
            }
        }
        None if property.get("items").is_some() => Err(format!("{name}: array without a type")),
        None => Ok(FieldType::Json),
        Some(other) => Err(format!("{name}: unsupported schema type {other:?}")),
    }
}

fn string_type(property: &Value, go_type: Option<&str>, format: Option<&str>) -> FieldType {
    if property.get("x-basecamp-auth-routable-url").is_some() {
        FieldType::AuthRoutableUrl
    } else if property["x-basecamp-sensitive"]["redact"].as_bool() == Some(true) {
        FieldType::SensitiveString
    } else {
        match (go_type, format) {
            (Some("types.FlexibleTime"), _) => FieldType::FlexibleTime,
            (Some("time.Time" | "types.DateTime"), _) | (None, Some("date-time")) => {
                FieldType::DateTime
            }
            (Some("types.Date"), _) | (None, Some("date")) => FieldType::Date,
            _ => FieldType::String,
        }
    }
}

fn build_services(
    openapi: &Value,
    behavior: &Value,
    naming: &Naming,
) -> Result<Vec<Service>, String> {
    let paths = openapi["paths"]
        .as_object()
        .ok_or("openapi.json has no paths")?;
    let behaviors = behavior["operations"]
        .as_object()
        .ok_or("behavior-model.json has no operations")?;
    let mut services: BTreeMap<String, Vec<Operation>> = BTreeMap::new();

    for (path, item) in paths {
        for (http_method, operation) in
            item.as_object().ok_or(format!("{path} is not an object"))?
        {
            if !matches!(
                http_method.as_str(),
                "get" | "post" | "put" | "patch" | "delete"
            ) {
                continue;
            }
            let id = operation["operationId"]
                .as_str()
                .ok_or(format!("{http_method} {path} has no operationId"))?;
            let service = naming.service_for(id, operation["tags"][0].as_str())?;
            let semantics = behaviors
                .get(id)
                .ok_or(format!("{id} is missing from behavior-model.json"))?;
            let sdk_path = path
                .strip_prefix("/{accountId}")
                .ok_or(format!("{path} is not account-scoped"))?
                .to_string();
            let path_params = path_params(operation, &sdk_path)?;
            let operation = Operation {
                id: id.to_string(),
                service: service.clone(),
                method_name: naming.method_for(id)?,
                description: description_of(operation),
                http_method: http_method.to_uppercase(),
                path: sdk_path,
                resource_type: naming.resource_type_for(id),
                path_params,
                query_params: query_params(operation)?,
                body: body_of(operation, naming)?,
                response: response_of(operation, naming)?,
                idempotent: semantics["idempotent"].as_bool() == Some(true),
                readonly: semantics["readonly"].as_bool() == Some(true),
                pagination: pagination(operation, semantics)?,
                retry: retry(id, semantics)?,
                write: write_semantics(operation),
                deprecated: deprecation_of(operation),
            };
            services.entry(service).or_default().push(operation);
        }
    }

    let mut result = Vec::new();
    for (name, mut operations) in services {
        operations.sort_by(|a, b| a.method_name.cmp(&b.method_name).then(a.id.cmp(&b.id)));
        for pair in operations.windows(2) {
            if pair[0].method_name == pair[1].method_name {
                return Err(format!(
                    "{} and {} both become {}::{}; add an [operation_methods] override to names.toml",
                    pair[0].id, pair[1].id, name, pair[0].method_name
                ));
            }
        }
        result.push(Service {
            module: module_name(&name),
            struct_name: struct_name(&name),
            name,
            operations,
        });
    }
    Ok(result)
}

/// The path parameters in the order the template names them — the order of the generated
/// method's arguments — each bound to exactly one placeholder.
fn path_params(operation: &Value, path: &str) -> Result<Vec<PathParam>, String> {
    let id = operation["operationId"].as_str().unwrap_or("operation");
    let mut params = Vec::new();
    for parameter in parameters_in(operation, "path") {
        let wire_name = parameter["name"]
            .as_str()
            .ok_or("path parameter without a name")?
            .to_string();
        if wire_name == "accountId" {
            continue;
        }
        let position = path
            .find(&format!("{{{wire_name}}}"))
            .ok_or(format!("{id}: path parameter {wire_name} is not in {path}"))?;
        params.push((
            position,
            PathParam {
                wire_name,
                kind: param_kind(parameter)?,
            },
        ));
    }
    params.sort_by_key(|(position, _)| *position);
    let placeholders = path.matches('{').count();
    if placeholders != params.len() {
        return Err(format!(
            "{id}: {path} names {placeholders} placeholder(s) but {} path parameter(s) are declared",
            params.len()
        ));
    }
    Ok(params.into_iter().map(|(_, param)| param).collect())
}

fn query_params(operation: &Value) -> Result<Vec<QueryParam>, String> {
    let mut params = Vec::new();
    for parameter in parameters_in(operation, "query") {
        let deprecated = if parameter["deprecated"].as_bool() == Some(true)
            || parameter["schema"]["deprecated"].as_bool() == Some(true)
        {
            Some(
                parameter["x-deprecated-reason"]
                    .as_str()
                    .or(parameter["schema"]["x-deprecated-reason"].as_str())
                    .unwrap_or("deprecated")
                    .to_string(),
            )
        } else {
            None
        };
        params.push(QueryParam {
            wire_name: parameter["name"]
                .as_str()
                .ok_or("query parameter without a name")?
                .to_string(),
            kind: param_kind(parameter)?,
            required: parameter["required"].as_bool().unwrap_or(false),
            description: description_of(parameter),
            deprecated,
        });
    }
    Ok(params)
}

fn parameters_in<'a>(operation: &'a Value, location: &'a str) -> impl Iterator<Item = &'a Value> {
    operation["parameters"]
        .as_array()
        .into_iter()
        .flatten()
        .filter(move |parameter| parameter["in"].as_str() == Some(location))
}

fn param_kind(parameter: &Value) -> Result<ParamKind, String> {
    let schema = &parameter["schema"];
    match (schema["type"].as_str(), schema["format"].as_str()) {
        (Some("string"), _) => Ok(ParamKind::String),
        (Some("boolean"), _) => Ok(ParamKind::Bool),
        (Some("integer"), Some("int32")) => Ok(ParamKind::Int32),
        (Some("integer"), _) => Ok(ParamKind::Int64),
        (Some("array"), _) => match schema["items"]["type"].as_str() {
            Some("integer") => Ok(ParamKind::Int64List),
            Some("string") => Ok(ParamKind::StringList),
            other => Err(format!(
                "parameter {}: unsupported array item type {other:?}",
                parameter["name"]
            )),
        },
        other => Err(format!(
            "parameter {}: unsupported type {other:?}",
            parameter["name"]
        )),
    }
}

fn body_of(operation: &Value, naming: &Naming) -> Result<Body, String> {
    let Some(content) = operation["requestBody"]["content"].as_object() else {
        return Ok(Body::None);
    };
    if let Some(json) = content.get("application/json") {
        let reference = json["schema"]["$ref"].as_str().ok_or(format!(
            "{}: JSON body without a $ref",
            operation["operationId"]
        ))?;
        Ok(Body::Json(naming.type_for(&reference_name(reference))))
    } else if content.contains_key("application/octet-stream") {
        Ok(Body::Octet)
    } else if content.contains_key("multipart/form-data") {
        let field = operation["x-basecamp-multipart"]["field"]
            .as_str()
            .ok_or(format!(
                "{}: multipart body without x-basecamp-multipart.field",
                operation["operationId"]
            ))?;
        Ok(Body::Multipart {
            field: field.to_string(),
        })
    } else {
        Err(format!(
            "{}: unsupported request content {:?}",
            operation["operationId"],
            content.keys().collect::<Vec<_>>()
        ))
    }
}

fn response_of(operation: &Value, naming: &Naming) -> Result<Response, String> {
    let responses = operation["responses"]
        .as_object()
        .ok_or("operation has no responses")?;
    let successes: Vec<&Value> = responses
        .iter()
        .filter(|(status, _)| status.starts_with('2'))
        .map(|(_, response)| response)
        .collect();
    let Some(first_success) = successes.first() else {
        return Err(format!("{} has no 2xx response", operation["operationId"]));
    };
    if successes
        .iter()
        .any(|response| response["content"] != first_success["content"])
    {
        return Err(format!(
            "{}: its 2xx responses disagree on the body",
            operation["operationId"].as_str().unwrap_or("operation")
        ));
    }
    for (status, response) in responses {
        if status.starts_with('2') {
            return match response["content"].as_object() {
                None => Ok(Response::Empty),
                Some(content) => match content.get("application/json") {
                    Some(json) => {
                        let reference = json["schema"]["$ref"].as_str().ok_or(format!(
                            "{}: JSON response without a $ref",
                            operation["operationId"]
                        ))?;
                        Ok(Response::Json(naming.type_for(&reference_name(reference))))
                    }
                    None => Err(format!(
                        "{}: unsupported response representation {:?}",
                        operation["operationId"],
                        content.keys().collect::<Vec<_>>()
                    )),
                },
            };
        }
    }
    Err(format!("{} has no 2xx response", operation["operationId"]))
}

fn pagination(operation: &Value, semantics: &Value) -> Result<Option<Pagination>, String> {
    let extension = &operation["x-basecamp-pagination"];
    if extension.is_null() {
        return Ok(None);
    }
    match extension["style"].as_str() {
        Some("link") => {}
        other => return Err(format!("unsupported pagination style {other:?}")),
    }
    if semantics["pagination"].is_null() {
        return Err(format!(
            "{} paginates in openapi.json but not in behavior-model.json",
            operation["operationId"]
        ));
    }
    Ok(Some(Pagination {
        key: extension["key"].as_str().map(str::to_string),
        total_count_header: extension["totalCountHeader"].as_str().map(str::to_string),
    }))
}

fn retry(id: &str, semantics: &Value) -> Result<Retry, String> {
    let retry = &semantics["retry"];
    if retry.is_null() {
        return Err(format!("{id} has no retry block in behavior-model.json"));
    }
    let max = retry["max"]
        .as_u64()
        .and_then(|max| u32::try_from(max).ok())
        .ok_or(format!("{id}: retry.max is not a number"))?;
    let base_delay_ms = retry["base_delay_ms"]
        .as_u64()
        .ok_or(format!("{id}: retry.base_delay_ms is not a number"))?;
    let backoff = retry["backoff"]
        .as_str()
        .ok_or(format!("{id}: retry.backoff is not a string"))?
        .to_string();
    let retry_on = retry["retry_on"]
        .as_array()
        .ok_or(format!("{id}: retry.retry_on is not a list"))?
        .iter()
        .map(|code| {
            code.as_u64()
                .and_then(|code| u16::try_from(code).ok())
                .ok_or(format!("{id}: retry_on holds a non-status"))
        })
        .collect::<Result<_, _>>()?;
    Ok(Retry {
        max,
        base_delay_ms,
        backoff,
        retry_on,
    })
}

fn write_semantics(operation: &Value) -> Option<WriteSemantics> {
    let extension = operation.get("x-basecamp-write-semantics")?;
    Some(WriteSemantics {
        clears_omitted: extension["clearsOmitted"].as_bool().unwrap_or(false),
        preserved_on_omission: extension["preservedOnOmission"]
            .as_array()
            .map(|list| {
                list.iter()
                    .filter_map(Value::as_str)
                    .map(str::to_string)
                    .collect()
            })
            .unwrap_or_default(),
    })
}

fn description_of(value: &Value) -> Option<String> {
    value["description"]
        .as_str()
        .or_else(|| value["summary"].as_str())
        .map(str::to_string)
}

fn reference_name(reference: &str) -> String {
    reference
        .trim_start_matches("#/components/schemas/")
        .to_string()
}

impl FieldType {
    fn mentions(&self, schema: &str) -> bool {
        match self {
            FieldType::Named(name) => name == schema,
            FieldType::List(inner) | FieldType::Map(inner) => inner.mentions(schema),
            _ => false,
        }
    }
}
