// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Derived from http://pdos.csail.mit.edu/xoc/xoc-unstable.tgz
// xoc-unstable/zeta/xoc/flow.zeta.
//
// Copyright (c) 2003-2007 Russ Cox, Tom Bergan, Austin Clements,
//                         Massachusetts Institute of Technology
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

//go:generate ./mksplit.sh

// Package flow provides access to control flow graph computation
// and analysis for Go programs.
package flow

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type Graph struct {
	FileSet *token.FileSet
	Start   ast.Node
	End     ast.Node
	Follow  map[ast.Node][]ast.Node
}

type Computation interface {
	Init(ast.Node)
	Transfer(ast.Node)
	Join(ast.Node, ast.Node) bool
}

type builder struct {
	interesting  func(ast.Node) bool
	followCache  map[ast.Node][]ast.Node
	end          ast.Node
	need         map[ast.Node]bool
	trimmed      map[ast.Node]bool
	followed     map[ast.Node]bool
	brk          []ast.Node
	cont         []ast.Node
	fall         []ast.Node
	brkLabel     map[string][]ast.Node
	contLabel    map[string][]ast.Node
	gotoLabel    map[string]ast.Node
	isGotoTarget map[string]bool
	stmtLabel    map[ast.Stmt]string
}

// Build constructs a control flow graph,
// filtered to include only interesting nodes.
func Build(fset *token.FileSet, body *ast.BlockStmt, interesting func(ast.Node) bool) *Graph {
	start := &ast.Ident{Name: "_cfg_start_"}
	end := &ast.Ident{Name: "_cfg_end_"}
	b := &builder{
		interesting:  interesting,
		followCache:  make(map[ast.Node][]ast.Node),
		end:          end,
		need:         make(map[ast.Node]bool),
		trimmed:      make(map[ast.Node]bool),
		followed:     make(map[ast.Node]bool),
		brkLabel:     make(map[string][]ast.Node),
		contLabel:    make(map[string][]ast.Node),
		gotoLabel:    make(map[string]ast.Node),
		isGotoTarget: make(map[string]bool),
		stmtLabel:    make(map[ast.Stmt]string),
	}

	b.scanGoto(body)
	b.followCache[start] = b.trimList(b.follow(body, []ast.Node{end}))
	return &Graph{
		FileSet: fset,
		Start:   start,
		End:     end,
		Follow:  b.followCache,
	}
}

func (b *builder) trimList(list []ast.Node) []ast.Node {
	if len(list) == 0 {
		return list
	}
	return mergef(b.trim(list[0]), b.trimList(list[1:]))
}

func (b *builder) trim(x ast.Node) []ast.Node {
	if x == nil {
		return nil
	}
	if !b.trimmed[x] {
		b.trimmed[x] = true
		fol := b.followCache[x]
		b.followCache[x] = []ast.Node{x} // during recursion
		b.followCache[x] = b.trimList(fol)
	}
	if !b.need[x] && len(b.followCache[x]) > 0 {
		return b.followCache[x]
	}
	return []ast.Node{x}
}

func mergef(l1, l2 []ast.Node) []ast.Node {
	if l1 == nil {
		return l2
	}
	if l2 == nil {
		return l1
	}
	var out []ast.Node
	seen := map[ast.Node]bool{}
	for _, x := range l1 {
		out = append(out, x)
		seen[x] = true
	}
	for _, x := range l2 {
		if !seen[x] {
			out = append(out, x)
		}
	}
	return out
}

func (b *builder) scanGoto(x ast.Node) {
	switch x := x.(type) {
	case *ast.LabeledStmt:
		b.gotoLabel[x.Label.Name] = x
	case *ast.BranchStmt:
		if x.Tok == token.GOTO {
			b.isGotoTarget[x.Label.Name] = true
		}
	}

	for _, y := range astSplit(x) {
		b.scanGoto(y)
	}
}

func (b *builder) followCond(cond ast.Expr, btrue, bfalse []ast.Node) []ast.Node {
	// Could be more precise by looking at cond to see if it is
	// a constant true or false, but that might lead to code
	// rewrites that make dead code no longer compile.
	switch x := cond.(type) {
	case *ast.BinaryExpr:
		switch x.Op {
		case token.LAND:
			return b.followCond(x.X, b.followCond(x.Y, btrue, bfalse), bfalse)
		case token.LOR:
			return b.followCond(x.X, btrue, b.followCond(x.Y, btrue, bfalse))
		}
	case *ast.UnaryExpr:
		switch x.Op {
		case token.NOT:
			return b.followCond(x.X, bfalse, btrue)
		}
	case *ast.ParenExpr:
		return b.followCond(x.X, btrue, bfalse)
	}
	return b.follow(cond, mergef(btrue, bfalse))
}

func (b *builder) addNode(x ast.Node, out []ast.Node) []ast.Node {
	b.followCache[x] = out
	if !b.need[x] && !b.interesting(x) {
		return out
	}
	b.need[x] = true
	return []ast.Node{x}
}

