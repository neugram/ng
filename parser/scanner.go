// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"fmt"
	"math/big"
	"unicode"
	"unicode/utf8"

	"numgrad.io/lang/token"
)

const bom = 0xFEFF // byte order marker

func newScanner() *Scanner {
	s := &Scanner{
		addSrc:  make(chan []byte),
		needSrc: make(chan bool),
	}
	//go s.next()
	//<-s.needSrc
	return s
}

type Scanner struct {
	// Current Token
	Line    int
	Offset  int
	Token   token.Token
	Literal interface{} // string, *big.Int, *big.Float

	// Scanner state
	src  []byte
	r    rune
	off  int
	semi bool
	err  error

	addSrc  chan []byte
	needSrc chan bool
	done    bool
}

func (s *Scanner) errorf(format string, a ...interface{}) {
	s.err = fmt.Errorf("numgrad: scanner: %s (off %d)", fmt.Sprintf(format, a...), s.Offset)
}

func (s *Scanner) next() {
	if s.off >= len(s.src) {
		if s.r == -1 {
			return
		}
		//fmt.Printf("need src\n")
		s.needSrc <- true
		b := <-s.addSrc
		//fmt.Printf("adding source: %q\n", string(b))
		if b == nil {
			s.needSrc <- false
			s.Offset = len(s.src)
			s.Token = token.Unknown
			s.Literal = nil
			s.r = -1
			return
		}
		s.src = append(s.src, b...)
	}

	s.Offset = s.off
	if s.r == '\n' {
		s.Line++
	}
	var w int
	s.r, w = rune(s.src[s.off]), 1
	switch {
	case s.r == 0:
		s.errorf("bad UTF-8: zero byte")
	case s.r >= 0x80:
		s.r, w = utf8.DecodeRune(s.src[s.off:])
		if s.r == utf8.RuneError && w == 1 {
			s.errorf("bad UTF-8")
		} else if s.r == bom {
			s.errorf("bad byte order marker")
		}
	}
	s.off += w
	return
}

func (s *Scanner) skipWhitespace() {
	for s.r == ' ' || s.r == '\t' || (s.r == '\n' && !s.semi) || s.r == '\r' {
		s.next()
	}
}

func (s *Scanner) scanIdentifier() string {
	off := s.Offset
	for unicode.IsLetter(s.r) || unicode.IsDigit(s.r) {
		s.next()
	}
	return string(s.src[off:s.Offset])
}

func (s *Scanner) scanMantissa() {
	for '0' <= s.r && s.r <= '9' {
		s.next()
	}
}

func (s *Scanner) scanNumber(seenDot bool) (token.Token, interface{}) {
	off := s.Offset
	tok := token.Int

	if seenDot {
		off--
		tok = token.Float
		s.scanMantissa()
		goto exponent
	}

	s.scanMantissa()

	// fraction
	if s.r == '.' {
		tok = token.Float
		s.next()
		s.scanMantissa()
	}

exponent:
	if s.r == 'e' || s.r == 'E' {
		tok = token.Float
		s.next()
		if s.r == '-' || s.r == '+' {
			s.next()
		}
		s.scanMantissa()
	}

	if s.r == 'i' {
		tok = token.Imaginary
		s.next()
	}

	str := string(s.src[off:s.Offset])
	var value interface{}
	switch tok {
	case token.Int:
		i, ok := big.NewInt(0).SetString(str, 10)
		if ok {
			value = i
		} else {
			s.errorf("bad int literal: %q", str)
			tok = token.Unknown
		}
	case token.Float:
		f, ok := big.NewFloat(0).SetString(str)
		if ok {
			value = f
		} else {
			s.errorf("bad float literal: %q", str)
			tok = token.Unknown
		}
	case token.Imaginary:
		panic("TODO token.Imaginary")
	}

	return tok, value
}

