// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"neugram.io/ng/eval"
	"neugram.io/ng/eval/environ"
	"neugram.io/ng/eval/shell"
	"neugram.io/ng/format"
	"neugram.io/ng/jupyter"
	"neugram.io/ng/parser"
	"neugram.io/ng/tipe"

	"github.com/peterh/liner"
)

var (
	origMode liner.ModeApplier

	lineNg        *liner.State // ng-mode line reader
	historyNgFile = ""
	historyNg     = make(chan string, 1)
	historyShFile = ""
	historySh     = make(chan string, 1)
	sigint        = make(chan os.Signal)

	p          *parser.Parser
	prg        *eval.Program
	shellState *shell.State
)

func exit(code int) {
	if lineNg != nil {
		lineNg.Close()
	}
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(code)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ng: "+format+"\n", args...)
	exit(1)
}

func mode() liner.ModeApplier {
	m, err := liner.TerminalMode()
	if err != nil {
		exitf("terminal mode: %v", err)
	}
	return m
}

const usageLine = "ng [programfile | -e cmd] [arguments]"

func usage() {
	fmt.Fprintf(os.Stderr, `ng - neugram scripting language and shell

Usage:
	%s

Options:
`, usageLine)
	flag.PrintDefaults()
}

func main() {
	shell.Init()

	flagJupyter := flag.String("jupyter", "", "path to jupyter kernel connection file")
	help := flag.Bool("h", false, "display help message and exit")
	e := flag.String("e", "", "program passed as a string")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s\n", usageLine)
		os.Exit(1)
	}
	flag.Parse()

	if *help {
		usage()
		os.Exit(0)
	}
	if *flagJupyter != "" {
		err := jupyter.Run(context.Background(), *flagJupyter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *e != "" {
		initProgram(filepath.Join(cwd, "ng-arg"))
		res := p.ParseLine([]byte(*e))
		handleResult(res)
		return
	}
	if args := flag.Args(); len(args) > 0 {
		// TODO: plumb through the rest of the args
		path := args[0]
		initProgram(path)
		f, err := os.Open(path)
		if err != nil {
			exitf("%v", err)
		}
		state, err := runFile(f)
		if err != nil {
			exitf("%v", err)
		}
		if state == parser.StateCmd {
			exitf("%s: ends in an unclosed shell statement", args[0])
		}
		return
	}

	lineNg = liner.NewLiner()
	defer lineNg.Close()

	loop()
}

func setWindowSize(env map[interface{}]interface{}) {
	// TODO windowsize
	// TODO
	// TODO
	// TODO
	/*
		rows, cols, err := job.WindowSize(os.Stderr.Fd())
		if err != nil {
			fmt.Printf("ng: could not get window size: %v\n", err)
		} else {
			// TODO: these are meant to be shell variables, not
			// environment variables. But then, how do programs
			// like `ls` read them?
			env["LINES"] = strconv.Itoa(rows)
			env["COLUMNS"] = strconv.Itoa(cols)
		}
	*/
}

func ps1(env *environ.Environ) string {
	v := env.Get("PS1")
	if v == "" {
		return "ng$ "
	}
	if strings.IndexByte(v, '\\') == -1 {
		return v
	}
	var buf []byte
	for {
		i := strings.IndexByte(v, '\\')
		if i == -1 || i == len(v)-1 {
			break
		}
		buf = append(buf, v[:i]...)
		b := v[i+1]
		v = v[i+2:]
		switch b {
		case 'h', 'H':
			out, err := exec.Command("hostname").CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ng: %v\n", err)
				continue
			}
			if b == 'h' {
				if i := bytes.IndexByte(out, '.'); i >= 0 {
					out = out[:i]
				}
			}
			if len(out) > 0 && out[len(out)-1] == '\n' {
				out = out[:len(out)-1]
			}
			buf = append(buf, out...)
		case 'n':
			buf = append(buf, '\n')
		case 'w', 'W':
			cwd := env.Get("PWD")
			if home := env.Get("HOME"); home != "" {
				cwd = strings.Replace(cwd, home, "~", 1)
			}
			if b == 'W' {
				cwd = filepath.Base(cwd)
			}
			buf = append(buf, cwd...)
		}
		// TODO: '!', '#', '$', 'nnn', 's', 'j', and more.
	}
	buf = append(buf, v...)
	return string(buf)
}

