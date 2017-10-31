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

func quotePkgPath(path string) string {
	return "wrap_" + strings.NewReplacer("/", "_", ".", "_").Replace(path)
}

func buildDataPkg(pkgPath string, pkg *types.Package) DataPkg {
	quotedPkgPath := quotePkgPath(pkgPath)
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
				exports[name] = "reflect.ValueOf(reflect.TypeOf((*" + quotedPkgPath + "." + name + ")(nil)).Elem())"
			} else {
				exports[name] = "reflect.ValueOf(reflect.TypeOf(" + quotedPkgPath + "." + name + nilexpr(obj.Type()) + "))"
			}
		case *types.Var, *types.Func, *types.Const:
			exports[name] = "reflect.ValueOf(" + quotedPkgPath + "." + name + ")"
		default:
			log.Printf("genwrap: unexpected obj: %T\n", obj)
		}
	}

	return DataPkg{
		Name:       pkg.Path(),
		QuotedName: quotedPkgPath,
		Exports:    exports,
	}
}

func GenGo(pkgPath, outPkgName string) ([]byte, error) {
	quotedPkgPath := quotePkgPath(pkgPath)

	pkg, err := importer.Default().Import(pkgPath)
	if err != nil {
		return nil, err
	}
	pkgs := make(map[string]DataPkg)
	pkgs[pkgPath] = buildDataPkg(pkgPath, pkg)
	imports := append([]*types.Package{}, pkg.Imports()...)
importsLoop:
	for i := 0; i < len(imports); i++ { // imports grows as we loop
		path := imports[i].Path()
		if _, exists := pkgs[path]; exists {
			continue
		}
		for _, dir := range strings.Split(path, "/") {
			if dir == "internal" || dir == "vendor" {
				continue importsLoop
			}
		}
		pkgs[path] = buildDataPkg(path, imports[i])
	}
	data := Data{
		OutPkgName: outPkgName,
	}
	for _, dataPkg := range pkgs {
		data.Pkgs = append(data.Pkgs, dataPkg)
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
				exports[name] = "reflect.ValueOf(reflect.TypeOf((*" + quotedPkgPath + "." + name + ")(nil)).Elem())"
			} else {
				exports[name] = "reflect.ValueOf(reflect.TypeOf(" + quotedPkgPath + "." + name + nilexpr(obj.Type()) + "))"
			}
		case *types.Var, *types.Func, *types.Const:
			exports[name] = "reflect.ValueOf(" + quotedPkgPath + "." + name + ")"
		default:
			log.Printf("genwrap: unexpected obj: %T\n", obj)
		}
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, data)
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
		switch t.Kind() {
		case types.Bool:
			return "(false)"
		case types.String:
			return `("")`
		case types.UnsafePointer:
			return "(nil)"
		default:
			return "(0)"
		}
	case *types.Struct:
		return "{}"
	case *types.Interface, *types.Map, *types.Pointer, *types.Slice, *types.Signature:
		return "(nil)"
	default:
		return fmt.Sprintf("(unexpected type: %T)", t)
	}
}

type Data struct {
	OutPkgName string
	Pkgs       []DataPkg
}

type DataPkg struct {
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

{{range .Pkgs}}
	{{.QuotedName}} "{{.Name}}"
{{end}}
)

{{range .Pkgs}}
var wrap_{{.QuotedName}} = &gowrap.Pkg{
	Exports: map[string]reflect.Value{
		{{with $data := .}}
		{{range $name, $export := $data.Exports}}
		"{{$name}}": {{$export}},{{end}}
		{{end}}
	},
}
{{end}}

{{range .Pkgs}}
func init() {
	if gowrap.Pkgs["{{.Name}}"] == nil {
		gowrap.Pkgs["{{.Name}}"] = wrap_{{.QuotedName}}
	}
}
{{end}}
`))
