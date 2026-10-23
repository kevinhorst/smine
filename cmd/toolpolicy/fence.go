package main

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

// argRole is how the fence treats one argument (D23).
type argRole int

const (
	// roleGeneric: checked for reading when path-shaped.
	roleGeneric argRole = iota
	// roleNone: never a path (patterns, scripts, names, numbers).
	roleNone
	// roleRead: always a path that is read.
	roleRead
	// roleWrite: always a path that is written.
	roleWrite
	// roleAtFile: a path read only when prefixed with "@" (curl -d @file).
	roleAtFile
)

type argWord struct {
	isResolved bool
	text       string
}

type fence struct {
	cwd        string
	hasCd      bool
	isCwdKnown bool
	policy     *toolpolicy.Policy
	roots      *sessionRoots
	vars       map[string]string
}

func newFence(cwd string, policy *toolpolicy.Policy, roots *sessionRoots) *fence {
	instance := &fence{
		cwd:        cwd,
		isCwdKnown: true,
		policy:     policy,
		roots:      roots,
		vars:       map[string]string{"HOME": roots.home, "PWD": cwd},
	}
	return instance
}

func (f *fence) assign(assigns []*syntax.Assign) string {
	for _, assign := range assigns {
		if assign.Naked || assign.Name == nil {
			continue
		}
		name := assign.Name.Value
		if slices.Contains(fenceReservedVars, name) {
			return f.policy.Causes.FenceConstruct
		}

		value, isResolved := f.expandAssign(assign)
		if !isResolved {
			delete(f.vars, name)
			continue
		}
		f.vars[name] = value
	}
	return ""
}

// awk denies programs that write or execute, then checks file operands.
func (f *fence) awk(call *fencedCall) string {
	for _, arg := range call.args {
		if arg.isResolved && awkWriteConstruct.MatchString(arg.text) {
			return f.policy.Causes.AwkWrite
		}
	}
	return f.checkSpec(call.args, awkSpec)
}

func (f *fence) binary(binary *syntax.BinaryCmd) string {
	switch binary.Op {
	case syntax.Pipe, syntax.PipeAll:
		if cause := f.clone().stmt(binary.X); cause != "" {
			return cause
		}
		return f.clone().stmt(binary.Y)
	case syntax.AndStmt:
		if cause := f.stmt(binary.X); cause != "" {
			return cause
		}
		return f.stmt(binary.Y)
	case syntax.OrStmt:
		if cause := f.stmt(binary.X); cause != "" {
			return cause
		}
		branch := f.clone()
		if cause := branch.stmt(binary.Y); cause != "" {
			return cause
		}
		f.join(branch)
		return ""
	}
	return f.policy.Causes.FenceConstruct
}

func (f *fence) caseClause(clause *syntax.CaseClause) string {
	if cause := f.substs(clause.Word); cause != "" {
		return cause
	}
	for _, item := range clause.Items {
		branch := f.clone()
		if cause := branch.stmts(item.Stmts); cause != "" {
			return cause
		}
		f.join(branch)
	}
	return ""
}

// changeDir moves the tracked cwd; the target must be readable (D24).
func (f *fence) changeDir(call *fencedCall) string {
	target := &argWord{isResolved: true, text: f.roots.home}
	for _, arg := range call.args {
		isPhysicalFlag := arg.text == "-L" || arg.text == "-P"
		isOption := arg.isResolved && isPhysicalFlag
		if !isOption {
			target = arg
			break
		}
	}
	if !target.isResolved {
		return f.policy.Causes.UnresolvablePath
	}
	if target.text == "-" {
		return f.policy.Causes.FenceConstruct
	}
	if cause := f.checkPath(target.text, roleRead); cause != "" {
		return cause
	}

	moved := resolvePath(f.cwd, hookconfig.ExpandHome(target.text, f.roots.home))
	f.hasCd = f.hasCd || moved != resolvePath(f.cwd, "")
	f.cwd = moved
	f.isCwdKnown = true
	f.vars["PWD"] = f.cwd
	return ""
}

