// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package shell

import (
	"os"
)

// executable returns the path to the executable of this process.
func executable() (string, error) {
	// TODO: handle " (deleted)"
	return os.Readlink("/proc/self/exe")
}
