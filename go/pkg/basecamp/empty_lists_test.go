package basecamp

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// The API sends an empty list as [] and the SDK must hand callers the same:
// a non-nil empty slice that marshals back as []. Converters used to guard
// with len(x) > 0, collapsing [] to nil, which then marshaled as null (no
// omitempty) or vanished (omitempty). This sweep feeds every
// *FromGenerated converter a fixture whose every list member is [] and checks
// each list the SDK type carries under the same JSON key.

type emptyListCase struct {
	name string
	// run decodes fixture into the converter's generated input, converts it,
	// and returns the SDK value plus the JSON paths the fixture set to [].
	run func(t *testing.T) (out any, emptyPaths [][]string)
}

// sweep builds a case for converter f: the fixture is a JSON object with
// every list member of the generated type G set to [] — at the top level and
// inside nested objects, a couple of levels down — decoded the way the
// generated client decodes a response body.
func sweep[G, T any](name string, f func(G) T) emptyListCase {
	return emptyListCase{name: name, run: func(t *testing.T) (any, [][]string) {
		var g G
		fixture, paths := emptyListFixture(reflect.TypeOf(g), nil, 0)
		raw, err := json.Marshal(fixture)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("decode fixture %s into %T: %v", raw, g, err)
		}
		return f(g), paths
	}}
}

const emptyListFixtureDepth = 2

// emptyListFixture builds a JSON object for struct type t whose list members
// are [] and whose object members recurse, returning the JSON paths of every
// [] it placed. Embedded structs flatten the way encoding/json flattens them.
func emptyListFixture(t reflect.Type, prefix []string, depth int) (map[string]any, [][]string) {
	obj := map[string]any{}
	var paths [][]string
	for i := range t.NumField() {
		f := t.Field(i)
		ft := f.Type
		if f.Anonymous && ft.Kind() == reflect.Struct {
			sub, subPaths := emptyListFixture(ft, prefix, depth)
			maps.Copy(obj, sub)
			paths = append(paths, subPaths...)
			continue
		}
		name := jsonName(f)
		if name == "" {
			continue
		}
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		path := append(slices.Clone(prefix), name)
		switch {
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() != reflect.Uint8:
			obj[name] = []any{}
			paths = append(paths, path)
		case ft.Kind() == reflect.Struct && depth < emptyListFixtureDepth && !isOpaqueJSON(ft):
			sub, subPaths := emptyListFixture(ft, path, depth+1)
			if len(subPaths) > 0 {
				obj[name] = sub
				paths = append(paths, subPaths...)
			}
		}
	}
	return obj, paths
}

// isOpaqueJSON reports whether a struct type decodes itself (times, dates,
// flexible scalars), so the fixture must not hand it an object.
func isOpaqueJSON(t reflect.Type) bool {
	u := reflect.TypeFor[json.Unmarshaler]()
	return t.Implements(u) || reflect.PointerTo(t).Implements(u)
}

