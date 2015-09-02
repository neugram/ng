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

type Table struct {
	Type Type
}

type Basic string

const (
	Invalid Basic = "invalid"
	Bool    Basic = "bool"
	Integer Basic = "integer"
	Float   Basic = "float"
	Complex Basic = "complex"
	String  Basic = "string"

	Int64   Basic = "int64"
	Float32 Basic = "float32"
	Float64 Basic = "float64"

	UntypedBool    Basic = "untyped bool"
	UntypedInteger Basic = "untyped integer"
	UntypedFloat   Basic = "untyped float"
	UntypedComplex Basic = "untyped complex"
)

type Named struct {
	Name string // not an identifier, only for debugging
	// TODO: move Ref to a Checker map?
	Ref        interface{} // a *typecheck.Obj after type checking
	Underlying Type
	// TODO: Methods []*Obj
}

type Unresolved struct {
	Package string
	Name    string
}

var (
	_ = Type(Basic(""))
	_ = Type((*Func)(nil))
	_ = Type((*Struct)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Basic) tipe()       {}
func (t *Func) tipe()       {}
func (t *Struct) tipe()     {}
func (t *Table) tipe()      {}
func (t *Named) tipe()      {}
func (t *Unresolved) tipe() {}

func (e Basic) Sexp() string { return fmt.Sprintf("(basictype %s)", string(e)) }
func (e *Func) Sexp() string {
	return fmt.Sprintf("(functype (in %s) (out %s))", fieldsStr(e.In), fieldsStr(e.Out))
}
func (e *Struct) Sexp() string {
	return fmt.Sprintf("(structtype %s)", fieldsStr(e.Fields))
}
func (e *Table) Sexp() string {
	u := "nil"
	if e.Type != nil {
		u = e.Type.Sexp()
	}
	return fmt.Sprintf("(tabletype %s)", u)
}
func (e *Named) Sexp() string {
	u := "nilunderlying"
	if e.Underlying != nil {
		u = e.Underlying.Sexp()
	}
	return fmt.Sprintf("(namedtype %s %s)", e.Name, u)
}
func (e *Unresolved) Sexp() string {
	if e.Package == "" {
		return fmt.Sprintf("(unresolved %s)", e.Name)
	}
	return fmt.Sprintf("(unresolved %s.%s)", e.Package, e.Name)
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

func Underlying(t Type) Type {
	if n, ok := t.(*Named); ok {
		return n.Underlying
	}
	return t
}

func IsInt(t Type) bool {
	b, ok := t.(Basic)
	if !ok {
		return false
	}
	return b == Integer || b == Int64
}

func IsFloat(t Type) bool {
	b, ok := t.(Basic)
	if !ok {
		return false
	}
	return b == Float || b == Float32 || b == Float64
}

func Equal(x, y Type) bool {
	if x == y {
		return true
	}
	fmt.Printf("tipe.Equal TODO\n")
	return false
}
