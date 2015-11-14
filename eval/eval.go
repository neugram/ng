// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package eval

import (
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"runtime/debug"

	"numgrad.io/lang/expr"
	"numgrad.io/lang/stmt"
	"numgrad.io/lang/tipe"
	"numgrad.io/lang/token"
	"numgrad.io/lang/typecheck"
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
	Types     *typecheck.Checker
	Returning bool
	Breaking  bool
}

func (p *Program) EvalCmd(argv []string) error {
	stdin := os.Stdin // TODO stdio
	stdout := os.Stdout
	stderr := os.Stderr
	switch argv[0] {
	case "cd":
		dir := ""
		if len(argv) == 1 {
			dir = os.Getenv("HOME")
		} else {
			dir = argv[1]
		}
		if err := os.Chdir(dir); err != nil {
			return err
		}
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", wd)
		return nil
	case "exit", "logout":
		return fmt.Errorf("ng does not know %q, try $$", argv[0])
	default:
		cmd := exec.Command(argv[0])
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Args = argv
		cmd.Run()
		return nil
	}
}

func (p *Program) Eval(s stmt.Stmt) (res []interface{}, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("ng eval panic: %v", x)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			debug.PrintStack()
			res = nil
		}
	}()

	if p.Cur == nil {
		p.Cur = p.Pkg["main"]
	}
	p.Types.Errs = p.Types.Errs[:0]
	p.Types.Add(s)
	if len(p.Types.Errs) > 0 {
		return nil, fmt.Errorf("typecheck: %v\n", p.Types.Errs[0])
	}

	res, err = p.evalStmt(s)
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
		typ := p.Types.Lookup(s.Name).Type.(*tipe.Go)
		equiv := typ.Equivalent.(*tipe.Package)
		fmt.Printf("import %s %#v: %#+v\n", s.Name, s, equiv)
		// TODO: p.Cur. set something or other here
		return nil, nil
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
			} else if len(fn.ResultNames) > 0 {
				return nil, fmt.Errorf("missing return %v", fn.ResultNames)
			}
			return res, nil
		}
	case *expr.Shell:
		for _, cmd := range e.Cmds {
			if err := p.EvalCmd(cmd); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	return nil, fmt.Errorf("TODO evalExpr(%s), %T", e.Sexp(), e)
}
