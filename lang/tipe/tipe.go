// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package tipe defines data structures representing Neugram types.
//
// Go took the usual spelling of type.
package tipe

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type Type interface {
	Sexp() string
	tipe()
}

type Func struct {
	Spec     Specialization
	Params   *Tuple
	Results  *Tuple
	Variadic bool // last value of Params is a slice
	FreeVars []string
}

type Struct struct {
	Spec       Specialization
	FieldNames []string
	Fields     []Type
}

type Methodik struct {
	Spec        Specialization
	Type        Type
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

/*
TODO remove, replace with side table.
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
*/

type Package struct {
	IsGo    bool
	Path    string
	Exports map[string]Type
}

type Interface struct {
	Methods map[string]*Func
}

// Specialization carries any type specialization data particular to this type.
//
// *Func, *Struct, *Methodik can be parameterized over the name num, which can
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
	Byte    Basic = "byte"
	Rune    Basic = "rune"
	Integer Basic = "integer"
	Float   Basic = "float"
	Complex Basic = "complex"
	String  Basic = "string"

	Int64   Basic = "int64"
	Float32 Basic = "float32"
	Float64 Basic = "float64"

	GoInt Basic = "goint"

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
	_ = Type((*Struct)(nil))
	_ = Type((*Methodik)(nil))
	_ = Type((*Table)(nil))
	_ = Type((*Tuple)(nil))
	_ = Type((*Pointer)(nil))
	_ = Type((*Package)(nil))
	_ = Type((*Interface)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Basic) tipe()       {}
func (t *Func) tipe()       {}
func (t *Struct) tipe()     {}
func (t *Methodik) tipe()   {}
func (t *Table) tipe()      {}
func (t *Tuple) tipe()      {}
func (t *Pointer) tipe()    {}
func (t *Package) tipe()    {}
func (t *Interface) tipe()  {}
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
func (e *Struct) Sexp() string {
	var s []string
	for i, tag := range e.FieldNames {
		s = append(s, fmt.Sprintf("(%s %s)", tag, e.Fields[i].Sexp()))
	}
	return fmt.Sprintf("(structtype %s %s)", e.Spec.Sexp(), strings.Join(s, " "))
}
func (e *Methodik) Sexp() string {
	u := "nil"
	if e.Type != nil {
		u = e.Type.Sexp()
	}
	var s []string
	for i, tag := range e.MethodNames {
		s = append(s, fmt.Sprintf("(%s %s)", tag, e.Methods[i].Sexp()))
	}
	return fmt.Sprintf("(methodiktype %s %s %s)", e.Spec.Sexp(), u, strings.Join(s, " "))
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
		ts := "<nil>"
		if t != nil {
			ts = t.Sexp()
		}
		elems = append(elems, ts)
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

func (e *Package) Sexp() string {
	var elems []string
	for n, t := range e.Exports {
		ts := "<nil>"
		if t != nil {
			ts = t.Sexp()
		}
		elems = append(elems, "("+n+" "+ts+")")
	}
	return fmt.Sprintf("(package %s)", strings.Join(elems, " "))
}

func (e *Interface) Sexp() string {
	var s []string
	for name, fn := range e.Methods {
		s = append(s, fmt.Sprintf("(%s %s)", name, fn.Sexp()))
	}
	return fmt.Sprintf("(interfacetype %s)", strings.Join(s, " "))
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
		GoInt,
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
	case *Struct:
		for _, t := range t.Fields {
			if UsesNum(t) {
				return true
			}
		}
	case *Methodik:
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
	case *Struct:
		y, ok := y.(*Struct)
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
		return true
	case *Methodik:
		y, ok := y.(*Methodik)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if x.Spec != y.Spec {
			return false
		}
		if !Equal(x.Type, y.Type) {
			return false
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
	case *Interface:
		y, ok := y.(*Interface)
		if !ok {
			return false
		}
		if x == nil && y == nil {
			return true
		}
		if x == nil || y == nil {
			return false
		}
		if len(x.Methods) != len(y.Methods) {
			return false
		}
		for xn, xt := range x.Methods {
			yt, ok := y.Methods[xn]
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

func (t Interface) String() string {
	if len(t.Methods) == 0 {
		return "interface{}"
	}
	s := "interface{"
	for name, m := range t.Methods {
		s += "\t" + name + m.Sexp() // TODO .String()
	}
	s += "\n}"
	return s
}

func Underlying(t Type) Type {
	if t == nil {
		return nil
	}
	switch t := t.(type) {
	// TODO case *Named:
	case *Methodik:
		return Underlying(t.Type)
	default:
		return t
	}
}

type Memory struct {
	methodNames map[Type][]string
	methods     map[Type][]Type
}

func NewMemory() *Memory {
	return &Memory{
		methodNames: make(map[Type][]string),
		methods:     make(map[Type][]Type),
	}
}

func (m *Memory) Methods(t Type) ([]string, []Type) {
	names := m.methodNames[t]
	if names != nil {
		return names, m.methods[t]
	}
	methodset := make(map[string]Type)
	methods(t, methodset, 0)

	for name := range methodset {
		names = append(names, name)
	}
	sort.Strings(names)
	var methods []Type
	for _, name := range names {
		methods = append(methods, methodset[name])
	}
	m.methodNames[t] = names
	m.methods[t] = methods
	return names, methods
}

func methods(t Type, methodset map[string]Type, pointersRemoved int) {
	switch t := t.(type) {
	// TODO case *Named:
	case *Pointer:
		if pointersRemoved < 1 {
			methods(t.Elem, methodset, pointersRemoved+1)
		}
	case *Methodik:
		for i, name := range t.MethodNames {
			if methodset[name] != nil {
				continue
			}
			methodset[name] = t.Methods[i]
		}
		methods(t.Type, methodset, pointersRemoved)
	}
}
