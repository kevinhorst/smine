package reviewscope

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/cmdline"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	probeReplaySlug = "probe-replay"
	resultGreen     = "green"
	resultRed       = "red"
)

// ProbeReplay is the probe-replay subcommand's output.
type ProbeReplay struct {
	Hits    []ProbeResult        `json:"hits"`
	Probes  []ProbeResult        `json:"probes"`
	Salvage *gitwt.SalvageResult `json:"salvage"`
}

type ProbeRequest struct {
	Acdsl     string
	Branch    string
	Build     string
	Head      string
	Repo      string
	Root      string
	RoundDirs []string
}

type ProbeResult struct {
	FindingId string `json:"finding_id"`
	Id        string `json:"id"`
	Result    string `json:"result"`
	Round     string `json:"round"`
}

// roundProbes is one round dir with the probes its review.json indexes.
type roundProbes struct {
	Dir    string
	Probes []reviewProbe
}

// reviewProbe is one probes[] entry of review.json.
type reviewProbe struct {
	FindingId string `json:"finding_id"`
	Id        string `json:"id"`
	Registry  string `json:"registry"`
}

// buildForReplay runs the build lines in the tree before the probes.
func buildForReplay(ctx context.Context, logOut io.Writer, tree string, build [][]string) error {
	command := strings.Join(joinLines(build), "; ")
	outcome := runLines(ctx, build, tree)
	fmt.Fprintf(logOut, "build (%s): %s\n", command, outcome)
	if outcome != "ok" {
		return fmt.Errorf("buildForReplay: Build %s %s", command, outcome)
	}
	return nil
}

func checkProbe(ctx context.Context, probe reviewProbe, program, roundDir, tree string) ProbeResult {
	result := ProbeResult{FindingId: probe.FindingId, Id: probe.Id, Round: filepath.Base(roundDir)}
	programArgs, err := cmdline.Split(program)
	if err != nil {
		result.Result = "error: " + err.Error()
		return result
	}

	registry := filepath.Join(roundDir, "probes", probe.Registry)
	checkArgs := []string{"check", "-rule", probe.Id, "-registry", registry}
	args := slices.Concat(programArgs, checkArgs)
	output, err := shell.Run(ctx, tree, args[0], args[1:]...)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.Result = resultGreen
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		result.Result = resultRed
	default:
		result.Result = "error: " + strings.TrimSpace(output)
	}
	return result
}

// copyMirrors copies into the tree root because acdsl discovers *.acdsl files there.
func copyMirrors(roundDirs []string, tree string) error {
	for _, roundDir := range roundDirs {
		mirrors, err := filepath.Glob(filepath.Join(roundDir, "probes", "*.acdsl"))
		if err != nil {
			return fmt.Errorf("copyMirrors: %w", err)
		}

		for _, mirror := range mirrors {
			content, err := os.ReadFile(mirror)
			if err != nil {
				return fmt.Errorf("copyMirrors: %w", err)
			}

			target := filepath.Join(tree, "railroad-probes-"+filepath.Base(roundDir)+"-"+filepath.Base(mirror))
			if err := os.WriteFile(target, content, 0o644); err != nil {
				return fmt.Errorf("copyMirrors: %w", err)
			}
		}
	}
	return nil
}

func countProbes(rounds []roundProbes) int {
	count := 0
	for _, round := range rounds {
		count += len(round.Probes)
	}
	return count
}

// joinLines renders each argv as one space-joined command line.
func joinLines(lines [][]string) []string {
	joined := make([]string, 0, len(lines))
	for _, argv := range lines {
		joined = append(joined, strings.Join(argv, " "))
	}
	return joined
}

// readRoundProbes collects the probes of every round before any worktree exists.
func readRoundProbes(roundDirs []string) ([]roundProbes, error) {
	rounds := make([]roundProbes, 0, len(roundDirs))
	for _, roundDir := range roundDirs {
		var review reviewDocument
		if err := readJson(filepath.Join(roundDir, reviewFile), &review); err != nil {
			return nil, err
		}
		rounds = append(rounds, roundProbes{Dir: roundDir, Probes: review.Probes})
	}
	return rounds, nil
}

// replayRounds builds the tree the way buildtest does, then checks every round's probes.
func replayRounds(ctx context.Context, logOut io.Writer, replay *ProbeReplay, request *ProbeRequest, rounds []roundProbes, tree string, build [][]string) error {
	if err := copyMirrors(request.RoundDirs, tree); err != nil {
		return err
	}

	if err := buildForReplay(ctx, logOut, tree, build); err != nil {
		return err
	}

	program := strings.TrimSpace(request.Acdsl)
	if program == "" {
		program = filepath.Join(tree, "bin", "acdsl")
		if _, err := os.Stat(program); err != nil {
			return fmt.Errorf("replayRounds: No bin/acdsl in %s after the build", tree)
		}
	}

	for _, round := range rounds {
		for _, probe := range round.Probes {
			result := checkProbe(ctx, probe, program, round.Dir, tree)
			replay.Probes = append(replay.Probes, result)
			if result.Result != resultGreen {
				replay.Hits = append(replay.Hits, result)
			}
		}
	}
	return nil
}

// ReplayProbes counts any non-green probe as a hit; without a probe it provisions nothing.
func ReplayProbes(ctx context.Context, request *ProbeRequest) (*ProbeReplay, error) {
	replay := &ProbeReplay{Hits: []ProbeResult{}, Probes: []ProbeResult{}}
	rounds, err := readRoundProbes(request.RoundDirs)
	if err != nil {
		return nil, err
	}

	if countProbes(rounds) == 0 {
		return replay, nil
	}

	build, err := commandLines("build", request.Build)
	if err != nil {
		return nil, fmt.Errorf("ReplayProbes: %w", err)
	}

	lastRound := request.RoundDirs[len(request.RoundDirs)-1]
	logOut, err := os.Create(filepath.Join(lastRound, "probe-replay.log"))
	if err != nil {
		return nil, fmt.Errorf("ReplayProbes: %w", err)
	}
	defer logOut.Close()

	provisionRequest := &gitwt.ProvisionRequest{Branch: request.Branch, Head: request.Head, Repo: request.Repo, Root: request.Root, Slug: probeReplaySlug}
	provisioned, err := gitwt.Provision(ctx, provisionRequest)
	if err != nil {
		return nil, err
	}

	replayErr := replayRounds(ctx, logOut, replay, request, rounds, provisioned.Path, build)
	salvageRequest := &gitwt.SalvageRequest{Base: request.Head, Dest: filepath.Join(lastRound, "salvage", probeReplaySlug), Repo: request.Repo, Worktree: provisioned.Path}
	replay.Salvage, err = gitwt.Salvage(ctx, salvageRequest)
	branchRequest := &gitwt.BranchRequest{Base: request.Head, Dest: filepath.Join(lastRound, "salvage", "branches"), Prefix: request.Branch, Repo: request.Repo}
	_, branchErr := gitwt.RemoveBranches(ctx, branchRequest)
	if joined := errors.Join(replayErr, err, branchErr); joined != nil {
		return replay, joined
	}
	return replay, nil
}
