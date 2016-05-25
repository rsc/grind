// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vardecl

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"go/types"

	"rsc.io/grind/block"
	"rsc.io/grind/flow"
	"rsc.io/grind/grinder"
)

func Grind(ctxt *grinder.Context, pkg *grinder.Package) {
	grinder.GrindFuncDecls(ctxt, pkg, grindFunc)
}

func grindFunc(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
	vars := analyzeFunc(pkg, edit, fn.Body)
	// fmt.Printf("%s", vardecl.PrintVars(conf.Fset, vars))
	for _, v := range vars {
		spec := v.Decl.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
		if len(spec.Names) > 1 {
			// TODO: Handle decls with multiple variables
			continue
		}
		if pkg.FileSet.Position(v.Decl.Pos()).Line != pkg.FileSet.Position(v.Decl.End()).Line {
			// Declaration spans line. Maybe not great to move or duplicate?
			continue
		}
		keepDecl := false
		for _, d := range v.Defs {
			if d.Init == v.Decl {
				keepDecl = true
				continue
			}
			switch x := d.Init.(type) {
			default:
				panic("unexpected init")
			case *ast.EmptyStmt:
				edit.CopyLine(v.Decl.Pos(), v.Decl.End(), x.Semicolon)
			case *ast.AssignStmt:
				edit.Insert(x.TokPos, ":")
				if !hasType(pkg, fn, x.Rhs[0], x.Lhs[0]) {
					typ := edit.TextAt(spec.Type.Pos(), spec.Type.End())
					if strings.Contains(typ, " ") || typ == "interface{}" || typ == "struct{}" || strings.HasPrefix(typ, "*") {
						typ = "(" + typ + ")"
					}
					edit.Insert(x.Rhs[0].Pos(), typ+"(")
					edit.Insert(x.Rhs[0].End(), ")")
				}
			}
		}
		if !keepDecl {
			edit.DeleteLine(v.Decl.Pos(), v.Decl.End())
		}
	}

	if edit.NumEdits() == 0 {
		initToDecl(ctxt, pkg, edit, fn)
	}
}

func hasType(pkg *grinder.Package, fn *ast.FuncDecl, x, v ast.Expr) bool {
	// Does x (by itself) default to v's type?
	// Find the scope in which x appears.
	xScope := pkg.Info.Scopes[fn.Type]
	ast.Inspect(fn.Body, func(z ast.Node) bool {
		if z == nil {
			return false
		}
		if x.Pos() < z.Pos() || z.End() <= x.Pos() {
			return false
		}
		scope := pkg.Info.Scopes[z]
		if scope != nil {
			xScope = scope
		}
		return true
	})
	xt, err := types.EvalNode(pkg.FileSet, x, pkg.Types, xScope)
	if err != nil {
		return false
	}
	vt := pkg.Info.Types[v]
	if types.Identical(xt.Type, vt.Type) {
		return true
	}

	// Might be untyped.
	vb, ok1 := vt.Type.(*types.Basic)
	xb, ok2 := xt.Type.(*types.Basic)
	if ok1 && ok2 {
		switch xb.Kind() {
		case types.UntypedInt:
			return vb.Kind() == types.Int
		case types.UntypedBool:
			return vb.Kind() == types.Bool
		case types.UntypedRune:
			return vb.Kind() == types.Rune
		case types.UntypedFloat:
			return vb.Kind() == types.Float64
		case types.UntypedComplex:
			return vb.Kind() == types.Complex128
		case types.UntypedString:
			return vb.Kind() == types.String
		}
	}
	return false
}

