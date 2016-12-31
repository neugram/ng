// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package typecheck is a Neugram type checker.
package typecheck

import (
	"bytes"
	"fmt"
	"go/constant"
	goimporter "go/importer"
	gotoken "go/token"
	gotypes "go/types"
	"math/big"

	"neugram.io/lang/expr"
	"neugram.io/lang/stmt"
	"neugram.io/lang/tipe"
	"neugram.io/lang/token"
)

type Checker struct {
	ImportGo func(path string) (*gotypes.Package, error)

	// TODO: we could put these on our AST. Should we?
	Types         map[expr.Expr]tipe.Type
	Defs          map[*expr.Ident]*Obj
	Values        map[expr.Expr]constant.Value
	GoPkgs        map[string]*tipe.Package // path -> pkg
	GoTypes       map[gotypes.Type]tipe.Type
	GoTypesToFill map[gotypes.Type]tipe.Type
	// TODO: GoEquiv is tricky and deserving of docs. Particular type instance is associated with a Go type. That means EqualType(t1, t2)==true but t1 could have GoEquiv and t2 not.
	GoEquiv map[tipe.Type]gotypes.Type
	NumSpec map[expr.Expr]tipe.Basic // *tipe.Call, *tipe.CompLiteral -> numeric basic type
	Errs    []error

	cur *Scope

	memory *tipe.Memory
}

func New() *Checker {
	return &Checker{
		ImportGo:      goimporter.Default().Import,
		Types:         make(map[expr.Expr]tipe.Type),
		Defs:          make(map[*expr.Ident]*Obj),
		Values:        make(map[expr.Expr]constant.Value),
		GoPkgs:        make(map[string]*tipe.Package),
		GoTypes:       make(map[gotypes.Type]tipe.Type),
		GoTypesToFill: make(map[gotypes.Type]tipe.Type),
		GoEquiv:       make(map[tipe.Type]gotypes.Type), // TODO remove?
		cur: &Scope{
			Parent: Universe,
			Objs:   make(map[string]*Obj),
		},
		memory: tipe.NewMemory(),
	}
}

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
			p := c.expr(rhs)
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
			partials = append(partials, c.expr(rhs))
		}
		if len(s.Left) == len(partials)-1 && IsError(partials[len(partials)-1].typ) {
			// func f() (T, error) { ... )
			// x := f()
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
				lhsP := c.expr(lhs)
				c.assign(&p, lhsP.typ)
			}
		}
		return nil

	case *stmt.Simple:
		p := c.expr(s.Expr)
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
		c.expr(s.Cond)
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
				c.errorf("cannot use %s as %s in return argument", got[i], want[i])
			}
		}
		return nil

	case *stmt.Import:
		c.checkImport(s)
		return nil

	default:
		panic(fmt.Sprintf("typecheck: unknown stmt %T", s))
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
		case gotypes.Float32:
			return tipe.Float32
		case gotypes.Float64:
			return tipe.Float64
		}
	case *gotypes.Named:
		if t.Obj().Id() == goErrorID {
			return Universe.Objs["error"].Type
		}
		return new(tipe.Methodik)
	case *gotypes.Slice:
		return &tipe.Slice{}
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
	case *gotypes.Slice:
		res.(*tipe.Slice).Elem = c.fromGoType(t.Elem())
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
	gopkg, err := c.ImportGo(path)
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
	return nil, fmt.Errorf("ng package TODO")
}

