// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package token defines data structures representing Neugram tokens.
package token

import "fmt"

// Token is a neugram lexical token.
type Token int

const (
	Unknown Token = iota
	Comment

	// Constants

	Ident     // E.g. funcName
	Int       // E.g. 1001 TODO: rename to Integer?
	Float     // E.g. 10.01
	Imaginary // E.g. 10.01i
	String    // E.g. "a string"

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
	Shell        // $$
	ShellWord    // [^\s|&;<>()]+
	ShellPipe    // |

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
	Pipe         // |

	// Keywords

	Package
	Import
	Importgo

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

	Map
	Struct
	Methodik
	Interface
	Type
)

var tokens = map[string]Token{
	"unknown":      Unknown,
	"comment":      Comment,
	"ident":        Ident,
	"integer":      Int,
	"float":        Float,
	"Imaginary":    Imaginary,
	"string":       String,
	"+":            Add,
	"-":            Sub,
	"*":            Mul,
	"/":            Div,
	"%":            Rem,
	"^":            Pow,
	"&":            Ref,
	"&&":           LogicalAnd,
	"||":           LogicalOr,
	"==":           Equal,
	"<":            Less,
	">":            Greater,
	"=":            Assign,
	"!":            Not,
	"!=":           NotEqual,
	"<=":           LessEqual,
	">=":           GreaterEqual,
	"$$":           Shell,
	"shellword":    ShellWord,
	"|":            ShellPipe,
	"++":           Inc,
	"--":           Dec,
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
	"Pipe":         Pipe,
}

var Keywords = map[string]Token{
	"package":     Package,
	"import":      Import,
	"importgo":    Importgo,
	"func":        Func,
	"return":      Return,
	"switch":      Switch,
	"case":        Case,
	"default":     Default,
	"fallthrough": Fallthrough,
	"const":       Const,
	"if":          If,
	"else":        Else,
	"for":         For,
	"range":       Range,
	"continue":    Continue,
	"break":       Break,
	"goto":        Goto,
	"go":          Go,
	"map":         Map,
	"struct":      Struct,
	"methodik":    Methodik,
	"interface":   Interface,
	"type":        Type,
}

func Keyword(n string) Token {
	return Keywords[n]
}

var tokenStrings = make(map[Token]string, len(tokens)+len(Keywords))

func init() {
	for s, t := range tokens {
		tokenStrings[t] = s
	}
	for s, t := range Keywords {
		tokenStrings[t] = s
	}
}

func (t Token) String() string {
	if s := tokenStrings[t]; s != "" {
		return s
	}
	return fmt.Sprintf("Token:%d", t)
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
	case Mul, Div, Rem, Pow:
		return 5
	}
	return 0
}
