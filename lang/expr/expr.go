// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package expr defines data structures representing Numengrad expressions.
package expr

import (
	"bytes"
	"fmt"

	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
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
	Op   token.Token // Not, Mul (deref), Ref, LeftParen
	Expr Expr
}

type Bad struct {
	Error error
}

type Selector struct {
	Left  *Ident
	Right *Ident
}

type BasicLiteral struct {
	Value interface{} // string, *big.Int, *big.Float
}

type FuncLiteral struct {
	Type *tipe.Func
	Body interface{} // *stmt.Block, breaking the package import cycle
}

type Ident struct {
	Name string
	// Type tipe.Type
}

type Call struct {
	Func Expr
	Args []Expr
}

var (
	_ = Expr((*Binary)(nil))
	_ = Expr((*Unary)(nil))
	_ = Expr((*Bad)(nil))
	_ = Expr((*Selector)(nil))
	_ = Expr((*BasicLiteral)(nil))
	_ = Expr((*FuncLiteral)(nil))
	_ = Expr((*Ident)(nil))
	_ = Expr((*Call)(nil))
)

func (e *Binary) expr()       {}
func (e *Unary) expr()        {}
func (e *Bad) expr()          {}
func (e *Selector) expr()     {}
func (e *BasicLiteral) expr() {}
func (e *FuncLiteral) expr()  {}
func (e *Ident) expr()        {}
func (e *Call) expr()         {}

func (e *Binary) Sexp() string {
	return fmt.Sprintf("(%s %s %s)", e.Op, e.Left.Sexp(), e.Right.Sexp())
}
func (e *Unary) Sexp() string        { return fmt.Sprintf("(%s %s)", e.Op, e.Expr.Sexp()) }
func (e *Bad) Sexp() string          { return fmt.Sprintf("(bad %v)", e.Error) }
func (e *Selector) Sexp() string     { return fmt.Sprintf("(sel %s %s)", e.Left.Sexp(), e.Right.Sexp()) }
func (e *BasicLiteral) Sexp() string { return fmt.Sprintf("(lit %T %s)", e.Value, e.Value) }
func (e *Ident) Sexp() string        { return fmt.Sprintf("%s", e.Name) }
func (e *Call) Sexp() string         { return fmt.Sprintf("(call %s %s)", e.Func.Sexp(), exprsStr(e.Args)) }
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
	return fmt.Sprintf("(func %s %s)", e.Type.Sexp(), body)
}

func exprsStr(e []Expr) string {
	buf := new(bytes.Buffer)
	for i, arg := range e {
		if i > 0 {
			buf.WriteRune(' ')
		}
		buf.WriteString(arg.Sexp())
	}
	return buf.String()
}
