package main

// Execution manifest (#602).
//
// The case census in case_census.go answers "did THIS runner account for every
// fixture case". It cannot answer the question #602 actually asks — "is any
// case executed by NO runner" — because a case every runner excludes leaves
// each runner's own census green: each one counted its own skip.
//
// Answering that needs the six exclusion sets compared in one place, and this
// file is how one runner contributes its set. Each runner writes the cases it
// did NOT execute, with the reason, to conformance/manifests/<runner>.json;
// scripts/check-fixture-execution reads all six and fails when a case appears
// in every one of them.
//
// WHY A FILE RATHER THAN PARSED OUTPUT. Five runners print a `SKIP: <name>`
// line and TypeScript does not — it expresses a skip as `it.skip`, which vitest
// reports in its own format. A gate that scraped stdout would therefore be
// blind to exactly one runner, and blind in the silent direction: TypeScript
// would contribute an empty exclusion set and no case could ever reach
// all-six. The manifest is written from the same counters the run loop
// increments, so it cannot drift from what the runner actually did.
//
// EXECUTED IS RECORDED, NOT JUST EXCLUDED. `executed + len(excluded)` must
// equal the census total, which is asserted here and re-asserted by the gate.
// Without it a runner that silently dropped a case would contribute a manifest
// that simply does not mention it, and "absent from the exclusion set" reads
// identically to "ran fine" — the collecting gate would then conclude the case
// is covered by that runner precisely when it was not.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ManifestExclusion is one case this runner did not execute.
type ManifestExclusion struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Manifest is one runner's contribution to the cross-runner comparison.
type Manifest struct {
	Runner   string              `json:"runner"`
	Total    int                 `json:"total_non_live"`
	Executed int                 `json:"executed"`
	Excluded []ManifestExclusion `json:"excluded"`
}

// manifestDir is where every runner deposits its manifest, relative to the
// repository root. Gitignored: it is a build product of a conformance run, and
// a stale committed copy would be read by the gate as this run's answer.
const manifestRelDir = "conformance/manifests"

// writeManifest serialises one runner's exclusion set.
//
// The exclusions are sorted by name so a re-run produces byte-identical output;
// a manifest that reordered itself between runs would make any future diffing
// of these files useless.
func writeManifest(repoRoot string, m Manifest) error {
	if m.Executed+len(m.Excluded) != m.Total {
		return fmt.Errorf(
			"manifest for %s is internally inconsistent: %d executed + %d excluded != %d non-live "+
				"cases; the run dropped a case without recording it as either",
			m.Runner, m.Executed, len(m.Excluded), m.Total)
	}

	sort.Slice(m.Excluded, func(i, j int) bool { return m.Excluded[i].Name < m.Excluded[j].Name })
	if m.Excluded == nil {
		// `null` and `[]` decode differently downstream, and "this runner
		// excluded nothing" is a real, checkable claim rather than a missing one.
		m.Excluded = []ManifestExclusion{}
	}

	dir := filepath.Join(repoRoot, manifestRelDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, m.Runner+".json"), append(body, '\n'), 0o644)
}
