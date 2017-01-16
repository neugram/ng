// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"sort"

	"neugram.io/lang/tipe"
)

func (p *printer) tipe(t tipe.Type) {
	switch t := t.(type) {
	case tipe.Basic:
		p.buf.WriteString(string(t))
	case *tipe.Struct:
		if len(t.Fields) == 0 {
			p.buf.WriteString("struct{}")
			return
		}
		p.buf.WriteString("struct {")
		p.indent++
		maxlen := 0
		for _, name := range t.FieldNames {
			if len(name) > maxlen {
				maxlen = len(name)
			}
		}
		for i, ft := range t.Fields {
			p.newline()
			name := "*ERROR*No*Name*"
			if i < len(t.FieldNames) {
				name = t.FieldNames[i]
			}
			p.buf.WriteString(name)
			for i := len(name); i <= maxlen; i++ {
				p.buf.WriteByte(' ')
			}
			p.tipe(ft)
		}
		p.indent--
		p.newline()
		p.buf.WriteByte('}')
	case *tipe.Unresolved:
		if t.Package != "" {
			p.buf.WriteString(t.Package)
			p.buf.WriteByte('.')
		}
		p.buf.WriteString(t.Name)
	case *tipe.Slice:
		p.buf.WriteString("[]")
		p.tipe(t.Elem)
	case *tipe.Interface:
		if len(t.Methods) == 0 {
			p.buf.WriteString("interface{}")
			return
		}
		p.buf.WriteString("interface {")
		p.indent++
		names := make([]string, 0, len(t.Methods))
		for name := range t.Methods {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			p.newline()
			p.buf.WriteString(name)
			p.tipe(t.Methods[name])
		}
		p.indent--
		p.newline()
		p.buf.WriteByte('}')
	case *tipe.Map:
		p.buf.WriteString("map[")
		p.tipe(t.Key)
		p.buf.WriteByte(']')
		p.tipe(t.Value)
	case *tipe.Chan:
		if t.Direction == tipe.ChanRecv {
			p.buf.WriteString("<-")
		}
		p.buf.WriteString("chan")
		if t.Direction == tipe.ChanSend {
			p.buf.WriteString("<-")
		}
		p.buf.WriteByte(' ')
		p.tipe(t.Elem)
	default:
		p.buf.WriteString("format: unknown type: ")
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
