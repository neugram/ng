// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package expr defines data structures representing Neugram expressions.
package expr

import (
	"neugram.io/ng/syntax/src"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
)

type Expr interface {
	expr()
	Pos() src.Pos // implements syntax.Node
}

type Binary struct {
	Position src.Pos
	Op       token.Token // Add, Sub, Mul, Div, Rem, Pow, And, Or, Equal, NotEqual, Less, Greater
	Left     Expr
	Right    Expr
}

type Unary struct {
	Position src.Pos
	Op       token.Token // Not, Mul (deref), Ref, LeftParen, Range
	Expr     Expr
}

type Bad struct {
	Position src.Pos
	Error    error
}

type Selector struct {
	Position src.Pos
	Left     Expr
	Right    *Ident
}

type Slice struct {
	Position src.Pos
	Low      Expr
	High     Expr
	Max      Expr
}

type Index struct {
	Position src.Pos
	Left     Expr
	Indicies []Expr
}

type TypeAssert struct {
	Position src.Pos
	Left     Expr
	Type     tipe.Type // asserted type; nil means type switch X.(type)
}

type BasicLiteral struct {
	Position src.Pos
	Value    interface{} // string, *big.Int, *big.Float
}

type FuncLiteral struct {
	Position        src.Pos
	Name            string // may be empty
	ReceiverName    string // if non-empty, this is a method
	PointerReceiver bool
	Type            *tipe.Func
	ParamNames      []string
	ResultNames     []string
	Body            interface{} // *stmt.Block, breaking the package import cycle
}

type CompLiteral struct {
	Position src.Pos
	Type     tipe.Type
	Keys     []Expr // TODO: could make this []string
	Values   []Expr
}

type MapLiteral struct {
	Position src.Pos
	Type     tipe.Type
	Keys     []Expr
	Values   []Expr
}

type ArrayLiteral struct {
	Position src.Pos
	Type     *tipe.Array
	Keys     []Expr // TODO: could make this []int
	Values   []Expr
}

type SliceLiteral struct {
	Position src.Pos
	Type     *tipe.Slice
	Keys     []Expr // TODO: could make this []int
	Values   []Expr
}

type TableLiteral struct {
	Position src.Pos
	Type     *tipe.Table
	ColNames []Expr
	Rows     [][]Expr
}

// Type is not a typical Neugram expression. It is used only for when
// types are passed as arguments to the builtin functions new and make.
type Type struct {
	Position src.Pos
	Type     tipe.Type
}

type Ident struct {
	Position src.Pos
	Name     string
	// Type tipe.Type
}

type Call struct {
	Position   src.Pos
	Func       Expr
	Args       []Expr
	Ellipsis   bool // last argument expands, e.g. f(x...)
	ElideError bool
}

type Range struct {
	Position src.Pos
	Start    Expr
	End      Expr
	Exact    Expr
}

type ShellList struct {
	Position src.Pos
	AndOr    []*ShellAndOr
}

type ShellAndOr struct {
	Position   src.Pos
	Pipeline   []*ShellPipeline
	Sep        []token.Token // '&&' or '||'. len(Sep) == len(Pipeline)-1
	Background bool
}

type ShellPipeline struct {
	Position src.Pos
	Bang     bool
	Cmd      []*ShellCmd // Cmd[0] | Cmd[1] | ...
}

type ShellCmd struct {
	Position  src.Pos
	SimpleCmd *ShellSimpleCmd // or:
	Subshell  *ShellList
}

type ShellSimpleCmd struct {
	Position src.Pos
	Redirect []*ShellRedirect
	Assign   []ShellAssign
	Args     []string
}

type ShellRedirect struct {
	Position src.Pos
	Number   *int
	Token    token.Token // '<', '<&', '>', '>&', '>>'
	Filename string
}

type ShellAssign struct {
	Position src.Pos
	Key      string
	Value    string
}

type Shell struct {
	Position   src.Pos
	Cmds       []*ShellList
	TrapOut    bool // override os.Stdout, outer language collect it
	DropOut    bool // send stdout to /dev/null (just an optimization)
	ElideError bool

	// FreeVars is a list of $-parameters referred to in this
	// shell expression that are declared statically in the
	// scope of the expression. Not all parameters have to be
	// declared statically in the scope, as they may be
	// referring to run time environment variables.
	FreeVars []string

	// TODO: Shell object for err := $$(stdin, stdout, stderr) cmd $$
}

func (e *Binary) expr()         {}
func (e *Unary) expr()          {}
func (e *Bad) expr()            {}
func (e *Selector) expr()       {}
func (e *Slice) expr()          {}
func (e *BasicLiteral) expr()   {}
func (e *FuncLiteral) expr()    {}
func (e *CompLiteral) expr()    {}
func (e *MapLiteral) expr()     {}
func (e *ArrayLiteral) expr()   {}
func (e *SliceLiteral) expr()   {}
func (e *TableLiteral) expr()   {}
func (e *Type) expr()           {}
func (e *Ident) expr()          {}
func (e *Call) expr()           {}
func (e *Index) expr()          {}
func (e *TypeAssert) expr()     {}
func (e *ShellList) expr()      {}
func (e *ShellAndOr) expr()     {}
func (e *ShellPipeline) expr()  {}
func (e *ShellSimpleCmd) expr() {}
func (e *ShellRedirect) expr()  {}
func (e *ShellAssign) expr()    {}
func (e *ShellCmd) expr()       {}
func (e *Shell) expr()          {}

func (e *Binary) Pos() src.Pos         { return e.Position }
func (e *Unary) Pos() src.Pos          { return e.Position }
func (e *Bad) Pos() src.Pos            { return e.Position }
func (e *Selector) Pos() src.Pos       { return e.Position }
func (e *Slice) Pos() src.Pos          { return e.Position }
func (e *BasicLiteral) Pos() src.Pos   { return e.Position }
func (e *FuncLiteral) Pos() src.Pos    { return e.Position }
func (e *CompLiteral) Pos() src.Pos    { return e.Position }
func (e *MapLiteral) Pos() src.Pos     { return e.Position }
func (e *ArrayLiteral) Pos() src.Pos   { return e.Position }
func (e *SliceLiteral) Pos() src.Pos   { return e.Position }
func (e *TableLiteral) Pos() src.Pos   { return e.Position }
func (e *Type) Pos() src.Pos           { return e.Position }
func (e *Ident) Pos() src.Pos          { return e.Position }
func (e *Call) Pos() src.Pos           { return e.Position }
func (e *Range) Pos() src.Pos          { return e.Position }
func (e *Index) Pos() src.Pos          { return e.Position }
func (e *TypeAssert) Pos() src.Pos     { return e.Position }
func (e *ShellList) Pos() src.Pos      { return e.Position }
func (e *ShellAndOr) Pos() src.Pos     { return e.Position }
func (e *ShellPipeline) Pos() src.Pos  { return e.Position }
func (e *ShellSimpleCmd) Pos() src.Pos { return e.Position }
func (e *ShellRedirect) Pos() src.Pos  { return e.Position }
func (e ShellAssign) Pos() src.Pos     { return e.Position }
func (e *ShellCmd) Pos() src.Pos       { return e.Position }
func (e *Shell) Pos() src.Pos          { return e.Position }
