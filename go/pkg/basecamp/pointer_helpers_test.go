package basecamp

import "testing"

// These helpers encode the optional-pointer contract (SPEC.md §10) at the seam
// between generated types and the wrapper surface. Their semantics are easy to
// regress by "simplification" — omitzero in particular deliberately collapses a
// present zero value to nil, which is correct ONLY where the zero value means
// "not provided". Sites needing an explicit zero on the wire must use ptr.

func TestOmitzero_CollapsesZeroToNil(t *testing.T) {
	if got := omitzero(""); got != nil {
		t.Errorf(`omitzero("") = %v, want nil`, *got)
	}
	if got := omitzero(0); got != nil {
		t.Errorf("omitzero(0) = %v, want nil", *got)
	}
	if got := omitzero(false); got != nil {
		t.Errorf("omitzero(false) = %v, want nil", *got)
	}
	if got := omitzero("x"); got == nil || *got != "x" {
		t.Error(`omitzero("x") must return a pointer to "x"`)
	}
	if got := omitzero(int32(7)); got == nil || *got != 7 {
		t.Error("omitzero(7) must return a pointer to 7")
	}
}

func TestPtr_AlwaysAddressesEvenZero(t *testing.T) {
	// The counterpart to omitzero: ptr is what a site uses when the zero value
	// is a real instruction (e.g. a zero-indexed position).
	if got := ptr(0); got == nil || *got != 0 {
		t.Error("ptr(0) must return a non-nil pointer to 0")
	}
	if got := ptr(false); got == nil || *got != false {
		t.Error("ptr(false) must return a non-nil pointer to false")
	}
}

func TestIntPtrFrom_PreservesNilAndZero(t *testing.T) {
	if got := intPtrFrom[int32](nil); got != nil {
		t.Errorf("intPtrFrom(nil) = %v, want nil (absence must survive)", *got)
	}
	zero := int32(0)
	if got := intPtrFrom(&zero); got == nil || *got != 0 {
		t.Error("an explicit zero must survive as a non-nil zero, not collapse to nil")
	}
	v := int32(42)
	if got := intPtrFrom(&v); got == nil || *got != 42 {
		t.Error("intPtrFrom(&42) must return a pointer to 42")
	}
}
