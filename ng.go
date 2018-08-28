// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"neugram.io/ng/eval/shell"
	"neugram.io/ng/gengo"
	"neugram.io/ng/jupyter"
	"neugram.io/ng/ngcore"
	"neugram.io/ng/parser"
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

		initSession(ng)
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

		initSession(ng)
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

var cwd string

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

func initSession(ng *ngcore.Session) {
	ng.Stdin = os.Stdin
	ng.Stdout = os.Stdout
	ng.Stderr = os.Stderr

	// TODO this env setup could be done in neugram code
	env := ng.Program.Environ()
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
				// to Ctrl-C, be overly aggressive and let the
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

	initSession(ng)

	err = ng.Run(ctx, startInShell, sigint)
	if err != nil {
		exitf("%v", err)
	}
	exit(0)
}
