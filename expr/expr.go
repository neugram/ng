// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package expr defines data structures representing Neugram expressions.
package expr

import (
	"neugram.io/ng/tipe"
	"neugram.io/ng/token"
)

type Expr interface {
	expr()
}

type Binary struct {
	Op    token.Token // Add, Sub, Mul, Div, Rem, Pow, And, Or, Equal, NotEqual, Less, Greater
	Left  Expr
	Right Expr
}

type Unary struct {
	Op   token.Token // Not, Mul (deref), Ref, LeftParen, Range
	Expr Expr
}

type Bad struct {
	Error error
}

type Selector struct {
	Left  Expr
	Right *Ident
}

type Slice struct {
	Low  Expr
	High Expr
	Max  Expr
}

type Index struct {
	Left     Expr
	Indicies []Expr
}

type TypeAssert struct {
	Left Expr
	Type tipe.Type // asserted type; nil means type switch X.(type)
}

type BasicLiteral struct {
	Value interface{} // string, *big.Int, *big.Float
}

type FuncLiteral struct {
	Name            string // may be empty
	ReceiverName    string // if non-empty, this is a method
	PointerReceiver bool
	Type            *tipe.Func
	ParamNames      []string
	ResultNames     []string
	Body            interface{} // *stmt.Block, breaking the package import cycle
}

type CompLiteral struct {
	Type     tipe.Type
	Keys     []Expr // TODO: could make this []string
	Elements []Expr
}

type MapLiteral struct {
	Type   tipe.Type
	Keys   []Expr
	Values []Expr
}

type SliceLiteral struct {
	Type  *tipe.Slice
	Elems []Expr
}

type TableLiteral struct {
	Type     *tipe.Table
	ColNames []Expr
	Rows     [][]Expr
}

// Type is not a typical Neugram expression. It is used only for when
// types are passed as arguments to the builtin functions new and make.
type Type struct {
	Type tipe.Type
}

type Ident struct {
	Name string
	// Type tipe.Type
}

type Call struct {
	Func       Expr
	Args       []Expr
	Ellipsis   bool // last argument expands, e.g. f(x...)
	ElideError bool
}

type Range struct {
	Start Expr
	End   Expr
	Exact Expr
}

type ShellList struct {
	AndOr []*ShellAndOr
}

type ShellAndOr struct {
	Pipeline   []*ShellPipeline
	Sep        []token.Token // '&&' or '||'. len(Sep) == len(Pipeline)-1
	Background bool
}

type ShellPipeline struct {
	Bang bool
	Cmd  []*ShellCmd // Cmd[0] | Cmd[1] | ...
}

type ShellCmd struct {
	SimpleCmd *ShellSimpleCmd // or:
	Subshell  *ShellList
}

type ShellSimpleCmd struct {
	Redirect []*ShellRedirect
	Assign   []ShellAssign
	Args     []string
}

type ShellRedirect struct {
	Number   *int
	Token    token.Token // '<', '<&', '>', '>&', '>>'
	Filename string
}

type ShellAssign struct {
	Key   string
	Value string
}

type Shell struct {
	Cmds       []*ShellList
	TrapOut    bool // override os.Stdout, outer language collect it
	DropOut    bool // send stdout to /dev/null (just an optimization)
	ElideError bool
	// TODO: Shell object for err := $$(stdin, stdout, stderr) cmd $$
}

var (
	_ = Expr((*Binary)(nil))
	_ = Expr((*Unary)(nil))
	_ = Expr((*Bad)(nil))
	_ = Expr((*Selector)(nil))
	_ = Expr((*Slice)(nil))
	_ = Expr((*BasicLiteral)(nil))
	_ = Expr((*FuncLiteral)(nil))
	_ = Expr((*CompLiteral)(nil))
	_ = Expr((*MapLiteral)(nil))
	_ = Expr((*SliceLiteral)(nil))
	_ = Expr((*TableLiteral)(nil))
	_ = Expr((*Type)(nil))
	_ = Expr((*Ident)(nil))
	_ = Expr((*Call)(nil))
	_ = Expr((*Index)(nil))
	_ = Expr((*TypeAssert)(nil))
	_ = Expr((*ShellList)(nil))
	_ = Expr((*ShellAndOr)(nil))
	_ = Expr((*ShellPipeline)(nil))
	_ = Expr((*ShellSimpleCmd)(nil))
	_ = Expr((*ShellRedirect)(nil))
	_ = Expr((*ShellAssign)(nil))
	_ = Expr((*ShellCmd)(nil))
	_ = Expr((*Shell)(nil))
)

func (e *Binary) expr()         {}
func (e *Unary) expr()          {}
func (e *Bad) expr()            {}
func (e *Selector) expr()       {}
func (e *Slice) expr()          {}
func (e *BasicLiteral) expr()   {}
func (e *FuncLiteral) expr()    {}
func (e *CompLiteral) expr()    {}
func (e *MapLiteral) expr()     {}
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
