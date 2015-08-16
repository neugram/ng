// Copyright 2015 The Numgrad Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package parser

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"strings"
)

func Print(w io.Writer, expr Expr) error {
	p := printer{w: w}
	p.printExpr(expr)
	return p.err
}

var indentText = []byte("|\t")

type printer struct {
	w          io.Writer
	numIndent  int
	needIndent bool
	err        error
}

func (p *printer) writeIndent() error {
	if !p.needIndent {
		return nil
	}
	p.needIndent = false
	for i := 0; i < p.numIndent; i++ {
		if _, err := p.w.Write(indentText); err != nil {
			return err
		}
	}
	return nil
}

func (p *printer) Write(b []byte) (n int, err error) {
	wrote := 0
	for len(b) > 0 {
		if err := p.writeIndent(); err != nil {
			return wrote, err
		}
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break
		}
		n, err = p.w.Write(b[0 : i+1])
		wrote += n
		b = b[i+1:]
		p.needIndent = true
		if err != nil {
			return wrote, err
		}
	}
	if len(b) > 0 {
		n, err = p.w.Write(b)
		wrote += n
	}
	return wrote, err
}

func (p *printer) printf(format string, a ...interface{}) {
	if p.err != nil {
		return
	}
	if _, err := fmt.Fprintf(p, format, a...); err != nil {
		p.err = err
	}
}

func (p *printer) printFields(f []*Field) {
	p.printf("Fields TODO")
}

func (p *printer) printStmt(s Stmt) {
	if p.err != nil {
		return
	}
	if s == nil {
		p.printf("<nilstmt>")
		return
	}
	switch s := s.(type) {
	case *ReturnStmt:
		p.printf("ReturnStmt{")
		switch len(s.Exprs) {
		case 0:
		case 1:
			p.printf("Exprs: [")
			p.printExpr(s.Exprs[0])
			p.printf("]")
		default:
			p.printf("\n")
			p.numIndent++
			p.printf("\nExprs: [")
			p.numIndent++
			for _, expr := range s.Exprs {
				p.printf("\n")
				p.printExpr(expr)
			}
			p.numIndent--
			p.printf("\n]")
			p.numIndent--
			p.printf("\n")
		}
		p.printf("}")
	default:
		p.err = fmt.Errorf("Unknown Stmt (%T)", s)
	}
}

func (p *printer) printExpr(expr Expr) {
	if p.err != nil {
		return
	}
	if expr == nil {
		p.printf("<nilexpr>")
		return
	}
	switch expr := expr.(type) {
	case *BinaryExpr:
		p.printf("BinaryExpr{\n")
		p.numIndent++
		p.printf("Op:    %s", expr.Op)
		p.printf("\nLeft:  ")
		p.printExpr(expr.Left)
		p.printf("\nRight: ")
		p.printExpr(expr.Right)
		p.numIndent--
		p.printf("\n}")
	case *UnaryExpr:
		p.printf("UnaryExpr{\n")
		p.numIndent++
		p.printf("Op:    %s\n", expr.Op)
		p.printf("Expr:  ")
		p.printExpr(expr.Expr)
		p.numIndent--
		p.printf("\n}")
	case *BadExpr:
		p.printf("BadExpr{Error: %v}", expr.Error)
	case *BasicLiteral:
		// TODO check types string, *big.Int, *big.Float
		switch expr.Value.(type) {
		case string, *big.Int, *big.Float:
			p.printf("BasicLiteral{Value: %#v}", expr.Value)
		default:
			p.printf("BasicLiteral{Value: Unknown Type=%T: %#v}", expr.Value, expr.Value)
		}
	case *FuncLiteral:
		p.printf("FuncLiteral{")
		p.numIndent++
		if len(expr.Type.In) > 0 || len(expr.Type.Out) > 0 {
			p.printf("\nType: FuncType{")
			p.numIndent++
			if len(expr.Type.In) > 0 {
				p.printf("\nIn: ")
				p.printFields(expr.Type.In)
			}
			if len(expr.Type.Out) > 0 {
				p.printf("\nOut: ")
				p.printFields(expr.Type.Out)
			}
			p.numIndent--
			p.printf("\n}")
		}
		p.printf("\nBody: [")
		p.numIndent++
		for _, s := range expr.Body {
			p.printf("\n")
			p.printStmt(s)
		}
		p.numIndent--
		p.printf("\n]")
		p.numIndent--
		p.printf("\n}")
	case *Ident:
		p.printf("Ident{Name: %q}", expr.Name)
	case *CallExpr:
		p.printf("CallExpr{\n")
		p.numIndent++
		p.printf("Func: ")
		p.printExpr(expr.Func)
		p.printf("\nArgs: [")
		p.numIndent++
		for _, expr := range expr.Args {
			p.printf("\n")
			p.printExpr(expr)
		}
		p.numIndent--
		p.printf("\n]")
		p.numIndent--
		p.printf("\n}")

	default:
		p.err = fmt.Errorf("Unknown Expr (%T)", expr)
	}
}

func printToFile(x Expr) (path string, err error) {
	f, err := ioutil.TempFile("", "numgrad-diff-")
	if err != nil {
		return "", err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	if err := Print(f, x); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func Diff(x, y Expr) string {
	fx, err := printToFile(x)
	if err != nil {
		return "diff print lhs error: " + err.Error()
	}
	defer os.Remove(fx)
	fy, err := printToFile(y)
	if err != nil {
		return "diff print rhs error: " + err.Error()
	}
	defer os.Remove(fy)

	b, _ := ioutil.ReadFile(fx)
	fmt.Printf("fx: %s\n", b)

	data, err := exec.Command("diff", "-U100", "-u", fx, fy).CombinedOutput()
	if err != nil && len(data) == 0 {
		// diff exits with a non-zero status when the files don't match.
		return "diff error: " + err.Error()
	}
	res := string(data)
	res = strings.Replace(res, fx, "/x", 1)
	res = strings.Replace(res, fy, "/y", 1)
	return res
}
