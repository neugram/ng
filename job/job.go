// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package job

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

type State int

const (
	Exited State = iota
	Stopped
	Running
)

type Job struct {
	state State
	State chan State

	path      string
	stdinStop chan struct{}
	stdinCont chan struct{}
	process   *os.Process
	pgid      int

	termios syscall.Termios

	Argv []string
	Env  []string
	Err  error
}

func init() {
	err := tcgetattr(os.Stdin.Fd(), &initialState)
	if err != nil {
		login = false
	}
	pgid, err = syscall.Getpgid(0)
	if err != nil {
		panic(err)
	}
}

var (
	login        = true
	initialState syscall.Termios
	pgid         int
)

func Start(argv, env []string) (*Job, error) {
	j := &Job{
		state: Running, // TODO mutex? TODO remove?
		State: make(chan State),

		stdinStop: make(chan struct{}),
		stdinCont: make(chan struct{}),

		Argv: argv,
		Env:  env,
	}

	var err error
	j.path, err = findExecInPath(argv[0], env)
	if err != nil {
		return nil, err
	}

	tcsetattr(os.Stdin.Fd(), &initialState) // TODO: save current shell state
	j.process, err = os.StartProcess(j.path, j.Argv, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys: &syscall.SysProcAttr{
			Setpgid: true, // job gets new pgid
		},
	})
	if err != nil {
		return nil, err
	}
	j.pgid, err = syscall.Getpgid(j.process.Pid)
	if err != nil {
		return nil, fmt.Errorf("cannot get pgid of new job: %v", err)
	}
	tcsetpgrp(os.Stdin.Fd(), j.pgid)

	go func() {
		pid := j.process.Pid
		// TODO: is it safe to do this before receiving SIGCHLD?
		wstatus := new(syscall.WaitStatus)
		for {
			fmt.Printf("calling wait4\n")
			_, err := syscall.Wait4(pid, wstatus, syscall.WUNTRACED, nil)
			if err != nil {
				j.Err = os.NewSyscallError("wait4", err)
			}
			fmt.Printf("wait4: %+v\n", wstatus)
			switch {
			case wstatus.Stopped():
				// move the shell process to the foreground
				tcgetattr(os.Stdin.Fd(), &j.termios)
				tcsetpgrp(os.Stdin.Fd(), pgid)
				j.state = Stopped
				j.State <- Stopped
			case wstatus.Continued():
				j.state = Running
				j.State <- Running
			case wstatus.Exited():
				tcsetpgrp(os.Stdin.Fd(), pgid)
				if exit := wstatus.ExitStatus(); j.Err == nil && exit != 0 {
					j.Err = fmt.Errorf("failed on exit: %d", exit)
				}
				j.state = Exited
				j.State <- Exited
				return
			}
		}
	}()
	return j, nil
}

func (j *Job) Continue() error {
	tcsetattr(os.Stdin.Fd(), &j.termios)
	tcsetpgrp(os.Stdin.Fd(), j.pgid)
	pid := j.process.Pid
	if err := syscall.Kill(pid, syscall.SIGCONT); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("cannot signal process %d to continue: %v", pid, err)
	}
	return nil
}

func (j *Job) Resize() {
	pid := j.process.Pid
	syscall.Kill(pid, syscall.SIGWINCH)
}

func (j *Job) Stat(jobspec int) string {
	return fmt.Sprintf("[%d]+  %s  %s", jobspec, j.state, strings.Join(j.Argv, " "))
}

func (s State) String() string {
	switch s {
	case Exited:
		return "Exited"
	case Stopped:
		return "Stopped"
	case Running:
		return "Running"
	default:
		return fmt.Sprintf("State(%d)", s)
	}
}
