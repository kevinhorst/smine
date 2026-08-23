package contextdocs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
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
	SkipAcdsl  bool // pass --no-acdsl: prose-only deploy, no gate slice
	SkipProse  bool // skip the prose pack (actions chapters + rules guides); AGENTS.md and context.json still deploy
	Symlink    bool
	Task       bool // pass --task: the acdsl slice also ships task-lifetime rules
	Target     string
}

// Sync runs sync_context.sh non-interactively: every prompt value is passed
// as a flag, the target repo as the positional argument. An empty ContextDir
// falls through to the script's own default (docs); an empty Role falls
// through to the target's context.json or the script default.
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
	if opts.SkipProse {
		args = append(args, "--no-prose")
	}
	if opts.SkipAcdsl {
		args = append(args, "--no-acdsl")
	}
	if opts.Task {
		args = append(args, "--task")
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
// shell.DialogTimeout expires and requires a GUI session; a dismissed dialog
// returns ErrCanceled.
func ChooseFolder(ctx context.Context, prompt string) (string, error) {
	if runtime.GOOS == "windows" {
		return chooseFolderWindows(ctx, prompt)
	}
	output, err := shell.RunDialog(ctx, "osascript", "-e",
		fmt.Sprintf("POSIX path of (choose folder with prompt %q)", prompt))
	if err != nil {
		if strings.Contains(output, "User canceled") {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("ChooseFolder: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// folderDialogScript is the PowerShell folder-picker template (hole: the
// prompt). The TopMost owner form brings the dialog to the foreground even
// when the spawning server is a background windowsgui process (addendum A6).
const folderDialogScript = `Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form -Property @{TopMost=$true; ShowInTaskbar=$false}
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = %q
if ($d.ShowDialog($owner) -eq 'OK') { Write-Output $d.SelectedPath } else { exit 3 }`

// chooseFolderWindows opens the WinForms folder dialog via powershell -STA
// (windows_support plan D19). A dismissed dialog returns ErrCanceled,
// mirroring the darwin branch.
func chooseFolderWindows(ctx context.Context, prompt string) (string, error) {
	script := fmt.Sprintf(folderDialogScript, prompt)
	output, err := shell.RunDialog(ctx, "powershell", "-NoProfile", "-STA", "-Command", script)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("chooseFolderWindows: %w", err)
	}
	return strings.TrimSpace(output), nil
}