func (b *builder) previsit(x ast.Node, out []ast.Node) []ast.Node {
	list := astSplit(x)
	for i := len(list) - 1; i >= 0; i-- {
		out = b.follow(list[i], out)
	}
	out = b.addNode(x, out)
	return out
}

func (b *builder) postvisit(x ast.Node, out []ast.Node) []ast.Node {
	out = b.addNode(x, out)
	list := astSplit(x)
	for i := len(list) - 1; i >= 0; i-- {
		out = b.follow(list[i], out)
	}
	return out
}

func (b *builder) follow(x ast.Node, out []ast.Node) []ast.Node {
	switch x.(type) {
	case ast.Expr, ast.Stmt:
		// ok
	default:
		return out
	}

	if b.followed[x] {
		panic("flow: already followed")
	}
	b.followed[x] = true

	if x, ok := x.(ast.Expr); ok {
		if x, ok := x.(*ast.BinaryExpr); ok {
			switch x.Op {
			case token.LAND:
				return b.followCond(x.X, b.follow(x.Y, out), out)
			case token.LOR:
				return b.followCond(x.X, out, b.follow(x.Y, out))
			}
		}
		return b.postvisit(x, out)
	}

	switch x := x.(type) {
	case *ast.BranchStmt:
		switch x.Tok {
		case token.BREAK:
			if x.Label != nil {
				return b.brkLabel[x.Label.Name]
			}
			return b.brk

		case token.CONTINUE:
			if x.Label != nil {
				return b.contLabel[x.Label.Name]
			}
			return b.cont

		case token.GOTO:
			return []ast.Node{b.gotoLabel[x.Label.Name]}

		case token.FALLTHROUGH:
			return b.fall
		}

	case *ast.LabeledStmt:
		b.stmtLabel[x.Stmt] = x.Label.Name
		out = b.follow(x.Stmt, out)
		if b.isGotoTarget[x.Label.Name] {
			out = b.addNode(x, out)
		}
		return out

	case *ast.ForStmt:
		oldBrk := b.brk
		b.brk = out
		oldCont := b.cont
		b.cont = b.follow(x.Post, []ast.Node{x}) // note: x matches b.addNode below, cleaned up by trim
		if label := b.stmtLabel[x]; label != "" {
			b.brkLabel[label] = b.brk
			b.contLabel[label] = b.cont
		}
		bin := b.follow(x.Body, b.cont)
		var condOut []ast.Node
		if x.Cond == nil {
			condOut = bin
		} else {
			condOut = mergef(bin, out)
		}
		b.brk = oldBrk
		b.cont = oldCont
		return b.follow(x.Init, b.addNode(x, b.follow(x.Cond, condOut)))

	case *ast.IfStmt:
		return b.followCond(x.Cond, b.follow(x.Body, out), b.follow(x.Else, out))

	case *ast.RangeStmt:
		oldBrk := b.brk
		b.brk = out
		oldCont := b.cont
		b.cont = []ast.Node{x} // note: x matches b.addNode below, cleaned up by trim
		if label := b.stmtLabel[x]; label != "" {
			b.brkLabel[label] = b.brk
			b.contLabel[label] = b.cont
		}
		out = b.addNode(x, mergef(b.follow(x.Key, b.follow(x.Value, b.follow(x.Body, b.cont))), out))
		b.brk = oldBrk
		b.cont = oldCont
		return b.follow(x.X, out)

	case *ast.ReturnStmt:
		return b.followExprs(x.Results, []ast.Node{b.end})

	case *ast.SelectStmt:
		oldBrk := b.brk
		b.brk = out
		if label := b.stmtLabel[x]; label != "" {
			b.brkLabel[label] = b.brk
		}
		var allCasOut []ast.Node
		for _, xcas := range x.Body.List {
			cas := xcas.(*ast.CommClause)
			casOut := b.followStmts(cas.Body, out)
			switch comm := cas.Comm.(type) {
			case *ast.AssignStmt:
				for i := len(comm.Lhs) - 1; i >= 0; i-- {
					casOut = b.follow(comm.Lhs[i], casOut)
				}
			}
			allCasOut = mergef(allCasOut, casOut)
		}
		out = allCasOut
		for i := len(x.Body.List) - 1; i >= 0; i-- {
			cas := x.Body.List[i].(*ast.CommClause)
			switch comm := cas.Comm.(type) {
			case *ast.SendStmt:
				out = b.follow(comm.Value, out)
				out = b.follow(comm.Chan, out)
			case *ast.AssignStmt:
				out = b.follow(comm.Rhs[0], out)
				// Lhs evaluated when case is selected; see above.
			case *ast.ExprStmt:
				out = b.follow(comm.X, out)
			}
		}
		b.brk = oldBrk
		return out

	case *ast.SwitchStmt:
		oldBrk := b.brk
		b.brk = out
		oldFall := b.fall
		b.fall = nil
		if label := b.stmtLabel[x]; label != "" {
			b.brkLabel[label] = b.brk
		}

		// default is last resort, after all switch expressions are evaluated
		var needFall *ast.CaseClause
		nextCase := out
		for i := len(x.Body.List) - 1; i >= 0; i-- {
			cas := x.Body.List[i].(*ast.CaseClause)
			if cas.List == nil {
				// Found default.
				// Set up fallthrough link if needed.
				if len(cas.Body) > 0 && isFallthrough(cas.Body[len(cas.Body)-1]) {
					if i+1 < len(x.Body.List) {
						needFall = x.Body.List[i+1].(*ast.CaseClause)
						b.fall = []ast.Node{needFall}
					}
				}
				nextCase = b.followStmts(cas.Body, out)
			}
		}

		// non-default cases
		for i := len(x.Body.List) - 1; i >= 0; i-- {
			cas := x.Body.List[i].(*ast.CaseClause)
			if cas.List == nil {
				continue
			}
			casOut := b.followStmts(cas.Body, out)
			if cas == needFall {
				casOut = b.addNode(cas, casOut)
			}
			b.fall = casOut
			for j := len(cas.List) - 1; j >= 0; j-- {
				nextCase = b.follow(cas.List[j], mergef(nextCase, casOut))
			}
		}

		b.brk = oldBrk
		b.fall = oldFall
		return b.follow(x.Init, b.follow(x.Tag, nextCase))

	case *ast.TypeSwitchStmt:
		// Easier than switch: no fallthrough, case values are not executable.
		oldBrk := b.brk
		b.brk = out
		if label := b.stmtLabel[x]; label != "" {
			b.brkLabel[label] = b.brk
		}

		var allCasOut []ast.Node
		defaultOut := out
		for i := len(x.Body.List) - 1; i >= 0; i-- {
			cas := x.Body.List[i].(*ast.CaseClause)
			if cas.List == nil {
				defaultOut = nil
			}
			allCasOut = mergef(allCasOut, b.followStmts(cas.Body, out))
		}
		b.brk = oldBrk
		return b.follow(x.Init, b.follow(x.Assign, mergef(allCasOut, defaultOut)))
	}

	return b.previsit(x, out)
}

