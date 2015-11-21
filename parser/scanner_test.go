// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"math/big"

	"neugram.io/lang/token"
)

type scannerTest struct {
	input       string
	token       token.Token
	literal     interface{}
	semiFollows bool
}

var scannerBothTests = []scannerTest{
	{"foo", token.Ident, "foo", true},
	{"+", token.Add, nil, false},
	{"*", token.Mul, nil, false},
	{"bar", token.Ident, "bar", true},
	{"9", token.Int, big.NewInt(9), true},
	{"+=", token.AddAssign, nil, false},
	{"++", token.Inc, nil, true},
	{"break", token.Break, nil, true},
	{"/* a block comment */", token.Comment, "/* a block comment */", false},
	{"(", token.LeftParen, nil, false},
	{"4.62e12", token.Float, big.NewFloat(4.62e12), true},
	{")", token.RightParen, nil, true},
	{"//final", token.Comment, "//final", false},
}

var scannerSepTests = append([]scannerTest{
	{"// a line comment", token.Comment, "// a line comment", false},
	{"      // spaced ", token.Comment, "// spaced ", false},
}, scannerBothTests...)

var scannerJoinTests = append([]scannerTest{}, scannerBothTests...)

/*
func TestScannerSep(t *testing.T) {
	for _, test := range scannerSepTests {
		s := NewScanner([]byte(test.input))
		if err := s.Next(); err != nil {
			t.Errorf("scanning %q: %v", test.input, err)
			continue
		}
		if s.Token != test.token {
			t.Errorf("%q: got %s, want %s", test.input, s.Token, test.token)
			continue
		}
		if !equalLiteral(s.Literal, test.literal) {
			t.Errorf("%q literal: got %s, want %s", test.input, s.Literal, test.literal)
			continue
		}
		if test.semiFollows {
			if err := s.Next(); err != nil {
				t.Errorf("%q: error expecting ';': %v", test.input, err)
				continue
			}
			if s.Token != token.Semicolon {
				t.Errorf("%q: expected ';', got: %v", test.input, s.Token)
				continue
			}
		}
		if err := s.Next(); err != io.EOF {
			t.Errorf("%q: expected EOF, got: %v", test.input, err)
		}
	}
}

func TestScannerJoin(t *testing.T) {
	var input []string
	for _, test := range scannerJoinTests {
		input = append(input, test.input)
	}
	s := NewScanner([]byte(strings.Join(input, " ")))
	for _, test := range scannerJoinTests {
		if err := s.Next(); err != nil {
			t.Fatalf("scanning for %q: %v", test.input, err)
		}
		if s.Token != test.token {
			t.Errorf("%q: got %s, want %s", test.input, s.Token, test.token)
		}
		if !equalLiteral(s.Literal, test.literal) {
			t.Errorf("%q literal: got %s, want %s", test.input, s.Literal, test.literal)
		}
	}
}
*/
