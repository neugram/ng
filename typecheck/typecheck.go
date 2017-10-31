// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package typecheck is a Neugram type checker.
package typecheck

import (
	"bufio"
	"fmt"
	"go/constant"
	goimporter "go/importer"
	gotoken "go/token"
	gotypes "go/types"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"neugram.io/ng/expr"
	"neugram.io/ng/format"
	"neugram.io/ng/parser"
	"neugram.io/ng/stmt"
	"neugram.io/ng/tipe"
	"neugram.io/ng/token"
)

type Checker struct {
	ImportGo func(path string) (*gotypes.Package, error)

	// TODO: we could put these on our AST. Should we?
	Types         map[expr.Expr]tipe.Type
	Defs          map[*expr.Ident]*Obj
	Values        map[expr.Expr]constant.Value
	NgPkgs        map[string]*tipe.Package // abs file path -> pkg
	GoPkgs        map[string]*tipe.Package // path -> pkg
	GoTypes       map[gotypes.Type]tipe.Type
	GoTypesToFill map[gotypes.Type]tipe.Type
	// TODO: GoEquiv is tricky and deserving of docs. Particular type instance is associated with a Go type. That means EqualType(t1, t2)==true but t1 could have GoEquiv and t2 not.
	GoEquiv map[tipe.Type]gotypes.Type
	NumSpec map[expr.Expr]tipe.Basic // *tipe.Call, *tipe.CompLiteral -> numeric basic type
	Errs    []error

	importWalk []string // in-process pkgs, used to detect cycles

	cur *Scope

	memory *tipe.Memory
}

func New(initPkg string) *Checker {
	if initPkg == "" {
		initPkg = "main"
	}
	return &Checker{
		ImportGo:      goimporter.Default().Import,
		Types:         make(map[expr.Expr]tipe.Type),
		Defs:          make(map[*expr.Ident]*Obj),
		Values:        make(map[expr.Expr]constant.Value),
		NgPkgs:        make(map[string]*tipe.Package),
		GoPkgs:        make(map[string]*tipe.Package),
		GoTypes:       make(map[gotypes.Type]tipe.Type),
		GoTypesToFill: make(map[gotypes.Type]tipe.Type),
		GoEquiv:       make(map[tipe.Type]gotypes.Type), // TODO remove?
		cur: &Scope{
			Parent: Universe,
			Objs:   make(map[string]*Obj),
		},
		importWalk: []string{initPkg},
		memory:     tipe.NewMemory(),
	}
}

type typeHint int

const (
	hintNone typeHint = iota
	hintElideErr
)

type partialMode int

const (
	modeInvalid partialMode = iota
	modeVoid
	modeConst
	modeVar
	modeBuiltin
	modeTypeExpr
	modeFunc
)

type partial struct {
	mode partialMode
	typ  tipe.Type
	val  constant.Value
	expr expr.Expr
}

func IsError(t tipe.Type) bool {
	return Universe.Objs["error"].Type == t
}

func (c *Checker) stmt(s stmt.Stmt, retType *tipe.Tuple) tipe.Type {
	switch s := s.(type) {
	case *stmt.Assign:
		var partials []partial
		for _, rhs := range s.Right {
			p := c.exprNoElide(rhs)
			if p.mode == modeInvalid {
				return nil
			}
			if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
				if len(s.Right) > 1 {
					c.errorf("multiple value %s in single-value context", rhs)
					return nil
				}
				for _, t := range tuple.Elems {
					partials = append(partials, partial{
						mode: modeVar,
						typ:  t,
					})
				}
				continue
			}
			partials = append(partials, c.exprNoElide(rhs))
		}
		if len(s.Right) == 1 && len(s.Left) == len(partials)-1 && IsError(partials[len(partials)-1].typ) {
			if c, isCall := s.Right[0].(*expr.Call); isCall {
				// func f() (T, error) { ... )
				// x := f()
				c.ElideError = true
			}
			partials = partials[:len(partials)-1]
		}

		if len(s.Left) != len(partials) {
			c.errorf("arity mismatch, left %d != right %d", len(s.Left), len(partials))
			return nil
		}

		if s.Decl {
			for i, lhs := range s.Left {
				p := partials[i]
				if isUntyped(p.typ) {
					c.constrainUntyped(&p, defaultType(p.typ))
				}
				if lhs.(*expr.Ident).Name == "_" {
					if len(s.Left) == 1 {
						c.errorf("no new variables in declaration")
						return nil
					}
					continue
				}
				obj := &Obj{
					Kind: ObjVar,
					Type: p.typ,
				}
				c.Defs[lhs.(*expr.Ident)] = obj
				c.cur.Objs[lhs.(*expr.Ident).Name] = obj
			}
		} else {
			for i, lhs := range s.Left {
				p := partials[i]
				if ident, isIdent := lhs.(*expr.Ident); isIdent && ident.Name == "_" {
					// "_" takes value of any type and drops it.
					continue
				}
				lhsP := c.expr(lhs)
				c.assign(&p, lhsP.typ)
			}
		}
		return nil

	case *stmt.Simple:
		p := c.exprNoElide(s.Expr)
		if c, isCall := s.Expr.(*expr.Call); isCall {
			isError := IsError(p.typ)
			if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
				if IsError(tuple.Elems[len(tuple.Elems)-1]) {
					isError = true
				}
			}
			if isError {
				// func f() (..., error) { ... )
				// f()
				c.ElideError = true
			}
		}
		if p.mode == modeFunc {
			fn := p.expr.(*expr.FuncLiteral)
			if fn.Name != "" {
				obj := &Obj{
					Kind: ObjVar,
					Type: p.typ,
				}
				// TODO: c.Defs?
				c.cur.Objs[fn.Name] = obj
			}
		}
		return p.typ

	case *stmt.Block:
		c.pushScope()
		defer c.popScope()
		for _, s := range s.Stmts {
			c.stmt(s, retType)
		}
		return nil

	case *stmt.Go:
		c.expr(s.Call)
		return nil

	case *stmt.If:
		if s.Init != nil {
			c.pushScope()
			defer c.popScope()
			c.stmt(s.Init, retType)
		}
		c.expr(s.Cond)
		c.stmt(s.Body, retType)
		if s.Else != nil {
			c.stmt(s.Else, retType)
		}
		return nil

	case *stmt.For:
		if s.Init != nil {
			c.pushScope()
			defer c.popScope()
			c.stmt(s.Init, retType)
		}
		if s.Cond != nil {
			c.expr(s.Cond)
		}
		if s.Post != nil {
			c.stmt(s.Post, retType)
		}
		c.stmt(s.Body, retType)
		return nil

	case *stmt.Range:
		c.pushScope()
		defer c.popScope()

		p := c.expr(s.Expr)
		var kt, vt tipe.Type
		switch t := p.typ.(type) {
		case *tipe.Array:
			kt = tipe.Int
			vt = t.Elem
		case *tipe.Slice:
			kt = tipe.Int
			vt = t.Elem
		case *tipe.Map:
			kt = t.Key
			vt = t.Value
		default:
			c.errorf("TODO range over non-slice: %T", t)
		}
		if s.Decl {
			if s.Key != nil {
				obj := &Obj{Kind: ObjVar, Type: kt}
				c.Defs[s.Key.(*expr.Ident)] = obj
				c.cur.Objs[s.Key.(*expr.Ident).Name] = obj
				c.Types[s.Key] = kt
			}
			if s.Val != nil {
				obj := &Obj{Kind: ObjVar, Type: vt}
				c.Defs[s.Val.(*expr.Ident)] = obj
				c.cur.Objs[s.Val.(*expr.Ident).Name] = obj
				c.Types[s.Val] = vt
			}
		} else {
			if s.Key != nil {
				p := c.expr(s.Key)
				c.assign(&p, kt)
				c.Types[s.Key] = kt
			}
			if s.Val != nil {
				p := c.expr(s.Val)
				c.assign(&p, vt)
				c.Types[s.Val] = vt
			}
		}
		c.stmt(s.Body, retType)
		return nil

	case *stmt.TypeDecl:
		t, _ := c.resolve(s.Type)
		s.Type = t

		obj := &Obj{
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		}
		c.cur.Objs[s.Name] = obj
		return nil

	case *stmt.MethodikDecl:
		var usesNum bool
		t, _ := c.resolve(s.Type)
		s.Type = t.(*tipe.Methodik)
		for _, f := range s.Type.Methods {
			usesNum = usesNum || tipe.UsesNum(f)
		}

		for _, m := range s.Methods {
			c.pushScope()
			if m.ReceiverName != "" {
				obj := &Obj{
					Kind: ObjVar,
					Type: s.Type,
				}
				c.cur.Objs[m.ReceiverName] = obj
			}
			c.expr(m)
			// TODO: uses num inside a method
			c.popScope()
		}

		if usesNum {
			s.Type.Spec.Num = tipe.Num
		}

		obj := &Obj{
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		}
		c.cur.Objs[s.Name] = obj
		return nil

	case *stmt.Return:
		if retType == nil || len(s.Exprs) > len(retType.Elems) {
			c.errorf("too many arguments to return")
		}
		var partials []partial
		for i, e := range s.Exprs {
			partials = append(partials, c.expr(e))
			c.constrainUntyped(&partials[i], retType.Elems[i])
		}
		for _, p := range partials {
			if p.mode == modeInvalid {
				return nil
			}
		}
		want := retType.Elems
		if len(want) == 0 && len(partials) == 0 {
			return nil
		}
		var got []tipe.Type
		if tup, ok := partials[0].typ.(*tipe.Tuple); ok {
			if len(partials) != 1 {
				c.errorf("multi-value %s in single-value context", partials[0])
				return nil
			}
			got = tup.Elems
		} else {
			for _, p := range partials {
				if _, ok := p.typ.(*tipe.Tuple); ok {
					c.errorf("multi-value %s in single-value context", partials[0])
					return nil
				}
				got = append(got, p.typ)
			}
		}
		if len(got) > len(want) {
			c.errorf("too many arguments to return")
			return nil
		}
		if len(got) < len(want) {
			c.errorf("too few arguments to return")
			return nil
		}

		for i := range want {
			if !c.assignable(want[i], got[i]) {
				c.errorf("cannot use %s as %s in return argument", format.Type(got[i]), format.Type(want[i]))
			}
		}
		return nil

	case *stmt.ImportSet:
		for _, imp := range s.Imports {
			c.checkImport(imp)
		}
		return nil

	case *stmt.Import:
		c.checkImport(s)
		return nil

	case *stmt.Send:
		p := c.expr(s.Chan)
		if p.mode == modeInvalid {
			return nil
		}
		cht, ok := p.typ.(*tipe.Chan)
		if !ok {
			c.errorf("cannot send to non-channel type: %s", format.Type(cht))
			return nil
		}
		p = c.expr(s.Value)
		if p.mode == modeInvalid {
			return nil
		}
		c.convert(&p, cht.Elem)
		if p.mode == modeInvalid {
			c.errorf("cannot send %s to %s", format.Type(p.typ), format.Type(cht))
		}
		return nil

	case *stmt.Branch:
		// TODO: make sure the branch is valid
		return nil

	case *stmt.Labeled:
		c.stmt(s.Stmt, retType)
		return nil

	default:
		panic("typecheck: unknown stmt: " + format.Debug(s))
	}
}

