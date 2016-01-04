// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package shell

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"neugram.io/lang/expr"
)

var Env []string

type Job struct {
	Cmd    *expr.ShellList
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File

	mu      sync.Mutex
	procs   []*proc
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

func (j *Job) execShellList(cmd interface{}, sio stdio) error {
	switch cmd := cmd.(type) {
	case *expr.ShellList:
		switch cmd.Segment {
		case expr.SegmentSemi:
			var err error
			for _, s := range cmd.List {
				if err1 := j.execShellList(s, sio); err == nil {
					err = err1
				}
			}
			return err
		case expr.SegmentAnd:
			for _, s := range cmd.List {
				if err := j.execShellList(s, sio); err != nil {
					return err
				}
			}
			return nil
		case expr.SegmentPipe:
			return j.execPipeline(cmd, sio)
		default:
			panic(fmt.Sprintf("unknown segment type %s", cmd.Segment))
		}
		// TODO SegmentOut
		// TODO SegmentIn
	case *expr.ShellCmd:
		return j.execCmd(cmd, sio)
	default:
		panic(fmt.Sprintf("impossible shell command type: %T", cmd))
	}
}

func (j *Job) execCmd(cmd *expr.ShellCmd, sio stdio) error {
	switch cmd.Argv[0] {
	case "cd":
		dir := ""
		if len(cmd.Argv) == 1 {
			dir = os.Getenv("HOME")
		} else {
			dir = cmd.Argv[1]
		}
		if err := os.Chdir(dir); err != nil {
			return err
		}
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "%s\n", wd)
		return nil
	case "fg":
		return bgFg(strings.Join(cmd.Argv[1:], " "))
	case "jobs":
		bgList(j.Stderr)
		return nil
	case "exit", "logout":
		return fmt.Errorf("ng does not know %q, try $$", cmd.Argv[0])
	default:
		p := &proc{
			job:  j,
			argv: cmd.Argv,
			sio:  sio,
		}
		if err := p.start(); err != nil {
			return err
		}
		return p.waitUntilDone()
	}
}

func (j *Job) execPipeline(cmd *expr.ShellList, sio stdio) error {
	origSio := sio

	errs := make(chan error, len(cmd.List))

	for i := 0; i < len(cmd.List)-1; i++ {
		r1, w1, err := os.Pipe() // closing is handled in proc.start()
		if err != nil {
			// TODO kill already running procs in pipeline job
			return err
		}
		sio.out = w1
		go func(i int, sio stdio) {
			errs <- j.execShellList(cmd.List[i], sio)
		}(i, sio)
		sio.in = r1
	}
	sio.out = origSio.out
	errs <- j.execShellList(cmd.List[len(cmd.List)-1], sio)

	var err error
	for i := 0; i < len(cmd.List); i++ {
		if err1 := <-errs; err1 != nil {
			err = err1
		}
	}
	return err
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

	signal.Reset(jobSignals...)
	p.process, err = os.StartProcess(p.path, p.argv, &os.ProcAttr{
		Env:   Env,
		Files: []*os.File{p.sio.in, p.sio.out, p.sio.err},
		Sys: &syscall.SysProcAttr{
			Setpgid:    true, // job gets new pgid
			Foreground: interactive,
			Pgid:       p.job.pgid,
		},
	})
	signal.Ignore(jobSignals...)
	if p.sio.in != p.job.Stdin {
		p.sio.in.Close()
	}
	if p.sio.out != p.job.Stdout {
		p.sio.out.Close()
	}
	if err != nil {
		return err
	}

	p.job.procs = append(p.job.procs, p)

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
			//fmt.Fprintf(p.Stderr, "process exited with %v\n", p.Err)
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
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGTSTP,
	syscall.SIGTTOU,
	syscall.SIGTTIN,
	syscall.SIGCHLD,
}

func Init() {
	var err error
	basicState, err = tcgetattr(os.Stdin.Fd())
	if err == nil {
		interactive = true
	}

	if interactive {
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
