interfaces: [{
	id:      "acdsl-check"
	version: 1
	kind:    "program"
	scope:   "repo"
	summary: "Runs the ACDSL gates, or one rule against a registry file, over the repo."
	example: "bin/acdsl"
	invocation: {cwd: "repo-root", args: "check [-rule <id>] [-registry <file>]", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "clean", "1": "violations", other: "usage or error"}, stdout: "one line per violation"}
	validate: {resolves: true}
}, {
	id:      "acdsl-fixtures"
	version: 1
	kind:    "program"
	scope:   "repo"
	summary: "Runs every ACDSL rule's fixtures and reports the ones that do not hold."
	example: "bin/acdsl"
	invocation: {cwd: "repo-root", args: "fixtures", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "all fixtures hold", other: "a fixture failed or usage"}, stdout: "one line per failing fixture"}
	validate: {resolves: true}
}, {
	id:      "agentwt-forward"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Fast-forwards a local branch to a commit that holds its tip."
	invocation: {cwd: "repo-root", args: "forward --onto <branch> <commit> [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "forwarded or unchanged", "3": "skipped or unknown ref", other: "error"}, stdout: "forward-v1 document"}
	validate: {resolves: true}
}, {
	id:      "agentwt-probe"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Answers per commit whether a pick onto a branch would apply, conflict or be skipped; writes nothing."
	invocation: {cwd: "repo-root", args: "probe --onto <branch> <commit>... [--format json]", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "answered", other: "error"}, stdout: "probe-v1 document"}
	validate: {resolves: true}
}, {
	id:      "agentwt-remove"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Removes agent worktrees and optionally their branches when the safety gate allows."
	invocation: {cwd: "repo-root", args: "remove [--dry-run] [--delete-branch] [--force] <branch>...", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "removed or skipped lines printed", "3": "dry run would skip, or an unknown or non-agent branch", other: "error"}, stdout: "one line per removed or skipped row"}
	validate: {resolves: true}
}, {
	id:      "agentwt-status"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the status document of every agent branch and worktree of the repo."
	invocation: {cwd: "repo-root", args: "status [--kind <kind>] [--format json]", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "document printed", other: "error"}, stdout: "status-v1 document"}
	validate: {resolves: true}
}, {
	id:      "agentwt-unplaced"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Lists the commits one branch still owes one target, oldest first."
	invocation: {cwd: "repo-root", args: "unplaced --onto <target> <branch> [--format json]", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "listed", "3": "unknown branch", other: "error"}, stdout: "unplaced-v1 document"}
	validate: {resolves: true}
}, {
	id:      "base"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the review base commit and its source."
	invocation: {cwd: "repo-root", args: "base --head <commit> --mainline <branch> [--base <ref>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "usage or error"}, stdout: "{commit, source}"}
	validate: {resolves: true}
}, {
	id:      "build"
	version: 1
	kind:    "command"
	scope:   "repo"
	summary: "Compiles the repo."
	example: "go build ./..."
	invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "20m"}
	result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "buildtest"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Builds and tests one commit in a disposable worktree with the repo's registered build and test commands."
	invocation: {cwd: "repo-root", args: "buildtest --repo <repo> --head <commit> --root <dir> --branch <branch> --salvage <dir> [--overlay <patch>] [--skip]", stdin: "none", timeout: "45m"}
	result: {exit: {"0": "result printed, red steps included", "2": "usage, provisioning error or no registered commands", other: "error"}, stdout: "{branches, build, salvage, tests}"}
	validate: {resolves: true}
}, {
	id:      "capture-ui"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Writes a PNG of a running UI and prints its absolute path."
	invocation: {cwd: "repo-root", args: "capture-ui <url|sim> <out.png> [--size WxH] [--wait-ms N]", stdin: "none", timeout: "2m"}
	result: {exit: {"0": "written", "64": "usage", "69": "no capture tool", "70": "nothing written", other: "error"}, stdout: "the PNG path"}
	validate: {resolves: true}
}, {
	id:      "check"
	version: 1
	kind:    "command"
	scope:   "repo"
	summary: "Runs everything required before a commit: build, tests, lint."
	example: "go vet ./..."
	invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "30m"}
	result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "clean"
	version: 1
	kind:    "command"
	scope:   "repo"
	summary: "Drops what a session leaves in its worktree beyond git's reach before the worktree removal: build caches, sandbox homes, read-only trees."
	example: "make clean"
	invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "clean", other: "fail"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "cleanup"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Salvages and removes every worktree and branch of one review run."
	invocation: {cwd: "repo-root", args: "cleanup --repo <repo> --prefix <prefix> --head <commit> --salvage <dir> [--run <id>]... [--root <dir>] [--overlay <patch>]", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "cleaned", other: "usage or error"}, stdout: "cleanup report JSON"}
	validate: {resolves: true}
}, {
	id:      "commit-skill"
	version: 1
	kind:    "skill"
	scope:   "repo"
	summary: "The skill an implementation hands its commit to once the check command is green."
	example: "package-commit"
	invocation: {cwd: "none", args: "none", stdin: "none", timeout: "none"}
	result: {exit: {"0": "n/a", other: "n/a"}, stdout: "n/a"}
	validate: {}
}, {
	id:      "demo-close"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Writes the heartbeat-demo closing marker."
	invocation: {cwd: "repo-root", args: "demo-close --dir <dir>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "written", other: "usage or error"}, stdout: "the marker path"}
	validate: {resolves: true}
}, {
	id:      "demo-value"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the heartbeat-demo producer value for a seed."
	invocation: {cwd: "repo-root", args: "demo-value --seed <s> [--salt <x>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "usage"}, stdout: "the value"}
	validate: {resolves: true}
}, {
	id:      "entries"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Lists the context entries carrying a marker tag."
	invocation: {cwd: "repo-root", args: "entries --marker <tag> [--dir <context dir>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "listed", other: "usage, no context dir or error"}, stdout: "one id and statement per line"}
	validate: {resolves: true}
}, {
	id:      "generate"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Renders context/context.json from the context tree."
	invocation: {cwd: "repo-root", args: "generate", stdin: "none", timeout: "2m"}
	result: {exit: {"0": "written", other: "violations or error"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-done"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Reports the calling session's work finished: delivered to its fleet or accepted for its landing target."
	invocation: {cwd: "repo-root", args: "done [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "accepted", other: "refused or daemon down"}, stdout: "the answer"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-join"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Joins the calling session to a fleet and prints its member token."
	invocation: {cwd: "repo-root", args: "join --fleet <fleet> [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "joined", other: "refused or daemon down"}, stdout: "the member token"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-land"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Requests, ends or releases the landing turn of a landing target."
	invocation: {cwd: "repo-root", args: "land (--target <branch> | --end landed|abandoned | --release <branch>) [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "answered", other: "refused or daemon down"}, stdout: "the landing answer"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-leave"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Leaves the calling joined session's fleet."
	invocation: {cwd: "repo-root", args: "leave [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "left", other: "not a member or daemon down"}, stdout: "the member row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-record"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Records an orchestrator act in its fleet's journal."
	invocation: {cwd: "repo-root", args: "record --kind <kind> --subject <line> [--member <letter>]... [--detail <text>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "recorded", other: "not the subscriber or daemon down"}, stdout: "the recorded row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-remove"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Removes a lettered member from a fleet."
	invocation: {cwd: "repo-root", args: "remove --fleet <fleet> --member <letter>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "removed", other: "unknown member or daemon down"}, stdout: "the member row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-runtime"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Watches skill runs through the runtime checker: subscribe, status, send, stop, unsubscribe."
	invocation: {cwd: "repo-root", args: "runtime <subscribe|status|send|stop|unsubscribe> [args] [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "answered", other: "refused or daemon down"}, stdout: "runtime-v1 document or the move's row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-status"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the heartbeat status document, scoped by fleet, session, repo or worktree."
	invocation: {cwd: "repo-root", args: "status [--fleet <id>] [--session <id>] [--worktree <path>] [--format json]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "daemon down"}, stdout: "status-v1 document"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-subscribe"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Subscribes the calling session to its fleet's digests as the orchestrator."
	invocation: {cwd: "repo-root", args: "subscribe --fleet <fleet> --target <branch> [--outside-collisions]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "subscribed", other: "refused or daemon down"}, stdout: "the subscription row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-unsubscribe"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Ends the calling session's fleet subscription."
	invocation: {cwd: "repo-root", args: "unsubscribe [--finish]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "unsubscribed", other: "not subscribed or daemon down"}, stdout: "the subscription row"}
	validate: {resolves: true}
}, {
	id:      "heartbeat-worker"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Registers a worker branch in the fleet or reports its work delivered."
	invocation: {cwd: "repo-root", args: "worker --branch claude/<name> --state running|delivered", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "registered", other: "not the subscriber or daemon down"}, stdout: "the worker's member token"}
	validate: {resolves: true}
}, {
	id:      "intake"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the review-config analysis of a range."
	invocation: {cwd: "repo-root", args: "intake --base <commit> --head <commit> [--paths <csv>] [--exclude <csv>] [--gate]", stdin: "none", timeout: "2m"}
	result: {exit: {"0": "printed", "2": "empty diff or usage", other: "error"}, stdout: "the analysis and the suggested invocation"}
	validate: {resolves: true}
}, {
	id:      "lanes"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Writes one review round's lane manifests and prints the lane plan."
	invocation: {cwd: "repo-root", args: "lanes --round-dir <dir> --directions <csv> --base <commit> --head <commit> [flags]", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "plan printed", "1": "a direction has no walk entries", "2": "empty diff, no context dir or usage", other: "error"}, stdout: "{idFloor, knownPath, lanes, missingContext, skipped}"}
	validate: {resolves: true}
}, {
	id:      "ledger"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Updates the known and coverage ledgers from a review round."
	invocation: {cwd: "repo-root", args: "ledger (--known <file> --round-dir <dir> --coverage <file> | --known <file> --round <n> --fix-state applied|fixed --ids <csv> | --reject <id> ... | --round-dir <dir> --void)", stdin: "none", timeout: "2m"}
	result: {exit: {"0": "updated", other: "usage, schema or error"}, stdout: "the autofix split, ignored ids and coverage gaps"}
	validate: {resolves: true}
}, {
	id:      "lint"
	version: 1
	kind:    "program"
	scope:   "repo"
	summary: "The repo's lint gate: runs the ACDSL gates, or one rule against a registry file, over the repo — the program a review's probes replay through."
	example: "bin/acdsl"
	invocation: {cwd: "repo-root", args: "check [-rule <id>] [-registry <file>]", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "clean", "1": "violations", other: "usage or error"}, stdout: "one line per violation"}
	validate: {resolves: true}
}, {
	id:      "merge-branch"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Creates or cleans up a merge-resolve work branch."
	invocation: {cwd: "repo-root", args: "merge-branch create <ours> <theirs> | cleanup <ours>", stdin: "none", timeout: "2m"}
	result: {exit: {"0": "done", other: "usage or error"}, stdout: "the work branch and its state"}
	validate: {resolves: true}
}, {
	id:      "pairs"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints a review range's test pairing."
	invocation: {cwd: "repo-root", args: "pairs --base <commit> --head <commit> [--exclude <csv>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "usage or error"}, stdout: "{partners, tests}"}
	validate: {resolves: true}
}, {
	id:      "permission-check"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Runs targets through the tool-policy gate and answers admitted or denied per target, with the candidate rule."
	invocation: {cwd: "repo-root", args: "check [--skill <leaf>]... '<target>'...", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "report printed", other: "usage or error"}, stdout: "N. <target> → <verdict> per line"}
	validate: {resolves: true}
}, {
	id:      "permission-freeze"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Freezes a plan dir's permissions.json in the gate's ledger."
	invocation: {cwd: "repo-root", args: "freeze <plan dir>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "frozen", other: "refused"}, stdout: "the FROZEN line with the rules"}
	validate: {resolves: true}
}, {
	id:      "plan-claims"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints a plan's refute ledger as JSON, one claim per Assumptions row."
	invocation: {cwd: "repo-root", args: "plan-claims <plan file>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "ledger printed", other: "usage or error"}, stdout: "the claims document"}
	validate: {resolves: true}
}, {
	id:      "plan-display"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Writes a plan's display copy into the session plan file."
	invocation: {cwd: "repo-root", args: "plan-display <plan file> <display file>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "written", other: "usage or error"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "plan-sections"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Checks a plan file's sections and prints one file:line: message per violation."
	invocation: {cwd: "repo-root", args: "plan-sections <plan file>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "clean", other: "violations or usage"}, stdout: "violations, then the ok line with the adds totals"}
	validate: {resolves: true}
}, {
	id:      "plans-archive"
	version: 1
	kind:    "path"
	scope:   "repo"
	summary: "The plans-archive repo root holding every repo's done plan dirs and stores."
	example: "~/GolandProjects/agent-plans"
	invocation: {cwd: "none", args: "none", stdin: "none", timeout: "none"}
	result: {exit: {"0": "n/a", other: "n/a"}, stdout: "n/a"}
	validate: {exists: "dir"}
}, {
	id:      "present"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints a review round's gate text or the plan's review section."
	invocation: {cwd: "repo-root", args: "present --review <review.json> [--ignore-level <level>] [--section <heading>]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "usage or error"}, stdout: "the gate text or section"}
	validate: {resolves: true}
}, {
	id:      "probe-replay"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Replays the persisted probes of review rounds against the post-fix head."
	invocation: {cwd: "repo-root", args: "probe-replay --repo <repo> --head <commit> --root <dir> --branch <branch> --rounds <csv> [--acdsl <program>]", stdin: "none", timeout: "45m"}
	result: {exit: {"0": "replayed, hits listed", "2": "usage, build failure or no registered commands", other: "error"}, stdout: "{hits, probes, salvage}"}
	validate: {resolves: true}
}, {
	id:      "project-plan"
	version: 1
	kind:    "program"
	scope:   "repo"
	summary: "Prints the ACDSL rules that would govern a path."
	example: "bin/acdsl"
	invocation: {cwd: "repo-root", args: "project -plan <path>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "usage or error"}, stdout: "one rule per line"}
	validate: {resolves: true}
}, {
	id:      "project-strip"
	version: 1
	kind:    "program"
	scope:   "repo"
	summary: "Removes every ACDSL projection block from the working copy."
	example: "bin/acdsl"
	invocation: {cwd: "repo-root", args: "project -strip", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "stripped", other: "error"}, stdout: "the stripped file count"}
	validate: {resolves: true}
}, {
	id:      "provision"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Creates one review worktree for a railroad-review worker."
	invocation: {cwd: "repo-root", args: "provision --repo <repo> --root <dir> --head <commit> --branch <branch> [--overlay <patch>] [--prebuilt <csv>] <slug>", stdin: "none", timeout: "5m"}
	result: {exit: {"0": "provisioned", other: "usage or error"}, stdout: "the worktree path"}
	validate: {resolves: true}
}, {
	id:      "prune"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Reduces a finished review's artifact dir to its keep list."
	invocation: {cwd: "repo-root", args: "prune --artifacts <dir> [--verbose]", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "pruned", other: "usage or error"}, stdout: "{kept, removed}"}
	validate: {resolves: true}
}, {
	id:      "reset-round"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Salvages one failed review round before its re-dispatch."
	invocation: {cwd: "repo-root", args: "reset-round --repo <repo> --prefix <prefix> --head <commit> --salvage <dir> --round-dir <dir> --round <r> [flags]", stdin: "none", timeout: "10m"}
	result: {exit: {"0": "reset", other: "usage or error"}, stdout: "the reset report"}
	validate: {resolves: true}
}, {
	id:      "routine-publish-style"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Publishes a staged style profile to the live profile path; run by the operator."
	invocation: {cwd: "repo-root", args: "publish-style <staged file>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "published", other: "usage or error"}, stdout: "the published path"}
	validate: {resolves: true}
}, {
	id:      "routine-worker"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Runs one fleet worker cell for an orchestrator session."
	invocation: {cwd: "repo-root", args: "worker -name <name> -skill <leaf> (-base <ref> | -resume <session id>) [-args <text>] [flags]", stdin: "none", timeout: "4h"}
	result: {exit: {"0": "cell finished", other: "usage, config or start failure"}, stdout: "the cell report"}
	validate: {resolves: true}
}, {
	id:      "snapshot"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Commits the working tree on top of HEAD through a temporary index."
	invocation: {cwd: "repo-root", args: "snapshot --scope wip|all", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "snapshot printed", other: "usage or error"}, stdout: "{snapshot, differs}"}
	validate: {resolves: true}
}, {
	id:      "store-root"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Prints the store root of the repo in the plans archive."
	invocation: {cwd: "repo-root", args: "store-root", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed", other: "error"}, stdout: "the absolute store root"}
	validate: {resolves: true}
}, {
	id:      "task-check"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Validates smine change task files against the task schema."
	invocation: {cwd: "repo-root", args: "task-check <task file>...", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "valid", other: "violations or usage"}, stdout: "one line per violation"}
	validate: {resolves: true}
}, {
	id:      "test"
	version: 1
	kind:    "command"
	scope:   "repo"
	summary: "Runs the repo's full test suite."
	example: "go test ./..."
	invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "20m"}
	result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}
	validate: {resolves: true}
}, {
	id:      "title-set"
	version: 1
	kind:    "program"
	scope:   "install"
	summary: "Confirms the session-title state a gate step sets for the turn end."
	invocation: {cwd: "repo-root", args: "title set <TOKEN>", stdin: "none", timeout: "1m"}
	result: {exit: {"0": "printed title: set <TOKEN>", "2": "unknown token", other: "error"}, stdout: "title: set <TOKEN>"}
	validate: {resolves: true}
}]
providers: [{
	id:        "acdsl"
	program:   ""
	implements: ["acdsl-check@1", "acdsl-fixtures@1", "lint@1", "project-plan@1", "project-strip@1"]
	probe: ["interfaces"]
}, {
	id:        "smine-agentwt"
	program:   "smine-agentwt"
	component: "agentwt"
	implements: ["agentwt-forward@1", "agentwt-probe@1", "agentwt-remove@1", "agentwt-status@1", "agentwt-unplaced@1"]
	probe: ["interfaces"]
}, {
	id:        "smine-heartbeat"
	program:   "smine-heartbeat"
	component: "heartbeat"
	implements: ["heartbeat-done@1", "heartbeat-join@1", "heartbeat-land@1", "heartbeat-leave@1", "heartbeat-record@1", "heartbeat-remove@1", "heartbeat-runtime@1", "heartbeat-status@1", "heartbeat-subscribe@1", "heartbeat-unsubscribe@1", "heartbeat-worker@1"]
	probe: ["interfaces"]
}, {
	id:        "smine-routine"
	program:   "smine-routine"
	component: "routines"
	implements: ["routine-publish-style@1", "routine-worker@1"]
	probe: ["interfaces"]
}, {
	id:        "smine-rules"
	program:   "smine-rules"
	component: "config-server"
	implements: ["base@1", "buildtest@1", "capture-ui@1", "cleanup@1", "demo-close@1", "demo-value@1", "entries@1", "generate@1", "intake@1", "lanes@1", "ledger@1", "merge-branch@1", "pairs@1", "plan-claims@1", "plan-display@1", "plan-sections@1", "present@1", "probe-replay@1", "provision@1", "prune@1", "reset-round@1", "snapshot@1", "store-root@1", "task-check@1", "title-set@1"]
	probe: ["interfaces"]
}, {
	id:        "toolpolicy"
	program:   "toolpolicy"
	binary:    "~/.claude/hooks/bin/toolpolicy"
	component: "gate"
	implements: ["permission-check@1", "permission-freeze@1"]
	probe: ["interfaces"]
}]
