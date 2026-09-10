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

/// A copy of the mini model with one edit applied to `openapi.json`, generated into a
/// scratch directory; answers the generator's stderr on failure.
fn refusal(edit: impl FnOnce(&mut serde_json::Value, &mut serde_json::Value)) -> String {
    let root = std::env::temp_dir().join(format!(
        "basecamp-sdk-generator-refusal-{}-{:?}-{}",
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
    let output = generate(&root, &root.join("out"));
    let _ = fs::remove_dir_all(&root);
    assert!(
        !output.status.success(),
        "the generator should have refused"
    );
    String::from_utf8_lossy(&output.stderr).into_owned()
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