var cwd string

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

func initProgram(path string) {
	p = parser.New()
	shellState = &shell.State{
		Env:   environ.New(),
		Alias: environ.New(),
	}
	prg = eval.New(path, shellState)

	// TODO this env setup could be done in neugram code
	env := prg.Environ()
	for _, s := range os.Environ() {
		i := strings.Index(s, "=")
		env.Set(s[:i], s[i+1:])
	}
	wd, err := os.Getwd()
	if err == nil {
		env.Set("PWD", wd)
	}
	//setWindowSize(env)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		for {
			s := <-sig
			select {
			case sigint <- s:
			case <-time.After(500 * time.Millisecond):
				// The evaluator has not handled the signal
				// promptly. There are several possible
				// reasons for this. The most likely right now
				// is the evaluator is in arbitrary Go code,
				// which does not have a way to be preempted.
				// It is also possible we have run into a
				// bug in the evaluator.
				//
				// Either way, instead of being one of those
				// obnoxious programs that refuses to respond
				// to Ctrl-C, be overly agressive and let the
				// entire ng process exit.
				//
				// This is bad if you use ng as your primary
				// shell, but good if you invoke ng to handle
				// scripts.
				fmt.Fprintf(os.Stderr, "ng: exiting on interrupt\n")
				exit(1)
			}
		}
	}()
}

func runFile(f *os.File) (parser.ParserState, error) {
	state := parser.StateStmt
	scanner := bufio.NewScanner(f)
	for i := 0; scanner.Scan(); i++ {
		b := scanner.Bytes()
		if i == 0 && len(b) > 2 && b[0] == '#' && b[1] == '!' { // shebang
			continue
		}
		res := p.ParseLine(b)
		handleResult(res)
		state = res.State
	}
	if err := scanner.Err(); err != nil {
		return state, fmt.Errorf("%s: %v", f.Name(), err)
	}
	switch state {
	case parser.StateStmtPartial, parser.StateCmdPartial:
		return state, fmt.Errorf("%s: ends in a partial statement", f.Name())
	default:
		return state, nil
	}
}

func loop() {
	path := filepath.Join(cwd, "ng-interactive")
	initProgram(path)

	state := parser.StateStmt
	if os.Args[0] == "ngsh" || os.Args[0] == "-ngsh" {
		initFile := filepath.Join(os.Getenv("HOME"), ".ngshinit")
		if f, err := os.Open(initFile); err == nil {
			var err error
			state, err = runFile(f)
			f.Close()
			if err != nil {
				exitf("%v", err)
			}
		}
		if state == parser.StateStmt {
			res := p.ParseLine([]byte("$$"))
			handleResult(res)
			state = res.State
		}
	}

	lineNg.SetTabCompletionStyle(liner.TabPrints)
	lineNg.SetWordCompleter(completer)
	lineNg.SetCtrlCAborts(true)

	if f, err := os.Open(historyShFile); err == nil {
		lineNg.SetMode("sh")
		lineNg.ReadHistory(f)
		f.Close()
	}
	go historyWriter(historyShFile, historySh)

	if f, err := os.Open(historyNgFile); err == nil {
		lineNg.SetMode("ng")
		lineNg.ReadHistory(f)
		f.Close()
	}
	go historyWriter(historyNgFile, historyNg)

	for {
		var (
			mode    string
			prompt  string
			history chan string
		)
		switch state {
		case parser.StateUnknown:
			mode, prompt, history = "ng", "??> ", historyNg
		case parser.StateStmt:
			mode, prompt, history = "ng", "ng> ", historyNg
		case parser.StateStmtPartial:
			mode, prompt, history = "ng", "..> ", historyNg
		case parser.StateCmd:
			mode, prompt, history = "sh", ps1(prg.Environ()), historySh
		case parser.StateCmdPartial:
			mode, prompt, history = "sh", "..$ ", historySh
		default:
			exitf("unkown parser state: %v", state)
		}
		lineNg.SetMode(mode)
		data, err := lineNg.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			switch state {
			case parser.StateStmtPartial:
				fmt.Printf("TODO interrupt partial statement\n")
			case parser.StateCmdPartial:
				fmt.Printf("TODO interrupt partial command\n")
			}
		} else if err != nil {
			if err == io.EOF {
				exit(0)
			}
			exitf("error reading input: %v", err)
		}
		if data == "" {
			continue
		}
		lineNg.AppendHistory(mode, data)
		history <- data
		select { // drain sigint
		case <-sigint:
		default:
		}
		res := p.ParseLine([]byte(data))
		handleResult(res)
		state = res.State
	}
}

