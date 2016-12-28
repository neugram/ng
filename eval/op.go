// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"

	"neugram.io/lang/token"
)

// TODO redo
func valEq(x, y interface{}) bool {
	if x == y {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	switch x := x.(type) {
	case *big.Int:
		switch y := y.(type) {
		case *big.Int:
			return x.Cmp(y) == 0
		}
	case *big.Float:
		switch y := y.(type) {
		case *big.Float:
			return x.Cmp(y) == 0
		}
		/*case *StructVal:
		switch y := y.(type) {
		case *StructVal:
			if len(x.Fields) != len(y.Fields) { // TODO compare tipe.Type
				return false
			}
			for i := range x.Fields {
				if !valEq(x.Fields[i], y.Fields[i]) {
					return false
				}
			}
			return true
		}
		*/
	}
	return false
}

func binOp(op token.Token, x, y interface{}) (interface{}, error) {
	switch op {
	case token.Add:
		switch x := x.(type) {
		case int:
			switch y := y.(type) {
			case int:
				return x + y, nil
			}
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
		case untypedInt:
			switch y := y.(type) {
			case untypedFloat:
				z := big.NewFloat(float64(x.Int.Int64()))
				return untypedFloat{z.Add(z, y.Float)}, nil
			case untypedInt:
				z := big.NewInt(0)
				return untypedInt{z.Add(x.Int, y.Int)}, nil
			}
		case untypedFloat:
			z := big.NewFloat(0)
			switch y := y.(type) {
			case untypedInt:
				z.SetInt(y.Int)
				return untypedFloat{z.Add(z, x.Float)}, nil
			case untypedFloat:
				return untypedFloat{z.Add(x.Float, y.Float)}, nil
			}
		case string:
			switch y := y.(type) {
			case string:
				return x + y, nil
			}
		}
	case token.Sub:
		switch x := x.(type) {
		case int:
			switch y := y.(type) {
			case int:
				return x - y, nil
			}
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
		case untypedInt:
			switch y := y.(type) {
			case untypedFloat:
				z := big.NewFloat(0)
				xf := big.NewFloat(float64(x.Int.Int64()))
				return untypedFloat{z.Sub(xf, y.Float)}, nil
			case untypedInt:
				z := big.NewInt(0)
				return untypedInt{z.Sub(x.Int, y.Int)}, nil
			}
		case untypedFloat:
			z := big.NewFloat(0)
			switch y := y.(type) {
			case untypedInt:
				yf := big.NewFloat(0)
				yf.SetInt(y.Int)
				return untypedFloat{z.Sub(x.Float, yf)}, nil
			case untypedFloat:
				return untypedFloat{z.Sub(x.Float, y.Float)}, nil
			}
		}
	case token.Mul:
		switch x := x.(type) {
		case int:
			switch y := y.(type) {
			case int:
				return x * y, nil
			}
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
		switch x := x.(type) {
		case int:
			switch y := y.(type) {
			case int:
				return x / y, nil
			}
		}
	case token.Rem:
	case token.Pow:
	case token.LogicalAnd, token.LogicalOr:
		panic("logical ops processed before binOp")
	case token.Equal:
		return valEq(x, y), nil
	case token.NotEqual:
		return !valEq(x, y), nil
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
	panic(fmt.Sprintf("binOp type mismatch Left: %+v (%T), Right: %+v (%T) op: %v", x, x, y, y, op))
}
