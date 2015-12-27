// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"
	"testing"

	"neugram.io/lang/stmt"
	"neugram.io/parser"
)

var exprTests = []struct {
	stmt string
	want *big.Int
}{
	{"2+3*(x+y-2)", big.NewInt(23)},
	{"func() num { return 7 }()", big.NewInt(7)},
	{
		// TODO: I believe our typechecking of this is woefully incomplete.
		// When the closure func() num is declared, it delcares a new num
		// parameter. But it closes over z, which inherits the num
		// parameter from the outer scope. For z to be silently used as a
		// num here, we are tying the two type parameters together. That's
		// kind-of a big deal.
		//
		`func() num {
			if x > 2 && x < 500 {
				return z+1
			} else {
				return z-1
			}
		}()`,
		big.NewInt(8),
	},
	{
		`func() num {
			x := 9
			x++
			if x > 5 {
				x = -x
			}
			return x
		}()`,
		big.NewInt(-10),
	},
	/* TODO: true {
		`func() num {
			f := func() bool {
				x++
				return true
			}
			if x == 4 || f() {
				x += 4
			}
			if x == 1 && f() {
				x *= 4
			}
			return x
		}()`,
		big.NewInt(8),
	},*/
	{
		`func() num {
			v := 2
			for i := 1; i < 4; i++ {
				v *= i
			}
			return v
		}()`,
		big.NewInt(12),
	},
	/*{
		`func() val {
			v := 2
			for {
				v++
				break
				v++
			}
			return v
		}()`,
		big.NewInt(3),
	},*/
}

func mkBasicProgram() (*Program, error) {
	p := New()
	if _, _, err := p.Eval(mustParse("x := 4")); err != nil {
		return nil, err
	}
	if _, _, err := p.Eval(mustParse("y := 5")); err != nil {
		return nil, err
	}
	if _, _, err := p.Eval(mustParse("z := 7")); err != nil {
		return nil, err
	}
	return p, nil
}

func TestExprs(t *testing.T) {
	for _, test := range exprTests {
		p, err := mkBasicProgram()
		if err != nil {
			t.Fatalf("mkBasicProgram: %v", err)
		}
		s := mustParse(test.stmt)
		res, _, err := p.Eval(s)
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
