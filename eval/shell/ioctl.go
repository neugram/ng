// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package shell

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func ioctl(fd uintptr, request ioctlRequest, argp unsafe.Pointer) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(request), uintptr(argp), 0, 0, 0)
	if err != 0 {
		return os.NewSyscallError(fmt.Sprintf("ioctl %s", request), err)
	}
	return nil
}

type ioctlRequest uintptr

const (
	_TIOCSPGRP  = ioctlRequest(syscall.TIOCSPGRP)
	_TIOCGPGRP  = ioctlRequest(syscall.TIOCGPGRP)
	_TIOCGWINSZ = ioctlRequest(syscall.TIOCGWINSZ)
)

var ioctlRequests = map[ioctlRequest]string{
	_TIOCGETS:   "TIOCGETS",
	_TIOCSETS:   "TIOCSETS",
	_TIOCSPGRP:  "TIOCSPGRP",
	_TIOCGWINSZ: "TIOCGWINSZ",
}

func (r ioctlRequest) String() string {
	s := ioctlRequests[r]
	if s == "" {
		s = "Unknown"
	}
	return fmt.Sprintf("%s(0x%x)", s, uintptr(r))
}

func tcgetattr(fd uintptr) (syscall.Termios, error) {
	var termios syscall.Termios
	return termios, ioctl(fd, _TIOCGETS, unsafe.Pointer(&termios))
}

func tcsetattr(fd uintptr, termios *syscall.Termios) error {
	return ioctl(fd, _TIOCSETS, unsafe.Pointer(termios))
}

func tcsetpgrp(fd uintptr, pgrp int) error {
	pgid := int32(pgrp) // TODO: uintptr?
	return ioctl(fd, _TIOCSPGRP, unsafe.Pointer(&pgid))
}

func tcgetpgrp(fd uintptr) (int, error) {
	var pgid int64
	err := ioctl(fd, _TIOCGPGRP, unsafe.Pointer(&pgid))
	return int(pgid), err
}

func WindowSize(fd uintptr) (rows, cols int, err error) {
	var sz struct{ rows, cols, _, _ uint16 }
	if err := ioctl(fd, _TIOCGWINSZ, unsafe.Pointer(&sz)); err != nil {
		return 0, 0, err
	}
	return int(sz.rows), int(sz.cols), nil
}
