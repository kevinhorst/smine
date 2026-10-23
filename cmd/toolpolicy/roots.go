package main

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

var (
	readGrantTools  = []string{toolNameRead, toolNameEdit, toolNameWrite}
	writeGrantTools = []string{toolNameEdit, toolNameWrite}
)

type pathAccess struct {
	cause  string
	reason string
}

// pathGrant pairs an invoked skill's path rule with one root it names.
type pathGrant struct {
	root string
	rule *Rule
}

type sessionRoots struct {
	anchor           string
	fenced           []string
	harnessWriteDirs []string
	home             string
	isAnchorRemoved  bool
	operatorPaths    []string
	policy           *toolpolicy.Policy
	protected        *protectedSet
	readDirs         []string
	readGrants       []*pathGrant
	writeDirs        []string
	writeGrants      []*pathGrant
}

func newSessionRoots(grants *Grants, home string, input *HookInput, policy *toolpolicy.Policy) *sessionRoots {
	anchor := grants.anchorRoot(input)
	own := make([]string, 0, len(grants.AdditionalDirectories)+rootsBeyondDirectories)
	if anchor != "" {
		own = append(own, anchor)
	}
	own = append(own, grants.AdditionalDirectories...)
	if input.ScratchpadDir != "" {
		own = append(own, input.ScratchpadDir)
	}

	harness := policy.ExpandedExtraWriteDirs(home)
	readDirs := slices.Concat(own, policy.ExpandedExtraReadDirs(home))
	if input.ScratchpadDir != "" {
		readDirs = append(readDirs, filepath.Dir(input.ScratchpadDir))
	}
	if root := sessionTempRoot(input, os.Getuid()); root != "" {
		readDirs = append(readDirs, root)
	}

	roots := &sessionRoots{
		anchor:           anchor,
		fenced:           policy.ReadFence,
		harnessWriteDirs: harness,
		home:             home,
		isAnchorRemoved:  grants.anchor.isMissing,
		operatorPaths:    operatorRoots(grants, policy, anchor),
		policy:           policy,
		protected:        newProtectedSet(input, policy, grants, home),
		readDirs:         readDirs,
		readGrants:       grants.skillPathGrants(home, readGrantTools),
		writeDirs:        slices.Concat(own, harness),
		writeGrants:      grants.skillPathGrants(home, writeGrantTools),
	}
	return roots
}

func (r *sessionRoots) engineProtectedCause(cwd string, target string) string {
	if IsInsideRoots(cwd, target, r.harnessWriteDirs) {
		return ""
	}
	if isEngineProtected(resolvePath(cwd, target), r.policy) {
		return r.policy.Causes.EngineProtected
	}
	return ""
}

func (r *sessionRoots) outsideAccess(cause string) pathAccess {
	if r.isAnchorRemoved {
		return pathAccess{cause: r.policy.Causes.AnchorRemoved}
	}
	return pathAccess{cause: cause}
}

func (r *sessionRoots) canRead(cwd string, path string) pathAccess {
	target := hookconfig.ExpandHome(path, r.home)
	switch {
	case IsInsideRoots(cwd, target, r.fenced):
		return pathAccess{cause: r.policy.Causes.Fenced}
	case IsInsideRoots(cwd, target, r.readDirs):
		return pathAccess{reason: r.policy.Reasons.Roots}
	case IsInsideRoots(cwd, target, systemReadDirs):
		return pathAccess{reason: r.policy.Reasons.SystemRead}
	}
	if grant := matchPathGrant(cwd, r.readGrants, target); grant != nil {
		return pathAccess{reason: r.policy.Reasons.SkillGrant + grant.rule.Raw}
	}
	return r.outsideAccess(r.policy.Causes.ReadOutside)
}

func (r *sessionRoots) canWrite(cwd string, path string) pathAccess {
	target := hookconfig.ExpandHome(path, r.home)
	if cause := r.protected.cause(cwd, r.home, target); cause != "" {
		return pathAccess{cause: cause}
	}
	if IsInsideRoots(cwd, target, r.fenced) {
		return pathAccess{cause: r.policy.Causes.Fenced}
	}
	if cause := r.engineProtectedCause(cwd, target); cause != "" {
		return pathAccess{cause: cause}
	}

	switch {
	case IsInsideRoots(cwd, target, r.writeDirs):
		return pathAccess{reason: r.policy.Reasons.Roots}
	case IsInsideRoots(cwd, target, systemWriteFiles):
		return pathAccess{reason: r.policy.Reasons.Roots}
	}
	if grant := matchPathGrant(cwd, r.writeGrants, target); grant != nil {
		return pathAccess{reason: r.policy.Reasons.SkillGrant + grant.rule.Raw}
	}
	if IsInsideRoots(cwd, target, r.policy.TmpDirs) {
		return pathAccess{cause: r.policy.Causes.TmpWrite}
	}
	return r.outsideAccess(r.policy.Causes.WriteOutside)
}

