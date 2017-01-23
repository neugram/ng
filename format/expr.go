// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

// TODO: general pretty printer
// TODO: remove s-expression generator from lang packages

import (
	"bytes"
	"fmt"

	"neugram.io/ng/expr"
	"neugram.io/ng/token"
)

type printer struct {
	buf    *bytes.Buffer
	indent int
}

func (p *printer) expr(e expr.Expr) {
	switch e := e.(type) {
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
