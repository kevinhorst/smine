//go:build windows

package routines

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// taskPath is the Task Scheduler folder holding every smine task; the task
// name is the routine label — the Windows analog of the gui/<uid>/<label>
// launchd target.
const taskPath = `\smine\`

// powershell runs one cmdlet pipeline; cmdlets over schtasks.exe because its
// CSV output is locale-dependent (plan D12).
func powershell(ctx context.Context, command string) (string, error) {
	return shell.Run(ctx, "", "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
}

// taskState returns the task's State enum name, "NotFound" for an
// unregistered task. The probe never exits non-zero for absence, so exit
// codes stay meaningful for real failures; State enum names print in English
// regardless of locale.
func taskState(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(
		`$t = Get-ScheduledTask -TaskPath '%s' -TaskName '%s' -ErrorAction SilentlyContinue; if ($t) { $t.State.ToString() } else { 'NotFound' }`,
		taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return "", fmt.Errorf("taskState: %s: %w", label, err)
	}
	return strings.TrimSpace(output), nil
}

// IsLoaded mirrors the launchd semantics: registered AND not disabled — the
// gui-domain loaded bit and the disable override collapse into Task
// Scheduler's single Enabled bit (concept mapping).
func IsLoaded(ctx context.Context, label string) (bool, error) {
	state, err := taskState(ctx, label)
	if err != nil {
		return false, fmt.Errorf("IsLoaded: %s: %w", label, err)
	}
	return state != "NotFound" && state != "Disabled", nil
}

// IsRunning reports whether the task has a live instance — Task Scheduler
// exposes it directly as the Running state. Used by the reload guard to
// avoid killing an active run.
func IsRunning(ctx context.Context, label string) (bool, error) {
	state, err := taskState(ctx, label)
	if err != nil {
		return false, fmt.Errorf("IsRunning: %s: %w", label, err)
	}
	return state == "Running", nil
}

// Start registers (or force-re-registers) the routine's task from its plist
// and enables it — the analog of launchctl enable + bootstrap. The plist is
// never handed to the OS; it is translated to Task XML here (plan D1).
func Start(ctx context.Context, label, plistPath string) (string, error) {
	schedule, err := parseSchedule(plistPath)
	if err != nil {
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}

	// Absolute paths only: the server's -routines flag is relative, the task
	// template has no WorkingDirectory, and Task Scheduler launches actions
	// from System32 — a relative Command fails with 0x80070002 at fire time.
	routineDir, err := filepath.Abs(filepath.Dir(plistPath))
	if err != nil {
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}
	repoRoot := filepath.Dir(filepath.Dir(routineDir))
	wrapperExe := filepath.Join(repoRoot, "bin", "routinewrap.exe")

	taskXML, err := TaskXML(label, schedule.Intervals, wrapperExe, routineDir)
	if err != nil {
		return "", fmt.Errorf("Start: %w", err)
	}

	xmlFile, err := os.CreateTemp("", "smine-task-*.xml")
	if err != nil {
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}
	defer os.Remove(xmlFile.Name())
	if _, err := xmlFile.WriteString(taskXML); err != nil {
		xmlFile.Close()
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}
	xmlFile.Close()

	// -User binds the principal to the current user — the shared template has
	// no <UserId>, and an unbound principal needs admin to register (see
	// registerServerTask in cmd/configserver/install_windows.go).
	command := fmt.Sprintf(
		`Register-ScheduledTask -TaskName '%s' -TaskPath '%s' -User $env:USERNAME -Xml (Get-Content -Raw '%s') -Force | Out-Null; Enable-ScheduledTask -TaskPath '%s' -TaskName '%s' | Out-Null`,
		label, taskPath, xmlFile.Name(), taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("Start: %s: %w", label, err)
	}
	return output, nil
}

// Stop disables the task and nothing more — never Unregister, so "present
// but disabled" persists across logins exactly like launchctl disable
// (plan D5).
func Stop(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(`Disable-ScheduledTask -TaskPath '%s' -TaskName '%s' | Out-Null`, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("Stop: %s: %w", label, err)
	}
	return output, nil
}

// RunNow fires the task asynchronously — same semantics as launchctl
// kickstart; callers poll results.jsonl for completion.
func RunNow(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(`Start-ScheduledTask -TaskPath '%s' -TaskName '%s'`, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("RunNow: %s: %w", label, err)
	}
	return output, nil
}

// SyncAll reconciles the routines directory with the task store (plan D6):
// unregistered -> register+enable (routine arrived via git pull);
// registered+enabled -> re-register with -Force (picks up outside plist
// edits); disabled -> untouched (the user's off-state persists). Per-routine
// failures are logged, never fatal — mirrors the darwin bootstrap behavior.
func SyncAll(ctx context.Context, dir string) {
	list, err := Scan(dir)
	if err != nil {
		log.Printf("routines: sync scan failed: %v", err)
		return
	}

	for index := range list {
		routine := &list[index]
		if routine.LoadError != "" {
			continue
		}
		state, err := taskState(ctx, routine.Label)
		if err != nil {
			log.Printf("routines: %s probe failed: %v", routine.Name, err)
			continue
		}
		if state == "NotFound" && routine.DefaultDisabled {
			log.Printf("routines: %s default-disabled, awaiting explicit enable", routine.Name)
			continue
		}
		if state == "Disabled" {
			log.Printf("routines: %s disabled, not syncing", routine.Name)
			continue
		}
		if _, err := Start(ctx, routine.Label, routine.PlistPath); err != nil {
			log.Printf("routines: %s sync failed: %v", routine.Name, err)
			continue
		}
		log.Printf("routines: %s synced", routine.Name)
	}
}
