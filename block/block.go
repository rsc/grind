// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package block

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
)

type Graph struct {
	Start   *Block
	Map     map[ast.Node]*Block
	Label   map[string]*ast.LabeledStmt
	Goto    map[string][]*ast.BranchStmt
	FileSet *token.FileSet
}

type Block struct {
	Depth  int
	ID     int
	Parent *Block
	Child  []*Block
	Root   ast.Node
}

type builder struct {
	g      *Graph
	nblock int
}

type builderVisitor struct {
	builder *builder
	current *Block
}

func (v *builderVisitor) Visit(x ast.Node) ast.Visitor {
	v.builder.g.Map[x] = v.current
	switch x := x.(type) {
	case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.CaseClause, *ast.CommClause:
		b := &Block{
			Depth:  v.current.Depth + 1,
			ID:     v.builder.nblock,
			Parent: v.current,
			Root:   x,
		}
		v.builder.nblock++
		v.current.Child = append(v.current.Child, b)
		return &builderVisitor{v.builder, b}
	case *ast.LabeledStmt:
		v.builder.g.Label[x.Label.Name] = x
	case *ast.BranchStmt:
		if x.Tok == token.GOTO {
			v.builder.g.Goto[x.Label.Name] = append(v.builder.g.Goto[x.Label.Name], x)
		}
	}
	return v
}

func Build(fset *token.FileSet, x ast.Node) *Graph {
	g := &Graph{
		Start:   &Block{},
		Map:     make(map[ast.Node]*Block),
		Label:   make(map[string]*ast.LabeledStmt),
		Goto:    make(map[string][]*ast.BranchStmt),
		FileSet: fset,
	}
	v := &builderVisitor{builder: &builder{g, 1}, current: g.Start}
	ast.Walk(v, x)
	return g
}

func (g *Graph) Dump() []byte {
	var buf bytes.Buffer
	var dump func(*Block)
	dump = func(b *Block) {
		fmt.Fprintf(&buf, "%d: depth=%d", b.ID, b.Depth)
		if b.Parent != nil {
			fmt.Fprintf(&buf, " parent=%d", b.Parent.ID)
		}
		if len(b.Child) > 0 {
			fmt.Fprintf(&buf, " child=")
			for i, c := range b.Child {
				if i > 0 {
					fmt.Fprintf(&buf, ",")
				}
				fmt.Fprintf(&buf, "%d", c.ID)
			}
		}
		if b.Root != nil {
			pos := g.FileSet.Position(b.Root.Pos())
			fmt.Fprintf(&buf, " root=%T %s:%d", b.Root, pos.Filename, pos.Line)
		}
		fmt.Fprintf(&buf, "\n")

		for _, c := range b.Child {
			dump(c)
		}
	}
	dump(g.Start)
	return buf.Bytes()
}
