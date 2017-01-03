// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package shell

import "syscall"

const (
	_TIOCGETS = ioctlRequest(syscall.TIOCGETA)
	_TIOCSETS = ioctlRequest(syscall.TIOCSETA)
)
