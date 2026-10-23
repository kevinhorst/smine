package skillmodel

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrAnnotationTier: no query can resolve against a model without tier: "strict".
var ErrAnnotationTier = errors.New("queries need the model tier")

type QueryResult struct {
	Core     []string `json:"core"`
	Scenario []string `json:"scenario"`
	Verdict  string   `json:"verdict"`
}

// RunQuery asserts the axioms and the negated expr, soft clauses left out: unsat means expr is valid.
func RunQuery(ctx context.Context, model *Model, expr string) (*QueryResult, error) {
	if !model.IsStrict {
		return nil, fmt.Errorf("RunQuery: %s is not tier strict: %w", model.Skill, ErrAnnotationTier)
	}

	parsed, err := ParseExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("RunQuery: %w", err)
	}

	checker := &kindChecker{
		info:    model.Vocabulary.Info(),
		owners:  model.Vocabulary.MemberOwner(),
		subject: "query",
	}
	checker.walk(parsed)
	if len(checker.violations) > 0 {
		violations := strings.Join(checker.violations, "; ")
		return nil, fmt.Errorf("RunQuery: %s", violations)
	}

	encoding, err := newEncoding(model)
	if err != nil {
		return nil, fmt.Errorf("RunQuery: %w", err)
	}

	builder := &strings.Builder{}
	emitOptions(builder)
	emitAxioms(builder, encoding, model)
	negation := newEmitter(model.Vocabulary).emitExpr(parsed)
	fmt.Fprintf(builder, "(assert (! (not %s) :named query.negation))\n", negation)
	builder.WriteString("(check-sat)\n(get-unsat-core)\n(get-model)\n")
	doc := SmtDoc{Checks: 1, Text: builder.String()}
	result, err := Solve(ctx, doc, model)
	if err != nil {
		return nil, fmt.Errorf("RunQuery: %w", err)
	}

	return result, nil
}
