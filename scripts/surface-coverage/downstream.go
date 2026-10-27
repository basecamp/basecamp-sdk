package main

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Repo is a downstream Go module's references to the SDK.
type Repo struct {
	Dir       string
	Module    string
	Patterns  []string
	PinnedSDK string // the SDK version the module's go.mod resolves
	Packages  []string
	Refs      []siteRef
	// TypeErrors counts type errors in the module's own packages. Each one
	// may hide call sites, so the report says so when it is not zero.
	TypeErrors int
	// Missing are type errors naming an SDK symbol that does not exist — a
	// call into an SDK the code was not written against.
	Missing []site
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
		if p.Module == nil || !p.Module.Main {
			continue
		}
		c, err := l.check(p.ImportPath)
		if err != nil {
			return nil, err
		}
		r.Packages = append(r.Packages, p.ImportPath)
		r.TypeErrors += len(c.Errors)
		for _, e := range c.Errors {
			if te, ok := e.(types.Error); ok {
				if missingSDKSymbol(te.Msg) {
					pos := l.fset.Position(te.Pos)
					r.Missing = append(r.Missing, site{File: rel(dir, pos.Filename), Line: pos.Line, Text: te.Msg})
				}
			}
		}
		checked = append(checked, c)
	}

	bindings := ifaceBindings{}
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
			for _, m := range bindings.expand(fn) {
				key := funcKey(m)
				// A raw HTTP method behind an interface: its path is the
				// interface caller's, so it is unreadable here.
				r.add(l, ref{Key: key, Pos: id.Pos(), Recv: recvName(m), Exported: m.Exported(), Raw: rawMethods[key]})
			}
		}
	}
	if len(r.Packages) == 0 {
		return nil, fmt.Errorf("no packages of %s matched %s", mod, strings.Join(patterns, " "))
	}
	sort.Slice(r.Refs, func(i, j int) bool {
		a, b := r.Refs[i].Site, r.Refs[j].Site
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return r, nil
}

func (r *Repo) add(l *loader, rf ref) {
	pos := l.fset.Position(rf.Pos)
	r.Refs = append(r.Refs, siteRef{ref: rf, Site: site{File: rel(r.Dir, pos.Filename), Line: pos.Line}})
}

var missingRE = regexp.MustCompile(`has no field or method|undefined: basecamp\.|undefined: generated\.`)

func missingSDKSymbol(msg string) bool {
	return missingRE.MatchString(msg) && (strings.Contains(msg, "basecamp.") || strings.Contains(msg, "generated."))
}

func rel(base, path string) string {
	if r, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}
