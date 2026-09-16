//! Generator correctness: a small model with every shape the generator has to read is
//! rendered and compared against golden output, and the models it must refuse are refused.
//!
//! Regenerate the golden files with `UPDATE_FIXTURES=1 cargo test -p basecamp-sdk-generator`.

#![allow(clippy::unwrap_used, clippy::expect_used)]

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures")
}

fn generate(root: &Path, output: &Path) -> std::process::Output {
    Command::new(env!("CARGO_BIN_EXE_basecamp-sdk-generator"))
        .arg("--root")
        .arg(root)
        .arg("--output")
        .arg(output)
        .output()
        .expect("the generator runs")
}

fn files(root: &Path, directory: &Path) -> Vec<PathBuf> {
    let mut found = Vec::new();
    for entry in fs::read_dir(directory).unwrap() {
        let path = entry.unwrap().path();
        if path.is_dir() {
            found.extend(files(root, &path));
        } else {
            found.push(path.strip_prefix(root).unwrap().to_path_buf());
        }
    }
    found.sort();
    found
}

#[test]
fn the_mini_model_renders_the_golden_output() {
    let root = fixtures().join("mini");
    let expected = root.join("expected");
    let scratch = std::env::temp_dir().join(format!(
        "basecamp-sdk-generator-fixture-{}",
        std::process::id()
    ));
    let _ = fs::remove_dir_all(&scratch);
    let output = generate(&root, &scratch);
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    if std::env::var_os("UPDATE_FIXTURES").is_some() {
        let _ = fs::remove_dir_all(&expected);
        for relative in files(&scratch, &scratch) {
            let target = expected.join(&relative);
            fs::create_dir_all(target.parent().unwrap()).unwrap();
            fs::copy(scratch.join(&relative), target).unwrap();
        }
    }
    let rendered = files(&scratch, &scratch);
    assert_eq!(
        rendered,
        files(&expected, &expected),
        "the set of generated files"
    );
    for relative in rendered {
        let actual = fs::read_to_string(scratch.join(&relative)).unwrap();
        let golden = fs::read_to_string(expected.join(&relative)).unwrap();
        assert_eq!(
            actual,
            golden,
            "{} differs from its golden file",
            relative.display()
        );
    }
    let _ = fs::remove_dir_all(&scratch);
}

/// A copy of the mini model with one edit applied, in a scratch directory ready
/// to generate from. The single place that knows which files the mini model is
/// made of -- `refusal` and `acceptance` both build on it, so a fourth input
/// cannot reach one and miss the other.
fn prepared(
    edit: impl FnOnce(&mut serde_json::Value, &mut serde_json::Value),
    kind: &str,
) -> PathBuf {
    let root = std::env::temp_dir().join(format!(
        "basecamp-sdk-generator-{}-{}-{:?}-{}",
        kind,
        std::process::id(),
        std::thread::current().id(),
        rand_suffix()
    ));
    let _ = fs::remove_dir_all(&root);
    fs::create_dir_all(root.join("rust/generator")).unwrap();
    let mini = fixtures().join("mini");
    let mut openapi: serde_json::Value =
        serde_json::from_str(&fs::read_to_string(mini.join("openapi.json")).unwrap()).unwrap();
    let mut behavior: serde_json::Value =
        serde_json::from_str(&fs::read_to_string(mini.join("behavior-model.json")).unwrap())
            .unwrap();
    edit(&mut openapi, &mut behavior);
    fs::write(
        root.join("openapi.json"),
        serde_json::to_string_pretty(&openapi).unwrap(),
    )
    .unwrap();
    fs::write(
        root.join("behavior-model.json"),
        serde_json::to_string_pretty(&behavior).unwrap(),
    )
    .unwrap();
    fs::copy(
        mini.join("rust/generator/names.toml"),
        root.join("rust/generator/names.toml"),
    )
    .unwrap();
    root
}

/// Generated from an edit the generator must REFUSE; answers its stderr.
fn refusal(edit: impl FnOnce(&mut serde_json::Value, &mut serde_json::Value)) -> String {
    let root = prepared(edit, "refusal");
    let output = generate(&root, &root.join("out"));
    let stderr = String::from_utf8_lossy(&output.stderr).into_owned();
    let refused = output.status.success();
    let _ = fs::remove_dir_all(&root);
    assert!(!refused, "the generator should have refused");
    stderr
}

