package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"strings"
)

// wild stands for the part of a raw path the code computes at run time.
const wild = "\x00"

// pathPattern reduces the expression a raw HTTP call passes as its path to a
// pattern: the literal text, with wild wherever a value is computed at run
// time. It understands string constants, + concatenation and fmt.Sprintf
// with a constant format, which is how raw paths are written in practice.
// ok is false for a path it cannot see into (a variable, a helper's result).
func pathPattern(e ast.Expr, info *types.Info) (string, bool) {
	e = ast.Unparen(e)
	if tv, found := info.Types[e]; found && tv.Value != nil && tv.Value.Kind() == constant.String {
		return constant.StringVal(tv.Value), true
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if x.Op.String() != "+" {
			return "", false
		}
		l, lok := pathPattern(x.X, info)
		r, rok := pathPattern(x.Y, info)
		if !lok {
			l = wild
		}
		if !rok {
			r = wild
		}
		if !lok && !rok {
			return "", false
		}
		return l + r, true
	case *ast.CallExpr:
		if isSprintf(x, info) && len(x.Args) > 0 {
			tv, found := info.Types[ast.Unparen(x.Args[0])]
			if !found || tv.Value == nil || tv.Value.Kind() != constant.String {
				return "", false
			}
			format := constant.StringVal(tv.Value)
			// Explicit argument indexes and * widths reorder the arguments;
			// such a path is left unread rather than guessed at.
			if strings.ContainsAny(format, "[*") {
				return "", false
			}
			return expandFormat(format, x.Args[1:], info), true
		}
	}
	return "", false
}

func isSprintf(call *ast.CallExpr, info *types.Info) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := info.Uses[sel.Sel].(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" && fn.Name() == "Sprintf"
}

var verbRE = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z%]`)

// expandFormat turns a Sprintf format, one without explicit argument indexes
// or * widths, into a pattern. A verb whose argument is a constant becomes
// that constant's text, so Sprintf("/todos/%s.json", "open") is
// /todos/open.json and not any to-do; any other verb becomes wild, and %%
// becomes %.
func expandFormat(format string, args []ast.Expr, info *types.Info) string {
	next := 0
	return verbRE.ReplaceAllStringFunc(format, func(v string) string {
		if v == "%%" {
			return "%"
		}
		i := next
		next++
		if i >= len(args) {
			return wild
		}
		tv, ok := info.Types[ast.Unparen(args[i])]
		if !ok || tv.Value == nil || len(v) != 2 {
			return wild
		}
		switch tv.Value.Kind() {
		case constant.String:
			if v == "%s" || v == "%v" {
				return constant.StringVal(tv.Value)
			}
		case constant.Int:
			if v == "%d" || v == "%v" {
				return tv.Value.ExactString()
			}
		}
		return wild
	})
}

// routeTable matches raw paths to catalog operations.
type routeTable struct {
	routes []route
}

type route struct {
	op       string
	method   string
	segments []string         // the account-relative path template, split on "/"
	params   []*regexp.Regexp // per segment: nil for a literal, else what fills it
}

var paramRE = regexp.MustCompile(`\{[^}]+\}`)

func newRouteTable(ops []Operation) *routeTable {
	t := &routeTable{}
	for _, op := range ops {
		p := strings.TrimPrefix(op.Path, "/{accountId}")
		r := route{op: op.ID, method: op.Method, segments: splitPath(p)}
		for _, seg := range r.segments {
			var re *regexp.Regexp
			if paramRE.MatchString(seg) {
				re = regexp.MustCompile("^" + segmentRegexp(seg) + "$")
			}
			r.params = append(r.params, re)
		}
		t.routes = append(t.routes, r)
	}
	return t
}

// splitPath normalizes a path for matching: no scheme, host, query or
// fragment, no ".json" suffix, and no leading or trailing slash.
func splitPath(p string) []string {
	// The query and fragment go first: a URL inside a query value is not the
	// request's host.
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if i := strings.Index(p, "://"); i >= 0 {
		p = p[i+3:]
		if j := strings.Index(p, "/"); j >= 0 {
			p = p[j:]
		} else {
			p = "/"
		}
	}
	p = strings.TrimSuffix(strings.Trim(p, "/"), ".json")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// match returns the operation a raw call reaches: the route with the given
// HTTP method whose template the pattern fits. A literal segment must match a
// literal segment exactly or fill a parameter; a computed (wild) segment may
// only fill a parameter, never stand in for a fixed word, so
// "/projects/%d.json" is GetProject and not some /projects/<word> listing.
// Among fits, the one with the most literal-to-literal segments wins; a tie
// leaves the call unresolved rather than guessing.
//
// accountRelative says the path may leave out the account, as AccountClient
// paths do; Client sends its path as given, so one of its paths must carry
// the account to reach an account route.
func (t *routeTable) match(method, pattern string, accountRelative bool) (string, bool) {
	segs := splitPath(pattern)
	// A path that starts with the account ID, computed or literal (an
	// absolute URL or "/<account>/..."), is tried with that segment dropped.
	var candidates [][]string
	if accountRelative {
		candidates = append(candidates, segs)
	}
	if len(segs) > 0 && (strings.Contains(segs[0], wild) || allDigits(segs[0])) {
		candidates = append(candidates, segs[1:])
	}
	best, bestScore, tie := "", -1, false
	for _, segs := range candidates {
		for _, r := range t.routes {
			if r.method != method || len(r.segments) != len(segs) {
				continue
			}
			score, ok := fit(segs, r)
			if !ok {
				continue
			}
			switch {
			case score > bestScore:
				best, bestScore, tie = r.op, score, false
			case score == bestScore && r.op != best:
				tie = true
			}
		}
		if bestScore >= 0 {
			break
		}
	}
	if bestScore < 0 || tie {
		return "", false
	}
	return best, true
}

// matchRef matches a raw call by its path, minding which client sent it.
//
// An AccountClient path may leave the account out, but only a relative one:
// AccountClient sends a path that starts http:// or https:// as given, and
// so is judged here, not by a URL that merely appears in a query.
func (t *routeTable) matchRef(r ref) (string, bool) {
	lower := strings.ToLower(r.Pattern)
	absolute := strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
	relative := strings.Contains(r.Key, ".AccountClient).") && !absolute
	return t.match(r.Raw, r.Pattern, relative)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func fit(pattern []string, r route) (int, bool) {
	score := 0
	for i, ps := range pattern {
		if r.params[i] == nil {
			if ps != r.segments[i] {
				return 0, false
			}
			score++
			continue
		}
		if !r.params[i].MatchString(strings.ReplaceAll(ps, wild, "1")) {
			return 0, false
		}
	}
	return score, true
}

// segmentRegexp turns a template segment such as "{todoId}" or
// "{sgid}.json" into a regexp in which each parameter matches one segment.
func segmentRegexp(ts string) string {
	var b strings.Builder
	last := 0
	for _, loc := range paramRE.FindAllStringIndex(ts, -1) {
		b.WriteString(regexp.QuoteMeta(ts[last:loc[0]]))
		b.WriteString("[^/]+")
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(ts[last:]))
	return b.String()
}
