// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stmt defines data structures representing Neugram statements.
package stmt

import (
	"neugram.io/ng/expr"
	"neugram.io/ng/tipe"
	"neugram.io/ng/token"
)

type Stmt interface {
	stmt()
}

type Import struct {
	Name string
	Path string
}

type ImportSet struct {
	Imports []*Import
}

type TypeDecl struct {
	Name string
	Type tipe.Type
}

type MethodikDecl struct {
	Name    string
	Type    *tipe.Methodik
	Methods []*expr.FuncLiteral
}

// TODO InterfaceLiteral struct { Name string, MethodNames []string, Methods []*tipe.Func }

type Const struct {
	Name  string
	Type  tipe.Type
	Value expr.Expr
}

type Assign struct {
	Decl  bool
	Left  []expr.Expr
	Right []expr.Expr // TODO: give up on multiple rhs values for now.
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

type Switch struct {
	Init  Stmt
	Cond  expr.Expr
	Cases []SwitchCase
}

type SwitchCase struct {
	Conds   []expr.Expr
	Default bool
	Body    *Block
}

type Go struct {
	Call *expr.Call
}

type Range struct {
	Decl bool
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

// Send is channel send statement, "a <- b".
type Send struct {
	Chan  expr.Expr
	Value expr.Expr
}

type Branch struct {
	Type  token.Token // Continue, Break, Goto, or Fallthrough
	Label string
}

type Labeled struct {
	Label string
	Stmt  Stmt
}

type Select struct {
	Cases []SelectCase
}

type SelectCase struct {
	Default bool
	Stmt    Stmt // a recv- or send-stmt
	Body    *Block
}

type Bad struct {
}

func (s Import) stmt()       {}
func (s ImportSet) stmt()    {}
func (s TypeDecl) stmt()     {}
func (s MethodikDecl) stmt() {}
func (s Const) stmt()        {}
func (s Assign) stmt()       {}
func (s Block) stmt()        {}
func (s If) stmt()           {}
func (s For) stmt()          {}
func (s Switch) stmt()       {}
func (s SwitchCase) stmt()   {}
func (s Go) stmt()           {}
func (s Range) stmt()        {}
func (s Return) stmt()       {}
func (s Simple) stmt()       {}
func (s Send) stmt()         {}
func (s Branch) stmt()       {}
func (s Labeled) stmt()      {}
func (s Select) stmt()       {}
func (s Bad) stmt()          {}
