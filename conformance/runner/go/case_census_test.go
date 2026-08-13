// Case-census contract (#602).
//
// The check is green on the real fixture tree by construction, so a live run
// only ever proves it can say yes. These cases run it against a SYNTHETIC
// fixture set and prove it can say no — the `mode: "moc"` case in particular,
// which every runner's "mock unless told otherwise" filter drops with nothing
// printed. That divergence is asserted end-to-end here: the census and the run
// loop's own predicate (isMockMode, shared with loadTests) disagree by one, and
// caseCountFailure reports it.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file carrying one case of each kind: a plain mock case (no `mode` at all,
// the common spelling), a live case the runners are meant to drop, and a typo'd
// mode that nothing recognizes.
const censusFixture = `[
  {"name": "plain", "operation": "GetProject"},
  {"name": "live one", "operation": "GetProject", "mode": "live"},
  {"name": "typo", "operation": "GetProject", "mode": "moc"}
]`

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCensusCountsEveryCaseThatIsNotExplicitlyLive(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "cases.json"), censusFixture)

	got, err := countNonLiveCases(dir)
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected the plain and typo'd cases to count (2); got %d", got)
	}
}

func TestATypoedModeMakesTheCountCheckFail(t *testing.T) {
	// The regression this whole check exists for. The runner's own filter keeps
	// one case; the census counts two; the difference is the case executed by
	// nothing.
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.json")
	writeFixture(t, path, censusFixture)

	loaded, err := loadTests(path)
	if err != nil {
		t.Fatalf("loadTests: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("the run loop should keep only the plain case; kept %d", len(loaded))
	}

	expected, err := countNonLiveCases(dir)
	if err != nil {
		t.Fatalf("census: %v", err)
	}

	msg := caseCountFailure(len(loaded), expected)
	if msg == "" {
		t.Fatal("a case no runner recognizes must fail the count check, not pass silently")
	}
	if !strings.Contains(msg, "1 executed by nothing") {
		t.Fatalf("failure should name how many cases went unrun; got %q", msg)
	}
}

func TestCensusFindsFixturesNestedBelowTheTestsDirectory(t *testing.T) {
	// No runner globs recursively, so a case parked one directory down is run
	// by nothing. The census walks, which is what makes that visible.
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "nested", "cases.json"), censusFixture)

	got, err := countNonLiveCases(dir)
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected the nested file to be censused (2 cases); got %d", got)
	}
}

func TestCensusRejectsAFixtureThatDoesNotParse(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "broken.json"), `[{"name": "truncated"`)

	if _, err := countNonLiveCases(dir); err == nil {
		t.Fatal("an unparseable fixture must fail the census, not be skipped")
	}
}

func TestCensusRejectsAFixtureThatIsNotAnArray(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "object.json"), `{"name": "not a list"}`)

	if _, err := countNonLiveCases(dir); err == nil {
		t.Fatal("a fixture that is not an array of cases must fail the census")
	}
}

func TestCensusRejectsAnEmptyTree(t *testing.T) {
	// A census that counted nothing certifies nothing: zero on both sides is
	// the shape a broken walk takes.
	if _, err := countNonLiveCases(t.TempDir()); err == nil {
		t.Fatal("a tree with no fixtures must fail the census")
	}
}

func TestCensusAcceptsATruncatedFixtureAsZeroCases(t *testing.T) {
	// `[]` parses, so the census counts zero for it — and the runner counts
	// zero too. The mismatch this produces is against the OTHER files' cases,
	// which is why the count is taken over the whole tree rather than per file.
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "empty.json"), `[]`)
	writeFixture(t, filepath.Join(dir, "cases.json"), censusFixture)

	got, err := countNonLiveCases(dir)
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	if got != 2 {
		t.Fatalf("an empty fixture contributes nothing; got %d", got)
	}
}

func TestCaseCountFailureAcceptsAgreement(t *testing.T) {
	if msg := caseCountFailure(42, 42); msg != "" {
		t.Fatalf("equal counts should pass; got %q", msg)
	}
}

func TestCaseCountFailureNamesAnOverCount(t *testing.T) {
	msg := caseCountFailure(43, 42)
	if msg == "" {
		t.Fatal("running more cases than the fixtures declare should fail")
	}
	if !strings.Contains(msg, "1 more than the fixtures declare") {
		t.Fatalf("failure should name the over-count; got %q", msg)
	}
}

func TestIsMockModeTreatsAbsenceAsMock(t *testing.T) {
	if !isMockMode("") {
		t.Fatal("an absent mode is a mock case")
	}
	if !isMockMode("mock") {
		t.Fatal("an explicit mock mode is a mock case")
	}
	if isMockMode("live") {
		t.Fatal("live cases belong to the TS live runner")
	}
	if isMockMode("moc") {
		t.Fatal("an unrecognized mode must not be run as mock; the census is what catches it")
	}
}
