// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"fmt"
	"io"
	"math/big"
	"unicode"
	"unicode/utf8"
)

const bom = 0xFEFF // byte order marker

func NewScanner(src []byte) *Scanner {
	s := &Scanner{src: src}
	s.next()
	return s
}

type Scanner struct {
	// Current Token
	Line    int
	Offset  int
	Token   Token
	Literal interface{} // string, *big.Int, *big.Float

	// Scanner state
	src  []byte
	r    rune
	off  int
	semi bool
	err  error
}

func (s *Scanner) errorf(format string, a ...interface{}) {
	s.err = fmt.Errorf("numgrad: scanner: %s (off %d)", fmt.Sprintf(format, a...), s.Offset)
}

func (s *Scanner) next() {
	if s.off >= len(s.src) {
		s.Offset = len(s.src)
		s.Token = Unknown
		s.Literal = nil
		s.r = -1
		return
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
	for s.r == ' ' || s.r == '\t' || s.r == '\n' && !s.semi || s.r == '\r' {
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

func (s *Scanner) scanNumber(seenDot bool) (Token, interface{}) {
	off := s.Offset
	tok := Int

	if seenDot {
		off--
		tok = Float
		s.scanMantissa()
		goto exponent
	}

	s.scanMantissa()

	// fraction
	if s.r == '.' {
		tok = Float
		s.next()
		s.scanMantissa()
	}

exponent:
	if s.r == 'e' || s.r == 'E' {
		tok = Float
		s.next()
		if s.r == '-' || s.r == '+' {
			s.next()
		}
		s.scanMantissa()
	}

	if s.r == 'i' {
		tok = Imaginary
		s.next()
	}

	str := string(s.src[off:s.Offset])
	var value interface{}
	switch tok {
	case Int:
		i, ok := big.NewInt(0).SetString(str, 10)
		if ok {
			value = i
		} else {
			s.errorf("bad int literal: %q", str)
			tok = Unknown
		}
	case Float:
		f, ok := big.NewFloat(0).SetString(str)
		if ok {
			value = f
		} else {
			s.errorf("bad float literal: %q", str)
			tok = Unknown
		}
	case Imaginary:
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

func (s *Scanner) Next() error {
	defer func() {
		fmt.Printf("Scanner.Next s.Token=%s, s.Offset=%d, s.off=%d", s.Token, s.Offset, s.off)
		if s.Literal != nil {
			fmt.Printf(" Literal=%s", s.Literal)
		}
		fmt.Printf("\n")
	}()
	s.skipWhitespace()
	//fmt.Printf("Next: s.r=%v (%s)\n", s.r, string(s.r))

	s.Literal = nil
	r := s.r
	switch {
	case unicode.IsLetter(r):
		var ok bool
		lit := s.scanIdentifier()
		s.Token, ok = tokens[lit]
		if !ok {
			s.Token = Identifier
			s.Literal = lit
		}
		switch s.Token {
		case Identifier, Break, Continue, Fallthrough, Return:
			s.semi = true
		}
		return s.err
	case unicode.IsDigit(r):
		s.semi = true
		s.Token, s.Literal = s.scanNumber(false)
		return s.err
	}

	s.semi = false
	s.next()
	switch r {
	case -1:
		// TODO semicolon insertion could be done here
		s.Token = Unknown
		return io.EOF
	case '\n':
		s.semi = false
		s.Token = Semicolon
	case '(':
		s.Token = LeftParen
	case ')':
		s.semi = true
		s.Token = RightParen
	case '[':
		s.Token = LeftBracket
	case ']':
		s.semi = true
		s.Token = RightBracket
	case '{':
		s.Token = LeftBrace
	case '}':
		s.semi = true
		s.Token = RightBrace
	case ',':
		s.Token = Comma
	case ';':
		s.Token = Semicolon
	case ':':
		switch s.r {
		case '=':
			s.next()
			s.Token = Define
		default:
			s.Token = Colon
		}
	case '+':
		switch s.r {
		case '=':
			s.next()
			s.Token = AddAssign
		case '+':
			s.next()
			s.Token = Inc
		default:
			s.Token = Add
		}
	case '-':
		switch s.r {
		case '=':
			s.next()
			s.Token = SubAssign
		case '-':
			s.next()
			s.Token = Dec
		default:
			s.Token = Sub
		}
	case '*':
		switch s.r {
		case '=':
			s.next()
			s.Token = MulAssign
		default:
			s.Token = Mul
		}
	case '/':
		switch s.r {
		case '/', '*': // comment
			// TODO if s.semi and no more tokens on this line, insert newline
			s.Literal = s.scanComment()
			s.Token = Comment
		case '=':
			s.next()
			s.Token = DivAssign
		default:
			s.Token = Div
		}
	case '%':
		switch s.r {
		case '=':
			s.next()
			s.Token = RemAssign
		default:
			s.Token = Rem
		}
	case '^':
		switch s.r {
		case '=':
			s.next()
			s.Token = PowAssign
		default:
			s.Token = Pow
		}
	default:
		s.Token = Unknown
		s.err = fmt.Errorf("parser: unknown r=%v", r)
	}

	return s.err
}
