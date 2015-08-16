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

type BasicLiteral struct {
	Value interface{} // string, *big.Int, *big.Float
}

type Ident struct {
	Name string
}

type CallExpr struct {
	Func Expr
	Args []Expr
}

func (e *BinaryExpr) String() string   { return fmt.Sprintf("(%s %s %s)", e.Op, e.Left, e.Right) }
func (e *UnaryExpr) String() string    { return fmt.Sprintf("(%s %s)", e.Op, e.Expr) }
func (e *BadExpr) String() string      { return fmt.Sprintf("(BAD %v)", e.Error) }
func (e *BasicLiteral) String() string { return fmt.Sprintf("(%s %T)", e.Value, e.Value) }
func (e *Ident) String() string        { return fmt.Sprintf("%s", e.Name) }
func (e *CallExpr) String() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "(call %s", e.Func)
	for _, arg := range e.Args {
		fmt.Fprintf(buf, " %s", arg)
	}
	buf.WriteRune(')')
	return buf.String()
}
