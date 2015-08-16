// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"bytes"
	"fmt"
)

type Expr interface {
}

type BinaryExpr struct {
	Op    Token // Add, Sub, Mul, Div, Rem, Pow, And, Or, Equal, NotEqual, Less, Greater
	Left  Expr
	Right Expr
}

type UnaryExpr struct {
	Op   Token // Not, Mul (deref), Ref, LeftParen
	Expr Expr
}

type BadExpr struct {
	Error error
}

type SelectorExpr struct {
	Left  *Ident
	Right *Ident
}

type BasicLiteral struct {
	Value interface{} // string, *big.Int, *big.Float
}

type Field struct {
	Name *Ident
	Type Expr
}

type FuncType struct {
	In  []*Field
	Out []*Field
}

type FuncLiteral struct {
	Type *FuncType
	Body []Stmt
}

type Ident struct {
	Name string
}

type CallExpr struct {
	Func Expr
	Args []Expr
}

type Stmt interface {
}

type AssignStmt struct {
	Left  []Expr
	Right []Expr
}

type ReturnStmt struct {
	Exprs []Expr
}

func (e *BinaryExpr) String() string   { return fmt.Sprintf("(%s %s %s)", e.Op, e.Left, e.Right) }
func (e *UnaryExpr) String() string    { return fmt.Sprintf("(%s %s)", e.Op, e.Expr) }
func (e *BadExpr) String() string      { return fmt.Sprintf("(BAD %v)", e.Error) }
func (e *BasicLiteral) String() string { return fmt.Sprintf("(%s %T)", e.Value, e.Value) }
func (e *Ident) String() string        { return fmt.Sprintf("%s", e.Name) }
func (e *CallExpr) String() string     { return fmt.Sprintf("(call %s %s)", e.Func, exprsStr(e.Args)) }
func (e *ReturnStmt) String() string   { return fmt.Sprintf("(return %s", exprsStr(e.Exprs)) }
func (e *AssignStmt) String() string {
	return fmt.Sprintf("(assign (%s) (%s))", exprsStr(e.Left), exprsStr(e.Right))
}
func (e *FuncLiteral) String() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "(func %s (", e.Type)
	for i, s := range e.Body {
		if i > 0 {
			buf.WriteRune(' ')
		}
		fmt.Fprintf(buf, "%s", s)
	}
	fmt.Fprintf(buf, "))")
	return buf.String()
}

func (e *FuncType) String() string {
	return fmt.Sprintf("((in %s) (out %s))", fieldsStr(e.In), fieldsStr(e.Out))
}

func fieldsStr(fields []*Field) string {
	buf := new(bytes.Buffer)
	for i, f := range fields {
		if i > 0 {
			buf.WriteRune(' ')
		}
		fmt.Fprintf(buf, "(%s %s)", f.Name, f.Type)
	}
	return buf.String()
}

func exprsStr(e []Expr) string {
	buf := new(bytes.Buffer)
	for i, arg := range e {
		if i > 0 {
			buf.WriteRune(' ')
		}
		fmt.Fprintf(buf, "%s", arg)
	}
	return buf.String()
}
