// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"sort"

	"neugram.io/ng/syntax/tipe"
)

func (p *printer) tipe(t tipe.Type) {
	if t == nil {
		p.buf.WriteString("<nil>")
		return
	}
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
		for _, sf := range t.Fields {
			if len(sf.Name) > maxlen {
				maxlen = len(sf.Name)
			}
		}
		for _, sf := range t.Fields {
			p.newline()
			name := sf.Name
			if name == "" {
				name = "*ERROR*No*Name*"
			}
			p.buf.WriteString(name)
			for i := len(name); i <= maxlen; i++ {
				p.buf.WriteByte(' ')
			}
			p.tipe(sf.Type)
		}
		p.indent--
		p.newline()
		p.buf.WriteByte('}')
	case *tipe.Named:
		p.buf.WriteString(t.Name)
	case *tipe.Pointer:
		p.buf.WriteByte('*')
		p.tipe(t.Elem)
	case *tipe.Unresolved:
		if t.Package != "" {
			p.buf.WriteString(t.Package)
			p.buf.WriteByte('.')
		}
		p.buf.WriteString(t.Name)
	case *tipe.Array:
		if t.Ellipsis {
			p.buf.WriteString("[...]")
		} else {
			fmt.Fprintf(p.buf, "[%d]", t.Len)
		}
		p.tipe(t.Elem)
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
			p.tipeFuncSig(t.Methods[name])
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
	case *tipe.Func:
		p.buf.WriteString("func")
		p.tipeFuncSig(t)
	case *tipe.Alias:
		p.buf.WriteString(t.Name)
	case *tipe.Tuple:
		p.buf.WriteString("(")
		for i, elt := range t.Elems {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.tipe(elt)
		}
		p.buf.WriteString(")")
	case *tipe.Ellipsis:
		p.buf.WriteString("...")
		p.tipe(t.Elem)
	default:
		p.buf.WriteString("format: unknown type: ")
		WriteDebug(p.buf, t)
	}
}

func (p *printer) tipeFuncSig(t *tipe.Func) {
	p.buf.WriteByte('(')
	if t.Params != nil {
		for i, elem := range t.Params.Elems {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.tipe(elem)
		}
	}
	p.buf.WriteByte(')')
	if t.Results != nil && len(t.Results.Elems) > 0 {
		p.buf.WriteByte(' ')
		if len(t.Results.Elems) > 1 {
			p.buf.WriteByte('(')
		}
		for i, elem := range t.Results.Elems {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.tipe(elem)
		}
		if len(t.Results.Elems) > 1 {
			p.buf.WriteByte(')')
		}
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
