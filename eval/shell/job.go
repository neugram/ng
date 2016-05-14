// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

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

	"neugram.io/eval/environ"
	"neugram.io/lang/expr"
)

var (
	Env   *environ.Environ
	Alias *environ.Environ
)

type Params interface {
	Get(name string) string
	Set(name, value string)
}

type Job struct {
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
	var s []string
	for _, c := range cmd.List {
		switch c := c.(type) {
		case *expr.ShellList:
			s = append(s, shellListString(c))
		case *expr.ShellCmd:
			s = append(s, strings.Join(c.Argv, " "))
		}
	}
	sep := "<unknown>"
	switch cmd.Segment {
	case expr.SegmentSemi:
		sep = "; "
	case expr.SegmentPipe:
		sep = " | "
	case expr.SegmentAnd:
		sep = " && "
	case expr.SegmentOut:
		sep = " > "
	case expr.SegmentIn:
		sep = " < "
	}
	return strings.Join(s, sep)
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
		bgAdd(j)
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
	procs, err := j.execShellList(j.Cmd, stdio{j.Stdin, j.Stdout, j.Stderr})

	for _, p := range procs {
		if err := p.waitUntilDone(); err != nil {
			// TODO: set $?
		}
	}

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

func (j *Job) execShellList(cmd interface{}, sio stdio) (procs []*proc, err error) {
	switch cmd := cmd.(type) {
	case *expr.ShellList:
		switch cmd.Segment {
		case expr.SegmentSemi:
			for _, s := range cmd.List {
				for _, p := range procs {
					err = p.waitUntilDone()
				}
				procs, err = j.execShellList(s, sio)
			}
			if len(procs) > 0 {
				err = nil
			}
			return procs, err
		case expr.SegmentAnd:
			for _, s := range cmd.List {
				for _, p := range procs {
					if err = p.waitUntilDone(); err != nil {
						return nil, err
					}
				}
				procs, err = j.execShellList(s, sio)
			}
			return procs, err
		case expr.SegmentPipe:
			return j.execPipeline(cmd, sio)
		default:
			panic(fmt.Sprintf("unknown segment type %s", cmd.Segment))
		}
		// TODO SegmentOut
		// TODO SegmentIn
	case *expr.ShellCmd:
		p, err := j.execCmd(cmd, sio)
		if err != nil {
			return nil, err
		}
		if p != nil {
			return []*proc{p}, nil
		}
		return nil, nil
	default:
		panic(fmt.Sprintf("impossible shell command type: %T", cmd))
	}
}

func (j *Job) execCmd(cmd *expr.ShellCmd, sio stdio) (*proc, error) {
	argv, err := expansion(cmd.Argv, j.Params)
	if err != nil {
		return nil, err
	}
	if a := Alias.Get(argv[0]); a != "" {
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
			dir = Env.Get("HOME")
		} else {
			dir = argv[1]
		}
		wd := ""
		if filepath.IsAbs(dir) {
			wd = filepath.Clean(dir)
		} else {
			wd = filepath.Join(Env.Get("PWD"), dir)
		}
		if err := os.Chdir(wd); err != nil {
			return nil, err
		}
		Env.Set("PWD", wd)
		fmt.Fprintf(os.Stdout, "%s\n", wd)
		return nil, nil
	case "fg":
		return nil, bgFg(strings.Join(argv[1:], " "))
	case "jobs":
		bgList(j.Stderr)
		return nil, nil
	case "exit", "logout":
		return nil, fmt.Errorf("ng does not know %q, try $$", argv[0])
	default:
		p := &proc{
			job:  j,
			argv: argv,
			sio:  sio,
		}
		if err := p.start(); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func (j *Job) execPipeline(cmd *expr.ShellList, sio stdio) (procs []*proc, err error) {
	origSio := sio

	if interactive && j.pgid == 0 {
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
			return nil, err
		}
		j.pgid = pgidLeader.Pid
		defer func() {
			pgidLeader.Kill()
			j.pgid = 0
		}()
	}

	for i, c := range cmd.List {
		var r1, w1 *os.File
		if i == len(cmd.List)-1 {
			sio.out = origSio.out
		} else {
			r1, w1, err = os.Pipe()
			if err != nil {
				// TODO kill already running procs in pipeline job
				return nil, err
			}
			sio.out = w1
		}
		ps, err := j.execShellList(c, sio)
		if err != nil {
			return nil, err // TODO kill already running
		}
		procs = append(procs, ps...)
		if sio.in != origSio.in {
			sio.in.Close()
		}
		if sio.out != origSio.out {
			sio.out.Close()
		}
		sio.in = r1
	}
	return procs, nil
}

func startPgidLeader() (*os.Process, error) {
	path, err := executable()
	if err != nil {
		return nil, fmt.Errorf("pgidleader init: %v", err)
	}
	//path := "/Users/crawshaw/bin/ng"
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

type proc struct {
	job     *Job
	argv    []string
	path    string
	process *os.Process
	sio     stdio
}

// start starts an OS process.
func (p *proc) start() error {
	p.job.mu.Lock()
	defer p.job.mu.Unlock()

	var err error
	p.path, err = findExecInPath(p.argv[0], Env)
	if err != nil {
		return err
	}

	attr := &os.ProcAttr{
		Env:   Env.List(),
		Files: []*os.File{p.sio.in, p.sio.out, p.sio.err},
	}
	if interactive {
		attr.Sys = &syscall.SysProcAttr{
			Setpgid:    true, // job gets new pgid
			Foreground: interactive,
			Pgid:       p.job.pgid,
		}
	}

	p.process, err = os.StartProcess(p.path, p.argv, attr)
	if p.sio.in != p.job.Stdin {
		p.sio.in.Close()
	}
	if p.sio.out != p.job.Stdout {
		p.sio.out.Close()
	}
	if err != nil {
		return err
	}

	if p.job.pgid == 0 {
		p.job.pgid, err = syscall.Getpgid(p.process.Pid)
		if err != nil {
			return fmt.Errorf("cannot get pgid of new process: %v", err)
		}
		if interactive {
			return tcsetpgrp(os.Stdin.Fd(), p.job.pgid)
		}
	}

	return nil
}

// waitUntilDone waits until the process is done.
func (p *proc) waitUntilDone() error {
	pid := p.process.Pid
	for {
		wstatus := new(syscall.WaitStatus)
		_, err := syscall.Wait4(pid, wstatus, syscall.WUNTRACED|syscall.WCONTINUED, nil)
		switch {
		case err != nil || wstatus.Exited():
			//fmt.Fprintf(os.Stderr, "process exited with %v\n", err)
			if c := wstatus.ExitStatus(); c != 0 {
				return fmt.Errorf("failed on exit: %d", c)
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

var (
	bgMu sync.Mutex
	bg   []*Job
)

func bgAdd(j *Job) {
	bgMu.Lock()
	defer bgMu.Unlock()
	bg = append(bg, j)
	fmt.Fprintf(j.Stderr, "\n[%d]+  Stopped  %s\n", len(bg), shellListString(j.Cmd))
}

func bgList(w io.Writer) {
	bgMu.Lock()
	defer bgMu.Unlock()
	for _, j := range bg {
		state := "Stopped"
		if j.running { // TODO: need to hold lock, but need to not deadlock
			state = "Running"
		}
		fmt.Fprintf(j.Stderr, "\n[%d]+  %s  %s\n", len(bg), state, shellListString(j.Cmd))
	}
}

func bgFg(spec string) error {
	jobspec := 1
	var err error
	if spec != "" {
		jobspec, err = strconv.Atoi(spec)
	}
	if err != nil {
		return fmt.Errorf("fg: %v", err)
	}

	bgMu.Lock()
	if len(bg) == 0 {
		bgMu.Unlock()
		return fmt.Errorf("fg: no jobs\n")
	}
	if jobspec > len(bg) {
		bgMu.Unlock()
		return fmt.Errorf("fg: %d: no such job\n", jobspec)
	}
	j := bg[jobspec-1]
	bg = append(bg[:jobspec-1], bg[jobspec:]...)
	fmt.Fprintf(j.Stderr, "%s\n", shellListString(j.Cmd))
	bgMu.Unlock()
	return j.Continue()
}
