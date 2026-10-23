// Package trash is the one stash for everything smine removes or replaces: the trash dirs, the owned-digest store and the journal under the state dir.
package trash

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	JournalFile = "journal.jsonl"
	OwnedDir    = "owned"
	TrashDir    = "trash"

	KindBranchDeleted   Kind = "branch-deleted"
	KindBranchRecreated Kind = "branch-recreated"
	KindForgotten       Kind = "forgotten"
	KindRemoved         Kind = "removed"
	KindReplaced        Kind = "replaced"
	KindRestored        Kind = "restored"

	digestSuffix      = ".sha256"
	goosWindows       = "windows"
	hoursPerDay       = 24
	journalLineBuffer = 64 * 1024
	journalLineMax    = 1024 * 1024
	// stampLayout is RFC 3339 with nanoseconds, the time's colons replaced: NTFS refuses a colon in a name.
	stampLayout = "2006-01-02T15-04-05.000000000Z"
)

// ErrTargetExists is Restore's refusal of a present target without isReplacing.
var ErrTargetExists = errors.New("restore target exists")

// rename is os.Rename; tests swap it to force the cross-device fallback.
var rename = os.Rename

// Entry is one journal line: what was removed or replaced, by which action, why, and where its bytes went.
type Entry struct {
	Action string    `json:"action"`
	Kind   Kind      `json:"kind"`
	Path   string    `json:"path"`
	Reason string    `json:"reason,omitempty"`
	Stash  string    `json:"stash,omitempty"`
	Time   time.Time `json:"time"`
}

// Kind names what an Entry witnesses.
type Kind string

// Stash is one stamp dir of the trash with its entries.
type Stash struct {
	Entries []StashEntry
	Stamp   string
	Time    time.Time
}

// StashEntry is one file of a stamp dir, Rel in the stash path scheme.
type StashEntry struct {
	IsDir bool
	Rel   string
	Size  int64
}

// Store is the stash, the owned-digest store and the journal of one state dir.
type Store struct {
	journal       string
	owned         string
	retentionDays atomic.Int64
	root          string
}

// New opens the store of a state dir; retentionDays bounds the stash (0 keeps forever); nothing is created until the first write.
func New(stateDir string, retentionDays int) *Store {
	store := &Store{
		journal: filepath.Join(stateDir, JournalFile),
		owned:   filepath.Join(stateDir, OwnedDir),
		root:    filepath.Join(stateDir, TrashDir),
	}
	store.retentionDays.Store(int64(retentionDays))
	return store
}

// append writes one journal line; a torn line is skipped by Tail.
func (s *Store) append(entry *Entry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("Store.append: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.journal), 0o755); err != nil {
		return fmt.Errorf("Store.append: %w", err)
	}

	file, err := os.OpenFile(s.journal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Store.append: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("Store.append: %w", err)
	}
	return nil
}

// place is the stash path of one source under a fresh stamp dir, its parent dirs created.
func (s *Store) place(path string) (string, string, error) {
	rel, err := stashRel(path)
	if err != nil {
		return "", "", err
	}
	stashPath := filepath.Join(stamp(time.Now().UTC()), filepath.FromSlash(rel))
	target := filepath.Join(s.root, stashPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", "", fmt.Errorf("Store.place: %w", err)
	}
	return stashPath, target, nil
}

// settle journals a stash and runs the retention sweep; a sweep failure is reported with the journal's, the stash itself stands.
func (s *Store) settle(entry *Entry) error {
	err := s.append(entry)
	if _, sweepErr := s.Sweep(); sweepErr != nil {
		err = errors.Join(err, sweepErr)
	}
	return err
}

// stashRoots is the set of stash paths Move and Keep journaled: the root of every restorable file or dir; an archive's stash path is not one.
func (s *Store) stashRoots() (map[string]bool, error) {
	entries, err := s.Tail(math.MaxInt)
	if err != nil {
		return nil, err
	}
	roots := map[string]bool{}
	for _, entry := range entries {
		isMoved := entry.Kind == KindRemoved || entry.Kind == KindReplaced
		if isMoved && entry.Stash != "" {
			roots[entry.Stash] = true
		}
	}
	return roots, nil
}

