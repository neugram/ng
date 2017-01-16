// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"

	"neugram.io/lang/tipe"
)

func (p *printer) tipe(t tipe.Type) {
	switch t := t.(type) {
	/*case *stmt.Simple:
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
		}*/
	default:
		p.printf("format: unknown type %T: ", t)
		WriteDebug(p.buf, t)
	}
}

func WriteType(buf *bytes.Buffer, t tipe.Type) {
	p := &printer{
		buf: buf,
	}
	p.tipe(t)
}

func Type(t tipe.Type) string {
	buf := new(bytes.Buffer)
	WriteType(buf, t)
	return buf.String()
}
