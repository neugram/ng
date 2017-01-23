// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"fmt"
	"math/big"
	"reflect"

	"neugram.io/ng/expr"
	"neugram.io/ng/stmt"
	"neugram.io/ng/tipe"
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
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Op == y.Op && EqualExpr(x.Left, y.Left) && EqualExpr(x.Right, y.Right)
	case *expr.Unary:
		y, ok := y.(*expr.Unary)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Op == y.Op && EqualExpr(x.Expr, y.Expr)
	case *expr.Bad:
		y, ok := y.(*expr.Bad)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Error == y.Error
	case *expr.BasicLiteral:
		y, ok := y.(*expr.BasicLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return equalLiteral(x.Value, y.Value)
	case *expr.FuncLiteral:
		y, ok := y.(*expr.FuncLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return equalFuncLiteral(x, y)
	case *expr.CompLiteral:
		y, ok := y.(*expr.CompLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Keys, y.Keys) {
			return false
		}
		if !equalExprs(x.Elements, y.Elements) {
			return false
		}
		return true
	case *expr.MapLiteral:
		y, ok := y.(*expr.MapLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Keys, y.Keys) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
			return false
		}
		return true
	case *expr.SliceLiteral:
		y, ok := y.(*expr.SliceLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Elems, y.Elems) {
			return false
		}
		return true
	case *expr.TableLiteral:
		y, ok := y.(*expr.TableLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.ColNames, y.ColNames) {
			return false
		}
		if len(x.Rows) != len(y.Rows) {
			return false
		}
		for i, xrow := range x.Rows {
			if !equalExprs(xrow, y.Rows[i]) {
				return false
			}
		}
		return true
	case *expr.Type:
		y, ok := y.(*expr.Type)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		return true
	case *expr.Ident:
		y, ok := y.(*expr.Ident)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
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
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.Func, y.Func) {
			return false
		}
		if !equalExprs(x.Args, y.Args) {
			return false
		}
		return true
	case *expr.Selector:
		y, ok := y.(*expr.Selector)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.Left, y.Left) {
			return false
		}
		if !EqualExpr(x.Right, y.Right) {
			return false
		}
		return true
	case *expr.Slice:
		y, ok := y.(*expr.Slice)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.Low, y.Low) {
			return false
		}
		if !EqualExpr(x.High, y.High) {
			return false
		}
		if !EqualExpr(x.Max, y.Max) {
			return false
		}
		return true
	case *expr.Index:
		y, ok := y.(*expr.Index)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.Left, y.Left) {
			return false
		}
		return equalExprs(x.Indicies, y.Indicies)
	case *expr.ShellList:
		y, ok := y.(*expr.ShellList)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if len(x.AndOr) != len(y.AndOr) {
			return false
		}
		for i := 0; i < len(x.AndOr); i++ {
			if !EqualExpr(x.AndOr[i], y.AndOr[i]) {
				return false
			}
		}
		return true
	case *expr.ShellAndOr:
		y, ok := y.(*expr.ShellAndOr)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if len(x.Pipeline) != len(y.Pipeline) {
			return false
		}
		if x.Background != y.Background {
			return false
		}
		for i := 0; i < len(x.Pipeline); i++ {
			if !EqualExpr(x.Pipeline[i], y.Pipeline[i]) {
				return false
			}
		}
		if len(x.Sep) != len(y.Sep) {
			return false
		}
		for i := 0; i < len(x.Sep); i++ {
			if x.Sep[i] != y.Sep[i] {
				return false
			}
		}
		return true
	case *expr.ShellPipeline:
		y, ok := y.(*expr.ShellPipeline)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if x.Bang != y.Bang {
			return false
		}
		if len(x.Cmd) != len(y.Cmd) {
			return false
		}
		for i := 0; i < len(x.Cmd); i++ {
			if !EqualExpr(x.Cmd[i], y.Cmd[i]) {
				return false
			}
		}
		return true
	case *expr.ShellCmd:
		y, ok := y.(*expr.ShellCmd)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.SimpleCmd, y.SimpleCmd) {
			return false
		}
		if !EqualExpr(x.Subshell, y.Subshell) {
			return false
		}
		return true
	case *expr.ShellSimpleCmd:
		y, ok := y.(*expr.ShellSimpleCmd)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if len(x.Redirect) != len(y.Redirect) {
			return false
		}
		for i, e := range x.Redirect {
			if !EqualExpr(e, y.Redirect[i]) {
				return false
			}
		}
		if len(x.Assign) != len(y.Assign) {
			return false
		}
		for i, e := range x.Assign {
			if e != y.Assign[i] {
				return false
			}
		}
		if len(x.Args) != len(y.Args) {
			return false
		}
		for i, e := range x.Args {
			if e != y.Args[i] {
				return false
			}
		}
		return true
	case *expr.ShellRedirect:
		y, ok := y.(*expr.ShellRedirect)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if xn, yn := x.Number, y.Number; xn == nil || yn == nil {
			if xn != nil || yn != nil {
				return false
			}
		} else {
			if *xn != *yn {
				return false
			}
		}
		if x.Token != y.Token {
			return false
		}
		if x.Filename != y.Filename {
			return false
		}
		return true
	case *expr.ShellAssign:
		y, ok := y.(*expr.ShellAssign)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if x.Key != y.Key {
			return false
		}
		if x.Value != y.Value {
			return false
		}
		return true
	case *expr.Shell:
		y, ok := y.(*expr.Shell)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if len(x.Cmds) != len(y.Cmds) {
			return false
		}
		for i, xc := range x.Cmds {
			if !EqualExpr(xc, y.Cmds[i]) {
				return false
			}
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

func equalTuple(x, y *tipe.Tuple) bool {
	if x == nil && y == nil {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	if len(x.Elems) != len(y.Elems) {
		return false
	}
	for i := range x.Elems {
		if !equalType(x.Elems[i], y.Elems[i]) {
			return false
		}
	}
	return true
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
	case tipe.Builtin:
		if t0 != t1 {
			return false
		}
	case *tipe.Func:
		t1, ok := t1.(*tipe.Func)
		if !ok {
			panic("not both tipe.Func")
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
		}
		if !equalTuple(t0.Params, t1.Params) {
			return false
		}
		if !equalTuple(t0.Results, t1.Results) {
			return false
		}
	case *tipe.Struct:
		t1, ok := t1.(*tipe.Struct)
		if !ok {
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
		}
		if t0.Spec != t1.Spec {
			return false
		}
		if !reflect.DeepEqual(t0.FieldNames, t1.FieldNames) {
			return false
		}
		if len(t0.Fields) != len(t1.Fields) {
			return false
		}
		for i := range t0.Fields {
			if !equalType(t0.Fields[i], t1.Fields[i]) {
				return false
			}
		}
	case *tipe.Map:
		t1, ok := t1.(*tipe.Map)
		if !ok {
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
		}
		if !equalType(t0.Key, t1.Key) {
			return false
		}
		if !equalType(t0.Value, t1.Value) {
			return false
		}
	case *tipe.Methodik:
		t1, ok := t1.(*tipe.Methodik)
		if !ok {
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
		}
		if t0.Spec != t1.Spec {
			return false
		}
		if !reflect.DeepEqual(t0.MethodNames, t1.MethodNames) {
			return false
		}
		if len(t0.Methods) != len(t1.Methods) {
			return false
		}
		for i := range t0.Methods {
			if !equalType(t0.Methods[i], t1.Methods[i]) {
				return false
			}
		}
	case *tipe.Slice:
		t1, ok := t1.(*tipe.Slice)
		if !ok {
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
		}
		if !equalType(t0.Elem, t1.Elem) {
			return false
		}
	case *tipe.Table:
		t1, ok := t1.(*tipe.Table)
		if !ok {
			return false
		}
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
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
		if t0 == nil || t1 == nil {
			return t0 == nil && t1 == nil
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
		if x.Path != y.Path {
			return false
		}
	case *stmt.MethodikDecl:
		y, ok := y.(*stmt.MethodikDecl)
		if !ok {
			return false
		}
		if x.Name != y.Name {
			return false
		}
		if !equalType(x.Type, y.Type) {
			return false
		}
		if len(x.Methods) != len(y.Methods) {
			return false
		}
		for i := range x.Methods {
			if !EqualExpr(x.Methods[i], y.Methods[i]) {
				return false
			}
		}
	case *stmt.TypeDecl:
		y, ok := y.(*stmt.TypeDecl)
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
	case *stmt.Go:
		y, ok := y.(*stmt.Go)
		if !ok {
			return false
		}
		if !EqualExpr(x.Call, y.Call) {
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
	case *stmt.Simple:
		y, ok := y.(*stmt.Simple)
		if !ok {
			return false
		}
		if !EqualExpr(x.Expr, y.Expr) {
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
