// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGolden(t *testing.T) {
	matches, err := filepath.Glob("testdata/cfg-*.go")
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

		g := Build(fset, fn.Body, isIdent)
		dot := g.Dot(nil)
		base := strings.TrimSuffix(file, ".go")
		golden, _ := ioutil.ReadFile(base + ".dot")
		if bytes.Equal(dot, golden) {
			continue
		}
		ioutil.WriteFile(base+".dot.xxx", dot, 0666)
		t.Errorf("%s: wrong graph; have %s.dot.xxx, want %s.dot", file, base, base)
	}
}

func isIdent(x ast.Node) bool {
	_, ok := x.(*ast.Ident)
	return ok
}