// Mirrors the engine's rm/rmdir check, which no hook allow overrides.
func (r *sessionRoots) isCriticalRemoval(cwd string, text string) bool {
	expanded := hookconfig.ExpandHome(text, r.home)
	prefix := globPrefix(expanded)
	isGlob := prefix != expanded
	plain := prefix
	if !filepath.IsAbs(plain) {
		plain = filepath.Join(cwd, plain)
	}
	targets := []string{filepath.Clean(plain), resolvePath(cwd, prefix)}
	for _, target := range targets {
		isTopLevel := target == "/" || filepath.Dir(target) == "/"
		isHome := target == filepath.Clean(r.home)
		if isTopLevel || isHome {
			return true
		}
		if !isGlob && isWorkingDirOrParent(cwd, r.anchor, target) {
			return true
		}
	}
	return false
}

func (r *sessionRoots) isOperatorPath(cwd string, path string) bool {
	return IsInsideRoots(cwd, hookconfig.ExpandHome(path, r.home), r.operatorPaths)
}

func isEngineProtected(resolved string, policy *toolpolicy.Policy) bool {
	segments := strings.Split(resolved, string(filepath.Separator))
	for index, segment := range segments {
		next := ""
		if index+1 < len(segments) {
			next = segments[index+1]
		}
		if isProtectedSegment(next, segment, policy.EngineProtectedDirs) {
			return true
		}
	}
	return isProtectedFileName(filepath.Base(resolved), policy.EngineProtectedFiles, policy.EngineProtectedFilePatterns)
}

func isProtectedFileName(name string, files []string, filePatterns []string) bool {
	if slices.Contains(files, name) {
		return true
	}
	return slices.ContainsFunc(filePatterns, func(pattern string) bool {
		isMatch, err := path.Match(pattern, name)
		return err == nil && isMatch
	})
}

func isProtectedSegment(next string, segment string, protectedDirs []string) bool {
	switch segment {
	case ".claude":
		return next != "worktrees"
	case ".config":
		return next == configGitSegment
	}
	return slices.Contains(protectedDirs, segment)
}

func isWorkingDirOrParent(cwd string, anchor string, target string) bool {
	for _, dir := range []string{anchor, cwd} {
		if dir == "" {
			continue
		}
		resolved := resolvePath(cwd, dir)
		isParent := strings.HasPrefix(resolved, target+string(filepath.Separator))
		if resolved == target || isParent {
			return true
		}
	}
	return false
}

func matchPathGrant(cwd string, grants []*pathGrant, target string) *pathGrant {
	for _, grant := range grants {
		grantRoots := []string{grant.root}
		if IsInsideRoots(cwd, target, grantRoots) {
			return grant
		}
	}
	return nil
}

func operatorRoots(grants *Grants, policy *toolpolicy.Policy, anchor string) []string {
	hasGateRepo := anchor != "" && grants.gateRepo != ""
	isGateWorktree := hasGateRepo && isLinkedWorktree(anchor)
	if !isGateWorktree || gitroot.MainRoot(anchor) != grants.gateRepo {
		return nil
	}

	root := gitroot.CheckoutRoot(anchor)
	roots := make([]string, 0, len(policy.OperatorPaths))
	for _, path := range policy.OperatorPaths {
		roots = append(roots, filepath.Join(root, path))
	}
	return roots
}

func pathRuleRoots(home string, repoPaths []string, spec string) []string {
	dir, isRecursive := strings.CutSuffix(spec, pathRuleRecursiveSuffix)
	if !isRecursive {
		return nil
	}
	switch {
	case dir == reposRuleRoot:
		return repoPaths
	case strings.HasPrefix(dir, "~/"):
		return []string{hookconfig.ExpandHome(dir, home)}
	case strings.HasPrefix(dir, "//"):
		return []string{dir[1:]}
	}
	return nil
}

// The harness keys its temp dir by a project dir that varies between a session's cwd, launch dir and Desktop project, and by the session id, which never varies.
func sessionTempRoot(input *HookInput, uid int) string {
	isGlobSafeId := input.SessionId != "" && !strings.ContainsAny(input.SessionId, globMetaChars)
	if !isGlobSafeId || uid < 0 {
		return ""
	}
	return filepath.Join(sessionTempBase, sessionTempPrefix+strconv.Itoa(uid), globMarker, input.SessionId)
}
