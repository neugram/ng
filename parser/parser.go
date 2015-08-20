// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
)

func ParseExpr(src []byte) (expr expr.Expr, err error) {
	p := newParser(src)
	if err := p.s.Next(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	expr = p.parseExpr(false)
	if len(p.err) > 0 {
		err = Errors(p.err)
	}
	if err == nil && p.s.err != io.EOF {
		err = p.s.err
	}
	return expr, err
}

func ParseStmt(src []byte) (stmt stmt.Stmt, err error) {
	p := newParser(src)
	if err := p.s.Next(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	s := p.parseStmt()
	if len(p.err) > 0 {
		err = Errors(p.err)
	}
	if err == nil && p.s.err != io.EOF {
		err = p.s.err
	}
	return s, err
}

type parser struct {
	s   *Scanner
	err []Error
}

func newParser(src []byte) *parser {
	p := &parser{
		s: NewScanner(src),
	}

	return p
}

func (p *parser) next() {
	p.s.Next()
	if p.s.Token == token.Comment {
		p.next()
	}
}

func (p *parser) parseExpr(lhs bool) expr.Expr {
	return p.parseBinaryExpr(lhs, 1)
}

func (p *parser) parseBinaryExpr(lhs bool, minPrec int) expr.Expr {
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

func (p *parser) parseUnaryExpr(lhs bool) expr.Expr {
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

func (p *parser) expectCommaOr(otherwise token.Token, msg string) bool {
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

func (p *parser) parseArgs() []expr.Expr {
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

func (p *parser) parsePrimaryExpr(lhs bool) expr.Expr {
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

func (p *parser) parseIn() (params []*tipe.Field) {
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

func (p *parser) parseOut() (params []*tipe.Field) {
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

func (p *parser) maybeParseType() tipe.Type {
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
		fmt.Printf("maybeParseType: token=%s\n", p.s.Token)
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

func (p *parser) parseExprs() []expr.Expr {
	exprs := []expr.Expr{p.parseExpr(false)}
	for p.s.Token == token.Comma {
		p.next()
		exprs = append(exprs, p.parseExpr(false))
	}
	return exprs
}

func (p *parser) parseSimpleStmt() stmt.Stmt {
	exprs := p.parseExprs()
	switch p.s.Token {
	case token.Define, token.Assign, token.AddAssign, token.SubAssign,
		token.MulAssign, token.DivAssign, token.RemAssign:
		decl := p.s.Token == token.Define
		p.next()
		// TODO: if p.s.Token == Range
		right := p.parseExprs()
		return &stmt.Assign{
			Decl:  decl,
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

func (p *parser) parseStmt() stmt.Stmt {
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
				if e, isExpr := s.Init.(expr.Expr); isExpr {
					s.Cond = e
				} else {
					fmt.Printf("expected boolean expression, found statement: %T: %s", s.Init, s.Init)
					s.Cond = &expr.Bad{p.error("expected boolean expression, found statement")}
				}
				s.Init = nil
			}
		}
		s.Body = p.parseBlock()
		if p.s.Token == token.Else {
			p.next()
			s.Else = p.parseStmt()
		} else {
			p.parseSemi()
		}
		return s
	case token.Ident, token.Int, token.Float, token.Add, token.Sub, token.Mul,
		token.Func, token.LeftBracket, token.LeftParen:
		// A "simple" statement, no control flow.
		s := p.parseSimpleStmt()
		p.parseSemi()
		return s
	case token.Return:
		p.next()
		s := &stmt.Return{Exprs: p.parseExprs()}
		p.parseSemi()
		return s
	case token.LeftBrace:
		s := p.parseBlock()
		p.parseSemi()
		return s
	}
	panic(fmt.Sprintf("TODO parseStmt %s", p.s.Token))
}

func (p *parser) parseBlock() stmt.Stmt {
	p.expect(token.LeftBrace)
	p.next()
	s := &stmt.Block{Stmts: p.parseStmts()}
	p.expect(token.RightBrace)
	p.next()
	return s
}

func (p *parser) parseStmts() (stmts []stmt.Stmt) {
	// TODO there are other kinds of blocks to exit from
	for p.s.Token > 0 && p.s.Token != token.RightBrace {
		stmts = append(stmts, p.parseStmt())
	}
	return stmts
}

func (p *parser) parseFuncType() *tipe.Func {
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

func (p *parser) parseOperand(lhs bool) expr.Expr {
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

func (p *parser) error(msg string) error {
	err := Error{
		Offset: p.s.Offset,
		Msg:    msg,
	}
	fmt.Printf("%v\n", err) // debug
	p.err = append(p.err, err)
	return err
}

func (p *parser) expect(t token.Token) bool {
	met := t == p.s.Token
	if !met {
		p.error(fmt.Sprintf("expected %q, found %q", t, p.s.Token))
	}
	return met
}

func (p *parser) parseSemi() {
	if p.s.Token == token.RightBrace {
		return
	}
	p.expect(token.Semicolon)
	p.next()
}

func (p *parser) parseIdent() *expr.Ident {
	name := "_"
	if p.expect(token.Ident) {
		name = p.s.Literal.(string)
	}
	p.next()
	return &expr.Ident{Name: name}
}
