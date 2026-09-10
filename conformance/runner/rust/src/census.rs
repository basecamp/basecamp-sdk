//! The case census (#742): an independent count of the non-live cases under
//! `conformance/tests`, taken by its own recursive walk so a fixture the runner's glob
//! cannot see is reported before the run rather than inferred from a short count afterwards.

use std::path::Path;

use serde::Deserialize;

#[derive(Deserialize)]
struct ModeOnly {
    mode: Option<String>,
}

pub fn count_non_live_cases(dir: &Path) -> Result<usize, String> {
    let mut cases = 0;
    let mut files = 0;
    walk(dir, &mut |path| {
        files += 1;
        let raw = std::fs::read(path).map_err(|error| format!("{}: {error}", path.display()))?;
        let parsed: Vec<ModeOnly> =
            serde_json::from_slice(&raw).map_err(|error| format!("{}: {error}", path.display()))?;
        if parsed.is_empty() {
            return Err(format!(
                "{}: fixture declares no cases; delete the file or restore its cases",
                path.display()
            ));
        }
        cases += parsed
            .iter()
            .filter(|case| case.mode.as_deref() != Some("live"))
            .count();
        Ok(())
    })?;
    if files == 0 {
        return Err(format!(
            "no *.json fixture files found under {}",
            dir.display()
        ));
    }
    Ok(cases)
}

fn walk(dir: &Path, visit: &mut dyn FnMut(&Path) -> Result<(), String>) -> Result<(), String> {
    let mut entries: Vec<_> = std::fs::read_dir(dir)
        .map_err(|error| format!("{}: {error}", dir.display()))?
        .collect::<Result<_, _>>()
        .map_err(|error| format!("{}: {error}", dir.display()))?;
    entries.sort_by_key(std::fs::DirEntry::path);
    for entry in entries {
        let path = entry.path();
        if path.is_dir() {
            walk(&path, visit)?;
        } else if path
            .extension()
            .is_some_and(|extension| extension == "json")
        {
            visit(&path)?;
        }
    }
    Ok(())
}

/// The failure message when the run's own accounting disagrees with the census, or
/// `None` when they agree.
pub fn case_count_failure(ran: usize, expected: usize) -> Option<String> {
    match ran.cmp(&expected) {
        std::cmp::Ordering::Equal => None,
        std::cmp::Ordering::Less => Some(format!(
            "case census: the run accounted for {ran} case(s) (passed+failed+skipped) but \
             conformance/tests holds {expected} non-live case(s) — {} executed by nothing. An \
             unrecognized `mode`, a fixture that failed to parse or was never globbed, or a \
             case dropped between load and dispatch will do this.",
            expected - ran
        )),
        std::cmp::Ordering::Greater => Some(format!(
            "case census: the run accounted for {ran} case(s) (passed+failed+skipped) but \
             conformance/tests holds only {expected} non-live case(s) — {} more than the \
             fixtures declare.",
            ran - expected
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tree(files: &[(&str, &str)]) -> tempdir::Dir {
        let dir = tempdir::Dir::new();
        for (name, body) in files {
            let path = dir.path().join(name);
            std::fs::create_dir_all(path.parent().unwrap()).unwrap();
            std::fs::write(path, body).unwrap();
        }
        dir
    }

    #[test]
    fn counts_non_live_cases_recursively() {
        let dir = tree(&[
            ("a.json", r#"[{"mode":"mock"},{},{"mode":"live"}]"#),
            ("nested/b.json", r#"[{"name":"x"}]"#),
        ]);
        assert_eq!(count_non_live_cases(dir.path()), Ok(3));
    }

    #[test]
    fn an_empty_fixture_is_an_error_not_zero() {
        let dir = tree(&[("a.json", "[]")]);
        let error = count_non_live_cases(dir.path()).unwrap_err();
        assert!(error.contains("declares no cases"), "{error}");
    }

    #[test]
    fn a_non_array_fixture_is_an_error() {
        let dir = tree(&[("a.json", r#"{"tests": []}"#)]);
        assert!(count_non_live_cases(dir.path()).is_err());
    }

    #[test]
    fn no_fixtures_at_all_is_an_error() {
        let dir = tree(&[("readme.txt", "not json")]);
        let error = count_non_live_cases(dir.path()).unwrap_err();
        assert!(error.contains("no *.json fixture files"), "{error}");
    }

    #[test]
    fn count_failure_names_the_direction() {
        assert_eq!(case_count_failure(3, 3), None);
        assert!(
            case_count_failure(2, 3)
                .unwrap()
                .contains("1 executed by nothing")
        );
        assert!(
            case_count_failure(4, 3)
                .unwrap()
                .contains("1 more than the fixtures declare")
        );
    }

    /// A throwaway directory removed on drop; the std library has no tempdir of its own.
    mod tempdir {
        use std::path::{Path, PathBuf};
        use std::sync::atomic::{AtomicU64, Ordering};

        static COUNTER: AtomicU64 = AtomicU64::new(0);

        pub struct Dir(PathBuf);

        impl Dir {
            pub fn new() -> Dir {
                let unique = COUNTER.fetch_add(1, Ordering::Relaxed);
                let path = std::env::temp_dir().join(format!(
                    "basecamp-conformance-{}-{unique}",
                    std::process::id()
                ));
                std::fs::create_dir_all(&path).unwrap();
                Dir(path)
            }

            pub fn path(&self) -> &Path {
                &self.0
            }
        }

        impl Drop for Dir {
            fn drop(&mut self) {
                let _ = std::fs::remove_dir_all(&self.0);
            }
        }
    }
}