func analyzeFunc(pkg *grinder.Package, edit *grinder.EditBuffer, body *ast.BlockStmt) []*Var {
	const debug = false

	// Build list of candidate var declarations.
	inClosure := make(map[*ast.Object]bool)
	var objs []*ast.Object
	vardecl := make(map[*ast.Object]*ast.DeclStmt)
	ast.Inspect(body, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.DeclStmt:
			decl := x.Decl.(*ast.GenDecl)
			if len(decl.Specs) != 1 || decl.Tok != token.VAR {
				break
			}
			spec := decl.Specs[0].(*ast.ValueSpec)
			if len(spec.Values) > 0 {
				break
			}
			for _, id := range spec.Names {
				if id.Obj != nil {
					objs = append(objs, id.Obj)
					vardecl[id.Obj] = x
				}
			}
		case *ast.FuncLit:
			ast.Inspect(x, func(x ast.Node) bool {
				switch x := x.(type) {
				case *ast.Ident:
					if x.Obj != nil {
						inClosure[x.Obj] = true
					}
				}
				return true
			})
			return false
		}
		return true
	})

	// Compute block information for entire AST.
	blocks := block.Build(pkg.FileSet, body)

	var vars []*Var
	// Handle each variable separately.
	for _, obj := range objs {
		// For now, refuse to touch variables shared with closures.
		// Could instead treat those variables as having their addresses
		// taken at the point where the closure appears in the source code.
		if inClosure[obj] {
			continue
		}
		// Build flow graph of nodes relevant to v.
		g := flow.Build(pkg.FileSet, body, func(x ast.Node) bool {
			return needForObj(pkg, obj, x)
		})

		// Find reaching definitions.
		m := newIdentMatcher(pkg, g, obj)
		g.Dataflow(m)

		// If an instance of v can refer to multiple definitions, merge them.
		t := newUnionFind()
		for _, x := range m.list {
			for _, def := range m.out[x].list {
				t.Add(def)
			}
		}
		for _, x := range m.list {
			defs := m.out[x].list
			if len(defs) > 1 {
				for _, def := range defs[1:] {
					t.Merge(defs[0], def)
				}
			}
		}

		// Build list of candidate definitions.
		var defs []*Def
		nodedef := make(map[ast.Node]*Def)
		for _, list := range t.Sets() {
			x := list[0].(ast.Node)
			d := &Def{}
			defs = append(defs, d)
			nodedef[x] = d
		}

		// Build map from uses to candidate definitions.
		idToDef := make(map[ast.Node]*Def)
		for _, x := range m.list {
			if _, ok := x.(*ast.Ident); ok {
				if debug {
					fmt.Printf("ID:IN %s\n", m.nodeIn(x))
					fmt.Printf("ID:OUT %s\n", m.nodeOut(x))
				}
				defs := m.out[x].list
				if len(defs) > 0 {
					idToDef[x] = nodedef[t.Find(defs[0]).(ast.Node)]
				}
			}
		}

		// Compute start/end of where defn is needed,
		// along with block where defn must be placed.
		for _, x := range m.list {
			// Skip declaration without initializer.
			// We can move the zero initialization forward.
			switch x := x.(type) {
			case *ast.DeclStmt:
				if x.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values == nil {
					continue
				}
			}
			// Must use all entries in list.
			// Although most defs have been merged in previous passes,
			// the implicit zero definition of a var decl has not been.
			for _, def := range m.out[x].list {
				d := nodedef[t.Find(def).(ast.Node)]
				bx := blocks.Map[x]
				if debug {
					ddepth := -1
					if d.Block != nil {
						ddepth = d.Block.Depth
					}
					fmt.Printf("ID:X %s | d=%p %p b=%p bxdepth=%d ddepth=%d\n", m.nodeIn(x), d, d.Block, bx, bx.Depth, ddepth)
				}
				if d.Block == nil {
					d.Block = blocks.Map[x]
				} else {
					// Hoist into block containing both preliminary d.Block and x.
					for bx.Depth > d.Block.Depth {
						bx = bx.Parent
					}
					for d.Block.Depth > bx.Depth {
						d.Start = d.Block.Root.Pos()
						d.Block = d.Block.Parent
					}
					for d.Block != bx {
						d.Start = d.Block.Root.Pos()
						d.Block = d.Block.Parent
						bx = bx.Parent
					}
				}
				if pos := x.Pos(); d.Start == 0 || pos < d.Start {
					d.Start = pos
				}
				if end := x.End(); end > d.End {
					d.End = end
				}
				if debug {
					fmt.Printf("ID:X -> %s:%d,%d (%d,%d) ddepth=%d\n", pkg.FileSet.Position(d.Start).Filename, pkg.FileSet.Position(d.Start).Line, pkg.FileSet.Position(d.End).Line, d.Start, d.End, d.Block.Depth)
				}
			}
		}

		// Move tentative declaration sites up as required.
		for {
			changed := false

			for di, d := range defs {
				if d == nil {
					continue
				}
				orig := blocks.Map[vardecl[obj]].Depth
				if d.Block == nil {
					continue
				}
				// Cannot move declarations into loops.
				// Without liveness analysis we cannot be sure the variable is dead on entry.
				for b := d.Block; b.Depth > orig; b = b.Parent {
					switch b.Root.(type) {
					case *ast.ForStmt, *ast.RangeStmt:
						for d.Block != b {
							d.Start = d.Block.Root.Pos()
							d.End = d.Block.Root.End()
							d.Block = d.Block.Parent
							changed = true
						}
					}
				}

				// Gotos.
				for labelname, list := range blocks.Goto {
					label := blocks.Label[labelname]
					for _, g := range list {
						// Cannot declare between backward goto (possibly in nested block)
						// and target label in same or outer block; without liveness information,
						// we can't be sure the variable is dead at the label.
						if vardecl[obj].Pos() < label.Pos() && label.Pos() < d.Start && d.Start < g.Pos() {
							for label.Pos() < d.Block.Root.Pos() {
								d.Block = d.Block.Parent
							}
							d.Start = label.Pos()
							changed = true
						}

						// Cannot declare between forward goto (possibly in nested block)
						// and target label in same block; Go disallows jumping over declaration.
						if g.Pos() < d.Start && d.Start <= label.Pos() && blocks.Map[label] == d.Block {
							if false {
								fmt.Printf("%s:%d: goto %s blocks declaration of %s here\n", pkg.FileSet.Position(d.Start).Filename, pkg.FileSet.Position(d.Start).Line, labelname, obj.Name)
							}
							d.Start = g.Pos()
							changed = true
						}
					}
				}

				// If we've decided on an implicit if/for/switch block,
				// make sure we can actually put the declaration there.
				// If we can't initialize the variable with := in the initializer,
				// must move it up out of the loop.
				// Need to do this now, so that the move is visible to the
				// goto and "one definition per block" checks below.
				// TODO(rsc): This should be done for all variables simultaneously,
				// to allow
				//	for x, y := range z
				// instead of
				//	var x, y int
				//	var x, y = range z
				if !canDeclare(d.Block.Root, obj) {
					d.Start = d.Block.Root.Pos()
					d.Block = d.Block.Parent
					changed = true
				}

				// From a purely flow control point of view, in something like:
				//	var x int
				//	{
				//		x = 2
				//		y = x+x
				//		x = y
				//	}
				//	use(x)
				// The declaration 'x = 2' could be turned into a :=, since that value
				// is only used on the next line, except that the := will make the assignment
				// on the next line no longer refer to the outer x.
				// For each instance of x, if the proposed declaration shadows the
				// actual target of that x, merge the proposal into the outer declaration.
				for x, xDef := range idToDef {
					if xDef != d && xDef.Block.Depth < d.Block.Depth && d.Start <= x.Pos() && x.Pos() < d.Block.Root.End() {
						// xDef is an outer definition, so its start is already earlier than d's.
						// No need to update xDef.Start.
						// Not clear we care about xDef.End.
						// Update idToDef mappings to redirect d to xDef.
						for y, yDef := range idToDef {
							if yDef == d {
								idToDef[y] = xDef
							}
						}
						defs[di] = nil
						changed = true // because idToDef changed
					}
				}
			}

			// There can only be one definition (with a given name) per block.
			// Merge as needed.
			blockdef := make(map[*block.Block]*Def)
			for di, d := range defs {
				if d == nil {
					continue
				}
				dd := blockdef[d.Block]
				if dd == nil {
					blockdef[d.Block] = d
					continue
				}
				//fmt.Printf("merge defs %p %p\n", d, dd)
				if d.Start < dd.Start {
					dd.Start = d.Start
				}
				if d.End > dd.End {
					dd.End = d.End
				}
				for y, yDef := range idToDef {
					if yDef == d {
						idToDef[y] = dd
					}
				}
				defs[di] = nil
				changed = true
			}

			if changed {
				continue
			}

			// Find place to put declaration.
			// We established canDeclare(d.Block, obj) above.
			for _, d := range defs {
				if d == nil || d.Block == nil {
					continue
				}
				switch x := d.Block.Root.(type) {
				default:
					panic(fmt.Sprintf("unexpected declaration block root %T", d.Block.Root))

				case *ast.BlockStmt:
					d.Init = placeInit(edit, d.Start, obj, vardecl[obj], x.List)

				case *ast.CaseClause:
					d.Init = placeInit(edit, d.Start, obj, vardecl[obj], x.Body)

				case *ast.CommClause:
					d.Init = placeInit(edit, d.Start, obj, vardecl[obj], x.Body)

				case *ast.IfStmt:
					if x.Init == nil {
						panic("if without init")
					}
					d.Init = x.Init

				case *ast.ForStmt:
					if x.Init == nil {
						panic("for without init")
					}
					d.Init = x.Init

				case *ast.RangeStmt:
					d.Init = x

				case *ast.SwitchStmt:
					if x.Init == nil {
						panic("switch without init")
					}
					d.Init = x

				case *ast.TypeSwitchStmt:
					if x.Init == nil {
						panic("type switch without init")
					}
					d.Init = x
				}
				if d.Init != nil && d.Init.Pos() < d.Start {
					d.Start = d.Init.Pos()
					changed = true
				}
			}

			if !changed {
				break
			}
		}

		// Build report.
		v := &Var{Obj: obj, Decl: vardecl[obj]}
		for _, d := range defs {
			if d == nil || d.Block == nil {
				continue
			}
			if debug {
				fset := pkg.FileSet
				fmt.Printf("\tdepth %d: %s:%d,%d (%d,%d)\n", d.Block.Depth, fset.Position(d.Start).Filename, fset.Position(d.Start).Line, fset.Position(d.End).Line, d.Start, d.End)
				for _, x := range m.list {
					if len(m.out[x].list) > 0 {
						if d.Block == nodedef[t.Find(m.out[x].list[0]).(ast.Node)].Block {
							fmt.Printf("\t%s:%d %T (%d)\n", fset.Position(x.Pos()).Filename, fset.Position(x.Pos()).Line, x, x.Pos())
						}
					}
				}
			}
			v.Defs = append(v.Defs, d)
		}

		if len(v.Defs) == 1 && v.Defs[0].Init == vardecl[obj] {
			// No changes suggested.
			continue
		}

		vars = append(vars, v)
	}

	return vars
}

