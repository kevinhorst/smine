// Command sessions selects, checks and marks smine session records.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2

	commandArgIndex   = 1
	contextDirArgs    = 2
	contextDirEnv     = "AGENT_CONTEXT_DIR_DEFAULT"
	defaultContextDir = "docs"
	defaultRoot       = "sessions"
	defaultWindowDays = peek.MiningWindowDays
	pendingTimeout    = 2 * time.Minute
	schemaPath        = "skills/smine/smine-batch/reference/session.schema.json"
)

// unsynthesizedRecord is one line of the unsynthesized listing.
type unsynthesizedRecord struct {
	Id string `json:"id"`

	LastActive string `json:"lastActive"`
	Path       string `json:"path"`
	Repo       string `json:"repo"`
	Week       string `json:"week"`
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) <= commandArgIndex {
		fmt.Fprintln(os.Stderr, "usage: sessions <check|context-record|mark|pending|unmark|unsynthesized> [flags]")
		return exitError
	}

	switch os.Args[1] {
	case "check":
		return runCheck(os.Args[2:])
	case "context-record":
		return runContextRecord(os.Args[2:])
	case "mark":
		return runMark(os.Args[2:])
	case "pending":
		return runPending(os.Args[2:])
	case "unmark":
		return runUnmark(os.Args[2:])
	case "unsynthesized":
		return runUnsynthesized(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "sessions: unknown command: %s\n", os.Args[1])
		return exitError
	}
}

func runCheck(args []string) int {
	transcriptPath, paths, err := parseCheckArgs(args)
	isContextMisused := transcriptPath != "" && len(paths) != 1
	isUsageError := len(paths) == 0 || isContextMisused
	if err != nil || isUsageError {
		fmt.Fprintln(os.Stderr, "usage: sessions check [--context <transcript.jsonl>] <record.json>...")
		return exitError
	}

	var usedContext *sessions.ContextRecord
	if transcriptPath != "" {
		record, err := contextrecord.Extract(transcriptPath, contextDir())
		if err != nil {
			fmt.Fprintln(os.Stderr, "sessions:", err)
			return exitError
		}

		usedContext = sessions.NewContextRecord(record)
	}

	status := exitClean
	for _, path := range paths {
		violations, err := sessions.CheckRecord(path, schemaPath, usedContext)
		status = max(status, reportViolations(err, path, violations))
	}
	return status
}

func runContextRecord(args []string) int {
	if len(args) == 0 || len(args) > contextDirArgs {
		fmt.Fprintln(os.Stderr, "usage: sessions context-record <transcript.jsonl> [ctx-dir]")
		return exitError
	}

	ctxDir := contextDir()
	if len(args) == contextDirArgs {
		ctxDir = args[1]
	}
	record, err := contextrecord.Extract(args[0], ctxDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}
	return printJson(record)
}

func runMark(args []string) int {
	flags := flag.NewFlagSet("mark", flag.ExitOnError)
	date := flags.String("synthesized", "", "synthesis date YYYY-MM-DD; required")
	root := flags.String("root", defaultRoot, "session records root")
	flags.Parse(args)
	if _, err := time.Parse(time.DateOnly, *date); err != nil || flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: sessions mark --synthesized <YYYY-MM-DD> <session-id>...")
		return exitError
	}

	records, err := sessions.RecordFiles(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}

	status := exitClean
	for _, id := range flags.Args() {
		path, ok := records[id]
		if !ok {
			fmt.Printf("%s: no record\n", id)
			status = max(status, exitViolations)
			continue
		}
		violations, err := sessions.MarkSynthesized(*date, path, schemaPath)
		status = max(status, reportViolations(err, path, violations))
	}
	return status
}

func runUnmark(args []string) int {
	flags := flag.NewFlagSet("unmark", flag.ExitOnError)
	root := flags.String("root", defaultRoot, "session records root")
	flags.Parse(args)

	records, err := sessions.RecordFiles(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}

	status := exitClean
	for _, path := range records {
		violations, err := sessions.MarkSynthesized("", path, schemaPath)
		status = max(status, reportViolations(err, path, violations))
	}
	return status
}