func (f *fence) checkArg(arg *argWord, role argRole) string {
	if arg.isResolved {
		return f.checkText(role, arg.text)
	}
	switch role {
	case roleNone:
		return ""
	case roleGeneric:
		if isPathShaped(arg.text) {
			return f.policy.Causes.UnresolvablePath
		}
		return ""
	case roleAtFile:
		if strings.HasPrefix(arg.text, "@") {
			return f.policy.Causes.UnresolvablePath
		}
		return ""
	case roleRead, roleWrite:
		return f.policy.Causes.UnresolvablePath
	}
	return f.policy.Causes.UnresolvablePath
}

func (f *fence) checkCall(args []*argWord, assigned []*argWord) string {
	if len(args) == 0 {
		return ""
	}

	call, cause := unwrapCall(args, f.policy)
	if call == nil {
		return cause
	}
	call.env = append(call.env, assigned...)
	if isPathShaped(call.head) {
		if cause := f.checkPath(call.head, roleRead); cause != "" {
			return cause
		}
		if f.roots.isOperatorPath(f.cwd, call.head) {
			return f.policy.Causes.OperatorCommand
		}
	}

	name := filepath.Base(call.head)
	if call.isXargs && !isReadOnlyCall(argTexts(call.args), hasUnresolved(call.args), name, f.policy) {
		return f.policy.Causes.XargsWrite
	}
	if cause := f.protectedArgs(name, call.args); cause != "" {
		return cause
	}
	switch name {
	case "awk", "gawk":
		return f.awk(call)
	case "bash", "sh", "zsh":
		return f.shell(call)
	case "cd", "pushd":
		return f.changeDir(call)
	case "dd":
		return f.dd(call)
	case "eval", "popd":
		return f.policy.Causes.FenceConstruct
	case findHead:
		return f.find(call)
	case gitHead:
		return f.git(call)
	case goHead:
		return f.goCall(call)
	case "rm", "rmdir":
		return f.remove(call)
	case sedHead:
		return f.sed(call)
	case "tar":
		return f.tar(call)
	}
	return f.checkSpec(call.args, specFor(name))
}

func (f *fence) checkFlag(args []*argWord, index int, spec *headSpec) (int, string) {
	text := args[index].text
	name, value, isJoined := strings.Cut(text, "=")
	roles, hasValue := spec.valueFlags[name]
	isClusterValue := !hasValue && !isJoined && isShortCluster(text) && len(text) > shortFlagLength
	if isClusterValue {
		name, value, isJoined = text[:shortFlagLength], text[shortFlagLength:], true
		roles, hasValue = spec.valueFlags[name]
	}

	switch {
	case !hasValue && isJoined:
		return 0, f.checkText(roleGeneric, value)
	case !hasValue:
		return 0, ""
	case isJoined:
		return 0, f.checkText(roles[0], value)
	}

	values := args[index+1:]
	if len(values) < len(roles) {
		return len(values), ""
	}
	for offset, role := range roles {
		if cause := f.checkArg(values[offset], role); cause != "" {
			return 0, cause
		}
	}
	return len(roles), ""
}

func (f *fence) checkPath(path string, role argRole) string {
	target := globPrefix(path)
	isRelative := !filepath.IsAbs(target) && !strings.HasPrefix(target, "~")
	if isRelative && !f.isCwdKnown {
		return f.policy.Causes.UnresolvablePath
	}
	if role == roleWrite {
		return f.roots.canWrite(f.cwd, target).cause
	}
	return f.roots.canRead(f.cwd, target).cause
}

func (f *fence) checkPositionals(hasScriptFlag bool, positionals []*argWord, spec *headSpec) string {
	leading := spec.leading
	if hasScriptFlag && len(leading) > 0 {
		leading = leading[1:]
	}
	for index, arg := range positionals {
		role := spec.positional
		isDestination := spec.isDestinationLast && len(positionals) > 1 && index == len(positionals)-1
		switch {
		case index < len(leading):
			role = leading[index]
		case isDestination:
			role = roleWrite
		}
		if cause := f.checkArg(arg, role); cause != "" {
			return cause
		}
	}
	return ""
}

func (f *fence) checkSpec(args []*argWord, spec *headSpec) string {
	positionals := make([]*argWord, 0, len(args))
	hasScriptFlag := false
	isFlagsDone := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		isFlag := !isFlagsDone && arg.isResolved && strings.HasPrefix(arg.text, "-") && arg.text != "-"
		switch {
		case isFlag && arg.text == "--":
			isFlagsDone = true
		case isFlag:
			consumed, cause := f.checkFlag(args, index, spec)
			if cause != "" {
				return cause
			}
			hasScriptFlag = hasScriptFlag || isScriptFlag(arg.text, spec)
			index += consumed
		default:
			positionals = append(positionals, arg)
		}
	}
	return f.checkPositionals(hasScriptFlag, positionals, spec)
}