func unlabel(x ast.Stmt) ast.Stmt {
	for {
		y, ok := x.(*ast.LabeledStmt)
		if !ok {
			return x
		}
		x = y.Stmt
	}
}

func placeInit(edit *grinder.EditBuffer, start token.Pos, obj *ast.Object, decl *ast.DeclStmt, list []ast.Stmt) ast.Node {
	declPos := -1
	i := 0
	for i < len(list) && edit.End(list[i]) < start {
		if unlabel(list[i]) == decl {
			declPos = i
		}
		i++
	}
	if i >= len(list) {
		panic(fmt.Sprintf("unexpected start position"))
	}
	switch x := unlabel(list[i]).(type) {
	case *ast.AssignStmt:
		if canDeclare(x, obj) {
			return x
		}
	}

	if declPos >= 0 && allSimple(list[declPos:i]) {
		return decl
	}

	for j := i + 1; j < len(list); j++ {
		if unlabel(list[j]) == decl {
			if allSimple(list[i:j]) {
				return decl
			}
			break
		}
	}

	x := list[i]
	for {
		xx, ok := x.(*ast.LabeledStmt)
		if !ok || xx.Stmt.Pos() > start {
			break
		}
		x = xx.Stmt
	}

	return &ast.EmptyStmt{
		Semicolon: x.Pos(),
	}
}

