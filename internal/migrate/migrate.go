// Package migrate moves machine state and stores out of the main checkout once, never overwriting a target.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	OutcomeAbsent      Outcome = "absent"
	OutcomeCopied      Outcome = "copied"
	OutcomeInitialized Outcome = "initialized"
	OutcomeKept        Outcome = "kept"
	OutcomeMoved       Outcome = "moved"
	OutcomeRestored    Outcome = "restored"
)

const (
	// gitInitSource names the archive step's source in its report line.
	gitInitSource = "git init"
	// migratedFileMode is the mode of every copied data file.
	migratedFileMode = 0o644
	// storeFileExtension is the extension of the backlog and the kind files.
	storeFileExtension = ".json"
)

// The set the code repo tracked when it dropped the stores; one path in checkout, history and store root.
var storeFiles = []string{
	path.Join(config.StoreBacklogDir, "backlog"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "acdsl"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "archive", "done.md"),
	path.Join(config.StoreProposalsDir, "archive", "postponed.md"),
	path.Join(config.StoreProposalsDir, "archive", "rejected.md"),
	path.Join(config.StoreProposalsDir, "context"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "permissions"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "problems"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "routines"+storeFileExtension),
	path.Join(config.StoreProposalsDir, "skills"+storeFileExtension),
}

// Outcome is what a migration did with one file.
type Outcome string

// Request names the three roots a migration works between.
type Request struct {
	MainCheckout string
	StateDir     string
	StoreRoot    string
}

func NewRequest(checkoutDir string) (*Request, error) {
	mainCheckout, err := filepath.Abs(checkoutDir)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}

	storeRoot, err := config.StoreRoot(mainCheckout)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}

	request := &Request{
		MainCheckout: mainCheckout,
		StateDir:     stateDir,
		StoreRoot:    storeRoot,
	}
	return request, nil
}

// On an error it returns the steps finished so far, so the caller can report what moved.
func (r *Request) Run(ctx context.Context) ([]*Step, error) {
	steps := make([]*Step, 0)
	for _, step := range r.moveSteps() {
		if err := move(step); err != nil {
			return steps, fmt.Errorf("Request.Run: %w", err)
		}

		steps = append(steps, step)
	}

	archiveStep, err := r.initArchive(ctx)
	if err != nil {
		return steps, fmt.Errorf("Request.Run: %w", err)
	}

	steps = append(steps, archiveStep)
	for _, step := range r.storeSteps() {
		if err := r.storeFile(ctx, step); err != nil {
			return steps, fmt.Errorf("Request.Run: %w", err)
		}

		steps = append(steps, step)
	}

	return steps, nil
}

func (r *Request) initArchive(ctx context.Context) (*Step, error) {
	archive := filepath.Dir(r.StoreRoot)
	step := &Step{Source: gitInitSource, Target: archive}
	isRepo, err := isPresent(filepath.Join(archive, ".git"))
	if err != nil {
		return nil, fmt.Errorf("Request.initArchive: %w", err)
	}

	if isRepo {
		step.Outcome = OutcomeKept
		return step, nil
	}

	if err := os.MkdirAll(archive, 0o755); err != nil {
		return nil, fmt.Errorf("Request.initArchive: %w", err)
	}

	output, err := shell.Run(ctx, archive, "git", "init", "--quiet")
	if err != nil {
		return nil, fmt.Errorf("Request.initArchive: Failed to init %s: %w: %s", archive, err, strings.TrimSpace(output))
	}

	step.Outcome = OutcomeInitialized
	return step, nil
}

// moveSteps lists the checkout files that move into the state dir.
func (r *Request) moveSteps() []*Step {
	registry := &Step{
		Source: filepath.Join(r.MainCheckout, config.StateRegistryFile),
		Target: filepath.Join(r.StateDir, config.StateRegistryFile),
	}
	return []*Step{registry}
}

