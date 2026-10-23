//! The `heartbeat` binary: clap surface, dispatch, rendering, exit codes. stdout carries only the payload (status
//! document or table, schema); every log line goes to stderr.
#![cfg_attr(windows, windows_subsystem = "windows")]

use std::io::Write as _;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::process::ExitCode;

use anyhow::Context as _;
use clap::{Args, Parser, Subcommand, ValueEnum};

use heartbeat::client::Client;
use heartbeat::config::{Caps, Startup};
use heartbeat::fleet::SubscribeRequest;
use heartbeat::ids::{FleetId, MessagingToken, SessionId};
use heartbeat::protocol::{
    DEFAULT_ADDR, EVERY_FLEET, FLEET_ID_CHARSET, FLEET_TABLE_WIDTHS, MAX_MEMBER_LETTERS,
    MEMBER_TABLE_WIDTHS, RUNTIME_TABLE_WIDTHS, SELF_SESSION, SESSION_ENV, SOCKET_ENV,
    STATUS_TABLE_WIDTHS, TOKEN_ENV,
};
use heartbeat::status::Filter;
use heartbeat::wire::{
    self, DoneAnswer, DoneBody, FindingRow, FleetRow, JoinBody, JournalBody, LandAnswerRow,
    LandBody, LandEndBody, LandEndWire, LandReleaseBody, LandStateWire, LeaveBody, MemberRow,
    ModeWire, MoveWire, RecordedRow, RemoveBody, RuntimeActBody, RuntimeDocument,
    RuntimeSubscribeBody, RuntimeSubscriptionRow, RuntimeUnsubscribeBody, SensorStateWire,
    SessionRow, StatusDocument, SubscribeBody, SubscriptionRow, UnsubscribeBody, WorkerBody,
    WorkerStateWire,
};

/// Fleet control plane: session lifecycle from hooks, the session registry, the denial ledger and process exits.
#[derive(Debug, Parser)]
#[command(name = "heartbeat", version, about, long_about = None)]
struct Cli {
    /// Daemon address (loopback).
    #[arg(long, global = true, env = "HEARTBEAT_ADDR", default_value = DEFAULT_ADDR)]
    addr: SocketAddr,
    /// Claude home directory; default `$HOME/.claude` (`%USERPROFILE%\.claude` on windows).
    #[arg(long = "claude-home", global = true, env = "CLAUDE_CONFIG_DIR")]
    claude_home: Option<PathBuf>,
    /// Append the daemon's log lines to this file instead of stderr (the Windows logon task passes it).
    #[arg(long = "log-file", global = true, env = "HEARTBEAT_LOG_FILE")]
    log_file: Option<PathBuf>,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Run the daemon (launchd runs this).
    Serve(ServeArgs),
    /// Print the status document.
    Status(StatusArgs),
    /// Print the JSON Schema of the status document (or of an archive file).
    Schema(SchemaArgs),
    /// Register the calling Claude Code session for its fleet's digests.
    Subscribe(SubscribeArgs),
    /// End the calling Claude Code session's subscription.
    Unsubscribe(UnsubscribeArgs),
    /// Join the calling Claude Code session to a fleet; prints its member token (`T1-MA`).
    Join(JoinArgs),
    /// Report the calling session's work as finished: a joined session is delivered to its fleet, any other is
    /// accepted for its branch's landing target, or done without a landing when its branch has none.
    Done(DoneArgs),
    /// Leave the calling joined session's fleet; the fleet dissolves once no member and no orchestrator remain.
    Leave(DoneArgs),
    /// Record an orchestrator act in its fleet's journal (the fleet's active subscriber only).
    Record(RecordArgs),
    /// Register a worker branch in the fleet, or report its work finished (the fleet's active subscriber only);
    /// prints the worker's member token.
    Worker(WorkerArgs),
    /// Remove a lettered member from a fleet (a human act; needs no session).
    Remove(RemoveArgs),
    /// Request, end or release the landing turn of a landing target.
    Land(LandArgs),
    /// Watch skill runs live through the runtime checker: subscribe, read, answer findings.
    Runtime(RuntimeArgs),
    /// Print the interface versions this build implements.
    Interfaces,
}

#[derive(Debug, Args)]
struct RuntimeArgs {
    #[command(subcommand)]
    command: RuntimeCommand,
}

#[derive(Debug, Subcommand)]
enum RuntimeCommand {
    /// Take the one runtime subscription for the calling Claude Code session.
    Subscribe(RuntimeSubscribeArgs),
    /// End the calling session's runtime subscription.
    Unsubscribe(DoneArgs),
    /// Print the runtime document: the subscription, runs and findings.
    Status(DoneArgs),
    /// Hand heartbeat's rendered template for a finding to the agent that raised it (act mode).
    Send(RuntimeMoveArgs),
    /// Stop the agent that raised a finding (act mode; triggered-stop and violated-rule findings only).
    Stop(RuntimeMoveArgs),
}

