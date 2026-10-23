//! Plan parsing for collision detection (concept D12): the planned-files table and the diff blocks under
//! `## Changes`, tolerant of the variants real plans carry. Pure.
//!
//! * Planned files: the first table under `## Changes` whose header starts `File | Kind`; from each File cell every
//!   inline-code span, else every bare path-like token. No table ⇒ the paths of every `location:` line. Both go through
//!   the same token rules ([`paths_of`]): one brace group expands (`a/{B.md,c.json}`), a bare file name resolves against
//!   the preceding path's directory, a trailing `/` is the directory glob `<dir>/**`, a path with `*` is a glob, and a
//!   token with neither `/` nor `.` (`RunArm`, `-out`, `:121-150`) is no path.
//! * Symbols: every signature in a diff block — an `@@ <text>` header or any diff line — attached to the one file it can
//!   belong to: an `@@ <path>` header's file, else the entry's single `location:` file of the signature's language; an
//!   entry with several such files contributes no symbol (a missed symbol degrades to planned-file, an invented one
//!   is a false alarm). Any `###`-or-deeper heading starts a new entry.
//! * Changed lines: every `+`/`-` line of a diff block, per `location:` file, for the version-only check.

use std::collections::{BTreeMap, BTreeSet};
use std::sync::LazyLock;

use regex::Regex;

use crate::protocol::{FENCE_MIN_WIDTH, PLAN_SIGNATURES};

/// What one work item's counted plan files intend to change.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct PlannedWork {
    /// Planned-files entries.
    pub files: BTreeSet<PlannedPath>,
    /// `(file, symbol)` pairs from the diff blocks.
    pub symbols: BTreeSet<(String, String)>,
    /// Changed lines, diff marker stripped, per file.
    pub changed: BTreeMap<String, Vec<String>>,
}

/// One planned-files entry.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct PlannedPath {
    /// Repo-relative path, or a glob over them.
    pub path: String,
    /// `path` holds a `*`.
    pub glob: bool,
}

impl PlannedWork {
    /// Folds another plan file's work in.
    pub fn merge(&mut self, other: PlannedWork) {
        self.files.extend(other.files);
        self.symbols.extend(other.symbols);
        for (file, lines) in other.changed {
            self.changed.entry(file).or_default().extend(lines);
        }
    }

    /// A diff block of an entry whose `location:` names `files`; an `@@ <path>:<line>` header narrows the lines after
    /// it to that one file (the multi-file hunk form real plans use).
    fn add_diff(&mut self, files: &[String], body: &[&str]) {
        let mut targets: Vec<String> = files.to_vec();
        for line in body {
            if let Some(path) = header_path(line) {
                targets = vec![path];
                continue;
            }
            let files = targets.as_slice();
            if let Some(changed) = changed_line(line) {
                for file in files {
                    self.changed
                        .entry(file.clone())
                        .or_default()
                        .push(changed.to_owned());
                }
            }
            if let Some((lang, name)) = signature_in(line) {
                let owners: Vec<&String> = files.iter().filter(|file| lang.matches(file)).collect();
                if let [owner] = owners.as_slice() {
                    self.symbols.insert(((*owner).clone(), name));
                }
            }
        }
    }
}

/// The plan dir of a repo-relative `plans/<feature>/<dir>/design/<file>.md` path when `<dir>` is a wip dir, as
/// `<feature>/<dir>`.
#[must_use]
pub fn plan_dir_of(rel: &str) -> Option<String> {
    let parts: Vec<&str> = rel.split('/').collect();
    match parts.as_slice() {
        ["plans", feature, dir, "design", file] if dir.contains("-wip-") && is_markdown(file) => {
            Some(format!("{feature}/{dir}"))
        }
        _ => None,
    }
}

/// `name` is a markdown file (`.md`, any case).
#[must_use]
pub fn is_markdown(name: &str) -> bool {
    std::path::Path::new(name)
        .extension()
        .is_some_and(|extension| extension.eq_ignore_ascii_case("md"))
}