var goErrorID = gotypes.Universe.Lookup("error").Id()

func (c *Checker) fromGoType(t gotypes.Type) (res tipe.Type) {
	if res = c.GoTypes[t]; res != nil {
		return res
	}
	defer func() {
		if res == nil {
			fmt.Printf("typecheck: unknown go type: %v\n", t)
		} else {
			c.GoTypes[t] = res
			c.GoTypesToFill[t] = res
			if _, isBasic := t.(*gotypes.Basic); !isBasic {
				c.GoEquiv[res] = t
			}
		}
	}()
	switch t := t.(type) {
	case *gotypes.Basic:
		switch t.Kind() {
		case gotypes.Bool:
			return tipe.Bool
		case gotypes.String:
			return tipe.String
		case gotypes.Int:
			return tipe.Int
		case gotypes.Int8:
			return tipe.Int8
		case gotypes.Int16:
			return tipe.Int16
		case gotypes.Int32:
			return tipe.Int32
		case gotypes.Int64:
			return tipe.Int64
		case gotypes.Uint:
			return tipe.Uint
		case gotypes.Uint8:
			return tipe.Uint8
		case gotypes.Uint16:
			return tipe.Uint16
		case gotypes.Uint32:
			return tipe.Uint32
		case gotypes.Uint64:
			return tipe.Uint64
		case gotypes.Uintptr:
			return tipe.Uintptr
		case gotypes.Float32:
			return tipe.Float32
		case gotypes.Float64:
			return tipe.Float64
		case gotypes.Complex64:
			return tipe.Complex64
		case gotypes.Complex128:
			return tipe.Complex128
		case gotypes.UntypedBool:
			return tipe.UntypedBool
		case gotypes.UntypedString:
			return tipe.UntypedString
		case gotypes.UntypedInt:
			return tipe.UntypedInteger
		case gotypes.UntypedFloat:
			return tipe.UntypedFloat
		case gotypes.UntypedComplex:
			return tipe.UntypedComplex
		case gotypes.UntypedRune:
			return tipe.UntypedRune
		}
	case *gotypes.Named:
		if t.Obj().Id() == goErrorID {
			return Universe.Objs["error"].Type
		}
		return new(tipe.Methodik)
	case *gotypes.Array:
		return &tipe.Array{}
	case *gotypes.Slice:
		return &tipe.Slice{}
	case *gotypes.Chan:
		return new(tipe.Chan)
	case *gotypes.Map:
		return new(tipe.Map)
	case *gotypes.Struct:
		return new(tipe.Struct)
	case *gotypes.Pointer:
		return new(tipe.Pointer)
	case *gotypes.Interface:
		return new(tipe.Interface)
	case *gotypes.Signature:
		return new(tipe.Func)
	}
	return nil
}

func (c *Checker) fillGoType(res tipe.Type, t gotypes.Type) {
	switch t := t.(type) {
	case *gotypes.Basic:
	case *gotypes.Named:
		if t.Obj().Id() == goErrorID {
			return
		}
		base := c.fromGoType(t.Underlying())
		mdik := res.(*tipe.Methodik)
		*mdik = tipe.Methodik{
			Type:    base,
			Name:    t.Obj().Name(),
			PkgName: t.Obj().Pkg().Name(),
			PkgPath: t.Obj().Pkg().Path(),
		}
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			mdik.MethodNames = append(mdik.MethodNames, m.Name())
			mdik.Methods = append(mdik.Methods, c.fromGoType(m.Type()).(*tipe.Func))
		}
	case *gotypes.Array:
		a := res.(*tipe.Array)
		a.Len = t.Len()
		a.Elem = c.fromGoType(t.Elem())
	case *gotypes.Slice:
		res.(*tipe.Slice).Elem = c.fromGoType(t.Elem())
	case *gotypes.Chan:
		ch := res.(*tipe.Chan)
		switch t.Dir() {
		case gotypes.SendRecv:
			ch.Direction = tipe.ChanBoth
		case gotypes.SendOnly:
			ch.Direction = tipe.ChanSend
		case gotypes.RecvOnly:
			ch.Direction = tipe.ChanRecv
		}
		ch.Elem = c.fromGoType(t.Elem())
	case *gotypes.Map:
		m := res.(*tipe.Map)
		m.Key = c.fromGoType(t.Key())
		m.Value = c.fromGoType(t.Elem())
	case *gotypes.Struct:
		s := res.(*tipe.Struct)
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			ft := c.fromGoType(f.Type())
			if ft == nil {
				continue
			}
			s.FieldNames = append(s.FieldNames, f.Name())
			s.Fields = append(s.Fields, ft)
		}
	case *gotypes.Pointer:
		elem := c.fromGoType(t.Elem())
		res.(*tipe.Pointer).Elem = elem
	case *gotypes.Interface:
		mthds := make(map[string]*tipe.Func)
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			mthds[m.Name()] = c.fromGoType(m.Type()).(*tipe.Func)
		}
		res.(*tipe.Interface).Methods = mthds
	case *gotypes.Signature:
		p := t.Params()
		r := t.Results()
		fn := tipe.Func{
			Params:   &tipe.Tuple{Elems: make([]tipe.Type, p.Len())},
			Results:  &tipe.Tuple{Elems: make([]tipe.Type, r.Len())},
			Variadic: t.Variadic(),
		}
		for i := 0; i < p.Len(); i++ {
			fn.Params.Elems[i] = c.fromGoType(p.At(i).Type())
		}
		for i := 0; i < r.Len(); i++ {
			fn.Results.Elems[i] = c.fromGoType(r.At(i).Type())
		}
		*res.(*tipe.Func) = fn
	}
}

