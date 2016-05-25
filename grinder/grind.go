// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package grinder defines the API for individual grinding bits.
package grinder

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"

	_ "golang.org/x/tools/go/gcimporter15"
	"golang.org/x/tools/go/types"
)

type Package struct {
	ImportPath string
	Files      []*ast.File
	Filenames  []string
	FileSet    *token.FileSet
	Types      *types.Package
	TypesError error
	Info       types.Info

	clean  bool
	oldSrc map[string]string
	newSrc map[string]string
}

func (p *Package) Src(name string) string {
	if content := p.newSrc[name]; content != "" {
		return content
	}
	return p.oldSrc[name]
}

func (p *Package) OrigSrc(name string) string {
	return p.oldSrc[name]
}

func (p *Package) Modified(name string) bool {
	_, ok := p.newSrc[name]
	return ok
}

func (p *Package) Rewrite(name, content string) {
	gofmt, err := format.Source([]byte(content))
	if err != nil {
		panic("rewrite " + name + " with bad source: " + err.Error() + "\n" + content)
	}
	// Cut blank lines at top of blocks declarations.
	gofmt = bytes.Replace(gofmt, []byte("{\n\n"), []byte("{\n"), -1)
	gofmt = bytes.Replace(gofmt, []byte("\n\n}"), []byte("\n}"), -1)
	p.newSrc[name] = string(gofmt)
	p.clean = false
}

type Func func(*Context, *Package)

type Context struct {
	Logf     func(format string, args ...interface{})
	Errors   bool
	Grinders []Func
}

func (ctxt *Context) Errorf(format string, args ...interface{}) {
	ctxt.Logf(format, args...)
	ctxt.Errors = true
}

func (ctxt *Context) GrindFiles(files ...string) *Package {
	pkg := &Package{
		ImportPath: ".",
		Filenames:  files,
		oldSrc:     make(map[string]string),
		newSrc:     make(map[string]string),
	}

	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			ctxt.Errorf("%v", err)
			return nil
		}
		src := string(data)
		pkg.oldSrc[file] = src
	}

	ctxt.grind(pkg)
	return pkg
}

func (ctxt *Context) GrindPackage(path string) *Package {
	buildCtxt := build.Default

	buildPkg, err := buildCtxt.Import(path, ".", 0)
	if err != nil {
		ctxt.Errorf("%v", err)
		return nil
	}
	if len(buildPkg.CgoFiles) > 0 {
		ctxt.Errorf("%s: packages using cgo not supported", path)
		return nil
	}

	pkg := &Package{
		ImportPath: path,
		oldSrc:     make(map[string]string),
		newSrc:     make(map[string]string),
	}

	for _, name := range buildPkg.GoFiles {
		filename := filepath.Join(buildPkg.Dir, name)
		pkg.Filenames = append(pkg.Filenames, filename)
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			ctxt.Errorf("%s: %v", path, err)
			return nil
		}
		src := string(data)
		pkg.oldSrc[filename] = src
	}

	ctxt.grind(pkg)
	return pkg
}

func (ctxt *Context) grind(pkg *Package) {
Loop:
	for loop := 0; ; loop++ {
		println(loop)
		pkg.FileSet = token.NewFileSet()

		pkg.Files = nil
		for _, name := range pkg.Filenames {
			f, err := parser.ParseFile(pkg.FileSet, name, pkg.Src(name), 0)
			if err != nil {
				if loop > 0 {
					ctxt.Errorf("%s: error parsing rewritten file: %v", pkg.ImportPath, err)
					return
				}
				ctxt.Errorf("%s: %v", pkg.ImportPath, err)
				return
			}
			pkg.Files = append(pkg.Files, f)
		}

		conf := new(types.Config)
		// conf.DisableUnusedImportCheck = true
		pkg.Info = types.Info{}
		pkg.Info.Types = make(map[ast.Expr]types.TypeAndValue)
		pkg.Info.Scopes = make(map[ast.Node]*types.Scope)
		pkg.Info.Defs = make(map[*ast.Ident]types.Object)
		pkg.Info.Uses = make(map[*ast.Ident]types.Object)
		typesPkg, err := conf.Check(pkg.ImportPath, pkg.FileSet, pkg.Files, &pkg.Info)
		if err != nil && typesPkg == nil {
			if loop > 0 {
				ctxt.Errorf("%s: error type checking rewritten package: %v", pkg.ImportPath, err)
				for _, name := range pkg.Filenames {
					if pkg.Modified(name) {
						ctxt.Errorf("%s <<<\n%s\n>>>", name, pkg.Src(name))
					}
				}
				return
			}
			ctxt.Errorf("%s: %v", pkg.ImportPath, err)
			return
		}
		pkg.Types = typesPkg
		pkg.TypesError = err

		for _, g := range ctxt.Grinders {
			pkg.clean = true
			g(ctxt, pkg)
			if !pkg.clean {
				continue Loop
			}
		}
		break
	}
}

func GrindFuncDecls(ctxt *Context, pkg *Package, fn func(ctxt *Context, pkg *Package, edit *EditBuffer, decl *ast.FuncDecl)) {
	for i, filename := range pkg.Filenames {
		file := pkg.Files[i]
		if strings.Contains(pkg.Src(filename), "\n//line ") {
			// Don't bother cleaning generated code.
			continue
		}
		edit := NewEditBuffer(pkg, filename, file)
		for _, decl := range file.Decls {
			decl, ok := decl.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			fn(ctxt, pkg, edit, decl)
		}
		if edit.NumEdits() > 0 {
			old := pkg.Src(filename)
			new := edit.Apply()
			if old != new {
				// TODO(rsc): It should not happen that old != new,
				// but sometimes we delete a var declaration only to
				// put it right back where we started.
				// Hopefully there are no cycles. Ugh.
				fmt.Printf("EDIT: %s\n%s\n", filename, Diff(old, new))
				pkg.Rewrite(filename, new)
			}
		}
	}
}
