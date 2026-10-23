package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/jsonschemacheck"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	backlogSchemaPath = "skills/concept/backlog/schema.json"
	storeBacklogFile  = "backlog.json"
)

// storeTarget is the store's files and the schema they follow.
type storeTarget struct {
	files      []string
	schemaPath string
}

func (t *storeTarget) check() ([]string, error) {
	schema, err := jsonschemacheck.NewSchema(t.schemaPath)
	if err != nil {
		return nil, fmt.Errorf("storeTarget.check: %w", err)
	}

	violations := make([]string, 0)
	for _, file := range t.files {
		lines, err := schema.Violations(file)
		if err != nil {
			return nil, fmt.Errorf("storeTarget.check: %w", err)
		}

		violations = append(violations, lines...)
	}
	return violations, nil
}

func mainStoreRoot(ctx context.Context) (string, error) {
	commonDir, err := shell.Run(ctx, "", "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("mainStoreRoot: %w", err)
	}

	root, err := config.StoreRoot(filepath.Dir(strings.TrimSpace(commonDir)))
	if err != nil {
		return "", fmt.Errorf("mainStoreRoot: %w", err)
	}
	return root, nil
}

func runStoreCheck(args []string) int {
	flags := flag.NewFlagSet("store-check", flag.ExitOnError)
	rootOverride := flags.String("root", "", "store root to check instead of the main checkout's")
	flags.Parse(args)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: rules store-check [-root <dir>]")
		return exitError
	}

	ctx := context.Background()
	checkout, err := shell.Run(ctx, "", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	storeRoot := *rootOverride
	if storeRoot == "" {
		storeRoot, err = mainStoreRoot(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}
	}

	target, err := backlogTarget(strings.TrimSpace(checkout), storeRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations, err := target.check()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	for _, violation := range violations {
		fmt.Println(violation)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "rules: %d violation(s) under %s\n", len(violations), storeRoot)
		return exitViolations
	}

	fmt.Printf("store-check: %d file(s) valid under %s\n", len(target.files), storeRoot)
	return exitClean
}

func runStoreRoot(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: rules store-root")
		return exitError
	}

	storeRoot, err := mainStoreRoot(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	fmt.Println(storeRoot)
	return exitClean
}

func backlogTarget(checkoutRoot, storeRoot string) (*storeTarget, error) {
	info, err := os.Stat(storeRoot)
	if err != nil {
		return nil, fmt.Errorf("backlogTarget: store root: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("backlogTarget: store root %s is not a directory", storeRoot)
	}

	backlogFiles := make([]string, 0)
	backlogFile := filepath.Join(storeRoot, config.StoreBacklogDir, storeBacklogFile)
	_, err = os.Stat(backlogFile)
	switch {
	case err == nil:
		backlogFiles = append(backlogFiles, backlogFile)
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("backlogTarget: %w", err)
	}

	backlog := &storeTarget{files: backlogFiles, schemaPath: filepath.Join(checkoutRoot, backlogSchemaPath)}
	return backlog, nil
}