/// Generated from an edit the generator must ACCEPT; answers the file at
/// `relative`. The mirror of `refusal`, for edits where the question is what
/// was emitted rather than what was said. Cleanup happens before the assert so
/// a failure does not leave the scratch directory behind.
fn acceptance(
    edit: impl FnOnce(&mut serde_json::Value, &mut serde_json::Value),
    relative: &str,
) -> String {
    let root = prepared(edit, "acceptance");
    let out = root.join("out");
    let output = generate(&root, &out);
    let rendered = fs::read_to_string(out.join(relative)).ok();
    let stderr = String::from_utf8_lossy(&output.stderr).into_owned();
    let accepted = output.status.success();
    let _ = fs::remove_dir_all(&root);
    assert!(accepted, "the generator should have accepted: {stderr}");
    rendered.unwrap_or_else(|| panic!("{relative} was not generated"))
}

fn rand_suffix() -> u64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    u64::try_from(
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos(),
    )
    .unwrap_or(u64::MAX)
}

#[test]
fn an_unsupported_response_representation_fails_generation() {
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/widgets.json"]["get"]["responses"]["200"]["content"] =
            serde_json::json!({"text/html": {"schema": {"type": "string"}}});
    });
    assert!(
        stderr.contains("unsupported response representation"),
        "{stderr}"
    );
}

#[test]
fn a_method_name_collision_fails_generation() {
    let stderr = refusal(|openapi, behavior| {
        openapi["paths"]["/{accountId}/widgets.json"]["post"]["operationId"] =
            serde_json::json!("Widgets");
        openapi["paths"]["/{accountId}/buckets/{bucketId}/widgets/{widgetId}"]["put"]["operationId"] =
            serde_json::json!("GetWidgets");
        behavior["operations"]["Widgets"] = behavior["operations"]["CreateWidget"].clone();
        behavior["operations"]["GetWidgets"] = behavior["operations"]["ReplaceWidget"].clone();
    });
    assert!(
        stderr.contains("GetWidgets and Widgets both become Widgets::widgets"),
        "{stderr}"
    );
}

#[test]
fn a_keyword_method_name_fails_generation() {
    let stderr = refusal(|openapi, behavior| {
        openapi["paths"]["/{accountId}/widgets.json"]["post"]["operationId"] =
            serde_json::json!("Move");
        behavior["operations"]["Move"] = behavior["operations"]["CreateWidget"].clone();
    });
    assert!(stderr.contains("Move becomes `move`"), "{stderr}");
}

#[test]
fn an_operation_missing_from_the_behavior_model_fails_generation() {
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/widgets.json"]["post"]["operationId"] =
            serde_json::json!("MintWidget");
    });
    assert!(
        stderr.contains("MintWidget is missing from behavior-model.json"),
        "{stderr}"
    );
}

#[test]
fn a_pagination_key_that_is_not_a_required_array_fails_generation() {
    let stderr = refusal(|openapi, _| {
        openapi["components"]["schemas"]["GetWidgetProgressResponseContent"]["properties"]["events"] =
            serde_json::json!({"type": "string"});
    });
    assert!(
        stderr.contains("GetWidgetProgress paginates over `events`, which is not an array"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"]["x-basecamp-pagination"]
            ["key"] = serde_json::json!("missing");
    });
    assert!(
        stderr.contains("GetWidgetProgress paginates over `missing`, which is not a member of GetWidgetProgressResponseContent"),
        "{stderr}"
    );
}

// The cursor-page mode: declared so the catalogue and behavior model describe
// the operation honestly, and generating no auto-pagination on purpose. A page
// carries its own opaque position, and the Link-following walk would flatten
// the pages and swallow every one of them -- leaving a crashed consumer with
// nothing to resume from. Before this, `style` was read only to reject
// anything that was not "link", so the mode the trait has always documented
// could not be spelled at all.
#[test]
fn the_cursor_style_generates_a_single_page_rather_than_a_flattening_walk() {
    let widgets = acceptance(
        |openapi, _| {
            openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"]["x-basecamp-pagination"]
                ["style"] = serde_json::json!("cursor");
        },
        "services/widgets.rs",
    );

    assert!(
        widgets.contains("Result<GetWidgetProgressResponseContent, Error>"),
        "a cursor-page operation returns one page, not a Page<>: {widgets}"
    );
    assert!(
        !widgets.contains("Page<GetWidgetProgressResponseContent>"),
        "a cursor-page operation must not be wired into the Link-following paginator: {widgets}"
    );
}

