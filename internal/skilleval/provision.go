package skilleval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routinerun"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	armWorktreePrefix  = "run-"
	defaultWtRootDir   = ".cache/claude-routine/worktrees"
	prebuildLogFile    = "prebuild.log"
	prebuildTimeout    = 15 * time.Minute
	referencePatchFile = "reference.patch"
	wtRootEnv          = "SKILLEVAL_WT_ROOT"
)

// Provision intake stops cmd/skilleval maps to their own exit codes.
var (
	ErrUnregisteredRepo = errors.New("repo name has no repos.json entry")
	ErrUnresolvableRef  = errors.New("ref does not resolve in the repo")
)

func mainCheckoutRoot(ctx context.Context) (string, error) {
	commonDir, err := shell.Run(ctx, "", "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("mainCheckoutRoot: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(commonDir)), nil
}

// prebuildArm refuses a missing declared output because the arm's workers link these outputs and build nothing.
func prebuildArm(ctx context.Context, arm *ManifestArm, prebuild *evals.DefinitionPrebuild) error {
	if prebuild == nil {
		return nil
	}

	outDir := arm.Worktree + outDirSuffix
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("prebuildArm: %w", err)
	}

	logOut, err := os.Create(filepath.Join(outDir, prebuildLogFile))
	if err != nil {
		return fmt.Errorf("prebuildArm: %w", err)
	}
	defer logOut.Close()

	spec := &routinerun.CellSpec{
		Args:    prebuild.Command,
		Dir:     arm.Worktree,
		Stderr:  logOut,
		Stdout:  logOut,
		Timeout: prebuildTimeout,
	}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		return fmt.Errorf("prebuildArm: %w", err)
	}

	if status != 0 {
		return fmt.Errorf("prebuildArm: Arm %s: Prebuild exited %d, see %s", arm.Label, status, logOut.Name())
	}

	for _, relative := range prebuild.Outputs {
		if _, err := os.Stat(filepath.Join(arm.Worktree, relative)); err != nil {
			return fmt.Errorf("prebuildArm: Arm %s: Output %s is missing: %w", arm.Label, relative, err)
		}
	}
	return nil
}

// resolveRef turns ref into its full SHA in repo; "" stays "".
func resolveRef(ctx context.Context, ref, repo string) (string, error) {
	if ref == "" {
		return "", nil
	}

	sha, err := shell.Run(ctx, repo, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolveRef: %s: %w", ref, ErrUnresolvableRef)
	}
	return strings.TrimSpace(sha), nil
}

// resolveRepo maps no name to the main checkout itself.
func resolveRepo(mainCheckout, name, registryPath string) (string, error) {
	if name == "" {
		return mainCheckout, nil
	}

	registry := repos.NewRegistry(registryPath)
	if err := registry.Reload(); err != nil {
		return "", fmt.Errorf("resolveRepo: %w", err)
	}

	repo, isFound := registry.Find(name)
	if !isFound {
		return "", fmt.Errorf("resolveRepo: %s: %w", name, ErrUnregisteredRepo)
	}
	return repo.Path, nil
}

// worktreeRoot is $SKILLEVAL_WT_ROOT, else ~/.cache/claude-routine/worktrees.
func worktreeRoot() (string, error) {
	if root := os.Getenv(wtRootEnv); root != "" {
		return root, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("worktreeRoot: %w", err)
	}
	return filepath.Join(home, defaultWtRootDir), nil
}

// writeReferencePatch writes the known-good reference commit as the compare judge's read-only diff.
func writeReferencePatch(ctx context.Context, manifest *RunManifest, runDir string) error {
	if manifest.Reference == "" {
		return nil
	}

	patch, err := shell.Run(ctx, manifest.Repo, "git", "show", "--binary", manifest.Reference)
	if err != nil {
		return fmt.Errorf("writeReferencePatch: %w", err)
	}
	if _, err := gitwt.WriteIfNonEmpty(patch, filepath.Join(runDir, referencePatchFile)); err != nil {
		return fmt.Errorf("writeReferencePatch: %w", err)
	}
	return nil
}

// checkModels checks arms as root sessions and the judge as a subagent.
func checkModels(manifest *RunManifest) error {
	vocabulary, err := models.LoadHome(filepath.Dir(config.DefaultPath()))
	if err != nil {
		return fmt.Errorf("checkModels: %w", err)
	}

	for _, arm := range manifest.Arms {
		if arm.Model == "" {
			continue
		}
		if err := vocabulary.Check(arm.Model); err != nil {
			return fmt.Errorf("checkModels: Arm %s: %w", arm.Label, err)
		}
	}
	if manifest.Judge == "" {
		return nil
	}
	if err := vocabulary.CheckSubagent(manifest.Judge); err != nil {
		return fmt.Errorf("checkModels: Judge: %w", err)
	}
	return nil
}

// Provision leaves partial worktrees of a failed run for the run's own cleanup.
func Provision(ctx context.Context, runDir string) error {
	manifest, err := loadManifest(runDir)
	if err != nil {
		return err
	}

	if err := checkModels(manifest); err != nil {
		return err
	}

	mainCheckout, err := mainCheckoutRoot(ctx)
	if err != nil {
		return err
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		return fmt.Errorf("Provision: %w", err)
	}

	registryPath := filepath.Join(stateDir, config.StateRegistryFile)
	manifest.Repo, err = resolveRepo(mainCheckout, manifest.RepoName, registryPath)
	if err != nil {
		return err
	}

	if manifest.At == "" {
		manifest.At = "HEAD"
	}
	if manifest.At, err = resolveRef(ctx, manifest.At, manifest.Repo); err != nil {
		return err
	}
	if manifest.Reference, err = resolveRef(ctx, manifest.Reference, manifest.Repo); err != nil {
		return err
	}

	wtRoot, err := worktreeRoot()
	if err != nil {
		return err
	}

	for index := range manifest.Arms {
		arm := &manifest.Arms[index]
		runId := armRunId(arm.Label, runDir)
		request := &gitwt.ProvisionRequest{
			Branch:  armBranchPrefix + runId,
			Head:    manifest.At,
			Overlay: manifest.armOverlay(arm),
			Repo:    manifest.Repo,
			Root:    wtRoot,
			Slug:    armWorktreePrefix + runId,
		}
		provisioned, err := gitwt.Provision(ctx, request)
		if err != nil {
			return fmt.Errorf("Provision: Arm %s: %w", arm.Label, err)
		}
		arm.Worktree = provisioned.Path
		// The generated context index is gitignored, so a fresh arm worktree lacks it; the session worktree hook writes it the same way.
		if err := contextdocs.Regenerate(filepath.Join(provisioned.Path, contextdocs.SourceContextDir)); err != nil {
			return fmt.Errorf("Provision: Arm %s: %w", arm.Label, err)
		}
		if err := prebuildArm(ctx, arm, manifest.Prebuild); err != nil {
			return err
		}
	}

	if err := writeReferencePatch(ctx, manifest, runDir); err != nil {
		return err
	}
	return writeJsonFile(filepath.Join(runDir, provisionFile), newProvisionRecord(manifest))
}
