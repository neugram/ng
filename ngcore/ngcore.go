// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ngcore presents a Neugram interpreter interface and
// the associated machinery that depends on the state of the
// interpreter, such as code completion.
//
// This package is designed for embedding Neugram into a program.
package ngcore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"

	"neugram.io/ng/eval"
	"neugram.io/ng/eval/environ"
	"neugram.io/ng/eval/shell"
	"neugram.io/ng/format"
	"neugram.io/ng/parser"
)

type Neugram struct {
	// TODO: Universe *eval.Scope for session initialization

	mu       sync.Mutex // guards map, not interior of *Session obj
	sessions map[string]*Session
}

func New() *Neugram {
	return &Neugram{
		sessions: make(map[string]*Session),
	}
}

type Session struct {
	Parser     *parser.Parser
	Program    *eval.Program
	ShellState *shell.State

	// Stdin, Stdout, and Stderr are the stdio to hook up to evaluator.
	// Nominally Stdout and Stderr are io.Writers.
	// If these interfaces have the concrete type *os.File the underlying
	// file descriptor is passed directly to shell jobs.
	Stdin  *os.File
	Stdout io.Writer
	Stderr io.Writer

	ExecCount int // number of statements executed
	// TODO: record execution statement history here

	name    string
	neugram *Neugram
}

func (n *Neugram) NewSession(ctx context.Context, name string) (*Session, error) {
	s := n.newSession(ctx, name)

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.sessions[name] != nil {
		return nil, fmt.Errorf("neugram: session %q already exists", name)
	}
	n.sessions[name] = s
	return s, nil
}

func (n *Neugram) GetSession(name string) *Session {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.sessions[name]
}

func (n *Neugram) newSession(ctx context.Context, name string) *Session {
	// TODO: default shell state
	shellState := &shell.State{
		Env:   environ.New(),
		Alias: environ.New(),
	}

	// TODO: wire ctx into *eval.Program for cancellation (replace sigint channel)
	return &Session{
		Parser:     parser.New(name),
		Program:    eval.New("session-"+name, shellState),
		ShellState: shellState,
		name:       name,
		neugram:    n,
	}
}

func (n *Neugram) GetOrNewSession(ctx context.Context, name string) *Session {
	n.mu.Lock()
	defer n.mu.Unlock()
	if s := n.sessions[name]; s != nil {
		return s
	}
	s := n.newSession(ctx, name)
	n.sessions[name] = s
	return s
}

func (s *Session) Exec(src []byte) error {
	stdout := s.Stdout
	if stdout == nil {
		stdout = ioutil.Discard
	}
	stderr := s.Stderr
	if stderr == nil {
		stderr = ioutil.Discard
	}

	res := s.Parser.ParseLine(src)
	if len(res.Errs) > 0 {
		errs := make([]error, len(res.Errs))
		for i, err := range res.Errs {
			errs[i] = err
		}
		return Error{Phase: "parser", List: errs}
	}
	for _, stmt := range res.Stmts {
		v, err := s.Program.Eval(stmt, nil)
		if err != nil {
			str := err.Error()
			if strings.HasPrefix(str, "typecheck: ") { // TODO: gross
				return Error{
					Phase: "typecheck",
					List: []error{
						errors.New(strings.TrimPrefix(str, "typecheck: ")),
					},
				}
			}
			return Error{Phase: "eval", List: []error{err}}
		}
		s.ExecCount++
		if len(v) > 1 {
			fmt.Fprint(stdout, "(")
		}
		for i, val := range v {
			if i > 0 {
				fmt.Fprint(stdout, ", ")
			}
			if val == (reflect.Value{}) {
				fmt.Print(stdout, "<nil>")
				continue
			}
			switch v := val.Interface().(type) {
			case eval.UntypedInt:
				fmt.Fprint(stdout, v.String())
			case eval.UntypedFloat:
				fmt.Fprint(stdout, v.String())
			case eval.UntypedComplex:
				fmt.Fprint(stdout, v.String())
			case eval.UntypedString:
				fmt.Fprint(stdout, v.String)
			case eval.UntypedRune:
				fmt.Fprintf(stdout, "%v", v.Rune)
			case eval.UntypedBool:
				fmt.Fprint(stdout, v.Bool)
			default:
				fmt.Fprint(stdout, format.Debug(v))
			}
		}
		if len(v) > 1 {
			fmt.Fprintln(stdout, ")")
		} else if len(v) == 1 {
			fmt.Fprintln(stdout, "")
		}
	}
	for _, cmd := range res.Cmds {
		outdone := make(chan struct{})
		var outr, outw, errw *os.File
		if f, isFile := stdout.(*os.File); isFile {
			outw = f
		}
		if f, isFile := stderr.(*os.File); isFile {
			errw = f
		}
		if outw == nil || errw == nil {
			r, w, err := os.Pipe()
			if err != nil {
				return Error{Phase: "shellexec", List: []error{err}}
			}
			outr = r
			if outw == nil {
				outw = w
			}
			if errw == nil {
				errw = w
			}
			go func() {
				io.Copy(stdout, outr)
				close(outdone)
			}()
		}

		j := &shell.Job{
			State:  s.ShellState,
			Cmd:    cmd,
			Params: s.Program,
			Stdin:  s.Stdin,
			Stdout: outw,
			Stderr: errw,
		}
		if err := j.Start(); err != nil {
			fmt.Fprintln(stdout, err)
			continue
		}
		done, err := j.Wait()
		if err != nil {
			return Error{Phase: "shell", List: []error{err}}
		}
		if outw != nil {
			outw.Close()
			<-outdone
		}
		if !done {
			break // TODO not right, instead we should just have one cmd, not Cmds here.
		}
	}
	return nil
}

func (s *Session) Close() {
	s.neugram.mu.Lock()
	delete(s.neugram.sessions, s.name)
	s.neugram.mu.Unlock()

	// TODO: clean up Program
}

type Error struct {
	Phase string
	List  []error
}

func (e Error) Error() string {
	listStr := ""
	switch len(e.List) {
	case 0:
		listStr = "empty error list"
	case 1:
		listStr = e.List[0].Error()
	default:
		listStr = fmt.Sprintf("%v (and %d more)", e.List[0].Error(), len(e.List)-1)
	}
	return fmt.Sprintf("ng: %s: %v", e.Phase, listStr)
}
