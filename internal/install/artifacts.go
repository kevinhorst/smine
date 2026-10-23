package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

// ActionKept is a step that writes nothing: the target differs from smine's last write, so it is the user's.
const ActionKept = "kept"

// ActionRecord is a step that writes only the owned digest of a guarded target already holding the render.
const ActionRecord = "record"

const goosWindows = "windows"

// binaryKind says where an installed binary lands.
type binaryKind int

const (
	binaryHook binaryKind = iota // ~/.claude/hooks/bin/<name>
	binaryPath                   // <smine bin dir>/smine-<name>: ~/go/bin, or %LOCALAPPDATA%\smine\bin on windows
)

// artifactComponent is the one Component implementation, driven entirely by its declared artifacts.
type artifactComponent struct {
	binaries []binary
	extras   []extra
	files    []file
	id       components.ID
	probe    func(ctx context.Context, env Env) Health
	rendered []renderedFile
}

// converge brings every artifact to its wanted state; without isWrite it only reports the steps.
func (c *artifactComponent) converge(ctx context.Context, env Env, isWrite bool) ([]Step, error) {
	buildDir, err := os.MkdirTemp("", "smine-install-*")
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.converge: %w", err)
	}
	defer os.RemoveAll(buildDir)

	writes, err := c.wantedFiles(ctx, env, buildDir)
	if err != nil {
		return nil, err
	}

	// Extras judge the deployed state before the writes: the heartbeat restart compares the old config file.
	due, err := c.dueExtras(ctx, env, true)
	if err != nil {
		return nil, err
	}

	var steps []Step
	for _, write := range writes {
		action, err := write.apply(env, isWrite)
		if err != nil {
			return steps, err
		}
		if action != "" {
			steps = append(steps, c.step(action, write.target))
		}
	}

	settingSteps, err := c.reconcileSettings(env, true, isWrite)
	steps = append(steps, settingSteps...)
	if err != nil {
		return steps, err
	}

	extraSteps, err := c.runExtras(ctx, env, due, true, isWrite)
	return append(steps, extraSteps...), err
}

// dueExtras are the extras due for the selected or the removed state.
func (c *artifactComponent) dueExtras(ctx context.Context, env Env, isSelected bool) ([]extra, error) {
	var due []extra
	for _, item := range c.extras {
		isDue, err := item.isDue(ctx, env, isSelected)
		if err != nil {
			return nil, fmt.Errorf("artifactComponent.dueExtras: %s: %w", item.detail, err)
		}
		if isDue {
			due = append(due, item)
		}
	}
	return due, nil
}

// installedTargets are the binary, file and rendered paths of the component.
func (c *artifactComponent) installedTargets(env Env) []string {
	var targets []string
	for _, b := range c.binaries {
		targets = append(targets, b.target(env))
	}
	for _, f := range c.files {
		targets = append(targets, filepath.Join(env.ClaudeHome, f.target))
	}
	for _, f := range c.rendered {
		targets = append(targets, filepath.Join(env.ClaudeHome, f.target))
	}
	return targets
}

// isOwnedTarget is retire's proof: a copied file by the owned digest, a rendered file or binary by name (master C2, C3).
func (c *artifactComponent) isOwnedTarget(env Env, target string) (bool, error) {
	current, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("artifactComponent.isOwnedTarget: %w", err)
	}

	for _, f := range c.files {
		if filepath.Join(env.ClaudeHome, f.target) != target {
			continue
		}
		return fileWrite{isGuarded: true, target: target}.isOurs(env, current)
	}
	return true, nil
}

// missing is the cheap presence check Health runs, never a build.
func (c *artifactComponent) missing(env Env) ([]string, error) {
	var missing []string
	for _, target := range c.installedTargets(env) {
		if _, err := os.Stat(target); err != nil {
			missing = append(missing, target)
		}
	}

	settingSteps, err := c.reconcileSettings(env, true, false)
	for _, step := range settingSteps {
		missing = append(missing, step.Detail)
	}
	return missing, err
}

