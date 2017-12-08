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
	stmt()
	Pos() src.Pos // implements syntax.Node
}

type Import struct {
	Position src.Pos
	Name     string
	Path     string
}

type ImportSet struct {
	Position src.Pos
	Imports  []*Import
}

type TypeDecl struct {
	Position src.Pos
	Name     string
	Type     *tipe.Named
}

type MethodikDecl struct {
	Position src.Pos
	Name     string
	Type     *tipe.Named
	Methods  []*expr.FuncLiteral
}

// TODO InterfaceLiteral struct { Name string, MethodNames []string, Methods []*tipe.Func }

type Const struct {
	Position src.Pos
	NameList []string
	Type     tipe.Type
	Values   []expr.Expr
}

type ConstSet struct {
	Position src.Pos
	Consts   []*Const
}

type VarSet struct {
	Position src.Pos
	Vars     []*Var
}

type Var struct {
	Position src.Pos
	NameList []string
	Type     tipe.Type
	Values   []expr.Expr
}

type Assign struct {
	Position src.Pos
	Decl     bool
	Left     []expr.Expr
	Right    []expr.Expr // TODO: give up on multiple rhs values for now.
}

type Block struct {
	Position src.Pos
	Stmts    []Stmt
}

type If struct {
	Position src.Pos
	Init     Stmt
	Cond     expr.Expr
	Body     Stmt // always *BlockStmt
	Else     Stmt
}

type For struct {
	Position src.Pos
	Init     Stmt
	Cond     expr.Expr
	Post     Stmt
	Body     Stmt // always *BlockStmt
}

type Switch struct {
	Position src.Pos
	Init     Stmt
	Cond     expr.Expr
	Cases    []SwitchCase
}

type SwitchCase struct {
	Position src.Pos
	Conds    []expr.Expr
	Default  bool
	Body     *Block
}

type TypeSwitch struct {
	Position src.Pos
	Init     Stmt // initialization statement; or nil
	Assign   Stmt // x := y.(type) or y.(type)
	Cases    []TypeSwitchCase
}

type TypeSwitchCase struct {
	Position src.Pos
	Default  bool
	Types    []tipe.Type
	Body     *Block
}

type Go struct {
	Position src.Pos
	Call     *expr.Call
}

type Range struct {
	Position src.Pos
	Decl     bool
	Key      expr.Expr
	Val      expr.Expr
	Expr     expr.Expr
	Body     Stmt // always *BlockStmt
}

type Return struct {
	Position src.Pos
	Exprs    []expr.Expr
}

type Defer struct {
	Position src.Pos
	Expr     expr.Expr
}

type Simple struct {
	Position src.Pos
	Expr     expr.Expr
}

// Send is channel send statement, "a <- b".
type Send struct {
	Position src.Pos
	Chan     expr.Expr
	Value    expr.Expr
}

type Branch struct {
	Position src.Pos
	Type     token.Token // Continue, Break, Goto, or Fallthrough
	Label    string
}

type Labeled struct {
	Position src.Pos
	Label    string
	Stmt     Stmt
}

type Select struct {
	Position src.Pos
	Cases    []SelectCase
}

type SelectCase struct {
	Position src.Pos
	Default  bool
	Stmt     Stmt // a recv- or send-stmt
	Body     *Block
}

type Bad struct {
	Position src.Pos
	Error    error
}

func (s *Import) stmt()         {}
func (s *ImportSet) stmt()      {}
func (s *TypeDecl) stmt()       {}
func (s *MethodikDecl) stmt()   {}
func (s *Const) stmt()          {}
func (s *ConstSet) stmt()       {}
func (s *Var) stmt()            {}
func (s *VarSet) stmt()         {}
func (s *Assign) stmt()         {}
func (s *Block) stmt()          {}
func (s *If) stmt()             {}
func (s *For) stmt()            {}
func (s *Switch) stmt()         {}
func (s *SwitchCase) stmt()     {}
func (s *TypeSwitch) stmt()     {}
func (s *TypeSwitchCase) stmt() {}
func (s *Go) stmt()             {}
func (s *Range) stmt()          {}
func (s *Return) stmt()         {}
func (s *Defer) stmt()          {}
func (s *Simple) stmt()         {}
func (s *Send) stmt()           {}
func (s *Branch) stmt()         {}
func (s *Labeled) stmt()        {}
func (s *Select) stmt()         {}
func (s *Bad) stmt()            {}

func (s *Import) Pos() src.Pos        { return s.Position }
func (s *ImportSet) Pos() src.Pos     { return s.Position }
func (s *TypeDecl) Pos() src.Pos      { return s.Position }
func (s *MethodikDecl) Pos() src.Pos  { return s.Position }
func (s *Const) Pos() src.Pos         { return s.Position }
func (s *ConstSet) Pos() src.Pos      { return s.Position }
func (s *Var) Pos() src.Pos           { return s.Position }
func (s *VarSet) Pos() src.Pos        { return s.Position }
func (s *Assign) Pos() src.Pos        { return s.Position }
func (s *Block) Pos() src.Pos         { return s.Position }
func (s *If) Pos() src.Pos            { return s.Position }
func (s *For) Pos() src.Pos           { return s.Position }
func (s *Switch) Pos() src.Pos        { return s.Position }
func (s SwitchCase) Pos() src.Pos     { return s.Position }
func (s *TypeSwitch) Pos() src.Pos    { return s.Position }
func (s TypeSwitchCase) Pos() src.Pos { return s.Position }
func (s *Go) Pos() src.Pos            { return s.Position }
func (s *Range) Pos() src.Pos         { return s.Position }
func (s *Return) Pos() src.Pos        { return s.Position }
func (s *Defer) Pos() src.Pos         { return s.Position }
func (s *Simple) Pos() src.Pos        { return s.Position }
func (s *Send) Pos() src.Pos          { return s.Position }
func (s *Branch) Pos() src.Pos        { return s.Position }
func (s *Labeled) Pos() src.Pos       { return s.Position }
func (s *Select) Pos() src.Pos        { return s.Position }
func (s SelectCase) Pos() src.Pos     { return s.Position }
func (s *Bad) Pos() src.Pos           { return s.Position }
