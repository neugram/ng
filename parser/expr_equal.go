package parser

import (
	"fmt"
	"math/big"
)

func EqualExpr(x, y Expr) bool {
	switch x := x.(type) {
	case *BinaryExpr:
		y, ok := y.(*BinaryExpr)
		if !ok {
			return false
		}
		return x.Op == y.Op && EqualExpr(x.Left, y.Left) && EqualExpr(x.Right, y.Right)
	case *UnaryExpr:
		y, ok := y.(*UnaryExpr)
		if !ok {
			return false
		}
		return x.Op == y.Op && EqualExpr(x.Expr, y.Expr)
	case *BadExpr:
		y, ok := y.(*BadExpr)
		if !ok {
			return false
		}
		return x.Error == y.Error
	case *BasicLiteral:
		y, ok := y.(*BasicLiteral)
		if !ok {
			return false
		}
		return equalLiteral(x.Value, y.Value)
	case *Ident:
		y, ok := y.(*Ident)
		if !ok {
			return false
		}
		return x.Name == y.Name
	case *CallExpr:
		y, ok := y.(*CallExpr)
		if !ok {
			return false
		}
		if !EqualExpr(x.Func, y.Func) {
			return false
		}
		if len(x.Args) != len(y.Args) {
			return false
		}
		for i, xarg := range x.Args {
			if !EqualExpr(xarg, y.Args[i]) {
				return false
			}
		}
		return true
	default:
		panic(fmt.Sprintf("unknown expr type %T: %#+v", x, x))
	}
}

func equalLiteral(lit0, lit1 interface{}) bool {
	if lit0 == lit1 {
		return true
	}
	switch lit0 := lit0.(type) {
	case *big.Int:
		if lit1, ok := lit1.(*big.Int); ok {
			return lit0.Cmp(lit1) == 0
		}
	case *big.Float:
		if lit1, ok := lit1.(*big.Float); ok {
			return lit0.Cmp(lit1) == 0
		}
	}
	return false
}
