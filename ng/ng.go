package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"numgrad.io/eval"
	"numgrad.io/lang/token"
	"numgrad.io/lang/typecheck"
	"numgrad.io/parser"

	"github.com/peterh/liner"
)

var (
	lineNg        *liner.State // ng-mode line reader
	historyNgFile = ""
	historyNg     = make(chan string, 1)
	lineSh        *liner.State // shell-mode line reader
	historyShFile = ""
	historySh     = make(chan string, 1)
)

func cleanup() {
	lineSh.Close()
	lineNg.Close()
}

func exit(code int) {
	cleanup()
	os.Exit(code)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ng: "+format, args...)
	exit(1)
}

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		exitf("interrupted")
	}()

	lineNg = liner.NewLiner()
	lineSh = liner.NewLiner()
	loop()
}

func loop() {
	prg := &eval.Program{
		Pkg: map[string]*eval.Scope{
			"main": &eval.Scope{Var: map[string]*eval.Variable{}},
		},
		Types: typecheck.New(),
	}
	p := parser.New()

	lineNg.SetCompleter(completer)
	if f, err := os.Open(historyNgFile); err == nil {
		lineNg.ReadHistory(f)
		f.Close()
	}
	go historyWriter(historyNgFile, historyNg)

	lineSh.SetCompleter(completerSh)
	if f, err := os.Open(historyShFile); err == nil {
		lineSh.ReadHistory(f)
		f.Close()
	}
	go historyWriter(historyShFile, historySh)

	state := parser.StateStmt
	for {
		var (
			prompt  string
			line    *liner.State
			history chan string
		)
		switch state {
		case parser.StateUnknown:
			prompt, line, history = "??> ", lineNg, historyNg
		case parser.StateStmt:
			prompt, line, history = "ng> ", lineNg, historyNg
		case parser.StateStmtPartial:
			prompt, line, history = "..> ", lineNg, historyNg
		case parser.StateCmd:
			prompt, line, history = "$ ", lineSh, historySh
		case parser.StateCmdPartial:
			prompt, line, history = "..$ ", lineSh, historySh
		default:
			exitf("unkown parser state: %v", state)
		}
		data, err := line.Prompt(prompt)
		if err != nil {
			if err == io.EOF {
				exit(0)
			}
			exitf("error reading input: %v", err)
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
		state = res.State
	}
}

func init() {
	if home := os.Getenv("HOME"); home != "" {
		historyNgFile = filepath.Join(home, ".ng_history")
		historyShFile = filepath.Join(home, ".ng_sh_history")
	}
}

func historyWriter(dst string, src <-chan string) {
	var batch []string
	ticker := time.Tick(250 * time.Millisecond)
	for {
		select {
		case line := <-src:
			batch = append(batch, line)
		case <-ticker:
			if len(batch) > 0 && dst != "" {
				f, err := os.OpenFile(dst, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
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

func completer(line string) []string {
	// TODO match on word not line.
	// TODO walk the scope for possible names.
	var res []string
	for keyword := range token.Keywords {
		if strings.HasPrefix(keyword, line) {
			res = append(res, keyword)
		}
	}
	return res
}

func completerSh(line string) []string {
	// TODO scan path
	var res []string
	return res
}