func allSimple(list []ast.Stmt) bool {
	for _, x := range list {
		switch unlabel(x).(type) {
		case *ast.DeclStmt, *ast.AssignStmt, *ast.ExprStmt, *ast.EmptyStmt, *ast.IncDecStmt:
			// ok
		default:
			return false
		}
	}
	return true
}

func canDeclare(x ast.Node, obj *ast.Object) bool {
	switch x := x.(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true

	case *ast.IfStmt:
		if canDeclare(x.Init, obj) {
			return true
		}

	case *ast.SwitchStmt:
		if canDeclare(x.Init, obj) {
			return true
		}

	case *ast.TypeSwitchStmt:
		if canDeclare(x.Init, obj) {
			return true
		}

	case *ast.ForStmt:
		if canDeclare(x.Init, obj) {
			return true
		}

	case *ast.RangeStmt:
		return false
		// TODO: Can enable this but only with type information.
		// Need to make sure the obj type matches the range variable type.
		// There's nowhere to insert a conversion if not.
		if isIdentObj(x.Key, obj) && (x.Value == nil || isBlank(x.Value)) || isBlank(x.Key) && isIdentObj(x.Value, obj) {
			return true
		}

	case *ast.AssignStmt: // for recursive calls
		if len(x.Lhs) == 1 && x.Tok == token.ASSIGN && isIdentObj(x.Lhs[0], obj) {
			onRhs := false
			for _, y := range x.Rhs {
				ast.Inspect(y, func(z ast.Node) bool {
					if isIdentObj(z, obj) {
						onRhs = true
					}
					return !onRhs
				})
			}
			if onRhs {
				return false
			}
			return true
		}
	}
	return false
}

