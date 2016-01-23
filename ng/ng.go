package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"neugram.io/eval"
	"neugram.io/eval/shell"
	"neugram.io/lang/tipe"
	"neugram.io/parser"

	"github.com/peterh/liner"
)

var (
	origMode liner.ModeApplier

	lineNg        *liner.State // ng-mode line reader
	historyNgFile = ""
	historyNg     = make(chan string, 1)
	lineSh        *liner.State // shell-mode line reader
	historyShFile = ""
	historySh     = make(chan string, 1)

	prg *eval.Program
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

func mode() liner.ModeApplier {
	m, err := liner.TerminalMode()
	if err != nil {
		exitf("terminal mode: %v", err)
	}
	return m
}

func main() {
	shell.Init()

	// TODO
	// This is getting a bit absurd. It's time to write our own liner
	// package, one that supports the two modes we need and meshes well
	// with our own signal handling.
	ch := make(chan os.Signal, 1)
	winch1 := make(chan os.Signal, 1)
	winch2 := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for {
			sig := <-ch
			fmt.Printf("got signal: %s\n", sig)
			switch sig {
			case syscall.SIGWINCH:
				// TODO: don't drop this signal.
				// Instead, rewrite liner and make sure we
				// are always processing this.
				select {
				case winch1 <- syscall.SIGWINCH:
				default:
				}
				select {
				case winch2 <- syscall.SIGWINCH:
				default:
				}
			}
		}
	}()

	origMode = mode()
	lineNg = liner.NewLiner(os.Stdin, winch1)
	lineSh = liner.NewLiner(os.Stdin, winch2)
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

func loop() {
	p := parser.New()
	prg = eval.New()

	// TODO this env setup could be done in neugram code
	env := prg.Cur.Lookup("env").Value.(map[interface{}]interface{})
	for _, s := range os.Environ() {
		i := strings.Index(s, "=")
		env[s[:i]] = s[i+1:]
	}
	setWindowSize(env)

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
		if data == "" {
			continue
		}
		line.AppendHistory(data)
		history <- data
		res := p.ParseLine([]byte(data))

		for _, s := range res.Stmts {
			v, t, err := prg.Eval(s)
			if err != nil {
				fmt.Printf("eval error: %v\n", err)
				continue
			}
			switch len(v) {
			case 0:
			case 1:
				printValue(t, v[0])
				fmt.Print("\n")
			default:
				fmt.Println(v)
			}
		}
		for _, err := range res.Errs {
			fmt.Println(err.Error())
		}
		//editMode := mode()
		//origMode.ApplyMode()
		if len(res.Cmds) > 0 {
			shell.Env = prg.Environ()
		}
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
		state = res.State
	}
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
	switch t := tipe.Underlying(t).(type) {
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
		fmt.Print(v)
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
