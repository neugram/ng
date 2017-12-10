// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"neugram.io/ng/eval/environ"
	"neugram.io/ng/eval/gowrap"
	"neugram.io/ng/eval/gowrap/genwrap"
	_ "neugram.io/ng/eval/gowrap/wrapbuiltin" // registers with gowrap
	"neugram.io/ng/eval/shell"
	"neugram.io/ng/format"
	"neugram.io/ng/internal/bigcplx"
	"neugram.io/ng/parser"
	"neugram.io/ng/syntax/expr"
	"neugram.io/ng/syntax/stmt"
	"neugram.io/ng/syntax/tipe"
	"neugram.io/ng/syntax/token"
	"neugram.io/ng/typecheck"
)

type Scope struct {
	Parent  *Scope
	VarName string
	Var     reflect.Value

	// Implicit is set if the Scope was created mid block and should be
	// unrolled when block ends.
	Implicit bool

	Label bool
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
	Cur       *Scope
	Types     *typecheck.Checker
	Pkgs      map[string]*gowrap.Pkg
	Path      string
	reflector *reflector

	ShellState *shell.State

	sigint     <-chan os.Signal
	sigintSeen bool

	branchType      branchType
	branchLabel     string
	mostRecentLabel string

	// builtinCalled is set by any builtin function that has
	// a generic return type. The intepreter has to unbox the
	// return type.
	builtinCalled bool

	tempdir string
}

type branchType int

const (
	brNone = branchType(iota)
	brBreak
	brContinue
	brGoto
	brFallthrough
	brReturn
)

type evalMap interface {
	GetVal(key interface{}) interface{}
	SetVal(key, val interface{})
}

func New(path string, shellState *shell.State) *Program {
	if shellState == nil {
		shellState = &shell.State{
			Env:   environ.New(),
			Alias: environ.New(),
		}
	}
	universe := new(Scope)
	p := &Program{
		Universe: universe,
		Types:    typecheck.New(path),
		Pkgs:     make(map[string]*gowrap.Pkg),
		Path:     path,
		Cur: &Scope{
			Parent: universe,
		},
		ShellState: shellState,
		reflector:  newReflector(),
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
	addUniverse("env", (evalMap)(p.ShellState.Env))
	addUniverse("alias", (evalMap)(p.ShellState.Alias))
	addUniverse("nil", nil)
	addUniverse("print", func(val ...interface{}) {
		fmt.Println(val...)
	})
	addUniverse("printf", func(format string, val ...interface{}) {
		fmt.Printf(format, val...)
	})
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
	addUniverse("panic", func(c interface{}) {
		c = promoteUntyped(c)
		panic(Panic{c})
	})
	addUniverse("close", func(ch interface{}) {
		rv := reflect.ValueOf(ch)
		rv.Close()
	})
	addUniverse("copy", func(dst, src interface{}) int {
		src = promoteUntyped(src)
		return reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf(src))
	})
	addUniverse("append", p.builtinAppend)
	addUniverse("delete", func(m, k interface{}) {
		k = promoteUntyped(k)
		reflect.ValueOf(m).SetMapIndex(reflect.ValueOf(k), reflect.Value{})
	})
	addUniverse("make", p.builtinMake)
	addUniverse("new", p.builtinNew)
	addUniverse("complex", p.builtinComplex)
	addUniverse("real", func(v interface{}) interface{} {
		p.builtinCalled = true
		switch v := v.(type) {
		case UntypedComplex:
			// FIXME: return UntypedFloat instead
			// to handle: imag(1+2i) + float32(2.3)
			// re := UntypedFloat{big.NewFloat(0).Set(v.Real)}
			re, _ := v.Real.Float64()
			return re
		case complex64:
			return real(v)
		case complex128:
			return real(v)
		}
		panic(fmt.Errorf("invalid type real(%T)", v))
	})
	addUniverse("imag", func(v interface{}) interface{} {
		p.builtinCalled = true
		switch v := v.(type) {
		case UntypedComplex:
			// FIXME: return UntypedFloat instead.
			// to handle: imag(1+2i) + float32(2.3)
			// im := UntypedFloat{big.NewFloat(0).Set(v.Imag)}
			im, _ := v.Imag.Float64()
			return im
		case complex64:
			return imag(v)
		case complex128:
			return imag(v)
		}
		panic(fmt.Errorf("invalid type imag(%T)", v))
	})
	return p
}

func EvalFile(path string, shellState *shell.State) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("eval: %v", err)
	}
	p := New(path, shellState)
	return p.evalFile()
}

func (p *Program) evalFile() error {
	prsr := parser.New(p.Path)
	f, err := os.Open(p.Path)
	if err != nil {
		return fmt.Errorf("eval: %v", err)
	}
	defer f.Close()

	// TODO: position information in the parser will replace i.
	scanner := bufio.NewScanner(f)
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Bytes()
		res := prsr.ParseLine(line)
		if len(res.Errs) > 0 {
			return fmt.Errorf("%d: %v", i+1, res.Errs[0])
		}
		for _, s := range res.Stmts {
			if _, err := p.Eval(s, p.sigint); err != nil {
				if _, isPanic := err.(Panic); isPanic {
					return err
				}
				return fmt.Errorf("%d: %v", i+1, err)
			}
		}
		for _, cmd := range res.Cmds {
			j := &shell.Job{
				State:  p.ShellState,
				Cmd:    cmd,
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			}
			if err := j.Start(); err != nil {
				return err
			}
			done, err := j.Wait()
			if err != nil {
				return err
			}
			if !done {
				break // TODO not right, instead we should just have one cmd, not Cmds here.
			}
		}
	}
	return nil

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
		size := 0
		if len(v) > 1 {
			size = v[1].(int)
		}
		return reflect.MakeChan(t, size).Interface()
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

