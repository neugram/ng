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
		[]string{"x := int64(4)"},
		[]identType{{"x", tipe.Int64}},
	},
	{
		[]string{"x := int64(4) + 2"},
		[]identType{{"x", tipe.Int64}},
	},
	{
		[]string{"x := 4"},
		[]identType{{"x", tipe.Num}},
	},
	{
		[]string{
			"x := 4 + 5 + 2",
			"y := x",
			"z := int64(x) + 2",
		},
		[]identType{
			{"y", tipe.Num},
			{"z", tipe.Int64},
		},
	},
	{
		[]string{
			"x := 4 + 5 + 2",
			`y := "foo"`,
			`{
				y := x
				t := y
			}`,
			`z := y`,
		},
		[]identType{
			{"y", tipe.String},
			{"z", tipe.String},
		},
	},
	{
		[]string{
			"add := func(x, y integer) int64 { return int64(x) + int64(y) }",
			"x := add(3, 4)",
		},
		[]identType{
			{"x", tipe.Int64},
		},
	},
	{
		[]string{
			`type A class {
				X int64
			}`,
			`a := A{34, 2}`, // TODO type error
		},
		[]identType{{"a", &tipe.Class{Tags: []string{"X"}, Fields: []tipe.Type{tipe.Int64}}}},
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
		for si, s := range stmts {
			t.Logf("Add((%p)%s)", s, s.Sexp())
			c.Add(s)
			if len(c.Errs) > 0 {
				t.Fatalf("%d: Add(%q): %v", i, test.stmts[si], c.Errs[0])
			}
		}
		t.Logf("%s", c)

		findDef := func(name string) *Obj {
			return c.cur.Objs[name]
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
