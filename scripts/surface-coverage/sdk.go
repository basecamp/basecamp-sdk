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

const (
	sdkModulePath    = "github.com/basecamp/basecamp-sdk/go"
	sdkPkgPath       = sdkModulePath + "/pkg/basecamp"
	generatedPkgPath = sdkModulePath + "/pkg/generated"
)

// rawMethods are the SDK's untyped HTTP calls, which name an operation only
// through the path they are given, and the HTTP method each one sends.
var rawMethods = map[string]string{}

func init() {
	for _, recv := range []string{"Client", "AccountClient"} {
		for name, method := range map[string]string{
			"Get": "GET", "GetAll": "GET", "GetAllWithLimit": "GET",
			"Post": "POST", "Put": "PUT", "Delete": "DELETE",
		} {
			rawMethods[fmt.Sprintf("(*%s.%s).%s", sdkPkgPath, recv, name)] = method
		}
	}
}

// SDK is one version of the Go SDK, reduced to what coverage needs: its
// operations, and the operations each function or method it declares
// performs.
type SDK struct {
	Version    string
	Dir        string
	Operations []Operation
	ops        map[string]Operation
	routes     *routeTable

	// declared holds every function and method the SDK's packages declare,
	// by types.Func.FullName — the key a downstream reference is looked up by.
	declared map[string]bool
	// reach maps a declared function or method to the operations it performs,
	// directly or through the other SDK functions it calls.
	reach map[string][]string
}

// loadSDK reads the SDK Go module rooted at dir.
func loadSDK(dir, version string) (*SDK, error) {
	ops, err := readCatalog(dir)
	if err != nil {
		return nil, err
	}
	s := &SDK{
		Version:    version,
		Dir:        dir,
		Operations: ops,
		ops:        map[string]Operation{},
		routes:     newRouteTable(ops),
		declared:   map[string]bool{},
		reach:      map[string][]string{},
	}
	for _, op := range ops {
		s.ops[op.ID] = op
	}

	listed, err := goList(dir, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, "./pkg/...")
	if err != nil {
		return nil, err
	}
	// Every package of the SDK module is checked from source, the generated
	// client included: a package imported from export data would bring its
	// own copy of any SDK package it imports, and the two copies' types
	// would not be identical.
	l := newLoader(listed)
	var checked []*checkedPackage
	for _, p := range listed {
		if p.Module == nil || p.Module.Path != sdkModulePath {
			continue
		}
		c, err := l.check(p.ImportPath)
		if err != nil {
			return nil, err
		}
		if len(c.Errors) > 0 {
			return nil, fmt.Errorf("SDK %s does not type-check: %v", version, c.Errors[0])
		}
		declare(c.Pkg, s.declared)
		if p.ImportPath != generatedPkgPath {
			checked = append(checked, c)
		}
	}

	direct := map[string]map[string]bool{}
	edges := map[string]map[string]bool{}
	for _, c := range checked {
		for _, f := range c.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, ok := c.Info.Defs[fd.Name].(*types.Func)
				if !ok {
					continue
				}
				key := funcKey(fn)
				own := recvName(fn)
				direct[key] = map[string]bool{}
				edges[key] = map[string]bool{}
				for _, r := range collectRefs(fd.Body, c.Info) {
					switch {
					case r.Generated != "":
						if op, ok := s.generatedOp(r.Generated); ok {
							direct[key][op] = true
						}
					case r.Raw != "":
						if op, ok := s.routes.match(r.Raw, r.Pattern); ok && r.Resolved {
							direct[key][op] = true
						}
					case r.Exported && r.Recv != own && strings.HasSuffix(r.Recv, "Service"):
						// A composite: this function performs another
						// service's public operation on the caller's behalf
						// (RecordingsService.Summarize reads through
						// TodosService.Get and two dozen others). That
						// operation is that service's to cover; reaching it
						// through a composite is not a way to perform it.
					default:
						edges[key][r.Key] = true
					}
				}
			}
		}
	}
	for key := range direct {
		set := map[string]bool{}
		closure(key, direct, edges, set, map[string]bool{})
		s.reach[key] = sortedKeys(set)
	}
	return s, nil
}

func closure(key string, direct, edges map[string]map[string]bool, ops, seen map[string]bool) {
	if seen[key] {
		return
	}
	seen[key] = true
	for op := range direct[key] {
		ops[op] = true
	}
	for callee := range edges[key] {
		closure(callee, direct, edges, ops, seen)
	}
}

// declare records every function and method pkg declares.
func declare(pkg *types.Package, into map[string]bool) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			into[funcKey(obj)] = true
		case *types.TypeName:
			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				into[funcKey(named.Method(i))] = true
			}
			if iface, ok := named.Underlying().(*types.Interface); ok {
				for i := 0; i < iface.NumMethods(); i++ {
					into[funcKey(iface.Method(i))] = true
				}
			}
		}
	}
}