// reconcileSettings moves the component's fragment entries in or out of the live settings documents.
func (c *artifactComponent) reconcileSettings(env Env, isSelected, isWrite bool) ([]Step, error) {
	fragment, err := config.LoadComponentFragment(settingsDir(env), string(c.id), env.markers())
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}

	files, err := loadSettingsFiles(env)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}

	files.component, files.record = c.id, record
	changes, err := files.reconcile(fragment, isSelected)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}

	retired, err := retiredHookIds(env)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}
	pruneChanges, err := files.prune(retired)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.reconcileSettings: %w", err)
	}
	changes = append(changes, pruneChanges...)

	action := ActionInstall
	if !isSelected {
		action = ActionRemove
	}
	var steps []Step
	for _, change := range changes {
		steps = append(steps, c.step(action, change))
	}
	if isWrite && len(changes) > 0 {
		err = files.save(env)
	}
	return steps, err
}

// retire moves every owned artifact of the component into the trash; without isWrite it only reports the steps.
func (c *artifactComponent) retire(ctx context.Context, env Env, isWrite bool) ([]Step, error) {
	var steps []Step
	for _, target := range c.installedTargets(env) {
		isOwned, err := c.isOwnedTarget(env, target)
		if err != nil {
			return steps, err
		}
		if !isOwned {
			if _, statErr := os.Stat(target); statErr == nil {
				steps = append(steps, c.step(ActionKept, target))
			}
			continue
		}

		steps = append(steps, c.step(ActionRemove, target))
		if !isWrite {
			continue
		}
		entry := &trash.Entry{Action: "retire " + string(c.id), Reason: reasonComponentOff}
		if _, err := env.Trash.Move(target, entry); err != nil {
			return steps, fmt.Errorf("artifactComponent.retire: %w", err)
		}
		if err := env.Trash.ForgetOwned(target); err != nil {
			return steps, fmt.Errorf("artifactComponent.retire: %w", err)
		}
	}

	settingSteps, err := c.reconcileSettings(env, false, isWrite)
	steps = append(steps, settingSteps...)
	if err != nil {
		return steps, err
	}

	due, err := c.dueExtras(ctx, env, false)
	if err != nil {
		return steps, err
	}
	extraSteps, err := c.runExtras(ctx, env, due, false, isWrite)
	return append(steps, extraSteps...), err
}

// runExtras runs the due extras for the selected or the removed state.
func (c *artifactComponent) runExtras(ctx context.Context, env Env, due []extra, isSelected, isWrite bool) ([]Step, error) {
	var steps []Step
	for _, item := range due {
		steps = append(steps, c.step(extraAction(isSelected), item.detail))
		if !isWrite {
			continue
		}

		if err := item.run(ctx, env, isSelected); err != nil {
			return steps, fmt.Errorf("artifactComponent.runExtras: %s: %w", item.detail, err)
		}
	}
	return steps, nil
}

func (c *artifactComponent) step(action, detail string) Step {
	return Step{Action: action, Component: c.id, Detail: detail}
}

// hookWrites are the hook binaries, the part the hooks sync target refreshes on its own.
func (c *artifactComponent) hookWrites(ctx context.Context, env Env, buildDir string) ([]fileWrite, error) {
	var writes []fileWrite
	for _, binary := range c.binaries {
		if binary.kind != binaryHook {
			continue
		}

		write, err := binary.write(ctx, env, buildDir)
		if err != nil {
			return nil, err
		}
		writes = append(writes, write)
	}

	hooksConfigWrites, err := c.renderedWrites(env, true)
	if err != nil {
		return nil, err
	}
	return append(writes, hooksConfigWrites...), nil
}

// renderedWrites renders the component's hooks config files, or every other rendered file without isHooksConfig.
func (c *artifactComponent) renderedWrites(env Env, isHooksConfig bool) ([]fileWrite, error) {
	if len(c.rendered) == 0 {
		return nil, nil
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.renderedWrites: %w", err)
	}

	var writes []fileWrite
	for _, f := range c.rendered {
		if (f.sink == components.SinkHooksConfig) != isHooksConfig {
			continue
		}

		write, err := f.write(env, record)
		if err != nil {
			return nil, fmt.Errorf("artifactComponent.renderedWrites: %w", err)
		}
		writes = append(writes, write)
	}
	return writes, nil
}

