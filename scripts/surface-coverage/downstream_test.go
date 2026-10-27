package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMissingSDKSymbols: a call into SDK surface the pinned SDK lacks is
// listed by where it is, whatever the SDK import is named.
func TestMissingSDKSymbols(t *testing.T) {
	repo, _ := copyFixture(t)
	broken := `package cmds

import (
	"context"

	sdk "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func broken(ctx context.Context, ac *sdk.AccountClient) {
	sdk.Gone()
	_ = ac.Things().Vanished(ctx)
	var n int
	n.Nope()
}
`
	if err := os.WriteFile(filepath.Join(repo, "internal/cmds/broken.go"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := loadRepo(repo, []string{"./cmd/app"})
	if err != nil {
		t.Fatal(err)
	}
	// n.Nope() is a type error, but not one about the SDK.
	if len(r.Missing) != 2 || r.Missing[0].Line != 10 || r.Missing[1].Line != 11 || filepath.Base(r.Missing[0].File) != "broken.go" {
		t.Errorf("missing = %+v, want lines 10 and 11 of broken.go", r.Missing)
	}
	if r.TypeErrors < 3 {
		t.Errorf("type errors = %d, want all three counted", r.TypeErrors)
	}
	// The rest of the package still type-checks, so its coverage stands.
	v1, _ := fixtureSDKs(t)
	rep := buildReport(v1, nil, v1, r)
	assertIDs(t, "uncovered beside broken code", ids(rep.Uncovered), []string{"GetGadget", "ReplaceThing"})
}

// TestWorkspaceSDKIsNotCoverage: a go.work that uses a local SDK checkout
// makes the SDK a main module too; its own call sites must not count.
func TestWorkspaceSDKIsNotCoverage(t *testing.T) {
	repo, sdkDir := copyFixture(t)
	work := "go 1.26\n\nuse (\n\t.\n\t" + sdkDir + "\n)\n"
	if err := os.WriteFile(filepath.Join(repo, "go.work"), []byte(work), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := loadRepo(repo, []string{"./cmd/app"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r.Packages {
		if !strings.HasPrefix(p, "example.com/downstream/") {
			t.Errorf("analyzed %s, which is not the repository's", p)
		}
	}
	for _, rf := range r.Refs {
		if !strings.HasPrefix(rf.Site.File, "internal/") && !strings.HasPrefix(rf.Site.File, "cmd/") {
			t.Errorf("reference counted from %s", rf.Site.File)
		}
	}
	v1, _ := fixtureSDKs(t)
	rep := buildReport(v1, nil, v1, r)
	// ThingsService.Summary reaches GadgetsService.Get inside the SDK; were
	// the SDK's packages counted, GetGadget would read as reached.
	assertIDs(t, "uncovered with the SDK in the workspace", ids(rep.Uncovered), []string{"GetGadget", "ReplaceThing"})
}

// TestUnloadablePatternFails: a mistyped -pkg pattern fails the run instead
// of leaving a report that covers only the patterns that loaded.
func TestUnloadablePatternFails(t *testing.T) {
	if _, err := loadRepo("testdata/downstream", []string{"./cmd/app", "./cmd/nope"}); err == nil {
		t.Error("loadRepo with a missing package pattern succeeded")
	}
	repo, _ := copyFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, "empty", "tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRepo(repo, []string{"./cmd/app", "./empty/..."}); err == nil {
		t.Error("loadRepo with a wildcard that matches nothing succeeded")
	}
}

// TestInvalidBindingIsNotCoverage: in code that does not compile, an SDK
// value assigned to an interface it does not satisfy binds nothing, though
// the interface has a method of the same name.
func TestInvalidBindingIsNotCoverage(t *testing.T) {
	repo, _ := copyFixture(t)
	src := `package cmds

import (
	"context"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

type byName interface {
	Get(ctx context.Context, id string) error
}

func invalid(ctx context.Context, ac *basecamp.AccountClient) {
	var b byName = ac.Gadgets()
	_ = b.Get(ctx, "x")
}
`
	if err := os.WriteFile(filepath.Join(repo, "internal/cmds/invalid.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := loadRepo(repo, []string{"./cmd/app"})
	if err != nil {
		t.Fatal(err)
	}
	if r.TypeErrors == 0 {
		t.Fatal("the invalid assignment type-checked")
	}
	v1, _ := fixtureSDKs(t)
	assertIDs(t, "uncovered beside an invalid binding", ids(buildReport(v1, nil, v1, r).Uncovered), []string{"GetGadget", "ReplaceThing"})
}

// TestCgoFilesAreRead: an SDK call in a file that imports "C" counts.
func TestCgoFilesAreRead(t *testing.T) {
	if out, err := exec.Command("go", "env", "CGO_ENABLED").Output(); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Skip("cgo is not enabled")
	}
	repo, _ := copyFixture(t)
	src := `package cmds

// int one(void) { return 1; }
import "C"

import (
	"context"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func viaCgo(ctx context.Context, ac *basecamp.AccountClient) {
	_ = ac.Gadgets().Get(ctx, int64(C.one()))
}
`
	if err := os.WriteFile(filepath.Join(repo, "internal/cmds/cgo.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := loadRepo(repo, []string{"./cmd/app"})
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := fixtureSDKs(t)
	rep := buildReport(v1, nil, v1, r)
	assertIDs(t, "uncovered with a cgo caller", ids(rep.Uncovered), []string{"ReplaceThing"})
}
