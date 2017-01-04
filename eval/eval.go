// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"io/ioutil"
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

type Scope struct {
	Parent  *Scope
	VarName string
	Var     reflect.Value

	// Implicit is set if the Scope was created mid block and should be
	// unrolled when block ends.
	Implicit bool
}

func (s *Scope) Lookup(name string) reflect.Value {
	for scope := s; scope != nil; scope = scope.Parent {
		if scope.VarName == name {
			return scope.Var
		}
	}
	return reflect.Value{}
}

type Program struct {
	Universe  *Scope
	PkgVars   map[string]*interface{}
	Cur       *Scope
	Types     *typecheck.Checker
	reflector *reflector

	Returning bool
	Breaking  bool

	// builtinCalled is set by any builtin function that has
	// a generic return type. The intepreter has to unbox the
	// return type.
	builtinCalled bool
}

type evalMap interface {
	GetVal(key interface{}) interface{}
	SetVal(key, val interface{})
}

func New() *Program {
	universe := new(Scope)
	p := &Program{
		Universe: universe,
		Types:    typecheck.New(),
		PkgVars:  make(map[string]*interface{}),
		Cur: &Scope{
			Parent: universe,
		},
		reflector: newReflector(),
	}
	addUniverse := func(name string, val interface{}) {
		p.Universe = &Scope{
			Parent:  p.Universe,
			VarName: name,
			Var:     reflect.ValueOf(val),
		}
		p.Cur.Parent = p.Universe
	}
	addUniverse("true", true)
	addUniverse("false", false)
	addUniverse("env", (evalMap)(environ.New()))
	addUniverse("alias", (evalMap)(environ.New()))
	addUniverse("nil", nil)
	addUniverse("print", fmt.Println)
	addUniverse("printf", fmt.Printf)
	addUniverse("errorf", fmt.Errorf)
	addUniverse("len", func(c interface{}) int {
		if c == nil {
			return 0
		}
		return reflect.ValueOf(c).Len()
	})
	addUniverse("cap", func(c interface{}) int {
		if c == nil {
			return 0
		}
		return reflect.ValueOf(c).Cap()
	})
	addUniverse("panic", func(c interface{}) { panic(Panic{c}) })
	addUniverse("copy", func(dst, src interface{}) int {
		return reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf(src))
	})
	addUniverse("append", p.builtinAppend)
	addUniverse("delete", func(m, k interface{}) {
		reflect.ValueOf(m).SetMapIndex(reflect.ValueOf(k), reflect.Value{})
	})
	addUniverse("make", p.builtinMake)
	addUniverse("new", p.builtinNew)
	return p
}

func (p *Program) builtinAppend(s interface{}, v ...interface{}) interface{} {
	p.builtinCalled = true
	res := reflect.ValueOf(s)
	for _, elem := range v {
		res = reflect.Append(res, reflect.ValueOf(elem))
	}
	return res.Interface()
}

func (p *Program) builtinNew(v interface{}) interface{} {
	p.builtinCalled = true
	t := v.(reflect.Type)
	return reflect.New(t).Interface()
}

func (p *Program) builtinMake(v ...interface{}) interface{} {
	p.builtinCalled = true
	t := v[0].(reflect.Type)
	switch t.Kind() {
	case reflect.Chan:
		panic("TODO make Chan")
	case reflect.Slice:
		var slen, scap int
		if len(v) > 1 {
			slen = v[1].(int)
		}
		if len(v) > 2 {
			scap = v[2].(int)
		} else {
			scap = slen
		}
		return reflect.MakeSlice(t, slen, scap).Interface()
	case reflect.Map:
		return reflect.MakeMap(t).Interface()
	}
	return nil
}

func (p *Program) Environ() *environ.Environ {
	return p.Universe.Lookup("env").Interface().(*environ.Environ)
}

func (p *Program) Alias() *environ.Environ {
	return p.Universe.Lookup("alias").Interface().(*environ.Environ)
}

// Get is part of the implementation of shell.Params.
func (p *Program) Get(name string) string {
	v := p.Cur.Lookup(name)
	if v == (reflect.Value{}) {
		return p.Environ().Get(name)
	}
	vi := v.Interface()
	if s, ok := vi.(string); ok {
		return s
	}
	return fmt.Sprint(vi)
}

