//! Generates the Rust Basecamp SDK's types, routes, metadata, services and accessors.
//!
//! Reads `openapi.json` and `behavior-model.json` from the repository root (`--root`, or
//! `--openapi`/`--behavior`/`--names` for each input) and writes
//! `rust/basecamp-sdk/src/generated/`. With `--check` it writes nothing and exits non-zero
//! when the checked-in files differ from what it would generate; with `--output <dir>` it
//! writes somewhere other than the checked-in tree, which is how the drift script diffs.

// The emitters write into `String`s, whose `write!` cannot fail.
#![allow(clippy::unwrap_used, clippy::expect_used)]

mod emit;
mod model;
mod naming;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};
use std::{env, fs};

use model::Model;
use naming::Naming;

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("error: {error}");
            ExitCode::FAILURE
        }
    }
}

fn run() -> Result<(), String> {
    let mut check = false;
    let mut root = default_root();
    let mut output = None;
    let mut openapi_path = None;
    let mut behavior_path = None;
    let mut names_path = None;
    let mut arguments = env::args().skip(1);
    while let Some(argument) = arguments.next() {
        let mut path = |flag: &str| -> Result<PathBuf, String> {
            arguments
                .next()
                .map(PathBuf::from)
                .ok_or_else(|| format!("{flag} needs a path"))
        };
        match argument.as_str() {
            "--check" => check = true,
            "--root" => root = path("--root")?,
            "--output" => output = Some(path("--output")?),
            "--openapi" => openapi_path = Some(path("--openapi")?),
            "--behavior" => behavior_path = Some(path("--behavior")?),
            "--names" => names_path = Some(path("--names")?),
            other => return Err(format!("unknown argument {other}")),
        }
    }

    let openapi = read_json(&openapi_path.unwrap_or_else(|| root.join("openapi.json")))?;
    let behavior = read_json(&behavior_path.unwrap_or_else(|| root.join("behavior-model.json")))?;
    let names = read(&names_path.unwrap_or_else(|| root.join("rust/generator/names.toml")))?;
    let naming = Naming::parse(&names)?;
    let model = Model::build(&openapi, &behavior, &naming)?;
    let files = render(&model)?;

    let target = output.unwrap_or_else(|| root.join("rust/basecamp-sdk/src/generated"));
    if check {
        verify(&target, &files)
    } else {
        write(&target, &files)?;
        println!(
            "{} operations, {} schemas, {} services",
            model.operations().count(),
            model.schemas.len(),
            model.services.len()
        );
        Ok(())
    }
}

fn render(model: &Model) -> Result<BTreeMap<PathBuf, String>, String> {
    let mut files = BTreeMap::new();
    files.insert(PathBuf::from("mod.rs"), render_mod(model));
    files.insert(PathBuf::from("types.rs"), emit::types::render(model));
    files.insert(PathBuf::from("routes.rs"), emit::routes::render(model));
    files.insert(PathBuf::from("metadata.rs"), emit::metadata::render(model));
    files.insert(
        PathBuf::from("accessors.rs"),
        emit::accessors::render(model),
    );
    files.insert(
        PathBuf::from("services/mod.rs"),
        emit::services::render_mod(model),
    );
    for service in &model.services {
        files.insert(
            PathBuf::from(format!("services/{}.rs", service.module)),
            emit::services::render_service(service, model)?,
        );
    }
    Ok(files)
}

fn render_mod(model: &Model) -> String {
    format!(
        "{}//! Generated from `openapi.json` and `behavior-model.json`. Never edited by hand.\n\npub mod accessors;\npub mod metadata;\npub mod routes;\npub mod services;\npub mod types;\n\n/// The Basecamp API version this SDK was generated against.\npub const API_VERSION: &str = \"{}\";\n\n/// How many operations the model declares.\npub const OPERATION_COUNT: usize = {};\n",
        emit::HEADER,
        model.api_version,
        model.operations().count()
    )
}

fn write(target: &Path, files: &BTreeMap<PathBuf, String>) -> Result<(), String> {
    if target.exists() {
        fs::remove_dir_all(target).map_err(|error| format!("{}: {error}", target.display()))?;
    }
    let paths = write_all(target, files)?;
    format(&paths)?;
    println!("Generated {} files in {}", paths.len(), target.display());
    Ok(())
}

fn verify(target: &Path, files: &BTreeMap<PathBuf, String>) -> Result<(), String> {
    let scratch = env::temp_dir().join(format!("basecamp-sdk-generator-{}", std::process::id()));
    let paths = write_all(&scratch, files)?;
    let formatted = format(&paths);
    let mut stale = Vec::new();
    for relative in files.keys() {
        let expected = fs::read_to_string(scratch.join(relative)).unwrap_or_default();
        let actual = fs::read_to_string(target.join(relative)).unwrap_or_default();
        if expected != actual {
            stale.push(relative.display().to_string());
        }
    }
    for existing in list_files(target, target) {
        if !files.contains_key(&existing) {
            stale.push(format!("{} (unexpected)", existing.display()));
        }
    }
    let _ = fs::remove_dir_all(&scratch);
    formatted?;
    if stale.is_empty() {
        println!("{} is up to date", target.display());
        Ok(())
    } else {
        Err(format!(
            "{} is out of date. Run `make rs-generate-services`. Stale files:\n  {}",
            target.display(),
            stale.join("\n  ")
        ))
    }
}

fn write_all(target: &Path, files: &BTreeMap<PathBuf, String>) -> Result<Vec<PathBuf>, String> {
    let mut paths = Vec::new();
    for (relative, content) in files {
        let path = target.join(relative);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|error| format!("{}: {error}", parent.display()))?;
        }
        fs::write(&path, content).map_err(|error| format!("{}: {error}", path.display()))?;
        paths.push(path);
    }
    Ok(paths)
}

/// Formats with the workspace's own `rustfmt.toml`, so generated code and hand-written code
/// share one configuration and the drift check sees no formatting differences.
fn format(paths: &[PathBuf]) -> Result<(), String> {
    let config = Path::new(env!("CARGO_MANIFEST_DIR")).join("../rustfmt.toml");
    let status = Command::new("rustfmt")
        .arg("--edition")
        .arg("2024")
        .arg("--config-path")
        .arg(config)
        .args(paths)
        .status()
        .map_err(|error| format!("rustfmt: {error}"))?;
    if status.success() {
        Ok(())
    } else {
        Err("rustfmt failed".into())
    }
}

fn list_files(root: &Path, directory: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    if let Ok(entries) = fs::read_dir(directory) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                files.extend(list_files(root, &path));
            } else if let Ok(relative) = path.strip_prefix(root) {
                files.push(relative.to_path_buf());
            }
        }
    }
    files
}

fn default_root() -> PathBuf {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR")).join("../..");
    manifest.canonicalize().unwrap_or(manifest)
}

fn read(path: &Path) -> Result<String, String> {
    fs::read_to_string(path).map_err(|error| format!("{}: {error}", path.display()))
}

fn read_json(path: &Path) -> Result<serde_json::Value, String> {
    serde_json::from_str(&read(path)?).map_err(|error| format!("{}: {error}", path.display()))
}
