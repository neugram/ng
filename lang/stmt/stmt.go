// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stmt defines data structures representing Neugram statements.
package stmt

import (
	"neugram.io/lang/expr"
	"neugram.io/lang/tipe"
)

type Stmt interface {
	stmt()
}

type Import struct {
	Name string
	Path string
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

func (s Import) stmt()       {}
func (s TypeDecl) stmt()     {}
func (s MethodikDecl) stmt() {}
func (s Const) stmt()        {}
func (s Assign) stmt()       {}
func (s Block) stmt()        {}
func (s If) stmt()           {}
func (s For) stmt()          {}
func (s Go) stmt()           {}
func (s Range) stmt()        {}
func (s Return) stmt()       {}
func (s Simple) stmt()       {}
