package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunAtRef drives the command end to end the way C calls it — an SDK
// version, a repository path, Markdown or JSON out — against a git ref whose
// tree differs from the working tree, so it also proves -ref reads the
// commit and not the checkout.
func TestRunAtRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo, sdkDir := copyFixture(t)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")

	report := func(args ...string) *Report {
		t.Helper()
		var out bytes.Buffer
		base := []string{"-sdk-dir", sdkDir, "-sdk", "v1.0.0", "-prev", "none", "-repo", repo, "-pkg", "./cmd/app", "-format", "json"}
		if err := run(append(base, args...), &out); err != nil {
			t.Fatal(err)
		}
		var r Report
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		return &r
	}

	if clean := report(); strings.HasSuffix(clean.Commit, "-dirty") || clean.Commit == "" {
		t.Errorf("clean working-tree commit = %q, want HEAD's short sha", clean.Commit)
	}

	// The working tree stops calling Delete; the commit still does.
	cmds := filepath.Join(repo, "internal/cmds/cmds.go")
	src, _ := os.ReadFile(cmds)
	os.WriteFile(cmds, []byte(strings.Replace(string(src), "_ = ac.Things().Delete(ctx, 2)", "", 1)), 0o644)

	atRef := report("-ref", "HEAD")
	if atRef.Ref != "HEAD" || atRef.Commit == "" {
		t.Errorf("ref = %q, commit = %q", atRef.Ref, atRef.Commit)
	}
	assertIDs(t, "uncovered at HEAD", ids(atRef.Uncovered), []string{"GetGadget", "ReplaceThing"})

	worktree := report()
	if worktree.Commit != atRef.Commit+"-dirty" {
		t.Errorf("working-tree commit = %q, want %q: the tree differs from HEAD", worktree.Commit, atRef.Commit+"-dirty")
	}
	assertIDs(t, "uncovered in the working tree", ids(worktree.Uncovered), []string{"DeleteThing", "GetGadget", "ReplaceThing"})

	var md bytes.Buffer
	if err := run([]string{"-sdk-dir", sdkDir, "-sdk", "v1.0.0", "-prev", "none", "-repo", repo, "-pkg", "./cmd/app"}, &md); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(md.String(), "### SDK operation coverage: downstream at `") {
		t.Errorf("markdown starts %q", md.String()[:60])
	}
}

// copyFixture copies the downstream fixture and the v1 SDK it pins into a
// temp dir, with the replace made absolute so the copy (or an archive of it)
// resolves from anywhere.
func copyFixture(t *testing.T) (repo, sdkDir string) {
	t.Helper()
	root := t.TempDir()
	sdkDir = filepath.Join(root, "sdk-v1")
	repo = filepath.Join(root, "downstream")
	copyTree(t, "testdata/sdk-v1", sdkDir)
	copyTree(t, "testdata/downstream", repo)
	gomod := filepath.Join(repo, "go.mod")
	data, _ := os.ReadFile(gomod)
	os.WriteFile(gomod, []byte(strings.Replace(string(data), "../sdk-v1", sdkDir, 1)), 0o644)
	return repo, sdkDir
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
