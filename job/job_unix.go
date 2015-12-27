// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

// +build !windows

package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func findExec(name string) error {
	fi, err := os.Stat(name)
	if err != nil {
		return err
	}
	if fi.IsDir() || fi.Mode()&0111 == 0 {
		return fmt.Errorf("%q is not an executable", name)
	}
	return nil
}

func findExecInPath(name string, env []string) (string, error) {
	if strings.Contains(name, "/") {
		err := findExec(name)
		if err == nil {
			return name, nil
		}
		return "", err
	}

	var path []string
	for _, s := range env {
		if strings.HasPrefix(s, "PATH=") {
			path = filepath.SplitList(s[len("PATH="):])
			break
		}
	}
	if len(path) == 0 {
		return "", fmt.Errorf("cannot find %q, no PATH", name)
	}

	for _, dir := range path {
		if dir == "" {
			dir = "."
		}
		file := dir + "/" + name
		if err := findExec(file); err == nil {
			return file, nil
		}
	}
	return "", fmt.Errorf("cannot find %q in PATH", name)
}

func exitCode(state *os.ProcessState) int {
	return state.Sys().(syscall.WaitStatus).ExitStatus()
}

func tcsetattr(fd uintptr, termios *syscall.Termios) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, termiosSet, uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if err != 0 {
		fmt.Printf("tcsetattr: %v\n", err)
		return err
	}
	return nil
}

func tcgetattr(fd uintptr, termios *syscall.Termios) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, termiosGet, uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if err != 0 {
		fmt.Printf("tcgetattr: %v\n", err)
		return err
	}
	return nil
}

func tcsetwinsize(fd uintptr) error {
	// TODO: this is for darwin, check linux
	var sz struct {
		rows uint16
		cols uint16
		hpx  uint16
		vpx  uint16
	}
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&sz)), 0, 0, 0)
	if err != 0 {
		fmt.Printf("tcsetwinsize: get %v\n", err)
		return err
	}
	fmt.Printf("sz: %#+v\n", sz)
	if sz.rows > 0 && sz.cols > 0 {
		// TODO: set $LINES and $COLUMNS
	}
	return nil
}
