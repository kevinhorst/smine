package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExpr(t *testing.T) {
	type testCase struct {
		_id         string
		_atoms      []string
		_kind       ExprKind
		_shouldPass bool

		input string
	}
	tests := make([]*testCase, 0)

	// and-binds-over-or
	tests = append(tests, &testCase{
		_id:         "and-binds-over-or",
		_atoms:      []string{"a", "b", "c"},
		_kind:       ExprOr,
		_shouldPass: true,

		input: "a or b and c",
	})

	// implies-right-assoc
	tests = append(tests, &testCase{
		_id:         "implies-right-assoc",
		_atoms:      []string{"a", "b", "c"},
		_kind:       ExprImplies,
		_shouldPass: true,

		input: "a => b => c",
	})

	// not-binds-tightest
	tests = append(tests, &testCase{
		_id:         "not-binds-tightest",
		_atoms:      []string{"nightly", "gate"},
		_kind:       ExprAnd,
		_shouldPass: true,

		input: "not nightly and gate",
	})

	// parens-override
	tests = append(tests, &testCase{
		_id:         "parens-override",
		_atoms:      []string{"a", "b", "c"},
		_kind:       ExprAnd,
		_shouldPass: true,

		input: "(a or b) and c",
	})

	// bare-atom
	tests = append(tests, &testCase{
		_id:         "bare-atom",
		_atoms:      []string{"round_end"},
		_kind:       ExprAtom,
		_shouldPass: true,

		input: "round_end",
	})

	// unclosed-paren
	tests = append(tests, &testCase{
		_id:         "unclosed-paren",
		_shouldPass: false,

		input: "(a and b",
	})

	// dangling-operator
	tests = append(tests, &testCase{
		_id:         "dangling-operator",
		_shouldPass: false,

		input: "a and",
	})

	// uppercase-ident
	tests = append(tests, &testCase{
		_id:         "uppercase-ident",
		_shouldPass: false,

		input: "Nightly",
	})

	// empty
	tests = append(tests, &testCase{
		_id:         "empty",
		_shouldPass: false,

		input: "  ",
	})

	// int comparison
	tests = append(tests, &testCase{
		_id:         "int-comparison",
		_atoms:      []string{"remaining_budget", "round_cost"},
		_kind:       ExprCmp,
		_shouldPass: true,

		input: "remaining_budget < round_cost",
	})

	// arithmetic term with literal
	tests = append(tests, &testCase{
		_id:         "arith-term-literal",
		_atoms:      []string{"lanes_spawned", "lanes_returned"},
		_kind:       ExprCmp,
		_shouldPass: true,

		input: "lanes_spawned - lanes_returned >= 1",
	})

	// enum member equality
	tests = append(tests, &testCase{
		_id:         "enum-member-eq",
		_atoms:      []string{"mode", "nightly"},
		_kind:       ExprCmp,
		_shouldPass: true,

		input: "mode = nightly",
	})

	// upper-enum-member
	tests = append(tests, &testCase{
		_id:         "upper-enum-member",
		_atoms:      []string{"recommendation", "APPROVE"},
		_kind:       ExprCmp,
		_shouldPass: true,

		input: "recommendation != APPROVE",
	})

	// not binds the whole comparison
	tests = append(tests, &testCase{
		_id:         "not-over-comparison",
		_atoms:      []string{"remaining_budget", "round_cost"},
		_kind:       ExprNot,
		_shouldPass: true,

		input: "not remaining_budget < round_cost",
	})

	// bare literal is not a boolean operand
	tests = append(tests, &testCase{
		_id:         "bare-literal",
		_shouldPass: false,

		input: "42",
	})

	// bare arithmetic term needs a comparison
	tests = append(tests, &testCase{
		_id:         "bare-arith",
		_shouldPass: false,

		input: "a + b",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			expr, err := ParseExpr(test.input)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			if !test._shouldPass {
				return
			}
			require.NotNil(t, expr)
			assert.Equal(t, test._kind, expr.Kind)
			assert.Equal(t, test._atoms, expr.Atoms())
		})
	}
}
