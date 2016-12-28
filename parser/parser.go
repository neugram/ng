// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"runtime/debug"
	"strconv"

	"neugram.io/lang/expr"
	"neugram.io/lang/stmt"
	"neugram.io/lang/tipe"
	"neugram.io/lang/token"
)

func New() *Parser {
	p := &Parser{
		s: newScanner(),
	}
	go p.work()
	<-p.s.needSrc
	return p
}

type ParserState int

const (
	StateUnknown ParserState = iota
	StateStmt
	StateStmtPartial
	StateCmd
	StateCmdPartial
)

func (p *Parser) ParseLine(line []byte) Result {
	p.s.addSrc <- append(line, '\n') // TODO: skip the append?
	<-p.s.needSrc
	r := p.res
	p.res = Result{State: r.State}
	return r
}

type Parser struct {
	res Result

	interactive bool
	noCompLit   bool // to resolve composite literal parsing
	s           *Scanner
}

// Result is the result of parsing a line of input.
// One of Stmts or Cmds will be non-nil (but may be empty).
type Result struct {
	State ParserState
	Stmts []stmt.Stmt
	Cmds  []*expr.ShellList
	Errs  []Error
}

func (p *Parser) Close() {
	close(p.s.addSrc)
}

func (p *Parser) work() {
	defer func() {
		// Work is processed on a separate goroutine. Avoid panicing
		// here so there's an oppertunity to clean up terminal state.
		if x := recover(); x != nil {
			err := p.errorf("panic: %v", x)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			debug.PrintStack()
			close(p.s.needSrc)
		}
	}()

	p.s.next()
	for {
		if p.res.State == StateUnknown {
			p.res.State = StateStmt
		}
		p.next()
		if p.s.Token == token.Unknown {
			break
		}

		// We parse top-level $$ expression-statements here.
		//
		// The normal parser can take care of this, but we want to return
		// the parsed output of a top-level $$ before the entire expression
		// is availble, so the REPL can evaluate as we go. That's what
		// makes a simple expression behave like an interactive shell.
		if p.res.State != StateCmd && p.s.Token == token.Shell {
			p.res.State = StateCmd
		} else if p.res.State == StateCmd {
			p.interactive = true
			cmd := p.parseShellList()
			p.interactive = false
			if cmd != nil {
				p.res.Cmds = append(p.res.Cmds, cmd)
			}
			// TODO StateCmdPartial, lines ending with '\'
			if p.s.Token == token.Shell {
				p.next()
				p.expect(token.Semicolon)
				p.res.State = StateUnknown
			}
		} else {
			p.res.State = StateStmtPartial
			p.res.Stmts = append(p.res.Stmts, p.parseStmt())
			p.res.State = StateStmt
		}
	}
}

func ParseStmt(src []byte) (stmt stmt.Stmt, err error) {
	p := New()
	defer p.Close()
	res := p.ParseLine(src)
	if res.State == StateStmtPartial {
		return nil, fmt.Errorf("parser.ParseStmt: partial statement")
	}
	if len(res.Errs) > 0 {
		return nil, Errors(res.Errs)
	}
	if len(res.Stmts) != 1 {
		return nil, fmt.Errorf("parser.ParseStmt: expected 1 statement, got %d", len(res.Stmts))
	}
	return res.Stmts[0], nil
}

func (p *Parser) next() {
	p.s.Next()
	if p.s.Token == token.Comment {
		p.next()
	}
}

