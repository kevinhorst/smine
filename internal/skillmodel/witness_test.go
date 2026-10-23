package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWitness(t *testing.T) {
	output := "unsat-core-error\nsat\n(\n" +
		"  (define-fun interactive () Bool\n    false)\n" +
		"  (define-fun lanes_returned () Int\n    5)\n" +
		"  (define-fun lanes_spawned () Int\n    (- 3))\n" +
		"  (define-fun apply_level () apply_level_sort\n    medium)\n" +
		"  (define-fun t0_s0 () Bool\n    true)\n" +
		"  (define-fun apply_level_rank ((x apply_level_sort)) Int 0)\n" +
		")\n"
	witness := ParseWitness(output, typedVocabulary())

	expected := map[string]string{
		"apply_level":    "medium",
		"interactive":    "false",
		"lanes_returned": "5",
		"lanes_spawned":  "-3",
	}
	assert.Equal(t, expected, witness)
}

func TestParseWitnessUnsatBranch(t *testing.T) {
	assert.Empty(t, ParseWitness("unsat\n(error \"model is not available\")\n", typedVocabulary()))
}
