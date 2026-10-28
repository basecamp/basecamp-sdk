package main

import (
	"go/importer"
	"go/token"
	"go/types"
	"testing"
)

// TestOnlyTheRepositorysInterfacesAreSeams: an interface from another module
// or the standard library (io.Reader, error) is not a seam, so SDK values
// handed to it are not bound, and the empty interface never is.
func TestOnlyTheRepositorysInterfacesAreSeams(t *testing.T) {
	b := newIfaceBindings("example.com/repo")
	io, err := importer.Default().Import("io")
	if err != nil {
		t.Fatal(err)
	}
	if key, _ := b.ifaceKey(io.Scope().Lookup("Reader").Type()); key != nil {
		t.Error("io.Reader is treated as the repository's interface")
	}
	if key, _ := b.ifaceKey(types.Universe.Lookup("error").Type()); key != nil {
		t.Error("error is treated as the repository's interface")
	}
	if key, _ := b.ifaceKey(types.NewInterfaceType(nil, nil)); key != nil {
		t.Error("the empty interface is treated as a seam")
	}

	own := types.NewPackage("example.com/repo/internal/x", "x")
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	m := types.NewFunc(token.NoPos, own, "Get", sig)
	named := types.NewNamed(types.NewTypeName(token.NoPos, own, "getter", nil), types.NewInterfaceType([]*types.Func{m}, nil), nil)
	if key, _ := b.ifaceKey(named); key == nil {
		t.Error("an interface declared in the repository is not a seam")
	}
	other := types.NewPackage("example.com/repository-other", "o")
	m2 := types.NewFunc(token.NoPos, other, "Get", sig)
	near := types.NewNamed(types.NewTypeName(token.NoPos, other, "getter", nil), types.NewInterfaceType([]*types.Func{m2}, nil), nil)
	if key, _ := b.ifaceKey(near); key != nil {
		t.Error("a module whose path merely starts with the repository's is treated as the repository")
	}
}
