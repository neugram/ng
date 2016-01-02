// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"fmt"
	"math/big"
	"testing"

	"neugram.io/lang/expr"
	"neugram.io/lang/stmt"
	"neugram.io/lang/tipe"
	"neugram.io/lang/token"
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
		"func() integer { return 7 }",
		&expr.FuncLiteral{
			Type: &tipe.Func{Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}}},
			Body: &stmt.Block{[]stmt.Stmt{
				&stmt.Return{Exprs: []expr.Expr{&expr.BasicLiteral{big.NewInt(7)}}},
			}},
		},
	},
	{
		"func(x, y val) (r0 val, r1 val) { return x, y }",
		&expr.FuncLiteral{
			Type: &tipe.Func{
				Params: &tipe.Tuple{Elems: []tipe.Type{
					&tipe.Unresolved{Name: "val"},
					&tipe.Unresolved{Name: "val"},
				}},
				Results: &tipe.Tuple{Elems: []tipe.Type{
					&tipe.Unresolved{Name: "val"},
					&tipe.Unresolved{Name: "val"},
				}},
			},
			ParamNames:  []string{"x", "y"},
			ResultNames: []string{"r0", "r1"},
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
			Type:        &tipe.Func{Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}}},
			ResultNames: []string{""},
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
			Type:        &tipe.Func{Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Int64}}},
			ResultNames: []string{""},
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
					Params:  &tipe.Tuple{Elems: []tipe.Type{&tipe.Unresolved{Name: "val"}}},
					Results: &tipe.Tuple{Elems: []tipe.Type{&tipe.Unresolved{Name: "val"}}},
				},
				ParamNames:  []string{""},
				ResultNames: []string{""},
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
	{"x.y.z", &expr.Selector{&expr.Selector{&expr.Ident{"x"}, &expr.Ident{"y"}}, &expr.Ident{"z"}}},
	//{"y * /* comment */ z", &expr.Binary{token.Mul, &expr.Ident{"y"}, &expr.Ident{"z"}}},
	//TODO{"y * z//comment", &expr.Binary{token.Mul, &expr.Ident{"y"}, &expr.Ident{"z"}}},
	{`"hello"`, &expr.BasicLiteral{"hello"}},
	{`"hello \"neugram\""`, &expr.BasicLiteral{`hello "neugram"`}},
	//TODO{`"\""`, &expr.BasicLiteral{`"\""`}}
	{"x[4]", &expr.Index{Expr: &expr.Ident{"x"}, Index: basic(4)}},
	{"x[1+2]", &expr.Index{
		Expr: &expr.Ident{"x"},
		Index: &expr.Binary{Op: token.Add,
			Left:  basic(1),
			Right: basic(2),
		},
	}},
	/* {"x[1:3]", &expr.TableIndex{Expr: &expr.Ident{"x"}, Cols: expr.Range{Start: &expr.BasicLiteral{big.NewInt(1)}, End: &expr.BasicLiteral{big.NewInt(3)}}}},
	{"x[1:]", &expr.TableIndex{Expr: &expr.Ident{"x"}, Cols: expr.Range{Start: &expr.BasicLiteral{big.NewInt(1)}}}},
	{"x[:3]", &expr.TableIndex{Expr: &expr.Ident{"x"}, Cols: expr.Range{End: &expr.BasicLiteral{big.NewInt(3)}}}},
	{"x[:]", &expr.TableIndex{Expr: &expr.Ident{"x"}}},
	{"x[,:]", &expr.TableIndex{Expr: &expr.Ident{"x"}}},
	{"x[:,:]", &expr.TableIndex{Expr: &expr.Ident{"x"}}},
	{`x["C1"|"C2"]`, &expr.TableIndex{Expr: &expr.Ident{"x"}, ColNames: []string{"C1", "C2"}}},
	{`x["C1",1:]`, &expr.TableIndex{
		Expr:     &expr.Ident{"x"},
		ColNames: []string{"C1"},
		Rows:     expr.Range{Start: &expr.BasicLiteral{big.NewInt(1)}},
	}},
	{"x[1:3,5:7]", &expr.TableIndex{
		Expr: &expr.Ident{"x"},
		Cols: expr.Range{Start: &expr.BasicLiteral{big.NewInt(1)}, End: &expr.BasicLiteral{big.NewInt(3)}},
		Rows: expr.Range{Start: &expr.BasicLiteral{big.NewInt(5)}, End: &expr.BasicLiteral{big.NewInt(7)}},
	}},

	/*{"[|]num{}", &expr.TableLiteral{Type: &tipe.Table{tipe.Num}}},
	{"[|]num{{0, 1, 2}}", &expr.TableLiteral{
		Type: &tipe.Table{tipe.Num},
		Rows: [][]expr.Expr{{basic(0), basic(1), basic(2)}},
	}},
	{`[|]num{{|"Col1"|}, {1}, {2}}`, &expr.TableLiteral{
		Type:     &tipe.Table{tipe.Num},
		ColNames: []expr.Expr{basic("Col1")},
		Rows:     [][]expr.Expr{{basic(1)}, {basic(2)}},
	}},
	*/

	{`($$ ls -l $$)`, &expr.Unary{Op: token.LeftParen, Expr: &expr.Shell{
		Cmds: []*expr.ShellList{{
			Segment: expr.SegmentSemi,
			List:    []expr.Expr{&expr.ShellCmd{Argv: []string{"ls", "-l"}}},
		}},
	}}},
	{`($$ ls | head $$)`, &expr.Unary{Op: token.LeftParen, Expr: &expr.Shell{
		Cmds: []*expr.ShellList{{
			Segment: expr.SegmentPipe,
			List: []expr.Expr{
				&expr.ShellCmd{Argv: []string{"ls"}},
				&expr.ShellCmd{Argv: []string{"head"}},
			},
		}},
	}}},
	{`($$ ls > flist $$)`, &expr.Unary{Op: token.LeftParen, Expr: &expr.Shell{
		Cmds: []*expr.ShellList{{
			List: []expr.Expr{&expr.ShellList{
				Segment:  expr.SegmentOut,
				Redirect: "flist",
				List:     []expr.Expr{&expr.ShellCmd{Argv: []string{"ls"}}},
			}},
		}},
	}}},
	// TODO: (echo one; echo two > f)
	// TODO echo hi | cat && true
	// TODO true && echo hi | cat
	{`($$
	echo one
	echo two > f
	echo 3
	echo -n 4
	echo 5 | wc
	$$)`, &expr.Unary{Op: token.LeftParen, Expr: &expr.Shell{
		Cmds: []*expr.ShellList{{
			Segment: expr.SegmentSemi,
			List: []expr.Expr{
				&expr.ShellCmd{Argv: []string{"echo", "one"}},
				&expr.ShellList{
					Segment:  expr.SegmentOut,
					Redirect: "f",
					List:     []expr.Expr{&expr.ShellCmd{Argv: []string{"echo", "two"}}},
				},
				&expr.ShellCmd{Argv: []string{"echo", "3"}},
				&expr.ShellCmd{Argv: []string{"echo", "-n", "4"}},
				&expr.ShellList{
					Segment: expr.SegmentPipe,
					List: []expr.Expr{
						&expr.ShellCmd{Argv: []string{"echo", "5"}},
						&expr.ShellCmd{Argv: []string{"wc"}},
					},
				},
			},
		}},
	}}},
}

