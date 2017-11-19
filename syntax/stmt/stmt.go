// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stmt defines data structures representing Neugram statements.
package stmt

import (
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/src"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
)

type Stmt interface {
	stmtfn()
	Pos() src.Pos // implements syntax.Node
}

type Import struct {
	stmt
	Name string
	Path string
}

type ImportSet struct {
	stmt
	Imports []*Import
}

type TypeDecl struct {
	stmt
	Name string
	Type tipe.Type
}

type MethodikDecl struct {
	stmt
	Name    string
	Type    *tipe.Methodik
	Methods []*expr.FuncLiteral
}

// TODO InterfaceLiteral struct { Name string, MethodNames []string, Methods []*tipe.Func }

type Const struct {
	stmt
	Name  string
	Type  tipe.Type
	Value expr.Expr
}

type Assign struct {
	stmt
	Decl  bool
	Left  []expr.Expr
	Right []expr.Expr // TODO: give up on multiple rhs values for now.
}

type Block struct {
	stmt
	Stmts []Stmt
}

type If struct {
	stmt
	Init Stmt
	Cond expr.Expr
	Body Stmt // always *BlockStmt
	Else Stmt
}

type For struct {
	stmt
	Init Stmt
	Cond expr.Expr
	Post Stmt
	Body Stmt // always *BlockStmt
}

type Switch struct {
	stmt
	Init  Stmt
	Cond  expr.Expr
	Cases []SwitchCase
}

type SwitchCase struct {
	stmt
	Conds   []expr.Expr
	Default bool
	Body    *Block
}

type TypeSwitch struct {
	stmt
	Init   Stmt // initialization statement; or nil
	Assign Stmt // x := y.(type) or y.(type)
	Cases  []TypeSwitchCase
}

type TypeSwitchCase struct {
	stmt
	Default bool
	Types   []tipe.Type
	Body    *Block
}

type Go struct {
	stmt
	Call *expr.Call
}

type Range struct {
	stmt
	Decl bool
	Key  expr.Expr
	Val  expr.Expr
	Expr expr.Expr
	Body Stmt // always *BlockStmt
}

type Return struct {
	stmt
	Exprs []expr.Expr
}

type Simple struct {
	stmt
	Expr expr.Expr
}

// Send is channel send statement, "a <- b".
type Send struct {
	stmt
	Chan  expr.Expr
	Value expr.Expr
}

type Branch struct {
	stmt
	Type  token.Token // Continue, Break, Goto, or Fallthrough
	Label string
}

type Labeled struct {
	stmt
	Label string
	Stmt  Stmt
}

type Select struct {
	stmt
	Cases []SelectCase
}

type SelectCase struct {
	stmt
	Default bool
	Stmt    Stmt // a recv- or send-stmt
	Body    *Block
}

type Bad struct {
	stmt
}

type stmt struct {
	Position src.Pos
}

func (s stmt) Pos() src.Pos { return s.Position }
func (stmt) stmtfn()        {}
