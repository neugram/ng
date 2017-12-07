// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jupyter

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

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

// kernelInfo is the message being sent when a client needs to know information
// about the kernel.
//
// see: http://jupyter-client.readthedocs.io/en/latest/messaging.html#kernel-info.
type kernelInfo struct {
	Version     string         `json:"protocol_version"`
	Impl        string         `json:"implementation"`
	ImplVersion string         `json:"implementation_version"`
	LangInfo    kernelLangInfo `json:"language_info"`
	Banner      string         `json:"banner"`
	HelpLinks   []helpLink     `json:"help_links"`
}

// kernelLangInfo is part of the kernelInfo message.
// It provides informations about the language of code for the kernel.
type kernelLangInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	MIMEType          string `json:"mimetype"`
	FileExtension     string `json:"file_extension"`
	PygmentsLexer     string `json:"pygments_lexer"`
	CodeMirrorMode    string `json:"codemirror_mode"`
	NBConvertExporter string `json:"nbconvert_exporter"`
}

// helpLink is part of the kernelInfo message.
// It will be used by the notebook UI's help menu.
type helpLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// shutdownRequest is sent when the kernel is requested to shut itself down.
// see: http://jupyter-client.readthedocs.io/en/latest/messaging.html#kernel-shutdown.
type shutdownRequest struct {
	// If Restart is true, shutdown precedes a restart.
	// Otherwise, it indicates a final shutdown.
	Restart bool `json:"restart"`
}

// shutdownReply is sent back by the kernel to notify the shutdown process
// has completed.
type shutdownReply struct {
	// If Restart is true, shutdown precedes a restart.
	// Otherwise, it indicates a final shutdown.
	Restart bool `json:"restart"`
}

// executeRequest is the message type used by frontends to ask the kernel
// to execute code on behalf of the user.
//
// see: http://jupyter-client.readthedocs.io/en/latest/messaging.html#execute.
type executeRequest struct {
	Code         string `json:"code"`          // source code to be executed by the kernel, one or more lines
	Silent       bool   `json:"silent"`        // if true, signals the kernel to execute this code as quietly as possible
	StoreHistory bool   `json:"store_history"` // if true, signals the kernel to populate history
	AllowStdin   bool   `json:"allow_stdin"`   // if true, code running in the kernel can prompt the user for input
	StopOnError  bool   `json:"stop_on_error"` // if true, does not abort the execution queue
}

// executeInput is the message type sent to the frontend to tell it the kernel
// is about to execute a snippet of code.
type executeInput struct {
	ExecutionCount int    `json:"execution_count,omitempty"`
	Code           string `json:"code"` // source code to be executed by the kernel, one or more lines
}

type executeReplyError struct {
	Status         string   `json:"status"` // set to "error"
	ExecutionCount int      `json:"execution_count,omitempty"`
	ErrName        string   `json:"ename"`
	Err            string   `json:"evalue"`
	Traceback      []string `json:"traceback"`
}

type executeReply struct {
	Status          string                   `json:"status"` // set to "ok"
	ExecutionCount  int                      `json:"execution_count,omitempty"`
	Payload         []map[string]interface{} `json:"payload"` // deprecated
	UserExpressions map[string]interface{}   `json:"user_expressions`
}

type executeResult struct {
	ExecutionCount int                    `json:"execution_count,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`     // map of MIME -> raw data in that MIME type
	Meta           map[string]interface{} `json:"metadata,omitempty"` // any metadata that describes the data
}

type kernelStatus struct {
	ExecState string `json:"execution_state"`
}

var (
	kernelBusy = kernelStatus{ExecState: "busy"}
	kernelIdle = kernelStatus{ExecState: "idle"}
)

// displayData is the message used to bring back data that should be displayed
// (text, html, svg, etc...) in the frontends.
// Each message can have multiple representations of the data.
//
// see: http://jupyter-client.readthedocs.io/en/latest/messaging.html#display-data.
type displayData struct {
	Data map[string]interface{} `json:"data,omitempty"`     // map of MIME -> raw data in that MIME type
	Meta map[string]interface{} `json:"metadata,omitempty"` // any metadata that describes the data

	// Transient informations not to be persisted to a notebook or other documents.
	// Transient is intended to live only during a live kernel session.
	Transient map[string]interface{} `json:"transient,omitempty"`
}
