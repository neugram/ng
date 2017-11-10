// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package shell

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"neugram.io/ng/eval/environ"
	"neugram.io/ng/expr"
	"neugram.io/ng/format"
	"neugram.io/ng/token"
)

type State struct {
	Env   *environ.Environ
	Alias *environ.Environ

	bgMu sync.Mutex
	bg   []*Job
}

type Params interface {
	Get(name string) string
	Set(name, value string)
}

type paramset interface {
	Get(name string) string
}

type Job struct {
	State  *State
	Cmd    *expr.ShellList
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
	Params Params

	mu      sync.Mutex
	err     error
	pgid    int
	termios syscall.Termios
	cond    sync.Cond
	done    bool
	running bool
}

func (j *Job) Start() (err error) {
	if interactive {
		shellState, err = tcgetattr(os.Stdin.Fd())
		if err != nil {
			return err
		}
		if err := tcsetattr(os.Stdin.Fd(), &basicState); err != nil {
			return err
		}
	}

	j.mu.Lock()
	j.cond.L = &j.mu
	j.running = true
	j.mu.Unlock()

	go j.exec()
	return nil
}

func (j *Job) Result() (done bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.done {
		return false, nil
	}
	return true, j.err
}

func (j *Job) Continue() error {
	j.mu.Lock()
	if j.done {
		j.mu.Unlock()
		return nil
	}

	if interactive {
		if err := tcsetpgrp(os.Stdin.Fd(), j.pgid); err != nil {
			j.mu.Unlock()
			return err
		}
		if err := tcsetattr(os.Stdin.Fd(), &j.termios); err != nil {
			j.mu.Unlock()
			return err
		}
	}

	j.running = true
	syscall.Kill(-j.pgid, syscall.SIGCONT)

	j.mu.Unlock()

	_, err := j.Wait()
	return err
}

func shellListString(cmd *expr.ShellList) string {
	return format.Expr(cmd)
}

// Wait waits until the job is stopped or complete.
func (j *Job) Wait() (done bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	for j.running {
		j.cond.Wait()
	}
	if !j.done {
		// Stopped, add Job to bg and save terminal.
		if interactive {
			j.termios, err = tcgetattr(os.Stdin.Fd())
			if err != nil {
				fmt.Fprintf(j.Stderr, "on stop: %v", err)
			}
		}
		j.State.bgAdd(j)
	}

	if interactive {
		// move the shell process to the foreground
		if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
			fmt.Fprintf(j.Stderr, "on stop: %v", err)
		}
		if err := tcsetattr(os.Stdin.Fd(), &shellState); err != nil {
			fmt.Fprintf(j.Stderr, "on stop: %v", err)
		}
	}

	return j.done, j.err
}

// exec traverses the Cmd tree, starting procs.
//
// Some of the traversal blocks until certain procs are running or
// complete, meaning exec lives until the job is complete.
func (j *Job) exec() {
	err := j.execShellList(j.Cmd, stdio{j.Stdin, j.Stdout, j.Stderr})

	j.mu.Lock()
	j.err = err
	j.running = false
	j.done = true
	j.cond.Broadcast()
	j.mu.Unlock()
}

type stdio struct {
	in  *os.File
	out *os.File
	err *os.File
}

func (j *Job) execShellList(cmd *expr.ShellList, sio stdio) error {
	for _, andor := range cmd.AndOr {
		if err := j.execShellAndOr(andor, sio); err != nil {
			return err
		}
	}
	return nil
}

