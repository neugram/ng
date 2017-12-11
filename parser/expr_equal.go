// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"fmt"
	"math/big"

	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Keys, y.Keys) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Keys, y.Keys) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
			return false
		}
		return true
	case *expr.ArrayLiteral:
		y, ok := y.(*expr.ArrayLiteral)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !tipe.EqualUnresolved(x.Type, y.Type) {
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Keys, y.Keys) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
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
		if x.Ellipsis != y.Ellipsis {
			return false
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
	case *expr.TypeAssert:
		y, ok := y.(*expr.TypeAssert)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		if !EqualExpr(x.Left, y.Left) {
			return false
		}
		return tipe.EqualUnresolved(x.Type, y.Type)
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
		if !tipe.EqualUnresolved(x.Elems[i], y.Elems[i]) {
			return false
		}
	}
	return true
}

func equalTypes(t0, t1 []tipe.Type) bool {
	if len(t0) != len(t1) {
		return false
	}
	for i := range t0 {
		if !tipe.EqualUnresolved(t0[i], t1[i]) {
			return false
		}
	}
	return true
}

func equalFuncLiteral(f0, f1 *expr.FuncLiteral) bool {
	if !tipe.EqualUnresolved(f0.Type, f1.Type) {
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

func equalSwitchCases(c1, c2 []stmt.SwitchCase) bool {
	if len(c1) != len(c2) {
		return false
	}
	for i := range c1 {
		if !equalSwitchCase(c1[i], c2[i]) {
			return false
		}
	}
	return true
}

func equalSwitchCase(c1, c2 stmt.SwitchCase) bool {
	if !equalExprs(c1.Conds, c2.Conds) {
		return false
	}
	if c1.Default != c2.Default {
		return false
	}
	if !EqualStmt(c1.Body, c2.Body) {
		return false
	}
	return true
}

func equalTypeSwitchCases(c1, c2 []stmt.TypeSwitchCase) bool {
	if len(c1) != len(c2) {
		return false
	}
	for i := range c1 {
		if !equalTypeSwitchCase(c1[i], c2[i]) {
			return false
		}
	}
	return true
}

func equalTypeSwitchCase(c1, c2 stmt.TypeSwitchCase) bool {
	if !equalTypes(c1.Types, c2.Types) {
		return false
	}
	if c1.Default != c2.Default {
		return false
	}
	if !EqualStmt(c1.Body, c2.Body) {
		return false
	}
	return true
}

func equalSelectCases(c1, c2 []stmt.SelectCase) bool {
	if len(c1) != len(c2) {
		return false
	}
	for i := range c1 {
		if !equalSelectCase(c1[i], c2[i]) {
			return false
		}
	}
	return true
}

func equalSelectCase(c1, c2 stmt.SelectCase) bool {
	if c1.Default != c2.Default {
		return false
	}
	if !EqualStmt(c1.Stmt, c2.Stmt) {
		return false
	}
	if !EqualStmt(c1.Body, c2.Body) {
		return false
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
	case *stmt.Defer:
		y, ok := y.(*stmt.Defer)
		if !ok {
			return false
		}
		if !EqualExpr(x.Expr, y.Expr) {
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
	case *stmt.ImportSet:
		y, ok := y.(*stmt.ImportSet)
		if !ok {
			return false
		}
		if len(x.Imports) != len(y.Imports) {
			return false
		}
		for i := range x.Imports {
			if !EqualStmt(x.Imports[i], y.Imports[i]) {
				return false
			}
		}
	case *stmt.MethodikDecl:
		y, ok := y.(*stmt.MethodikDecl)
		if !ok {
			return false
		}
		if x.Name != y.Name {
			return false
		}
		if !tipe.EqualUnresolved(x.Type, y.Type) {
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
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
	case *stmt.TypeDeclSet:
		y, ok := y.(*stmt.TypeDeclSet)
		if !ok {
			return false
		}
		if len(x.TypeDecls) != len(y.TypeDecls) {
			return false
		}
		for i := range x.TypeDecls {
			if !EqualStmt(x.TypeDecls[i], y.TypeDecls[i]) {
				return false
			}
		}
	case *stmt.Const:
		y, ok := y.(*stmt.Const)
		if !ok {
			return false
		}
		if len(x.NameList) != len(y.NameList) {
			return false
		}
		for i := range x.NameList {
			if x.NameList[i] != y.NameList[i] {
				return false
			}
		}
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
			return false
		}
	case *stmt.ConstSet:
		y, ok := y.(*stmt.ConstSet)
		if !ok {
			return false
		}
		if len(x.Consts) != len(y.Consts) {
			return false
		}
		for i := range x.Consts {
			if !EqualStmt(x.Consts[i], y.Consts[i]) {
				return false
			}
		}
	case *stmt.Var:
		y, ok := y.(*stmt.Var)
		if !ok {
			return false
		}
		if len(x.NameList) != len(y.NameList) {
			return false
		}
		for i := range x.NameList {
			if x.NameList[i] != y.NameList[i] {
				return false
			}
		}
		if !tipe.EqualUnresolved(x.Type, y.Type) {
			return false
		}
		if !equalExprs(x.Values, y.Values) {
			return false
		}
	case *stmt.VarSet:
		y, ok := y.(*stmt.VarSet)
		if !ok {
			return false
		}
		if len(x.Vars) != len(y.Vars) {
			return false
		}
		for i := range x.Vars {
			if !EqualStmt(x.Vars[i], y.Vars[i]) {
				return false
			}
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
	case *stmt.Send:
		y, ok := y.(*stmt.Send)
		if !ok {
			return false
		}
		if !EqualExpr(x.Chan, y.Chan) {
			return false
		}
		if !EqualExpr(x.Value, y.Value) {
			return false
		}
	case *stmt.Branch:
		y, ok := y.(*stmt.Branch)
		if !ok {
			return false
		}
		if x.Type != y.Type {
			return false
		}
		if x.Label != y.Label {
			return false
		}
	case *stmt.Labeled:
		y, ok := y.(*stmt.Labeled)
		if !ok {
			return false
		}
		if x.Label != y.Label {
			return false
		}
		if !EqualStmt(x.Stmt, y.Stmt) {
			return false
		}
	case *stmt.Switch:
		y, ok := y.(*stmt.Switch)
		if !ok {
			return false
		}
		if !EqualStmt(x.Init, y.Init) {
			return false
		}
		if !EqualExpr(x.Cond, y.Cond) {
			return false
		}
		if !equalSwitchCases(x.Cases, y.Cases) {
			return false
		}
	case *stmt.TypeSwitch:
		y, ok := y.(*stmt.TypeSwitch)
		if !ok {
			return false
		}
		if !EqualStmt(x.Init, y.Init) {
			return false
		}
		if !EqualStmt(x.Assign, y.Assign) {
			return false
		}
		if !equalTypeSwitchCases(x.Cases, y.Cases) {
			return false
		}
	case *stmt.Select:
		y, ok := y.(*stmt.Select)
		if !ok {
			return false
		}
		if !equalSelectCases(x.Cases, y.Cases) {
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
