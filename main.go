// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	_ "golang.org/x/tools/go/gcimporter15"
	"rsc.io/grind/deadcode"
	"rsc.io/grind/gotoinline"
	"rsc.io/grind/grinder"
	"rsc.io/grind/vardecl"
)

var diff = flag.Bool("diff", false, "print diffs")
var verbose = flag.Bool("v", false, "verbose")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: grind [-diff] [-v] packagepath... (or file...)\n")
	os.Exit(2)
}

var ctxt = grinder.Context{
	Logf: log.Printf,
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
	}

	ctxt.Grinders = []grinder.Func{
		deadcode.Grind,
		gotoinline.Grind,
		vardecl.Grind,
		DeleteUnusedLabels,
	}

	defer func() {
		if ctxt.Errors {
			os.Exit(1)
		}
	}()

	if strings.HasSuffix(flag.Arg(0), ".go") {
		grind(ctxt.GrindFiles(flag.Args()...))
		return
	}

	for _, path := range flag.Args() {
		grind(ctxt.GrindPackage(path))
	}
}

func grind(pkg *grinder.Package) {
	for _, name := range pkg.Filenames {
		if !pkg.Modified(name) {
			continue
		}

		if *diff {
			diffText, err := runDiff([]byte(pkg.OrigSrc(name)), []byte(pkg.Src(name)))
			if err != nil {
				ctxt.Errorf("%v", err)
			}
			os.Stdout.Write(diffText)
			continue
		}

		if err := ioutil.WriteFile(name, []byte(pkg.Src(name)), 0666); err != nil {
			ctxt.Errorf("%v", err)
		}
	}
}

func runDiff(b1, b2 []byte) (data []byte, err error) {
	f1, err := ioutil.TempFile("", "grind-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "grind-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write(b1)
	f2.Write(b2)

	data, err = exec.Command("git", "diff", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	return
}
