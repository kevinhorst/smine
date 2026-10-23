// Package semver parses and orders the numeric versions skills and hook groups carry.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Parts is the component count of a full version; a shorter one counts its missing components as zero.
const Parts = 3

// Compare orders two versions numerically per component, a missing component counting as zero.
func Compare(version, other string) (int, error) {
	left, err := Parse(version)
	if err != nil {
		return 0, err
	}

	right, err := Parse(other)
	if err != nil {
		return 0, err
	}

	for index := range Parts {
		difference := ComponentAt(left, index) - ComponentAt(right, index)
		if difference < 0 {
			return -1, nil
		}
		if difference > 0 {
			return 1, nil
		}
	}
	return 0, nil
}

// ComponentAt reads one component, zero past the end.
func ComponentAt(components []int, index int) int {
	if index < len(components) {
		return components[index]
	}
	return 0
}

// Parse reads a two- or three-part version into its numeric components.
func Parse(version string) ([]int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > Parts {
		return nil, fmt.Errorf("Parse: Version %q has %d components, want 2 or 3", version, len(parts))
	}

	components := make([]int, 0, len(parts))
	for _, part := range parts {
		component, err := strconv.Atoi(part)
		if err != nil || component < 0 {
			return nil, fmt.Errorf("Parse: Version %q has a non-numeric component %q", version, part)
		}

		components = append(components, component)
	}
	return components, nil
}
