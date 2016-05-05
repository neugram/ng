// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"errors"
	"fmt"
	goimporter "go/importer"
	gotypes "go/types"
	"math/big"
	"os"
	"reflect"
	"runtime/debug"

	"neugram.io/eval/environ"
	"neugram.io/eval/gowrap"
	"neugram.io/eval/shell"
	"neugram.io/lang/expr"
	"neugram.io/lang/stmt"
	"neugram.io/lang/tipe"
	"neugram.io/lang/token"
	"neugram.io/lang/typecheck"
)

// A Variable is an addressable Value.
type Variable struct {
	// Value has the type:
	//	nil
	//	int64
	//	float32
	//	float64
	//	*big.Int
	//	*big.Float
	//
	//	*Ptr
	//
	//	*expr.FuncLiteral
	//	*Closure
	//
	//	*GoFunc
	// 	*GoPkg
	// 	*GoValue
	Value interface{} // TODO introduce a Value type
}

func (v *Variable) Assign(val interface{}) { v.Value = val }

type Ptr struct {
	Elem *Variable
}

type Assignable interface {
	Assign(v interface{})
}

type mapKey struct {
	m mapImpl
	k interface{}
}

func (m mapKey) Assign(val interface{}) { m.m.SetVal(m.k, val) }

type goPtr struct {
	v reflect.Value
}

func (p goPtr) Assign(val interface{}) { p.v.Elem().Set(reflect.ValueOf(val)) }

type MethodikDeclScope struct {
	Decl  *stmt.MethodikDecl
	Scope *Scope
}

// TODO The Var map needs a lock. Right now two goroutines running neugram code
// with no shared memory will conflict over Var map writes.
type Scope struct {
	Parent *Scope
	Var    map[string]*Variable // variable name -> variable
	Mdik   map[*tipe.Methodik]MethodikDeclScope

	// Implicit is set if the Scope was created mid block and should be
	// unrolled when block ends.
	Implicit bool
}

