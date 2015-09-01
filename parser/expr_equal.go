package parser

import (
	"fmt"
	"math/big"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
)

func EqualExpr(x, y expr.Expr) bool {
	if x == nil && y == nil {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	switch x := x.(type) {
	case *expr.Binary:
		y, ok := y.(*expr.Binary)
		if !ok {
			return false
		}
		return x.Op == y.Op && EqualExpr(x.Left, y.Left) && EqualExpr(x.Right, y.Right)
	case *expr.Unary:
		y, ok := y.(*expr.Unary)
		if !ok {
			return false
		}
		return x.Op == y.Op && EqualExpr(x.Expr, y.Expr)
	case *expr.Bad:
		y, ok := y.(*expr.Bad)
		if !ok {
			return false
		}
		return x.Error == y.Error
	case *expr.BasicLiteral:
		y, ok := y.(*expr.BasicLiteral)
		if !ok {
			return false
		}
		return equalLiteral(x.Value, y.Value)
	case *expr.FuncLiteral:
		y, ok := y.(*expr.FuncLiteral)
		if !ok {
			return false
		}
		return equalFuncLiteral(x, y)
	case *expr.Ident:
		y, ok := y.(*expr.Ident)
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
	case *expr.Call:
		y, ok := y.(*expr.Call)
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

func equalExprs(x, y []expr.Expr) bool {
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

func equalFields(f0, f1 []*tipe.Field) bool {
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

func equalField(f0, f1 *tipe.Field) bool {
	if f0.Name != f1.Name {
		return false
	}
	return equalType(f0.Type, f1.Type)
}

func equalType(t0, t1 tipe.Type) bool {
	if t0 == nil && t1 == nil {
		return true
	}
	if t0 == nil || t1 == nil {
		return false
	}
	switch t0 := t0.(type) {
	case tipe.Basic:
		if t0 != t1 {
			return false
		}
	case *tipe.Func:
		t1, ok := t1.(*tipe.Func)
		if !ok {
			panic("not both tipe.Func")
			return false
		}
		if !equalFields(t0.In, t1.In) {
			panic("!equalFields(t0.In, t1.In)")
			return false
		}
		if !equalFields(t0.Out, t1.Out) {
			panic("!equalFields(t0.Out, t1.Out)")
			return false
		}
	case *tipe.Struct:
		t1, ok := t1.(*tipe.Struct)
		if !ok {
			return false
		}
		if !equalFields(t0.Fields, t1.Fields) {
			return false
		}
	case *tipe.Frame:
		t1, ok := t1.(*tipe.Frame)
		if !ok {
			return false
		}
		if !equalType(t0.Type, t1.Type) {
			return false
		}
	case *tipe.Unresolved:
		// TODO a correct definition for a parser, but not for a type checker
		t1, ok := t1.(*tipe.Unresolved)
		if !ok {
			return false
		}
		if t0.Name != t1.Name {
			return false
		}
	default:
		panic(fmt.Sprintf("unknown type: %T", t0))
	}
	return true
}

func equalFuncLiteral(f0, f1 *expr.FuncLiteral) bool {
	if !equalType(f0.Type, f1.Type) {
		return false
	}
	if f0.Body != nil || f1.Body != nil {
		if f0.Body == nil || f1.Body == nil {
			return false
		}
		b0, ok := f0.Body.(*stmt.Block)
		if !ok {
			return false
		}
		b1, ok := f1.Body.(*stmt.Block)
		if !ok {
			return false
		}
		return EqualStmt(b0, b1)
	}
	return true
}

func EqualStmt(x, y stmt.Stmt) bool {
	if x == nil && y == nil {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	switch x := x.(type) {
	case *stmt.Return:
		y, ok := y.(*stmt.Return)
		if !ok {
			return false
		}
		if !equalExprs(x.Exprs, y.Exprs) {
			return false
		}
	case *stmt.Import:
		y, ok := y.(*stmt.Import)
		if !ok {
			return false
		}
		if x.Name != y.Name {
			return false
		}
		if !EqualExpr(x.Path, y.Path) {
			return false
		}
	case *stmt.Type:
		y, ok := y.(*stmt.Type)
		if !ok {
			return false
		}
		if x.Name != y.Name {
			return false
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
	case *stmt.Const:
		y, ok := y.(*stmt.Const)
		if !ok {
			return false
		}
		if x.Name != y.Name {
			return false
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if !EqualExpr(x.Value, y.Value) {
			return false
		}
	case *stmt.Assign:
		y, ok := y.(*stmt.Assign)
		if !ok {
			return false
		}
		if !equalExprs(x.Left, y.Left) {
			return false
		}
		if !equalExprs(x.Right, y.Right) {
			return false
		}
	case *stmt.Block:
		y, ok := y.(*stmt.Block)
		if !ok {
			return false
		}
		if len(x.Stmts) != len(y.Stmts) {
			return false
		}
		for i := range x.Stmts {
			if !EqualStmt(x.Stmts[i], y.Stmts[i]) {
				return false
			}
		}
	case *stmt.If:
		y, ok := y.(*stmt.If)
		if !ok {
			return false
		}
		if !EqualStmt(x.Init, y.Init) {
			return false
		}
		if !EqualExpr(x.Cond, y.Cond) {
			return false
		}
		if !EqualStmt(x.Body, y.Body) {
			return false
		}
		if !EqualStmt(x.Else, y.Else) {
			return false
		}
	case *stmt.For:
		y, ok := y.(*stmt.For)
		if !ok {
			return false
		}
		if !EqualStmt(x.Init, y.Init) {
			return false
		}
		if !EqualExpr(x.Cond, y.Cond) {
			return false
		}
		if !EqualStmt(x.Post, y.Post) {
			return false
		}
		if !EqualStmt(x.Body, y.Body) {
			return false
		}
	case *stmt.Range:
		y, ok := y.(*stmt.Range)
		if !ok {
			return false
		}
		if !EqualExpr(x.Key, y.Key) {
			return false
		}
		if !EqualExpr(x.Val, y.Val) {
			return false
		}
		if !EqualExpr(x.Expr, y.Expr) {
			return false
		}
		if !EqualStmt(x.Body, y.Body) {
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
