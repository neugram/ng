// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	gotypes "go/types"
	"reflect"

	"neugram.io/eval/gowrap"
	"neugram.io/lang/expr"
	"neugram.io/lang/tipe"
)

type GoPkg struct {
	Type  *tipe.Package
	GoPkg *gotypes.Package
	Wrap  *gowrap.Pkg
}

type GoValue struct {
	Type  tipe.Type
	Value interface{} // TODO: Value should be a reflect.Value?
}

type GoFunc struct {
	Type *tipe.Func
	Func interface{}
}

func (f GoFunc) call(args []interface{}) (res []interface{}, err error) {
	var vargs []reflect.Value
	var vres []reflect.Value
	v := reflect.ValueOf(f.Func)
	if f.Type.Variadic {
		nonVarLen := len(f.Type.Params.Elems) - 1
		for i := 0; i < nonVarLen; i++ {
			vargs = append(vargs, reflect.ValueOf(args[i]))
		}
		if len(args) > nonVarLen {
			vargs = append(vargs, reflect.ValueOf(args[nonVarLen:]))
		} else {
			vargs = append(vargs, reflect.ValueOf([]interface{}{}))
		}
		vres = v.CallSlice(vargs)
	} else {
		var vargs []reflect.Value
		for _, arg := range args {
			vargs = append(vargs, reflect.ValueOf(arg))
		}
		vres = v.Call(vargs)
	}

	res = make([]interface{}, len(vres))
	for i, v := range vres {
		switch v.Kind() {
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Float32, reflect.Float64:
			res[i] = v.Interface()
		// TODO Int16 Int32 Int64 Uint Uint8 Uint16 Uint32 Uint64 Uintptr
		// TODO Complex64 Complex128
		default:
			res[i] = &GoValue{
				Type:  f.Type.Results.Elems[i],
				Value: v.Interface(),
			}
		}
	}
	return res, nil
}

func makeGoStruct(e *expr.CompLiteral, t tipe.Type, goT *gotypes.TypeName) *GoValue {
	pkg := goT.Pkg()
	reflectT := reflect.TypeOf(
		gowrap.Pkgs[pkg.Path()].Exports[goT.Name()],
	)

	v := reflect.New(reflectT)
	if len(e.Elements) > 0 {
		panic("TODO CompLiteral with values for GoValue\n")
	}
	res := &GoValue{
		Type:  t,
		Value: v.Elem().Interface(),
	}
	return res
}
