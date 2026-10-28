package main

import (
	"go/token"
	"strings"
	"testing"
)

// TestEveryOperationHasAnSDKEntryPoint runs the tool's SDK half against this
// checkout's own go/ module: every operation in the catalog must be performed
// by at least one exported SDK function or method. It fails when the SDK's
// wrappers change shape in a way the call graph no longer follows (a new
// dispatch idiom, a renamed generated entry point) — in the SDK pull request
// that changes them, rather than as a report downstream that silently counts
// every operation as unreached.
func TestEveryOperationHasAnSDKEntryPoint(t *testing.T) {
	dir, err := sdkModuleDir("../..")
	if err != nil {
		t.Skip("not inside an SDK checkout:", err)
	}
	s, err := loadSDK(dir, "checkout")
	if err != nil {
		t.Fatal(err)
	}
	reached := map[string]bool{}
	for key, ops := range s.reach {
		if !exportedSymbol(key) {
			continue
		}
		for _, op := range ops {
			reached[op] = true
		}
	}
	for _, op := range s.Operations {
		if !reached[op.ID] {
			t.Errorf("%s: no exported SDK function or method performs it", op.ID)
		}
	}
}

// exportedSymbol reports whether a types.Func.FullName names an exported
// function, or an exported method of an exported type.
func exportedSymbol(key string) bool {
	name := key[strings.LastIndex(key, ".")+1:]
	if !token.IsExported(name) {
		return false
	}
	if i := strings.Index(key, ")."); i >= 0 {
		recv := key[:i]
		recv = recv[strings.LastIndex(recv, ".")+1:]
		return token.IsExported(recv)
	}
	return true
}