// Set is part of the implementation of shell.Params.
func (p *Program) Set(name, value string) {
	s := &Scope{
		Parent:   p.Cur,
		VarName:  name,
		Var:      reflect.ValueOf(value),
		Implicit: true,
	}
	p.Cur = s
}

func (p *Program) Eval(s stmt.Stmt) (res []reflect.Value, err error) {
	defer func() {
		x := recover()
		if x == nil {
			return
		}
		switch p := x.(type) {
		case interpPanic:
			err = p.reason
			return
		case Panic:
			err = p
			return
		default:
			//panic(x)
			err = fmt.Errorf("ng eval panic: %v", x)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			debug.PrintStack()
			res = nil
		}
	}()

	p.Types.Errs = p.Types.Errs[:0]
	p.Types.Add(s)
	if len(p.Types.Errs) > 0 {
		return nil, fmt.Errorf("typecheck: %v\n", p.Types.Errs[0])
	}

	p.Returning = false
	res = p.evalStmt(s)
	return res, nil
}

func (p *Program) pushScope() {
	p.Cur = &Scope{
		Parent: p.Cur,
	}
}

func (p *Program) popScope() {
	for p.Cur.Implicit {
		p.Cur = p.Cur.Parent
	}
	p.Cur = p.Cur.Parent
}

func (p *Program) evalStmt(s stmt.Stmt) []reflect.Value {
	switch s := s.(type) {
	case *stmt.Assign:
		types := make([]tipe.Type, 0, len(s.Left))
		vals := make([]reflect.Value, 0, len(s.Left))
		for _, rhs := range s.Right {
			v := p.evalExpr(rhs)
			t := p.Types.Types[rhs]
			if len(v) > 1 {
				types = append(types, t.(*tipe.Tuple).Elems...)
			} else {
				types = append(types, t)
			}
			// TODO: insert an implicit interface type conversion here
			vals = append(vals, v...)
		}

		vars := make([]reflect.Value, len(s.Left))
		if s.Decl {
			for i, lhs := range s.Left {
				t := p.reflector.ToRType(types[i])
				s := &Scope{
					Parent:   p.Cur,
					VarName:  lhs.(*expr.Ident).Name,
					Var:      reflect.New(t).Elem(),
					Implicit: true,
				}
				p.Cur = s
				vars[i] = s.Var
			}
		} else {
			for i, lhs := range s.Left {
				if e, isIndex := lhs.(*expr.Index); isIndex {
					if _, isMap := tipe.Underlying(p.Types.Types[e.Left]).(*tipe.Map); isMap {
						container := p.evalExprOne(e.Left)
						k := p.evalExprOne(e.Indicies[0])
						if env, ok := container.Interface().(evalMap); ok {
							env.SetVal(k.String(), vals[i].String())
						} else {
							container.SetMapIndex(k, vals[i])
						}
						continue
					}
				}
				v := p.evalExprOne(lhs)
				vars[i] = v
			}
		}

		for i := range vars {
			if vars[i].IsValid() {
				vars[i].Set(vals[i])
			}
		}

		// Dynamic elision of final error.
		// TODO: move into Call case? Would miss Shell case.
		// But we need to get g(elidedErrorFunc()).
		isLastError := false
		if len(s.Right) == 1 {
			isLastError = isError(types[len(types)-1])
		}
		if isLastError && len(vars) == len(vals)-1 {
			// last error is ignored, panic if non-nil
			errVal := vals[len(vals)-1]
			if errVal.IsValid() && errVal.Interface() != nil {
				panic(Panic{val: errVal.Interface()})
			}
		}
		return nil
	case *stmt.Block:
		p.pushScope()
		defer p.popScope()
		for _, s := range s.Stmts {
			res := p.evalStmt(s)
			if p.Returning || p.Breaking {
				return res
			}
		}
		return nil
	case *stmt.For:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			p.evalStmt(s.Init)
		}
		for {
			if s.Cond != nil {
				cond := p.evalExprOne(s.Cond)
				if cond.Kind() == reflect.Bool && !cond.Bool() {
					break
				}
			}
			p.evalStmt(s.Body)
			if p.Returning {
				break
			}
			if p.Breaking {
				p.Breaking = false // TODO: break label
				break
			}
			if s.Post != nil {
				p.evalStmt(s.Post)
			}
		}
		return nil
	case *stmt.If:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			p.evalStmt(s.Init)
		}
		cond := p.evalExprOne(s.Cond)
		if cond.Kind() == reflect.Bool && cond.Bool() {
			return p.evalStmt(s.Body)
		} else if s.Else != nil {
			return p.evalStmt(s.Else)
		}
		return nil
	case *stmt.Import:
		//typ := p.Types.Lookup(s.Name).Type.(*tipe.Package)
		p.Cur = &Scope{
			Parent:   p.Cur,
			VarName:  s.Name,
			Var:      reflect.ValueOf(gowrap.Pkgs[s.Name]),
			Implicit: true,
		}
		return nil
	case *stmt.Range:
		p.pushScope()
		defer p.popScope()
		var key, val reflect.Value
		if s.Decl {
			if s.Key != nil {
				key = reflect.New(p.reflector.ToRType(p.Types.Types[s.Key])).Elem()
				name := s.Key.(*expr.Ident).Name
				p.Cur = &Scope{
					Parent:   p.Cur,
					VarName:  name,
					Var:      key,
					Implicit: true,
				}
			}
			if s.Val != nil {
				val = reflect.New(p.reflector.ToRType(p.Types.Types[s.Val])).Elem()
				name := s.Val.(*expr.Ident).Name
				p.Cur = &Scope{
					Parent:   p.Cur,
					VarName:  name,
					Var:      val,
					Implicit: true,
				}
			}
		} else {
			key = p.evalExprOne(s.Key)
			val = p.evalExprOne(s.Val)
		}
		src := p.evalExprOne(s.Expr)
		switch src.Kind() {
		case reflect.Slice:
			slen := src.Len()
			for i := 0; i < slen; i++ {
				key.SetInt(int64(i))
				if val != (reflect.Value{}) {
					val.Set(src.Index(i))
				}
				p.evalStmt(s.Body)
				if p.Returning {
					break
				}
				if p.Breaking {
					p.Breaking = false // TODO: break label
					break
				}
			}
		case reflect.Map:
			keys := src.MapKeys()
			for _, k := range keys {
				key.Set(k)
				v := src.MapIndex(key)
				val.Set(v)
				p.evalStmt(s.Body)
				if p.Returning {
					break
				}
				if p.Breaking {
					p.Breaking = false // TODO: break label
					break
				}
			}
		default:
			panic(interpPanic{fmt.Errorf("unknown range type: %T", src)})
		}
		return nil
	case *stmt.Return:
		var res []reflect.Value
		for _, expr := range s.Exprs {
			res = append(res, p.evalExpr(expr)...)
		}
		p.Returning = true
		return res
	case *stmt.Simple:
		res := p.evalExpr(s.Expr)
		if fn, isFunc := s.Expr.(*expr.FuncLiteral); isFunc && fn.Name != "" {
			s := &Scope{
				Parent:   p.Cur,
				VarName:  fn.Name,
				Var:      res[0],
				Implicit: true,
			}
			p.Cur = s
		}
		return res
	case *stmt.TypeDecl:
		return nil
	case *stmt.MethodikDecl:
		return nil
	}
	panic(fmt.Sprintf("TODO evalStmt: %T: %s", s, s.Sexp()))
}

