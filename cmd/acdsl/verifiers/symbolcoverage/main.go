// Command symbolcoverage checks a task contract's coverage primitive: the
// named function's test coverage in the anchored package is at least min
// percent. Runs go test -coverprofile on the single package the anchored
// files live in, then parses go tool cover -func output.
//
// Contract: args = <files-list path> symbol=<func> min=<percent>; one
// violation per stdout line as file:line: message; exit 0 pass, 1
// violations, 2 error.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/shell"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "symbolcoverage: usage: <files-list> symbol=<func> min=<percent>")
		return 2
	}
	params := parseParams(args[1:])
	symbol := params["symbol"]
	minPct, err := strconv.ParseFloat(params["min"], 64)
	if symbol == "" || err != nil {
		fmt.Fprintln(os.Stderr, "symbolcoverage: symbol= and numeric min= are required")
		return 2
	}
	files, err := readLines(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "symbolcoverage:", err)
		return 2
	}
	pkg, err := singleDir(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, "symbolcoverage:", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	profile := filepath.Join(os.TempDir(), fmt.Sprintf("symbolcoverage-%d.out", os.Getpid()))
	defer os.Remove(profile)
	if _, err := shell.Run(ctx, ".", "go", "test", "-count=1", "-coverprofile="+profile, "./"+pkg); err != nil {
		fmt.Fprintf(out, "%s:1: go test failed for ./%s: %v\n", files[0], pkg, err)
		return 1
	}
	funcReport, err := shell.Run(ctx, ".", "go", "tool", "cover", "-func="+profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "symbolcoverage: cover -func:", err)
		return 2
	}

	return judge(funcReport, symbol, minPct, files[0], out)
}

// judge scans cover -func lines (path:line: name pct%) for the symbol and
// compares the worst match against min. Split out so unit tests run on
// canned reports instead of a nested go toolchain.
func judge(funcReport, symbol string, minPct float64, anchor string, out io.Writer) int {
	worst, location := -1.0, ""
	for _, line := range strings.Split(funcReport, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[1] != symbol {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		if err != nil {
			continue
		}
		if worst < 0 || pct < worst {
			worst, location = pct, strings.TrimSuffix(fields[0], ":")
		}
	}
	if worst < 0 {
		fmt.Fprintf(out, "%s:1: symbol %q absent from coverage profile — no test reaches it\n", anchor, symbol)
		return 1
	}
	if worst < minPct {
		fmt.Fprintf(out, "%s: coverage of %s is %.1f%% < planned %.1f%%\n", location, symbol, worst, minPct)
		return 1
	}
	return 0
}

// singleDir requires every anchored file in one directory — the package under
// test; a multi-package anchor is an authoring error for this primitive.
func singleDir(files []string) (string, error) {
	dir := ""
	for _, file := range files {
		fileDir := filepath.Dir(file)
		if dir != "" && fileDir != dir {
			return "", fmt.Errorf("singleDir: Anchored files span %s and %s — anchor one package", dir, fileDir)
		}
		dir = fileDir
	}
	if dir == "" {
		return "", fmt.Errorf("singleDir: Empty files list")
	}
	return dir, nil
}

func parseParams(args []string) map[string]string {
	params := map[string]string{}
	for _, arg := range args {
		if key, value, found := strings.Cut(arg, "="); found {
			params[key] = value
		}
	}
	return params
}

func readLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines, nil
}
