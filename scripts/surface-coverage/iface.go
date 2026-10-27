package main

import (
	"go/ast"
	"go/types"
	"strings"
)

// ifaceBindings records which SDK types the repository hands to which of its
// own interfaces. Code that wraps an SDK service behind a small interface —
// a seam for tests, say — calls the interface's method, not the SDK's, and
// without this the call would be invisible:
//
//	type embedClient interface{ Create(ctx context.Context, url string) (*basecamp.Embed, error) }
//	createEmbed(ctx, app.Account().Embeds(), url) // *basecamp.EmbedsService becomes an embedClient
//	client.Create(ctx, url)                       // reaches EmbedsService.Create
//
// A binding is recorded wherever a value of an SDK type meets an interface
// the repository declares, or writes inline: a call argument, an assignment,
// a typed var, a return, a composite-literal element or a conversion. It
// follows the value one step; an SDK value first wrapped in a struct of the
// repository's own is not followed.
//
// Bindings are per interface, not per call, so a call through an interface
// that more than one SDK method could answer is not credited to any of them
// (see expand): which one a given call reaches is beyond what this sees, and
// crediting both would hide a gap.
type ifaceBindings struct {
	mod string // the repository's module path
	// m is keyed by a named interface's declaration, or for an interface
	// written inline by the *types.Interface itself, which is also the
	// receiver of the methods called through it.
	m map[any]map[string]types.Type
}

func newIfaceBindings(mod string) *ifaceBindings {
	return &ifaceBindings{mod: mod, m: map[any]map[string]types.Type{}}
}

// ifaceKey identifies an interface the repository owns: one declared in its
// module, or one written inline. Others (error, the SDK's own interfaces,
// another module's) and the empty interface are not seams.
func (b *ifaceBindings) ifaceKey(t types.Type) (any, *types.Interface) {
	switch x := types.Unalias(t).(type) {
	case *types.Named:
		it, ok := x.Underlying().(*types.Interface)
		pkg := x.Obj().Pkg()
		if ok && it.NumMethods() > 0 && pkg != nil && (pkg.Path() == b.mod || strings.HasPrefix(pkg.Path(), b.mod+"/")) {
			return x.Origin().Obj(), it
		}
	case *types.Interface:
		if x.NumMethods() > 0 {
			return x, x
		}
	}
	return nil, nil
}

func (b *ifaceBindings) collect(file *ast.File, info *types.Info) {
	var sigs []*types.Signature
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if fn, ok := info.Defs[x.Name].(*types.Func); ok && x.Body != nil {
				sigs = append(sigs, fn.Signature())
				ast.Inspect(x.Body, visit)
				sigs = sigs[:len(sigs)-1]
			}
			return false
		case *ast.FuncLit:
			if sig, ok := info.Types[x].Type.(*types.Signature); ok {
				sigs = append(sigs, sig)
				ast.Inspect(x.Body, visit)
				sigs = sigs[:len(sigs)-1]
			}
			return false
		case *ast.CallExpr:
			fun := info.Types[x.Fun]
			if fun.IsType() && len(x.Args) == 1 {
				b.bind(fun.Type, x.Args[0], info)
				return true
			}
			sig, ok := fun.Type.(*types.Signature)
			if !ok {
				return true
			}
			params := sig.Params()
			for i, arg := range x.Args {
				var pt types.Type
				switch {
				case sig.Variadic() && i >= params.Len()-1:
					pt = params.At(params.Len() - 1).Type()
					if !x.Ellipsis.IsValid() {
						if s, ok := pt.(*types.Slice); ok {
							pt = s.Elem()
						}
					}
				case i < params.Len():
					pt = params.At(i).Type()
				default:
					continue
				}
				b.bind(pt, arg, info)
			}
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i := range x.Lhs {
					if tv, ok := info.Types[x.Lhs[i]]; ok {
						b.bind(tv.Type, x.Rhs[i], info)
					}
				}
			}
		case *ast.ValueSpec:
			if x.Type != nil && len(x.Values) == len(x.Names) {
				for _, v := range x.Values {
					b.bind(info.Types[x.Type].Type, v, info)
				}
			}
		case *ast.ReturnStmt:
			if len(sigs) > 0 {
				res := sigs[len(sigs)-1].Results()
				if res.Len() == len(x.Results) {
					for i, e := range x.Results {
						b.bind(res.At(i).Type(), e, info)
					}
				}
			}
		case *ast.CompositeLit:
			b.bindComposite(x, info)
		}
		return true
	}
	ast.Inspect(file, visit)
}

func (b *ifaceBindings) bindComposite(lit *ast.CompositeLit, info *types.Info) {
	tv, ok := info.Types[lit]
	if !ok {
		return
	}
	t := tv.Type
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for i, el := range lit.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok {
					for j := 0; j < u.NumFields(); j++ {
						if u.Field(j).Name() == key.Name {
							b.bind(u.Field(j).Type(), kv.Value, info)
						}
					}
				}
			} else if i < u.NumFields() {
				b.bind(u.Field(i).Type(), el, info)
			}
		}
	case *types.Slice, *types.Array, *types.Map:
		elem := u.(interface{ Elem() types.Type }).Elem()
		for _, el := range lit.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			b.bind(elem, el, info)
		}
	}
}

// bind records src flowing into dst when dst is a named interface and src's
// type is declared in the SDK.
func (b *ifaceBindings) bind(dst types.Type, src ast.Expr, info *types.Info) {
	if key, _ := b.ifaceKey(dst); key == nil {
		return
	}
	tv, ok := info.Types[src]
	if !ok || tv.Type == nil || !sdkType(tv.Type) {
		return
	}
	// Only a value that really satisfies the interface: in code that does
	// not compile, a same-named method with another signature is no binding.
	if !types.AssignableTo(tv.Type, dst) {
		return
	}
	b.add(dst, tv.Type)
}

func (b *ifaceBindings) add(iface types.Type, t types.Type) {
	key, it := b.ifaceKey(iface)
	if key == nil {
		return
	}
	if b.m[key] == nil {
		b.m[key] = map[string]types.Type{}
	}
	id := types.TypeString(t, nil)
	if _, seen := b.m[key][id]; seen {
		return
	}
	b.m[key][id] = t
	// A method of an embedded interface is called with that interface as its
	// receiver, so bind the embedded interfaces too.
	for i := 0; i < it.NumEmbeddeds(); i++ {
		b.add(it.EmbeddedType(i), t)
	}
}

func sdkType(t types.Type) bool {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj().Pkg() != nil && inSDK(n.Obj().Pkg().Path())
}

// expand returns, for a reference to a method of an interface the
// repository owns, the SDK method it dispatches to. ambiguous is true when
// the SDK values handed to the interface answer the call with more than one
// SDK method; then none is returned.
func (b *ifaceBindings) expand(fn *types.Func) (m *types.Func, ambiguous bool) {
	recv := fn.Signature().Recv()
	if recv == nil {
		return nil, false
	}
	key, _ := b.ifaceKey(recv.Type())
	if key == nil {
		return nil, false
	}
	found := map[string]*types.Func{}
	for _, t := range b.m[key] {
		obj, _, _ := types.LookupFieldOrMethod(t, true, fn.Pkg(), fn.Name())
		if sm, ok := obj.(*types.Func); ok && sm.Pkg() != nil && inSDK(sm.Pkg().Path()) {
			found[funcKey(sm)] = sm
		}
	}
	if len(found) > 1 {
		return nil, true
	}
	for _, sm := range found {
		return sm, false
	}
	return nil, false
}