func TestParseExpr(t *testing.T) {
	for _, test := range parserTests {
		fmt.Printf("Parsing %q\n", test.input)
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
			t.Errorf("ParseExpr(%q):\n%v", test.input, DiffExpr(test.want, got))
		}
	}
}

type stmtTest struct {
	input string
	want  stmt.Stmt
}

var stmtTests = []stmtTest{
	{"for {}", &stmt.For{Body: &stmt.Block{}}},
	{"for ;; {}", &stmt.For{Body: &stmt.Block{}}},
	{"for true {}", &stmt.For{Cond: &expr.Ident{"true"}, Body: &stmt.Block{}}},
	{"for ; true; {}", &stmt.For{Cond: &expr.Ident{"true"}, Body: &stmt.Block{}}},
	{"for range x {}", &stmt.Range{Expr: &expr.Ident{"x"}, Body: &stmt.Block{}}},
	{"for k, v := range x {}", &stmt.Range{
		Key:  &expr.Ident{"k"},
		Val:  &expr.Ident{"v"},
		Expr: &expr.Ident{"x"},
		Body: &stmt.Block{},
	}},
	{"for k := range x {}", &stmt.Range{
		Key:  &expr.Ident{"k"},
		Expr: &expr.Ident{"x"},
		Body: &stmt.Block{},
	}},
	{
		"for i := 0; i < 10; i++ { x = i }",
		&stmt.For{
			Init: &stmt.Assign{
				Decl:  true,
				Left:  []expr.Expr{&expr.Ident{"i"}},
				Right: []expr.Expr{&expr.BasicLiteral{big.NewInt(0)}},
			},
			Cond: &expr.Binary{
				Op:    token.Less,
				Left:  &expr.Ident{"i"},
				Right: &expr.BasicLiteral{big.NewInt(10)},
			},
			Post: &stmt.Assign{
				Left: []expr.Expr{&expr.Ident{"i"}},
				Right: []expr.Expr{
					&expr.Binary{
						Op:    token.Add,
						Left:  &expr.Ident{"i"},
						Right: &expr.BasicLiteral{big.NewInt(1)},
					},
				},
			},
			Body: &stmt.Block{Stmts: []stmt.Stmt{&stmt.Assign{
				Left:  []expr.Expr{&expr.Ident{"x"}},
				Right: []expr.Expr{&expr.Ident{"i"}},
			}}},
		},
	},
	{"const x = 4", &stmt.Const{Name: "x", Value: &expr.BasicLiteral{big.NewInt(4)}}},
	{"x.y", &stmt.Simple{&expr.Selector{&expr.Ident{"x"}, &expr.Ident{"y"}}}},
	{
		"const x int64 = 4",
		&stmt.Const{
			Name:  "x",
			Type:  tipe.Int64,
			Value: &expr.BasicLiteral{big.NewInt(4)},
		},
	},
	{
		`type A integer`,
		&stmt.TypeDecl{Name: "A", Type: tipe.Integer},
	},
	{
		`type S struct { x integer }`,
		&stmt.TypeDecl{
			Name: "S",
			Type: &tipe.Struct{
				FieldNames: []string{"x"},
				Fields:     []tipe.Type{tipe.Integer},
			},
		},
	},
	{
		`methodik AnInt integer {
			func (a) f() integer { return a }
		}
		`,
		&stmt.MethodikDecl{
			Name: "AnInt",
			Type: &tipe.Methodik{
				Type:        tipe.Integer,
				MethodNames: []string{"f"},
				Methods: []tipe.Type{
					&tipe.Func{
						Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
					},
				},
			},
			Methods: []*expr.FuncLiteral{{
				Name:         "f",
				ReceiverName: "a",
				Type: &tipe.Func{
					Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
				},
				Body: &stmt.Block{Stmts: []stmt.Stmt{
					&stmt.Return{Exprs: []expr.Expr{&expr.Ident{"a"}}},
				}},
			}},
		},
	},
	{
		`methodik T *struct{
			x integer
			y [|]int64
		} {
			func (a) f(x integer) integer {
				return a.x
			}
		}
		`,
		&stmt.MethodikDecl{
			Name: "T",
			Type: &tipe.Methodik{
				Type: &tipe.Struct{
					FieldNames: []string{"x", "y"},
					Fields:     []tipe.Type{tipe.Integer, &tipe.Table{tipe.Int64}},
				},
				MethodNames: []string{"f"},
				Methods: []tipe.Type{
					&tipe.Func{
						Params:  &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
						Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
					},
				},
			},
			Methods: []*expr.FuncLiteral{{
				Name:            "f",
				ReceiverName:    "a",
				PointerReceiver: true,
				Type: &tipe.Func{
					Params:  &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
					Results: &tipe.Tuple{Elems: []tipe.Type{tipe.Integer}},
				},
				ParamNames: []string{"x"},
				Body: &stmt.Block{Stmts: []stmt.Stmt{
					&stmt.Return{Exprs: []expr.Expr{&expr.Selector{
						Left:  &expr.Ident{"a"},
						Right: &expr.Ident{"x"},
					}}},
				}},
			}},
		},
	},
	{"S{ X: 7 }", &stmt.Simple{&expr.CompLiteral{
		Type:     &tipe.Unresolved{Name: "S"},
		Keys:     []expr.Expr{&expr.Ident{"X"}},
		Elements: []expr.Expr{&expr.BasicLiteral{big.NewInt(7)}},
	}}},
	{`map[string]string{ "foo": "bar" }`, &stmt.Simple{&expr.MapLiteral{
		Type:   &tipe.Map{Key: tipe.String, Value: tipe.String},
		Keys:   []expr.Expr{basic("foo")},
		Values: []expr.Expr{basic("bar")},
	}}},
	{"x.y", &stmt.Simple{&expr.Selector{&expr.Ident{"x"}, &expr.Ident{"y"}}}},
	{"sync.Mutex{}", &stmt.Simple{&expr.CompLiteral{
		Type: &tipe.Unresolved{Package: "sync", Name: "Mutex"},
	}}},
}

func TestParseStmt(t *testing.T) {
	for _, test := range stmtTests {
		fmt.Printf("Parsing stmt %q\n", test.input)
		got, err := ParseStmt([]byte(test.input))
		if err != nil {
			t.Errorf("ParseStmt(%q): error: %v", test.input, err)
			continue
		}
		if got == nil {
			t.Errorf("ParseStmt(%q): nil stmt", test.input)
			continue
		}
		if !EqualStmt(got, test.want) {
			t.Errorf("ParseStmt(%q):\n%v", test.input, DiffStmt(test.want, got))
		}
	}
}

func basic(x interface{}) *expr.BasicLiteral {
	switch x := x.(type) {
	case int:
		return &expr.BasicLiteral{big.NewInt(int64(x))}
	case int64:
		return &expr.BasicLiteral{big.NewInt(x)}
	case string:
		return &expr.BasicLiteral{x}
	default:
		panic(fmt.Sprintf("unknown basic %v (%T)", x, x))
	}
}
