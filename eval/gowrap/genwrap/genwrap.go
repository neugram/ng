// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package genwrap

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/types"
	"log"
	"strings"
	"text/template"
)

func GenGo(pkgName, outPkgName string) ([]byte, error) {
	quotedPkgName := strings.Replace(pkgName, "/", "_", -1)

	pkg, err := importer.Default().Import(pkgName)
	if err != nil {
		return nil, err
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
			log.Printf("genwrap: unexpected obj: %T\n", obj)
		}
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, data{
		OutPkgName: outPkgName,
		Name:       pkgName,
		QuotedName: quotedPkgName,
		Exports:    exports,
	})
	if err != nil {
		return nil, fmt.Errorf("genwrap: %v", err)
	}
	res, err := format.Source(buf.Bytes())
	if err != nil {
		lines := new(bytes.Buffer)
		for i, line := range strings.Split(buf.String(), "\n") {
			fmt.Fprintf(lines, "%3d: %s\n", i, line)
		}
		return nil, fmt.Errorf("genwrap: bad generated source: %v\n%s", err, lines.String())
	}
	return res, nil
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
	OutPkgName string
	Name       string
	QuotedName string
	Exports    map[string]string
}

var tmpl = template.Must(template.New("genwrap").Parse(`
// Generated file, do not edit.

package {{.OutPkgName}}

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	{{.QuotedName}} "{{.Name}}"
)

var wrap_{{.QuotedName}} = &gowrap.Pkg{
	Exports: map[string]reflect.Value{
		{{with $data := .}}
		{{range $name, $export := $data.Exports}}
		"{{$name}}": {{$export}},{{end}}
		{{end}}
	},
}

func init() {
	gowrap.Pkgs["{{.Name}}"] = wrap_{{.QuotedName}}
}
`))
