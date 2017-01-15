// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strings"
)

type debugPrinter struct {
	buf     *bytes.Buffer
	ptrseen map[interface{}]int // ptr -> count seen
	ptrdone map[interface{}]bool
	indent  int
}

func (p *debugPrinter) collectPtrs(v reflect.Value) {
	switch v.Kind() {
	case reflect.Ptr:
		ptr := v.Interface()
		p.ptrseen[ptr]++
		if p.ptrseen[ptr] == 1 {
			p.collectPtrs(v.Elem())
		}
	case reflect.Interface:
		p.collectPtrs(v.Elem())
	case reflect.Map:
		for _, key := range v.MapKeys() {
			p.collectPtrs(key)
			p.collectPtrs(v.MapIndex(key))
		}
	case reflect.Array:
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			p.collectPtrs(v.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			p.collectPtrs(v.Field(i))
		}
	}
}

func (p *debugPrinter) printf(format string, args ...interface{}) {
	fmt.Fprintf(p.buf, format, args...)
}

func (p *debugPrinter) newline() {
	p.buf.WriteByte('\n')
	for i := 0; i < p.indent; i++ {
		p.buf.WriteByte('\t')
	}
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	}
	return false
}

func (p *debugPrinter) printv(v reflect.Value) {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if v.IsNil() {
			p.buf.WriteString("nil")
			return
		}
	}

	switch v.Kind() {
	case reflect.Ptr:
		p.printf("&")
		ptr := v.Interface()
		if p.ptrdone[ptr] {
			p.printf("%p", ptr)
		} else if p.ptrseen[ptr] > 1 {
			p.printv(v.Elem())
			p.ptrdone[ptr] = true
			p.printf(" (ptr %p)", ptr)
		} else {
			p.printv(v.Elem())
		}
	case reflect.Interface:
		p.printv(v.Elem())
	case reflect.Map:
		p.printf("%s{", v.Type())
		if v.Len() == 1 {
			key := v.MapKeys()[0]
			p.printf("%s: ", key)
			p.printv(v.MapIndex(key))
		} else if v.Len() > 0 {
			p.indent++
			for _, key := range v.MapKeys() {
				p.newline()
				p.printf("%s: ", key)
				p.printv(v.MapIndex(key))
				p.buf.WriteByte(',')
			}
			p.indent--
			p.newline()
		}
		p.buf.WriteByte('}')
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Int8 {
			s := v.Bytes()
			p.printf("%#q", s)
			return
		}
		fallthrough
	case reflect.Array:
		p.printf("%s{", v.Type())
		if v.Len() > 0 {
			p.indent++
			for i := 0; i < v.Len(); i++ {
				p.newline()
				p.printv(v.Index(i))
				p.buf.WriteByte(',')
			}
			p.indent--
			p.newline()
		}
		p.buf.WriteByte('}')
	case reflect.Struct:
		t := v.Type()
		p.printf("%s{", t)
		if v.NumField() > 0 {
			p.indent++
			for i := 0; i < v.NumField(); i++ {
				if isZero(v.Field(i)) {
					continue
				}
				p.newline()
				p.printf("%s: ", t.Field(i).Name)
				p.printv(v.Field(i))
				p.buf.WriteByte(',')
			}
			p.indent--
			p.newline()
		}
		p.buf.WriteByte('}')
	default:
		if v.Kind() == reflect.String {
			p.printf("%q", v.String())
		} else if v.CanInterface() {
			p.printf("%#v", v.Interface())
		} else {
			p.printf("?")
		}
	}
}

func printToFile(x interface{}) (path string, err error) {
	f, err := ioutil.TempFile("", "neugram-diff-")
	if err != nil {
		return "", err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	str := Debug(x)
	if _, err := io.WriteString(f, str); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func diffVal(x, y interface{}) (string, error) {
	fx, err := printToFile(x)
	if err != nil {
		return "", fmt.Errorf("diff print lhs error: %v", err)
	}
	defer os.Remove(fx)
	fy, err := printToFile(y)
	if err != nil {
		return "", fmt.Errorf("diff print rhs error: %v", err)
	}
	defer os.Remove(fy)

	b, _ := ioutil.ReadFile(fx)
	fmt.Printf("fx: %s\n", b)

	data, err := exec.Command("diff", "-U100", "-u", fx, fy).CombinedOutput()
	if err != nil && len(data) == 0 {
		// diff exits with a non-zero status when the files don't match.
		return "", fmt.Errorf("diff error: %v", err)
	}
	res := string(data)
	res = strings.Replace(res, fx, "/x", 1)
	res = strings.Replace(res, fy, "/y", 1)
	return res, nil
}

func WriteDebug(buf *bytes.Buffer, e interface{}) {
	p := debugPrinter{
		buf:     buf,
		ptrseen: make(map[interface{}]int),
		ptrdone: make(map[interface{}]bool),
	}
	v := reflect.ValueOf(e)
	p.collectPtrs(v)
	p.printv(v)
}

func Debug(e interface{}) string {
	buf := new(bytes.Buffer)
	WriteDebug(buf, e)
	return buf.String()
}

func Diff(x, y interface{}) string {
	s, err := diffVal(x, y)
	if err != nil {
		return fmt.Sprintf("format.Diff: %v", err)
	}
	return s
}
