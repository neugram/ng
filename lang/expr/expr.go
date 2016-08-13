// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package expr defines data structures representing Neugram expressions.
package expr

import (
	"bytes"
	"fmt"
	"strings"

	"neugram.io/lang/tipe"
	"neugram.io/lang/token"
)

type Expr interface {
	Sexp() string
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
	Left Expr
	Low  Expr
	High Expr
	Max  Expr
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
	Func Expr
	Args []Expr
}

type Range struct {
	Start Expr
	End   Expr
	Exact Expr
}

type Index struct {
	Expr  Expr
	Index Expr
}

type TableIndex struct {
	Expr     Expr
	ColNames []string
	Cols     Range
	Rows     Range
}

type ShellSeg int

const (
	SegmentSemi ShellSeg = iota // ;
	SegmentPipe                 // |
	SegmentAnd                  // &&
	SegmentOut                  // >
	SegmentIn                   // <
	// TODO SegmentOr for ||?
	// TODO must we support the awful monster 2>&1, or leave outside $$?
	// TODO what about the less painful &>?
)

// x ; y; z   -- sequential execution
// x && z     -- sequential execution conditional on success
// x | y | z  -- pipeline
// x < f      -- f (denoted by Redirect) isstdin to x
// x > f      -- run x, stdout to Redirect
type ShellList struct {
	Segment  ShellSeg
	Redirect string
	List     []Expr // ShellList or ShellCmd
}

type ShellCmd struct {
	Argv []string
}

type Shell struct {
	Cmds []*ShellList
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
	_ = Expr((*TableIndex)(nil))
	_ = Expr((*ShellList)(nil))
	_ = Expr((*ShellCmd)(nil))
	_ = Expr((*Shell)(nil))
)

func (e *Binary) expr()       {}
func (e *Unary) expr()        {}
func (e *Bad) expr()          {}
func (e *Selector) expr()     {}
func (e *Slice) expr()        {}
func (e *BasicLiteral) expr() {}
func (e *FuncLiteral) expr()  {}
func (e *CompLiteral) expr()  {}
func (e *MapLiteral) expr()   {}
func (e *SliceLiteral) expr() {}
func (e *TableLiteral) expr() {}
func (e *Type) expr()         {}
func (e *Ident) expr()        {}
func (e *Call) expr()         {}
func (e *Index) expr()        {}
func (e *TableIndex) expr()   {}
func (e *ShellList) expr()    {}
func (e *ShellCmd) expr()     {}
func (e *Shell) expr()        {}

func (e *Binary) Sexp() string {
	if e == nil {
		return "nilbin"
	}
	return fmt.Sprintf("(%s %s %s)", e.Op, exprSexp(e.Left), exprSexp(e.Right))
}
func (e *Unary) Sexp() string {
	if e == nil {
		return "nilunary"
	}
	return fmt.Sprintf("(%s %s)", e.Op, exprSexp(e.Expr))
}
func (e *Bad) Sexp() string { return fmt.Sprintf("(bad %v)", e.Error) }
func (e *Selector) Sexp() string {
	if e == nil {
		return "nilsel"
	}
	return fmt.Sprintf("(sel %s %s)", exprSexp(e.Left), exprSexp(e.Right))
}
func (e *Slice) Sexp() string {
	if e == nil {
		return "nilsel"
	}
	return fmt.Sprintf("(slice %s %s %s %s)", exprSexp(e.Left), exprSexp(e.Low), exprSexp(e.High), exprSexp(e.Max))
}
func (e *BasicLiteral) Sexp() string {
	if e == nil {
		return "nillit"
	}
	return fmt.Sprintf("(lit %T %q)", e.Value, e.Value)
}
func (e *Ident) Sexp() string {
	if e == nil {
		return "nilident"
	}
	return fmt.Sprintf("%s", e.Name)
}
func (e *Call) Sexp() string {
	if e == nil {
		return "nilcall"
	}
	return fmt.Sprintf("(call %s %s)", exprSexp(e.Func), exprsStr(e.Args))
}

