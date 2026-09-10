use std::fmt::Write;

use crate::emit::{HEADER, doc_comment, string_literal};
use crate::model::{Field, FieldType, Model, Role, Schema, Shape};
use crate::naming::{field_ident, variant_name};

pub(crate) fn render(model: &Model) -> String {
    let mut out = String::from(HEADER);
    out.push_str("//! The request and response shapes the Basecamp API speaks.\n\n");
    // A field of a deprecated type warns at its declaration; the field carries its own
    // `#[deprecated]` for callers, so the module's own reference is the one silenced.
    out.push_str("#![allow(deprecated, clippy::doc_markdown)]\n\n");
    out.push_str("use std::collections::BTreeMap;\n\n");
    out.push_str("use serde::{Deserialize, Serialize};\n\n");
    out.push_str(
        "use crate::types::{AuthRoutableUrl, Date, DateTime, FlexibleTime, SensitiveString};\n\n",
    );
    for schema in &model.schemas {
        render_schema(&mut out, schema);
    }
    out
}

fn render_schema(out: &mut String, schema: &Schema) {
    match &schema.shape {
        Shape::Alias(kind) => {
            doc(out, schema, &format!("`{}`.", schema.name));
            writeln!(
                out,
                "pub type {} = {};\n",
                schema.name,
                rust_type(kind, false)
            )
            .unwrap();
        }
        Shape::Bytes => {
            doc(out, schema, "A raw byte payload, sent as the request body.");
            writeln!(out, "pub type {} = Vec<u8>;\n", schema.name).unwrap();
        }
        Shape::Enum(values) => render_enum(out, schema, values),
        Shape::Struct(fields) => render_struct(out, schema, fields),
    }
}

fn doc(out: &mut String, schema: &Schema, fallback: &str) {
    match &schema.description {
        Some(description) => out.push_str(&doc_comment(Some(description), "")),
        None => writeln!(out, "/// {fallback}").unwrap(),
    }
    if let Some(note) = &schema.deprecated {
        writeln!(out, "#[deprecated(note = {})]", string_literal(note)).unwrap();
    }
}

fn render_struct(out: &mut String, schema: &Schema, fields: &[Field]) {
    doc(
        out,
        schema,
        &format!("The `{}` shape of the Basecamp API.", schema.name),
    );
    out.push_str("#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]\n");
    if schema.role == Role::Response {
        out.push_str("#[non_exhaustive]\n");
    }
    writeln!(out, "pub struct {} {{", schema.name).unwrap();
    for field in fields {
        let ident = field_ident(&field.wire_name);
        match &field.description {
            Some(description) => out.push_str(&doc_comment(Some(description), "    ")),
            None => writeln!(out, "    /// `{}`.", field.wire_name).unwrap(),
        }
        if let Some(note) = &field.deprecated {
            writeln!(out, "    #[deprecated(note = {})]", string_literal(note)).unwrap();
        }
        let mut attributes = Vec::new();
        if ident.trim_start_matches("r#") != field.wire_name {
            attributes.push(format!("rename = {}", string_literal(&field.wire_name)));
        }
        let kind = rust_type(&field.kind, field.recursive);
        let declared = match (field.required, field.nullable, &field.kind) {
            // Present-and-nullable: the key must be there, the value may be null. Serde's
            // missing-means-None shortcut is switched off so an absent key still fails.
            (true, true, FieldType::FlexInt) => {
                attributes
                    .push("deserialize_with = \"crate::types::flex_int::deserialize\"".into());
                format!("Option<{kind}>")
            }
            (true, true, _) => {
                attributes.push("deserialize_with = \"serde::Deserialize::deserialize\"".into());
                format!("Option<{kind}>")
            }
            (true, false, FieldType::FlexInt) => {
                attributes.push("default".into());
                attributes
                    .push("deserialize_with = \"crate::types::flex_int::deserialize\"".into());
                format!("Option<{kind}>")
            }
            (true, false, FieldType::FlexibleInt64) => {
                attributes
                    .push("deserialize_with = \"crate::types::flexible_i64::deserialize\"".into());
                kind
            }
            // A required member is always present (SPEC §10): an absent key is a malformed
            // body, never a zero value read in its place.
            (true, false, _) => kind,
            (false, _, FieldType::FlexInt) => {
                attributes.push("default".into());
                attributes
                    .push("deserialize_with = \"crate::types::flex_int::deserialize\"".into());
                attributes.push("skip_serializing_if = \"Option::is_none\"".into());
                format!("Option<{kind}>")
            }
            (false, _, FieldType::FlexibleInt64) => {
                attributes.push("default".into());
                attributes.push(
                    "deserialize_with = \"crate::types::flexible_i64::deserialize_optional\""
                        .into(),
                );
                attributes.push("skip_serializing_if = \"Option::is_none\"".into());
                format!("Option<{kind}>")
            }
            (false, _, _) => {
                attributes.push("default".into());
                attributes.push("skip_serializing_if = \"Option::is_none\"".into());
                format!("Option<{kind}>")
            }
        };
        if !attributes.is_empty() {
            writeln!(out, "    #[serde({})]", attributes.join(", ")).unwrap();
        }
        writeln!(out, "    pub {ident}: {declared},").unwrap();
    }
    out.push_str("}\n\n");
}

