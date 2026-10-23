package server

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

// unsetReposNamed caps the repos an unset label names; the rest is a count, so a badge title stays one short line.
const unsetReposNamed = 3

// unsetVariables resolves a repo skill's values for the pages that carry no env widget; a home skill or an unloadable model has none.
func (s *Server) unsetVariables(ctx context.Context, skill *skills.Skill) []string {
	if skill.Origin != skills.OriginRepo {
		return nil
	}

	model, err := skillgraph.Load(skill.Path)
	if err != nil {
		return nil
	}

	values, rows := s.variableValues(ctx, model)
	return unsetRequired(model, values, rows, s.repoRegistry.Repos())
}

// variableValues resolves a repo skill's variables as the deploy does — the record's rows, the component set and one prober per call, deferral on — and returns the rows it read.
func (s *Server) variableValues(ctx context.Context, model *skillgraph.Skill) ([]skills.VariableValue, *skills.Rows) {
	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.variableValues: %v", err)
	}

	rows := install.SkillRows(record)
	set := install.ExpandedComponents(record, s.machine.GOOS)
	values := skills.Values(ctx, s.repoDir, rows, true, skills.NewProber(), set, model)
	return values, rows
}

// unsetRequired names the required variables nothing fills, sorted: an install-scoped one without a value, a repo-scoped one with neither a default row nor a global default that some registered repo lacks — the invocation aborts there.
func unsetRequired(model *skillgraph.Skill, values []skills.VariableValue, rows *skills.Rows, registered []repos.Repo) []string {
	deferred := make(map[string]bool, len(values))
	byName := make(map[string]string, len(values))
	for _, value := range values {
		deferred[value.Name] = value.IsDeferred
		byName[value.Name] = value.Value
	}
	unset := skillgraph.UnconfiguredVariables(deferred, model, byName)
	for _, value := range values {
		isRepoScopedRequired := value.IsDeferred && value.Required
		hasDefault := rows.Skills[value.Name] != "" || skills.GlobalDefault(value.Interface, rows) != ""
		if !isRepoScopedRequired || hasDefault {
			continue
		}

		var lacking []string
		for _, repo := range registered {
			if repo.Env[value.Name] == "" {
				lacking = append(lacking, repo.Name)
			}
		}
		if len(registered) == 0 || len(lacking) > 0 {
			unset = append(unset, unsetLabel(lacking, value.Name, len(registered)))
		}
	}
	slices.Sort(unset)
	return unset
}

// unsetLabel is the variable name, followed by the registered repos lacking it — the first few named, the rest counted.
func unsetLabel(lacking []string, name string, registered int) string {
	if len(lacking) == 0 {
		return name
	}

	named := lacking
	rest := ""
	if len(lacking) > unsetReposNamed {
		named = lacking[:unsetReposNamed]
		rest = fmt.Sprintf(", +%d more", len(lacking)-unsetReposNamed)
	}
	return fmt.Sprintf("%s (missing in %d of %d repos: %s%s)", name, len(lacking), registered, strings.Join(named, ", "), rest)
}
