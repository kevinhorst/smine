// Command skillmodel works the typed skill model and the semantic model derived from it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
)

const (
	batteryTimeout = 2 * time.Minute
	solveTimeout   = 60 * time.Second
)

type emission struct {
	bound         int
	completeBound int
	doc           skillmodel.SmtDoc
	query         *skillmodel.Query
}

func (e *emission) boundWarning() string {
	if e.bound >= e.completeBound {
		return ""
	}
	return fmt.Sprintf("skillmodel: warning: bound %d is below the complete bound %d — an unsat verdict is not a proof", e.bound, e.completeBound)
}

func (e *emission) reachLine(verdict string) string {
	switch {
	case verdict == skillmodel.VerdictSat:
		return "reachable"
	case e.bound >= e.completeBound:
		return "DEAD — unreachable within the complete bound"
	default:
		return fmt.Sprintf("unreachable within bound %d", e.bound)
	}
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args[1:]) == 0 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel <atoms|bundle|check|diagram|export|import|model|query|render|renumber|smt|solve> [flags] <skill dir>…")
		return exitError
	}
	switch os.Args[1] {
	case "atoms":
		return runAtoms(os.Args[2:])
	case "bundle":
		return runBundle(os.Args[2:])
	case "check":
		return runCheck(os.Args[2:])
	case "diagram":
		return runDiagram(os.Args[2:])
	case "export":
		return runExport(os.Args[2:])
	case "import":
		return runImport(os.Args[2:])
	case "model":
		return runModel(os.Args[2:])
	case "query":
		return runQuery(os.Args[2:])
	case "render":
		return runRender(os.Args[2:])
	case "renumber":
		return runRenumber(os.Args[2:])
	case "smt":
		return runSmt(os.Args[2:], false)
	case "solve":
		return runSmt(os.Args[2:], true)
	default:
		fmt.Fprintf(os.Stderr, "skillmodel: unknown command: %s\n", os.Args[1])
		return exitError
	}
}

func emitDoc(model *skillmodel.Model, consistency bool, queryPath, reach string, bound int) (*emission, error) {
	if bound < 0 {
		return nil, errors.New("bound must be >= 1; omit for the default")
	}

	switch {
	case consistency:
		doc, err := skillmodel.EmitConsistency(model)
		if err != nil {
			return nil, err
		}

		emitted := &emission{doc: doc}
		return emitted, nil
	case queryPath != "":
		return emitQueryFile(bound, model, queryPath)
	case reach != "":
		return emitReach(nil, model, bound, reach)
	}
	return nil, errors.New("one of --consistency, --query, --reach is required")
}

func emitQueryFile(bound int, model *skillmodel.Model, path string) (*emission, error) {
	query, err := loadQuery(path)
	if err != nil {
		return nil, err
	}

	if query.Reach == "" {
		doc, err := skillmodel.EmitQuery(model, *query)
		if err != nil {
			return nil, err
		}

		emitted := &emission{doc: doc, query: query}
		return emitted, nil
	}

	hasBound := query.Bound != nil
	if hasBound && *query.Bound < 1 {
		return nil, errors.New("bound must be >= 1; omit for the default")
	}

	if hasBound {
		bound = *query.Bound
	}
	emitted, err := emitReach(query.Assume, model, bound, query.Reach)
	if err != nil {
		return nil, err
	}

	emitted.query = query
	return emitted, nil
}

func emitReach(assume []string, model *skillmodel.Model, requested int, transitionId string) (*emission, error) {
	extra := make([]*skillmodel.Expr, 0, len(assume))
	for index, text := range assume {
		expr, err := skillmodel.ParseExpr(text)
		if err != nil {
			return nil, fmt.Errorf("emitReach: assume[%d]: %w", index, err)
		}

		extra = append(extra, expr)
	}
	completeBound, err := skillmodel.CompleteBound(extra, model)
	if err != nil {
		return nil, fmt.Errorf("emitReach: %w", err)
	}

	bound := requested
	if bound == 0 {
		bound = completeBound
	}
	doc, err := skillmodel.EmitReachability(model, transitionId, bound, assume)
	if err != nil {
		return nil, err
	}

	emitted := &emission{
		bound:         bound,
		completeBound: completeBound,
		doc:           doc,
	}
	return emitted, nil
}

// loadQuery reads one committed query file.
func loadQuery(path string) (*skillmodel.Query, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadQuery: %w", err)
	}
	query := &skillgraph.Query{}
	if err := json.Unmarshal(data, query); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return query, nil
}

func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel check <skill dir>")
		return exitError
	}
	all, err := loadAll(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), batteryTimeout)
	defer cancel()
	findings, err := skillmodel.RunBattery(ctx, all.model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel: battery:", err)
		return exitError
	}

	hasError := false
	warningCount := 0
	for _, finding := range findings {
		fmt.Fprintf(
			os.Stderr,
			"%s: %s %s: %s\n",
			flags.Arg(0),
			finding.Severity,
			finding.Check,
			finding.Message,
		)
		switch finding.Severity {
		case skillmodel.SeverityError:
			hasError = true
		case skillmodel.SeverityWarning:
			warningCount++
		}
	}
	if hasError {
		return exitViolations
	}

	tier := "annotations"
	if all.model.IsStrict {
		tier = "model"
	}
	formalized, coverable := all.model.Stats()
	fmt.Printf("skillmodel: %s OK — tier: %s, %d/%d coverable entries formalized, %d invariants, %d guarded transitions\n",
		flags.Arg(0), tier, formalized, coverable, len(all.model.Invariants), guardedCount(all.model))
	if all.model.IsStrict {
		fmt.Printf("skillmodel: battery: %d warning(s)\n", warningCount)
	}
	return exitClean
}

