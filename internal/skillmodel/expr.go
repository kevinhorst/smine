package skillmodel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExprKind is a closed set dispatched by switch, never an interface.
type ExprKind int

const (
	ExprAtom ExprKind = iota
	ExprNot
	ExprAnd
	ExprOr
	ExprImplies
	ExprCmp
)

// CmpOp enumerates the comparison operators of an ExprCmp node.
type CmpOp int

const (
	CmpEq CmpOp = iota
	CmpNe
	CmpLt
	CmpLe
	CmpGt
	CmpGe
)

type Cmp struct {
	Left  *Term
	Op    CmpOp
	Right *Term
}

// TermKind enumerates the term node kinds of a comparison side.
type TermKind int

const (
	TermIdent TermKind = iota
	TermLit
	TermAdd
	TermSub
)

// Term's identifier may be an atom or an enum member.
type Term struct {
	Ident string
	Kind  TermKind
	Left  *Term
	Lit   int
	Right *Term
}

// Idents returns every identifier in the term, in encounter order.
func (t *Term) Idents() []string {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case TermIdent:
		return []string{t.Ident}
	case TermLit:
		return nil
	case TermAdd, TermSub:
		return append(t.Left.Idents(), t.Right.Idents()...)
	}
	return nil
}

// Expr's unary nodes carry their operand in Left.
type Expr struct {
	Atom  string
	Cmp   *Cmp
	Kind  ExprKind
	Left  *Expr
	Right *Expr
}

// exprParser precedence, loosest first: => (right-assoc), or, and, not, atom/parens.
type exprParser struct {
	position int
	tokens   []exprToken
}

// exprToken's position is the rune offset for error messages.
type exprToken struct {
	position int
	text     string
}

func ParseExpr(input string) (*Expr, error) {
	tokens, err := lexExpr(input)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("ParseExpr: empty expression")
	}
	parser := &exprParser{tokens: tokens}
	expr, err := parser.parseImplies()
	if err != nil {
		return nil, err
	}
	if parser.position < len(parser.tokens) {
		token := parser.tokens[parser.position]
		return nil, fmt.Errorf("ParseExpr: unexpected %q at %d", token.text, token.position)
	}
	return expr, nil
}

// Atoms keeps duplicates and enum members; resolution checks dedup and classify themselves.
func (e *Expr) Atoms() []string {
	if e == nil {
		return nil
	}
	switch e.Kind {
	case ExprAtom:
		return []string{e.Atom}
	case ExprNot:
		return e.Left.Atoms()
	case ExprAnd, ExprOr, ExprImplies:
		return append(e.Left.Atoms(), e.Right.Atoms()...)
	case ExprCmp:
		return append(e.Cmp.Left.Idents(), e.Cmp.Right.Idents()...)
	}
	return nil
}

func (e *Expr) comparisons() []*Cmp {
	if e == nil {
		return nil
	}
	switch e.Kind {
	case ExprCmp:
		return []*Cmp{e.Cmp}
	case ExprAtom:
		return nil
	case ExprNot:
		return e.Left.comparisons()
	case ExprAnd, ExprOr, ExprImplies:
		return append(e.Left.comparisons(), e.Right.comparisons()...)
	}
	return nil
}

func (p *exprParser) parseImplies() (*Expr, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.accept("=>") {
		return left, nil
	}
	right, err := p.parseImplies()
	if err != nil {
		return nil, err
	}
	implies := &Expr{Kind: ExprImplies, Left: left, Right: right}
	return implies, nil
}

func (p *exprParser) parseOr() (*Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.accept("or") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Expr{Kind: ExprOr, Left: left, Right: right}
	}
	return left, nil
}

func (p *exprParser) parseAnd() (*Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.accept("and") {
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Expr{Kind: ExprAnd, Left: left, Right: right}
	}
	return left, nil
}

func (p *exprParser) parseUnary() (*Expr, error) {
	if p.accept("not") {
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		negation := &Expr{Kind: ExprNot, Left: operand}
		return negation, nil
	}
	if p.accept("(") {
		inner, err := p.parseImplies()
		if err != nil {
			return nil, err
		}
		if !p.accept(")") {
			return nil, fmt.Errorf("ParseExpr: missing closing parenthesis at %d", p.errorPosition())
		}
		return inner, nil
	}
	return p.parseOperand()
}

// cmpOps maps comparison tokens to their operator.
var cmpOps = map[string]CmpOp{
	"=":  CmpEq,
	"!=": CmpNe,
	"<":  CmpLt,
	"<=": CmpLe,
	">":  CmpGt,
	">=": CmpGe,
}