/*func (p *Parser) parseShellCmd() *expr.ShellCmd {
	res := &expr.ShellCmd{}
	for p.s.Token == token.ShellWord {
		res.Argv = append(res.Argv, p.s.Literal.(string))
		p.next()
	}
	return res
}

func (p *Parser) parseShellList(interactive bool) *expr.ShellList {
	sl := new(expr.ShellList)

	switch p.s.Token {
	case token.LeftParen:
		p.next()
		sl.List = []expr.Expr{p.parseShellList(false)}
		p.expect(token.RightParen)
		p.next()
	case token.ShellWord:
		sl.List = []expr.Expr{p.parseShellCmd()}
	}

	// TODO: do shells have a useful notion of operator precedence?
	if p.s.Token == token.Shell {
		return sl
	}
	if interactive && p.s.Token == token.Semicolon {
		return sl
	}
	sl.Segment = segOp(p.s.Token)
	p.next()

	switch sl.Segment {
	case expr.SegmentOut, expr.SegmentIn:
		if p.expect(token.ShellWord) {
			sl.Redirect = p.s.Literal.(string)
		}
		p.next()
		sl = &expr.ShellList{List: []expr.Expr{sl}}
	}

	if p.s.Token == token.Shell {
		return sl
	}
	y := p.parseShellList(interactive)

	if y == nil {
		//
	} else if y.Segment == sl.Segment {
		sl.List = append(sl.List, y.List...)
	} else if len(y.List) == 1 && y.Segment == expr.SegmentSemi {
		sl.List = append(sl.List, y.List...)
	} else {
		sl.List = append(sl.List, y)
	}

	return sl
}

func segOp(t token.Token) expr.ShellSeg {
	switch t {
	case token.Semicolon:
		return expr.SegmentSemi
	case token.ShellPipe:
		return expr.SegmentPipe
	case token.LogicalAnd:
		return expr.SegmentAnd
	case token.Ref:
		panic("TODO bg task")
	case token.Greater:
		return expr.SegmentOut
	case token.Less:
		return expr.SegmentIn
	default:
		panic(fmt.Sprintf("unknown segment op: %v", t))
	}
}*/

func (p *Parser) parseExpr() expr.Expr {
	return p.parseBinaryExpr(1)
}

func (p *Parser) parseBinaryExpr(minPrec int) expr.Expr {
	x := p.parseUnaryExpr()
	for prec := p.s.Token.Precedence(); prec >= minPrec; prec-- {
		for {
			op := p.s.Token
			if op.Precedence() != prec {
				break
			}
			p.next()
			y := p.parseBinaryExpr(prec + 1)
			// TODO: distinguish expr from types, when we have types
			// TODO record position
			x = &expr.Binary{
				Op:    op,
				Left:  x,
				Right: y,
			}
		}
	}
	return x
}

func (p *Parser) parseUnaryExpr() expr.Expr {
	switch p.s.Token {
	case token.Add, token.Sub, token.Not, token.Ref:
		op := p.s.Token
		p.next()
		if p.s.err != nil {
			return &expr.Bad{Error: p.s.err}
		}
		x := p.parseUnaryExpr()
		// TODO: distinguish expr from types, when we have types
		return &expr.Unary{Op: op, Expr: x}
	case token.Mul:
		p.next()
		x := p.parseUnaryExpr()
		return &expr.Unary{Op: token.Mul, Expr: x}
	default:
		return p.parsePrimaryExpr()
	}
}

func (p *Parser) expectCommaOr(otherwise token.Token, msg string) bool {
	switch {
	case p.s.Token == token.Comma:
		return true
	case p.s.Token != otherwise:
		p.error("missing ',' in " + msg + " (got " + p.s.Token.String() + ")")
		return true // fake it
	default:
		return false
	}
}

func (p *Parser) parseArgs() []expr.Expr {
	p.expect(token.LeftParen)
	p.next()
	var args []expr.Expr
	for p.s.Token != token.RightParen && p.s.r > 0 {
		// TODO: if this might be a builtin, accept a type name
		args = append(args, p.parseExpr())
		if !p.expectCommaOr(token.RightParen, "arguments") {
			break
		}
		p.next()
	}
	p.expect(token.RightParen)
	p.next()
	return args
}

func (p *Parser) parsePrimaryExpr() expr.Expr {
	x := p.parseOperand()
	for {
		switch p.s.Token {
		case token.Period:
			p.next()
			switch p.s.Token {
			case token.Ident:
				x = &expr.Selector{
					Left:  x,
					Right: p.parseIdent(),
				}
			case token.LeftParen:
				panic("TODO parse type assertion")
			default:
				panic("TODO expect selector type assertion")
			}
		case token.LeftBracket:
			x = p.parseIndex(x)
		case token.LeftParen:
			args := p.parseArgs()
			x = &expr.Call{Func: x, Args: args}
		case token.LeftBrace:
			if tExpr, isType := x.(*expr.Type); isType {
				switch t := tExpr.Type.(type) {
				case *tipe.Slice:
					x = p.parseSliceLiteral(t)
				case *tipe.Table:
					x = p.parseTableLiteral(t)
				case *tipe.Map:
					x = p.parseMapLiteral(t)
				default:
					x = p.parseCompLiteral(t)
				}
			}

			// The problem is that in expressions like
			//	for x := y; x.v++; x = T{}
			// and
			//	if x == T {}
			// are the final braces part of a composite literal
			// T{}, or the body of the block? Resolve this by
			// requiring parens around CompLiteral in loop
			// definition.
			if p.noCompLit {
				return x
			}

			if xpr, isIdent := x.(*expr.Ident); isIdent {
				x = &expr.Type{Type: &tipe.Unresolved{Name: xpr.Name}}
			} else if t := maybePackageType(x); t != nil {
				x = &expr.Type{Type: t}
			} else {
				return x // end of statement
			}
		default:
			return x
		}
	}

	return x
}

