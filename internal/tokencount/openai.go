package tokencount

import "strings"

// o200kModelPrefixes mirror tiktoken's model map; other families count approximate until it maps them.
var o200kModelPrefixes = []string{"gpt-4.1", "gpt-4o", "gpt-5", "o1", "o3", "o4"}

func encodingApprox(model string) bool {
	for _, prefix := range o200kModelPrefixes {
		if strings.HasPrefix(model, prefix) {
			return false
		}
	}
	return true
}
