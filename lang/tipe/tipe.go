// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package tipe defines data structures representing Numengrad types.
//
// Go took the usual spelling of type.
package tipe

import (
	"fmt"
	gotypes "go/types"
	"reflect"
	"strings"
)

type Type interface {
	Sexp() string
	tipe()
}

type Func struct {
	Spec    Specialization
	Params  *Tuple
	Results *Tuple
}

type Class struct {
	Spec        Specialization
	FieldNames  []string
	Fields      []Type
	MethodNames []string
	Methods     []Type
}

type Table struct {
	Type Type
}

type Tuple struct {
	Elems []Type
}

type Pointer struct {
	Elem Type
}

// Go is a type imported from Go.
// It has an equivalent type in this system.
//
// Either GoType or GoPkg is set.
//
// Technically in go/types a *gotypes.Package is not a type, but we
// model a package as a type for neugram, so we fake it.
type Go struct {
	GoType     gotypes.Type
	GoPkg      *gotypes.Package
	Equivalent Type
}

type Package struct {
	Exports map[string]Type
}

// Specialization carries any type specialization data particular to this type.
//
// Both *Func and *Class can be parameterized over the name num, which can
// take any of:
//
//	integer, int64, float, float32, float64, complex, complex128
//
// At the defnition of a class or function, the matching Type will have the
// Num filed set to Invalid if it is not parameterized, or Num if it is.
//
// On a value of a parameterized class or a Call of a parameterized function,
// Num will either Num or one of the basic numeric types (if specialized).
type Specialization struct {
	// Num is the specialization of the type parameter num in
	Num Basic
}

type Basic string

const (
	Invalid Basic = "invalid"
	Num     Basic = "num" // type parameter
	Bool    Basic = "bool"
	Integer Basic = "integer"
	Float   Basic = "float"
	Complex Basic = "complex"
	String  Basic = "string"

	Int64   Basic = "int64"
	Float32 Basic = "float32"
	Float64 Basic = "float64"

	UntypedBool    Basic = "untyped bool" // TODO remove if we are not going to have named types
	UntypedInteger Basic = "untyped integer"
	UntypedFloat   Basic = "untyped float"
	UntypedComplex Basic = "untyped complex"
)

type Unresolved struct {
	Package string
	Name    string
}

