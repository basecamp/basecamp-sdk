use std::collections::BTreeMap;

use heck::{ToPascalCase, ToSnakeCase};
use serde::Deserialize;

/// The naming tables every Basecamp SDK generator carries by hand, read from `names.toml`.
/// `services` and `operation_services` are the `TAG_TO_SERVICE` and `SERVICE_SPLITS` tables
/// the other five generators transcribe; `operation_methods` is `METHOD_NAME_OVERRIDES`;
/// `operation_resource_types` is `RESOURCE_TYPE_OVERRIDES`. They must agree operation by
/// operation, and `scripts/check-operation-assignment-parity` fails when they do not.
#[derive(Deserialize, Default)]
pub(crate) struct Naming {
    #[serde(default)]
    services: BTreeMap<String, String>,
    #[serde(default)]
    operation_services: BTreeMap<String, String>,
    #[serde(default)]
    operation_methods: BTreeMap<String, String>,
    #[serde(default)]
    operation_resource_types: BTreeMap<String, String>,
    #[serde(default)]
    type_names: BTreeMap<String, String>,
}

const KEYWORDS: &[&str] = &[
    "abstract", "as", "async", "await", "become", "box", "break", "const", "continue", "crate",
    "do", "dyn", "else", "enum", "extern", "false", "final", "fn", "for", "gen", "if", "impl",
    "in", "let", "loop", "macro", "match", "mod", "move", "mut", "override", "priv", "pub", "ref",
    "return", "self", "static", "struct", "super", "trait", "true", "try", "type", "typeof",
    "unsafe", "unsized", "use", "virtual", "where", "while", "yield",
];

/// The verb prefixes the SPEC §18 naming algorithm strips, in the order every generator
/// tries them.
const VERB_PATTERNS: &[(&str, &str)] = &[
    ("Subscribe", "subscribe"),
    ("Unsubscribe", "unsubscribe"),
    ("List", "list"),
    ("Get", "get"),
    ("Create", "create"),
    ("Update", "update"),
    ("Replace", "replace"),
    ("Delete", "delete"),
    ("Trash", "trash"),
    ("Archive", "archive"),
    ("Unarchive", "unarchive"),
    ("Complete", "complete"),
    ("Uncomplete", "uncomplete"),
    ("Enable", "enable"),
    ("Disable", "disable"),
    ("Reposition", "reposition"),
    ("Move", "move"),
    ("Clone", "clone"),
    ("Set", "set"),
    ("Pin", "pin"),
    ("Unpin", "unpin"),
    ("Pause", "pause"),
    ("Resume", "resume"),
    ("Search", "search"),
];

/// Resource names (lowercased) that collapse to the bare verb: `GetProject` in the projects
/// service is `get`.
const SIMPLE_RESOURCES: &[&str] = &[
    "todo",
    "todos",
    "todolist",
    "todolists",
    "todoset",
    "message",
    "messages",
    "comment",
    "comments",
    "card",
    "cards",
    "cardtable",
    "cardcolumn",
    "cardstep",
    "column",
    "step",
    "project",
    "projects",
    "person",
    "people",
    "campfire",
    "campfires",
    "chatbot",
    "chatbots",
    "webhook",
    "webhooks",
    "vault",
    "vaults",
    "document",
    "documents",
    "upload",
    "uploads",
    "schedule",
    "scheduleentry",
    "scheduleentries",
    "event",
    "events",
    "recording",
    "recordings",
    "template",
    "templates",
    "attachment",
    "question",
    "questions",
    "answer",
    "answers",
    "questionnaire",
    "subscription",
    "forward",
    "forwards",
    "inbox",
    "messageboard",
    "messagetype",
    "messagetypes",
    "tool",
    "lineupmarker",
    "clientapproval",
    "clientapprovals",
    "clientcorrespondence",
    "clientcorrespondences",
    "clientreply",
    "clientreplies",
    "forwardreply",
    "forwardreplies",
    "campfireline",
    "campfirelines",
    "todolistgroup",
    "todolistgroups",
    "todolistorgroup",
    "uploadversions",
    "boost",
    "boosts",
    "hillchart",
    "hillcharts",
    "wormhole",
    "wormholes",
];

impl Naming {
    pub(crate) fn parse(source: &str) -> Result<Naming, String> {
        toml::from_str(source).map_err(|error| format!("names.toml: {error}"))
    }

    /// The service an operation belongs to, `PascalCase` (`CardTables`): the split table
    /// first, then the tag table, then the tag with its spaces removed.
    pub(crate) fn service_for(&self, operation_id: &str, tag: &str) -> String {
        if let Some(service) = self.operation_services.get(operation_id) {
            service.clone()
        } else if let Some(service) = self.services.get(tag) {
            service.clone()
        } else {
            tag.replace(' ', "")
        }
    }

    /// SPEC §18's method-naming algorithm, then `snake_case`d.
    pub(crate) fn method_for(&self, operation_id: &str) -> Result<String, String> {
        let camel = match self.operation_methods.get(operation_id) {
            Some(method) => method.clone(),
            None => derive_method(operation_id),
        };
        let method = camel.to_snake_case();
        if method.is_empty() || KEYWORDS.contains(&method.as_str()) {
            Err(format!(
                "{operation_id} becomes `{method}`, which is not a usable Rust method name; add an [operation_methods] override to names.toml"
            ))
        } else {
            Ok(method)
        }
    }

