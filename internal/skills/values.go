package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/cmdline"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// Sources of a variable's value, the order Values tries them.
const (
	SourceEnvironment   = "environment"
	SourceGlobalDefault = "global default"
	SourceInvocation    = "invocation"
	SourceProvider      = "provider"
	SourceRecord        = "record"
	SourceUnset         = "unset"
)

const (
	homePrefix        = "~/"
	probeTimeout      = 10 * time.Second
	providerKeyPrefix = "PROVIDER_"
)

// Rows are the install record's skill rows: the global default per repo-scoped interface id, the SKILL_* variables and the PROVIDER_<ID> program overrides.
type Rows struct {
	Defaults  map[string]string
	Providers map[string]string
	Skills    map[string]string
}

// ProviderKey is the record's key of a provider's program override: PROVIDER_ + the id uppercased with hyphens as underscores.
func ProviderKey(providerId string) string {
	return providerKeyPrefix + strings.ToUpper(strings.ReplaceAll(providerId, "-", "_"))
}

// Prober answers a provider's handshake once per executable; the deploy run owns one, tests pass a fake.
type Prober interface {
	Claims(ctx context.Context, provider *skillgraph.Provider) ([]string, error)
}

// VariableValue is one declared variable with the value this install resolved, its source and the cause when unset.
type VariableValue struct {
	Cause      string
	Interface  *skillgraph.Interface
	IsDeferred bool
	Name       string
	Provider   string
	Required   bool
	Source     string
	Value      string
}

// isResolvable looks program up on PATH; a ~/ path is expanded, a relative one with a slash joined onto dir.
func isResolvable(dir, program string) bool {
	path := ExpandHome(program)
	isRelativePath := strings.Contains(path, "/") && !filepath.IsAbs(path)
	if isRelativePath {
		path = filepath.Join(dir, path)
	}

	_, err := exec.LookPath(path)
	return err == nil
}

// resolveValue: SKILL_ environment, SKILL_ record row, the global default of a repo-scoped interface, then the provider — its PROVIDER_ environment, its record row, its catalog program when installed on this OS.
func resolveValue(iface *skillgraph.Interface, installed map[components.ID]bool, isEverything bool, name string, provider *skillgraph.Provider, rows *Rows) (string, string, string) {
	if value := os.Getenv(name); value != "" {
		return SourceEnvironment, value, ""
	}
	if rows != nil && rows.Skills[name] != "" {
		return SourceRecord, rows.Skills[name], ""
	}
	if value := GlobalDefault(iface, rows); value != "" {
		return SourceGlobalDefault, value, ""
	}
	if provider == nil {
		return SourceUnset, "", ""
	}

	providerSource := SourceProvider + " " + provider.Id
	providerName := ProviderKey(provider.Id)
	if value := os.Getenv(providerName); value != "" {
		return providerSource, value, provider.Id
	}
	if rows != nil && rows.Providers[providerName] != "" {
		return providerSource, rows.Providers[providerName], provider.Id
	}

	isInstalled := isEverything || installed[provider.Component]
	if isInstalled && provider.IsProvided() {
		return providerSource, provider.Program, provider.Id
	}
	return SourceUnset, "", ""
}

// validateValue returns the first failure: the value rules, then resolves or exists under dir, then the provider's claim.
func validateValue(ctx context.Context, dir string, iface *skillgraph.Interface, prober Prober, provider *skillgraph.Provider, value *VariableValue) string {
	if err := skillgraph.CheckValue(iface, value.Value); err != nil {
		return err.Error()
	}
	if cause := ValidateLocation(dir, iface, value.Value); cause != "" {
		return cause
	}

	isProbed := provider != nil && value.Provider == provider.Id
	if !isProbed || prober == nil {
		return ""
	}

	claims, err := prober.Claims(ctx, provider)
	if err != nil {
		return fmt.Sprintf("%s does not answer `%s interfaces`: %v", provider.Id, provider.Executable(), err)
	}
	if !slices.Contains(claims, iface.Ref()) {
		return fmt.Sprintf("%s does not implement %s (answered: %s)", provider.Id, iface.Ref(), strings.Join(claims, ", "))
	}
	return ""
}

