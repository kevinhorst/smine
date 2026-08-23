// Command goldendiff runs an observation anchor: GET the recipe URL, apply
// the normalization spec, materialize the observation at observed=, and diff
// it line-wise against the golden file the rule anchors. The archived
// observation is traceability made physical — the report shows observed vs
// expected, not just FAIL.
//
// Contract: args = <files-list path> url=<url> observed=<path>
// [normalize=<spec>]; the files list must hold exactly the golden file; one
// violation per stdout line as file:line: message; exit 0 pass, 1
// violations, 2 error.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxReported = 10

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "goldendiff: usage: <files-list> url=<url> observed=<path> [normalize=<spec>]")
		return 2
	}
	params := parseParams(args[1:])
	url, observedPath := params["url"], params["observed"]
	if url == "" || observedPath == "" {
		fmt.Fprintln(os.Stderr, "goldendiff: url= and observed= are required")
		return 2
	}
	files, err := readLines(args[0])
	if err != nil || len(files) != 1 {
		fmt.Fprintln(os.Stderr, "goldendiff: anchor must resolve to exactly the golden file")
		return 2
	}
	golden := files[0]

	body, err := fetch(url)
	if err != nil {
		// Unreachable target is a red contract, not tool breakage: exit 2
		// would abort doctrine gating in the same -lifetime all run.
		fmt.Fprintf(out, "%s:1: request failed: %v\n", golden, err)
		return 1
	}
	if spec := params["normalize"]; spec != "" {
		if body, err = applyNormalize(spec, body); err != nil {
			fmt.Fprintln(os.Stderr, "goldendiff:", err)
			return 2
		}
	}
	if err := os.MkdirAll(filepath.Dir(observedPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "goldendiff:", err)
		return 2
	}
	if err := os.WriteFile(observedPath, []byte(body), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "goldendiff:", err)
		return 2
	}

	goldenRaw, err := os.ReadFile(golden)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goldendiff:", err)
		return 2
	}
	return diff(string(goldenRaw), body, golden, observedPath, out)
}

// diff compares line-wise and reports up to maxReported divergences anchored
// to golden line numbers.
func diff(golden, observed, goldenPath, observedPath string, out io.Writer) int {
	goldenLines := strings.Split(golden, "\n")
	observedLines := strings.Split(observed, "\n")
	reported := 0
	for i := 0; i < len(goldenLines) || i < len(observedLines); i++ {
		want, got := lineAt(goldenLines, i), lineAt(observedLines, i)
		if want == got {
			continue
		}
		if reported < maxReported {
			fmt.Fprintf(out, "%s:%d: expected %q, observed %q (full observation: %s)\n", goldenPath, i+1, want, got, observedPath)
		}
		reported++
	}
	if reported > maxReported {
		fmt.Fprintf(out, "%s:1: … %d further differing line(s)\n", goldenPath, reported-maxReported)
	}
	if reported > 0 {
		return 1
	}
	return 0
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<missing>"
}

func fetch(url string) (string, error) {
	client := http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch: Unexpected status %s", response.Status)
	}
	body, err := io.ReadAll(response.Body)
	return string(body), err
}

// applyNormalize applies each spec line s<delim>re2<delim>repl<delim> to the
// whole body — timestamps, generated IDs, machine paths become stable tokens.
func applyNormalize(specPath, body string) (string, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}
	for lineNo, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) < 4 || line[0] != 's' {
			return "", fmt.Errorf("%s:%d: spec line must be s<delim>re<delim>repl<delim>", specPath, lineNo+1)
		}
		parts := strings.Split(line[1:], line[1:2])
		if len(parts) != 4 || parts[0] != "" || parts[3] != "" {
			return "", fmt.Errorf("%s:%d: spec line must be s<delim>re<delim>repl<delim>", specPath, lineNo+1)
		}
		pattern, err := regexp.Compile(parts[1])
		if err != nil {
			return "", fmt.Errorf("%s:%d: %w", specPath, lineNo+1, err)
		}
		body = pattern.ReplaceAllString(body, parts[2])
	}
	return body, nil
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