// parseOperand requires a term without comparison to be a single identifier, a boolean atom.
func (p *exprParser) parseOperand() (*Expr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for token, operator := range cmpOps {
		if !p.accept(token) {
			continue
		}
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		comparison := &Expr{Cmp: &Cmp{Left: left, Op: operator, Right: right}, Kind: ExprCmp}
		return comparison, nil
	}
	if left.Kind != TermIdent {
		return nil, fmt.Errorf("ParseExpr: arithmetic term without a comparison at %d", p.errorPosition())
	}
	atom := &Expr{Atom: left.Ident, Kind: ExprAtom}
	return atom, nil
}

// parseTerm parses a flat additive chain: `primary (("+"|"-") primary)*`.
func (p *exprParser) parseTerm() (*Term, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		var kind TermKind
		switch {
		case p.accept("+"):
			kind = TermAdd
		case p.accept("-"):
			kind = TermSub
		default:
			return left, nil
		}
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		left = &Term{Kind: kind, Left: left, Right: right}
	}
}

// parsePrimary parses one identifier or integer literal.
func (p *exprParser) parsePrimary() (*Term, error) {
	token, ok := p.next()
	if !ok {
		return nil, errors.New("ParseExpr: unexpected end of expression")
	}
	if isExprLiteral(token.text) {
		value, err := strconv.Atoi(token.text)
		if err != nil {
			return nil, fmt.Errorf("ParseExpr: bad literal %q at %d", token.text, token.position)
		}
		literal := &Term{Kind: TermLit, Lit: value}
		return literal, nil
	}
	if !isExprIdent(token.text) {
		return nil, fmt.Errorf("ParseExpr: unexpected %q at %d", token.text, token.position)
	}
	ident := &Term{Ident: token.text, Kind: TermIdent}
	return ident, nil
}

// accept consumes the next token when it equals text.
func (p *exprParser) accept(text string) bool {
	if p.position < len(p.tokens) && p.tokens[p.position].text == text {
		p.position++
		return true
	}
	return false
}

func (p *exprParser) errorPosition() int {
	if p.position < len(p.tokens) {
		return p.tokens[p.position].position
	}
	if len(p.tokens) > 0 {
		return p.tokens[len(p.tokens)-1].position
	}
	return 0
}

func (p *exprParser) next() (exprToken, bool) {
	if p.position >= len(p.tokens) {
		return exprToken{}, false
	}
	token := p.tokens[p.position]
	p.position++
	return token, true
}

// isExprIdent admits lower_snake atoms and members and UPPER_SNAKE enum members; mixed case is a typo.
func isExprIdent(text string) bool {
	if text == "" || text == "and" || text == "or" || text == "not" {
		return false
	}

	isUpper := unicode.IsUpper([]rune(text)[0])
	for index, character := range text {
		isLetter := unicode.IsLower(character)
		if isUpper {
			isLetter = unicode.IsUpper(character)
		}
		if index == 0 && !isLetter {
			return false
		}
		if !isLetter && !unicode.IsDigit(character) && character != '_' {
			return false
		}
	}
	return true
}

// isExprLiteral reports whether the token is an integer literal.
func isExprLiteral(text string) bool {
	if text == "" {
		return false
	}
	for _, character := range text {
		if !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

// lexExpr tokenizes the DSL: identifiers, `=>`, and single parentheses.
func lexExpr(input string) ([]exprToken, error) {
	var tokens []exprToken
	runes := []rune(input)
	for index := 0; index < len(runes); {
		character := runes[index]
		switch {
		case unicode.IsSpace(character):
			index++
		case character == '(' || character == ')' || character == '+' || character == '-':
			tokens = append(tokens, exprToken{position: index, text: string(character)})
			index++
		case character == '=':
			if index+1 < len(runes) && runes[index+1] == '>' {
				tokens = append(tokens, exprToken{position: index, text: "=>"})
				index += 2
				continue
			}
			tokens = append(tokens, exprToken{position: index, text: "="})
			index++
		case character == '!':
			if index+1 >= len(runes) || runes[index+1] != '=' {
				return nil, fmt.Errorf("lexExpr: unexpected %q at %d", string(character), index)
			}
			tokens = append(tokens, exprToken{position: index, text: "!="})
			index += 2
		case character == '<' || character == '>':
			if index+1 < len(runes) && runes[index+1] == '=' {
				tokens = append(tokens, exprToken{position: index, text: string(character) + "="})
				index += 2
				continue
			}
			tokens = append(tokens, exprToken{position: index, text: string(character)})
			index++
		case unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_':
			start := index
			for index < len(runes) && (unicode.IsLetter(runes[index]) || unicode.IsDigit(runes[index]) || runes[index] == '_') {
				index++
			}
			tokens = append(tokens, exprToken{position: start, text: string(runes[start:index])})
		default:
			return nil, fmt.Errorf("lexExpr: unexpected %q at %d", string(character), index)
		}
	}
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	return tokens, nil
}
