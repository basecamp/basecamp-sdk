package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Repo is a downstream Go module's references to the SDK.
type Repo struct {
	Dir       string
	Module    string
	Patterns  []string
	PinnedSDK string // the SDK version the module's go.mod resolves; "" when a go.work supplies it
	Packages  []string
	Refs      []siteRef
	// TypeErrors counts type errors in the module's own packages. Each one
	// may hide call sites, so the report says so when it is not zero.
	TypeErrors int
	// Missing are type errors naming an SDK symbol that does not exist — a
	// call into an SDK the code was not written against.
	Missing []site
	// Ambiguous are calls through an interface of the repository's that
	// more than one SDK method could answer, credited to none of them.
	Ambiguous []site
}

type siteRef struct {
	ref
	Site site
}

type site struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text,omitempty"`
}

// defaultPatterns are the packages that make up each known downstream's
// shipped binary, so test harnesses and developer tools under cmd/ do not
// count as coverage. Any other module defaults to every package in it.
var defaultPatterns = map[string][]string{
	"github.com/basecamp/basecamp-cli":        {"./cmd/basecamp"},
	"github.com/basecamp/basecamp-mcp-server": {"./cmd/basecamp-mcp"},
}

func modulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod declares no module", dir)
}

// loadRepo type-checks the module's own packages among patterns (and their
// in-module imports) and collects every reference they make to the SDK.
// Test files are not loaded: coverage is what the shipped code reaches.
func loadRepo(dir string, patterns []string) (*Repo, error) {
	dir, err := filepath.Abs(dir)
	if err == nil {
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		return nil, err
	}
	mod, err := modulePath(dir)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		patterns = defaultPatterns[mod]
	}
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	r := &Repo{Dir: dir, Module: mod, Patterns: patterns}

	// A repository without a go.work of its own is loaded without one, so a
	// workspace in a directory above it (as for this tool's own fixtures)
	// does not capture it.
	var env []string
	if _, err := os.Stat(filepath.Join(dir, "go.work")); err != nil {
		env = []string{"GOWORK=off"}
	}
	listed, err := goList(dir, env, patterns...)
	if err != nil {
		return nil, err
	}
	l := newLoader(listed)
	var checked []*checkedPackage
	for _, p := range listed {
		if p.Module != nil && p.Module.Path == sdkModulePath && r.PinnedSDK == "" {
			r.PinnedSDK = p.Module.Version
		}
		// Only the repository's own module is checked from source and
		// analyzed; everything else, including the other modules of a go.work
		// (Module.Main is true for each of them), comes from export data. A
		// workspace that uses a local SDK checkout makes the SDK a main module
		// too, and its wrappers are not the repository's coverage.
		if p.Module == nil || p.Module.Path != mod {
			continue
		}
		c, err := l.check(p.ImportPath)
		if err != nil {
			return nil, err
		}
		r.Packages = append(r.Packages, p.ImportPath)
		r.TypeErrors += len(c.Errors)
		r.Missing = append(r.Missing, missingSDKSymbols(c, l.fset, dir)...)
		checked = append(checked, c)
	}

	bindings := newIfaceBindings(mod)
	for _, c := range checked {
		for _, f := range c.Files {
			bindings.collect(f, c.Info)
		}
	}
	for _, c := range checked {
		for _, f := range c.Files {
			for _, rf := range collectRefs(f, c.Info) {
				r.add(l, rf)
			}
		}
		for id, obj := range c.Info.Uses {
			fn, ok := obj.(*types.Func)
			if !ok || fn.Pkg() == nil || inSDK(fn.Pkg().Path()) {
				continue
			}
			m, ambiguous := bindings.expand(fn)
			if ambiguous {
				pos := l.fset.Position(id.Pos())
				r.Ambiguous = append(r.Ambiguous, site{File: rel(r.Dir, pos.Filename), Line: pos.Line, Text: fn.FullName()})
				continue
			}
			if m == nil {
				continue
			}
			key := funcKey(m)
			rf := ref{Key: key, Pos: id.Pos(), Recv: recvName(m), Exported: m.Exported()}
			switch {
			case rawMethods[key] != "":
				// A raw HTTP method behind an interface: its path is the
				// interface caller's, so it is unreadable here.
				rf.Raw = rawMethods[key]
			case m.Pkg().Path() == generatedPkgPath:
				rf.Generated = m.Name()
			}
			r.add(l, rf)
		}
	}
	if len(r.Packages) == 0 {
		return nil, fmt.Errorf("no packages of %s matched %s", mod, strings.Join(patterns, " "))
	}
	sortSites(r.Ambiguous)
	sort.Slice(r.Refs, func(i, j int) bool {
		a, b := r.Refs[i].Site, r.Refs[j].Site
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return r, nil
}

func sortSites(ss []site) {
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].File != ss[j].File {
			return ss[i].File < ss[j].File
		}
		return ss[i].Line < ss[j].Line
	})
}

func (r *Repo) add(l *loader, rf ref) {
	pos := l.fset.Position(rf.Pos)
	r.Refs = append(r.Refs, siteRef{ref: rf, Site: site{File: rel(r.Dir, pos.Filename), Line: pos.Line}})
}

// missingSDKSymbols are the type errors at a selector into the SDK that
// resolves to nothing: a package-level name the SDK package lacks, however
// the import is named, or a method or field an SDK type lacks.
func missingSDKSymbols(c *checkedPackage, fset *token.FileSet, dir string) []site {
	if len(c.Errors) == 0 {
		return nil
	}
	sels := map[token.Pos]*ast.SelectorExpr{}
	for _, f := range c.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				sels[sel.Sel.Pos()] = sel
			}
			return true
		})
	}
	var out []site
	for _, e := range c.Errors {
		te, ok := e.(types.Error)
		if !ok {
			continue
		}
		sel := sels[te.Pos]
		if sel == nil || c.Info.Uses[sel.Sel] != nil {
			continue
		}
		into := false
		if id, ok := sel.X.(*ast.Ident); ok {
			if pn, ok := c.Info.Uses[id].(*types.PkgName); ok {
				into = inSDK(pn.Imported().Path())
			}
		}
		if tv, ok := c.Info.Types[sel.X]; ok && tv.Type != nil && sdkType(tv.Type) {
			into = true
		}
		if into {
			pos := fset.Position(te.Pos)
			out = append(out, site{File: rel(dir, pos.Filename), Line: pos.Line, Text: te.Msg})
		}
	}
	return out
}

func rel(base, path string) string {
	if r, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}