func maybePackageType(x expr.Expr) *tipe.Unresolved {
	sel, isSel := x.(*expr.Selector)
	if !isSel {
		return nil
	}
	ident, isIdent := sel.Left.(*expr.Ident)
	if !isIdent {
		return nil
	}
	return &tipe.Unresolved{
		Package: ident.Name,
		Name:    sel.Right.Name,
	}
}

func (p *Parser) parseIndex(lhs expr.Expr) expr.Expr {
	p.expect(token.LeftBracket)
	p.next()

	if p.s.Token == token.Colon {
		// [:expr]
		p.next()
		high := p.parseExpr()
		p.expect(token.RightBracket)
		p.next()
		return &expr.Slice{
			Left: lhs,
			High: high,
		}
	}

	e := p.parseExpr()
	if p.s.Token == token.RightBracket {
		// [expr]
		p.next()
		return &expr.Index{
			Expr:  lhs,
			Index: e,
		}
	}
	p.expect(token.Colon)
	p.next()
	if p.s.Token == token.RightBracket {
		// [expr:]
		p.next()
		return &expr.Slice{
			Left: lhs,
			Low:  e,
		}
	}
	high := p.parseExpr()
	if p.s.Token == token.RightBracket {
		// [expr:high]
		p.next()
		return &expr.Slice{
			Left: lhs,
			Low:  e,
			High: high,
		}
	}
	p.expect(token.Colon)
	p.next()
	max := p.parseExpr()
	p.expect(token.RightBracket)
	p.next()
	// [expr:high:max]
	return &expr.Slice{
		Left: lhs,
		Low:  e,
		High: high,
		Max:  max,
	}
}

/*
func (p *Parser) parseTableIndex(lhs expr.Expr, first *expr.BasicLiteral) *expr.TableIndex {
	x := &expr.TableIndex{
		Expr: lhs,
	}

	// Cols
	if p.s.Token == token.Pipe {
		x.ColNames = append(x.ColNames, first)
	}
	for p.s.Token == token.Pipe {
		p.next()
		// "Col1"|"Col2"
		if p.s.Token != token.String {
			return x
		}
		name, _ := strconv.Unquote(p.s.Literal.(string))
		x.ColNames = append(x.ColNames, name)
		p.next()
	}
	if p.s.Token == token.RightBracket {
		// ["Col1"|"Col2"]
		p.next()
		return x
	}
	if p.s.Token != token.Comma {
		if len(x.ColNames) != 0 {
			p.errorf("expected ',' or ']' after column names, got %s", p.s.Token)
			return x
		}
		// [0:1, [:1, [0:], or [0:,
		x.Cols = p.parseRange()
	}
	if p.s.Token == token.RightBracket {
		p.next()
		return x
	}
	if p.s.Token != token.Comma {
		p.errorf("expected ',' or ']' after column range, got %s", p.s.Token)
		return x
	}
	p.next()

	// Rows
	x.Rows = p.parseRange()

	p.expect(token.RightBracket)
	p.next()
	return x
}
*/

func (p *Parser) parseRange() (r expr.Range) {
	var x expr.Expr
	if p.s.Token != token.Colon {
		// case 0, 0: or 0:1
		x = p.parseExpr()
	}
	if p.s.Token == token.Comma || p.s.Token == token.RightBracket {
		// case 0
		r.Exact = x
		return r
	}
	r.Start = x
	p.expect(token.Colon)
	p.next()
	if p.s.Token == token.Comma || p.s.Token == token.RightBracket {
		// 0: or :
		return r
	}
	// 0:1 or :1
	r.End = p.parseExpr()
	return r
}