func (c *Checker) goPkg(path string) (*tipe.Package, error) {
	if pkg := c.GoPkgs[path]; pkg != nil {
		return pkg, nil
	}
	goPath := path
	if path == "mat" {
		goPath = "neugram.io/ng/vendor/mat" // TODO: remove "mat" exception
	}
	// Make sure our '.a' files are fresh.
	out, err := exec.Command("go", "install", goPath).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go install %s: %v\n%s", goPath, err, out)
	}
	gopkg, err := c.ImportGo(goPath)
	if err != nil {
		return nil, err
	}
	pkg := &tipe.Package{
		GoPkg:   gopkg,
		Path:    gopkg.Path(),
		Exports: make(map[string]tipe.Type),
	}
	c.GoPkgs[path] = pkg

	for _, name := range gopkg.Scope().Names() {
		obj := gopkg.Scope().Lookup(name)
		if !obj.Exported() {
			continue
		}
		pkg.Exports[name] = c.fromGoType(obj.Type())
	}
	for len(c.GoTypesToFill) > 0 {
		for gotyp, t := range c.GoTypesToFill {
			c.fillGoType(t, gotyp)
			delete(c.GoTypesToFill, gotyp)
		}
	}
	return pkg, nil
}

func (c *Checker) ngPkg(path string) (*tipe.Package, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(c.importWalk[len(c.importWalk)-1]), path)
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("ng package import: %v", err)
		}
	}
	for i, p := range c.importWalk {
		if p == path {
			cycle := c.importWalk[i]
			for _, p := range c.importWalk[i+1:] {
				cycle += "-> " + p
			}
			cycle += "-> " + path
			return nil, fmt.Errorf("package import cycle: %s", cycle)
		}
	}
	if pkg := c.NgPkgs[path]; pkg != nil {
		return pkg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ng package import: %v", err)
	}
	c.importWalk = append(c.importWalk, path)
	oldcur := c.cur
	defer func() {
		f.Close()
		c.importWalk = c.importWalk[:len(c.importWalk)-1]
		c.cur = oldcur
	}()

	c.cur = &Scope{
		Parent: Universe,
		Objs:   make(map[string]*Obj),
	}
	if err := c.parseFile(f); err != nil {
		return nil, fmt.Errorf("ng import parse: %v", err)
	}
	pkg := &tipe.Package{
		Path:    path,
		Exports: make(map[string]tipe.Type),
	}
	c.NgPkgs[path] = pkg
	for c.cur != Universe {
		for name, obj := range c.cur.Objs {
			if !isExported(name) {
				continue
			}
			if _, exists := pkg.Exports[name]; exists {
				continue
			}
			pkg.Exports[name] = obj.Type
			fmt.Printf("package exports %q\n", name)
		}
		c.cur = c.cur.Parent
	}
	return pkg, nil
}

func isExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

func (c *Checker) parseFile(f *os.File) error {
	p := parser.New()

	scanner := bufio.NewScanner(f)
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Bytes()
		res := p.ParseLine(line)
		if len(res.Errs) > 0 {
			return fmt.Errorf("%d: %v", i+1, res.Errs[0]) // TODO: position information
		}
		for _, s := range res.Stmts {
			c.Add(s)
			if len(c.Errs) > 0 {
				return fmt.Errorf("%d: typecheck: %v\n", i+1, c.Errs[0])
			}
		}
	}
	return scanner.Err()
}

func (c *Checker) checkImport(s *stmt.Import) {
	if strings.HasPrefix(s.Path, "/") {
		c.errorf("imports do not support absolute paths: %q", s.Path)
		return
	}
	var pkg *tipe.Package
	var err error
	if strings.HasSuffix(s.Path, ".ng") {
		pkg, err = c.ngPkg(s.Path)
		if err != nil {
			c.errorf("importing of ng package failed: %v", err)
			return
		}
		if s.Name == "" {
			s.Name = strings.TrimSuffix(filepath.Base(s.Path), ".ng")
		}
	} else {
		pkg, err = c.goPkg(s.Path)
		if err != nil {
			c.errorf("importing of go package failed: %v", err)
			return
		}
		if s.Name == "" {
			s.Name = pkg.GoPkg.(*gotypes.Package).Name()
		}
	}
	obj := &Obj{
		Kind: ObjPkg,
		Type: pkg,
		// TODO Decl?
	}
	c.cur.Objs[s.Name] = obj
}

func (c *Checker) expr(e expr.Expr) (p partial) {
	// TODO more mode adjustment
	p = c.exprPartial(e, hintElideErr)
	if p.mode == modeTypeExpr {
		p.mode = modeInvalid
		c.errorf("type %s is not an expression", format.Type(p.typ))
	}
	return p
}

func (c *Checker) exprNoElide(e expr.Expr) (p partial) {
	p = c.exprPartial(e, hintNone)
	// TODO: dedup with expr()
	if p.mode == modeTypeExpr {
		p.mode = modeInvalid
		c.errorf("type %s is not an expression", format.Type(p.typ))
	}
	return p
}

func (c *Checker) exprType(e expr.Expr) tipe.Type {
	p := c.exprPartial(e, hintNone)
	if p.mode == modeTypeExpr {
		return p.typ
	}
	switch e := p.expr.(type) {
	case *expr.Selector:
		if pkg, ok := e.Left.(*expr.Ident); ok {
			if t := c.lookupPkgType(pkg.Name, e.Right.Name); t != nil {
				return t
			}
		}
	}
	c.errorf("argument %s is not a type (%#+v)", format.Expr(e), p)
	return nil
}

func (c *Checker) resolve(t tipe.Type) (ret tipe.Type, resolved bool) {
	switch t := t.(type) {
	case *tipe.Func:
		p, r1 := c.resolve(t.Params)
		r, r2 := c.resolve(t.Results)
		t.Params = p.(*tipe.Tuple)
		t.Results = r.(*tipe.Tuple)
		return t, r1 && r2
	case *tipe.Interface:
		resolved := true
		m := make(map[string]*tipe.Func, len(t.Methods))
		for name, f := range t.Methods {
			f, r1 := c.resolve(f)
			m[name] = f.(*tipe.Func)
			resolved = resolved && r1
		}
		t.Methods = m
		return t, resolved
	case *tipe.Map:
		var r1, r2 bool
		t.Key, r1 = c.resolve(t.Key)
		t.Value, r2 = c.resolve(t.Value)
		return t, r1 && r2
	case *tipe.Methodik:
		t.Type, resolved = c.resolve(t.Type)
		for i, f := range t.Methods {
			f, r1 := c.resolve(f)
			t.Methods[i] = f.(*tipe.Func)
			resolved = resolved && r1
		}
		return t, resolved
	case *tipe.Pointer:
		t.Elem, resolved = c.resolve(t.Elem)
		return t, resolved
	case *tipe.Array:
		t.Elem, resolved = c.resolve(t.Elem)
		return t, resolved
	case *tipe.Slice:
		t.Elem, resolved = c.resolve(t.Elem)
		return t, resolved
	case *tipe.Chan:
		t.Elem, resolved = c.resolve(t.Elem)
		return t, resolved
	case *tipe.Struct:
		usesNum := false
		resolved := true
		for i, f := range t.Fields {
			f, r1 := c.resolve(f)
			usesNum = usesNum || tipe.UsesNum(f)
			t.Fields[i] = f
			resolved = resolved && r1
		}
		if usesNum {
			t.Spec.Num = tipe.Num
		}
		return t, resolved
	case *tipe.Table:
		t.Type, resolved = c.resolve(t.Type)
		return t, resolved
	case *tipe.Tuple:
		if t == nil {
			return t, true
		}
		resolved = true
		for i, e := range t.Elems {
			var r bool
			t.Elems[i], r = c.resolve(e)
			resolved = resolved && r
		}
		return t, resolved
	case *tipe.Unresolved:
		if t.Package != "" {
			res := c.lookupPkgType(t.Package, t.Name)
			if res == nil {
				return t, false
			}
			return res, true
		}
		obj := c.cur.LookupRec(t.Name)
		if obj == nil {
			c.errorf("type %s not declared", t.Name)
			return t, false
		}
		if obj.Kind != ObjType {
			c.errorf("symbol %s is not a type", t.Name)
			return t, false
		}
		return obj.Type, true
		// TODO many more types
	default:
		return t, true
	}
}

func (c *Checker) lookupPkgType(pkgName, sel string) tipe.Type {
	name := pkgName + "." + sel
	obj := c.cur.LookupRec(pkgName)
	if obj == nil {
		c.errorf("undefined %s in %s", pkgName, name)
		return nil
	}
	if obj.Kind != ObjPkg {
		c.errorf("%s is not a packacge", pkgName)
		return nil
	}
	pkg := obj.Type.(*tipe.Package)
	res := pkg.Exports[sel]
	if res == nil {
		c.errorf("%s not in package %s", name, pkgName)
		return nil
	}
	return res
}