    /// The noun an operation acts on, as the hooks report it: the override table, else the
    /// verb-stripped remainder, `snake_case`d and singular.
    pub(crate) fn resource_type_for(&self, operation_id: &str) -> String {
        if let Some(resource_type) = self.operation_resource_types.get(operation_id) {
            return resource_type.clone();
        }
        for (prefix, _) in VERB_PATTERNS {
            if let Some(remainder) = operation_id.strip_prefix(prefix) {
                if remainder.is_empty() {
                    return "resource".to_string();
                }
                return singular(&remainder.to_snake_case());
            }
        }
        "resource".to_string()
    }

    /// What a schema is called in Rust. A shape whose Smithy name collides with something
    /// the language already has is renamed here; the wire is untouched.
    pub(crate) fn type_for(&self, schema: &str) -> String {
        self.type_names
            .get(schema)
            .cloned()
            .unwrap_or_else(|| schema.to_string())
    }
}

fn derive_method(operation_id: &str) -> String {
    for (prefix, method) in VERB_PATTERNS {
        if let Some(remainder) = operation_id.strip_prefix(prefix) {
            if remainder.is_empty() {
                return (*method).to_string();
            }
            if SIMPLE_RESOURCES.contains(&remainder.to_lowercase().as_str()) {
                return (*method).to_string();
            }
            return if *method == "get" {
                lower_first(remainder)
            } else {
                format!("{method}{remainder}")
            };
        }
    }
    lower_first(operation_id)
}

fn lower_first(word: &str) -> String {
    let mut characters = word.chars();
    match characters.next() {
        Some(first) => first.to_lowercase().collect::<String>() + characters.as_str(),
        None => String::new(),
    }
}

/// `card_columns` → `card_column`, `entries` → `entry`, `progress` → `progress`.
fn singular(word: &str) -> String {
    if word.ends_with("ss") {
        word.to_string()
    } else if let Some(stem) = word.strip_suffix("ies") {
        format!("{stem}y")
    } else if let Some(stem) = word.strip_suffix("ses") {
        stem.to_string()
    } else if let Some(stem) = word.strip_suffix('s') {
        stem.to_string()
    } else {
        word.to_string()
    }
}

pub(crate) fn module_name(service: &str) -> String {
    service.to_snake_case()
}

pub(crate) fn struct_name(service: &str) -> String {
    format!("{}Service", service.to_pascal_case())
}

/// A wire name as a Rust field or parameter: `bucket_ids[]` → `bucket_ids`, `type` → `r#type`.
pub(crate) fn field_ident(wire_name: &str) -> String {
    let ident = wire_name
        .replace(['[', ']'], "_")
        .trim_end_matches('_')
        .to_snake_case();
    if KEYWORDS.contains(&ident.as_str()) {
        format!("r#{ident}")
    } else {
        ident
    }
}

pub(crate) fn constant_name(operation_id: &str) -> String {
    operation_id.to_snake_case().to_uppercase()
}

pub(crate) fn variant_name(value: &str) -> String {
    let name = value.to_pascal_case();
    if name.chars().next().is_some_and(|c| c.is_ascii_digit()) {
        format!("V{name}")
    } else {
        name
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn methods_follow_the_spec_algorithm() {
        let naming = Naming::default();
        assert_eq!(naming.method_for("ListProjects").unwrap(), "list");
        assert_eq!(naming.method_for("GetProject").unwrap(), "get");
        assert_eq!(
            naming.method_for("GetProjectTimeline").unwrap(),
            "project_timeline"
        );
        assert_eq!(naming.method_for("CreateScheduleEntry").unwrap(), "create");
        assert_eq!(naming.method_for("Search").unwrap(), "search");
        assert_eq!(
            naming.method_for("RecordProjectVisit").unwrap(),
            "record_project_visit"
        );
        assert!(naming.method_for("MoveCard").is_err());
    }

    #[test]
    fn overrides_win_and_are_snake_cased() {
        let naming =
            Naming::parse("[operation_methods]\nUpdateCard = \"updateVerbatim\"\n").unwrap();
        assert_eq!(naming.method_for("UpdateCard").unwrap(), "update_verbatim");
    }

    #[test]
    fn resource_types_are_singular_snake_case() {
        let naming = Naming::default();
        assert_eq!(naming.resource_type_for("ListCardColumns"), "card_column");
        assert_eq!(
            naming.resource_type_for("GetProgressReport"),
            "progress_report"
        );
        assert_eq!(
            naming.resource_type_for("ListScheduleEntries"),
            "schedule_entry"
        );
        assert_eq!(
            naming.resource_type_for("DestroyTimesheetEntry"),
            "resource"
        );
    }

    #[test]
    fn fields_escape_keywords_and_brackets() {
        assert_eq!(field_ident("type"), "r#type");
        assert_eq!(field_ident("bucket_ids[]"), "bucket_ids");
        assert_eq!(field_ident("projectId"), "project_id");
    }

    #[test]
    fn service_names_come_from_the_split_then_the_tag() {
        let naming = Naming::parse(
            "[services]\n\"Card Tables\" = \"CardTables\"\n[operation_services]\nGetCard = \"Cards\"\n",
        )
        .unwrap();
        assert_eq!(naming.service_for("GetCard", "Card Tables"), "Cards");
        assert_eq!(
            naming.service_for("GetCardTable", "Card Tables"),
            "CardTables"
        );
        assert_eq!(naming.service_for("ListFolders", "Folders"), "Folders");
        assert_eq!(module_name("CardTables"), "card_tables");
        assert_eq!(struct_name("CardTables"), "CardTablesService");
    }
}