func (c *Checker) checkImport(s *stmt.Import) {
	pkg, err := c.goPkg(s.Path)
	if err != nil {
		pkg, err = c.ngPkg(s.Path)
		if err != nil {
			c.errorf("importing of go/ng package failed: %v", err)
			return
		}
	}
	if s.Name == "" {
		s.Name = pkg.GoPkg.(*gotypes.Package).Name()
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
	p = c.exprPartial(e)
	if p.mode == modeTypeExpr {
		p.mode = modeInvalid
		c.errorf("type %s is not an expression", p.typ)
	}
	return p
}

func (c *Checker) exprType(e expr.Expr) tipe.Type {
	p := c.exprPartial(e)
	if p.mode == modeTypeExpr {
		return p.typ
	}
	c.errorf("argument %s is not a type", e)
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
	case *tipe.Slice:
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
			name := t.Package + "." + t.Name
			obj := c.cur.LookupRec(t.Package)
			if obj == nil {
				c.errorf("undefined %s in %s", t.Package, name)
				return t, false
			}
			if obj.Kind != ObjPkg {
				c.errorf("%s is not a packacge", t.Package)
				return t, false
			}
			pkg := obj.Type.(*tipe.Package)
			res := pkg.Exports[t.Name]
			if res == nil {
				c.errorf("%s not in package %s", name, t.Package)
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
			c.errorf("first argument to append must be a slice, got %s", arg0.typ)
			return p
		}
		p.typ = arg0.typ
		// TODO: append(x, y...)
		for _, arg := range e.Args[1:] {
			argp := c.expr(arg)
			c.convert(&argp, slice.Elem)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use %s as type %s in argument to append", arg, slice.Elem)
				return p
			}
		}
		return p
	case tipe.Close:
		p.typ = nil
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
			c.errorf("copy source must be slice or string, have %s", src.typ)
			return p
		}
		if t, isSlice := tipe.Underlying(dst.typ).(*tipe.Slice); isSlice {
			dstElem = t.Elem
		} else {
			p.mode = modeInvalid
			c.errorf("copy destination must be a slice, have %s", dst.typ)
			return p
		}
		if !c.convertible(dstElem, srcElem) {
			p.mode = modeInvalid
			c.errorf("copy source type %s is not convertible to destination %s", dstElem, srcElem)
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
			c.errorf("first argument to delete must be a map, got %s (type %s)", e.Args[0], arg0.typ)
			return p
		}
		if !c.convertible(keyType, arg1.typ) {
			p.mode = modeInvalid
			c.errorf("second argument to delete must match the key type %s, got type %s", keyType, arg1.typ)
			return p
		}
		return p
	case tipe.Len, tipe.Cap:
		p.typ = tipe.Int
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("%s takes exactly 1 argument, got %d", p.typ, len(e.Args))
			return p
		}
		arg0 := c.expr(e.Args[0])
		switch t := tipe.Underlying(arg0.typ).(type) {
		case *tipe.Slice, *tipe.Map: // TODO Chan, Array
			return p
		case tipe.Basic:
			if t == tipe.String {
				return p
			}
		}
		p.mode = modeInvalid
		c.errorf("invalid argument %s (%s) for %s ", e.Args[0], arg0.typ, p.typ)
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
			case *tipe.Slice, *tipe.Map: // TODO Chan:
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
	case tipe.Recover:
	default:
		panic(fmt.Sprintf("unknown builtin: %s", p.typ))
	}
	panic("TODO builtin")
}

func (c *Checker) exprPartialCall(e *expr.Call) partial {
	p := c.exprPartial(e.Func)
	switch p.mode {
	default:
		panic(fmt.Sprintf("unreachable, unknown call mode: %v", p.mode))
	case modeInvalid:
		return p
	case modeTypeExpr:
		// type conversion
		if len(e.Args) == 0 {
			p.mode = modeInvalid
			c.errorf("type conversion to %s is missing an argument", p.typ)
			return p
		} else if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorf("type conversion to %s has too many arguments", p.typ)
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
			c.errorf("too few arguments (%d) to variadic function %s", len(e.Args), funct)
			return p
		}
		for i := 0; i < len(params)-1; i++ {
			t := params[i]
			argp := c.expr(e.Args[i])
			c.convert(&argp, t)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use type %s as type %s in argument %d to function", argp.typ, t, i)
				return p
			}
		}
		vart := params[len(params)-1].(*tipe.Slice).Elem
		varargs := e.Args[len(params)-1:]
		for _, arg := range varargs {
			argp := c.expr(arg)
			c.convert(&argp, vart)
			if argp.mode == modeInvalid {
				p.mode = modeInvalid
				c.errorf("cannot use type %s as type %s in variadic argument to function", argp.typ, vart)
				return p
			}
		}
		return p
	}

	if len(e.Args) != len(params) {
		p.mode = modeInvalid
		c.errorf("wrong number of arguments (%d) to function %s", len(e.Args), funct)
		return p
	}
	for i, arg := range e.Args {
		t := params[i]
		argp := c.expr(arg)
		fmt.Printf("argp i=%d: %#+v (arg=%#+v)\n", i, argp, arg)
		c.convert(&argp, t)
		if argp.mode == modeInvalid {
			p.mode = modeInvalid
			c.errorf("cannot use type %s as type %s in argument to function", argp.typ, t)
			break
		}
	}

	return p
}

