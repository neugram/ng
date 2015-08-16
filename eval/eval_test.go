// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"
	"testing"

	"numgrad.io/parser"
)

func TestTrivialEval(t *testing.T) {
	s := &Scope{
		Ident: map[string]interface{}{
			"x": big.NewInt(4),
			"y": big.NewInt(5),
		},
	}
	expr := mustParse("2+3*(x+y-2)")
	res, err := Eval(s, expr)
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

func mustParse(src string) parser.Expr {
	expr, err := parser.ParseExpr([]byte(src))
	if err != nil {
		panic(fmt.Sprintf("mustParse: %v", err))
	}
	return expr
}
