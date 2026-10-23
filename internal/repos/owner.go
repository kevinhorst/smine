package repos

import (
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/gitroot"
)

func longestPrefixOwner(path string, roster []Repo) string {
	name, longest := "", 0
	for index := range roster {
		repo := &roster[index]
		isLonger := len(repo.Path) > longest
		if isLonger && IsUnder(path, repo.Path) {
			name, longest = repo.Name, len(repo.Path)
		}
	}
	return name
}

func IsUnder(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// OwnerOf resolves a linked worktree outside every repo only while its directory exists.
func OwnerOf(path string, roster []Repo) string {
	if path == "" {
		return ""
	}
	if name := longestPrefixOwner(path, roster); name != "" {
		return name
	}

	main := gitroot.MainRoot(path)
	for index := range roster {
		if filepath.Clean(roster[index].Path) == main {
			return roster[index].Name
		}
	}
	return ""
}