func (s *Scanner) scanComment() string {
	off := s.Offset - 1 // already ate the first '/'

	if s.r == '/' {
		// single line "// comment"
		s.next()
		for s.r > 0 && s.r != '\n' {
			s.next()
		}
	} else {
		// multi-line "/* comment */"
		s.next()
		terminated := false
		for s.r > 0 {
			r := s.r
			s.next()
			if r == '*' && s.r == '/' {
				s.next()
				terminated = true
				break
			}
		}
		if !terminated {
			s.err = fmt.Errorf("multi-line comment not terminated") // TODO offset
		}
	}

	lit := s.src[off:s.Offset]
	// TODO remove any \r in comments?
	return string(lit)
}

func (s *Scanner) Next() {
	defer func() {
		fmt.Printf("Scanner.Next s.Token=%s, s.Offset=%d, s.off=%d", s.Token, s.Offset, s.off)
		if s.Literal != nil {
			fmt.Printf(" Literal=%s", s.Literal)
		}
		fmt.Printf("\n")
	}()
	s.skipWhitespace()
	//fmt.Printf("Next: s.r=%v (%s) s.off=%d\n", s.r, string(s.r), s.off)

	wasSemi := s.semi
	s.semi = false
	s.Literal = nil
	r := s.r
	switch {
	case unicode.IsLetter(r):
		lit := s.scanIdentifier()
		s.Token = token.Keyword(lit)
		if s.Token == token.Unknown {
			s.Token = token.Ident
			s.Literal = lit
		}
		switch s.Token {
		case token.Ident, token.Break, token.Continue, token.Fallthrough, token.Return:
			s.semi = true
		}
		return
	case unicode.IsDigit(r):
		s.semi = true
		s.Token, s.Literal = s.scanNumber(false)
		return
	case r == '\n':
		s.semi = false
		s.Token = token.Semicolon
		return
	}

	s.next()
	switch r {
	case -1:
		if wasSemi {
			s.Token = token.Semicolon
			return
		}
		s.Token = token.Unknown
		return
	case '\n':
		s.semi = false
		s.Token = token.Semicolon
	case '(':
		s.Token = token.LeftParen
	case ')':
		s.semi = true
		s.Token = token.RightParen
	case '[':
		s.Token = token.LeftBracket
	case ']':
		s.semi = true
		s.Token = token.RightBracket
	case '{':
		s.Token = token.LeftBrace
	case '}':
		s.semi = true
		s.Token = token.RightBrace
	case ',':
		s.Token = token.Comma
	case ';':
		s.Token = token.Semicolon
	case ':':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.Define
		default:
			s.Token = token.Colon
		}
	case '+':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.AddAssign
		case '+':
			s.next()
			s.Token = token.Inc
			s.semi = true
		default:
			s.Token = token.Add
		}
	case '-':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.SubAssign
		case '-':
			s.next()
			s.Token = token.Dec
			s.semi = true
		default:
			s.Token = token.Sub
		}
	case '=':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.Equal
		default:
			s.Token = token.Assign
		}
	case '*':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.MulAssign
		default:
			s.Token = token.Mul
		}
	case '/':
		switch s.r {
		case '/', '*': // comment
			// TODO if s.semi and no more tokens on this line, insert newline
			s.Literal = s.scanComment()
			s.Token = token.Comment
		case '=':
			s.next()
			s.Token = token.DivAssign
		default:
			s.Token = token.Div
		}
	case '%':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.RemAssign
		default:
			s.Token = token.Rem
		}
	case '^':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.PowAssign
		default:
			s.Token = token.Pow
		}
	case '>':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.GreaterEqual
		default:
			s.Token = token.Greater
		}
	case '<':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.LessEqual
		default:
			s.Token = token.Less
		}
	case '&':
		switch s.r {
		case '&':
			s.next()
			s.Token = token.LogicalAnd
		default:
			s.Token = token.Ref
		}
	case '|':
		if s.r != '|' {
			s.Token = token.Unknown
			s.err = fmt.Errorf("parser: unexpected '|'")
			return
		}
		s.next()
		s.Token = token.LogicalOr
	case '!':
		switch s.r {
		case '=':
			s.next()
			s.Token = token.NotEqual
		default:
			s.Token = token.Not
		}
	default:
		s.Token = token.Unknown
		fmt.Printf("Scanner.Next unknown r=%v (%q) s.off=%d\n", r, string(rune(r)), s.off)
		s.err = fmt.Errorf("parser: unknown r=%v", r)
	}
}
