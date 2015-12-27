// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package job

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/kr/pty"
)

type Job struct {
	running   bool
	path      string
	stdinErr  chan error
	stdinStop chan struct{}
	stdinCont chan struct{}
	process   *os.Process

	tty     *os.File
	pty     *os.File
	termios syscall.Termios

	Argv []string
	Env  []string
	Err  error
	Done chan struct{}
}

var Stdin *PollReader

func Start(argv, env []string) (*Job, error) {
	j := &Job{
		running:   true,
		stdinErr:  make(chan error), // TODO use or lose
		stdinStop: make(chan struct{}),
		stdinCont: make(chan struct{}),

		Done: make(chan struct{}),
		Argv: argv,
		Env:  env,
	}

	var err error
	j.path, err = findExecInPath(argv[0], env)
	if err != nil {
		return nil, err
	}

	// TODO this is overkill, and is probably what's causing us so much trouble
	// refreshing vim coming back from the background. All we should need is
	// one PTY and to let the kernel make them foreground or background.
	j.pty, j.tty, err = pty.Open()
	if err != nil {
		return nil, err
	}

	go j.stdinPump()
	go j.stdoutPump()

	j.process, err = os.StartProcess(j.path, j.Argv, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{j.tty, j.tty, j.tty},
		Sys: &syscall.SysProcAttr{
			Setctty: true,
			Setsid:  true,
		},
	})

	go func() {
		state, err := j.process.Wait()
		j.tty.Close()
		j.pty.Close() // TODO: what valid errors need to be handled here?
		if err != nil {
			j.Err = err
		} else if !state.Success() {
			code := exitCode(state)
			j.Err = fmt.Errorf("failed on exit: %d", code)
		}
		close(j.Done)
	}()
	return j, nil
}

func (j *Job) stdinPump() {
	buf := make([]byte, 4096)
	var ready chan struct{}
	if Stdin != nil {
		ready = Stdin.Ready
	} else {
		ready = make(chan struct{}) // no stdin, never ready
	}
	stopped := false
	for {
		if stopped {
			select {
			case <-j.stdinCont:
				stopped = false
				j.pty.Write([]byte("\r\n"))
			case <-j.Done:
				return
			}
		}
		select {
		case <-j.stdinStop:
			stopped = true
		case <-j.Done:
			return
		case <-ready:
			n, err := Stdin.Read(buf)
			_, err2 := j.pty.Write(buf[:n])
			if err == nil {
				err = err2
			}
			if pe, ok := err.(*os.PathError); ok {
				if pe.Err == syscall.EBADF {
					return
				}
			}
			if err != nil {
				//j.stdinErr <- err
				fmt.Printf("stdin error: %v\n", err)
				return
			}
		}
	}
}

func (j *Job) stdoutPump() {
	// TODO: can we find a way to send SIGTTOU if !j.running?
	_, err := io.Copy(os.Stdout, j.pty)
	if err != nil {
		fmt.Printf("stdout pipe error: %v\n", err)
	}
	//j.stdinErr <- err
}

func (j *Job) Continue() error {
	pid := j.process.Pid
	j.running = true
	if err := syscall.Kill(pid, syscall.SIGCONT); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("cannot signal process %d to continue: %v", pid, err)
	}
	j.stdinCont <- struct{}{}
	tcsetattr(os.Stdin.Fd(), &j.termios)
	//rows, cols := winsize(os.Stdin.Fd())
	return nil
}

func (j *Job) Stop() error {
	pid := j.process.Pid
	tcgetattr(os.Stdin.Fd(), &j.termios)
	if err := syscall.Kill(pid, syscall.SIGTSTP); err != nil {
		return fmt.Errorf("cannot signal process %d to stop: %v", pid, err)
	}
	j.running = false
	j.stdinStop <- struct{}{}
	return nil
}

func (j *Job) Interrupt() error {
	pid := j.process.Pid
	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("cannot signal process %d to interrupt: %v", pid, err)
	}
	return nil
}

func (j *Job) Resize() {
	pid := j.process.Pid
	syscall.Kill(pid, syscall.SIGWINCH)
}

func (j *Job) Stat(jobspec int) string {
	run := "Stopped"
	if j.running {
		run = "Running"
	}
	return fmt.Sprintf("[%d]+  %s  %s", jobspec, run, strings.Join(j.Argv, " "))
}