func (s *Scope) Lookup(name string) *Variable {
	if v := s.Var[name]; v != nil {
		return v
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil
}

func (s *Scope) LookupMdik(m *tipe.Methodik) MethodikDeclScope {
	if v, ok := s.Mdik[m]; ok {
		return v
	}
	if s.Parent != nil {
		return s.Parent.LookupMdik(m)
	}
	return MethodikDeclScope{}
}

func builtinPrint(v ...interface{})                 { fmt.Println(v...) }
func builtinPrintf(format string, v ...interface{}) { fmt.Printf(format, v...) }

func zeroVariable(t tipe.Type) *Variable {
	switch t := t.(type) {
	case tipe.Basic:
		// TODO: propogate specialization context for Num
		switch t {
		case tipe.Bool:
			return &Variable{Value: false}
		case tipe.Byte:
			return &Variable{Value: byte(0)}
		case tipe.Rune:
			return &Variable{Value: rune(0)}
		//case tipe.Integer:
		//case tipe.Float:
		//case tipe.Complex:
		case tipe.String:
			return &Variable{Value: ""}
		case tipe.Int64:
			return &Variable{Value: int64(0)}
		case tipe.Float32:
			return &Variable{Value: float32(0)}
		case tipe.Float64:
			return &Variable{Value: float64(0)}
		default:
			panic(fmt.Sprintf("TODO zero Basic: %s", t))
		}
	case *tipe.Func:
		return &Variable{Value: nil}
	case *tipe.Struct:
		s := &StructVal{Fields: make([]*Variable, len(t.Fields))}
		for i, ft := range t.Fields {
			s.Fields[i] = zeroVariable(ft)
		}
		return &Variable{Value: s}
	case *tipe.Methodik:
		return &Variable{Value: nil}
	// TODO _ = Type((*Table)(nil))
	case *tipe.Pointer:
		return &Variable{Value: nil}
	default:
		panic(fmt.Sprintf("don't know the zero value of type %T", t))
	}
}

// A *StructVal is a Neugram object for a *tipe.Struct.
//
// Replace this with reflect.MakeStruct when it exists, for compatibility
// with packages that use reflection (like encoding/json and fmt).
type StructVal struct {
	// TODO: if a Neugram interface value is satisfied by a *StructVal,
	// then we are going to have to carry the underlying tipe.Type here.
	Fields []*Variable
}

type MethodikVal struct {
	Value   interface{}
	Methods map[string]*Closure
}

type Closure struct {
	Func  *expr.FuncLiteral
	Scope *Scope
}

var panicFunc = &expr.FuncLiteral{
	Name: "panic",
	Type: typecheck.Universe.Objs["panic"].Type.(*tipe.Func),
}

type Panic struct {
	str string
}

func (p Panic) Error() string {
	return fmt.Sprintf("neugram panic: %s", p.str)
}

func New() *Program {
	universe := &Scope{Var: map[string]*Variable{
		"true":  &Variable{Value: true},
		"false": &Variable{Value: false},
		"env":   &Variable{Value: environ.New()},
		"nil":   &Variable{Value: nil},
		"print": &Variable{Value: &GoFunc{
			Type: typecheck.Universe.Objs["print"].Type.(*tipe.Func),
			Func: builtinPrint,
		}},
		"printf": &Variable{Value: &GoFunc{
			Type: typecheck.Universe.Objs["printf"].Type.(*tipe.Func),
			Func: builtinPrintf,
		}},
		"panic": &Variable{Value: panicFunc},
	}}

	p := &Program{
		Universe: universe,
		Types:    typecheck.New(),
		Pkg: map[string]*Scope{
			"main": &Scope{
				Parent: universe,
				Var:    map[string]*Variable{},
			},
		},
	}
	p.Cur = p.Pkg["main"]
	p.Types.ImportGo = p.importGo
	return p
}

type Program struct {
	Pkg       map[string]*Scope // package -> scope
	Universe  *Scope
	Cur       *Scope
	Types     *typecheck.Checker
	Returning bool
	Breaking  bool
}

func (p *Program) Environ() *environ.Environ {
	return p.Universe.Lookup("env").Value.(*environ.Environ)
}

// Get is part of the implementation of shell.Params.
func (p *Program) Get(name string) string {
	v := p.Cur.Lookup(name)
	if v == nil {
		return p.Environ().Get(name)
	}
	// TODO this is handy. Could we reasonably define a string(val)
	// constructor in the language that follows this same logic?
	val := v.Value
	if gv, ok := val.(*GoValue); ok {
		val = gv.Value
	}
	switch val := val.(type) {
	case nil:
		return ""
	case rune:
		return string(val)
	case string:
		return val
	case int64:
		return fmt.Sprintf("%d", val)
	case float32:
		return fmt.Sprintf("%f", val)
	case float64:
		return fmt.Sprintf("%f", val)
	case *big.Int:
		return val.String()
	case *big.Float:
		return val.String()
	default:
		return ""
	}
}

// Set is part of the implementation of shell.Params.
func (p *Program) Set(name, value string) {
	panic("TODO Scope.Set")
}

func (p *Program) importGo(path string) (*gotypes.Package, error) {
	if gowrap.Pkgs[path] == nil {
		return nil, fmt.Errorf("neugram: Go package %q not known", path)
	}
	pkg, err := goimporter.Default().Import(path)
	if err != nil {
		return nil, err
	}
	return pkg, err
}

func (p *Program) Eval(s stmt.Stmt) (res []interface{}, resType tipe.Type, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("ng eval panic: %v", x)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			debug.PrintStack()
			res = nil
		}
	}()

	p.Types.Errs = p.Types.Errs[:0]
	resType = p.Types.Add(s)
	if len(p.Types.Errs) > 0 {
		return nil, nil, fmt.Errorf("typecheck: %v\n", p.Types.Errs[0])
	}

	res, err = p.evalStmt(s)
	if err != nil {
		return nil, nil, err
	}
	for i, v := range res {
		res[i], err = p.readVar(v)
		if err != nil {
			return nil, nil, err
		}
	}
	return res, resType, nil
}

