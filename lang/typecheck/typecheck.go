// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// Package typecheck is a Numengrad type checker.
package typecheck

import (
	"bytes"
	"fmt"
	"go/constant"
	gotoken "go/token"
	"math/big"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
)

type Checker struct {
	// TODO: we could put these on our AST. Should we?
	Types   map[expr.Expr]tipe.Type
	Defs    map[*expr.Ident]*Obj
	Values  map[expr.Expr]constant.Value
	NumSpec map[expr.Expr]tipe.Basic // *tipe.Call, *tipe.CompLiteral -> numeric basic type
	Errs    []error

	// TODO NamedInfo map[*tipe.Named]NamedInfo

	cur *Scope
}

// TODO type NamedInfo struct {
//	Obj *Obj
//	Methods []*Obj
//}

func New() *Checker {
	return &Checker{
		Types:  make(map[expr.Expr]tipe.Type),
		Defs:   make(map[*expr.Ident]*Obj),
		Values: make(map[expr.Expr]constant.Value),
		cur: &Scope{
			Parent: base,
			Objs:   make(map[string]*Obj),
		},
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
)

type partial struct {
	mode partialMode
	typ  tipe.Type
	val  constant.Value
	expr expr.Expr
}

func (c *Checker) stmt(s stmt.Stmt) {
	switch s := s.(type) {
	case *stmt.Assign:
		if len(s.Left) != len(s.Right) {
			panic("TODO artity mismatch, i.e. x, y := f()")
		}
		var partials []partial
		for _, rhs := range s.Right {
			partials = append(partials, c.expr(rhs))
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
				if isUntyped(p.typ) {
					c.constrainUntyped(&p, lhsP.typ)
				}
			}
		}

	case *stmt.Simple:
		c.expr(s.Expr)

	case *stmt.Block:
		c.pushScope()
		defer c.popScope()
		for _, s := range s.Stmts {
			c.stmt(s)
		}

	case *stmt.ClassDecl:
		obj := &Obj{
			Kind: ObjType,
			Type: s.Type,
			Decl: s,
		}
		c.cur.Objs[s.Name] = obj

	default:
		panic(fmt.Sprintf("typecheck: unknown stmt %T", s))
	}
}

func (c *Checker) expr(e expr.Expr) (p partial) {
	// TODO more mode adjustment
	p = c.exprPartial(e)
	if p.mode == modeConst {
		c.Values[p.expr] = p.val
		c.Types[p.expr] = p.typ
	}
	return p
}

func (c *Checker) exprPartial(e expr.Expr) (p partial) {
	fmt.Printf("exprPartial(%s)\n", e.Sexp())
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
		case ObjVar:
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
		p.typ = e.Type
		return p
	case *expr.CompLiteral:
		// resolve TODO break out into function?
		if u, unresolved := e.Type.(*tipe.Unresolved); unresolved {
			if u.Package != "" {
				// TODO look up package in scope, extract type from it.
				panic("TODO type in package")
			}
			obj := c.cur.LookupRec(u.Name)
			if obj == nil {
				c.errorf("type %s not declared", u.Name)
				p.mode = modeInvalid
				return p
			}
			if obj.Kind != ObjType {
				c.errorf("symbol %s is not a type", u.Name)
				p.mode = modeInvalid
				return p
			}
			// TODO: typecheck fields
			e.Type = obj.Type
			p.typ = e.Type
			p.expr = e
			return p
		}
		// TODO map, slice, table, etc.
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
		p := c.expr(e.Func)
		switch p.mode {
		case modeVar:
			// function call
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

			if len(e.Args) != len(params) {
				p.mode = modeInvalid
				c.errorf("wrong number of arguments (%d) to function %s", len(e.Args), funct)
			}

			if p.mode != modeInvalid {
				var argsp []partial
				for i, arg := range e.Args {
					t := params[i]
					argp := c.expr(arg)
					c.convert(&argp, t)
					if argp.mode == modeInvalid {
						p.mode = modeInvalid
						c.errorf("cannot use type %s as type %s in argument to function", argp.typ, t)
						break
					}
					argsp = append(argsp, argp)
				}
			}
			if p.mode == modeInvalid {
				return p
			}
			p.expr = e
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
		default:
			panic("unreachable, unknown call mode")
		}
	}
	panic(fmt.Sprintf("expr TODO: %T", e))
}

func (c *Checker) convert(p *partial, t tipe.Type) {
	fmt.Printf("Checker.convert(p=%#+v, t=%s)\n", p, t)
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

	if !convertible(p.typ, t) {
		// TODO p is assignable to t, lots of possibilities
		// (interface satisfaction, etc)
		c.errorf("cannot use %s as %s", p.typ, t)
		p.mode = modeInvalid
		return
	}

	if isUntyped(p.typ) {
		c.constrainUntyped(p, t)
	} else {
		p.typ = t
	}
}

func convertible(dst, src tipe.Type) bool {
	if dst == src {
		return true
	}
	// TODO several other forms of "identical" types,
	// e.g. maps where keys and value are identical,

	// numerics can be converted to one another
	if tipe.IsNumeric(dst) && tipe.IsNumeric(src) {
		return true
	}

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
					c.errorf("cannot convert %s to %s", p.typ, t)
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
	fmt.Printf("typecheck error: %s\n", err)
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

func (c *Checker) Add(s stmt.Stmt) {
	c.stmt(s)
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

	// TODO: NumSpec tipe.Type?
}

func (s *Scope) LookupRec(name string) *Obj {
	for s != nil {
		if o := s.Objs[name]; o != nil {
			return o
		}
		s = s.Parent
	}
	return nil
}

type ObjKind int

const (
	ObjUnknown ObjKind = iota
	ObjVar
	ObjType
)

func (o ObjKind) String() string {
	switch o {
	case ObjUnknown:
		return "ObjUnknown"
	case ObjVar:
		return "ObjVar"
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
	case tipe.UntypedBool, tipe.UntypedInteger, tipe.UntypedFloat, tipe.UntypedComplex:
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
