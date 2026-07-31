package contextdocs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const syncScript = "sync_context.sh"

// ErrCanceled reports that the user dismissed the folder dialog.
var ErrCanceled = errors.New("folder choice canceled")

type SyncOptions struct {
	ContextDir string
	Langs      []string
	Role       string
	Symlink    bool
	Target     string
}

// Sync runs sync_context.sh non-interactively: every prompt value is passed
// as a flag, the target repo as the positional argument. An empty ContextDir
// falls through to the script's own default (docs); an empty Role falls
// through to the target's context-pack.json or the script default.
func Sync(ctx context.Context, opts SyncOptions, scriptsDir string) (string, error) {
	if strings.TrimSpace(opts.Target) == "" {
		return "", fmt.Errorf("Sync: target path is empty")
	}

	symlink := "--no-symlink"
	if opts.Symlink {
		symlink = "--symlink"
	}
	args := []string{
		"--context-dir", opts.ContextDir,
		"--langs", strings.Join(opts.Langs, ","),
	}
	if opts.Role != "" {
		args = append(args, "--role", opts.Role)
	}
	args = append(args, symlink, opts.Target)

	output, err := shell.Run(ctx, "", filepath.Join(scriptsDir, syncScript), args...)
	if err != nil {
		return output, fmt.Errorf("Sync: %w", err)
	}
	return output, nil
}

// ChooseFolder opens the native macOS folder picker with the given prompt and
// returns the chosen POSIX path. It blocks until the user picks or
// shell.Timeout expires and requires a GUI session; a dismissed dialog
// returns ErrCanceled.
func ChooseFolder(ctx context.Context, prompt string) (string, error) {
	output, err := shell.Run(ctx, "", "osascript", "-e",
		fmt.Sprintf("POSIX path of (choose folder with prompt %q)", prompt))
	if err != nil {
		if strings.Contains(output, "User canceled") {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("ChooseFolder: %w", err)
	}
	return strings.TrimSpace(output), nil
}