func (b *builder) followExprs(x []ast.Expr, out []ast.Node) []ast.Node {
	for i := len(x) - 1; i >= 0; i-- {
		out = b.follow(x[i], out)
	}
	return out
}

func (b *builder) followStmts(x []ast.Stmt, out []ast.Node) []ast.Node {
	for i := len(x) - 1; i >= 0; i-- {
		out = b.follow(x[i], out)
	}
	return out
}

func isFallthrough(x ast.Stmt) bool {
	br, ok := x.(*ast.BranchStmt)
	return ok && br.Tok == token.FALLTHROUGH
}

func (g *Graph) Dataflow(compute Computation) {
	if g == nil || g.Start == nil {
		return
	}

	compute.Init(g.Start)
	var workq, nextq []ast.Node
	workq = append(workq, g.Start)
	for {
		for _, x := range workq {
			compute.Transfer(x)
			for _, y := range g.Follow[x] {
				if compute.Join(x, y) {
					nextq = append(nextq, y)
				}
			}
		}
		workq, nextq = nextq, workq[:0]
	}
}

type printer struct {
	visited map[ast.Node]bool
	id      map[ast.Node]int
	buf     bytes.Buffer
	edge    func(ast.Node, ast.Node) string
	g       *Graph
}

func (p *printer) name(x ast.Node) string {
	if x == nil {
		return "nil"
	}
	if p.id[x] == 0 {
		p.id[x] = len(p.id) + 1
	}
	return fmt.Sprintf("n%d", p.id[x])
}

var escaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
)

func (p *printer) print(x ast.Node) {
	if x == nil || p.visited[x] {
		return
	}
	p.visited[x] = true
	name := p.name(x) // allocate id now
	for _, y := range p.g.Follow[x] {
		p.print(y)
	}

	pos := p.g.FileSet.Position(x.Pos())
	label := fmt.Sprintf("%T %s:%d", x, pos.Filename, pos.Line)
	switch x := x.(type) {
	case *ast.Ident:
		label = x.Name + " " + label
	case *ast.SelectorExpr:
		label = "." + x.Sel.Name + " " + label
	}

	fmt.Fprintf(&p.buf, "%s [label=\"%s\"];\n", name, escaper.Replace(label))
	for _, y := range p.g.Follow[x] {
		e := escaper.Replace(p.edge(x, y))
		if strings.HasPrefix(e, "!") {
			e = e[1:] + `", color="red`
		}
		fmt.Fprintf(&p.buf, "%s -> %s [label=\"%s\"];\n", name, p.name(y), e)
	}
}

func (g *Graph) Dot(edge func(src, dst ast.Node) string) []byte {
	if edge == nil {
		edge = func(src, dst ast.Node) string { return "" }
	}
	p := &printer{
		visited: make(map[ast.Node]bool),
		id:      make(map[ast.Node]int),
		edge:    edge,
		g:       g,
	}

	fmt.Fprintf(&p.buf, "digraph cfg {\n")
	p.print(g.Start)
	fmt.Fprintf(&p.buf, "}\n")
	return p.buf.Bytes()
}
