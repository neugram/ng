// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"io"
	"math/big"
	"strings"
	"testing"
)

type scannerTest struct {
	input   string
	token   Token
	literal interface{}
}

var scannerBothTests = []scannerTest{
	{"foo", Identifier, "foo"},
	{"+", Add, nil},
	{"*", Mul, nil},
	{"bar", Identifier, "bar"},
	{"9", Int, big.NewInt(9)},
	{"+=", AddAssign, nil},
	{"++", Inc, nil},
	{"break", Break, nil},
	{"/* a block comment */", Comment, "/* a block comment */"},
	{"(", LeftParen, nil},
	{"4.62e12", Float, big.NewFloat(4.62e12)},
	{")", RightParen, nil},
	{"//final", Comment, "//final"},
}

var scannerSepTests = append([]scannerTest{
	{"// a line comment", Comment, "// a line comment"},
	{"      // spaced ", Comment, "// spaced "},
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