func handleResult(res parser.Result) {
	// TODO: use ngcore for this
	for _, s := range res.Stmts {
		v, err := prg.Eval(s, sigint)
		if err != nil {
			fmt.Printf("ng: %v\n", err)
			continue
		}
		if len(v) > 1 {
			fmt.Print("(")
		}
		for i, val := range v {
			if i > 0 {
				fmt.Print(", ")
			}
			if val == (reflect.Value{}) {
				fmt.Print("<nil>")
				continue
			}
			switch v := val.Interface().(type) {
			case eval.UntypedInt:
				fmt.Print(v.String())
			case eval.UntypedFloat:
				fmt.Print(v.String())
			case eval.UntypedComplex:
				fmt.Print(v.String())
			case eval.UntypedString:
				fmt.Print(v.String)
			case eval.UntypedRune:
				fmt.Printf("%v", v.Rune)
			case eval.UntypedBool:
				fmt.Print(v.Bool)
			default:
				fmt.Print(format.Debug(v))
			}
		}
		if len(v) > 1 {
			fmt.Println(")")
		} else if len(v) == 1 {
			fmt.Println("")
		}
	}
	for _, err := range res.Errs {
		fmt.Println(err.Error())
	}
	for _, cmd := range res.Cmds {
		j := &shell.Job{
			State:  shellState,
			Cmd:    cmd,
			Params: prg,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		if err := j.Start(); err != nil {
			fmt.Println(err)
			continue
		}
		done, err := j.Wait()
		if err != nil {
			fmt.Println(err)
			continue
		}
		if !done {
			break // TODO not right, instead we should just have one cmd, not Cmds here.
		}
	}
	//editMode.ApplyMode()
}

func printValue(t tipe.Type, v interface{}) {
	// This is, effectively, a primitive type-aware printf implementation
	// that understands the neugram evaluator data layout. A far better
	// version of this would be an "ngfmt" package, that implemented the
	// printing command in neugram, using a "ngreflect" package. But it
	// will be a while until I build a reflect package, so this will have
	// to do.
	//
	// Still: avoid putting too much machinary in this. At some point soon
	// it's not worth the effort.
	/*switch t := tipe.Underlying(t).(type) {
	case *tipe.Struct:
	fmt.Print("{")
	for i, name := range t.FieldNames {
		fmt.Printf("%s: ", name)
		printValue(t.Fields[i], v.(*eval.StructVal).Fields[i].Value)
		if i < len(t.FieldNames)-1 {
			fmt.Print(", ")
		}
	}
	fmt.Print("}")
	default:
	}*/
	fmt.Print(v)
}

func init() {
	if home := os.Getenv("HOME"); home != "" {
		historyNgFile = filepath.Join(home, ".ng_history")
		historyShFile = filepath.Join(home, ".ngsh_history")
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
				// TODO: FcntlFlock
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