// RelOf is a path under the root relative to it, slash-separated; "" for a path outside.
func (s *Store) RelOf(path string) string {
	rel, err := filepath.Rel(s.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// SetRetention changes the retention a running process sweeps with, after the setting was saved.
func (s *Store) SetRetention(days int) {
	if s == nil {
		return
	}
	s.retentionDays.Store(int64(days))
}

// StampDir creates and returns a fresh stamp dir for a caller that writes several files under one stamp.
func (s *Store) StampDir() (string, error) {
	dir := filepath.Join(s.root, stamp(time.Now().UTC()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("Store.StampDir: %w", err)
	}
	return dir, nil
}

// Keep copies the current bytes of path into the stash before a caller overwrites it; a missing path keeps nothing.
func (s *Store) Keep(path string, entry *Entry) (string, error) {
	if s == nil {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("Store.Keep: %w", err)
	}

	stashPath, target, err := s.place(path)
	if err != nil {
		return "", fmt.Errorf("Store.Keep: %w", err)
	}
	if err := fsx.WriteFileAtomic(data, target, 0o644); err != nil {
		return "", fmt.Errorf("Store.Keep: %w", err)
	}

	entry.Kind, entry.Path, entry.Stash = KindReplaced, absolutePath(path), filepath.ToSlash(stashPath)
	if err := s.settle(entry); err != nil {
		return stashPath, err
	}
	return stashPath, nil
}

// List reads the stamp dirs newest first; a stashed dir the journal names is one entry, any other file is its own.
func (s *Store) List() ([]*Stash, error) {
	dirs, err := os.ReadDir(s.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Store.List: %w", err)
	}

	roots, err := s.stashRoots()
	if err != nil {
		return nil, fmt.Errorf("Store.List: %w", err)
	}

	var stashes []*Stash
	for _, dir := range dirs {
		stashedAt, err := time.Parse(stampLayout, dir.Name())
		if !dir.IsDir() || err != nil {
			continue
		}

		stash := &Stash{Stamp: dir.Name(), Time: stashedAt}
		stampRoot := filepath.Join(s.root, dir.Name())
		walkErr := filepath.WalkDir(stampRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(stampRoot, path)
			if rel == "." {
				return nil
			}

			slashed := filepath.ToSlash(rel)
			isRoot := roots[dir.Name()+"/"+slashed]
			switch {
			case entry.IsDir() && isRoot:
				size, err := treeSize(path)
				if err != nil {
					return err
				}
				stash.Entries = append(stash.Entries, StashEntry{IsDir: true, Rel: slashed, Size: size})
				return fs.SkipDir
			case entry.IsDir():
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("Failed to stat %s: %w", path, err)
			}
			stash.Entries = append(stash.Entries, StashEntry{Rel: slashed, Size: info.Size()})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("Store.List: %w", walkErr)
		}
		stashes = append(stashes, stash)
	}

	slices.SortFunc(stashes, func(left, right *Stash) int { return right.Time.Compare(left.Time) })
	return stashes, nil
}

// Move renames a file or dir into the stash; a cross-device rename copies then removes.
func (s *Store) Move(path string, entry *Entry) (string, error) {
	if s == nil {
		return "", nil
	}
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	stashPath, target, err := s.place(path)
	if err != nil {
		return "", fmt.Errorf("Store.Move: %w", err)
	}
	if err := moveTree(path, target); err != nil {
		return "", fmt.Errorf("Store.Move: %w", err)
	}

	entry.Kind, entry.Path, entry.Stash = KindRemoved, absolutePath(path), filepath.ToSlash(stashPath)
	if err := s.settle(entry); err != nil {
		return stashPath, err
	}
	return stashPath, nil
}

// Note journals an event that moved no file: a forgotten row, a deleted branch, a recreated branch.
func (s *Store) Note(entry *Entry) error {
	if s == nil {
		return nil
	}
	if err := s.append(entry); err != nil {
		return fmt.Errorf("Store.Note: %w", err)
	}
	return nil
}

// Restore moves a stash entry back to its original path; a present target is refused unless isReplacing, which stashes it first.
func (s *Store) Restore(stamp, rel string, isReplacing bool) error {
	source := filepath.Join(s.root, stamp, filepath.FromSlash(rel))
	if _, err := os.Lstat(source); err != nil {
		return fmt.Errorf("Store.Restore: %w", err)
	}

	target := targetOf(rel)
	_, statErr := os.Lstat(target)
	isPresent := statErr == nil
	if isPresent && !isReplacing {
		return fmt.Errorf("Store.Restore: %s: %w", target, ErrTargetExists)
	}
	if isPresent {
		replaced := &Entry{Action: "trash restore", Reason: "replaced by the restored " + stamp}
		if _, err := s.Move(target, replaced); err != nil {
			return fmt.Errorf("Store.Restore: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("Store.Restore: %w", err)
	}
	if err := moveTree(source, target); err != nil {
		return fmt.Errorf("Store.Restore: %w", err)
	}

	removeEmptyParents(filepath.Dir(source), filepath.Join(s.root, stamp))
	restored := &Entry{Action: "trash restore", Kind: KindRestored, Path: target, Stash: stamp + "/" + rel}
	if err := s.append(restored); err != nil {
		return fmt.Errorf("Store.Restore: %w", err)
	}
	return nil
}

// Sweep removes every stamp dir older than the store's retention; a retention of 0 keeps everything.
func (s *Store) Sweep() (int, error) {
	if s == nil {
		return 0, nil
	}
	retentionDays := s.retentionDays.Load()
	if retentionDays <= 0 {
		return 0, nil
	}

	stashes, err := s.List()
	if err != nil {
		return 0, fmt.Errorf("Store.Sweep: %w", err)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * hoursPerDay * time.Hour)
	removed := 0
	for _, stash := range stashes {
		if !stash.Time.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.root, stash.Stamp)); err != nil {
			return removed, fmt.Errorf("Store.Sweep: %w", err)
		}
		removed++
	}
	return removed, nil
}

// Tail reads the newest limit journal lines, newest first; a line that does not parse is skipped.
func (s *Store) Tail(limit int) ([]*Entry, error) {
	file, err := os.Open(s.journal)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Store.Tail: %w", err)
	}
	defer file.Close()

	var entries []*Entry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, journalLineBuffer), journalLineMax)
	for scanner.Scan() {
		entry := &Entry{}
		if err := json.Unmarshal(scanner.Bytes(), entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Store.Tail: %w", err)
	}

	slices.Reverse(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// absolutePath is path made absolute for the journal; a path Abs refuses is journaled as given.
func absolutePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

// copyTree copies a file, or a dir with its files, modes kept; symlinks are copied as links.
func copyTree(source, target string) error {
	if err := filepath.WalkDir(source, copyEntry(source, target)); err != nil {
		return fmt.Errorf("copyTree: %w", err)
	}
	return nil
}

// copyEntry is copyTree's walk callback for one source root and its target.
func copyEntry(source, target string) fs.WalkDirFunc {
	return func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("copyTree: %w", err)
		}

		destination := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("copyTree: %w", err)
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(destination, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("copyTree: %w", err)
			}
			return os.Symlink(link, destination)
		}
		return copyFile(path, destination, info.Mode().Perm())
	}
}