func (p *Program) evalExprOne(e expr.Expr) reflect.Value {
	v := p.evalExpr(e)
	if len(v) != 1 {
		panic(interpPanic{fmt.Errorf("expression returns %d values instead of 1", len(v))})
	}
	return v[0]
}

type (
	untypedInt   struct{ *big.Int }
	untypedFloat struct{ *big.Float }
)

func convert(v reflect.Value, t reflect.Type) reflect.Value {
	if v.Type() == t {
		return v
	}
	//fmt.Printf("convert(%s -> %s)\n", v.Type(), t)
	switch val := v.Interface().(type) {
	case untypedInt:
		if t == reflect.TypeOf(untypedFloat{}) {
			res := untypedFloat{new(big.Float)}
			res.Float.SetInt64(val.Int64())
			return reflect.ValueOf(res)
		}
		ret := reflect.New(t).Elem()
		switch t.Kind() {
		case reflect.Interface:
			ret.Set(reflect.ValueOf(int(val.Int64())))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			ret.SetUint(val.Uint64())
		default:
			ret.SetInt(val.Int64())
		}
		return ret
	case untypedFloat:
		ret := reflect.New(t).Elem()
		f, _ := val.Float64()
		if t.Kind() == reflect.Interface {
			ret.Set(reflect.ValueOf(float64(f)))
		} else {
			ret.SetFloat(f)
		}
		return ret
	default:
		panic(fmt.Sprintf("TODO convert(%v, %s)", v, t))
	}
}

