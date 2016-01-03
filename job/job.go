// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// TODO: rename this package to eval/shell
package job

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var BG []*Job

func FG(spec string) error {
	jobspec := 1
	if len(BG) == 0 {
		return fmt.Errorf("fg: no jobs\n")
	}
	var err error
	fmt.Printf("spec: %q\n", spec)
	if spec != "" {
		jobspec, err = strconv.Atoi(spec)
	}
	if err != nil {
		return fmt.Errorf("fg: %v", err)
	}
	if jobspec > len(BG) {
		return fmt.Errorf("fg: %d: no such job\n", jobspec)
	}
	j := BG[jobspec-1]
	BG = append(BG[:jobspec-1], BG[jobspec:]...)
	fmt.Println(strings.Join(j.Argv, " ")) // TODO depends on termios state?
	if err := j.Continue(); err != nil {
		return fmt.Errorf("fg: %v", err)
	}
	j.Wait()
	return nil
}

type State int

const (
	Unstarted State = iota
	Exited
	Stopped
	Running
)

func init() {
	var err error
	basicState, err = tcgetattr(os.Stdin.Fd())
	if err == nil {
		loginShell = true
	} else {
		loginShell = false
	}
	shellPgid, err = syscall.Getpgid(0)
	if err != nil {
		panic(err)
	}
}

var (
	loginShell bool
	basicState syscall.Termios
	shellState syscall.Termios
	shellPgid  int
)

type Job struct {
	state State // TODO mutex?

	path    string
	process *os.Process
	Pgid    int

	termios syscall.Termios

	Argv   []string
	Env    []string
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
	Err    error
}

func (j *Job) Start() error {
	j.state = Running

	var err error
	j.path, err = findExecInPath(j.Argv[0], j.Env)
	if err != nil {
		return err
	}

	if loginShell {
		shellState, err = tcgetattr(os.Stdin.Fd())
		if err != nil {
			return err
		}
		if err := tcsetattr(os.Stdin.Fd(), &basicState); err != nil {
			return err
		}
	}
	j.process, err = os.StartProcess(j.path, j.Argv, &os.ProcAttr{
		Env:   j.Env,
		Files: []*os.File{j.Stdin, j.Stdout, j.Stderr},
		Sys: &syscall.SysProcAttr{
			Setpgid: true, // job gets new pgid
			Pgid:    j.Pgid,
		},
	})
	if err != nil {
		return err
	}
	if j.Pgid == 0 {
		j.Pgid, err = syscall.Getpgid(j.process.Pid)
		if err != nil {
			return fmt.Errorf("cannot get pgid of new job: %v", err)
		}
	}
	if loginShell {
		return tcsetpgrp(os.Stdin.Fd(), j.Pgid)
	}
	return nil
}

func (j *Job) Continue() (err error) {
	if loginShell {
		shellState, err = tcgetattr(os.Stdin.Fd())
		if err != nil {
			return err
		}
		if err := tcsetpgrp(os.Stdin.Fd(), j.Pgid); err != nil {
			return err
		}
		if err := tcsetattr(os.Stdin.Fd(), &j.termios); err != nil {
			return err
		}
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

func (j *Job) Wait() {
	pid := j.process.Pid
	wstatus := new(syscall.WaitStatus)
	for {
		_, err := syscall.Wait4(pid, wstatus, syscall.WUNTRACED, nil)
		if err != nil {
			fmt.Fprintf(j.Stderr, "wait failed: %v\n", err)
			return
		}
		switch {
		case wstatus.Stopped():
			if loginShell {
				// move the shell process to the foreground
				if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
					fmt.Fprintf(j.Stderr, "on stop: %v", err)
				}
				j.termios, err = tcgetattr(os.Stdin.Fd())
				if err != nil {
					fmt.Fprintf(j.Stderr, "on stop: %v", err)
				}
				if err := tcsetattr(os.Stdin.Fd(), &shellState); err != nil {
					fmt.Fprintf(j.Stderr, "on stop: %v", err)
				}
			}
			j.state = Stopped
			BG = append(BG, j)
			fmt.Println(j.Stat(len(BG)))
			return
		case wstatus.Continued():
			j.state = Running
		case wstatus.Exited():
			if loginShell {
				if err := tcsetpgrp(os.Stdin.Fd(), shellPgid); err != nil {
					j.Err = err
				}
				if err := tcsetattr(os.Stdin.Fd(), &shellState); j.Err == nil && err != nil {
					j.Err = err
				}
			}
			if c := wstatus.ExitStatus(); j.Err == nil && c != 0 {
				j.Err = fmt.Errorf("failed on exit: %d", c)
			}
			j.state = Exited
			if j.Err != nil {
				// TODO distinguish error code, don't print,
				// instead set $?.
				fmt.Fprintf(j.Stderr, "process exited with %v\n", j.Err)
			}
			return
		case wstatus.Signaled():
			// ignore
		default:
			panic(fmt.Sprintf("unexpected wstatus: %#+v", wstatus))
		}
	}
}

func (j *Job) Stat(jobspec int) string {
	return fmt.Sprintf("[%d]+  %s  %s", jobspec, j.state, strings.Join(j.Argv, " "))
}

func (s State) String() string {
	switch s {
	case Unstarted:
		return "Unstarted"
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
