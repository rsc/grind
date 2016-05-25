// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package grinder

import (
	"go/ast"
	"go/token"

	"go/types"

	"rsc.io/grind/block"
)

func Unlabel(x ast.Stmt) ast.Stmt {
	for {
		y, ok := x.(*ast.LabeledStmt)
		if !ok {
			return x
		}
		x = y.Stmt
	}
}

func IsGotoTarget(blocks *block.Graph, x ast.Stmt) bool {
	for {
		y, ok := x.(*ast.LabeledStmt)
		if !ok {
			return false
		}
		if len(blocks.Goto[y.Label.Name]) > 0 {
			return true
		}
		x = y.Stmt
	}
}

func IsTerminatingStmt(blocks *block.Graph, x ast.Stmt) bool {
	// Like http://golang.org/ref/spec#Terminating_statements
	// but added break and continue for use in non-end-of-function
	// contexts.
	label := ""
	for {
		y, ok := x.(*ast.LabeledStmt)
		if !ok {
			break
		}
		label = y.Label.Name
		x = y.Stmt
	}

	switch x := x.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		switch x.Tok {
		case token.BREAK, token.CONTINUE, token.GOTO:
			return true
		}
	case *ast.IfStmt:
		return x.Else != nil && IsTerminatingStmt(blocks, x.Body) && IsTerminatingStmt(blocks, x.Else)
	case *ast.ForStmt:
		return x.Cond == nil && len(blocks.Break[label]) == 0 && !hasBreak(x.Body)
	case *ast.SwitchStmt:
		if len(blocks.Break[label]) > 0 || hasBreak(x.Body) {
			return false
		}
		hasDefault := false
		for _, cas := range x.Body.List {
			cas := cas.(*ast.CaseClause)
			if cas.List == nil {
				hasDefault = true
			}
			if len(cas.Body) == 0 {
				return false
			}
			last := cas.Body[len(cas.Body)-1]
			if !IsTerminatingStmt(blocks, last) && !isFallthrough(last) {
				return false
			}
		}
		if !hasDefault {
			return false
		}
		return true
	case *ast.TypeSwitchStmt:
		if len(blocks.Break[label]) > 0 || hasBreak(x.Body) {
			return false
		}
		hasDefault := false
		for _, cas := range x.Body.List {
			cas := cas.(*ast.CaseClause)
			if cas.List == nil {
				hasDefault = true
			}
			if len(cas.Body) == 0 {
				return false
			}
			last := cas.Body[len(cas.Body)-1]
			if !IsTerminatingStmt(blocks, last) && !isFallthrough(last) {
				return false
			}
		}
		if !hasDefault {
			return false
		}
		return true
	case *ast.SelectStmt:
		if len(blocks.Break[label]) > 0 || hasBreak(x.Body) {
			return false
		}
		for _, cas := range x.Body.List {
			cas := cas.(*ast.CommClause)
			if len(cas.Body) == 0 {
				return false
			}
			last := cas.Body[len(cas.Body)-1]
			if !IsTerminatingStmt(blocks, last) && !isFallthrough(last) {
				return false
			}
		}
		return true
	}
	return false
}

func isFallthrough(x ast.Stmt) bool {
	xx, ok := x.(*ast.BranchStmt)
	return ok && xx.Tok == token.FALLTHROUGH
}

func hasBreak(x ast.Stmt) bool {
	found := false
	ast.Inspect(x, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			return false
		case *ast.BranchStmt:
			if x.Tok == token.BREAK && x.Label == nil {
				found = true
			}
		case ast.Expr:
			return false
		}
		return !found
	})
	return found
}

func (pkg *Package) LookupAtPos(fn *ast.FuncDecl, pos token.Pos, name string) types.Object {
	scope := pkg.Info.Scopes[fn.Type]
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		if x == nil {
			return false
		}
		if pos < x.Pos() || x.End() <= pos {
			return false
		}
		s := pkg.Info.Scopes[x]
		if s != nil {
			scope = s
		}
		return true
	})

	pkgScope := pkg.Types.Scope()
	for s := scope; s != nil; s = s.Parent() {
		obj := s.Lookup(name)
		if obj != nil && (s == pkgScope || obj.Pos() < pos) {
			return obj
		}
	}

	return nil
}

// BlockList returns the list of statements contained by the block x,
// when x is an *ast.BlockStmt, *ast.CommClause, or *ast.CaseClause.
// Otherwise BlockList returns nil.
func BlockList(x ast.Node) []ast.Stmt {
	switch x := x.(type) {
	case *ast.BlockStmt:
		return x.List
	case *ast.CommClause:
		return x.Body
	case *ast.CaseClause:
		return x.Body
	}
	return nil
}
