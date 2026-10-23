package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompleteBound(t *testing.T) {
	type testCase struct {
		_expectedBound int
		_id            string
		_shouldPass    bool

		extra []string
		model *Model
	}

	tests := make([]*testCase, 0)

	// linear-model
	tests = append(tests, &testCase{_expectedBound: 3, _id: "linear-model", _shouldPass: true, model: linearModel()})

	// assume-widens
	tests = append(tests, &testCase{_expectedBound: 4, _id: "assume-widens", _shouldPass: true, extra: []string{"logged"}, model: linearModel()})

	// exclusivity-closure
	closure := linearModel()
	closure.Vocabulary.Exclusive = [][]string{{"ready", "logged"}}
	tests = append(tests, &testCase{_expectedBound: 4, _id: "exclusivity-closure", _shouldPass: true, model: closure})

	// chained-groups
	chained := linearModel()
	chained.Vocabulary.Sorts["event"] = append(chained.Vocabulary.Sorts["event"], Atom{Id: "bridge"})
	chained.Vocabulary.Exclusive = [][]string{{"bridge", "logged"}, {"ready", "bridge"}}
	tests = append(tests, &testCase{_expectedBound: 4, _id: "chained-groups", _shouldPass: true, model: chained})

	// nil-extra
	tests = append(tests, &testCase{_expectedBound: 3, _id: "nil-extra", _shouldPass: true, extra: nil, model: linearModel()})

	// no-posts
	noPosts := linearModel()
	for index := range noPosts.Transitions {
		noPosts.Transitions[index].Post = nil
	}
	tests = append(tests, &testCase{_expectedBound: 1, _id: "no-posts", _shouldPass: true, model: noPosts})

	// fail-bad-pred
	badPred := linearModel()
	badPred.Transitions[1].Pred = "ready and"
	tests = append(tests, &testCase{_expectedBound: 0, _id: "fail-bad-pred", _shouldPass: false, model: badPred})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var extra []*Expr
			for _, text := range test.extra {
				extra = append(extra, mustParse(t, text))
			}
			bound, err := CompleteBound(extra, test.model)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedBound, bound)
		})
	}
}
