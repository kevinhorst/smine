//! The `agentwt` binary: clap surface, dispatch, rendering, exit codes. The library never prints; this file owns
//! stdout (payload only) and stderr (everything else) through anstream.

use std::io::Write as _;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::Arc;
use std::sync::atomic::AtomicBool;

use anyhow::Context as _;
use clap::{Args, Parser, Subcommand, ValueEnum};

use agentwt::inventory::{self, agent_kind};
use agentwt::ops::{Git, RemovePlan};
use agentwt::wire::{
    Document, Forward, ForwardOutcome, OpResult, Pick, PickOutcome, Probe, ProbeEntry,
    ProbeOutcome, TrainOutcome, Unplaced,
};
use agentwt::{status, unplaced};

/// Integration state, merge order and operations for agent worktrees.
#[derive(Debug, Parser)]
#[command(name = "agentwt", version, about, long_about = None)]
struct Cli {
    /// Repository path (any worktree of it).
    #[arg(short = 'C', long = "repo", global = true, default_value = ".")]
    repo: PathBuf,
    /// Output format.
    #[arg(long, global = true, value_enum, default_value_t = Format::Text)]
    format: Format,
    /// Claude home directory (repeatable); default `$HOME/.claude`.
    #[arg(long = "claude-home", global = true)]
    claude_homes: Vec<PathBuf>,
    /// Codex home directory; default `$HOME/.codex` when it exists.
    #[arg(long = "codex-home", global = true)]
    codex_home: Option<PathBuf>,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum Format {
    Text,
    Json,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// The status document for every agent branch and detached worktree, or for one kind's (`--kind`).
    Status(StatusArgs),
    /// Remove worktrees (and optionally branches) when safe or forced.
    Remove(RemoveArgs),
    /// The commits one branch still owes one target: its non-merge commits the target does not hold, oldest first.
    Unplaced(UnplacedArgs),
    /// Pick one commit onto the tip of a local branch; exit 3 when it conflicts or is skipped.
    #[command(after_long_help = "\
A checked-out target is picked in its own worktree (`git cherry-pick -x`, aborted on a conflict) and must be clean; \
any other target gets a commit built from `git merge-tree` and the branch moved by a compare-and-swap `git update-ref`. \
Both record the `(cherry picked from commit <sha>)` trailer. Root commits, merge commits and commits the target \
already holds are skipped.")]
    Pick(PickArgs),
    /// Decide for each commit whether a pick onto the tip of a local branch would apply, conflict or be skipped;
    /// nothing moves, exit 0 whatever the outcomes.
    #[command(after_long_help = "\
Every commit is probed with `git merge-tree` against the target's tip, after the refusals `pick` applies (root and \
merge commits, commits the target already holds). One call answers for every commit of a batch.")]
    Probe(ProbeArgs),
    /// Fast-forward a local branch to a commit that holds its tip; exit 3 when it is skipped.
    #[command(after_long_help = "\
A checked-out target moves in its own worktree (`git merge --ff-only`), so its index and files follow, and must have \
no tracked changes; any other target moves by a compare-and-swap `git update-ref`. A commit that does not hold the \
target's tip is skipped — never a merge commit, never a forced move.")]
    Forward(ForwardArgs),
    /// Print the JSON Schema of a document.
    Schema(SchemaArgs),
    /// Print the interface versions this build implements.
    Interfaces,
}

#[derive(Debug, Args)]
struct StatusArgs {
    /// Skip the merge-order computation.
    #[arg(long)]
    no_order: bool,
    /// Also compute the merge train seeded with this local non-agent branch's tip (repeatable).
    #[arg(long = "train-base", value_parser = clap::builder::NonEmptyStringValueParser::new())]
    train_bases: Vec<String>,
    /// Also track this local branch as a containment candidate, agent-owned or not (repeatable); it may then be a
    /// `--train-base` too.
    #[arg(long = "candidate", value_parser = clap::builder::NonEmptyStringValueParser::new())]
    candidates: Vec<String>,
    /// Only this kind's rows (`claude`, `claude-routines`, …, `codex`); the document's `kinds` still lists every kind.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    kind: Option<String>,
}

#[derive(Debug, Args)]
struct RemoveArgs {
    /// Also delete the branch, under the same safety rule.
    #[arg(long)]
    delete_branch: bool,
    /// Report what would be removed without changing anything; exit 3 when anything would be skipped.
    #[arg(long)]
    dry_run: bool,
    /// Override the safety predicate.
    #[arg(long)]
    force: bool,
    #[arg(required = true)]
    branches: Vec<String>,
}

#[derive(Debug, Args)]
struct UnplacedArgs {
    /// The local branch the commits are judged against, agent-owned or not.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    onto: String,
    /// The local branch whose commits are listed, agent-owned or not.
    #[arg(value_parser = clap::builder::NonEmptyStringValueParser::new())]
    branch: String,
}

#[derive(Debug, Args)]
struct PickArgs {
    /// The local branch the commit is picked onto.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    onto: String,
    /// The commit (any commit-ish).
    #[arg(value_parser = clap::builder::NonEmptyStringValueParser::new())]
    commit: String,
}

#[derive(Debug, Args)]
struct ProbeArgs {
    /// The local branch the commits would be picked onto.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    onto: String,
    /// The commits (any commit-ish), answered in this order.
    #[arg(required = true, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    commits: Vec<String>,
}

#[derive(Debug, Args)]
struct ForwardArgs {
    /// The local branch that moves.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    onto: String,
    /// The commit it moves to (any commit-ish, resolved in the caller's checkout).
    #[arg(value_parser = clap::builder::NonEmptyStringValueParser::new())]
    commit: String,
}

#[derive(Debug, Args)]
struct SchemaArgs {
    /// The document whose schema is printed.
    #[arg(value_enum, default_value_t = SchemaDocument::Status)]
    document: SchemaDocument,
}

#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum SchemaDocument {
    Status,
    Unplaced,
    Pick,
    Forward,
    Probe,
}

