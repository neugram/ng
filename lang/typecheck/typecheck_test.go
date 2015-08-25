// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package typecheck

import (
	"testing"

	"numgrad.io/lang/stmt"
	"numgrad.io/parser"
)

type typeTest struct {
	stmts []string
	want  interface{} // TODO
}

var typeTests = []typeTest{
	{
		[]string{
			"x := 4 + 5 + 2",
			"y := x",
			"z := int64(x) + 2",
		},
		nil,
	},
}

func TestBasic(t *testing.T) {
	for _, test := range typeTests {
		var stmts []stmt.Stmt
		for _, str := range test.stmts {
			s, err := parser.ParseStmt([]byte(str))
			if err != nil {
				t.Fatalf("parser.ParseStmt(%q): %v", str, err)
			}
			stmts = append(stmts, s)
		}

		c := New()
		for _, s := range stmts {
			t.Logf("Add((%p)%s)", s, s.Sexp())
			c.Add(s)
		}
		t.Logf("%s", c)
		t.Errorf("TODO")
	}
}
