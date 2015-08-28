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
		p.Waiting <- p.inStmt
		if !more {
			close(p.Waiting)
			return
		}
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
	p.Close()
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
				panic("TODO parse selector")
			case token.LeftParen:
				panic("TODO parse type assertion")
			default:
				panic("TODO expect selector type assertion")
			}
		case token.LeftBracket:
			panic("TODO array index")
		case token.LeftParen:
			args := p.parseArgs()
			return &expr.Call{Func: x, Args: args}
		case token.LeftBrace:
			// TODO could be composite literal, check type
			// If not a composite literal, end of statement.
			return x
		default:
			return x
		}
	}

	return x
}

func (p *Parser) parseIn() (params []*tipe.Field) {
	for p.s.Token > 0 && p.s.Token != token.RightParen {
		f := &tipe.Field{
			Name: p.parseIdent().Name,
			Type: p.maybeParseType(),
		}
		if f.Type != nil {
			for i := len(params) - 1; i >= 0 && params[i].Type == nil; i-- {
				params[i].Type = f.Type
			}
		}
		if p.s.Token == token.Comma {
			p.next()
		}
		params = append(params, f)
	}
	return params
}

func (p *Parser) parseOut() (params []*tipe.Field) {
	for p.s.Token > 0 && p.s.Token != token.RightParen {
		f := &tipe.Field{}
		t := p.maybeParseType()
		n, ok := t.(*tipe.Unresolved)
		if ok {
			f.Name, ok = n.Name.(string)
		}
		if ok {
			f.Type = p.maybeParseType()
		} else {
			f.Type = t
		}
		if p.s.Token == token.Comma {
			p.next()
		}
		params = append(params, f)
	}
	named := false
	for _, param := range params {
		if param.Type != nil {
			named = true
			break
		}
	}
	if !named {
		// In an output list, a sequence (a, b, c) is a list
		// of types, not names.
		for _, param := range params {
			param.Type = &tipe.Unresolved{param.Name}
			param.Name = ""
		}
	}
	return params
}

func (p *Parser) parseType() tipe.Type {
	t := p.maybeParseType()
	if t == nil {
		p.errorf("expected type, got %s", p.s.Token)
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
			return &tipe.Unresolved{&expr.Selector{ident, sel}}
		}
		return &tipe.Unresolved{ident.Name}
	case token.Struct:
		p.next()
		p.expect(token.LeftBrace)
		p.next()
		s := &tipe.Struct{}
		for p.s.Token > 0 && p.s.Token != token.RightBrace {
			s.Fields = append(s.Fields, &tipe.Field{
				Name: p.parseIdent().Name,
				Type: p.parseType(),
			})
			if p.s.Token == token.Comma {
				p.next()
			} else if p.s.Token != token.RightBrace {
				p.expect(token.Comma) // prodce error
			}
		}
		p.expect(token.RightBrace)
		p.next()
		return s
	case token.Mul: // pointer type
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.Func:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.Map:
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
	case token.LeftParen:
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
		token.Func, token.LeftBracket, token.LeftParen:
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
		s := &stmt.Type{
			Name: p.parseIdent().Name,
			Type: p.parseType(),
		}
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

func (p *Parser) parseFuncType() *tipe.Func {
	f := &tipe.Func{}
	p.expect(token.LeftParen)
	p.next()
	if p.s.Token != token.RightParen {
		f.In = p.parseIn()
	}
	p.expect(token.RightParen)
	p.next()

	if p.s.Token == token.LeftParen {
		p.expect(token.LeftParen)
		p.next()
		if p.s.Token != token.RightParen {
			f.Out = p.parseOut()
		}
		p.expect(token.RightParen)
		p.next()
	} else {
		typ := p.maybeParseType()
		if typ != nil {
			f.Out = []*tipe.Field{{Type: typ}}
		}
	}
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
		p.next()
		ty := p.parseFuncType()
		if p.s.Token != token.LeftBrace {
			p.next()
			return &expr.Bad{p.error("TODO just a function type")}
		}
		body := p.parseBlock()
		return &expr.FuncLiteral{
			Type: ty,
			Body: body,
		}
	}
	// TODO: other cases, eventually Func, etc

	p.next()
	return &expr.Bad{p.error("expected operand")}
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