/// What a command renders.
enum Rendered {
    Document(Box<Document>),
    Op(OpResult),
    Unplaced(Unplaced),
    Pick(Pick),
    Forward(Forward),
    Probe(Probe),
    Schema(String),
    Interfaces(String),
}

impl Rendered {
    /// Exit code of a successfully rendered result: a pick that conflicted or was skipped and a skipped forward are
    /// 3, everything else 0 — a probe's outcomes are its document, never its exit.
    fn exit_code(&self) -> ExitCode {
        match self {
            Rendered::Pick(pick) => match pick.outcome {
                PickOutcome::Picked => ExitCode::SUCCESS,
                PickOutcome::Conflict | PickOutcome::Skipped => ExitCode::from(3),
            },
            Rendered::Forward(forward) => match forward.outcome {
                ForwardOutcome::Forwarded | ForwardOutcome::Unchanged => ExitCode::SUCCESS,
                ForwardOutcome::Skipped => ExitCode::from(3),
            },
            Rendered::Document(_)
            | Rendered::Op(_)
            | Rendered::Unplaced(_)
            | Rendered::Probe(_)
            | Rendered::Schema(_)
            | Rendered::Interfaces(_) => ExitCode::SUCCESS,
        }
    }
}

fn main() -> ExitCode {
    let interrupted = Arc::new(AtomicBool::new(false));
    for signal in [signal_hook::consts::SIGINT, signal_hook::consts::SIGTERM] {
        if let Err(err) = signal_hook::flag::register(signal, Arc::clone(&interrupted)) {
            let _ = writeln!(
                anstream::stderr(),
                "agentwt: could not install signal handler: {err}"
            );
        }
    }
    let cli = Cli::parse();
    let format = cli.format;
    match run(cli, interrupted) {
        Ok(rendered) => match render(&rendered, format) {
            Ok(()) => rendered.exit_code(),
            Err(err) => report(&err),
        },
        Err(err) => report(&err),
    }
}