func (f *fence) checkText(role argRole, text string) string {
	switch role {
	case roleNone:
		return ""
	case roleGeneric:
		if !isPathShaped(text) {
			return ""
		}
		return f.checkPath(text, roleRead)
	case roleAtFile:
		file, isFile := strings.CutPrefix(text, "@")
		if !isFile {
			return ""
		}
		return f.checkPath(file, roleRead)
	case roleRead, roleWrite:
		return f.checkPath(text, role)
	}
	return f.policy.Causes.UnresolvablePath
}

func (f *fence) clone() *fence {
	copied := *f
	copied.vars = maps.Clone(f.vars)
	return &copied
}

func (f *fence) command(command syntax.Command) string {
	switch typed := command.(type) {
	case nil:
		return ""
	case *syntax.CallExpr:
		return f.call(typed)
	case *syntax.BinaryCmd:
		return f.binary(typed)
	case *syntax.Subshell:
		return f.clone().stmts(typed.Stmts)
	case *syntax.Block:
		return f.stmts(typed.Stmts)
	case *syntax.IfClause:
		return f.ifClause(typed)
	case *syntax.WhileClause:
		return f.whileClause(typed)
	case *syntax.ForClause:
		return f.forClause(typed)
	case *syntax.CaseClause:
		return f.caseClause(typed)
	case *syntax.DeclClause:
		return f.declClause(typed)
	case *syntax.TimeClause:
		if typed.Stmt == nil {
			return ""
		}
		return f.stmt(typed.Stmt)
	case *syntax.TestClause, *syntax.ArithmCmd, *syntax.LetClause:
		return f.substs(typed)
	}
	return f.policy.Causes.FenceConstruct
}

func (f *fence) call(call *syntax.CallExpr) string {
	for _, assign := range call.Assigns {
		if cause := f.substs(assign); cause != "" {
			return cause
		}
	}
	if len(call.Args) == 0 {
		return f.assign(call.Assigns)
	}

	args, cause := f.expandArgs(call.Args)
	if cause != "" {
		return cause
	}
	return f.checkCall(args, assignPairs(call.Assigns))
}

func (f *fence) config() *expand.Config {
	pairs := make([]string, 0, len(f.vars))
	for name, value := range f.vars {
		pairs = append(pairs, name+"="+value)
	}
	config := &expand.Config{
		Env:     expand.ListEnviron(pairs...),
		NoUnset: true,
	}
	return config
}

// dd reads if= and writes of=.
func (f *fence) dd(call *fencedCall) string {
	for _, arg := range call.args {
		if !arg.isResolved {
			return f.policy.Causes.UnresolvablePath
		}
		key, value, _ := strings.Cut(arg.text, "=")
		role := roleNone
		switch key {
		case "if":
			role = roleRead
		case "of":
			role = roleWrite
		}
		if cause := f.checkText(role, value); cause != "" {
			return cause
		}
	}
	return ""
}

// declClause fences export/local/declare values and propagates literal ones.
func (f *fence) declClause(clause *syntax.DeclClause) string {
	for _, assign := range clause.Args {
		if cause := f.substs(assign); cause != "" {
			return cause
		}
	}
	return f.assign(clause.Args)
}

func (f *fence) expandArgs(words []*syntax.Word) ([]*argWord, string) {
	args := make([]*argWord, 0, len(words))
	for _, word := range words {
		if cause := f.substs(word); cause != "" {
			return nil, cause
		}
		fields, err := expand.Fields(f.config(), word)
		if err != nil {
			args = append(args, &argWord{text: rawWord(word)})
			continue
		}
		for _, field := range fields {
			args = append(args, &argWord{isResolved: true, text: field})
		}
	}
	return args, ""
}