func (c *Checker) exprBuiltinCall(e *expr.Call) partial {
	p := c.expr(e.Func)
	p.expr = e

	switch p.typ.(tipe.Builtin) {
	case tipe.Append:
		if len(e.Args) == 0 {
			p.mode = modeInvalid
			c.errorf("too few arguments to append")
			return p
		}
		arg0 := c.expr(e.Args[0])
		slice, isSlice := tipe.Underlying(arg0.typ).(*tipe.Slice)
		if !isSlice {
			p.mode = modeInvalid
			c.errorf("first argument to append must be a slice, got %s", format.Type(arg0.typ))
			return p
		}
		p.typ = arg0.typ
		// TODO: append(x, y...)
		for _, arg := range e.Args[1:] {
			argp := c.expr(arg)
			argpTyp := argp.typ
			c.convert(&argp, slice.Elem)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use %s (type %s) as type %s in argument to append", format.Expr(arg), format.Type(argpTyp), format.Type(slice.Elem))
				return p
			}
		}
		return p
	case tipe.Close:
		p.typ = nil
	case tipe.ComplexFunc:
		p.typ = tipe.Complex
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorf("complex takes two arguments, got %d", len(e.Args))
			return p
		}
		arg0 := c.expr(e.Args[0])
		arg1 := c.expr(e.Args[1])
		switch arg0.typ {
		case tipe.UntypedInteger:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float, tipe.Float32, tipe.Float64:
			default:
				p.mode = modeInvalid
				c.errorf("second argument to complex must be a float, got %s", format.Type(arg1.typ))
				return p
			}
		case tipe.UntypedFloat:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float, tipe.Float32, tipe.Float64:
			default:
				p.mode = modeInvalid
				c.errorf("second argument to complex must be a float, got %s", format.Type(arg1.typ))
				return p
			}
		case tipe.Float:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float, tipe.Float32, tipe.Float64:
			default:
				p.mode = modeInvalid
				c.errorf("second argument to complex must be a float, got %s", format.Type(arg1.typ))
				return p
			}
		case tipe.Float32:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float, tipe.Float32:
			case tipe.Float64:
				p.mode = modeInvalid
				c.errorf("invalid operation: complex(%s, %s) (mismatched types float32 and float64)", format.Expr(e.Args[0]), format.Expr(e.Args[1]))
				return p
			default:
				p.mode = modeInvalid
				c.errorf("second argument to complex must be a float, got %s", format.Type(arg1.typ))
				return p
			}
		case tipe.Float64:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float, tipe.Float64:
			case tipe.Float32:
				p.mode = modeInvalid
				c.errorf("invalid operation: complex(%s, %s) (mismatched types float64 and float32)", format.Expr(e.Args[0]), format.Expr(e.Args[1]))
				return p
			default:
				p.mode = modeInvalid
				c.errorf("second argument to complex must be a float, got %s", format.Type(arg1.typ))
				return p
			}
		default:
			p.mode = modeInvalid
			c.errorf("first argument to complex must be a float, got %s", format.Type(arg0.typ))
			return p
		}
		return p
	case tipe.Copy:
		p.typ = tipe.Int
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorf("copy takes two arguments, got %d", len(e.Args))
			return p
		}
		dst, src := c.expr(e.Args[0]), c.expr(e.Args[1])
		var srcElem, dstElem tipe.Type
		srcTyp := tipe.Underlying(src.typ)
		if t, isSlice := srcTyp.(*tipe.Slice); isSlice {
			srcElem = t.Elem
		} else if srcTyp == tipe.String {
			srcElem = tipe.Byte
		} else {
			p.mode = modeInvalid
			c.errorf("copy source must be slice or string, got %s", format.Type(src.typ))
			return p
		}
		if t, isSlice := tipe.Underlying(dst.typ).(*tipe.Slice); isSlice {
			dstElem = t.Elem
		} else {
			p.mode = modeInvalid
			c.errorf("copy destination must be a slice, have %s", format.Type(dst.typ))
			return p
		}
		if !c.convertible(dstElem, srcElem) {
			p.mode = modeInvalid
			c.errorf("copy source type %s is not convertible to destination %s", format.Type(dstElem), format.Type(srcElem))
			return p
		}
		return p
	case tipe.Delete:
		p.typ = nil
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorf("delete takes exactly two arguments, got %d", len(e.Args))
			return p
		}
		arg0, arg1 := c.expr(e.Args[0]), c.expr(e.Args[1])
		var keyType tipe.Type
		if t, isMap := tipe.Underlying(arg0.typ).(*tipe.Map); isMap {
			keyType = t.Key
		} else {
			p.mode = modeInvalid
			c.errorf("first argument to delete must be a map, got %s (type %s)", format.Expr(e.Args[0]), format.Type(arg0.typ))
			return p
		}
		if !c.convertible(keyType, arg1.typ) {
			p.mode = modeInvalid
			c.errorf("second argument to delete must match the key type %s, got type %s", format.Type(keyType), format.Type(arg1.typ))
			return p
		}
		return p
	case tipe.Imag:
		p.typ = tipe.Float
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("imag takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		arg := c.expr(e.Args[0])
		switch arg.typ {
		case tipe.Complex, tipe.Complex64, tipe.Complex128, tipe.UntypedComplex:
		default:
			p.mode = modeInvalid
			c.errorf("argument to imag must be a complex, got %s (type %s)", format.Expr(e.Args[0]), format.Type(arg.typ))
			return p
		}
		return p
	case tipe.Len, tipe.Cap:
		p.typ = tipe.Int
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("%s takes exactly 1 argument, got %d", format.Type(p.typ), len(e.Args))
			return p
		}
		arg0 := c.expr(e.Args[0])
		switch t := tipe.Underlying(arg0.typ).(type) {
		case *tipe.Array, *tipe.Slice, *tipe.Map, *tipe.Chan:
			return p
		case tipe.Basic:
			if t == tipe.String {
				return p
			}
		}
		p.mode = modeInvalid
		c.errorf("invalid argument %s (%s) for %s ", format.Expr(e.Args[0]), format.Type(arg0.typ), format.Type(p.typ))
		return p
	case tipe.Make:
		switch len(e.Args) {
		case 3:
			arg := c.expr(e.Args[2])
			c.convert(&arg, tipe.Int)
			if arg.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			fallthrough
		case 2:
			arg := c.expr(e.Args[1])
			c.convert(&arg, tipe.Int)
			if arg.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			fallthrough
		case 1:
		default:
			p.mode = modeInvalid
			c.errorf("make requires 1-3 arguments")
			return p
		}

		arg0 := c.exprType(e.Args[0])
		if arg0 != nil {
			switch t := arg0.(type) {
			case *tipe.Slice, *tipe.Map, *tipe.Chan:
				p.typ = t
			}
		}
		if p.typ == nil {
			p.mode = modeInvalid
			c.errorf("make argument must be a slice, map, or channel")
		}
		return p
	case tipe.New:
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("new takes exactly one argument, got %d", len(e.Args))
			return p
		}
		arg0 := c.exprType(e.Args[0])
		if arg0 == nil {
			p.mode = modeInvalid
			c.errorf("argument to new must be a type")
			return p
		}
		e.Args[0] = &expr.Type{Type: arg0}
		p.typ = &tipe.Pointer{Elem: arg0}
		return p
	case tipe.Panic:
		p.typ = nil
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("panic takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		if arg0 := c.expr(e.Args[0]); arg0.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		return p
	case tipe.Real:
		p.typ = tipe.Float
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("real takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		arg := c.expr(e.Args[0])
		switch arg.typ {
		case tipe.Complex, tipe.Complex64, tipe.Complex128, tipe.UntypedComplex:
		default:
			p.mode = modeInvalid
			c.errorf("argument to real must be a complex, got %s (type %s)", format.Expr(e.Args[0]), format.Type(arg.typ))
			return p
		}
		return p
	case tipe.Recover:
	default:
		panic(fmt.Sprintf("unknown builtin: %s", p.typ))
	}
	panic("TODO builtin")
}

