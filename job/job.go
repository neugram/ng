// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package job

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func New(argv []string) (*Job, error) {
	done := make(chan error, 1)

	j := &Job{
		cond: sync.NewCond(new(sync.Mutex)),
		in:   pausableReader{r: os.Stdin},
		out:  pausableWriter{w: os.Stdout},
		err:  pausableWriter{w: os.Stderr},

		Cmd:  exec.Command(argv[0], argv[1:]...),
		Done: done,
		Argv: argv,
	}

	j.in.j = j
	j.out.j = j
	j.err.j = j

	j.Cmd.Stdin = j.in
	j.Cmd.Stdout = j.out
	j.Cmd.Stderr = j.err

	if err := j.Cmd.Start(); err != nil {
		return nil, err
	}
	j.running = true
	go func() {
		done <- j.Cmd.Wait()
	}()
	return j, nil
}

type Job struct {
	cond    *sync.Cond
	running bool // guarded by cond.L
	in      pausableReader
	out     pausableWriter
	err     pausableWriter

	Cmd  *exec.Cmd
	Done <-chan error
	Argv []string
}

func (j *Job) StartIO() {
	j.cond.L.Lock()
	j.running = true
	j.cond.Broadcast()
	j.cond.L.Unlock()
}

func (j *Job) StopIO() {
	j.cond.L.Lock()
	j.running = false
	j.cond.Broadcast()
	j.cond.L.Unlock()
}

func (j *Job) Stat(jobspec int) string {
	run := "Stopped"
	j.cond.L.Lock()
	if j.running {
		run = "Running"
	}
	j.cond.L.Unlock()
	return fmt.Sprintf("[%d]+  %s  %s", jobspec, run, strings.Join(j.Argv, " "))
}

type pausableReader struct {
	j *Job
	r io.Reader
}

func (p pausableReader) Read(data []byte) (n int, err error) {
	p.j.cond.L.Lock()
	for !p.j.running {
		p.j.cond.Wait()
	}
	p.j.cond.L.Unlock()

	return p.r.Read(data)
}

type pausableWriter struct {
	j *Job
	w io.Writer
}

func (p pausableWriter) Write(data []byte) (n int, err error) {
	p.j.cond.L.Lock()
	for !p.j.running {
		p.j.cond.Wait()
	}
	p.j.cond.L.Unlock()

	return p.w.Write(data)
}