// Reads the first parent of the last first-parent commit that deleted the file.
func (r *Request) restoreFile(ctx context.Context, step *Step) error {
	output, err := shell.Run(
		ctx,
		r.MainCheckout,
		"git",
		"log",
		"--first-parent",
		"--diff-filter=D",
		"-1",
		"--format=%H",
		"--",
		step.Source,
	)
	if err != nil {
		return fmt.Errorf("Request.restoreFile:Failed to find the deletion of %s: %w: %s", step.Source, err, strings.TrimSpace(output))
	}

	deletion := strings.TrimSpace(output)
	if deletion == "" {
		step.Outcome = OutcomeAbsent
		return nil
	}

	revision := deletion + "^:" + step.Source
	content, err := shell.Run(ctx, r.MainCheckout, "git", "show", revision)
	if err != nil {
		return fmt.Errorf("Request.restoreFile:Failed to read %s: %w: %s", revision, err, strings.TrimSpace(content))
	}

	if err := fsx.WriteFileAtomic([]byte(content), step.Target, migratedFileMode); err != nil {
		return fmt.Errorf("Request.restoreFile:%w", err)
	}

	step.Outcome = OutcomeRestored
	step.Source = revision
	return nil
}

// The working copy wins while git has not deleted it, else the content git history holds.
func (r *Request) storeFile(ctx context.Context, step *Step) error {
	isTargetPresent, err := isPresent(step.Target)
	if err != nil {
		return fmt.Errorf("Request.storeFile:%w", err)
	}

	if isTargetPresent {
		step.Outcome = OutcomeKept
		return nil
	}

	workingCopy := filepath.Join(r.MainCheckout, filepath.FromSlash(step.Source))
	isWorkingCopyPresent, err := isPresent(workingCopy)
	if err != nil {
		return fmt.Errorf("Request.storeFile:%w", err)
	}

	if !isWorkingCopyPresent {
		return r.restoreFile(ctx, step)
	}

	if err := copyFile(workingCopy, step.Target); err != nil {
		return fmt.Errorf("Request.storeFile:%w", err)
	}

	step.Outcome = OutcomeCopied
	step.Source = workingCopy
	return nil
}

func (r *Request) storeSteps() []*Step {
	steps := make([]*Step, 0, len(storeFiles))
	for _, gitPath := range storeFiles {
		step := &Step{
			Source: gitPath,
			Target: filepath.Join(r.StoreRoot, filepath.FromSlash(gitPath)),
		}
		steps = append(steps, step)
	}
	return steps
}

type Step struct {
	Outcome Outcome
	Source  string
	Target  string
}

// Line renders the step as the one line the installers print.
func (s *Step) Line() string {
	return fmt.Sprintf("%-11s %s <- %s", s.Outcome, s.Target, s.Source)
}

// Resets the executable mode fsx.CopyFile writes.
func copyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	if err := fsx.CopyFile(source, target); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	if err := os.Chmod(target, migratedFileMode); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	return nil
}

// The copy lands first, so a failed remove leaves two copies, never none.
func copyThenRemove(source, target string) error {
	if err := copyFile(source, target); err != nil {
		return fmt.Errorf("copyThenRemove: %w", err)
	}

	if err := os.Remove(source); err != nil {
		return fmt.Errorf("copyThenRemove: %w", err)
	}
	return nil
}

// A stat failure other than absence is an error, never a silent "absent".
func isPresent(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("isPresent: %w", err)
}

// Falls back to copy-then-remove when the rename fails, as it does across volumes.
func move(step *Step) error {
	isTargetPresent, err := isPresent(step.Target)
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}

	if isTargetPresent {
		step.Outcome = OutcomeKept
		return nil
	}

	isSourcePresent, err := isPresent(step.Source)
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}

	if !isSourcePresent {
		step.Outcome = OutcomeAbsent
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(step.Target), 0o755); err != nil {
		return fmt.Errorf("move: %w", err)
	}

	renameErr := os.Rename(step.Source, step.Target)
	if renameErr != nil {
		if err := copyThenRemove(step.Source, step.Target); err != nil {
			return fmt.Errorf("move: Rename failed (%w), copy fallback failed: %w", renameErr, err)
		}
	}

	step.Outcome = OutcomeMoved
	return nil
}
