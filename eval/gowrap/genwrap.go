// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

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
	"text/template"
)

func main() {
	genpkg(os.Args[1])
}

func genpkg(pkgName string) {
	output, err := os.Create("wrap_" + pkgName + ".go")
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
			exports[name] = name + nilexpr(scope.Lookup(name).Type())
		case *types.Var, *types.Func, *types.Const:
			exports[name] = name
		default:
			fmt.Printf("unexpected obj: %T\n", obj)
		}
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, data{
		Name:    pkgName,
		Exports: exports,
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
	Name    string
	Exports map[string]string
}

var tmpl = template.Must(template.New("genwrap").Parse(`
// Generated file, do not edit.

package gowrap

import "{{.Name}}"

var wrap_{{.Name}} = &Pkg{
	Exports: map[string]interface{}{
		{{with $data := .}}
		{{range $name, $export := $data.Exports}}
		"{{$name}}": {{$data.Name}}.{{$export}},{{end}}
		{{end}}
	},
}

func init() {
	Pkgs["{{.Name}}"] = wrap_{{.Name}}
}
`))
