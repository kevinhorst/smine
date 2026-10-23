package backlog

import "regexp"

var (
	// Skills outside the map leave the status untouched.
	dispatchStages = map[string]string{
		"clarify":             StatusPlanning,
		"concept":             StatusPlanning,
		"fdesign":             StatusPlanning,
		"fexplore":            StatusPlanning,
		"fimplement":          StatusImplementing,
		"idea":                StatusPlanning,
		"railroad-review":     StatusReview,
		"skillroutine-create": StatusImplementing,
	}
	itemIdRe = regexp.MustCompile(`\b(B-\d+):`)
)

func ItemIdOf(prompt string) string {
	match := itemIdRe.FindStringSubmatch(prompt)
	if match == nil {
		return ""
	}
	return match[1]
}

func StageForSkill(skill string) string {
	return dispatchStages[skill]
}