func (f *fence) expandAssign(assign *syntax.Assign) (string, bool) {
	isScalar := !assign.Append && assign.Array == nil && assign.Index == nil
	if !isScalar {
		return "", false
	}
	if assign.Value == nil {
		return "", true
	}
	if f.substs(assign.Value) != "" {
		return "", false
	}

	value, err := expand.Literal(f.config(), assign.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

// -exec and friends are denied later by isExecCapableFind.
func (f *fence) find(call *fencedCall) string {
	index := 0
	for index < len(call.args) && isFindOption(call.args[index]) {
		index++
	}
	for ; index < len(call.args); index++ {
		arg := call.args[index]
		if isFindExpressionStart(arg) {
			break
		}
		if cause := f.checkArg(arg, roleRead); cause != "" {
			return cause
		}
	}
	return f.checkSpec(call.args[index:], findExpressionSpec)
}

func (f *fence) forClause(clause *syntax.ForClause) string {
	switch loop := clause.Loop.(type) {
	case *syntax.WordIter:
		return f.wordLoop(clause.Do, loop)
	case *syntax.CStyleLoop:
		if cause := f.substs(loop); cause != "" {
			return cause
		}
		return f.repeat(clause.Do, f.clone())
	}
	return f.policy.Causes.FenceConstruct
}

func (f *fence) git(call *fencedCall) string {
	if f.hasCd {
		return f.policy.Causes.CdGit
	}
	if slices.ContainsFunc(call.env, isRefusedGitEnvironment) {
		return f.policy.Causes.GitEnvironment
	}

	end, isAdmitted := gitGlobalsEnd(alignedTexts(call.args), f.policy.GitAdmittedFlags)
	if !isAdmitted {
		return f.policy.Causes.GitGlobalOption
	}
	rest := call.args[end:]
	if cause := gitExecCause(&f.policy.Causes, alignedTexts(rest)); cause != "" {
		return cause
	}

	scope := f.clone()
	isWrite := !isReadOnlyCall(argTexts(call.args), hasUnresolved(call.args), gitHead, f.policy)
	if cause := scope.gitDirectories(call.args[:end], isWrite); cause != "" {
		return cause
	}
	if cause := scope.gitIndexFiles(call.env); cause != "" {
		return cause
	}

	key, subArgs := gitSubcommand(rest)
	spec, isKnown := gitSubcommandSpecs[key]
	if !isKnown {
		spec = gitDefaultSpec
	}
	return scope.checkSpec(subArgs, spec)
}

func (f *fence) gitDirectories(globals []*argWord, isWrite bool) string {
	for index := 0; index < len(globals); index++ {
		if globals[index].text != gitDirectoryFlag {
			continue
		}
		index++
		if cause := f.gitTarget(globals[index], isWrite); cause != "" {
			return cause
		}
	}
	return ""
}

// gitIndexFiles checks every GIT_INDEX_FILE value as a written path.
func (f *fence) gitIndexFiles(env []*argWord) string {
	for _, pair := range env {
		name, value, _ := strings.Cut(pair.text, "=")
		if name != gitIndexFileName {
			continue
		}
		if !pair.isResolved {
			return f.policy.Causes.UnresolvablePath
		}
		if cause := f.checkPath(value, roleWrite); cause != "" {
			return cause
		}
	}
	return ""
}

func (f *fence) gitTarget(arg *argWord, isWrite bool) string {
	if !arg.isResolved {
		return f.policy.Causes.UnresolvablePath
	}

	role := roleRead
	if isWrite {
		role = roleWrite
	}
	if cause := f.checkPath(arg.text, role); cause != "" {
		if f.isSameRepository(arg.text) {
			return f.policy.Causes.GitSameRepo
		}
		return cause
	}
	f.cwd = resolvePath(f.cwd, hookconfig.ExpandHome(arg.text, f.roots.home))
	return ""
}

// Words after -args are the test binary's own flags, not go's.
func (f *fence) goCall(call *fencedCall) string {
	for _, arg := range call.args {
		if !arg.isResolved {
			return f.policy.Causes.GoProgramFlag
		}
		isArgsSeparator := arg.text == goArgsSeparator || arg.text == "-"+goArgsSeparator
		if isArgsSeparator {
			break
		}
		if isGoProgramFlag(arg.text) {
			return f.policy.Causes.GoProgramFlag
		}
	}
	return f.checkSpec(call.args, specFor(goHead))
}

func (f *fence) ifClause(clause *syntax.IfClause) string {
	if cause := f.stmts(clause.Cond); cause != "" {
		return cause
	}
	then := f.clone()
	if cause := then.stmts(clause.Then); cause != "" {
		return cause
	}
	f.join(then)
	if clause.Else == nil {
		return ""
	}

	otherwise := f.clone()
	if cause := otherwise.ifClause(clause.Else); cause != "" {
		return cause
	}
	f.join(otherwise)
	return ""
}

func (f *fence) isSameRepository(path string) bool {
	if f.roots.anchor == "" {
		return false
	}
	target := resolvePath(f.cwd, hookconfig.ExpandHome(path, f.roots.home))
	return gitroot.MainRoot(target) == gitroot.MainRoot(f.roots.anchor)
}

func (f *fence) join(branch *fence) {
	isCwdChanged := branch.cwd != f.cwd || !branch.isCwdKnown
	if isCwdChanged {
		f.isCwdKnown = false
		delete(f.vars, "PWD")
	}
	f.hasCd = f.hasCd || branch.hasCd
	for name, value := range f.vars {
		branchValue, isKept := branch.vars[name]
		if !isKept || branchValue != value {
			delete(f.vars, name)
		}
	}
}

func (f *fence) nestedScript(script *argWord) string {
	if !script.isResolved {
		return f.policy.Causes.UnresolvablePath
	}

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(script.text), "")
	if err != nil {
		return f.policy.Causes.FenceUnparseable
	}
	if cause := preflightCause(&f.policy.Causes, file); cause != "" {
		return cause
	}
	return f.clone().stmts(file.Stmts)
}

// The role checks see listed flags only, so every word and joined -flag=value is checked here.
func (f *fence) protectedArgs(name string, args []*argWord) string {
	if isReadOnlyCall(argTexts(args), hasUnresolved(args), name, f.policy) {
		return ""
	}
	for _, arg := range args {
		candidate, isCandidate := pathCandidate(arg)
		if !isCandidate {
			continue
		}
		if cause := f.roots.protected.cause(f.cwd, f.roots.home, candidate); cause != "" {
			return cause
		}
	}
	return ""
}

func (f *fence) redirect(redirect *syntax.Redirect) string {
	if redirect.Hdoc != nil {
		if cause := f.substs(redirect.Hdoc); cause != "" {
			return cause
		}
	}
	if redirect.Word == nil {
		return ""
	}
	if cause := f.substs(redirect.Word); cause != "" {
		return cause
	}

	fields, err := expand.Fields(f.config(), redirect.Word)
	isResolved := err == nil && len(fields) == 1
	target := rawWord(redirect.Word)
	if isResolved {
		target = fields[0]
	}

	role := roleNone
	switch {
	case redirect.Op == syntax.RdrIn:
		role = roleRead
	case isWriteRedirect(redirect.Op, target):
		role = roleWrite
	}
	if role == roleNone {
		return ""
	}
	if !isResolved && f.hasCd {
		return f.policy.Causes.CdRedirect
	}
	if !isResolved {
		return f.policy.Causes.UnresolvablePath
	}
	return f.checkPath(target, role)
}

func (f *fence) remove(call *fencedCall) string {
	for _, arg := range call.args {
		isTarget := arg.isResolved && !strings.HasPrefix(arg.text, "-")
		if isTarget && f.roots.isCriticalRemoval(f.cwd, arg.text) {
			return f.policy.Causes.CriticalRemove
		}
	}
	return f.checkSpec(call.args, writePathSpec)
}

func (f *fence) repeat(body []*syntax.Stmt, loop *fence) string {
	for range loopIterationsModeled {
		if cause := loop.stmts(body); cause != "" {
			return cause
		}
	}
	f.join(loop)
	return ""
}

func (f *fence) sed(call *fencedCall) string {
	args := dropSedSuffixWord(call.args)
	for _, arg := range args {
		if !arg.isResolved {
			return f.policy.Causes.UnresolvablePath
		}
		isScriptWrite := sedWriteCommand.MatchString(arg.text) || sedSubstituteWrite.MatchString(arg.text)
		if isScriptWrite && !strings.HasPrefix(arg.text, "-") {
			return f.policy.Causes.SedScriptWrite
		}
	}
	if slices.ContainsFunc(args, isSedInPlaceFlag) {
		return f.checkSpec(args, sedWriteSpec)
	}
	return f.checkSpec(args, sedReadSpec)
}

func (f *fence) shell(call *fencedCall) string {
	for index := 0; index < len(call.args); index++ {
		arg := call.args[index]
		switch {
		case !arg.isResolved:
			return f.policy.Causes.UnresolvablePath
		case arg.text == "-o" || arg.text == "+o":
			index++
		case isShortCluster(arg.text) && strings.ContainsRune(arg.text, 'c'):
			if index+1 >= len(call.args) {
				return ""
			}
			return f.nestedScript(call.args[index+1])
		case strings.HasPrefix(arg.text, "-"):
		default:
			if f.roots.isOperatorPath(f.cwd, arg.text) {
				return f.policy.Causes.OperatorCommand
			}
			return f.checkArg(arg, roleRead)
		}
	}
	return ""
}

func (f *fence) stmt(stmt *syntax.Stmt) string {
	for _, redirect := range stmt.Redirs {
		if cause := f.redirect(redirect); cause != "" {
			return cause
		}
	}
	return f.command(stmt.Cmd)
}

func (f *fence) stmts(stmts []*syntax.Stmt) string {
	for _, stmt := range stmts {
		if cause := f.stmt(stmt); cause != "" {
			return cause
		}
	}
	return ""
}

func (f *fence) substs(node syntax.Node) string {
	cause := ""
	syntax.Walk(node, func(child syntax.Node) bool {
		if cause != "" {
			return false
		}
		subst, isSubst := child.(*syntax.CmdSubst)
		if !isSubst {
			return true
		}
		cause = f.clone().stmts(subst.Stmts)
		return false
	})
	return cause
}

func (f *fence) tar(call *fencedCall) string {
	args := splitTarClusters(call.args)
	mode := tarMode(args)
	archiveRole, dirRole, positionalRole := roleRead, roleRead, roleRead
	if strings.ContainsAny(mode, "cru") {
		archiveRole = roleWrite
	}
	if strings.Contains(mode, "x") {
		dirRole = roleWrite
		positionalRole = roleNone
	}

	spec := &headSpec{
		positional: positionalRole,
		valueFlags: map[string][]argRole{
			directoryFlag:    {dirRole},
			"--exclude-from": {roleRead},
			fileFlag:         {archiveRole},
			"--files-from":   {roleRead},
			"-C":             {dirRole},
			"-T":             {roleRead},
			"-X":             {roleRead},
			"-f":             {archiveRole},
		},
	}
	return f.checkSpec(args, spec)
}

func (f *fence) whileClause(clause *syntax.WhileClause) string {
	loop := f.clone()
	for range loopIterationsModeled {
		if cause := loop.stmts(clause.Cond); cause != "" {
			return cause
		}
		if cause := loop.stmts(clause.Do); cause != "" {
			return cause
		}
	}
	f.join(loop)
	return ""
}

func (f *fence) wordLoop(body []*syntax.Stmt, iter *syntax.WordIter) string {
	name := iter.Name.Value
	if slices.Contains(fenceReservedVars, name) {
		return f.policy.Causes.FenceConstruct
	}
	items, cause := f.expandArgs(iter.Items)
	if cause != "" {
		return cause
	}

	loop := f.clone()
	isEnumerable := iter.InPos.IsValid() && len(items) <= f.policy.MaxLoopItems && !hasUnresolved(items)
	if !isEnumerable {
		delete(loop.vars, name)
		return f.repeat(body, loop)
	}
	for _, item := range items {
		loop.vars[name] = item.text
		if cause := loop.stmts(body); cause != "" {
			return cause
		}
	}
	f.join(loop)
	return ""
}

// fencedCall is one simple command after wrapper unwrapping.
type fencedCall struct {
	args []*argWord
	// env holds the prefix assignments and env wrapper NAME=value pairs.
	env     []*argWord
	head    string
	isXargs bool
}

// headSpec is one command's argument roles.
type headSpec struct {
	// isDestinationLast: with two or more positionals the last is written.
	isDestinationLast bool
	// leading: roles of the first positionals (a grep pattern, a sed script).
	leading []argRole
	// positional: role of every other positional.
	positional argRole
	// scriptFlags: a flag that supplies the script, so leading[0] is dropped.
	scriptFlags []string
	// valueFlags: flags whose value(s) follow, with the value roles.
	valueFlags map[string][]argRole
}

func fenceBash(input *HookInput, policy *toolpolicy.Policy, roots *sessionRoots) (cause string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			cause = policy.Causes.FenceUnparseable
		}
	}()

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(input.ToolInput.Command), "")
	if err != nil {
		return policy.Causes.FenceUnparseable
	}
	if cause := preflightCause(&policy.Causes, file); cause != "" {
		return cause
	}
	return newFence(input.Cwd, policy, roots).stmts(file.Stmts)
}