func (p *Parser) parseIn() (names []string, params *tipe.Tuple) {
	params = &tipe.Tuple{}
	for p.s.Token > 0 && p.s.Token != token.RightParen {
		n := p.parseIdent().Name
		t := p.maybeParseType()
		if t != nil {
			for i := len(params.Elems) - 1; i >= 0 && params.Elems[i] == nil; i-- {
				params.Elems[i] = t
			}
		}
		if p.s.Token == token.Comma {
			p.next()
		}
		names = append(names, n)
		params.Elems = append(params.Elems, t)
	}
	return names, params
}

func (p *Parser) parseOut() (names []string, params *tipe.Tuple) {
	typeToName := func(t tipe.Type) string {
		if t == nil {
			panic("nil type")
		}
		switch t := t.(type) {
		case tipe.Basic:
			return string(t)
		case *tipe.Unresolved:
			if t.Package != "" {
				p.errorf("invalid return value name %s.%s", t.Package, t.Name)
			}
			return t.Name
		default:
			p.errorf("expected return value name, got %T", t)
			return "BAD:" + t.Sexp() // TODO something better
		}
	}

	// Either none of the output parameters have names, or all do.
	var types []tipe.Type
	var named bool
	for p.s.Token > 0 && p.s.Token != token.RightParen {
		t := p.maybeParseType()
		if t == nil {
			p.errorf("expected return value name or type, got %s", p.s.Token)
			p.next() // make progress
			continue
		}
		if p.s.Token > 0 && p.s.Token != token.RightParen && p.s.Token != token.Comma {
			// Type was actually a name.
			names = append(names, typeToName(t))
			types = append(types, p.maybeParseType())
			named = true
		} else {
			// Single element parameter, assume a type for now.
			names = append(names, "")
			types = append(types, t)
		}

		if p.s.Token == token.Comma {
			p.next()
		}
	}

	if named {
		// (a, b T1, b T2)
		// All dangling types are really names.
		for i, name := range names {
			if name == "" {
				names[i] = typeToName(types[i])
				types[i] = nil
			}
		}
	} else {
		// (T1, T2)
	}
	params = &tipe.Tuple{Elems: types}
	return names, params
}

func (p *Parser) parseMethodik(name string) stmt.Stmt {
	c := &stmt.MethodikDecl{
		Name: name,
		Type: &tipe.Methodik{
			// TODO Spec
			Type: p.parseType(),
		},
	}
	p.expect(token.LeftBrace)
	p.next()

	tags := make(map[string]bool)
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		p.expect(token.Func)
		m := p.parseFunc(true)
		if tags[m.Name] {
			p.errorf("func %s redeclared in methodik %s", m.Name, c.Name)
		} else {
			tags[m.Name] = true
			c.Type.MethodNames = append(c.Type.MethodNames, m.Name)
			c.Type.Methods = append(c.Type.Methods, m.Type)
			c.Methods = append(c.Methods, m)
		}
		if p.s.Token == token.Semicolon {
			p.next()
		}
	}
	p.expect(token.RightBrace)
	p.next()

	return c
}

func (p *Parser) parseType() tipe.Type {
	t := p.maybeParseType()
	if t == nil {
		p.errorf("expected type , got %s", p.s.Token)
	}
	return t
}

func (p *Parser) maybeParseType() tipe.Type {
	switch p.s.Token {
	case token.Ident:
		ident := p.parseIdent()
		if p.s.Token == token.Period {
			p.next()
			sel := p.parseIdent()
			return &tipe.Unresolved{
				Package: ident.Name,
				Name:    sel.Name,
			}
		}
		// It is an error to declare a variable with the name of a
		// type parameter, so we can resolve it immediately.
		if ident.Name == "num" {
			return tipe.Num
		}
		return &tipe.Unresolved{Name: ident.Name}
	case token.LeftBracket:
		p.next()
		table := false
		if p.s.Token == token.Pipe {
			table = true
			p.next()
		}
		p.expect(token.RightBracket)
		p.next()
		if table {
			return &tipe.Table{Type: p.parseType()}
		} else {
			return &tipe.Slice{Elem: p.parseType()}
		}
	case token.Mul:
		p.next()
		return &tipe.Pointer{Elem: p.parseType()}
	case token.Struct:
		p.next()
		p.expect(token.LeftBrace)
		p.next()
		s := &tipe.Struct{}
		tags := make(map[string]bool)
		for p.s.Token > 0 && p.s.Token != token.RightBrace {
			n := p.parseIdent().Name
			t := p.parseType()
			if tags[n] {
				p.errorf("field %s redeclared in struct %s", n, s)
			} else {
				tags[n] = true
				s.FieldNames = append(s.FieldNames, n)
				s.Fields = append(s.Fields, t)
			}
			if p.s.Token == token.Comma || p.s.Token == token.Semicolon {
				p.next()
			} else if p.s.Token != token.RightBrace {
				p.expect(token.Comma) // produce error
			}
		}
		p.expect(token.RightBrace)
		p.next()
		return s
	case token.Func:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.Map:
		// map[T]U
		s := &tipe.Map{}
		p.next()
		p.expect(token.LeftBracket)
		p.next()
		s.Key = p.parseType()
		p.expect(token.RightBracket)
		p.next()
		s.Value = p.parseType()
		return s
	default:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	}
	return nil
}

