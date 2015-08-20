// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/token"
)

type Variable struct {
	// Value has the type:
	//	nil
	//	int64
	//	float32
	//	float64
	//	*big.Int
	//	*big.Float
	//	*Struct
	Value interface{}
}

type Scope struct {
	Parent *Scope
	Var    map[string]*Variable // variable name -> variable
}

type Program struct {
	Pkg       map[string]*Scope // package -> scope
	Cur       *Scope
	Returning bool
}

func (p *Program) Eval(s stmt.Stmt) ([]interface{}, error) {
	if p.Cur == nil {
		p.Cur = p.Pkg["main"]
	}
	fmt.Printf("eval2.Eval(%q)\n", s.Sexp())
	return p.evalStmt(s)
}

func (p *Program) pushScope() {
	p.Cur = &Scope{
		Parent: p.Cur,
		Var:    make(map[string]*Variable),
	}
}
func (p *Program) popScope() {
	p.Cur = p.Cur.Parent
}

func (p *Program) evalStmt(s stmt.Stmt) ([]interface{}, error) {
	switch s := s.(type) {
	case *stmt.Simple:
		res, err := p.evalExpr(s.Expr)
		fmt.Printf("Returning Simple: %#+v\n", res)
		return res, err
	case *stmt.Block:
		p.pushScope()
		defer p.popScope()
		for _, s := range s.Stmts {
			res, err := p.evalStmt(s)
			if err != nil {
				return nil, err
			}
			if p.Returning {
				return res, nil
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
	}
	panic(fmt.Sprintf("TODO evalStmt: %T: %s", s, s))
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
	res, err := p.evalExpr(e)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 { // TODO these kinds of invariants are the job of the type checker
		return nil, fmt.Errorf("multi-valued (%d) expression in single-value context", len(res))
	}
	return p.readVar(res[0])
}

func (p *Program) readVar(e interface{}) (interface{}, error) {
	switch v := e.(type) {
	case *expr.BasicLiteral:
		return v.Value, nil
	case *Variable:
		return v.Value, nil
	case bool, int64, float32, float64, *big.Int, *big.Float:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected type %T for value", v)
	}
}

func (p *Program) evalExpr(e expr.Expr) ([]interface{}, error) {
	switch e := e.(type) {
	case *expr.BasicLiteral, *expr.FuncLiteral:
		return []interface{}{e}, nil
	case *expr.Ident:
		for sc := p.Cur; sc != nil; sc = sc.Parent {
			if v, ok := sc.Var[e.Name]; ok {
				return []interface{}{v}, nil
			}
		}
		return nil, fmt.Errorf("eval: undefined identifier: %q", e.Name)
	case *expr.Unary:
		switch e.Op {
		case token.LeftParen:
			return p.evalExpr(e.Expr)
		case token.Not:
			v, err := p.evalExprAndReadVar(e.Expr)
			if err != nil {
				return nil, err
			}
			if v, ok := v.(bool); ok {
				return []interface{}{!v}, nil
			}
			return nil, fmt.Errorf("negation operator expects boolean expression, not %T", v)
		}
	case *expr.Binary:
		lhs, err := p.evalExprAndReadVar(e.Left)
		if err != nil {
			return nil, err
		}

		switch e.Op {
		case token.LogicalAnd:
			panic("TODO LogicalAnd") // maybe skip RHS
		case token.LogicalOr:
			panic("TODO LogicalOr") // maybe skip RHS
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
		res, err := p.evalExpr(e.Func)
		if err != nil {
			return nil, err
		}
		if len(res) != 1 {
			return nil, fmt.Errorf("multi-valued (%d) expression when expecting single-value function", len(res))
		}
		switch fn := res[0].(type) {
		case *expr.FuncLiteral:
			p.pushScope()
			defer p.popScope()
			res, err := p.evalStmt(fn.Body.(*stmt.Block))
			if p.Returning {
				p.Returning = false
			} else if len(fn.Type.Out) > 0 {
				return nil, fmt.Errorf("missing return %v", fn.Type.Out)
			}
			fmt.Printf("Returning: %#+v\n", res)
			return res, err
		}
	}
	return nil, fmt.Errorf("TODO evalExpr(%s), %T", e, e)
}
