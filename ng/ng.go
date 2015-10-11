package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"numgrad.io/eval"
	"numgrad.io/lang/token"
	"numgrad.io/lang/typecheck"
	"numgrad.io/parser"

	"github.com/peterh/liner"
)

func main() {
	prg := &eval.Program{
		Pkg: map[string]*eval.Scope{
			"main": &eval.Scope{Var: map[string]*eval.Variable{}},
		},
		Types: typecheck.New(),
	}
	p := parser.New()

	line := liner.NewLiner()
	defer line.Close()
	line.SetCompleter(func(line string) []string {
		// TODO match on word not line.
		// TODO walk the scope for possible names.
		var res []string
		for keyword := range token.Keywords {
			if strings.HasPrefix(keyword, line) {
				res = append(res, keyword)
			}
		}
		return res
	})
	if f, err := os.Open(historyFile); err == nil {
		line.ReadHistory(f)
		f.Close()
	}
	go historyWriter()

	scanner := bufio.NewScanner(os.Stdin)
	prompt := "ng> "
	for {
		data, err := line.Prompt(prompt)
		if err != nil {
			line.Close()
			if err == io.EOF {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "ng: error reading input: %v", err)
			os.Exit(1)
		}
		line.AppendHistory(data)
		history <- data
		res := p.ParseLine([]byte(data))

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
		for _, cmd := range res.Cmds {
			if err := prg.EvalCmd(cmd); err != nil {
				fmt.Printf("cmmand error: %v\n", err)
				continue
			}
		}

		switch res.State {
		case parser.StateUnknown:
			prompt = "??> "
		case parser.StateStmt:
			prompt = "ng> "
		case parser.StateStmtPartial:
			prompt = "..> "
		case parser.StateCmd:
			prompt = "$ "
		case parser.StateCmdPartial:
			prompt = "..$ "
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

var (
	historyFile = ""
	history     = make(chan string, 1)
)

func init() {
	if home := os.Getenv("HOME"); home != "" {
		historyFile = filepath.Join(home, ".ng_history")
	}
}

func historyWriter() {
	var batch []string
	ticker := time.Tick(250 * time.Millisecond)
	for {
		select {
		case line := <-history:
			batch = append(batch, line)
		case <-ticker:
			if len(batch) > 0 && historyFile != "" {
				f, err := os.OpenFile(historyFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
				if err == nil {
					for _, line := range batch {
						fmt.Fprintf(f, "%s\n", line)
					}
					f.Close()
				}
			}
			batch = nil
		}
	}
}