// wantedFiles resolves the bytes every binary and file should hold.
func (c *artifactComponent) wantedFiles(ctx context.Context, env Env, buildDir string) ([]fileWrite, error) {
	writes, err := c.hookWrites(ctx, env, buildDir)
	if err != nil {
		return nil, err
	}

	for _, binary := range c.binaries {
		if binary.kind == binaryHook {
			continue
		}

		write, err := binary.write(ctx, env, buildDir)
		if err != nil {
			return nil, err
		}
		writes = append(writes, write)
	}

	for _, f := range c.files {
		data, err := os.ReadFile(filepath.Join(env.RepoDir, f.source))
		if err != nil {
			return nil, fmt.Errorf("artifactComponent.wantedFiles: %w", err)
		}
		write := fileWrite{data: env.markers().Expand(data), dirPerm: f.dirPerm, isGuarded: true, perm: 0o644, target: filepath.Join(env.ClaudeHome, f.target)}
		writes = append(writes, write)
	}

	renderedWrites, err := c.renderedWrites(env, false)
	if err != nil {
		return nil, err
	}
	return append(writes, renderedWrites...), nil
}

func (c *artifactComponent) Health(ctx context.Context, env Env) Health {
	missing, err := c.missing(env)
	if err != nil {
		return Health{Detail: err.Error(), Fix: fixReapply}
	}
	if len(missing) > 0 {
		return Health{Detail: "missing: " + strings.Join(missing, ", "), Fix: fixReapply}
	}
	if c.probe != nil {
		return c.probe(ctx, env)
	}
	return Health{Detail: "installed", Ok: true}
}

func (c *artifactComponent) ID() components.ID {
	return c.id
}

func (c *artifactComponent) Install(ctx context.Context, env Env) error {
	_, err := c.converge(ctx, env, true)
	return err
}

// InstallHooks writes only the hook binaries; settings, files and extras stay with Install.
func (c *artifactComponent) InstallHooks(ctx context.Context, env Env) ([]Step, error) {
	buildDir, err := os.MkdirTemp("", "smine-install-*")
	if err != nil {
		return nil, fmt.Errorf("artifactComponent.InstallHooks: %w", err)
	}
	defer os.RemoveAll(buildDir)

	writes, err := c.hookWrites(ctx, env, buildDir)
	if err != nil {
		return nil, err
	}

	var steps []Step
	for _, write := range writes {
		action, err := write.apply(env, true)
		if err != nil {
			return steps, err
		}
		if action != "" {
			steps = append(steps, c.step(action, write.target))
		}
	}
	return steps, nil
}

func (c *artifactComponent) Plan(ctx context.Context, env Env, selected bool) ([]Step, error) {
	if selected {
		return c.converge(ctx, env, false)
	}
	return c.retire(ctx, env, false)
}

func (c *artifactComponent) Remove(ctx context.Context, env Env) error {
	_, err := c.retire(ctx, env, true)
	return err
}

// binary is one program a component installs, Rust prebuilt in bin/ and Go built from cmd/<name>.
type binary struct {
	isRust bool
	kind   binaryKind
	name   string
}

// prebuilt reads the checkout's bin/<name>: a Rust binary always, every binary of a release archive.
func (b binary) prebuilt(env Env) ([]byte, error) {
	path := filepath.Join(env.RepoDir, "bin", b.fileName(env.GOOS))
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if b.isRust {
			return nil, fmt.Errorf("binary.prebuilt: %s missing — run make rust-build (needs cargo)", path)
		}
		return nil, fmt.Errorf("binary.prebuilt: %s missing from the release archive", path)
	}
	if err != nil {
		return nil, fmt.Errorf("binary.prebuilt: %w", err)
	}
	return data, nil
}

func (b binary) fileName(goos string) string {
	if goos == goosWindows {
		return b.name + ".exe"
	}
	return b.name
}

// source builds a Go binary from cmd/<name> in a development checkout; a release archive (bin/PAYLOAD) ships every binary prebuilt.
func (b binary) source(ctx context.Context, env Env, buildDir string) ([]byte, error) {
	if b.isRust || env.IsPayload {
		return b.prebuilt(env)
	}

	if _, err := exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("binary.source: go not on PATH and no %s: %w", PayloadMarkerPath(env.RepoDir), err)
	}

	output := filepath.Join(buildDir, b.name)
	if out, err := shell.RunSync(ctx, env.RepoDir, "go", "build", "-o", output, "./cmd/"+b.name); err != nil {
		return nil, fmt.Errorf("binary.source: go build %s: %w\n%s", b.name, err, out)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		return nil, fmt.Errorf("binary.source: %w", err)
	}
	return data, nil
}

func (b binary) write(ctx context.Context, env Env, buildDir string) (fileWrite, error) {
	data, err := b.source(ctx, env, buildDir)
	if err != nil {
		return fileWrite{}, err
	}
	write := fileWrite{data: data, dirPerm: 0o755, isBinary: true, perm: 0o755, target: b.target(env)}
	return write, nil
}

