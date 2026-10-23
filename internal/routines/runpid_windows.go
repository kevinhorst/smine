//go:build windows

package routines

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// RunPid finds the routine.exe whose command line ends with the task's Exec arguments; 0 when the task is absent or no run is live.
func RunPid(ctx context.Context, label string) (int, error) {
	command := fmt.Sprintf(
		`$t = Get-ScheduledTask -TaskPath '%s' -TaskName '%s' -ErrorAction SilentlyContinue; if (-not $t) { '0'; exit 0 }; $a = $t.Actions[0].Arguments; $p = Get-CimInstance Win32_Process -Filter "Name='routine.exe'" | Where-Object { $_.CommandLine -and $_.CommandLine.EndsWith($a) } | Select-Object -First 1; if ($p) { $p.ProcessId } else { '0' }`,
		taskPath, label)
	output, err := powershell(ctx, command)
	if err != nil {
		return 0, fmt.Errorf("RunPid: %s: %w", label, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return 0, fmt.Errorf("RunPid: %s: Unexpected output %q", label, strings.TrimSpace(output))
	}
	return pid, nil
}
