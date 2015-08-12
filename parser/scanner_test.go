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

var scannerTests = []scannerTest{
	{"foo", Ident, "foo"},
	{"9", Int, big.NewInt(9)},
	{"+", Add, nil},
	{"+=", AddAssign, nil},
	{"++", Inc, nil},
	{"break", Break, nil},
	{"(", LeftParen, nil},
	{"4.62e12", Float, big.NewFloat(4.62e12)},
	{")", RightParen, nil},
}

func eq(lit0, lit1 interface{}) bool {
	//fmt.Printf("lit0=%v (%T), lit1=%v (%T)\n", lit0, lit0, lit1, lit1)
	if lit0 == lit1 {
		return true
	}
	switch lit0 := lit0.(type) {
	case *big.Int:
		if lit1, ok := lit1.(*big.Int); ok {
			return lit0.Cmp(lit1) == 0
		}
	case *big.Float:
		if lit1, ok := lit1.(*big.Float); ok {
			return lit0.Cmp(lit1) == 0
		}
	}
	return false
}

func TestScannerInTheSmall(t *testing.T) {
	for _, test := range scannerTests {
		s := NewScanner([]byte(test.input))
		if err := s.Next(); err != nil {
			t.Errorf("scanning %q: %v", test.input, err)
			continue
		}
		if s.Token != test.token {
			t.Errorf("%q: got %s, want %s", test.input, s.Token, test.token)
		}
		if !eq(s.Literal, test.literal) {
			t.Errorf("%q literal: got %s, want %s", test.input, s.Literal, test.literal)
		}
		if err := s.Next(); err != io.EOF {
			t.Errorf("%q: expected EOF, got: %v", test.input, err)
		}
	}
}

func TestScannerJoinedSeq(t *testing.T) {
	var input []string
	for _, test := range scannerTests {
		input = append(input, test.input)
	}
	s := NewScanner([]byte(strings.Join(input, " ")))
	for _, test := range scannerTests {
		if err := s.Next(); err != nil {
			t.Fatalf("scanning for %q: %v", test.input, err)
		}
		if s.Token != test.token {
			t.Errorf("%q: got %s, want %s", test.input, s.Token, test.token)
		}
		if !eq(s.Literal, test.literal) {
			t.Errorf("%q literal: got %s, want %s", test.input, s.Literal, test.literal)
		}
	}
}
