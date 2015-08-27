// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package typecheck

import (
	"testing"

	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/parser"
)

type identType struct {
	name string
	t    tipe.Type
}

type typeTest struct {
	stmts []string
	want  []identType
}

var typeTests = []typeTest{
	{
		[]string{"x := 4"},
		[]identType{{"x", tipe.Integer}},
	},
	{
		[]string{
			"x := 4 + 5 + 2",
			"y := x",
			"z := int64(x) + 2",
		},
		[]identType{
			{"y", tipe.Integer},
			{"z", tipe.Int64},
		},
	},
}

func TestBasic(t *testing.T) {
	for i, test := range typeTests {
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

		findDef := func(name string) *Obj {
			for ident, obj := range c.Defs {
				if ident.Name == name {
					return obj
				}
			}
			return nil
		}

		for _, want := range test.want {
			obj := findDef(want.name)
			if obj == nil {
				t.Errorf("%d: want %s=%s, is missing", i, want.name, want.t, want.name)
				continue
			}
			if !tipe.Equal(obj.Type, want.t) {
				t.Errorf("%d: want %s=%s, got %s", i, want.name, want.t, obj.Type)
			}
		}
	}
}
