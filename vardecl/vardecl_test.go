// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vardecl

import (
	"testing"

	"rsc.io/grind/grinder"
	"rsc.io/grind/grindtest"
)

/*
func TestVardeclGolden(t *testing.T) {
	matches, err := filepath.Glob("testdata/vardecl-*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no testdata found")
	}
	fset := token.NewFileSet()
	for _, file := range matches {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(f.Decls) == 0 {
			t.Errorf("%s: no decls", file)
			continue
		}
		fn, ok := f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
		if !ok {
			t.Errorf("%s: found %T, want *ast.FuncDecl", file, f.Decls[0])
			continue
		}
		vars := Analyze(fset, fn.Body)
		dump := PrintVars(fset, vars)
		base := strings.TrimSuffix(file, ".go")
		golden, _ := ioutil.ReadFile(base + ".dump")
		if !bytes.Equal(dump, golden) {
			ioutil.WriteFile(base+".dump.xxx", dump, 0666)
			t.Errorf("%s: wrong; diff %s.dump.xxx %s.dump", file, base, base)
			continue
		}
	}
}
*/

func TestVardecl(t *testing.T) {
	grindtest.TestGlob(t, "testdata/grind-*.go", []grinder.Func{Grind})
}
