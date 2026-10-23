package trash

import (
	"bytes"
)

// MarkerPrefix opens the one-line ownership marker a rendered text file carries: the digest of its body without the marker line.
const MarkerPrefix = "# smine-deployed "

// HasMarker reports whether line (1-based) of data starts with the marker prefix.
func HasMarker(data []byte, line int) bool {
	lines := bytes.SplitN(data, []byte{'\n'}, line+1)
	if len(lines) < line {
		return false
	}
	return bytes.HasPrefix(lines[line-1], []byte(MarkerPrefix))
}

// MarkerLine is the marker for body: the prefix plus body's digest.
func MarkerLine(body []byte) string {
	return MarkerPrefix + digest(body)
}

// StripMarker returns data without its line (1-based) when that line is a marker, else data unchanged; a writer digests the stripped bytes.
func StripMarker(data []byte, line int) []byte {
	if !HasMarker(data, line) {
		return data
	}
	lines := bytes.SplitN(data, []byte{'\n'}, line+1)
	return bytes.Join(append(lines[:line-1], lines[line:]...), []byte{'\n'})
}
