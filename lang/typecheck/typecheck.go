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
	Types   map[expr.Expr]tipe.Type
	Defs    map[*expr.Ident]*Obj
	Values  map[expr.Expr]constant.Value
	GoPkgs  map[*tipe.Package]*gotypes.Package
	GoTypes map[gotypes.Type]tipe.Type
	// TODO: GoEquiv is tricky and deserving of docs. Particular type instance is associated with a Go type. That means EqualType(t1, t2)==true but t1 could have GoEquiv and t2 not.
	GoEquiv map[tipe.Type]gotypes.Type
	NumSpec map[expr.Expr]tipe.Basic // *tipe.Call, *tipe.CompLiteral -> numeric basic type
	Errs    []error

	cur *Scope

	Memory *tipe.Memory
}

func New() *Checker {
	return &Checker{
		ImportGo: goimporter.Default().Import,
		Types:    make(map[expr.Expr]tipe.Type),
		Defs:     make(map[*expr.Ident]*Obj),
		Values:   make(map[expr.Expr]constant.Value),
		GoPkgs:   make(map[*tipe.Package]*gotypes.Package),
		GoTypes:  make(map[gotypes.Type]tipe.Type),
		GoEquiv:  make(map[tipe.Type]gotypes.Type),
		cur: &Scope{
			Parent: Universe,
			Objs:   make(map[string]*Obj),
		},
		Memory: tipe.NewMemory(),
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

func isError(t tipe.Type) bool {
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
		if len(s.Left) == len(partials)-1 && isError(partials[len(partials)-1].typ) {
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

	case *stmt.TypeDecl:
		if t, ok := s.Type.(*tipe.Struct); ok {
			var usesNum bool
			var resolved bool
			for i, f := range t.Fields {
				t.Fields[i], resolved = c.resolve(f)
				usesNum = usesNum || tipe.UsesNum(t.Fields[i])
				if !resolved {
					return nil
				}
			}
			if usesNum {
				t.Spec.Num = tipe.Num
			}
		}

		obj := &Obj{
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		}
		c.cur.Objs[s.Name] = obj
		return nil

	case *stmt.MethodikDecl:
		var usesNum bool
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
	defer func() {
		if res == nil {
			fmt.Printf("typecheck: unknown go type: %v\n", t)
		} else {
			c.GoTypes[t] = res
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
		case gotypes.Byte:
			return tipe.Byte
		case gotypes.Rune:
			return tipe.Rune
		case gotypes.String:
			return tipe.String
		case gotypes.Int64:
			return tipe.Int64
		case gotypes.Float32:
			return tipe.Float32
		case gotypes.Float64:
			return tipe.Float64
		case gotypes.Int:
			return tipe.GoInt
		}
	case *gotypes.Named:
		if t.Obj().Id() == goErrorID {
			return Universe.Objs["error"].Type
		}
		base := c.fromGoType(t.Underlying())
		if t.NumMethods() == 0 {
			return base
		}
		mdik := &tipe.Methodik{
			Type: base,
		}
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			mdik.MethodNames = append(mdik.MethodNames, m.Name())
			mdik.Methods = append(mdik.Methods, c.fromGoType(m.Type()).(*tipe.Func))
		}
		return mdik
	case *gotypes.Slice:
		return &tipe.Table{Type: c.fromGoType(t.Elem())}
	case *gotypes.Struct:
		res := new(tipe.Struct)
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			ft := c.fromGoType(f.Type())
			if ft == nil {
				continue
			}
			res.FieldNames = append(res.FieldNames, f.Name())
			res.Fields = append(res.Fields, ft)
		}
		return res
	case *gotypes.Pointer:
		elem := c.fromGoType(t.Elem())
		if elem == nil {
			return nil
		}
		return &tipe.Pointer{Elem: elem}
	case *gotypes.Interface:
		res := &tipe.Interface{Methods: make(map[string]*tipe.Func)}
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			res.Methods[m.Name()] = c.fromGoType(m.Type()).(*tipe.Func)
		}
		return res
	case *gotypes.Signature:
		p := t.Params()
		r := t.Results()
		res := &tipe.Func{
			Params:   &tipe.Tuple{Elems: make([]tipe.Type, p.Len())},
			Results:  &tipe.Tuple{Elems: make([]tipe.Type, r.Len())},
			Variadic: t.Variadic(),
		}
		for i := 0; i < p.Len(); i++ {
			res.Params.Elems[i] = c.fromGoType(p.At(i).Type())
		}
		for i := 0; i < r.Len(); i++ {
			res.Results.Elems[i] = c.fromGoType(r.At(i).Type())
		}
		return res
	}
	return nil
}