func alignedTexts(args []*argWord) []string {
	texts := make([]string, 0, len(args))
	for _, arg := range args {
		if !arg.isResolved {
			texts = append(texts, "")
			continue
		}
		texts = append(texts, arg.text)
	}
	return texts
}

func argTexts(args []*argWord) []string {
	texts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.isResolved {
			texts = append(texts, arg.text)
		}
	}
	return texts
}

func assignPairs(assigns []*syntax.Assign) []*argWord {
	pairs := make([]*argWord, 0, len(assigns))
	for _, assign := range assigns {
		if assign.Name == nil {
			continue
		}
		value, isLiteral := "", true
		if assign.Value != nil {
			value, isLiteral = wordLiteral(assign.Value)
		}
		pair := &argWord{isResolved: isLiteral, text: assign.Name.Value + "=" + value}
		pairs = append(pairs, pair)
	}
	return pairs
}

// BSD sed takes an in-place suffix word ("" or ".bak") after a bare -i.
func dropSedSuffixWord(args []*argWord) []*argWord {
	kept := make([]*argWord, 0, len(args))
	for index := 0; index < len(args); index++ {
		kept = append(kept, args[index])
		isBareInPlace := args[index].isResolved && args[index].text == "-i"
		if isBareInPlace && index+1 < len(args) && isSedSuffixWord(args[index+1]) {
			index++
		}
	}
	return kept
}

