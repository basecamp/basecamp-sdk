package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
)

// checkedPackage is one package type-checked from source.
type checkedPackage struct {
	Path   string
	Dir    string
	Files  []*ast.File
	Pkg    *types.Package
	Info   *types.Info
	Errors []error
}

// loader type-checks a chosen set of packages from source and imports
// everything else from the export data `go list -export` produced.
//
// Source-checked packages import each other as checked from source, not from
// export data, so a package that fails to compile still yields the types its
// importers need: one broken package does not blind the analysis to call
// sites in every package that imports it.
type loader struct {
	fset    *token.FileSet
	listed  map[string]*listedPackage
	checked map[string]*checkedPackage
	gc      types.ImporterFrom
}

func newLoader(pkgs []*listedPackage) *loader {
	l := &loader{
		fset:    token.NewFileSet(),
		listed:  map[string]*listedPackage{},
		checked: map[string]*checkedPackage{},
	}
	for _, p := range pkgs {
		l.listed[p.ImportPath] = p
	}
	l.gc = importer.ForCompiler(l.fset, "gc", func(path string) (io.ReadCloser, error) {
		p := l.listed[path]
		if p == nil || p.Export == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(p.Export)
	}).(types.ImporterFrom)
	return l
}

// pkgImporter resolves one package's imports through its ImportMap (vendored
// and replaced paths) to either a source-checked package or export data.
type pkgImporter struct {
	l    *loader
	from *listedPackage
}

func (i pkgImporter) Import(path string) (*types.Package, error) {
	return i.ImportFrom(path, "", 0)
}

func (i pkgImporter) ImportFrom(path, dir string, mode types.ImportMode) (*types.Package, error) {
	if mapped, ok := i.from.ImportMap[path]; ok {
		path = mapped
	}
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if c := i.l.checked[path]; c != nil {
		return c.Pkg, nil
	}
	return i.l.gc.ImportFrom(path, dir, mode)
}

// check type-checks the listed package at path from its source files. Call
// it in dependency order so a source-checked import is ready when needed.
func (l *loader) check(path string) (*checkedPackage, error) {
	lp := l.listed[path]
	if lp == nil {
		return nil, fmt.Errorf("package %s was not listed", path)
	}
	files := lp.CompiledGoFiles
	if len(files) == 0 {
		files = lp.GoFiles
	}
	c := &checkedPackage{Path: path, Dir: lp.Dir}
	// Every compiled file is Go source, though cgo's output sits in the build
	// cache under names without a .go extension.
	for _, name := range files {
		if !filepath.IsAbs(name) {
			name = filepath.Join(lp.Dir, name)
		}
		f, err := parser.ParseFile(l.fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			c.Errors = append(c.Errors, err)
			if f == nil {
				continue
			}
		}
		c.Files = append(c.Files, f)
	}
	c.Info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{
		Importer: pkgImporter{l: l, from: lp},
		Error:    func(err error) { c.Errors = append(c.Errors, err) },
	}
	// Errors are collected through conf.Error; the package is usable even
	// when they are not empty.
	c.Pkg, _ = conf.Check(path, l.fset, c.Files, c.Info)
	l.checked[path] = c
	return c, nil
}