func (c *Checker) exprPartialCall(e *expr.Call) partial {
	p := c.exprPartial(e.Func, hintElideErr)
	switch p.mode {
	default:
		panic(fmt.Sprintf("unreachable, unknown call mode: %v", p.mode))
	case modeInvalid:
		return p
	case modeTypeExpr:
		// type conversion
		if len(e.Args) == 0 {
			p.mode = modeInvalid
			c.errorf("type conversion to %s is missing an argument", format.Type(p.typ))
			return p
		} else if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("type conversion to %s has too many arguments", format.Type(p.typ))
			return p
		}
		t := p.typ
		p = c.expr(e.Args[0])
		if p.mode == modeInvalid {
			return p
		}
		c.convert(&p, t)
		p.expr = e
		return p
	case modeVar, modeFunc:
		// function call, below
	}

	if _, ok := p.typ.(tipe.Builtin); ok {
		return c.exprBuiltinCall(e)
	}

	p.mode = modeVar
	p.expr = e
	funct := p.typ.(*tipe.Func)
	var params, results []tipe.Type
	if funct.Params != nil {
		params = funct.Params.Elems
	}
	if funct.Results != nil {
		results = funct.Results.Elems
	}

	switch len(results) {
	case 0:
		p.typ = nil
	case 1:
		p.typ = results[0]
	default:
		p.typ = funct.Results
	}

	if funct.Variadic {
		if len(e.Args) < len(params)-1 {
			p.mode = modeInvalid
			c.errorf("too few arguments (%d) to variadic function %s", len(e.Args), format.Type(funct))
			return p
		}
		for i := 0; i < len(params)-1; i++ {
			t := params[i]
			argp := c.expr(e.Args[i])
			c.convert(&argp, t)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use type %s as type %s in argument %d to function", format.Type(argp.typ), format.Type(t), i)
				return p
			}
		}
		vart := params[len(params)-1].(*tipe.Slice).Elem
		varargs := e.Args[len(params)-1:]
		for _, arg := range varargs {
			argp := c.exprPartial(arg, hintNone)
			if argp.mode == modeTypeExpr {
				p.mode = modeInvalid
				c.errorf("type %s is not an expression", format.Type(p.typ))
				return p
			}
			c.convert(&argp, vart)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use type %s as type %s in variadic argument to function", format.Type(argp.typ), format.Type(vart))
				return p
			}
		}
		return p
	}

	if len(e.Args) != len(params) {
		p.mode = modeInvalid
		c.errorf("wrong number of arguments (%d) to function %s", len(e.Args), format.Type(funct))
		return p
	}
	for i, arg := range e.Args {
		t := params[i]
		argp := c.expr(arg)
		//fmt.Printf("argp i=%d: %#+v (arg=%#+v)\n", i, argp, arg)
		c.convert(&argp, t)
		if argp.mode == modeInvalid {
			p.mode = modeInvalid
			c.errorf("cannot use type %s as type %s in argument to function", format.Type(argp.typ), format.Type(t))
			break
		}
	}

	return p
}