func (p *Parser) parseExprs() []expr.Expr {
	exprs := []expr.Expr{p.parseExpr()}
	for p.s.Token == token.Comma {
		p.next()
		exprs = append(exprs, p.parseExpr())
	}
	return exprs
}

func arithAssignOp(t token.Token) token.Token {
	switch t {
	case token.AddAssign:
		return token.Add
	case token.SubAssign:
		return token.Sub
	case token.MulAssign:
		return token.Mul
	case token.DivAssign:
		return token.Div
	case token.RemAssign:
		return token.Rem
	case token.PowAssign:
		return token.Pow
	default:
		return token.Unknown
	}
}

func (p *Parser) parseSimpleStmt() stmt.Stmt {
	exprs := p.parseExprs()

	switch p.s.Token {
	case token.Define, token.Assign, token.AddAssign, token.SubAssign,
		token.MulAssign, token.DivAssign, token.RemAssign, token.PowAssign:
		tok := p.s.Token

		p.next()
		var right []expr.Expr
		if p.s.Token == token.Range {
			p.next()
			if tok != token.Define && tok != token.Assign {
				right = []expr.Expr{&expr.Bad{p.error("range can only be used inside ':=' or '='")}}
			} else {
				right = []expr.Expr{&expr.Unary{
					Op:   token.Range,
					Expr: p.parseExpr(),
				}}
			}
		} else {
			right = p.parseExprs()
		}
		if tok == token.Define {
			for i, e := range exprs {
				if _, ok := e.(*expr.Ident); !ok {
					exprs[i] = &expr.Bad{p.error("expected identifier as declaration")}
				}
			}
		}
		if arithOp := arithAssignOp(tok); arithOp != token.Unknown {
			if len(exprs) != 1 || len(right) != 1 {
				right = []expr.Expr{&expr.Bad{p.error(fmt.Sprintf("arithmetic assignement %q only accepts one argument", tok))}}
			} else {
				right[0] = &expr.Binary{
					Op:    arithOp,
					Left:  exprs[0],
					Right: right[0],
				}
			}
		}
		return &stmt.Assign{
			Decl:  tok == token.Define,
			Left:  exprs,
			Right: right,
		}
	case token.Inc, token.Dec:
		// TODO: do we want to introduce a specialized statement for this?
		op := token.Add
		if p.s.Token == token.Dec {
			op = token.Sub
		}
		p.next()
		return &stmt.Assign{
			Left: []expr.Expr{exprs[0]},
			Right: []expr.Expr{&expr.Binary{
				Op:    op,
				Left:  exprs[0],
				Right: &expr.BasicLiteral{big.NewInt(1)},
			}},
		}
	}

	// TODO len==1
	return &stmt.Simple{exprs[0]}
	//panic(fmt.Sprintf("TODO parseSimpleStmt, Token=%s", p.s.Token))
}

func (p *Parser) extractExpr(s stmt.Stmt) expr.Expr {
	if e, isExpr := s.(*stmt.Simple); isExpr {
		return e.Expr
	}
	fmt.Printf("expected boolean expression, found statement: %s", s.Sexp())
	return &expr.Bad{p.error("expected boolean expression, found statement")}
}

func extractRange(s stmt.Stmt) (res *stmt.Range) {
	//defer fmt.Printf("extractRange(%s) res=%s\n", s.Sexp(), res)
	a, ok := s.(*stmt.Assign)
	if !ok || len(a.Right) != 1 {
		return nil
	}
	r, ok := a.Right[0].(*expr.Unary)
	if !ok || r.Op != token.Range {
		return nil
	}
	var key, val expr.Expr
	if len(a.Left) > 2 {
		return nil
	}
	key = a.Left[0]
	if len(a.Left) == 2 {
		val = a.Left[1]
	}
	return &stmt.Range{Decl: a.Decl, Key: key, Val: val, Expr: r.Expr}
}