// DeferredNames is the set of variables a resolution deferred to invocation.
func DeferredNames(values []VariableValue) map[string]bool {
	deferred := map[string]bool{}
	for _, value := range values {
		if value.IsDeferred {
			deferred[value.Name] = true
		}
	}
	return deferred
}

// ExpandHome replaces a leading ~/ with the user's home dir; any other value is returned as it is.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, homePrefix) {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, homePrefix))
}

// GlobalDefault is the record's global default of a repo-scoped interface, "" for any other or a nil one.
func GlobalDefault(iface *skillgraph.Interface, rows *Rows) string {
	if rows == nil || iface == nil {
		return ""
	}
	if !iface.IsRepoScoped() {
		return ""
	}
	return rows.Defaults[iface.Id]
}

// Unconfigured renders the skip reason: each unset required variable with its cause; a deferred one is filled at invocation, never unset.
func Unconfigured(values []VariableValue) []string {
	var reasons []string
	for _, value := range values {
		isSet := value.Value != "" || value.IsDeferred
		if !value.Required || isSet {
			continue
		}

		reason := value.Name
		if value.Cause != "" {
			reason += ": " + value.Cause
		}
		reasons = append(reasons, reason)
	}
	return reasons
}

// ValidateLocation: resolves looks each command line's first word up on PATH, a relative word with a slash under dir; exists stats the ~-expanded path; a skill name has no location.
func ValidateLocation(dir string, iface *skillgraph.Interface, value string) string {
	switch iface.Kind {
	case skillgraph.KindCommand, skillgraph.KindProgram:
		lines, err := cmdline.SplitLines(value)
		if err != nil {
			return fmt.Sprintf("%q is not a command line", value)
		}
		for _, words := range lines {
			if !isResolvable(dir, words[0]) {
				return words[0] + " does not resolve to an executable"
			}
		}
	case skillgraph.KindPath:
		info, err := os.Stat(ExpandHome(value))
		isWrongKind := err == nil && (iface.Validate.Exists == "dir") != info.IsDir()
		if err != nil || isWrongKind {
			return fmt.Sprintf("%s is not an existing %s", value, iface.Validate.Exists)
		}
	case skillgraph.KindSkill:
	}
	return ""
}

// ValueMap is the name → value view the render takes; unset variables are absent, the map is never nil.
func ValueMap(values []VariableValue) map[string]string {
	filled := make(map[string]string, len(values))
	for _, value := range values {
		if value.Value != "" {
			filled[value.Name] = value.Value
		}
	}
	return filled
}

// Values resolves every variable of skill from the environment, the record rows and the providers, validates each against dir, sorted by name; a deferring resolution leaves every repo-scoped one to invocation; a nil set fills every provider as if installed, a nil prober skips the handshake.
func Values(ctx context.Context, dir string, rows *Rows, isDeferring bool, prober Prober, set []components.ID, skill *skillgraph.Skill) []VariableValue {
	installed := map[components.ID]bool{}
	for _, id := range set {
		installed[id] = true
	}

	catalog := skill.Catalog
	if catalog == nil {
		catalog = &skillgraph.Catalog{}
	}

	repoScoped := skill.RepoScoped()
	var values []VariableValue
	for name, variable := range skill.VariableNames() {
		iface, isKnown := catalog.Interface(variable.Interface)
		value := VariableValue{Interface: iface, Name: name, Required: variable.Required}
		if isDeferring && repoScoped[name] {
			value.IsDeferred, value.Source = true, SourceInvocation
			values = append(values, value)
			continue
		}

		provider := catalog.ProviderFor(variable.Interface)
		value.Source, value.Value, value.Provider = resolveValue(iface, installed, set == nil, name, provider, rows)
		switch {
		case value.Value == "":
		case !isKnown:
			value.Cause = fmt.Sprintf("interface %s is not in %s", variable.Interface, skillgraph.FileCatalog)
		default:
			value.Cause = validateValue(ctx, dir, iface, prober, provider, &value)
		}
		if value.Cause != "" {
			value.Source, value.Value = SourceUnset, ""
		}
		values = append(values, value)
	}
	slices.SortFunc(values, func(a, b VariableValue) int { return strings.Compare(a.Name, b.Name) })
	return values
}
