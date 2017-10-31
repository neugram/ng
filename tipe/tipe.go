// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tipe defines data structures representing Neugram types.
//
// Go took the usual spelling of type.
package tipe

import (
	"fmt"
	"reflect"
	"sort"
)

type Type interface {
	tipe()
}

type Func struct {
	Spec     Specialization
	Params   *Tuple
	Results  *Tuple
	Variadic bool // last value of Params is a slice
	FreeVars []string
	FreeMdik []*Methodik
}

type Struct struct {
	Spec       Specialization
	FieldNames []string
	Fields     []Type
}

// TODO rename to Named
type Methodik struct {
	// TODO: need to track the definition package so the evaluator can
	// extract the mscope from the right place. Is this the only
	// instance of needing the source package? What about debug printing?
	Spec Specialization
	Type Type

	PkgName string
	PkgPath string
	Name    string

	MethodNames []string
	Methods     []*Func
}

type Array struct {
	Len      int64
	Elem     Type
	Ellipsis bool // array was defined as [...]T
}

type Slice struct {
	Elem Type
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

type ChanDirection int

const (
	ChanBoth ChanDirection = iota
	ChanSend
	ChanRecv
)

type Chan struct {
	Direction ChanDirection
	Elem      Type
}

type Map struct {
	Key   Type
	Value Type
}

type Package struct {
	GoPkg   interface{} // *gotypes.Package
	Path    string
	Exports map[string]Type
}

type Interface struct {
	Methods map[string]*Func
}

type Alias struct {
	Name string
	Type Type
}

var (
	Byte = &Alias{Name: "byte", Type: Uint8}
	Rune = &Alias{Name: "rune", Type: Int32}
)

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
	Integer Basic = "integer"
	Float   Basic = "float"
	Complex Basic = "complex"
	String  Basic = "string"

	Int   Basic = "int"
	Int8  Basic = "int8"
	Int16 Basic = "int16"
	Int32 Basic = "int32"
	Int64 Basic = "int64"

	Uint    Basic = "uint"
	Uint8   Basic = "uint8"
	Uint16  Basic = "uint16"
	Uint32  Basic = "uint32"
	Uint64  Basic = "uint64"
	Uintptr Basic = "uintptr"

	Float32 Basic = "float32"
	Float64 Basic = "float64"

	Complex64  Basic = "complex64"
	Complex128 Basic = "complex128"

	UntypedNil     Basic = "untyped nil" // nil pointer or nil interface
	UntypedString  Basic = "untyped string"
	UntypedBool    Basic = "untyped bool"
	UntypedInteger Basic = "untyped integer"
	UntypedFloat   Basic = "untyped float"
	UntypedRune    Basic = "untyped rune"
	UntypedComplex Basic = "untyped complex"
)

type Builtin string

const (
	Append      Builtin = "builtin append"
	Cap         Builtin = "builtin cap"
	Close       Builtin = "builtin close"
	ComplexFunc Builtin = "builtin complex"
	Copy        Builtin = "builtin copy"
	Delete      Builtin = "builtin delete"
	Imag        Builtin = "builtin imag"
	Len         Builtin = "builtin len"
	Make        Builtin = "builtin make"
	New         Builtin = "builtin new"
	Panic       Builtin = "builtin panic"
	Real        Builtin = "builtin real"
	Recover     Builtin = "builtin recover"
	// TODO Real Complex, Imaginary, Print
)

type Unresolved struct {
	Package string
	Name    string
}

