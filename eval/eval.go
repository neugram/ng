// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"

	"numgrad.io/parser"
)

type Variable struct {
	Value interface{}
}

type Scope struct {
	Parent *Scope
	Ident  map[string]*Variable
}

func newScope(parent *Scope) *Scope {
	return &Scope{
		Parent: parent,
		Ident:  make(map[string]*Variable),
	}
}

// eval reduces expr to a *Variable or literal
func eval(s *Scope, expr parser.Expr) (interface{}, error) {
	switch expr := expr.(type) {
	case *parser.BinaryExpr:
		lhs, err := Eval(s, expr.Left)
		if err != nil {
			return nil, err
		}
		rhs, err := Eval(s, expr.Right)
		if err != nil {
			return nil, err
		}

		v, err := binOp(expr.Op, lhs, rhs)
		if err != nil {
			return nil, fmt.Errorf("eval: %v evaluating %s", err, expr)
		}
		return v, nil
	case *parser.UnaryExpr:
		sub, err := eval(s, expr.Expr)
		if err != nil {
			return nil, err
		}
		switch expr.Op {
		case parser.Not:
		case parser.Mul: // deref
		case parser.Ref:
		case parser.Sub:
			v, err := evalLitOrVar(sub)
			if err != nil {
				return nil, err
			}
			return binOp(parser.Sub, big.NewInt(0), v)
		case parser.LeftParen:
			return sub, nil
		}
	case *parser.CallExpr:
		fn, err := eval(s, expr.Func)
		if err != nil {
			return nil, err
		}
		switch fn := fn.(type) {
		case *parser.FuncLiteral:
			fs := newScope(s)
			for _, stmt := range fn.Body {
				retVals, returned, err := evalStmt(fs, stmt)
				if err != nil {
					return nil, err
				}
				if returned {
					return retVals, nil
				}
			}
			if len(fn.Type.Out) > 0 {
				return nil, fmt.Errorf("expected return %v", fn.Type.Out)
			}
		}
	case *parser.Ident:
		for s != nil {
			if v, ok := s.Ident[expr.Name]; ok {
				return v, nil
			}
			s = s.Parent
		}
		// TODO only add to scope if it was declared.
		v := new(Variable)
		s.Ident[expr.Name] = v
		return v, nil
		//return nil, fmt.Errorf("eval: undefined identifier: %q", expr.Name)
	case *parser.BasicLiteral, *parser.FuncLiteral, *Variable:
		return expr, nil
	}
	return nil, fmt.Errorf("TODO eval(%#+v), %T", expr, expr)
}

func evalStmt(s *Scope, stmt parser.Stmt) (retVals []interface{}, returned bool, err error) {
	switch stmt := stmt.(type) {
	case *parser.AssignStmt:
		for i, lhs := range stmt.Left {
			lhs, err = eval(s, lhs)
			if err != nil {
				return nil, false, err
			}
			lhsvar, ok := lhs.(*Variable)
			if !ok {
				return nil, false, fmt.Errorf("assignment to non-variable: %s", lhs)
			}
			v, err := eval(s, stmt.Right[i])
			if err != nil {
				return nil, false, err
			}
			lhsvar.Value = v
		}
		return nil, false, nil
	case *parser.IfStmt:
		is := newScope(s)
		if stmt.Init != nil {
			_, _, err := evalStmt(is, stmt.Init)
			if err != nil {
				return nil, false, err
			}
		}
		cond, err := Eval(s, stmt.Cond)
		if err != nil {
			return nil, false, err
		}
		doBody, ok := cond.(bool)
		if !ok {
			return nil, false, fmt.Errorf("if condition not a boolean expression, got: %v", cond)
		}
		if doBody {
			return evalStmt(is, stmt.Body)
		} else {
			return evalStmt(is, stmt.Else)
		}
	case *parser.ReturnStmt:
		retVals := make([]interface{}, len(stmt.Exprs))
		for i, e := range stmt.Exprs {
			retVals[i], err = eval(s, e)
			if err != nil {
				return nil, false, err
			}
		}
		return retVals, true, nil
	case *parser.BlockStmt:
		bs := newScope(s)
		for _, s := range stmt.Stmts {
			retVals, ret, err := evalStmt(bs, s)
			if err != nil || ret {
				return retVals, ret, err
			}
		}
		return nil, false, nil
	}
	panic(fmt.Sprintf("TODO evalStmt: %T: %s", stmt, stmt))
}

