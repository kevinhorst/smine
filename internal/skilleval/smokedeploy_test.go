package skilleval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The deploy marker records the source hash; only a change that ships moves it.
func TestCheckDeployed(t *testing.T) {
	skillsSource, err := filepath.Abs(filepath.Join("..", "..", smokeSkillsDir))
	require.NoError(t, err)
	fixture := newSmokeFixture(t, nil, skillsSource)
	fixture.enter(t)

	assert.NoError(t, checkDeployed(fixture.skillDir()), "fresh-deploy-current")

	writeTestFile(t, filepath.Join(fixture.skillDir(), "eval", "definition.json"), "{}\n")
	assert.NoError(t, checkDeployed(fixture.skillDir()), "eval-edit-still-current")

	writeTestFile(t, filepath.Join(fixture.skillDir(), "README.md"), "edited\n")
	assert.ErrorIs(t, checkDeployed(fixture.skillDir()), ErrDeployedDiffers, "source-edit-differs")

	require.NoError(t, os.Remove(filepath.Join(fixture.deployedDir(), ".smine-deployed")))
	assert.ErrorIs(t, checkDeployed(fixture.skillDir()), ErrDeployedDiffers, "marker-missing-differs")
}