fn render_enum(out: &mut String, schema: &Schema, values: &[String]) {
    doc(
        out,
        schema,
        &format!("The `{}` values the Basecamp API declares.", schema.name),
    );
    out.push_str("#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]\n#[non_exhaustive]\n");
    writeln!(out, "pub enum {} {{", schema.name).unwrap();
    for (index, value) in values.iter().enumerate() {
        if index == 0 {
            out.push_str("    #[default]\n");
        }
        writeln!(out, "    /// `{value}`.\n    {},", variant_name(value)).unwrap();
    }
    out.push_str("    /// A value this SDK does not know, kept as the API sent it so a read-modify-write\n    /// round-trips it.\n    Unknown(String),\n}\n\n");

    writeln!(out, "impl {} {{", schema.name).unwrap();
    out.push_str("    /// The value as the API spells it.\n    pub fn as_str(&self) -> &str {\n        match self {\n");
    for value in values {
        writeln!(
            out,
            "            {}::{} => {},",
            schema.name,
            variant_name(value),
            string_literal(value)
        )
        .unwrap();
    }
    writeln!(out, "            {}::Unknown(value) => value,", schema.name).unwrap();
    out.push_str("        }\n    }\n}\n\n");

    writeln!(out, "impl From<&str> for {} {{", schema.name).unwrap();
    out.push_str("    fn from(value: &str) -> Self {\n        match value {\n");
    for value in values {
        writeln!(
            out,
            "            {} => {}::{},",
            string_literal(value),
            schema.name,
            variant_name(value)
        )
        .unwrap();
    }
    writeln!(
        out,
        "            other => {}::Unknown(other.to_string()),",
        schema.name
    )
    .unwrap();
    out.push_str("        }\n    }\n}\n\n");

    writeln!(
        out,
        "impl std::fmt::Display for {} {{\n    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {{\n        f.write_str(self.as_str())\n    }}\n}}\n",
        schema.name
    )
    .unwrap();
    writeln!(
        out,
        "impl Serialize for {} {{\n    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {{\n        serializer.serialize_str(self.as_str())\n    }}\n}}\n",
        schema.name
    )
    .unwrap();
    writeln!(
        out,
        "impl<'de> Deserialize<'de> for {} {{\n    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {{\n        let value = String::deserialize(deserializer)?;\n        Ok(Self::from(value.as_str()))\n    }}\n}}\n",
        schema.name
    )
    .unwrap();
}

pub(crate) fn rust_type(kind: &FieldType, recursive: bool) -> String {
    match kind {
        FieldType::String => "String".into(),
        FieldType::SensitiveString => "SensitiveString".into(),
        FieldType::AuthRoutableUrl => "AuthRoutableUrl".into(),
        FieldType::DateTime => "DateTime".into(),
        FieldType::Date => "Date".into(),
        FieldType::FlexibleTime => "FlexibleTime".into(),
        FieldType::Bool => "bool".into(),
        FieldType::Int32 | FieldType::FlexInt => "i32".into(),
        FieldType::Int64 | FieldType::FlexibleInt64 => "i64".into(),
        FieldType::Float => "f64".into(),
        FieldType::Json => "serde_json::Value".into(),
        FieldType::Named(name) if recursive => format!("::std::boxed::Box<{name}>"),
        FieldType::Named(name) => name.clone(),
        FieldType::List(inner) => format!("Vec<{}>", rust_type(inner, false)),
        FieldType::Map(inner) => format!("BTreeMap<String, {}>", rust_type(inner, false)),
    }
}
