//! The execution manifest (#743): the cases this runner did not execute, written to
//! `conformance/manifests/rust.json` for `scripts/check-fixture-execution.rb` to compare
//! against every other runner's.

use std::path::Path;

use serde::Serialize;

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Serialize)]
pub struct Exclusion {
    pub file: String,
    pub name: String,
    pub reason: String,
}

#[derive(Debug, Serialize)]
pub struct Manifest {
    pub runner: &'static str,
    #[serde(rename = "total_non_live")]
    pub total: usize,
    pub executed: usize,
    pub excluded: Vec<Exclusion>,
}

pub const MANIFEST_REL_DIR: &str = "conformance/manifests";

pub fn write_manifest(repo_root: &Path, mut manifest: Manifest) -> Result<(), String> {
    if manifest.executed + manifest.excluded.len() != manifest.total {
        return Err(format!(
            "manifest for {} is internally inconsistent: {} executed + {} excluded != {} \
             non-live cases; the run dropped a case without recording it as either",
            manifest.runner,
            manifest.executed,
            manifest.excluded.len(),
            manifest.total
        ));
    }
    manifest.excluded.sort();
    let dir = repo_root.join(MANIFEST_REL_DIR);
    std::fs::create_dir_all(&dir).map_err(|error| format!("{}: {error}", dir.display()))?;
    let path = dir.join(format!("{}.json", manifest.runner));
    let mut body = serde_json::to_vec_pretty(&manifest).map_err(|error| error.to_string())?;
    body.push(b'\n');
    std::fs::write(&path, body).map_err(|error| format!("{}: {error}", path.display()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn scratch() -> std::path::PathBuf {
        let path = std::env::temp_dir().join(format!(
            "basecamp-conformance-manifest-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        std::fs::create_dir_all(&path).unwrap();
        path
    }

    #[test]
    fn an_inconsistent_manifest_is_refused_before_it_is_written() {
        let root = scratch();
        let error = write_manifest(
            &root,
            Manifest {
                runner: "rust",
                total: 3,
                executed: 1,
                excluded: vec![],
            },
        )
        .unwrap_err();
        assert!(error.contains("internally inconsistent"), "{error}");
        assert!(!root.join(MANIFEST_REL_DIR).join("rust.json").exists());
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn exclusions_are_written_sorted_by_file_then_name() {
        let root = scratch();
        write_manifest(
            &root,
            Manifest {
                runner: "rust",
                total: 3,
                executed: 1,
                excluded: vec![
                    Exclusion {
                        file: "b.json".into(),
                        name: "z".into(),
                        reason: "r".into(),
                    },
                    Exclusion {
                        file: "a.json".into(),
                        name: "y".into(),
                        reason: "r".into(),
                    },
                ],
            },
        )
        .unwrap();
        let written =
            std::fs::read_to_string(root.join(MANIFEST_REL_DIR).join("rust.json")).unwrap();
        let parsed: serde_json::Value = serde_json::from_str(&written).unwrap();
        assert_eq!(parsed["runner"], "rust");
        assert_eq!(parsed["total_non_live"], 3);
        assert_eq!(parsed["excluded"][0]["file"], "a.json");
        assert_eq!(parsed["excluded"][1]["file"], "b.json");
        assert!(written.ends_with('\n'));
        let _ = std::fs::remove_dir_all(root);
    }
}
