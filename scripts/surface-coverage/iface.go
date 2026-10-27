package main

import (
	"go/ast"
	"go/types"
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
// type declared in the repository: a call argument, an assignment, a typed
// var, a return, a composite-literal element or a conversion. It follows the
// value one step; an SDK value first wrapped in a struct of the repository's
// own is not followed.
type ifaceBindings map[*types.TypeName]map[types.Type]bool

func (b ifaceBindings) collect(file *ast.File, info *types.Info) {
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

func (b ifaceBindings) bindComposite(lit *ast.CompositeLit, info *types.Info) {
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
func (b ifaceBindings) bind(dst types.Type, src ast.Expr, info *types.Info) {
	named, ok := types.Unalias(dst).(*types.Named)
	if !ok || !types.IsInterface(named) {
		return
	}
	tv, ok := info.Types[src]
	if !ok || tv.Type == nil || !sdkType(tv.Type) {
		return
	}
	b.add(named, tv.Type)
}

func (b ifaceBindings) add(iface *types.Named, t types.Type) {
	obj := iface.Origin().Obj()
	if b[obj] == nil {
		b[obj] = map[types.Type]bool{}
	}
	if b[obj][t] {
		return
	}
	b[obj][t] = true
	// A method of an embedded interface is called with that interface as its
	// receiver, so bind the embedded interfaces too.
	it := iface.Underlying().(*types.Interface)
	for i := 0; i < it.NumEmbeddeds(); i++ {
		if e, ok := types.Unalias(it.EmbeddedType(i)).(*types.Named); ok && types.IsInterface(e) {
			b.add(e, t)
		}
	}
}

func sdkType(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && inSDK(n.Obj().Pkg().Path())
}

// expand returns, for a reference to an interface method the repository
// declares, the SDK methods it dispatches to.
func (b ifaceBindings) expand(fn *types.Func) []*types.Func {
	recv := fn.Signature().Recv()
	if recv == nil {
		return nil
	}
	named, ok := types.Unalias(recv.Type()).(*types.Named)
	if !ok || !types.IsInterface(named) {
		return nil
	}
	var out []*types.Func
	for t := range b[named.Origin().Obj()] {
		obj, _, _ := types.LookupFieldOrMethod(t, true, fn.Pkg(), fn.Name())
		if m, ok := obj.(*types.Func); ok && m.Pkg() != nil && inSDK(m.Pkg().Path()) {
			out = append(out, m)
		}
	}
	return out
}
