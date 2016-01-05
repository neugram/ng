// Copyright 2015 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package shell

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// executable returns the path to the executable of this process.
func executable() (string, error) {
	pid := int32(os.Getpid())

	// TODO: other BSDs have different sysctls.
	const CTL_KERN = 1
	var cfg [4]int32
	if runtime.GOOS == "darwin" {
		const KERN_PROCARGS = 38 // <sys/sysctl.h>
		cfg = [4]int32{CTL_KERN, KERN_PROCARGS, pid, -1}
	} else {
		panic(fmt.Sprintf("executable: unsupported GOOS: %q", runtime.GOOS))
	}

	// Get the size of the process args, then take the first of
	// the NUL-separated list.
	var n uintptr
	np := unsafe.Pointer(&n)
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&cfg[0])),
		4, 0, uintptr(np), 0, 0,
	)
	if errno != 0 {
		return "", errno
	}
	if n == 0 {
		return "", errors.New("shell executable: sysctl returned zero size")
	}
	buf := make([]byte, n)
	_, _, errno = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&cfg[0])),
		4, uintptr(unsafe.Pointer(&buf[0])), uintptr(np), 0, 0,
	)
	if errno != 0 {
		return "", errno
	}
	if i := bytes.IndexByte(buf, 0); i > 0 {
		buf = buf[:i]
	}
	path := string(buf)
	if !filepath.IsAbs(path) {
		path = filepath.Join(initialCwd, string(buf))
	}
	return path, nil
}

func init() {
	var err error
	initialCwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

var initialCwd string