type interpPanic struct {
	reason error
}

func (p *Program) evalExpr(e expr.Expr) []reflect.Value {
	switch e := e.(type) {
	case *expr.BasicLiteral:
		var v reflect.Value
		switch val := e.Value.(type) {
		case *big.Int:
			v = reflect.ValueOf(untypedInt{val})
		case *big.Float:
			v = reflect.ValueOf(untypedFloat{val})
		default:
			v = reflect.ValueOf(val)
		}
		t := p.reflector.ToRType(p.Types.Types[e])
		return []reflect.Value{convert(v, t)}
	case *expr.Binary:
		lhs := p.evalExpr(e.Left)
		switch e.Op {
		case token.LogicalAnd:
			v := lhs[0].Interface()
			if !v.(bool) {
				return []reflect.Value{reflect.ValueOf(false)}
			}
			rhs := p.evalExpr(e.Right)
			v = rhs[0].Interface()
			return []reflect.Value{reflect.ValueOf(v)}
		case token.LogicalOr:
			v := lhs[0].Interface()
			if v.(bool) {
				return []reflect.Value{reflect.ValueOf(true)}
			}
			rhs := p.evalExpr(e.Right)
			v = rhs[0].Interface()
			return []reflect.Value{reflect.ValueOf(v)}
		}
		rhs := p.evalExpr(e.Right)
		x := lhs[0].Interface()
		y := rhs[0].Interface()
		v, err := binOp(e.Op, x, y)
		if err != nil {
			panic(interpPanic{err})
		}
		t := p.reflector.ToRType(p.Types.Types[e])
		return []reflect.Value{convert(reflect.ValueOf(v), t)}
	case *expr.Call:
		fn := p.evalExprOne(e.Func)
		args := make([]reflect.Value, len(e.Args))
		for i, arg := range e.Args {
			v := p.evalExprOne(arg)
			if t := fn.Type(); t.Kind() == reflect.Func && !t.IsVariadic() {
				// Implicit interface conversion on use.
				// Bonus: this makes up for the fact that the
				// evaluator currently stores custom ng
				// interfaces in a Go empty interface{}.
				argt := fn.Type().In(i)
				if argt.Kind() == reflect.Interface && argt != v.Type() {
					underlying := reflect.ValueOf(v.Interface())
					v = reflect.New(argt).Elem()
					v.Set(underlying) // re-box with right type
				}
			}
			args[i] = v
		}
		if t, isTypeConv := fn.Interface().(reflect.Type); isTypeConv {
			return []reflect.Value{typeConv(t, args[0])}
		}
		var t []reflect.Type
		switch resTyp := p.Types.Types[e].(type) {
		case *tipe.Tuple:
			t = make([]reflect.Type, len(resTyp.Elems))
			for i, elemTyp := range resTyp.Elems {
				t[i] = p.reflector.ToRType(elemTyp)
			}
		default:
			t = []reflect.Type{p.reflector.ToRType(resTyp)}
		}
		// TODO: have typecheck do the error elision for us
		// so we can insert the dynamic panic check once, right here.
		res := fn.Call(args)
		if p.builtinCalled {
			p.builtinCalled = false
			fmt.Printf("have builtin make\n")
			for i := range res {
				// Necessary to turn the return type of append
				// from an interface{} into a slice so it can
				// be set.
				res[i] = reflect.ValueOf(res[i].Interface())
			}
		}
		return res
	case *expr.CompLiteral:
		t := p.reflector.ToRType(e.Type)
		switch t.Kind() {
		case reflect.Struct:
			st := reflect.New(t).Elem()
			if len(e.Keys) > 0 {
				for i, elem := range e.Elements {
					name := e.Keys[i].(*expr.Ident).Name
					v := p.evalExprOne(elem)
					st.FieldByName(name).Set(v)
				}
			} else {
				for i, expr := range e.Elements {
					v := p.evalExprOne(expr)
					st.Field(i).Set(v)
				}
			}
			return []reflect.Value{st}
		case reflect.Map:
			panic("TODO CompLiteral map")
		}
	case *expr.FuncLiteral:
		s := &Scope{
			Parent: p.Universe,
		}
		for _, name := range e.Type.FreeVars {
			if s.VarName != "" {
				s = &Scope{Parent: s}
			}
			s.VarName = name
			s.Var = p.Cur.Lookup(name)
		}
		// TODO for _, mdik := range e.Type.FreeMdik

		t := p.reflector.ToRType(e.Type)
		fn := reflect.MakeFunc(t, func(args []reflect.Value) (results []reflect.Value) {
			p := &Program{
				Universe:  p.Universe,
				Types:     p.Types, // TODO race cond, clone type list
				Cur:       s,
				reflector: p.reflector,
			}
			p.pushScope()
			defer p.popScope()
			for i, name := range e.ParamNames {
				p.Cur = &Scope{
					Parent:   p.Cur,
					VarName:  name,
					Var:      args[i],
					Implicit: true,
				}
			}
			res := p.evalStmt(e.Body.(*stmt.Block))
			return res
		})
		return []reflect.Value{fn}
	case *expr.Ident:
		if e.Name == "nil" { // TODO: make sure it's the Universe nil
			t := p.reflector.ToRType(p.Types.Types[e])
			fmt.Printf("nil has type %v (from %s)\n", t, p.Types.Types[e])
			return []reflect.Value{reflect.New(t).Elem()}
		}
		if v := p.Cur.Lookup(e.Name); v != (reflect.Value{}) {
			return []reflect.Value{v}
		}
		t := p.Types.Types[e]
		if t != nil {
			return []reflect.Value{reflect.ValueOf(p.reflector.ToRType(p.Types.Types[e]))}
		}
		panic(interpPanic{fmt.Errorf("eval: undefined identifier: %q", e.Name)})
	case *expr.Index:
		container := p.evalExprOne(e.Left)
		if len(e.Indicies) != 1 {
			panic(interpPanic{fmt.Errorf("eval: TODO table slicing")})
		}
		if e, isSlice := e.Indicies[0].(*expr.Slice); isSlice {
			var i, j int
			if e.Low != nil {
				i = int(p.evalExprOne(e.Low).Int())
			}
			if e.High != nil {
				j = int(p.evalExprOne(e.High).Int())
			} else {
				j = container.Len()
			}
			if e.Max != nil {
				k := int(p.evalExprOne(e.Max).Int())
				return []reflect.Value{container.Slice3(i, j, k)}
			}
			return []reflect.Value{container.Slice(i, j)}
		}
		k := p.evalExprOne(e.Indicies[0])
		if env, ok := container.Interface().(evalMap); ok {
			return []reflect.Value{reflect.ValueOf(env.GetVal(k.String()))}
		}
		switch container.Kind() {
		case reflect.Slice, reflect.String:
			i := int(k.Int())
			if int64(i) != k.Int() {
				panic(interpPanic{fmt.Errorf("eval: index too big: %d", k.Int())})
			}
			return []reflect.Value{container.Index(i)}
		case reflect.Map:
			v := container.MapIndex(k)
			exists := v != (reflect.Value{})
			if !exists {
				v = reflect.Zero(container.Type().Elem())
			}
			if t, returnExists := p.Types.Types[e].(*tipe.Tuple); returnExists {
				// TODO: type checker is not generating this tuple yet.
				fmt.Printf("index t=%v\n", t)
				return []reflect.Value{v, reflect.ValueOf(exists)}
			}
			return []reflect.Value{v}
		case reflect.Ptr:
			panic(interpPanic{fmt.Errorf("eval: *expr.Index unsupported ptr kind: %v", container.Elem().Kind())})
		default:
			panic(interpPanic{fmt.Errorf("eval: *expr.Index unsupported kind: %v", container.Kind())})
		}
	case *expr.MapLiteral:
		t := p.reflector.ToRType(e.Type)
		m := reflect.MakeMap(t)
		for i, kexpr := range e.Keys {
			k := p.evalExprOne(kexpr)
			v := p.evalExprOne(e.Values[i])
			m.SetMapIndex(k, v)
		}
		return []reflect.Value{m}
	case *expr.Selector:
		lhs := p.evalExprOne(e.Left)
		if pkg, ok := lhs.Interface().(*gowrap.Pkg); ok {
			return []reflect.Value{pkg.Exports[e.Right.Name]}
		}
		v := lhs.MethodByName(e.Right.Name)
		if v == (reflect.Value{}) && lhs.Kind() != reflect.Ptr {
			v = lhs.Addr().MethodByName(e.Right.Name)
		}
		if v == (reflect.Value{}) && lhs.Kind() == reflect.Struct {
			v = lhs.FieldByName(e.Right.Name)
		}
		return []reflect.Value{v}
	case *expr.Shell:
		p.pushScope()
		defer p.popScope()
		res := make(chan string)
		out := os.Stdout
		if e.DropOut {
			out = devNull
			close(res)
		} else if e.TrapOut {
			r, w, err := os.Pipe()
			if err != nil {
				panic(err)
			}
			out = w
			go func() {
				b, err := ioutil.ReadAll(r)
				if err != nil {
					panic(err)
				}
				res <- string(b)
			}()
		} else {
			close(res)
		}
		var err error
		for _, cmd := range e.Cmds {
			j := &shell.Job{
				Cmd:    cmd,
				Params: p,
				Stdin:  os.Stdin,
				Stdout: out,
				Stderr: os.Stderr,
			}
			if err = j.Start(); err != nil {
				break
			}
			var done bool
			done, err = j.Wait()
			if err != nil {
				break
			}
			if !done {
				break // TODO not right, instead we should just have one cmd, not Cmds here.
			}
		}
		if e.TrapOut {
			out.Close()
		}
		str := reflect.ValueOf(<-res)
		if err != nil {
			fmt.Printf("shell err: %v\n", err)
			return []reflect.Value{str, reflect.ValueOf(err)}
		}
		errt := reflect.TypeOf((*error)(nil)).Elem()
		nilerr := reflect.New(errt).Elem()
		return []reflect.Value{str, nilerr}
	case *expr.SliceLiteral:
		t := p.reflector.ToRType(e.Type)
		slice := reflect.MakeSlice(t, len(e.Elems), len(e.Elems))
		for i, elem := range e.Elems {
			v := p.evalExprOne(elem)
			slice.Index(i).Set(v)
		}
		return []reflect.Value{slice}
	case *expr.Type:
		t := p.reflector.ToRType(e.Type)
		return []reflect.Value{reflect.ValueOf(t)}
	case *expr.Unary:
		var v reflect.Value
		switch e.Op {
		case token.LeftParen:
			v = p.evalExprOne(e.Expr)
		case token.Ref:
			v = p.evalExprOne(e.Expr)
			return []reflect.Value{v.Addr()}
		case token.Mul: // deref
			v := p.evalExprOne(e.Expr)
			return []reflect.Value{v.Elem()}
		case token.Not:
			v = p.evalExprOne(e.Expr)
			v.SetBool(!v.Bool())
		case token.Sub:
			rhs := p.evalExprOne(e.Expr)
			var lhs interface{}
			switch rhs.Interface().(type) {
			case int:
				lhs = int(0)
			case int64:
				lhs = int64(0)
			case float32:
				lhs = float32(0)
			case float64:
				lhs = float64(0)
			case untypedInt:
				lhs = untypedInt{big.NewInt(0)}
			case untypedFloat:
				lhs = untypedFloat{big.NewFloat(0)}
			}
			res, err := binOp(token.Sub, lhs, rhs.Interface())
			if err != nil {
				panic(interpPanic{err})
			}
			v = reflect.ValueOf(res)
		}
		t := p.reflector.ToRType(p.Types.Types[e])
		return []reflect.Value{convert(v, t)}
	}
	panic(interpPanic{fmt.Errorf("TODO evalExpr(%s), %T", e.Sexp(), e)})
}

