package main

import (
	"bufio"
	"fmt"
	"os"

	"numgrad.io/eval"
	"numgrad.io/lang/typecheck"
	"numgrad.io/parser"
)

func main() {
	prg := &eval.Program{
		Pkg: map[string]*eval.Scope{
			"main": &eval.Scope{Var: map[string]*eval.Variable{}},
		},
		Types: typecheck.New(),
	}
	p := parser.New()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("ng> ")
	for scanner.Scan() {
		res := p.ParseLine(scanner.Bytes())

		for _, s := range res.Stmts {
			v, err := prg.Eval(s)
			if err != nil {
				fmt.Printf("eval error: %v\n", err)
				continue
			}
			switch len(v) {
			case 0:
			case 1:
				fmt.Println(v[0])
			default:
				fmt.Println(v)
			}
		}
		for _, err := range res.Errs {
			fmt.Println(err.Error())
		}

		switch res.State {
		case parser.StateUnknown:
			fmt.Print("??> ")
		case parser.StateStmt:
			fmt.Print("ng> ")
		case parser.StateStmtPartial:
			fmt.Print("..> ")
		case parser.StateCmd:
			fmt.Print("$ ")
		case parser.StateCmdPartial:
			fmt.Print("..$ ")
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
