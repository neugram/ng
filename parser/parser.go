// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"math/big"
	"os"
	"runtime/debug"
	"strconv"

	"neugram.io/ng/format"
	"neugram.io/ng/syntax"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/src"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
)

func New(filename string) *Parser {
	p := &Parser{
		filename: filename,
		s:        newScanner(),
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

// Parse parses a neugram source file.
// If non-nil, the returned error is either of type Error or Errors.
func (p *Parser) Parse(source []byte) (*syntax.File, error) {
	f := &syntax.File{Filename: p.filename}
	var errs Errors
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for i := 0; scanner.Scan(); i++ {
		b := scanner.Bytes()
		if i == 0 && len(b) > 2 && b[0] == '#' && b[1] == '!' { // shebang
			p.s.Line++
			continue
		}
		res := p.ParseLine(b)
		if len(res.Stmts) > 0 {
			f.Stmts = append(f.Stmts, res.Stmts...)
		}
		if len(res.Cmds) == 1 {
			s := &stmt.Simple{
				Position: res.Cmds[0].Pos(),
				Expr: &expr.Shell{
					Position: res.Cmds[0].Pos(),
					Cmds:     res.Cmds,
				},
			}
			f.Stmts = append(f.Stmts, s)
		} else if len(res.Cmds) > 1 {
			s := &stmt.Block{
				Position: res.Cmds[0].Position,
			}
			for _, cmd := range res.Cmds {
				simple := &stmt.Simple{
					Position: cmd.Pos(),
					Expr: &expr.Shell{
						Position: cmd.Pos(),
						Cmds:     []*expr.ShellList{cmd},
					},
				}
				s.Stmts = append(s.Stmts, simple)
			}
			f.Stmts = append(f.Stmts, s)
		}
		if len(res.Errs) > 0 {
			errs = append(errs, res.Errs...)
		}
		if len(errs) > 10 {
			// Too many errors. Call it quits.
			return f, errs
		}
	}
	if err := scanner.Err(); err != nil {
		panic("parser.Parse: impossible scanner error: " + err.Error())
	}

	if len(errs) == 1 {
		return f, errs[0]
	}
	if len(errs) > 1 {
		return f, errs
	}
	return f, nil
}

func (p *Parser) ParseLine(line []byte) Result {
	p.s.addSrc <- append(line, '\n') // TODO: skip the append?
	<-p.s.needSrc
	r := p.res
	p.res = Result{State: r.State}
	return r
}

type Parser struct {
	filename string

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
			if p.s.r == -1 {
				break // no more work
			}
			p.errorf("unknown token: '%s'", p.s.Literal)
			p.s.drain()
			continue
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
	p := New("<single statement>")
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

func (p *Parser) pos() src.Pos {
	col := p.s.Column // positioned on the last byte of the token
	if p.s.Literal != nil {
		if s, isString := p.s.Literal.(string); isString {
			col -= int16(len(s) - 1)
		}
	} else {
		str := p.s.Token.String()
		col -= int16(len(str) - 1)
	}

	return src.Pos{
		Filename: p.filename,
		Line:     p.s.Line,
		Column:   col,
	}
}

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
			pos := p.pos()
			p.next()
			y := p.parseBinaryExpr(prec + 1)
			// TODO: distinguish expr from types, when we have types
			// TODO record position
			binOp := &expr.Binary{
				Position: pos,
				Op:       op,
				Left:     x,
				Right:    y,
			}
			x = binOp
		}
	}
	return x
}

