// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	gotypes "go/types"
	"reflect"

	"neugram.io/eval/gowrap"
	"neugram.io/lang/tipe"
)

type GoPkg struct {
	Type  *tipe.Package
	GoPkg *gotypes.Package
	Wrap  *gowrap.Pkg
}

type GoValue struct {
	Type  tipe.Type
	Value interface{}
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
		res[i] = &GoValue{
			Type:  f.Type.Results.Elems[i],
			Value: v.Interface(),
		}
	}
	return res, nil
}