fn report(err: &anyhow::Error) -> ExitCode {
    let code = err
        .downcast_ref::<agentwt::Error>()
        .map_or(1, agentwt::Error::exit_code);
    let _ = writeln!(anstream::stderr(), "agentwt: {err:#}");
    ExitCode::from(u8::try_from(code).unwrap_or(1))
}

fn run(cli: Cli, interrupted: Arc<AtomicBool>) -> anyhow::Result<Rendered> {
    let home = std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from);
    let claude_homes = if cli.claude_homes.is_empty() {
        home.iter().map(|home| home.join(".claude")).collect()
    } else {
        cli.claude_homes.clone()
    };
    let codex_home = cli.codex_home.clone().or_else(|| {
        home.as_ref()
            .map(|home| home.join(".codex"))
            .filter(|path| path.is_dir())
    });
    let options = status::Options {
        repo: cli.repo.clone(),
        claude_homes,
        codex_home,
        skip_order: !matches!(
            cli.command,
            Command::Status(StatusArgs {
                no_order: false,
                ..
            })
        ),
        branches: match &cli.command {
            Command::Remove(args) => Some(args.branches.clone()),
            Command::Status(_)
            | Command::Unplaced(_)
            | Command::Pick(_)
            | Command::Probe(_)
            | Command::Forward(_)
            | Command::Schema(_)
            | Command::Interfaces => None,
        },
        train_bases: match &cli.command {
            Command::Status(args) => args.train_bases.clone(),
            Command::Remove(_)
            | Command::Unplaced(_)
            | Command::Pick(_)
            | Command::Probe(_)
            | Command::Forward(_)
            | Command::Schema(_)
            | Command::Interfaces => Vec::new(),
        },
        candidates: match &cli.command {
            Command::Status(args) => args.candidates.clone(),
            Command::Remove(_)
            | Command::Unplaced(_)
            | Command::Pick(_)
            | Command::Probe(_)
            | Command::Forward(_)
            | Command::Schema(_)
            | Command::Interfaces => Vec::new(),
        },
    };
    let git = Git::new(interrupted);

    let rendered = match cli.command {
        Command::Status(args) => Rendered::Document(Box::new(
            status::build_kind(&options, args.kind.as_deref()).context("status")?,
        )),
        Command::Remove(args) => Rendered::Op(remove(&git, &options, &args)?),
        Command::Unplaced(args) => Rendered::Unplaced(
            unplaced::build(&options.repo, &args.branch, &args.onto).context("unplaced")?,
        ),
        Command::Pick(args) => {
            let main_checkout = inventory::main_checkout(&options.repo).context("pick")?;
            let plan = git.plan_pick(&main_checkout, &args.commit, &args.onto)?;
            Rendered::Pick(Pick::from(git.pick(&main_checkout, &plan)?))
        }
        Command::Probe(args) => {
            let main_checkout = inventory::main_checkout(&options.repo).context("probe")?;
            Rendered::Probe(Probe::from(git.probe(
                &main_checkout,
                &args.commits,
                &args.onto,
            )?))
        }
        Command::Forward(args) => {
            let main_checkout = inventory::main_checkout(&options.repo).context("forward")?;
            let plan = git.plan_forward(&options.repo, &main_checkout, &args.commit, &args.onto)?;
            Rendered::Forward(Forward::from(git.forward(&main_checkout, &plan)?))
        }
        Command::Schema(args) => {
            let schema = match args.document {
                SchemaDocument::Status => schemars::schema_for!(Document),
                SchemaDocument::Unplaced => schemars::schema_for!(Unplaced),
                SchemaDocument::Pick => schemars::schema_for!(Pick),
                SchemaDocument::Forward => schemars::schema_for!(Forward),
                SchemaDocument::Probe => schemars::schema_for!(Probe),
            };
            Rendered::Schema(serde_json::to_string_pretty(&schema)?)
        }
        Command::Interfaces => {
            Rendered::Interfaces(serde_json::to_string(&agentwt::interfaces::answer())?)
        }
    };
    Ok(rendered)
}

