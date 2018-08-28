// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"testing"

	"neugram.io/ng/format"
	"neugram.io/ng/parser"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
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
		[]identType{{"x", tipe.Int}},
	},
	{
		[]string{
			"x := 4 + 5 + 2",
			"y := x",
			"z := int64(x) + 2",
		},
		[]identType{
			{"y", tipe.Int},
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
			"f := func() int64 { return 7 }",
			"func g() int64 { return 7 }",
		},
		[]identType{
			{"f", &tipe.Func{Params: &tipe.Tuple{}, Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}}}},
			{"g", &tipe.Func{Params: &tipe.Tuple{}, Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}}}},
		},
	},
	{
		[]string{
			"add := func(x, y int64) int64 { return int64(x) + int64(y) }",
			"x := add(3, 4)",
		},
		[]identType{
			{"x", tipe.Int64},
		},
	},
	{
		[]string{
			`type A struct {
				X float64
			}`,
			`a := A{34.1}`,
			`b := a.X`,
		},
		[]identType{
			{"a", &tipe.Named{Name: "A", Type: &tipe.Struct{Fields: []tipe.StructField{{Name: "X", Type: tipe.Float64}}}}},
			{"b", tipe.Float64},
		},
	},
	{
		[]string{
			`a := [|]int64{{|"Col1","Col2"|}, {1, 2}, {3, 4}}`,
		},
		[]identType{{"a", &tipe.Table{Type: tipe.Int64}}},
	},
	{
		[]string{
			`methodik A struct{ X int64 } {
				func (a) Y() int64 { return a.X }
				func (a) Z() int64 { return a.Y() + a.X }
			}
			`,
			`a := A{34}`,
			`z := a.Z()`,
		},
		[]identType{{"z", tipe.Int64}},
	},
	{
		[]string{
			`err := error(nil)`,
			`err = nil`,
		},
		[]identType{{"err", Universe.Objs["error"].Type}},
	},
	{
		[]string{"x := int32(int64(16))"},
		[]identType{{"x", tipe.Int32}},
	},
	{
		[]string{
			`x, err := $$ echo hi $$`,
			`y := $$ echo hi $$`,
		},
		[]identType{
			{"x", tipe.String},
			{"err", Universe.Objs["error"].Type},
			{"y", tipe.String},
		},
	},
	{
		[]string{
			`x := []int64{1,2}`,
			`y := x[0]`,
			`z := x[0:1]`,
		},
		[]identType{
			{"x", &tipe.Slice{Elem: tipe.Int64}},
			{"y", tipe.Int64},
			{"z", &tipe.Slice{Elem: tipe.Int64}},
		},
	},
	{
		[]string{
			`type A interface {
				M() int64
				N(x, y int8) (int32, error)
			}`,
			`type B interface { M() int64 }`,
			`a := A(nil)`,
			`b := B(a)`,
			`m := b.M()`,
		},
		[]identType{
			{"a", &tipe.Named{
				Name: "A",
				Type: &tipe.Interface{Methods: map[string]*tipe.Func{
					"M": {
						Params:  &tipe.Tuple{},
						Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}},
					},
					"N": {
						Params: &tipe.Tuple{Elems: []tipe.Type{
							tipe.Int8, tipe.Int8,
						}},
						Results: &tipe.Tuple{Elems: []tipe.Type{
							tipe.Int32, Universe.Objs["error"].Type,
						}},
					},
				}},
			}},
			{"b", &tipe.Named{
				Name: "B",
				Type: &tipe.Interface{Methods: map[string]*tipe.Func{
					"M": {
						Params:  &tipe.Tuple{},
						Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}},
					},
				}},
			}},
			{"m", tipe.Int64},
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

		c := New("")
		for si, s := range stmts {
			c.Add(s)
			if errs := c.Errs(); len(errs) > 0 {
				t.Fatalf("%d: Add(%q): %v", i, test.stmts[si], errs[0])
			}
		}
		//t.Logf("%s", c)

		findDef := func(name string) *Obj {
			return c.cur.Objs[name]
		}

		for _, want := range test.want {
			obj := findDef(want.name)
			if obj == nil {
				t.Errorf("%d: want %s=%s, is missing", i, want.name, want.t)
				continue
			}
			if !tipe.Equal(obj.Type, want.t) {
				t.Errorf("%d: want %s=%s, got %s", i, want.name, format.Type(want.t), format.Type(obj.Type))
			}
		}
	}
}
