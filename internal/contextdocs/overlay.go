package contextdocs

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
)

// OverlayPatchName is the patch file beside a repo's build record.
const OverlayPatchName = "overlay.patch"

type OverlayFile struct {
	Content []byte
	Path    string
}

// RenderOverlayPatch writes one new-file hunk per file in the form git apply takes at -p1; every file must end in a newline so no end-of-file marker is needed.
func RenderOverlayPatch(files []OverlayFile) ([]byte, error) {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b OverlayFile) int { return strings.Compare(a.Path, b.Path) })

	patch := &bytes.Buffer{}
	for _, file := range sorted {
		if !bytes.HasSuffix(file.Content, []byte("\n")) {
			return nil, fmt.Errorf("RenderOverlayPatch: %s: Content must end in a newline", file.Path)
		}
		lines := strings.Split(strings.TrimSuffix(string(file.Content), "\n"), "\n")
		fmt.Fprintf(patch, "diff --git a/%s b/%s\n", file.Path, file.Path)
		patch.WriteString("new file mode 100644\n")
		patch.WriteString("--- /dev/null\n")
		fmt.Fprintf(patch, "+++ b/%s\n", file.Path)
		fmt.Fprintf(patch, "@@ -0,0 +1,%d @@\n", len(lines))
		for _, line := range lines {
			patch.WriteString("+" + line + "\n")
		}
	}
	return patch.Bytes(), nil
}