func (p *Parser) parseStmt() stmt.Stmt {
	switch p.s.Token {
	// TODO: many many kinds of statements
	case token.If:
		s := &stmt.If{}
		p.next()
		p.noCompLit = true
		if p.s.Token == token.Semicolon {
			// Blank Init statement.
			p.next()
			s.Cond = p.parseExpr()
		} else {
			s.Init = p.parseSimpleStmt()
			if p.s.Token == token.Semicolon {
				p.next()
				s.Cond = p.parseExpr()
			} else {
				// No Init statement, make it the condition
				s.Cond = p.extractExpr(s.Init)
				s.Init = nil
			}
		}
		p.noCompLit = false
		s.Body = p.parseBlock()
		if p.s.Token == token.Else {
			p.next()
			s.Else = p.parseStmt()
		} else {
			p.expectSemi()
		}
		return s
	case token.Ident, token.Int, token.Float, token.Add, token.Sub, token.Mul, token.Map,
		token.Func, token.LeftBracket, token.LeftParen, token.String, token.Rune, token.Shell:
		// A "simple" statement, no control flow.
		s := p.parseSimpleStmt()
		p.expectSemi()
		return s
	case token.Return:
		p.next()
		s := &stmt.Return{Exprs: p.parseExprs()}
		p.expectSemi()
		return s
	case token.LeftBrace:
		s := p.parseBlock()
		p.expectSemi()
		return s
	case token.For:
		s := p.parseFor()
		p.expectSemi()
		return s
	case token.Const:
		p.next()
		s := &stmt.Const{
			Name: p.parseIdent().Name,
		}
		if p.s.Token != token.Assign {
			s.Type = p.parseType()
		}
		p.expect(token.Assign)
		p.next()
		s.Value = p.parseExpr()
		p.expectSemi()
		return s
	case token.Methodik:
		p.next()
		m := p.parseMethodik(p.parseIdent().Name)
		p.expectSemi()
		return m
	case token.Type:
		p.next()
		s := &stmt.TypeDecl{
			Name: p.parseIdent().Name,
			Type: p.parseType(),
		}
		p.expectSemi()
		return s
	case token.Importgo:
		p.next()
		name := ""
		if p.s.Token == token.Ident {
			name = p.s.Literal.(string)
			name = name[1 : len(name)-1]
			p.next()
		}
		p.expect(token.String)
		path := p.s.Literal.(string)
		s := &stmt.Import{
			Name:   name,
			Path:   path[1 : len(path)-1],
			FromGo: true,
		}
		p.next()
		p.expectSemi()
		return s
	}
	panic(fmt.Sprintf("TODO parseStmt %s", p.s.Token))
}

func (p *Parser) parseFor() stmt.Stmt {
	p.expect(token.For)
	p.next()

	p.noCompLit = true
	body := func() stmt.Stmt {
		p.noCompLit = false
		b := p.parseBlock()
		p.expectSemi()
		return b
	}

	if p.s.Token == token.LeftBrace {
		// for {}
		return &stmt.For{Body: body()}
	}
	if p.s.Token == token.Range {
		// for range r { }
		p.next()
		return &stmt.Range{Expr: p.parseExpr(), Body: body()}
	}
	if p.s.Token == token.Semicolon {
		p.next()
		if p.s.Token == token.Semicolon {
			p.next()
			if p.s.Token == token.LeftBrace {
				// for ;; { }
				return &stmt.For{Body: body()}
			}
			// for ;;i2 { }
			i2 := p.parseSimpleStmt()
			return &stmt.For{Post: i2, Body: body()}
		}
		i1 := p.parseSimpleStmt()
		if p.s.Token == token.Semicolon {
			// for ;i1; { }
			p.next()
			return &stmt.For{Cond: p.extractExpr(i1), Body: body()}
		}
		// for ;i1;i2 { }
		panic("TODO parseFor 'for ;'") // TODO
	} else {
		i0 := p.parseSimpleStmt()
		if p.s.Token == token.LeftBrace {
			if r := extractRange(i0); r != nil {
				// for k := range r { }
				// for k, _ := range r { }
				r.Body = body()
				return r
			} else {
				// for i0 {}
				return &stmt.For{Cond: p.extractExpr(i0), Body: body()}
			}
		}
		p.expectSemi()
		p.next()
		if p.s.Token == token.Semicolon {
			// for i0;; {}
			return &stmt.For{Init: i0, Body: body()}
		}
		i1 := p.parseSimpleStmt()
		p.expectSemi()
		p.next()
		if p.s.Token == token.LeftBrace {
			// for i0;i1; { }
			return &stmt.For{Init: i0, Cond: p.extractExpr(i1), Body: body()}
		}
		i2 := p.parseSimpleStmt()
		p.expect(token.LeftBrace)
		// for i0; i1; i2 { }
		return &stmt.For{
			Init: i0,
			Cond: p.extractExpr(i1),
			Post: i2,
			Body: body(),
		}
	}

	// TODO
	panic("TODO parseFor range")
}

