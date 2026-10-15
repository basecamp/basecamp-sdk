package basecamp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"
)

// TestListResults_EmptyListIsNotNil drives every AccountClient service method
// against a server that answers every request with an empty JSON array, and
// asserts that whatever list the method returns is an empty, non-nil slice.
//
// A nil slice encodes as JSON null, so a consumer that serializes a result
// (an MCP server, a CLI printing JSON) reported "no folders" as null rather
// than as []. The test enumerates the services by reflection rather than by
// a hand-kept list, so a list method added later is covered without anyone
// remembering to add it. Methods the empty array cannot satisfy (those that
// decode an object, or that need arguments the zero value cannot supply)
// return an error or panic and are skipped; the floor below fails the test if
// that ever leaves too few list methods exercised for the result to mean
// anything.
func TestListResults_EmptyListIsNotNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	ac := reflect.ValueOf(NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999"))

	var exercised []string
	for i := range ac.NumMethod() {
		m := ac.Type().Method(i)
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || !isServicePointer(m.Type.Out(0)) {
			continue
		}
		svc := ac.Method(i).Call(nil)[0]
		for j := range svc.NumMethod() {
			sm := svc.Type().Method(j)
			if !returnsList(svc.Method(j).Type()) {
				continue
			}
			name := svc.Elem().Type().Name() + "." + sm.Name
			out, ok := callWithZeroArgs(svc.Method(j))
			if !ok {
				continue
			}
			exercised = append(exercised, name)
			for _, path := range nilLists(out[0]) {
				t.Errorf("%s: %s is nil on an empty response; it encodes as null, not []", name, path)
			}
		}
	}

	sort.Strings(exercised)
	t.Logf("exercised %d list methods: %v", len(exercised), exercised)
	// The SDK has well over this many list methods that decode a top-level
	// array; far fewer exercised means the harness stopped reaching them.
	const floor = 50
	if len(exercised) < floor {
		t.Fatalf("exercised only %d list methods (want at least %d): the harness no longer reaches them", len(exercised), floor)
	}
}

// isServicePointer reports whether t is a pointer to a *Service struct.
func isServicePointer(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct &&
		len(t.Elem().Name()) > len("Service") && t.Elem().Name()[len(t.Elem().Name())-len("Service"):] == "Service"
}

// returnsList reports whether a method returns (list, error), where list is a
// slice or a pointer to a struct carrying one.
func returnsList(ft reflect.Type) bool {
	if ft.NumOut() != 2 || ft.Out(1) != reflect.TypeFor[error]() {
		return false
	}
	if ft.NumIn() == 0 || ft.In(0) != reflect.TypeFor[context.Context]() {
		return false
	}
	out := ft.Out(0)
	if out.Kind() == reflect.Slice {
		return out.Elem().Kind() != reflect.Uint8
	}
	if out.Kind() != reflect.Pointer || out.Elem().Kind() != reflect.Struct {
		return false
	}
	for i := range out.Elem().NumField() {
		f := out.Elem().Field(i)
		if f.IsExported() && f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() != reflect.Uint8 {
			return true
		}
	}
	return false
}

// callWithZeroArgs calls fn with a bounded context and placeholder arguments
// (1 for integers, "x" for strings, zero values otherwise). It reports false
// when the call errors or panics — the empty array was not a response it
// accepts — so the method is not counted as exercised.
func callWithZeroArgs(fn reflect.Value) (out []reflect.Value, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = nil, false
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ft := fn.Type()
	args := make([]reflect.Value, ft.NumIn())
	args[0] = reflect.ValueOf(ctx)
	for i := 1; i < ft.NumIn(); i++ {
		in := ft.In(i)
		v := reflect.New(in).Elem()
		switch in.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			v.SetInt(1)
		case reflect.String:
			v.SetString("x")
		default:
			// Zero value: nil options, nil slices, false.
		}
		args[i] = v
	}
	if ft.IsVariadic() {
		out = fn.CallSlice(args)
	} else {
		out = fn.Call(args)
	}
	if !out[1].IsNil() {
		return nil, false
	}
	return out, true
}

// nilLists names the nil lists in a returned value: the value itself when it
// is a slice, or else each exported slice field of the struct it points to.
// A nil pointer is reported too: it encodes as null just the same.
func nilLists(v reflect.Value) []string {
	if v.Kind() == reflect.Slice {
		if v.IsNil() {
			return []string{"the result"}
		}
		return nil
	}
	if v.IsNil() {
		return []string{"the result"}
	}
	var out []string
	s := v.Elem()
	for i := range s.NumField() {
		f := s.Type().Field(i)
		if f.IsExported() && f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() != reflect.Uint8 && s.Field(i).IsNil() {
			out = append(out, fmt.Sprintf("result.%s", f.Name))
		}
	}
	return out
}
