package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
)

// addComponentLine matches one AddComponent call of the Windows setup script;
// a quote inside the caption is Pascal-escaped by doubling it.
var addComponentLine = regexp.MustCompile(`AddComponent\('([a-z-]+)', '((?:[^']|'')*)', (True|False)\);`)

func TestIssComponentsMatchCatalog(t *testing.T) {
	script, err := os.ReadFile("../../installer/windows/smine.iss")
	require.NoError(t, err)

	var issIds, issCaptions []string
	var issEnabled []bool
	for _, match := range addComponentLine.FindAllStringSubmatch(string(script), -1) {
		issIds = append(issIds, match[1])
		issCaptions = append(issCaptions, strings.ReplaceAll(match[2], "''", "'"))
		issEnabled = append(issEnabled, match[3] == "True")
	}

	var catalogIds, catalogCaptions []string
	var catalogEnabled []bool
	for _, component := range components.Catalog() {
		catalogIds = append(catalogIds, string(component.ID))
		catalogCaptions = append(catalogCaptions, component.Title+" - "+component.Summary)
		catalogEnabled = append(catalogEnabled, components.Available(component.ID, "windows") && !component.Fixed)
	}

	// one-add-component-per-catalog-id-catalog-order
	assert.Equal(t, catalogIds, issIds)

	// caption-title-summary
	assert.Equal(t, catalogCaptions, issCaptions)

	// enabled-available-on-windows-and-not-fixed
	assert.Equal(t, catalogEnabled, issEnabled)
}