func (p *Parser) parseBlock() stmt.Stmt {
	p.expect(token.LeftBrace)
	p.next()
	s := &stmt.Block{Stmts: p.parseStmts()}
	p.expect(token.RightBrace)
	p.next()
	return s
}

func (p *Parser) parseStmts() (stmts []stmt.Stmt) {
	// TODO there are other kinds of blocks to exit from
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		stmts = append(stmts, p.parseStmt())
		if p.s.Token == token.Semicolon {
			p.next()
		}
	}
	return stmts
}

// parseFuncType just parses the top of the func (the part woven
// into the type declaration), not the body.
func (p *Parser) parseFuncType(method bool) *expr.FuncLiteral {
	p.expect(token.Func)
	p.next()

	f := &expr.FuncLiteral{
		Type: &tipe.Func{},
	}

	if method {
		// func (a) f()
		f.PointerReceiver = true // TODO remove?
		p.expect(token.LeftParen)
		p.next()
		f.ReceiverName = p.parseIdent().Name
		p.expect(token.RightParen)
		p.next()
	}

	if p.s.Token == token.Ident {
		f.Name = p.parseIdent().Name
	} else if method {
		p.errorf("class method missing name")
	}

	p.expect(token.LeftParen)
	p.next()
	if p.s.Token != token.RightParen {
		f.ParamNames, f.Type.Params = p.parseIn()
	} else {
		f.Type.Params = new(tipe.Tuple)
	}
	p.expect(token.RightParen)
	p.next()

	if p.s.Token == token.LeftParen {
		p.expect(token.LeftParen)
		p.next()
		if p.s.Token != token.RightParen {
			f.ResultNames, f.Type.Results = p.parseOut()
		}
		p.expect(token.RightParen)
		p.next()
	} else {
		typ := p.maybeParseType()
		if typ != nil {
			f.ResultNames = []string{""}
			f.Type.Results = &tipe.Tuple{Elems: []tipe.Type{typ}}
		}
	}
	return f
}

func (p *Parser) parseFunc(method bool) *expr.FuncLiteral {
	p.expect(token.Func)
	f := p.parseFuncType(method)
	if p.s.Token != token.LeftBrace {
		p.next()
		p.errorf("missing function body")
		return f
	}
	f.Body = p.parseBlock()
	return f
}

func (p *Parser) parseOperand() expr.Expr {
	switch p.s.Token {
	case token.Ident:
		x := p.parseIdent()
		return x
	case token.Int, token.Float, token.Imaginary:
		x := &expr.BasicLiteral{Value: p.s.Literal}
		p.next()
		return x
	case token.Rune:
		x := &expr.BasicLiteral{Value: p.s.Literal}
		p.next()
		return x
	case token.String:
		s, _ := strconv.Unquote(p.s.Literal.(string))
		x := &expr.BasicLiteral{Value: s}
		p.next()
		return x
	case token.LeftParen:
		origNoCompLit := p.noCompLit
		p.noCompLit = false
		p.next()
		ex := p.parseExpr() // TODO or a type?
		p.expect(token.RightParen)
		p.next()
		p.noCompLit = origNoCompLit
		return &expr.Unary{Op: token.LeftParen, Expr: ex}
	case token.Func:
		return p.parseFunc(false)
	case token.Shell:
		p.next()
		x := &expr.Shell{}
		for p.s.Token > 0 && p.s.Token != token.Shell {
			restore := p.interactive
			p.interactive = false
			cmd := p.parseShellList()
			p.interactive = restore
			x.Cmds = append(x.Cmds, cmd)
		}
		p.expect(token.Shell)
		p.next()
		return x
	}

	if t := p.maybeParseType(); t != nil {
		return &expr.Type{Type: t}
	}

	p.next()
	return &expr.Bad{p.errorf("expected operand, got %s", p.s.Token)}
}

