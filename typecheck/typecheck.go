// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package typecheck is a Neugram type checker.
package typecheck

import (
	"fmt"
	"go/constant"
	goimporter "go/importer"
	gotoken "go/token"
	gotypes "go/types"
	"io/ioutil"
	"math/big"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"neugram.io/ng/format"
	"neugram.io/ng/internal/bigcplx"
	"neugram.io/ng/parser"
	"neugram.io/ng/syntax"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/shell"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
)

type Checker struct {
	ImportGo func(path string) (*gotypes.Package, error)

	mu            *sync.Mutex
	types         map[expr.Expr]tipe.Type      // computed type for each expression
	consts        map[expr.Expr]constant.Value // component constant for const expressions
	idents        map[*expr.Ident]*Obj         // map of idents to the Obj they represent
	pkgs          map[string]*Package          // (ng abs file path or go import path) -> pkg
	goTypes       map[gotypes.Type]tipe.Type   // cache for the fromGoType method
	goTypesToFill map[gotypes.Type]tipe.Type
	errs          []error
	importWalk    []string // in-process pkgs, used to detect cycles
	memory        *tipe.Memory

	cur    *Scope
	curPkg *Package
}

func New(initPkg string) *Checker {
	if initPkg == "" {
		initPkg = "main"
	}
	return &Checker{
		mu:            new(sync.Mutex),
		types:         make(map[expr.Expr]tipe.Type),
		ImportGo:      goimporter.Default().Import,
		consts:        make(map[expr.Expr]constant.Value),
		idents:        make(map[*expr.Ident]*Obj),
		pkgs:          make(map[string]*Package),
		goTypes:       make(map[gotypes.Type]tipe.Type),
		goTypesToFill: make(map[gotypes.Type]tipe.Type),
		cur: &Scope{
			Parent: Universe,
			Objs:   make(map[string]*Obj),
		},
		curPkg: &Package{
			Path: initPkg,
			Type: &tipe.Package{
				Path:    initPkg,
				Exports: make(map[string]tipe.Type),
			},
			GlobalNames: make(map[string]*Obj),
		},
		importWalk: []string{initPkg},
		memory:     tipe.NewMemory(),
	}
}

// Check typechecks a Neugram package.
func (c *Checker) Check(path string) (*Package, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = c.errs[:0]

	pkg, err := c.ngPkg(path)
	if err != nil {
		return pkg, err
	}
	if len(c.errs) > 0 {
		return pkg, c.errs[0]
	}
	return pkg, nil
}

