package basecamp

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed api-provenance.json
var provenanceJSON []byte

// APIProvenance describes which upstream API revisions the SDK was built against.
type APIProvenance struct {
	BC3    UpstreamRef `json:"bc3"`
	BC3API UpstreamRef `json:"bc3_api"`
}

// UpstreamRef is a git revision and the date it was synced.
type UpstreamRef struct {
	Revision string `json:"revision"`
	Date     string `json:"date"`
}

var (
	provenance     APIProvenance
	provenanceOnce sync.Once
)

// Provenance returns the upstream API revisions this SDK was built against.
func Provenance() APIProvenance {
	provenanceOnce.Do(func() {
		if err := json.Unmarshal(provenanceJSON, &provenance); err != nil {
			panic("basecamp: invalid embedded API provenance JSON: " + err.Error())
		}
	})
	return provenance
}