func (c *Checker) goPackage(gopkg *gotypes.Package) *tipe.Package {
	names := gopkg.Scope().Names()

	pkg := &tipe.Package{
		IsGo:    true,
		Path:    gopkg.Path(),
		Exports: make(map[string]tipe.Type),
	}
	for _, name := range names {
		obj := gopkg.Scope().Lookup(name)
		if !obj.Exported() {
			continue
		}
		pkg.Exports[name] = c.fromGoType(obj.Type())
	}
	return pkg
}

func (c *Checker) checkImport(s *stmt.Import) {
	if s.FromGo {
		gopkg, err := c.ImportGo(s.Path)
		if err != nil {
			c.errorf("importing go package: %v", err)
			return
		}
		if s.Name == "" {
			s.Name = gopkg.Name()
		}
		pkg := c.goPackage(gopkg)
		c.GoPkgs[pkg] = gopkg
		obj := &Obj{
			Kind: ObjPkg,
			Type: pkg,
			// TODO Decl?
		}
		c.cur.Objs[s.Name] = obj
	} else {
		c.errorf("TODO import of non-Go package")
	}
}

func (c *Checker) expr(e expr.Expr) (p partial) {
	// TODO more mode adjustment
	p = c.exprPartial(e)
	if p.mode == modeConst {
		c.Values[p.expr] = p.val
	}
	if p.mode != modeInvalid {
		c.Types[p.expr] = p.typ
	}
	return p
}

func (c *Checker) resolve(t tipe.Type) (ret tipe.Type, resolved bool) {
	switch t := t.(type) {
	case *tipe.Table:
		t.Type, resolved = c.resolve(t.Type)
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

func (c *Checker) exprPartialCall(e *expr.Call) partial {
	p := c.expr(e.Func)
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
	p.mode = modeVar
	p.expr = e

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
		vart := params[len(params)-1].(*tipe.Table).Type
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
		}
		return p
	case *expr.FuncLiteral:
		c.pushScope()
		defer c.popScope()
		c.cur.foundInParent = make(map[string]bool)
		if e.Type.Params != nil {
			for i, t := range e.Type.Params.Elems {
				e.Type.Params.Elems[i], _ = c.resolve(t)
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

	case *expr.Unary:
		switch e.Op {
		case token.LeftParen, token.Not, token.Sub:
			return c.expr(e.Expr)
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
		// TODO check for comparison
		if left.mode == modeConst && right.mode == modeConst {
			left.val = constant.BinaryOp(left.val, convGoOp(e.Op), right.val)
			// TODO check rounding
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

		methodNames, methods := c.Memory.Methods(left.typ)
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
		left := c.expr(e.Expr)
		if left.mode == modeInvalid {
			return left
		}
		ind := c.expr(e.Index)
		if ind.mode == modeInvalid {
			return ind
		}
		switch lt := tipe.Underlying(left.typ).(type) {
		case *tipe.Map:
			c.assign(&ind, lt.Key)

			p.mode = modeVar // TODO not really? because not addressable
			p.typ = lt.Value
			return p
		}

		panic(fmt.Sprintf("typecheck.expr TODO Index: %+v", e))
	case *expr.TableIndex:
		panic(fmt.Sprintf("typecheck.expr TODO TableIndex: %+v", e))
	case *expr.Shell:
		p.mode = modeVoid
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
		fmt.Printf("convert round p.typ=%s, p.val=%s, t=%s\n", p.typ, p.val, t)
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
	if idst, ok := dst.(*tipe.Interface); ok {
		// Everything can be assigned to interface{}.
		if len(idst.Methods) == 0 {
			return true
		}
		if src == tipe.UntypedNil {
			return true
		}
		srcNames, srcTypes := c.Memory.Methods(src)
		srcm := make(map[string]tipe.Type)
		for i, name := range srcNames {
			srcm[name] = srcTypes[i]
		}
		dstNames, dstTypes := c.Memory.Methods(dst)
		//panic(fmt.Sprintf("dst: %s, dstNames: %s, dstTypes: %s\n", dst, dstNames, dstTypes))
		for i, name := range dstNames {
			if !tipe.Equal(dstTypes[i], srcm[name]) {
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
			c.errorf("cannot convert %s to %s", p.typ, t)
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
		case tipe.Int64:
			if _, ok := constant.Int64Val(v); ok {
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

	foundInParent map[string]bool
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
	if o != nil && s.foundInParent != nil && o.Kind == ObjVar {
		s.foundInParent[name] = true
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
		return tipe.Num
	case tipe.UntypedFloat:
		return tipe.Num
	}
	return t
}
