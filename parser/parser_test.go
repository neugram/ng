// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"math/big"
	"testing"
)

type parserTest struct {
	input string
	want  Expr
}

var parserTests = []parserTest{
	{"foo", &Ident{"foo"}},
	{"x + y", &BinaryExpr{Add, &Ident{"x"}, &Ident{"y"}}},
	{
		"x + y + 9",
		&BinaryExpr{
			Add,
			&BinaryExpr{Add, &Ident{"x"}, &Ident{"y"}},
			&BasicLiteral{big.NewInt(9)},
		},
	},
	{
		"x + (y + 7)",
		&BinaryExpr{
			Add,
			&Ident{"x"},
			&UnaryExpr{
				Op: LeftParen,
				Expr: &BinaryExpr{
					Add,
					&Ident{"y"},
					&BasicLiteral{big.NewInt(7)},
				},
			},
		},
	},
	{"y * /* comment */ z", &BinaryExpr{Mul, &Ident{"y"}, &Ident{"z"}}},
	{"y * z//comment", &BinaryExpr{Mul, &Ident{"y"}, &Ident{"z"}}},
	{
		"x + y * z",
		&BinaryExpr{
			Add,
			&Ident{"x"},
			&BinaryExpr{Mul, &Ident{"y"}, &Ident{"z"}},
		},
	},
}

func TestParseExpr(t *testing.T) {
	for _, test := range parserTests {
		got, err := ParseExpr([]byte(test.input))
		if err != nil {
			t.Errorf("%q: %v", test.input, err)
			continue
		}
		if !EqualExpr(got, test.want) {
			t.Errorf("%q:\n%v", test.input, Diff(test.want, got))
		}
	}
}
