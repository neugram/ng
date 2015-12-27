// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package job

import (
	"bytes"
	"io"
	"sync"
)

type PollReader struct {
	Ready chan struct{}

	src io.Reader

	mu  sync.Mutex
	err error
	buf bytes.Buffer
}

func (r *PollReader) Read(buf []byte) (n int, err error) {
	r.mu.Lock()
	if r.buf.Len() == 0 {
		r.mu.Unlock()
		<-r.Ready // block until ready
		r.mu.Lock()
	}
	n, err = r.buf.Read(buf)
	r.mu.Unlock()

	if err == io.EOF {
		err = nil
	}
	if err == nil {
		err = r.err
	}
	return n, err
}

func (r *PollReader) readLoop() {
	b := make([]byte, 4096)
	for {
		n, err := r.src.Read(b)
		if n > 0 {
			r.mu.Lock()
			r.buf.Write(b[:n])
			r.mu.Unlock()

			select {
			case r.Ready <- struct{}{}:
			}
		}
		if err != nil {
			r.mu.Lock()
			r.err = err
			r.mu.Unlock()

			return
		}
	}
}

func NewPollReader(src io.Reader) *PollReader {
	r := &PollReader{
		Ready: make(chan struct{}, 1),
		src:   src,
	}
	go r.readLoop()
	return r
}