func globPrefix(text string) string {
	index := strings.IndexAny(text, "*?[")
	if index < 0 {
		return text
	}
	cut := strings.LastIndex(text[:index], "/")
	switch {
	case cut < 0:
		return "."
	case cut == 0:
		return "/"
	}
	return text[:cut]
}

func gitSubcommand(rest []*argWord) (string, []*argWord) {
	if len(rest) == 0 {
		return "", nil
	}
	key := rest[0].text
	if key == "worktree" && len(rest) > 1 {
		return key + " " + rest[1].text, rest[2:]
	}
	return key, rest[1:]
}

func hasTildeUser(word *syntax.Word) bool {
	if len(word.Parts) == 0 {
		return false
	}
	literal, isLiteral := word.Parts[0].(*syntax.Lit)
	if !isLiteral || !strings.HasPrefix(literal.Value, "~") {
		return false
	}
	return len(literal.Value) > 1 && literal.Value[1] != '/'
}

func hasUnresolved(args []*argWord) bool {
	return slices.ContainsFunc(args, func(arg *argWord) bool {
		return !arg.isResolved
	})
}

func isFenceWrapper(head string, wrapperCommands []string) bool {
	return slices.Contains(wrapperCommands, head) || slices.Contains(fenceOnlyWrappers, head)
}

