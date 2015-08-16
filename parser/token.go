// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import "fmt"

// Token is a numgrad lexical token.
type Token int

const (
	Unknown Token = iota
	Comment

	// Constants

	Identifier // funcName
	Int        // 1001
	Float      // 10.01
	Imaginary  // 10.01i
	String     // "a string"

	// Expression Operators

	Add          // +
	Sub          // -
	Mul          // *
	Div          // /
	Rem          // %
	Pow          // ^
	Ref          // &
	LogicalAnd   // &&
	LogicalOr    // ||
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

	LeftParen    // (
	LeftBracket  // [
	LeftBrace    // {
	RightParen   // )
	RightBracket // ]
	RightBrace   // }
	Comma        // ,
	Period       // .
	Semicolon    // ;
	Colon        // :

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
	Val
)

var tokens = map[string]Token{
	"Unknown":      Unknown,
	"Comment":      Comment,
	"Identifier":   Identifier,
	"Int":          Int,
	"Float":        Float,
	"Imaginary":    Imaginary,
	"string":       String,
	"Add":          Add,
	"Sub":          Sub,
	"Mul":          Mul,
	"Div":          Div,
	"Rem":          Rem,
	"Pow":          Pow,
	"LogicalAnd":   LogicalAnd,
	"LogicalOr":    LogicalOr,
	"Equal":        Equal,
	"Less":         Less,
	"Greater":      Greater,
	"Assign":       Assign,
	"Not":          Not,
	"NotEqual":     NotEqual,
	"LessEqual":    LessEqual,
	"GreaterEqual": GreaterEqual,
	"Inc":          Inc,
	"Dec":          Dec,
	"AddAssign":    AddAssign,
	"SubAssign":    SubAssign,
	"MulAssign":    MulAssign,
	"DivAssign":    DivAssign,
	"RemAssign":    RemAssign,
	"PowAssign":    PowAssign,
	"Define":       Define,
	"LeftParen":    LeftParen,
	"LeftBracket":  LeftBracket,
	"LeftBrace":    LeftBrace,
	"RightParen":   RightParen,
	"RightBracket": RightBracket,
	"RightBrace":   RightBrace,
	"Comma":        Comma,
	"Period":       Period,
	"Semicolon":    Semicolon,
	"Colon":        Colon,
	"package":      Package,
	"import":       Import,
	"func":         Func,
	"return":       Return,
	"switch":       Switch,
	"case":         Case,
	"default":      Default,
	"fallthrough":  Fallthrough,
	"const":        Const,
	"if":           If,
	"else":         Else,
	"for":          For,
	"range":        Range,
	"continue":     Continue,
	"break":        Break,
	"goto":         Goto,
	"go":           Go,
	"frame":        Frame,
	"map":          Map,
	"struct":       Struct,
	"type":         Type,
	"val":          Val,
}

var tokenStrings = make(map[Token]string, len(tokens))

func init() {
	for s, t := range tokens {
		tokenStrings[t] = s
	}
}

func (t Token) String() string {
	if s := tokenStrings[t]; s != "" {
		return s
	}
	return fmt.Sprintf("parser.Token(%d)", t)
}

func (t Token) Precedence() int {
	switch t {
	case LogicalOr:
		return 1
	case LogicalAnd:
		return 2
	case Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual:
		return 3
	case Add, Sub:
		return 4
	case Mul, Div, Rem:
		return 5
	}
	return 0
}
