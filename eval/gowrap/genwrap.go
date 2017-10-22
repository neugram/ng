// Copyright 2015 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"neugram.io/ng/eval/gowrap/genwrap"
)

func main() {
	pkgName := os.Args[1]
	b, err := genwrap.GenGo(pkgName, "wrapbuiltin")
	if err != nil {
		log.Fatal(err)
	}
	quotedPkgName := strings.Replace(pkgName, "/", "_", -1)
	err = ioutil.WriteFile("wrapbuiltin/wrap_"+quotedPkgName+".go", b, 0666)
	if err != nil {
		log.Fatal(err)
	}
}