func evalLitOrVar(v interface{}) (interface{}, error) {
	fmt.Printf("evalLitOrVar: %#+v\n", v)
	switch v := v.(type) {
	case *parser.BasicLiteral:
		return v.Value, nil
	case *parser.FuncLiteral:
		return nil, fmt.Errorf("function not called")
	case *Variable:
		// TODO eww, but catched BasicLiteral in Variable
		return evalLitOrVar(v.Value)
	default:
		return v, nil
	}
}

func Eval(s *Scope, expr parser.Expr) (interface{}, error) {
	v, err := eval(s, expr)
	if err != nil {
		return nil, err
	}
	if v, ok := v.([]interface{}); ok {
		for i, iv := range v {
			v[i], err = evalLitOrVar(iv)
			if err != nil {
				return nil, err
			}
		}
		if len(v) == 1 {
			return v[0], nil
		}
		return v, nil
	}
	return evalLitOrVar(v)
}

func binOp(op parser.Token, x, y interface{}) (interface{}, error) {
	switch op {
	case parser.Add:
		switch x := x.(type) {
		case int64:
			switch y := y.(type) {
			case int64:
				return x + y, nil
			}
		case float32:
			switch y := y.(type) {
			case float32:
				return x + y, nil
			}
		case float64:
			switch y := y.(type) {
			case float64:
				return x + y, nil
			}
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				z := big.NewInt(0)
				return z.Add(x, y), nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				z := big.NewFloat(0)
				return z.Add(x, y), nil
			}
		}
	case parser.Sub:
		switch x := x.(type) {
		case int64:
			switch y := y.(type) {
			case int64:
				return x - y, nil
			}
		case float32:
			switch y := y.(type) {
			case float32:
				return x - y, nil
			}
		case float64:
			switch y := y.(type) {
			case float64:
				return x - y, nil
			}
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				z := big.NewInt(0)
				return z.Sub(x, y), nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				z := big.NewFloat(0)
				return z.Sub(x, y), nil
			}
		}
	case parser.Mul:
		switch x := x.(type) {
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				z := big.NewInt(0)
				return z.Mul(x, y), nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				z := big.NewFloat(0)
				return z.Mul(x, y), nil
			}
		}
	case parser.Div:
	case parser.Rem:
	case parser.Pow:
	case parser.LogicalAnd:
		// TODO shortcut eval
		if x, ok := x.(bool); ok {
			if y, ok := y.(bool); ok {
				return x && y, nil
			}
		}
	case parser.LogicalOr:
		// TODO shortcut eval
		if x, ok := x.(bool); ok {
			if y, ok := y.(bool); ok {
				return x || y, nil
			}
		}
	case parser.Equal:
		if x == y {
			return true, nil
		}
		switch x := x.(type) {
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				return x.Cmp(y) == 0, nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				return x.Cmp(y) == 0, nil
			}
		}
	case parser.NotEqual:
		if x == y {
			return false, nil
		}
		switch x := x.(type) {
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				return x.Cmp(y) != 0, nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				return x.Cmp(y) != 0, nil
			}
		}
	case parser.Less:
		switch x := x.(type) {
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				return x.Cmp(y) == -1, nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				return x.Cmp(y) == -1, nil
			}
		}
	case parser.Greater:
		switch x := x.(type) {
		case *big.Int:
			switch y := y.(type) {
			case *big.Int:
				return x.Cmp(y) == 1, nil
			}
		case *big.Float:
			switch y := y.(type) {
			case *big.Float:
				return x.Cmp(y) == 1, nil
			}
		}
	}
	//return nil, fmt.Errorf("type mismatch Left: %T, Right: %T", x, y)
	panic(fmt.Sprintf("type mismatch Left: %T, Right: %T", x, y))
}