func (c *Checker) exprPartial(e expr.Expr, hint typeHint) (p partial) {
	defer func() {
		if p.mode == modeConst {
			c.Values[p.expr] = p.val
		}
		if p.mode != modeInvalid {
			c.Types[p.expr] = p.typ
		}
	}()
	p.expr = e
	switch e := e.(type) {
	case *expr.Ident:
		if e.Name == "_" {
			p.mode = modeInvalid
			c.errorf("cannot use _ as a value")
			return p
		}
		obj := c.cur.LookupRec(e.Name)
		if obj == nil {
			p.mode = modeInvalid
			c.errorf("undeclared identifier: %s", e.Name)
			return p
		}
		c.Defs[e] = obj // TODO Defs is more than definitions? rename?
		// TODO: is a partial's mode just an ObjKind?
		// not every partial has an Obj, but we could reuse the type.
		switch obj.Kind {
		case ObjVar, ObjPkg:
			p.mode = modeVar
		case ObjConst:
			p.mode = modeConst
			if v, ok := obj.Decl.(constant.Value); ok {
				p.val = v
			}
		case ObjType:
			p.mode = modeTypeExpr
		}
		p.typ = obj.Type
		return p
	case *expr.BasicLiteral:
		// TODO: use constant.Value in BasicLiteral directly.
		switch v := e.Value.(type) {
		case *big.Int:
			p.mode = modeConst
			p.typ = tipe.UntypedInteger
			p.val = constant.MakeFromLiteral(v.String(), gotoken.INT, 0)
		case *big.Float:
			p.mode = modeConst
			p.typ = tipe.UntypedFloat
			p.val = constant.MakeFromLiteral(v.String(), gotoken.FLOAT, 0)
		case string:
			p.mode = modeConst
			p.typ = tipe.UntypedString
			p.val = constant.MakeFromLiteral(v, gotoken.STRING, 0)
		case rune:
			p.mode = modeConst
			p.typ = tipe.UntypedRune
			p.val = constant.MakeFromLiteral(string(v), gotoken.CHAR, 0)
		case bool:
			p.mode = modeConst
			p.typ = tipe.UntypedBool
			p.val = constant.MakeBool(v)
		}
		return p
	case *expr.FuncLiteral:
		c.pushScope()
		defer c.popScope()
		c.cur.foundInParent = make(map[string]bool)
		c.cur.foundMdikInParent = make(map[*tipe.Methodik]bool)
		if e.Type.Params != nil {
			for i, t := range e.Type.Params.Elems {
				t, _ = c.resolve(t)
				e.Type.Params.Elems[i] = t
				obj := &Obj{
					Kind: ObjVar,
					Type: t,
				}
				c.cur.Objs[e.ParamNames[i]] = obj
			}
		}
		if e.Type.Results != nil {
			for i, t := range e.Type.Results.Elems {
				e.Type.Results.Elems[i], _ = c.resolve(t)
			}
		}
		c.stmt(e.Body.(*stmt.Block), e.Type.Results)
		for name := range c.cur.foundInParent {
			e.Type.FreeVars = append(e.Type.FreeVars, name)
		}
		for mdik := range c.cur.foundMdikInParent {
			e.Type.FreeMdik = append(e.Type.FreeMdik, mdik)
		}
		p.typ = e.Type
		p.mode = modeFunc
		return p
	case *expr.CompLiteral:
		p.mode = modeVar
		structName := fmt.Sprintf("%s", e.Type)
		if t, resolved := c.resolve(e.Type); resolved {
			e.Type = t
			p.typ = t
		} else {
			p.mode = modeInvalid
			return p
		}
		t, isStruct := tipe.Underlying(e.Type).(*tipe.Struct)
		if !isStruct {
			c.errorf("cannot construct type %s with a composite literal", format.Type(e.Type))
			p.mode = modeInvalid
			return p
		}
		elemsp := make([]partial, len(e.Elements))
		for i, elem := range e.Elements {
			elemsp[i] = c.expr(elem)
			if elemsp[i].mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
		}
		if len(e.Keys) == 0 {
			if len(e.Elements) == 0 {
				return p
			}
			if len(e.Elements) != len(t.Fields) {
				c.errorf("wrong number of elements, %d, when %s expects %d", len(e.Elements), structName, len(t.Fields))
				p.mode = modeInvalid
				return p
			}
			for i, ft := range t.Fields {
				c.assign(&elemsp[i], ft)
				if elemsp[i].mode == modeInvalid {
					p.mode = modeInvalid
					return p
				}
			}
		} else {
			namedp := make(map[string]partial)
			for i, elemp := range elemsp {
				ident, ok := e.Keys[i].(*expr.Ident)
				if !ok {
					c.errorf("invalid field name %s in struct initializer", e.Keys[i])
					p.mode = modeInvalid
					return p
				}
				namedp[ident.Name] = elemp
			}
			for i, ft := range t.Fields {
				elemp, found := namedp[t.FieldNames[i]]
				if !found {
					continue
				}
				c.assign(&elemp, ft)
				if elemp.mode == modeInvalid {
					p.mode = modeInvalid
					return p
				}
			}
			//panic("TODO: named CompLiteral")
		}
		if p.mode != modeInvalid {
			p.expr = e
		}
		return p

	case *expr.MapLiteral:
		p.mode = modeVar
		if t, resolved := c.resolve(e.Type); resolved {
			e.Type = t
			p.typ = t
		} else {
			p.mode = modeInvalid
			return p
		}
		t, isMap := tipe.Underlying(e.Type).(*tipe.Map)
		if !isMap {
			c.errorf("cannot construct type %s with a map composite literal", format.Type(e.Type))
			p.mode = modeInvalid
			return p
		}
		for _, k := range e.Keys {
			kp := c.expr(k)
			if kp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			c.assign(&kp, t.Key)
			if kp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
		}
		for _, v := range e.Values {
			vp := c.expr(v)
			if vp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			c.assign(&vp, t.Value)
			if vp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
		}
		p.expr = e
		return p

	case *expr.SliceLiteral:
		p.mode = modeVar
		var sliceType *tipe.Slice
		if t, resolved := c.resolve(e.Type); resolved {
			t, isSlice := t.(*tipe.Slice)
			if !isSlice {
				c.errorf("type %s is not a slice", format.Type(t))
				p.mode = modeInvalid
				return p
			}
			e.Type = t
			p.typ = t
			sliceType = t
		} else {
			p.mode = modeInvalid
			return p
		}

		for _, v := range e.Elems {
			vp := c.expr(v)
			if vp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			c.assign(&vp, sliceType.Elem)
			if vp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
		}
		p.expr = e
		return p

	case *expr.TableLiteral:
		p.mode = modeVar

		var elemType tipe.Type
		if t, resolved := c.resolve(e.Type); resolved {
			t, isTable := t.(*tipe.Table)
			if !isTable {
				c.errorf("type %s is not a table", format.Type(t))
				p.mode = modeInvalid
				return p
			}
			elemType = t.Type
			e.Type = t
			p.typ = t
		} else {
			p.mode = modeInvalid
			return p
		}

		for _, colNameExpr := range e.ColNames {
			colp := c.expr(colNameExpr)
			c.assign(&colp, tipe.String)
			if colp.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
		}
		if len(e.Rows) == 0 {
			return p
		}

		// Check everyone agrees on the width.
		w := len(e.Rows[0])
		if len(e.ColNames) > 0 && len(e.ColNames) != w {
			c.errorf("table literal has %d column names but a width of %d", len(e.ColNames), w)
			p.mode = modeInvalid
			return p
		}
		for _, r := range e.Rows {
			if len(r) != w {
				c.errorf("table literal has rows of different lengths (%d and %d)", w, len(r))
				p.mode = modeInvalid
				return p
			}
			for _, elem := range r {
				elemp := c.expr(elem)
				c.assign(&elemp, elemType)
				if elemp.mode == modeInvalid {
					p.mode = modeInvalid
					return p
				}
			}
		}
		return p

	case *expr.Type:
		if t, resolved := c.resolve(e.Type); resolved {
			e.Type = t
			p.mode = modeTypeExpr
			p.typ = e.Type
			return p
		}
		p.mode = modeInvalid
		return p

	case *expr.Unary:
		switch e.Op {
		case token.LeftParen, token.Not, token.Sub:
			sub := c.exprPartial(e.Expr, hintElideErr)
			p.mode = sub.mode
			p.typ = sub.typ
			return p
		case token.Ref:
			sub := c.expr(e.Expr)
			if sub.mode == modeInvalid {
				return p
			}
			p.mode = modeVar
			p.typ = &tipe.Pointer{Elem: sub.typ}
			if goElem := c.GoEquiv[sub.typ]; goElem != nil {
				goType := gotypes.NewPointer(goElem)
				c.GoEquiv[p.typ] = goType
				c.GoTypes[goType] = p.typ
			}
			return p
		case token.Mul:
			sub := c.expr(e.Expr)
			if sub.mode == modeInvalid {
				return p
			}
			if t, ok := sub.typ.(*tipe.Pointer); ok {
				p.mode = modeVar
				p.typ = t.Elem
				return p
			}
			c.errorf("invalid dereference of %s", e.Expr)
			p.mode = modeInvalid
			return p
		case token.ChanOp:
			sub := c.expr(e.Expr)
			if sub.mode == modeInvalid {
				p.mode = modeInvalid
				return p
			}
			t, ok := sub.typ.(*tipe.Chan)
			if !ok {
				c.errorf("receive from non-chan type %s", format.Type(sub.typ))
				p.mode = modeInvalid
				return p
			}
			p.mode = modeVar
			p.typ = t.Elem
			return p
		}
	case *expr.Binary:
		left := c.expr(e.Left)
		if left.mode == modeInvalid {
			return left
		}
		right := c.expr(e.Right)
		if right.mode == modeInvalid {
			return right
		}
		ltOrig, rtOrig := left.typ, right.typ
		c.constrainUntyped(&left, right.typ)
		c.constrainUntyped(&right, left.typ)
		left.expr = e

		switch e.Op {
		case token.Equal, token.NotEqual, token.LessEqual, token.GreaterEqual, token.Less, token.Greater:
			// comparison
			lt, rt := left.typ, right.typ
			if !c.assignable(lt, rt) && !c.assignable(rt, lt) {
				c.errorf("incomparable types %s and %s", format.Type(lt), format.Type(rt))
				left.mode = modeInvalid
				return left
			}
			switch e.Op {
			case token.Equal, token.NotEqual:
				if !isComparable(lt) {
					if canBeNil(lt) || canBeNil(rt) {
						if ltOrig != tipe.UntypedNil && rtOrig != tipe.UntypedNil {
							c.errorf("type %s only comparable to nil", format.Type(lt))
							left.mode = modeInvalid
							return left
						}
					} else {
						c.errorf("incomparable type %s", format.Type(lt))
						left.mode = modeInvalid
						return left
					}
				}
			case token.LessEqual, token.GreaterEqual, token.Less, token.Greater:
				if !isOrdered(lt) {
					c.errorf("unordered type %s", format.Type(lt))
					left.mode = modeInvalid
					return left
				}
			}
			left.typ = tipe.Bool
			return left
		}

		// TODO check for division by zero
		if left.mode == modeConst && right.mode == modeConst {
			left.val = constant.BinaryOp(left.val, convGoOp(e.Op), right.val)
			// TODO check rounding
			// TODO check for comparison, result is untyped bool
			return left
		}

		if !tipe.Equal(left.typ, right.typ) {
			c.errorf("inoperable types %s and %s", format.Type(left.typ), format.Type(right.typ))
			left.mode = modeInvalid
			return left
		}
		return left
	case *expr.Call:
		p := c.exprPartialCall(e)
		if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple && hint == hintElideErr {
			if IsError(tuple.Elems[len(tuple.Elems)-1]) {
				tuple.Elems = tuple.Elems[:len(tuple.Elems)-1]
				if len(tuple.Elems) == 1 {
					p.typ = tuple.Elems[0]
				}
				e.ElideError = true
			}
		}
		return p
	case *expr.Selector:
		right := e.Right.Name
		left := c.expr(e.Left)
		if left.mode == modeInvalid {
			return left
		}

		methodNames, methods := c.memory.Methods(left.typ)
		for i, name := range methodNames {
			if name == right {
				p.mode = modeVar // modeFunc?
				p.typ = methods[i]
				return
			}
		}

		lt := tipe.Underlying(left.typ)
		if t, isPtr := lt.(*tipe.Pointer); isPtr {
			lt = t.Elem
		}
		switch lt := lt.(type) {
		case *tipe.Struct:
			for i, name := range lt.FieldNames {
				if name == right {
					p.mode = modeVar
					p.typ = lt.Fields[i]
					return
				}
			}
			p.mode = modeInvalid
			c.errorf("%s undefined (type %s has no field or method %s)", e, format.Type(lt), right)
			return p
		case *tipe.Package:
			for name, t := range lt.Exports {
				if name == e.Right.Name {
					p.typ = t
					if lt.GoPkg != nil {
						s := lt.GoPkg.(*gotypes.Package).Scope()
						obj := s.Lookup(name)
						if _, isAType := obj.Type().(*gotypes.Named); isAType {
							p.mode = modeTypeExpr
							return p
						}
					}
					p.mode = modeVar // TODO modeFunc?
					return p
				}
			}
			p.mode = modeInvalid
			c.errorf("%s not in package %s", e, lt)
			return p
		}
		p.mode = modeInvalid
		c.errorf("%s undefined (type %s is not a struct or package)", e, format.Type(left.typ))
		return p
	case *expr.Index:
		left := c.expr(e.Left)
		if left.mode == modeInvalid {
			return left
		}
		lt := tipe.Underlying(left.typ)
		switch lt := lt.(type) {
		case *tipe.Map:
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot table slice %s (type %s)", e.Left, format.Type(left.typ))
				return p
			}
			ind := c.expr(e.Indicies[0])
			if ind.mode == modeInvalid {
				return ind
			}
			c.assign(&ind, lt.Key)
			if ind.mode == modeInvalid {
				return ind
			}
			p.mode = modeVar // TODO not really? because not addressable
			p.typ = lt.Value
			return p
		case *tipe.Array:
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot table slice %s (type %s)", e.Left, format.Type(left.typ))
				return p
			}
			ind := c.expr(e.Indicies[0])
			if ind.mode == modeInvalid {
				return ind
			}
			c.assign(&ind, tipe.Int)
			if ind.mode == modeInvalid {
				return ind
			}
			p.mode = modeVar
			p.typ = lt.Elem
			return p
		case *tipe.Slice:
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot table slice %s (type %s)", e.Left, format.Type(left.typ))
				return p
			}
			if s, isSlice := e.Indicies[0].(*expr.Slice); isSlice {
				p.mode = modeVar
				p.typ = left.typ
				ints := func(exprs ...expr.Expr) (p partial) {
					for _, e := range exprs {
						if e == nil {
							continue
						}
						p := c.expr(e)
						if p.mode == modeInvalid {
							return p
						}
						c.convert(&p, tipe.Int)
						if p.mode == modeInvalid {
							return p
						}
					}
					p.mode = modeVar
					return p
				}
				if p := ints(s.Low, s.High, s.Max); p.mode == modeInvalid {
					return p
				}
				return p
			}
			ind := c.expr(e.Indicies[0])
			if ind.mode == modeInvalid {
				return ind
			}
			c.assign(&ind, tipe.Int)
			if ind.mode == modeInvalid {
				return ind
			}
			p.mode = modeVar
			p.typ = lt.Elem
			return p
		case *tipe.Table:
			p.mode = modeInvalid
			c.errorf("TODO table slicing support")
			return p
		}
		if atTyp := c.memory.Method(lt, "At"); atTyp != nil {
			want := "At(i, j int) T"
			if len(e.Indicies) == 1 {
				want = "At(i int) T"
			}
			if dim := len(atTyp.Params.Elems); dim == 0 || dim > 2 || dim != len(e.Indicies) ||
				atTyp.Params.Elems[0] != tipe.Int || (dim == 2 && atTyp.Params.Elems[1] != tipe.Int) ||
				len(atTyp.Results.Elems) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot slice type %s, expecting method %q but type has %q", left.typ, want, format.Type(atTyp))
				return p
			}
			p.mode = modeVar
			p.typ = atTyp.Results.Elems[0]
			return p
		}
		if setTyp := c.memory.Method(lt, "Set"); setTyp != nil {
			p.mode = modeInvalid
			c.errorf("TODO Set index")
			return p
		}

		panic(fmt.Sprintf("typecheck.expr TODO Index: %s, %s", format.Debug(e))) //, format.Debug(tipe.Underlying(left.typ))))
	case *expr.Shell:
		p.mode = modeVar
		if hint == hintElideErr {
			p.typ = tipe.String
			e.ElideError = true
		} else {
			p.typ = &tipe.Tuple{Elems: []tipe.Type{
				tipe.String, Universe.Objs["error"].Type,
			}}
		}
		return p
	}
	panic(fmt.Sprintf("expr TODO: %s", format.Debug(e)))
}

