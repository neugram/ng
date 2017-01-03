// Copyright 2016 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