func runModel(args []string) int {
	flags := flag.NewFlagSet("model", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel model <skill dir>")
		return exitError
	}
	all, err := loadAll(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	data, err := json.MarshalIndent(all.model, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	fmt.Println(string(data))
	return exitClean
}

func runQuery(args []string) int {
	flags := flag.NewFlagSet("query", flag.ExitOnError)
	expr := flags.String("expr", "", "expression the model's invariants must imply")
	flags.Parse(args)
	if flags.NArg() != 1 || *expr == "" {
		fmt.Fprintln(os.Stderr, "usage: skillmodel query --expr '<expr>' <skill dir>")
		return exitError
	}

	all, err := loadAll(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	if !all.model.IsStrict {
		fmt.Fprintf(
			os.Stderr,
			"skillmodel: %s is annotation-only — set tier: \"strict\" in its model file to opt into the semantic model\n",
			flags.Arg(0),
		)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), solveTimeout)
	defer cancel()
	result, err := skillmodel.RunQuery(ctx, all.model, *expr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	printResult(result)
	if result.Verdict == skillmodel.VerdictUnknown {
		return exitError
	}
	return exitClean
}

func runSmt(args []string, solve bool) int {
	flags := flag.NewFlagSet("smt", flag.ExitOnError)
	bound := flags.Int("bound", 0, "reachability bound (default: the complete bound)")
	consistency := flags.Bool("consistency", false, "emit the whole-model consistency check")
	queryPath := flags.String("query", "", "query file to emit")
	reach := flags.String("reach", "", "transition entry id for the reachability check")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel smt|solve (--consistency | --query <q.json> | --reach <id> [--bound n]) <skill dir>")
		return exitError
	}
	all, err := loadAll(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	if !all.model.IsStrict {
		fmt.Fprintf(os.Stderr, "skillmodel: %s is annotation-only — set tier: \"strict\" in its model file to opt into the semantic model\n", flags.Arg(0))
		return exitError
	}
	emitted, err := emitDoc(all.model, *consistency, *queryPath, *reach, *bound)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	if warning := emitted.boundWarning(); warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	if !solve {
		fmt.Print(emitted.doc.Text)
		return exitClean
	}
	return solveDoc(emitted, all.model, *reach)
}

type loaded struct {
	model *skillmodel.Model
	skill *skillgraph.Skill
}

func loadAll(dir string) (*loaded, error) {
	skill, err := skillgraph.Load(dir)
	if err != nil {
		return nil, err
	}

	model, err := skillmodel.Derive(skill)
	if err != nil {
		return nil, err
	}

	all := &loaded{
		model: model,
		skill: skill,
	}
	return all, nil
}

func runDiagram(args []string) int {
	flags := flag.NewFlagSet("diagram", flag.ExitOnError)
	asJson := flags.Bool("json", false, "print the projection document as JSON instead of Mermaid")
	phasesOnly := flags.Bool("phases", false, "render the execution chain only, without context sections")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel diagram [--json] [--phases] <skill dir>")
		return exitError
	}
	all, err := loadAll(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	projection := skillmodel.Project(all.skill, all.model)
	if *asJson {
		data, err := json.MarshalIndent(projection, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillmodel:", err)
			return exitError
		}
		fmt.Println(string(data))
		return exitClean
	}
	fmt.Print(projection.Mermaid(*phasesOnly))
	return exitClean
}

// guardedCount counts the transitions carrying a pred clause.
func guardedCount(model *skillmodel.Model) int {
	count := 0
	for _, transition := range model.Transitions {
		if transition.Pred != "" {
			count++
		}
	}
	return count
}

func printResult(result *skillmodel.QueryResult) {
	fmt.Printf("verdict: %s\n", result.Verdict)
	switch result.Verdict {
	case skillmodel.VerdictSat:
		if len(result.Scenario) > 0 {
			scenario := strings.Join(result.Scenario, ", ")
			fmt.Printf("scenario: %s\n", scenario)
		}
	case skillmodel.VerdictUnknown:
	case skillmodel.VerdictUnsat:
		core := strings.Join(result.Core, ", ")
		fmt.Printf("core: %s\n", core)
	}
}

func solveDoc(emitted *emission, model *skillmodel.Model, reach string) int {
	ctx, cancel := context.WithTimeout(context.Background(), solveTimeout)
	defer cancel()
	result, err := skillmodel.Solve(ctx, emitted.doc, model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	printResult(result)
	if result.Verdict == skillmodel.VerdictUnknown {
		return exitError
	}

	switch {
	case emitted.query != nil:
		isMismatch := emitted.query.Expect != "" && emitted.query.Expect != result.Verdict
		if isMismatch {
			fmt.Fprintf(os.Stderr, "skillmodel: expected %s, got %s\n", emitted.query.Expect, result.Verdict)
			return exitViolations
		}
	case reach != "":
		line := emitted.reachLine(result.Verdict)
		fmt.Println(line)
	}
	return exitClean
}