func isBlank(x ast.Node) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == "_"
}

func isIdentObj(x ast.Node, obj *ast.Object) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Obj == obj
}

func PrintVars(fset *token.FileSet, vars []*Var) []byte {
	var buf bytes.Buffer

	for _, v := range vars {
		pos := v.Decl.Pos()
		fmt.Fprintf(&buf, "var %s %s:%d\n", v.Obj.Name, fset.Position(pos).Filename, fset.Position(pos).Line)
		for _, d := range v.Defs {
			fmt.Fprintf(&buf, "\tdepth %d (%T): %s:%d,%d\n", d.Block.Depth, d.Block.Root, fset.Position(d.Start).Filename, fset.Position(d.Start).Line, fset.Position(d.End).Line)
			if d.Init != nil {
				x := d.Init
				fmt.Fprintf(&buf, "\t\tinit: %s:%d %T\n", fset.Position(x.Pos()).Filename, fset.Position(x.Pos()).Line, x)
			}
		}
	}

	return buf.Bytes()
}

type Var struct {
	Obj  *ast.Object
	Decl *ast.DeclStmt
	Defs []*Def
}

type Def struct {
	Block *block.Block
	Start token.Pos
	End   token.Pos
	Init  ast.Node
}

func commonBlock(x, y *block.Block) *block.Block {
	if x == nil {
		return y
	}
	for x.Depth > y.Depth {
		x = x.Parent
	}
	for y.Depth > x.Depth {
		y = y.Parent
	}
	for x != y {
		x = x.Parent
		y = y.Parent
	}
	return x
}

// Because we don't have a proper liveness analysis, we don't know
// which variables can be moved inside loop bodies and which cannot.
// (We might also want escape analysis to avoid causing allocations.)
// Move up to the outermost ForStmt if present.
// (The ForStmt itself is not part of the looping control flow.)
func outsideLoop(x *block.Block) *block.Block {
	for y := x.Parent; y != nil; y = y.Parent {
		switch y.Root.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			x = y
		}
	}
	return x
}

func needForObj(pkg *grinder.Package, obj *ast.Object, x ast.Node) (need bool) {
	switch x := x.(type) {
	case *ast.Ident:
		if x.Obj == obj {
			return true
		}

	case *ast.UnaryExpr:
		if x.Op == token.AND {
			y := x.X
			for {
				switch yy := y.(type) {
				case *ast.ParenExpr:
					y = yy.X
					continue
				case *ast.SelectorExpr:
					// If yy.X is a pointer, stop.
					t := pkg.Info.Types[yy.X].Type
					if t != nil {
						t = t.Underlying()
						if t == nil {
							panic("underlying nil")
						}
						_, ok := t.(*types.Pointer)
						if ok {
							break
						}
					}
					y = yy.X
					continue
				case *ast.IndexExpr:
					// If yy.X is a pointer or slice, stop.
					t := pkg.Info.Types[yy.X].Type
					if t != nil {
						t = t.Underlying()
						if t == nil {
							panic("underlying nil")
						}
						_, ok := t.(*types.Pointer)
						if ok {
							break
						}
						_, ok = t.(*types.Slice)
						if ok {
							break
						}
					}
					y = yy.X
					continue
				}
				break
			}

			switch y := y.(type) {
			case *ast.Ident:
				if y.Obj == obj {
					return true
				}
			}
		}

	case *ast.DeclStmt:
		g := x.Decl.(*ast.GenDecl)
		if g.Tok != token.VAR {
			break
		}
		for _, spec := range g.Specs {
			vs := spec.(*ast.ValueSpec)
			for _, id := range vs.Names {
				if id.Obj == obj {
					return true
				}
			}
		}

	case *ast.AssignStmt:
		if x.Tok != token.ASSIGN {
			break
		}
		for _, y := range x.Lhs {
			y = unparen(y)
			switch y := y.(type) {
			case *ast.Ident:
				if y.Obj == obj {
					return true
				}
			}
		}

	case *ast.IncDecStmt:
		y := unparen(x.X)
		switch y := y.(type) {
		case *ast.Ident:
			if y.Obj == obj {
				return true
			}
		}
	}
	return false
}