/// `remove`: refuses non-agent and unknown branches, then plans and executes (or previews) each named branch.
fn remove(git: &Git, options: &status::Options, args: &RemoveArgs) -> anyhow::Result<OpResult> {
    for branch in &args.branches {
        if agent_kind(branch).is_none() {
            return Err(
                agentwt::Error::from(agentwt::Refusal::NotAgentBranch(branch.clone())).into(),
            );
        }
    }
    let document = status::build(options).context("status for remove")?;
    let main_checkout = PathBuf::from(&document.repo.path);
    let mut lines = Vec::new();
    for branch in &args.branches {
        let row = document
            .rows
            .iter()
            .find(|row| row.branch.as_deref() == Some(branch.as_str()))
            .ok_or_else(|| agentwt::Error::from(agentwt::Refusal::UnknownBranch(branch.clone())))?;
        let plan = RemovePlan {
            branch: branch.clone(),
            worktree: row.worktree.as_ref().map(|wt| PathBuf::from(&wt.path)),
            safe: row.safe_to_remove,
            unsafe_reasons: row.unsafe_reasons.clone(),
            force: args.force,
            delete_branch: args.delete_branch,
        };
        let result = if args.dry_run {
            plan.preview()?
        } else {
            git.remove(&main_checkout, &plan)?
        };
        lines.extend(result.lines);
    }
    Ok(OpResult {
        operation: "remove".to_owned(),
        lines,
    })
}

fn render(rendered: &Rendered, format: Format) -> anyhow::Result<()> {
    let mut out = anstream::stdout();
    match (rendered, format) {
        (Rendered::Document(document), Format::Json) => {
            serde_json::to_writer_pretty(&mut out, document)?;
        }
        (Rendered::Document(document), Format::Text) => write_table(&mut out, document)?,
        (Rendered::Op(result), Format::Json) => serde_json::to_writer_pretty(&mut out, result)?,
        (Rendered::Op(result), Format::Text) => {
            for line in &result.lines {
                writeln!(out, "{}", line.text)?;
            }
        }
        (Rendered::Unplaced(unplaced), Format::Json) => {
            serde_json::to_writer_pretty(&mut out, unplaced)?;
        }
        (Rendered::Unplaced(unplaced), Format::Text) => {
            for commit in &unplaced.commits {
                writeln!(out, "{} {}", commit.short_sha, commit.subject)?;
            }
        }
        (Rendered::Pick(pick), Format::Json) => serde_json::to_writer_pretty(&mut out, pick)?,
        (Rendered::Pick(pick), Format::Text) => write!(out, "{}", pick_line(pick))?,
        (Rendered::Probe(probe), Format::Json) => serde_json::to_writer_pretty(&mut out, probe)?,
        (Rendered::Probe(probe), Format::Text) => {
            let lines: Vec<String> = probe
                .commits
                .iter()
                .map(|entry| probe_line(entry, &probe.onto))
                .collect();
            write!(out, "{}", lines.join("\n"))?;
        }
        (Rendered::Forward(forward), Format::Json) => {
            serde_json::to_writer_pretty(&mut out, forward)?;
        }
        (Rendered::Forward(forward), Format::Text) => write!(out, "{}", forward_line(forward))?,
        (Rendered::Schema(schema), _) => write!(out, "{schema}")?,
        (Rendered::Interfaces(interfaces), _) => write!(out, "{interfaces}")?,
    }
    writeln!(out)?;
    Ok(())
}

