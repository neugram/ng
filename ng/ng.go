// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"neugram.io/eval"
	"neugram.io/eval/environ"
	"neugram.io/eval/shell"
	"neugram.io/lang/tipe"
	"neugram.io/parser"

	"github.com/kr/pretty"
	"github.com/peterh/liner"
)

var (
	origMode liner.ModeApplier

	lineNg        *liner.State // ng-mode line reader
	historyNgFile = ""
	historyNg     = make(chan string, 1)
	historyShFile = ""
	historySh     = make(chan string, 1)

	prg *eval.Program
)

func exit(code int) {
	lineNg.Close()
	os.Exit(code)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ng: "+format, args...)
	exit(1)
}

func mode() liner.ModeApplier {
	m, err := liner.TerminalMode()
	if err != nil {
		exitf("terminal mode: %v", err)
	}
	return m
}

func main() {
	shell.Init()

	origMode = mode()
	lineNg = liner.NewLiner()
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

func loop() {
	p := parser.New()
	prg = eval.New()
	shell.Env = prg.Environ()
	shell.Alias = prg.Alias()

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

	lineNg.SetTabCompletionStyle(liner.TabPrints)
	lineNg.SetWordCompleter(completer)

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

	state := parser.StateStmt
	if os.Args[0] == "ngsh" || os.Args[0] == "-ngsh" {
		initFile := filepath.Join(os.Getenv("HOME"), ".ngshinit")
		if f, err := os.Open(initFile); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				res := p.ParseLine(scanner.Bytes())
				handleResult(res)
				state = res.State
			}
			if err := scanner.Err(); err != nil {
				exitf(".ngshinit: %v", err)
			}
			f.Close()
		}
		switch state {
		case parser.StateStmtPartial, parser.StateCmdPartial:
			exitf(".ngshinit: ends in a partial statement")
		case parser.StateStmt:
			res := p.ParseLine([]byte("$$"))
			handleResult(res)
			state = res.State
		}
	}

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
			mode, prompt, history = "sh", ps1(env), historySh
		case parser.StateCmdPartial:
			mode, prompt, history = "sh", "..$ ", historySh
		default:
			exitf("unkown parser state: %v", state)
		}
		lineNg.SetMode(mode)
		data, err := lineNg.Prompt(prompt)
		if err != nil {
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
		res := p.ParseLine([]byte(data))
		handleResult(res)
		state = res.State
	}
}

func handleResult(res parser.Result) {
	for _, s := range res.Stmts {
		v, err := prg.Eval(s)
		if err != nil {
			fmt.Printf("eval error: %v\n", err)
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
			} else {
				pretty.Print(val.Interface())
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
	//editMode := mode()
	//origMode.ApplyMode()
	for _, cmd := range res.Cmds {
		j := &shell.Job{
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
