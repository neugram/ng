// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

// TODO: general pretty printer

import (
	"bytes"
	"fmt"

	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/token"
)

type printer struct {
	buf    *bytes.Buffer
	indent int
}

func (p *printer) expr(e expr.Expr) {
	switch e := e.(type) {
	case *expr.Binary:
		WriteExpr(p.buf, e.Left)
		p.buf.WriteString(e.Op.String())
		WriteExpr(p.buf, e.Right)
	case *expr.Unary:
		p.buf.WriteString(e.Op.String())
		WriteExpr(p.buf, e.Expr)
		if e.Op == token.LeftParen {
			p.buf.WriteByte(')')
		}
	case *expr.Bad:
		fmt.Fprintf(p.buf, "bad(%q)", e.Error)
	case *expr.Slice:
		if e.Low != nil {
			p.expr(e.Low)
		}
		p.buf.WriteString(":")
		if e.High != nil {
			p.expr(e.High)
		}
		if e.Max != nil {
			p.buf.WriteString(":")
			p.expr(e.Max)
		}
	case *expr.Selector:
		p.expr(e.Left)
		p.buf.WriteString("." + e.Right.Name)
	case *expr.BasicLiteral:
		p.buf.WriteString(fmt.Sprintf("%v", e.Value))
	case *expr.FuncLiteral:
		p.buf.WriteString("func")
		if e.ReceiverName != "" {
			ptr := ""
			if e.PointerReceiver {
				ptr = "*"
			}
			fmt.Fprintf(p.buf, " (%s%s)", ptr, e.ReceiverName)
		}
		if e.Name != "" {
			p.buf.WriteByte(' ')
			p.buf.WriteString(e.Name)
		}
		p.buf.WriteByte('(')

		// Similar to tipeFuncSig, but with parameter names.
		if len(e.ParamNames) > 0 {
			for i, name := range e.ParamNames {
				if i > 0 {
					p.buf.WriteString(", ")
				}
				if name != "" {
					p.buf.WriteString(name)
					p.buf.WriteByte(' ')
				}
				// TODO: elide types for equal subsequent params
				p.tipe(e.Type.Params.Elems[i])
			}
		}
		p.buf.WriteString(")")
		if len(e.ResultNames) == 1 && e.ResultNames[0] == "" {
			p.buf.WriteString(" ")
			p.tipe(e.Type.Results.Elems[0])
		} else if len(e.ResultNames) != 0 {
			p.buf.WriteString(" (")
			for i, name := range e.ResultNames {
				if i > 0 {
					p.buf.WriteString(", ")
				}
				if name != "" {
					p.buf.WriteString(name)
					p.buf.WriteByte(' ')
				}
				// TODO: elide types for equal subsequent params
				p.tipe(e.Type.Results.Elems[i])
			}
			p.buf.WriteString(")")
		}

		if e.Body != nil {
			p.buf.WriteString(" ")
			p.stmt(e.Body.(*stmt.Block))
		}
	case *expr.CompLiteral:
		p.tipe(e.Type)
		p.print("{")
		if len(e.Keys) > 0 {
			p.indent++
			for i, key := range e.Keys {
				p.newline()
				p.expr(key)
				p.print(": ")
				p.expr(e.Values[i])
				p.print(",")
			}
			p.indent--
			p.newline()
		} else if len(e.Values) > 0 {
			for i, elem := range e.Values {
				if i > 0 {
					p.print(", ")
				}
				p.expr(elem)
			}
		}
		p.print("}")
	case *expr.MapLiteral:
		p.tipe(e.Type)
		p.print("{")
		p.indent++
		for i, key := range e.Keys {
			p.newline()
			p.expr(key)
			p.print(": ")
			p.expr(e.Values[i])
			p.print(",")
		}
		p.indent--
		p.newline()
		p.print("}")
	case *expr.ArrayLiteral:
		p.tipe(e.Type)
		p.print("{")
		switch len(e.Keys) {
		case 0:
			for i, elem := range e.Values {
				if i > 0 {
					p.print(", ")
				}
				p.expr(elem)
			}
		default:
			for i, elem := range e.Values {
				if i > 0 {
					p.print(", ")
				}
				p.expr(e.Keys[i])
				p.print(": ")
				p.expr(elem)
			}
		}
		p.print("}")
	case *expr.SliceLiteral:
		p.tipe(e.Type)
		p.print("{")
		switch len(e.Keys) {
		case 0:
			for i, elem := range e.Values {
				if i > 0 {
					p.print(", ")
				}
				p.expr(elem)
			}
		default:
			for i, elem := range e.Values {
				if i > 0 {
					p.print(", ")
				}
				p.expr(e.Keys[i])
				p.print(": ")
				p.expr(elem)
			}
		}
		p.print("}")
	case *expr.TableLiteral:
		p.tipe(e.Type)
		p.print("{")
		if len(e.ColNames) > 0 {
			p.print("{|")
			for i, col := range e.ColNames {
				if i > 0 {
					p.print(", ")
				}
				p.expr(col)
			}
			p.print("|}")
		}
		if len(e.Rows) > 0 {
			p.print(", ")
			for i, row := range e.Rows {
				if i > 0 {
					p.print(", ")
				}
				p.print("{")
				for j, r := range row {
					if j > 0 {
						p.print(", ")
					}
					p.expr(r)
				}
				p.print("}")
			}
			p.print("}")
		}
		p.print("}")
	case *expr.Type:
		p.tipe(e.Type)
	case *expr.Ident:
		p.buf.WriteString(e.Name)
	case *expr.Index:
		p.expr(e.Left)
		p.buf.WriteString("[")
		for i, idx := range e.Indicies {
			if i > 0 {
				p.buf.WriteString(":")
			}
			p.expr(idx)
		}
		p.buf.WriteString("]")
	case *expr.TypeAssert:
		p.expr(e.Left)
		p.buf.WriteString(".(")
		if e.Type == nil {
			p.buf.WriteString("type")
		} else {
			WriteType(p.buf, e.Type)
		}
		p.buf.WriteString(")")
	case *expr.Call:
		WriteExpr(p.buf, e.Func)
		p.buf.WriteString("(")
		for i, arg := range e.Args {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			WriteExpr(p.buf, arg)
		}
		p.buf.WriteString(")")
	case *expr.Shell:
		if len(e.Cmds) == 1 {
			p.buf.WriteString("$$ ")
			p.expr(e.Cmds[0])
			p.buf.WriteString(" $$")
		} else {
			p.buf.WriteString("$$")
			for _, cmd := range e.Cmds {
				p.newline()
				p.expr(cmd)
			}
			p.newline()
			p.buf.WriteString("$$")
		}
	case *expr.ShellList:
		for i, andor := range e.AndOr {
			if i > 0 {
				if e.AndOr[i-1].Background {
					p.buf.WriteByte(' ')
				} else {
					p.buf.WriteString("; ")
				}
			}
			p.expr(andor)
		}
	case *expr.ShellAndOr:
		for i, pl := range e.Pipeline {
			p.expr(pl)
			if i < len(e.Sep) {
				switch e.Sep[i] {
				case token.LogicalAnd:
					p.buf.WriteString(" && ")
				case token.LogicalOr:
					p.buf.WriteString(" || ")
				default:
					p.printf(" <bad separator: %v> ", e.Sep[i])
				}
			} else if len(e.Sep) < len(e.Pipeline)-1 {
				p.buf.WriteString(" <missing separator> ")
			}
		}
		if e.Background {
			p.buf.WriteString(" &")
		}
	case *expr.ShellPipeline:
		if e.Bang {
			p.buf.WriteString("! ")
		}
		for i, cmd := range e.Cmd {
			if i > 0 {
				p.buf.WriteString(" | ")
			}
			p.expr(cmd)
		}
	case *expr.ShellCmd:
		if e.SimpleCmd != nil {
			if e.Subshell != nil {
				p.printf("<bad shellcmd has simple and subshell> ")
			}
			p.expr(e.SimpleCmd)
		} else if e.Subshell != nil {
			p.buf.WriteByte('(')
			p.expr(e.Subshell)
			p.buf.WriteByte(')')
		} else {
			p.printf("<bad shellcmd is empty>")
		}
	case *expr.ShellSimpleCmd:
		for i, kv := range e.Assign {
			if i > 0 {
				p.buf.WriteByte(' ')
			}
			p.printf("%s=%s", kv.Key, kv.Value)
		}
		if len(e.Assign) > 0 && len(e.Args) > 0 {
			p.buf.WriteByte(' ')
		}
		for i, arg := range e.Args {
			if i > 0 {
				p.buf.WriteByte(' ')
			}
			p.buf.WriteString(arg)
		}
		if (len(e.Assign) > 0 || len(e.Args) > 0) && len(e.Redirect) > 0 {
			p.buf.WriteByte(' ')
		}
		for i, r := range e.Redirect {
			if i > 0 {
				p.buf.WriteByte(' ')
			}
			if r.Number != nil {
				p.printf("%d", *r.Number)
			}
			p.buf.WriteString(r.Token.String())
			p.buf.WriteString(r.Filename)
		}
	default:
		p.printf("format: unknown expr %T: ", e)
		WriteDebug(p.buf, e)
	}
}

func (p *printer) printf(format string, args ...interface{}) {
	fmt.Fprintf(p.buf, format, args...)
}

func (p *printer) print(args ...interface{}) {
	fmt.Fprint(p.buf, args...)
}

func (p *printer) newline() {
	p.buf.WriteByte('\n')
	for i := 0; i < p.indent; i++ {
		p.buf.WriteByte('\t')
	}
}

func WriteExpr(buf *bytes.Buffer, e expr.Expr) {
	p := &printer{
		buf: buf,
	}
	p.expr(e)
}

func Expr(e expr.Expr) string {
	buf := new(bytes.Buffer)
	WriteExpr(buf, e)
	return buf.String()
}
