package main

import (
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/kevinhorst/smine/internal/toolpolicy"
)

type classifier struct {
	execCalls           int
	hasUnknownConstruct bool
	isReadOnly          bool
	policy              *toolpolicy.Policy
}

func newClassifier(policy *toolpolicy.Policy) *classifier {
	instance := &classifier{
		isReadOnly: true,
		policy:     policy,
	}
	return instance
}

func (c *classifier) resolveCall(call *syntax.CallExpr) (*resolvedCall, bool) {
	words := call.Args
	head, isLiteral := wordLiteral(words[0])
	if !isLiteral {
		return nil, false
	}

	words = words[1:]
	for slices.Contains(c.policy.WrapperCommands, head) {
		words = skipWrapperTokens(head, words)
		if len(words) == 0 {
			// A bare wrapper (plain xargs, env) executes nothing itself.
			bare := &resolvedCall{head: "echo"}
			return bare, true
		}

		next, nextIsLiteral := wordLiteral(words[0])
		if !nextIsLiteral {
			return nil, false
		}

		head = next
		words = words[1:]
	}

	resolved := &resolvedCall{
		args: make([]string, 0, len(words)),
		head: head,
	}
	for _, word := range words {
		literal, isArgLiteral := wordLiteral(word)
		if !isArgLiteral {
			resolved.hasNonLiteralArg = true
			continue
		}
		resolved.args = append(resolved.args, literal)
	}
	return resolved, true
}

func (c *classifier) visit(node syntax.Node) bool {
	switch typed := node.(type) {
	case *syntax.Stmt:
		if typed.Background || typed.Coprocess {
			c.isReadOnly = false
		}
	case *syntax.CallExpr:
		c.visitCall(typed)
	case *syntax.Redirect:
		if !isSafeRedirect(typed) {
			c.isReadOnly = false
		}
	case *syntax.DeclClause, *syntax.FuncDecl:
		c.isReadOnly = false
	}
	return true
}

func (c *classifier) visitCall(call *syntax.CallExpr) {
	// A non-literal value (LD_PRELOAD=$EVIL) poisons the executed command's environment.
	for _, assign := range call.Assigns {
		if !isLiteralAssign(assign) {
			c.isReadOnly = false
		}
	}

	if len(call.Args) == 0 {
		// Assignment-only statement: a command substitution in the value is walked on its own.
		return
	}

	c.execCalls++
	resolved, ok := c.resolveCall(call)
	if !ok {
		c.hasUnknownConstruct = true
		return
	}

	if !isReadOnlyCall(resolved.args, resolved.hasNonLiteralArg, resolved.head, c.policy) {
		c.isReadOnly = false
	}
}

type pipelineSegment struct {
	call *resolvedCall
	text string
}

type resolvedCall struct {
	args             []string
	hasNonLiteralArg bool
	head             string
}

func Classify(cmd string, policy *toolpolicy.Policy) Verdict {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		if strings.ContainsAny(cmd, ";|&\n") {
			verdict := Verdict{
				Cause:    policy.Causes.UnparseableCompound,
				Decision: decisionDeny,
				Reason:   steerReason(policy, policy.Causes.UnparseableCompound),
			}
			return verdict
		}
		return Verdict{Decision: decisionNone}
	}

	walker := newClassifier(policy)
	syntax.Walk(file, walker.visit)

	isMulti := len(file.Stmts) > 1 || walker.execCalls > 1
	switch {
	case walker.execCalls > 0 && !walker.hasUnknownConstruct && walker.isReadOnly:
		verdict := Verdict{
			Decision: decisionAllow,
			Reason:   policy.Reasons.ReadOnly,
		}
		return verdict
	case isMulti:
		verdict := Verdict{
			Cause:    policy.Causes.Compound,
			Decision: decisionDeny,
			Reason:   steerReason(policy, policy.Causes.Compound),
		}
		return verdict
	default:
		return Verdict{Decision: decisionNone}
	}
}

// isDigits reports whether value is a non-empty run of ASCII digits.
func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// Non-literal positionals (find $DIR) are fine: the flags themselves are always literal.
func isExecCapableFind(cmd string, policy *toolpolicy.Policy) bool {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil || len(file.Stmts) != 1 {
		return false
	}

	call, isCall := file.Stmts[0].Cmd.(*syntax.CallExpr)
	if !isCall || len(call.Args) == 0 {
		return false
	}

	resolved, ok := newClassifier(policy).resolveCall(call)
	if !ok || resolved.head != "find" {
		return false
	}
	return !isReadOnlyFind(resolved.args)
}

// Redirects and literal assignments are ignored: they do not change what make receives.
func singleMakeCall(cmd string, policy *toolpolicy.Policy) (*resolvedCall, bool) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil || len(file.Stmts) != 1 {
		return nil, false
	}

	call, isCall := file.Stmts[0].Cmd.(*syntax.CallExpr)
	if !isCall || len(call.Args) == 0 {
		return nil, false
	}

	resolved, ok := newClassifier(policy).resolveCall(call)
	if !ok || resolved.head != makeHead {
		return nil, false
	}
	return resolved, true
}

// An unparseable command yields none: Classify owns that verdict.
func cdTargets(cmd string, policy *toolpolicy.Policy) []string {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil
	}

	walker := newClassifier(policy)
	targets := make([]string, 0)
	syntax.Walk(file, func(node syntax.Node) bool {
		call, isCall := node.(*syntax.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		resolved, ok := walker.resolveCall(call)
		if !ok || !slices.Contains(cdCommands, resolved.head) {
			return true
		}
		index := slices.IndexFunc(resolved.args, func(arg string) bool {
			return !strings.HasPrefix(arg, "-")
		})
		if index >= 0 {
			targets = append(targets, resolved.args[index])
		}
		return true
	})
	return targets
}