// The catalogue is the half that is easy to get wrong: collapsing cursor into
// "no pagination" makes the shipped route table say a paginated operation
// answers once, which is the opposite of true.
#[test]
fn the_cursor_style_reaches_the_route_catalogue_as_cursor_not_as_none() {
    let routes = acceptance(
        |openapi, _| {
            openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"]["x-basecamp-pagination"]
                ["style"] = serde_json::json!("cursor");
        },
        "routes.rs",
    );

    // Scoped to the one route the edit touched -- the fixture has another
    // Link-paginated operation and several unpaginated ones, so a whole-file
    // match would prove nothing.
    let route = routes
        .split("pub static ")
        .find(|block| block.starts_with("GET_WIDGET_PROGRESS:"))
        .expect("the edited route is in the catalogue");

    assert!(
        route.contains("Pagination::Cursor"),
        "the catalogue must name the cursor mode: {route}"
    );
    assert!(
        route.contains(r#"key: Some("events")"#),
        "the cursor entry must carry the page's items key: {route}"
    );
    assert!(
        !route.contains("Pagination::None"),
        "a cursor operation is paginated; the catalogue must not say it answers once: {route}"
    );
    assert!(
        !route.contains("Pagination::Link"),
        "a cursor operation must not be catalogued as a Link walk: {route}"
    );
}

#[test]
fn a_style_that_is_not_link_or_cursor_is_refused() {
    // Two cases that differ: "page" is a style the trait advertised for years
    // and nothing implemented, and a typo is what actually happens. Both must
    // fail loudly rather than read as "not paginated", which would ship a
    // method that silently never walks.
    for style in ["page", "linkk", "Link", ""] {
        let stderr = refusal(move |openapi, _| {
            openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"]["x-basecamp-pagination"]
                ["style"] = serde_json::json!(style);
        });
        assert!(
            stderr.contains("unsupported pagination style"),
            "style {style:?} must be refused by name: {stderr}"
        );
    }
}

// The style is checked before the behavior model, so an unsupported style is
// reported as what it is rather than sending the reader off to regenerate a
// behavior model that was never the problem.
#[test]
fn an_unsupported_style_is_named_even_when_the_behavior_model_is_silent() {
    let stderr = refusal(|openapi, behavior| {
        openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"]["x-basecamp-pagination"]
            ["style"] = serde_json::json!("page");
        behavior["operations"]["GetWidgetProgress"]["pagination"] = serde_json::Value::Null;
    });
    assert!(
        stderr.contains("unsupported pagination style"),
        "the style is the problem, not the behavior model: {stderr}"
    );
}

#[test]
fn shapes_the_generator_cannot_spell_are_refused() {
    let stderr = refusal(|openapi, _| {
        openapi["components"]["schemas"]["Widget"]["properties"]["width"]["type"] =
            serde_json::json!(["integer", "string"]);
    });
    assert!(
        stderr.contains("width: a union of integer and string has no Rust type"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        openapi["components"]["schemas"]["Widget"]["properties"]["parent"] = serde_json::json!({"oneOf": [{"$ref": "#/components/schemas/Owner"}, {"type": "string"}]});
    });
    assert!(
        stderr.contains("parent: oneOf has no Rust shape"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        openapi["components"]["schemas"]["Widget"]["properties"]["labels"] = serde_json::json!({
            "type": "array",
            "items": {"type": ["string", "null"]}
        });
    });
    assert!(
        stderr.contains("labels: nullable array items have no Rust shape"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/widgets.json"]["get"]["responses"]["201"] = serde_json::json!({"description": "also ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}});
    });
    assert!(
        stderr.contains("ListWidgets: its 2xx responses disagree on the body"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        let responses = &mut openapi["paths"]["/{accountId}/widgets.json"]["get"]["responses"];
        *responses = serde_json::json!({"404": {"description": "gone"}});
    });
    assert!(
        stderr.contains("\"ListWidgets\" has no 2xx response"),
        "{stderr}"
    );
}

