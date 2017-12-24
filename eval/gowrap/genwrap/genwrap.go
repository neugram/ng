// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package genwrap

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"log"
	"strings"
	"text/template"

	"neugram.io/ng/gotool"
)

func quotePkgPath(path string) string {
	return "wrap_" + strings.NewReplacer(
		"/", "_",
		".", "_",
		"-", "_",
	).Replace(path)
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
		case *types.Var:
			exports[name] = "reflect.ValueOf(&" + quotedPkgPath + "." + name + ").Elem()"
		case *types.Func, *types.Const:
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

// GenGo generates a wrapper package naemd outPkgName that
// registers the exported symbols of pkgPath with the global
// map gopkg.Pkgs.
//
// Any other packages that pkgPath depends on for defining its
// exported symbols are also registered, unless skipDeps is set.
func GenGo(pkgPath, outPkgName string, skipDeps bool) ([]byte, error) {
	pkg, err := gotool.M.ImportGo(pkgPath)
	if err != nil {
		return nil, err
	}
	pkgs := make(map[string]DataPkg)
	pkgs[pkgPath] = buildDataPkg(pkgPath, pkg)
	imports := []*types.Package{}
	if !skipDeps {
		for _, imp := range pkg.Imports() {
			// Re-import package to get all exported symbols.
			imppkg, err := gotool.M.ImportGo(imp.Path())
			if err != nil {
				return nil, err
			}
			imports = append(imports, imppkg)
		}
	}

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
	case *types.Array, *types.Struct:
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
var pkg_{{.QuotedName}} = &gowrap.Pkg{
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
		gowrap.Pkgs["{{.Name}}"] = pkg_{{.QuotedName}}
	}
}
{{end}}
`))