// TODO make thread safe
type reflector struct {
	fwd map[tipe.Type]reflect.Type
	rev map[reflect.Type]tipe.Type
}

func newReflector() *reflector {
	return &reflector{
		fwd: make(map[tipe.Type]reflect.Type),
		rev: make(map[reflect.Type]tipe.Type),
	}
}

func (r *reflector) ToRType(t tipe.Type) reflect.Type {
	if t == nil {
		return nil
	}

	rtype := r.fwd[t]
	if rtype != nil {
		return rtype
	}
	switch t := t.(type) {
	case tipe.Basic:
		switch t {
		case tipe.Invalid:
			return nil
		case tipe.Num:
			panic("TODO rtype for Num")
		case tipe.Bool:
			rtype = reflect.TypeOf(false)
		case tipe.Byte:
			rtype = reflect.TypeOf(byte(0))
		case tipe.Rune:
			rtype = reflect.TypeOf(rune(0))
		case tipe.Integer:
			rtype = reflect.TypeOf((*big.Int)(nil))
		case tipe.Float:
			rtype = reflect.TypeOf((*big.Float)(nil))
		case tipe.Complex:
			panic("TODO rtype for Complex")
		case tipe.String:
			rtype = reflect.TypeOf("")
		case tipe.Int:
			rtype = reflect.TypeOf(int(0))
		case tipe.Int8:
			rtype = reflect.TypeOf(int8(0))
		case tipe.Int16:
			rtype = reflect.TypeOf(int16(0))
		case tipe.Int32:
			rtype = reflect.TypeOf(int32(0))
		case tipe.Int64:
			rtype = reflect.TypeOf(int64(0))
		case tipe.Uint:
			rtype = reflect.TypeOf(uint(0))
		case tipe.Uint8:
			rtype = reflect.TypeOf(uint8(0))
		case tipe.Uint16:
			rtype = reflect.TypeOf(uint16(0))
		case tipe.Uint32:
			rtype = reflect.TypeOf(uint32(0))
		case tipe.Uint64:
			rtype = reflect.TypeOf(uint64(0))
		case tipe.Float32:
			rtype = reflect.TypeOf(float32(0))
		case tipe.Float64:
			rtype = reflect.TypeOf(float64(0))
		case tipe.UntypedNil:
			panic("TODO UntypedNil")
		case tipe.UntypedBool:
			panic("TODO Untyped Bool")
		case tipe.UntypedInteger:
			rtype = reflect.TypeOf(untypedInt{})
		case tipe.UntypedFloat:
			rtype = reflect.TypeOf(untypedFloat{})
		case tipe.UntypedComplex:
			panic("TODO Untyped Complex")
		}
	case *tipe.Func:
		var in, out []reflect.Type
		if t.Params != nil {
			for _, p := range t.Params.Elems {
				in = append(in, r.ToRType(p))
			}
		}
		if t.Results != nil {
			for _, p := range t.Results.Elems {
				out = append(out, r.ToRType(p))
			}
		}
		rtype = reflect.FuncOf(in, out, t.Variadic)
	case *tipe.Struct:
		var fields []reflect.StructField
		for i, f := range t.Fields {
			fields = append(fields, reflect.StructField{
				Name: t.FieldNames[i],
				Type: r.ToRType(f),
			})
		}
		rtype = reflect.StructOf(fields)
	case *tipe.Methodik:
		if t.PkgPath != "" {
			v := gowrap.Pkgs[t.PkgPath].Exports[t.Name]
			if _, isIface := tipe.Underlying(t).(*tipe.Interface); isIface {
				rtype = v.Type().Elem()
			} else {
				rtype = v.Type()
			}
		} else {
			panic("TODO unnamed Methodik")
		}
	case *tipe.Slice:
		rtype = reflect.SliceOf(r.ToRType(t.Elem))
	// TODO case *Table:
	case *tipe.Pointer:
		rtype = reflect.PtrTo(r.ToRType(t.Elem))
	case *tipe.Map:
		rtype = reflect.MapOf(r.ToRType(t.Key), r.ToRType(t.Value))
	// TODO case *Interface:
	// TODO need more reflect support, MakeInterface
	// TODO needs reflect.InterfaceOf
	//case *Tuple:
	//case *Package:
	default:
		if typecheck.IsError(t) {
			rtype = reflect.TypeOf((*error)(nil)).Elem()
		} else {
			rtype = reflect.TypeOf((*interface{})(nil)).Elem()
			//panic(fmt.Sprintf("cannot make rtype of %s", t.Sexp()))
		}
	}
	r.fwd[t] = rtype
	return rtype
}

func (r *reflector) FromRType(rtype reflect.Type) tipe.Type {
	if t := r.rev[rtype]; t != nil {
		return t
	}
	var t tipe.Type // TODO
	r.rev[rtype] = t
	return t
}

func isError(t tipe.Type) bool {
	return typecheck.Universe.Objs["error"].Type == t
}

type Panic struct {
	val interface{}
}

func (p Panic) Error() string {
	return fmt.Sprintf("neugram panic: %v", p.val)
}

var devNull *os.File

func init() {
	var err error
	devNull, err = os.Open("/dev/null")
	if err != nil {
		panic(err)
	}
}