// generatedOp names the operation a generated-client function performs.
// oapi-codegen spells every operation's entry points from its operationId:
// Op, OpWithBody, OpWithResponse, OpWithBodyWithResponse, and the request
// builders NewOpRequest and NewOpRequestWithBody.
func (s *SDK) generatedOp(name string) (string, bool) {
	core := strings.TrimSuffix(name, "WithResponse")
	core = strings.TrimSuffix(core, "WithBody")
	if _, ok := s.ops[core]; ok {
		return core, true
	}
	if strings.HasPrefix(name, "New") {
		core = strings.TrimPrefix(name, "New")
		core = strings.TrimSuffix(core, "RequestWithBody")
		core = strings.TrimSuffix(core, "Request")
		if _, ok := s.ops[core]; ok {
			return core, true
		}
	}
	return "", false
}

// opsFor returns the operations a reference to an SDK symbol performs, and
// whether this SDK version declares the symbol at all.
func (s *SDK) opsFor(r ref) ([]string, bool) {
	// The symbol must exist in this version before its name or path says
	// anything: a generated entry point renamed away, or a raw helper
	// removed, is a stale reference even while its operation lives on.
	if !s.declared[r.Key] {
		return nil, false
	}
	switch {
	case r.Generated != "":
		if op, ok := s.generatedOp(r.Generated); ok {
			return []string{op}, true
		}
		return nil, true
	case r.Raw != "":
		if !r.Resolved {
			return nil, true
		}
		if op, ok := s.routes.match(r.Raw, r.Pattern); ok {
			return []string{op}, true
		}
		return nil, true
	}
	return s.reach[r.Key], s.declared[r.Key]
}

// ref is one reference from code to an SDK function or method.
type ref struct {
	Key string // types.Func.FullName of the referenced SDK symbol
	Pos token.Pos

	// Generated is set for a reference into the generated client: the
	// function or method name, which spells its operation.
	Generated string
	// Raw is set for a call to an untyped HTTP method: the HTTP method it
	// sends. Pattern is its path (see pathPattern) when Resolved.
	Raw      string
	Pattern  string
	Resolved bool

	// Recv is the referenced method's receiver type name ("" for a
	// function), and Exported whether the symbol is exported.
	Recv     string
	Exported bool
}

// collectRefs finds every reference to an SDK function or method under node.
// A reference need not be a call — a method value passed along still
// reaches the operation — so it reads identifier uses, and reads call
// expressions only to recover the path given to a raw HTTP call.
func collectRefs(node ast.Node, info *types.Info) []ref {
	var refs []ref
	rawAt := map[token.Pos]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fn, ok := info.Uses[sel.Sel].(*types.Func)
			if !ok {
				return true
			}
			key := funcKey(fn)
			method, isRaw := rawMethods[key]
			if !isRaw {
				return true
			}
			r := ref{Key: key, Pos: sel.Sel.Pos(), Raw: method}
			if len(x.Args) >= 2 {
				r.Pattern, r.Resolved = pathPattern(x.Args[1], info)
			}
			refs = append(refs, r)
			rawAt[sel.Sel.Pos()] = true
		case *ast.Ident:
			fn, ok := info.Uses[x].(*types.Func)
			if !ok || fn.Pkg() == nil || !inSDK(fn.Pkg().Path()) || rawAt[x.Pos()] {
				return true
			}
			key := funcKey(fn)
			if _, isRaw := rawMethods[key]; isRaw {
				// A raw method referenced but not called here (a method
				// value): its path is not visible.
				refs = append(refs, ref{Key: key, Pos: x.Pos(), Raw: rawMethods[key]})
				return true
			}
			r := ref{Key: key, Pos: x.Pos(), Recv: recvName(fn), Exported: fn.Exported()}
			if fn.Pkg().Path() == generatedPkgPath {
				r.Generated = fn.Name()
			}
			refs = append(refs, r)
		}
		return true
	})
	return refs
}

// recvName is the name of fn's receiver type, or "" for a function.
func recvName(fn *types.Func) string {
	recv := fn.Origin().Signature().Recv()
	if recv == nil {
		return ""
	}
	t := recv.Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

func inSDK(path string) bool {
	return path == sdkModulePath || strings.HasPrefix(path, sdkModulePath+"/")
}

func funcKey(fn *types.Func) string {
	return fn.Origin().FullName()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sdkModuleDir finds the Go module inside an SDK checkout: the directory
// itself when it is the module, or its go/ subdirectory when it is the
// repository root.
func sdkModuleDir(dir string) (string, error) {
	for _, d := range []string{dir, filepath.Join(dir, "go")} {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil && strings.Contains(string(data), "module "+sdkModulePath+"\n") {
			return d, nil
		}
	}
	return "", fmt.Errorf("%s is not the SDK's Go module (%s) or a checkout containing it at go/", dir, sdkModulePath)
}
