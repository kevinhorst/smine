package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	captureChromeApp     = "Google Chrome.app/Contents/MacOS/Google Chrome"
	captureDefaultSize   = "1440,1000"
	captureDefaultWaitMs = "500"
	captureExecutable    = 0o111
	captureExitNoImage   = 70
	captureExitNoTool    = 69
	captureExitUsage     = 64
	captureSimulator     = "sim"
	captureUsage         = "usage: rules capture-ui <url|sim> <out.png> [--size WxH] [--wait-ms N]"
)

// captureChromeCommands are the PATH names tried after the app bundles.
var captureChromeCommands = []string{"google-chrome", "chromium", "chromium-browser"}

// captureRequest is one parsed capture-ui invocation.
type captureRequest struct {
	out    string
	size   string
	source string
	waitMs string
}

func runCaptureUi(args []string) int {
	request, isValid := parseCapture(args)
	if !isValid {
		fmt.Fprintln(os.Stderr, captureUsage)
		return captureExitUsage
	}

	out, err := filepath.Abs(request.out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if status := capture(context.Background(), out, request); status != exitClean {
		return status
	}

	info, err := os.Stat(out)
	if err != nil || info.Size() == 0 {
		fmt.Fprintf(os.Stderr, "capture-ui: no image written to %s\n", out)
		return captureExitNoImage
	}

	fmt.Println(out)
	return exitClean
}

func capture(ctx context.Context, out string, request *captureRequest) int {
	if request.source == captureSimulator {
		if _, err := shell.Run(ctx, "", "xcrun", "simctl", "io", "booted", "screenshot", out); err != nil {
			fmt.Fprintln(os.Stderr, "capture-ui: no booted simulator or xcrun simctl unavailable")
			return captureExitNoTool
		}
		return exitClean
	}

	chrome := findChrome()
	if chrome == "" {
		fmt.Fprintln(os.Stderr, "capture-ui: no Chrome or Chromium found")
		return captureExitNoTool
	}

	_, _ = shell.Run(
		ctx,
		"",
		chrome,
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--no-first-run",
		"--window-size="+request.size,
		"--virtual-time-budget="+request.waitMs,
		"--screenshot="+out,
		request.source,
	)
	return exitClean
}

func findChrome() string {
	dirs := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}

	for _, dir := range dirs {
		candidate := filepath.Join(dir, captureChromeApp)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().Perm()&captureExecutable != 0 {
			return candidate
		}
	}

	for _, name := range captureChromeCommands {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func parseCapture(args []string) (*captureRequest, bool) {
	if len(args) < 2 || len(args)%2 != 0 {
		return nil, false
	}

	request := &captureRequest{
		out:    args[1],
		size:   captureDefaultSize,
		source: args[0],
		waitMs: captureDefaultWaitMs,
	}
	for index := 2; index < len(args); index += 2 {
		switch args[index] {
		case "--size":
			request.size = strings.Replace(args[index+1], "x", ",", 1)
		case "--wait-ms":
			request.waitMs = args[index+1]
		default:
			return nil, false
		}
	}
	return request, true
}
