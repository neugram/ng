// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package tipe defines data structures representing Numengrad types.
//
// Go took the usual spelling of type.
package tipe

import (
	"fmt"
	"strings"
)

type Type interface {
	Sexp() string
	tipe()
}

type Func struct {
	Params  *Tuple
	Results *Tuple
}

type Struct struct {
	Tags   []string
	Fields []Type
}

type Table struct {
	Type Type
}

type Tuple struct {
	Elems []Type
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
	_ = Type((*Struct)(nil))
	_ = Type((*Table)(nil))
	_ = Type((*Tuple)(nil))
	_ = Type((*Unresolved)(nil))
)

func (t Basic) tipe()       {}
func (t *Func) tipe()       {}
func (t *Struct) tipe()     {}
func (t *Table) tipe()      {}
func (t *Tuple) tipe()      {}
func (t *Unresolved) tipe() {}

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
	return fmt.Sprintf("(functype %s %s)", p, r)
}
func (e *Struct) Sexp() string {
	var fields []string
	for i, tag := range e.Tags {
		fields = append(fields, fmt.Sprintf("(%s %s)", tag, e.Fields[i].Sexp()))
	}
	return fmt.Sprintf("(structtype %s)", strings.Join(fields, " "))
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

func Equal(x, y Type) bool {
	if x == y {
		return true
	}
	fmt.Printf("tipe.Equal TODO\n")
	return false
}
