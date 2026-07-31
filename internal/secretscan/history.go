package secretscan

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type catFileSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func newCatFileSession(repoPath string) (*catFileSession, error) {
	cmd := exec.Command("git", "-C", repoPath, "cat-file", "--batch")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("newCatFileSession: Failed to open stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("newCatFileSession: Failed to open stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("newCatFileSession: Failed to start git cat-file: %w", err)
	}

	return &catFileSession{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdoutPipe)}, nil
}

func (s *catFileSession) close() error {
	s.stdin.Close()

	return s.cmd.Wait()
}

// readBlob requests one object and returns its content; nil for non-blobs and
// blobs above maxFileSize (their bytes are consumed and discarded). Strictly
// one request, then one full response — no concurrent pipe use, no goroutines.
func (s *catFileSession) readBlob(oid string) ([]byte, error) {
	if _, err := io.WriteString(s.stdin, oid+"\n"); err != nil {
		return nil, fmt.Errorf("catFileSession.readBlob: Failed to request %s: %w", oid, err)
	}

	header, err := s.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("catFileSession.readBlob: Failed to read header for %s: %w", oid, err)
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return nil, fmt.Errorf("catFileSession.readBlob: Unexpected header %q for %s", strings.TrimSpace(header), oid)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("catFileSession.readBlob: Malformed size in header %q: %w", strings.TrimSpace(header), err)
	}
	if fields[1] != "blob" || size > maxFileSize {
		if _, err := io.CopyN(io.Discard, s.stdout, size+1); err != nil {
			return nil, fmt.Errorf("catFileSession.readBlob: Failed to drain %s: %w", oid, err)
		}
		return nil, nil
	}

	content := make([]byte, size)
	if _, err := io.ReadFull(s.stdout, content); err != nil {
		return nil, fmt.Errorf("catFileSession.readBlob: Failed to read %s: %w", oid, err)
	}
	if _, err := s.stdout.Discard(1); err != nil {
		return nil, fmt.Errorf("catFileSession.readBlob: Failed to read trailer of %s: %w", oid, err)
	}

	return content, nil
}

type HistoryFinding struct {
	BlobOid    string   `json:"blobOid"`
	Commits    []string `json:"commits"`
	Confidence string   `json:"confidence"`
	Detector   string   `json:"detector"`
	Excerpt    string   `json:"excerpt"`
	Line       int      `json:"line"`
	Path       string   `json:"path"`
}

func gitOutput(repoPath string, args ...string) ([]byte, error) {
	gitArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", gitArgs...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gitOutput: git %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}

	return output, nil
}

// listObjects returns every object reachable from any ref, mapped to the path
// under which rev-list first saw it (empty for commits and the root tree).
func listObjects(repoPath string) (map[string]string, error) {
	output, err := gitOutput(repoPath, "rev-list", "--objects", "--all")
	if err != nil {
		return nil, err
	}

	objectPaths := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		oid, path, _ := strings.Cut(line, " ")
		objectPaths[oid] = path
	}

	return objectPaths, nil
}

// scanHistory scans every unique object reachable from any ref exactly once;
// non-blobs are discarded from the batch stream. Findings map back to the
// commits touching the blob.
func scanHistory(repoPath string, patterns []compiledPattern) ([]HistoryFinding, error) {
	objectPaths, err := listObjects(repoPath)
	if err != nil {
		return nil, err
	}

	oids := make([]string, 0, len(objectPaths))
	for oid := range objectPaths {
		oids = append(oids, oid)
	}
	sort.Strings(oids)

	session, err := newCatFileSession(repoPath)
	if err != nil {
		return nil, err
	}
	defer session.close()

	findings := make([]HistoryFinding, 0)
	for _, oid := range oids {
		if matchesAnyGlob(skipFileGlobs, path.Base(objectPaths[oid])) {
			continue
		}
		content, err := session.readBlob(oid)
		if err != nil {
			return nil, err
		}
		if content == nil {
			continue
		}
		if isBinary(content) {
			continue
		}
		blobFindings := scanContent(patterns, objectPaths[oid], content)
		if len(blobFindings) == 0 {
			continue
		}

		commits, err := touchingCommits(repoPath, oid)
		if err != nil {
			return nil, err
		}
		for _, finding := range blobFindings {
			findings = append(findings, HistoryFinding{
				BlobOid:    oid,
				Commits:    commits,
				Confidence: finding.Confidence,
				Detector:   finding.Detector,
				Excerpt:    finding.Excerpt,
				Line:       finding.Line,
				Path:       finding.Path,
			})
		}
	}

	sortHistoryFindings(findings)
	return findings, nil
}

func sortHistoryFindings(findings []HistoryFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Detector != findings[j].Detector {
			return findings[i].Detector < findings[j].Detector
		}

		return findings[i].BlobOid < findings[j].BlobOid
	})
}

// touchingCommits lists the commits that added or removed the blob, newest
// first — the last entry is the commit that introduced it.
func touchingCommits(repoPath string, oid string) ([]string, error) {
	output, err := gitOutput(repoPath, "log", "--all", "--format=%h", "--find-object="+oid)
	if err != nil {
		return nil, err
	}

	return strings.Fields(string(output)), nil
}
