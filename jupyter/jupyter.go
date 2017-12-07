// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jupyter implements a jupyter kernel for Neugram.
//
// It is built directly on top of the zmtp package because the gomq
// project is missing zeromq features required by jupyter. However
// the total feature set used by jupyter is small enough that we can
// do it manually here, and that way avoid any C/C++ dependencies.
//
// Specifications of interest:
//
//	https://rfc.zeromq.org/spec:23/ZMTP
//	http://jupyter-client.readthedocs.io/en/latest/messaging.html
//
// Unfortunately the ZMTP spec (and the relevant linked specifications)
// are not clear on how they interact with TCP bind/listen/accept, so
// that was determined empirically.
//
// Similarly, many details are missing from the jupyter wire format docs.
// When a message is broadcast on the IOPUB socket, does it include the
// parent_header of the session that caused it? When reporting an error,
// what (if any!) of the documented fields are used by jupyter?
// These questions were answered by poking at a running jupyter instance.
// Some of the answers are probably wrong.
//
// Usage
//
// On linux, install the "ng" binary on your PATH, and add a file
// named $HOME/.local/share/jupyter/kernels/neugram/kernel.json containing:
//
//	{
//		"argv": [ "ng", "-jupyter", "{connection_file}" ],
//		"display_name": "Neugram",
//		"language": "neugram"
//	}
//
// Then start a Jupyter notebook server by running "jupyter notebook".
package jupyter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sideshowdave7/gomq/zmtp"
	"neugram.io/ng/ngcore"
)

