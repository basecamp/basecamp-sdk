//! The conformance runner for the Rust Basecamp SDK.
//!
//! It reads the shared case definitions from `conformance/tests/`, runs each mock-mode case
//! against the real `basecamp_sdk::Client` through a scripted transport, takes the case
//! census (#742) and writes the execution manifest (#743). Live cases are the TypeScript
//! runner's and are filtered out at load.

mod assertions;
mod census;
mod fixtures;
mod header_tokens;
mod manifest;
mod operations;
mod transport;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::ExitCode;

use fixtures::TestCase;
use manifest::{Exclusion, Manifest};

/// Cases the Rust SDK deliberately does not execute. Empty: the scripted transport lets the
/// two origin-normalization cases Go cannot dial run here, and the `link-header` fixture
/// runs with only its `requestCount` assertion suppressed (assertions.rs). Every entry
/// added here must also be rostered in `spec/zero-skip-roster.yml`, or
/// `make check-fixture-execution` fails.
const RUST_SKIPS: &[(&str, &str)] = &[];

#[tokio::main]
async fn main() -> ExitCode {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let tests_dir = repo_root.join("conformance/tests");

    // Case census (#602) — taken up front, by its own recursive walk, so a fixture this
    // runner's glob cannot see is reported before the run rather than inferred from a short
    // count afterwards.
    let expected_cases = match census::count_non_live_cases(&tests_dir) {
        Ok(count) => count,
        Err(error) => {
            eprintln!("Error taking fixture census: {error}");
            return ExitCode::FAILURE;
        }
    };

    // No early exit on an empty glob: the census walks recursively and this glob does not,
    // so "the census found fixtures but this runner globbed none" is exactly the
    // under-count the census exists to reject, and it must reach the comparison below.
    let files = match case_files(&tests_dir) {
        Ok(files) => files,
        Err(error) => {
            eprintln!("Error finding test files: {error}");
            return ExitCode::FAILURE;
        }
    };
    if files.is_empty() {
        println!("No test files found in {}", tests_dir.display());
    }

    let skips: BTreeMap<&str, &str> = RUST_SKIPS.iter().copied().collect();
    let (mut passed, mut failed, mut skipped) = (0usize, 0usize, 0usize);
    // Exclusions recorded from the same branch that increments `skipped`, so the manifest
    // cannot claim a different set than the run.
    let mut excluded = Vec::new();

    for file in &files {
        let file_name = file
            .file_name()
            .unwrap_or_default()
            .to_string_lossy()
            .to_string();
        let cases = match load_cases(file) {
            Ok(cases) => cases,
            Err(error) => {
                eprintln!("Error loading {}: {error}", file.display());
                continue;
            }
        };
        println!("\n=== {file_name} ===");
        for case in &cases {
            if let Some(reason) = skips.get(case.name.as_str()) {
                skipped += 1;
                excluded.push(Exclusion {
                    file: file_name.clone(),
                    name: case.name.clone(),
                    reason: (*reason).to_string(),
                });
                println!("  SKIP: {} ({reason})", case.name);
                continue;
            }
            match run_case(case).await {
                Ok(()) => {
                    passed += 1;
                    println!("  PASS: {}", case.name);
                }
                Err(message) => {
                    failed += 1;
                    let sanitized = message.replace(['\n', '\r'], " ");
                    println!("  FAIL: {}\n        {sanitized}", case.name);
                }
            }
        }
    }

    let ran = passed + failed + skipped;
    println!("\n=== Summary ===");
    println!(
        "Passed: {passed}, Failed: {failed}, Skipped: {skipped}, Total: {ran} (fixtures declare {expected_cases} non-live case(s))"
    );

    let count_failure = census::case_count_failure(ran, expected_cases);
    if let Some(message) = &count_failure {
        eprintln!("\nFAIL: {message}");
    }

    // Written even when the run failed: a failing runner still has a truthful exclusion
    // set, and the collecting gate reads a missing manifest as "this runner did not
    // report" — a second, unrelated failure that would obscure the first.
    let manifest_failure = manifest::write_manifest(
        &repo_root,
        Manifest {
            runner: "rust",
            total: expected_cases,
            executed: passed + failed,
            excluded,
        },
    )
    .err();
    if let Some(message) = &manifest_failure {
        eprintln!("\nFAIL: could not write execution manifest: {message}");
    }

    if failed > 0 || count_failure.is_some() || manifest_failure.is_some() {
        ExitCode::FAILURE
    } else {
        ExitCode::SUCCESS
    }
}

fn case_files(directory: &Path) -> Result<Vec<PathBuf>, std::io::Error> {
    let mut files: Vec<PathBuf> = std::fs::read_dir(directory)?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_file() && path.extension().is_some_and(|ext| ext == "json"))
        .collect();
    files.sort();
    Ok(files)
}

fn load_cases(file: &Path) -> Result<Vec<TestCase>, String> {
    let contents = std::fs::read(file).map_err(|error| error.to_string())?;
    let cases: Vec<TestCase> =
        serde_json::from_slice(&contents).map_err(|error| error.to_string())?;
    Ok(cases.into_iter().filter(TestCase::is_mock_mode).collect())
}

async fn run_case(case: &TestCase) -> Result<(), String> {
    // Defense-in-depth backstop for the operationally harmful mockResponses shapes; the
    // authoritative oneOf enforcement is `make conformance-fixtures-check`.
    for (i, mock) in case.mock_responses.iter().enumerate() {
        if (mock.status != 0) == mock.network_error {
            return Err(format!(
                "mockResponses[{i}] must set exactly one of status or networkError (got status={}, networkError={})",
                mock.status, mock.network_error
            ));
        }
    }
    let transport =
        transport::ScriptedTransport::new(case.mock_responses.clone(), case.auto_paginates());
    let outcome = operations::execute_case(case, transport.clone()).await;
    let recorded = transport.recorded();
    assertions::check_all(&assertions::Run {
        case,
        outcome: &outcome,
        recorded: &recorded,
    })
}
