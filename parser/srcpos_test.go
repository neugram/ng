// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"math/big"
	"reflect"
	"testing"

	"neugram.io/ng/format"
	"neugram.io/ng/syntax"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/src"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
)

var srcposInput = `ch := make(chan int)
go func() {
	ch <- 41 + 1
	close(ch)
}()
if v, ok := <- ch; ok {
	print(v)
}
`

var srcposWant = &syntax.File{
	Filename: "srctest.ng",
	Stmts: []stmt.Stmt{
		&stmt.Assign{
			Position: src.Pos{
				Filename: "", // TODO
			},
			Decl: bool(true),
			Left: []expr.Expr{
				&expr.Ident{
					Position: src.Pos{
						Filename: "", // TODO
					},
					Name: "ch",
				},
			},
			Right: []expr.Expr{
				&expr.Call{
					Position: src.Pos{
						Filename: "", // TODO
					},
					Func: &expr.Ident{
						Position: src.Pos{
							Filename: "", // TODO
						},
						Name: "make",
					},
					Args: []expr.Expr{
						&expr.Type{
							Position: src.Pos{
								Filename: "", // TODO
							},
							Type: &tipe.Chan{
								Elem: &tipe.Unresolved{
									Package: "",
									Name:    "int",
								},
							},
						},
					},
				},
			},
		},
		&stmt.Go{
			Position: src.Pos{
				Filename: "", // TODO
			},
			Call: &expr.Call{
				Position: src.Pos{
					Filename: "", // TODO
				},
				Func: &expr.FuncLiteral{
					Position: src.Pos{
						Filename: "", // TODO
					},
					Name:         "",
					ReceiverName: "",
					Type: &tipe.Func{
						Spec: tipe.Specialization{
							Num: "",
						},
						Params: &tipe.Tuple{},
					},
					Body: &stmt.Block{
						Position: src.Pos{
							Filename: "", // TODO
						},
						Stmts: []stmt.Stmt{
							&stmt.Send{
								Position: src.Pos{
									Filename: "", // TODO
								},
								Chan: &expr.Ident{
									Position: src.Pos{
										Filename: "", // TODO
									},
									Name: "ch",
								},
								Value: &expr.Binary{
									Position: src.Pos{
										Filename: "srctest.ng",
										Line:     int32(3),
										Column:   int16(12),
									},
									Op: token.Add,
									Left: &expr.BasicLiteral{
										Position: src.Pos{
											Filename: "", // TODO
										},
										Value: big.NewInt(41),
									},
									Right: &expr.BasicLiteral{
										Position: src.Pos{
											Filename: "", // TODO
										},
										Value: big.NewInt(1),
									},
								},
							},
							&stmt.Simple{
								Position: src.Pos{
									Filename: "", // TODO
								},
								Expr: &expr.Call{
									Position: src.Pos{
										Filename: "", // TODO
									},
									Func: &expr.Ident{
										Position: src.Pos{
											Filename: "", // TODO
										},
										Name: "close",
									},
									Args: []expr.Expr{
										&expr.Ident{
											Position: src.Pos{
												Filename: "", // TODO
											},
											Name: "ch",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		&stmt.If{
			Position: src.Pos{
				Filename: "", // TODO
			},
			Init: &stmt.Assign{
				Position: src.Pos{
					Filename: "", // TODO
				},
				Decl: bool(true),
				Left: []expr.Expr{
					&expr.Ident{
						Position: src.Pos{
							Filename: "", // TODO
						},
						Name: "v",
					},
					&expr.Ident{
						Position: src.Pos{
							Filename: "", // TODO
						},
						Name: "ok",
					},
				},
				Right: []expr.Expr{
					&expr.Unary{
						Position: src.Pos{
							Filename: "srctest.ng",
							Line:     int32(6),
							Column:   int16(15),
						},
						Op: token.ChanOp,
						Expr: &expr.Ident{
							Position: src.Pos{
								Filename: "", // TODO
							},
							Name: "ch",
						},
					},
				},
			},
			Cond: &expr.Ident{
				Position: src.Pos{
					Filename: "", // TODO
				},
				Name: "ok",
			},
			Body: &stmt.Block{
				Position: src.Pos{
					Filename: "", // TODO
				},
				Stmts: []stmt.Stmt{
					&stmt.Simple{
						Position: src.Pos{
							Filename: "", // TODO
						},
						Expr: &expr.Call{
							Position: src.Pos{
								Filename: "", // TODO
							},
							Func: &expr.Ident{
								Position: src.Pos{
									Filename: "", // TODO
								},
								Name: "print",
							},
							Args: []expr.Expr{
								&expr.Ident{
									Position: src.Pos{
										Filename: "", // TODO
									},
									Name: "v",
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestPos(t *testing.T) {
	p := New("srctest.ng")
	got, err := p.Parse([]byte(srcposInput))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, srcposWant) {
		t.Errorf("unexpected source positions:\n%s", format.Debug(got))
	}
}
