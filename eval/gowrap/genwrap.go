// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/types"
	"log"
	"os"
	"strings"
	"text/template"
)

func main() {
	genpkg(os.Args[1])
}

func genpkg(pkgName string) {
	quotedPkgName := strings.Replace(pkgName, "/", "_", -1)
	output, err := os.Create("wrap_" + quotedPkgName + ".go")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := output.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	pkg, err := importer.Default().Import(pkgName)
	if err != nil {
		log.Fatal(err)
	}
	scope := pkg.Scope()
	exports := map[string]string{}
	for _, name := range scope.Names() {
		if !ast.IsExported(name) {
			continue
		}
		obj := scope.Lookup(name)
		switch obj.(type) {
		case *types.TypeName:
			if _, ok := obj.Type().Underlying().(*types.Interface); ok {
				exports[name] = "reflect.ValueOf((*" + quotedPkgName + "." + name + ")(nil))"
			} else {
				exports[name] = "reflect.ValueOf(" + quotedPkgName + "." + name + nilexpr(obj.Type()) + ")"
			}
		case *types.Var, *types.Func, *types.Const:
			exports[name] = "reflect.ValueOf(" + quotedPkgName + "." + name + ")"
		default:
			fmt.Printf("unexpected obj: %T\n", obj)
		}
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, data{
		Name:       pkgName,
		QuotedName: quotedPkgName,
		Exports:    exports,
	})
	if err != nil {
		log.Fatal(err)
	}
	res, err := format.Source(buf.Bytes())
	if err != nil {
		os.Stderr.Write(buf.Bytes())
		log.Fatal(err)
	}
	if _, err := fmt.Fprintf(output, "%s", res); err != nil {
		log.Fatal(err)
	}
}

func nilexpr(t types.Type) string {
	t = t.Underlying()
	switch t := t.(type) {
	case *types.Basic:
		return "(0)"
	case *types.Struct:
		return "{}"
	case *types.Interface, *types.Map, *types.Pointer, *types.Slice:
		return "(nil)"
	default:
		return fmt.Sprintf("(unexpected type: %T)", t)
	}
}

type data struct {
	Name       string
	QuotedName string
	Exports    map[string]string
}

var tmpl = template.Must(template.New("genwrap").Parse(`
// Generated file, do not edit.

package gowrap

import (
	"reflect"
	{{.QuotedName}} "{{.Name}}"
)

var wrap_{{.QuotedName}} = &Pkg{
	Exports: map[string]reflect.Value{
		{{with $data := .}}
		{{range $name, $export := $data.Exports}}
		"{{$name}}": {{$export}},{{end}}
		{{end}}
	},
}

func init() {
	Pkgs["{{.Name}}"] = wrap_{{.QuotedName}}
}
`))