func (p *Parser) parseSliceLiteral(t tipe.Type) *expr.SliceLiteral {
	x := &expr.SliceLiteral{Type: t.(*tipe.Slice)}
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		e := p.parseExpr()
		x.Elems = append(x.Elems, e)
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.expect(token.RightBrace)
	p.next()
	return x
}

func (p *Parser) parseTableLiteral(t tipe.Type) *expr.TableLiteral {
	x := &expr.TableLiteral{Type: t.(*tipe.Table)}
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		p.expect(token.LeftBrace)
		p.next()
		if p.s.Token == token.Pipe {
			// column names: {|"x","y"|},
			if len(x.ColNames) != 0 || len(x.Rows) != 0 {
				p.errorf("column names can only appear at beginning of table literal")
			}
			p.next()
			for p.s.Token > 0 && p.s.Token != token.Pipe {
				x.ColNames = append(x.ColNames, p.parseExpr())
				if p.s.Token != token.Comma {
					break
				}
				p.next()
			}
			p.expect(token.Pipe)
			p.next()
		} else {
			var row []expr.Expr
			for p.s.Token > 0 && p.s.Token != token.RightBrace {
				row = append(row, p.parseExpr())
				if p.s.Token != token.Comma {
					break
				}
				p.next()
			}
			x.Rows = append(x.Rows, row)
		}
		p.expect(token.RightBrace)
		p.next()
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.next()
	return x
}

func (p *Parser) parseMapLiteral(t tipe.Type) *expr.MapLiteral {
	x := &expr.MapLiteral{Type: t}
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		k := p.parseExpr()
		p.expect(token.Colon)
		p.next()
		v := p.parseExpr()
		x.Keys = append(x.Keys, k)
		x.Values = append(x.Values, v)
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.expect(token.RightBrace)
	p.next()
	return x
}

func (p *Parser) parseCompLiteral(t tipe.Type) *expr.CompLiteral {
	x := &expr.CompLiteral{Type: t}
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		e := p.parseExpr()
		if p.s.Token == token.Colon {
			p.next()
			v := p.parseExpr()

			if len(x.Elements) > 0 && len(x.Keys) == 0 {
				p.errorf("mixture of keyed fields and value initializers")
				continue
			}

			x.Keys = append(x.Keys, e)
			x.Elements = append(x.Elements, v)
		} else {
			if len(x.Elements) > 0 && len(x.Keys) > 0 {
				p.errorf("mixture of keyed fields and value initializers")
				continue
			}
			x.Elements = append(x.Elements, e)
		}
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.expect(token.RightBrace)
	p.next()
	return x
}

type Errors []Error

func (e Errors) Error() string {
	buf := new(bytes.Buffer)
	buf.WriteString("neugram: parser erorrs:\n")
	for _, err := range e {
		fmt.Fprintf(buf, "off %5d: %v\n", err.Offset, err.Msg)
	}
	return buf.String()
}

type Error struct {
	Offset int
	Msg    string
}

func (e Error) Error() string {
	return fmt.Sprintf("neugram: parser: %s (off %d)", e.Msg, e.Offset)
}

func (p *Parser) errorf(format string, a ...interface{}) error {
	return p.error(fmt.Sprintf(format, a...))
}

func (p *Parser) error(msg string) error {
	err := Error{
		Offset: p.s.Offset,
		Msg:    msg,
	}
	p.res.Errs = append(p.res.Errs, err)
	return err
}

func (p *Parser) expect(t token.Token) bool {
	met := t == p.s.Token
	if !met {
		p.error(fmt.Sprintf("expected %q, found %q", t, p.s.Token))
	}
	return met
}

func (p *Parser) expectSemi() {
	if p.s.Token == token.RightBrace {
		return
	}
	p.expect(token.Semicolon)
}

func (p *Parser) parseIdent() *expr.Ident {
	name := "_"
	if p.expect(token.Ident) {
		name = p.s.Literal.(string)
	}
	p.next()
	return &expr.Ident{Name: name}
}