func isFindExpressionStart(arg *argWord) bool {
	if !arg.isResolved {
		return false
	}
	return strings.HasPrefix(arg.text, "-") || slices.Contains(findExpressionOpeners, arg.text)
}

func isFindOption(arg *argWord) bool {
	return arg.isResolved && slices.Contains(findLeadingOptions, arg.text)
}

func isGoProgramFlag(text string) bool {
	name, _, _ := strings.Cut(strings.TrimLeft(text, "-"), "=")
	return strings.HasPrefix(text, "-") && slices.Contains(goProgramFlags, name)
}

func isPathShaped(text string) bool {
	if strings.Contains(text, "://") {
		return false
	}
	switch {
	case text == "." || text == "..":
		return true
	case strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~"):
		return true
	}
	return strings.Contains(text, "/")
}

func isScriptFlag(text string, spec *headSpec) bool {
	name, _, _ := strings.Cut(text, "=")
	if slices.Contains(spec.scriptFlags, name) {
		return true
	}
	return isShortCluster(text) && len(text) > shortFlagLength && slices.Contains(spec.scriptFlags, text[:shortFlagLength])
}

func isSedInPlaceFlag(arg *argWord) bool {
	if !arg.isResolved {
		return false
	}
	if strings.HasPrefix(arg.text, "--in-place") {
		return true
	}
	return isShortCluster(arg.text) && strings.ContainsRune(arg.text[1:], 'i')
}