func isLiteralAssign(assign *syntax.Assign) bool {
	if assign.Append || assign.Array != nil || assign.Index != nil {
		return false
	}
	if assign.Value == nil {
		return true
	}

	_, isLiteral := wordLiteral(assign.Value)
	return isLiteral
}

func isSafeRedirect(redirect *syntax.Redirect) bool {
	target := ""
	if redirect.Word != nil {
		literal, isLiteral := wordLiteral(redirect.Word)
		if !isLiteral {
			return false
		}
		target = literal
	}

	switch redirect.Op {
	case syntax.RdrIn, syntax.DplIn:
		return true
	case syntax.DplOut:
		return isDigits(target)
	case syntax.RdrOut, syntax.AppOut:
		return target == "/dev/null"
	case syntax.RdrInOut, syntax.RdrClob, syntax.AppClob, syntax.Hdoc,
		syntax.DashHdoc, syntax.WordHdoc, syntax.RdrAll, syntax.RdrAllClob,
		syntax.AppAll, syntax.AppAllClob:
		return false
	}
	return false
}

func isWriteRedirect(op syntax.RedirOperator, word string) bool {
	switch op {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut, syntax.RdrClob, syntax.AppClob,
		syntax.RdrAll, syntax.RdrAllClob, syntax.AppAll, syntax.AppAllClob:
		return true
	case syntax.DplOut:
		return word != "-" && !isDigits(word)
	case syntax.RdrIn, syntax.DplIn, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
		return false
	}
	return true
}

func pipelineSegments(cmd string, policy *toolpolicy.Policy) ([]*pipelineSegment, bool) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil || len(file.Stmts) != 1 {
		return nil, false
	}

	stmt := file.Stmts[0]
	if stmt.Background || stmt.Coprocess || stmt.Negated || len(stmt.Redirs) > 0 {
		return nil, false
	}

	pipe, isBinary := stmt.Cmd.(*syntax.BinaryCmd)
	if !isBinary || pipe.Op != syntax.Pipe {
		return nil, false
	}

	sides := []*syntax.Stmt{pipe.X, pipe.Y}
	segments := make([]*pipelineSegment, 0, pipelineSegmentCount)
	for _, side := range sides {
		segment, ok := resolveSegment(cmd, side, policy)
		if !ok {
			return nil, false
		}
		segments = append(segments, segment)
	}
	return segments, true
}

func resolveSegment(cmd string, stmt *syntax.Stmt, policy *toolpolicy.Policy) (*pipelineSegment, bool) {
	for _, redirect := range stmt.Redirs {
		if !isSafeRedirect(redirect) {
			return nil, false
		}
	}

	resolved, ok := resolveStmtCall(stmt, policy)
	if !ok {
		return nil, false
	}

	segment := &pipelineSegment{
		call: resolved,
		text: strings.TrimSpace(cmd[stmt.Pos().Offset():stmt.End().Offset()]),
	}
	return segment, true
}

func resolveStmtCall(stmt *syntax.Stmt, policy *toolpolicy.Policy) (*resolvedCall, bool) {
	if stmt.Background || stmt.Coprocess || stmt.Negated {
		return nil, false
	}

	call, isCall := stmt.Cmd.(*syntax.CallExpr)
	if !isCall || len(call.Args) == 0 {
		return nil, false
	}
	for _, assign := range call.Assigns {
		if !isLiteralAssign(assign) {
			return nil, false
		}
	}

	walker := newClassifier(policy)
	resolved, ok := walker.resolveCall(call)
	if !ok || resolved.hasNonLiteralArg {
		return nil, false
	}
	return resolved, true
}

func singleLiteralCall(cmd string, policy *toolpolicy.Policy) (*resolvedCall, bool) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil || len(file.Stmts) != 1 {
		return nil, false
	}

	stmt := file.Stmts[0]
	if len(stmt.Redirs) > 0 {
		return nil, false
	}

	call, isCall := stmt.Cmd.(*syntax.CallExpr)
	if !isCall || len(call.Assigns) > 0 {
		return nil, false
	}
	return resolveStmtCall(stmt, policy)
}

func skipWrapperTokens(wrapper string, words []*syntax.Word) []*syntax.Word {
	remaining := words
	skippedTimeoutDuration := false
	for len(remaining) > 0 {
		literal, isLiteral := wordLiteral(remaining[0])
		if !isLiteral {
			return remaining
		}

		switch {
		case strings.HasPrefix(literal, "-"):
			remaining = remaining[1:]
		case wrapper == envWrapper && strings.Contains(literal, "="):
			remaining = remaining[1:]
		case wrapper == "timeout" && !skippedTimeoutDuration:
			skippedTimeoutDuration = true
			remaining = remaining[1:]
		default:
			return remaining
		}
	}
	return remaining
}

// steerReason is the fixed steering text fed back to the model on a Bash deny.
func steerReason(policy *toolpolicy.Policy, cause string) string {
	return "bash-policy: " + cause + ". " + policy.Steers.Bash
}

func wordLiteral(word *syntax.Word) (string, bool) {
	builder := strings.Builder{}
	for _, part := range word.Parts {
		switch typed := part.(type) {
		case *syntax.Lit:
			builder.WriteString(typed.Value)
		case *syntax.SglQuoted:
			builder.WriteString(typed.Value)
		case *syntax.DblQuoted:
			for _, inner := range typed.Parts {
				innerLit, isLit := inner.(*syntax.Lit)
				if !isLit {
					return "", false
				}
				builder.WriteString(innerLit.Value)
			}
		default:
			return "", false
		}
	}
	return builder.String(), true
}