#[derive(Debug, Args)]
struct RuntimeSubscribeArgs {
    /// The one session to watch: its id or exact title. Only its findings reach this watcher.
    #[arg(long)]
    target: String,
    /// Report findings only, or also answer them with heartbeat's moves; fixed for the subscription.
    #[arg(long, value_enum, default_value_t = RuntimeMode::Report)]
    mode: RuntimeMode,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct RuntimeMoveArgs {
    /// The finding id (`runtime status` lists them).
    #[arg(value_parser = clap::builder::NonEmptyStringValueParser::new())]
    finding: String,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

/// `runtime subscribe --mode`.
#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum RuntimeMode {
    Report,
    Act,
}

impl From<RuntimeMode> for ModeWire {
    fn from(mode: RuntimeMode) -> ModeWire {
        match mode {
            RuntimeMode::Report => ModeWire::Report,
            RuntimeMode::Act => ModeWire::Act,
        }
    }
}

#[derive(Debug, Args)]
struct ServeArgs {
    /// Append every received hook body to this JSONL file (replay format).
    #[arg(long)]
    record: Option<PathBuf>,
    /// The config server's repo registry (`repos.json`); git state is off without it.
    #[arg(long)]
    repos: Option<PathBuf>,
    /// The rendered heartbeat config (`<claude home>/heartbeat/config.json`); default: that path under `--claude-home`.
    #[arg(long)]
    config: Option<PathBuf>,
}

#[derive(Debug, Args)]
struct StatusArgs {
    /// Only members of this fleet; without a value (or `*`), list every fleet.
    #[arg(long, num_args = 0..=1, default_missing_value = EVERY_FLEET, value_parser = fleet_scope)]
    fleet: Option<FleetScope>,
    /// One session: an id, or `self` for the calling Claude Code session.
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    session: Option<String>,
    /// Only sessions whose cwd lies under this path.
    #[arg(long)]
    repo: Option<PathBuf>,
    /// Only the git worktree holding this path (`.`: the working directory's): its branches row and its sessions.
    #[arg(long, value_parser = worktree_path)]
    worktree: Option<PathBuf>,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct SubscribeArgs {
    /// The fleet this session orchestrates (the fleet id in its members' titles).
    #[arg(long, value_parser = fleet_id)]
    fleet: FleetId,
    /// The fleet's integration branch; member branches are judged against its merge train. The repository is the
    /// one this session's cwd lies in.
    #[arg(long, value_parser = target_branch)]
    target: String,
    /// Also hear collisions between a member and a branch outside the fleet (default: fleet branches only).
    #[arg(long = "outside-collisions")]
    outside_collisions: bool,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct UnsubscribeArgs {
    /// The run is over: every joined member leaves too, so the fleet dissolves into its archive.
    #[arg(long)]
    finish: bool,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct JoinArgs {
    /// The fleet to join.
    #[arg(long, value_parser = fleet_id)]
    fleet: FleetId,
    /// Output format; text is exactly the member token, the session-title hook reads it.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct SchemaArgs {
    /// The schema of an archive file (`<claude home>/heartbeat/archive/<instance>.json`) instead.
    #[arg(long, conflicts_with = "runtime")]
    archive: bool,
    /// The schema of the runtime document (`heartbeat runtime status --format json`) instead.
    #[arg(long)]
    runtime: bool,
}

#[derive(Debug, Args)]
struct DoneArgs {
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
#[command(group(
    clap::ArgGroup::new("act")
        .required(true)
        .multiple(false)
        .args(["target", "end", "release"])
))]
struct LandArgs {
    /// Request the turn for this branch; the repository is the one the calling session's cwd lies in.
    #[arg(long, value_parser = target_branch)]
    target: Option<String>,
    /// End the calling session's entry: `landed` for the turn holder, `abandoned` for the holder or a waiter.
    #[arg(long, value_enum)]
    end: Option<LandEndArg>,
    /// Operator override: end the holder's turn for this branch in the repository of the working directory.
    #[arg(long, value_parser = target_branch)]
    release: Option<String>,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

/// `land --end`.
#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum LandEndArg {
    Landed,
    Abandoned,
}

impl From<LandEndArg> for LandEndWire {
    fn from(end: LandEndArg) -> LandEndWire {
        match end {
            LandEndArg::Landed => LandEndWire::Landed,
            LandEndArg::Abandoned => LandEndWire::Abandoned,
        }
    }
}

#[derive(Debug, Args)]
struct RecordArgs {
    /// `assigned`, `decided`, `merged`, `spawned` or `note`.
    #[arg(long)]
    kind: String,
    /// One line: what.
    #[arg(long)]
    subject: String,
    /// A member letter it concerns (`A`); repeatable.
    #[arg(long = "member")]
    members: Vec<String>,
    /// Why, or the evidence.
    #[arg(long)]
    detail: Option<String>,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

#[derive(Debug, Args)]
struct WorkerArgs {
    /// The worker's branch, `<worker_branch_prefix><name>` (`claude/<name>` by default).
    #[arg(long, value_parser = non_empty)]
    branch: String,
    /// `running` registers the worker or reopens it, `delivered` reports its work finished.
    #[arg(long, value_enum)]
    state: WorkerStateArg,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

/// What `worker --state` reports.
#[derive(Debug, Clone, Copy, PartialEq, Eq, ValueEnum)]
enum WorkerStateArg {
    /// Registered and working.
    Running,
    /// Work finished.
    Delivered,
}

impl From<WorkerStateArg> for WorkerStateWire {
    fn from(state: WorkerStateArg) -> WorkerStateWire {
        match state {
            WorkerStateArg::Running => WorkerStateWire::Running,
            WorkerStateArg::Delivered => WorkerStateWire::Delivered,
        }
    }
}

/// `--branch`: non-empty; [`worker_branch`] checks the shape against the configured prefix.
fn non_empty(text: &str) -> Result<String, String> {
    if text.is_empty() {
        return Err("must not be empty".to_owned());
    }
    Ok(text.to_owned())
}

/// A worker branch is `<prefix><name>`: `name` of lowercase letters, digits and `-`.
fn worker_branch(text: &str, prefix: &str) -> anyhow::Result<String> {
    let name = text.strip_prefix(prefix).unwrap_or_default();
    let is_name = !name.is_empty()
        && name
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-');
    anyhow::ensure!(
        is_name,
        "--branch {text}: a worker branch is {prefix}<name>: lowercase letters, digits and -"
    );
    Ok(text.to_owned())
}

/// The config's start-time keys for a CLI command that needs one (`worker`): the daemon's config file when it exists,
/// else the defaults; a file that does not parse is an error.
fn cli_startup(claude_home: &std::path::Path) -> anyhow::Result<Startup> {
    let path = claude_home
        .join(heartbeat::protocol::STATE_DIR)
        .join(heartbeat::protocol::CONFIG_FILE);
    if !path.exists() {
        return Ok(Startup::default());
    }
    let file_config = heartbeat::edges::config::read(&path)
        .with_context(|| format!("heartbeat config {}", path.display()))?;
    Ok(file_config.startup)
}

#[derive(Debug, Args)]
struct RemoveArgs {
    /// The fleet.
    #[arg(long, value_parser = fleet_id)]
    fleet: FleetId,
    /// The member letter to remove (`A` for `T1-MA`); the operator `T1-O` has none.
    #[arg(long, value_parser = member_letter)]
    member: String,
    /// Output format.
    #[arg(long, value_enum, default_value_t = Format::Text)]
    format: Format,
}

/// What `status --fleet` asks for.
#[derive(Debug, Clone, PartialEq, Eq)]
enum FleetScope {
    /// Bare `--fleet`: the list of every fleet.
    Every,
    /// `--fleet <id>`: that fleet's members and git rows.
    Named(FleetId),
}

#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum Format {
    Text,
    Json,
}

/// What a command renders.
enum Rendered {
    Nothing,
    Document(Box<StatusDocument>, Format),
    Fleets(Vec<FleetRow>, Format),
    Subscription(Box<SubscriptionRow>, Format),
    Unsubscribed(Box<SubscriptionRow>, Format),
    Joined(Box<MemberRow>, Format),
    Done(Box<DoneAnswer>, Format),
    Land(Box<LandAnswerRow>, Format),
    Left(Box<MemberRow>, Format),
    Recorded(Box<RecordedRow>, Format),
    Schema(String),
    Interfaces(String),
    RuntimeSubscribed(Box<RuntimeSubscriptionRow>, Format),
    RuntimeUnsubscribed(Box<RuntimeSubscriptionRow>, Format),
    Runtime(Box<RuntimeDocument>, Format),
    Moved(MoveWire, Box<FindingRow>, Format),
}

fn main() -> ExitCode {
    let cli = Cli::parse();
    match run(cli).and_then(|rendered| render(&rendered)) {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => report(&err),
    }
}

fn report(err: &anyhow::Error) -> ExitCode {
    let code = err
        .downcast_ref::<heartbeat::Error>()
        .map_or(1, heartbeat::Error::exit_code);
    let _ = writeln!(anstream::stderr(), "heartbeat: {err:#}");
    ExitCode::from(u8::try_from(code).unwrap_or(1))
}

#[allow(clippy::too_many_lines)]
fn run(cli: Cli) -> anyhow::Result<Rendered> {
    if let Some(path) = &cli.log_file {
        heartbeat::log::set_file(path).with_context(|| format!("log file {}", path.display()))?;
    }
    let claude_home = match cli.claude_home {
        Some(home) => home,
        None => home_dir()
            .map(|home| home.join(".claude"))
            .context("HOME (USERPROFILE on windows) is not set; pass --claude-home")?,
    };
    match cli.command {
        Command::Schema(args) => Ok(Rendered::Schema(if args.archive {
            wire::archive_schema()?
        } else if args.runtime {
            wire::runtime_schema()?
        } else {
            wire::schema()?
        })),
        Command::Interfaces => Ok(Rendered::Interfaces(serde_json::to_string(
            &heartbeat::interfaces::answer(),
        )?)),
        Command::Runtime(args) => runtime(&client(cli.addr, claude_home), args.command),
        Command::Serve(args) => serve(cli.addr, claude_home, args),
        Command::Status(args) => {
            let filter = filter(&args)?;
            let document = client(cli.addr, claude_home).status(&filter)?;
            if args.fleet == Some(FleetScope::Every) {
                return Ok(Rendered::Fleets(document.fleets, args.format));
            }
            Ok(Rendered::Document(Box::new(document), args.format))
        }
        Command::Subscribe(args) => {
            let body = SubscribeBody {
                fleet: args.fleet.0,
                session_id: required_env("subscribe", SESSION_ENV)?,
                socket_path: PathBuf::from(required_env("subscribe", SOCKET_ENV)?),
                token: MessagingToken(required_env("subscribe", TOKEN_ENV)?),
                target: args.target,
                outside_collisions: args.outside_collisions,
            };
            let row = client(cli.addr, claude_home).subscribe(&body)?;
            Ok(Rendered::Subscription(Box::new(row), args.format))
        }
        Command::Unsubscribe(args) => {
            let body = UnsubscribeBody {
                session_id: required_env("unsubscribe", SESSION_ENV)?,
                token: MessagingToken(required_env("unsubscribe", TOKEN_ENV)?),
                finish: args.finish,
            };
            let row = client(cli.addr, claude_home).unsubscribe(&body)?;
            Ok(Rendered::Unsubscribed(Box::new(row), args.format))
        }
        Command::Join(args) => {
            let body = JoinBody {
                fleet: args.fleet.0,
                session_id: required_env("join", SESSION_ENV)?,
                token: MessagingToken(required_env("join", TOKEN_ENV)?),
            };
            let row = client(cli.addr, claude_home).join(&body)?;
            Ok(Rendered::Joined(Box::new(row), args.format))
        }
        Command::Done(args) => {
            let body = DoneBody {
                session_id: required_env("done", SESSION_ENV)?,
                token: MessagingToken(required_env("done", TOKEN_ENV)?),
            };
            let answer = client(cli.addr, claude_home).done(&body)?;
            Ok(Rendered::Done(Box::new(answer), args.format))
        }
        Command::Leave(args) => {
            let body = LeaveBody {
                session_id: required_env("leave", SESSION_ENV)?,
                token: MessagingToken(required_env("leave", TOKEN_ENV)?),
            };
            let row = client(cli.addr, claude_home).leave(&body)?;
            Ok(Rendered::Left(Box::new(row), args.format))
        }
        Command::Record(args) => {
            let body = JournalBody {
                session_id: required_env("record", SESSION_ENV)?,
                token: MessagingToken(required_env("record", TOKEN_ENV)?),
                kind: args.kind,
                members: args.members,
                subject: args.subject,
                detail: args.detail,
            };
            let row = client(cli.addr, claude_home).record(&body)?;
            Ok(Rendered::Recorded(Box::new(row), args.format))
        }
        Command::Worker(args) => {
            let startup = cli_startup(&claude_home)?;
            let body = WorkerBody {
                session_id: required_env("worker", SESSION_ENV)?,
                token: MessagingToken(required_env("worker", TOKEN_ENV)?),
                branch: worker_branch(&args.branch, startup.worker_branch_prefix())?,
                state: WorkerStateWire::from(args.state),
            };
            let row = client(cli.addr, claude_home).worker(&body)?;
            Ok(Rendered::Joined(Box::new(row), args.format))
        }
        Command::Remove(args) => {
            let body = RemoveBody {
                fleet: args.fleet.0,
                member: args.member,
            };
            let row = client(cli.addr, claude_home).remove(&body)?;
            Ok(Rendered::Left(Box::new(row), args.format))
        }
        Command::Land(args) => land(&client(cli.addr, claude_home), args),
    }
}

/// The daemon client; the CLI reads no config file for it, so the request budget is the `client_timeout` default.
fn client(addr: SocketAddr, claude_home: PathBuf) -> Client {
    Client::new(addr, claude_home, Startup::default().client_timeout())
}

/// `heartbeat land`: a request and an end prove the session with its id and token; a release needs neither and names
/// the repository by the working directory.
fn land(client: &Client, args: LandArgs) -> anyhow::Result<Rendered> {
    let row = if let Some(target) = args.target {
        client.land(&LandBody {
            session_id: required_env("land", SESSION_ENV)?,
            token: MessagingToken(required_env("land", TOKEN_ENV)?),
            target,
        })?
    } else if let Some(end) = args.end {
        client.land_end(&LandEndBody {
            session_id: required_env("land", SESSION_ENV)?,
            token: MessagingToken(required_env("land", TOKEN_ENV)?),
            outcome: end.into(),
        })?
    } else if let Some(target) = args.release {
        let cwd = std::env::current_dir().context("working directory")?;
        let cwd = dunce::canonicalize(&cwd)
            .with_context(|| format!("working directory {}", cwd.display()))?;
        client.land_release(&LandReleaseBody { cwd, target })?
    } else {
        anyhow::bail!("land needs --target, --end or --release");
    };
    Ok(Rendered::Land(Box::new(row), args.format))
}

/// `heartbeat runtime …`: subscribe and unsubscribe hand over the same variables as `subscribe`; send and stop prove
/// the subscriber with its session id and token.
fn runtime(client: &Client, command: RuntimeCommand) -> anyhow::Result<Rendered> {
    match command {
        RuntimeCommand::Subscribe(args) => {
            let body = RuntimeSubscribeBody {
                session_id: required_env("runtime subscribe", SESSION_ENV)?,
                socket_path: PathBuf::from(required_env("runtime subscribe", SOCKET_ENV)?),
                token: MessagingToken(required_env("runtime subscribe", TOKEN_ENV)?),
                mode: args.mode.into(),
                target: args.target,
            };
            let row = client.runtime_subscribe(&body)?;
            Ok(Rendered::RuntimeSubscribed(Box::new(row), args.format))
        }
        RuntimeCommand::Unsubscribe(args) => {
            let body = RuntimeUnsubscribeBody {
                session_id: required_env("runtime unsubscribe", SESSION_ENV)?,
                token: MessagingToken(required_env("runtime unsubscribe", TOKEN_ENV)?),
            };
            let row = client.runtime_unsubscribe(&body)?;
            Ok(Rendered::RuntimeUnsubscribed(Box::new(row), args.format))
        }
        RuntimeCommand::Status(args) => {
            let document = client.runtime_status()?;
            Ok(Rendered::Runtime(Box::new(document), args.format))
        }
        RuntimeCommand::Send(args) => act(client, MoveWire::Send, args),
        RuntimeCommand::Stop(args) => act(client, MoveWire::Stop, args),
    }
}

fn act(client: &Client, action: MoveWire, args: RuntimeMoveArgs) -> anyhow::Result<Rendered> {
    let command = format!("runtime {}", action.as_str());
    let body = RuntimeActBody {
        session_id: required_env(&command, SESSION_ENV)?,
        token: MessagingToken(required_env(&command, TOKEN_ENV)?),
        finding: args.finding,
        action,
    };
    let row = client.runtime_act(&body)?;
    Ok(Rendered::Moved(action, Box::new(row), args.format))
}

/// `HOME`, else `USERPROFILE`: the user's home on every OS the daemon runs on.
fn home_dir() -> Option<PathBuf> {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
}

fn serve(addr: SocketAddr, claude_home: PathBuf, args: ServeArgs) -> anyhow::Result<Rendered> {
    let config_path = args.config.unwrap_or_else(|| {
        claude_home
            .join(heartbeat::protocol::STATE_DIR)
            .join(heartbeat::protocol::CONFIG_FILE)
    });
    let file_config = heartbeat::edges::config::read(&config_path)
        .with_context(|| format!("heartbeat config {}", config_path.display()))?;
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(file_config.startup.workers())
        .enable_all()
        .build()
        .context("tokio runtime")?;
    let shutdown = file_config.startup.shutdown_timeout();
    let config = heartbeat::serve::Config {
        addr,
        claude_home,
        record: args.record,
        repos: args.repos,
        config_path,
        file_config,
    };
    let served = runtime.block_on(heartbeat::serve::run(config));
    runtime.shutdown_timeout(shutdown);
    served?;
    Ok(Rendered::Nothing)
}

fn filter(args: &StatusArgs) -> anyhow::Result<Filter> {
    let session = match args.session.as_deref() {
        Some(SELF_SESSION) => Some(SessionId(
            std::env::var(SESSION_ENV)
                .ok()
                .filter(|id| !id.is_empty())
                .ok_or(heartbeat::Error::NoSelfSession)?,
        )),
        Some(id) => Some(SessionId(id.to_owned())),
        None => None,
    };
    let repo = match &args.repo {
        Some(repo) => {
            let canonical =
                dunce::canonicalize(repo).with_context(|| format!("--repo {}", repo.display()))?;
            canonical
                .to_str()
                .ok_or_else(|| heartbeat::Error::NonUtf8Path(canonical.clone()))?;
            Some(canonical)
        }
        None => None,
    };
    let fleet = match &args.fleet {
        Some(FleetScope::Named(id)) => Some(id.clone()),
        Some(FleetScope::Every) | None => None,
    };
    let worktree = args.worktree.as_deref().map(toplevel).transpose()?;
    Ok(Filter {
        session,
        repo,
        fleet,
        worktree,
    })
}

/// `--worktree`: the canonical root of the git worktree holding `path`, relative paths against the working directory.
fn toplevel(path: &std::path::Path) -> anyhow::Result<PathBuf> {
    let what = || format!("--worktree {}", path.display());
    let start = dunce::canonicalize(path).with_context(what)?;
    let repo = gix::discover(&start).with_context(what)?;
    let root = repo
        .workdir()
        .with_context(|| format!("--worktree {}: a bare repository", path.display()))?;
    let root = dunce::canonicalize(root).with_context(what)?;
    root.to_str()
        .ok_or_else(|| heartbeat::Error::NonUtf8Path(root.clone()))?;
    Ok(root)
}

fn render(rendered: &Rendered) -> anyhow::Result<()> {
    let mut out = anstream::stdout();
    match rendered {
        Rendered::Nothing => return Ok(()),
        Rendered::Document(document, Format::Json) => {
            serde_json::to_writer_pretty(&mut out, document)?;
        }
        Rendered::Document(document, Format::Text) => write_table(&mut out, document)?,
        Rendered::Fleets(fleets, Format::Json) => serde_json::to_writer_pretty(&mut out, fleets)?,
        Rendered::Fleets(fleets, Format::Text) => write_fleets(&mut out, fleets)?,
        Rendered::Subscription(row, Format::Json) | Rendered::Unsubscribed(row, Format::Json) => {
            serde_json::to_writer_pretty(&mut out, row)?;
        }
        Rendered::Subscription(row, Format::Text) => write!(
            out,
            "subscribed {} to fleet {} ({} {})",
            row.session_id,
            row.fleet,
            row.repo.as_deref().unwrap_or("-"),
            row.target
        )?,
        Rendered::Unsubscribed(row, Format::Text) => write!(
            out,
            "unsubscribed {} from fleet {}",
            row.session_id, row.fleet
        )?,
        Rendered::Joined(row, Format::Json) | Rendered::Left(row, Format::Json) => {
            serde_json::to_writer_pretty(&mut out, row)?;
        }
        Rendered::Done(answer, Format::Json) => serde_json::to_writer_pretty(&mut out, answer)?,
        Rendered::Land(row, Format::Json) => serde_json::to_writer_pretty(&mut out, row)?,
        Rendered::Left(row, Format::Text) => write!(
            out,
            "left {}",
            row.token.as_deref().unwrap_or(&row.session_id)
        )?,
        Rendered::Joined(row, Format::Text) => {
            write!(out, "{}", row.token.as_deref().unwrap_or("-"))?;
        }
        Rendered::Done(answer, Format::Text) => write!(out, "{}", done_text(answer))?,
        Rendered::Land(row, Format::Text) => write!(out, "{}", land_text(row))?,
        Rendered::Recorded(row, Format::Json) => serde_json::to_writer_pretty(&mut out, row)?,
        Rendered::Recorded(row, Format::Text) => {
            write!(out, "recorded {} in fleet {}", row.entry.kind, row.fleet)?;
        }
        Rendered::Schema(schema) => write!(out, "{schema}")?,
        Rendered::Interfaces(interfaces) => write!(out, "{interfaces}")?,
        Rendered::RuntimeSubscribed(row, Format::Json)
        | Rendered::RuntimeUnsubscribed(row, Format::Json) => {
            serde_json::to_writer_pretty(&mut out, row)?;
        }
        Rendered::RuntimeSubscribed(row, Format::Text) => {
            write!(
                out,
                "subscribed ({}): {} watches {} ({})",
                row.mode.as_str(),
                row.session,
                row.target,
                row.target_title
            )?;
        }
        Rendered::RuntimeUnsubscribed(row, Format::Text) => {
            write!(out, "unsubscribed {}", row.session)?;
        }
        Rendered::Runtime(document, Format::Json) => {
            serde_json::to_writer_pretty(&mut out, document)?;
        }
        Rendered::Runtime(document, Format::Text) => write_runtime(&mut out, document)?,
        Rendered::Moved(_, row, Format::Json) => serde_json::to_writer_pretty(&mut out, row)?,
        Rendered::Moved(action, row, Format::Text) => write!(
            out,
            "{} {}: {}",
            action.as_str(),
            row.id,
            row.acted.as_str()
        )?,
    }
    writeln!(out)?;
    Ok(())
}

/// The text line of a done answer (merge strategy targets C3): `delivered <token>` for a joined member, `accepted
/// <target> <state>[ <detail>]` for a session accepted for its landing target, `no-target <base>` for a session
/// whose branch has none.
fn done_text(answer: &DoneAnswer) -> String {
    match answer {
        DoneAnswer::Delivered { member } => format!(
            "delivered {}",
            member.token.as_deref().unwrap_or(&member.session_id)
        ),
        DoneAnswer::Accepted { landing } => format!(
            "accepted {} {}{}",
            landing.branch,
            landing.state.as_str(),
            land_detail(landing)
        ),
        DoneAnswer::NoTarget { base } => {
            format!("no-target {}", base.as_deref().unwrap_or("unknown"))
        }
    }
}

/// The text line of a land answer (merge strategy C3): `<state> <branch>[ <detail>]`.
fn land_text(row: &LandAnswerRow) -> String {
    format!("{} {}{}", row.state.as_str(), row.branch, land_detail(row))
}

/// The words after the branch in a land line: nothing, the queue place, the end's outcome or the released session.
fn land_detail(row: &LandAnswerRow) -> String {
    match row.state {
        LandStateWire::Granted => String::new(),
        LandStateWire::Queued => format!(" {}", row.place.unwrap_or_default()),
        LandStateWire::Ended => format!(" {}", row.outcome.map_or("-", LandEndWire::as_str)),
        LandStateWire::Released => format!(" {}", row.session_id.as_deref().unwrap_or("-")),
    }
}

/// The runtime document as text: one line per watch, one line per run, one line per finding.
fn write_runtime(out: &mut impl std::io::Write, document: &RuntimeDocument) -> anyhow::Result<()> {
    if document.subscriptions.is_empty() {
        writeln!(out, "watches: none")?;
    }
    for row in &document.subscriptions {
        let last = row
            .last_digest
            .as_ref()
            .map(|digest| format!(" last {} {}", digest.id, digest.outcome.as_str()))
            .unwrap_or_default();
        writeln!(
            out,
            "watch: {} watches {} ({}) {} since {}{last}",
            row.session,
            row.target,
            row.target_title,
            row.mode.as_str(),
            row.since
        )?;
    }
    let [w0, w1, w2, w3, w4, w5] = RUNTIME_TABLE_WIDTHS;
    writeln!(
        out,
        "{:<w0$} {:<w1$} {:<w2$} {:<w3$} {:>w4$} {:>w5$} SESSION / REASON",
        "RUN", "STATE", "SKILL", "VERSION", "FINDINGS", "FIRED"
    )?;
    for run in &document.runs {
        let short = run.session.get(..8).unwrap_or(&run.session);
        let reason = if run.reason.is_empty() {
            String::new()
        } else {
            format!(" — {}", run.reason)
        };
        writeln!(
            out,
            "{:<w0$} {:<w1$} {:<w2$} {:<w3$} {:>w4$} {:>w5$} {short}{reason}",
            format!("r{}", run.id),
            run.state.as_str(),
            run.skill,
            run.version,
            run.findings,
            run.anchors.len(),
        )?;
    }
    for finding in &document.findings {
        let moves: Vec<&str> = finding.moves.iter().map(|action| action.as_str()).collect();
        writeln!(
            out,
            "finding {} r{} {} {} {} moves [{}]: {}",
            finding.id,
            finding.run,
            finding.kind.as_str(),
            finding.entry,
            finding.acted.as_str(),
            moves.join(","),
            finding.reason,
        )?;
    }
    Ok(())
}

/// Human table: one line per session (`*` marks `since_restart`), then one line per degraded sensor. SKILL and TITLE
/// are as wide as their longest value; SINCE shows whole seconds so every row has the same width (JSON keeps full
/// precision).
fn write_table(out: &mut impl std::io::Write, document: &StatusDocument) -> anyhow::Result<()> {
    let skill_width = column_width("SKILL", document.sessions.iter().map(skill));
    let title_width = column_width("TITLE", document.sessions.iter().map(title));
    let [w0, w1, w2] = STATUS_TABLE_WIDTHS;
    writeln!(
        out,
        "{:<w0$} {:<w1$} {:<w2$} {:<skill_width$} {:<title_width$} CWD",
        "SESSION", "STATE", "SINCE", "SKILL", "TITLE"
    )?;
    for row in &document.sessions {
        let short = row.session_id.get(..8).unwrap_or(&row.session_id);
        let marker = if row.since_restart { "*" } else { "" };
        writeln!(
            out,
            "{:<w0$} {:<w1$} {:<w2$} {:<skill_width$} {:<title_width$} {}",
            short,
            format!("{}{marker}", row.state.as_str()),
            whole_seconds(&row.state_since),
            skill(row),
            title(row),
            row.cwd.as_deref().unwrap_or("-"),
        )?;
    }
    for sensor in document
        .sensors
        .iter()
        .filter(|sensor| sensor.state == SensorStateWire::Degraded)
    {
        writeln!(
            out,
            "degraded: {} since {}: {}",
            sensor.name.as_str(),
            sensor.since,
            sensor.detail.as_deref().unwrap_or("-"),
        )?;
    }
    for row in document
        .fleets
        .iter()
        .flat_map(|fleet| &fleet.subscriptions)
    {
        let last = row
            .last_digest
            .as_ref()
            .map(|digest| format!(" last {} {:?}", digest.id, digest.outcome))
            .unwrap_or_default();
        let detail = row
            .detail
            .as_deref()
            .map(|detail| format!(" ({detail})"))
            .unwrap_or_default();
        writeln!(
            out,
            "subscription: {} fleet {} target {} {:?}{detail} pending {}{last}",
            row.session_id, row.fleet, row.target, row.state, row.pending
        )?;
    }
    write_members(out, &document.members)?;
    Ok(())
}

/// The member block of a fleet-filtered status: token (else the session id), why a member, delivered, branch.
fn write_members(out: &mut impl std::io::Write, members: &[MemberRow]) -> anyhow::Result<()> {
    if members.is_empty() {
        return Ok(());
    }
    let [w0, w1, w2] = MEMBER_TABLE_WIDTHS;
    writeln!(
        out,
        "{:<w0$} {:<w1$} {:<w2$} BRANCH",
        "TOKEN", "VIA", "DELIVERED"
    )?;
    for member in members {
        let short = member.session_id.get(..8).unwrap_or(&member.session_id);
        writeln!(
            out,
            "{:<w0$} {:<w1$} {:<w2$} {}",
            member.token.as_deref().unwrap_or(short),
            member.via.as_str(),
            if member.delivered { "yes" } else { "no" },
            member.branch.as_deref().unwrap_or("-"),
        )?;
    }
    Ok(())
}

/// One line per fleet: id, run (first 8 of the UUID), target, target state, member count, subscriber state.
fn write_fleets(out: &mut impl std::io::Write, fleets: &[FleetRow]) -> anyhow::Result<()> {
    let [w0, w1, w2, w3, w4] = FLEET_TABLE_WIDTHS;
    writeln!(
        out,
        "{:<w0$} {:<w1$} {:<w2$} {:<w3$} {:>w4$} SUBSCRIBER",
        "FLEET", "RUN", "TARGET", "STATE", "MEMBERS"
    )?;
    for fleet in fleets {
        let run = fleet
            .instance
            .as_deref()
            .and_then(|id| id.get(..8))
            .unwrap_or("-");
        let subscriber = fleet
            .subscriptions
            .last()
            .map_or("-", |row| row.state.as_str());
        writeln!(
            out,
            "{:<w0$} {:<w1$} {:<w2$} {:<w3$} {:>w4$} {subscriber}",
            fleet.id,
            run,
            fleet.target.as_deref().unwrap_or("-"),
            fleet.target_state.map_or("-", |state| state.as_str()),
            fleet.members,
        )?;
    }
    Ok(())
}

/// `--fleet`: a `FleetId` of at most the default `max_id_len` (the CLI reads no config file for it).
fn fleet_id(value: &str) -> Result<FleetId, String> {
    let max_len = Caps::default().max_id_len();
    FleetId::parse(value, max_len)
        .ok_or_else(|| format!("must be 1-{max_len} characters of {FLEET_ID_CHARSET}"))
}

/// `--fleet`: `*` (what a bare flag stands for) lists every fleet, anything else must be a `FleetId`.
fn fleet_scope(value: &str) -> Result<FleetScope, String> {
    if value == EVERY_FLEET {
        return Ok(FleetScope::Every);
    }
    fleet_id(value).map(FleetScope::Named)
}

/// `--worktree`: any non-empty path; [`toplevel`] resolves it.
fn worktree_path(value: &str) -> Result<PathBuf, String> {
    if value.is_empty() {
        return Err("must be a path; `.` is the working directory's worktree".to_owned());
    }
    Ok(PathBuf::from(value))
}

/// `A` … `ZZZZ`: an upper-case member letter.
fn member_letter(text: &str) -> Result<String, String> {
    let is_letter = !text.is_empty()
        && text.len() <= MAX_MEMBER_LETTERS
        && text.bytes().all(|byte| byte.is_ascii_uppercase());
    if is_letter {
        return Ok(text.to_owned());
    }
    Err(format!(
        "a member letter: 1-{MAX_MEMBER_LETTERS} upper-case ASCII letters (A for T1-MA)"
    ))
}

/// `--target`: the same branch-name check the daemon's HTTP edge applies, at the default `max_field`.
fn target_branch(value: &str) -> Result<String, String> {
    SubscribeRequest::check_target(value, Caps::default().max_field()).map(|()| value.to_owned())
}

/// A non-empty environment variable Claude Code sets in its Bash tool.
fn required_env(command: &str, name: &str) -> anyhow::Result<String> {
    std::env::var(name)
        .ok()
        .filter(|value| !value.is_empty())
        .with_context(|| format!("{command} needs {name} (run it from a Claude Code Bash tool)"))
}

/// The SKILL cell.
fn skill(row: &SessionRow) -> &str {
    row.active_skill.as_deref().unwrap_or("-")
}

/// The TITLE cell.
fn title(row: &SessionRow) -> &str {
    row.title.as_deref().unwrap_or("-")
}

/// The width of a column: its longest cell or its header, in chars.
fn column_width<'a>(header: &str, cells: impl Iterator<Item = &'a str>) -> usize {
    cells
        .map(|cell| cell.chars().count())
        .fold(header.chars().count(), usize::max)
}

/// An RFC 3339 timestamp at whole seconds (`2026-09-27T20:49:14Z`); unparseable input is shown as is.
fn whole_seconds(at: &str) -> String {
    at.parse::<jiff::Timestamp>()
        .map_or_else(|_| at.to_owned(), |timestamp| format!("{timestamp:.0}"))
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    fn status_fleet(argv: &[&str]) -> Result<Option<FleetScope>, clap::Error> {
        let cli = Cli::try_parse_from(argv)?;
        match cli.command {
            Command::Status(status) => Ok(status.fleet),
            _ => unreachable!("status argv"),
        }
    }

    #[test]
    fn fleet_flag() {
        assert_eq!(
            status_fleet(&["heartbeat", "status", "--fleet"]).expect("bare"),
            Some(FleetScope::Every),
            "bare --fleet lists fleets"
        );
        assert_eq!(
            status_fleet(&["heartbeat", "status", "--fleet", "T1"]).expect("value"),
            Some(FleetScope::Named(FleetId("T1".to_owned()))),
        );
        assert_eq!(
            status_fleet(&["heartbeat", "status"]).expect("absent"),
            None
        );
        assert!(
            status_fleet(&["heartbeat", "status", "--fleet="]).is_err(),
            "empty value rejected"
        );
    }

    #[test]
    fn worktree_flag() {
        let parsed = |argv: &[&str]| {
            Cli::try_parse_from(argv).map(|cli| match cli.command {
                Command::Status(status) => status.worktree,
                _ => unreachable!("status argv"),
            })
        };
        assert_eq!(
            parsed(&["heartbeat", "status", "--worktree", "."]).expect("dot"),
            Some(PathBuf::from("."))
        );
        assert_eq!(parsed(&["heartbeat", "status"]).expect("absent"), None);
        assert!(
            parsed(&["heartbeat", "status", "--worktree="]).is_err(),
            "empty value rejected"
        );
    }

    #[test]
    fn remove_member_letter() {
        let parsed = |member: &str| {
            Cli::try_parse_from(["heartbeat", "remove", "--fleet", "T1", "--member", member])
        };
        assert!(parsed("B").is_ok());
        assert!(parsed("a").is_err(), "lower case");
        assert!(parsed("ABCDE").is_err(), "too long");
    }

    #[test]
    fn worker_args() {
        let parsed = |branch: &str, state: &str| -> Option<(String, WorkerStateArg)> {
            let cli =
                Cli::try_parse_from(["heartbeat", "worker", "--branch", branch, "--state", state])
                    .ok()?;
            match cli.command {
                Command::Worker(args) => {
                    let prefix = Startup::default();
                    let branch = worker_branch(&args.branch, prefix.worker_branch_prefix()).ok()?;
                    Some((branch, args.state))
                }
                _ => unreachable!("worker argv"),
            }
        };
        let cases = [
            (
                "worker-branch-accepted",
                "claude/t1-build-2",
                "running",
                Some(WorkerStateArg::Running),
            ),
            ("worker-branch-without-prefix", "t1-build", "running", None),
            (
                "worker-branch-other-prefix",
                "feature/t1-build",
                "running",
                None,
            ),
            (
                "worker-branch-uppercase",
                "claude/T1-build",
                "running",
                None,
            ),
            ("worker-branch-empty-name", "claude/", "running", None),
            (
                "worker-state-values-delivered",
                "claude/t1-a",
                "delivered",
                Some(WorkerStateArg::Delivered),
            ),
            ("worker-state-values-unknown", "claude/t1-a", "failed", None),
        ];
        for (label, branch, state, want) in cases {
            let got = parsed(branch, state);
            assert_eq!(got.as_ref().map(|(_, state)| *state), want, "{label}");
            if let Some((parsed_branch, _)) = got {
                assert_eq!(parsed_branch, branch, "{label}");
            }
        }
    }

    #[test]
    fn write_fleets_rows() {
        let row = |id: &str, instance: Option<&str>, target: Option<&str>| FleetRow {
            id: id.to_owned(),
            instance: instance.map(str::to_owned),
            started_at: None,
            repo: None,
            target: target.map(str::to_owned),
            merge_branch: None,
            synced: None,
            target_state: target.map(|_| wire::TargetStateWire::Open),
            members: 2,
            foreign: Vec::new(),
            subscriptions: Vec::new(),
            journal: Vec::new(),
        };
        let fleets = [
            row(
                "T1",
                Some("01a0e899-5ba7-7431-8000-000000000000"),
                Some("main"),
            ),
            row("T9", None, None),
        ];
        let mut out = Vec::new();
        write_fleets(&mut out, &fleets).expect("write");
        let text = String::from_utf8(out).expect("utf8");
        let lines: Vec<&str> = text.lines().collect();
        assert_eq!(lines.len(), 3, "{text}");
        assert!(lines[1].starts_with("T1       01a0e899  main"), "{text}");
        assert!(lines[1].contains(" open "), "{text}");
        assert!(lines[2].starts_with("T9       -         -"), "{text}");
    }

    #[test]
    fn land_flags() {
        let parsed = |args: &[&str]| {
            let argv = std::iter::once("heartbeat")
                .chain(std::iter::once("land"))
                .chain(args.iter().copied());
            Cli::try_parse_from(argv)
        };
        let cases: [(&str, &[&str], bool); 8] = [
            ("target", &["--target", "develop"], true),
            ("end landed", &["--end", "landed"], true),
            ("end abandoned", &["--end", "abandoned"], true),
            ("release", &["--release", "develop"], true),
            ("none of them", &[], false),
            (
                "two of them",
                &["--target", "develop", "--end", "landed"],
                false,
            ),
            ("unknown end word", &["--end", "merged"], false),
            ("invalid branch name", &["--target", "a..b"], false),
        ];
        for (label, args, ok) in cases {
            assert_eq!(parsed(args).is_ok(), ok, "{label}");
        }
    }

    #[test]
    fn land_lines() {
        let row = |state, place, outcome, session: Option<&str>| LandAnswerRow {
            state,
            branch: "develop".to_owned(),
            place,
            outcome,
            session_id: session.map(str::to_owned),
        };
        let cases = [
            (
                row(LandStateWire::Granted, None, None, None),
                "granted develop",
            ),
            (
                row(LandStateWire::Queued, Some(2), None, None),
                "queued develop 2",
            ),
            (
                row(LandStateWire::Ended, None, Some(LandEndWire::Landed), None),
                "ended develop landed",
            ),
            (
                row(LandStateWire::Released, None, None, Some("s1")),
                "released develop s1",
            ),
        ];
        for (answer, want) in cases {
            assert_eq!(land_text(&answer), want);
        }
        let accepted = [
            (
                row(LandStateWire::Granted, None, None, None),
                "accepted develop granted",
            ),
            (
                row(LandStateWire::Queued, Some(1), None, None),
                "accepted develop queued 1",
            ),
        ];
        for (landing, want) in accepted {
            assert_eq!(done_text(&DoneAnswer::Accepted { landing }), want);
        }
        let feature = LandAnswerRow {
            branch: "feature/x".to_owned(),
            ..row(LandStateWire::Granted, None, None, None)
        };
        assert_eq!(
            done_text(&DoneAnswer::Accepted { landing: feature }),
            "accepted feature/x granted"
        );
        let no_target = [
            (Some("main"), "no-target main"),
            (None, "no-target unknown"),
        ];
        for (base, want) in no_target {
            let answer = DoneAnswer::NoTarget {
                base: base.map(str::to_owned),
            };
            assert_eq!(done_text(&answer), want);
        }
    }
}