func (e *FuncLiteral) Sexp() string {
	body := "nilbody"
	if e.Body != nil {
		b, ok := e.Body.(interface {
			Sexp() string
		})
		if ok {
			body = b.Sexp()
		} else {
			body = fmt.Sprintf("badbody:%T", e.Body)
		}
	}
	if e.ReceiverName != "" {
		pointer := ""
		if e.PointerReceiver {
			pointer = "*"
		}
		return fmt.Sprintf("(method (%s%s) %s %s %s)", pointer, e.ReceiverName, e.Name, tipeSexp(e.Type), body)
	} else {
		return fmt.Sprintf("(func %s %s %s)", e.Name, tipeSexp(e.Type), body)
	}
}

func (e *CompLiteral) Sexp() string {
	return fmt.Sprintf("(comp %s %s %s)", tipeSexp(e.Type), exprsStr(e.Keys), exprsStr(e.Elements))
}
func (e *MapLiteral) Sexp() string {
	return fmt.Sprintf("(map %s %s %s)", tipeSexp(e.Type), exprsStr(e.Keys), exprsStr(e.Values))
}
func (e *SliceLiteral) Sexp() string {
	return fmt.Sprintf("(slice %s %s%s)", tipeSexp(e.Type), exprsStr(e.Elems))
}
func (e *TableLiteral) Sexp() string {
	rows := ""
	for _, row := range e.Rows {
		rows += " " + exprsStr(row)
	}
	if rows != "" {
		rows = " (" + rows[1:] + ")"
	}
	return fmt.Sprintf("(table %s %s%s)", tipeSexp(e.Type), exprsStr(e.ColNames), rows)
}
func (e *Type) Sexp() string {
	return fmt.Sprintf("(typeexpr %s)", tipeSexp(e.Type))
}
func (e *Index) Sexp() string {
	return fmt.Sprintf("(index %s %s", exprSexp(e.Expr), exprSexp(e.Index))
}
func (e *TableIndex) Sexp() string {
	names := strings.Join(e.ColNames, `"|"`)
	if names != "" {
		names = ` "` + names + `"`
	}
	rangeSexp := func(r Range) string {
		rs := ""
		if r.Start != nil || r.End != nil {
			if r.Start != nil {
				rs += exprSexp(r.Start)
			}
			rs += ":"
			if r.End != nil {
				rs += exprSexp(r.End)
			}
		}
		exact := ""
		if r.Exact != nil {
			if rs != "" {
				rs += " "
			}
			exact = exprSexp(r.Exact)
		}
		return fmt.Sprintf("(%s%s)", rs, exact)
	}
	return fmt.Sprintf("(tableindex %s%s %s %s", exprSexp(e.Expr), names, rangeSexp(e.Cols), rangeSexp(e.Rows))
}

func (e *ShellList) Sexp() string {
	if e == nil {
		return "(nilshelllist)"
	}
	seg := "unknownsegment"
	switch e.Segment {
	case SegmentPipe:
		seg = "|"
	case SegmentAnd:
		seg = "&&"
	case SegmentSemi:
		seg = ";"
	case SegmentOut:
		seg = ">"
	case SegmentIn:
		seg = "<"
	}
	r := ""
	if e.Redirect != "" {
		r = "(redirect " + e.Redirect + ")"
	}
	return fmt.Sprintf("(shelllist %q %s%s)", seg, exprsStr(e.List), r)
}

func (e *ShellCmd) Sexp() string {
	return fmt.Sprintf("(shellcmd %s)", strings.Join(e.Argv, " "))
}

func (e *Shell) Sexp() string {
	cmds := ""
	for _, cmd := range e.Cmds {
		cmds += " " + exprSexp(cmd)
	}
	return fmt.Sprintf("(shell%s)", cmds)
}

func tipeSexp(t tipe.Type) string {
	if t == nil {
		return "niltype"
	}
	return t.Sexp()
}

func exprSexp(e Expr) string {
	if e == nil {
		return "nilexpr"
	}
	return e.Sexp()
}

func exprsStr(e []Expr) string {
	buf := new(bytes.Buffer)
	for i, arg := range e {
		if i > 0 {
			buf.WriteRune(' ')
		}
		buf.WriteString(exprSexp(arg))
	}
	return buf.String()
}