/// Parses one plan file: the planned work under `## Changes`.
#[must_use]
pub fn parse(markdown: &str) -> PlannedWork {
    scan(&changes_lines(markdown))
}

/// The lines under `## Changes`; headings inside fenced blocks are content.
fn changes_lines(markdown: &str) -> Vec<&str> {
    let mut lines = Vec::new();
    let mut in_changes = false;
    let mut fence: Option<Fence> = None;
    for line in markdown.lines() {
        if let Some(open) = &fence {
            if open.closes(line) {
                fence = None;
            }
        } else if let Some(title) = line.strip_prefix("## ") {
            in_changes = title.trim().starts_with("Changes");
            continue;
        } else {
            fence = Fence::open(line);
        }
        if in_changes {
            lines.push(line);
        }
    }
    lines
}

/// The planned work of one unit's lines: the first `File | Kind` table (else every `location:` path), plus the diff
/// blocks' symbols and changed lines.
fn scan(lines: &[&str]) -> PlannedWork {
    let mut work = PlannedWork::default();
    let mut table = Table::Before;
    let mut locations: Vec<String> = Vec::new();
    let mut all_locations: BTreeSet<String> = BTreeSet::new();
    let mut lines = lines.iter().copied();
    while let Some(line) = lines.next() {
        if let Some(fence) = Fence::open(line) {
            let body: Vec<&str> = lines
                .by_ref()
                .take_while(|line| !fence.closes(line))
                .collect();
            if fence.info == "diff" {
                work.add_diff(&locations, &body);
            }
            continue;
        }
        if line.starts_with("###") {
            locations.clear();
            continue;
        }
        if let Some(rest) = line.trim_start().strip_prefix("location:") {
            locations = location_paths(rest);
            all_locations.extend(locations.iter().cloned());
            continue;
        }
        table = table.feed(line, &mut work.files);
    }
    if table == Table::Before {
        work.files
            .extend(all_locations.into_iter().map(planned_path));
    }
    work
}

/// Where the planned-files table scan stands.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Table {
    /// No `File | Kind` header seen yet.
    Before,
    /// Reading rows.
    Inside,
    /// The first table ended; later tables are ignored.
    Done,
}

impl Table {
    fn feed(self, line: &str, files: &mut BTreeSet<PlannedPath>) -> Table {
        let cells = row_cells(line);
        match self {
            Table::Before => match cells.as_deref() {
                Some([file, kind, ..]) if *file == "File" && *kind == "Kind" => Table::Inside,
                Some(_) | None => Table::Before,
            },
            Table::Inside => match cells {
                None => Table::Done,
                Some(cells) => {
                    if !is_separator(&cells)
                        && let Some(cell) = cells.first()
                    {
                        files.extend(file_cell(cell));
                    }
                    Table::Inside
                }
            },
            Table::Done => Table::Done,
        }
    }
}

/// A fenced block opener: ` ``` ` or `~~~`, three or more, with its info string.
struct Fence {
    marker: char,
    width: usize,
    info: String,
}

impl Fence {
    fn open(line: &str) -> Option<Fence> {
        let text = line.trim_start();
        let marker = text.chars().next().filter(|c| *c == '`' || *c == '~')?;
        let width = text.chars().take_while(|c| *c == marker).count();
        (width >= FENCE_MIN_WIDTH).then(|| Fence {
            marker,
            width,
            info: text[width..].trim().to_owned(),
        })
    }

    fn closes(&self, line: &str) -> bool {
        let text = line.trim();
        text.len() >= self.width && text.chars().all(|c| c == self.marker)
    }
}

/// The trimmed cells of a `| a | b |` row; `None` for any other line.
fn row_cells(line: &str) -> Option<Vec<&str>> {
    let inner = line.trim().strip_prefix('|')?.strip_suffix('|')?;
    Some(inner.split('|').map(str::trim).collect())
}

fn is_separator(cells: &[&str]) -> bool {
    cells
        .iter()
        .all(|cell| !cell.is_empty() && cell.chars().all(|c| matches!(c, '-' | ':' | ' ')))
}