func (c *Checker) assign(p *partial, t tipe.Type) {
	if p.mode == modeInvalid {
		return
	}
	if isUntyped(p.typ) {
		c.constrainUntyped(p, t)
		return
	}
	if !tipe.Equal(p.typ, t) { // TODO interfaces, etc
		c.errorf("cannot assign %s to %s", format.Type(p.typ), format.Type(t))
		p.mode = modeInvalid
	}
}

func (c *Checker) convert(p *partial, t tipe.Type) {
	//fmt.Printf("Checker.convert(p=%#+v, t=%s)\n", p, t)
	_, tIsConst := t.(tipe.Basic)
	if p.mode == modeConst && tIsConst {
		// TODO or integer -> string conversion
		if round(p.val, t.(tipe.Basic)) == nil {
			// p.val does not fit in t
			c.errorf("constant %s does not fit in %s", p.val, format.Type(t))
			p.mode = modeInvalid
			return
		}
	}

	if !c.convertible(t, p.typ) {
		c.errorf("cannot convert %s to %s", format.Type(p.typ), format.Type(t))
		p.mode = modeInvalid
		return
	}

	if isUntyped(p.typ) {
		c.constrainUntyped(p, t)
	} else {
		p.typ = t
	}
}

func (c *Checker) assignable(dst, src tipe.Type) bool {
	if tipe.Equal(dst, src) {
		return true
	}
	dst, src = tipe.Unalias(dst), tipe.Unalias(src)
	if src == tipe.UntypedNil {
		switch tipe.Underlying(dst).(type) {
		case *tipe.Interface, *tipe.Pointer, *tipe.Slice, *tipe.Map, *tipe.Chan, *tipe.Func:
			return true
		}
	}
	if src == tipe.UntypedString && tipe.Underlying(dst) == tipe.String {
		return true
	}

	if idst, ok := tipe.Underlying(dst).(*tipe.Interface); ok {
		// Everything can be assigned to interface{}.
		if len(idst.Methods) == 0 {
			return true
		}
		srcNames, srcTypes := c.memory.Methods(src)
		srcm := make(map[string]tipe.Type)
		for i, name := range srcNames {
			srcm[name] = srcTypes[i]
		}
		dstNames, dstTypes := c.memory.Methods(dst)
		//panic(fmt.Sprintf("dst: %s, dstNames: %s, dstTypes: %s\n", dst, dstNames, dstTypes))
		for i, name := range dstNames {
			if !tipe.Equal(dstTypes[i], srcm[name]) {
				//panic(fmt.Sprintf("assignable name=%s, dst=%s, srcm[name]=%s\n", name, pretty.Sprint(dstTypes[i]), pretty.Sprint(srcm[name])))
				// TODO: report missing method?
				return false
			}
		}
		return true
	}

	// bidirectional channels can be assigned to directional channels
	if srcCh, ok := src.(*tipe.Chan); ok && srcCh.Direction == tipe.ChanBoth {
		if dstCh, ok := dst.(*tipe.Chan); ok {
			return tipe.Equal(srcCh.Elem, dstCh.Elem)
		}
	}

	return false
}

func isString(t tipe.Type) bool {
	t = tipe.Underlying(t)
	return t == tipe.String || t == tipe.UntypedString
}

func (c *Checker) convertible(dst, src tipe.Type) bool {
	if c.assignable(dst, src) {
		return true
	}
	if tipe.Equal(tipe.Underlying(dst), tipe.Underlying(src)) {
		return true
	}
	// numerics can be converted to one another
	if tipe.IsNumeric(dst) && tipe.IsNumeric(src) {
		return true
	}
	dst, src = tipe.Unalias(dst), tipe.Unalias(src)
	if dst, isSlice := dst.(*tipe.Slice); isSlice {
		if tipe.Equal(dst.Elem, tipe.Uint8) && isString(src) {
			return true
		}
	}
	if src, isSlice := src.(*tipe.Slice); isSlice {
		if tipe.Equal(src.Elem, tipe.Uint8) && isString(dst) {
			return true
		}
	}

	// TODO several other forms of "identical" types,
	// e.g. maps where keys and value are identical,
	return false
}

func (c *Checker) constrainUntyped(p *partial, t tipe.Type) {
	if p.mode == modeInvalid || isTyped(p.typ) || t == tipe.Invalid {
		return
	}

	// catch invalid constraints
	if isUntyped(t) {
		switch {
		case t == tipe.UntypedFloat && p.typ == tipe.UntypedInteger:
			// promote untyped int to float
		case t == tipe.UntypedComplex && (p.typ == tipe.UntypedInteger || p.typ == tipe.UntypedFloat):
			// promote untyped int or float to complex
		case t == tipe.Num && (p.typ == tipe.UntypedInteger || p.typ == tipe.UntypedFloat):
			// promote untyped int or float to num type parameter
		case t != p.typ:
			c.errorf("cannot convert %s to untyped %s", format.Type(p.typ), format.Type(t))
		}
	} else {
		switch t := tipe.Unalias(t).(type) {
		case tipe.Basic:
			switch p.mode {
			case modeConst:
				p.val = round(p.val, t)
				if p.val == nil {
					c.errorf("cannot convert const %s to %s", format.Type(p.typ), format.Type(t))
					// TODO more details about why
				}
			case modeVar:
				panic(fmt.Sprintf("TODO coerce var to basic: t=%s, p.typ=%s", t, format.Type(p.typ)))
			}
		}
	}

	p.typ = t
	c.constrainExprType(p.expr, p.typ)
}

// constrainExprType descends an expression constraining the type.
func (c *Checker) constrainExprType(e expr.Expr, t tipe.Type) {
	oldt := c.Types[e]
	if oldt == t {
		return
	}
	c.Types[e] = t

	switch e := e.(type) {
	case *expr.Bad, *expr.FuncLiteral: // TODO etc
		return
	case *expr.Binary:
		if c.Values[e] != nil {
			break
		}
		switch e.Op {
		case token.Equal, token.NotEqual,
			token.Less, token.LessEqual,
			token.Greater, token.GreaterEqual:
			// comparisons generate their own bool type
			return
		}
		c.constrainExprType(e.Left, t)
		c.constrainExprType(e.Right, t)
	}

	c.Types[e] = t
}

func (c *Checker) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	c.Errs = append(c.Errs, err)
}

func (c *Checker) pushScope() {
	c.cur = &Scope{
		Parent: c.cur,
		Objs:   make(map[string]*Obj),
	}
}
func (c *Checker) popScope() {
	c.cur = c.cur.Parent
}