var (
	_ = Type(Basic(""))
	_ = Type(Builtin(""))
	_ = Type((*Func)(nil))
	_ = Type((*Struct)(nil))
	_ = Type((*Methodik)(nil))
	_ = Type((*Array)(nil))
	_ = Type((*Slice)(nil))
	_ = Type((*Table)(nil))
	_ = Type((*Tuple)(nil))
	_ = Type((*Pointer)(nil))
	_ = Type((*Chan)(nil))
	_ = Type((*Map)(nil))
	_ = Type((*Package)(nil))
	_ = Type((*Interface)(nil))
	_ = Type((*Alias)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Basic) tipe()       {}
func (t Builtin) tipe()     {}
func (t *Func) tipe()       {}
func (t *Struct) tipe()     {}
func (t *Methodik) tipe()   {}
func (t *Array) tipe()      {}
func (t *Slice) tipe()      {}
func (t *Table) tipe()      {}
func (t *Tuple) tipe()      {}
func (t *Pointer) tipe()    {}
func (t *Chan) tipe()       {}
func (t *Map) tipe()        {}
func (t *Package) tipe()    {}
func (t *Interface) tipe()  {}
func (t *Alias) tipe()      {}
func (t *Unresolved) tipe() {}

func IsNumeric(t Type) bool {
	t = Unalias(t)
	b, ok := Underlying(t).(Basic)
	if !ok {
		return false
	}
	switch b {
	case Num, Integer, Float, Complex,
		Int, Int8, Int16, Int32, Int64,
		Uint8, Uint16, Uint32, Uint64,
		Float32, Float64, Complex64, Complex128,
		UntypedInteger, UntypedFloat, UntypedComplex:
		return true
	}
	return false
}

func UsesNum(t Type) bool {
	t = Unalias(t)
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
	case *Array:
		if UsesNum(t.Elem) {
			return true
		}
	case *Slice:
		if UsesNum(t.Elem) {
			return true
		}
	case *Table:
		if UsesNum(t.Type) {
			return true
		}
	case Basic:
		return t == Num
	case Builtin:
		return false
	}
	return false
}

func Unalias(t Type) Type {
	for {
		if u, ok := t.(*Alias); ok {
			t = u.Type
		} else {
			break
		}
	}
	return t
}

func Equal(x, y Type) bool {
	x, y = Unalias(x), Unalias(y)
	if x == y {
		return true
	}
	switch x := x.(type) {
	case Basic:
		y, ok := y.(Basic)
		if !ok {
			return false
		}
		return x == y
	case Builtin:
		y, ok := y.(Builtin)
		if !ok {
			return false
		}
		return x == y
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
	case *Array:
		y, ok := y.(*Array)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if x.Len != y.Len {
			return false
		}
		return Equal(x.Elem, y.Elem)
	case *Slice:
		y, ok := y.(*Slice)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		return Equal(x.Elem, y.Elem)
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
	case *Chan:
		y, ok := y.(*Chan)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if x.Direction != y.Direction {
			return false
		}
		return Equal(x.Elem, y.Elem)
	case *Map:
		y, ok := y.(*Map)
		if !ok {
			return false
		}
		if x == nil || y == nil {
			return false
		}
		if !Equal(x.Key, y.Key) {
			return false
		}
		return Equal(x.Value, y.Value)
	}
	fmt.Printf("tipe.Equal TODO %T\n", x)
	return false
}

func (t Interface) String() string {
	if len(t.Methods) == 0 {
		return "interface{}"
	}
	s := "interface{"
	for name, _ := range t.Methods {
		s += "\t" + name + "(TODO)"
	}
	s += "\n}"
	return s
}

func Underlying(t Type) Type {
	if t == nil {
		return nil
	}
	switch t := t.(type) {
	case *Alias:
		return Underlying(t.Type)
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

func (m *Memory) Methods(t Type) ([]string, []Type) { // TODO: ([]string, []*Func)
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

func (m *Memory) Method(t Type, name string) *Func {
	names, types := m.Methods(t)
	i := sort.Search(len(names), func(i int) bool { return names[i] >= name })
	if i == len(names) {
		return nil
	}
	if names[i] == name {
		return types[i].(*Func)
	}
	return nil
}

func methods(t Type, methodset map[string]Type, pointersRemoved int) {
	t = Unalias(t)
	switch t := t.(type) {
	// TODO case *Named:
	case *Pointer:
		if pointersRemoved < 1 {
			methods(t.Elem, methodset, pointersRemoved+1)
		}
	case *Interface:
		for name, typ := range t.Methods {
			if methodset[name] != nil {
				continue
			}
			methodset[name] = typ
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