/// The planned paths of one File cell: every inline-code span, else every bare token; prose around them (`10 ×`, `→`,
/// `(15 skills)`, `run by …`) is ignored. See [`paths_of`].
fn file_cell(cell: &str) -> Vec<PlannedPath> {
    let spans = code_spans(cell);
    if spans.is_empty() {
        let tokens = cell
            .split(|c: char| c.is_whitespace() || c == ',')
            .map(|token| token.trim_matches(|c| matches!(c, '(' | ')' | ':' | ';')));
        return paths_of(tokens);
    }
    paths_of(spans.into_iter())
}

/// The distinct files of a `location:` line, in order: its code spans with the `:line` suffix cut, through
/// [`paths_of`], so a bare name resolves against the previous path's directory and a span that is no path (`RunArm`,
/// `-out`, `:121-150`) drops out.
fn location_paths(rest: &str) -> Vec<String> {
    let spans: Vec<String> = code_spans(rest)
        .into_iter()
        .map(strip_line_suffix)
        .collect();
    let mut paths: Vec<String> = Vec::new();
    for planned in paths_of(spans.iter().map(String::as_str)) {
        if !paths.contains(&planned.path) {
            paths.push(planned.path);
        }
    }
    paths
}

/// Planned paths from candidate tokens: one brace group expands (`a/{B.md,c.json}` → `a/B.md`, `a/c.json`), a bare file
/// name resolves against the preceding path's directory (`a/SKILL.md` + `changelog.json` → `a/changelog.json`), a
/// trailing `/` makes the directory glob `<dir>/**`, and a token with neither `/` nor `.` is no path.
fn paths_of<'a>(candidates: impl Iterator<Item = &'a str>) -> Vec<PlannedPath> {
    let mut directory: Option<String> = None;
    let mut paths = Vec::new();
    for candidate in candidates.flat_map(expand_braces) {
        let is_path_like = candidate.contains('/') || candidate.contains('.');
        if candidate.is_empty() || candidate.contains(char::is_whitespace) || !is_path_like {
            continue;
        }
        let path = match (&directory, candidate.contains('/')) {
            (Some(dir), false) => format!("{dir}/{candidate}"),
            (Some(_) | None, _) => candidate,
        };
        if let Some((dir, _)) = path.trim_end_matches('/').rsplit_once('/') {
            directory = Some(dir.to_owned());
        }
        paths.push(planned_path(path));
    }
    paths
}

/// `dir/` → the glob `dir/**`; anything else literal unless it holds `*`.
fn planned_path(path: String) -> PlannedPath {
    match path.strip_suffix('/') {
        Some(dir) => PlannedPath {
            path: format!("{dir}/**"),
            glob: true,
        },
        None => PlannedPath {
            glob: path.contains('*'),
            path,
        },
    }
}

/// `a/{B.md,c.json}` → `a/B.md`, `a/c.json`: the first closed brace group, no nesting; anything else unchanged.
fn expand_braces(token: &str) -> Vec<String> {
    let group = token
        .find('{')
        .and_then(|open| Some((open, open + token[open..].find('}')?)));
    let Some((open, close)) = group else {
        return vec![token.to_owned()];
    };
    let (prefix, suffix) = (&token[..open], &token[close + 1..]);
    token[open + 1..close]
        .split(',')
        .map(|part| format!("{prefix}{}{suffix}", part.trim()))
        .collect()
}

/// The contents of every `` `…` `` span: the odd pieces between backticks.
fn code_spans(text: &str) -> Vec<&str> {
    text.split('`')
        .enumerate()
        .filter(|(index, _)| index & 1 == 1)
        .map(|(_, piece)| piece.trim())
        .filter(|span| !span.is_empty())
        .collect()
}

/// `path:120` or `path:12-40` → `path`.
fn strip_line_suffix(span: &str) -> String {
    span.rsplit_once(':')
        .filter(|(_, lines)| {
            !lines.is_empty() && lines.chars().all(|c| c.is_ascii_digit() || c == '-')
        })
        .map_or(span, |(path, _)| path)
        .to_owned()
}