func convGoOp(op token.Token) gotoken.Token {
	switch op {
	case token.Add:
		return gotoken.ADD
	case token.Sub:
		return gotoken.SUB
	case token.Mul:
		return gotoken.MUL
	case token.Div:
		return gotoken.QUO // TODO: QUO_ASSIGN for int div
	case token.Rem:
		return gotoken.REM
	case token.Pow:
		panic("TODO token.Pow")
		return gotoken.REM
	case token.LogicalAnd:
		return gotoken.LAND
	case token.LogicalOr:
		return gotoken.LOR
	default:
		panic(fmt.Sprintf("typecheck: bad op: %s", op))
	}
}

func round(v constant.Value, t tipe.Basic) constant.Value {
	switch v.Kind() {
	case constant.Unknown:
		return v
	case constant.Bool:
		if t == tipe.Bool || t == tipe.UntypedBool {
			return v
		}
		return nil
	case constant.String:
		if t == tipe.String || t == tipe.UntypedString {
			return v
		}
		return nil
	case constant.Int:
		switch t {
		case tipe.Integer, tipe.UntypedInteger:
			return v
		case tipe.Float, tipe.UntypedFloat, tipe.Complex, tipe.UntypedComplex:
			return v
		case tipe.Num:
			return v
		case tipe.Int:
			if i, ok := constant.Int64Val(v); ok {
				if int64(int(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Int8:
			if i, ok := constant.Int64Val(v); ok {
				if int64(int8(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Int16:
			if i, ok := constant.Int64Val(v); ok {
				if int64(int16(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Int32:
			if i, ok := constant.Int64Val(v); ok {
				if int64(int32(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Int64:
			if _, ok := constant.Int64Val(v); ok {
				return v
			} else {
				return nil
			}
		case tipe.Uint:
			if i, ok := constant.Uint64Val(v); ok {
				if uint64(uint(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Uint8:
			if i, ok := constant.Uint64Val(v); ok {
				if uint64(uint8(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Uint16:
			if i, ok := constant.Uint64Val(v); ok {
				if uint64(uint16(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Uint32:
			if i, ok := constant.Uint64Val(v); ok {
				if uint64(uint32(i)) != i {
					return nil
				}
				return v
			} else {
				return nil
			}
		case tipe.Uint64:
			if _, ok := constant.Uint64Val(v); ok {
				return v
			} else {
				return nil
			}

		case tipe.Float32:
			r, _ := constant.Float32Val(v)
			return constant.MakeFloat64(float64(r))
		case tipe.Float64:
			r, _ := constant.Float64Val(v)
			return constant.MakeFloat64(float64(r))
		case tipe.Complex64:
			re, _ := constant.Float32Val(v)
			return constant.ToComplex(constant.MakeFloat64(float64(re)))
		case tipe.Complex128:
			re, _ := constant.Float64Val(v)
			return constant.ToComplex(constant.MakeFloat64(float64(re)))
		}
	case constant.Float:
		switch t {
		case tipe.Float, tipe.UntypedFloat, tipe.UntypedComplex:
			return v
		case tipe.Float32:
			r, _ := constant.Float32Val(v)
			return constant.MakeFloat64(float64(r))
		case tipe.Float64:
			r, _ := constant.Float64Val(v)
			return constant.MakeFloat64(float64(r))
		case tipe.Complex64:
			re, _ := constant.Float32Val(v)
			return constant.ToComplex(constant.MakeFloat64(float64(re)))
		case tipe.Complex128:
			re, _ := constant.Float64Val(v)
			return constant.ToComplex(constant.MakeFloat64(float64(re)))
		case tipe.Num:
			return v
		}
	case constant.Complex:
		switch t {
		case tipe.UntypedComplex:
			return v
		case tipe.Complex64:
			re, _ := constant.Float32Val(constant.Real(v))
			im, _ := constant.Float32Val(constant.Imag(v))
			return constant.ToComplex(constant.BinaryOp(
				constant.MakeFloat64(float64(re)),
				gotoken.ADD,
				constant.MakeFloat64(float64(im)),
			))
		case tipe.Complex128:
			re, _ := constant.Float64Val(constant.Real(v))
			im, _ := constant.Float64Val(constant.Imag(v))
			return constant.ToComplex(constant.BinaryOp(
				constant.MakeFloat64(float64(re)),
				gotoken.ADD,
				constant.MakeFloat64(float64(im)),
			))
		}
	}
	// TODO many more comparisons
	return nil
}

func (c *Checker) Add(s stmt.Stmt) tipe.Type {
	return c.stmt(s, nil)
}

func (c *Checker) Lookup(name string) *Obj {
	return c.cur.LookupRec(name)
}

type Scope struct {
	Parent *Scope
	Objs   map[string]*Obj

	// foundInParent tracks variables which were found in the Parent scope.
	// foundMdikInParent tracks any Methodik defined up the scope chain.
	// These are used to build a list of free variables.
	foundInParent     map[string]bool
	foundMdikInParent map[*tipe.Methodik]bool
	// TODO: NumSpec tipe.Type?
}

func (s *Scope) LookupRec(name string) *Obj {
	if o := s.Objs[name]; o != nil {
		return o
	}
	if s.Parent == nil {
		return nil
	}
	o := s.Parent.LookupRec(name)
	if o == nil {
		return nil
	}
	if s.foundInParent != nil && (o.Kind == ObjVar || o.Kind == ObjPkg) {
		s.foundInParent[name] = true
	}
	if s.foundMdikInParent != nil && o.Kind == ObjType {
		if mdik, ok := o.Type.(*tipe.Methodik); ok {
			s.foundMdikInParent[mdik] = true
		}
	}
	return o
}

type ObjKind int

const (
	ObjUnknown ObjKind = iota
	ObjVar
	ObjConst
	ObjPkg
	ObjType
)

func (o ObjKind) String() string {
	switch o {
	case ObjUnknown:
		return "ObjUnknown"
	case ObjVar:
		return "ObjVar"
	case ObjConst:
		return "ObjConst"
	case ObjPkg:
		return "ObjPkg"
	case ObjType:
		return "ObjType"
	default:
		return fmt.Sprintf("ObjKind(%d)", int(o))
	}
}

// An Obj represents a declared constant, type, variable, or function.
type Obj struct {
	Kind ObjKind
	Type tipe.Type
	Decl interface{} // *expr.FuncLiteral, *stmt.MethodikDecl, constant.Value
	Used bool
}

func isTyped(t tipe.Type) bool {
	return t != tipe.Invalid && !isUntyped(t)
}

func isUntyped(t tipe.Type) bool {
	switch t {
	case tipe.UntypedNil, tipe.UntypedBool, tipe.UntypedString, tipe.UntypedRune,
		tipe.UntypedInteger, tipe.UntypedFloat, tipe.UntypedComplex:
		return true
	}
	return false
}

func isComparable(t tipe.Type) bool {
	switch t := tipe.Underlying(t).(type) {
	case tipe.Basic:
		return t != tipe.Invalid && t != tipe.UntypedNil
	case *tipe.Chan, *tipe.Interface, *tipe.Pointer:
		return true
	case *tipe.Struct:
		for _, f := range t.Fields {
			if !isComparable(f) {
				return false
			}
		}
		return true
	default:
		return false
	}
	// TODO Array, Table
}

func isOrdered(t tipe.Type) bool {
	switch tipe.Underlying(t) {
	case tipe.Num, tipe.Byte, tipe.Rune, tipe.Integer, tipe.Float, tipe.Complex, tipe.String,
		tipe.Int, tipe.Int8, tipe.Int16, tipe.Int32, tipe.Int64,
		tipe.Uint, tipe.Uint8, tipe.Uint16, tipe.Uint32, tipe.Uint64,
		tipe.Float32, tipe.Float64,
		tipe.Complex64, tipe.Complex128,
		tipe.UntypedInteger, tipe.UntypedFloat, tipe.UntypedComplex, tipe.UntypedString:
		return true
	default:
		return false
	}
}

func canBeNil(t tipe.Type) bool {
	// TODO: unsafe.Pointer
	switch tipe.Underlying(t).(type) {
	case *tipe.Chan, *tipe.Interface, *tipe.Map, *tipe.Pointer, *tipe.Slice, *tipe.Func:
		return true
	default:
		return false
	}
}

func defaultType(t tipe.Type) tipe.Type {
	b, ok := t.(tipe.Basic)
	if !ok {
		return t
	}
	switch b {
	case tipe.UntypedBool:
		return tipe.Bool
	case tipe.UntypedString:
		return tipe.String
	case tipe.UntypedInteger:
		return tipe.Int // tipe.Num
	case tipe.UntypedFloat:
		return tipe.Float64 // tipe.Num
	case tipe.UntypedComplex:
		return tipe.Complex128 // tipe.Num
	}
	return t
}
