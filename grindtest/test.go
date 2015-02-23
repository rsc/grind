// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package grindtest

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"rsc.io/grind/grinder"
)

var run = flag.String("grindrun", "", "only run golden tests for files with names matching this regexp")

func TestGlob(t *testing.T, pattern string, grinders []grinder.Func) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	if len(matches) == 0 {
		t.Errorf("no matches for %s", pattern)
		return
	}
	var re *regexp.Regexp
	if *run != "" {
		var err error
		re, err = regexp.Compile(*run)
		if err != nil {
			t.Errorf("invalid regexp passed to -grindrun: %v", err)
			return
		}
	}
	for _, file := range matches {
		if re != nil && !re.MatchString(file) {
			continue
		}
		var buf bytes.Buffer
		ctxt := &grinder.Context{
			Grinders: grinders,
		}
		ctxt.Logf = func(format string, args ...interface{}) {
			fmt.Fprintf(&buf, format, args...)
			if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
				buf.WriteByte('\n')
			}
		}
		pkg := ctxt.GrindFiles(file)
		if pkg == nil || ctxt.Errors {
			t.Errorf("%s: grind failed:\n%s", file, buf.String())
			continue
		}
		data, err := ioutil.ReadFile(file + ".out")
		if err != nil {
			if os.IsNotExist(err) {
				if pkg.Modified(file) {
					t.Errorf("%s: should not modify, but made changes:\n%s", file, grinder.Diff(pkg.OrigSrc(file), pkg.Src(file)))
				}
				continue
			}
			t.Errorf("%s: reading golden output: %v", file, err)
			continue
		}
		want := string(data)
		have := pkg.Src(file)
		if have != want {
			t.Errorf("%s: incorrect output\nhave:\n%s\nwant:\n%s\ndiff want have:\n%s", file, have, want, grinder.Diff(want, have))
		}
	}
}
