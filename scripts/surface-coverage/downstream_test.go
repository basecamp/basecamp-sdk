package main

import (
	"os"
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
