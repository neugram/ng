// Copyright 2018 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command ng-gengo generates a Go package file from a Neugram script.
// ng-gengo is a debugging command for testing the ng/gengo package.
//
//  Usage: ng-gengo [options] file1.ng
//
//  ex:
//   $> ng-gengo ./eval/testdata/defer1.ng
//   $> ng-gengo ./eval/testdata/defer1.ng > defer1.go
//   $> ng-gengo -pkg=main ./eval/testdata/defer1.ng
//
//  options:
//    -pkg string
//      	name of the output Go package (default "main")
//
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"neugram.io/ng/gengo"
)

func main() {
	log.SetPrefix("gengo: ")
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: ng-gengo [options] file1.ng

ex:
 $> ng-gengo ./eval/testdata/defer1.ng
 $> ng-gengo ./eval/testdata/defer1.ng > defer1.go
 $> ng-gengo -pkg=main ./eval/testdata/defer1.ng

options:
`,
		)
		flag.PrintDefaults()
	}

	pkg := flag.String("pkg", "main", "name of the output Go package")

	flag.Parse()

	if *pkg == "" {
		flag.Usage()
		log.Fatalf("invalid output Go package name")
	}

	if flag.NArg() != 1 {
		flag.Usage()
		log.Fatalf("missing path to Neugram script")
	}

	src, err := gengo.GenGo(flag.Arg(0), *pkg)
	if err != nil {
		log.Fatalf("could not generate Go package %s: %v", *pkg, err)
	}

	_, err = os.Stdout.Write(src)
	if err != nil {
		log.Fatal(err)
	}
}
