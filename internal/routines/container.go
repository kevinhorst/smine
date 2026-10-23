package routines

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// Docker verbs SignalContainers accepts.
const (
	ContainerPause   = "pause"
	ContainerUnpause = "unpause"
)

// Plist env keys of the container opt-in, its limits and its repo mounts.
const (
	EnvContainer       = "ROUTINE_CONTAINER"
	EnvContainerCpus   = "ROUTINE_CONTAINER_CPUS"
	EnvContainerMemory = "ROUTINE_CONTAINER_MEMORY"
	EnvContainerMounts = "ROUTINE_CONTAINER_MOUNTS"
)

// Limits of a container cell whose plist leaves the key unset.
const (
	DefaultContainerCpus   = "4"
	DefaultContainerMemory = "5g"
)

const containerLabelKey = "smine.routine"

const (
	MountReadOnly  MountAccess = "ro"
	MountReadWrite MountAccess = "rw"
)

// ContainerSupported: a cell mounts every host path at that path and runs as the host uid, which a Windows host cannot give.
var ContainerSupported = runtime.GOOS != "windows"

var mountAccesses = []MountAccess{MountReadOnly, MountReadWrite}

// ContainerMount names a registered repo by its registry name, never by path.
type ContainerMount struct {
	Access MountAccess
	Repo   string
}

type MountAccess string

// ContainerLabel marks both the cell and its network holder.
func ContainerLabel(routineName string) string {
	return containerLabelKey + "=" + routineName
}

func FormatContainerMounts(mounts []*ContainerMount) string {
	entries := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		entries = append(entries, mount.Repo+":"+string(mount.Access))
	}
	return strings.Join(entries, ",")
}

// ParseContainerFlag reads ROUTINE_CONTAINER: absent or 0 is off, 1 is on.
func ParseContainerFlag(value string) (bool, error) {
	switch value {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	}
	return false, fmt.Errorf("ParseContainerFlag: Invalid value %q for %s: must be 0 or 1", value, EnvContainer)
}

// ParseContainerMounts reads ROUTINE_CONTAINER_MOUNTS: comma-separated <repo>:ro|rw entries.
func ParseContainerMounts(value string) ([]*ContainerMount, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var mounts []*ContainerMount
	seen := make(map[string]bool)
	for entry := range strings.SplitSeq(value, ",") {
		repo, access, isPair := strings.Cut(strings.TrimSpace(entry), ":")
		if !isPair || repo == "" {
			return nil, fmt.Errorf("ParseContainerMounts: Invalid mount %q: must be <repo>:ro or <repo>:rw", entry)
		}

		mountAccess := MountAccess(access)
		if !slices.Contains(mountAccesses, mountAccess) {
			return nil, fmt.Errorf("ParseContainerMounts: Invalid access %q of %s: must be ro or rw", access, repo)
		}
		if seen[repo] {
			return nil, fmt.Errorf("ParseContainerMounts: Duplicate mount %s", repo)
		}
		seen[repo] = true

		mount := &ContainerMount{Access: mountAccess, Repo: repo}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

// SignalContainers exists because a stopped docker client does not stop its container.
func SignalContainers(ctx context.Context, routineDir, verb string) error {
	_, env, err := PlistMeta(routineDir)
	if err != nil {
		return fmt.Errorf("SignalContainers: %w", err)
	}

	isContainerActive, err := ParseContainerFlag(env[EnvContainer])
	if err != nil {
		return fmt.Errorf("SignalContainers: %w", err)
	}
	if !isContainerActive {
		return nil
	}

	label := ContainerLabel(filepath.Base(routineDir))
	output, err := shell.Run(ctx, "", "docker", "ps", "--quiet", "--filter", "label="+label)
	if err != nil {
		return fmt.Errorf("SignalContainers: %w", err)
	}

	ids := strings.Fields(output)
	if len(ids) == 0 {
		return nil
	}

	args := append([]string{verb}, ids...)
	if _, err := shell.Run(ctx, "", "docker", args...); err != nil {
		return fmt.Errorf("SignalContainers: %w", err)
	}
	return nil
}
