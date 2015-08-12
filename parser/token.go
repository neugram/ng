// Copyright 2015 The Numgrad Authors. All rights reserved.
// Use of this source code is governed by a license that
// can be found in the LICENSE file.

package parser

//go:generate stringer -type=Token

// Token is a numgrad lexical token.
type Token int

const (
	Unknown Token = iota
	EOF
	Comment

	// Constants

	Ident     // funcName
	Int       // 1001
	Float     // 10.01
	Imaginary // 10.01i
	String    // "a string"

	// Expression Operators

	Add          // +
	Sub          // -
	Mul          // *
	Div          // /
	Rem          // %
	Pow          // ^
	And          // &&
	Or           // ||
	Equal        // ==
	Less         // <
	Greater      // >
	Assign       // =
	Not          // !
	NotEqual     // !=
	LessEqual    // <=
	GreaterEqual // >=

	// Statement Operators

	Inc       // ++
	Dec       // --
	AddAssign // +=
	SubAssign // -=
	MulAssign // *=
	DivAssign // /=
	RemAssign // %=
	PowAssign // ^=
	Define    // :=

	// Keywords

	Package
	Import

	Func
	Return

	Switch
	Case
	Default
	Fallthrough

	Const

	If
	Else

	For
	Range
	Continue
	Break
	Goto

	Go

	Frame
	Map
	Struct
	Type
)