/// The file of an `@@ <path>` / `@@ <path>:<line>` header.
fn header_path(line: &str) -> Option<String> {
    let path = strip_line_suffix(line.strip_prefix("@@")?.trim());
    (path.contains('/') && !path.contains(char::is_whitespace)).then_some(path)
}

fn changed_line(line: &str) -> Option<&str> {
    if line.starts_with("+++") || line.starts_with("---") {
        return None;
    }
    line.strip_prefix('+').or_else(|| line.strip_prefix('-'))
}

/// The signature a diff line carries: an `@@` header's text, else the line without its diff marker.
fn signature_in(line: &str) -> Option<(Lang, String)> {
    if let Some(header) = line.strip_prefix("@@") {
        let text = header
            .rsplit_once("@@")
            .map_or(header, |(_, after)| after)
            .trim();
        return header_signature(text);
    }
    signature(line.strip_prefix([' ', '+', '-']).unwrap_or(line))
}

/// An `@@` header: a signature, else its leading identifier (`@@ LoadGrants`, `@@ run_cell()`); `None` for empty
/// and `path:line` headers.
fn header_signature(text: &str) -> Option<(Lang, String)> {
    if text.is_empty() || text.contains('/') || strip_line_suffix(text) != text {
        return None;
    }
    signature(text).or_else(|| {
        let name: String = text
            .chars()
            .take_while(|c| c.is_ascii_alphanumeric() || matches!(c, '_' | '.' | ':'))
            .collect();
        let starts_ok = name
            .chars()
            .next()
            .is_some_and(|c| c.is_ascii_alphabetic() || c == '_');
        starts_ok.then_some((Lang::Any, name))
    })
}

/// Languages a signature pattern belongs to, matched against a location file's extension.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Lang {
    /// `.rs`.
    Rust,
    /// `.go`.
    Go,
    /// `.py`.
    Python,
    /// `.sh`, `.bash`.
    Shell,
    /// `.js`, `.mjs`, `.ts`, `.tsx`.
    Js,
    /// Any file.
    Any,
}

impl Lang {
    fn matches(self, file: &str) -> bool {
        let extension = file.rsplit_once('.').map(|(_, extension)| extension);
        match self {
            Lang::Rust => extension == Some("rs"),
            Lang::Go => extension == Some("go"),
            Lang::Python => extension == Some("py"),
            Lang::Shell => matches!(extension, Some("sh" | "bash")),
            Lang::Js => matches!(extension, Some("js" | "mjs" | "ts" | "tsx")),
            Lang::Any => true,
        }
    }
}

/// How a pattern's captures become a symbol name.
#[derive(Debug, Clone, Copy)]
pub(crate) enum Shape {
    /// Capture 1.
    Name,
    /// `Type.Method` from captures 1 and 2 (Go receivers).
    Method,
    /// `impl Type` from capture 1.
    Impl,
}

static PATTERNS: LazyLock<Vec<(Lang, Shape, Regex)>> = LazyLock::new(|| {
    PLAN_SIGNATURES
        .iter()
        .filter_map(|(lang, shape, source)| {
            Regex::new(source).ok().map(|regex| (*lang, *shape, regex))
        })
        .collect()
});

fn signature(text: &str) -> Option<(Lang, String)> {
    PATTERNS.iter().find_map(|(lang, shape, regex)| {
        let captures = regex.captures(text)?;
        let first = captures.get(1)?.as_str();
        let name = match shape {
            Shape::Name => first.to_owned(),
            Shape::Impl => format!("impl {first}"),
            Shape::Method => {
                let second = captures.iter().skip(1).nth(1).flatten()?.as_str();
                format!("{first}.{second}")
            }
        };
        Some((*lang, name))
    })
}

#[cfg(test)]
mod tests {
    use std::fmt::Write as _;

    use super::*;

    /// Label, plan text, expected `(file, symbol)` pairs.
    type SymbolCase = (
        &'static str,
        String,
        &'static [(&'static str, &'static str)],
    );

    /// A plan whose `## Changes` section holds `table` followed by `entries`.
    fn plan(table: &str, entries: &str) -> String {
        format!("# P\n\n## Changes\n{table}\n{entries}\n\n## Hot items\n- none\n")
    }