func (p *Program) pushScope() {
	p.Cur = &Scope{
		Parent: p.Cur,
		Var:    make(map[string]*Variable),
	}
}
func (p *Program) pushImplicitScope() {
	p.Cur = &Scope{
		Parent:   p.Cur,
		Var:      make(map[string]*Variable),
		Implicit: true,
	}
}
func (p *Program) popScope() {
	for p.Cur.Implicit {
		p.Cur = p.Cur.Parent
	}
	p.Cur = p.Cur.Parent
}

func isError(t tipe.Type) bool {
	return typecheck.Universe.Objs["error"].Type == t
}

func (p *Program) evalStmt(s stmt.Stmt) ([]interface{}, error) {
	switch s := s.(type) {
	case *stmt.Assign:
		vars := make([]Assignable, len(s.Left))
		if s.Decl {
			for i, lhs := range s.Left {
				v := new(Variable)
				vars[i] = v
				p.Cur.Var[lhs.(*expr.Ident).Name] = v
			}
		} else {
			// TODO: order of evaluation, left-then-right,
			// or right-then-left?
			for i, lhs := range s.Left {
				v, err := p.evalExprAsAssignable(lhs)
				if err != nil {
					return nil, err
				}
				vars[i] = v
			}
		}
		vals := make([]interface{}, 0, len(s.Left))
		for _, rhs := range s.Right {
			v, err := p.evalExprAndReadVars(rhs)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v...)
		}
		for i := range vars {
			vars[i].Assign(vals[i])
		}
		isLastError := false
		if len(s.Right) == 1 {
			t := p.Types.Types[s.Right[0]]
			if isError(t) {
				isLastError = true
			} else if tuple, isTuple := t.(*tipe.Tuple); isTuple {
				if isError(tuple.Elems[len(tuple.Elems)-1]) {
					isLastError = true
				}
			}
		}
		if isLastError && len(vars) == len(vals)-1 {
			// last error is ignored, panic if non-nil
			errVal := vals[len(vals)-1]
			if errVal != nil {
				// TODO: Go object
				errFn := errVal.(*MethodikVal).Methods["Error"]
				res, err := p.callClosure(errFn, nil)
				if err != nil {
					return nil, Panic{str: fmt.Sprintf("panic during error panic: %v", err)}
				}
				v, err := p.readVar(res[0])
				if err != nil {
					return nil, Panic{str: fmt.Sprintf("panic during error result: %v", err)}
				}
				return nil, Panic{str: fmt.Sprintf("uncaught error: %v", v)}
			}
		}
		return nil, nil
	case *stmt.Simple:
		return p.evalExpr(s.Expr)
	case *stmt.Block:
		p.pushScope()
		defer p.popScope()
		for _, s := range s.Stmts {
			res, err := p.evalStmt(s)
			if err != nil {
				return nil, err
			}
			if p.Returning || p.Breaking {
				return res, nil
			}
		}
		return nil, nil
	case *stmt.If:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			if _, err := p.evalStmt(s.Init); err != nil {
				return nil, err
			}
		}
		cond, err := p.evalExprAndReadVar(s.Cond)
		if err != nil {
			return nil, err
		}
		if cond.(bool) {
			return p.evalStmt(s.Body)
		} else if s.Else != nil {
			return p.evalStmt(s.Else)
		}
		return nil, nil
	case *stmt.For:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			if _, err := p.evalStmt(s.Init); err != nil {
				return nil, err
			}
		}
		for {
			cond, err := p.evalExprAndReadVar(s.Cond)
			if err != nil {
				return nil, err
			}
			if !cond.(bool) {
				break
			}
			if _, err := p.evalStmt(s.Body); err != nil {
				return nil, err
			}
			if s.Post != nil {
				if _, err := p.evalStmt(s.Post); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	case *stmt.Return:
		var err error
		var res []interface{}
		if len(s.Exprs) == 1 {
			res, err = p.evalExprAndReadVars(s.Exprs[0])
		} else {
			res = make([]interface{}, len(s.Exprs))
			for i, e := range s.Exprs {
				res[i], err = p.evalExprAndReadVar(e)
				if err != nil {
					break
				}
			}
		}
		p.Returning = true
		if err != nil {
			return nil, err
		}
		return res, nil
	case *stmt.Import:
		typ := p.Types.Lookup(s.Name).Type.(*tipe.Package)
		p.Cur.Var[s.Name] = &Variable{
			Value: &GoPkg{
				Type:  typ,
				GoPkg: p.Types.GoPkgs[typ],
			},
		}
		return nil, nil
	case *stmt.TypeDecl:
		return nil, nil
	case *stmt.MethodikDecl:
		// When a Methodik is defined, we capture its current scope.
		scope := &Scope{
			Parent: p.Universe,
			Var:    make(map[string]*Variable),
		}
		for _, m := range s.Methods {
			for _, freeVar := range m.Type.FreeVars {
				scope.Var[freeVar] = p.Cur.Lookup(freeVar)
			}
		}
		p.pushImplicitScope()
		p.Cur.Mdik = map[*tipe.Methodik]MethodikDeclScope{
			s.Type: MethodikDeclScope{
				Decl:  s,
				Scope: scope,
			},
		}
		return nil, nil
	}
	if s == nil {
		return nil, fmt.Errorf("Parser.evalStmt: statement is nil")
	}
	panic(fmt.Sprintf("TODO evalStmt: %T: %s", s, s.Sexp()))
}

