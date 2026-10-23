package routinerun

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/shell"
)

const gitTimeout = 10 * time.Minute

// commitChanges leaves every staged change outside pathspecs staged.
func commitChanges(ctx context.Context, body, dir string, pathspecs []string, subject string) (bool, error) {
	statusArgs := []string{"status", "--porcelain", "--"}
	statusArgs = append(statusArgs, pathspecs...)
	status, err := git(ctx, dir, statusArgs...)
	if err != nil {
		return false, fmt.Errorf("commitChanges: %w", err)
	}
	if status == "" {
		return false, nil
	}

	addArgs := []string{"add", "-A", "--"}
	addArgs = append(addArgs, pathspecs...)
	if _, err := git(ctx, dir, addArgs...); err != nil {
		return false, fmt.Errorf("commitChanges: %w", err)
	}

	commitArgs := []string{"commit", "--quiet", "-m", subject}
	if body != "" {
		commitArgs = append(commitArgs, "-m", body)
	}
	commitArgs = append(commitArgs, "--")
	commitArgs = append(commitArgs, pathspecs...)
	if _, err := git(ctx, dir, commitArgs...); err != nil {
		return false, fmt.Errorf("commitChanges: %w", err)
	}
	return true, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	gitCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	argv := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(gitCtx, "git", argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	shell.HideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git: %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(output)), nil
}

func nonEmptyLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
