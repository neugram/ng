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
	Types  map[expr.Expr]tipe.Type
	Defs   map[*expr.Ident]*Obj
	Values map[expr.Expr]constant.Value

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
		cur:    &Scope{Objs: make(map[string]*Obj)},
	}
}

type partialMode int

const (
	modeInvalid partialMode = iota
	modeVoid
	modeConst
	modeVar
	modeBuiltin
)

type partial struct {
	mode partialMode
	typ  tipe.Type
	val  constant.Value
	expr expr.Expr
}

func (c *Checker) errorf(format string, args ...interface{}) {
	fmt.Printf("typecheck error: %s\n", fmt.Sprintf(format, args...))
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
		return tipe.Integer
	case tipe.UntypedFloat:
		return tipe.Float
	}
	return t
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
				obj := &Obj{Type: partials[i].typ}
				c.Defs[lhs.(*expr.Ident)] = obj
				c.cur.Objs[lhs.(*expr.Ident).Name] = obj
			}
		} else {
			for i, lhs := range s.Left {
				p := partials[i]
				lhsP := c.expr(lhs)
				if isUntyped(p.typ) {
					c.constrainUntyped(&p, c.Types[lhsP.expr])
				}
			}
		}

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
		p.mode = modeVar
		return p
	case *expr.BasicLiteral:
		p.mode = modeConst
		// TODO: use constant.Value in BasicLiteral directly.
		switch v := e.Value.(type) {
		case *big.Int:
			p.typ = tipe.UntypedInteger
			p.val = constant.MakeFromLiteral(v.String(), gotoken.INT, 0)
		case *big.Float:
			p.typ = tipe.UntypedFloat
			p.val = constant.MakeFromLiteral(v.String(), gotoken.FLOAT, 0)
		}
		return p
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
	default:
		panic(fmt.Sprintf("expr TODO: %T", e))
	}
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
		case t != p.typ:
			panic("cannot convert untyped")
			// TODO c.errorf("cannot convert %s to %s", x, typ)
		}
	} else {
		switch t := Underlying(t).(type) {
		case tipe.Basic:
			switch p.mode {
			case modeConst:
				p.val = round(p.val, t)
				if p.val == nil {
					panic("cannot convert")
					// TODO c.errorf
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
		fmt.Fprintf(buf, "\t\t(%p)%s: (%p).Type:%s\n", k, k.Sexp(), v, t)
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

// An Obj represents a declared constant, type, variable, or function.
type Obj struct {
	Type tipe.Type
	Used bool
}

func Underlying(t tipe.Type) tipe.Type {
	if n, ok := t.(*tipe.Named); ok {
		return n.Underlying
	}
	return t
}

func isTyped(t tipe.Type) bool {
	return Underlying(t) != tipe.Invalid && !isUntyped(t)
}

func isUntyped(t tipe.Type) bool {
	switch Underlying(t) {
	case tipe.UntypedBool, tipe.UntypedInteger, tipe.UntypedFloat, tipe.UntypedComplex:
		return true
	}
	return false
}