var (
	_ = Type(Basic(""))
	_ = Type((*Func)(nil))
	_ = Type((*Class)(nil))
	_ = Type((*Table)(nil))
	_ = Type((*Tuple)(nil))
	_ = Type((*Pointer)(nil))
	_ = Type((*Go)(nil))
	_ = Type((*Package)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Basic) tipe()       {}
func (t *Func) tipe()       {}
func (t *Class) tipe()      {}
func (t *Table) tipe()      {}
func (t *Tuple) tipe()      {}
func (t *Pointer) tipe()    {}
func (t *Go) tipe()         {}
func (t *Package) tipe()    {}
func (t *Unresolved) tipe() {}

func (e Specialization) Sexp() string {
	return fmt.Sprintf("(spec num=%s)", e.Num.Sexp())
}

func (e Basic) Sexp() string { return fmt.Sprintf("(basictype %s)", string(e)) }
func (e *Func) Sexp() string {
	p := "nilparams"
	if e.Params != nil {
		p = e.Params.Sexp()
	}
	r := "nilresults"
	if e.Results != nil {
		p = e.Results.Sexp()
	}
	return fmt.Sprintf("(functype %s %s %s)", e.Spec.Sexp(), p, r)
}
func (e *Class) Sexp() string {
	var s []string
	for i, tag := range e.FieldNames {
		s = append(s, fmt.Sprintf("(%s %s)", tag, e.Fields[i].Sexp()))
	}
	for i, tag := range e.MethodNames {
		s = append(s, fmt.Sprintf("(%s %s)", tag, e.Methods[i].Sexp()))
	}
	return fmt.Sprintf("(classtype %s %s)", e.Spec.Sexp(), strings.Join(s, " "))
}
func (e *Table) Sexp() string {
	u := "nil"
	if e.Type != nil {
		u = e.Type.Sexp()
	}
	return fmt.Sprintf("(tabletype %s)", u)
}
func (e *Tuple) Sexp() string {
	var elems []string
	for _, t := range e.Elems {
		elems = append(elems, t.Sexp())
	}
	return fmt.Sprintf("(tupletype %s)", strings.Join(elems, " "))
}
func (e *Pointer) Sexp() string {
	u := "nil"
	if e.Elem != nil {
		u = e.Elem.Sexp()
	}
	return fmt.Sprintf("(* %s)", u)
}
func (e *Go) Sexp() string {
	u := "nilgo"
	if e.Equivalent != nil {
		u = e.Equivalent.Sexp()
	}
	return fmt.Sprintf("(gotype %s)", u)
}
func (e *Package) Sexp() string {
	var elems []string
	for n, t := range e.Exports {
		elems = append(elems, "("+n+" "+t.Sexp()+")")
	}
	return fmt.Sprintf("(package %s)", strings.Join(elems, " "))
}
func (e *Unresolved) Sexp() string {
	if e.Package == "" {
		return fmt.Sprintf("(unresolved %s)", e.Name)
	}
	return fmt.Sprintf("(unresolved %s.%s)", e.Package, e.Name)
}

func IsNumeric(t Type) bool {
	b, ok := t.(Basic)
	if !ok {
		return false
	}
	switch b {
	case Num, Integer, Float, Complex,
		Int64, Float32, Float64,
		UntypedInteger, UntypedFloat, UntypedComplex:
		return true
	}
	return false
}

func UsesNum(t Type) bool {
	switch t := t.(type) {
	case *Func:
		if t.Params != nil {
			for _, t := range t.Params.Elems {
				if UsesNum(t) {
					return true
				}
			}
		}
		if t.Results != nil {
			for _, t := range t.Results.Elems {
				if UsesNum(t) {
					return true
				}
			}
		}
	case *Class:
		for _, t := range t.Fields {
			if UsesNum(t) {
				return true
			}
		}
		for _, t := range t.Methods {
			if UsesNum(t) {
				return true
			}
		}
	case *Table:
		if UsesNum(t.Type) {
			return true
		}
	case Basic:
		return t == Num
	}
	return false
}

func Equal(x, y Type) bool {
	if x == y {
		return true
	}
	switch x := x.(type) {
	case *Func:
		y, ok := y.(*Func)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if x.Spec != y.Spec {
			return false
		}
		if !Equal(x.Params, y.Params) {
			return false
		}
		if !Equal(x.Results, y.Results) {
			return false
		}
		return true
	case *Class:
		y, ok := y.(*Class)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if x.Spec != y.Spec {
			return false
		}
		if !reflect.DeepEqual(x.FieldNames, y.FieldNames) {
			return false
		}
		if len(x.Fields) != len(y.Fields) {
			return false
		}
		for i := range x.Fields {
			if !Equal(x.Fields[i], y.Fields[i]) {
				return false
			}
		}
		if !reflect.DeepEqual(x.MethodNames, y.MethodNames) {
			return false
		}
		if len(x.Methods) != len(y.Methods) {
			return false
		}
		for i := range x.Methods {
			if !Equal(x.Methods[i], y.Methods[i]) {
				return false
			}
		}
		return true
	case *Table:
		y, ok := y.(*Table)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		return Equal(x.Type, y.Type)
	case *Tuple:
		y, ok := y.(*Tuple)
		if !ok {
			return false
		}
		if x == nil && y == nil {
			return true
		}
		if x == nil || y == nil {
			return false
		}
		if len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !Equal(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	case *Go:
		y, ok := y.(*Go)
		if !ok {
			return false
		}
		if x == nil && y == nil {
			return true
		}
		if x == nil || y == nil {
			return false
		}
		if !Equal(x.Equivalent, y.Equivalent) {
			return false
		}
		return true
	case *Package:
		y, ok := y.(*Package)
		if !ok {
			return false
		}
		if x == nil && y == nil {
			return true
		}
		if x == nil || y == nil {
			return false
		}
		if len(x.Exports) != len(y.Exports) {
			return false
		}
		for xn, xt := range x.Exports {
			yt, ok := y.Exports[xn]
			if !ok {
				return false
			}
			if !Equal(xt, yt) {
				return false
			}
		}
		return true
	case *Pointer:
		y, ok := y.(*Pointer)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		return Equal(x.Elem, y.Elem)
	}
	fmt.Printf("tipe.Equal TODO %T\n", x)
	return false
}
