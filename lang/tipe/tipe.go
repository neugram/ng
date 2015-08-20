// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package tipe defines data structures representing Numengrad types.
//
// Go took the usual spelling of type.
package tipe

import (
	"bytes"
	"fmt"
)

type Type interface {
	Sexp() string
	tipe()
}

type Field struct {
	Name string
	Type Type
}

type Func struct {
	In  []*Field
	Out []*Field
}

type Struct struct {
	Fields []*Field
}

type BasicKind int

const (
	Invalid BasicKind = iota
	Bool
	Byte
	Int64
	Float32
	Float64
	Integer
	Float
	String
)

type Basic struct {
	Kind BasicKind
	Name string
}

type Unresolved struct {
	Name interface{} // string or *expr.Selector
}

var (
	_ = Type((*Func)(nil))
	_ = Type((*Struct)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Func) tipe()       {}
func (t Struct) tipe()     {}
func (t Unresolved) tipe() {}

func (e *Func) Sexp() string {
	return fmt.Sprintf("(functype (in %s) (out %s))", fieldsStr(e.In), fieldsStr(e.Out))
}
func (e *Struct) Sexp() string {
	return fmt.Sprintf("(struct )", fieldsStr(e.Fields))
}
func (e *Unresolved) Sexp() string {
	switch n := e.Name.(type) {
	case string:
		return n
	case interface {
		Sexp() string
	}:
		return "(type " + n.Sexp() + ")"
	default:
		return fmt.Sprintf("unknown:%s", e)
	}
}

func fieldsStr(fields []*Field) string {
	buf := new(bytes.Buffer)
	for i, f := range fields {
		if i > 0 {
			buf.WriteRune(' ')
		}
		fmt.Fprintf(buf, "(%s %s)", f.Name, f.Type.Sexp())
	}
	return buf.String()
}