func (p *Program) builtinComplex(re, im interface{}) interface{} {
	p.builtinCalled = true
	switch re := re.(type) {
	case UntypedInt:
		switch im := im.(type) {
		case UntypedInt:
			return complex(float64(re.Int64()), float64(im.Int64()))
		case UntypedFloat:
			f, _ := im.Float64()
			return complex(float64(re.Int64()), f)
		case float32:
			return complex(float32(re.Int64()), im)
		case float64:
			return complex(float64(re.Int64()), im)
		}
	case UntypedFloat:
		switch im := im.(type) {
		case UntypedInt:
			fre, _ := re.Float64()
			fim := float64(im.Int64())
			return complex(fre, fim)
		case UntypedFloat:
			fre, _ := re.Float64()
			fim, _ := im.Float64()
			return complex(fre, fim)
		case float32:
			fre, _ := re.Float64()
			return complex(float32(fre), float32(im))
		case float64:
			fre, _ := re.Float64()
			return complex(fre, im)
		}
	case float32:
		switch im := im.(type) {
		case UntypedInt:
			fim := float32(im.Int64())
			return complex(re, fim)
		case UntypedFloat:
			fim, _ := im.Float64()
			return complex(re, float32(fim))
		case float32:
			return complex(re, im)
		case float64:
			panic("impossible")
		}
	case float64:
		switch im := im.(type) {
		case UntypedInt:
			fim := float64(im.Int64())
			return complex(re, fim)
		case UntypedFloat:
			fim, _ := im.Float64()
			return complex(re, fim)
		case float32:
			panic("impossible")
		case float64:
			return complex(re, im)
		}
	}
	panic(fmt.Errorf("invalid types %T,%T", re, im))
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

func (p *Program) interrupted() bool {
	if p.sigintSeen {
		return true
	}
	select {
	case <-p.sigint:
		p.sigintSeen = true
		return true
	default:
		return false
	}
}

var nosig = (<-chan os.Signal)(make(chan os.Signal))

func (p *Program) Eval(s stmt.Stmt, sigint <-chan os.Signal) (res []reflect.Value, err error) {
	if sigint != nil {
		p.sigint = sigint
	} else {
		p.sigint = nosig
	}
	defer func() {
		p.sigint = nosig
		p.sigintSeen = false
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

	p.Types.Add(s)
	if errs := p.Types.Errs(); len(errs) > 0 {
		// Friendly interactive shell error messages.
		if s, isSimple := s.(*stmt.Simple); isSimple {
			if e, isIdent := s.Expr.(*expr.Ident); isIdent && (e.Name == "exit" || e.Name == "logout") {
				if p.Cur.Lookup(e.Name) == (reflect.Value{}) {
					return nil, fmt.Errorf("use Ctrl-D to exit")
				}
			}
		}

		return nil, fmt.Errorf("typecheck: %v\n", errs[0])
	}

	p.branchType = brNone
	p.branchLabel = ""
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
	mostRecentLabel := p.mostRecentLabel
	p.mostRecentLabel = ""
	switch s := s.(type) {
	case *stmt.Const:
		return p.evalConst(s)
	case *stmt.ConstSet:
		for _, v := range s.Consts {
			p.evalConst(v)
		}
		return nil
	case *stmt.Var:
		return p.evalVar(s)
	case *stmt.VarSet:
		for _, v := range s.Vars {
			p.evalVar(v)
		}
		return nil
	case *stmt.Assign:
		types := make([]tipe.Type, 0, len(s.Left))
		vals := make([]reflect.Value, 0, len(s.Left))
		for _, rhs := range s.Right {
			v := p.evalExpr(rhs)
			t := p.Types.Type(rhs)
			if tuple, isTuple := t.(*tipe.Tuple); isTuple {
				types = append(types, tuple.Elems...)
			} else {
				types = append(types, t)
			}
			// TODO: insert an implicit interface type conversion here
			vals = append(vals, v...)
		}

		vars := make([]reflect.Value, len(s.Left))
		if s.Decl {
			for i, lhs := range s.Left {
				if lhs.(*expr.Ident).Name == "_" {
					continue
				}
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
				if e, isIdent := lhs.(*expr.Ident); isIdent && e.Name == "_" {
					continue
				}
				if e, isIndex := lhs.(*expr.Index); isIndex {
					if _, isMap := tipe.Underlying(p.Types.Type(e.Left)).(*tipe.Map); isMap {
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

		return nil
	case *stmt.Block:
		p.pushScope()
		defer p.popScope()
		for _, s := range s.Stmts {
			res := p.evalStmt(s)
			if p.branchType != brNone || p.interrupted() {
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
	loop:
		for {
			if s.Cond != nil {
				cond := p.evalExprOne(s.Cond)
				if cond.Kind() == reflect.Bool && !cond.Bool() {
					break
				}
			}
			p.evalStmt(s.Body)
			// Note there are three extremely similar loops:
			//	*stmt.For, *stmt.Range (slice, and map)
			if p.interrupted() {
				break
			}
			switch p.branchType {
			default:
				break loop
			case brNone:
			case brBreak:
				if p.branchLabel == mostRecentLabel {
					p.branchType = brNone
					p.branchLabel = ""
				}
				break loop
			case brContinue:
				if p.branchLabel == mostRecentLabel {
					p.branchType = brNone
					p.branchLabel = ""
					if s.Post != nil {
						p.evalStmt(s.Post)
					}
					continue loop
				}
				break loop
			}
			if s.Post != nil {
				p.evalStmt(s.Post)
			}
		}
		return nil
	case *stmt.Go:
		fn, args := p.prepCall(s.Call)
		for i, arg := range args {
			v := reflect.New(arg.Type()).Elem()
			v.Set(arg)
			args[i] = v
		}
		go fn.Call(args)
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
	case *stmt.ImportSet:
		for _, imp := range s.Imports {
			p.evalStmt(imp)
		}
		return nil
	case *stmt.Import:
		var pkg *gowrap.Pkg
		if strings.HasSuffix(s.Path, ".ng") {
			path := filepath.Join(filepath.Dir(p.Path), s.Path)
			pkg = p.Pkgs[path]
			if pkg == nil {
				typ := p.Types.Pkg(path)
				if typ == nil {
					panic(Panic{val: fmt.Errorf("cannot find package typechecking: %v", path)})
				}
				oldPath := p.Path
				oldCur := p.Cur
				oldTypes := p.Types
				p.Path = path
				p.Cur = p.Universe
				p.Types = oldTypes.NewScope()
				func() {
					defer func() {
						p.Path = oldPath
						p.Cur = oldCur
						p.Types = oldTypes
					}()
					if err := p.evalFile(); err != nil {
						panic(Panic{val: fmt.Errorf("%s: %v", p.Path, err)})
					}

					pkg = &gowrap.Pkg{Exports: make(map[string]reflect.Value)}
					for p.Cur != p.Universe {
						pkg.Exports[p.Cur.VarName] = p.Cur.Var
						p.Cur = p.Cur.Parent
					}
				}()
				p.Pkgs[path] = pkg
			}
		} else {
			pkg = gowrap.Pkgs[s.Path]
			if pkg == nil {
				// TODO: go install pkg before genwrap for update importer?
				src, err := genwrap.GenGo(s.Path, "main", false)
				if err != nil {
					panic(Panic{val: fmt.Errorf("plugin: wrapper gen failed for Go package %q: %v", s.Name, err)})
				}
				if p.tempdir == "" {
					p.tempdir, err = ioutil.TempDir("", "ng-tmp-")
					if err != nil {
						panic(Panic{val: err})
					}
				}
				name := "ng-plugin-" + strings.Replace(s.Path, "/", "_", -1) + ".go"
				err = ioutil.WriteFile(filepath.Join(p.tempdir, name), src, 0666)
				if err != nil {
					panic(Panic{val: err})
				}
				cmd := exec.Command("go", "build", "-buildmode=plugin", "-i", name)
				cmd.Dir = p.tempdir
				out, err := cmd.CombinedOutput()
				if err != nil {
					panic(Panic{val: fmt.Errorf("plugin: building wrapper failed for Go package %q: %v\n%s", s.Name, err, out)})
				}
				pluginName := name[:len(name)-3] + ".so"
				_, err = plugin.Open(filepath.Join(p.tempdir, pluginName))
				if err != nil {
					panic(Panic{val: fmt.Errorf("plugin: failed to open Go package %q: %v", s.Name, err)})
				}
				pkg = gowrap.Pkgs[s.Path]
				if pkg == nil {
					panic(Panic{val: fmt.Errorf("plugin: contents missing from Go package %q", s.Name)})
				}
			}
		}
		p.Cur = &Scope{
			Parent:   p.Cur,
			VarName:  s.Name,
			Var:      reflect.ValueOf(pkg),
			Implicit: true,
		}
		return nil
	case *stmt.Range:
		p.pushScope()
		defer p.popScope()
		var key, val reflect.Value
		if s.Decl {
			if s.Key != nil {
				key = reflect.New(p.reflector.ToRType(p.Types.Type(s.Key))).Elem()
				name := s.Key.(*expr.Ident).Name
				p.Cur = &Scope{
					Parent:   p.Cur,
					VarName:  name,
					Var:      key,
					Implicit: true,
				}
			}
			if s.Val != nil {
				val = reflect.New(p.reflector.ToRType(p.Types.Type(s.Val))).Elem()
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
		case reflect.Array, reflect.Slice:
			slen := src.Len()
		sliceLoop:
			for i := 0; i < slen; i++ {
				key.SetInt(int64(i))
				if val != (reflect.Value{}) {
					val.Set(src.Index(i))
				}
				p.evalStmt(s.Body)
				if p.interrupted() {
					break
				}
				switch p.branchType {
				default:
					break sliceLoop
				case brNone:
				case brBreak:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
					}
					break sliceLoop
				case brContinue:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
						continue sliceLoop
					}
					break sliceLoop
				}
			}
		case reflect.Map:
			keys := src.MapKeys()
		mapLoop:
			for _, k := range keys {
				key.Set(k)
				v := src.MapIndex(key)
				val.Set(v)
				p.evalStmt(s.Body)
				if p.interrupted() {
					break
				}
				switch p.branchType {
				default:
					break mapLoop
				case brNone:
				case brBreak:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
					}
					break mapLoop
				case brContinue:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
						continue mapLoop
					}
					break mapLoop
				}
			}
		case reflect.Chan:
		chanLoop:
			for {
				v, ok := src.Recv()
				if !ok {
					break chanLoop
				}
				key.Set(v)
				p.evalStmt(s.Body)
				if p.interrupted() {
					break
				}
				switch p.branchType {
				default:
					break chanLoop
				case brNone:
				case brBreak:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
					}
					break chanLoop
				case brContinue:
					if p.branchLabel == mostRecentLabel {
						p.branchType = brNone
						p.branchLabel = ""
						continue chanLoop
					}
					break chanLoop
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
		p.branchType = brReturn
		p.branchLabel = ""
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
	case *stmt.Send:
		ch := p.evalExprOne(s.Chan)
		v := p.evalExprOne(s.Value)
		ch.Send(v)
		return nil
	case *stmt.TypeDecl, *stmt.TypeDeclSet:
		return nil
	case *stmt.MethodikDecl:
		t := s.Type
		r := p.reflector

		// Hack. See the file comment in methodpool.go.
		st, isStruct := t.Type.(*tipe.Struct)
		if !isStruct {
			panic("eval only supports methodik on struct types")
		}
		var fields []reflect.StructField
		for i, name := range t.MethodNames {
			funcType := r.ToRType(t.Methods[i])
			funcImpl := p.evalFuncLiteral(s.Methods[i], t)

			embType := methodPoolAssign(name, funcType, funcImpl)
			fields = append(fields, reflect.StructField{
				Name:      embType.Name(),
				Type:      embType,
				Anonymous: true,
			})
		}
		for _, f := range st.Fields {
			fields = append(fields, reflect.StructField{
				Name: f.Name,
				Type: r.ToRType(f.Type),
			})
		}
		rtype := reflect.StructOf(fields)
		r.fwd[t] = rtype
		return nil
	case *stmt.Labeled:
		p.Cur = &Scope{
			Parent:  p.Cur,
			VarName: s.Label,
			Label:   true,
		}
		defer p.popScope()
		p.mostRecentLabel = s.Label
		return p.evalStmt(s.Stmt)
	case *stmt.Branch:
		p.branchLabel = s.Label
		switch s.Type {
		case token.Continue:
			p.branchType = brContinue
		case token.Break:
			p.branchType = brBreak
		case token.Goto:
			p.branchType = brGoto
		case token.Fallthrough:
			p.branchType = brFallthrough
		default:
			panic("bad branch type: " + s.Type.String())
		}
		return nil
	case *stmt.Switch:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			p.evalStmt(s.Init)
		}
		cond := reflect.ValueOf(true)
		if s.Cond != nil {
			cond = p.evalExprOne(s.Cond)
		}
		var (
			dflt    *stmt.SwitchCase
			match   = false
			through = false
		)
	loopCases:
		for i, cse := range s.Cases {
			if cse.Default {
				dflt = &s.Cases[i]
			}
			// only go through the evaluation of the cases when not
			// in fallthrough mode.
			if !through {
				for j := range cse.Conds {
					e := p.evalExprOne(cse.Conds[j])
					if reflect.DeepEqual(cond.Interface(), e.Interface()) {
						match = true
						break
					}
				}
			}
			if match || through {
				through = false
				p.evalStmt(cse.Body)
				switch p.branchType {
				case brFallthrough:
					through = true
					p.branchType = brNone
					continue loopCases
				}
				return nil
			}
		}
		// no case were triggered.
		// execute the default one, if any.
		if !match && dflt != nil {
			p.evalStmt(dflt.Body)
		}
		return nil
	case *stmt.TypeSwitch:
		if s.Init != nil {
			p.pushScope()
			defer p.popScope()
			p.evalStmt(s.Init)
		}
		p.pushScope()
		defer p.popScope()
		var v reflect.Value
		switch st := s.Assign.(type) {
		case *stmt.Simple:
			v = p.evalStmt(st)[0]
		case *stmt.Assign:
			p.evalStmt(st)
			v = p.Cur.Lookup(st.Left[0].(*expr.Ident).Name)
		default:
			panic(Panic{fmt.Sprintf("invalid type-switch guard type (%T)", st)})
		}
		t := reflect.TypeOf(v.Interface())
		var dflt *stmt.TypeSwitchCase
		for i, cse := range s.Cases {
			if cse.Default {
				dflt = &s.Cases[i]
				continue
			}
			for _, typ := range cse.Types {
				rt := p.reflector.ToRType(typ)
				if t == rt {
					return p.evalStmt(cse.Body)
				}
			}
		}
		// no case were triggered.
		// execute the default one, if any.
		if dflt != nil {
			return p.evalStmt(dflt.Body)
		}
		return nil
	case *stmt.Select:
		cases := make([]reflect.SelectCase, len(s.Cases))
		works := make([]struct {
			Chan     reflect.Value
			Vars     []reflect.Value
			Names    []string
			Implicit []bool
		}, len(s.Cases))
		for i, cse := range s.Cases {
			if cse.Default {
				cases[i].Dir = reflect.SelectDefault
				continue
			}
			switch cse := cse.Stmt.(type) {
			case *stmt.Assign:
				works[i].Chan = p.evalExprOne(cse.Right[0].(*expr.Unary).Expr)
				works[i].Names = make([]string, len(cse.Left))
				works[i].Vars = make([]reflect.Value, len(cse.Left))
				works[i].Implicit = make([]bool, len(cse.Left))
				if cse.Decl {
					for j, lhs := range cse.Left {
						works[i].Implicit[j] = true
						name := lhs.(*expr.Ident).Name
						if name == "_" {
							continue
						}
						works[i].Names[j] = name
					}
				}
				cases[i].Chan = works[i].Chan
				cases[i].Dir = reflect.SelectRecv
			case *stmt.Simple:
				works[i].Chan = p.evalExprOne(cse.Expr.(*expr.Unary).Expr)
				cases[i].Chan = works[i].Chan
				cases[i].Dir = reflect.SelectRecv
			case *stmt.Send:
				works[i].Chan = p.evalExprOne(cse.Chan)
				send := p.evalExprOne(cse.Value)
				cases[i].Dir = reflect.SelectSend
				cases[i].Chan = works[i].Chan
				cases[i].Send = send
			default:
				panic(interpPanic{fmt.Errorf("unknown select case type: %T", cse)})
			}
		}
		chosen, recv, recvOK := reflect.Select(cases)
		p.pushScope()
		defer p.popScope()
		work := &works[chosen]
		// prepare scope for body evaluation
		switch cases[chosen].Dir {
		case reflect.SelectRecv:
			switch n := len(work.Vars); n {
			case 0:
			case 1:
				work.Vars[0] = recv
				s := &Scope{
					Parent:   p.Cur,
					VarName:  work.Names[0],
					Var:      recv,
					Implicit: work.Implicit[0],
				}
				p.Cur = s
			case 2:
				work.Vars[0] = recv
				work.Vars[1] = reflect.New(reflect.TypeOf(recvOK)).Elem()
				work.Vars[1].SetBool(recvOK)
				for i := range work.Vars {
					name := work.Names[i]
					if name == "" {
						continue
					}
					s := &Scope{
						Parent:   p.Cur,
						VarName:  name,
						Var:      work.Vars[i],
						Implicit: work.Implicit[i],
					}
					p.Cur = s
				}
			default:
				panic(interpPanic{fmt.Errorf("internal error: invalid number of vars (%d)", n)})
			}
		case reflect.SelectSend:
		case reflect.SelectDefault:
		default:
			panic(interpPanic{fmt.Errorf("invalid select case chan-dir: %v", cases[chosen].Dir)})
		}
		cse := &s.Cases[chosen]
		p.evalStmt(cse.Body)
		return nil
	}
	panic(fmt.Sprintf("TODO evalStmt: %s", format.Stmt(s)))
}

func (p *Program) evalExprOne(e expr.Expr) reflect.Value {
	v := p.evalExpr(e)
	if len(v) != 1 {
		panic(interpPanic{fmt.Errorf("expression returns %d values instead of 1", len(v))})
	}
	return v[0]
}

func (p *Program) evalConst(s *stmt.Const) []reflect.Value {
	types := make([]tipe.Type, 0, len(s.NameList))
	vals := make([]reflect.Value, 0, len(s.NameList))
	for _, rhs := range s.Values {
		v := p.evalExpr(rhs)
		t := p.Types.Type(rhs)
		if tuple, isTuple := t.(*tipe.Tuple); isTuple {
			types = append(types, tuple.Elems...)
		} else {
			types = append(types, t)
		}
		// TODO: insert an implicit interface type conversion here
		vals = append(vals, v...)
	}
	if s.Type != nil {
		types = make([]tipe.Type, len(s.NameList))
		for i := range types {
			types[i] = s.Type
		}
	}

	vars := make([]reflect.Value, len(s.NameList))
	for i, name := range s.NameList {
		if name == "_" {
			continue
		}
		t := p.reflector.ToRType(types[i])
		s := &Scope{
			Parent:   p.Cur,
			VarName:  name,
			Var:      reflect.New(t).Elem(),
			Implicit: true,
		}
		p.Cur = s
		vars[i] = s.Var
	}
	for i := range vars {
		if !vars[i].IsValid() {
			continue
		}
		if len(vals) <= i {
			continue
		}
		v := vals[i]
		if vars[i].Type() != v.Type() {
			v = v.Convert(vars[i].Type())
		}
		vars[i].Set(v)
	}
	return nil
}

func (p *Program) evalVar(s *stmt.Var) []reflect.Value {
	types := make([]tipe.Type, 0, len(s.NameList))
	vals := make([]reflect.Value, 0, len(s.NameList))
	for _, rhs := range s.Values {
		v := p.evalExpr(rhs)
		t := p.Types.Type(rhs)
		if tuple, isTuple := t.(*tipe.Tuple); isTuple {
			types = append(types, tuple.Elems...)
		} else {
			types = append(types, t)
		}
		// TODO: insert an implicit interface type conversion here
		vals = append(vals, v...)
	}
	if s.Type != nil {
		types = make([]tipe.Type, len(s.NameList))
		for i := range types {
			types[i] = s.Type
		}
	}

	vars := make([]reflect.Value, len(s.NameList))
	for i, name := range s.NameList {
		if name == "_" {
			continue
		}
		t := p.reflector.ToRType(types[i])
		s := &Scope{
			Parent:   p.Cur,
			VarName:  name,
			Var:      reflect.New(t).Elem(),
			Implicit: true,
		}
		p.Cur = s
		vars[i] = s.Var
	}
	for i := range vars {
		if !vars[i].IsValid() {
			continue
		}
		if len(vals) <= i {
			continue
		}
		v := vals[i]
		if vars[i].Type() != v.Type() {
			v = v.Convert(vars[i].Type())
		}
		vars[i].Set(v)
	}
	return nil
}

type UntypedInt struct{ *big.Int }
type UntypedFloat struct{ *big.Float }
type UntypedComplex struct{ Real, Imag *big.Float }
type UntypedString struct{ String string }
type UntypedRune struct{ Rune rune }
type UntypedBool struct{ Bool bool }

func (uc UntypedComplex) String() string {
	return uc.Real.String() + "+" + uc.Imag.String() + "i"
}

func promoteUntyped(x interface{}) interface{} {
	switch x := x.(type) {
	case UntypedInt:
		return int(x.Int64())
	case UntypedFloat:
		f, _ := x.Float64()
		return float64(f)
	case UntypedComplex:
		r, _ := x.Real.Float64()
		i, _ := x.Imag.Float64()
		return complex(r, i)
	case UntypedString:
		return x.String
	case UntypedRune:
		return x.Rune
	case UntypedBool:
		return x.Bool
	default:
		return x
	}
}

// TODO merge convert func into typeConv func?
func convert(v reflect.Value, t reflect.Type) reflect.Value {
	if v.Type() == t {
		return v
	}
	switch val := v.Interface().(type) {
	case reflect.Type:
		return v // type conversion
	case UntypedInt:
		switch t {
		case reflect.TypeOf(UntypedFloat{}):
			res := UntypedFloat{new(big.Float)}
			res.Float.SetInt(val.Int)
			return reflect.ValueOf(res)
		case reflect.TypeOf(UntypedComplex{}):
			res := UntypedComplex{new(big.Float), new(big.Float)}
			res.Real.SetInt(val.Int)
			return reflect.ValueOf(res)
		}
		ret := reflect.New(t).Elem()
		switch t.Kind() {
		case reflect.Interface:
			ret.Set(reflect.ValueOf(int(val.Int64())))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			ret.SetUint(val.Uint64())
		case reflect.Float32, reflect.Float64:
			ret.SetFloat(float64(val.Int64()))
		case reflect.Complex64, reflect.Complex128:
			ret.SetComplex(complex(float64(val.Int64()), 0))
		default:
			ret.SetInt(val.Int64())
		}
		return ret
	case UntypedFloat:
		switch t {
		case reflect.TypeOf(UntypedComplex{}):
			res := UntypedComplex{new(big.Float), new(big.Float)}
			res.Real.Set(val.Float)
			return reflect.ValueOf(res)
		}
		ret := reflect.New(t).Elem()
		f, _ := val.Float64()
		switch t.Kind() {
		case reflect.Interface:
			ret.Set(reflect.ValueOf(float64(f)))
		case reflect.Complex64, reflect.Complex128:
			ret.SetComplex(complex(float64(f), 0))
		default:
			ret.SetFloat(f)
		}
		return ret
	case UntypedComplex:
		ret := reflect.New(t).Elem()
		r, _ := val.Real.Float64()
		i, _ := val.Imag.Float64()
		if t.Kind() == reflect.Interface {
			ret.Set(reflect.ValueOf(complex(r, i)))
		} else {
			ret.SetComplex(complex(r, i))
		}
		return ret
	case UntypedString:
		ret := reflect.New(t).Elem()
		s := val.String
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			ret.Set(reflect.ValueOf([]byte(s)))
		} else if t.Kind() == reflect.Interface {
			ret.Set(reflect.ValueOf(s))
		} else {
			ret.SetString(s)
		}
		return ret
	case UntypedRune:
		ret := reflect.New(t).Elem()
		r := val.Rune
		if t.Kind() == reflect.Interface {
			ret.Set(reflect.ValueOf(r))
		} else {
			ret.SetInt(int64(r))
		}
		return ret
	case UntypedBool:
		ret := reflect.New(t).Elem()
		b := val.Bool
		if t.Kind() == reflect.Interface {
			ret.Set(reflect.ValueOf(b))
		} else {
			ret.SetBool(b)
		}
		return ret
	default:
		ret := reflect.New(t).Elem()
		ret.Set(v)
		return ret
	}
}

type interpPanic struct {
	reason error
}

func (p interpPanic) Error() string {
	return p.reason.Error()
}

func (p *Program) prepCall(e *expr.Call) (fn reflect.Value, args []reflect.Value) {
	fn = p.evalExprOne(e.Func)
	args = make([]reflect.Value, 0, len(e.Args))
	i := 0
	for _, arg := range e.Args {
		vals := p.evalExpr(arg)
		for _, v := range vals {
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
			args = append(args, v)
			i++
		}
	}
	if e.Ellipsis {
		last := args[len(args)-1] // this is a slice
		args = args[:len(args)-1]
		for i, l := 0, last.Len(); i < l; i++ {
			args = append(args, last.Index(i))
		}
	}
	return fn, args
}

func (p *Program) evalExpr(e expr.Expr) []reflect.Value {
	switch e := e.(type) {
	case *expr.BasicLiteral:
		var v reflect.Value
		switch val := e.Value.(type) {
		case *big.Int:
			v = reflect.ValueOf(UntypedInt{val})
		case *big.Float:
			v = reflect.ValueOf(UntypedFloat{val})
		case *bigcplx.Complex:
			v = reflect.ValueOf(UntypedComplex{val.Real, val.Imag})
		case string:
			v = reflect.ValueOf(UntypedString{val})
		case rune:
			v = reflect.ValueOf(UntypedRune{val})
		case bool:
			v = reflect.ValueOf(UntypedBool{val})
		default:
			v = reflect.ValueOf(val)
		}
		t := p.reflector.ToRType(p.Types.Type(e))
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
		if (e.Op == token.Equal || e.Op == token.NotEqual) && (lhs[0].Kind() == reflect.Func || rhs[0].Kind() == reflect.Func) {
			// functions can only be compared to nil
			if lhs[0].IsNil() || rhs[0].IsNil() {
				v := lhs[0].IsNil()
				if e.Op == token.Equal {
					v = v == rhs[0].IsNil()
				} else {
					v = v != rhs[0].IsNil()
				}
				return []reflect.Value{reflect.ValueOf(v)}
			}
			panic("comparing uncomparable type " + format.Type(p.Types.Type(e.Left)))
		}
		x := lhs[0].Interface()
		y := rhs[0].Interface()
		v, err := binOp(e.Op, x, y)
		if err != nil {
			panic(interpPanic{err})
		}
		t := p.reflector.ToRType(p.Types.Type(e))
		return []reflect.Value{convert(reflect.ValueOf(v), t)}
	case *expr.Call:
		fn, args := p.prepCall(e)
		if t, isTypeConv := fn.Interface().(reflect.Type); isTypeConv {
			return []reflect.Value{typeConv(t, args[0])}
		}
		res := fn.Call(args)
		if p.builtinCalled {
			p.builtinCalled = false
			for i := range res {
				// Necessary to turn the return type of append
				// from an interface{} into a slice so it can
				// be set.
				res[i] = reflect.ValueOf(res[i].Interface())
			}
		}
		if e.ElideError {
			// Dynamic elision of final error.
			errv := res[len(res)-1]
			if errv.IsValid() && errv.CanInterface() && errv.Interface() != nil {
				panic(Panic{val: errv.Interface()})
			}
			res = res[:len(res)-1]
			if len(res) == 0 {
				res = nil
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
		return []reflect.Value{p.evalFuncLiteral(e, nil)}
	case *expr.Ident:
		if e.Name == "nil" { // TODO: make sure it's the Universe nil
			t := p.reflector.ToRType(p.Types.Type(e))
			return []reflect.Value{reflect.New(t).Elem()}
		}
		if v := p.Cur.Lookup(e.Name); v != (reflect.Value{}) {
			return []reflect.Value{v}
		}
		t := p.Types.Type(e)
		if t != nil {
			return []reflect.Value{reflect.ValueOf(p.reflector.ToRType(p.Types.Type(e)))}
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
		case reflect.Array, reflect.Slice, reflect.String:
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
			if _, returnExists := p.Types.Type(e).(*tipe.Tuple); returnExists {
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
		if lhs.Kind() == reflect.Ptr {
			if pkg, ok := lhs.Interface().(*gowrap.Pkg); ok {
				name := e.Right.Name
				return []reflect.Value{pkg.Exports[name]}
			}
		}
		v := lhs.MethodByName(e.Right.Name)
		if v == (reflect.Value{}) && lhs.Kind() != reflect.Ptr && lhs.CanAddr() {
			v = lhs.Addr().MethodByName(e.Right.Name)
		}
		if v == (reflect.Value{}) && lhs.Kind() == reflect.Struct {
			v = lhs.FieldByName(e.Right.Name)
		}
		if v == (reflect.Value{}) && lhs.Kind() == reflect.Ptr {
			if lhs := lhs.Elem(); lhs.Kind() == reflect.Struct {
				v = lhs.FieldByName(e.Right.Name)
			}
		}
		return []reflect.Value{v}
	case *expr.Shell:
		p.pushScope()
		defer p.popScope()
		res, err := shell.Run(p.ShellState, p, e)
		str := reflect.ValueOf(res)
		if e.ElideError {
			// Dynamic elision of final error.
			if err != nil {
				panic(Panic{val: err})
			}
			return []reflect.Value{str}
		}
		if err != nil {
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
	case *expr.TypeAssert:
		v := p.evalExprOne(e.Left)
		if e.Type == nil {
			// this is coming from a switch x.(type) statement
			return []reflect.Value{v}
		}
		t := p.reflector.ToRType(e.Type)
		convertible := v.Type().ConvertibleTo(t)
		if convertible {
			v = v.Convert(t)
		} else {
			if v.CanInterface() {
				viface := v.Interface()
				v = reflect.ValueOf(viface)
				convertible = v.Type().ConvertibleTo(t)
				if convertible {
					v = v.Convert(t)
				}
			}
			if !convertible {
				v = reflect.Zero(t)
			}
		}
		if _, commaOK := p.Types.Type(e).(*tipe.Tuple); commaOK {
			return []reflect.Value{v, reflect.ValueOf(convertible)}
		}
		if !convertible {
			panic(Panic{val: fmt.Errorf("value of type %s cannot be converted to %s", v.Type(), e.Type)})
		}
		return []reflect.Value{v}
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
			b := v.Bool()
			if !v.CanAddr() {
				v = reflect.New(reflect.TypeOf(false)).Elem()
			}
			v.SetBool(!b)
		case token.Add, token.Sub:
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
			case complex64:
				lhs = complex64(0)
			case complex128:
				lhs = complex128(0)
			case UntypedInt:
				lhs = UntypedInt{big.NewInt(0)}
			case UntypedFloat:
				lhs = UntypedFloat{big.NewFloat(0)}
			case UntypedComplex:
				lhs = UntypedComplex{Real: big.NewFloat(0), Imag: big.NewFloat(0)}
			}
			res, err := binOp(e.Op, lhs, rhs.Interface())
			if err != nil {
				panic(interpPanic{err})
			}
			v = reflect.ValueOf(res)
		case token.ChanOp:
			ch := p.evalExprOne(e.Expr)
			res, ok := ch.Recv()
			switch et := p.Types.Type(e).(type) {
			case *tipe.Tuple:
				t := p.reflector.ToRType(et.Elems[0])
				return []reflect.Value{convert(res, t), reflect.ValueOf(ok)}
			}
			v = res
		}
		t := p.reflector.ToRType(p.Types.Type(e))
		return []reflect.Value{convert(v, t)}
	}
	panic(interpPanic{fmt.Errorf("TODO evalExpr(%s), %T", format.Expr(e), e)})
}

func (p *Program) evalFuncLiteral(e *expr.FuncLiteral, recvt *tipe.Named) reflect.Value {
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

	funct := *e.Type
	if recvt != nil {
		params := make([]tipe.Type, 1+len(funct.Params.Elems))
		copy(params[1:], funct.Params.Elems)
		params[0] = &tipe.Interface{} // not recvt, breaking cycle
		funct.Params = &tipe.Tuple{params}
	}
	rt := p.reflector.ToRType(&funct)
	fn := reflect.MakeFunc(rt, func(args []reflect.Value) (res []reflect.Value) {
		p := &Program{
			Universe:  p.Universe,
			Types:     p.Types, // TODO race cond, clone type list
			Cur:       s,
			reflector: p.reflector,
		}
		p.pushScope()
		defer p.popScope()
		if recvt != nil {
			// TODO args[0]
			args = args[1:]
		}
		for i, name := range e.ParamNames {
			// A function argument defines an addressable value,
			// but the reflect.Value args passed to a MakeFunc
			// implementation are not addressable.
			// So we copy them here.
			arg := reflect.New(args[i].Type()).Elem()
			arg.Set(args[i])
			p.Cur = &Scope{
				Parent:   p.Cur,
				VarName:  name,
				Var:      arg,
				Implicit: true,
			}
		}
		resValues := p.evalStmt(e.Body.(*stmt.Block))
		// Define new result values to hold return
		// types so any necessary interface boxing
		// is done. For example:
		//
		//	func f(x int) interface{} { return x }
		//
		// The int x needs to become an interface{}.
		res = make([]reflect.Value, len(resValues))
		for i, v := range resValues {
			t := p.reflector.ToRType(funct.Results.Elems[i])
			res[i] = reflect.New(t).Elem()
			res[i].Set(v)
		}
		return res
	})
	return fn
}

type reflector struct {
	mu  sync.RWMutex
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

	r.mu.RLock()
	rtype := r.fwd[t]
	r.mu.RUnlock()

	if rtype != nil {
		return rtype
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.toRType(t)
}

func (r *reflector) toRType(t tipe.Type) reflect.Type {
	rtype := r.fwd[t]
	if rtype != nil {
		return rtype
	}

	if t == tipe.Byte {
		rtype = reflect.TypeOf(byte(0))
		r.fwd[t] = rtype
		return rtype
	}
	if t == tipe.Rune {
		rtype = reflect.TypeOf(rune(0))
		r.fwd[t] = rtype
		return rtype
	}
	t = tipe.Unalias(t)
	switch t := t.(type) {
	case tipe.Basic:
		switch t {
		case tipe.Invalid:
			return nil
		case tipe.Num:
			panic("TODO rtype for Num")
		case tipe.Bool:
			rtype = reflect.TypeOf(false)
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
		case tipe.Complex64:
			rtype = reflect.TypeOf(complex64(0))
		case tipe.Complex128:
			rtype = reflect.TypeOf(complex128(0))
		case tipe.UntypedNil:
			panic("TODO UntypedNil")
		case tipe.UntypedInteger:
			rtype = reflect.TypeOf(UntypedInt{})
		case tipe.UntypedFloat:
			rtype = reflect.TypeOf(UntypedFloat{})
		case tipe.UntypedComplex:
			rtype = reflect.TypeOf(UntypedComplex{})
		case tipe.UntypedString:
			rtype = reflect.TypeOf(UntypedString{})
		case tipe.UntypedRune:
			rtype = reflect.TypeOf(UntypedRune{})
		case tipe.UntypedBool:
			rtype = reflect.TypeOf(UntypedBool{})
		}
	case *tipe.Func:
		var in, out []reflect.Type
		if t.Params != nil {
			for _, p := range t.Params.Elems {
				in = append(in, r.toRType(p))
			}
		}
		if t.Results != nil {
			for _, p := range t.Results.Elems {
				out = append(out, r.toRType(p))
			}
		}
		rtype = reflect.FuncOf(in, out, t.Variadic)
	case *tipe.Struct:
		var fields []reflect.StructField
		for _, f := range t.Fields {
			fields = append(fields, reflect.StructField{
				Name: f.Name,
				Type: r.toRType(f.Type),
			})
		}
		rtype = reflect.StructOf(fields)
	case *tipe.Named:
		if typecheck.IsError(t) {
			rtype = reflect.TypeOf((*error)(nil)).Elem()
		} else if t.PkgPath != "" {
			path := t.PkgPath
			if path == "neugram.io/ng/vendor/mat" {
				path = "mat" // TODO: remove "mat" exception
			}
			v := gowrap.Pkgs[path].Exports[t.Name]
			if typ, isType := v.Interface().(reflect.Type); isType {
				rtype = typ
			} else if _, isIface := tipe.Underlying(t).(*tipe.Interface); isIface {
				rtype = v.Type().Elem()
			} else {
				rtype = v.Type()
			}
		} else {
			rtype = r.toRType(t.Type)
		}
	case *tipe.Array:
		rtype = reflect.ArrayOf(int(t.Len), r.toRType(t.Elem))
	case *tipe.Slice:
		rtype = reflect.SliceOf(r.toRType(t.Elem))
	case *tipe.Ellipsis:
		rtype = reflect.SliceOf(r.toRType(t.Elem))
	// TODO case *Table:
	case *tipe.Pointer:
		rtype = reflect.PtrTo(r.toRType(t.Elem))
	case *tipe.Chan:
		var dir reflect.ChanDir
		switch t.Direction {
		case tipe.ChanBoth:
			dir = reflect.BothDir
		case tipe.ChanSend:
			dir = reflect.SendDir
		case tipe.ChanRecv:
			dir = reflect.RecvDir
		default:
			panic(fmt.Sprintf("bad channel direction: %v", t.Direction))
		}
		rtype = reflect.ChanOf(dir, r.toRType(t.Elem))
	case *tipe.Map:
		rtype = reflect.MapOf(r.toRType(t.Key), r.toRType(t.Value))
	// TODO case *Interface:
	// TODO need more reflect support, MakeInterface
	// TODO needs reflect.InterfaceOf
	//case *Tuple:
	//case *Package:
	default:
		rtype = reflect.TypeOf((*interface{})(nil)).Elem()
	}
	r.fwd[t] = rtype
	return rtype
}

func (r *reflector) FromRType(rtype reflect.Type) tipe.Type {
	panic("TODO FromRType")
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
