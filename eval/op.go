// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"math/big"
	"reflect"

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
		case int8:
			switch y := y.(type) {
			case int8:
				return x + y, nil
			}
		case int16:
			switch y := y.(type) {
			case int16:
				return x + y, nil
			}
		case int32:
			switch y := y.(type) {
			case int32:
				return x + y, nil
			}
		case int64:
			switch y := y.(type) {
			case int64:
				return x + y, nil
			}
		case uint:
			switch y := y.(type) {
			case uint:
				return x + y, nil
			}
		case uint8:
			switch y := y.(type) {
			case uint8:
				return x + y, nil
			}
		case uint16:
			switch y := y.(type) {
			case uint16:
				return x + y, nil
			}
		case uint32:
			switch y := y.(type) {
			case uint32:
				return x + y, nil
			}
		case uint64:
			switch y := y.(type) {
			case uint64:
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
		case int8:
			switch y := y.(type) {
			case int8:
				return x - y, nil
			}
		case int16:
			switch y := y.(type) {
			case int16:
				return x - y, nil
			}
		case int32:
			switch y := y.(type) {
			case int32:
				return x - y, nil
			}
		case int64:
			switch y := y.(type) {
			case int64:
				return x - y, nil
			}
		case uint:
			switch y := y.(type) {
			case uint:
				return x - y, nil
			}
		case uint8:
			switch y := y.(type) {
			case uint8:
				return x - y, nil
			}
		case uint16:
			switch y := y.(type) {
			case uint16:
				return x - y, nil
			}
		case uint32:
			switch y := y.(type) {
			case uint32:
				return x - y, nil
			}
		case uint64:
			switch y := y.(type) {
			case uint64:
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
		case int8:
			switch y := y.(type) {
			case int8:
				return x * y, nil
			}
		case int16:
			switch y := y.(type) {
			case int16:
				return x * y, nil
			}
		case int32:
			switch y := y.(type) {
			case int32:
				return x * y, nil
			}
		case int64:
			switch y := y.(type) {
			case int64:
				return x * y, nil
			}
		case uint:
			switch y := y.(type) {
			case uint:
				return x * y, nil
			}
		case uint8:
			switch y := y.(type) {
			case uint8:
				return x * y, nil
			}
		case uint16:
			switch y := y.(type) {
			case uint16:
				return x * y, nil
			}
		case uint32:
			switch y := y.(type) {
			case uint32:
				return x * y, nil
			}
		case uint64:
			switch y := y.(type) {
			case uint64:
				return x * y, nil
			}
		case float32:
			switch y := y.(type) {
			case float32:
				return x * y, nil
			}
		case float64:
			switch y := y.(type) {
			case float64:
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
		case int:
			switch y := y.(type) {
			case int:
				return x < y, nil
			}
		case int8:
			switch y := y.(type) {
			case int8:
				return x < y, nil
			}
		case int16:
			switch y := y.(type) {
			case int16:
				return x < y, nil
			}
		case int32:
			switch y := y.(type) {
			case int32:
				return x < y, nil
			}
		case int64:
			switch y := y.(type) {
			case int64:
				return x < y, nil
			}
		case uint:
			switch y := y.(type) {
			case uint:
				return x < y, nil
			}
		case uint8:
			switch y := y.(type) {
			case uint8:
				return x < y, nil
			}
		case uint16:
			switch y := y.(type) {
			case uint16:
				return x < y, nil
			}
		case uint32:
			switch y := y.(type) {
			case uint32:
				return x < y, nil
			}
		case uint64:
			switch y := y.(type) {
			case uint64:
				return x < y, nil
			}
		case float32:
			switch y := y.(type) {
			case float32:
				return x < y, nil
			}
		case float64:
			switch y := y.(type) {
			case float64:
				return x < y, nil
			}
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
		case int:
			switch y := y.(type) {
			case int:
				return x > y, nil
			}
		case int8:
			switch y := y.(type) {
			case int8:
				return x > y, nil
			}
		case int16:
			switch y := y.(type) {
			case int16:
				return x > y, nil
			}
		case int32:
			switch y := y.(type) {
			case int32:
				return x > y, nil
			}
		case int64:
			switch y := y.(type) {
			case int64:
				return x > y, nil
			}
		case uint:
			switch y := y.(type) {
			case uint:
				return x > y, nil
			}
		case uint8:
			switch y := y.(type) {
			case uint8:
				return x > y, nil
			}
		case uint16:
			switch y := y.(type) {
			case uint16:
				return x > y, nil
			}
		case uint32:
			switch y := y.(type) {
			case uint32:
				return x > y, nil
			}
		case uint64:
			switch y := y.(type) {
			case uint64:
				return x > y, nil
			}
		case float32:
			switch y := y.(type) {
			case float32:
				return x > y, nil
			}
		case float64:
			switch y := y.(type) {
			case float64:
				return x > y, nil
			}
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

func typeConv(t reflect.Type, v reflect.Value) (res reflect.Value) {
	if v.Type() == t {
		return v
	}
	if v.Kind() == reflect.Chan && t.Kind() == reflect.Chan {
		// bidirectional channel restricting to send/recv-only
		ret := reflect.New(t).Elem()
		ret.Set(v)
		return ret
	}
	switch t.Kind() {
	case reflect.Int:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(int(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(int(v.Float()))
		}
	case reflect.Int8:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int8(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(int8(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(int8(v.Float()))
		}
	case reflect.Int16:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int16(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(int16(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(int16(v.Float()))
		}
	case reflect.Int32:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int32(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(int32(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(int32(v.Float()))
		}
	case reflect.Int64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(int64(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(int64(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(int64(v.Float()))
		}
	case reflect.Uint:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(uint(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(uint(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(uint(v.Float()))
		}
	case reflect.Uint8:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(uint8(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(uint8(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(uint8(v.Float()))
		}
	case reflect.Uint16:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(uint16(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(uint16(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(uint16(v.Float()))
		}
	case reflect.Uint32:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(uint32(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(uint32(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(uint32(v.Float()))
		}
	case reflect.Uint64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(uint64(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return reflect.ValueOf(uint64(v.Uint()))
		case reflect.Float32, reflect.Float64:
			return reflect.ValueOf(uint64(v.Float()))
		}
	case reflect.Float64:
		return reflect.ValueOf(float64(v.Int()))
	case reflect.Interface:
		return reflect.ValueOf(v.Interface())
	case reflect.String:
		switch src := v.Interface().(type) {
		case []byte:
			return reflect.ValueOf(string(src))
		case rune:
			return reflect.ValueOf(string(src))
		}
	}
	panic(interpPanic{fmt.Errorf("unknown type conv: %v <- %v", t, v.Type())})
}
