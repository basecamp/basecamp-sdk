package basecamp

// Optional fields on this SDK's request and response types are pointers, so
// that absence stays distinguishable from a value (SPEC.md §10): nil means the
// field was not addressed, and a non-nil pointer means this exact value — the
// zero value included. Go has no literal syntax for the address of a constant,
// so writing to those fields otherwise costs a named variable each, and reading
// from them silently compiles into a nil dereference. Ptr and Deref are the two
// halves of that round trip.

// Ptr returns a pointer to v, for setting an optional field.
//
// A nil optional field is omitted from the request; a non-nil one is sent
// verbatim. Ptr therefore always allocates, and never collapses false or "" to
// nil — sending an explicit zero is the whole reason these fields are pointers.
//
// Being generic over every type, one helper covers the scalar fields and the
// pointer-to-slice fields alike:
//
//	entry, err := account.Schedules().UpdateEntry(ctx, entryID, &basecamp.UpdateScheduleEntryRequest{
//	    Summary:        basecamp.Ptr("Kickoff, moved"),
//	    AllDay:         basecamp.Ptr(false),      // an explicit false, not "unset"
//	    ParticipantIDs: basecamp.Ptr([]int64{}),  // an explicit empty list: remove everyone
//	})
//
// T is inferred from the argument, so a field whose type is not an untyped
// literal's default needs the conversion written out: basecamp.Ptr(int32(5))
// for an *int32 field, not basecamp.Ptr(5).
func Ptr[T any](v T) *T {
	return &v
}

// Deref returns the value p points at, or the zero value of T when p is nil.
//
// Reading an optional field is the half that fails quietly. Go auto-dereferences
// a value-receiver method call, so hc.UpdatedAt.IsZero() still compiles against
// a *time.Time and panics at run time on a hill chart that has never moved.
// Deref is total, and makes the absent case an ordinary value:
//
//	hc, err := account.HillCharts().Get(ctx, todosetID)
//	// ...
//	if updated := basecamp.Deref(hc.UpdatedAt); !updated.IsZero() {
//	    fmt.Println("last moved", updated)
//	}
//
// Collapsing absence to the zero value is only correct where the caller cannot
// tell the two apart anyway. Where the difference carries meaning — a string the
// server really sent as empty versus a field it omitted — compare against nil
// instead.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
