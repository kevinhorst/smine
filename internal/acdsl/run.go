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

	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

// violationLineRe is the verifier output contract: one violation per line
// as file:line: message. Anything else in the combined output (go run's own
// "exit status 1" on stderr, compiler chatter) is tool noise, not a finding.
var violationLineRe = regexp.MustCompile(`^\S+:\d+: `)

// Diagnostic is one gate finding, machine-readable so retry prompts are
// generated, not written.
type Diagnostic struct {
	RuleId   string `json:"id"`
	Verifier string `json:"verifier"`
	Message  string `json:"message"` // verifier output line, file:line: text
	Why      string `json:"why"`
}

// FileUniverse lists the repo's files — tracked plus untracked unignored,
// minus deleted-but-indexed — sorted. No pathspecs means every file: the
// deterministic universe anchors resolve against. Untracked inclusion is the
// point: the gate must see what an agent just generated, not what git
// already blessed.
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

// Check resolves every rule's anchor and runs its registered verifier.
// Violations come back as diagnostics; a broken verifier (exit ≥2, missing
// registry entry, unresolvable anchor) is an error — tool breakage, not a finding.
func Check(ctx context.Context, root string, rules []Rule, registry map[string]RegistryEntry, universe []string) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	for _, rule := range rules {
		if rule.Reach == reach.None {
			continue
		}
		// needs= names artifacts the verifier reads (a generated index, a
		// schema). A tree without them — a public clone whose private
		// artifacts are excluded — cannot run the rule; skip, not breakage.
		if len(missingNeeds(root, rule)) > 0 {
			continue
		}
		entry, ok := registry[rule.Verifier]
		if !ok {
			return nil, fmt.Errorf("Check: %s: verifier %q not registered", rule.Id, rule.Verifier)
		}
		matched, err := ResolveAnchor(universe, rule)
		if err != nil {
			// empty="ok" marks a transient artifact class (e.g. active design
			// plans, all archived) — zero matches is a legitimate state.
			if rule.AllowEmpty && errors.Is(err, ErrAnchorEmpty) {
				continue
			}
			if rule.Lifetime == LifetimeTask && errors.Is(err, ErrAnchorEmpty) {
				diagnostics = append(diagnostics, Diagnostic{
					RuleId:   rule.Id,
					Verifier: rule.Verifier,
					Message:  fmt.Sprintf("%s:%d: anchor matched no files — planned artifact not yet present", rule.File, rule.Line),
					Why:      rule.Why,
				})
				continue
			}
			return nil, fmt.Errorf("Check: %w", err)
		}
		ruleDiagnostics, err := runVerifier(ctx, root, rule, entry, matched)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, ruleDiagnostics...)
	}
	return diagnostics, nil
}

// runVerifier executes one rule's verifier against the given files and
// maps the exit code per the contract: 0 = pass, 1 = violations (one per
// contract-shaped stdout line), anything else = tool error.
func runVerifier(ctx context.Context, root string, rule Rule, entry RegistryEntry, files []string) ([]Diagnostic, error) {
	entry = resolveVerifierBinary(root, entry)
	listPath, err := writeFilesList(files)
	if err != nil {
		return nil, fmt.Errorf("runVerifier: %s: %w", rule.Id, err)
	}
	defer os.Remove(listPath)

	args := append(append([]string{}, entry.Argv[1:]...), listPath)
	for _, key := range sortedKeys(rule.Params) {
		args = append(args, key+"="+rule.Params[key])
	}

	runCtx, cancel := context.WithTimeout(ctx, entry.Timeout())
	output, err := shell.Run(runCtx, root, entry.Argv[0], args...)
	cancel()
	if err == nil {
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
	return diagnostics, nil
}

// missingNeeds returns the rule's needs= paths absent under root. Check and
// RunFixtures share it: a rule whose declared artifacts are missing is
// skipped in both, so the gate and its fixtures agree on applicability.
func missingNeeds(root string, rule Rule) []string {
	var missing []string
	for _, path := range rule.Needs {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			missing = append(missing, path)
		}
	}
	return missing
}

// isExitOne reports whether the error chain ends in a verifier's exit
// status 1 — the violations verdict, as opposed to tool breakage.
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
