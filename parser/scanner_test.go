// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"io"
	"math/big"
	"strings"
	"testing"

	"numgrad.io/lang/token"
)

type scannerTest struct {
	input   string
	token   token.Token
	literal interface{}
}

var scannerBothTests = []scannerTest{
	{"foo", token.Ident, "foo"},
	{"+", token.Add, nil},
	{"*", token.Mul, nil},
	{"bar", token.Ident, "bar"},
	{"9", token.Int, big.NewInt(9)},
	{"+=", token.AddAssign, nil},
	{"++", token.Inc, nil},
	{"break", token.Break, nil},
	{"/* a block comment */", token.Comment, "/* a block comment */"},
	{"(", token.LeftParen, nil},
	{"4.62e12", token.Float, big.NewFloat(4.62e12)},
	{")", token.RightParen, nil},
	{"//final", token.Comment, "//final"},
}

var scannerSepTests = append([]scannerTest{
	{"// a line comment", token.Comment, "// a line comment"},
	{"      // spaced ", token.Comment, "// spaced "},
}, scannerBothTests...)

var scannerJoinTests = append([]scannerTest{}, scannerBothTests...)

func TestScannerSep(t *testing.T) {
	for _, test := range scannerSepTests {
		s := NewScanner([]byte(test.input))
		if err := s.Next(); err != nil {
			t.Errorf("scanning %q: %v", test.input, err)
			continue
		}
		if s.Token != test.token {
			t.Errorf("%q: got %s, want %s", test.input, s.Token, test.token)
		}
		if !equalLiteral(s.Literal, test.literal) {
			t.Errorf("%q literal: got %s, want %s", test.input, s.Literal, test.literal)
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
