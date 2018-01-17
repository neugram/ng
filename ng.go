// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"neugram.io/ng/eval"
	"neugram.io/ng/eval/environ"
	"neugram.io/ng/eval/shell"
	"neugram.io/ng/gengo"
	"neugram.io/ng/jupyter"
	"neugram.io/ng/ngcore"
	"neugram.io/ng/parser"

	"github.com/peterh/liner"
)

var (
	sigint = make(chan os.Signal)
	ng     = ngcore.New()
)

func exit(code int) {
	ng.Close()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(code)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ng: "+format+"\n", args...)
	exit(1)
}

const usageLine = "ng [programfile | -e cmd | -jupyter file] [arguments]"

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
	flagShell := flag.Bool("shell", false, "start in shell mode")
	flagHelp := flag.Bool("h", false, "display help message and exit")
	flagE := flag.String("e", "", "program passed as a string")
	flagO := flag.String("o", "", "compile the program to the named file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s\n", usageLine)
		os.Exit(1)
	}
	flag.Parse()

	if *flagHelp {
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
	if *flagE != "" {
		ng, err := ng.NewSession(context.Background(), filepath.Join(cwd, "ng-arg"), os.Environ())
		if err != nil {
			exitf("%v", err)
		}
		defer ng.Close()
		ng.Stdin = os.Stdin
		ng.Stdout = os.Stdout
		ng.Stderr = os.Stderr

		initProgram(ng.Program)
		vals, err := ng.Exec([]byte(*flagE))
		if err != nil {
			exitf("%v", err)
		}
		ng.Display(ng.Stdout, vals)
		return
	}
	if args := flag.Args(); len(args) > 0 {
		// TODO: plumb through the rest of the args
		path := args[0]
		if *flagO != "" {
			res, err := gengo.GenGo(path, "main")
			if err != nil {
				exitf("%v", err)
			}
			fmt.Printf("%s\n", res)
			exitf("TODO gengo")
			return
		}
		ng, err := ng.NewSession(context.Background(), path, os.Environ())
		if err != nil {
			exitf("%v", err)
		}
		defer ng.Close()
		ng.Stdin = os.Stdin
		ng.Stdout = os.Stdout
		ng.Stderr = os.Stderr

		initProgram(ng.Program)
		f, err := os.Open(path)
		if err != nil {
			exitf("%v", err)
		}
		defer f.Close()
		state, err := ng.RunScript(f)
		if err != nil {
			exitf("%v", err)
		}
		if state == parser.StateCmd {
			exitf("%s: ends in an unclosed shell statement", args[0])
		}
		return
	}
	if *flagO != "" {
		exitf("-o specified but no program file provided")
	}

	loop(context.Background(), os.Args[0] == "ngsh" || os.Args[0] == "-ngsh" || *flagShell)
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

func initProgram(prg *eval.Program) {
	// TODO this env setup could be done in neugram code
	env := prg.Environ()
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

func loop(ctx context.Context, startInShell bool) {
	path := filepath.Join(cwd, "ng-interactive")
	ng, err := ng.NewSession(ctx, path, os.Environ())
	if err != nil {
		exitf("%v", err)
	}
	defer ng.Close()
	ng.Stdin = os.Stdin
	ng.Stdout = os.Stdout
	ng.Stderr = os.Stderr
	initProgram(ng.Program)

	state := parser.StateStmt
	if startInShell {
		initFile := filepath.Join(os.Getenv("HOME"), ".ngshinit")
		if f, err := os.Open(initFile); err == nil {
			var err error
			state, err = ng.RunScript(f)
			f.Close()
			if err != nil {
				exitf("%v", err)
			}
		}
		if state == parser.StateStmt {
			res, err := ng.Exec([]byte("$$"))
			if err != nil {
				exitf("%v", err)
			}
			ng.Display(ng.Stdout, res)
			state = ng.ParserState
		}
	}

	ng.Liner.SetTabCompletionStyle(liner.TabPrints)
	ng.Liner.SetWordCompleter(ng.Completer)
	ng.Liner.SetCtrlCAborts(true)

	if home := os.Getenv("HOME"); home != "" {
		ng.History.Ng.Name = filepath.Join(home, ".ng_history")
		ng.History.Sh.Name = filepath.Join(home, ".ngsh_history")
	}

	if f, err := os.Open(ng.History.Sh.Name); err == nil {
		ng.Liner.SetMode("sh")
		ng.Liner.ReadHistory(f)
		f.Close()
	}
	go ng.History.Sh.Run(ctx)

	if f, err := os.Open(ng.History.Ng.Name); err == nil {
		ng.Liner.SetMode("ng")
		ng.Liner.ReadHistory(f)
		f.Close()
	}
	go ng.History.Ng.Run(ctx)

	for {
		var (
			mode    string
			prompt  string
			history chan string
		)
		switch state {
		case parser.StateUnknown:
			mode, prompt, history = "ng", "??> ", ng.History.Ng.Chan
		case parser.StateStmt:
			mode, prompt, history = "ng", "ng> ", ng.History.Ng.Chan
		case parser.StateStmtPartial:
			mode, prompt, history = "ng", "..> ", ng.History.Ng.Chan
		case parser.StateCmd:
			mode, prompt, history = "sh", ps1(ng.Program.Environ()), ng.History.Sh.Chan
		case parser.StateCmdPartial:
			mode, prompt, history = "sh", "..$ ", ng.History.Sh.Chan
		default:
			exitf("unkown parser state: %v", state)
		}
		ng.Liner.SetMode(mode)
		data, err := ng.Liner.Prompt(prompt)
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
		ng.Liner.AppendHistory(mode, data)
		history <- data
		select { // drain sigint
		case <-sigint:
		default:
		}
		res, err := ng.Exec([]byte(data))
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		ng.Display(ng.Stdout, res)
		state = ng.ParserState
	}
}
