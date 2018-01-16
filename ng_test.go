// Copyright 2018 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

var exeSuffix string // set to ".exe" on GOOS=windows
var testng string    // name of testng binary

func init() {
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	testng = "./testng" + exeSuffix
}

// The TestMain function creates an ng  command for testing purposes.
func TestMain(m *testing.M) {
	out, err := exec.Command("go", "build", "-o", testng).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "building testng failed: %v\n%s", err, out)
		os.Exit(2)
	}

	r := m.Run()

	os.Remove(testng)

	os.Exit(r)
}

func TestPrintf(t *testing.T) {
	// The printf builtin should not return any values, unlike fmt.Printf.
	out, err := exec.Command(testng, "-e", `printf("%x", 42)`).CombinedOutput()
	if err != nil {
		t.Errorf("testng failed: %v\n%s", err, out)
	}

	got := string(out)
	want := "2a"
	if got != want {
		t.Errorf("printf returned %q, want %q", got, want)
	}
}

func TestExitMsg(t *testing.T) {
	out, err := exec.Command(testng, "-e", `exit`).CombinedOutput()
	if err == nil {
		t.Fatalf("testng failed: got a nil error. want 'Ctrl-D' exit")
	}
	if got := string(out); !strings.Contains(got, "Ctrl-D") {
		t.Errorf("exit error does not mention Ctrl-D: %q", got)
	}
	switch e := err.(type) {
	case *exec.ExitError:
	default:
		t.Errorf("testng should have failed with a non-zero exit code: %#v", e)
	}
}

func TestGofmt(t *testing.T) {
	exe, err := exec.LookPath("gofmt")

	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(exe, "-d", "-s", ".")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = buf

	err = cmd.Run()
	if err != nil {
		t.Fatalf("error running %s:\n%s\n%v", exe, string(buf.Bytes()), err)
	}

	if len(buf.Bytes()) != 0 {
		t.Errorf("some files were not gofmt'ed:\n%s\n", string(buf.Bytes()))
	}
}