func (p *Parser) parseUnaryExpr() expr.Expr {
	pos := p.pos()
	switch p.s.Token {
	case token.Add, token.Sub, token.Not, token.Ref:
		op := p.s.Token
		p.next()
		if p.s.err != nil {
			bad := &expr.Bad{
				Position: pos,
				Error:    p.s.err,
			}
			return bad
		}
		x := p.parseUnaryExpr()
		// TODO: distinguish expr from types, when we have types
		unary := &expr.Unary{Position: pos, Op: op, Expr: x}
		return unary
	case token.Mul:
		p.next()
		x := p.parseUnaryExpr()
		unary := &expr.Unary{Position: pos, Op: token.Mul, Expr: x}
		return unary
	case token.ChanOp:
		// channel type or receive expression
		p.next()
		x := p.parseUnaryExpr()

		if extyp, ok := x.(*expr.Type); ok {
			// parsed a channel type, add in the receive prefix '<-'
			if t, ok := extyp.Type.(*tipe.Chan); ok {
				if t.Direction == tipe.ChanRecv {
					p.error(`expected "chan", found "<-"`)
				}
				// TODO: nested channel types
				t.Direction = tipe.ChanRecv
			} else {
				p.errorf(`expected "chan", found %q`, format.Type(t))
			}
			return x
		}
		// parsed a receive expression
		unary := &expr.Unary{Position: pos, Op: token.ChanOp, Expr: x}
		return unary
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
		pos := p.pos()
		switch p.s.Token {
		case token.Period:
			p.next()
			switch p.s.Token {
			case token.Ident:
				selector := &expr.Selector{
					Position: pos,
					Left:     x,
					Right:    p.parseIdent(),
				}
				x = selector
			case token.LeftParen:
				x = p.parseTypeAssert(x)
			default:
				panic("TODO expect selector type assertion")
			}
		case token.LeftBracket:
			x = p.parseIndex(x)
		case token.LeftParen:
			p.next()
			var args []expr.Expr
			var ellipsis bool
			for p.s.Token != token.RightParen && p.s.r > 0 && !ellipsis {
				args = append(args, p.parseExpr())
				if p.s.Token == token.Ellipsis {
					ellipsis = true
					p.next()
				}
				if !p.expectCommaOr(token.RightParen, "arguments") {
					break
				}
				p.next()
			}
			p.expect(token.RightParen)
			p.next()

			x = &expr.Call{
				Position: pos,
				Func:     x,
				Args:     args,
				Ellipsis: ellipsis,
			}
		case token.LeftBraceTable, token.LeftBrace:
			if tExpr, isType := x.(*expr.Type); isType {
				switch t := tExpr.Type.(type) {
				case *tipe.Array:
					x = p.parseArrayLiteral(t)
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
				t := &expr.Type{Position: pos, Type: &tipe.Unresolved{Name: xpr.Name}}
				x = t
			} else if t := maybePackageType(x); t != nil {
				t := &expr.Type{Position: pos, Type: t}
				x = t
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

func (p *Parser) parseTypeAssert(lhs expr.Expr) expr.Expr {
	pos := p.pos()
	p.expect(token.LeftParen)
	p.next()

	var typ tipe.Type
	if p.s.Token == token.Type {
		p.expect(token.Type)
		p.next()
	} else {
		typ = p.parseType()
	}

	p.expect(token.RightParen)
	p.next()

	ex := &expr.TypeAssert{
		Position: pos,
		Left:     lhs,
		Type:     typ,
	}
	return ex
}

func (p *Parser) parseIndex(lhs expr.Expr) expr.Expr {
	p.expect(token.LeftBracket)

	res := &expr.Index{
		Position: p.pos(),
		Left:     lhs,
	}

	p.next()

	for p.s.Token != 0 && p.s.Token != token.RightBracket {
		if len(res.Indicies) != 0 {
			if !p.expect(token.Comma) {
				break
			}
			p.next()
		}

		if p.s.Token == token.Colon {
			slice := &expr.Slice{Position: p.pos()}
			// [:expr]
			p.next()
			if p.s.Token == token.RightBracket || p.s.Token == token.Comma {
				res.Indicies = append(res.Indicies, slice)
				continue
			}
			slice.High = p.parseExpr()
			res.Indicies = append(res.Indicies, slice)
			continue
		}

		e := p.parseExpr()
		if p.s.Token == token.RightBracket || p.s.Token == token.Comma {
			// [expr]
			res.Indicies = append(res.Indicies, e)
			continue
		}
		p.expect(token.Colon)
		colonPos := p.pos()
		p.next()
		if p.s.Token == token.RightBracket || p.s.Token == token.Comma {
			// [expr:]
			res.Indicies = append(res.Indicies, &expr.Slice{Position: colonPos, Low: e})
			continue
		}
		high := p.parseExpr()
		if p.s.Token == token.RightBracket || p.s.Token == token.Comma {
			// [expr:high]
			res.Indicies = append(res.Indicies, &expr.Slice{
				Position: colonPos,
				Low:      e,
				High:     high,
			})
			continue
		}
		p.expect(token.Colon)
		p.next()
		max := p.parseExpr()
		// [expr:high:max]
		res.Indicies = append(res.Indicies, &expr.Slice{
			Position: colonPos,
			Low:      e,
			High:     high,
			Max:      max,
		})

	}
	p.expect(token.RightBracket)
	p.next()
	return res
}

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

func (p *Parser) maybeParseParamType() (t tipe.Type) {
	if p.s.Token == token.Ellipsis {
		p.next()
		typ := p.maybeParseType()
		return &tipe.Ellipsis{Elem: typ}
	}
	return p.maybeParseType()
}

func (p *Parser) parseParam() (name string, t tipe.Type) {
	// Scan what may be a type, or may be a parameter name.
	first := p.maybeParseParamType()
	if n := typeAsName(first); n != "" && p.s.Token > 0 && p.s.Token != token.Comma && p.s.Token != token.RightParen {
		// Looks like a type may follow. Treat first as a name.
		name = n
		t = p.maybeParseParamType()
	} else {
		t = first
	}
	if t == nil {
		p.errorf("expected name or type, got %s", p.s.Token)
		p.next() // make progress
	} else if p.s.Token == token.Comma {
		p.next()
	}
	return name, t
}

func typeAsName(t tipe.Type) string {
	if u, ok := t.(*tipe.Unresolved); ok && u.Package == "" {
		return u.Name
	}
	return ""
}

func (p *Parser) parseParamTuple() (names []string, params *tipe.Tuple) {
	params = &tipe.Tuple{}
	for p.s.Token > 0 && p.s.Token != token.RightParen {
		name, t := p.parseParam()
		if t == nil {
			continue
		}
		names = append(names, name)
		params.Elems = append(params.Elems, t)
	}
	// Either none of the parameters have names, or all do.
	named := false
	for _, n := range names {
		if n != "" {
			named = true
		}
	}
	if named {
		for i, n := range names {
			if n == "" {
				names[i] = typeAsName(params.Elems[i])
				if names[i] == "" {
					p.error("function signature mixes named and unnamed arguments")
					return nil, &tipe.Tuple{}
				}
				params.Elems[i] = nil
			} else {
				// Back-propagate types for named, typeless params.
				t := params.Elems[i]
				for j := i - 1; j >= 0 && params.Elems[j] == nil; j-- {
					params.Elems[j] = t
				}
			}
		}
		for _, t := range params.Elems {
			if t == nil {
				p.error("function signature mixes named and unnamed arguments")
				return nil, &tipe.Tuple{}
			}
		}
	}
	return names, params
}

func (p *Parser) parseMethodik(name string) *stmt.MethodikDecl {
	c := &stmt.MethodikDecl{
		Name: name,
		Type: &tipe.Named{
			// TODO Spec
			Name: name,
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
		switch p.s.Token {
		case token.RightBracket:
			p.next()
			if table {
				return &tipe.Table{Type: p.parseType()}
			} else {
				return &tipe.Slice{Elem: p.parseType()}
			}
		case token.Int:
			sz := p.s.Literal.(*big.Int).Int64()
			p.next()
			p.expect(token.RightBracket)
			p.next()
			return &tipe.Array{Len: sz, Elem: p.parseType()}
		case token.Ellipsis:
			p.next()
			p.expect(token.RightBracket)
			p.next()
			return &tipe.Array{Elem: p.parseType(), Ellipsis: true}
		default:
			p.errorf("invalid token=%v in type declaration", p.s.Token)
			return nil
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
			var n string
			var t tipe.Type
			var embed bool
			var ftag tipe.StructTag
			switch p.s.Token {
			case token.Mul:
				t = p.parseType()
			default:
				n = p.parseIdent().Name
			}
			if p.s.Token != token.Comma {
				switch p.s.Token {
				case token.RightBrace, token.Semicolon:
					// embedded type field
					if n != "" {
						t = &tipe.Unresolved{Name: n}
					} else {
						n = t.(*tipe.Pointer).Elem.(*tipe.Unresolved).Name
					}
					embed = true
				default:
					t = p.parseType()
				}
				if p.s.Token == token.String {
					str, err := strconv.Unquote(p.s.Literal.(string))
					if err != nil {
						p.errorf("error while parsing struct field tag: %v", err)
						return s
					}
					ftag = tipe.StructTag(str)
					p.next()
				}
			}
			if n != "" && n != "_" && tags[n] {
				p.errorf("field %s redeclared in struct %s", n, format.Type(s))
			} else {
				tags[n] = true
				s.Fields = append(s.Fields, tipe.StructField{
					Name:     n,
					Type:     t,
					Tag:      ftag,
					Embedded: embed,
				})
			}
			if p.s.Token == token.Comma || p.s.Token == token.Semicolon {
				p.next()
			} else if p.s.Token != token.RightBrace {
				p.expect(token.Comma) // produce error
			}
		}
		p.expect(token.RightBrace)
		p.next()
		// consolidate fields types.
		// a nil field type is one from e.g.:
		//  type T struct { x, y int }
		// x has no type (but we want it to have 'int'.)
		for i := len(s.Fields) - 1; i > 0; i-- {
			sf := &s.Fields[i]
			sfn := &s.Fields[i-1]
			if sfn.Type == nil {
				sfn.Type = sf.Type
			}
		}
		return s
	case token.Interface:
		p.next()
		p.expect(token.LeftBrace)
		p.next()
		iface := &tipe.Interface{Methods: make(map[string]*tipe.Func)}
		for p.s.Token > 0 && p.s.Token != token.RightBrace {
			f := p.parseFuncType(false)
			// TODO: we are throwing away a lot of information
			// not technically part of the type but that we want
			// in the AST for pretty printing. Recover it.
			iface.Methods[f.Name] = f.Type
			if p.s.Token == token.Semicolon {
				p.next()
			}
		}
		p.expect(token.RightBrace)
		p.next()
		return iface
	case token.Func:
		p.next()
		lit := p.parseFuncType(false)
		return lit.Type
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
	case token.ChanOp:
		// <-chan T, a read-only channel
		p.next()
		p.expect(token.Chan)
		p.next()
		s := &tipe.Chan{
			Direction: tipe.ChanRecv,
			Elem:      p.parseType(),
		}
		return s
	case token.Chan:
		// chan T, or chan<- T
		p.next()
		s := &tipe.Chan{}
		if p.s.Token == token.ChanOp {
			s.Direction = tipe.ChanSend
			p.next()
		} else {
			s.Direction = tipe.ChanBoth
		}
		s.Elem = p.parseType()
		return s
	case token.Semicolon, token.Comma, token.RightParen, token.LeftBrace:
		// no type
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
		tokPos := p.pos()

		p.next()
		var right []expr.Expr
		if p.s.Token == token.Range {
			rangePos := p.pos()
			p.next()
			if tok != token.Define && tok != token.Assign {
				right = []expr.Expr{&expr.Bad{
					Position: rangePos,
					Error:    p.error("range can only be used inside ':=' or '='"),
				}}
			} else {
				right = []expr.Expr{&expr.Unary{
					Position: rangePos,
					Op:       token.Range,
					Expr:     p.parseExpr(),
				}}
			}
		} else {
			right = p.parseExprs()
		}
		if tok == token.Define {
			for i, e := range exprs {
				if _, ok := e.(*expr.Ident); !ok {
					exprs[i] = &expr.Bad{
						Position: e.Pos(),
						Error:    p.error("expected identifier as declaration"),
					}
				}
			}
		}
		if arithOp := arithAssignOp(tok); arithOp != token.Unknown {
			if len(exprs) != 1 || len(right) != 1 {
				right = []expr.Expr{&expr.Bad{
					Position: tokPos,
					Error:    p.error(fmt.Sprintf("arithmetic assignement %q only accepts one argument", tok)),
				}}
			} else {
				right[0] = &expr.Binary{
					Position: tokPos,
					Op:       arithOp,
					Left:     exprs[0],
					Right:    right[0],
				}
			}
		}
		if e, isShell := right[0].(*expr.Shell); isShell {
			if lhs, isIdent := exprs[0].(*expr.Ident); isIdent {
				if lhs.Name == "_" {
					e.DropOut = true
				}
			}
		}
		return &stmt.Assign{
			Position: tokPos,
			Decl:     tok == token.Define,
			Left:     exprs,
			Right:    right,
		}
	}

	if len(exprs) != 1 {
		p.error("expected one expression")
	}

	switch p.s.Token {
	case token.Inc, token.Dec:
		// TODO: do we want to introduce a specialized statement for this?
		op := token.Add
		if p.s.Token == token.Dec {
			op = token.Sub
		}
		opPos := p.pos()
		p.next()
		return &stmt.Assign{
			Position: opPos,
			Left:     []expr.Expr{exprs[0]},
			Right: []expr.Expr{&expr.Binary{
				Position: opPos,
				Op:       op,
				Left:     exprs[0],
				Right:    &expr.BasicLiteral{Value: big.NewInt(1)},
			}},
		}
	case token.ChanOp:
		pos := p.pos()
		p.next()
		return &stmt.Send{
			Position: pos,
			Chan:     exprs[0],
			Value:    p.parseExpr(),
		}
	case token.Colon:
		// check whether this is 'case <-channel:'
		if e, isUnary := exprs[0].(*expr.Unary); isUnary && e.Op == token.ChanOp {
			return &stmt.Simple{Expr: e}
		}
		p.next()
		// TODO: we can be stricter here, sometimes it is invalid to declare a label.
		if lhs, isIdent := exprs[0].(*expr.Ident); isIdent {
			return &stmt.Labeled{
				Position: exprs[0].Pos(),
				Label:    lhs.Name,
				Stmt:     p.parseStmt(),
			}
		}
		return &stmt.Bad{
			Position: p.pos(),
			Error:    p.error("bad label declaration"),
		}
	}

	// TODO len==1
	if e, isShell := exprs[0].(*expr.Shell); isShell {
		e.TrapOut = false
	}
	return &stmt.Simple{Expr: exprs[0]}
}

func (p *Parser) extractExpr(s stmt.Stmt) expr.Expr {
	if e, isExpr := s.(*stmt.Simple); isExpr {
		return e.Expr
	}
	return &expr.Bad{
		Position: s.Pos(),
		Error:    p.error("expected boolean expression, found statement"),
	}
}

func extractRange(s stmt.Stmt) (res *stmt.Range) {
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
	return &stmt.Range{Position: r.Pos(), Decl: a.Decl, Key: key, Val: val, Expr: r.Expr}
}

func (p *Parser) parseStmt() stmt.Stmt {
	switch p.s.Token {
	// TODO: many many kinds of statements
	case token.If:
		s := &stmt.If{Position: p.pos()}
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
	case token.Ident, token.Int, token.Float,
		token.Add, token.Sub, token.Mul, token.ChanOp, token.Not, token.Map,
		token.Func, token.LeftBracket, token.LeftParen, token.String, token.Rune, token.Shell:
		// A "simple" statement, no control flow.
		s := p.parseSimpleStmt()
		p.expectSemi()
		return s
	case token.Return:
		pos := p.pos()
		p.next()
		s := &stmt.Return{Position: pos}
		if p.s.Token != token.Semicolon && p.s.Token != token.RightBrace {
			s.Exprs = p.parseExprs()
		}
		p.expectSemi()
		return s
	case token.Defer:
		pos := p.pos()
		p.next()
		fct := p.parseExpr()
		if _, ok := fct.(*expr.Call); !ok {
			return &stmt.Bad{
				Position: pos,
				Error:    p.error("expression in defer must be function call"),
			}
		}
		s := &stmt.Defer{
			Position: pos,
			Expr:     fct,
		}
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
	case token.Go:
		s := p.parseGo()
		p.expectSemi()
		return s
	case token.Const:
		pos := p.pos()
		p.next()
		if p.s.Token == token.LeftParen {
			p.next()
			s := &stmt.ConstSet{Position: pos}
			for p.s.Token > 0 && p.s.Token != token.RightParen {
				s.Consts = append(s.Consts, p.parseConst())
				if p.s.Token == token.Semicolon {
					p.next()
				}
			}
			p.expect(token.RightParen)
			p.next()
			p.expectSemi()
			return s
		}
		s := p.parseConst()
		s.Position = pos
		p.expectSemi()
		return s
	case token.Var:
		pos := p.pos()
		p.next()
		if p.s.Token == token.LeftParen {
			p.next()
			s := &stmt.VarSet{Position: pos}
			for p.s.Token > 0 && p.s.Token != token.RightParen {
				s.Vars = append(s.Vars, p.parseVar())
				if p.s.Token == token.Semicolon {
					p.next()
				}
			}
			p.expect(token.RightParen)
			p.next()
			p.expectSemi()
			return s
		}
		s := p.parseVar()
		s.Position = pos
		p.expectSemi()
		return s
	case token.Methodik:
		pos := p.pos()
		p.next()
		m := p.parseMethodik(p.parseIdent().Name)
		m.Position = pos
		p.expectSemi()
		return m
	case token.Type:
		pos := p.pos()
		p.next()
		if p.s.Token == token.LeftParen {
			p.next()
			s := &stmt.TypeDeclSet{Position: pos}
			for p.s.Token > 0 && p.s.Token != token.RightParen {
				s.TypeDecls = append(s.TypeDecls, p.parseTypeDecl())
				if p.s.Token == token.Semicolon {
					p.next()
				}
			}
			p.expect(token.RightParen)
			p.next()
			p.expectSemi()
			return s
		}
		s := p.parseTypeDecl()
		s.Position = pos
		p.expectSemi()
		return s
	case token.Import:
		pos := p.pos()
		p.next()
		if p.s.Token == token.LeftParen {
			p.next()
			s := &stmt.ImportSet{Position: pos}
			for p.s.Token > 0 && p.s.Token != token.RightParen {
				s.Imports = append(s.Imports, p.parseImport())
				if p.s.Token == token.Semicolon {
					p.next()
				}
			}
			p.expect(token.RightParen)
			p.next()
			p.expectSemi()
			return s
		}
		s := p.parseImport()
		s.Position = pos
		p.expectSemi()
		return s
	case token.Continue, token.Break, token.Goto, token.Fallthrough:
		s := p.parseBranch()
		p.expectSemi()
		return s
	case token.Switch:
		s := p.parseSwitch()
		p.expectSemi()
		return s
	case token.Select:
		s := p.parseSelect()
		p.expectSemi()
		return s
	}
	panic(fmt.Sprintf("TODO parseStmt %s", p.s.Token))
}

func (p *Parser) parseImport() (s *stmt.Import) {
	name := ""
	if p.s.Token == token.Ident {
		name = p.s.Literal.(string)
		p.next()
	}
	if !p.expect(token.String) {
		p.next()
		return &stmt.Import{}
	}
	path := p.s.Literal.(string)
	s = &stmt.Import{
		Name: name,
		Path: path[1 : len(path)-1],
	}
	p.next()
	return s
}

func (p *Parser) parseBranch() *stmt.Branch {
	s := &stmt.Branch{
		Position: p.pos(),
		Type:     p.s.Token,
	}
	p.next()
	if p.s.Token == token.Ident {
		s.Label = p.s.Literal.(string)
		p.next()
	}
	return s
}

func (p *Parser) parseGo() stmt.Stmt {
	g := &stmt.Go{
		Position: p.pos(),
	}
	p.expect(token.Go)
	p.next()
	e := p.parsePrimaryExpr()
	call, ok := e.(*expr.Call)
	if !ok {
		return &stmt.Bad{
			Position: g.Pos(),
			Error:    p.errorf("go statement must invoke function"),
		}
	}
	g.Call = call
	return g
}

func (p *Parser) parseConst() *stmt.Const {
	s := &stmt.Const{
		Position: p.pos(),
	}
items:
	for {
		p.expect(token.Ident)
		s.NameList = append(s.NameList, p.s.Literal.(string))
		p.next()
		switch p.s.Token {
		case token.Ident:
			s.Type = p.parseType()
			if p.s.Token == token.Assign {
				p.next()
				s.Values = p.parseExprs()
			}
			if p.s.Token == token.Semicolon {
				break items
			}
		case token.Comma:
			p.next()
			continue
		case token.Assign:
			p.next()
			s.Values = p.parseExprs()
			break items
		case token.Semicolon:
			break items
		default:
			p.errorf("invalid token %v in const declaration", p.s.Token)
			break items
		}
	}
	p.expectSemi()
	if len(s.Values) == 0 && s.Type != nil {
		p.errorf("const declaration cannot have type without expression")
	}
	switch {
	case len(s.Values) != 0 && len(s.NameList) > len(s.Values):
		p.errorf("missing value in const declaration")

	case len(s.Values) != 0 && len(s.NameList) < len(s.Values):
		p.errorf("extra expression in const declaration")
	}

	return s
}

func (p *Parser) parseVar() *stmt.Var {
	s := &stmt.Var{
		Position: p.pos(),
	}
items:
	for {
		p.expect(token.Ident)
		s.NameList = append(s.NameList, p.s.Literal.(string))
		p.next()
		switch p.s.Token {
		case token.Chan, token.Ident, token.LeftBracket, token.Map, token.Struct:
			s.Type = p.parseType()
			if p.s.Token == token.Assign {
				p.next()
				s.Values = p.parseExprs()
			}
			if p.s.Token == token.Semicolon {
				break items
			}
		case token.Comma:
			p.next()
			continue
		case token.Assign:
			p.next()
			s.Values = p.parseExprs()
			break items
		case token.Semicolon:
			break items
		default:
			p.errorf("invalid token %v in var declaration", p.s.Token)
			break items
		}
	}
	p.expectSemi()
	return s
}

func (p *Parser) parseTypeDecl() *stmt.TypeDecl {
	pos := p.pos()
	t := &tipe.Named{
		Name: p.parseIdent().Name,
		Type: p.parseType(),
	}
	s := &stmt.TypeDecl{
		Position: pos,
		Name:     t.Name,
		Type:     t,
	}
	return s
}

func (p *Parser) parseSelect() stmt.Stmt {
	p.expect(token.Select)
	s := &stmt.Select{
		Position: p.pos(),
	}
	p.next()
	p.expect(token.LeftBrace)
	p.next()

	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		var c stmt.SelectCase
		c.Position = p.pos()
		switch p.s.Token {
		case token.Case:
			p.expect(token.Case)
			p.next()
			c.Stmt = p.parseSimpleStmt()
		case token.Default:
			p.expect(token.Default)
			p.next()
			c.Default = true
		}
		p.expect(token.Colon)
		p.next()
		c.Body = &stmt.Block{Stmts: p.parseStmts()}
		s.Cases = append(s.Cases, c)
	}
	p.expect(token.RightBrace)
	p.next()
	p.expectSemi()
	return s
}

func (p *Parser) parseFor() stmt.Stmt {
	pos := p.pos()
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
		return &stmt.For{Position: pos, Body: body()}
	}
	if p.s.Token == token.Range {
		// for range r { }
		p.next()
		return &stmt.Range{Position: pos, Expr: p.parseExpr(), Body: body()}
	}
	if p.s.Token == token.Semicolon {
		p.next()
		if p.s.Token == token.Semicolon {
			p.next()
			if p.s.Token == token.LeftBrace {
				// for ;; { }
				return &stmt.For{Position: pos, Body: body()}
			}
			// for ;;i2 { }
			i2 := p.parseSimpleStmt()
			return &stmt.For{Position: pos, Post: i2, Body: body()}
		}
		i1 := p.parseSimpleStmt()
		if p.s.Token == token.Semicolon {
			// for ;i1; { }
			p.next()
			return &stmt.For{Position: pos, Cond: p.extractExpr(i1), Body: body()}
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
				return &stmt.For{Position: pos, Cond: p.extractExpr(i0), Body: body()}
			}
		}
		p.expectSemi()
		p.next()
		if p.s.Token == token.Semicolon {
			// for i0;; {}
			return &stmt.For{Position: pos, Init: i0, Body: body()}
		}
		i1 := p.parseSimpleStmt()
		p.expectSemi()
		p.next()
		if p.s.Token == token.LeftBrace {
			// for i0;i1; { }
			return &stmt.For{Position: pos, Init: i0, Cond: p.extractExpr(i1), Body: body()}
		}
		i2 := p.parseSimpleStmt()
		p.expect(token.LeftBrace)
		// for i0; i1; i2 { }
		return &stmt.For{
			Position: pos,
			Init:     i0,
			Cond:     p.extractExpr(i1),
			Post:     i2,
			Body:     body(),
		}
	}

	// TODO
	panic("TODO parseFor range")
}

func (p *Parser) parseSwitch() stmt.Stmt {
	pos := p.pos()
	s1, s2, isTypeSwitch := p.parseSwitchHeader()

	if isTypeSwitch {
		s := &stmt.TypeSwitch{
			Position: pos,
			Init:     s1,
			Assign:   s2,
		}
		p.parseTypeSwitch(s)
		return s
	}

	s := &stmt.Switch{
		Position: pos,
		Init:     s1,
	}
	if s2 != nil {
		s.Cond = p.extractExpr(s2)
	}
	p.parseExprSwitch(s)
	return s
}

func (p *Parser) parseSwitchHeader() (stmt.Stmt, stmt.Stmt, bool) {
	p.expect(token.Switch)
	p.next()

	var (
		// consider:
		// switch <s1>; <s2> { ... }
		// from s1 and s2, we need to decide whether we are dealing with
		// an expression-switch or a type-switch.
		s1           stmt.Stmt
		s2           stmt.Stmt
		isTypeSwitch = false
	)

	if p.s.Token != token.LeftBrace {
		p.noCompLit = true
		s1 = p.parseSimpleStmt()
		switch p.s.Token {
		case token.Semicolon:
			p.next()
			s2 = p.parseSimpleStmt()
			switch s2 := s2.(type) {
			default:
			// switch x := foo(); x { ... }
			case *stmt.Simple:
				// switch x := foo(); x.(type) { ... }
				_, isTypeSwitch = s2.Expr.(*expr.TypeAssert)
			case *stmt.Assign:
				// switch x := foo(); y := x.(type) { ... }
				if len(s2.Right) == 1 {
					_, isTypeSwitch = s2.Right[0].(*expr.TypeAssert)
				}
			}
		default:
			switch init := s1.(type) {
			default:
				// switch foo() { ... }
			case *stmt.Simple:
				// switch x.(type) { ... }
				_, isTypeSwitch = init.Expr.(*expr.TypeAssert)
			case *stmt.Assign:
				// switch x := x.(type) { ... }
				if len(init.Right) == 1 {
					_, isTypeSwitch = init.Right[0].(*expr.TypeAssert)

				}
			}
			// expression-switch or type-switch,
			// without any init statement: make it the condition
			s2 = s1
			s1 = nil
		}
		p.noCompLit = false
	}
	p.expect(token.LeftBrace)

	return s1, s2, isTypeSwitch
}

func (p *Parser) parseExprSwitch(s *stmt.Switch) {
	p.expect(token.LeftBrace)
	p.next()

	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		var c stmt.SwitchCase
		switch p.s.Token {
		case token.Case:
			p.expect(token.Case)
			p.next()
			c.Conds = p.parseExprs()
		case token.Default:
			p.expect(token.Default)
			p.next()
			c.Default = true
		default:
			p.errorf("syntax error: got token %q, want %q or %q", p.s.Token, token.Case, token.Default)
			return
		}
		p.expect(token.Colon)
		p.next()
		c.Body = &stmt.Block{Stmts: p.parseStmts()}
		s.Cases = append(s.Cases, c)
	}
	p.expect(token.RightBrace)
	p.next()
	p.expectSemi()

	for i, cse := range s.Cases {
		lastCase := i == len(s.Cases)-1
		for j, e := range cse.Body.Stmts {
			// TODO: detect fallthrough statements in non-top-level statements
			lastStmt := j == len(cse.Body.Stmts)-1
			switch e := e.(type) {
			case *stmt.Branch:
				if e.Type != token.Fallthrough {
					continue
				}
				if !lastStmt {
					p.error("fallthrough statement out of place")
					return
				}
				if lastCase {
					p.error("cannot fallthrough final case in switch")
					return
				}
			}
		}
	}
}

func (p *Parser) parseTypeSwitch(s *stmt.TypeSwitch) {
	p.expect(token.LeftBrace)
	p.next()

	for p.s.Token != token.RightBrace {
		var c stmt.TypeSwitchCase
		switch p.s.Token {
		case token.Case:
			p.expect(token.Case)
			p.next()
			for p.s.Token != token.Colon {
				c.Types = append(c.Types, p.parseType())
				if p.s.Token == token.Comma {
					p.next()
				}
			}
		case token.Default:
			p.expect(token.Default)
			p.next()
			c.Default = true
		}
		p.expect(token.Colon)
		p.next()
		c.Body = &stmt.Block{Stmts: p.parseStmts()}
		for _, e := range c.Body.Stmts {
			// TODO: detect fallthrough statements in non-top-level statements
			switch e := e.(type) {
			case *stmt.Branch:
				if e.Type == token.Fallthrough {
					p.error("cannot fallthrough in type switch")
				}
			}
		}

		s.Cases = append(s.Cases, c)
	}

	p.expect(token.RightBrace)
	p.next()
	p.expectSemi()
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
	for p.s.Token > 0 && p.s.Token != token.RightBrace &&
		p.s.Token != token.Case && p.s.Token != token.Default {
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
	f := &expr.FuncLiteral{
		Type: &tipe.Func{},
	}

	if method {
		// func (a) f()
		p.expect(token.LeftParen)
		p.next()
		if p.s.Token == token.Mul {
			f.PointerReceiver = true
			p.next()
		}
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
		f.ParamNames, f.Type.Params = p.parseParamTuple()
		if params := f.Type.Params; len(params.Elems) > 0 {
			last := params.Elems[len(params.Elems)-1]
			if _, variadic := last.(*tipe.Ellipsis); variadic {
				f.Type.Variadic = true
			}
		}
	} else {
		f.Type.Params = new(tipe.Tuple)
	}
	p.expect(token.RightParen)
	p.next()

	if p.s.Token == token.LeftParen {
		p.expect(token.LeftParen)
		p.next()
		if p.s.Token != token.RightParen {
			f.ResultNames, f.Type.Results = p.parseParamTuple()
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
	funcPos := p.pos()
	p.next()
	f := p.parseFuncType(method)
	f.Position = funcPos
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
		x := &expr.BasicLiteral{
			Position: p.pos(),
			Value:    p.s.Literal,
		}
		p.next()
		return x
	case token.Rune:
		x := &expr.BasicLiteral{
			Position: p.pos(),
			Value:    p.s.Literal,
		}
		p.next()
		return x
	case token.String:
		s, _ := strconv.Unquote(p.s.Literal.(string))
		x := &expr.BasicLiteral{
			Position: p.pos(),
			Value:    s,
		}
		p.next()
		return x
	case token.LeftParen:
		origNoCompLit := p.noCompLit
		pos := p.pos()
		p.noCompLit = false
		p.next()
		ex := p.parseExpr() // TODO or a type?
		p.expect(token.RightParen)
		p.next()
		p.noCompLit = origNoCompLit
		return &expr.Unary{
			Position: pos,
			Op:       token.LeftParen, Expr: ex,
		}
	case token.Func:
		return p.parseFunc(false)
	case token.Shell:
		x := &expr.Shell{
			Position: p.pos(),
			TrapOut:  true,
		}
		p.next()
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

	pos := p.pos()
	if t := p.maybeParseType(); t != nil {
		return &expr.Type{
			Position: pos,
			Type:     t,
		}
	}

	p.next()
	res := &expr.Bad{
		Position: pos,
		Error:    p.errorf("expected operand, got %s", p.s.Token),
	}
	return res
}

func (p *Parser) parseArrayLiteral(t tipe.Type) *expr.ArrayLiteral {
	x := &expr.ArrayLiteral{Position: p.pos(), Type: t.(*tipe.Array)}
	x.Keys, x.Values = p.parseKeyedLiteral()
	if x.Type.Ellipsis || len(x.Keys) > 0 {
		n := int64(len(x.Values))
		if len(x.Keys) > 0 {
			for _, k := range x.Keys {
				i := k.(*expr.BasicLiteral).Value.(*big.Int).Int64()
				if i+1 > n {
					n = i + 1
				}
			}
		}
		x.Type.Len = n
	}
	return x
}

func (p *Parser) parseSliceLiteral(t tipe.Type) *expr.SliceLiteral {
	x := &expr.SliceLiteral{Position: p.pos(), Type: t.(*tipe.Slice)}
	x.Keys, x.Values = p.parseKeyedLiteral()
	return x
}

func (p *Parser) parseTableLiteral(t tipe.Type) *expr.TableLiteral {
	x := &expr.TableLiteral{Position: p.pos(), Type: t.(*tipe.Table)}
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		switch p.s.Token {
		case token.LeftBraceTable:
			p.next()
			// column names: {|"x","y"|},
			if len(x.ColNames) != 0 || len(x.Rows) != 0 {
				p.errorf("column names can only appear at beginning of table literal")
			}
			for p.s.Token > 0 && p.s.Token != token.Pipe {
				x.ColNames = append(x.ColNames, p.parseExpr())
				if p.s.Token != token.Comma {
					break
				}
				p.next()
			}
			p.expect(token.RightBraceTable)
			p.next()
		case token.LeftBrace:
			p.next()
			var row []expr.Expr
			for p.s.Token > 0 && p.s.Token != token.RightBrace {
				row = append(row, p.parseExpr())
				if p.s.Token != token.Comma {
					break
				}
				p.next()
			}
			x.Rows = append(x.Rows, row)
			p.expect(token.RightBrace)
			p.next()
		}
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.next()
	return x
}

func (p *Parser) parseMapLiteral(t tipe.Type) *expr.MapLiteral {
	x := &expr.MapLiteral{Position: p.pos(), Type: t}
	x.Keys, x.Values = p.parseKeyedLiteral()
	return x
}

func (p *Parser) parseCompLiteral(t tipe.Type) *expr.CompLiteral {
	x := &expr.CompLiteral{Position: p.pos(), Type: t}
	x.Keys, x.Values = p.parseKeyedLiteral()
	return x
}

func (p *Parser) parseKeyedLiteral() (keys []expr.Expr, values []expr.Expr) {
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		e := p.parseExpr()
		if p.s.Token == token.Colon {
			p.next()
			v := p.parseExpr()

			if len(values) > 0 && len(keys) == 0 {
				p.errorf("mixture of keyed fields and value initializers")
				continue
			}

			keys = append(keys, e)
			values = append(values, v)
		} else {
			if len(values) > 0 && len(keys) > 0 {
				p.errorf("mixture of keyed fields and value initializers")
				continue
			}
			values = append(values, e)
		}
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.expect(token.RightBrace)
	p.next()
	return keys, values
}

type Errors []Error

func (e Errors) Error() string {
	buf := new(bytes.Buffer)
	buf.WriteString("neugram: parser errors:\n")
	for _, err := range e {
		fmt.Fprintf(buf, "off %5d: %v\n", err.Offset, err.Msg)
	}
	return buf.String()
}

type Error struct {
	Pos    src.Pos
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
		Pos:    p.pos(),
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
	res := &expr.Ident{Position: p.pos(), Name: "_"}
	if p.expect(token.Ident) {
		res.Name = p.s.Literal.(string)
	}
	p.next()
	return res
}
