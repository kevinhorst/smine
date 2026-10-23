package skillmodel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// Verdicts z3 prints for one check; unknown means the check hit the timeout.
const (
	VerdictSat     = "sat"
	VerdictUnknown = "unknown"
	VerdictUnsat   = "unsat"
)

const (
	outputTailLines = 20
	z3ErrorPrefix   = "(error"
)

// coreNamePrefixes are the assertion-name prefixes the documents put in front of an entry's smtName.
var coreNamePrefixes = []string{"post.", "stopblock.", "reach."}

// coreStepSuffix matches the step suffix of an unrolled assertion name.
var coreStepSuffix = regexp.MustCompile(`\.s[0-9]+$`)

var z3Verdicts = []string{VerdictSat, VerdictUnknown, VerdictUnsat}

// coreEntryIds keeps only names that equal an entry's smtName, so axiom and frame names never reach the result.
func coreEntryIds(model *Model, output string) []string {
	positions := entryPositions(model)
	idsByName := make(map[string]string, len(positions))
	for id := range positions {
		idsByName[smtName(id)] = id
	}
	coreLine := unsatCore(output)
	core := strings.Trim(coreLine, "()")
	var ids []string
	for token := range strings.FieldsSeq(core) {
		if id, ok := idsByName[coreName(token)]; ok {
			ids = append(ids, id)
		}
	}
	return sortEntryIds(ids, positions)
}

func coreName(token string) string {
	name := coreStepSuffix.ReplaceAllString(token, "")
	for _, prefix := range coreNamePrefixes {
		if stripped, ok := strings.CutPrefix(name, prefix); ok {
			return stripped
		}
	}
	return name
}

// entryPositions defines the model order cores and findings sort by; the model keeps no position across kinds.
func entryPositions(model *Model) map[string]int {
	positions := map[string]int{}
	for _, transition := range model.Transitions {
		positions[transition.Id] = len(positions)
	}
	for _, invariant := range model.Invariants {
		positions[invariant.Id] = len(positions)
	}
	for _, soft := range model.Soft {
		positions[soft.Id] = len(positions)
	}
	return positions
}

// outputTail returns the last outputTailLines lines of the trimmed z3 output.
func outputTail(output string) string {
	trimmed := strings.TrimSpace(output)
	lines := strings.Split(trimmed, "\n")
	if len(lines) > outputTailLines {
		lines = lines[len(lines)-outputTailLines:]
	}
	return strings.Join(lines, "\n")
}

// parseVerdicts fails on an early error line: z3 keeps going after a rejected assertion and answers a partial document.
func parseVerdicts(output string) ([]string, error) {
	var verdicts []string
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if slices.Contains(z3Verdicts, trimmed) {
			verdicts = append(verdicts, trimmed)
			continue
		}

		isEarlyError := len(verdicts) == 0 && strings.HasPrefix(trimmed, z3ErrorPrefix)
		if isEarlyError {
			return nil, fmt.Errorf("parseVerdicts: Error before the first verdict: %s", trimmed)
		}
	}
	return verdicts, nil
}

// runZ3 ignores z3's exit status alone: z3 exits nonzero whenever it printed any error line.
func runZ3(ctx context.Context, doc SmtDoc) ([]string, string, error) {
	if _, err := exec.LookPath("z3"); err != nil {
		return nil, "", errors.New("runZ3: z3 binary not found — install it with: brew install z3")
	}

	path, err := writeSmtDoc(doc)
	if err != nil {
		return nil, "", fmt.Errorf("runZ3: %w", err)
	}
	defer os.Remove(path)

	output, runErr := shell.Run(ctx, ".", "z3", path)
	if err := ctx.Err(); err != nil {
		return nil, output, fmt.Errorf("runZ3: %w", err)
	}

	verdicts, err := parseVerdicts(output)
	if err != nil {
		return nil, output, fmt.Errorf("runZ3: %w", err)
	}

	if len(verdicts) != doc.Checks {
		countErr := fmt.Errorf(
			"runZ3: Expected %d verdicts, got %d: %s",
			doc.Checks,
			len(verdicts),
			outputTail(output),
		)
		return nil, output, errors.Join(countErr, runErr)
	}

	return verdicts, output, nil
}

func sortEntryIds(ids []string, positions map[string]int) []string {
	var known []string
	for _, id := range ids {
		_, isEntry := positions[id]
		if isEntry && !slices.Contains(known, id) {
			known = append(known, id)
		}
	}
	slices.SortFunc(known, func(left, right string) int {
		return positions[left] - positions[right]
	})
	return known
}

// unsatCore skips an error line after the verdict: a get-model refused on the unsat branch.
func unsatCore(output string) string {
	isAfterVerdict := false
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == VerdictUnsat {
			isAfterVerdict = true
			continue
		}

		isCoreLine := isAfterVerdict && strings.HasPrefix(trimmed, "(") && !strings.HasPrefix(trimmed, z3ErrorPrefix)
		if isCoreLine {
			return trimmed
		}
	}
	return ""
}

func witnessScenario(output string, vocab *Vocabulary) []string {
	witness := ParseWitness(output, vocab)
	atoms := make([]string, 0, len(witness))
	for atom := range witness {
		atoms = append(atoms, atom)
	}
	slices.Sort(atoms)
	scenario := make([]string, 0, len(atoms))
	for _, atom := range atoms {
		scenario = append(scenario, atom+"="+witness[atom])
	}
	return scenario
}

// writeSmtDoc leaves removing the temp file to the caller.
func writeSmtDoc(doc SmtDoc) (string, error) {
	input, err := os.CreateTemp("", "skillmodel-*.smt2")
	if err != nil {
		return "", fmt.Errorf("writeSmtDoc: %w", err)
	}

	path := input.Name()
	_, writeErr := input.WriteString(doc.Text)
	closeErr := input.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("writeSmtDoc: %w", err)
	}

	return path, nil
}

func Solve(ctx context.Context, doc SmtDoc, model *Model) (*QueryResult, error) {
	if doc.Checks != 1 {
		return nil, fmt.Errorf("Solve: Expected a single-check document, got %d checks", doc.Checks)
	}

	verdicts, output, err := runZ3(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("Solve: %w", err)
	}

	result := &QueryResult{Verdict: verdicts[0]}
	switch result.Verdict {
	case VerdictSat:
		result.Scenario = witnessScenario(output, model.Vocabulary)
	case VerdictUnsat:
		result.Core = coreEntryIds(model, output)
	case VerdictUnknown:
	}
	return result, nil
}
