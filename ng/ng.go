package main

import (
	"bufio"
	"fmt"
	"os"

	"numgrad.io/eval"
	"numgrad.io/parser"
)

func main() {
	prg := &eval.Program{
		Pkg: map[string]*eval.Scope{
			"main": &eval.Scope{Var: map[string]*eval.Variable{}},
		},
	}
	p := parser.New()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("ng> ")
	for scanner.Scan() {
		p.Add(append(scanner.Bytes(), '\n'))
		var partial bool
	parse:
		for {
			select {
			case res := <-p.Result:
				v, err := prg.Eval(res.Stmt)
				if err != nil {
					fmt.Printf("error: %v\n", err)
					continue
				}
				switch len(v) {
				case 0:
				case 1:
					fmt.Println(v[0])
				default:
					fmt.Println(v)
				}
			case partial = <-p.Waiting:
				break parse
			}
		}

		if partial {
			fmt.Print("..> ")
			continue
		}
		fmt.Print("ng> ")
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