func runPending(args []string) int {
	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}

	flags := flag.NewFlagSet("pending", flag.ExitOnError)
	agentsCsv := flags.String("agents", "claude,codex", "agents to list")
	isDev := flags.Bool("dev", false, "include the pipeline's own sessions")
	limit := flags.Int("limit", 0, "newest-first cap; 0 = all")
	peekBin := flags.String("peek-bin", filepath.Join(os.Getenv("HOME"), "go", "bin", "peek-mcp"), "peek-mcp binary")
	registryPath := flags.String("registry", filepath.Join(stateDir, config.StateRegistryFile), "repo registry")
	root := flags.String("root", defaultRoot, "session records root")
	windowDays := flags.Int("window-days", defaultWindowDays, "only sessions active within this many days")
	flags.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), pendingTimeout)
	defer cancel()

	repoRoot, err := mainCheckoutRoot(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}
	request, err := sessions.NewPendingRequest(*isDev, *limit, *registryPath, repoRoot, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}
	request.Since = time.Now().AddDate(0, 0, -*windowDays)

	peekArgs := peek.StdioArgs(os.Getenv("HOME"), *windowDays)
	candidates, err := peek.NewStdioClient(peekArgs, *peekBin).Sessions(ctx, strings.Split(*agentsCsv, ","))
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}
	return printJson(sessions.SelectPending(candidates, request))
}

func runUnsynthesized(args []string) int {
	flags := flag.NewFlagSet("unsynthesized", flag.ExitOnError)
	root := flags.String("root", defaultRoot, "session records root")
	flags.Parse(args)

	records, err := sessions.RecordFiles(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}

	listing := make([]unsynthesizedRecord, 0)
	archivedSegment := string(filepath.Separator) + "archived" + string(filepath.Separator)
	for _, path := range records {
		if strings.Contains(path, archivedSegment) {
			continue
		}
		session, violations, err := sessions.ReadRecord(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sessions:", err)
			return exitError
		}
		if len(violations) > 0 {
			fmt.Fprintf(os.Stderr, "sessions: %s: %d violation(s), skipped — run check\n", path, len(violations))
			continue
		}
		if session.Synthesized != "" {
			continue
		}
		record := unsynthesizedRecord{
			Id:         session.Id,
			LastActive: session.LastActive,
			Path:       path,
			Repo:       session.Repo,
			Week:       session.Week,
		}
		listing = append(listing, record)
	}
	slices.SortFunc(listing, func(a, b unsynthesizedRecord) int { return strings.Compare(a.LastActive, b.LastActive) })
	return printJson(listing)
}

func contextDir() string {
	if env := os.Getenv(contextDirEnv); env != "" {
		return env
	}
	return defaultContextDir
}

func mainCheckoutRoot(ctx context.Context) (string, error) {
	commonDir, err := shell.Run(ctx, "", "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("mainCheckoutRoot: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(commonDir)), nil
}

// parseCheckArgs accepts --context before, between or after the record paths.
func parseCheckArgs(args []string) (string, []string, error) {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	transcriptPath := flags.String("context", "", "Claude transcript whose used-context record replaces the context key")

	var paths []string
	remaining := args
	for {
		if err := flags.Parse(remaining); err != nil {
			return "", nil, fmt.Errorf("parseCheckArgs: %w", err)
		}
		if flags.NArg() == 0 {
			return *transcriptPath, paths, nil
		}
		paths = append(paths, flags.Arg(0))
		remaining = flags.Args()[1:]
	}
}

func printJson(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "sessions:", err)
		return exitError
	}
	fmt.Println(string(data))
	return exitClean
}

// reportViolations prints a record's violations as "<path>: <violation>".
func reportViolations(err error, path string, violations []string) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "sessions: %s: %v\n", path, err)
		return exitError
	}
	for _, violation := range violations {
		fmt.Printf("%s: %s\n", path, violation)
	}
	if len(violations) > 0 {
		return exitViolations
	}
	fmt.Printf("%s: ok\n", path)
	return exitClean
}
