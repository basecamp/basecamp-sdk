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
