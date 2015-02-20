// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import (
	"bytes"
	"fmt"
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

		g := Build(fset, fn.Body, isIdentOrAssign)
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

func isIdentOrAssign(x ast.Node) bool {
	switch x.(type) {
	case *ast.Ident, *ast.AssignStmt:
		return true
	}
	return false
}

func TestFlowdef(t *testing.T) {
	matches, err := filepath.Glob("testdata/flowdef-*.go")
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

		g := Build(fset, fn.Body, identUpdate)
		dot := g.Dot(nil)
		base := strings.TrimSuffix(file, ".go")
		golden, _ := ioutil.ReadFile(base + ".dot")
		if !bytes.Equal(dot, golden) {
			ioutil.WriteFile(base+".dot.xxx", dot, 0666)
			t.Errorf("%s: wrong graph; have %s.dot.xxx, want %s.dot", file, base, base)
			continue
		}

		m := newIdentMatcher(fset)
		g.Dataflow(m)

		var buf bytes.Buffer
		for _, x := range m.list {
			fmt.Fprintf(&buf, "%s\n", m.nodeIn(x))
		}
		eq := buf.Bytes()
		golden, _ = ioutil.ReadFile(base + ".reach")
		if !bytes.Equal(eq, golden) {
			ioutil.WriteFile(base+".reach.xxx", eq, 0666)
			t.Errorf("%s: wrong dataflow; have %s.reach.xxx, want %s.reach", file, base, base)
			continue
		}
	}
}

func identUpdate(x ast.Node) bool {
	switch x := x.(type) {
	case *ast.DeclStmt:
		return true
	case *ast.ParenExpr:
		return identUpdate(x.X)
	case *ast.Ident:
		return true
	case *ast.AssignStmt:
		for _, y := range x.Lhs {
			if identUpdate(y) {
				return true
			}
		}
	case *ast.IncDecStmt:
		if identUpdate(x.X) {
			return true
		}
	}
	return false
}

type identMatcher struct {
	in   map[ast.Node][]ast.Node
	out  map[ast.Node][]ast.Node
	list []ast.Node
	fset *token.FileSet
}

func newIdentMatcher(fset *token.FileSet) *identMatcher {
	return &identMatcher{
		in:   make(map[ast.Node][]ast.Node),
		out:  make(map[ast.Node][]ast.Node),
		fset: fset,
	}
}

func (m *identMatcher) nodeIn(x ast.Node) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s:", nodeLabel(m.fset, x))
	for _, y := range m.in[x] {
		fmt.Fprintf(&buf, " %s", nodeLabel(m.fset, y))
	}
	return buf.String()
}

func (m *identMatcher) nodeOut(x ast.Node) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s:", nodeLabel(m.fset, x))
	for _, y := range m.out[x] {
		fmt.Fprintf(&buf, " %s", nodeLabel(m.fset, y))
	}
	return buf.String()
}

func (m *identMatcher) Init(x ast.Node) {
	m.in[x] = nil
}

func (m *identMatcher) Transfer(x ast.Node) {
	_, ok := m.out[x]
	if !ok {
		m.list = append(m.list, x)
	}

	switch x := x.(type) {
	case *ast.DeclStmt, *ast.IncDecStmt:
		m.out[x] = []ast.Node{x}
		return
	case *ast.AssignStmt:
		for _, y := range x.Lhs {
			for {
				yy, ok := y.(*ast.ParenExpr)
				if !ok {
					break
				}
				y = yy.X
			}
			_, ok := y.(*ast.Ident)
			if !ok {
				continue
			}
			m.out[x] = []ast.Node{x}
			return
		}
	}
	m.out[x] = m.in[x]
}

func (m *identMatcher) Join(x, y ast.Node) bool {
	new := mergef(m.in[x], m.out[y])
	if len(new) > len(m.in[x]) {
		m.in[x] = new
		return true
	}
	return false
}
