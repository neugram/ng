// Copyright 2016 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

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
