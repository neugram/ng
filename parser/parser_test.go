// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"math/big"
	"testing"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
)

type parserTest struct {
	input string
	want  expr.Expr
}

var parserTests = []parserTest{
	{"foo", &expr.Ident{"foo"}},
	{"x + y", &expr.Binary{token.Add, &expr.Ident{"x"}, &expr.Ident{"y"}}},
	{
		"x + y + 9",
		&expr.Binary{
			token.Add,
			&expr.Binary{token.Add, &expr.Ident{"x"}, &expr.Ident{"y"}},
			&expr.BasicLiteral{big.NewInt(9)},
		},
	},
	{
		"x + (y + 7)",
		&expr.Binary{
			token.Add,
			&expr.Ident{"x"},
			&expr.Unary{
				Op: token.LeftParen,
				Expr: &expr.Binary{
					token.Add,
					&expr.Ident{"y"},
					&expr.BasicLiteral{big.NewInt(7)},
				},
			},
		},
	},
	{
		"x + y * z",
		&expr.Binary{
			token.Add,
			&expr.Ident{"x"},
			&expr.Binary{token.Mul, &expr.Ident{"y"}, &expr.Ident{"z"}},
		},
	},
	{"y * /* comment */ z", &expr.Binary{token.Mul, &expr.Ident{"y"}, &expr.Ident{"z"}}},
	// TODO {"y * z//comment", &expr.Binary{token.Mul, &expr.Ident{"y"}, &expr.Ident{"z"}}},
	{
		"quit()",
		&expr.Call{Func: &expr.Ident{Name: "quit"}},
	},
	{
		"foo(4)",
		&expr.Call{
			Func: &expr.Ident{Name: "foo"},
			Args: []expr.Expr{&expr.BasicLiteral{Value: big.NewInt(4)}},
		},
	},
	{
		"min(1, 2)",
		&expr.Call{
			Func: &expr.Ident{Name: "min"},
			Args: []expr.Expr{
				&expr.BasicLiteral{Value: big.NewInt(1)},
				&expr.BasicLiteral{Value: big.NewInt(2)},
			},
		},
	},
	{
		"func() int { return 7 }",
		&expr.FuncLiteral{
			Type: &tipe.Func{Out: []*tipe.Field{{Type: &tipe.Unresolved{"int"}}}},
			Body: &stmt.Block{[]stmt.Stmt{
				&stmt.Return{Exprs: []expr.Expr{&expr.BasicLiteral{big.NewInt(7)}}},
			}},
		},
	},
	{
		"func(x, y val) (r0 val, r1 val) { return x, y }",
		&expr.FuncLiteral{
			Type: &tipe.Func{
				In: []*tipe.Field{
					&tipe.Field{Name: "x", Type: &tipe.Unresolved{"val"}},
					&tipe.Field{Name: "y", Type: &tipe.Unresolved{"val"}},
				},
				Out: []*tipe.Field{
					&tipe.Field{Name: "r0", Type: &tipe.Unresolved{"val"}},
					&tipe.Field{Name: "r1", Type: &tipe.Unresolved{"val"}},
				},
			},
			Body: &stmt.Block{[]stmt.Stmt{
				&stmt.Return{Exprs: []expr.Expr{
					&expr.Ident{Name: "x"},
					&expr.Ident{Name: "y"},
				}},
			}},
		},
	},
	{
		`func() int64 {
			x := 7
			return x
		}`,
		&expr.FuncLiteral{
			Type: &tipe.Func{Out: []*tipe.Field{{Type: &tipe.Unresolved{"int64"}}}},
			Body: &stmt.Block{[]stmt.Stmt{
				&stmt.Assign{
					Left:  []expr.Expr{&expr.Ident{"x"}},
					Right: []expr.Expr{&expr.BasicLiteral{big.NewInt(7)}},
				},
				&stmt.Return{Exprs: []expr.Expr{&expr.Ident{"x"}}},
			}},
		},
	},
	{
		`func() int64 {
			if x := 9; x > 3 {
				return x
			} else {
				return 1-x
			}
		}`,
		&expr.FuncLiteral{
			Type: &tipe.Func{Out: []*tipe.Field{{Type: &tipe.Unresolved{"int64"}}}},
			Body: &stmt.Block{[]stmt.Stmt{&stmt.If{
				Init: &stmt.Assign{
					Left:  []expr.Expr{&expr.Ident{"x"}},
					Right: []expr.Expr{&expr.BasicLiteral{big.NewInt(9)}},
				},
				Cond: &expr.Binary{
					Op:    token.Greater,
					Left:  &expr.Ident{"x"},
					Right: &expr.BasicLiteral{big.NewInt(3)},
				},
				Body: &stmt.Block{Stmts: []stmt.Stmt{
					&stmt.Return{Exprs: []expr.Expr{&expr.Ident{"x"}}},
				}},
				Else: &stmt.Block{Stmts: []stmt.Stmt{
					&stmt.Return{Exprs: []expr.Expr{
						&expr.Binary{
							Op:    token.Sub,
							Left:  &expr.BasicLiteral{big.NewInt(1)},
							Right: &expr.Ident{"x"},
						},
					}},
				}},
			}}},
		},
	},
	{
		"func(x val) val { return 3+x }(1)",
		&expr.Call{
			Func: &expr.FuncLiteral{
				Type: &tipe.Func{
					In:  []*tipe.Field{{Name: "x", Type: &tipe.Unresolved{"val"}}},
					Out: []*tipe.Field{{Type: &tipe.Unresolved{"val"}}},
				},
				Body: &stmt.Block{[]stmt.Stmt{
					&stmt.Return{Exprs: []expr.Expr{
						&expr.Binary{
							Op:    token.Add,
							Left:  &expr.BasicLiteral{big.NewInt(3)},
							Right: &expr.Ident{"x"},
						},
					}},
				}},
			},
			Args: []expr.Expr{&expr.BasicLiteral{big.NewInt(1)}},
		},
	},
	{
		"func() { x = -x }",
		&expr.FuncLiteral{
			Type: &tipe.Func{},
			Body: &stmt.Block{[]stmt.Stmt{&stmt.Assign{
				Left:  []expr.Expr{&expr.Ident{"x"}},
				Right: []expr.Expr{&expr.Unary{Op: token.Sub, Expr: &expr.Ident{"x"}}},
			}}},
		},
	},
}

func TestParseExpr(t *testing.T) {
	for _, test := range parserTests {
		s, err := ParseStmt([]byte(test.input))
		if err != nil {
			t.Errorf("ParseExpr(%q): error: %v", test.input, err)
			continue
		}
		if s == nil {
			t.Errorf("ParseExpr(%q): nil stmt", test.input)
			continue
		}
		got := s.(*stmt.Simple).Expr
		if !EqualExpr(got, test.want) {
			t.Errorf("ParseExpr(%q):\n%v", test.input, Diff(test.want, got))
		}
	}
}
