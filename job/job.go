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

	path    string
	process *os.Process
	pgid    int

	termios syscall.Termios

	Argv []string
	Env  []string
	Err  error
}

func init() {
	var err error
	basicState, err = tcgetattr(os.Stdin.Fd())
	if err != nil {
		// not a login shell, do something very different.
		fmt.Printf("note, could not tcgetattr: %v", err)
	}
	shellPgid, err = syscall.Getpgid(0)
	if err != nil {
		panic(err)
	}
}

var (
	basicState syscall.Termios
	shellState syscall.Termios
	shellPgid  int
)

func Start(argv, env []string) (*Job, error) {
	j := &Job{
		state: Running, // TODO mutex? TODO remove?
		State: make(chan State),

		Argv: argv,
		Env:  env,
	}

	var err error
	j.path, err = findExecInPath(argv[0], env)
	if err != nil {
		return nil, err
	}

	shellState, err = tcgetattr(os.Stdin.Fd())
	if err != nil {
		return nil, err
	}
	if err := tcsetattr(os.Stdin.Fd(), &basicState); err != nil {
		return nil, err
	}
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
	if err := tcsetpgrp(os.Stdin.Fd(), j.pgid); err != nil {
		return nil, err
	}

	go func() {
		pid := j.process.Pid
		wstatus := new(syscall.WaitStatus)
		for {
			_, err := syscall.Wait4(pid, wstatus, syscall.WUNTRACED, nil)
			if err != nil {
				j.Err = os.NewSyscallError("wait4 WUNTRACED", err)
			}
			switch {
			case wstatus.Stopped():
				// move the shell process to the foreground
				if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
					fmt.Fprintf(os.Stderr, "on stop: %v", err)
				}
				j.termios, err = tcgetattr(os.Stdin.Fd())
				if err != nil {
					fmt.Fprintf(os.Stderr, "on stop: %v", err)
				}
				if err := tcsetattr(os.Stdin.Fd(), &shellState); err != nil {
					fmt.Fprintf(os.Stderr, "on stop: %v", err)
				}
				j.state = Stopped
				j.State <- Stopped
			case wstatus.Continued():
				j.state = Running
				j.State <- Running
			case wstatus.Exited():
				if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
					j.Err = err
				}
				if err := tcsetattr(os.Stdin.Fd(), &shellState); j.Err == nil && err != nil {
					j.Err = err
				}
				if c := wstatus.ExitStatus(); j.Err == nil && c != 0 {
					j.Err = fmt.Errorf("failed on exit: %d", c)
				}
				j.state = Exited
				j.State <- Exited
				return
			case wstatus.Signaled():
				// ignore
			default:
				panic(fmt.Sprintf("unexpected wstatus: %#+v", wstatus))
			}
		}
	}()
	return j, nil
}

func (j *Job) Continue() (err error) {
	shellState, err = tcgetattr(os.Stdin.Fd())
	if err != nil {
		return err
	}
	if err := tcsetpgrp(os.Stdin.Fd(), j.pgid); err != nil {
		return err
	}
	if err := tcsetattr(os.Stdin.Fd(), &j.termios); err != nil {
		return err
	}
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
