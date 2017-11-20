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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

	s := &server{
		ctx:     ctx,
		neugram: ngcore.New(),
		key:     []byte(info.Key),
		iopubs:  make(map[chan [][]byte]struct{}),
	}

	if err := s.shellListener(info.Transport, info.IP, info.ShellPort); err != nil {
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
	default:
		err = fmt.Errorf("unhandled message: %q", req.Header.Type)
	}

	if err != nil {
		log.Printf("jupyter: %v", err)
	}
}

func (s *server) execute(c *conn, req *message) error {
	reqContent, validContent := req.Content.(map[string]interface{})
	if !validContent {
		return fmt.Errorf("malformed request content: %T", req.Content)
	}

	session := s.session(req)

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

	err = session.Exec([]byte(reqContent["code"].(string)))

	w.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, r)
	r.Close()
	os.Stdout = stdout

	if err != nil {
		return errResp(err)
	}
	content := &executeReply{
		Status:         "ok",
		ExecutionCount: session.ExecCount,
	}
	// TODO user_expressions
	s.publishIO("stream", map[string]string{
		"name": "stdout",
		"text": buf.String(),
	}, req)

	if err := s.shellReply(c, "execute_reply", content, req); err != nil {
		return err
	}
	return s.publishIO("execute_result", content, req)
}

func (s *server) kernelInfo(c *conn, req *message) error {
	content := &kernelInfo{
		Version:     protocolVersion,
		Impl:        "ng",
		ImplVersion: "0.1",
		LangInfo: kernelLangInfo{
			Name:          "neugram",
			Version:       "unreleased",
			FileExtension: ".ng",
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

// connectionInfo is the JSON contents of the initial connection file.
type connectionInfo struct {
	SignatureScheme string `json:"signature_scheme"`
	Transport       string `json:"transport"`
	ControlPort     int    `json:"control_port"`
	HBPort          int    `json:"hb_port"`
	IOPubPort       int    `json:"iopub_port"`
	ShellPort       int    `json:"shell_port"`
	StdinPort       int    `json:"stdin_port"`
	Key             string `json:"key"`
	IP              string `json:"ip"`
}

// message is a jupyter wire protocol message.
// Distinguished by "<IDS|MSG>".
// It is transmitted as a multipart zeromq router message.
// Defined in the "General Message Format" section of:
//	http://jupyter-client.readthedocs.io/en/latest/messaging.html
type message struct {
	IDs          [][]byte
	HMAC         string
	Header       header
	ParentHeader header
	Metadata     map[string]interface{}
	Content      interface{}
}

// Defined in the "General Message Format" section of:
//	http://jupyter-client.readthedocs.io/en/latest/messaging.html
type header struct {
	ID       string `json:"msg_id"`
	Username string `json:"username"`
	Session  string `json:"session"`
	Date     string `json:"date"`
	Type     string `json:"msg_type"`
	Version  string `json:"version"`
}

var msgHeader = []byte("<IDS|MSG>")

func (msg *message) decode(parts [][]byte) error {
	*msg = message{}
	for len(parts) > 0 {
		s := parts[0]
		parts = parts[1:]
		if bytes.Equal(s, msgHeader) {
			break
		}
		msg.IDs = append(msg.IDs, s)
	}

	if len(parts) != 5 {
		return fmt.Errorf("msg decode needs 5 parts, has %d parts", len(parts))
	}
	msg.HMAC = string(parts[0])
	if err := json.Unmarshal(parts[1], &msg.Header); err != nil {
		return fmt.Errorf("msg decode header: %v", err)
	}
	if err := json.Unmarshal(parts[2], &msg.ParentHeader); err != nil {
		return fmt.Errorf("msg decode parent header: %v", err)
	}
	if err := json.Unmarshal(parts[3], &msg.Metadata); err != nil {
		return fmt.Errorf("msg decode metadata header: %v", err)
	}
	if err := json.Unmarshal(parts[4], &msg.Content); err != nil {
		return fmt.Errorf("msg decode content header: %v", err)
	}
	return nil
}

func (msg *message) encode(key []byte) (parts [][]byte, err error) {
	parts = make([][]byte, 5)
	parts[1], err = json.Marshal(msg.Header)
	if err != nil {
		return nil, fmt.Errorf("msg encode header: %v", err)
	}
	parts[2], err = json.Marshal(msg.ParentHeader)
	if err != nil {
		return nil, fmt.Errorf("msg encode parent header: %v", err)
	}
	parts[3], err = json.Marshal(msg.Metadata)
	if err != nil {
		return nil, fmt.Errorf("msg encode metadata: %v", err)
	}
	parts[4], err = json.Marshal(msg.Content)
	if err != nil {
		return nil, fmt.Errorf("msg encode content: %v", err)
	}

	if len(key) != 0 {
		mac := hmac.New(sha256.New, key)
		for _, b := range parts[1:] {
			mac.Write(b)
		}
		parts[0] = make([]byte, hex.EncodedLen(mac.Size()))
		hex.Encode(parts[0], mac.Sum(nil))
	}

	return parts, nil
}

type kernelLangInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	MIMEType          string `json:"mimetype"`
	FileExtension     string `json:"file_extension"`
	PygmentsLexer     string `json:"pygments_lexer"`
	CodeMirrorMode    string `json:"codemirror_mode"`
	NBConvertExporter string `json:"nbconvert_exporter"`
}

type helpLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type kernelInfo struct {
	Version     string         `json:"protocol_version"`
	Impl        string         `json:"implementation"`
	ImplVersion string         `json:"implementation_version"`
	LangInfo    kernelLangInfo `json:"language_info"`
	Banner      string         `json:"banner"`
	HelpLinks   []helpLink     `json:"help_links"`
}

type executeReplyError struct {
	Status         string   `json:"status"` // set to "error"
	ExecutionCount int      `json:"status"`
	ErrName        string   `json:"ename"`
	Err            string   `json:"evalue"`
	Traceback      []string `json:"traceback"`
}

type executeReply struct {
	Status          string                   `json:"status"` // set to "ok"
	ExecutionCount  int                      `json:"status"`
	Payload         []map[string]interface{} `json:"payload"` // deprecated
	UserExpressions map[string]interface{}   `json:"user_expressions`
}

func newUUID() string {
	var uuid [16]byte
	if _, err := io.ReadFull(rand.Reader, uuid[:]); err != nil {
		log.Fatal("cannot generate random data for UUID: %v", err)
	}
	uuid[8] = uuid[8]&^0xc0 | 0x80
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