// Run runs a jupyter kernel, serving on the sockets described in connFile.
func Run(ctx context.Context, connFile string) error {
	b, err := ioutil.ReadFile(connFile)
	if err != nil {
		return fmt.Errorf("jupyter: failed to read connection file: %v", err)
	}

	var info connectionInfo
	if err = json.Unmarshal(b, &info); err != nil {
		return fmt.Errorf("jupyter: failed to parse connection file: %s: %v", connFile, err)
	}

	if debug {
		log.Printf("jupyter conn info: %#+v\n", info)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &server{
		ctx:     ctx,
		cancel:  cancel,
		neugram: ngcore.New(),
		key:     []byte(info.Key),
		iopubs:  make(map[chan [][]byte]struct{}),
	}

	if err := s.shellListener(info.Transport, info.IP, info.ShellPort); err != nil {
		return err
	}
	if err := s.ctlListener(info.Transport, info.IP, info.ControlPort); err != nil {
		return err
	}
	if err := s.iopubListener(info.Transport, info.IP, info.IOPubPort); err != nil {
		return err
	}

	// TODO: heartbeat?
	// TODO: shutdown the sockets on ctx.Done

	<-ctx.Done()
	return nil
}

const protocolVersion = "5.2" // value reported by jupyter as of late 2017

const debug = false

type conn struct {
	conn net.Conn
	zmtp *zmtp.Connection
}

type server struct {
	ctx     context.Context // server lifecycle
	cancel  context.CancelFunc
	neugram *ngcore.Neugram
	key     []byte

	mu     sync.Mutex
	iopubs map[chan [][]byte]struct{}
}

func (s *server) shellListener(transport, ip string, port int) error {
	ln, err := net.Listen(transport, fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return fmt.Errorf("jupyter: failed to listen on shell socket: %v", err)
	}
	go func() {
		for {
			netConn, err := ln.Accept()
			if err != nil {
				log.Printf("jupyter: shell listener exiting: %v", err)
				return
			}
			zmtpConn := zmtp.NewConnection(netConn)
			_, err = zmtpConn.Prepare(new(zmtp.SecurityNull), zmtp.RouterSocketType, "", true, nil)
			if err != nil {
				log.Printf("jupyter: failed prepare shell socket ROUTER: %v", err)
				netConn.Close()
				continue
			}
			c := &conn{netConn, zmtpConn}
			go s.shellHandler(c)
		}
	}()
	return nil
}

func (s *server) ctlListener(transport, ip string, port int) error {
	ln, err := net.Listen(transport, fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return fmt.Errorf("jupyter: failed to listen on control socket: %v", err)
	}
	go func() {
		for {
			netConn, err := ln.Accept()
			if err != nil {
				log.Printf("jupyter: control listener exiting: %v", err)
				return
			}
			zmtpConn := zmtp.NewConnection(netConn)
			_, err = zmtpConn.Prepare(new(zmtp.SecurityNull), zmtp.RouterSocketType, "", true, nil)
			if err != nil {
				log.Printf("jupyter: failed prepare control socket ROUTER: %v", err)
				netConn.Close()
				continue
			}
			c := &conn{netConn, zmtpConn}
			go s.ctlHandler(c)
		}
	}()
	return nil
}

func (s *server) iopubListener(transport, ip string, port int) error {
	ln, err := net.Listen(transport, fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return fmt.Errorf("jupyter: failed to listen on iopub socket: %v", err)
	}
	go func() {
		for {
			netConn, err := ln.Accept()
			if err != nil {
				log.Printf("jupyter: iopub listener exiting: %v", err)
				return
			}
			zmtpConn := zmtp.NewConnection(netConn)
			_, err = zmtpConn.Prepare(new(zmtp.SecurityNull), zmtp.PubSocketType, "", true, nil)
			if err != nil {
				log.Printf("jupyter: failed prepare iopub socket PUB: %v", err)
				netConn.Close()
				continue
			}
			c := &conn{netConn, zmtpConn}
			go s.iopubHandler(c)
		}
	}()
	return nil
}

func (s *server) shellHandler(c *conn) {
	if debug {
		log.Printf("shell handler started\n")
	}
	ch := make(chan *zmtp.Message)
	c.zmtp.Recv(ch)
	for {
		zmsg := <-ch
		switch zmsg.MessageType {
		case zmtp.UserMessage:
			var msg message
			if err := msg.decode(zmsg.Body); err != nil {
				log.Printf("jupyter: shell msg: %v", err)
				continue
			}
			s.shellRequest(c, &msg)
		case zmtp.CommandMessage:
			log.Printf("TODO shell cmd msg body=\n") // TODO
			for _, b := range zmsg.Body {
				log.Printf("\t%s\n", b)
			}
		case zmtp.ErrorMessage:
			if zmsg.Err == io.EOF {
				c.conn.Close()
				return
			}
			log.Printf("shell err msg: %v\n", zmsg.Err)
		}
	}
}

func (s *server) ctlHandler(c *conn) {
	if debug {
		log.Printf("control handler started\n")
	}
	ch := make(chan *zmtp.Message)
	c.zmtp.Recv(ch)
	for {
		zmsg := <-ch
		switch zmsg.MessageType {
		case zmtp.UserMessage:
			var msg message
			if err := msg.decode(zmsg.Body); err != nil {
				log.Printf("jupyter: control msg: %v", err)
				continue
			}
			s.shellRequest(c, &msg)
		case zmtp.CommandMessage:
			log.Printf("TODO control cmd msg body=\n") // TODO
			for _, b := range zmsg.Body {
				log.Printf("\t%s\n", b)
			}
		case zmtp.ErrorMessage:
			if zmsg.Err == io.EOF {
				c.conn.Close()
				return
			}
			log.Printf("control err msg: %v\n", zmsg.Err)
		}
	}
}

func (s *server) publishIO(typeName string, content interface{}, req *message) error {
	b, err := replyMessage(typeName, s.key, content, req)
	if err != nil {
		return err
	}

	s.mu.Lock()
	for ch := range s.iopubs {
		select {
		case ch <- b:
		default:
		}
	}
	s.mu.Unlock()

	return nil
}

func (s *server) iopubHandler(c *conn) {
	if debug {
		log.Printf("iopub handler started\n")
	}
	ch := make(chan [][]byte, 4)

	s.mu.Lock()
	s.iopubs[ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.iopubs, ch)
		s.mu.Unlock()
	}()

	for {
		if err := c.zmtp.SendMultipart(<-ch); err != nil {
			log.Printf("iopub send failed: %v", err)
			return
		}
	}
}

func (s *server) session(req *message) *ngcore.Session {
	return s.neugram.GetOrNewSession(s.ctx, req.Header.Session)
}

func (s *server) shellRequest(c *conn, req *message) {
	if debug {
		log.Printf("shell user req: %#+v\n", req)
	}

	var err error // failure to respond to frontend

	switch req.Header.Type {
	case "kernel_info_request":
		err = s.kernelInfo(c, req)
	case "execute_request":
		err = s.execute(c, req)
	case "shutdown_request":
		err = s.shutdown(c, req)
	default:
		err = fmt.Errorf("unhandled message: %q", req.Header.Type)
	}

	if err != nil {
		log.Printf("jupyter: %v", err)
	}
}

func (s *server) shutdown(c *conn, req *message) error {
	defer s.cancel()

	reqContent, validContent := req.Content.(map[string]interface{})
	if !validContent {
		return fmt.Errorf("malformed request content: %T", req.Content)
	}

	restart := reqContent["restart"].(bool)
	if err := s.shellReply(c, "shutdown_reply", &shutdownReply{Restart: restart}, req); err != nil {
		return err
	}

	return nil
}

