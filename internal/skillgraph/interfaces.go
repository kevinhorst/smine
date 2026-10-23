package skillgraph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/cmdline"
)

// Kinds of an interface.
const (
	KindCommand = "command"
	KindPath    = "path"
	KindProgram = "program"
	KindSkill   = "skill"
)

// Scopes of an interface: an install-scoped value is filled at deploy, a repo-scoped one at invocation.
const (
	ScopeInstall = "install"
	ScopeRepo    = "repo"
)

const (
	exitOther = "other"
	exitZero  = "0"
	noTimeout = "none"
)

// leafNamePattern is a skill value: one deployed leaf name.
var leafNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Interface returns the record for "<id>@<version>".
func (c *Catalog) Interface(ref string) (*Interface, bool) {
	for _, iface := range c.Interfaces {
		if iface.Ref() == ref {
			return iface, true
		}
	}
	return nil, false
}

// Provider returns the provider by id.
func (c *Catalog) Provider(id string) (*Provider, bool) {
	for _, provider := range c.Providers {
		if provider.Id == id {
			return provider, true
		}
	}
	return nil, false
}

// ProviderFor returns the one provider claiming the interface version, nil when none does.
func (c *Catalog) ProviderFor(ref string) *Provider {
	for _, provider := range c.Providers {
		if slices.Contains(provider.Implements, ref) {
			return provider
		}
	}
	return nil
}

// ArgsHead is the first word of a program interface's args, the subcommand a reference must be followed by.
func (i *Interface) ArgsHead() string {
	return firstWord(i.Invocation.Args)
}

// IsRepoScoped reports whether the value is filled at invocation for the session's repo.
func (i *Interface) IsRepoScoped() bool {
	return i.Scope == ScopeRepo
}

// Ref renders "<id>@<version>".
func (i *Interface) Ref() string {
	return fmt.Sprintf("%s@%d", i.Id, i.Version)
}

// Claims is the provider's sorted implements list, the handshake's expected answer.
func (p *Provider) Claims() []string {
	return slices.Sorted(slices.Values(p.Implements))
}

// Executable is the path the probe runs: Binary when set, else Program.
func (p *Provider) Executable() string {
	if p.Binary != "" {
		return p.Binary
	}
	return p.Program
}

// IsProvided reports whether the provider names a program the install puts on PATH.
func (p *Provider) IsProvided() bool {
	return p.Program != ""
}

// RepoScoped is the set of the skill's variable names bound to a repo-scoped record.
func (s *Skill) RepoScoped() map[string]bool {
	scoped := map[string]bool{}
	for name, variable := range s.VariableNames() {
		iface, isKnown := s.catalog().Interface(variable.Interface)
		if isKnown && iface.IsRepoScoped() {
			scoped[name] = true
		}
	}
	return scoped
}

// checkCatalog refuses duplicate ids, a validate block that does not fit the kind (a skill validates by name alone), an exit table without "0" and "other", an unparsable timeout, a program interface without an args head, a command outside scope repo, a repo-scoped record without an example, and provider claims that name no record, clash or mix scopes.
func checkCatalog(catalog *Catalog) []string {
	var violations []string
	seen := map[string]bool{}
	for _, iface := range catalog.Interfaces {
		ref := iface.Ref()
		if seen[ref] {
			violations = append(violations, fmt.Sprintf("catalog: interface %s declared twice", ref))
		}
		seen[ref] = true
		violations = append(violations, interfaceViolations(iface)...)
	}

	claimed := map[string]string{}
	providerIds := map[string]bool{}
	for _, provider := range catalog.Providers {
		if providerIds[provider.Id] {
			violations = append(violations, fmt.Sprintf("catalog: provider %s declared twice", provider.Id))
		}
		providerIds[provider.Id] = true
		if provider.Binary != "" && provider.Program == "" {
			violations = append(violations, fmt.Sprintf("catalog: provider %s: binary without a program", provider.Id))
		}
		scopes := map[string]bool{}
		for _, ref := range provider.Implements {
			if !seen[ref] {
				violations = append(violations, fmt.Sprintf("catalog: provider %s implements %s, which no interface record declares", provider.Id, ref))
			}
			if iface, isKnown := catalog.Interface(ref); isKnown {
				scopes[iface.Scope] = true
			}
			if other, isClaimed := claimed[ref]; isClaimed {
				violations = append(violations, fmt.Sprintf("catalog: %s is implemented by %s and %s — one provider per interface version", ref, other, provider.Id))
			}
			claimed[ref] = provider.Id
		}
		if len(scopes) > 1 {
			violations = append(violations, fmt.Sprintf("catalog: provider %s implements install- and repo-scoped records — one scope per provider", provider.Id))
		}
	}
	return violations
}

func firstWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func interfaceViolations(iface *Interface) []string {
	var violations []string
	ref := iface.Ref()
	if iface.Kind == KindCommand && !iface.IsRepoScoped() {
		violations = append(violations, fmt.Sprintf("catalog: %s: a command interface is repo-scoped — its value differs per repo", ref))
	}
	if iface.IsRepoScoped() && iface.Example == "" {
		violations = append(violations, fmt.Sprintf("catalog: %s: a repo-scoped interface carries an example the abort sentence shows", ref))
	}
	switch iface.Kind {
	case KindCommand, KindProgram:
		if !iface.Validate.Resolves || iface.Validate.Exists != "" {
			violations = append(violations, fmt.Sprintf("catalog: %s: a %s interface validates with resolves: true and no exists", ref, iface.Kind))
		}
	case KindPath:
		if iface.Validate.Exists == "" || iface.Validate.Resolves {
			violations = append(violations, fmt.Sprintf("catalog: %s: a path interface validates with exists: dir|file and no resolves", ref))
		}
	case KindSkill:
		if iface.Validate.Exists != "" || iface.Validate.Resolves {
			violations = append(violations, fmt.Sprintf("catalog: %s: a skill interface validates by its leaf name — no resolves, no exists", ref))
		}
	}
	if iface.Kind == KindProgram && iface.ArgsHead() == "" {
		violations = append(violations, fmt.Sprintf("catalog: %s: a program interface's args open with the subcommand word", ref))
	}

	_, hasZero := iface.Result.Exit[exitZero]
	_, hasOther := iface.Result.Exit[exitOther]
	if !hasZero || !hasOther {
		violations = append(violations, fmt.Sprintf("catalog: %s: result.exit needs the keys %q and %q", ref, exitZero, exitOther))
	}
	if iface.Invocation.Timeout != noTimeout {
		if _, err := time.ParseDuration(iface.Invocation.Timeout); err != nil {
			violations = append(violations, fmt.Sprintf("catalog: %s: invocation.timeout %q is neither a duration nor %q", ref, iface.Invocation.Timeout, noTimeout))
		}
	}
	return violations
}

// CheckValue applies the value rules every value meets, then the form of its interface's kind.
func CheckValue(iface *Interface, value string) error {
	switch {
	case strings.ContainsAny(value, "\n\r") && iface.Kind != KindCommand:
		return errors.New("value: spans more than one line")
	case strings.Contains(value, "\r"):
		return errors.New("value: holds a carriage return")
	case strings.Contains(value, "`"):
		return errors.New("value: holds a backtick")
	case strings.Contains(value, "$"+variablePrefix):
		return errors.New("value: references $SKILL_")
	case strings.ContainsAny(value, "()"):
		return errors.New("value: holds a parenthesis — grants splice values into Bash(…)")
	}

	switch iface.Kind {
	case KindCommand:
		if _, err := cmdline.SplitLines(value); err != nil {
			return fmt.Errorf("value: %w", err)
		}
	case KindProgram:
		words, err := cmdline.Split(value)
		if err != nil {
			return fmt.Errorf("value: %w", err)
		}
		if len(words) != 1 {
			return fmt.Errorf("value: a program is one word, got %d", len(words))
		}
	case KindPath:
		if !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, "/") {
			return errors.New("value: a path opens with ~/ or /")
		}
	case KindSkill:
		if !leafNamePattern.MatchString(value) {
			return errors.New("value: a skill is one leaf name — lowercase letters, digits and hyphens")
		}
	}
	return nil
}

// FindCatalog loads the skills root's interfaces.cue; a dir outside a skills root, or a root without the file, yields an empty catalog.
func FindCatalog(dir string) (*Catalog, error) {
	root, err := FindRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("FindCatalog: %w", err)
	}

	empty := &Catalog{}
	if root == "" {
		return empty, nil
	}

	catalogPath := filepath.Join(root, FileCatalog)
	hasCatalog, err := fileExists(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("FindCatalog: %w", err)
	}

	if !hasCatalog {
		return empty, nil
	}
	return LoadCatalog(catalogPath)
}

// LoadCatalog reads, schema-validates and checks one interfaces.cue.
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadCatalog: Failed to read %s: %w", path, err)
	}

	catalog, err := decodeCatalog(data, path)
	if err != nil {
		return nil, fmt.Errorf("LoadCatalog: %w", err)
	}

	if violations := checkCatalog(catalog); len(violations) > 0 {
		return nil, fmt.Errorf("LoadCatalog: %s: %s", path, strings.Join(violations, "; "))
	}
	return catalog, nil
}

// VariableKey is the env key an interface binds to: the id uppercased with hyphens as underscores.
func VariableKey(interfaceId string) string {
	return strings.ToUpper(strings.ReplaceAll(interfaceId, "-", "_"))
}
