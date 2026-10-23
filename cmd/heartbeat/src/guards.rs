//! guards.rs — source guards of the crate: no behavioural literal outside config.rs and
//! protocol.rs, no stray print, accessor defaults equal to the catalog's. Runs with `cargo test`.

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use std::fs;
    use std::path::{Path, PathBuf};

    use regex::Regex;

    const EXEMPT: [&str; 2] = ["config.rs", "protocol.rs"];

    fn sources() -> Vec<PathBuf> {
        let mut files = Vec::new();
        walk(
            Path::new(env!("CARGO_MANIFEST_DIR")).join("src").as_path(),
            &mut files,
        );
        files.sort();
        files
    }

    fn walk(dir: &Path, files: &mut Vec<PathBuf>) {
        for entry in fs::read_dir(dir).expect("src dir") {
            let path = entry.expect("entry").path();
            if path.is_dir() {
                walk(&path, files);
            } else if path.extension().is_some_and(|ext| ext == "rs") {
                files.push(path);
            }
        }
    }

    /// The text before the first `#[cfg(test)]` line: test modules may hold literals.
    fn production_text(path: &Path) -> String {
        let text = fs::read_to_string(path).expect("source");
        text.split("#[cfg(test)]").next().unwrap_or("").to_owned()
    }

    #[test]
    fn no_behavioural_literal_outside_config_and_protocol() {
        let item = Regex::new(
            r"^\s*(pub(\(crate\))? )?(const|static) [A-Z_0-9]+\s*:[^=]*=\s*(\d|\[|&?\x22)",
        )
        .expect("regex");
        let number = Regex::new(r"[^A-Za-z_0-9.](\d[\d_]*)\b").expect("regex");
        let mut findings = Vec::new();
        for path in sources() {
            let name = path
                .file_name()
                .and_then(|name| name.to_str())
                .unwrap_or("");
            if EXEMPT.contains(&name) || name == "guards.rs" {
                continue;
            }
            for (index, line) in production_text(&path).lines().enumerate() {
                let trimmed = line.trim_start();
                if trimmed.starts_with("//") {
                    continue;
                }
                if item.is_match(line) {
                    findings.push(format!(
                        "{}:{}: const/static literal — a catalog row or protocol.rs",
                        path.display(),
                        index + 1
                    ));
                    continue;
                }
                for capture in number.captures_iter(line) {
                    let digits = capture[1].replace('_', "");
                    if digits != "0" && digits != "1" && !trimmed.contains("Timestamp::") {
                        findings.push(format!(
                            "{}:{}: numeric literal {} — a catalog row",
                            path.display(),
                            index + 1,
                            &capture[1]
                        ));
                    }
                }
            }
        }
        assert!(
            findings.is_empty(),
            "literals outside config.rs and protocol.rs:\n{}",
            findings.join("\n")
        );
    }

    #[test]
    fn no_stray_print() {
        let print = Regex::new(r"(^|[^e])print(ln)?!").expect("regex");
        let mut findings = Vec::new();
        for path in sources() {
            for (index, line) in production_text(&path).lines().enumerate() {
                if print.is_match(line) && !line.trim_start().starts_with("//") {
                    findings.push(format!("{}:{}", path.display(), index + 1));
                }
            }
        }
        assert!(
            findings.is_empty(),
            "stray print!/println!:\n{}",
            findings.join("\n")
        );
    }

    /// Every accessor default equals the catalog default: `tests/fixtures/config-defaults.json` is `make settings-defaults`.
    #[test]
    fn defaults_match_the_catalog_fixture() {
        let fixture =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/config-defaults.json");
        let expected: serde_json::Value = serde_json::from_slice(
            &fs::read(&fixture).expect("fixture — run make settings-defaults"),
        )
        .expect("fixture json");
        let empty = crate::config::parse(b"{}").expect("empty config parses");
        let rendered = crate::config::render_defaults(&empty);
        assert_eq!(
            rendered, expected,
            "accessor defaults drifted from the catalog — change one, change both"
        );
    }
}
