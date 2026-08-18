package eventfeed

// Drives the shared, data-only vectors in
// conformance/event-feed-digest/fixtures/srv1-vectors.json: the published
// srv1 digest table (SPEC.md §23 "Checkpoint Identity") and the checkpoint
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

// srv1Fixture mirrors the fixture file's shape.
type srv1Fixture struct {
	Srv1Vectors []struct {
		Name          string         `json:"name"`
		Filters       fixtureFilters `json:"filters"`
		CanonicalJSON string         `json:"canonical_json"`
		Digest        string         `json:"digest"`
	} `json:"srv1_vectors"`
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
	Types    []string          `json:"types"`
	Buckets  []json.RawMessage `json:"buckets"`
	Creators []json.RawMessage `json:"creators"`
}

func (ff fixtureFilters) toFilters(t *testing.T) Filters {
	t.Helper()
	return Filters{
		Types:    ff.Types,
		Buckets:  fixtureIDs(t, ff.Buckets),
		Creators: fixtureIDs(t, ff.Creators),
	}
}

// fixtureIDs coerces raw fixture id entries — JSON numbers or strings — to
// int64, the same base-10 coercion the srv1 contract applies ("1" and "01"
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

func loadSrv1Fixture(t *testing.T) srv1Fixture {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	path := filepath.Join(root, "conformance", "event-feed-digest", "fixtures", "srv1-vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading srv1 fixture %s: %v", path, err)
	}
	var fx srv1Fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parsing srv1 fixture %s: %v", path, err)
	}
	if len(fx.Srv1Vectors) == 0 || len(fx.FlatKeyCases) == 0 {
		t.Fatalf("srv1 fixture %s carries no vectors (srv1_vectors=%d, flat_key_cases=%d)",
			path, len(fx.Srv1Vectors), len(fx.FlatKeyCases))
	}
	return fx
}

func TestFiltersDigest_Srv1Vectors(t *testing.T) {
	fx := loadSrv1Fixture(t)
	for _, v := range fx.Srv1Vectors {
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

func TestCheckpointKeyFlatKey_Srv1FlatKeyCases(t *testing.T) {
	fx := loadSrv1Fixture(t)
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