func (j *Job) execShellAndOr(andor *expr.ShellAndOr, sio stdio) error {
	for i, p := range andor.Pipeline {
		err := j.execPipeline(p, sio)
		if i < len(andor.Pipeline)-1 {
			switch andor.Sep[i] {
			case token.LogicalAnd:
				if err != nil {
					return err
				}
			case token.LogicalOr:
				if err == nil {
					return nil
				}
			default:
				panic("unknown AndOr separator: " + andor.Sep[i].String())
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (j *Job) execPipeline(plcmd *expr.ShellPipeline, sio stdio) (err error) {
	if interactive && j.pgid == 0 && len(plcmd.Cmd) > 1 {
		// All the processes of a pipeline run with the same
		// process group ID. To do this, a shell will typically
		// use the pid of the first process as the pgid for the
		// entire pipeline.
		//
		// This technique has an edge case. If the first process
		// exits before the second gets a chance to start, then
		// the pgid is invalid and the pipeline will fail to start.
		// An easy way to run into this is if the first process is
		// a short echo, that can fit its entire output into the
		// kernel pipe buffer. For example:
		//
		//	echo hello | cat
		//
		// The solution adopted by typical shells is pipe
		// communication between the fork and exec calls of the
		// first process. The first process waits for the pipe to
		// close, and the shell closes it after the whole pipeline
		// is started, effectively pausing the first process
		// before it starts.
		//
		// That's not an option for us if we use the syscall
		// package's implementation of ForkExec. Rather than
		// reinvent the wheel, we try a different trick: we start
		// a well-behaved placeholder process, the pgidLeader, to
		// pin a pgid for the duration of pipeline creation.
		//
		// What an unusual contraption.
		pgidLeader, err := startPgidLeader()
		if err != nil {
			return err
		}
		j.pgid = pgidLeader.Pid
		defer func() {
			pgidLeader.Kill()
			j.pgid = 0
		}()
	}
	defer func() {
		j.pgid = 0
	}()

	sios := make([]stdio, len(plcmd.Cmd))
	sios[0].in = sio.in
	sios[len(sios)-1].out = sio.out
	for i := range sios {
		sios[i].err = sio.err
	}
	defer func() {
		if err != nil {
			for i := 0; i < len(sios)-1; i++ {
				if sios[i].out != nil {
					sios[i].out.Close()
				}
				if sios[i+1].in != nil {
					sios[i+1].in.Close()
				}
			}
		}
	}()
	for i := 0; i < len(sios)-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		sios[i].out = w
		sios[i+1].in = r
	}
	pl := &pipeline{
		job: j,
	}
	for i, cmd := range plcmd.Cmd {
		if cmd.Subshell != nil {
			return fmt.Errorf("missing subshell support") // TODO
		}
		p, err := j.setupSimpleCmd(cmd.SimpleCmd, sios[i])
		if err != nil {
			return err
		}
		if p != nil {
			pl.proc = append(pl.proc, p)
		}
	}
	if len(pl.proc) > 0 {
		if err := pl.start(); err != nil {
			return err
		}
		if err := pl.waitUntilDone(); err != nil {
			return err
		}
	}
	return nil
}

func (j *Job) setupSimpleCmd(cmd *expr.ShellSimpleCmd, sio stdio) (*proc, error) {
	if len(cmd.Args) == 0 {
		for _, v := range cmd.Assign {
			j.Params.Set(v.Key, v.Value)
		}
		return nil, nil
	}
	argv, err := expansion(cmd.Args, j.Params)
	if err != nil {
		return nil, err
	}
	if a := j.State.Alias.Get(argv[0]); a != "" {
		// TODO: This is entirely wrong. The alias string needs to be
		// parsed like a typical shell command. That is:
		//	alias["gsm"] = `go build "-ldflags=-w -s"`
		// should be three args, not four.
		aliasArgs := strings.Split(a, " ")
		argv = append(aliasArgs, argv[1:]...)
	}
	switch argv[0] {
	case "cd":
		dir := ""
		if len(argv) == 1 {
			dir = j.State.Env.Get("HOME")
		} else {
			dir = argv[1]
		}
		wd := ""
		if filepath.IsAbs(dir) {
			wd = filepath.Clean(dir)
		} else {
			wd = filepath.Join(j.State.Env.Get("PWD"), dir)
		}
		if err := os.Chdir(wd); err != nil {
			return nil, err
		}
		j.State.Env.Set("PWD", wd)
		fmt.Fprintf(os.Stdout, "%s\n", wd)
		return nil, nil
	case "fg":
		return nil, j.State.bgFg(strings.Join(argv[1:], " "))
	case "jobs":
		j.State.bgList(j.Stderr)
		return nil, nil
	case "export":
		return nil, j.export(argv[1:])
	case "exit", "logout":
		return nil, fmt.Errorf("ng does not know %q, try $$", argv[0])
	}
	env := j.State.Env.List()
	if len(cmd.Assign) != 0 {
		baseEnv := env
		env = make([]string, 0, len(cmd.Assign)+len(baseEnv))
		for _, kv := range cmd.Assign {
			env = append(env, kv.Key+"="+kv.Value)
		}
		env = append(env, baseEnv...)
	}
	p := &proc{
		job:  j,
		argv: argv,
		sio:  sio,
		env:  env,
	}
	for _, r := range cmd.Redirect {
		switch r.Token {
		case token.Greater, token.TwoGreater, token.AndGreater:
			flag := os.O_RDWR | os.O_CREATE
			if r.Token == token.Greater || r.Token == token.AndGreater {
				flag |= os.O_TRUNC
			} else {
				flag |= os.O_APPEND
			}
			f, err := os.OpenFile(r.Filename, flag, 0666)
			if err != nil {
				return nil, err
			}
			if r.Token == token.AndGreater {
				p.sio.out = f
				p.sio.err = f
			} else if r.Number == nil || *r.Number == 1 {
				p.sio.out = f
			} else if *r.Number == 2 {
				p.sio.err = f
			}
		case token.GreaterAnd:
			dstnum, err := strconv.Atoi(r.Filename)
			if err != nil {
				return nil, fmt.Errorf("bad redirect target: %q", r.Filename)
			}
			var dst *os.File
			switch dstnum {
			case 1:
				dst = p.sio.out
			case 2:
				dst = p.sio.err
			}
			switch *r.Number {
			case 1:
				p.sio.out = dst
			case 2:
				p.sio.err = dst
			}
		case token.Less:
			return nil, fmt.Errorf("TODO: %s", r.Token)
		default:
			return nil, fmt.Errorf("unknown shell redirect %s", r.Token)
		}
	}
	return p, nil
}

func startPgidLeader() (*os.Process, error) {
	path, err := executable()
	if err != nil {
		return nil, fmt.Errorf("pgidleader init: %v", err)
	}
	argv := []string{os.Args[0], "-pgidleader"}

	p, err := os.StartProcess(path, argv, &os.ProcAttr{
		Files: []*os.File{}, //r, os.Stdout, os.Stderr},
		Sys: &syscall.SysProcAttr{
			Setpgid: true, // job gets new pgid
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pgidleader init start: %v", err)
	}
	return p, nil
}

type pipeline struct {
	job  *Job
	proc []*proc
	//pgid int
}

func (pl *pipeline) start() (err error) {
	pl.job.mu.Lock()
	defer pl.job.mu.Unlock()

	for _, p := range pl.proc {
		p.path, err = findExecInPath(p.argv[0], pl.job.State.Env)
		if err != nil {
			return err
		}
	}

	defer func() {
		if err != nil {
			for _, p := range pl.proc {
				if p.process != nil {
					p.process.Kill()
					p.process = nil
				}
			}
		}
	}()
	for i, p := range pl.proc {
		attr := &os.ProcAttr{
			Env:   p.env,
			Files: []*os.File{p.sio.in, p.sio.out, p.sio.err},
		}
		attr.Sys = &syscall.SysProcAttr{
			Setpgid:    true, // job gets new pgid
			Foreground: interactive,
			Pgid:       pl.job.pgid,
		}
		p.process, err = os.StartProcess(p.path, p.argv, attr)
		if i == 0 && p.sio.in != p.job.Stdin {
			p.sio.in.Close()
		}
		if i == len(pl.proc)-1 && p.sio.out != p.job.Stdout {
			p.sio.out.Close()
		}
		if err != nil {
			return err
		}

		if pl.job.pgid == 0 {
			pl.job.pgid, err = syscall.Getpgid(p.process.Pid)
			if err != nil {
				return fmt.Errorf("cannot get pgid of new process: %v", err)
			}
			if interactive {
				if err := tcsetpgrp(os.Stdin.Fd(), pl.job.pgid); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (pl *pipeline) waitUntilDone() error {
	var err error
	for _, p := range pl.proc {
		err = p.waitUntilDone()
	}
	return err
}

type exitError struct {
	code int
}

func (err exitError) Error() string { return fmt.Sprintf("exit code: %d", err.code) }

func (p *proc) waitUntilDone() error {
	pid := p.process.Pid
	//pid := pl.job.pgid
	for {
		wstatus := new(syscall.WaitStatus)
		_, err := syscall.Wait4(pid, wstatus, syscall.WUNTRACED|syscall.WCONTINUED, nil)
		switch {
		case err != nil || wstatus.Exited():
			// TODO: should we close these right after the process forks?
			if p.sio.in != p.job.Stdin {
				p.sio.in.Close()
			}
			if p.sio.out != p.job.Stdout {
				p.sio.out.Close()
			}
			//fmt.Fprintf(os.Stderr, "process exited with %v\n", err)
			if c := wstatus.ExitStatus(); c != 0 {
				return exitError{code: c}
			}
			return nil
		case wstatus.Stopped():
			p.job.cond.L.Lock()
			p.job.running = false
			p.job.cond.Broadcast()
			p.job.cond.L.Unlock()
		case wstatus.Continued():
			// BUG: on darwin at least, this isn't firing.
		case wstatus.Signaled():
			// ignore
		default:
			panic(fmt.Sprintf("unexpected wstatus: %#+v", wstatus))
		}
	}
}

type proc struct {
	job     *Job
	argv    []string
	env     []string
	path    string
	process *os.Process
	sio     stdio
}

// TODO: make interactive a property of a *shell.State.
// Ensure that only one State at a time in a process can be interactive.
var (
	interactive bool
	basicState  syscall.Termios
	shellState  syscall.Termios
	shellPgid   int
)

var jobSignals = []os.Signal{
	syscall.SIGQUIT,
	syscall.SIGTTOU,
	syscall.SIGTTIN,
}

// TODO: make this a method on shell.State
func Init() {
	if len(os.Args) == 2 && os.Args[1] == "-pgidleader" {
		select {}
	}

	var err error
	basicState, err = tcgetattr(os.Stdin.Fd())
	if err == nil {
		interactive = true
	}

	if interactive {
		// Become foreground process group.
		for {
			shellPgid, err = syscall.Getpgid(syscall.Getpid())
			if err != nil {
				panic(err)
			}
			foreground, err := tcgetpgrp(os.Stdin.Fd())
			if err != nil {
				panic(err)
			}
			if foreground == shellPgid {
				break
			}
			syscall.Kill(-shellPgid, syscall.SIGTTIN)
		}

		// We ignore SIGTSTP and SIGINT with signal.Notify to avoid setting
		// the signal to SIG_IGN, a state that exec(3) will pass on to
		// child processes, making it impossible to Ctrl+Z any process that
		// does not install its own SIGTSTP handler.
		ignoreCh := make(chan os.Signal, 1)
		go func() {
			for range ignoreCh {
			}
		}()
		signal.Notify(ignoreCh, syscall.SIGTSTP, syscall.SIGINT)
		signal.Ignore(jobSignals...)

		shellPgid = os.Getpid()
		if err := syscall.Setpgid(shellPgid, shellPgid); err != nil {
			panic(err)
		}
		if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
			panic(err)
		}
	}
}

func (s *State) bgAdd(j *Job) {
	s.bgMu.Lock()
	defer s.bgMu.Unlock()
	s.bg = append(s.bg, j)
	fmt.Fprintf(j.Stderr, "\n[%d]+  Stopped  %s\n", len(s.bg), shellListString(j.Cmd))
}

func (s *State) bgList(w io.Writer) {
	s.bgMu.Lock()
	defer s.bgMu.Unlock()
	for _, j := range s.bg {
		state := "Stopped"
		if j.running { // TODO: need to hold lock, but need to not deadlock
			state = "Running"
		}
		fmt.Fprintf(j.Stderr, "\n[%d]+  %s  %s\n", len(s.bg), state, shellListString(j.Cmd))
	}
}

func (s *State) bgFg(spec string) error {
	jobspec := 1
	var err error
	if spec != "" {
		jobspec, err = strconv.Atoi(spec)
	}
	if err != nil {
		return fmt.Errorf("fg: %v", err)
	}

	s.bgMu.Lock()
	if len(s.bg) == 0 {
		s.bgMu.Unlock()
		return fmt.Errorf("fg: no jobs\n")
	}
	if jobspec > len(s.bg) {
		s.bgMu.Unlock()
		return fmt.Errorf("fg: %d: no such job\n", jobspec)
	}
	j := s.bg[jobspec-1]
	s.bg = append(s.bg[:jobspec-1], s.bg[jobspec:]...)
	fmt.Fprintf(j.Stderr, "%s\n", shellListString(j.Cmd))
	s.bgMu.Unlock()
	return j.Continue()
}

func (j *Job) export(pairs []string) error {
	for _, p := range pairs {
		parts := strings.SplitN(p, "=", 2)
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}
		j.State.Env.Set(parts[0], val)
	}
	return nil
}