func (p *Program) evalExprAsAssignable(e expr.Expr) (Assignable, error) {
	switch e := e.(type) {
	case *expr.Index:
		container, err := p.evalExpr(e.Expr)
		if err != nil {
			return nil, err
		}
		if _, isMap := p.Types.Types[e.Expr].(*tipe.Map); isMap {
			kvar, err := p.evalExpr(e.Index)
			if err != nil {
				return nil, err
			}
			k, err := p.readVar(kvar[0])
			if err != nil {
				return nil, err
			}
			m := container[0].(*Variable).Value.(mapImpl)
			return mapKey{m: m, k: k}, nil
		}
		panic("evalExprAsAssignable Index TODO")
	case *expr.Unary: // dereference
		if e.Op != token.Mul {
			panic(fmt.Sprintf("evalExprAsAssignable Unary unknown op: %s", e.Op))
		}
		v, err := p.evalExprAsVar(e.Expr)
		if err != nil {
			return nil, err
		}
		switch v := v.(type) {
		case *Variable:
			return v.Value.(*Ptr).Elem, nil
		case *GoValue:
			return goPtr{v: reflect.ValueOf(v.Value)}, nil
		default:
			panic(fmt.Sprintf("TODO deref of unexpected type: %T", v))
		}
	default:
		res, err := p.evalExprAsVar(e)
		if err != nil {
			return nil, err
		}
		if v, ok := res.(*Variable); ok {
			return v, nil
		}
		return nil, fmt.Errorf("eval: %s not assignable", e)
	}
}

