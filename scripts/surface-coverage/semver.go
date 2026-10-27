package main

import (
	"strconv"
	"strings"
)

// semverLess reports whether version a sorts before b under semantic
// versioning, including pre-release precedence (Go pseudo-versions are
// pre-releases). Malformed versions sort first.
func semverLess(a, b string) bool {
	return semverCompare(a, b) < 0
}

func semverCompare(a, b string) int {
	an, ap, aok := parseSemver(a)
	bn, bp, bok := parseSemver(b)
	switch {
	case !aok && !bok:
		return strings.Compare(a, b)
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for i := 0; i < 3; i++ {
		if an[i] != bn[i] {
			if an[i] < bn[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case ap == "" && bp == "":
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	}
	ai, bi := strings.Split(ap, "."), strings.Split(bp, ".")
	for i := 0; i < len(ai) && i < len(bi); i++ {
		if c := compareIdent(ai[i], bi[i]); c != 0 {
			return c
		}
	}
	return len(ai) - len(bi)
}

func compareIdent(a, b string) int {
	an, aerr := strconv.ParseUint(a, 10, 64)
	bn, berr := strconv.ParseUint(b, 10, 64)
	switch {
	case aerr == nil && berr == nil:
		if an < bn {
			return -1
		} else if an > bn {
			return 1
		}
		return 0
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	}
	return strings.Compare(a, b)
}

func parseSemver(v string) (nums [3]uint64, pre string, ok bool) {
	if !strings.HasPrefix(v, "v") {
		return nums, "", false
	}
	v = v[1:]
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nums, "", false
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nums, "", false
		}
		nums[i] = n
	}
	return nums, pre, true
}

// isRelease reports a tagged release: valid, with no pre-release part.
func isRelease(v string) bool {
	_, pre, ok := parseSemver(v)
	return ok && pre == ""
}

// previousRelease returns the newest release in versions that sorts before v.
func previousRelease(v string, versions []string) string {
	best := ""
	for _, c := range versions {
		if isRelease(c) && semverLess(c, v) && (best == "" || semverLess(best, c)) {
			best = c
		}
	}
	return best
}
