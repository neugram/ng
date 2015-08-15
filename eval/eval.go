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
		return binOp(expr.Op, lhs, rhs)
	case *parser.UnaryExpr:
		sub, err := Eval(s, expr.Expr)
		if err != nil {
			return nil, err
		}
		_ = sub // TODO
		switch expr.Op {
		case parser.Not:
		case parser.Mul: // deref
		case parser.Ref:
		case parser.LeftParen:
		}
	case *parser.CallExpr:
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
		}
	case parser.Sub:
	case parser.Mul:
	case parser.Div:
	case parser.Rem:
	case parser.Pow:
	case parser.LogicalAnd:
	case parser.LogicalOr:
	case parser.Equal:
	case parser.NotEqual:
	case parser.Less:
	case parser.Greater:
		return nil, fmt.Errorf("eval: unknown binop: %s", op)
	}
	panic("TODO")
}
