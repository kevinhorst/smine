package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/secretscan"
)

const (
	exitClean    = 0
	exitError    = 2
	exitFindings = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	jsonOutput := flag.Bool("json", false, "emit the report as JSON instead of text")
	scanHistory := flag.Bool("history", false, "also scan all blobs reachable from any ref")
	updateBaseline := flag.Bool("update-baseline", false, "accept all current tree findings into the baseline")
	flag.Parse()

	repoPath := flag.Arg(0)
	if repoPath == "" {
		fmt.Fprintln(os.Stderr, "usage: secretscan [-json] [-history] [-update-baseline] <repo-path>")
		return exitError
	}

	options := secretscan.Options{ShouldScanHistory: *scanHistory}
	result, err := secretscan.Scan(repoPath, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, "secretscan:", err)
		return exitError
	}

	if *updateBaseline {
		acceptedCount := len(result.NewFindings)
		if err := secretscan.WriteBaseline(repoPath, result); err != nil {
			fmt.Fprintln(os.Stderr, "secretscan:", err)
			return exitError
		}
		fmt.Fprintf(os.Stderr, "secretscan: baseline updated, %d new findings accepted\n", acceptedCount)
	}

	if *jsonOutput {
		data, err := secretscan.RenderJson(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "secretscan:", err)
			return exitError
		}
		os.Stdout.Write(data)
	} else {
		fmt.Print(secretscan.RenderText(result))
	}

	if len(result.NewFindings) > 0 {
		return exitFindings
	}
	if len(result.HistoryFindings) > 0 {
		return exitFindings
	}

	return exitClean
}
