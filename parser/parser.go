// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"bytes"
	"fmt"
	"math/big"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
)

type Result struct {
	Stmt stmt.Stmt
	Err  error
}

func New() *Parser {
	p := &Parser{
		Result:  make(chan Result),
		Waiting: make(chan bool),
		s:       newScanner(),
	}
	go p.forwardWaiting()
	go p.work()
	<-p.Waiting
	return p
}

type Parser struct {
	Result  chan Result
	Waiting chan bool

	inStmt bool // true when stmt is partially parsed
	s      *Scanner
	err    []Error
}

func (p *Parser) Add(b []byte) {
	p.s.addSrc <- b
}

func (p *Parser) Close() {
	close(p.s.addSrc)
}

func (p *Parser) forwardWaiting() {
	for {
		more := <-p.s.needSrc
		if !more {
			close(p.Waiting)
			return
		}
		p.Waiting <- p.inStmt
	}
}

func (p *Parser) work() {
	p.s.next()
	for {
		p.inStmt = false
		p.next()
		if p.s.Token == token.Unknown {
			break
		}
		p.inStmt = true
		r := Result{Stmt: p.parseStmt()}
		if len(p.err) > 0 {
			r.Err = Errors(p.err)
			p.err = nil
		}
		p.Result <- r
	}
}

func ParseStmt(src []byte) (stmt stmt.Stmt, err error) {
	b := make([]byte, 0, len(src)+1)
	b = append(b, src...)
	b = append(b, '\n') // TODO: should be unnecessary now we have Close?
	p := New()
	p.Add(b)
	var res Result
	select {
	case partial := <-p.Waiting:
		if partial {
			panic("unexpected partial statement")
			return nil, fmt.Errorf("parser.ParseStmt: unexpected partial statement")
		}
		return nil, fmt.Errorf("parser.ParseStmt: incomplete result")
	case res = <-p.Result:
	}
	if res.Err != nil {
		return nil, err
	}
	partial := <-p.Waiting
	if partial {
		return nil, fmt.Errorf("parser.ParseStmt: trailing partial statement")
	}
	p.Close()
	return res.Stmt, nil
}

func (p *Parser) next() {
	p.s.Next()
	if p.s.Token == token.Comment {
		p.next()
	}
}

func (p *Parser) parseExpr(lhs bool) expr.Expr {
	return p.parseBinaryExpr(lhs, 1)
}