func unparen(x ast.Expr) ast.Expr {
	for {
		p, ok := x.(*ast.ParenExpr)
		if !ok {
			return x
		}
		x = p.X
	}
}

type defSet struct {
	list      []ast.Node
	addrTaken bool
}

type identMatcher struct {
	in   map[ast.Node]defSet
	out  map[ast.Node]defSet
	list []ast.Node
	fset *token.FileSet
	g    *flow.Graph
	obj  *ast.Object
	pkg  *grinder.Package
}

func newIdentMatcher(pkg *grinder.Package, g *flow.Graph, obj *ast.Object) *identMatcher {
	return &identMatcher{
		in:   make(map[ast.Node]defSet),
		out:  make(map[ast.Node]defSet),
		pkg:  pkg,
		fset: pkg.FileSet,
		g:    g,
		obj:  obj,
	}
}

func (m *identMatcher) nodeIn(x ast.Node) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s:", nodeLabel(m.fset, x))
	for _, y := range m.in[x].list {
		fmt.Fprintf(&buf, " %s", nodeLabel(m.fset, y))
	}
	return buf.String()
}

func (m *identMatcher) nodeOut(x ast.Node) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s:", nodeLabel(m.fset, x))
	for _, y := range m.out[x].list {
		fmt.Fprintf(&buf, " %s", nodeLabel(m.fset, y))
	}
	return buf.String()
}

func (m *identMatcher) Init(x ast.Node) {
	m.in[x] = defSet{}
}

func (m *identMatcher) Transfer(x ast.Node) {
	if !needForObj(m.pkg, m.obj, x) {
		m.out[x] = m.in[x]
		return
	}

	_, ok := m.out[x]
	if !ok && x != m.g.Start && x != m.g.End {
		m.list = append(m.list, x)
	}

	switch x := x.(type) {
	case *ast.Ident:
		if len(m.in[x].list) == 1 && isDecl(m.in[x].list[0]) { // first use
			m.out[x] = defSet{[]ast.Node{x}, false}
			return
		}

	case *ast.DeclStmt:
		m.out[x] = defSet{[]ast.Node{x}, false}
		return

	case *ast.UnaryExpr:
		m.out[x] = defSet{m.in[x].list, true}
		return

	case *ast.AssignStmt:
		if m.in[x].addrTaken {
			m.out[x] = defSet{mergef(m.in[x].list, []ast.Node{x}), true}
		} else {
			m.out[x] = defSet{[]ast.Node{x}, false}
		}
		return

	case *ast.IncDecStmt:
		m.out[x] = defSet{mergef(m.in[x].list, []ast.Node{x}), m.in[x].addrTaken}
		return
	}
	m.out[x] = m.in[x]
}

func isDecl(x ast.Node) bool {
	_, ok := x.(*ast.DeclStmt)
	return ok
}

func (m *identMatcher) Join(x, y ast.Node) bool {
	dx := m.in[x]
	dy := m.out[y]
	new := mergef(dx.list, dy.list)
	if len(new) > len(dx.list) || !dx.addrTaken && dy.addrTaken {
		m.in[x] = defSet{new, dx.addrTaken || dy.addrTaken}
		return true
	}
	return false
}

