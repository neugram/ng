// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"

	"neugram.io/lang/stmt"
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
