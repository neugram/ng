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
	case *FuncLiteral:
		y, ok := y.(*FuncLiteral)
		if !ok {
			return false
		}
		return equalFuncLiteral(x, y)
	case *Ident:
		y, ok := y.(*Ident)
		if !ok {
			return false
		}
		if x == nil {
			if y == nil {
				return true
			} else {
				return false
			}
		} else {
			if y == nil {
				return false
			} else {
				return x.Name == y.Name
			}
		}
	case *CallExpr:
		y, ok := y.(*CallExpr)
		if !ok {
			return false
		}
		if !EqualExpr(x.Func, y.Func) {
			return false
		}
		if !equalExprs(x.Args, y.Args) {
			return false
		}
		return true
	default:
		panic(fmt.Sprintf("unknown expr type %T: %#+v", x, x))
	}
}

func equalExprs(x, y []Expr) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !EqualExpr(x[i], y[i]) {
			return false
		}
	}
	return true
}

func equalFields(f0, f1 []*Field) bool {
	if len(f0) != len(f1) {
		return false
	}
	for i := range f0 {
		if !equalField(f0[i], f1[i]) {
			return false
		}
	}
	return true
}

func equalField(f0, f1 *Field) bool {
	if !EqualExpr(f0.Name, f1.Name) {
		return false
	}
	return EqualExpr(f0.Type, f1.Type)
}

func equalFuncLiteral(f0, f1 *FuncLiteral) bool {
	if !equalFields(f0.Type.In, f1.Type.In) {
		return false
	}
	if !equalFields(f0.Type.Out, f1.Type.Out) {
		return false
	}
	if len(f0.Body) != len(f1.Body) {
		return false
	}
	for i := range f0.Body {
		if !equalStmt(f0.Body[i], f1.Body[i]) {
			return false
		}
	}
	return true
}

func equalStmt(x, y Stmt) bool {
	switch x := x.(type) {
	case *ReturnStmt:
		y, ok := y.(*ReturnStmt)
		if !ok {
			return false
		}
		if !equalExprs(x.Exprs, y.Exprs) {
			return false
		}
	default:
		panic(fmt.Sprintf("unknown stmt type %T: %#+v", x, x))
	}
	return true
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
