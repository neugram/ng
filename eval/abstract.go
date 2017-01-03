// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import "reflect"

// Awkward proxy objects to allow some basic methodik definitions
// without reflect.NamedOf. https://golang.org/issue/16522
//
// An alternative is to construct types using plugins. It's entirely
// possible that macOS plugin support can be done before NamedOf.

type dictEmbed struct {
	getFunc reflect.Value
	setFunc reflect.Value
	delFunc reflect.Value
	rngFunc reflect.Value
	zkvFunc reflect.Value
}

func (e *dictEmbed) Get(key interface{}) (interface{}, error) {
	res := e.getFunc.Call([]reflect.Value{reflect.ValueOf(key)})
	i := res[0].Interface()
	err := res[1].Interface()
	if err != nil {
		return i, err.(error)
	}
	return i, nil
}

func (e *dictEmbed) Set(key, value interface{}) {
	e.setFunc.Call([]reflect.Value{reflect.ValueOf(key), reflect.ValueOf(value)})
}

func (e *dictEmbed) Delete(key interface{}) {
	e.delFunc.Call([]reflect.Value{reflect.ValueOf(key)})
}

func (e *dictEmbed) Range() interface {
	Next() (interface{}, interface{}, error)
} {
	panic("TODO")
}

func (e *dictEmbed) ZeroKeyValue() (interface{}, interface{}) {
	res := e.zkvFunc.Call(nil)
	return res[0].Interface(), res[1].Interface()
}

type tableEmbed struct {
	getFunc reflect.Value
	dimFunc reflect.Value
	lenFunc reflect.Value
	zerFunc reflect.Value
}

func (e *tableEmbed) Get(index ...int) interface{} {
	panic("TODO")
}

func (e *tableEmbed) Dim() int {
	panic("TODO")
}

func (e *tableEmbed) Len(dim int) int {
	panic("TODO")
}

func (e *tableEmbed) ZeroValue() interface{} {
	panic("TODO")
}
