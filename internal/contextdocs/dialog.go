package contextdocs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// ErrCanceled reports that the user dismissed the folder dialog.
var ErrCanceled = errors.New("folder choice canceled")

// Blocks until a pick or shell.DialogTimeout and needs a GUI session; a dismissed dialog is ErrCanceled.
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

// The TopMost owner is shown first: an unshown owner left the dialog behind the browser (addendum A6).
const folderDialogScript = `Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form -Property @{TopMost=$true; ShowInTaskbar=$false; Opacity=0}
$owner.Show()
$owner.Activate()
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = %q
$result = $d.ShowDialog($owner)
$owner.Close()
if ($result -eq 'OK') { Write-Output $d.SelectedPath } else { exit 3 }`

// A dismissed dialog returns ErrCanceled, mirroring the darwin branch (windows_support D19).
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
