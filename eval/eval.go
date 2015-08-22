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
	res, err := p.evalStmt(s)
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
	case *stmt.Assign:
		vars := make([]*Variable, len(s.Left))
		if s.Decl {
			for i, lhs := range s.Left {
				vars[i] = new(Variable)
				p.Cur.Var[lhs.(*expr.Ident).Name] = vars[i]
			}
		} else {
			// TODO: order of evaluation, left-then-right,
			// or right-then-left?
			for i, lhs := range s.Left {
				v, err := p.evalExpr(lhs)
				if err != nil {
					return nil, err
				}
				vars[i] = v[0].(*Variable)
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
			vars[i].Value = vals[i]
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
			if p.Returning {
				return res, nil
			}
		}
		return nil, nil
	case *stmt.If:
		if s.Init != nil {
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
	if s == nil {
		return nil, fmt.Errorf("Parser.evalStmt: statement is nil")
	}
	panic(fmt.Sprintf("TODO evalStmt: %T: %s", s, s.Sexp()))
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
	case *expr.FuncLiteral:
		// lack of symmetry with BasicLiteral is unfortunate
		return v, nil
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
		case token.Sub:
			rhs, err := p.evalExprAndReadVar(e.Expr)
			if err != nil {
				return nil, err
			}
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
		switch fn := res.(type) {
		case *expr.FuncLiteral:
			p.pushScope()
			defer p.popScope()
			res, err := p.evalStmt(fn.Body.(*stmt.Block))
			if err != nil {
				return nil, err
			}
			if p.Returning {
				p.Returning = false
			} else if len(fn.Type.Out) > 0 {
				return nil, fmt.Errorf("missing return %v", fn.Type.Out)
			}
			return res, nil
		}
	}
	return nil, fmt.Errorf("TODO evalExpr(%s), %T", e.Sexp(), e)
}
