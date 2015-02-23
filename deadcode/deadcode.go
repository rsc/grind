// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deadcode

import (
	"go/ast"

	"rsc.io/grind/block"
	"rsc.io/grind/grinder"
)

func Grind(ctxt *grinder.Context, pkg *grinder.Package) {
	grinder.GrindFuncDecls(ctxt, pkg, grindFunc)
}

func grindFunc(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	blocks := block.Build(pkg.FileSet, fn.Body)
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		var list []ast.Stmt
		switch x := x.(type) {
		default:
			return true
		case *ast.BlockStmt:
			list = x.List
		case *ast.CommClause:
			list = x.Body
		case *ast.CaseClause:
			list = x.Body
		}

		for i := 0; i < len(list); i++ {
			x := list[i]
			if !fallsThrough(x) {
				end := i + 1
				for end < len(list) && !isGotoTarget(blocks, list[end]) {
					end++
				}
				if end > i+1 {
					edit.Delete(x.End(), list[end-1].End())
					i = end - 1 // after i++, next iteration starts at end
				}
			}
		}
		return true
	})
}

func fallsThrough(x ast.Stmt) bool {
	switch x.(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return false
	}
	// TODO: for loop, switch etc with certain bodies
	return true
}

func isGotoTarget(blocks *block.Graph, x ast.Stmt) bool {
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