    fn table(rows: &[&str]) -> String {
        let mut text = "| File | Kind | Entry |\n| :--- | :--- | :--- |\n".to_owned();
        for row in rows {
            let _ = writeln!(text, "| {row} | modified | x |");
        }
        text
    }

    fn files(work: &PlannedWork) -> Vec<&str> {
        work.files.iter().map(|file| file.path.as_str()).collect()
    }

    fn symbols(work: &PlannedWork) -> Vec<(&str, &str)> {
        work.symbols
            .iter()
            .map(|(file, name)| (file.as_str(), name.as_str()))
            .collect()
    }

    /// One `### Entry` with a `location:` line and a diff block.
    fn entry(locations: &str, diff: &[&str]) -> String {
        format!(
            "### Entry (modified)\nlocation: {locations}\n\n```diff\n{}\n```\n",
            diff.join("\n")
        )
    }

    #[test]
    fn planned_files() {
        let cases: [(&str, String, &[&str]); 10] = [
            (
                "backticked-and-bare",
                plan(&table(&["`a/b.rs`", "c/d.go"]), ""),
                &["a/b.rs", "c/d.go"],
            ),
            (
                "comma-and-plus-pairs",
                plan(
                    &table(&["`x/one.rs`, `x/two.rs`", "`y/a.rs` + `y/b.rs`"]),
                    "",
                ),
                &["x/one.rs", "x/two.rs", "y/a.rs", "y/b.rs"],
            ),
            (
                "glob-with-prose",
                plan(&table(&["`skills/*/SKILL.md` (15 skills)"]), ""),
                &["skills/*/SKILL.md"],
            ),
            (
                "count-prefix",
                plan(&table(&["10 × `skills/*/changelog.json`"]), ""),
                &["skills/*/changelog.json"],
            ),
            (
                "arrow",
                plan(&table(&["`old/a.rs` → `new/a.rs`"]), ""),
                &["new/a.rs", "old/a.rs"],
            ),
            (
                "bare-continuation",
                plan(&table(&["`x/SKILL.md` + `changelog.json`"]), ""),
                &["x/SKILL.md", "x/changelog.json"],
            ),
            (
                "prose-suffix",
                plan(&table(&["`cmd/*.go` run by `make audit`"]), ""),
                &["cmd/*.go"],
            ),
            (
                "kind-suffix",
                plan(
                    "| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `g/gen.json` | modified (generated) | x |\n",
                    "",
                ),
                &["g/gen.json"],
            ),
            (
                "baseline-table-first",
                format!(
                    "# P\n\n## Baseline\n| ID | Fact | Location |\n| :--- | :--- | :--- |\n| F1 | x | `z/base.rs` |\n\n{}",
                    plan(&table(&["`a/b.rs`"]), "")
                ),
                &["a/b.rs"],
            ),
            (
                "second-table-ignored",
                plan(&table(&["`a/b.rs`"]), &table(&["`later/c.rs`"])),
                &["a/b.rs"],
            ),
        ];
        for (label, markdown, expected) in cases {
            assert_eq!(files(&parse(&markdown)), expected, "{label}");
        }
    }

    #[test]
    fn location_fallback() {
        let markdown = plan("", &entry("`a/b.rs:120`", &[" fn x() {}"]));
        let work = parse(&markdown);
        assert_eq!(files(&work), ["a/b.rs"]);
        assert!(work.files.iter().all(|file| !file.glob));
    }

    #[test]
    fn fenced_template_ignored() {
        let markdown = format!(
            "# P\n\n````markdown\n## Changes\n{}\n````\n",
            table(&["`t/template.rs`"])
        );
        assert_eq!(parse(&markdown), PlannedWork::default());
    }

