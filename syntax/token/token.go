// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
	Rune      // E.g. '\u1f4a9'

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
	ShellNewline // \n
	GreaterAnd   // >&
	AndGreater   // &>
	TwoGreater   // >>
	ChanOp       // <-
	Ellipsis     // ...

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

	LeftParen       // (
	LeftBracket     // [
	LeftBrace       // {
	LeftBraceTable  // {|
	RightParen      // )
	RightBracket    // ]
	RightBrace      // }
	RightBraceTable // |}
	Comma           // ,
	Period          // .
	Semicolon       // ;
	Colon           // :
	Pipe            // |

	// Keywords

	Package
	Import

	Func
	Return
	Defer

	Select
	Switch
	Case
	Default
	Fallthrough

	Const
	Var

	If
	Else

	For
	Range
	Continue
	Break
	Goto

	Go

	Chan
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
	"rune":         Rune,
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
	"shellpipe":    ShellPipe, // TODO: use Pipe
	"shellnewline": ShellNewline,
	">&":           GreaterAnd,
	"&>":           AndGreater,
	">>":           TwoGreater,
	"<-":           ChanOp,
	"...":          Ellipsis,
	"++":           Inc,
	"--":           Dec,
	"+=":           AddAssign,
	"-=":           SubAssign,
	"*=":           MulAssign,
	"/=":           DivAssign,
	"RemAssign":    RemAssign,
	"PowAssign":    PowAssign,
	":=":           Define,
	"(":            LeftParen,
	"[":            LeftBracket,
	"{":            LeftBrace,
	"{|":           LeftBraceTable,
	")":            RightParen,
	"]":            RightBracket,
	"|}":           RightBraceTable,
	",":            Comma,
	".":            Period,
	";":            Semicolon,
	":":            Colon,
	"|":            Pipe,
}

var Keywords = map[string]Token{
	"package":     Package,
	"import":      Import,
	"func":        Func,
	"return":      Return,
	"select":      Select,
	"switch":      Switch,
	"case":        Case,
	"default":     Default,
	"defer":       Defer,
	"fallthrough": Fallthrough,
	"const":       Const,
	"var":         Var,
	"if":          If,
	"else":        Else,
	"for":         For,
	"range":       Range,
	"continue":    Continue,
	"break":       Break,
	"goto":        Goto,
	"go":          Go,
	"chan":        Chan,
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
	// see:
	// https://golang.org/ref/spec#Operator_precedence
	switch t {
	case LogicalOr:
		return 1
	case LogicalAnd:
		return 2
	case Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual:
		return 3
	case Add, Sub, Pipe:
		return 4
	case Mul, Div, Rem, Pow:
		return 5
	}
	return 0
}
