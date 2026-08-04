package basecamp

import (
	"testing"
	"time"
)

// Ptr and Deref are the exported halves of the optional-pointer contract
// (SPEC.md §10), and both have a failure mode that a plausible "simplification"
// would introduce silently. Ptr must allocate unconditionally — collapsing a
// zero value to nil would turn "set this to false" into "leave it alone", which
// is the distinction the pointer exists to carry. Deref must be total — the
// obvious one-liner `return *p` compiles, passes every non-nil test, and panics
// on exactly the absent field it was reached for.

// assertPtrRoundTrip pins Ptr's whole contract for one comparable type: a
// non-nil pointer, addressing that value.
func assertPtrRoundTrip[T comparable](t *testing.T, v T) {
	t.Helper()
	got := Ptr(v)
	if got == nil {
		t.Fatalf("Ptr(%v) = nil, want a non-nil pointer (a zero value must still reach the wire)", v)
	}
	if *got != v {
		t.Errorf("*Ptr(%v) = %v, want %v", v, *got, v)
	}
}

func TestPtr_AddressesTheValueIncludingZero(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"string zero", func(t *testing.T) { assertPtrRoundTrip(t, "") }},
		{"string value", func(t *testing.T) { assertPtrRoundTrip(t, "Kickoff, moved") }},
		{"bool false", func(t *testing.T) { assertPtrRoundTrip(t, false) }},
		{"bool true", func(t *testing.T) { assertPtrRoundTrip(t, true) }},
		{"int zero", func(t *testing.T) { assertPtrRoundTrip(t, 0) }},
		{"int32", func(t *testing.T) { assertPtrRoundTrip(t, int32(5)) }},
		{"int64", func(t *testing.T) { assertPtrRoundTrip(t, int64(1069479400)) }},
		{"time zero", func(t *testing.T) { assertPtrRoundTrip(t, time.Time{}) }},
		{"time value", func(t *testing.T) {
			assertPtrRoundTrip(t, time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// The pointer-to-slice fields are the reason this is one generic helper rather
// than a set of typed constructors: UpdateScheduleEntryRequest.ParticipantIDs is
// *[]int64, where nil leaves the participants alone and a pointer to an empty
// slice removes everyone. Slices are not comparable, so this case cannot ride
// on assertPtrRoundTrip.
func TestPtr_AddressesSlicesIncludingTheEmptyOne(t *testing.T) {
	tests := []struct {
		name string
		in   []int64
	}{
		{"nil slice", nil},
		{"empty slice", []int64{}},
		{"populated slice", []int64{1069479400, 1069479401}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ptr(tt.in)
			if got == nil {
				t.Fatalf("Ptr(%v) = nil, want a non-nil pointer even for an empty or nil slice", tt.in)
			}
			if len(*got) != len(tt.in) {
				t.Fatalf("len(*Ptr(%v)) = %d, want %d", tt.in, len(*got), len(tt.in))
			}
			for i, want := range tt.in {
				if (*got)[i] != want {
					t.Errorf("(*Ptr(%v))[%d] = %d, want %d", tt.in, i, (*got)[i], want)
				}
			}
		})
	}
}

// Every call allocates. A shared address would alias every request built in a
// loop onto whatever the last iteration wrote.
func TestPtr_ReturnsADistinctPointerPerCall(t *testing.T) {
	first, second := Ptr("Kickoff"), Ptr("Kickoff")
	if first == second {
		t.Fatal("two Ptr calls returned the same address; requests built in a loop would alias")
	}
	*first = "moved"
	if *second != "Kickoff" {
		t.Errorf("writing through one pointer changed the other: *second = %q, want %q", *second, "Kickoff")
	}
}

// The load-bearing case. `return *p` passes everything above and fails only
// here — by panicking, which is precisely the run-time break these tests exist
// to prevent (hc.UpdatedAt.IsZero() on an absent timestamp compiles fine).
func TestDeref_ReturnsTheZeroValueForNilRatherThanPanicking(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"*string", func(t *testing.T) {
			if got := Deref[string](nil); got != "" {
				t.Errorf("Deref[string](nil) = %q, want %q", got, "")
			}
		}},
		{"*bool", func(t *testing.T) {
			if got := Deref[bool](nil); got != false {
				t.Errorf("Deref[bool](nil) = %v, want false", got)
			}
		}},
		{"*int", func(t *testing.T) {
			if got := Deref[int](nil); got != 0 {
				t.Errorf("Deref[int](nil) = %d, want 0", got)
			}
		}},
		{"*int64", func(t *testing.T) {
			if got := Deref[int64](nil); got != 0 {
				t.Errorf("Deref[int64](nil) = %d, want 0", got)
			}
		}},
		{"*time.Time", func(t *testing.T) {
			// The exact shape the migration guide flags: HillChart.UpdatedAt is
			// nil on a chart that has never moved.
			var absent *time.Time
			if got := Deref(absent); !got.IsZero() {
				t.Errorf("Deref((*time.Time)(nil)) = %v, want the zero time", got)
			}
		}},
		{"*[]int64", func(t *testing.T) {
			if got := Deref[[]int64](nil); got != nil {
				t.Errorf("Deref[[]int64](nil) = %v, want nil", got)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestDeref_ReturnsThePointedAtValue(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"string", func(t *testing.T) {
			if got := Deref(Ptr("Kickoff")); got != "Kickoff" {
				t.Errorf("Deref(Ptr(%q)) = %q, want %q", "Kickoff", got, "Kickoff")
			}
		}},
		{"explicit false survives", func(t *testing.T) {
			// A present false and an absent field both read back as false; that
			// collapse is Deref's documented cost, and the round trip through
			// Ptr is what proves the value was carried, not manufactured.
			if got := Deref(Ptr(false)); got != false {
				t.Errorf("Deref(Ptr(false)) = %v, want false", got)
			}
		}},
		{"explicit empty string survives", func(t *testing.T) {
			if got := Deref(Ptr("")); got != "" {
				t.Errorf("Deref(Ptr(%q)) = %q, want %q", "", got, "")
			}
		}},
		{"time", func(t *testing.T) {
			want := time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC)
			if got := Deref(Ptr(want)); !got.Equal(want) {
				t.Errorf("Deref(Ptr(%v)) = %v, want %v", want, got, want)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// The unexported spellings forward to the exported ones, so the contract the
// hundreds of internal conversion sites rely on is the same code consumers get.
// Pin that: a future reimplementation of either half has to keep them agreeing.
func TestUnexportedSpellingsForwardToTheExportedOnes(t *testing.T) {
	t.Run("deref matches Deref on nil", func(t *testing.T) {
		var absent *time.Time
		if got, want := deref(absent), Deref(absent); !got.Equal(want) {
			t.Errorf("deref(nil) = %v, Deref(nil) = %v; the two must not diverge", got, want)
		}
	})
	t.Run("deref matches Deref on a value", func(t *testing.T) {
		p := Ptr("Kickoff")
		if got, want := deref(p), Deref(p); got != want {
			t.Errorf("deref(p) = %q, Deref(p) = %q; the two must not diverge", got, want)
		}
	})
	t.Run("ptr matches Ptr on a zero value", func(t *testing.T) {
		if got, want := ptr(false), Ptr(false); got == nil || want == nil || *got != *want {
			t.Error("ptr(false) and Ptr(false) must both address a non-nil false")
		}
	})
}