func isSedSuffixWord(arg *argWord) bool {
	return arg.isResolved && (arg.text == "" || strings.HasPrefix(arg.text, "."))
}

func isShortCluster(text string) bool {
	return len(text) > 1 && text[0] == '-' && text[1] != '-'
}

func pathCandidate(arg *argWord) (string, bool) {
	if !arg.isResolved {
		return "", false
	}
	if !strings.HasPrefix(arg.text, "-") {
		return arg.text, true
	}
	_, value, isJoined := strings.Cut(arg.text, "=")
	return value, isJoined
}

// The expander must never see ProcSubst (it panics) or ~user (it calls os/user).
func preflightCause(causes *hookconfig.ToolpolicyCauses, file *syntax.File) string {
	cause := ""
	syntax.Walk(file, func(node syntax.Node) bool {
		switch typed := node.(type) {
		case *syntax.ProcSubst:
			cause = causes.ProcSubst
		case *syntax.FuncDecl, *syntax.CoprocClause:
			cause = causes.FenceConstruct
		case *syntax.Word:
			if hasTildeUser(typed) {
				cause = causes.TildeUser
			}
		}
		return cause == ""
	})
	return cause
}

func rawWord(word *syntax.Word) string {
	builder := strings.Builder{}
	if err := syntax.NewPrinter().Print(&builder, word); err != nil {
		return ""
	}
	return builder.String()
}

func skipWrapperArgs(args []*argWord, wrapper string) ([]*argWord, []*argWord) {
	valueFlags := wrapperValueFlags[wrapper]
	isDurationSkipped := wrapper != timeoutWrapper
	env := make([]*argWord, 0)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case !arg.isResolved:
			return args[index:], env
		case slices.Contains(valueFlags, arg.text):
			index++
		case strings.HasPrefix(arg.text, "-"):
		case wrapper == envWrapper && strings.Contains(arg.text, "="):
			env = append(env, arg)
		case !isDurationSkipped:
			isDurationSkipped = true
		default:
			return args[index:], env
		}
	}
	return nil, env
}

func specFor(head string) *headSpec {
	if spec, isKnown := headSpecs[head]; isKnown {
		return spec
	}
	return genericSpec
}

func splitTarClusters(args []*argWord) []*argWord {
	split := make([]*argWord, 0, len(args))
	for index, arg := range args {
		text := arg.text
		isBundledMode := index == 0 && arg.isResolved && text != "" && strings.Trim(text, tarModeLetters) == ""
		if isBundledMode {
			text = "-" + text
		}
		isCluster := arg.isResolved && isShortCluster(text) && len(text) > shortFlagLength
		if !isCluster {
			split = append(split, arg)
			continue
		}
		for _, letter := range text[1:] {
			split = append(split, &argWord{isResolved: true, text: "-" + string(letter)})
		}
	}
	return split
}

// tarMode collects tar's operation letters from its flags.
func tarMode(args []*argWord) string {
	mode := strings.Builder{}
	for _, arg := range args {
		letter, isLong := tarLongModes[arg.text]
		switch {
		case isLong:
			mode.WriteString(letter)
		case len(arg.text) == 2 && arg.text[0] == '-':
			mode.WriteByte(arg.text[1])
		}
	}
	return mode.String()
}

func unwrapCall(args []*argWord, policy *toolpolicy.Policy) (*fencedCall, string) {
	if !args[0].isResolved {
		return nil, policy.Causes.UnresolvableHead
	}

	call := &fencedCall{args: args[1:], head: args[0].text}
	for isFenceWrapper(filepath.Base(call.head), policy.WrapperCommands) {
		wrapper := filepath.Base(call.head)
		call.isXargs = call.isXargs || wrapper == xargsWrapper
		remaining, env := skipWrapperArgs(call.args, wrapper)
		call.env = append(call.env, env...)
		if len(remaining) == 0 {
			return nil, ""
		}
		if !remaining[0].isResolved {
			return nil, policy.Causes.UnresolvableHead
		}
		call.head = remaining[0].text
		call.args = remaining[1:]
	}
	return call, ""
}
