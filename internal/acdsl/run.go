package acdsl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

// Verifier output not shaped file:line: message is tool noise, not a finding.
var violationLineRe = regexp.MustCompile(`^\S+:\d+: `)

// Small pool: the heavy verifiers (golangci, exhaustive) are internally parallel already.
const checkWorkerCount = 4

// Machine-readable so retry prompts are generated, not written.
type Diagnostic struct {
	RuleId   string `json:"id"`
	Verifier string `json:"verifier"`
	Message  string `json:"message"` // verifier output line, file:line: text
	Why      string `json:"why"`
}

type verifierJob struct {
	entry  RegistryEntry
	files  []string
	hashes *contentHashes
	index  int
	rule   Rule
}

// verifierResult carries one job's outcome back from the pool.
type verifierResult struct {
	diagnostics []Diagnostic
	err         error
	index       int
}

// Untracked files are included: the gate must see what an agent just generated.
func FileUniverse(ctx context.Context, root string, pathspecs ...string) ([]string, error) {
	args := append([]string{"ls-files", "--cached", "--others", "--exclude-standard", "--"}, pathspecs...)
	output, err := shell.Run(ctx, root, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("FileUniverse: %w", err)
	}
	var files []string
	for _, path := range strings.Fields(strings.TrimSpace(output)) {
		if _, statErr := os.Stat(filepath.Join(root, path)); statErr == nil {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files, nil
}

// A broken verifier (exit ≥2, missing entry, unresolvable anchor) is an error, not a finding.
func Check(ctx context.Context, root string, rules []Rule, registry map[string]RegistryEntry, universe []string) ([]Diagnostic, error) {
	ordered := make([][]Diagnostic, len(rules))
	jobs := make([]verifierJob, 0, len(rules))
	hashes := hashUniverse(root, universe)
	for index, rule := range rules {
		if rule.Reach == reach.None {
			continue
		}
		// A tree without the needs= artifacts (a public clone) cannot run the rule: skip, not breakage.
		if len(missingNeeds(root, rule)) > 0 {
			continue
		}
		entry, ok := registry[rule.Verifier]
		if !ok {
			return nil, fmt.Errorf("Check: %s: verifier %q not registered", rule.Id, rule.Verifier)
		}
		matched, err := ResolveAnchor(universe, rule)
		if err != nil {
			// empty="ok" marks a transient artifact class: zero matches is a legitimate state.
			if rule.AllowEmpty && errors.Is(err, ErrAnchorEmpty) {
				continue
			}
			if rule.Lifetime == LifetimeTask && errors.Is(err, ErrAnchorEmpty) {
				ordered[index] = append(ordered[index], Diagnostic{
					RuleId:   rule.Id,
					Verifier: rule.Verifier,
					Message:  fmt.Sprintf("%s:%d: anchor matched no files — planned artifact not yet present", rule.File, rule.Line),
					Why:      rule.Why,
				})
				continue
			}
			return nil, fmt.Errorf("Check: %w", err)
		}
		job := verifierJob{entry: entry, files: matched, hashes: hashes, index: index, rule: rule}
		jobs = append(jobs, job)
	}

	workChan := make(chan verifierJob, len(jobs))
	for _, job := range jobs {
		workChan <- job
	}
	close(workChan)

	resultChan := make(chan verifierResult, len(jobs))

	var wg sync.WaitGroup
	for range checkWorkerCount {
		wg.Go(func() {
			for job := range workChan {
				resultChan <- runVerifierJob(ctx, root, job)
			}
		})
	}
	wg.Wait()
	close(resultChan)

	var firstErr error
	firstErrIndex := len(rules)
	for result := range resultChan {
		if result.err != nil {
			if result.index < firstErrIndex {
				firstErr = result.err
				firstErrIndex = result.index
			}
			continue
		}
		ordered[result.index] = result.diagnostics
	}
	if firstErr != nil {
		return nil, firstErr
	}

	var diagnostics []Diagnostic
	for _, ruleDiagnostics := range ordered {
		diagnostics = append(diagnostics, ruleDiagnostics...)
	}
	return diagnostics, nil
}

func runVerifierJob(ctx context.Context, root string, job verifierJob) verifierResult {
	diagnostics, err := runVerifier(ctx, root, job.rule, job.entry, job.files, job.hashes)
	result := verifierResult{diagnostics: diagnostics, err: err, index: job.index}
	return result
}

// Exit 0 = pass, 1 = one violation per contract-shaped stdout line, anything else = tool error; nil hashes = no cache.
func runVerifier(ctx context.Context, root string, rule Rule, entry RegistryEntry, files []string, hashes *contentHashes) ([]Diagnostic, error) {
	entry = resolveVerifierBinary(root, entry)
	verdictKey := cacheKey(entry, rule, files, hashes)
	if verdictKey != "" {
		if verdict, ok := loadVerdict(root, verdictKey); ok {
			return verdict.Diagnostics, nil
		}
	}

	listPath, err := writeFilesList(files)
	if err != nil {
		return nil, fmt.Errorf("runVerifier: %s: %w", rule.Id, err)
	}
	defer os.Remove(listPath)

	args := append(append([]string{}, entry.Argv[1:]...), listPath)
	for _, key := range sortedKeys(rule.Params) {
		args = append(args, key+"="+rule.Params[key])
	}

	output, err := shell.RunWithin(ctx, root, entry.Argv[0], entry.Timeout(), args...)
	if err == nil {
		if verdictKey != "" {
			storeVerdict(root, verdictKey, &cachedVerdict{Exit: 0})
		}
		return nil, nil // exit 0: pass
	}
	if !isExitOne(err) {
		return nil, fmt.Errorf("runVerifier: %s: verifier %s failed: %w\n%s", rule.Id, rule.Verifier, err, output)
	}

	var diagnostics []Diagnostic
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if !violationLineRe.MatchString(line) {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{RuleId: rule.Id, Verifier: rule.Verifier, Message: line, Why: rule.Why})
	}
	if len(diagnostics) == 0 {
		// Exit 1 with no contract-shaped line: never let a failure vanish.
		diagnostics = append(diagnostics, Diagnostic{RuleId: rule.Id, Verifier: rule.Verifier, Message: strings.TrimSpace(output), Why: rule.Why})
	}
	if verdictKey != "" {
		storeVerdict(root, verdictKey, &cachedVerdict{Diagnostics: diagnostics, Exit: 1})
	}
	return diagnostics, nil
}

// Check and RunFixtures share it so the gate and its fixtures agree on applicability.
func missingNeeds(root string, rule Rule) []string {
	var missing []string
	for _, path := range rule.Needs {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			missing = append(missing, path)
		}
	}
	return missing
}

func isExitOne(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func writeFilesList(files []string) (string, error) {
	tmp, err := os.CreateTemp("", "acdsl-files-*.txt")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(strings.Join(files, "\n") + "\n"); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return filepath.Clean(tmp.Name()), nil
}

func sortedKeys(params map[string]string) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
