package install

import (
	"os"
	"path/filepath"
)

// PayloadMarker is the file a release archive carries under bin/: its presence makes every binary prebuilt, so nothing builds.
const PayloadMarker = "PAYLOAD"

// HasPayload reports whether the checkout at repoDir is an extracted release archive.
func HasPayload(repoDir string) bool {
	_, err := os.Stat(PayloadMarkerPath(repoDir))
	return err == nil
}

func PayloadMarkerPath(repoDir string) string {
	return filepath.Join(repoDir, "bin", PayloadMarker)
}
