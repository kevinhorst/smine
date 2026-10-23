package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// One marker-bearing quoted literal: the unit Collapse restores from the fragment.
var markerLiteralRe = regexp.MustCompile(`"[^"\n]*\{\{(?:HEARTBEAT_PORT|HOME|PEEK_MCP|PEEK_CONTROL_PORT|REPO_DIR|NOTIFICATION_SOUND|STOP_SOUND)\}\}[^"\n]*"`)

// Markers are the deploy-time values the {{…}} markers of fragments, copied files and the plist expand to.
type Markers struct {
	HeartbeatPort     string
	Home              string
	NotificationSound string
	PeekControlPort   string
	PeekMcp           string
	RepoDir           string
	StopSound         string
}

// The result must expand back to data byte for byte, else it is refused.
func (m Markers) Collapse(data, template []byte) ([]byte, error) {
	if m.Home == "" {
		return nil, errors.New("Collapse: No home directory to collapse")
	}

	type pair struct{ expanded, literal string }
	var pairs []pair
	for _, literal := range markerLiteralRe.FindAllString(string(template), -1) {
		pairs = append(pairs, pair{expanded: string(m.Expand([]byte(literal))), literal: literal})
	}
	// Longest expansion first: a literal never gets split by a shorter one it contains.
	slices.SortFunc(pairs, func(left, right pair) int { return len(right.expanded) - len(left.expanded) })

	collapsed := string(data)
	for _, p := range pairs {
		collapsed = strings.ReplaceAll(collapsed, p.expanded, p.literal)
	}
	collapsed = strings.ReplaceAll(collapsed, `"`+m.PeekMcp+`"`, `"{{PEEK_MCP}}"`)
	homeRe := regexp.MustCompile(regexp.QuoteMeta(m.Home) + `([/"])`)
	collapsed = homeRe.ReplaceAllString(collapsed, "{{HOME}}$1")

	if !bytes.Equal(m.Expand([]byte(collapsed)), data) {
		return nil, errors.New("Collapse: Result does not expand back to the live file — it already holds a literal {{…}} marker; edit the fragment by hand")
	}
	return []byte(collapsed), nil
}

// Expand replaces the deploy-time markers with this install's values.
func (m Markers) Expand(data []byte) []byte {
	replacer := strings.NewReplacer(
		"{{HEARTBEAT_PORT}}", m.HeartbeatPort,
		"{{HOME}}", m.Home,
		"{{NOTIFICATION_SOUND}}", m.NotificationSound,
		"{{PEEK_CONTROL_PORT}}", m.PeekControlPort,
		"{{PEEK_MCP}}", m.PeekMcp,
		"{{REPO_DIR}}", m.RepoDir,
		"{{STOP_SOUND}}", m.StopSound,
	)
	return []byte(replacer.Replace(string(data)))
}

// NativeHome is the user home with forward slashes on Windows (cygpath -m), $HOME elsewhere: the form every {{HOME}} expansion and path join takes.
func NativeHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("NativeHome: %w", err)
	}
	return filepath.ToSlash(home), nil
}

// PeekMcpPath is the peek binary the marker names.
func PeekMcpPath(home string) string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(os.Getenv("LOCALAPPDATA")) + "/smine/bin/peek-mcp.exe"
	}
	return home + "/go/bin/peek-mcp"
}