func nodeLabel(fset *token.FileSet, x ast.Node) string {
	pos := fset.Position(x.Pos())
	label := fmt.Sprintf("%T %s:%d", x, pos.Filename, pos.Line)
	switch x := x.(type) {
	case *ast.Ident:
		label = x.Name + " " + label
	case *ast.SelectorExpr:
		label = "." + x.Sel.Name + " " + label
	}
	return label
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

type unionFind struct {
	parent map[interface{}]interface{}
	rank   map[interface{}]int
	all    []interface{}
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[interface{}]interface{}),
		rank:   make(map[interface{}]int),
	}
}

func (u *unionFind) Add(x interface{}) {
	_, ok := u.parent[x]
	if ok {
		return
	}
	u.parent[x] = x
	u.rank[x] = 0
	u.all = append(u.all, x)
}

func (u *unionFind) Merge(x, y interface{}) {
	xRoot := u.Find(x)
	yRoot := u.Find(y)
	if xRoot == yRoot {
		return
	}

	if u.rank[xRoot] < u.rank[yRoot] {
		u.parent[xRoot] = yRoot
	} else if u.rank[xRoot] > u.rank[yRoot] {
		u.parent[yRoot] = xRoot
	} else {
		u.parent[yRoot] = xRoot
		u.rank[xRoot]++
	}
}

func (u *unionFind) Find(x interface{}) interface{} {
	if u.parent[x] != x {
		u.parent[x] = u.Find(u.parent[x])
	}
	return u.parent[x]
}

func (u *unionFind) Sets() [][]interface{} {
	var out [][]interface{}
	m := make(map[interface{}][]interface{})
	for _, x := range u.all {
		u.Find(x)
	}
	for _, x := range u.all {
		root := u.Find(x)
		list := m[root]
		if list == nil {
			list = append(list, root)
		}
		if root != x {
			list = append(list, x)
		}
		m[root] = list
	}
	for _, x := range u.all {
		if u.Find(x) == x {
			out = append(out, m[x])
		}
	}
	return out
}

func initToDecl(ctxt *grinder.Context, pkg *grinder.Package, edit *grinder.EditBuffer, fn *ast.FuncDecl) {
	// Rewrite x := T{} (for struct or array type T) and x := (*T)(nil) to var x T.
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		list := grinder.BlockList(x)
		for _, stmt := range list {
			as, ok := stmt.(*ast.AssignStmt)
			if !ok || len(as.Lhs) > 1 || as.Tok != token.DEFINE {
				continue
			}
			var typ string
			if t, ok := isNilPtr(pkg, edit, as.Rhs[0]); ok {
				typ = t
			} else if t, ok := isStructOrArrayLiteral(pkg, edit, as.Rhs[0]); ok {
				typ = t
			}
			if typ != "" {
				edit.Replace(stmt.Pos(), stmt.End(), "var "+as.Lhs[0].(*ast.Ident).Name+" "+typ)
			}
		}
		return true
	})
}

func isNilPtr(pkg *grinder.Package, edit *grinder.EditBuffer, x ast.Expr) (typ string, ok bool) {
	conv, ok := x.(*ast.CallExpr)
	if !ok || len(conv.Args) != 1 {
		return "", false
	}
	id, ok := unparen(conv.Args[0]).(*ast.Ident)
	if !ok || id.Name != "nil" {
		return "", false
	}
	if obj := pkg.Info.Uses[id]; obj == nil || obj.Pkg() != nil {
		return "", false
	}
	fn := unparen(conv.Fun)
	tv, ok := pkg.Info.Types[fn]
	if !ok || !tv.IsType() {
		return "", false
	}
	return edit.TextAt(fn.Pos(), fn.End()), true
}

func isStructOrArrayLiteral(pkg *grinder.Package, edit *grinder.EditBuffer, x ast.Expr) (typ string, ok bool) {
	lit, ok := x.(*ast.CompositeLit)
	if !ok || len(lit.Elts) > 0 {
		return "", false
	}
	tv, ok := pkg.Info.Types[x]
	if !ok {
		return "", false
	}
	t := tv.Type
	if name, ok := t.(*types.Named); ok {
		t = name.Underlying()
	}
	switch t.(type) {
	default:
		return "", false
	case *types.Struct, *types.Array:
		// ok
	}
	return edit.TextAt(lit.Type.Pos(), lit.Type.End()), true
}