// Errs returns any errors encountered during type checking.
func (c *Checker) Errs() []error {
	if len(c.errs) == 0 {
		return nil
	}
	res := append([]error{}, c.errs...)
	c.errs = c.errs[:0]
	return res
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
	modeUnpacked
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
	case *stmt.ConstSet:
		for _, v := range s.Consts {
			c.checkConst(v)
		}
		return nil
	case *stmt.Const:
		return c.checkConst(s)
	case *stmt.VarSet:
		for _, v := range s.Vars {
			c.checkVar(v)
		}
		return nil
	case *stmt.Var:
		return c.checkVar(s)
	case *stmt.Assign:
		var partials []partial
		for _, rhs := range s.Right {
			p := c.exprNoElide(rhs)
			if p.mode == modeInvalid {
				return nil
			}
			if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
				if len(s.Right) > 1 {
					c.errorfmt("multiple value %s in single-value context", rhs)
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
		if len(s.Right) == 1 && len(s.Left) == 2 && len(s.Left) == len(partials)+1 {
			partials = c.checkCommaOK(s.Right[0], partials)
		}

		if len(s.Right) == 1 && len(s.Left) == len(partials)-1 && IsError(partials[len(partials)-1].typ) {
			markElideError(s.Right[0])
			partials = partials[:len(partials)-1]
		}

		if len(s.Left) != len(partials) {
			c.errorfmt("arity mismatch, left %d != right %d", len(s.Left), len(partials))
			return nil
		}

		if s.Decl {
			// make sure at least one of the lhs hasn't been previously declared.
			ndecls := 0
			for _, lhs := range s.Left {
				name := lhs.(*expr.Ident).Name
				if name == "_" {
					continue
				}
				if obj := c.cur.Objs[name]; obj == nil || obj.Kind != ObjVar {
					ndecls++
				}
			}
			if ndecls == 0 {
				c.errorfmt("no new variables on left side of :=")
				return nil
			}

			for i, lhs := range s.Left {
				p := partials[i]
				if isUntyped(p.typ) {
					c.constrainUntyped(&p, defaultType(p.typ))
				}
				name := lhs.(*expr.Ident).Name
				if name == "_" {
					if len(s.Left) == 1 {
						c.errorfmt("no new variables in declaration")
						return nil
					}
					continue
				}
				obj := &Obj{
					Name: name,
					Kind: ObjVar,
					Type: p.typ,
					Decl: s,
				}
				c.addObj(obj)
				c.idents[lhs.(*expr.Ident)] = obj
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
		// TODO: explain why thi isn't just c.exprPartial(s.Expr, hintElideErr)
		isError := IsError(p.typ)
		if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
			if IsError(tuple.Elems[len(tuple.Elems)-1]) {
				isError = true
			}
		}
		if isError {
			markElideError(s.Expr)
		}
		if p.mode == modeFunc {
			fn := p.expr.(*expr.FuncLiteral)
			if fn.Name != "" {
				c.addObj(&Obj{
					Name: fn.Name,
					Kind: ObjVar,
					Type: p.typ,
					Decl: s,
				})
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
		case *tipe.Chan:
			kt = t.Elem
		default:
			c.errorfmt("TODO range over non-slice: %T", t)
		}
		if s.Decl {
			if _, exists := c.types[s.Key]; s.Key != nil && !exists {
				obj := &Obj{
					Name: s.Key.(*expr.Ident).Name,
					Kind: ObjVar, Type: kt,
				}
				c.addObj(obj)
				c.idents[s.Key.(*expr.Ident)] = obj
				c.types[s.Key] = kt
			}
			if _, exists := c.types[s.Val]; s.Val != nil && !exists {
				obj := &Obj{
					Name: s.Val.(*expr.Ident).Name,
					Kind: ObjVar, Type: vt,
				}
				c.addObj(obj)
				c.idents[s.Val.(*expr.Ident)] = obj
				c.types[s.Val] = vt
			}
		} else {
			if _, exists := c.types[s.Key]; s.Key != nil && !exists {
				p := c.expr(s.Key)
				c.assign(&p, kt)
				c.types[s.Key] = kt
			}
			if _, exists := c.types[s.Val]; s.Val != nil && !exists {
				p := c.expr(s.Val)
				c.assign(&p, vt)
				c.types[s.Val] = vt
			}
		}
		c.stmt(s.Body, retType)
		return nil

	case *stmt.TypeDecl:
		t, _ := c.resolve(s.Type)
		s.Type = t.(*tipe.Named)

		c.addObj(&Obj{
			Name: s.Name,
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		})
		return nil

	case *stmt.TypeDeclSet:
		for _, s := range s.TypeDecls {
			c.stmt(s, retType)
		}
		return nil

	case *stmt.MethodikDecl:
		var usesNum bool
		t, _ := c.resolve(s.Type)
		s.Type = t.(*tipe.Named)
		for _, f := range s.Type.Methods {
			usesNum = usesNum || tipe.UsesNum(f)
		}

		for _, m := range s.Methods {
			c.pushScope()
			st := tipe.Type(s.Type)
			if m.PointerReceiver {
				st = &tipe.Pointer{Elem: st}
			}
			if m.ReceiverName != "" {
				c.addObj(&Obj{
					Name: m.ReceiverName,
					Kind: ObjVar,
					Type: st,
				})
			}
			c.expr(m)
			// TODO: uses num inside a method
			c.popScope()

			for _, name := range m.Type.FreeVars {
				var obj *Obj
				for scope := c.cur; scope != Universe; scope = scope.Parent {
					obj = scope.Objs[name]
					if obj != nil {
						break
					}
				}
				//obj := c.cur.LookupRec(name)
				if c.curPkg.GlobalNames[name] != obj {
					c.errorfmt("variable %s is not defined in the global scope", name)
				}
			}
		}

		if usesNum {
			s.Type.Spec.Num = tipe.Num
		}

		c.addObj(&Obj{
			Name: s.Name,
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		})
		return nil

	case *stmt.Return:
		if retType == nil {
			if len(s.Exprs) != 0 {
				c.errorfmt("too many arguments to return")
			}
			return nil
		}
		switch {
		case len(s.Exprs) > len(retType.Elems):
			c.errorfmt("too many arguments to return")
			return nil
		case len(s.Exprs) < len(retType.Elems):
			c.errorfmt("not enough arguments to return")
			return nil
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
				c.errorfmt("multi-value %v in single-value context", partials[0])
				return nil
			}
			got = tup.Elems
		} else {
			for _, p := range partials {
				if _, ok := p.typ.(*tipe.Tuple); ok {
					c.errorfmt("multi-value %v in single-value context", partials[0])
					return nil
				}
				got = append(got, p.typ)
			}
		}
		if len(got) > len(want) {
			c.errorfmt("too many arguments to return")
			return nil
		}
		if len(got) < len(want) {
			c.errorfmt("too few arguments to return")
			return nil
		}

		for i := range want {
			if !c.assignable(want[i], got[i]) {
				c.errorfmt("cannot use %s as %s in return argument", got[i], want[i])
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
			c.errorfmt("cannot send to non-channel type: %s", cht)
			return nil
		}
		p = c.expr(s.Value)
		if p.mode == modeInvalid {
			return nil
		}
		c.convert(&p, cht.Elem)
		if p.mode == modeInvalid {
			c.errorfmt("cannot send %s to %s", p.typ, cht)
		}
		return nil

	case *stmt.Branch:
		// TODO: make sure the branch is valid
		return nil

	case *stmt.Labeled:
		c.stmt(s.Stmt, retType)
		return nil

	case *stmt.Switch:
		if s.Init != nil {
			c.pushScope()
			defer c.popScope()
			c.stmt(s.Init, retType)
		}
		var typ tipe.Type = tipe.Bool
		if s.Cond != nil {
			p := c.expr(s.Cond)
			if p.mode == modeInvalid {
				return nil
			}
			typ = p.typ
		}
		typ, ok := c.resolve(typ)
		if !ok {
		}
		ndefaults := 0
		set := make(map[expr.Expr]struct{})
		for _, cse := range s.Cases {
			if cse.Default {
				ndefaults++
				if ndefaults > 1 {
					c.errorfmt("multiple defaults in switch")
				}
			}
			for _, cond := range cse.Conds {
				for k := range set {
					if parser.EqualExpr(cond, k) {
						c.errorfmt("duplicate case %s in switch", cond)
					}
				}
				set[cond] = struct{}{}
				p := c.expr(cond)
				if p.mode == modeInvalid {
					return nil
				}
				if !c.convertible(typ, p.typ) {
					c.errorfmt(
						"invalid case %s in switch (mismatched types %s and %s)",
						format.Expr(cond),
						format.Type(p.typ), format.Type(typ),
					)
					return nil
				}
				if isUntyped(p.typ) {
					c.constrainUntyped(&p, typ)
				}
			}
			c.stmt(cse.Body, retType)
		}
		return nil

	case *stmt.TypeSwitch:
		if s.Init != nil {
			c.pushScope()
			defer c.popScope()
			c.stmt(s.Init, retType)
		}
		c.pushScope()
		defer c.popScope()
		if s.Assign == nil {
			c.errorfmt("type switch needs a type switch guard")
			return nil
		}
		c.stmt(s.Assign, retType)
		var (
			e  *expr.TypeAssert
			id expr.Expr
		)
		switch st := s.Assign.(type) {
		case *stmt.Simple:
			e = st.Expr.(*expr.TypeAssert)
			id = e.Left
		case *stmt.Assign:
			e = st.Right[0].(*expr.TypeAssert)
			id = st.Left[0]
		}
		p := c.expr(e)
		if p.mode == modeInvalid {
			return nil
		}
		styp, ok := c.resolve(p.typ)
		if !ok {
			c.errorfmt("type switch could not resolve type switch guard type")
			return nil
		}
		p.typ = styp
		iface, ok := tipe.Underlying(styp).(*tipe.Interface)
		if !ok {
			c.errorfmt("cannot type switch on non-interface value %s (type %s)", id, styp)
			return nil
		}
		dflts := 0
		set := make(map[tipe.Type]struct{})
		for _, cse := range s.Cases {
			if cse.Default {
				dflts++
				if dflts > 1 {
					c.errorfmt("multiple defaults in switch")
				}
			}
			for i, typ := range cse.Types {
				typ, resolved := c.resolve(typ)
				if resolved {
					cse.Types[i] = typ
				}
				for k := range set {
					if tipe.Equal(typ, k) {
						c.errorfmt("duplicate case %s in type switch", typ)
					}
				}
				set[typ] = struct{}{}
				if !c.typeAssert(iface, typ) {
					// TODO: explain why it can't implement the interface.
					c.errorfmt(
						"impossible type switch case: %s (type %s) cannot have dynamic type %s",
						format.Expr(p.expr), format.Type(iface), format.Type(typ),
					)
				}
			}
			c.stmt(cse.Body, retType)
		}
		return nil

	case *stmt.Select:
		dflts := 0
		set := make(map[stmt.Stmt]struct{})
		for _, cse := range s.Cases {
			if cse.Default {
				dflts++
				if dflts > 1 {
					c.errorfmt("multiple defaults in select")
				}
			}
			for k := range set {
				if parser.EqualStmt(k, cse.Stmt) {
					c.errorfmt("duplicate case %s in select", cse.Stmt)
				}
			}
			set[cse.Stmt] = struct{}{}
			if cse.Stmt != nil {
				switch st := cse.Stmt.(type) {
				case *stmt.Assign:
					if len(st.Right) != 1 {
						c.errorfmt("select case must be receive send or assign recv")
						return nil
					}
					sr, ok := st.Right[0].(*expr.Unary)
					if !ok || sr.Op != token.ChanOp {
						c.errorfmt("select case must be receive send or assign recv")
						return nil
					}
				case *stmt.Send:
				case *stmt.Simple:
					sr, ok := st.Expr.(*expr.Unary)
					if !ok || sr.Op != token.ChanOp {
						c.errorfmt("select case must be receive send or assign recv")
						return nil
					}
				default:
					c.errorfmt("select case must be receive, send or assign recv")
					return nil
				}
			}

			func(cse *stmt.SelectCase) {
				if cse.Stmt != nil {
					c.pushScope()
					defer c.popScope()
					c.stmt(cse.Stmt, retType)
				}
				c.stmt(cse.Body, retType)
			}(&cse)
		}
		return nil
	default:
		panic("typecheck: unknown stmt: " + format.Debug(s))
	}
}

func (c *Checker) checkConst(s *stmt.Const) tipe.Type {
	if s.Type != nil {
		if t, ok := c.resolve(s.Type); ok {
			s.Type = t
		}
	}
	var partials []partial
	for _, rhs := range s.Values {
		p := c.exprNoElide(rhs)
		if p.mode == modeInvalid {
			return nil
		}
		if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
			if len(s.Values) > 1 {
				c.errorfmt("multiple value %s in single-value context", rhs)
				return nil
			}
			for _, t := range tuple.Elems {
				partials = append(partials, partial{
					mode: modeConst,
					typ:  t,
				})
			}
			continue
		}
		partials = append(partials, c.exprNoElide(rhs))
	}
	if len(s.Values) == 1 && len(s.NameList) == 2 && len(s.NameList) == len(partials)+1 {
		partials = c.checkCommaOK(s.Values[0], partials)
	}

	if len(s.Values) == 1 && len(s.NameList) == len(partials)-1 && IsError(partials[len(partials)-1].typ) {
		markElideError(s.Values[0])
		partials = partials[:len(partials)-1]
	}

	if len(s.NameList) != len(partials) {
		if s.Type == nil || len(partials) != 0 {
			c.errorfmt("arity mismatch, left %d != right %d", len(s.NameList), len(partials))
			return nil
		}
	}

	// make sure none of the lhs have been previously declared.
	for _, name := range s.NameList {
		if name == "_" {
			continue
		}
		if obj := c.cur.Objs[name]; obj != nil {
			c.errorfmt("%s redeclared in this block", name)
			return nil
		}
	}

	for i, name := range s.NameList {
		if name == "_" {
			continue
		}
		var typ tipe.Type
		if len(partials) > i {
			p := partials[i]
			if isUntyped(p.typ) {
				if s.Type != nil {
					c.constrainUntyped(&p, s.Type)
				}
			}
			typ = p.typ

			if s.Type != nil && !c.assignable(s.Type, p.typ) {
				switch len(s.NameList) {
				case 1:
					c.errorfmt("cannot use %v (type %v) as type %v in assignment", format.Expr(s.Values[i]), format.Type(p.typ), format.Type(s.Type))
				default:
					c.errorfmt("cannot assign %v to %s (type %v) in multiple assignment", format.Type(p.typ), name, format.Type(s.Type))
				}
				return nil
			}
		}
		if s.Type != nil {
			typ = s.Type
		}
		c.addObj(&Obj{
			Name: name,
			Kind: ObjConst,
			Type: typ,
			Decl: c.consts[s.Values[i]],
		})
	}
	return nil
}

func (c *Checker) checkVar(s *stmt.Var) tipe.Type {
	if s.Type != nil {
		if t, ok := c.resolve(s.Type); ok {
			s.Type = t
		}
	}
	var partials []partial
	for _, rhs := range s.Values {
		p := c.exprNoElide(rhs)
		if p.mode == modeInvalid {
			return nil
		}
		if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
			if len(s.Values) > 1 {
				c.errorfmt("multiple value %s in single-value context", rhs)
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
	if len(s.Values) == 1 && len(s.NameList) == 2 && len(s.NameList) == len(partials)+1 {
		partials = c.checkCommaOK(s.Values[0], partials)
	}

	if len(s.Values) == 1 && len(s.NameList) == len(partials)-1 && IsError(partials[len(partials)-1].typ) {
		markElideError(s.Values[0])
		partials = partials[:len(partials)-1]
	}

	if len(s.NameList) != len(partials) {
		if s.Type == nil || len(partials) != 0 {
			c.errorfmt("arity mismatch, left %d != right %d", len(s.NameList), len(partials))
			return nil
		}
	}

	// make sure none of the lhs have been previously declared.
	for _, name := range s.NameList {
		if name == "_" {
			continue
		}
		if obj := c.cur.Objs[name]; obj != nil {
			c.errorfmt("%s redeclared in this block", name)
			return nil
		}
	}

	for i, name := range s.NameList {
		if name == "_" {
			continue
		}
		var typ tipe.Type
		if len(partials) > i {
			p := partials[i]
			if isUntyped(p.typ) {
				if s.Type != nil {
					c.constrainUntyped(&p, s.Type)
				} else {
					c.constrainUntyped(&p, defaultType(p.typ))
				}
			}
			typ = p.typ

			if s.Type != nil && !c.assignable(s.Type, p.typ) {
				switch len(s.NameList) {
				case 1:
					c.errorfmt("cannot use %v (type %v) as type %v in assignment", format.Expr(s.Values[i]), format.Type(p.typ), format.Type(s.Type))
				default:
					c.errorfmt("cannot assign %v to %s (type %v) in multiple assignment", format.Type(p.typ), name, format.Type(s.Type))
				}
				return nil
			}
		}
		if s.Type != nil {
			typ = s.Type
		}
		c.addObj(&Obj{
			Name: name,
			Kind: ObjVar,
			Type: typ,
			Decl: s,
		})
	}
	return nil
}

func (c *Checker) checkCommaOK(e expr.Expr, partials []partial) []partial {
	switch e := e.(type) {
	case *expr.Index:
		// v, ok = m[key]
		switch typ := c.types[e.Left].(type) {
		case *tipe.Map:
			// the general case for a map-index is to only typecheck
			// for returning the map-value.
			// in the case of indexing into a map with a comma-ok,
			// we need to also return the boolean indicating whther
			// the key was found in the map.
			// so, replace the original tipe.Type with the tuple:
			//  (tipe.Type, tipe.Bool)
			// this way, the eval of that statement will do the right thing.
			// we still need to add the boolean to the partials, though.
			t := &tipe.Tuple{Elems: []tipe.Type{typ.Value, tipe.Bool}}
			partials = append(partials, partial{
				mode: modeVar,
				typ:  tipe.Bool,
			})
			if curTyp := c.types[e]; curTyp != t {
				c.types[e] = t
			}
		}
	case *expr.Unary:
		// v, ok = <-ch
		switch e.Op {
		case token.ChanOp:
			// the general case for a chan-receive is to only typecheck
			// for the chan element.
			// in the case of a chan-receive with a comma-ok, we need to
			// also get the boolean indicating whether the value received
			// corresponds to a send.
			// so, replace the original tipe.Type with the tuple:
			//   (tipe.Type,tipe.Bool)
			// this way, the eval of that statement will do the right thing.
			// we still need to add the boolean to the partials, though.
			typ := &tipe.Tuple{Elems: []tipe.Type{partials[0].typ, tipe.Bool}}
			if curTyp := c.types[e]; curTyp != typ {
				c.types[e] = typ
			}
			partials = append(partials, partial{
				mode: modeVar,
				typ:  tipe.Bool,
			})
		}
	case *expr.TypeAssert:
		// v, ok = rhs.(T)
		//
		// A type assertion can return its typical one value,
		// the value as the asserted type, or the value and a
		// comma-ok for whether the assertion was successful.
		// This is the latter case, so we replace the
		// original return type with a tuple.
		typ := &tipe.Tuple{Elems: []tipe.Type{partials[0].typ, tipe.Bool}}
		if curTyp := c.types[e]; curTyp != typ {
			c.types[e] = typ
		}
		partials = append(partials, partial{
			mode: modeVar,
			typ:  tipe.Bool,
		})
	}
	return partials
}

var goErrorID = gotypes.Universe.Lookup("error").Id()

func (c *Checker) fromGoType(t gotypes.Type) (res tipe.Type) {
	if res = c.goTypes[t]; res != nil {
		return res
	}
	defer func() {
		if res == nil {
			fmt.Printf("typecheck: unknown go type: %v\n", t)
		} else {
			c.goTypes[t] = res
			c.goTypesToFill[t] = res
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
		case gotypes.UnsafePointer:
			return tipe.UnsafePointer
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
		return new(tipe.Named)
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
		mdik := res.(*tipe.Named)
		*mdik = tipe.Named{
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
			s.Fields = append(s.Fields, tipe.StructField{
				Name: f.Name(),
				Type: ft,
				// TODO Embedded
			})
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

func (c *Checker) goPkg(path string) (*Package, error) {
	if pkg := c.pkgs[path]; pkg != nil {
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
	pkg := &Package{
		GoPkg: gopkg,
		Path:  gopkg.Path(),
		Type: &tipe.Package{
			GoPkg:   gopkg,
			Path:    gopkg.Path(),
			Exports: make(map[string]tipe.Type),
		},
		GlobalNames: make(map[string]*Obj),
	}
	c.pkgs[path] = pkg

	for _, name := range gopkg.Scope().Names() {
		goobj := gopkg.Scope().Lookup(name)
		obj := &Obj{
			Name: goobj.Name(), // TODO: use goobj.Id()?
			Type: c.fromGoType(goobj.Type()),
		}
		switch goobj.(type) {
		case *gotypes.Const:
			obj.Kind = ObjConst
		case *gotypes.Var:
			obj.Kind = ObjVar
		case *gotypes.TypeName:
			obj.Kind = ObjType
		}
		pkg.Globals = append(pkg.Globals, obj)
		pkg.GlobalNames[obj.Name] = obj
		if goobj.Exported() {
			pkg.Type.Exports[obj.Name] = obj.Type
		}
	}
	for len(c.goTypesToFill) > 0 {
		for gotyp, t := range c.goTypesToFill {
			c.fillGoType(t, gotyp)
			delete(c.goTypesToFill, gotyp)
		}
	}
	return pkg, nil
}

// Pkg returns the type-checked Neugram or Go package.
// If the type checker has not processed the package, nil is returned.
func (c *Checker) Pkg(path string) *Package {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pkgs[path]
}

func (c *Checker) ngPkg(path string) (*Package, error) {
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
	if pkg := c.pkgs[path]; pkg != nil {
		return pkg, nil
	}

	source, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ng package import: %v", err)
	}
	c.importWalk = append(c.importWalk, path)
	oldcur := c.cur
	oldcurPkg := c.curPkg
	defer func() {
		c.importWalk = c.importWalk[:len(c.importWalk)-1]
		c.cur = oldcur
		c.curPkg = oldcurPkg
	}()

	c.cur = &Scope{
		Parent: Universe,
		Objs:   make(map[string]*Obj),
	}
	c.curPkg = &Package{
		Path: path,
		Type: &tipe.Package{
			Path:    path,
			Exports: make(map[string]tipe.Type),
		},
		GlobalNames: make(map[string]*Obj),
	}
	if err := c.parseFile(path, source); err != nil {
		return nil, fmt.Errorf("ng import parse: %v", err)
	}
	c.pkgs[path] = c.curPkg
	return c.curPkg, nil
}

func isExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

func (c *Checker) parseFile(filename string, source []byte) error {
	p := parser.New(filename)
	f, err := p.Parse(source)
	if err != nil {
		return err
	}
	c.curPkg.Syntax = f

	for _, s := range f.Stmts {
		c.stmt(s, nil)
		if len(c.errs) > 0 {
			return c.errs[0]
		}
	}

	return nil
}

func (c *Checker) checkImport(s *stmt.Import) {
	if strings.HasPrefix(s.Path, "/") {
		c.errorfmt("imports do not support absolute paths: %q", s.Path)
		return
	}
	var pkg *Package
	var err error
	if strings.HasSuffix(s.Path, ".ng") {
		pkg, err = c.ngPkg(s.Path)
		if err != nil {
			c.errorfmt("importing of ng package failed: %v", err)
			return
		}
		if s.Name == "" {
			s.Name = strings.TrimSuffix(filepath.Base(s.Path), ".ng")
		}
	} else {
		pkg, err = c.goPkg(s.Path)
		if err != nil {
			c.errorfmt("importing of go package failed: %v", err)
			return
		}
		if s.Name == "" {
			s.Name = pkg.GoPkg.Name()
		}
	}
	c.addObj(&Obj{
		Name: s.Name,
		Kind: ObjPkg,
		Type: pkg.Type,
		Decl: pkg,
	})
}

func (c *Checker) expr(e expr.Expr) (p partial) {
	// TODO more mode adjustment
	p = c.exprPartial(e, hintElideErr)
	if p.mode == modeTypeExpr {
		p.mode = modeInvalid
		c.errorfmt("type %s is not an expression", p.typ)
	}
	return p
}

func (c *Checker) exprNoElide(e expr.Expr) (p partial) {
	p = c.exprPartial(e, hintNone)
	// TODO: dedup with expr()
	if p.mode == modeTypeExpr {
		p.mode = modeInvalid
		c.errorfmt("type %s is not an expression", p.typ)
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
	c.errorfmt("argument %s is not a type (%#+v)", e, p)
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
	case *tipe.Named:
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
	case *tipe.Ellipsis:
		t.Elem, resolved = c.resolve(t.Elem)
		return t, resolved
	case *tipe.Struct:
		usesNum := false
		resolved := true
		for i, sf := range t.Fields {
			ft, r1 := c.resolve(sf.Type)
			usesNum = usesNum || tipe.UsesNum(ft)
			t.Fields[i].Type = ft
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
			c.errorfmt("type %s not declared", t.Name)
			return t, false
		}
		if obj.Kind != ObjType {
			c.errorfmt("symbol %s is not a type", t.Name)
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
		c.errorfmt("undefined %s in %s", pkgName, name)
		return nil
	}
	if obj.Kind != ObjPkg {
		c.errorfmt("%s is not a packacge", pkgName)
		return nil
	}
	pkg := obj.Decl.(*Package)
	res := pkg.GlobalNames[sel]
	if res == nil || !isExported(sel) {
		c.errorfmt("%s not in package %s", name, pkgName)
		return nil
	}
	return res.Type
}

func (c *Checker) exprBuiltinCall(e *expr.Call) partial {
	p := c.expr(e.Func)
	p.expr = e

	switch p.typ.(tipe.Builtin) {
	case tipe.Append:
		if len(e.Args) == 0 {
			p.mode = modeInvalid
			c.errorfmt("too few arguments to append")
			return p
		}
		arg0 := c.expr(e.Args[0])
		slice, isSlice := tipe.Underlying(arg0.typ).(*tipe.Slice)
		if !isSlice {
			p.mode = modeInvalid
			c.errorfmt("first argument to append must be a slice, got %s", arg0.typ)
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
				c.errorfmt("cannot use %s (type %s) as type %s in argument to append", arg, argpTyp, slice.Elem)
				return p
			}
		}
		return p
	case tipe.Close:
		p.typ = nil
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("too few arguments to close")
			return p
		}
		arg := c.expr(e.Args[0])
		ch, isChan := tipe.Underlying(arg.typ).(*tipe.Chan)
		if !isChan {
			p.mode = modeInvalid
			c.errorfmt("argument to close must be a chan, got %s", arg.typ)
			return p
		}
		if ch.Direction == tipe.ChanRecv {
			p.mode = modeInvalid
			c.errorfmt("%s: cannot close receive-only channel", e)
			return p
		}
		return p
	case tipe.ComplexFunc:
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorfmt("complex takes two arguments, got %d", len(e.Args))
			return p
		}
		arg0 := c.expr(e.Args[0])
		arg1 := c.expr(e.Args[1])
		switch arg0.typ {
		case tipe.UntypedInteger:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat:
				// FIXME: return UntypedComplex
				p.typ = tipe.Complex128
			case tipe.Float:
				p.typ = tipe.Complex
			case tipe.Float32:
				p.typ = tipe.Complex64
			case tipe.Float64:
				p.typ = tipe.Complex128
			default:
				p.mode = modeInvalid
				c.errorfmt("second argument to complex must be a float, got %s", arg1.typ)
				return p
			}
		case tipe.UntypedFloat:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat:
				// FIXME: return UntypedComplex
				p.typ = tipe.Complex128
			case tipe.Float:
				p.typ = tipe.Complex
			case tipe.Float32:
				p.typ = tipe.Complex64
			case tipe.Float64:
				p.typ = tipe.Complex128
			default:
				p.mode = modeInvalid
				c.errorfmt("second argument to complex must be a float, got %s", arg1.typ)
				return p
			}
		case tipe.Float:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float:
				p.typ = tipe.Complex
			default:
				p.mode = modeInvalid
				c.errorfmt("second argument to complex must be a float, got %s", arg1.typ)
				return p
			}
		case tipe.Float32:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float32:
				p.typ = tipe.Complex64
			case tipe.Float:
				p.mode = modeInvalid
				c.errorfmt("invalid operation: complex(%s, %s) (mismatched types float32 and float)", e.Args[0], e.Args[1])
				return p
			case tipe.Float64:
				p.mode = modeInvalid
				c.errorfmt("invalid operation: complex(%s, %s) (mismatched types float32 and float64)", e.Args[0], e.Args[1])
				return p
			default:
				p.mode = modeInvalid
				c.errorfmt("second argument to complex must be a float32, got %s", arg1.typ)
				return p
			}
		case tipe.Float64:
			switch arg1.typ {
			case tipe.UntypedInteger, tipe.UntypedFloat, tipe.Float64:
				p.typ = tipe.Complex128
			case tipe.Float:
				p.mode = modeInvalid
				c.errorfmt("invalid operation: complex(%s, %s) (mismatched types float64 and float)", e.Args[0], e.Args[1])
				return p
			case tipe.Float32:
				p.mode = modeInvalid
				c.errorfmt("invalid operation: complex(%s, %s) (mismatched types float64 and float32)", e.Args[0], e.Args[1])
				return p
			default:
				p.mode = modeInvalid
				c.errorfmt("second argument to complex must be a float64, got %s", arg1.typ)
				return p
			}
		default:
			p.mode = modeInvalid
			c.errorfmt("first argument to complex must be a float, got %s", arg0.typ)
			return p
		}
		return p
	case tipe.Copy:
		p.typ = tipe.Int
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorfmt("copy takes two arguments, got %d", len(e.Args))
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
			c.errorfmt("copy source must be slice or string, got %s", src.typ)
			return p
		}
		if t, isSlice := tipe.Underlying(dst.typ).(*tipe.Slice); isSlice {
			dstElem = t.Elem
		} else {
			p.mode = modeInvalid
			c.errorfmt("copy destination must be a slice, have %s", dst.typ)
			return p
		}
		if !c.convertible(dstElem, srcElem) {
			p.mode = modeInvalid
			c.errorfmt("copy source type %s is not convertible to destination %s", dstElem, srcElem)
			return p
		}
		return p
	case tipe.Delete:
		p.typ = nil
		if len(e.Args) != 2 {
			p.mode = modeInvalid
			c.errorfmt("delete takes exactly two arguments, got %d", len(e.Args))
			return p
		}
		arg0, arg1 := c.expr(e.Args[0]), c.expr(e.Args[1])
		var keyType tipe.Type
		if t, isMap := tipe.Underlying(arg0.typ).(*tipe.Map); isMap {
			keyType = t.Key
		} else {
			p.mode = modeInvalid
			c.errorfmt("first argument to delete must be a map, got %s (type %s)", e.Args[0], arg0.typ)
			return p
		}
		if !c.convertible(keyType, arg1.typ) {
			p.mode = modeInvalid
			c.errorfmt("second argument to delete must match the key type %s, got type %s", keyType, arg1.typ)
			return p
		}
		return p
	case tipe.Imag:
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("imag takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		arg := c.expr(e.Args[0])
		switch arg.typ {
		case tipe.Complex:
			p.typ = tipe.Float
		case tipe.UntypedComplex:
			// FIXME: return UntypedFloat instead.
			p.typ = tipe.Float64
		case tipe.Complex64:
			p.typ = tipe.Float32
		case tipe.Complex128:
			p.typ = tipe.Float64
		default:
			p.mode = modeInvalid
			c.errorfmt("argument to imag must be a complex, got %s (type %s)", e.Args[0], arg.typ)
			return p
		}
		return p
	case tipe.Len, tipe.Cap:
		p.typ = tipe.Int
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("%s takes exactly 1 argument, got %d", p.typ, len(e.Args))
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
		c.errorfmt("invalid argument %s (%s) for %s ", e.Args[0], arg0.typ, p.typ)
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
			c.errorfmt("make requires 1-3 arguments")
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
			c.errorfmt("make argument must be a slice, map, or channel")
		}
		return p
	case tipe.New:
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("new takes exactly one argument, got %d", len(e.Args))
			return p
		}
		arg0 := c.exprType(e.Args[0])
		if arg0 == nil {
			p.mode = modeInvalid
			c.errorfmt("argument to new must be a type")
			return p
		}
		e.Args[0] = &expr.Type{Type: arg0}
		p.typ = &tipe.Pointer{Elem: arg0}
		return p
	case tipe.Panic:
		p.typ = nil
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("panic takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		if arg0 := c.expr(e.Args[0]); arg0.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		return p
	case tipe.Real:
		if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("real takes exactly 1 argument, got %d", len(e.Args))
			return p
		}
		arg := c.expr(e.Args[0])
		switch arg.typ {
		case tipe.Complex:
			p.typ = tipe.Float
		case tipe.UntypedComplex:
			// FIXME: return UntypedFloat instead.
			p.typ = tipe.Float64
		case tipe.Complex64:
			p.typ = tipe.Float32
		case tipe.Complex128:
			p.typ = tipe.Float64
		default:
			p.mode = modeInvalid
			c.errorfmt("argument to real must be a complex, got %s (type %s)", e.Args[0], arg.typ)
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
			c.errorfmt("type conversion to %s is missing an argument", p.typ)
			return p
		} else if len(e.Args) != 1 {
			p.mode = modeInvalid
			c.errorfmt("type conversion to %s has too many arguments", p.typ)
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
	funct := tipe.Underlying(p.typ).(*tipe.Func)
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

	// When we have exactly one argument, the Go spec allows this
	// to be treated as multiple arguments in a few cases, such as
	// when calling f(g()) and g returns multiple values. Handle this.
	// TODO: also handle the comma-ok cases.
	unpacked, ok := c.unpackExprs(hintNone, e.Args...)
	if !ok {
		p.mode = modeInvalid
		return p
	}

	// If we have f([a, b,] c...), check whether that is permissible.
	if e.Ellipsis {
		if !funct.Variadic {
			p.mode = modeInvalid
			c.errorfmt("cannot use ... with non-variadic function %s", funct)
			return p
		}
		if len(e.Args) == 1 && len(unpacked) > 1 {
			p.mode = modeInvalid
			c.errorfmt("cannot use ... with multi-valued function %s", funct)
			return p
		}
	}

	// Type-check each argument against the called function
	for i, pi := range unpacked {
		// Determine the type of the corresponding parameter in the
		// called function.
		var typ tipe.Type
		switch {
		case i < len(params):
			typ = params[i]
		case funct.Variadic:
			typ = params[len(params)-1]
		default:
			// Too many arguments. However, if we have an unpacked
			// last argument that is an error type, elide the error.
			if i == len(unpacked)-1 && pi.mode == modeUnpacked && IsError(pi.typ) {
				markElideError(pi.expr)
				continue
			}
			p.mode = modeInvalid
			c.errorfmt("too many arguments to function %s", funct)
			return p
		}

		// If this is the variadic parameter and we have "...",
		// check that the parameter is a slice.
		if e.Ellipsis && i == len(params)-1 {
			_, ok := pi.typ.(*tipe.Slice)
			if !ok && !tipe.IsUntypedNil(pi.typ) {
				p.mode = modeInvalid
				c.errorfmt("cannot use type %s as type %s in argument %d to function", pi.typ, typ, i)
				return p
			}
			// We're using "x..." and are at the position of the
			// variadic parameter. To allow for typechecking,
			// change the Ellipsis type to a Slice type.
			typ = &tipe.Slice{Elem: typ.(*tipe.Ellipsis).Elem}
		} else if !e.Ellipsis && funct.Variadic && i >= len(params)-1 {
			// We're not using "x...", but we are at (or beyond)
			// the position of the variadic parameter. That means that
			// typ is a slice, but since we're passing in individual
			// arguments we should typecheck against the underlying
			// element type rather than the ellipsis type.

			// TODO there seems to be some cases where we get a slice here
			// rather than an ellipsis. It's not clear why that is.
			switch t := typ.(type) {
			case *tipe.Ellipsis:
				typ = t.Elem
			case *tipe.Slice:
				typ = t.Elem
			}
		}

		// If we have an unpacked argument that is not an error,
		// that means that we have a multi-valued function that
		// returns something else than just (T, error).
		// We only allow this case when there is a single argument
		// f(g()) to stay close to the Go spec.
		if pi.mode == modeUnpacked && !IsError(pi.typ) && len(e.Args) > 1 {
			p.mode = modeInvalid
			c.errorfmt("multi-valued %s used in single-valued context", pi.expr)
			return p
		}

		// Typecheck the argument against the declared type of the
		// matching function parameter.
		c.convert(&pi, typ)
		if pi.mode == modeInvalid {
			p.mode = modeInvalid
			c.errorfmt("cannot use type %s as type %s in argument %d to function", pi.typ, typ, i)
			return p
		}
	}

	// Check if we have too few arguments
	numArgs := len(unpacked)
	if funct.Variadic {
		// a variadic function accepts an "empty"
		// last argument: count one extra
		numArgs++
	}
	if numArgs < len(params) {
		p.mode = modeInvalid
		c.errorfmt("too few arguments in call to %s", funct)
		return p
	}

	return p
}

func (c *Checker) exprPartial(e expr.Expr, hint typeHint) (p partial) {
	defer func() {
		if p.mode == modeConst {
			if _, exists := c.consts[p.expr]; !exists {
				c.consts[p.expr] = p.val
			}
		}
		if p.mode != modeInvalid {
			if _, exists := c.types[p.expr]; !exists {
				c.types[p.expr] = p.typ
			}
		}
	}()
	p.expr = e
	switch e := e.(type) {
	case *expr.Ident:
		if e.Name == "_" {
			p.mode = modeInvalid
			c.errorfmt("cannot use _ as a value")
			return p
		}
		obj := c.cur.LookupRec(e.Name)
		if obj == nil {
			p.mode = modeInvalid
			c.errorfmt("undeclared identifier: %s", e.Name)
			return p
		}
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
		c.idents[e] = obj
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
		case *bigcplx.Complex:
			p.mode = modeConst
			p.typ = tipe.UntypedComplex
			p.val = constant.MakeFromLiteral(v.String(), gotoken.IMAG, 0)
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
		c.cur.foundMdikInParent = make(map[*tipe.Named]bool)
		if e.Type.Params != nil {
			for i, t := range e.Type.Params.Elems {
				t, _ = c.resolve(t)
				e.Type.Params.Elems[i] = t
				if e.Type.Variadic && i == len(e.Type.Params.Elems)-1 {
					if elt, ellipsis := t.(*tipe.Ellipsis); ellipsis {
						t = &tipe.Slice{Elem: elt.Elem}
					}
				}
				if e.ParamNames[i] != "" {
					c.addObj(&Obj{
						Name: e.ParamNames[i],
						Kind: ObjVar,
						Type: t,
					})
				}
			}
		}
		if e.Type.Results != nil {
			for i, t := range e.Type.Results.Elems {
				e.Type.Results.Elems[i], _ = c.resolve(t)
			}
		}
		c.stmt(e.Body.(*stmt.Block), e.Type.Results)
		for _, pname := range e.ParamNames {
			delete(c.cur.foundInParent, pname)
		}
		if e.ReceiverName != "" {
			delete(c.cur.foundInParent, e.ReceiverName)
		}
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
		if t, resolved := c.resolve(e.Type); resolved {
			e.Type = t
			p.typ = t
		} else {
			p.mode = modeInvalid
			return p
		}
		switch t := tipe.Underlying(e.Type).(type) {
		case *tipe.Struct:
			return c.checkStructLiteral(e, t, p)
		case *tipe.Array:
			p = c.checkArrayLiteral(e, e.Keys, e.Values, t, p)
		case *tipe.Slice:
			p = c.checkSliceLiteral(e, e.Keys, e.Values, t, p)
		case *tipe.Map:
			p = c.checkMapLiteral(e, e.Keys, e.Values, t, p)
		default:
			c.errorfmt("cannot construct type %s with a composite literal", e.Type)
			p.mode = modeInvalid
			return p
		}
		p.expr = e
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
			c.errorfmt("cannot construct type %s with a map composite literal", e.Type)
			p.mode = modeInvalid
			return p
		}
		return c.checkMapLiteral(e, e.Keys, e.Values, t, p)

	case *expr.ArrayLiteral:
		p.mode = modeVar
		var arrayType *tipe.Array
		if t, resolved := c.resolve(e.Type); resolved {
			t, isArray := t.(*tipe.Array)
			if !isArray {
				c.errorfmt("type %s is not an array", t)
				p.mode = modeInvalid
				return p
			}
			e.Type = t
			p.typ = t
			arrayType = t
		} else {
			p.mode = modeInvalid
			return p
		}
		return c.checkArrayLiteral(e, e.Keys, e.Values, arrayType, p)

	case *expr.SliceLiteral:
		p.mode = modeVar
		var sliceType *tipe.Slice
		if t, resolved := c.resolve(e.Type); resolved {
			t, isSlice := t.(*tipe.Slice)
			if !isSlice {
				c.errorfmt("type %s is not a slice", t)
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
		return c.checkSliceLiteral(e, e.Keys, e.Values, sliceType, p)

	case *expr.TableLiteral:
		p.mode = modeVar

		var elemType tipe.Type
		if t, resolved := c.resolve(e.Type); resolved {
			t, isTable := t.(*tipe.Table)
			if !isTable {
				c.errorfmt("type %s is not a table", t)
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
			c.errorfmt("table literal has %d column names but a width of %d", len(e.ColNames), w)
			p.mode = modeInvalid
			return p
		}
		for _, r := range e.Rows {
			if len(r) != w {
				c.errorfmt("table literal has rows of different lengths (%d and %d)", w, len(r))
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

	case *expr.TypeAssert:
		left := c.expr(e.Left)
		if left.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		leftTyp, isInterface := tipe.Underlying(left.typ).(*tipe.Interface)
		if !isInterface {
			c.errorfmt("%s is not an interface", leftTyp)
			p.mode = modeInvalid
			return p
		}
		if e.Type == nil {
			// switch x.(type) {...}
			p.mode = left.mode
			p.typ, _ = c.resolve(left.typ)
			return p
		}
		t, resolved := c.resolve(e.Type)
		if !resolved {
			p.mode = modeInvalid
			return p
		}
		if c.typeAssert(leftTyp, t) {
			p.mode = left.mode
			p.typ = t
			return p
		}
		c.errorfmt("%s does not implement %s", t, leftTyp)
		p.mode = modeInvalid
		return p

	case *expr.Unary:
		switch e.Op {
		case token.LeftParen, token.Not, token.Sub, token.Add:
			sub := c.exprPartial(e.Expr, hintElideErr)
			p.mode = sub.mode
			p.typ = sub.typ
			p.val = sub.val
			return p
		case token.Ref:
			sub := c.expr(e.Expr)
			if sub.mode == modeInvalid {
				return p
			}
			p.mode = modeVar
			p.typ = &tipe.Pointer{Elem: sub.typ}
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
			c.errorfmt("invalid dereference of %s", e.Expr)
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
				c.errorfmt("receive from non-chan type %s", sub.typ)
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
				c.errorfmt("incomparable types %s and %s", lt, rt)
				left.mode = modeInvalid
				return left
			}
			switch e.Op {
			case token.Equal, token.NotEqual:
				if !isComparable(lt) {
					if canBeNil(lt) || canBeNil(rt) {
						if ltOrig != tipe.UntypedNil && rtOrig != tipe.UntypedNil {
							c.errorfmt("type %s only comparable to nil", lt)
							left.mode = modeInvalid
							return left
						}
					} else {
						c.errorfmt("incomparable type %s", lt)
						left.mode = modeInvalid
						return left
					}
				}
			case token.LessEqual, token.GreaterEqual, token.Less, token.Greater:
				if !isOrdered(lt) {
					c.errorfmt("unordered type %s", lt)
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
			c.errorfmt("inoperable types %s and %s", left.typ, right.typ)
			left.mode = modeInvalid
			return left
		}
		return left
	case *expr.Call:
		p := c.exprPartialCall(e)
		if hint == hintElideErr {
			if tuple, isTuple := p.typ.(*tipe.Tuple); isTuple {
				els := tuple.Elems
				num := len(els)
				if num > 0 && IsError(els[num-1]) {
					// We're eliding the error. If we mutate the tuple here
					// we're actually mutating the underlying *tipe.Func object
					// itself, so create a new tuple and copy over everything
					// but the last elem.
					e.ElideError = true
					tup2 := &tipe.Tuple{Elems: els[:num-1]}
					if len(tup2.Elems) == 1 {
						p.typ = tup2.Elems[0] // unwrap (x,) -> x
					} else {
						p.typ = tup2
					}
				}
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
			lt = tipe.Underlying(t.Elem)
		}
		switch lt := lt.(type) {
		case *tipe.Struct:
			for _, sf := range lt.Fields {
				if sf.Name == right {
					p.mode = modeVar
					p.typ = sf.Type
					return
				}
			}
			p.mode = modeInvalid
			c.errorfmt("%s undefined (type %s has no field or method %s)", e, lt, right)
			return p
		case *tipe.Package:
			for name, t := range lt.Exports {
				if name == e.Right.Name {
					p.typ = t
					if lt.GoPkg != nil {
						s := lt.GoPkg.(*gotypes.Package).Scope()
						obj := s.Lookup(name)
						if _, isAType := obj.(*gotypes.TypeName); isAType {
							p.mode = modeTypeExpr
							return p
						}
					}
					p.mode = modeVar // TODO modeFunc?
					return p
				}
			}
			p.mode = modeInvalid
			c.errorfmt("%s not in package %s", e, lt)
			return p
		}
		p.mode = modeInvalid
		c.errorfmt("%s undefined (type %s is not a struct or package)", e, left.typ)
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
				c.errorfmt("cannot table slice %s (type %s)", e.Left, left.typ)
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
				c.errorfmt("cannot table slice %s (type %s)", e.Left, left.typ)
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
		case *tipe.Slice, tipe.Basic:
			if basic, isBasic := lt.(tipe.Basic); isBasic && basic != tipe.String {
				p.mode = modeInvalid
				c.errorfmt("cannot index type %s", left.typ)
				return p
			}
			if len(e.Indicies) != 1 {
				p.mode = modeInvalid
				c.errorfmt("cannot table slice %s (type %s)", e.Left, left.typ)
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
			switch lt := lt.(type) {
			case *tipe.Slice:
				p.typ = lt.Elem
			case tipe.Basic: // tipe.String
				p.typ = tipe.Uint8
			}
			return p
		case *tipe.Table:
			p.mode = modeInvalid
			c.errorfmt("TODO table slicing support")
			return p
		default:
			p.mode = modeInvalid
			c.errorfmt("TODO index %T", lt)
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
				c.errorfmt("cannot slice type %s, expecting method %s but type has %s", left.typ, want, atTyp)
				return p
			}
			p.mode = modeVar
			p.typ = atTyp.Results.Elems[0]
			return p
		}
		if setTyp := c.memory.Method(lt, "Set"); setTyp != nil {
			p.mode = modeInvalid
			c.errorfmt("TODO Set index")
			return p
		}

		panic(fmt.Sprintf("typecheck.expr TODO Index: %s", format.Debug(e))) //, format.Debug(tipe.Underlying(left.typ))))
	case *expr.Shell:
		c.pushScope()
		defer c.popScope()
		c.cur.foundInParent = make(map[string]bool)

		p.mode = modeVar
		if hint == hintElideErr {
			p.typ = tipe.String
			e.ElideError = true
		} else {
			p.typ = &tipe.Tuple{Elems: []tipe.Type{
				tipe.String, Universe.Objs["error"].Type,
			}}
		}

		for _, cmd := range e.Cmds {
			c.shell(cmd)
		}

		for name := range c.cur.foundInParent {
			e.FreeVars = append(e.FreeVars, name)
		}

		return p
	}
	panic(fmt.Sprintf("expr TODO: %s", format.Debug(e)))
}

func (c *Checker) checkStructLiteral(e *expr.CompLiteral, t *tipe.Struct, p partial) partial {
	structName := fmt.Sprintf("%s", e.Type)
	elemsp := make([]partial, len(e.Values))
	for i, elem := range e.Values {
		elemsp[i] = c.expr(elem)
		if elemsp[i].mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
	}
	if len(e.Keys) == 0 {
		if len(e.Values) == 0 {
			return p
		}
		if len(e.Values) != len(t.Fields) {
			c.errorfmt("wrong number of elements, %d, when %s expects %d", len(e.Values), structName, len(t.Fields))
			p.mode = modeInvalid
			return p
		}
		for i, sf := range t.Fields {
			c.assign(&elemsp[i], sf.Type)
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
				c.errorfmt("invalid field name %s in struct initializer", e.Keys[i])
				p.mode = modeInvalid
				return p
			}
			namedp[ident.Name] = elemp
		}
		for _, sf := range t.Fields {
			elemp, found := namedp[sf.Name]
			if !found {
				continue
			}
			c.assign(&elemp, sf.Type)
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

}

func (c *Checker) checkMapLiteral(e expr.Expr, keys, vals []expr.Expr, t *tipe.Map, p partial) partial {
	for _, k := range keys {
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
	for _, v := range vals {
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
}

func (c *Checker) checkArrayLiteral(e expr.Expr, keys, vals []expr.Expr, t *tipe.Array, p partial) partial {
	for _, k := range keys {
		kp := c.expr(k)
		if kp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		c.assign(&kp, tipe.Int)
		if kp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
	}
	for _, v := range vals {
		vp := c.expr(v)
		if vp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		c.assign(&vp, t.Elem)
		if vp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
	}
	p.expr = e
	return p
}

func (c *Checker) checkSliceLiteral(e expr.Expr, keys, vals []expr.Expr, t *tipe.Slice, p partial) partial {
	for _, k := range keys {
		kp := c.expr(k)
		if kp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		c.assign(&kp, tipe.Int)
		if kp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
	}
	for _, v := range vals {
		vp := c.expr(v)
		if vp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
		c.assign(&vp, t.Elem)
		if vp.mode == modeInvalid {
			p.mode = modeInvalid
			return p
		}
	}
	p.expr = e
	return p
}

func (c *Checker) shell(cmd expr.Expr) {
	switch cmd := cmd.(type) {
	case *expr.ShellList:
		for _, andor := range cmd.AndOr {
			c.shell(andor)
		}
	case *expr.ShellAndOr:
		if len(cmd.Pipeline) == 1 {
			c.shell(cmd.Pipeline[0])
		}
		for _, pipeline := range cmd.Pipeline {
			c.pushScope()
			c.shell(pipeline)
			c.popScope()
		}
	case *expr.ShellPipeline:
		if len(cmd.Cmd) == 1 {
			c.shell(cmd.Cmd[0])
		}
		for _, cmd := range cmd.Cmd {
			c.pushScope()
			c.shell(cmd)
			c.popScope()
		}
	case *expr.ShellCmd:
		if cmd.SimpleCmd != nil {
			c.shell(cmd.SimpleCmd)
		}
		if cmd.Subshell != nil {
			c.pushScope()
			defer c.popScope()
			c.shell(cmd.Subshell)
		}
	case *expr.ShellSimpleCmd:
		if len(cmd.Args) > 0 {
			if cmd.Args[0] == "export" {
				// TODO: pull out export in parser and give it a syntax node?
				for _, p := range cmd.Args[1:] {
					name := strings.SplitN(p, "=", 2)[0]
					c.addObj(&Obj{
						Name: name,
						Kind: ObjVar,
						Type: tipe.String,
						Decl: cmd,
					})
				}
				return
			}

			c.pushScope()
			defer c.popScope()
		}

		for _, assign := range cmd.Assign {
			c.addObj(&Obj{
				Name: assign.Key,
				Kind: ObjVar,
				Type: tipe.String,
				Decl: cmd,
			})
		}

		params, err := shell.Parameters(cmd.Args)
		if err != nil {
			c.errorfmt("%v", err)
		}
		for _, name := range params {
			c.cur.LookupRec(name) // foundInParent
		}
	}
}

// unpackExprs evaluates a list of expr.Exprs using the hint.
// Each expr must be used in the same "context", such as
// representing multiple arguments to a single function call.
//
// For each expr, unpackExprs evaluates it and if the result
// is multi-valued (for example in the case of function calls
// with multiple return values, or "comma-ok" expressions)
// it unpacks the multiple values and returns them as separate
// partials.
//
// Since the Go spec does not support expressions like f(g(), "foo")
// if g returns more than one value, we only support single-valued
// expressions for every position but the last. However, error
// elision is applied, if possible, to turn a multi-valued (T, error)
// into a single-valued T, before applying that rule.
func (c *Checker) unpackExprs(hint typeHint, exprs ...expr.Expr) (partials []partial, ok bool) {
	partials = make([]partial, 0, len(exprs))
	for i, expr := range exprs {
		// We only elide errors in here for the "interior" expressions.
		// The last expression is never elided, to allow the caller
		// to decide what to do with it.
		interiorExpr := i < len(exprs)-1
		p := c.exprPartial(expr, hint)

		// Handle single expressions that are multi-valued.
		// TODO: handle comma-ok cases (map index, type assert, chan recv)
		if tup, ok := p.typ.(*tipe.Tuple); ok {
			// Treat each entry as the same underlying
			// expression, but update the types

			// If this is not the last expression, we do not
			// support multiple values unless we can apply error elision.
			elems := tup.Elems
			if interiorExpr && len(tup.Elems) > 1 {
				if len(tup.Elems) == 2 && IsError(tup.Elems[1]) {
					markElideError(expr)
					elems = elems[:1]
					c.types[p.expr] = &tipe.Tuple{Elems: elems}
				} else {
					c.errorfmt("multi-valued %s used in single-valued context", expr)
					return nil, false
				}
			}

			for i, typ := range elems {
				p2 := p // copy
				p2.typ = typ
				if i > 0 {
					// Hint that this argument was unpacked
					// from another expression.
					p2.mode = modeUnpacked
				}
				partials = append(partials, p2)
			}
		} else {
			partials = append(partials, p)
		}
	}
	return partials, true
}

func (c *Checker) assign(p *partial, t tipe.Type) {
	if p.mode == modeInvalid {
		return
	}
	if isUntyped(p.typ) {
		c.constrainUntyped(p, t)
		return
	}
	if !tipe.Equal(p.typ, t) {
		switch iface := tipe.Underlying(t).(type) {
		case *tipe.Interface:
			// make sure p.typ implements all methods of iface.
			if c.typeAssert(iface, p.typ) {
				return
			}
			// TODO: explain why p.typ does not implement t
		}
		c.errorfmt("cannot assign %s to %s", p.typ, t)
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
			c.errorfmt("constant %s does not fit in %s", p.val, t)
			p.mode = modeInvalid
			return
		}
	}

	if !c.convertible(t, p.typ) {
		c.errorfmt("cannot convert %s to %s", p.typ, t)
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
	if src == tipe.UntypedBool && tipe.Underlying(dst) == tipe.Bool {
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
			c.errorfmt("cannot convert %s to %s", p.typ, t)
		}
	} else {
		switch t := tipe.Unalias(t).(type) {
		case tipe.Basic:
			switch p.mode {
			case modeConst:
				p.val = round(p.val, t)
				if p.val == nil {
					c.errorfmt("cannot convert const %s to %s", p.typ, t)
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
	oldt := c.types[e]
	if oldt == t {
		return
	}
	c.types[e] = t

	switch e := e.(type) {
	case *expr.Bad, *expr.FuncLiteral: // TODO etc
		return
	case *expr.Binary:
		if c.consts[e] != nil {
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

	c.types[e] = t
}

// typeAssert returns true if a value of type iface can be type asserted
// to the type t.
//
// The static check for this is making sure that the type t implements
// all methods of iface. (Note that this does not mean that the concrete
// type under iface can be a t, just that as far as we know statically
// it might be.)
func (c *Checker) typeAssert(iface *tipe.Interface, t tipe.Type) bool {
	if len(iface.Methods) == 0 {
		return true // interface{} might be anything
	}

	if tiface, tIsIface := tipe.Underlying(t).(*tipe.Interface); tIsIface {
		for name, method := range iface.Methods {
			if !tipe.Equal(tiface.Methods[name], method) {
				return false
			}
		}
	} else {
		for name, method := range iface.Methods {
			mt := findMember(t, name)
			if !tipe.Equal(method, mt) {
				return false
			}
		}
	}

	return true
}

// findMember finds the field or method with name in type t.
//
// TODO: there is a lot to do here re: embedding. We have to think
// if we are enumerating the order correctly, worry about infinite
// recursion, and think about the pkg the field belongs to.
func findMember(t tipe.Type, name string) (mt tipe.Type) {
	if tp, isPointer := tipe.Underlying(t).(*tipe.Pointer); isPointer {
		t = tp.Elem
	}

	for t != nil {
		if methodik, isNamed := t.(*tipe.Named); isNamed {
			for i, mname := range methodik.MethodNames {
				if mname == name {
					return methodik.Methods[i]
				}
			}
			t = methodik.Type
		}

		if st, isStruct := t.(*tipe.Struct); isStruct {
			for _, sf := range st.Fields {
				if sf.Name == name {
					return sf.Type
				}
				// TODO: if the field is an embedding,
				// collect it onto the list of types
				// to analyze.
			}
		}
		return nil
	}
	panic("unreachable")
}

func (c *Checker) errorfmt(formatstr string, args ...interface{}) {
	for i, arg := range args {
		switch v := arg.(type) {
		case stmt.Stmt:
			args[i] = format.Stmt(v)
		case tipe.Type:
			// TODO: use a short mode for printing
			// types like methodik, which can scroll
			// on for many pages.
			args[i] = format.Type(v)
		case expr.Expr:
			args[i] = format.Expr(v)
		}
	}

	err := fmt.Errorf(formatstr, args...)
	c.errs = append(c.errs, err)
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

func (c *Checker) Type(e expr.Expr) (t tipe.Type) {
	c.mu.Lock()
	t = c.types[e]
	c.mu.Unlock()
	return t
}

// Ident reports the object an identifier refers to.
func (c *Checker) Ident(e *expr.Ident) *Obj {
	c.mu.Lock()
	id := c.idents[e]
	c.mu.Unlock()
	return id
}

// NewScope make a copy of Checker with a new, blank current scope.
// The two checkers share all type checked data.
func (c *Checker) NewScope() *Checker {
	c.mu.Lock()
	defer c.mu.Unlock()

	newc := *c
	newc.cur = &Scope{
		Parent: Universe,
		Objs:   make(map[string]*Obj),
	}
	return &newc
}

// TypesWithPrefix returns the names of all types currently in
// scope that start with prefix.
func (c *Checker) TypesWithPrefix(prefix string) (res []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for scope := c.cur; scope != nil; scope = scope.Parent {
		for k, obj := range scope.Objs {
			if obj.Kind != ObjType {
				continue
			}
			if strings.HasPrefix(k, prefix) {
				res = append(res, k)
			}
		}
	}
	return res
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
		case tipe.UntypedComplex, tipe.Complex:
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stmt(s, nil)
}

func (c *Checker) Lookup(name string) *Obj {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur.LookupRec(name)
}

func (c *Checker) addObj(obj *Obj) {
	c.cur.Objs[obj.Name] = obj

	if c.cur.Parent == Universe {
		if c.curPkg.GlobalNames[obj.Name] == nil {
			c.curPkg.Globals = append(c.curPkg.Globals, obj)
			c.curPkg.GlobalNames[obj.Name] = obj
			if isExported(obj.Name) {
				c.curPkg.Type.Exports[obj.Name] = obj.Type
			}
		}
	}
}

type Scope struct {
	Parent *Scope
	Objs   map[string]*Obj

	// foundInParent tracks variables which were found in the Parent scope.
	// foundMdikInParent tracks any Methodik defined up the scope chain.
	// These are used to build a list of free variables.
	foundInParent     map[string]bool
	foundMdikInParent map[*tipe.Named]bool
}

func (s *Scope) LookupRec(name string) *Obj {
	if o := s.Objs[name]; o != nil {
		return o
	}
	if s.Parent == nil {
		return nil
	}
	if name == "" {
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
		if mdik, ok := o.Type.(*tipe.Named); ok {
			s.foundMdikInParent[mdik] = true
		}
	}
	return o
}

func (s *Scope) DebugPrint(indent int) {
	for i := 0; i < indent; i++ {
		fmt.Print("\t")
	}
	scopeName := ""
	if s == Universe {
		scopeName = " (universe)"
	}
	fmt.Printf("scope %p%s:\n", s, scopeName)
	var names []string
	for name := range s.Objs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for i := 0; i <= indent; i++ {
			fmt.Print("\t")
		}
		fmt.Printf("%s\n", name)
	}
	if s.Parent != nil {
		s.Parent.DebugPrint(indent + 1)
	}
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
	Name string
	Kind ObjKind
	Type tipe.Type
	Decl interface{} // *expr.FuncLiteral, *stmt.MethodikDecl, constant.Value, *stmt.TypeDecl, *Package
	Used bool
}

type Package struct {
	GoPkg       *gotypes.Package
	Path        string
	Type        *tipe.Package
	Globals     []*Obj
	GlobalNames map[string]*Obj
	Syntax      *syntax.File
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
		for _, sf := range t.Fields {
			if !isComparable(sf.Type) {
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
	case tipe.UntypedRune:
		return tipe.Rune
	}
	return t
}

// markElideError marks the expression to dynamically
// elide errors at runtime. If the expression type does not
// support eliding errors, it does nothing.
func markElideError(e expr.Expr) {
	switch e := e.(type) {
	case *expr.Call:
		e.ElideError = true
	case *expr.Shell:
		e.ElideError = true
	}
}
