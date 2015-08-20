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

type Return struct {
	Exprs []expr.Expr
}

type Simple struct {
	Expr expr.Expr
}

func (s Assign) stmt() {}
func (s Block) stmt()  {}
func (s If) stmt()     {}
func (s Return) stmt() {}
func (s Simple) stmt() {}

func (e *Block) Sexp() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "(block")
	for _, s := range e.Stmts {
		fmt.Fprintf(buf, " %s", s.Sexp())
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
	return fmt.Sprintf("(if %s %s %s %s)", e.Init.Sexp(), e.Cond.Sexp(), e.Body.Sexp(), e.Else.Sexp())
}
func (e *Simple) Sexp() string { return fmt.Sprintf("(simple %s)", e.Expr.Sexp()) }

func exprsStr(e []expr.Expr) string {
	buf := new(bytes.Buffer)
	for i, arg := range e {
		if i > 0 {
			buf.WriteRune(' ')
		}
		buf.WriteString(arg.Sexp())
	}
	return buf.String()
}
