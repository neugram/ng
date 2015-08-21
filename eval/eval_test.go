// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"
	"testing"

	"numgrad.io/lang/stmt"
	"numgrad.io/parser"
)

var exprTests = []struct {
	stmt string
	want *big.Int
}{
	{"2+3*(x+y-2)", big.NewInt(23)},
	{"func() val { return 7 }()", big.NewInt(7)},
	{
		`func() val {
			if x > 2 {
				return z+1
			} else {
				return z-1
			}
		}()`,
		big.NewInt(8),
	},
	{
		`func() val {
			x := 9
			x++
			if x > 5 {
				x = -x
			}
			return x
		}()`,
		big.NewInt(-10),
	},
	{
		`func() val {
			x++
			return x
		}()`,
		big.NewInt(5),
	},
}

func mkBasicProgram() *Program {
	s := &Scope{
		Var: map[string]*Variable{
			"x": &Variable{big.NewInt(4)},
			"y": &Variable{big.NewInt(5)},
		},
		Parent: &Scope{
			Var: map[string]*Variable{
				"x": &Variable{big.NewInt(1)}, // shadowed by child scope
				"z": &Variable{big.NewInt(7)},
			},
		},
	}
	return &Program{
		Pkg: map[string]*Scope{
			"main": s,
		},
	}
}

func TestExprs(t *testing.T) {
	for _, test := range exprTests {
		p := mkBasicProgram()
		s := mustParse(test.stmt)
		res, err := p.Eval(s)
		if err != nil {
			t.Errorf("Eval(%s) error: %v", s.Sexp(), err)
		}
		if len(res) != 1 {
			t.Errorf("Eval(%s) want *big.Int, got multi-valued (%d) result: %v", s.Sexp(), len(res), res)
			continue
		}
		fmt.Printf("Returning Eval: %#+v\n", res)
		got, ok := res[0].(*big.Int)
		if !ok {
			t.Errorf("Eval(%s) want *big.Int, got: %s (%T)", s.Sexp(), got, got)
			continue
		}
		if test.want.Cmp(got) != 0 {
			t.Errorf("Eval(%s)=%s, want %s", s.Sexp(), got, test.want)
		}
	}
}

/*
func TestTrivialEval(t *testing.T) {
	p := &Program{
		Pkg: map[string]*Scope{
			"main": &Scope{Var: map[string]*Variable{
				"x": &Variable{big.NewInt(4)},
				"y": &Variable{big.NewInt(5)},
			}},
		},
	}
	s := mustParse("2+3*(x+y-2)")
	res, err := p.Eval(s)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := res.(*big.Int)
	if !ok {
		t.Fatalf("Eval(%s) want *big.Int, got: %s (%T)", expr, got, got)
	}
	if want := big.NewInt(23); want.Cmp(got) != 0 {
		t.Errorf("Eval(%s)=%s, want %s", expr, got, want)
	}
}
*/

func mustParse(src string) stmt.Stmt {
	expr, err := parser.ParseStmt([]byte(src))
	if err != nil {
		panic(fmt.Sprintf("mustParse(%q): %v", src, err))
	}
	return expr
}
