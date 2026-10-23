package acdsl

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// The match includes the leading space so replacement and removal are symmetric.
var reachAttrRe = regexp.MustCompile(` reach="[^"]*"`)

// A missing attribute is inserted after the verifier token; the rest of the file stays byte-identical.
func SetRuleReach(path string, line int, value string) error {
	if !reach.Valid(value) {
		return fmt.Errorf("SetRuleReach: invalid reach %q", value)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("SetRuleReach: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return fmt.Errorf("SetRuleReach: %s has no line %d", path, line)
	}
	marker := lines[line-1]
	if !strings.Contains(marker, Marker) {
		return fmt.Errorf("SetRuleReach: %s:%d is not an acdsl marker line", path, line)
	}
	if reachAttrRe.MatchString(marker) {
		marker = reachAttrRe.ReplaceAllString(marker, ` reach="`+value+`"`)
	} else {
		fields := strings.SplitN(marker, " ", 3)
		if len(fields) < 2 {
			return fmt.Errorf("SetRuleReach: %s:%d: marker has no verifier token", path, line)
		}
		insert := fields[0] + " " + fields[1] + ` reach="` + value + `"`
		if len(fields) == 3 {
			insert += " " + fields[2]
		}
		marker = insert
	}
	lines[line-1] = marker
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fmt.Errorf("SetRuleReach: %w", err)
	}
	return nil
}