func jsonName(f reflect.StructField) string {
	if !f.IsExported() {
		return ""
	}
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// fieldByJSONName finds the field of struct value v encoded under key.
func fieldByJSONName(v reflect.Value, key string) (reflect.Value, bool) {
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			if fv, ok := fieldByJSONName(v.Field(i), key); ok {
				return fv, true
			}
			continue
		}
		if jsonName(f) == key {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// fieldByJSONPath walks v along path, through pointers, stopping where the
// SDK type doesn't carry the API's key.
func fieldByJSONPath(v reflect.Value, path []string) (reflect.Value, bool) {
	for _, key := range path {
		for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		var ok bool
		if v, ok = fieldByJSONName(v, key); !ok {
			return reflect.Value{}, false
		}
	}
	return v, true
}

// encodedAt returns the JSON found at path in raw, and whether it was found.
func encodedAt(raw []byte, path []string) (string, bool) {
	for _, key := range path {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", false
		}
		var ok bool
		if raw, ok = obj[key]; !ok {
			return "", false
		}
	}
	return string(raw), true
}

func emptyListCases() []emptyListCase {
	return []emptyListCase{
		sweep("bookmarkFromGenerated", bookmarkFromGenerated),
		sweep("boostFromGenerated", boostFromGenerated),
		sweep("bucketCardsGroupFromGenerated", bucketCardsGroupFromGenerated),
		sweep("bucketTodosGroupFromGenerated", bucketTodosGroupFromGenerated),
		sweep("calendarFromGenerated", calendarFromGenerated),
		sweep("campfireFromGenerated", campfireFromGenerated),
		sweep("campfireLineFromGenerated", campfireLineFromGenerated),
		sweep("cardColumnFromGenerated", cardColumnFromGenerated),
		sweep("cardFromGenerated", cardFromGenerated),
		sweep("cardTableFromGenerated", cardTableFromGenerated),
		sweep("chatbotFromGenerated", chatbotFromGenerated),
		sweep("clientApprovalFromGenerated", clientApprovalFromGenerated),
		sweep("clientCorrespondenceFromGenerated", clientCorrespondenceFromGenerated),
		sweep("clientReplyFromGenerated", clientReplyFromGenerated),
		sweep("cloudFileFromGenerated", cloudFileFromGenerated),
		sweep("commentFromGenerated", commentFromGenerated),
		sweep("dockItemFromGenerated", dockItemFromGenerated),
		sweep("documentFromGenerated", documentFromGenerated),
		sweep("doorFromGenerated", doorFromGenerated),
		sweep("draftFromGenerated", draftFromGenerated),
		sweep("eventFeedPageFromGenerated", eventFeedPageFromGenerated),
		sweep("eventFromGenerated", eventFromGenerated),
		sweep("everythingFileFromGenerated", everythingFileFromGenerated),
		sweep("feedEventFromGenerated", feedEventFromGenerated),
		sweep("filingFromGenerated", filingFromGenerated),
		sweep("folderFromGenerated", folderFromGenerated),
		sweep("folderWithProjectsFromGenerated", folderWithProjectsFromGenerated),
		sweep("forwardFromGenerated", forwardFromGenerated),
		sweep("forwardReplyFromGenerated", forwardReplyFromGenerated),
		sweep("googleDocumentFromGenerated", googleDocumentFromGenerated),
		sweep("hillChartFromGenerated", hillChartFromGenerated),
		sweep("hillChartVersionFromGenerated", hillChartVersionFromGenerated),
		sweep("inboxFromGenerated", inboxFromGenerated),
		sweep("inboxItemFromGenerated", inboxItemFromGenerated),
		sweep("inboxPageFromGenerated", inboxPageFromGenerated),
		sweep("lineupMarkerFromGenerated", lineupMarkerFromGenerated),
		sweep("messageBoardFromGenerated", messageBoardFromGenerated),
		sweep("messageFromGenerated", messageFromGenerated),
		sweep("messageTypeFromGenerated", messageTypeFromGenerated),
		sweep("myNoteFromGenerated", myNoteFromGenerated),
		sweep("overdueTodosFromGenerated", overdueTodosFromGenerated),
		sweep("personFromGenerated", personFromGenerated),
		sweep("pingFromGenerated", pingFromGenerated),
		sweep("projectConstructionFromGenerated", projectConstructionFromGenerated),
		sweep("projectFromGenerated", projectFromGenerated),
		sweep("questionAnswerFromGenerated", questionAnswerFromGenerated),
		sweep("questionFromGenerated", questionFromGenerated),
		sweep("questionnaireFromGenerated", questionnaireFromGenerated),
		sweep("questionNotificationSettingsFromGenerated", questionNotificationSettingsFromGenerated),
		sweep("questionReminderFromGenerated", questionReminderFromGenerated),
		sweep("recordingFromGenerated", recordingFromGenerated),
		sweep("richTextAttachmentFromGenerated", richTextAttachmentFromGenerated),
		sweep("scheduleEntryFromGenerated", scheduleEntryFromGenerated),
		sweep("scheduleFromGenerated", scheduleFromGenerated),
		sweep("searchResultAttachmentFromGenerated", searchResultAttachmentFromGenerated),
		sweep("searchResultFromGenerated", searchResultFromGenerated),
		sweep("streamTicketFromGenerated", streamTicketFromGenerated),
		sweep("subscriptionFromGenerated", subscriptionFromGenerated),
		sweep("subtaskFromGenerated", subtaskFromGenerated),
		sweep("templateFromGenerated", templateFromGenerated),
		sweep("templateLibraryCardTablesFromGenerated", templateLibraryCardTablesFromGenerated),
		sweep("templateLibraryCopyFromGenerated", templateLibraryCopyFromGenerated),
		sweep("templateLibraryTodolistsFromGenerated", templateLibraryTodolistsFromGenerated),
		sweep("templatificationFromGenerated", templatificationFromGenerated),
		sweep("timelineAttachmentFromGenerated", timelineAttachmentFromGenerated),
		sweep("timelineEventFromGenerated", timelineEventFromGenerated),
		sweep("timesheetEntryFromGenerated", timesheetEntryFromGenerated),
		sweep("todoFromGenerated", todoFromGenerated),
		sweep("todolistFromGenerated", todolistFromGenerated),
		sweep("todosetFromGenerated", todosetFromGenerated),
		sweep("toolFromGenerated", toolFromGenerated),
		sweep("uploadFromGenerated", uploadFromGenerated),
		sweep("uploadVersionFromGenerated", uploadVersionFromGenerated),
		sweep("vaultFromGenerated", vaultFromGenerated),
		sweep("webhookEventFromGenerated", webhookEventFromGenerated),
		sweep("webhookFromGenerated", webhookFromGenerated),
		sweep("webhookPersonFromGenerated", webhookPersonFromGenerated),
		sweep("wormholeFromGenerated", wormholeFromGenerated),
	}
}

func TestConvertersKeepServerSentEmptyListsEmpty(t *testing.T) {
	checked := 0
	for _, c := range emptyListCases() {
		t.Run(c.name, func(t *testing.T) {
			out, paths := c.run(t)
			raw, err := json.Marshal(out)
			if err != nil {
				t.Fatalf("marshal %T: %v", out, err)
			}
			for _, path := range paths {
				fv, ok := fieldByJSONPath(reflect.ValueOf(out), path)
				if !ok {
					continue // the SDK type doesn't carry this list under the API's key
				}
				key := strings.Join(path, ".")
				if fv.Kind() == reflect.Pointer && fv.Type().Elem().Kind() == reflect.Slice {
					checked++
					if fv.IsNil() {
						t.Errorf("%q was [] from the API but converted to a nil pointer", key)
						continue
					}
					fv = fv.Elem()
				} else if fv.Kind() == reflect.Slice {
					checked++
				} else {
					continue
				}
				if fv.IsNil() {
					t.Errorf("%q was [] from the API but converted to nil", key)
				}
				if got, _ := encodedAt(raw, path); got != "[]" {
					t.Errorf("%q was [] from the API but marshals as %q", key, got)
				}
			}
		})
	}
	// Guards against the sweep passing vacuously or losing reach: the floor is
	// the count this sweep reached when written, so a new list field raises
	// the count and a fixture or key that stops reaching a converter fails.
	// Raise the floor when you add lists.
	const minChecked = 87
	t.Logf("checked %d list fields across %d converters", checked, len(emptyListCases()))
	if checked < minChecked {
		t.Errorf("sweep checked only %d list fields; expected the converters' lists to be reached", checked)
	}
}

// Every converter in the package is enrolled above, so a new one can't skip
// the sweep: adding a *FromGenerated converter without a case fails here.
func TestEmptyListSweepCoversEveryConverter(t *testing.T) {
	enrolled := map[string]bool{}
	for _, c := range emptyListCases() {
		enrolled[c.name] = true
	}
	// Slice-in/slice-out helpers and the multi-argument completion merge are
	// exercised through the converters that call them.
	exempt := map[string]bool{
		"searchTypesFromGenerated":            true,
		"richTextAttachmentsFromGenerated":    true,
		"richTextAttachmentsPtrFromGenerated": true,
		"completionFromGenerated":             true,
	}
	conv := regexp.MustCompile(`^\w+FromGenerated$`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !conv.MatchString(fn.Name.Name) {
				continue
			}
			found++
			if !enrolled[fn.Name.Name] && !exempt[fn.Name.Name] {
				t.Errorf("%s (%s) is not in emptyListCases", fn.Name.Name, file)
			}
		}
	}
	if found == 0 {
		t.Fatal("found no converters; is the test running in the package directory?")
	}
}

// Generated types must keep decoding [] as a non-nil empty slice; if the
// generator ever switched list members to pointers or custom decoders, the
// sweep's fixtures would stop meaning what they say.
func TestGeneratedDecodeKeepsEmptyList(t *testing.T) {
	var s generated.Subscription
	if err := json.Unmarshal([]byte(`{"subscribers":[]}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Subscribers == nil {
		t.Fatal("generated.Subscription decoded [] as nil")
	}
}

func TestMapSlicePreservesNilAndEmpty(t *testing.T) {
	double := func(i int) int { return i * 2 }
	if got := mapSlice([]int(nil), double); got != nil {
		t.Errorf("nil input: got %#v, want nil", got)
	}
	if got := mapSlice([]int{}, double); got == nil || len(got) != 0 {
		t.Errorf("empty input: got %#v, want non-nil empty", got)
	}
	if got := mapSlice([]int{1, 2}, double); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Errorf("got %#v", got)
	}
}
