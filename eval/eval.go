// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"

	"numgrad.io/parser"
)

type Scope struct {
	Parent *Scope
	Ident  map[string]interface{}
}

func Eval(s *Scope, expr parser.Expr) (interface{}, error) {
	switch expr := expr.(type) {
	case *parser.Ident:
		for s != nil {
			if v, ok := s.Ident[expr.Name]; ok {
				return v, nil
			}
			s = s.Parent
		}
		return nil, fmt.Errorf("eval: undefined identifier: %q", expr.Name)
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
		sub, err := Eval(s, expr.Expr)
		if err != nil {
			return nil, err
		}
		switch expr.Op {
		case parser.Not:
		case parser.Mul: // deref
		case parser.Ref:
		case parser.LeftParen:
			return sub, nil
		}
	case *parser.CallExpr:
	case *parser.BasicLiteral:
		return expr.Value, nil
	}
	return nil, fmt.Errorf("eval TODO %#+v, %T", expr, expr)
}

func binOp(op parser.Token, x, y interface{}) (interface{}, error) {
	switch op {
	case parser.Add:
		switch x := x.(type) {
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
	case parser.Greater:
		return nil, fmt.Errorf("unknown binop %s", op)
	}
	return nil, fmt.Errorf("type mismatch Left: %T, Right: %T", x, y)
}