func (b binary) target(env Env) string {
	switch b.kind {
	case binaryHook:
		return filepath.Join(env.ClaudeHome, "hooks", "bin", b.name)
	case binaryPath:
		return filepath.Join(config.SmineBinDir(env.GOOS, env.Home), "smine-"+b.fileName(env.GOOS))
	}
	return ""
}

// extra is one custom step a component owns beside its plain artifacts; a nil remove makes isDue answer false for the removed state.
type extra struct {
	detail  string
	install func(ctx context.Context, env Env) error
	isDue   func(ctx context.Context, env Env, isSelected bool) (bool, error)
	remove  func(ctx context.Context, env Env) error
}

func (x extra) run(ctx context.Context, env Env, isSelected bool) error {
	if isSelected {
		return x.install(ctx, env)
	}
	return x.remove(ctx, env)
}

// file is one repo file copied into the Claude home, markers expanded.
type file struct {
	dirPerm os.FileMode
	source  string // repo-relative
	target  string // Claude-home-relative
}

// fileWrite is one target with the bytes it should hold.
type fileWrite struct {
	data    []byte
	dirPerm os.FileMode
	// isBinary targets are replaced without a stash; isGuarded ones only while unchanged since smine wrote them.
	isBinary  bool
	isGuarded bool
	perm      os.FileMode
	target    string
}

// apply returns install, update, kept or "" for an equal target, writing only when isWrite (master C1, C3, C4).
func (w fileWrite) apply(env Env, isWrite bool) (string, error) {
	current, err := os.ReadFile(w.target)
	action := ActionUpdate
	switch {
	case errors.Is(err, fs.ErrNotExist):
		action = ActionInstall
	case err != nil:
		return "", fmt.Errorf("fileWrite.apply: %w", err)
	case bytes.Equal(current, w.data):
		return w.recordEqual(env, isWrite)
	}
	if action == ActionUpdate {
		isOurs, err := w.isOurs(env, current)
		if err != nil {
			return "", err
		}
		if !isOurs {
			return ActionKept, nil
		}
	}
	if !isWrite {
		return action, nil
	}

	if err := os.MkdirAll(filepath.Dir(w.target), w.dirPerm); err != nil {
		return action, fmt.Errorf("fileWrite.apply: %w", err)
	}
	if action == ActionUpdate && !w.isBinary {
		entry := &trash.Entry{Action: "apply", Reason: "artifact rewritten"}
		if _, err := env.Trash.Keep(w.target, entry); err != nil {
			return action, fmt.Errorf("fileWrite.apply: %w", err)
		}
	}
	if err := fsx.WriteFileAtomic(w.data, w.target, w.perm); err != nil {
		return action, fmt.Errorf("fileWrite.apply: %w", err)
	}
	if w.isGuarded {
		if err := env.Trash.RecordOwned(w.target, w.data); err != nil {
			return action, fmt.Errorf("fileWrite.apply: %w", err)
		}
	}
	return action, nil
}

// isOurs is the ownership proof of a present target: a guarded copy by the owned digest, everything else by name.
func (w fileWrite) isOurs(env Env, current []byte) (bool, error) {
	switch {
	case w.isGuarded:
		isUnchanged, err := env.Trash.IsUnchanged(w.target)
		if err != nil {
			return false, fmt.Errorf("fileWrite.isOurs: %w", err)
		}
		return isUnchanged, nil
	}
	return true, nil
}

// recordEqual is the step of a target that already holds the render: a guarded one without a digest is a record step, since equal bytes prove it ours and its next update must not be refused as foreign.
func (w fileWrite) recordEqual(env Env, isWrite bool) (string, error) {
	if !w.isGuarded {
		return "", nil
	}
	isUnchanged, err := env.Trash.IsUnchanged(w.target)
	if err != nil {
		return "", fmt.Errorf("fileWrite.recordEqual: %w", err)
	}
	if isUnchanged {
		return "", nil
	}
	if !isWrite {
		return ActionRecord, nil
	}
	if err := env.Trash.RecordOwned(w.target, w.data); err != nil {
		return ActionRecord, fmt.Errorf("fileWrite.recordEqual: %w", err)
	}
	return ActionRecord, nil
}

func extraAction(isSelected bool) string {
	if isSelected {
		return ActionInstall
	}
	return ActionRemove
}
