// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"unsafe"

	"neugram.io/ng/format"
	"neugram.io/ng/gengo"
	"neugram.io/ng/gotool"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
)

func (p *Program) ifaceDecl(t *tipe.Named) {
	// TODO: lock reflector
	if _, exists := p.reflector.fwd[t]; exists {
		return
	}

	rtype, err := p.reflectNamedType(t, nil)
	if err != nil {
		panic(err)
	}
	p.reflector.fwd[t] = rtype
}

func (p *Program) methodikDecl(s *stmt.MethodikDecl) {
	t := s.Type
	// TODO: lock reflector
	if _, exists := p.reflector.fwd[t]; exists {
		return
	}

	embType, err := p.reflectNamedType(s.Type, s.Methods)
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

func (p *Program) reflectNamedType(t *tipe.Named, methods []*expr.FuncLiteral) (reflect.Type, error) {
	adjPkgPath, dir, err := gotool.M.Dir(path.Join("methodik", t.Name))
	if err != nil {
		return nil, err
	}

	pkgb, mainb, err := gengo.GenNamedType(t, methods, adjPkgPath, p.typePlugins)
	if err != nil {
		return nil, err
	}

	p.typePlugins[t] = adjPkgPath

	// Do not remove the pkgGo file as future builds that
	// import this package will need the file to exist.
	pkgGo := filepath.Join(dir, t.Name+".go")
	if err := ioutil.WriteFile(pkgGo, pkgb, 0666); err != nil {
		return nil, err
	}
	name := t.Name + "-main"
	mainGo := filepath.Join(dir, name, t.Name+".go")
	os.Mkdir(filepath.Dir(mainGo), 0775)
	if err := ioutil.WriteFile(mainGo, mainb, 0666); err != nil {
		return nil, err
	}
	defer os.Remove(mainGo)

	plg, err := gotool.M.Open(path.Join(adjPkgPath, name))
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", name, err)
	}

	v, err := plg.Lookup("Zero")
	if err != nil {
		return nil, err
	}
	rt := reflect.TypeOf(v).Elem()

	for _, m := range methods {
		funcImpl := p.evalFuncLiteral(m, t)

		v, err := plg.Lookup("Type_Method_" + m.Name)
		if err != nil {
			return nil, err
		}
		**v.(**reflect.Value) = funcImpl
	}

	return rt, nil
}

// evalMethRecv puts the method receiver in the current scope.
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
