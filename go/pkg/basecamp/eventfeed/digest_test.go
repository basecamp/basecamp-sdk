package eventfeed

// Drives the shared, data-only vectors in
// conformance/event-feed-digest/fixtures/srv2-vectors.json: the published
// srv2 digest table (SPEC.md §23 "Checkpoint Identity") and the checkpoint
// flat-key cases. Every SDK asserts every case; the fixture file is the
// single source — no vector value is inlined here.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// srv2Fixture mirrors the fixture file's shape.
type srv2Fixture struct {
	Srv2Vectors []struct {
		Name          string         `json:"name"`
		Filters       fixtureFilters `json:"filters"`
		CanonicalJSON string         `json:"canonical_json"`
		Digest        string         `json:"digest"`
	} `json:"srv2_vectors"`
	FlatKeyCases []struct {
		Name              string         `json:"name"`
		Origin            string         `json:"origin"`
		AccountID         string         `json:"account_id"`
		ConsumerNamespace string         `json:"consumer_namespace"`
		Filters           fixtureFilters `json:"filters"`
		FilterKey         string         `json:"filter_key"`
		FlatKey           string         `json:"flat_key"`
	} `json:"flat_key_cases"`
}

// fixtureFilters carries id lists as raw JSON: the fixture spells ids both as
// numbers and as strings ("1", "01") to pin post-coercion dedup, so each
// entry is base-10 coerced here exactly as a query parameter would be.
type fixtureFilters struct {
	Types             []string          `json:"types"`
	Buckets           []json.RawMessage `json:"buckets"`
	Creators          []json.RawMessage `json:"creators"`
	Performers        []json.RawMessage `json:"performers"`
	ExcludePerformers []json.RawMessage `json:"exclude_performers"`
	ActorTypes        []string          `json:"actor_types"`
	Reasons           []string          `json:"reasons"`
}

func (ff fixtureFilters) toFilters(t *testing.T) Filters {
	t.Helper()
	return Filters{
		Types:             ff.Types,
		Buckets:           fixtureIDs(t, ff.Buckets),
		Creators:          fixtureIDs(t, ff.Creators),
		Performers:        fixtureIDs(t, ff.Performers),
		ExcludePerformers: fixtureIDs(t, ff.ExcludePerformers),
		ActorTypes:        ff.ActorTypes,
		Reasons:           ff.Reasons,
	}
}

// fixtureIDs coerces raw fixture id entries — JSON numbers or strings — to
// int64, the same base-10 coercion the srv2 contract applies ("1" and "01"
// are one id).
func fixtureIDs(t *testing.T, raws []json.RawMessage) []int64 {
	t.Helper()
	if len(raws) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(raws))
	for _, raw := range raws {
		s := string(raw)
		if len(s) > 0 && s[0] == '"' {
			if err := json.Unmarshal(raw, &s); err != nil {
				t.Fatalf("unquoting fixture id %s: %v", raw, err)
			}
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("coercing fixture id %s: %v", raw, err)
		}
		ids = append(ids, id)
	}
	return ids
}

func loadSrv2Fixture(t *testing.T) srv2Fixture {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	path := filepath.Join(root, "conformance", "event-feed-digest", "fixtures", "srv2-vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading srv2 fixture %s: %v", path, err)
	}
	var fx srv2Fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parsing srv2 fixture %s: %v", path, err)
	}
	if len(fx.Srv2Vectors) == 0 || len(fx.FlatKeyCases) == 0 {
		t.Fatalf("srv2 fixture %s carries no vectors (srv2_vectors=%d, flat_key_cases=%d)",
			path, len(fx.Srv2Vectors), len(fx.FlatKeyCases))
	}
	return fx
}

func TestFiltersDigest_Srv2Vectors(t *testing.T) {
	fx := loadSrv2Fixture(t)
	for _, v := range fx.Srv2Vectors {
		t.Run(v.Name, func(t *testing.T) {
			f := v.Filters.toFilters(t)
			if got := f.canonicalJSON(); got != v.CanonicalJSON {
				t.Errorf("canonicalJSON() = %q, want %q", got, v.CanonicalJSON)
			}
			if got := f.Digest(); got != v.Digest {
				t.Errorf("Digest() = %q, want %q", got, v.Digest)
			}
		})
	}
}

func TestCheckpointKeyFlatKey_Srv2FlatKeyCases(t *testing.T) {
	fx := loadSrv2Fixture(t)
	for _, c := range fx.FlatKeyCases {
		t.Run(c.Name, func(t *testing.T) {
			f := c.Filters.toFilters(t)
			if got := f.FilterKey(); got != c.FilterKey {
				t.Errorf("FilterKey() = %q, want %q", got, c.FilterKey)
			}
			origin, err := CanonicalOrigin(c.Origin)
			if err != nil {
				t.Fatalf("CanonicalOrigin(%q): %v", c.Origin, err)
			}
			key := CheckpointKey{
				Origin:            origin,
				AccountID:         c.AccountID,
				ConsumerNamespace: c.ConsumerNamespace,
				FilterKey:         f.FilterKey(),
			}
			if got := key.FlatKey(); got != c.FlatKey {
				t.Errorf("FlatKey() = %q, want %q", got, c.FlatKey)
			}
		})
	}
}
