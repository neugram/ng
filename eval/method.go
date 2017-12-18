// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"reflect"

	"neugram.io/ng/gengo"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
)

func (p *Program) methodikDecl(s *stmt.MethodikDecl) {
	t := s.Type
	// TODO: lock reflector
	if _, exists := p.reflector.fwd[t]; exists {
		return
	}

	st, isStruct := t.Type.(*tipe.Struct)
	if !isStruct {
		// TODO: we can do much better than just methodik struct types now.
		panic("eval only supports methodik on struct types")
	}

	embType, err := p.embMethodType(s)
	if err != nil {
		panic(err)
	}
	fields := []reflect.StructField{{
		Name:      embType.Name(),
		Type:      reflect.PtrTo(embType),
		Anonymous: true,
	}}
	for _, f := range st.Fields {
		fields = append(fields, reflect.StructField{
			Name: f.Name,
			Type: p.reflector.ToRType(f.Type),
		})
	}
	rtype := reflect.StructOf(fields)
	p.reflector.fwd[t] = rtype
}

func (p *Program) embMethodType(s *stmt.MethodikDecl) (reflect.Type, error) {
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