#[test]
fn path_parameters_follow_the_template_and_must_all_be_bound() {
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/buckets/{bucketId}/widgets/{widgetId}"]["put"]["parameters"]
            .as_array_mut()
            .unwrap()
            .retain(|parameter| parameter["name"] != "widgetId");
    });
    assert!(
        stderr.contains("ReplaceWidget: /buckets/{bucketId}/widgets/{widgetId} names 2 placeholder(s) but 1 path parameter(s) are declared"),
        "{stderr}"
    );
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/buckets/{bucketId}/widgets/{widgetId}"]["put"]["parameters"]
            [2]["name"] = serde_json::json!("gadgetId");
    });
    assert!(
        stderr.contains("ReplaceWidget: path parameter gadgetId is not in /buckets/{bucketId}/widgets/{widgetId}"),
        "{stderr}"
    );

    // The declared order is not the argument order: the template's is.
    let root = std::env::temp_dir().join(format!(
        "basecamp-sdk-generator-order-{}",
        std::process::id()
    ));
    let _ = fs::remove_dir_all(&root);
    fs::create_dir_all(root.join("rust/generator")).unwrap();
    let mini = fixtures().join("mini");
    let mut openapi: serde_json::Value =
        serde_json::from_str(&fs::read_to_string(mini.join("openapi.json")).unwrap()).unwrap();
    let parameters = openapi["paths"]["/{accountId}/buckets/{bucketId}/widgets/{widgetId}"]["put"]
        ["parameters"]
        .as_array_mut()
        .unwrap();
    parameters.swap(1, 2);
    fs::write(
        root.join("openapi.json"),
        serde_json::to_string(&openapi).unwrap(),
    )
    .unwrap();
    fs::copy(
        mini.join("behavior-model.json"),
        root.join("behavior-model.json"),
    )
    .unwrap();
    fs::copy(
        mini.join("rust/generator/names.toml"),
        root.join("rust/generator/names.toml"),
    )
    .unwrap();
    let output = generate(&root, &root.join("out"));
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let rendered = fs::read_to_string(root.join("out/services/widgets.rs")).unwrap();
    assert!(
        rendered.contains("pub async fn replace_widget(\n        &self,\n        bucket_id: i64,\n        widget_id: i64,"),
        "{rendered}"
    );
    let _ = fs::remove_dir_all(&root);
}

#[test]
fn two_operations_sharing_an_envelope_must_agree_on_its_collection() {
    let stderr = refusal(|openapi, behavior| {
        let mut twin =
            openapi["paths"]["/{accountId}/widgets/{widgetId}/progress.json"]["get"].clone();
        twin["operationId"] = serde_json::json!("GetWidgetHistory");
        twin["x-basecamp-pagination"]["key"] = serde_json::json!("owner");
        openapi["components"]["schemas"]["GetWidgetProgressResponseContent"]["properties"]["owner"] =
            serde_json::json!({"type": "array", "items": {"$ref": "#/components/schemas/Owner"}});
        openapi["paths"]["/{accountId}/widgets/{widgetId}/history.json"] =
            serde_json::json!({"get": twin});
        behavior["operations"]["GetWidgetHistory"] =
            behavior["operations"]["GetWidgetProgress"].clone();
    });
    assert!(
        stderr.contains("paginates GetWidgetProgressResponseContent over `owner`, but another operation paginates it over `events`")
            || stderr.contains("paginates GetWidgetProgressResponseContent over `events`, but another operation paginates it over `owner`"),
        "{stderr}"
    );
}

#[test]
fn an_operation_without_a_tag_or_a_split_entry_fails_generation() {
    let stderr = refusal(|openapi, _| {
        openapi["paths"]["/{accountId}/widgets.json"]["get"]
            .as_object_mut()
            .unwrap()
            .remove("tags");
    });
    assert!(
        stderr.contains("ListWidgets has no tag and no [operation_services] entry in names.toml"),
        "{stderr}"
    );
}