/// The one text line of a pick.
fn pick_line(pick: &Pick) -> String {
    let short = |sha: &str| sha.get(..8).unwrap_or(sha).to_owned();
    let commit = short(&pick.commit);
    let onto = &pick.onto;
    let reason = pick.reason.as_deref().unwrap_or_default();
    match pick.outcome {
        PickOutcome::Picked => format!(
            "picked {commit} onto {onto} as {}",
            short(pick.new_commit.as_deref().unwrap_or_default())
        ),
        PickOutcome::Conflict => format!("conflict {commit} onto {onto}: {reason}"),
        PickOutcome::Skipped => format!("skipped {commit}: {reason}"),
    }
}

/// The one text line of a probed commit.
fn probe_line(entry: &ProbeEntry, onto: &str) -> String {
    let commit = entry.commit.get(..8).unwrap_or(&entry.commit);
    let reason = entry.reason.as_deref().unwrap_or_default();
    match entry.outcome {
        ProbeOutcome::Applies => format!("applies {commit} onto {onto}"),
        ProbeOutcome::Conflict => format!("conflict {commit} onto {onto}: {reason}"),
        ProbeOutcome::Skipped => format!("skipped {commit}: {reason}"),
    }
}

/// The one text line of a forward.
fn forward_line(forward: &Forward) -> String {
    let short = |sha: &str| sha.get(..8).unwrap_or(sha).to_owned();
    let onto = &forward.onto;
    let to = short(&forward.to);
    match forward.outcome {
        ForwardOutcome::Forwarded => {
            format!("forwarded {onto} {}..{to}", short(&forward.from))
        }
        ForwardOutcome::Unchanged => format!("unchanged {onto} at {to}"),
        ForwardOutcome::Skipped => format!(
            "skipped {onto}: {}",
            forward.reason.as_deref().unwrap_or_default()
        ),
    }
}

/// Human table: one line per row, the columns the operator scans first.
fn write_table(out: &mut impl std::io::Write, document: &Document) -> anyhow::Result<()> {
    const HEADER: [&str; 10] = [
        "BRANCH", "FROM", "DIRTY", "UNTR", "AHEAD", "BEHIND", "UNPICKED", "IN", "TRAIN", "WORKTREE",
    ];
    let h = HEADER;
    writeln!(
        out,
        "{:<44} {:<14} {:>5} {:>5} {:>6} {:>6} {:>8} {:<40} {:<8} {}",
        h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], h[8], h[9]
    )?;
    for row in &document.rows {
        let branch = row
            .branch
            .clone()
            .unwrap_or_else(|| format!("(detached {})", row.head.short_sha));
        let from = row
            .from
            .name
            .clone()
            .unwrap_or_else(|| "unknown".to_owned());
        let (dirty, untracked) = row
            .worktree
            .as_ref()
            .map_or(("-".to_owned(), "-".to_owned()), |wt| {
                (wt.dirty.to_string(), wt.untracked.to_string())
            });
        let (ahead, behind) = row
            .counts
            .vs_from
            .map_or(("-".to_owned(), "-".to_owned()), |c| {
                (c.ahead.to_string(), c.behind.to_string())
            });
        let contained = row.containment.as_ref().map_or("-".to_owned(), |c| {
            format!(
                "{} ({})",
                c.branch,
                serde_json::to_string(&c.reason)
                    .unwrap_or_default()
                    .trim_matches('"')
            )
        });
        let train = row
            .order
            .as_ref()
            .map_or("-".to_owned(), |order| match &order.train {
                TrainOutcome::Clean => format!("#{}", order.train_position),
                TrainOutcome::Blocked { .. } => format!("#{}!", order.train_position),
                TrainOutcome::Nothing => "-".to_owned(),
            });
        writeln!(
            out,
            "{:<44} {:<14} {:>5} {:>5} {:>6} {:>6} {:>8} {:<40} {:<8} {}",
            branch,
            from,
            dirty,
            untracked,
            ahead,
            behind,
            row.unpicked,
            contained,
            train,
            row.worktree.as_ref().map_or("–", |wt| wt.path.as_str()),
        )?;
    }
    Ok(())
}
