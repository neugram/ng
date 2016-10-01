// Copyright 2016 The Neugram Authors. All rights reserved.
// See the LICENSE file for rights to use this source code.

package shell

import "os"

func init() {
	var err error
	initialCwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

var initialCwd string
