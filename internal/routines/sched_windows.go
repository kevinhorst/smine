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

// taskPath plus the label is the Windows analog of the gui/<uid>/<label> launchd target.
const taskPath = `\smine\`

// powershell uses cmdlets over schtasks.exe because its CSV output is locale-dependent.
func powershell(ctx context.Context, command string) (string, error) {
	return shell.Run(ctx, "", "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
}

// taskState never exits non-zero for absence, so exit codes stay meaningful for real failures.
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

// IsLoaded collapses launchd's loaded bit and disable override into Task Scheduler's Enabled bit.
func IsLoaded(ctx context.Context, label string) (bool, error) {
	state, err := taskState(ctx, label)
	if err != nil {
		return false, fmt.Errorf("IsLoaded: %s: %w", label, err)
	}
	return state != "NotFound" && state != "Disabled", nil
}

func IsRunning(ctx context.Context, label string) (bool, error) {
	state, err := taskState(ctx, label)
	if err != nil {
		return false, fmt.Errorf("IsRunning: %s: %w", label, err)
	}
	return state == "Running", nil
}

// Start never hands the plist to the OS; it is translated to Task XML here.
func Start(ctx context.Context, label, plistPath string) (string, error) {
	schedule, err := parseSchedule(plistPath)
	if err != nil {
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}

	// Absolute only: Task Scheduler runs actions from System32, so a relative Command fails with 0x80070002.
	routineDir, err := filepath.Abs(filepath.Dir(plistPath))
	if err != nil {
		return "", fmt.Errorf("Start: %s: %w", label, err)
	}
	repoRoot := filepath.Dir(filepath.Dir(routineDir))
	runnerExe := filepath.Join(repoRoot, "bin", "routine.exe")

	taskXML, err := TaskXML(label, schedule.Intervals, runnerExe, routineDir)
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

	// -User binds the principal: the template has no <UserId>, and an unbound one needs admin to register.
	command := fmt.Sprintf(
		`Register-ScheduledTask -TaskName '%s' -TaskPath '%s' -User $env:USERNAME -Xml (Get-Content -Raw '%s') -Force | Out-Null; Enable-ScheduledTask -TaskPath '%s' -TaskName '%s' | Out-Null`,
		label, taskPath, xmlFile.Name(), taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("Start: %s: %w", label, err)
	}
	return output, nil
}

// Stop never unregisters, so the off state persists across logins like launchctl disable.
func Stop(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(`Disable-ScheduledTask -TaskPath '%s' -TaskName '%s' | Out-Null`, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("Stop: %s: %w", label, err)
	}
	return output, nil
}

// Unregister removes the scheduled task so a deleted routine leaves no task behind; a never-registered task is no error.
func Unregister(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(
		`if (Get-ScheduledTask -TaskPath '%s' -TaskName '%s' -ErrorAction SilentlyContinue) { Unregister-ScheduledTask -TaskPath '%s' -TaskName '%s' -Confirm:$false | Out-Null }`,
		taskPath, label, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("Unregister: %s: %w", label, err)
	}
	return output, nil
}

// RegisterLogonTask registers or replaces the task from its XML and starts it; -User binds the principal like Start.
func RegisterLogonTask(ctx context.Context, label, xml string) (string, error) {
	xmlFile, err := os.CreateTemp("", "smine-task-*.xml")
	if err != nil {
		return "", fmt.Errorf("RegisterLogonTask: %s: %w", label, err)
	}
	defer os.Remove(xmlFile.Name())
	if _, err := xmlFile.WriteString(xml); err != nil {
		xmlFile.Close()
		return "", fmt.Errorf("RegisterLogonTask: %s: %w", label, err)
	}
	xmlFile.Close()

	command := fmt.Sprintf(
		`Register-ScheduledTask -TaskName '%s' -TaskPath '%s' -User $env:USERNAME -Xml (Get-Content -Raw '%s') -Force | Out-Null; Start-ScheduledTask -TaskPath '%s' -TaskName '%s'`,
		label, taskPath, xmlFile.Name(), taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("RegisterLogonTask: %s: %w", label, err)
	}
	return output, nil
}

// StopTask ends the task's running instance; the task stays registered and enabled, unlike Stop, and a task not registered yet is no error.
func StopTask(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(
		`if (Get-ScheduledTask -TaskPath '%s' -TaskName '%s' -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskPath '%s' -TaskName '%s' }`,
		taskPath, label, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("StopTask: %s: %w", label, err)
	}
	return output, nil
}

// RunNow is asynchronous like launchctl kickstart; callers poll results.jsonl.
func RunNow(ctx context.Context, label string) (string, error) {
	command := fmt.Sprintf(`Start-ScheduledTask -TaskPath '%s' -TaskName '%s'`, taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return output, fmt.Errorf("RunNow: %s: %w", label, err)
	}
	return output, nil
}

// SyncAll leaves disabled tasks untouched so the user's off state persists.
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