    #[test]
    fn symbols_by_language() {
        let cases: [SymbolCase; 8] = [
            (
                "rust-context-line",
                entry(
                    "`src/x.rs`",
                    &[" pub fn build(options: &Options) -> Result<Document> {"],
                ),
                &[("src/x.rs", "build")],
            ),
            (
                "go-method-header",
                entry(
                    "`internal/peek/client.go`",
                    &["@@ func (c *Client) SessionEvents(ctx context.Context) error {"],
                ),
                &[("internal/peek/client.go", "Client.SessionEvents")],
            ),
            (
                "go-removed-func",
                entry("`cmd/x.go`", &["-func Status(ctx context.Context) error {"]),
                &[("cmd/x.go", "Status")],
            ),
            (
                "path-line-header",
                entry(
                    "`internal/peek/events.go`",
                    &["@@ internal/peek/events.go:21"],
                ),
                &[],
            ),
            (
                "markdown-hunk",
                entry("`README.md`", &["+## Title", "+Some prose."]),
                &[],
            ),
            (
                "impl-for",
                entry("`src/y.rs`", &[" impl Foo for Bar {"]),
                &[("src/y.rs", "impl Bar")],
            ),
            (
                "go-unnamed-receiver",
                entry("`srv/s.go`", &[" func (Server) Name() string {"]),
                &[("srv/s.go", "Server.Name")],
            ),
            (
                "bare-header-two-files-no-owner",
                entry("`a.go`, `b.md`", &["@@ LoadGrants"]),
                &[],
            ),
        ];
        for (label, entries, expected) in cases {
            assert_eq!(symbols(&parse(&plan("", &entries))), expected, "{label}");
        }
    }

    /// Lines 433, 434, 2581, 3217, 3465 and 3713 of the T1 worktree-confined-access plan (commit 7ba605759).
    #[test]
    fn t1_plan_lines() {
        let braces = plan(
            "| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `skills/context/context-bootstrap/{SKILL.md,changelog.json,atoms.json}` | modified | [Skill edits](#changes) |\n",
            "",
        );
        assert_eq!(
            files(&parse(&braces)),
            [
                "skills/context/context-bootstrap/SKILL.md",
                "skills/context/context-bootstrap/atoms.json",
                "skills/context/context-bootstrap/changelog.json",
            ],
            "433: one brace group expands"
        );

        let directories = plan(
            "| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `skills/verdict/investigate`, `skills/feature/fimplement`, `skills/quality/aqms-review-wip`, `skills/quality/superpowers-review` (SKILL.md, changelog, moved `vendor/` files, arm prompts) | modified | [Skill edits](#changes) |\n",
            "",
        );
        let work = parse(&directories);
        assert!(
            work.files
                .iter()
                .any(|file| file.path == "vendor/**" && file.glob),
            "434: a trailing slash is a directory glob: {:?}",
            files(&work)
        );

        let grants = parse(&plan(
            "",
            &entry(
                "`cmd/toolpolicy/grants.go:34-123`, `grants.go:165-205`, `grants.go:252-266`",
                &[" func LoadGrants(input *HookInput) (*Grants, error) {"],
            ),
        ));
        assert_eq!(
            files(&grants),
            ["cmd/toolpolicy/grants.go"],
            "2581: bare name resolved"
        );
        assert_eq!(
            symbols(&grants),
            [("cmd/toolpolicy/grants.go", "LoadGrants")],
            "2581: one owner"
        );

        let continuation = parse(&plan(
            "",
            &entry(
                "`internal/config/installenv.go`, `internal/server/config_sync.go:45-90`, `:121-150`",
                &[" func (s *Server) handleConfigSyncOp(w http.ResponseWriter) {"],
            ),
        ));
        assert_eq!(
            files(&continuation),
            [
                "internal/config/installenv.go",
                "internal/server/config_sync.go"
            ],
            "3217: a line-range span is no path"
        );
        assert_eq!(symbols(&continuation), [], "3217: two Go files, no owner");

        let several = parse(&plan(
            "",
            &entry(
                "`internal/skilleval/provision.go`, `internal/skilleval/manifest.go:46-64`, `internal/skilleval/arm.go:171-210`, `cmd/skilleval/main.go:30-93`, `internal/gitwt/gitwt.go:184`",
                &["+func RunManifest(ctx context.Context) error {"],
            ),
        ));
        assert_eq!(symbols(&several), [], "3465: never gitwt.go::RunManifest");

        let prose = parse(&plan(
            "",
            &entry(
                "`internal/skilleval/arm.go` (types in type-block order; call in `RunArm` after the `-out` artifacts `MkdirAll`, ~line 410)",
                &["+func writeArmSettings(dir string) error {"],
            ),
        ));
        assert_eq!(
            files(&prose),
            ["internal/skilleval/arm.go"],
            "3713: prose spans dropped"
        );
        assert_eq!(
            symbols(&prose),
            [("internal/skilleval/arm.go", "writeArmSettings")]
        );
    }