func (c *Checker) exprPartial(e expr.Expr) (p partial) {
	//fmt.Printf("exprPartial(%s)\n", e.Sexp())
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
			p.mode = modeVar
			p.typ = tipe.String
		case rune:
			p.mode = modeVar
			p.typ = tipe.Rune
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
			c.errorf("cannot construct type %s with a composite literal", e.Type)
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
			c.errorf("cannot construct type %s with a map composite literal", e.Type)
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
				c.errorf("type %s is not a slice", t)
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
				c.errorf("type %s is not a table", t)
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
			sub := c.expr(e.Expr)
			p.mode = modeVar
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
		}
	case *expr.Binary:
		left := c.expr(e.Left)
		right := c.expr(e.Right)
		c.constrainUntyped(&left, right.typ)
		c.constrainUntyped(&right, left.typ)
		if left.mode == modeInvalid {
			return left
		}
		if right.mode == modeInvalid {
			return right
		}
		left.expr = e
		// TODO check for division by zero
		if left.mode == modeConst && right.mode == modeConst {
			left.val = constant.BinaryOp(left.val, convGoOp(e.Op), right.val)
			// TODO check rounding
			// TODO check for comparison, result is untyped bool
		}
		switch e.Op {
		case token.Equal, token.NotEqual, token.Less, token.Greater:
			left.typ = tipe.Bool
		}
		return left
	case *expr.Call:
		return c.exprPartialCall(e)
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

		switch lt := tipe.Underlying(left.typ).(type) {
		case *tipe.Struct:
			for i, name := range lt.FieldNames {
				if name == right {
					p.mode = modeVar
					p.typ = lt.Fields[i]
					return
				}
			}
			p.mode = modeInvalid
			c.errorf("%s undefined (type %s has no field or method %s)", e, lt, right)
			return p
		case *tipe.Package:
			for name, t := range lt.Exports {
				if name == e.Right.Name {
					p.mode = modeVar // TODO modeFunc?
					p.typ = t
					return p
				}
			}
			p.mode = modeInvalid
			c.errorf("%s not in package %s", e, lt)
			return p
		}
		p.mode = modeInvalid
		c.errorf("%s undefined (type %s is not a struct or package)", e, left.typ)
		return p
	case *expr.Index:
		left := c.expr(e.Left)
		if left.mode == modeInvalid {
			return left
		}
		switch lt := tipe.Underlying(left.typ).(type) {
		case *tipe.Map:
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot table slice %s (type %s)", e.Left, left.typ)
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
		case *tipe.Slice:
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorf("cannot table slice %s (type %s)", e.Left, left.typ)
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

		panic(fmt.Sprintf("typecheck.expr TODO Index: %+v", e))
	case *expr.Shell:
		p.mode = modeVar
		p.typ = &tipe.Tuple{Elems: []tipe.Type{
			tipe.String, Universe.Objs["error"].Type,
		}}
		return p
	}
	panic(fmt.Sprintf("expr TODO: %T", e))
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
		c.errorf("cannot assign %s to %s", p.typ, t)
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
			c.errorf("constant %s does not fit in %s", p.val, t)
			p.mode = modeInvalid
			return
		}
	}

	if !c.convertible(t, p.typ) {
		c.errorf("cannot convert %s to %s", p.typ, t)
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
	if idst, ok := tipe.Underlying(dst).(*tipe.Interface); ok {
		// Everything can be assigned to interface{}.
		if len(idst.Methods) == 0 {
			return true
		}
		if src == tipe.UntypedNil {
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
	return false
}

func (c *Checker) convertible(dst, src tipe.Type) bool {
	if c.assignable(dst, src) {
		return true
	}
	// numerics can be converted to one another
	if tipe.IsNumeric(dst) && tipe.IsNumeric(src) {
		return true
	}
	if dst, isSlice := dst.(*tipe.Slice); isSlice {
		if (dst.Elem == tipe.Uint8 || dst.Elem == tipe.Byte) && src == tipe.String {
			return true
		}
	}
	if src, isSlice := src.(*tipe.Slice); isSlice {
		if (src.Elem == tipe.Uint8 || src.Elem == tipe.Byte) && dst == tipe.String {
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
			c.errorf("cannot convert %s to untyped %s", p.typ, t)
		}
	} else {
		switch t := t.(type) {
		case tipe.Basic:
			switch p.mode {
			case modeConst:
				p.val = round(p.val, t)
				if p.val == nil {
					c.errorf("cannot convert const %s to %s", p.typ, t)
					// TODO more details about why
				}
			case modeVar:
				panic("TODO coerce var to basic")
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
		} else {
			return nil
		}
	case constant.Int:
		switch t {
		case tipe.Integer, tipe.UntypedInteger:
			return v
		case tipe.Float, tipe.UntypedFloat, tipe.UntypedComplex:
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
		case tipe.Byte, tipe.Int8: // wrong, byte is an alias of int8
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
		case tipe.Num:
			return v
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

func (c *Checker) String() string {
	buf := new(bytes.Buffer)
	buf.WriteString("typecheck.Checker{\n")
	buf.WriteString("\tTypes: map[expr.Expr]tipe.Type{\n")
	for k, v := range c.Types {
		fmt.Fprintf(buf, "\t\t(%p)%s: %s\n", k, k.Sexp(), v.Sexp())
	}
	buf.WriteString("\t},\n")
	buf.WriteString("\tDefs: map[*expr.Ident]*Obj{\n")
	for k, v := range c.Defs {
		t := "niltype"
		if v.Type != nil {
			t = v.Type.Sexp()
		}
		fmt.Fprintf(buf, "\t\t(%p)%s: (%p)*Obj{Kind: %v, Type:%s}\n", k, k.Sexp(), v, v.Kind, t)
	}
	buf.WriteString("\t},\n")
	buf.WriteString("\tValues : map[expr.Expr]constant.Value{\n")
	for k, v := range c.Values {
		fmt.Fprintf(buf, "\t\t(%p)%s: %s\n", k, k.Sexp(), v)
	}
	buf.WriteString("\t},\n")
	buf.WriteString("}")
	return buf.String()
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
	if s.foundInParent != nil && o.Kind == ObjVar {
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
	ObjPkg
	ObjType
)

func (o ObjKind) String() string {
	switch o {
	case ObjUnknown:
		return "ObjUnknown"
	case ObjVar:
		return "ObjVar"
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
	Decl interface{} // *expr.FuncLiteral, *stmt.ClassDecl
	Used bool
}

func isTyped(t tipe.Type) bool {
	return t != tipe.Invalid && !isUntyped(t)
}

func isUntyped(t tipe.Type) bool {
	switch t {
	case tipe.UntypedNil, tipe.UntypedBool,
		tipe.UntypedInteger, tipe.UntypedFloat, tipe.UntypedComplex:
		return true
	}
	return false
}

func defaultType(t tipe.Type) tipe.Type {
	b, ok := t.(tipe.Basic)
	if !ok {
		return t
	}
	switch b {
	case tipe.UntypedBool:
		return tipe.Bool
	case tipe.UntypedInteger:
		return tipe.Int // tipe.Num
	case tipe.UntypedFloat:
		return tipe.Float64 // tipe.Num
	}
	return t
}
