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
	exprfn()
	Pos() src.Pos // implements syntax.Node
}

type Binary struct {
	expr
	Op    token.Token // Add, Sub, Mul, Div, Rem, Pow, And, Or, Equal, NotEqual, Less, Greater
	Left  Expr
	Right Expr
}

type Unary struct {
	expr
	Op   token.Token // Not, Mul (deref), Ref, LeftParen, Range
	Expr Expr
}

type Bad struct {
	expr
	Error error
}

type Selector struct {
	expr
	Left  Expr
	Right *Ident
}

type Slice struct {
	expr
	Low  Expr
	High Expr
	Max  Expr
}

type Index struct {
	expr
	Left     Expr
	Indicies []Expr
}

type TypeAssert struct {
	expr
	Left Expr
	Type tipe.Type // asserted type; nil means type switch X.(type)
}

type BasicLiteral struct {
	expr
	Value interface{} // string, *big.Int, *big.Float
}

type FuncLiteral struct {
	expr
	Name            string // may be empty
	ReceiverName    string // if non-empty, this is a method
	PointerReceiver bool
	Type            *tipe.Func
	ParamNames      []string
	ResultNames     []string
	Body            interface{} // *stmt.Block, breaking the package import cycle
}

type CompLiteral struct {
	expr
	Type     tipe.Type
	Keys     []Expr // TODO: could make this []string
	Elements []Expr
}

type MapLiteral struct {
	expr
	Type   tipe.Type
	Keys   []Expr
	Values []Expr
}

type SliceLiteral struct {
	expr
	Type  *tipe.Slice
	Elems []Expr
}

type TableLiteral struct {
	expr
	Type     *tipe.Table
	ColNames []Expr
	Rows     [][]Expr
}

// Type is not a typical Neugram expression. It is used only for when
// types are passed as arguments to the builtin functions new and make.
type Type struct {
	expr
	Type tipe.Type
}

type Ident struct {
	expr
	Name string
	// Type tipe.Type
}

type Call struct {
	expr
	Func       Expr
	Args       []Expr
	Ellipsis   bool // last argument expands, e.g. f(x...)
	ElideError bool
}

type Range struct {
	expr
	Start Expr
	End   Expr
	Exact Expr
}

type ShellList struct {
	expr
	AndOr []*ShellAndOr
}

type ShellAndOr struct {
	expr
	Pipeline   []*ShellPipeline
	Sep        []token.Token // '&&' or '||'. len(Sep) == len(Pipeline)-1
	Background bool
}

type ShellPipeline struct {
	expr
	Bang bool
	Cmd  []*ShellCmd // Cmd[0] | Cmd[1] | ...
}

type ShellCmd struct {
	expr
	SimpleCmd *ShellSimpleCmd // or:
	Subshell  *ShellList
}

type ShellSimpleCmd struct {
	expr
	Redirect []*ShellRedirect
	Assign   []ShellAssign
	Args     []string
}

type ShellRedirect struct {
	expr
	Number   *int
	Token    token.Token // '<', '<&', '>', '>&', '>>'
	Filename string
}

type ShellAssign struct {
	expr
	Key   string
	Value string
}

type Shell struct {
	expr
	Cmds       []*ShellList
	TrapOut    bool // override os.Stdout, outer language collect it
	DropOut    bool // send stdout to /dev/null (just an optimization)
	ElideError bool
	// TODO: Shell object for err := $$(stdin, stdout, stderr) cmd $$
}

type expr struct {
	Position src.Pos
}

func (e expr) Pos() src.Pos { return e.Position }
func (expr) exprfn()        {}
