package skillmodel

import (
	"regexp"
	"strings"
)

var witnessPattern = regexp.MustCompile(`\(define-fun ([a-z][a-z0-9_]*) \(\) [A-Za-z0-9_]+ ((?:\(- [0-9]+\))|[a-z0-9_-]+)\)`)

// ParseWitness skips non-vocabulary symbols: rank functions, occurrence variables, step copies.
func ParseWitness(output string, vocab *Vocabulary) map[string]string {
	atomIds := vocab.AtomIds()
	witness := map[string]string{}
	collapsed := strings.Join(strings.Fields(output), " ")
	for _, match := range witnessPattern.FindAllStringSubmatch(collapsed, -1) {
		name := match[1]
		if !atomIds[name] {
			continue
		}
		value := match[2]
		if magnitude, ok := strings.CutPrefix(value, "(- "); ok {
			value = "-" + strings.TrimSuffix(magnitude, ")")
		}
		witness[name] = value
	}
	return witness
}