    #[test]
    fn deeper_heading_resets_locations() {
        let markdown = plan(
            "",
            "#### Style publish (modified)\nlocation: `a/one.go`\n\n#### Skill edits (modified)\n\n```diff\n+func Leak() {}\n```\n",
        );
        assert_eq!(
            symbols(&parse(&markdown)),
            [],
            "the next entry has no location"
        );
    }

    #[test]
    fn path_header_owns_its_symbols() {
        let markdown = plan(
            "",
            &entry(
                "`a/one.go`, `a/two.go`",
                &["@@ a/two.go:12", " func Two() error {"],
            ),
        );
        assert_eq!(symbols(&parse(&markdown)), [("a/two.go", "Two")]);
    }

    #[test]
    fn changed_lines() {
        let one = parse(&plan(
            "",
            &entry("`a/SKILL.md`", &["-version: 1.0.0", "+version: 1.1.0"]),
        ));
        assert_eq!(
            one.changed.get("a/SKILL.md").map(Vec::as_slice),
            Some(["version: 1.0.0".to_owned(), "version: 1.1.0".to_owned()].as_slice())
        );

        let narrowed = parse(&plan(
            "",
            &entry(
                "`a/SKILL.md`, `b/x.md`",
                &["@@ a/SKILL.md:5", "+version: 2.0.0"],
            ),
        ));
        assert_eq!(narrowed.changed.keys().collect::<Vec<_>>(), ["a/SKILL.md"]);

        let headers = parse(&plan(
            "",
            &entry("`c.rs`", &["--- a/c.rs", "+++ b/c.rs", "+let x = 1;"]),
        ));
        assert_eq!(
            headers.changed.get("c.rs").map(Vec::as_slice),
            Some(["let x = 1;".to_owned()].as_slice())
        );
    }

    #[test]
    fn plan_dir_of_wip_design_files() {
        let cases = [
            (
                "feature-design",
                "plans/2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234/design/raw.md",
                Some("2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234"),
            ),
            (
                "old-depth",
                "plans/2026-09-27-x-wip-abc1234/design/raw.md",
                None,
            ),
            (
                "done-dir",
                "plans/2026-09-27-x-abc1234/2026-09-27-x-done-abc1234/design/raw.md",
                None,
            ),
            (
                "nested",
                "plans/2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234/design/sub/raw.md",
                None,
            ),
            (
                "non-md",
                "plans/2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234/design/ui.png",
                None,
            ),
        ];
        for (label, rel, expected) in cases {
            assert_eq!(plan_dir_of(rel).as_deref(), expected, "{label}");
        }
    }

    #[test]
    fn changes_only() {
        let markdown = "## Changes\n\n| File | Kind | Entry |\n| --- | --- | --- |\n| `a.rs` | modified | §1 |\n\n\
            ## Addenda\n\n### A1 — one\n\n| File | Kind | Entry |\n| --- | --- | --- |\n| `b.rs` | modified | A1 |\n\n\
            ### A2 — two\n\nlocation: `c.rs`\n\n```diff\n+fn added() {}\n```\n\n## Changelog\n";
        let work = parse(markdown);
        assert_eq!(files(&work), ["a.rs"]);
        assert_eq!(symbols(&work), []);
        assert!(work.changed.is_empty());
    }

    #[test]
    fn patterns_all_compile() {
        assert_eq!(PATTERNS.len(), PLAN_SIGNATURES.len());
    }
}