func copyFile(source, destination string, perm fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return fmt.Errorf("copyFile: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// moveTree renames, and on a refused rename (another volume) copies then removes, so a failure leaves two copies, never none.
func moveTree(source, target string) error {
	if err := rename(source, target); err == nil {
		return nil
	}
	if err := copyTree(source, target); err != nil {
		return fmt.Errorf("moveTree: %w", err)
	}
	if err := os.RemoveAll(source); err != nil {
		return fmt.Errorf("moveTree: %w", err)
	}
	return nil
}

// removeEmptyParents removes dir and its parents while they are empty, stopping at root (root itself removed when empty).
func removeEmptyParents(dir, root string) {
	for strings.HasPrefix(dir, root) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		if dir == root {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// treeSize is the summed size of the files under dir.
func treeSize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("treeSize: %w", err)
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("treeSize: %w", err)
	}
	return size, nil
}

// stashRel is the slash-separated absolute path without its volume and leading separator: /Users/k/.claude/x → Users/k/.claude/x, C:\Users\k → C/Users/k.
func stashRel(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("stashRel: %w", err)
	}

	volume := filepath.VolumeName(absolute)
	rest := strings.TrimLeft(absolute[len(volume):], `\/`)
	volume = strings.TrimSuffix(strings.TrimLeft(volume, `\/`), ":")
	rel := filepath.ToSlash(rest)
	if rel == "" {
		return "", fmt.Errorf("stashRel: %s names no file", path)
	}
	if volume != "" {
		rel = filepath.ToSlash(volume) + "/" + rel
	}
	return rel, nil
}

func stamp(now time.Time) string {
	return now.UTC().Format(stampLayout)
}

// targetOf inverts stashRel for this OS.
func targetOf(rel string) string {
	if runtime.GOOS != goosWindows {
		return string(filepath.Separator) + filepath.FromSlash(rel)
	}
	volume, rest, _ := strings.Cut(rel, "/")
	return volume + `:\` + filepath.FromSlash(rest)
}
