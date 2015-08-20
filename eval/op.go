// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"

	"numgrad.io/lang/token"
)

func binOp(op token.Token, x, y interface{}) (interface{}, error) {
	if v, ok := x.(*Variable); ok {
		x = v.Value
	}
	if v, ok := y.(*Variable); ok {
		y = v.Value
	}

	switch op {
	case token.Add:
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
	case token.Sub:
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
	case token.Mul:
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
	case token.Div:
	case token.Rem:
	case token.Pow:
	case token.LogicalAnd, token.LogicalOr:
		panic("logical ops processed before binOp")
	case token.Equal:
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
	case token.NotEqual:
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
	case token.Less:
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
	case token.Greater:
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
	panic(fmt.Sprintf("binOp type mismatch Left: %T, Right: %T", x, y))
}