func (s *server) execute(c *conn, req *message) error {
	reqContent, validContent := req.Content.(map[string]interface{})
	if !validContent {
		return fmt.Errorf("malformed request content: %T", req.Content)
	}

	session := s.session(req)

	if err := s.publishIO("status", kernelBusy, req); err != nil {
		log.Printf("jupyter: failed to communicate busy status: %v", err)
	}
	defer func() {
		if err := s.publishIO("status", kernelIdle, req); err != nil {
			log.Printf("jupyter: failed to communicate idle status: %v", err)
		}
	}()

	code := reqContent["code"].(string)
	if err := s.publishIO("execute_input", &executeInput{ExecutionCount: session.ExecCount, Code: code}, req); err != nil {
		return err
	}

	errResp := func(err error) error {
		content := &executeReplyError{
			Status:         "error",
			ExecutionCount: session.ExecCount,
		}
		if ngerr, isNgerr := err.(ngcore.Error); isNgerr {
			content.ErrName = ngerr.Phase
			content.Err = ngerr.List[0].Error()
		} else {
			content.Err = err.Error()
		}
		if content.ErrName == "" {
			content.ErrName = "error"
		}
		// Put the error in the traceback, because it appears that's
		// all that jupyter actually prints. Huh.
		content.Traceback = []string{err.Error()}
		if err := s.shellReply(c, "execute_reply", content, req); err != nil {
			return err
		}
		return s.publishIO("error", content, req)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return errResp(err)
	}

	session.Stdout = w
	session.Stderr = w
	stdout := os.Stdout
	os.Stdout = w
	stderr := os.Stderr
	os.Stderr = w

	done := make(chan struct{})
	buf := new(bytes.Buffer)
	go func() {
		io.Copy(buf, r)
		close(done)
	}()

	vals, err := session.Exec([]byte(code))

	w.Close()
	<-done
	r.Close()
	os.Stdout = stdout
	os.Stderr = stderr

	if err != nil {
		return errResp(err)
	}
	content := &executeReply{
		Status:         "ok",
		ExecutionCount: session.ExecCount,
	}
	// TODO user_expressions
	if txt := buf.String(); len(txt) != 0 {
		// TODO: distinguish b/w stdout/stderr
		// TODO: publish stderr stream
		s.publishIO("stream", map[string]string{
			"name": "stdout",
			"text": txt,
		}, req)
	}

	if err := s.shellReply(c, "execute_reply", content, req); err != nil {
		return err
	}

	nvals := 0
	for i := range vals {
		if len(vals[i]) > 0 {
			nvals++
		}
	}
	if nvals == 0 {
		return nil
	}

	var dd displayData
	for _, tuple := range vals {
		dd, err = genDisplayData(session, tuple)
		if err != nil {
			return err
		}
	}

	result := &executeResult{
		ExecutionCount: session.ExecCount,
		Data:           dd.Data,
		Meta:           dd.Meta,
	}
	return s.publishIO("execute_result", result, req)
}

func (s *server) kernelInfo(c *conn, req *message) error {
	content := &kernelInfo{
		Version:     protocolVersion,
		Impl:        "ng",
		ImplVersion: "0.1",
		LangInfo: kernelLangInfo{
			Name:           "neugram",
			Version:        "unreleased",
			FileExtension:  ".ng",
			PygmentsLexer:  "go",
			CodeMirrorMode: "go",
		},
		Banner: "Neugram",
		HelpLinks: []helpLink{{
			Text: "Neugram",
			URL:  "https://neugram.io",
		}},
	}
	return s.shellReply(c, "kernel_info_reply", content, req)
}

func (s *server) shellReply(c *conn, typeName string, content interface{}, req *message) error {
	b, err := replyMessage(typeName, s.key, content, req)
	if err != nil {
		return err
	}
	if err := c.zmtp.SendMultipart(b); err != nil {
		return fmt.Errorf("%s: %v", typeName, err)
	}
	return nil
}

func replyMessage(typeName string, key []byte, content interface{}, req *message) ([][]byte, error) {
	var rep message
	rep.Header.ID = newUUID()
	rep.Header.Username = req.Header.Username
	rep.Header.Session = req.Header.Session
	rep.Header.Date = time.Now().UTC().Format(time.RFC3339)
	rep.Header.Type = typeName
	rep.Header.Version = protocolVersion
	rep.ParentHeader = req.Header
	rep.Content = content

	parts, err := rep.encode(key)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", typeName, err)
	}

	var b [][]byte
	b = append(b, req.IDs...)
	b = append(b, msgHeader)
	b = append(b, parts...)
	return b, nil
}

func newUUID() string {
	var uuid [16]byte
	if _, err := io.ReadFull(rand.Reader, uuid[:]); err != nil {
		log.Fatalf("cannot generate random data for UUID: %v", err)
	}
	uuid[8] = uuid[8]&^0xc0 | 0x80
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
