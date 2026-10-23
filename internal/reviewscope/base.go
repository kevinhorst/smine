package reviewscope

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// Derived base sources, in the order DeriveBase tries them.
const (
	BaseSourceBracket  = "bracket"
	BaseSourceMainline = "mainline"
	BaseSourceUpstream = "upstream"
)

type DerivedBase struct {
	Commit string `json:"commit"`
	Source string `json:"source"`
}

// gitLine runs one git command in repo and returns its trimmed output.
func gitLine(ctx context.Context, repo string, args ...string) (string, error) {
	output, err := shell.Run(ctx, repo, "git", args...)
	if err != nil {
		return "", fmt.Errorf("gitLine: git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(output), nil
}

// DeriveBase tries the scope bracket, then head's fork with its upstream, then with mainline.
func DeriveBase(ctx context.Context, repo, head, mainline, bracket string) (*DerivedBase, error) {
	if bracket != "" {
		commit, err := gitLine(ctx, repo, "rev-parse", "--verify", bracket+"^{commit}")
		if err != nil {
			return nil, fmt.Errorf("DeriveBase: Bracket base %s does not resolve: %w", bracket, err)
		}
		return &DerivedBase{Commit: commit, Source: BaseSourceBracket}, nil
	}

	if upstream, err := gitLine(ctx, repo, "rev-parse", "--verify", "--quiet", head+"@{upstream}"); err == nil && upstream != "" {
		fork, err := gitLine(ctx, repo, "merge-base", head, upstream)
		if err != nil {
			return nil, err
		}
		return &DerivedBase{Commit: fork, Source: BaseSourceUpstream}, nil
	}

	fork, err := gitLine(ctx, repo, "merge-base", head, mainline)
	if err != nil {
		return nil, err
	}
	return &DerivedBase{Commit: fork, Source: BaseSourceMainline}, nil
}