func (p *Parser) parseBinaryExpr(lhs bool, minPrec int) expr.Expr {
	x := p.parseUnaryExpr(lhs)
	for prec := p.s.Token.Precedence(); prec >= minPrec; prec-- {
		for {
			op := p.s.Token
			if op.Precedence() != prec {
				break
			}
			p.next()
			y := p.parseBinaryExpr(false, prec+1)
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

func (p *Parser) parseUnaryExpr(lhs bool) expr.Expr {
	switch p.s.Token {
	case token.Add, token.Sub, token.Not:
		op := p.s.Token
		p.next()
		if p.s.err != nil {
			return &expr.Bad{Error: p.s.err}
		}
		x := p.parseUnaryExpr(false)
		// TODO: distinguish expr from types, when we have types
		return &expr.Unary{Op: op, Expr: x}
	case token.Mul:
		p.next()
		x := p.parseUnaryExpr(false)
		return &expr.Unary{Op: token.Mul, Expr: x}
	default:
		return p.parsePrimaryExpr(lhs)
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
		args = append(args, p.parseExpr(false))
		if !p.expectCommaOr(token.RightParen, "arguments") {
			break
		}
		p.next()
	}
	p.expect(token.RightParen)
	p.next()
	return args
}

func (p *Parser) parsePrimaryExpr(lhs bool) expr.Expr {
	x := p.parseOperand(lhs)
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
			ind := p.parseTableIndex()
			ind.Expr = x
			x = ind
		case token.LeftParen:
			args := p.parseArgs()
			x = &expr.Call{Func: x, Args: args}
		case token.LeftBrace:
			switch x := x.(type) {
			case *expr.TableLiteral:
				p.parseTableLiteral(x)
			case *expr.CompLiteral:
				p.parseCompLiteral(x)
			default:
				return x // end of statement
			}
		default:
			return x
		}
	}

	return x
}

func (p *Parser) parseTableIndex() *expr.TableIndex {
	x := &expr.TableIndex{}

	p.expect(token.LeftBracket)
	p.next()

	// Cols
	for p.s.Token == token.String {
		// "Col1"|"Col2"
		x.ColNames = append(x.ColNames, p.s.Literal.(string))
		p.next()
		if p.s.Token != token.Pipe {
			break
		}
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

func (p *Parser) parseRange() (r expr.Range) {
	var x expr.Expr
	if p.s.Token != token.Colon {
		// case 0, 0: or 0:1
		x = p.parseExpr(false)
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
	r.End = p.parseExpr(false)
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

func (p *Parser) parseTypeDecl(name string) stmt.Stmt {
	switch p.s.Token {
	case token.Interface:
		p.errorf("TODO interface declaration")
		return nil
	case token.Class:
		p.next()
		p.expect(token.LeftBrace)
		p.next()
		c := &stmt.ClassDecl{
			Name: name,
			Type: &tipe.Class{},
		}
		for p.s.Token > 0 && p.s.Token != token.RightBrace {
			if p.s.Token == token.Func {
				c.Methods = append(c.Methods, p.parseFunc(true))
			} else {
				if len(c.Methods) > 0 {
					p.errorf("class fields must be declared before methods")
				}
				n := p.parseIdent().Name
				t := p.parseType()
				c.Type.Tags = append(c.Type.Tags, n)
				c.Type.Fields = append(c.Type.Fields, t)
			}
			if p.s.Token == token.Comma || p.s.Token == token.Semicolon {
				p.next()
			} else if p.s.Token != token.RightBrace {
				p.expect(token.Comma) // produce error
			}
		}
		p.expect(token.RightBrace)
		p.next()
		return c
	default:
		p.errorf("expected type declaration, got %s", p.s.Token)
		return nil
	}
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
		// basic type, so we can resolve these types immediately.
		switch ident.Name {
		case "bool":
			return tipe.Bool
		case "integer":
			return tipe.Integer
		case "float":
			return tipe.Float
		case "complex":
			return tipe.Complex
		case "string":
			return tipe.String
		case "int64":
			return tipe.Int64
		case "float32":
			return tipe.Float32
		case "float64":
			return tipe.Float64
		default:
			return &tipe.Unresolved{Name: ident.Name}
		}
	case token.LeftBracket:
		p.next()
		p.expect(token.Pipe)
		p.next()
		p.expect(token.RightBracket)
		p.next()
		return &tipe.Table{Type: p.parseType()}
	case token.Mul: // pointer type
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.Func:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.Map:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	default:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	}
	return nil
}

func (p *Parser) parseExprs() []expr.Expr {
	exprs := []expr.Expr{p.parseExpr(false)}
	for p.s.Token == token.Comma {
		p.next()
		exprs = append(exprs, p.parseExpr(false))
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
					Expr: p.parseExpr(false),
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
	defer fmt.Printf("extractRange(%s) res=%s\n", s.Sexp(), res)
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
	return &stmt.Range{Key: key, Val: val, Expr: r.Expr}
}

func (p *Parser) parseStmt() stmt.Stmt {
	switch p.s.Token {
	// TODO: many many kinds of statements
	case token.If:
		s := &stmt.If{}
		p.next()
		if p.s.Token == token.Semicolon {
			// Blank Init statement.
			p.next()
			s.Cond = p.parseExpr(true)
		} else {
			s.Init = p.parseSimpleStmt()
			if p.s.Token == token.Semicolon {
				p.next()
				s.Cond = p.parseExpr(false)
			} else {
				// No Init statement, make it the condition
				s.Cond = p.extractExpr(s.Init)
				s.Init = nil
			}
		}
		s.Body = p.parseBlock()
		if p.s.Token == token.Else {
			p.next()
			s.Else = p.parseStmt()
		} else {
			p.expectSemi()
		}
		return s
	case token.Ident, token.Int, token.Float, token.Add, token.Sub, token.Mul,
		token.Func, token.LeftBracket, token.LeftParen, token.String:
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
		s.Value = p.parseExpr(false)
		p.expectSemi()
		return s
	case token.Type:
		p.next()
		s := p.parseTypeDecl(p.parseIdent().Name)
		p.expectSemi()
		return s
	}
	panic(fmt.Sprintf("TODO parseStmt %s", p.s.Token))
}

func (p *Parser) parseFor() stmt.Stmt {
	p.expect(token.For)
	p.next()

	body := func() stmt.Stmt {
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
		return &stmt.Range{Expr: p.parseExpr(false), Body: body()}
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
		// TODO come up with syntax for not-pointer-receiver
		f.PointerReceiver = true
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

func (p *Parser) parseOperand(lhs bool) expr.Expr {
	switch p.s.Token {
	case token.Ident:
		return p.parseIdent()
	case token.Int, token.Float, token.Imaginary, token.String:
		x := &expr.BasicLiteral{Value: p.s.Literal}
		p.next()
		return x
	case token.LeftParen:
		p.next()
		ex := p.parseExpr(false) // TODO or a type?
		p.expect(token.RightParen)
		p.next()
		return &expr.Unary{Op: token.LeftParen, Expr: ex}
	case token.Func:
		return p.parseFunc(false)
	}

	if t := p.maybeParseType(); t != nil {
		if t, ok := t.(*tipe.Table); ok {
			return &expr.TableLiteral{Type: t}
		} else {
			return &expr.CompLiteral{Type: t}
		}
	}

	p.next()
	return &expr.Bad{p.errorf("expected operand, got %s", p.s.Token)}
}

func (p *Parser) parseTableLiteral(x *expr.TableLiteral) {
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
				x.ColNames = append(x.ColNames, p.parseExpr(false))
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
				row = append(row, p.parseExpr(false))
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
}

func (p *Parser) parseCompLiteral(x *expr.CompLiteral) {
	p.next()
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		// TODO FieldName: value
		x.Elements = append(x.Elements, p.parseExpr(false))
		if p.s.Token != token.Comma {
			break
		}
		p.next()
	}
	p.expect(token.RightBrace)
	p.next()
}

type Errors []Error

func (e Errors) Error() string {
	buf := new(bytes.Buffer)
	buf.WriteString("numgrad: parser erorrs:\n")
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
	return fmt.Sprintf("numgrad: parser: %s (off %d)", e.Msg, e.Offset)
}

func (p *Parser) errorf(format string, a ...interface{}) error {
	return p.error(fmt.Sprintf(format, a...))
}

func (p *Parser) error(msg string) error {
	err := Error{
		Offset: p.s.Offset,
		Msg:    msg,
	}
	panic(fmt.Sprintf("%v\n", err)) // debug
	p.err = append(p.err, err)
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
