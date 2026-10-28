package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestPathPattern(t *testing.T) {
	src := `package p
import ("fmt"; "strconv")
const base = "/projects"
var id int64
var dyn string
var (
	lit    = "/my/profile.json"
	konst  = base + "/1.json"
	concat = "/people/" + strconv.FormatInt(id, 10) + ".json"
	spf    = fmt.Sprintf("/buckets/%d/todos/%v.json?x=%%s", id, id)
	spfDyn = fmt.Sprintf(dyn, id)
	spfK   = fmt.Sprintf("/todos/%s.json", "open")
	spfMix = fmt.Sprintf("/x/%d/%s/%d", id, "y", 7)
	spfIdx = fmt.Sprintf("/todos/%[1]s.json", "open")
	spfStr = fmt.Sprintf("/x/%*d", 3, id)
	bare   = dyn
	both   = dyn + dyn
)`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("p", fset, []*ast.File{f}, info); err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		pattern string
		ok      bool
	}{
		"lit":    {"/my/profile.json", true},
		"konst":  {"/projects/1.json", true},
		"concat": {"/people/" + wild + ".json", true},
		"spf":    {"/buckets/" + wild + "/todos/" + wild + ".json?x=%s", true},
		"spfDyn": {"", false},
		// A constant argument is part of the path.
		"spfK":   {"/todos/open.json", true},
		"spfMix": {"/x/" + wild + "/y/7", true},
		// Explicit indexes and * widths reorder the arguments: left unread,
		// so the call is listed rather than credited to /todos/{todoId}.
		"spfIdx": {"", false},
		"spfStr": {"", false},
		"bare":   {"", false},
		"both":   {"", false},
	}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			if len(vs.Values) == 0 {
				continue
			}
			name := vs.Names[0].Name
			got, ok := pathPattern(vs.Values[0], info)
			if w := want[name]; got != w.pattern || ok != w.ok {
				t.Errorf("%s: pathPattern = %q, %v; want %q, %v", name, got, ok, w.pattern, w.ok)
			}
		}
	}
}

func TestRouteMatch(t *testing.T) {
	routes := newRouteTable([]Operation{
		{ID: "GetProject", Method: "GET", Path: "/{accountId}/projects/{projectId}.json"},
		{ID: "ListRecentProjects", Method: "GET", Path: "/{accountId}/projects/recent.json"},
		{ID: "TrashProject", Method: "DELETE", Path: "/{accountId}/projects/{projectId}.json"},
		{ID: "GetMyProfile", Method: "GET", Path: "/{accountId}/my/profile.json"},
		{ID: "GetMyThing", Method: "GET", Path: "/{accountId}/my/{thing}.json"},
		{ID: "GetEmbed", Method: "GET", Path: "/{accountId}/embeds/{sgid}.json"},
		{ID: "GetA", Method: "GET", Path: "/{accountId}/x/{a}.json"},
		{ID: "GetB", Method: "GET", Path: "/{accountId}/x/{b}.json"},
	})
	for _, c := range []struct {
		method, pattern, want string
	}{
		// AccountClient paths may leave the account out (the default below).
		{"GET", "/projects/" + wild + ".json", "GetProject"},
		// A computed segment never stands in for a fixed word.
		{"GET", "/projects/recent.json", "ListRecentProjects"},
		{"DELETE", "/projects/" + wild + ".json", "TrashProject"},
		// A literal fits a parameter, but an exact literal wins.
		{"GET", "/my/profile.json", "GetMyProfile"},
		{"GET", "/my/settings.json", "GetMyThing"},
		{"GET", "/embeds/" + wild, "GetEmbed"},
		{"GET", "/projects/" + wild + ".json?page=2", "GetProject"},
		// A URL in the query is not the request's host.
		{"GET", "/projects/" + wild + ".json?next=https://example.com/x", "GetProject"},
		{"GET", "https://3.basecampapi.com/" + wild + "/projects/" + wild + ".json", "GetProject"},
		{"GET", "/" + wild + "/projects/" + wild + ".json", "GetProject"},
		// A literal account ID, which AccountClient accepts too.
		{"GET", "/12345/projects/" + wild + ".json", "GetProject"},
		{"GET", "https://3.basecampapi.com/12345/projects/1.json", "GetProject"},
		{"POST", "/projects/" + wild + ".json", ""},
		{"GET", "/nowhere.json", ""},
		// Two equally good fits: unresolved rather than a guess.
		{"GET", "/x/" + wild + ".json", ""},
	} {
		got, ok := routes.match(c.method, c.pattern, true)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("match(%s %q) = %q, %v; want %q", c.method, display(c.pattern), got, ok, c.want)
		}
	}
}

// TestClientPathsCarryTheAccount: Client sends its path as given, so an
// account-relative path from it reaches no account route.
func TestClientPathsCarryTheAccount(t *testing.T) {
	routes := newRouteTable([]Operation{{ID: "GetMyProfile", Method: "GET", Path: "/{accountId}/my/profile.json"}})
	for pattern, want := range map[string]string{
		"/my/profile.json":                              "",
		"/123/my/profile.json":                          "GetMyProfile",
		"/" + wild + "/my/profile.json":                 "GetMyProfile",
		"https://3.basecampapi.com/123/my/profile.json": "GetMyProfile",
	} {
		if got, _ := routes.match("GET", pattern, false); got != want {
			t.Errorf("Client match(%q) = %q, want %q", display(pattern), got, want)
		}
	}
	client := "(*" + sdkPkgPath + ".Client).Get"
	account := "(*" + sdkPkgPath + ".AccountClient).Get"
	if _, ok := routes.matchRef(ref{Key: client, Raw: "GET", Pattern: "/my/profile.json", Resolved: true}); ok {
		t.Error("Client.Get of an account-relative path matched")
	}
	if _, ok := routes.matchRef(ref{Key: account, Raw: "GET", Pattern: "/my/profile.json", Resolved: true}); !ok {
		t.Error("AccountClient.Get of an account-relative path did not match")
	}
	// AccountClient sends an absolute URL as given, so it must carry the account.
	if _, ok := routes.matchRef(ref{Key: account, Raw: "GET", Pattern: "https://3.basecampapi.com/my/profile.json", Resolved: true}); ok {
		t.Error("AccountClient.Get of an absolute URL without the account matched")
	}
	if _, ok := routes.matchRef(ref{Key: account, Raw: "GET", Pattern: "https://3.basecampapi.com/123/my/profile.json", Resolved: true}); !ok {
		t.Error("AccountClient.Get of an absolute URL with the account did not match")
	}
	// A URL in the query does not make a relative path absolute.
	if _, ok := routes.matchRef(ref{Key: account, Raw: "GET", Pattern: "/my/profile.json?next=https://example.com/x", Resolved: true}); !ok {
		t.Error("AccountClient.Get of a relative path with a URL in its query did not match")
	}
}
