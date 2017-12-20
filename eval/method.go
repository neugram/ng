// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"reflect"
	"unsafe"

	"neugram.io/ng/format"
	"neugram.io/ng/gengo"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
)

func (p *Program) methodikDecl(s *stmt.MethodikDecl) {
	t := s.Type
	// TODO: lock reflector
	if _, exists := p.reflector.fwd[t]; exists {
		return
	}

	embType, err := p.reflectNamedType(s)
	if err != nil {
		panic(err)
	}

	var rtype reflect.Type

	_, isBasic := tipe.Underlying(t.Type).(tipe.Basic)
	_, isStruct := t.Type.(*tipe.Struct)
	if isBasic || isStruct {
		rtype = embType
	} else {
		panic(fmt.Sprintf("eval does not support methods on type %s", format.Type(t.Type)))
	}

	p.reflector.fwd[t] = rtype
}

func (p *Program) reflectNamedType(s *stmt.MethodikDecl) (reflect.Type, error) {
	src, err := gengo.GenNamedType(s, p.Types)
	if err != nil {
		return nil, err
	}
	path, err := p.pluginFile(s.Name, src)
	if err != nil {
		return nil, err
	}
	plg, err := p.pluginOpen(path)
	if err != nil {
		return nil, err
	}

	v, err := plg.Lookup("Zero")
	if err != nil {
		return nil, err
	}
	t := reflect.TypeOf(v).Elem()

	for _, m := range s.Methods {
		funcImpl := p.evalFuncLiteral(m, s.Type)

		v, err := plg.Lookup("Type_Method_" + m.Name)
		if err != nil {
			return nil, err
		}
		*v.(*reflect.Value) = funcImpl
	}

	return t, nil
}

// evalMethRecv puts the method reciever in the current scope.
func (p *Program) evalMethRecv(e *expr.FuncLiteral, recvt *tipe.Named, v reflect.Value) {
	var arg reflect.Value

	// All generated reflect trampolines pass an unsafe.Pointer
	// to the value in the first field.
	ptr := v.Interface().(unsafe.Pointer)

	// Here we adjust the pointer type to the correct value,
	// and if the neugram method does not want a pointer, remove
	// a level of indirection.
	rt := p.reflector.ToRType(recvt)
	arg = reflect.NewAt(rt, ptr)
	if !e.PointerReceiver {
		arg = arg.Elem()
	}

	p.Cur = &Scope{
		Parent:   p.Cur,
		VarName:  e.ReceiverName,
		Var:      arg,
		Implicit: true,
	}
}
