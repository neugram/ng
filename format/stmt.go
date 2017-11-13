// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"

	"neugram.io/ng/stmt"
)

func (p *printer) stmt(s stmt.Stmt) {
	switch s := s.(type) {
	case *stmt.Simple:
		p.expr(s.Expr)
	case *stmt.Return:
		p.buf.WriteString("return")
		if len(s.Exprs) > 0 {
			p.buf.WriteByte(' ')
		}
		for i, e := range s.Exprs {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.expr(e)
		}
	case *stmt.Assign:
		for i, e := range s.Left {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.expr(e)
		}
		p.buf.WriteString(" ")
		if s.Decl {
			p.buf.WriteString(":")
		}
		p.buf.WriteString("= ")
		for i, e := range s.Right {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.expr(e)
		}
	case *stmt.Send:
		p.expr(s.Chan)
		p.buf.WriteString("<-")
		p.expr(s.Value)
	case *stmt.Select:
		p.buf.WriteString("select {")
		for _, c := range s.Cases {
			switch c.Default {
			case true:
				p.buf.WriteString("default:")
			default:
				p.buf.WriteString("case ")
				p.stmt(c.Stmt)
				p.buf.WriteString(":")
			}
			p.stmt(c.Body)
		}
		p.buf.WriteString("}")
	case *stmt.Block:
		p.buf.WriteString("{")
		for _, s := range s.Stmts {
			p.stmt(s)
		}
		p.buf.WriteString("}")
	default:
		p.printf("format: unknown stmt %T: ", s)
		WriteDebug(p.buf, s)
	}
}

func WriteStmt(buf *bytes.Buffer, s stmt.Stmt) {
	p := &printer{
		buf: buf,
	}
	p.stmt(s)
}

func Stmt(e stmt.Stmt) string {
	buf := new(bytes.Buffer)
	WriteStmt(buf, e)
	return buf.String()
}
