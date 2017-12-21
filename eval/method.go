// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
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

func (p *Program) pluginDir(pkgPath string) (adjPkgPath, dir string) {
	if p.tempdir == "" {
		var err error
		p.tempdir, err = ioutil.TempDir("", "ng-tmp-")
		if err != nil {
			panic(Panic{val: err})
		}
	}
	gopath := p.tempdir
	if err := os.MkdirAll(filepath.Join(gopath, "src"), 0775); err != nil {
		panic(Panic{val: err})
	}

	adjPkgPath = pkgPath
	dir = filepath.Join(gopath, "src", adjPkgPath)
	i := 0
	for {
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			break
		}
		i++
		adjPkgPath = filepath.Join(fmt.Sprintf("p%d", i), pkgPath)
		dir = filepath.Join(gopath, "src", adjPkgPath)
	}
	if err := os.MkdirAll(dir, 0775); err != nil {
		panic(Panic{val: err})
	}
	return adjPkgPath, dir
}

func (p *Program) reflectNamedType(s *stmt.MethodikDecl) (reflect.Type, error) {
	adjPkgPath, dir := p.pluginDir(path.Join("methodik", s.Name))

	pkgb, mainb, err := gengo.GenNamedType(s, p.Types, adjPkgPath, p.typePlugins)
	if err != nil {
		return nil, err
	}

	p.typePlugins[s.Type] = adjPkgPath

	if err := ioutil.WriteFile(filepath.Join(dir, s.Name+".go"), pkgb, 0666); err != nil {
		return nil, err
	}
	name := s.Name + "-main"
	mainGo := filepath.Join(dir, name, s.Name+".go")
	os.Mkdir(filepath.Dir(mainGo), 0775)
	if err := ioutil.WriteFile(mainGo, mainb, 0666); err != nil {
		return nil, err
	}

	cmd := exec.Command("go", "build", "-buildmode=plugin", adjPkgPath+"/"+name)
	cmd.Env = append(os.Environ(), "GOPATH="+p.tempdir)
	cmd.Dir = p.tempdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("building plugin %s failed: %v\n%s", name, err, out)
	}

	pluginName := name + ".so" // TODO: p%d
	plg, err := plugin.Open(filepath.Join(p.tempdir, pluginName))
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", name, err)
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
		**v.(**reflect.Value) = funcImpl
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
