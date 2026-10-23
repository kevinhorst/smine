// Command acdsl runs the agentic context DSL: one rule source, two projections.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kevinhorst/smine/internal/acdsl"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: acdsl <check|guard|project|fixtures|interfaces> [flags]")
		return exitError
	}
	switch os.Args[1] {
	case "check":
		return runCheck(os.Args[2:])
	case "interfaces":
		return runInterfaces(os.Args[2:])
	case "guard":
		return runGuard(os.Args[2:])
	case "project":
		return runProject(os.Args[2:])
	case "fixtures":
		return runFixtures(os.Args[2:])
	case "verdicts":
		return runVerdicts(os.Args[2:])
	case "dist":
		return runDist(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "acdsl: unknown command: %s\n", os.Args[1])
		return exitError
	}
}

func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ExitOnError)
	asJson := flags.Bool("json", false, "emit diagnostics as JSON records")
	lifetime := flags.String("lifetime", acdsl.LifetimeDoctrine, "rule lifetimes to gate: doctrine|task|all")
	ruleId := flags.String("rule", "", "gate only this rule id (any lifetime) — the generation dry-run")
	reportPath := flags.String("report", "", "also write the grouped violations report to this file")
	registryPath := flags.String("registry", "", "registry overlaid on the root's acdsl/registry.json, entries winning by name")
	root := flags.String("root", ".", "module root")
	flags.Parse(args)

	ctx := context.Background()
	discovery, err := acdsl.DiscoverRules(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	staged, err := acdsl.ValidateStagedClean(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	committed, err := acdsl.ValidateCommittedClean(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	violations := append(discovery.Violations, staged...)
	violations = append(violations, committed...)
	if len(violations) > 0 {
		for _, violation := range violations {
			fmt.Println(violation)
		}
		fmt.Fprintf(os.Stderr, "acdsl: %d authoring violation(s)\n", len(violations))
		return exitViolations
	}

	gated, err := acdsl.FilterLifetime(discovery.Rules, *lifetime)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if *ruleId != "" {
		gated, err = acdsl.FilterId(discovery.Rules, *ruleId)
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
	}
	registry, err := acdsl.LoadRegistryOverlay(*registryPath, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	policy, err := acdsl.LoadPolicy(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	policyDiagnostics, err := acdsl.CheckPolicy(ctx, *root, policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	diagnostics, err := acdsl.Check(ctx, *root, gated, registry, discovery.Universe)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	diagnostics = append(policyDiagnostics, diagnostics...)
	logVerdicts(ctx, *root, gated, diagnostics)
	if len(diagnostics) > 0 {
		for _, diagnostic := range diagnostics {
			if *asJson {
				record, _ := json.Marshal(diagnostic)
				fmt.Println(string(record))
			} else {
				fmt.Printf("%s: [%s] %s — %s\n", diagnostic.Message, diagnostic.RuleId, diagnostic.Verifier, diagnostic.Why)
			}
		}
		report := acdsl.BuildReport(registry, diagnostics)
		fmt.Println(report)
		if *reportPath != "" {
			if err := os.WriteFile(*reportPath, []byte(report), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "acdsl: report not written:", err)
			}
		}
		fmt.Fprintf(os.Stderr, "acdsl: %d violation(s) across %d rule(s)\n", len(diagnostics), len(gated))
		return exitViolations
	}
	fmt.Printf("acdsl: %d rule(s) OK\n", len(gated))
	return exitClean
}

func runGuard(args []string) int {
	flags := flag.NewFlagSet("guard", flag.ExitOnError)
	root := flags.String("root", ".", "module root")
	flags.Parse(args)

	ctx := context.Background()
	staged, err := acdsl.ValidateStagedClean(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if len(staged) > 0 {
		for _, violation := range staged {
			fmt.Println(violation)
		}
		fmt.Fprintf(os.Stderr, "acdsl: %d staged projection violation(s)\n", len(staged))
		return exitViolations
	}
	return exitClean
}

func logVerdicts(ctx context.Context, root string, rules []acdsl.Rule, diagnostics []acdsl.Diagnostic) {
	if os.Getenv("ACDSL_VERDICTS_ENABLED") == "0" {
		return
	}
	path, err := acdsl.DefaultVerdictsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl: verdict log skipped:", err)
		return
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	branch, err := acdsl.GitBranch(ctx, root)
	if err != nil {
		branch = ""
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	record := acdsl.BuildVerdictRecord(ts, absRoot, branch, os.Getenv("CLAUDE_SESSION_ID"), rules, diagnostics)
	if err := acdsl.AppendVerdict(path, record); err != nil {
		fmt.Fprintln(os.Stderr, "acdsl: verdict log skipped:", err)
	}
}

func runDist(args []string) int {
	flags := flag.NewFlagSet("dist", flag.ExitOnError)
	target := flags.String("target", "", "deploy-target repo name (dir basename); required")
	dest := flags.String("dest", "", "target repo root to write into; required")
	root := flags.String("root", ".", "module root")
	task := flags.Bool("task", false, "also ship task-lifetime rules that reach the target")
	flags.Parse(args)
	if *target == "" || *dest == "" {
		fmt.Fprintln(os.Stderr, "usage: acdsl dist -target <name> -dest <dir> [-root <dir>] [-task]")
		return exitError
	}
	lines, err := acdsl.Dist(context.Background(), *root, *target, *dest, *task)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return exitClean
}

func runVerdicts(args []string) int {
	flags := flag.NewFlagSet("verdicts", flag.ExitOnError)
	path := flags.String("path", "", "verdict log (default: $ACDSL_VERDICTS_PATH or ~/.claude/acdsl/verdicts.jsonl)")
	since := flags.Duration("since", 0, "only runs newer than now-<duration> (default: all)")
	flags.Parse(args)

	sink := *path
	if sink == "" {
		resolved, err := acdsl.DefaultVerdictsPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
		sink = resolved
	}
	cutoff := ""
	if *since > 0 {
		cutoff = time.Now().UTC().Add(-*since).Format(time.RFC3339)
	}
	records, skipped, err := acdsl.ReadVerdicts(sink, cutoff)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	stats := acdsl.AggregateVerdicts(records)
	if len(stats) == 0 {
		fmt.Println("acdsl: no verdicts recorded")
		return exitClean
	}
	fmt.Printf("%-18s %-9s %5s %5s %10s  %s\n", "rule", "projected", "runs", "red", "violations", "last-red")
	for _, stat := range stats {
		fmt.Printf("%-18s %-9t %5d %5d %10d  %s\n", stat.Id, stat.Projected, stat.Runs, stat.RedRuns, stat.Violations, stat.LastRed)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "acdsl: %d unparseable line(s) skipped\n", skipped)
	}
	return exitClean
}

func runProject(args []string) int {
	flags := flag.NewFlagSet("project", flag.ExitOnError)
	file := flags.String("file", "", "repo-relative file whose projection to sync in place")
	strip := flags.Bool("strip", false, "remove every projection block (pre-commit sweep)")
	plan := flags.String("plan", "", "repo-relative path that need not exist: print the rules that would govern it")
	contextFile := flags.String("context", "", "repo-relative file: print its delivered rules as plain lines, the read hook's data source")
	root := flags.String("root", ".", "module root")
	flags.Parse(args)
	modes := 0
	for _, selected := range []bool{*file != "", *strip, *plan != "", *contextFile != ""} {
		if selected {
			modes++
		}
	}
	if modes != 1 {
		fmt.Fprintln(os.Stderr, "acdsl: project requires exactly one of -file <path>, -strip, -plan <path>, -context <path>")
		return exitError
	}

	ctx := context.Background()
	if *strip {
		universe, err := acdsl.FileUniverse(ctx, *root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
		stripped, err := acdsl.StripAll(*root, universe)
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
		fmt.Printf("acdsl: stripped %d file(s)\n", stripped)
		return exitClean
	}

	discovery, err := acdsl.DiscoverRules(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if len(discovery.Violations) > 0 {
		for _, violation := range discovery.Violations {
			fmt.Fprintln(os.Stderr, violation)
		}
		return exitViolations
	}
	if *plan != "" {
		governing, err := acdsl.RulesForPlannedPath(discovery.Rules, *plan)
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
		governing = acdsl.Delivered(governing)
		if len(governing) == 0 {
			fmt.Printf("acdsl: no rules would govern %s\n", *plan)
			return exitClean
		}
		for _, rule := range governing {
			tag := ""
			if rule.Lifetime == acdsl.LifetimeTask {
				tag = " (task)"
			}
			fmt.Printf("- [%s]%s %s\n", rule.Id, tag, rule.Why)
		}
		return exitClean
	}
	if *contextFile != "" {
		governing, err := acdsl.RulesForFile(discovery.Rules, discovery.Universe, *contextFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "acdsl:", err)
			return exitError
		}
		for _, rule := range acdsl.Delivered(governing) {
			fmt.Printf("- [%s] %s\n", rule.Id, rule.Why)
		}
		return exitClean
	}
	governing, err := acdsl.RulesForFile(discovery.Rules, discovery.Universe, *file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if !acdsl.Projectable(*file) {
		if len(governing) == 0 {
			fmt.Printf("acdsl: no rules govern %s\n", *file)
		} else {
			fmt.Printf("acdsl: %s is not file-projectable (%d rule(s) enforced; no comment syntax)\n", *file, len(governing))
		}
		return exitClean
	}
	delivered := acdsl.Delivered(governing)
	changed, err := acdsl.ProjectFile(*root, *file, delivered)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	switch {
	case len(delivered) == 0 && changed:
		fmt.Printf("acdsl: stripped stale projection from %s\n", *file)
	case len(governing) == 0:
		fmt.Printf("acdsl: no rules govern %s\n", *file)
	case len(delivered) == 0:
		fmt.Printf("acdsl: %s is gate-only (%d rule(s) enforced, none projected)\n", *file, len(governing))
	case changed:
		fmt.Printf("acdsl: projected %s (%d rule(s))\n", *file, len(delivered))
	default:
		fmt.Printf("acdsl: %s already current (%d rule(s))\n", *file, len(delivered))
	}
	return exitClean
}

func runFixtures(args []string) int {
	flags := flag.NewFlagSet("fixtures", flag.ExitOnError)
	lifetime := flags.String("lifetime", acdsl.LifetimeDoctrine, "rule lifetimes to prove: doctrine|task|all")
	registryPath := flags.String("registry", "", "registry overlaid on the root's acdsl/registry.json, entries winning by name")
	root := flags.String("root", ".", "module root")
	flags.Parse(args)

	ctx := context.Background()
	discovery, err := acdsl.DiscoverRules(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if len(discovery.Violations) > 0 {
		for _, violation := range discovery.Violations {
			fmt.Println(violation)
		}
		fmt.Fprintf(os.Stderr, "acdsl: %d authoring violation(s)\n", len(discovery.Violations))
		return exitViolations
	}
	gated, err := acdsl.FilterLifetime(discovery.Rules, *lifetime)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	registry, err := acdsl.LoadRegistryOverlay(*registryPath, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	report, err := acdsl.RunFixtures(ctx, *root, gated, registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}
	if len(report.Failures) > 0 {
		for _, failure := range report.Failures {
			fmt.Println(failure)
		}
		fmt.Fprintf(os.Stderr, "acdsl: %d fixture failure(s) · %s\n", len(report.Failures), report.Meta())
		return exitViolations
	}
	fmt.Printf("acdsl: fixtures OK (%d rule(s) with examples) · %s\n", report.Checked, report.Meta())
	return exitClean
}
