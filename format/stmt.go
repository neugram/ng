// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"

	"neugram.io/ng/syntax/stmt"
)

func (p *printer) stmt(s stmt.Stmt) {
	switch s := s.(type) {
	case *stmt.Import:
		if s.Name != "" {
			fmt.Fprintf(p.buf, "import %s %q", s.Name, s.Path)
		} else {
			fmt.Fprintf(p.buf, "import %q", s.Path)
		}
	case *stmt.ImportSet:
		p.buf.WriteString("import (")
		if len(s.Imports) == 0 {
			p.buf.WriteString(")")
			return
		}
		p.indent++
		for _, imp := range s.Imports {
			p.newline()
			if imp.Name != "" {
				fmt.Fprintf(p.buf, "%s %q", imp.Name, imp.Path)
			} else {
				fmt.Fprintf(p.buf, "%q", imp.Path)
			}
		}
		p.indent--
		p.newline()
		p.buf.WriteString(")")
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
	case *stmt.Switch:
		p.buf.WriteString("switch ")
		if s.Init != nil {
			p.stmt(s.Init)
			if s.Cond != nil {
				p.buf.WriteString("; ")
			}
		}
		if s.Cond != nil {
			p.expr(s.Cond)
		}
		p.buf.WriteString("{")
		if len(s.Cases) > 0 {
			p.buf.WriteString("\n")
		}
		for _, c := range s.Cases {
			switch c.Default {
			case true:
				p.buf.WriteString("default:")
			default:
				p.buf.WriteString("case ")
				for i, e := range c.Conds {
					if i > 0 {
						p.buf.WriteString(", ")
					}
					p.expr(e)
				}
				p.buf.WriteString(":\n")
			}
			p.stmt(c.Body)
		}
		p.buf.WriteString("}")
	case *stmt.TypeSwitch:
		p.buf.WriteString("switch ")
		if s.Init != nil {
			p.stmt(s.Init)
			if s.Assign != nil {
				p.buf.WriteString("; ")
			}
		}
		if s.Assign != nil {
			p.stmt(s.Assign)
			p.buf.WriteString(" ")
		}
		p.buf.WriteString("{")
		if len(s.Cases) > 0 {
			p.buf.WriteString("\n")
		}
		for _, c := range s.Cases {
			switch c.Default {
			case true:
				p.buf.WriteString("default:\n")
			default:
				p.buf.WriteString("case ")
				for i, typ := range c.Types {
					if i > 0 {
						p.buf.WriteString(", ")
					}
					p.tipe(typ)
				}
				p.buf.WriteString(":\n")
			}
			if len(c.Body.Stmts) > 0 {
				p.stmt(c.Body)
				p.buf.WriteString("\n")
			}
		}
		p.buf.WriteString("}")
	case *stmt.Select:
		p.buf.WriteString("select {")
		if len(s.Cases) > 0 {
			p.buf.WriteString("\n")
		}
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
