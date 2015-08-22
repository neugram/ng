// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package stmt defines data structures representing Numengrad statements.
package stmt

import (
	"bytes"
	"fmt"

	"numgrad.io/lang/expr"
)

type Stmt interface {
	Sexp() string
	stmt()
}

type Assign struct {
	Decl  bool
	Left  []expr.Expr
	Right []expr.Expr
}

type Block struct {
	Stmts []Stmt
}

type If struct {
	Init Stmt
	Cond expr.Expr
	Body Stmt // always *BlockStmt
	Else Stmt
}

type For struct {
	Init Stmt
	Cond expr.Expr
	Post Stmt
	Body Stmt // always *BlockStmt
}

type Range struct {
	Key  expr.Expr
	Val  expr.Expr
	Expr expr.Expr
	Body Stmt // always *BlockStmt
}

type Return struct {
	Exprs []expr.Expr
}

type Simple struct {
	Expr expr.Expr
}

func (s Assign) stmt() {}
func (s Block) stmt()  {}
func (s If) stmt()     {}
func (s For) stmt()    {}
func (s Range) stmt()  {}
func (s Return) stmt() {}
func (s Simple) stmt() {}

func (s *Block) Sexp() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "(block")
	for _, s := range s.Stmts {
		buf.WriteRune(' ')
		buf.WriteString(stmtSexp(s))
	}
	fmt.Fprintf(buf, ")")
	return buf.String()
}
func (e *Return) Sexp() string { return fmt.Sprintf("(return %s)", exprsStr(e.Exprs)) }
func (e *Assign) Sexp() string {
	decl := ""
	if e.Decl {
		decl = " decl"
	}
	return fmt.Sprintf("(assign%s (%s) (%s))", decl, exprsStr(e.Left), exprsStr(e.Right))
}
func (e *If) Sexp() string {
	return fmt.Sprintf("(if %s %s %s %s)", stmtSexp(e.Init), exprSexp(e.Cond), stmtSexp(e.Body), stmtSexp(e.Else))
}
func (e *For) Sexp() string {
	return fmt.Sprintf("(for %s %s %s %s)", stmtSexp(e.Init), exprSexp(e.Cond), stmtSexp(e.Post), stmtSexp(e.Body))
}
func (e *Range) Sexp() string {
	return fmt.Sprintf("(range %s %s %s %s)", exprSexp(e.Key), exprSexp(e.Val), exprSexp(e.Expr), stmtSexp(e.Body))
}
func (e *Simple) Sexp() string { return fmt.Sprintf("(simple %s)", exprSexp(e.Expr)) }

func stmtSexp(s Stmt) string {
	if s == nil {
		return "nilstmt"
	}
	return s.Sexp()
}

func exprSexp(e expr.Expr) string {
	if e == nil {
		return "nilexpr"
	}
	return e.Sexp()
}

func exprsStr(e []expr.Expr) string {
	buf := new(bytes.Buffer)
	for i, arg := range e {
		if i > 0 {
			buf.WriteRune(' ')
		}
		buf.WriteString(exprSexp(arg))
	}
	return buf.String()
}