func (p *Program) evalExprAndReadVars(e expr.Expr) ([]interface{}, error) {
	res, err := p.evalExpr(e)
	if err != nil {
		return nil, err
	}
	for i, v := range res {
		res[i], err = p.readVar(v)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (p *Program) evalExprAndReadVar(e expr.Expr) (interface{}, error) {
	res, err := p.evalExprAsVar(e)
	if err != nil {
		return nil, err
	}
	return p.readVar(res)
}

func (p *Program) evalExprAsVar(e expr.Expr) (interface{}, error) {
	res, err := p.evalExpr(e)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		// backup error message, caught by typechecker
		return nil, errors.New("multi-valued expression in single-value context")
	}
	return res[0], nil
}

func (p *Program) readVar(e interface{}) (interface{}, error) {
	if e == nil {
		return nil, nil
	}
	switch v := e.(type) {
	case *expr.FuncLiteral, *GoFunc, *GoValue, *Closure, *Ptr:
		// lack of symmetry with BasicLiteral is unfortunate
		return v, nil
	case *expr.BasicLiteral:
		return v.Value, nil
	case *Variable:
		return v.Value, nil
	case bool, int64, float32, float64, *big.Int, *big.Float:
		return v, nil
	case int, string: // TODO: are these all GoValues now?
		return v, nil
	case *StructVal:
		return v, nil
	case *MethodikVal:
		return v, nil
	case mapImpl:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected type %T for value", v)
	}
}

func (p *Program) callClosure(c *Closure, args []interface{}) ([]interface{}, error) {
	oldCur := p.Cur
	p.Cur = c.Scope
	defer func() { p.Cur = oldCur }()
	return p.callFuncLiteral(c.Func, args)
}

func (p *Program) callFuncLiteral(f *expr.FuncLiteral, args []interface{}) ([]interface{}, error) {
	if f == panicFunc {
		return nil, Panic{str: args[0].(string)}
	}

	p.pushScope()
	defer p.popScope()

	if f.Type.Variadic {
		return nil, fmt.Errorf("TODO call FuncLiteral with variadic args")
	} else {
		for i, name := range f.ParamNames {
			v := &Variable{Value: args[i]}
			p.Cur.Var[name] = v
		}
	}

	res, err := p.evalStmt(f.Body.(*stmt.Block))
	if err != nil {
		return nil, err
	}
	if p.Returning {
		p.Returning = false
	} else if len(f.ResultNames) > 0 {
		return nil, fmt.Errorf("missing return %v", f.ResultNames)
	}
	return res, nil
}

func (p *Program) evalSelector(e *expr.Selector) ([]interface{}, error) {
	lhs, err := p.evalExprAsVar(e.Left)
	if err != nil {
		return nil, err
	}

	if v, ok := lhs.(*Variable); ok {
		lhs = v.Value
	}
	t := p.Types.Types[e.Left]
	switch lhs := lhs.(type) {
	case *MethodikVal:
		if m := lhs.Methods[e.Right.Name]; m != nil {
			return []interface{}{m}, nil
		}
		// TODO: non-struct methodik
		structt := tipe.Underlying(t).(*tipe.Struct)
		for i, n := range structt.FieldNames {
			if n == e.Right.Name {
				return []interface{}{lhs.Value.(*StructVal).Fields[i]}, nil
			}
		}
		return nil, fmt.Errorf("unknown method or field %s in %s", e.Right.Name, t)
	case *StructVal:
		t := tipe.Underlying(t).(*tipe.Struct)
		for i, n := range t.FieldNames {
			if n == e.Right.Name {
				return []interface{}{lhs.Fields[i]}, nil
			}
		}
		return nil, fmt.Errorf("unknown field %s in %s", e.Right.Name, t)
	case *GoValue:
		v := reflect.ValueOf(lhs.Value)
		if m := v.MethodByName(e.Right.Name); (m != reflect.Value{}) {
			res := &GoFunc{
				Type: p.Types.Types[e].(*tipe.Func),
				Func: m.Interface(),
			}
			return []interface{}{res}, nil
		}
		panic("field selector on a GoValue")
	case *GoPkg:
		v := lhs.Type.Exports[e.Right.Name]
		if v == nil {
			return nil, fmt.Errorf("%s not found in Go package %s", e, e.Left)
		}
		switch v := v.(type) {
		case *tipe.Func:
			res := &GoFunc{
				Type: v,
				Func: gowrap.Pkgs[lhs.Type.Path].Exports[e.Right.Name],
			}
			return []interface{}{res}, nil
		}
		return nil, fmt.Errorf("TODO GoPkg: %#+v\n", lhs)
	}

	fmt.Printf("lhs: %#+v\n", lhs)
	return nil, fmt.Errorf("unexpected selector LHS: %s, %T", e.Left.Sexp(), lhs)
}

func (p *Program) evalExpr(e expr.Expr) ([]interface{}, error) {
	switch e := e.(type) {
	case *expr.BasicLiteral:
		return []interface{}{e}, nil
	case *expr.CompLiteral:
		if goType := p.Types.GoEquiv[e.Type]; goType != nil {
			typeName := goType.(*gotypes.Named).Obj()
			return []interface{}{makeGoStruct(e, e.Type, typeName)}, nil
		}
		t := e.Type
		/* TODO peel off *Named
		for {
			named, ok := t.(*tipe.Named)
			if !ok {
				break
			}
			t = named.Type
		}
		*/
		var res interface{}
		switch t := tipe.Underlying(t).(type) {
		default:
			return nil, fmt.Errorf("non-struct composite literal: %T", e.Type)
		case *tipe.Struct:
			s := &StructVal{
				Fields: make([]*Variable, len(t.Fields)),
			}
			if len(e.Keys) > 0 {
				named := make(map[string]expr.Expr)
				for i, elem := range e.Elements {
					named[e.Keys[i].(*expr.Ident).Name] = elem
				}
				for i, ft := range t.Fields {
					expr, ok := named[t.FieldNames[i]]
					if ok {
						v, err := p.evalExprAndReadVar(expr)
						if err != nil {
							return nil, err
						}
						s.Fields[i] = &Variable{Value: v}
					} else {
						s.Fields[i] = zeroVariable(ft)
					}
				}
			} else {
				for i, expr := range e.Elements {
					v, err := p.evalExprAndReadVar(expr)
					if err != nil {
						return nil, err
					}
					s.Fields[i] = &Variable{Value: v}
				}
			}
			res = s
		}
		if mdik, ok := t.(*tipe.Methodik); ok {
			// TODO: find right p.Pkg[pkgName] if Methodik is local.
			pkgScope := p.Cur
			mscope := pkgScope.LookupMdik(mdik)
			mval := &MethodikVal{
				Value:   res,
				Methods: make(map[string]*Closure),
			}
			res = mval
			for i, name := range mdik.MethodNames {
				mlit := mscope.Decl.Methods[i]
				scope := mscope.Scope
				if mlit.ReceiverName != "" {
					// TODO: support PointerReceiver
					scope = &Scope{
						Parent: scope,
						Var: map[string]*Variable{
							mlit.ReceiverName: &Variable{mval},
						},
					}
				}
				mval.Methods[name] = &Closure{
					Func:  mlit,
					Scope: scope,
				}
			}
		}
		return []interface{}{res}, nil
	case *expr.MapLiteral:
		//t := e.Type.(*tipe.Map)
		m := make(mapImplM, len(e.Keys))
		for i, kexpr := range e.Keys {
			k, err := p.evalExprAndReadVar(kexpr)
			if err != nil {
				return nil, err
			}
			v, err := p.evalExprAndReadVar(e.Values[i])
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		return []interface{}{m}, nil
	case *expr.FuncLiteral:
		var res interface{}
		if len(e.Type.FreeVars) > 0 {
			c := &Closure{
				Func:  e,
				Scope: &Scope{Parent: p.Universe},
			}
			for _, name := range e.Type.FreeVars {
				if c.Scope.Var == nil {
					c.Scope.Var = make(map[string]*Variable)
				}
				c.Scope.Var[name] = p.Cur.Lookup(name)
			}
			for _, mdik := range e.Type.FreeMdik {
				if c.Scope.Mdik == nil {
					c.Scope.Mdik = make(map[*tipe.Methodik]MethodikDeclScope)
				}
				c.Scope.Mdik[mdik] = p.Cur.LookupMdik(mdik)
			}
			res = c
		} else {
			res = e
		}
		if e.Name != "" {
			p.Cur.Var[e.Name] = &Variable{Value: res}
		}
		return []interface{}{res}, nil
	case *expr.Ident:
		if v := p.Cur.Lookup(e.Name); v != nil {
			return []interface{}{v}, nil
		}
		return nil, fmt.Errorf("eval: undefined identifier: %q", e.Name)
	case *expr.Unary:
		switch e.Op {
		case token.LeftParen:
			return p.evalExpr(e.Expr)
		case token.Ref:
			v, err := p.evalExprAsVar(e.Expr)
			if err != nil {
				return nil, err
			}
			t := p.Types.Types[e]
			switch v := v.(type) {
			case *Variable:
				return []interface{}{&Ptr{Elem: v}}, nil
			// TODO *GoFunc?
			case *GoValue:
				rv := reflect.New(reflect.TypeOf(v.Value))
				rv.Elem().Set(reflect.ValueOf(v.Value))
				res := &GoValue{
					Type:  t,
					Value: rv.Interface(),
				}
				return []interface{}{res}, nil
			default:
				panic(fmt.Sprintf("TODO Ref of unexpected type: %T", v))
			}
		case token.Mul: // deref
			v, err := p.evalExprAsVar(e.Expr)
			if err != nil {
				return nil, err
			}
			switch v := v.(type) {
			case *Variable:
				return []interface{}{v.Value.(*Ptr).Elem}, nil
			default:
				panic(fmt.Sprintf("TODO deref of unexpected type: %T", v))
			}
		case token.Not:
			v, err := p.evalExprAndReadVar(e.Expr)
			if err != nil {
				return nil, err
			}
			if v, ok := v.(bool); ok {
				return []interface{}{!v}, nil
			}
			return nil, fmt.Errorf("negation operator expects boolean expression, not %T", v)
		case token.Sub:
			rhs, err := p.evalExprAndReadVar(e.Expr)
			if err != nil {
				return nil, err
			}
			// TODO use exprType := p.Types.Types[e.Expr]
			var lhs interface{}
			switch rhs.(type) {
			case int64:
				lhs = int64(0)
			case float32:
				lhs = float32(0)
			case float64:
				lhs = float64(0)
			case *big.Int:
				lhs = big.NewInt(0)
			case *big.Float:
				lhs = big.NewFloat(0)
			}
			v, err := binOp(token.Sub, lhs, rhs)
			if err != nil {
				return nil, err
			}
			return []interface{}{v}, nil
		}
	case *expr.Binary:
		lhs, err := p.evalExprAndReadVar(e.Left)
		if err != nil {
			return nil, err
		}

		switch e.Op {
		case token.LogicalAnd, token.LogicalOr:
			if e.Op == token.LogicalAnd && !lhs.(bool) {
				return []interface{}{false}, nil
			}
			if e.Op == token.LogicalOr && lhs.(bool) {
				return []interface{}{true}, nil
			}
			rhs, err := p.evalExprAndReadVar(e.Right)
			if err != nil {
				return nil, err
			}
			return []interface{}{rhs.(bool)}, nil
		}

		rhs, err := p.evalExprAndReadVar(e.Right)
		if err != nil {
			return nil, err
		}

		v, err := binOp(e.Op, lhs, rhs)
		if err != nil {
			return nil, err
		}
		return []interface{}{v}, nil
	case *expr.Call:
		res, err := p.evalExprAndReadVar(e.Func)
		if err != nil {
			return nil, err
		}

		args := make([]interface{}, len(e.Args))
		for i, arg := range e.Args {
			// TODO calling g(f()) where:
			//	g(T, U) and f() (T, U)
			v, err := p.evalExprAndReadVar(arg)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}

		switch fn := res.(type) {
		case *Closure:
			return p.callClosure(fn, args)
		case *expr.FuncLiteral:
			return p.callFuncLiteral(fn, args)
		case *GoFunc:
			res, err := fn.call(args)
			if err != nil {
				return nil, err
			}
			return res, nil
		default:
			return nil, fmt.Errorf("do not know how to call %T", fn)
		}
	case *expr.Shell:
		for _, cmd := range e.Cmds {
			j := &shell.Job{
				Cmd:    cmd,
				Params: p,
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			}
			if err := j.Start(); err != nil {
				return nil, err
			}
			done, err := j.Wait()
			if err != nil {
				return nil, err
			}
			if !done {
				break // TODO not right, instead we should just have one cmd, not Cmds here.
			}
		}
		return nil, nil
	case *expr.Selector:
		return p.evalSelector(e)

	case *expr.Index:
		container, err := p.evalExprAndReadVar(e.Expr)
		if err != nil {
			return nil, err
		}
		if _, isMap := p.Types.Types[e.Expr].(*tipe.Map); isMap {
			k, err := p.evalExprAndReadVar(e.Index)
			if err != nil {
				return nil, err
			}
			v := container.(mapImpl).GetVal(k)
			return []interface{}{v}, nil
		}
	}
	return nil, fmt.Errorf("TODO evalExpr(%s), %T", e.Sexp(), e)
}

type mapImpl interface {
	GetVal(key interface{}) interface{}
	SetVal(key, val interface{})
}

type mapImplM map[interface{}]interface{}

func (m mapImplM) GetVal(key interface{}) interface{} { return m[key] }
func (m mapImplM) SetVal(key, val interface{})        { m[key] = val }
